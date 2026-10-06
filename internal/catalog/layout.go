package catalog

import (
	"fmt"
	"strings"
	"time"

	"github.com/lestrrat-go/strftime"
)

// Layout controls where experiments live and how new experiment IDs are named.
type Layout struct {
	ExperimentsDir   string `yaml:"experiments_dir,omitempty"`
	ExperimentFormat string `yaml:"experiment_format,omitempty"`
}

// WithDefaults fills omitted settings with the original flat experiment layout.
func (l Layout) WithDefaults() Layout {
	if l.ExperimentsDir == "" {
		l.ExperimentsDir = "experiments"
	}
	if l.ExperimentFormat == "" {
		l.ExperimentFormat = "{date:%Y%m%d}-{slug}"
	}
	return l
}

// Validate checks project-relative paths and formats with optional slug, date, and time placeholders.
func (l Layout) Validate() error {
	l = l.WithDefaults()
	if !validRelativePath(l.ExperimentsDir) {
		return fmt.Errorf("experiments_dir must be a nonempty relative path without dot segments, backslashes, or empty components")
	}
	id, err := formatExperimentID(l.ExperimentFormat, "slug", time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC))
	if err != nil {
		return fmt.Errorf("experiment_format: %w", err)
	}
	if err := validateID(id); err != nil {
		return fmt.Errorf("experiment_format: %w", err)
	}
	return nil
}

func formatExperimentID(format, slug string, now time.Time) (string, error) {
	var result strings.Builder
	for format != "" {
		literal, remaining, found := strings.Cut(format, "{")
		if strings.Contains(literal, "}") {
			return "", fmt.Errorf("unmatched closing brace")
		}
		result.WriteString(literal)
		if !found {
			break
		}
		token, rest, closed := strings.Cut(remaining, "}")
		if !closed || strings.Contains(token, "{") {
			return "", fmt.Errorf("unclosed or nested placeholder")
		}
		key, layout, custom := strings.Cut(token, ":")
		switch key {
		case "slug":
			if custom {
				return "", fmt.Errorf("{slug} does not accept a format")
			}
			result.WriteString(slug)
		case "date", "time":
			if custom && layout == "" {
				return "", fmt.Errorf("{%s:...} requires a nonempty strftime format", key)
			}
			if !custom {
				layout = "%Y-%m-%d"
				if key == "time" {
					layout = "%H-%M-%S"
				}
			}
			value, err := strftime.Format(layout, now)
			if err != nil {
				return "", fmt.Errorf("invalid %s format %q: %w", key, layout, err)
			}
			result.WriteString(value)
		default:
			return "", fmt.Errorf("unknown placeholder {%s}; use {slug}, {date}, or {time}", token)
		}
		format = rest
	}
	return result.String(), nil
}
