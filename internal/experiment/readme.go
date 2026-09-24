package experiment

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// time.Parse also accepts single-digit hours, comma fractions, and out-of-range offsets.
var createdAtPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`)

// Record holds structured metadata and the unchanged Markdown body.
// Extra retains metadata fields added by people or later versions of ExpLedger.
type Record struct {
	ID        string               `yaml:"id"`
	Title     string               `yaml:"title"`
	CreatedAt time.Time            `yaml:"created_at"`
	BasedOn   []string             `yaml:"based_on,omitempty"`
	Extra     map[string]yaml.Node `yaml:",inline"`
	Body      []byte               `yaml:"-"`
}

// Parse reads YAML front matter delimited by --- lines, followed by Markdown.
func Parse(data []byte) (Record, error) {
	header, body, err := splitFrontMatter(data)
	if err != nil {
		return Record{}, err
	}
	var node yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(header))
	if err := decoder.Decode(&node); err != nil {
		return Record{}, fmt.Errorf("parse metadata: %w", err)
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Record{}, errors.New("front matter must contain exactly one YAML document")
	}
	if len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return Record{}, errors.New("metadata must be a YAML mapping")
	}
	if err := validateYAML(&node); err != nil {
		return Record{}, err
	}
	fields := node.Content[0].Content
	for i := 0; i < len(fields); i += 2 {
		if fields[i].Kind != yaml.ScalarNode || fields[i].Tag != "!!str" {
			return Record{}, errors.New("metadata keys must be strings")
		}
		key, value := fields[i].Value, fields[i+1]
		switch key {
		case "id", "title":
			if value.Tag != "!!str" {
				return Record{}, fmt.Errorf("%s must be a string", key)
			}
		case "created_at":
			if value.Kind != yaml.ScalarNode || (value.Tag != "!!str" && value.Tag != "!!timestamp") {
				return Record{}, errors.New("created_at must be an RFC3339 timestamp with a timezone")
			}
			if _, err := time.Parse(time.RFC3339Nano, value.Value); err != nil || !createdAtPattern.MatchString(value.Value) {
				return Record{}, fmt.Errorf("created_at must be an RFC3339 timestamp with a timezone (for example, 2026-09-24T14:30:00Z); got %q", value.Value)
			}
		case "based_on":
			if value.Kind != yaml.SequenceNode {
				return Record{}, errors.New("based_on must be a list of strings")
			}
			for _, parent := range value.Content {
				if parent.Tag != "!!str" {
					return Record{}, errors.New("based_on must be a list of strings")
				}
			}
		}
	}
	var record Record
	if err := node.Decode(&record); err != nil {
		return Record{}, fmt.Errorf("decode metadata: %w", err)
	}
	if err := record.validate(); err != nil {
		return Record{}, err
	}
	record.Body = body
	return record, nil
}

// Marshal encodes metadata and appends the Markdown body byte for byte.
// YAML formatting and comments are not retained when metadata is re-encoded.
func (r Record) Marshal() ([]byte, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	extra := make(map[string]yaml.Node, len(r.Extra))
	for key, node := range r.Extra {
		extra[key] = withExplicitNulls(node)
	}
	r.Extra = extra
	metadata, err := yaml.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("encode metadata: %w", err)
	}
	data := append([]byte("---\n"), metadata...)
	data = append(data, []byte("---\n")...)
	return append(data, r.Body...), nil
}

func withExplicitNulls(node yaml.Node) yaml.Node {
	// yaml.v3 otherwise emits empty nulls in flow collections as empty strings.
	if node.Kind == yaml.ScalarNode && node.Tag == "!!null" && node.Value == "" {
		node.Style |= yaml.TaggedStyle
	}
	if node.Content != nil {
		content := make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			copy := withExplicitNulls(*child)
			content[i] = &copy
		}
		node.Content = content
	}
	return node
}

func (r Record) validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return errors.New("id is required and must be a nonempty string")
	}
	if strings.TrimSpace(r.Title) == "" {
		return errors.New("title is required and must be a nonempty string")
	}
	if r.CreatedAt.IsZero() {
		return errors.New("created_at is required and must be a nonzero RFC3339 timestamp with a timezone")
	}
	if _, err := r.CreatedAt.MarshalText(); err != nil {
		return fmt.Errorf("created_at must be an RFC3339 timestamp with a timezone: %w", err)
	}
	if _, offset := r.CreatedAt.Zone(); offset%60 != 0 {
		return errors.New("created_at timezone offset must be a whole number of minutes")
	}
	for _, parent := range r.BasedOn {
		if strings.TrimSpace(parent) == "" {
			return errors.New("based_on entries must be nonempty strings")
		}
	}
	for _, key := range []string{"id", "title", "created_at", "based_on"} {
		if _, exists := r.Extra[key]; exists {
			return fmt.Errorf("extra metadata cannot override %s", key)
		}
	}
	for _, value := range r.Extra {
		if err := validateYAML(&value); err != nil {
			return err
		}
	}
	return nil
}

func validateYAML(node *yaml.Node) error {
	// Explicit fields keep references valid when metadata is re-encoded.
	if node.Kind == yaml.AliasNode || node.Tag == "!!merge" {
		return errors.New("metadata requires explicit values; YAML aliases and merge keys are unsupported")
	}
	for _, child := range node.Content {
		if err := validateYAML(child); err != nil {
			return err
		}
	}
	return nil
}

func splitFrontMatter(data []byte) ([]byte, []byte, error) {
	first, rest, found := bytes.Cut(data, []byte("\n"))
	if !found || string(bytes.TrimSuffix(first, []byte("\r"))) != "---" {
		return nil, nil, errors.New("README must begin with a --- line")
	}
	start := len(data) - len(rest)
	for offset := start; offset < len(data); {
		line, tail, hasNewline := bytes.Cut(data[offset:], []byte("\n"))
		if string(bytes.TrimSuffix(line, []byte("\r"))) == "---" {
			return data[start:offset], tail, nil
		}
		if !hasNewline {
			break
		}
		offset = len(data) - len(tail)
	}
	return nil, nil, errors.New("YAML front matter is missing its closing --- line")
}
