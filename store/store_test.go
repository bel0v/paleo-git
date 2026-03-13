package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/bel0v/paleo-git/engine"
)

func makeResult(metricID, metricHash, commit string, value int) engine.Result {
	return engine.Result{
		MetricID:   metricID,
		MetricHash: metricHash,
		Commit:     commit,
		AuthorDate: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		Value:      value,
		Status:     engine.StatusOK,
		DurationMs: 100,
	}
}

func writeFixtureFile(t *testing.T, dir, metricID string, results []engine.Result) {
	t.Helper()
	metricsDir := filepath.Join(dir, "metrics")
	if err := os.MkdirAll(metricsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(filepath.Join(metricsDir, metricID+".jsonl"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	for _, r := range results {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if _, err := f.Write(line); err != nil {
			t.Fatalf("write: %v", err)
		}
		if _, err := f.Write([]byte("\n")); err != nil {
			t.Fatalf("write newline: %v", err)
		}
	}
}

func TestRead_ReturnsResultsFromFile(t *testing.T) {
	dir := t.TempDir()
	want := []engine.Result{
		makeResult("legacy-imports", "abc123", "commit1", 42),
		makeResult("legacy-imports", "abc123", "commit2", 37),
	}
	writeFixtureFile(t, dir, "legacy-imports", want)

	d := NewDir(dir)
	got, err := d.Read(context.Background(), "legacy-imports")
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d results, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i].MetricID != want[i].MetricID || got[i].Commit != want[i].Commit || got[i].Value != want[i].Value {
			t.Errorf("result %d mismatch: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestRead_ReturnsEmptyForMissingFile(t *testing.T) {
	dir := t.TempDir()
	d := NewDir(dir)
	got, err := d.Read(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d results", len(got))
	}
}

func TestAlreadyMeasured_ReadsAllMetricFiles(t *testing.T) {
	dir := t.TempDir()
	writeFixtureFile(t, dir, "metric-a", []engine.Result{
		makeResult("metric-a", "hash-a", "commit1", 10),
		makeResult("metric-a", "hash-a", "commit2", 20),
	})
	writeFixtureFile(t, dir, "metric-b", []engine.Result{
		makeResult("metric-b", "hash-b", "commit1", 5),
	})

	d := NewDir(dir)
	keys, err := d.AlreadyMeasured(context.Background())
	if err != nil {
		t.Fatalf("AlreadyMeasured error: %v", err)
	}
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}

	// Sort for deterministic comparison
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].MetricID != keys[j].MetricID {
			return keys[i].MetricID < keys[j].MetricID
		}
		return keys[i].Commit < keys[j].Commit
	})

	if keys[0].MetricID != "metric-a" || keys[0].MetricHash != "hash-a" || keys[0].Commit != "commit1" {
		t.Errorf("unexpected key[0]: %+v", keys[0])
	}
	if keys[1].MetricID != "metric-a" || keys[1].Commit != "commit2" {
		t.Errorf("unexpected key[1]: %+v", keys[1])
	}
	if keys[2].MetricID != "metric-b" || keys[2].MetricHash != "hash-b" || keys[2].Commit != "commit1" {
		t.Errorf("unexpected key[2]: %+v", keys[2])
	}
}

func TestAlreadyMeasured_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	d := NewDir(dir)
	keys, err := d.AlreadyMeasured(context.Background())
	if err != nil {
		t.Fatalf("AlreadyMeasured error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected empty slice, got %d keys", len(keys))
	}
}

func TestAppend_CreatesDirectoryAndFile(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")

	d := NewDir(dataDir)
	results := []engine.Result{
		makeResult("my-metric", "hash1", "commit1", 42),
	}
	if err := d.Append(context.Background(), results); err != nil {
		t.Fatalf("Append error: %v", err)
	}

	got, err := d.Read(context.Background(), "my-metric")
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].Value != 42 {
		t.Errorf("expected value 42, got %d", got[0].Value)
	}
}

func TestAppend_AppendsToExistingFile(t *testing.T) {
	dir := t.TempDir()
	d := NewDir(dir)
	ctx := context.Background()

	batch1 := []engine.Result{makeResult("my-metric", "hash1", "commit1", 10)}
	batch2 := []engine.Result{makeResult("my-metric", "hash1", "commit2", 20)}

	if err := d.Append(ctx, batch1); err != nil {
		t.Fatalf("Append batch1 error: %v", err)
	}
	if err := d.Append(ctx, batch2); err != nil {
		t.Fatalf("Append batch2 error: %v", err)
	}

	got, err := d.Read(ctx, "my-metric")
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0].Value != 10 || got[1].Value != 20 {
		t.Errorf("unexpected values: %d, %d", got[0].Value, got[1].Value)
	}
}

func TestAppend_GroupsByMetricID(t *testing.T) {
	dir := t.TempDir()
	d := NewDir(dir)
	ctx := context.Background()

	results := []engine.Result{
		makeResult("metric-a", "hash-a", "commit1", 10),
		makeResult("metric-b", "hash-b", "commit1", 20),
		makeResult("metric-a", "hash-a", "commit2", 30),
	}
	if err := d.Append(ctx, results); err != nil {
		t.Fatalf("Append error: %v", err)
	}

	gotA, err := d.Read(ctx, "metric-a")
	if err != nil {
		t.Fatalf("Read metric-a error: %v", err)
	}
	if len(gotA) != 2 {
		t.Fatalf("expected 2 results for metric-a, got %d", len(gotA))
	}

	gotB, err := d.Read(ctx, "metric-b")
	if err != nil {
		t.Fatalf("Read metric-b error: %v", err)
	}
	if len(gotB) != 1 {
		t.Fatalf("expected 1 result for metric-b, got %d", len(gotB))
	}
}

func TestRead_RejectsUnsafeMetricID(t *testing.T) {
	dir := t.TempDir()
	d := NewDir(dir)

	for _, id := range []string{"../escape", "../../etc/passwd", "foo/bar", "", ".."} {
		_, err := d.Read(context.Background(), id)
		if err == nil {
			t.Errorf("expected error for metric id %q, got nil", id)
		}
	}
}

func TestAppend_RejectsUnsafeMetricID(t *testing.T) {
	dir := t.TempDir()
	d := NewDir(dir)

	results := []engine.Result{makeResult("../escape", "hash", "commit1", 1)}
	err := d.Append(context.Background(), results)
	if err == nil {
		t.Fatal("expected error for unsafe metric id in Append")
	}
}
