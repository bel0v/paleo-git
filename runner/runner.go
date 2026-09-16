package runner

import "context"

type Runner interface {
	// Validate checks a metric's runner config at config load time, before
	// any commit is measured.
	Validate(config map[string]any) error
	Run(ctx context.Context, req RunRequest) (*RunResult, error)
}

type RunRequest struct {
	Commit       string
	RepoPath     string
	Config       map[string]any
	PathsInclude []string
	PathsExclude []string
}

type RunResult struct {
	Value int
	Files []string
}
