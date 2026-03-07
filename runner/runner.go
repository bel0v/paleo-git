package runner

import "context"

type Runner interface {
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
