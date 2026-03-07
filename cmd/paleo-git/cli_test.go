package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bel0v/paleo-git/engine"
	"github.com/bel0v/paleo-git/internal/testutil"
)

func writeTestConfig(t *testing.T, dir string) string {
	t.Helper()
	cfg := `
version: 1
traversals:
  default:
    range:
      start: "HEAD~4"
      end: "HEAD"
    mode: first_parent
    sampling:
      every: 1
metrics:
  - id: legacy-imports
    traversal: default
    paths:
      include: ["src/"]
    runner:
      builtin: git_grep_count
      config:
        pattern: "@legacy/"
`
	path := filepath.Join(dir, "paleo.yml")
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestMeasureCmd_PrintsJSON(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfgPath := writeTestConfig(t, repo)

	out := captureOutput(t, func() {
		runCLI(t, "measure", "--config", cfgPath, "--repo", repo)
	})

	var results []engine.Result
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, out)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != engine.StatusOK {
		t.Errorf("expected status ok, got %s", results[0].Status)
	}
	if results[0].Value != 3 {
		t.Errorf("expected value 3, got %d", results[0].Value)
	}
}

func TestMeasureCmd_IsDeterministic(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfgPath := writeTestConfig(t, repo)

	out1 := captureOutput(t, func() {
		runCLI(t, "measure", "--config", cfgPath, "--repo", repo)
	})
	out2 := captureOutput(t, func() {
		runCLI(t, "measure", "--config", cfgPath, "--repo", repo)
	})

	// Parse both to compare values (timestamps may differ)
	var r1, r2 []engine.Result
	json.Unmarshal([]byte(out1), &r1)
	json.Unmarshal([]byte(out2), &r2)

	if len(r1) != len(r2) {
		t.Fatalf("different result counts: %d vs %d", len(r1), len(r2))
	}
	for i := range r1 {
		if r1[i].Value != r2[i].Value {
			t.Errorf("result[%d] value differs: %d vs %d", i, r1[i].Value, r2[i].Value)
		}
	}
}

func TestScanCmd_StreamsNDJSON(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfgPath := writeTestConfig(t, repo)

	out := captureOutput(t, func() {
		runCLI(t, "scan", "--config", cfgPath, "--repo", repo)
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 {
		t.Fatal("expected at least 1 NDJSON line")
	}

	for i, line := range lines {
		var r engine.Result
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("line %d: invalid JSON: %v\nline: %s", i, err, line)
		}
		if r.MetricID != "legacy-imports" {
			t.Errorf("line %d: unexpected metric id: %s", i, r.MetricID)
		}
	}
}

func TestScanCmd_SkipResume(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfgPath := writeTestConfig(t, repo)

	// First scan
	out1 := captureOutput(t, func() {
		runCLI(t, "scan", "--config", cfgPath, "--repo", repo)
	})

	// Write first results to a skip file
	skipPath := filepath.Join(t.TempDir(), "results.jsonl")
	if err := os.WriteFile(skipPath, []byte(out1), 0o644); err != nil {
		t.Fatalf("write skip file: %v", err)
	}

	// Second scan with skip
	out2 := captureOutput(t, func() {
		runCLI(t, "scan", "--config", cfgPath, "--repo", repo, "--skip", skipPath)
	})

	if strings.TrimSpace(out2) != "" {
		t.Errorf("expected no output on resumed scan, got: %s", out2)
	}
}
