package gitfilecount

import (
	"context"
	"testing"

	"github.com/bel0v/paleo-git/internal/testutil"
	"github.com/bel0v/paleo-git/runner"
)

func TestGitFileCount_CountsFilesUnderPaths(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	result, err := New().Run(context.Background(), runner.RunRequest{
		Commit:       "HEAD",
		RepoPath:     repo,
		PathsInclude: []string{"src/"},
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	// src/a.ts, src/b.ts, src/c.tsx
	if result.Value != 3 {
		t.Errorf("expected 3, got %d", result.Value)
	}
	if len(result.Files) != 3 {
		t.Errorf("expected 3 files, got %d: %v", len(result.Files), result.Files)
	}
}

func TestGitFileCount_RespectsExclude(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	result, err := New().Run(context.Background(), runner.RunRequest{
		Commit:       "HEAD",
		RepoPath:     repo,
		PathsInclude: []string{"src/", "test/"},
		PathsExclude: []string{"test/"},
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.Value != 3 {
		t.Errorf("expected 3 with test/ excluded, got %d", result.Value)
	}
}

func TestGitFileCount_ValidateRejectsConfig(t *testing.T) {
	r := New()
	if err := r.Validate(nil); err != nil {
		t.Errorf("nil config: unexpected error: %v", err)
	}
	if err := r.Validate(map[string]any{}); err != nil {
		t.Errorf("empty config: unexpected error: %v", err)
	}
	if err := r.Validate(map[string]any{"pattern": "x"}); err == nil {
		t.Error("expected error for unexpected config field")
	}
}

func TestGitFileCount_NoFilesReturnsZero(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	result, err := New().Run(context.Background(), runner.RunRequest{
		Commit:       "HEAD",
		RepoPath:     repo,
		PathsInclude: []string{"nonexistent/"},
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.Value != 0 {
		t.Errorf("expected 0, got %d", result.Value)
	}
}
