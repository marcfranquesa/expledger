// Package experiment defines experiment records and their YAML metadata format.
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
var timestampPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`)

// Schema identifies the supported ExpLedger metadata format.
const Schema = "expledger/v1"

// Record holds an experiment's structured metadata.
type Record struct {
	Schema    string      `yaml:"schema"`
	ID        string      `yaml:"id"`
	Title     string      `yaml:"title"`
	CreatedAt time.Time   `yaml:"created_at"`
	BasedOn   []string    `yaml:"based_on,omitempty"`
	LastRun   *RunReceipt `yaml:"last_run,omitempty"`
}

// Parse reads a single YAML metadata document with the supported schema.
// Custom fields are accepted but not retained in the record.
func Parse(data []byte) (Record, error) {
	record, _, err := parseMetadata(data)
	return record, err
}

func parseMetadata(data []byte) (Record, *yaml.Node, error) {
	var node yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&node); err != nil {
		return Record{}, nil, fmt.Errorf("parse metadata: %w", err)
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Record{}, nil, errors.New("metadata must contain exactly one YAML document")
	}
	if len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return Record{}, nil, errors.New("metadata must be a YAML mapping")
	}
	if err := validateYAML(&node); err != nil {
		return Record{}, nil, err
	}
	fields := node.Content[0].Content
	for i := 0; i < len(fields); i += 2 {
		if fields[i].Kind != yaml.ScalarNode || fields[i].Tag != "!!str" {
			return Record{}, nil, errors.New("metadata keys must be strings")
		}
		key, value := fields[i].Value, fields[i+1]
		switch key {
		case "schema", "id", "title":
			if value.Tag != "!!str" {
				return Record{}, nil, fmt.Errorf("%s must be a string", key)
			}
		case "created_at":
			if err := validateTimestampNode(key, value); err != nil {
				return Record{}, nil, err
			}
		case "last_run":
			if err := validateRunReceiptNode(value); err != nil {
				return Record{}, nil, err
			}
		case "based_on":
			if value.Kind != yaml.SequenceNode {
				return Record{}, nil, errors.New("based_on must be a list of strings")
			}
			for _, parent := range value.Content {
				if parent.Tag != "!!str" {
					return Record{}, nil, errors.New("based_on must be a list of strings")
				}
			}
		}
	}
	var record Record
	if err := node.Decode(&record); err != nil {
		return Record{}, nil, fmt.Errorf("decode metadata: %w", err)
	}
	if err := record.validate(); err != nil {
		return Record{}, nil, err
	}
	return record, &node, nil
}

// Marshal encodes the record's standard fields as one YAML metadata document.
func (r Record) Marshal() ([]byte, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	metadata, err := yaml.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("encode metadata: %w", err)
	}
	return metadata, nil
}

func (r Record) validate() error {
	if r.Schema == "" {
		return errors.New("schema is required; expected " + Schema)
	}
	if r.Schema != Schema {
		return fmt.Errorf("unsupported schema %q; expected %s", r.Schema, Schema)
	}
	if strings.TrimSpace(r.ID) == "" {
		return errors.New("id is required and must be a nonempty string")
	}
	if strings.TrimSpace(r.Title) == "" {
		return errors.New("title is required and must be a nonempty string")
	}
	if err := validateTimestamp("created_at", r.CreatedAt); err != nil {
		return err
	}
	if r.LastRun != nil {
		if err := r.LastRun.validate(); err != nil {
			return err
		}
	}
	for _, parent := range r.BasedOn {
		if strings.TrimSpace(parent) == "" {
			return errors.New("based_on entries must be nonempty strings")
		}
	}
	return nil
}

func validateTimestampNode(name string, node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode || (node.Tag != "!!str" && node.Tag != "!!timestamp") {
		return fmt.Errorf("%s must be an RFC3339 timestamp with a timezone", name)
	}
	if _, err := time.Parse(time.RFC3339Nano, node.Value); err != nil || !timestampPattern.MatchString(node.Value) {
		return fmt.Errorf("%s must be an RFC3339 timestamp with a timezone (for example, 2026-09-24T14:30:00Z); got %q", name, node.Value)
	}
	return nil
}

func validateTimestamp(name string, value time.Time) error {
	if value.IsZero() {
		return fmt.Errorf("%s is required and must be a nonzero RFC3339 timestamp with a timezone", name)
	}
	if _, err := value.MarshalText(); err != nil {
		return fmt.Errorf("%s must be an RFC3339 timestamp with a timezone: %w", name, err)
	}
	if _, offset := value.Zone(); offset%60 != 0 {
		return fmt.Errorf("%s timezone offset must be a whole number of minutes", name)
	}
	return nil
}

func validateYAML(node *yaml.Node) error {
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
