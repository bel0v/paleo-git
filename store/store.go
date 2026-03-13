package store

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bel0v/paleo-git/engine"
)

// Dir provides read/write access to a data directory of metrics/*.jsonl files.
type Dir struct {
	path string
}

// NewDir creates a Dir for the given data directory path.
func NewDir(path string) Dir {
	return Dir{path: path}
}

// safeMetricID returns an error if the metric ID contains path separators
// or traversal sequences that could escape the metrics directory.
func safeMetricID(id string) error {
	if id == "" {
		return fmt.Errorf("store: metric id must not be empty")
	}
	if strings.ContainsAny(id, "/\\") || id == "." || id == ".." || strings.Contains(id, "..") {
		return fmt.Errorf("store: unsafe metric id %q", id)
	}
	return nil
}

// Read returns all results for a single metric. Returns an empty slice
// (not an error) if the file does not exist.
func (d Dir) Read(ctx context.Context, metricID string) ([]engine.Result, error) {
	if err := safeMetricID(metricID); err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(d.path, "metrics", metricID+".jsonl"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading metric %q: %w", metricID, err)
	}
	defer f.Close()

	var results []engine.Result
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r engine.Result
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, fmt.Errorf("parsing line in %s.jsonl: %w", metricID, err)
		}
		results = append(results, r)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading metric %q: %w", metricID, err)
	}
	return results, nil
}

// AlreadyMeasured reads all metric files and returns deduplicated
// (MetricID, MetricHash, Commit) keys for use as a skip list.
func (d Dir) AlreadyMeasured(ctx context.Context) ([]engine.MeasuredKey, error) {
	pattern := filepath.Join(d.path, "metrics", "*.jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("globbing metrics: %w", err)
	}

	seen := make(map[engine.MeasuredKey]bool)
	var keys []engine.MeasuredKey

	for _, path := range matches {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		metricID := strings.TrimSuffix(filepath.Base(path), ".jsonl")
		results, err := d.Read(ctx, metricID)
		if err != nil {
			return nil, err
		}
		for _, r := range results {
			k := engine.MeasuredKey{MetricID: r.MetricID, MetricHash: r.MetricHash, Commit: r.Commit}
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	return keys, nil
}

// Append groups results by MetricID and appends each to its
// corresponding metrics/{metric_id}.jsonl file, creating the
// directory and file if needed.
func (d Dir) Append(ctx context.Context, results []engine.Result) error {
	grouped := make(map[string][]engine.Result)
	for _, r := range results {
		if err := safeMetricID(r.MetricID); err != nil {
			return err
		}
		grouped[r.MetricID] = append(grouped[r.MetricID], r)
	}

	metricsDir := filepath.Join(d.path, "metrics")
	if err := os.MkdirAll(metricsDir, 0o755); err != nil {
		return fmt.Errorf("creating metrics directory: %w", err)
	}

	for metricID, batch := range grouped {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := appendToFile(filepath.Join(metricsDir, metricID+".jsonl"), batch); err != nil {
			return err
		}
	}
	return nil
}

func appendToFile(path string, results []engine.Result) (retErr error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening %s: %w", filepath.Base(path), err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && retErr == nil {
			retErr = fmt.Errorf("closing %s: %w", filepath.Base(path), closeErr)
		}
	}()

	for _, r := range results {
		line, err := json.Marshal(r)
		if err != nil {
			return fmt.Errorf("marshalling result: %w", err)
		}
		line = append(line, '\n')
		if _, err := f.Write(line); err != nil {
			return fmt.Errorf("writing to %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}
