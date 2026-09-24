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
var createdAtPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`)

// Schema identifies the supported ExpLedger metadata format.
const Schema = "expledger/v1"

// Record holds an experiment's structured metadata.
type Record struct {
	Schema    string    `yaml:"schema"`
	ID        string    `yaml:"id"`
	Title     string    `yaml:"title"`
	CreatedAt time.Time `yaml:"created_at"`
	BasedOn   []string  `yaml:"based_on,omitempty"`
}

// Parse reads a single YAML metadata document with the supported schema.
// Custom fields are accepted but not retained in the record.
func Parse(data []byte) (Record, error) {
	var node yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&node); err != nil {
		return Record{}, fmt.Errorf("parse metadata: %w", err)
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Record{}, errors.New("metadata must contain exactly one YAML document")
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
		case "schema", "id", "title":
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
	return record, nil
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
