package vcs

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// flagsWithValue lists git global flags that consume the next argument.
var flagsWithValue = map[string]bool{
	"-C":             true,
	"-c":             true,
	"--git-dir":      true,
	"--work-tree":    true,
	"--namespace":    true,
	"--super-prefix": true,
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
	out, _, err := runGit(ctx, args, false)
	return out, err
}

// runGit executes git. With tolerateExit1, an exit status of 1 is reported
// through the returned bool with nil output and no error: git grep uses it
// to mean "nothing found".
func runGit(ctx context.Context, args []string, tolerateExit1 bool) (out []byte, nothingFound bool, err error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err = cmd.Output()
	if err == nil {
		return out, false, nil
	}
	if ctx.Err() != nil {
		return nil, false, fmt.Errorf("git %s: %w", findSubcommand(args), ctx.Err())
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 && tolerateExit1 {
		return nil, true, nil
	}
	return nil, false, gitError(args, stderr.String(), err)
}

type CommitMeta struct {
	SHA        string
	AuthorDate time.Time
}

// ResolveCommit returns metadata for a single commit ref.
func ResolveCommit(ctx context.Context, repoPath, ref string) (CommitMeta, error) {
	if err := validateRepoPath(repoPath); err != nil {
		return CommitMeta{}, err
	}
	if err := validateRef(ref, "ref"); err != nil {
		return CommitMeta{}, err
	}

	out, err := gitRun(ctx, "-C", repoPath, "--no-pager", "log", "-1", "--format=%H %aI", ref)
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
		return CommitMeta{}, fmt.Errorf("parsing author date: %w", err)
	}

	return CommitMeta{SHA: parts[0], AuthorDate: t}, nil
}

// ListCommits returns commits in the range (start, end] in oldest-first order.
// If firstParent is true, only follows first parents (linear history).
// The every parameter controls sampling stride: 1 = every commit, 2 = every other, etc.
func ListCommits(ctx context.Context, repoPath, start, end string, firstParent bool, every int) ([]CommitMeta, error) {
	if err := validateRepoPath(repoPath); err != nil {
		return nil, err
	}
	if err := validateRef(start, "start"); err != nil {
		return nil, err
	}
	if err := validateRef(end, "end"); err != nil {
		return nil, err
	}

	args := []string{"-C", repoPath, "--no-pager", "rev-list", "--format=%H %aI", "--reverse"}
	if firstParent {
		args = append(args, "--first-parent")
	}
	args = append(args, start+".."+end)

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
			return nil, fmt.Errorf("parsing author date in rev-list output: %w", err)
		}
		all = append(all, CommitMeta{SHA: parts[0], AuthorDate: t})
	}

	if every <= 1 {
		return all, nil
	}

	var sampled []CommitMeta
	for i, c := range all {
		if i%every == 0 {
			sampled = append(sampled, c)
		}
	}
	// Always include the last commit
	if len(all) > 0 && (len(sampled) == 0 || sampled[len(sampled)-1].SHA != all[len(all)-1].SHA) {
		sampled = append(sampled, all[len(all)-1])
	}
	return sampled, nil
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
	out, _, err := runGit(ctx, args, true)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "cannot use Perl") || strings.Contains(msg, "PCRE") {
			return nil, fmt.Errorf("git grep -P (Perl regex) not supported; install git with PCRE support (e.g. brew install git)")
		}
		return nil, err
	}
	return out, nil
}
