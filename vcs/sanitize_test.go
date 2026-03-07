package vcs

import (
	"strings"
	"testing"
)

func TestListCommits_RejectsFlagLikeRef(t *testing.T) {
	_, err := ListCommits("/tmp/repo", "--exec=bad", "HEAD", true, 1)
	if err == nil {
		t.Fatal("expected error for flag-like start ref")
	}
	if !strings.Contains(err.Error(), "must not start with '-'") {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = ListCommits("/tmp/repo", "HEAD~10", "--exec=bad", true, 1)
	if err == nil {
		t.Fatal("expected error for flag-like end ref")
	}
}

func TestListChangedFiles_RejectsFlagLikeCommit(t *testing.T) {
	_, err := ListChangedFiles("/tmp/repo", "--exec=bad")
	if err == nil {
		t.Fatal("expected error for flag-like commit")
	}
}

func TestGrepCount_RejectsFlagLikePattern(t *testing.T) {
	_, _, err := GrepCount("/tmp/repo", "HEAD", "--open-files-in-pager=bad", nil)
	if err == nil {
		t.Fatal("expected error for flag-like pattern")
	}
}

func TestGrepCount_RejectsFlagLikeRepoPath(t *testing.T) {
	_, _, err := GrepCount("-badpath", "HEAD", "pattern", nil)
	if err == nil {
		t.Fatal("expected error for flag-like repo path")
	}
}

func TestListCommits_RejectsEmptyRef(t *testing.T) {
	_, err := ListCommits("/tmp/repo", "", "HEAD", true, 1)
	if err == nil {
		t.Fatal("expected error for empty start ref")
	}
}
