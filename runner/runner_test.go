package runner

import (
	"context"
	"testing"
)

func TestRunnerInterface_Compiles(t *testing.T) {
	// Verify the interface and types are usable
	var _ Runner = mockRunner{}

	req := RunRequest{
		Commit:       "abc123",
		RepoPath:     "/tmp/repo",
		Config:       map[string]any{"pattern": "foo"},
		PathsInclude: []string{"src/**"},
		PathsExclude: []string{"**/*.test.*"},
	}
	if req.Commit != "abc123" {
		t.Error("RunRequest fields not set correctly")
	}
}

type mockRunner struct{}

func (m mockRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	return &RunResult{Value: 1, Files: []string{"a.go"}}, nil
}
