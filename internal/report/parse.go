package report

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.yaml.in/yaml/v3"
)

func parse(data []byte) (*Report, error) {
	var document yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("report must contain a YAML mapping: %w", err)
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("report must contain exactly one YAML document")
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("report must be a YAML mapping")
	}
	if err := validateYAML(&document); err != nil {
		return nil, err
	}
	fields, err := mapping(document.Content[0], "report", "blocks")
	if err != nil {
		return nil, err
	}
	count := 0
	blocks, err := parseBlocks(fields["blocks"], "blocks", false, &count)
	if err != nil {
		return nil, err
	}
	return &Report{Blocks: blocks}, nil
}

func validateYAML(node *yaml.Node) error {
	type entry struct {
		node     *yaml.Node
		location string
	}
	pending := []entry{{node, "report"}}
	for len(pending) > 0 {
		last := len(pending) - 1
		item := pending[last]
		pending = pending[:last]
		node, location := item.node, item.location
		if node.Kind == yaml.AliasNode || node.Tag == "!!merge" {
			return fmt.Errorf("%s requires explicit values; YAML aliases and merge keys are unsupported", location)
		}
		if node.Kind == yaml.MappingNode {
			if node.Tag != "!!map" {
				return fmt.Errorf("%s: unsupported YAML tag %q", location, node.Tag)
			}
			seen := make(map[string]bool)
			for i := 0; i < len(node.Content); i += 2 {
				key := node.Content[i]
				if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
					return fmt.Errorf("%s keys must be strings; YAML merge keys are unsupported", location)
				}
				if seen[key.Value] {
					return fmt.Errorf("%s: duplicate key %q", location, key.Value)
				}
				seen[key.Value] = true
				pending = append(pending, entry{node.Content[i+1], location + "." + key.Value})
			}
			continue
		}
		if node.Kind == yaml.SequenceNode && node.Tag != "!!seq" {
			return fmt.Errorf("%s: unsupported YAML tag %q", location, node.Tag)
		}
		for i, child := range node.Content {
			childLocation := location
			if node.Kind == yaml.SequenceNode {
				childLocation = fmt.Sprintf("%s[%d]", location, i)
			}
			pending = append(pending, entry{child, childLocation})
		}
	}
	return nil
}

func mapping(node *yaml.Node, location string, allowed ...string) (map[string]*yaml.Node, error) {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s must be a mapping", location)
	}
	fields := make(map[string]*yaml.Node)
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		known := false
		for _, field := range allowed {
			known = known || key == field
		}
		if !known {
			return nil, fmt.Errorf("%s: unknown field %q", location, key)
		}
		fields[key] = node.Content[i+1]
	}
	return fields, nil
}

func nonemptyString(node *yaml.Node, location string) (string, error) {
	if node == nil || node.Kind != yaml.ScalarNode || node.Tag != "!!str" || strings.TrimSpace(node.Value) == "" {
		return "", fmt.Errorf("%s must be a nonempty string", location)
	}
	return node.Value, nil
}

func parseBlocks(node *yaml.Node, location string, insideRow bool, count *int) ([]Block, error) {
	if node == nil || node.Kind != yaml.SequenceNode || len(node.Content) == 0 {
		return nil, fmt.Errorf("%s must be a nonempty list of blocks", location)
	}
	blocks := make([]Block, 0, len(node.Content))
	for i, item := range node.Content {
		where := fmt.Sprintf("%s[%d]", location, i)
		*count += 1
		if *count > maxBlocks {
			return nil, fmt.Errorf("report exceeds %d blocks", maxBlocks)
		}
		if item.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%s must be a mapping", where)
		}
		var typeNode *yaml.Node
		for j := 0; j < len(item.Content); j += 2 {
			if item.Content[j].Value == "type" {
				typeNode = item.Content[j+1]
			}
		}
		kind, err := nonemptyString(typeNode, where+".type")
		if err != nil {
			return nil, err
		}
		allowed := []string{"type", "source"}
		switch kind {
		case "markdown":
		case "line":
			allowed = append(allowed, "title", "x", "y")
		case "row":
			if insideRow {
				return nil, fmt.Errorf("%s: nested rows are unsupported", where)
			}
			allowed = []string{"type", "blocks"}
		default:
			return nil, fmt.Errorf("%s: unknown block type %q", where, kind)
		}
		fields, err := mapping(item, where, allowed...)
		if err != nil {
			return nil, err
		}
		block := Block{Type: kind}
		if kind == "row" {
			block.Blocks, err = parseBlocks(fields["blocks"], where+".blocks", true, count)
		} else {
			block.Source, err = nonemptyString(fields["source"], where+".source")
			if err == nil {
				block.Source, err = cleanSource(block.Source)
			}
			if err != nil {
				return nil, fmt.Errorf("%s: %w", where, err)
			}
			if kind == "line" {
				if block.Title, err = nonemptyString(fields["title"], where+".title"); err != nil {
					return nil, err
				}
				if block.X, err = nonemptyString(fields["x"], where+".x"); err != nil {
					return nil, err
				}
				y := fields["y"]
				if y == nil || y.Kind != yaml.SequenceNode || len(y.Content) == 0 || len(y.Content) > maxSeries {
					return nil, fmt.Errorf("%s.y must be a list of 1 to %d column names", where, maxSeries)
				}
				seen := make(map[string]bool)
				for j, column := range y.Content {
					name, columnErr := nonemptyString(column, fmt.Sprintf("%s.y[%d]", where, j))
					if columnErr != nil {
						return nil, columnErr
					}
					if seen[name] {
						return nil, fmt.Errorf("%s.y: duplicate column %q", where, name)
					}
					seen[name] = true
					block.Y = append(block.Y, name)
				}
			}
		}
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}
