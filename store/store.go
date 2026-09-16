package store

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/bel0v/paleo-git/engine"
)

// Dir provides read/write access to a data directory:
//
//	metrics/<metric_id>.jsonl   one Row per line, append-only
//	files/<files_ref>.json      a sorted JSON array of paths, written once
//
// File lists repeat almost verbatim from one commit to the next, so rows
// carry a content hash of their file set instead of the list itself.
type Dir struct {
	path string
}

// Row is a stored measurement: an engine.Result whose Files have been
// replaced by FilesRef, the content hash of the file set. Load the paths
// with Dir.Files.
type Row struct {
	engine.Result
	FilesRef string `json:"files_ref,omitempty"`
}

// FileSetHash returns the content hash of a set of paths, independent of
// their order. It is the files_ref of every row measuring that exact set.
func FileSetHash(files []string) string {
	sorted := slices.Clone(files)
	slices.Sort(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\x00")))
	return hex.EncodeToString(sum[:16])
}

var filesRefPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

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

// Read returns all rows for a single metric, oldest first. Returns an empty
// slice (not an error) if the file does not exist.
func (d Dir) Read(ctx context.Context, metricID string) ([]Row, error) {
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

	var rows []Row
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r Row
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, fmt.Errorf("parsing line in %s.jsonl: %w", metricID, err)
		}
		rows = append(rows, r)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading metric %q: %w", metricID, err)
	}
	return rows, nil
}

// Files returns the sorted paths of the file set a Row refers to.
func (d Dir) Files(_ context.Context, filesRef string) ([]string, error) {
	if !filesRefPattern.MatchString(filesRef) {
		return nil, fmt.Errorf("store: invalid files_ref %q", filesRef)
	}
	data, err := os.ReadFile(d.fileSetPath(filesRef))
	if err != nil {
		return nil, fmt.Errorf("reading file set %s: %w", filesRef, err)
	}
	var files []string
	if err := json.Unmarshal(data, &files); err != nil {
		return nil, fmt.Errorf("parsing file set %s: %w", filesRef, err)
	}
	return files, nil
}

func (d Dir) fileSetPath(filesRef string) string {
	return filepath.Join(d.path, "files", filesRef+".json")
}

// writeFileSet stores a file set under its hash unless it already exists.
// The write goes through a temp file and rename so a crash never leaves a
// truncated set behind under a valid name.
func (d Dir) writeFileSet(files []string) (string, error) {
	ref := FileSetHash(files)
	path := d.fileSetPath(ref)
	if _, err := os.Stat(path); err == nil {
		return ref, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("creating files directory: %w", err)
	}
	sorted := slices.Clone(files)
	slices.Sort(sorted)
	data, err := json.Marshal(sorted)
	if err != nil {
		return "", fmt.Errorf("marshalling file set: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ref+".*.tmp")
	if err != nil {
		return "", fmt.Errorf("creating temp file set: %w", err)
	}
	discard := func() { _ = os.Remove(tmp.Name()) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		discard()
		return "", fmt.Errorf("writing file set %s: %w", ref, err)
	}
	if err := tmp.Close(); err != nil {
		discard()
		return "", fmt.Errorf("closing file set %s: %w", ref, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		discard()
		return "", fmt.Errorf("storing file set %s: %w", ref, err)
	}
	return ref, nil
}

// AlreadyMeasured reads all metric files and returns deduplicated
// (MetricID, MetricHash, Commit) keys for use as a skip list. Results with
// an error status are not included, so a failed measurement is retried on
// the next run rather than becoming a permanent gap.
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
			if r.Status != engine.StatusOK {
				continue
			}
			k := engine.MeasuredKey{MetricID: r.MetricID, MetricHash: r.MetricHash, Commit: r.Commit}
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	return keys, nil
}

// Append stores results: each file set is written once under files/ and
// each result becomes a Row appended to metrics/<metric_id>.jsonl, creating
// directories and files as needed.
func (d Dir) Append(ctx context.Context, results []engine.Result) error {
	grouped := make(map[string][]Row)
	for _, r := range results {
		if err := safeMetricID(r.MetricID); err != nil {
			return err
		}
		row := Row{Result: r}
		if len(r.Files) > 0 {
			ref, err := d.writeFileSet(r.Files)
			if err != nil {
				return err
			}
			row.FilesRef = ref
		}
		row.Files = nil
		grouped[r.MetricID] = append(grouped[r.MetricID], row)
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

func appendToFile(path string, rows []Row) (retErr error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening %s: %w", filepath.Base(path), err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && retErr == nil {
			retErr = fmt.Errorf("closing %s: %w", filepath.Base(path), closeErr)
		}
	}()

	for _, r := range rows {
		line, err := json.Marshal(r)
		if err != nil {
			return fmt.Errorf("marshalling row: %w", err)
		}
		line = append(line, '\n')
		if _, err := f.Write(line); err != nil {
			return fmt.Errorf("writing to %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}
