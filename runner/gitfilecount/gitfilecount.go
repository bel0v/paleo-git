package gitfilecount

import (
	"context"
	"fmt"

	"github.com/bel0v/paleo-git/runner"
	"github.com/bel0v/paleo-git/vcs"
)

// GitFileCount counts the files under the metric's paths at a commit.
type GitFileCount struct{}

func New() *GitFileCount {
	return &GitFileCount{}
}

func (g *GitFileCount) Run(ctx context.Context, req runner.RunRequest) (*runner.RunResult, error) {
	files, err := vcs.ListFiles(ctx, req.RepoPath, req.Commit, req.PathsInclude, req.PathsExclude)
	if err != nil {
		return nil, fmt.Errorf("git_file_count: %w", err)
	}
	return &runner.RunResult{Value: len(files), Files: files}, nil
}
