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

func (g *GitGrepCount) Validate(config map[string]any) error {
	_, err := patternFrom(config)
	return err
}

func (g *GitGrepCount) Run(ctx context.Context, req runner.RunRequest) (*runner.RunResult, error) {
	pattern, err := patternFrom(req.Config)
	if err != nil {
		return nil, err
	}

	count, files, err := vcs.GrepCount(ctx, req.RepoPath, req.Commit, pattern, req.PathsInclude, req.PathsExclude)
	if err != nil {
		return nil, fmt.Errorf("git_grep_count: %w", err)
	}

	return &runner.RunResult{Value: count, Files: files}, nil
}

func patternFrom(config map[string]any) (string, error) {
	patternRaw, ok := config["pattern"]
	if !ok {
		return "", fmt.Errorf("git_grep_count: missing required config field 'pattern'")
	}
	pattern, ok := patternRaw.(string)
	if !ok {
		return "", fmt.Errorf("git_grep_count: 'pattern' must be a string, got %T", patternRaw)
	}
	if pattern == "" {
		return "", fmt.Errorf("git_grep_count: 'pattern' must not be empty")
	}
	for key := range config {
		if key != "pattern" {
			return "", fmt.Errorf("git_grep_count: unknown config field %q", key)
		}
	}
	return pattern, nil
}
