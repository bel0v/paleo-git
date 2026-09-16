package engine

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/bel0v/paleo-git/config"
	"github.com/bel0v/paleo-git/runner"
	"github.com/bel0v/paleo-git/vcs"
)

// MeasuredKey uniquely identifies a measurement: the metric definition
// (ID + hash of its config) at a specific commit. Consumers use these
// as skip-list keys to avoid re-measuring unchanged work.
type MeasuredKey struct {
	MetricID   string
	MetricHash string
	Commit     string
}

// ScanOptions configures a Scan run. AlreadyMeasured provides keys to
// skip, enabling incremental scans that resume where a previous run left off.
type ScanOptions struct {
	AlreadyMeasured []MeasuredKey
}

// runnerTimeout bounds a single measurement so one hung runner (a stuck
// external script, a pathological grep) cannot stall a scan or a CI job
// indefinitely. Generous on purpose: a legitimate measurement on a large
// repository takes seconds, not minutes.
var runnerTimeout = 10 * time.Minute

// resolvedMetric holds a pre-resolved runner and hash for a metric,
// avoiding repeated resolution and hashing in the hot loop.
type resolvedMetric struct {
	metric config.Metric
	runner runner.Runner
	hash   string
}

// resolveMetrics instantiates each metric's runner. Runner configs are
// expected to have been validated with config.Validate; an unresolvable
// runner here is a programming error, not a per-commit measurement failure.
func resolveMetrics(metrics []config.Metric) ([]resolvedMetric, error) {
	resolved := make([]resolvedMetric, len(metrics))
	for i, m := range metrics {
		r, err := config.ResolveRunner(m.Runner)
		if err != nil {
			return nil, fmt.Errorf("metric %q: %w", m.ID, err)
		}
		resolved[i] = resolvedMetric{metric: m, runner: r, hash: config.MetricHash(m)}
	}
	return resolved, nil
}

func runResolved(ctx context.Context, rm *resolvedMetric, repoPath, commit string) Result {
	start := time.Now()

	runCtx, cancel := context.WithTimeout(ctx, runnerTimeout)
	defer cancel()

	res, err := rm.runner.Run(runCtx, runner.RunRequest{
		Commit:       commit,
		RepoPath:     repoPath,
		Config:       rm.metric.Runner.Config,
		PathsInclude: rm.metric.Paths.Include,
		PathsExclude: rm.metric.Paths.Exclude,
	})

	duration := int(time.Since(start).Milliseconds())

	// Only the per-run deadline is a measurement failure; a cancelled parent
	// context means the whole scan is stopping and is reported as such.
	if err != nil && ctx.Err() == nil && errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		err = fmt.Errorf("timed out after %s", runnerTimeout)
	}

	if err != nil {
		return Result{
			MetricID:   rm.metric.ID,
			MetricHash: rm.hash,
			Commit:     commit,
			Status:     StatusError,
			Error:      err.Error(),
			DurationMs: duration,
		}
	}

	return Result{
		MetricID:   rm.metric.ID,
		MetricHash: rm.hash,
		Commit:     commit,
		Value:      res.Value,
		Files:      res.Files,
		Status:     StatusOK,
		DurationMs: duration,
	}
}

// Measure runs all metrics at a single commit and returns results.
// The commit ref is resolved to its SHA and author date.
// One metric's failure does not prevent others from running.
func Measure(ctx context.Context, cfg config.Config, repoPath, commit string) ([]Result, error) {
	meta, err := vcs.ResolveCommit(ctx, repoPath, commit)
	if err != nil {
		return nil, fmt.Errorf("resolving commit %q: %w", commit, err)
	}

	resolved, err := resolveMetrics(cfg.Metrics)
	if err != nil {
		return nil, err
	}

	var results []Result
	for i := range resolved {
		r := runResolved(ctx, &resolved[i], repoPath, meta.SHA)
		r.AuthorDate = meta.AuthorDate
		results = append(results, r)
	}
	return results, nil
}

