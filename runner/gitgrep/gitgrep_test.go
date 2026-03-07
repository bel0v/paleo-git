package gitgrep

import (
	"context"
	"testing"

	"github.com/bel0v/paleo-git/internal/testutil"
	"github.com/bel0v/paleo-git/runner"
)

func TestGitGrepCount_CountsMatchesAtHEAD(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	r := New()
	result, err := r.Run(context.Background(), runner.RunRequest{
		Commit:       "HEAD",
		RepoPath:     repo,
		Config:       map[string]any{"pattern": "@legacy/"},
		PathsInclude: []string{"src/"},
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	// src/a.ts has 2 matches, src/b.ts has 1 = 3 total
	if result.Value != 3 {
		t.Errorf("expected 3, got %d", result.Value)
	}
	if len(result.Files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(result.Files), result.Files)
	}
}

func TestGitGrepCount_RespectsPathsFilter(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	r := New()
	// Include all files — should find matches in test/ too
	result, err := r.Run(context.Background(), runner.RunRequest{
		Commit:       "HEAD",
		RepoPath:     repo,
		Config:       map[string]any{"pattern": "@legacy/"},
		PathsInclude: []string{"src/", "test/"},
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.Value != 4 {
		t.Errorf("expected 4 matches with src/ + test/, got %d", result.Value)
	}
}

func TestGitGrepCount_MissingPatternReturnsError(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	r := New()
	_, err := r.Run(context.Background(), runner.RunRequest{
		Commit:       "HEAD",
		RepoPath:     repo,
		Config:       map[string]any{},
		PathsInclude: []string{"src/"},
	})
	if err == nil {
		t.Fatal("expected error for missing pattern")
	}
}

func TestGitGrepCount_NoMatchesReturnsZero(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	r := New()
	result, err := r.Run(context.Background(), runner.RunRequest{
		Commit:       "HEAD",
		RepoPath:     repo,
		Config:       map[string]any{"pattern": "nonexistent_pattern_xyz"},
		PathsInclude: []string{"src/"},
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.Value != 0 {
		t.Errorf("expected 0 matches, got %d", result.Value)
	}
}
