package vcs

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// flagsWithValue lists git global flags that consume the next argument.
var flagsWithValue = map[string]bool{
	"-C":          true,
	"-c":          true,
	"--git-dir":   true,
	"--work-tree": true,
	"--namespace": true,
}

// findSubcommand extracts the git subcommand from args, skipping known
// flags (and their values) to find the first positional argument.
func findSubcommand(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsWithValue[arg] {
			i++ // skip flag value
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg
	}
	return "unknown"
}

// gitError formats an error from a git command with the subcommand name.
func gitError(args []string, stderr string, err error) error {
	subcmd := findSubcommand(args)
	msg := strings.TrimSpace(stderr)
	if msg != "" {
		return fmt.Errorf("git %s: %s", subcmd, msg)
	}
	return fmt.Errorf("git %s: %w", subcmd, err)
}

// gitRun executes a git command and returns stdout. If the command fails,
// the error includes stderr for actionable diagnostics.
func gitRun(ctx context.Context, args ...string) ([]byte, error) {
	return runGit(ctx, args, false)
}

// runGit executes git. With tolerateExit1, an exit status of 1 yields nil
// output and no error: git grep uses it to mean "nothing found".
func runGit(ctx context.Context, args []string, tolerateExit1 bool) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("git %s: %w", findSubcommand(args), ctx.Err())
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 && tolerateExit1 {
		return nil, nil
	}
	return nil, gitError(args, stderr.String(), err)
}

// CommitMeta identifies a commit and when it landed on the traversed branch.
// CommitDate is git's committer date: for a merge or a rebased commit that is
// the moment it reached the branch, unlike the author date, which is when the
// change was written and may be far older.
type CommitMeta struct {
	SHA        string
	CommitDate time.Time
}

// ResolveCommit returns metadata for a single commit ref.
func ResolveCommit(ctx context.Context, repoPath, ref string) (CommitMeta, error) {
	if err := validateRepoPath(repoPath); err != nil {
		return CommitMeta{}, err
	}
	if err := validateRef(ref, "ref"); err != nil {
		return CommitMeta{}, err
	}

	out, err := gitRun(ctx, "-C", repoPath, "--no-pager", "log", "-1", "--format=%H %cI", ref)
	if err != nil {
		return CommitMeta{}, err
	}

	line := strings.TrimSpace(string(out))
	parts := strings.SplitN(line, " ", 2)
	if len(parts) != 2 {
		return CommitMeta{}, fmt.Errorf("unexpected git log output: %q", line)
	}

	t, err := time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return CommitMeta{}, fmt.Errorf("parsing commit date: %w", err)
	}

	return CommitMeta{SHA: parts[0], CommitDate: t}, nil
}

// Sampling selects which commits of a traversal are measured.
//
// Bucket "commit" makes Every a stride over the commit list counted from the
// range start (1 = every commit). Buckets "day", "week" and "month" are UTC
// calendar buckets of the committer date: the last commit that landed in each
// is kept and Every strides over buckets, counted from the Unix epoch so the
// chosen buckets do not shift when the range start moves. The last commit of
// the range is always included.
type Sampling struct {
	Bucket string
	Every  int
}

var buckets = map[string]bool{"commit": true, "day": true, "week": true, "month": true}

var isoDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// IsDate reports whether s is a YYYY-MM-DD date rather than a git revision.
func IsDate(s string) bool {
	return isoDate.MatchString(s)
}

// ResolveStart turns a traversal start into a revision. A git revision is
// returned unchanged; a YYYY-MM-DD date resolves to the commit just before
// the first commit (in walk order, first parents only when firstParent) that
// landed on or after that day, so the traversal (start, end] begins with that
// first commit. If every commit landed on or after the day the result is "",
// which ListCommits treats as "from the first commit".
//
// Committer dates on a first-parent walk are monotonic in practice, so the
// oldest-first walk and "the newest commit before the day" agree; the walk
// is used because it also degrades safely (starting early, never late) on
// the odd repository where they don't.
func ResolveStart(ctx context.Context, repoPath, start, end string, firstParent bool) (string, error) {
	if !IsDate(start) {
		return start, nil
	}
	day, err := time.Parse("2006-01-02", start)
	if err != nil {
		return "", fmt.Errorf("start: invalid date %q: %w", start, err)
	}
	commits, err := listCommits(ctx, repoPath, "", end, firstParent)
	if err != nil {
		return "", err
	}
	for i, c := range commits {
		if c.CommitDate.Before(day) {
			continue
		}
		if i == 0 {
			return "", nil
		}
		return commits[i-1].SHA, nil
	}
	return "", fmt.Errorf("start: no commit reachable from %s landed on or after %s", end, start)
}

