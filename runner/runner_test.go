package runner

import "context"

// Compile-time interface compliance check.
var _ Runner = (*mockRunner)(nil)

type mockRunner struct{}

func (m *mockRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	return &RunResult{Value: 1, Files: []string{"a.go"}}, nil
}
