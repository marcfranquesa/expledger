package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"slices"
	"strings"
	"time"
)

func gitRoot(ctx context.Context, cwd string, isolateRepository bool) (string, error) {
	output, err := gitOutput(ctx, cwd, isolateRepository, "rev-parse", "--show-toplevel")
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			detail := strings.TrimSpace(string(exitErr.Stderr))
			if strings.HasPrefix(detail, "fatal: not a git repository (or any") {
				return "", fmt.Errorf("no Git repository found in %q or its parent directories\nRun expledger from an existing Git repository, or run 'git init' in your project directory first.", cwd)
			}
			if detail != "" {
				return "", fmt.Errorf("find Git working tree: %s: %w", detail, err)
			}
		}
		return "", fmt.Errorf("find Git working tree: %w", err)
	}
	return output, nil
}

func gitOutput(ctx context.Context, cwd string, isolateRepository bool, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = cwd
	cmd.Env = cmd.Environ()
	if isolateRepository {
		cmd.Env = append(gitEnvironment(cmd.Env), "GIT_NO_REPLACE_OBJECTS=1")
	}
	// Keep diagnostics stable and bound inherited pipes after cancellation or exit.
	cmd.Env = append(cmd.Env, "LC_ALL=C")
	cmd.WaitDelay = time.Second
	output, err := cmd.Output()
	if err != nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	return strings.TrimSuffix(strings.TrimSuffix(string(output), "\n"), "\r"), err
}

func remoteLocation(ctx context.Context, root string) (repositoryURL, branch string, err error) {
	branch, err = gitOutput(ctx, root, false, "branch", "--show-current")
	if err != nil {
		return "", "", fmt.Errorf("read current Git branch: %w", err)
	}
	if branch == "" {
		return "", "", nil
	}
	remotes, err := gitOutput(ctx, root, false, "remote")
	if err != nil {
		return "", "", fmt.Errorf("read Git remotes: %w", err)
	}
	if !slices.Contains(strings.Split(remotes, "\n"), "origin") {
		return "", branch, nil
	}
	remote, err := gitOutput(ctx, root, false, "remote", "get-url", "origin")
	if err != nil {
		return "", "", fmt.Errorf("read origin remote: %w", err)
	}
	repositoryURL, err = githubRepositoryURL(remote)
	if err != nil {
		return "", branch, nil
	}
	return repositoryURL, branch, nil
}

func githubRepositoryURL(remote string) (string, error) {
	if host, path, ok := strings.Cut(remote, ":"); ok && strings.EqualFold(host, "git@github.com") {
		remote = "ssh://git@github.com/" + path
	}
	u, err := url.Parse(remote)
	if err != nil || (u.Scheme != "https" && u.Scheme != "ssh") || !strings.EqualFold(u.Hostname(), "github.com") || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("origin must be a GitHub HTTPS or SSH remote")
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/"), "/")
	if len(parts) != 2 {
		return "", errors.New("origin must point to a GitHub repository (owner/repository)")
	}
	parts[1] = strings.TrimSuffix(parts[1], ".git")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", errors.New("origin must point to a GitHub repository (owner/repository)")
		}
	}
	return "https://github.com/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]), nil
}

func gitEnvironment(env []string) []string {
	clean := make([]string, 0, len(env))
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "GIT_CONFIG") {
			continue
		}
		switch name {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE",
			"GIT_LITERAL_PATHSPECS", "GIT_GLOB_PATHSPECS", "GIT_NOGLOB_PATHSPECS", "GIT_ICASE_PATHSPECS",
			"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_GRAFT_FILE",
			"GIT_SHALLOW_FILE", "GIT_REPLACE_REF_BASE", "GIT_PREFIX", "GIT_IMPLICIT_WORK_TREE",
			"GIT_CEILING_DIRECTORIES", "GIT_DISCOVERY_ACROSS_FILESYSTEM":
			continue
		}
		clean = append(clean, entry)
	}
	return clean
}

func runGit(ctx context.Context, cwd string, args ...string) (string, error) {
	output, err := gitOutput(ctx, cwd, true, args...)
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return "", fmt.Errorf("git: %s: %w", strings.TrimSpace(string(exit.Stderr)), err)
	}
	return output, err
}
