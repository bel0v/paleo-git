package vcs

import (
	"context"
	"testing"

	"github.com/bel0v/paleo-git/internal/testutil"
)

func TestListCommits_FirstParentReturnsLinearHistory(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	commits, err := ListCommits(context.Background(), repo, "HEAD~4", "HEAD", true, 1)
	if err != nil {
		t.Fatalf("ListCommits error: %v", err)
	}

	// 5 commits total, HEAD~4 to HEAD inclusive = 4 commits (excludes start)
	if len(commits) < 3 {
		t.Fatalf("expected at least 3 commits, got %d", len(commits))
	}

	// Verify order: oldest first
	for i := 1; i < len(commits); i++ {
		if commits[i].Order <= commits[i-1].Order {
			t.Errorf("commits not in ascending order: %d <= %d", commits[i].Order, commits[i-1].Order)
		}
	}
}

func TestListCommits_SamplingStrideSkipsCommits(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	all, err := ListCommits(context.Background(), repo, "HEAD~4", "HEAD", true, 1)
	if err != nil {
		t.Fatalf("ListCommits error: %v", err)
	}

	sampled, err := ListCommits(context.Background(), repo, "HEAD~4", "HEAD", true, 2)
	if err != nil {
		t.Fatalf("ListCommits error: %v", err)
	}

	if len(sampled) >= len(all) {
		t.Errorf("sampled (%d) should be fewer than all (%d)", len(sampled), len(all))
	}
}

func TestGrepCount_ReturnsCorrectCount(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	// At HEAD, there are 3 files with @legacy: src/a.ts (2 lines), src/b.ts (1), test/d.test.ts (1)
	// Total lines matching: 4
	count, files, err := GrepCount(context.Background(), repo, "HEAD", "@legacy/", nil, nil)
	if err != nil {
		t.Fatalf("GrepCount error: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 matches, got %d", count)
	}
	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d: %v", len(files), files)
	}
}

func TestGrepCount_RespectsPathFilter(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	// Only search in src/ — should exclude test/d.test.ts
	count, files, err := GrepCount(context.Background(), repo, "HEAD", "@legacy/", []string{"src/"}, nil)
	if err != nil {
		t.Fatalf("GrepCount error: %v", err)
	}
	// src/a.ts has 2 matches, src/b.ts has 1 = 3 total
	if count != 3 {
		t.Errorf("expected 3 matches with src/ filter, got %d", count)
	}
	for _, f := range files {
		if f == "test/d.test.ts" {
			t.Error("test/d.test.ts should not be in filtered results")
		}
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}
}

func TestGrepCount_RespectsExcludeFilter(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	// Search all files but exclude test/
	count, files, err := GrepCount(context.Background(), repo, "HEAD", "@legacy/", nil, []string{"test/"})
	if err != nil {
		t.Fatalf("GrepCount error: %v", err)
	}
	// src/a.ts has 2 matches, src/b.ts has 1 = 3 total (test/d.test.ts excluded)
	if count != 3 {
		t.Errorf("expected 3 matches with test/ excluded, got %d", count)
	}
	for _, f := range files {
		if f == "test/d.test.ts" {
			t.Error("test/d.test.ts should not be in excluded results")
		}
	}
}

func TestResolveCommit_ReturnsMetadata(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	meta, err := ResolveCommit(context.Background(), repo, "HEAD")
	if err != nil {
		t.Fatalf("ResolveCommit error: %v", err)
	}
	if len(meta.SHA) != 40 {
		t.Errorf("expected 40-char SHA, got %q", meta.SHA)
	}
	if meta.AuthorDate.IsZero() {
		t.Error("expected non-zero AuthorDate")
	}
}

func TestResolveCommit_RejectsFlagLikeRef(t *testing.T) {
	_, err := ResolveCommit(context.Background(), "/tmp/repo", "--exec=bad")
	if err == nil {
		t.Fatal("expected error for flag-like ref")
	}
}

func TestListCommits_RespectsContextCancellation(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := ListCommits(ctx, repo, "HEAD~4", "HEAD", true, 1)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
