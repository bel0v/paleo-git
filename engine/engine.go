package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/bel0v/paleo-git/config"
	"github.com/bel0v/paleo-git/runner"
	runnerexec "github.com/bel0v/paleo-git/runner/exec"
	"github.com/bel0v/paleo-git/runner/gitgrep"
	"github.com/bel0v/paleo-git/vcs"
)

type ScanOptions struct {
	AlreadyMeasured []string
}

func resolveRunner(ref config.RunnerRef) (runner.Runner, error) {
	if ref.Builtin != "" {
		switch ref.Builtin {
		case "git_grep_count":
			return gitgrep.New(), nil
		default:
			return nil, fmt.Errorf("unknown builtin runner: %q", ref.Builtin)
		}
	}
	if len(ref.Exec) > 0 {
		return runnerexec.New(ref.Exec), nil
	}
	return nil, fmt.Errorf("runner must specify builtin or exec")
}

func runMetric(ctx context.Context, m config.Metric, repoPath, commit string) Result {
	start := time.Now()

	r, err := resolveRunner(m.Runner)
	if err != nil {
		return Result{
			MetricID: m.ID,
			Commit:   commit,
			Status:   "error",
			Error:    err.Error(),
		}
	}

	res, err := r.Run(ctx, runner.RunRequest{
		Commit:       commit,
		RepoPath:     repoPath,
		Config:       m.Runner.Config,
		PathsInclude: m.Paths.Include,
		PathsExclude: m.Paths.Exclude,
	})

	duration := int(time.Since(start).Milliseconds())

	if err != nil {
		return Result{
			MetricID:   m.ID,
			Commit:     commit,
			Status:     "error",
			Error:      err.Error(),
			DurationMs: duration,
		}
	}

	return Result{
		MetricID:   m.ID,
		Commit:     commit,
		Value:      res.Value,
		Files:      res.Files,
		Status:     "ok",
		DurationMs: duration,
	}
}

// Measure runs all metrics at a single commit and returns results.
// One metric's failure does not prevent others from running.
func Measure(cfg config.Config, repoPath, commit string) ([]Result, error) {
	ctx := context.Background()
	var results []Result
	for _, m := range cfg.Metrics {
		results = append(results, runMetric(ctx, m, repoPath, commit))
	}
	return results, nil
}

// Scan traverses commits per traversal, runs matching metrics at each commit,
// and calls onResult for each measurement.
func Scan(cfg config.Config, repoPath string, opts ScanOptions, onResult func(Result)) error {
	skip := make(map[string]bool, len(opts.AlreadyMeasured))
	for _, sha := range opts.AlreadyMeasured {
		skip[sha] = true
	}

	// Group metrics by traversal
	byTraversal := make(map[string][]config.Metric)
	for _, m := range cfg.Metrics {
		byTraversal[m.Traversal] = append(byTraversal[m.Traversal], m)
	}

	ctx := context.Background()

	for travName, metrics := range byTraversal {
		trav, ok := cfg.Traversals[travName]
		if !ok {
			return fmt.Errorf("traversal %q not found in config", travName)
		}

		firstParent := trav.Mode == "first_parent"
		every := trav.Sampling.Every
		if every < 1 {
			every = 1
		}

		commits, err := vcs.ListCommits(repoPath, trav.Range.Start, trav.Range.End, firstParent, every)
		if err != nil {
			return fmt.Errorf("listing commits for traversal %q: %w", travName, err)
		}

		for _, c := range commits {
			if skip[c.SHA] {
				continue
			}
			for _, m := range metrics {
				result := runMetric(ctx, m, repoPath, c.SHA)
				result.AuthorDate = c.AuthorDate
				onResult(result)
			}
		}
	}

	return nil
}
