package config

import (
	"strings"
	"testing"
)

const minimalValidYAML = `
version: 1
traversals:
  default:
    range:
      start: "main~100"
      end: "HEAD"
    mode: first_parent
    sampling:
      bucket: commit
      every: 10
metrics:
  - id: legacy-imports
    traversal: default
    output: { files: list }
    paths:
      include: ["src/**/*.ts"]
    runner:
      builtin: git_grep_count
      config:
        pattern: "from '@legacy/"
`

func mustParse(t *testing.T, yaml string) Config {
	t.Helper()
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return cfg
}

func TestParseConfig_MinimalValidConfig(t *testing.T) {
	cfg, err := Parse([]byte(minimalValidYAML))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if len(cfg.Traversals) != 1 {
		t.Fatalf("expected 1 traversal, got %d", len(cfg.Traversals))
	}
	tr := cfg.Traversals["default"]
	if tr.Range.Start != "main~100" {
		t.Errorf("expected start main~100, got %s", tr.Range.Start)
	}
	if tr.Range.End != "HEAD" {
		t.Errorf("expected end HEAD, got %s", tr.Range.End)
	}
	if tr.Mode != "first_parent" {
		t.Errorf("expected mode first_parent, got %s", tr.Mode)
	}
	if tr.Sampling.Every != 10 {
		t.Errorf("expected sampling every 10, got %d", tr.Sampling.Every)
	}
	if len(cfg.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(cfg.Metrics))
	}
	m := cfg.Metrics[0]
	if m.ID != "legacy-imports" {
		t.Errorf("expected id legacy-imports, got %s", m.ID)
	}
	if m.Traversal != "default" {
		t.Errorf("expected traversal default, got %s", m.Traversal)
	}
	if m.Runner.Builtin != "git_grep_count" {
		t.Errorf("expected builtin git_grep_count, got %s", m.Runner.Builtin)
	}
}

func TestValidateConfig_TraversalReferenceMustExist(t *testing.T) {
	yaml := `
version: 1
traversals:
  default:
    range: { start: "main~100", end: "HEAD" }
    mode: first_parent
    sampling: { bucket: commit, every: 10 }
metrics:
  - id: test
    traversal: nonexistent
    output: { files: list }
    paths:
      include: ["src/**"]
    runner:
      builtin: git_grep_count
      config:
        pattern: "foo"
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for nonexistent traversal")
	}
	if !strings.Contains(err.Error(), "metrics[0]") {
		t.Errorf("expected error to reference metrics[0], got: %v", err)
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("expected error to mention nonexistent traversal, got: %v", err)
	}
}

func TestValidateConfig_RunnerMustBeExactlyOneOfBuiltinOrExec(t *testing.T) {
	// Both builtin and exec
	yamlBoth := `
version: 1
traversals:
  default:
    range: { start: "main~100", end: "HEAD" }
    mode: first_parent
    sampling: { bucket: commit, every: 10 }
metrics:
  - id: test
    traversal: default
    output: { files: list }
    paths:
      include: ["src/**"]
    runner:
      builtin: git_grep_count
      exec: ["node", "check.js"]
`
	cfg, err := Parse([]byte(yamlBoth))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error when both builtin and exec are set")
	}
	if !strings.Contains(err.Error(), "metrics[0].runner") {
		t.Errorf("expected error to reference metrics[0].runner, got: %v", err)
	}

	// Neither builtin nor exec
	yamlNeither := `
version: 1
traversals:
  default:
    range: { start: "main~100", end: "HEAD" }
    mode: first_parent
    sampling: { bucket: commit, every: 10 }
metrics:
  - id: test
    traversal: default
    output: { files: list }
    paths:
      include: ["src/**"]
    runner: {}
`
	cfg, err = Parse([]byte(yamlNeither))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error when neither builtin nor exec is set")
	}
	if !strings.Contains(err.Error(), "metrics[0].runner") {
		t.Errorf("expected error to reference metrics[0].runner, got: %v", err)
	}
}

func TestValidateConfig_PathsMustHaveIncludes(t *testing.T) {
	yaml := `
version: 1
traversals:
  default:
    range: { start: "main~100", end: "HEAD" }
    mode: first_parent
    sampling: { bucket: commit, every: 10 }
metrics:
  - id: test
    traversal: default
    output: { files: list }
    paths:
      include: []
    runner:
      builtin: git_grep_count
      config:
        pattern: "foo"
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for empty paths.include")
	}
	if !strings.Contains(err.Error(), "metrics[0].paths.include") {
		t.Errorf("expected error to reference metrics[0].paths.include, got: %v", err)
	}
}

