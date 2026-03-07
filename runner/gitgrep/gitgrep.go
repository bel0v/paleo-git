package gitgrep

import (
	"context"
	"fmt"

	"github.com/bel0v/paleo-git/runner"
	"github.com/bel0v/paleo-git/vcs"
)

type GitGrepCount struct{}

func New() *GitGrepCount {
	return &GitGrepCount{}
}

func (g *GitGrepCount) Run(ctx context.Context, req runner.RunRequest) (*runner.RunResult, error) {
	patternRaw, ok := req.Config["pattern"]
	if !ok {
		return nil, fmt.Errorf("git_grep_count: missing required config field 'pattern'")
	}
	pattern, ok := patternRaw.(string)
	if !ok {
		return nil, fmt.Errorf("git_grep_count: 'pattern' must be a string, got %T", patternRaw)
	}

	count, files, err := vcs.GrepCount(req.RepoPath, req.Commit, pattern, req.PathsInclude)
	if err != nil {
		return nil, fmt.Errorf("git_grep_count: %w", err)
	}

	return &runner.RunResult{Value: count, Files: files}, nil
}
