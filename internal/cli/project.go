package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/marcfranquesa/expledger/internal/catalog"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

type projectConfig struct {
	catalog.Layout `yaml:",inline"`
	RemoteURL      string   `yaml:"remote_url,omitempty"`
	Sources        []string `yaml:"sources,omitempty"`
}

func readProjectConfig(path string) (projectConfig, error) {
	config := projectConfig{Layout: (catalog.Layout{}).WithDefaults()}
	info, err := os.Stat(path)
	if err != nil {
		return config, err
	}
	if !info.Mode().IsRegular() {
		return config, errors.New("project config must be a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return config, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil && !errors.Is(err, io.EOF) {
		return config, fmt.Errorf("parse project config: %w", err)
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return config, errors.New("project config must contain exactly one YAML document")
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return config, errors.New("project config must be a YAML mapping (use {} for a local-only project)")
	}
	fields := document.Content[0].Content
	for i := 0; i < len(fields); i += 2 {
		key, value := fields[i], fields[i+1]
		if key.Value == "schema" && value.Value == "expledger/v1" {
			return config, errors.New("old experiment filename: rename this experiment's expledger.yaml to experiment.yaml, then run expledger init at the project root")
		}
	}
	seen := make(map[string]bool)
	for i := 0; i < len(fields); i += 2 {
		key, value := fields[i], fields[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || (key.Value != "remote_url" && key.Value != "sources" && key.Value != "experiments_dir" && key.Value != "experiment_format") {
			return config, fmt.Errorf("unknown project setting %q; supported settings are remote_url, sources, experiments_dir, and experiment_format", key.Value)
		}
		if seen[key.Value] {
			return config, fmt.Errorf("duplicate project setting %s", key.Value)
		}
		seen[key.Value] = true
		switch key.Value {
		case "remote_url":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return config, errors.New("remote_url must be a string")
			}
			config.RemoteURL = value.Value
		case "experiments_dir", "experiment_format":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return config, fmt.Errorf("%s must be a string", key.Value)
			}
			if value.Value == "" {
				return config, fmt.Errorf("%s must not be empty", key.Value)
			}
			if key.Value == "experiments_dir" {
				config.ExperimentsDir = value.Value
			} else {
				config.ExperimentFormat = value.Value
			}
		case "sources":
			if value.Kind != yaml.SequenceNode || len(value.Content) == 0 {
				return config, errors.New("sources must be a nonempty list of project-root paths")
			}
			for _, source := range value.Content {
				if source.Kind != yaml.ScalarNode || source.Tag != "!!str" || strings.TrimSpace(source.Value) == "" {
					return config, errors.New("sources entries must be nonempty strings")
				}
				config.Sources = append(config.Sources, source.Value)
			}
		}
	}
	if err := validateRemoteURL(config.RemoteURL); err != nil {
		return config, err
	}
	if err := config.Layout.Validate(); err != nil {
		return config, err
	}
	return config, nil
}

func validateRemoteURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(raw, "#") || strings.ContainsAny(raw, "\\ \t\r\n") {
		return errors.New("remote_url must be an HTTP(S) experiments-directory URL with a host and no credentials, query, or fragment")
	}
	if port := u.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return errors.New("remote_url has an invalid port")
		}
	}
	host := u.Hostname()
	if net.ParseIP(host) == nil {
		for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
			if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return errors.New("remote_url has an invalid host")
			}
			for _, char := range label {
				if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-') {
					return errors.New("remote_url has an invalid host; use an ASCII hostname or IP address")
				}
			}
		}
	}
	return nil
}

func discoverProject(cwd string) (string, projectConfig, error) {
	root, err := filepath.Abs(cwd)
	if err != nil {
		return "", projectConfig{}, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", projectConfig{}, err
	}
	for {
		path := filepath.Join(root, "expledger.yaml")
		_, err := os.Lstat(path)
		if err == nil {
			config, err := readProjectConfig(path)
			if err != nil {
				return "", config, fmt.Errorf("read %s: %w", path, err)
			}
			return root, config, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", projectConfig{}, fmt.Errorf("read %s: %w", path, err)
		}
		parent := filepath.Dir(root)
		if parent == root {
			return "", projectConfig{}, fmt.Errorf("no expledger.yaml found in %q or its parents; run expledger init in your project directory", cwd)
		}
		root = parent
	}
}

func initProjectCommand(app *application) *cobra.Command {
	var remote string
	cmd := &cobra.Command{
		Use: "init", Short: "Initialize an ExpLedger project in the current directory",
		Long:    "Create expledger.yaml in the current directory. Existing valid config is left unchanged.\nUse --remote-url for the full browser URL of the remote experiments directory.",
		Example: "  expledger init\n  expledger init --remote-url https://github.com/owner/repo/tree/main/experiments",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateRemoteURL(remote); err != nil {
				return err
			}
			path := filepath.Join(app.cwd, "expledger.yaml")
			file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if errors.Is(err, os.ErrExist) {
				config, err := readProjectConfig(path)
				if err != nil {
					return fmt.Errorf("read %s: %w", path, err)
				}
				if cmd.Flags().Changed("remote-url") && remote != config.RemoteURL {
					return fmt.Errorf("%s already exists with a different remote_url; edit it explicitly", path)
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), path)
				return err
			}
			if err != nil {
				return fmt.Errorf("create %s: %w", path, err)
			}
			data, err := yaml.Marshal(projectConfig{RemoteURL: remote})
			if err == nil {
				_, err = file.Write(data)
			}
			if err := errors.Join(err, file.Close()); err != nil {
				return fmt.Errorf("write %s: %w", path, errors.Join(err, os.Remove(path)))
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), path)
			return err
		},
	}
	cmd.Flags().StringVar(&remote, "remote-url", "", "Full browser `URL` of the remote experiments directory")
	return cmd
}
