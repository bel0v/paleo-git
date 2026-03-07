package engine

import (
	"testing"

	"github.com/bel0v/paleo-git/config"
)

func TestMeasure_StubReturnsError(t *testing.T) {
	cfg := config.Config{}
	_, err := Measure(cfg, "HEAD")
	if err == nil {
		t.Fatal("expected error from stub implementation")
	}
}

func TestScan_StubReturnsError(t *testing.T) {
	cfg := config.Config{}
	err := Scan(cfg, ScanOptions{}, func(r Result) {})
	if err == nil {
		t.Fatal("expected error from stub implementation")
	}
}

func TestResult_FieldsExist(t *testing.T) {
	r := Result{
		MetricID:   "test",
		Commit:     "abc123",
		Value:      42,
		Files:      []string{"src/old.ts"},
		Status:     "ok",
		DurationMs: 100,
	}
	if r.MetricID != "test" {
		t.Error("MetricID not set")
	}
	if r.Value != 42 {
		t.Error("Value not set")
	}
	if len(r.Files) != 1 {
		t.Error("Files not set")
	}
}