// scanTask represents a single (commit, metric) unit of work.
type scanTask struct {
	index  int // preserves output order
	commit vcs.CommitMeta
	rm     *resolvedMetric
}

// Scan traverses commits per traversal, runs matching metrics at each commit,
// and calls onResult for each measurement. Metrics run concurrently across
// commits using a bounded worker pool. Results stream to onResult in commit
// order as they become ready.
func Scan(ctx context.Context, cfg config.Config, repoPath string, opts ScanOptions, onResult func(Result)) error {
	skip := make(map[MeasuredKey]bool)
	for _, k := range opts.AlreadyMeasured {
		skip[k] = true
	}

	// Group metrics by traversal and pre-resolve runners.
	byTraversal := make(map[string][]config.Metric)
	for _, m := range cfg.Metrics {
		byTraversal[m.Traversal] = append(byTraversal[m.Traversal], m)
	}

	travNames := make([]string, 0, len(byTraversal))
	for name := range byTraversal {
		travNames = append(travNames, name)
	}
	sort.Strings(travNames)

	for _, travName := range travNames {
		metrics := byTraversal[travName]
		trav, ok := cfg.Traversals[travName]
		if !ok {
			return fmt.Errorf("traversal %q not found in config", travName)
		}

		resolved, err := resolveMetrics(metrics)
		if err != nil {
			return err
		}

		firstParent := trav.Mode == "first_parent"
		every := trav.Sampling.Every
		if every < 1 {
			every = 1
		}

		commits, err := vcs.ListCommits(ctx, repoPath, trav.Range.Start, trav.Range.End, firstParent, every)
		if err != nil {
			return fmt.Errorf("listing commits for traversal %q: %w", travName, err)
		}

		// Build task list, skipping already-measured pairs.
		var tasks []scanTask
		for _, c := range commits {
			for i := range resolved {
				if skip[MeasuredKey{MetricID: resolved[i].metric.ID, MetricHash: resolved[i].hash, Commit: c.SHA}] {
					continue
				}
				tasks = append(tasks, scanTask{
					index:  len(tasks),
					commit: c,
					rm:     &resolved[i],
				})
			}
		}

		if len(tasks) == 0 {
			continue
		}

		if err := runTasks(ctx, tasks, repoPath, onResult); err != nil {
			return err
		}
	}

	return nil
}

// runTasks executes scan tasks concurrently and delivers results to onResult
// in original order as they complete. Returns early if ctx is cancelled.
func runTasks(ctx context.Context, tasks []scanTask, repoPath string, onResult func(Result)) error {
	workers := runtime.NumCPU()
	if workers > len(tasks) {
		workers = len(tasks)
	}

	results := make([]Result, len(tasks))
	ready := make([]chan struct{}, len(tasks))
	for i := range ready {
		ready[i] = make(chan struct{})
	}

	taskCh := make(chan scanTask, workers*2)

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range taskCh {
				if ctx.Err() != nil {
					close(ready[t.index])
					continue
				}
				r := runResolved(ctx, t.rm, repoPath, t.commit.SHA)
				r.AuthorDate = t.commit.AuthorDate
				results[t.index] = r
				close(ready[t.index])
			}
		}()
	}

	// Deliver results in order as they become ready.
	deliverDone := make(chan error, 1)
	go func() {
		for i := range results {
			select {
			case <-ready[i]:
				if ctx.Err() != nil {
					deliverDone <- ctx.Err()
					return
				}
				onResult(results[i])
			case <-ctx.Done():
				deliverDone <- ctx.Err()
				return
			}
		}
		deliverDone <- nil
	}()

	// Feed tasks to workers. Respects ctx so we don't block on a full channel.
loop:
	for _, t := range tasks {
		select {
		case taskCh <- t:
		case <-ctx.Done():
			break loop
		}
	}
	close(taskCh)
	wg.Wait()

	return <-deliverDone
}
