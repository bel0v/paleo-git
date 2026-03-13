package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version    int                  `yaml:"version"`
	Traversals map[string]Traversal `yaml:"traversals"`
	Metrics    []Metric             `yaml:"metrics"`
}

type Traversal struct {
	Range    Range    `yaml:"range"`
	Mode     string   `yaml:"mode"`
	Sampling Sampling `yaml:"sampling"`
}

type Range struct {
	Start string `yaml:"start"`
	End   string `yaml:"end"`
}

type Sampling struct {
	Every int `yaml:"every"`
}

type Metric struct {
	ID          string    `yaml:"id"`
	Description string    `yaml:"description,omitempty"`
	Traversal   string    `yaml:"traversal"`
	Paths       Paths     `yaml:"paths"`
	Runner      RunnerRef `yaml:"runner"`
}

type Paths struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude,omitempty"`
}

type RunnerRef struct {
	Builtin string         `yaml:"builtin,omitempty"`
	Exec    []string       `yaml:"exec,omitempty"`
	Config  map[string]any `yaml:"config,omitempty"`
}

func Parse(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("config parse error: %w", err)
	}
	return cfg, nil
}

var validModes = map[string]bool{
	"first_parent": true,
}

func Validate(cfg Config) error {
	var errs []string

	if cfg.Version != 1 {
		errs = append(errs, fmt.Sprintf("version: unsupported version %d (supported: 1)", cfg.Version))
	}

	for name, tr := range cfg.Traversals {
		if tr.Range.Start == "" {
			errs = append(errs, fmt.Sprintf("traversals[%s].range.start: must not be empty", name))
		}
		if tr.Range.End == "" {
			errs = append(errs, fmt.Sprintf("traversals[%s].range.end: must not be empty", name))
		}
		if !validModes[tr.Mode] {
			errs = append(errs, fmt.Sprintf("traversals[%s].mode: invalid mode %q (supported: first_parent)", name, tr.Mode))
		}
		if tr.Sampling.Every < 1 {
			errs = append(errs, fmt.Sprintf("traversals[%s].sampling.every: must be at least 1 (got %d)", name, tr.Sampling.Every))
		}
	}

	seenIDs := make(map[string]int)
	for i, m := range cfg.Metrics {
		if prev, ok := seenIDs[m.ID]; ok {
			errs = append(errs, fmt.Sprintf("metrics[%d].id: duplicate metric id %q (first defined at metrics[%d])", i, m.ID, prev))
		}
		seenIDs[m.ID] = i

		if _, ok := cfg.Traversals[m.Traversal]; !ok {
			errs = append(errs, fmt.Sprintf("metrics[%d].traversal: references nonexistent traversal %q", i, m.Traversal))
		}

		hasBuiltin := m.Runner.Builtin != ""
		hasExec := len(m.Runner.Exec) > 0
		if hasBuiltin && hasExec {
			errs = append(errs, fmt.Sprintf("metrics[%d].runner: must specify exactly one of builtin or exec, not both", i))
		}
		if !hasBuiltin && !hasExec {
			errs = append(errs, fmt.Sprintf("metrics[%d].runner: must specify exactly one of builtin or exec", i))
		}

		if len(m.Paths.Include) == 0 {
			errs = append(errs, fmt.Sprintf("metrics[%d].paths.include: must not be empty", i))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation errors:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}