func TestValidateConfig_MultipleTraversalsAndMetrics(t *testing.T) {
	yaml := `
version: 1
traversals:
  full:
    range: { start: "main~2000", end: "HEAD" }
    mode: first_parent
    sampling: { bucket: commit, every: 25 }
  recent:
    range: { start: "2025-11-01", end: "HEAD" }
    mode: first_parent
    sampling: { bucket: commit, every: 5 }
metrics:
  - id: legacy-imports
    traversal: full
    output: { files: list }
    paths:
      include: ["src/**/*.ts"]
    runner:
      builtin: git_grep_count
      config:
        pattern: "from '@legacy/"
  - id: new-migration
    traversal: recent
    output: { files: list }
    paths:
      include: ["src/**/*.tsx"]
      exclude: ["**/*.test.*"]
    runner:
      exec: ["node", "check.js"]
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err = Validate(cfg)
	if err != nil {
		t.Fatalf("expected no validation error, got: %v", err)
	}
	if len(cfg.Traversals) != 2 {
		t.Errorf("expected 2 traversals, got %d", len(cfg.Traversals))
	}
	if len(cfg.Metrics) != 2 {
		t.Errorf("expected 2 metrics, got %d", len(cfg.Metrics))
	}
}

func TestValidateConfig_TraversalRangeMustHaveStartAndEnd(t *testing.T) {
	yaml := `
version: 1
traversals:
  default:
    range: { start: "", end: "HEAD" }
    mode: first_parent
    sampling: { bucket: commit, every: 10 }
metrics:
  - id: test
    traversal: default
    output: { files: list }
    paths:
      include: ["src/**"]
    runner:
      builtin: git_grep_count
      config:
        pattern: "foo"
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for empty range.start")
	}
	if !strings.Contains(err.Error(), "traversals[default].range.start") {
		t.Errorf("expected error to reference traversals[default].range.start, got: %v", err)
	}
}

func TestValidateConfig_ModeMustBeValid(t *testing.T) {
	yaml := `
version: 1
traversals:
  default:
    range: { start: "main~100", end: "HEAD" }
    mode: invalid_mode
    sampling: { bucket: commit, every: 10 }
metrics:
  - id: test
    traversal: default
    output: { files: list }
    paths:
      include: ["src/**"]
    runner:
      builtin: git_grep_count
      config:
        pattern: "foo"
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for invalid mode")
	}
}

func TestParseConfig_MalformedYAML(t *testing.T) {
	_, err := Parse([]byte("not: [valid: yaml"))
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestParseConfig_EmptyInput(t *testing.T) {
	cfg, err := Parse([]byte(""))
	if err != nil {
		t.Fatalf("expected no error for empty input, got: %v", err)
	}
	// Empty config should have zero values
	if cfg.Version != 0 {
		t.Errorf("expected version 0, got %d", cfg.Version)
	}
}

func TestValidateConfig_UnsupportedVersion(t *testing.T) {
	yaml := `
version: 99
traversals:
  default:
    range: { start: "main~100", end: "HEAD" }
    mode: first_parent
    sampling: { bucket: commit, every: 10 }
metrics:
  - id: test
    traversal: default
    output: { files: list }
    paths:
      include: ["src/**"]
    runner:
      builtin: git_grep_count
      config:
        pattern: "foo"
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for unsupported version")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("expected error to mention version, got: %v", err)
	}
}

func TestValidateConfig_SamplingMustBePositive(t *testing.T) {
	yaml := `
version: 1
traversals:
  default:
    range: { start: "main~100", end: "HEAD" }
    mode: first_parent
    sampling: { every: -5 }
metrics:
  - id: test
    traversal: default
    output: { files: list }
    paths:
      include: ["src/**"]
    runner:
      builtin: git_grep_count
      config:
        pattern: "foo"
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for negative sampling")
	}
	if !strings.Contains(err.Error(), "sampling.every") {
		t.Errorf("expected error to reference sampling.every, got: %v", err)
	}
}

func TestValidateConfig_RejectsUnknownBuiltin(t *testing.T) {
	cfg := mustParse(t, `
version: 1
traversals:
  default:
    range: { start: "HEAD~10", end: "HEAD" }
    mode: first_parent
    sampling: { bucket: commit, every: 1 }
metrics:
  - id: m
    traversal: default
    output: { files: list }
    paths: { include: ["src/"] }
    runner:
      builtin: git_grep_cuont
      config: { pattern: "foo" }
`)
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for unknown builtin")
	}
	if !strings.Contains(err.Error(), "git_grep_cuont") || !strings.Contains(err.Error(), "git_grep_count") {
		t.Errorf("error should name the bad runner and list supported ones, got: %v", err)
	}
}

func TestValidateConfig_RunnerConfigIsChecked(t *testing.T) {
	cases := map[string]string{
		"grep without pattern": `
      builtin: git_grep_count`,
		"file_count with config": `
      builtin: git_file_count
      config: { pattern: "foo" }`,
		"exec with empty command": `
      exec: [""]`,
	}
	for name, runnerYAML := range cases {
		cfg := mustParse(t, `
version: 1
traversals:
  default:
    range: { start: "HEAD~10", end: "HEAD" }
    mode: first_parent
    sampling: { bucket: commit, every: 1 }
metrics:
  - id: m
    traversal: default
    output: { files: list }
    paths: { include: ["src/"] }
    runner:`+runnerYAML+`
`)
		if err := Validate(cfg); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

func TestValidateConfig_AcceptsAllBuiltins(t *testing.T) {
	cfg := mustParse(t, `
version: 1
traversals:
  default:
    range: { start: "HEAD~10", end: "HEAD" }
    mode: first_parent
    sampling: { bucket: commit, every: 1 }
metrics:
  - id: grep
    traversal: default
    output: { files: list }
    paths: { include: ["src/"] }
    runner:
      builtin: git_grep_count
      config: { pattern: "foo" }
  - id: files
    traversal: default
    output: { files: list }
    paths: { include: ["src/"] }
    runner:
      builtin: git_file_count
  - id: ext
    traversal: default
    output: { files: list }
    paths: { include: ["src/"] }
    runner:
      exec: ["node", "check.js"]
      config: { anything: true }
`)
	if err := Validate(cfg); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateConfig_DuplicateMetricIDs(t *testing.T) {
	yaml := `
version: 1
traversals:
  default:
    range: { start: "main~100", end: "HEAD" }
    mode: first_parent
    sampling: { bucket: commit, every: 10 }
metrics:
  - id: my-metric
    traversal: default
    output: { files: list }
    paths:
      include: ["src/**"]
    runner:
      builtin: git_grep_count
      config:
        pattern: "foo"
  - id: my-metric
    traversal: default
    output: { files: list }
    paths:
      include: ["lib/**"]
    runner:
      builtin: git_grep_count
      config:
        pattern: "bar"
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for duplicate metric id")
	}
	if !strings.Contains(err.Error(), "duplicate metric id") {
		t.Errorf("expected error to mention duplicate metric id, got: %v", err)
	}
}

