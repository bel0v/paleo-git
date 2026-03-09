package engine

import (
	"context"
	"testing"

	"github.com/bel0v/paleo-git/config"
	"github.com/bel0v/paleo-git/internal/testutil"
)

func makeTestConfig() config.Config {
	return config.Config{
		Version: 1,
		Traversals: map[string]config.Traversal{
			"default": {
				Range:    config.Range{Start: "HEAD~4", End: "HEAD"},
				Mode:     "first_parent",
				Sampling: config.Sampling{Every: 1},
			},
		},
		Metrics: []config.Metric{
			{
				ID:        "legacy-imports",
				Traversal: "default",
				Paths:     config.Paths{Include: []string{"src/"}},
				Runner: config.RunnerRef{
					Builtin: "git_grep_count",
					Config:  map[string]any{"pattern": "@legacy/"},
				},
			},
		},
	}
}

func TestMeasure_RunsAllMetricsAtCommit(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfg := makeTestConfig()

	results, err := Measure(context.Background(), cfg, repo, "HEAD")
	if err != nil {
		t.Fatalf("Measure error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.MetricID != "legacy-imports" {
		t.Errorf("expected metric id legacy-imports, got %s", r.MetricID)
	}
	if r.Status != StatusOK {
		t.Errorf("expected status ok, got %s (error: %s)", r.Status, r.Error)
	}
	// src/a.ts has 2 matches, src/b.ts has 1 = 3 (test/ excluded by path filter)
	if r.Value != 3 {
		t.Errorf("expected value 3, got %d", r.Value)
	}
}

func TestMeasure_ResolvesCommitMetadata(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfg := makeTestConfig()

	results, err := Measure(context.Background(), cfg, repo, "HEAD")
	if err != nil {
		t.Fatalf("Measure error: %v", err)
	}
	r := results[0]
	if r.AuthorDate.IsZero() {
		t.Error("expected AuthorDate to be populated, got zero")
	}
	if len(r.Commit) != 40 {
		t.Errorf("expected resolved SHA (40 hex chars), got %q", r.Commit)
	}
}

func TestMeasure_ContinuesPastFailingMetric(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfg := config.Config{
		Version: 1,
		Traversals: map[string]config.Traversal{
			"default": {
				Range:    config.Range{Start: "HEAD~1", End: "HEAD"},
				Mode:     "first_parent",
				Sampling: config.Sampling{Every: 1},
			},
		},
		Metrics: []config.Metric{
			{
				ID:        "bad-runner",
				Traversal: "default",
				Paths:     config.Paths{Include: []string{"src/"}},
				Runner:    config.RunnerRef{Builtin: "nonexistent_runner"},
			},
			{
				ID:        "good-metric",
				Traversal: "default",
				Paths:     config.Paths{Include: []string{"src/"}},
				Runner: config.RunnerRef{
					Builtin: "git_grep_count",
					Config:  map[string]any{"pattern": "@legacy/"},
				},
			},
		},
	}

	results, err := Measure(context.Background(), cfg, repo, "HEAD")
	if err != nil {
		t.Fatalf("Measure error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Status != StatusError {
		t.Errorf("expected first metric to have status error, got %s", results[0].Status)
	}
	if results[1].Status != StatusOK {
		t.Errorf("expected second metric to have status ok, got %s", results[1].Status)
	}
}

func TestScan_CallsOnResultForEachPair(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfg := makeTestConfig()

	var results []Result
	err := Scan(context.Background(), cfg, repo, ScanOptions{}, func(r Result) {
		results = append(results, r)
	})
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result from scan")
	}
	// All results should be for "legacy-imports"
	for _, r := range results {
		if r.MetricID != "legacy-imports" {
			t.Errorf("unexpected metric id: %s", r.MetricID)
		}
	}
}

func TestScan_SkipsAlreadyMeasured(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfg := makeTestConfig()

	// First scan — collect all commits
	var firstResults []Result
	err := Scan(context.Background(), cfg, repo, ScanOptions{}, func(r Result) {
		firstResults = append(firstResults, r)
	})
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	// Second scan — skip all metric+commit pairs from the first scan
	var skipKeys []MeasuredKey
	for _, r := range firstResults {
		skipKeys = append(skipKeys, MeasuredKey{MetricID: r.MetricID, Commit: r.Commit})
	}
	var secondResults []Result
	err = Scan(context.Background(), cfg, repo, ScanOptions{AlreadyMeasured: skipKeys}, func(r Result) {
		secondResults = append(secondResults, r)
	})
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if len(secondResults) != 0 {
		t.Errorf("expected 0 results after skipping all, got %d", len(secondResults))
	}
}

func TestScan_ResultsInCommitOrder(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfg := makeTestConfig()

	var results []Result
	err := Scan(context.Background(), cfg, repo, ScanOptions{}, func(r Result) {
		results = append(results, r)
	})
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("need at least 2 results to verify order, got %d", len(results))
	}
	// Results should arrive in oldest-first order (ascending AuthorDate).
	for i := 1; i < len(results); i++ {
		if results[i].AuthorDate.Before(results[i-1].AuthorDate) {
			t.Errorf("result %d (commit %s, %v) is older than result %d (commit %s, %v)",
				i, results[i].Commit[:8], results[i].AuthorDate,
				i-1, results[i-1].Commit[:8], results[i-1].AuthorDate)
		}
	}
}

func TestScan_DifferentTraversals(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	cfg := config.Config{
		Version: 1,
		Traversals: map[string]config.Traversal{
			"full": {
				Range:    config.Range{Start: "HEAD~4", End: "HEAD"},
				Mode:     "first_parent",
				Sampling: config.Sampling{Every: 1},
			},
			"recent": {
				Range:    config.Range{Start: "HEAD~2", End: "HEAD"},
				Mode:     "first_parent",
				Sampling: config.Sampling{Every: 1},
			},
		},
		Metrics: []config.Metric{
			{
				ID:        "full-metric",
				Traversal: "full",
				Paths:     config.Paths{Include: []string{"src/"}},
				Runner: config.RunnerRef{
					Builtin: "git_grep_count",
					Config:  map[string]any{"pattern": "@legacy/"},
				},
			},
			{
				ID:        "recent-metric",
				Traversal: "recent",
				Paths:     config.Paths{Include: []string{"src/"}},
				Runner: config.RunnerRef{
					Builtin: "git_grep_count",
					Config:  map[string]any{"pattern": "@new/"},
				},
			},
		},
	}

	var results []Result
	err := Scan(context.Background(), cfg, repo, ScanOptions{}, func(r Result) {
		results = append(results, r)
	})
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	fullCount := 0
	recentCount := 0
	for _, r := range results {
		switch r.MetricID {
		case "full-metric":
			fullCount++
		case "recent-metric":
			recentCount++
		}
	}
	if fullCount <= recentCount {
		t.Errorf("full-metric should have more results (%d) than recent-metric (%d)", fullCount, recentCount)
	}
}
