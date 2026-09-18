package config

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/bel0v/paleo-git/runner"
	"github.com/bel0v/paleo-git/runner/builtins"
	runnerexec "github.com/bel0v/paleo-git/runner/exec"
	"github.com/bel0v/paleo-git/vcs"
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
	// Bucket is what Every strides over: individual commits, or the latest
	// commit of each UTC calendar day, week or month.
	Bucket Bucket `yaml:"bucket"`
	Every  int    `yaml:"every"`
}

// Bucket is the unit a traversal is sampled in.
type Bucket string

const (
	BucketCommit Bucket = "commit"
	BucketDay    Bucket = "day"
	BucketWeek   Bucket = "week"
	BucketMonth  Bucket = "month"
)

// IsValid reports whether b is a known bucket.
func (b Bucket) IsValid() bool {
	switch b {
	case BucketCommit, BucketDay, BucketWeek, BucketMonth:
		return true
	}
	return false
}

type Metric struct {
	ID          string    `yaml:"id"`
	Description string    `yaml:"description,omitempty"`
	Traversal   string    `yaml:"traversal"`
	Paths       Paths     `yaml:"paths"`
	Runner      RunnerRef `yaml:"runner"`
	Output      Output    `yaml:"output,omitempty"`
}

// Output controls what a measurement emits beyond its value. It does not
// affect the value itself and is therefore not part of MetricHash.
type Output struct {
	Files FilesOutput `yaml:"files,omitempty"`
}

// FilesOutput says what a metric emits for the files it matched.
type FilesOutput string

const (
	// FilesList emits the matching file paths (the default).
	FilesList FilesOutput = "list"
	// FilesNone drops them from results and the store.
	FilesNone FilesOutput = "none"
)

// IsValid reports whether f is a known value; the empty value means FilesList.
func (f FilesOutput) IsValid() bool {
	switch f {
	case "", FilesList, FilesNone:
		return true
	}
	return false
}

// EmitsFiles reports whether results for this metric carry file paths.
func (m Metric) EmitsFiles() bool {
	return m.Output.Files != FilesNone
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
		} else if vcs.IsDate(tr.Range.Start) {
			if _, err := time.Parse("2006-01-02", tr.Range.Start); err != nil {
				errs = append(errs, fmt.Sprintf("traversals[%s].range.start: invalid date %q", name, tr.Range.Start))
			}
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
		if !tr.Sampling.Bucket.IsValid() {
			errs = append(errs, fmt.Sprintf("traversals[%s].sampling.bucket: invalid value %q (supported: %s, %s, %s, %s)", name, tr.Sampling.Bucket, BucketCommit, BucketDay, BucketWeek, BucketMonth))
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
		switch {
		case hasBuiltin && hasExec:
			errs = append(errs, fmt.Sprintf("metrics[%d].runner: must specify exactly one of builtin or exec, not both", i))
		case !hasBuiltin && !hasExec:
			errs = append(errs, fmt.Sprintf("metrics[%d].runner: must specify exactly one of builtin or exec", i))
		default:
			if err := validateRunner(m.Runner); err != nil {
				errs = append(errs, fmt.Sprintf("metrics[%d].runner: %v", i, err))
			}
		}

		if len(m.Paths.Include) == 0 {
			errs = append(errs, fmt.Sprintf("metrics[%d].paths.include: must not be empty", i))
		}

		if !m.Output.Files.IsValid() {
			errs = append(errs, fmt.Sprintf("metrics[%d].output.files: invalid value %q (supported: %s, %s)", i, m.Output.Files, FilesList, FilesNone))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation errors:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// ResolveRunner instantiates the runner a metric refers to. The RunnerRef
// must have passed Validate.
func ResolveRunner(ref RunnerRef) (runner.Runner, error) {
	if ref.Builtin != "" {
		r, ok := builtins.New(ref.Builtin)
		if !ok {
			return nil, fmt.Errorf("unknown builtin runner %q (supported: %s)", ref.Builtin, strings.Join(builtins.Names(), ", "))
		}
		return r, nil
	}
	if len(ref.Exec) > 0 {
		return runnerexec.New(ref.Exec), nil
	}
	return nil, fmt.Errorf("runner must specify builtin or exec")
}

func validateRunner(ref RunnerRef) error {
	r, err := ResolveRunner(ref)
	if err != nil {
		return err
	}
	return r.Validate(ref.Config)
}