// ListCommits returns the sampled commits in the range (start, end] in
// oldest-first order, or every commit reachable from end when start is "".
// If firstParent is true, only first parents are followed (linear history).
func ListCommits(ctx context.Context, repoPath, start, end string, firstParent bool, sampling Sampling) ([]CommitMeta, error) {
	if start != "" {
		if err := validateRef(start, "start"); err != nil {
			return nil, err
		}
	}
	if !buckets[sampling.Bucket] {
		return nil, fmt.Errorf("sampling: unknown bucket %q", sampling.Bucket)
	}
	if sampling.Every < 1 {
		return nil, fmt.Errorf("sampling: every must be at least 1 (got %d)", sampling.Every)
	}
	all, err := listCommits(ctx, repoPath, start, end, firstParent)
	if err != nil {
		return nil, err
	}
	return sample(all, sampling), nil
}

// listCommits lists (start, end] oldest first, or everything reachable from
// end when start is empty.
func listCommits(ctx context.Context, repoPath, start, end string, firstParent bool) ([]CommitMeta, error) {
	if err := validateRepoPath(repoPath); err != nil {
		return nil, err
	}
	if err := validateRef(end, "end"); err != nil {
		return nil, err
	}

	args := []string{"-C", repoPath, "--no-pager", "rev-list", "--format=%H %cI", "--reverse"}
	if firstParent {
		args = append(args, "--first-parent")
	}
	if start == "" {
		args = append(args, end)
	} else {
		args = append(args, start+".."+end)
	}

	out, err := gitRun(ctx, args...)
	if err != nil {
		return nil, err
	}

	var all []CommitMeta
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		// rev-list --format outputs alternating "commit <sha>" and "<sha> <date>" lines
		if strings.HasPrefix(line, "commit ") {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("unexpected rev-list line: %q", line)
		}
		t, err := time.Parse(time.RFC3339, parts[1])
		if err != nil {
			return nil, fmt.Errorf("parsing commit date in rev-list output: %w", err)
		}
		all = append(all, CommitMeta{SHA: parts[0], CommitDate: t})
	}
	return all, nil
}

