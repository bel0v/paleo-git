package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bel0v/paleo-git/engine"
	"github.com/bel0v/paleo-git/internal/testutil"
	"github.com/bel0v/paleo-git/store"
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

func readStore(t *testing.T, dataDir, metricID string) []engine.Result {
	t.Helper()
	d, err := store.NewDir(dataDir)
	if err != nil {
		t.Fatalf("NewDir error: %v", err)
	}
	results, err := d.Read(context.Background(), metricID)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	return results
}

func TestMeasureCmd_PrintsJSON(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfgPath := writeTestConfig(t, repo)

	out := captureOutput(t, "measure", "--config", cfgPath, "--repo", repo)

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

	out1 := captureOutput(t, "measure", "--config", cfgPath, "--repo", repo)
	out2 := captureOutput(t, "measure", "--config", cfgPath, "--repo", repo)

	var r1, r2 []engine.Result
	if err := json.Unmarshal([]byte(out1), &r1); err != nil {
		t.Fatalf("invalid JSON from run 1: %v", err)
	}
	if err := json.Unmarshal([]byte(out2), &r2); err != nil {
		t.Fatalf("invalid JSON from run 2: %v", err)
	}

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

	out := captureOutput(t, "scan", "--config", cfgPath, "--repo", repo)

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

func TestMeasureCmd_SaveDir(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfgPath := writeTestConfig(t, repo)
	dataDir := filepath.Join(t.TempDir(), "data")

	out := captureOutput(t, "measure", "--config", cfgPath, "--repo", repo, "--save-dir", dataDir)

	var stdoutResults []engine.Result
	if err := json.Unmarshal([]byte(out), &stdoutResults); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, out)
	}
	if len(stdoutResults) != 1 {
		t.Fatalf("expected 1 result on stdout, got %d", len(stdoutResults))
	}

	saved := readStore(t, dataDir, "legacy-imports")
	if len(saved) != 1 {
		t.Fatalf("expected 1 saved result, got %d", len(saved))
	}
	if saved[0].Value != stdoutResults[0].Value {
		t.Errorf("saved value %d != stdout value %d", saved[0].Value, stdoutResults[0].Value)
	}
}

func TestMeasureCmd_SaveDirQuiet(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfgPath := writeTestConfig(t, repo)
	dataDir := filepath.Join(t.TempDir(), "data")

	out := captureOutput(t, "measure", "--config", cfgPath, "--repo", repo, "--save-dir", dataDir, "--quiet")

	if strings.TrimSpace(out) != "" {
		t.Errorf("expected no stdout with --quiet, got: %s", out)
	}

	saved := readStore(t, dataDir, "legacy-imports")
	if len(saved) != 1 {
		t.Fatalf("expected 1 saved result, got %d", len(saved))
	}
}

func TestMeasureCmd_LoadDirSaveDir(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfgPath := writeTestConfig(t, repo)
	dataDir := filepath.Join(t.TempDir(), "data")

	// First measure — save results
	captureOutput(t, "measure", "--config", cfgPath, "--repo", repo, "--save-dir", dataDir)

	saved1 := readStore(t, dataDir, "legacy-imports")
	if len(saved1) != 1 {
		t.Fatalf("expected 1 saved result after first measure, got %d", len(saved1))
	}

	// Second measure with --load-dir + --save-dir — should not re-append
	out := captureOutput(t, "measure", "--config", cfgPath, "--repo", repo, "--load-dir", dataDir, "--save-dir", dataDir)

	// Stdout prints only new results (none in this case)
	var stdoutResults []engine.Result
	if err := json.Unmarshal([]byte(out), &stdoutResults); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(stdoutResults) != 0 {
		t.Errorf("expected 0 new results on stdout, got %d", len(stdoutResults))
	}

	// Data dir should still have exactly 1 result (no duplicate)
	saved2 := readStore(t, dataDir, "legacy-imports")
	if len(saved2) != 1 {
		t.Errorf("expected 1 saved result (no duplicate), got %d", len(saved2))
	}
}

func TestScanCmd_LoadDirSaveDir(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfgPath := writeTestConfig(t, repo)
	dataDir := filepath.Join(t.TempDir(), "data")

	// First scan — save results
	out1 := captureOutput(t, "scan", "--config", cfgPath, "--repo", repo, "--save-dir", dataDir)
	lines1 := strings.Split(strings.TrimSpace(out1), "\n")
	if len(lines1) == 0 {
		t.Fatal("expected NDJSON output from first scan")
	}

	// Second scan — load + save, should produce no new results
	out2 := captureOutput(t, "scan", "--config", cfgPath, "--repo", repo, "--load-dir", dataDir, "--save-dir", dataDir)
	if strings.TrimSpace(out2) != "" {
		t.Errorf("expected no output on resumed scan, got: %s", out2)
	}

	// Verify data dir still has the original results (no duplicates)
	saved := readStore(t, dataDir, "legacy-imports")
	if len(saved) != len(lines1) {
		t.Errorf("expected %d saved results, got %d", len(lines1), len(saved))
	}
}

func TestScanCmd_LoadDirOnly(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfgPath := writeTestConfig(t, repo)
	dataDir := filepath.Join(t.TempDir(), "data")

	// First scan — save to build skip data
	captureOutput(t, "scan", "--config", cfgPath, "--repo", repo, "--save-dir", dataDir)

	// Second scan — load-dir only (no save-dir), output goes to stdout
	out := captureOutput(t, "scan", "--config", cfgPath, "--repo", repo, "--load-dir", dataDir)

	if strings.TrimSpace(out) != "" {
		t.Errorf("expected no output when all skipped, got: %s", out)
	}
}

func TestScanCmd_SaveDirQuiet(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfgPath := writeTestConfig(t, repo)
	dataDir := filepath.Join(t.TempDir(), "data")

	out := captureOutput(t, "scan", "--config", cfgPath, "--repo", repo, "--save-dir", dataDir, "--quiet")

	if strings.TrimSpace(out) != "" {
		t.Errorf("expected no stdout with --quiet, got: %s", out)
	}

	saved := readStore(t, dataDir, "legacy-imports")
	if len(saved) == 0 {
		t.Error("expected saved results in data dir")
	}
}

func TestMeasureCmd_BackwardCompatible(t *testing.T) {
	// Without --load-dir/--save-dir, measure prints JSON to stdout
	repo := testutil.CreateFixtureRepo(t)
	cfgPath := writeTestConfig(t, repo)

	out := captureOutput(t, "measure", "--config", cfgPath, "--repo", repo)

	var results []engine.Result
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(results) != 1 || results[0].Value != 3 {
		t.Errorf("backward compatibility broken: got %+v", results)
	}
}

func TestScanCmd_BackwardCompatible(t *testing.T) {
	// Without --load-dir/--save-dir, scan streams NDJSON to stdout
	repo := testutil.CreateFixtureRepo(t)
	cfgPath := writeTestConfig(t, repo)

	out := captureOutput(t, "scan", "--config", cfgPath, "--repo", repo)

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 {
		t.Fatal("expected NDJSON output")
	}
	for _, line := range lines {
		var r engine.Result
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("invalid NDJSON line: %v", err)
		}
	}
}
