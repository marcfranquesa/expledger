package experiment

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"go.yaml.in/yaml/v3"
)

var projectCommitPattern = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

// RunReceipt records the checkout baseline and project changes observed at launch.
type RunReceipt struct {
	ProjectCommit string    `yaml:"project_commit"`
	ProjectDirty  bool      `yaml:"project_dirty"`
	StartedAt     time.Time `yaml:"started_at"`
}

// WithRunReceipt updates only last_run in a valid metadata document.
// Other YAML values and comments are retained, but layout may change.
func WithRunReceipt(data []byte, receipt RunReceipt) ([]byte, error) {
	_, document, err := parseMetadata(data)
	if err != nil {
		return nil, err
	}
	if err := receipt.validate(); err != nil {
		return nil, err
	}
	var replacement yaml.Node
	if err := replacement.Encode(receipt); err != nil {
		return nil, fmt.Errorf("encode run receipt: %w", err)
	}
	mapping := document.Content[0]
	found := false
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == "last_run" {
			previous := mapping.Content[i+1]
			replacement.HeadComment = previous.HeadComment
			replacement.LineComment = previous.LineComment
			replacement.FootComment = previous.FootComment
			mapping.Content[i+1] = &replacement
			found = true
			break
		}
	}
	if !found {
		mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "last_run"}, &replacement)
	}
	explicitEmptyNulls(document)
	updated, err := yaml.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode metadata: %w", err)
	}
	return updated, nil
}

func explicitEmptyNulls(node *yaml.Node) {
	// yaml.v3 otherwise emits empty nulls in flow collections as empty strings.
	if node.Kind == yaml.ScalarNode && node.Tag == "!!null" && node.Value == "" {
		node.Style |= yaml.TaggedStyle
	}
	for _, child := range node.Content {
		explicitEmptyNulls(child)
	}
}

func (r RunReceipt) validate() error {
	if !projectCommitPattern.MatchString(r.ProjectCommit) {
		return errors.New("last_run.project_commit must be a full lowercase Git commit hash")
	}
	return validateTimestamp("last_run.started_at", r.StartedAt)
}

func validateRunReceiptNode(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return errors.New("last_run must be a mapping with project_commit and started_at")
	}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
			return errors.New("last_run keys must be strings")
		}
		switch key.Value {
		case "project_commit":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return errors.New("last_run.project_commit must be a string")
			}
		case "project_dirty":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!bool" {
				return errors.New("last_run.project_dirty must be a boolean")
			}
		case "started_at":
			if err := validateTimestampNode("last_run.started_at", value); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown last_run field %q", key.Value)
		}
	}
	return nil
}
