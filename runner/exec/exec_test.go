package exec

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/bel0v/paleo-git/runner"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "internal", "testdata", "runner", name)
}

func TestExecRunner_HappyPath(t *testing.T) {
	r := New([]string{"bash", testdataPath("happy.sh")})
	result, err := r.Run(context.Background(), runner.RunRequest{
		Commit:   "abc123",
		RepoPath: "/tmp/repo",
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.Value != 42 {
		t.Errorf("expected value 42, got %d", result.Value)
	}
	if len(result.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(result.Files))
	}
}

func TestExecRunner_NonZeroExitIsError(t *testing.T) {
	r := New([]string{"bash", testdataPath("failing.sh")})
	_, err := r.Run(context.Background(), runner.RunRequest{
		Commit:   "abc123",
		RepoPath: "/tmp/repo",
	})
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("expected stderr in error, got: %v", err)
	}
}

func TestExecRunner_TimeoutIsReported(t *testing.T) {
	r := New([]string{"bash", testdataPath("slow.sh")})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := r.Run(ctx, runner.RunRequest{
		Commit:   "abc123",
		RepoPath: "/tmp/repo",
	})
	if err == nil {
		t.Fatal("expected error for timeout")
	}
	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "killed") && !strings.Contains(err.Error(), "context") {
		t.Errorf("expected timeout-related error, got: %v", err)
	}
}

func TestExecRunner_InvalidJSON(t *testing.T) {
	r := New([]string{"bash", testdataPath("bad_json.sh")})
	_, err := r.Run(context.Background(), runner.RunRequest{
		Commit:   "abc123",
		RepoPath: "/tmp/repo",
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExecRunner_ConfigEnvVar(t *testing.T) {
	r := New([]string{"bash", testdataPath("echo_config.sh")})
	result, err := r.Run(context.Background(), runner.RunRequest{
		Commit:   "abc123",
		RepoPath: "/tmp/repo",
		Config:   map[string]any{"key": "val"},
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if result.Value != 99 {
		t.Errorf("expected value 99 (config was set), got %d", result.Value)
	}
}
