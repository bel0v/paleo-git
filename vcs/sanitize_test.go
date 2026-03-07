package vcs

import (
	"context"
	"strings"
	"testing"
)

func TestListCommits_RejectsFlagLikeRef(t *testing.T) {
	ctx := context.Background()
	_, err := ListCommits(ctx, "/tmp/repo", "--exec=bad", "HEAD", true, 1)
	if err == nil {
		t.Fatal("expected error for flag-like start ref")
	}
	if !strings.Contains(err.Error(), "must not start with '-'") {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = ListCommits(ctx, "/tmp/repo", "HEAD~10", "--exec=bad", true, 1)
	if err == nil {
		t.Fatal("expected error for flag-like end ref")
	}
}

func TestGrepCount_RejectsFlagLikePattern(t *testing.T) {
	_, _, err := GrepCount(context.Background(), "/tmp/repo", "HEAD", "--open-files-in-pager=bad", nil, nil)
	if err == nil {
		t.Fatal("expected error for flag-like pattern")
	}
}

func TestGrepCount_RejectsFlagLikeRepoPath(t *testing.T) {
	_, _, err := GrepCount(context.Background(), "-badpath", "HEAD", "pattern", nil, nil)
	if err == nil {
		t.Fatal("expected error for flag-like repo path")
	}
}

func TestListCommits_RejectsEmptyRef(t *testing.T) {
	_, err := ListCommits(context.Background(), "/tmp/repo", "", "HEAD", true, 1)
	if err == nil {
		t.Fatal("expected error for empty start ref")
	}
}