func sample(all []CommitMeta, sampling Sampling) []CommitMeta {
	if len(all) == 0 {
		return nil
	}
	every := sampling.Every
	var picked []CommitMeta
	if sampling.Bucket == "commit" {
		for i, c := range all {
			if i%every == 0 {
				picked = append(picked, c)
			}
		}
	} else {
		latest := make(map[int64]CommitMeta)
		for _, c := range all {
			key := bucketIndex(c.CommitDate, sampling.Bucket)
			if prev, seen := latest[key]; !seen || c.CommitDate.After(prev.CommitDate) {
				latest[key] = c
			}
		}
		keys := make([]int64, 0, len(latest))
		for key := range latest {
			if key%int64(every) == 0 {
				keys = append(keys, key)
			}
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		for _, key := range keys {
			picked = append(picked, latest[key])
		}
	}
	// The tip is always measured. In bucket mode it is normally its bucket's
	// own pick; add by identity so it is never duplicated, and keep the
	// output in commit-date order.
	tip := all[len(all)-1]
	if !slices.ContainsFunc(picked, func(c CommitMeta) bool { return c.SHA == tip.SHA }) {
		picked = append(picked, tip)
	}
	if sampling.Bucket != "commit" {
		sort.SliceStable(picked, func(i, j int) bool { return picked[i].CommitDate.Before(picked[j].CommitDate) })
	}
	return picked
}

// bucketIndex numbers UTC calendar buckets from the Unix epoch: days since
// 1970-01-01, ISO weeks (Monday-based) since 1969-12-29, or months since
// 1970-01.
func bucketIndex(t time.Time, bucket string) int64 {
	u := t.UTC()
	days := u.Unix() / 86400
	switch bucket {
	case "week":
		return (days + 3) / 7
	case "month":
		return int64(u.Year())*12 + int64(u.Month()) - 1
	default:
		return days
	}
}

// GrepCount counts lines matching a pattern at a given commit.
// includePaths limits the search to matching paths. excludePaths removes paths from the search.
// Returns the match count and the list of files that contained matches.
func GrepCount(ctx context.Context, repoPath, commit, pattern string, includePaths, excludePaths []string) (int, []string, error) {
	if err := validateRepoPath(repoPath); err != nil {
		return 0, nil, err
	}
	if err := validateRef(commit, "commit"); err != nil {
		return 0, nil, err
	}
	if err := validatePattern(pattern); err != nil {
		return 0, nil, err
	}

	args := grepArgs(repoPath, "-c", "-e", pattern, commit)
	args = appendPathspec(args, includePaths, excludePaths)

	out, err := gitGrep(ctx, args)
	if err != nil {
		return 0, nil, err
	}

	// With -z, each record is "<commit>:<path>\0<count>\n": the path is
	// NUL-terminated instead of colon-terminated, so paths containing colons
	// stay unambiguous. The commit is echoed back exactly as it was passed.
	prefix := commit + ":"
	count := 0
	var files []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		path, countStr, ok := strings.Cut(line, "\x00")
		if !ok {
			return 0, nil, fmt.Errorf("unexpected git grep output: %q", line)
		}
		n, err := strconv.Atoi(countStr)
		if err != nil {
			return 0, nil, fmt.Errorf("parsing match count: %w", err)
		}
		count += n
		files = append(files, strings.TrimPrefix(path, prefix))
	}
	return count, files, nil
}

// ListFiles returns the paths of all files at a commit that fall under
// includePaths and outside excludePaths.
//
// It runs `git grep -L` with a pattern that can never match, so every file
// under the pathspec is reported as "unmatched" — including empty and binary
// files. git grep is used instead of ls-tree because it is the only tree
// listing command that honours the full pathspec syntax (":(glob)" etc.),
// keeping include/exclude semantics identical to GrepCount.
func ListFiles(ctx context.Context, repoPath, commit string, includePaths, excludePaths []string) ([]string, error) {
	if err := validateRepoPath(repoPath); err != nil {
		return nil, err
	}
	if err := validateRef(commit, "commit"); err != nil {
		return nil, err
	}

	args := grepArgs(repoPath, "-L", "-e", "(?!)", commit)
	args = appendPathspec(args, includePaths, excludePaths)

	out, err := gitGrep(ctx, args)
	if err != nil {
		return nil, err
	}

	// With -L -z, each record is "<commit>:<path>\0".
	prefix := commit + ":"
	var files []string
	for _, record := range strings.Split(string(out), "\x00") {
		if record == "" {
			continue
		}
		files = append(files, strings.TrimPrefix(record, prefix))
	}
	return files, nil
}

// grepArgs builds a git grep invocation. grep.threads is pinned to 1: the
// engine already runs one git process per CPU, and git's own default of 8
// threads per process oversubscribes the machine (40% slower on a full scan).
func grepArgs(repoPath string, extra ...string) []string {
	args := []string{"-C", repoPath, "--no-pager", "-c", "grep.threads=1", "grep", "-P", "-z"}
	return append(args, extra...)
}

func appendPathspec(args, includePaths, excludePaths []string) []string {
	if len(includePaths) == 0 && len(excludePaths) == 0 {
		return args
	}
	args = append(args, "--")
	args = append(args, includePaths...)
	for _, ex := range excludePaths {
		args = append(args, ":!"+ex)
	}
	return args
}

// gitGrep runs a git grep invocation. Exit status 1 (nothing reported) yields
// empty output and no error.
func gitGrep(ctx context.Context, args []string) ([]byte, error) {
	out, err := runGit(ctx, args, true)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "cannot use Perl") || strings.Contains(msg, "PCRE") {
			return nil, fmt.Errorf("git grep -P (Perl regex) not supported; install git with PCRE support (e.g. brew install git)")
		}
		return nil, err
	}
	return out, nil
}