func TestValidateConfig_OutputFiles(t *testing.T) {
	metric := func(output string) string {
		return `
version: 1
traversals:
  default:
    range: { start: "HEAD~10", end: "HEAD" }
    mode: first_parent
    sampling: { bucket: commit, every: 1 }
metrics:
  - id: m
    traversal: default
    paths: { include: ["src/"] }
    runner:
      builtin: git_grep_count
      config: { pattern: "foo" }` + output + `
`
	}

	for name, output := range map[string]string{
		"list": "\n    output: { files: list }",
		"none": "\n    output: { files: none }",
	} {
		cfg := mustParse(t, metric(output))
		if err := Validate(cfg); err != nil {
			t.Errorf("%s: unexpected validation error: %v", name, err)
		}
	}

	for name, output := range map[string]string{
		"absent":  "",
		"unknown": "\n    output: { files: no }",
	} {
		err := Validate(mustParse(t, metric(output)))
		if err == nil {
			t.Errorf("%s: expected validation error", name)
			continue
		}
		if !strings.Contains(err.Error(), "output.files") || !strings.Contains(err.Error(), "none") {
			t.Errorf("%s: error should name the field and list valid values, got: %v", name, err)
		}
	}
}

func TestMetric_EmitsFiles(t *testing.T) {
	if !(Metric{Output: Output{Files: FilesList}}).EmitsFiles() {
		t.Error("list should emit files")
	}
	if (Metric{Output: Output{Files: FilesNone}}).EmitsFiles() {
		t.Error("none should not emit files")
	}
}

func TestValidateConfig_SamplingBucket(t *testing.T) {
	traversal := func(sampling string) string {
		return `
version: 1
traversals:
  default:
    range: { start: "HEAD~10", end: "HEAD" }
    mode: first_parent
    sampling: ` + sampling + `
metrics:
  - id: m
    traversal: default
    output: { files: list }
    paths: { include: ["src/"] }
    runner:
      builtin: git_grep_count
      config: { pattern: "foo" }
`
	}
	for _, ok := range []string{"{ bucket: commit, every: 1 }", "{ bucket: day, every: 1 }", "{ bucket: week, every: 2 }", "{ bucket: month, every: 1 }"} {
		if err := Validate(mustParse(t, traversal(ok))); err != nil {
			t.Errorf("%s: unexpected error: %v", ok, err)
		}
	}
	for _, bad := range []string{"{ every: 1 }", "{ bucket: fortnight, every: 1 }", "{ bucket: day }"} {
		err := Validate(mustParse(t, traversal(bad)))
		if err == nil {
			t.Errorf("%s: expected validation error", bad)
			continue
		}
		if !strings.Contains(err.Error(), "sampling") {
			t.Errorf("%s: error should name sampling, got: %v", bad, err)
		}
	}
}

func TestValidateConfig_RangeStartDate(t *testing.T) {
	cfg := func(start string) string {
		return `
version: 1
traversals:
  default:
    range: { start: "` + start + `", end: "HEAD" }
    mode: first_parent
    sampling: { bucket: commit, every: 1 }
metrics:
  - id: m
    traversal: default
    output: { files: list }
    paths: { include: ["src/"] }
    runner:
      builtin: git_grep_count
      config: { pattern: "foo" }
`
	}
	for _, ok := range []string{"2025-01-01", "main~100", "v1.2.3"} {
		if err := Validate(mustParse(t, cfg(ok))); err != nil {
			t.Errorf("%s: unexpected error: %v", ok, err)
		}
	}
	err := Validate(mustParse(t, cfg("2025-13-45")))
	if err == nil || !strings.Contains(err.Error(), "invalid date") {
		t.Errorf("expected invalid date error, got: %v", err)
	}
}
