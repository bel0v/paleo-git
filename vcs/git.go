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
	cmd := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("git %s: %w", findSubcommand(args), ctx.Err())
		}
		return nil, gitError(args, stderr.String(), err)
	}
	return out, nil
}

type CommitMeta struct {
	SHA        string
	AuthorDate time.Time
	Order      int
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
	order := 0
	for scanner.Scan() {
		line := scanner.Text()
		// rev-list --format outputs alternating "commit <sha>" and "<sha> <date>" lines
		if strings.HasPrefix(line, "commit ") {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		t, err := time.Parse(time.RFC3339, parts[1])
		if err != nil {
			continue
		}
		order++
		all = append(all, CommitMeta{SHA: parts[0], AuthorDate: t, Order: order})
	}

	if every <= 1 {
		return all, nil
	}

	var sampled []CommitMeta
	for i, c := range all {
		if i%(every) == 0 {
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

	args := []string{"-C", repoPath, "--no-pager", "grep", "-P", "-c", "-e", pattern, commit}
	if len(includePaths) > 0 || len(excludePaths) > 0 {
		args = append(args, "--")
		args = append(args, includePaths...)
		for _, ex := range excludePaths {
			args = append(args, ":!"+ex)
		}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return 0, nil, fmt.Errorf("git %s: %w", findSubcommand(args), ctx.Err())
		}
		// git grep exits 1 when no matches found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return 0, nil, nil
		}
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "cannot use Perl") || strings.Contains(stderrStr, "PCRE") {
			return 0, nil, fmt.Errorf("git grep -P (Perl regex) not supported; install git with PCRE support (e.g. brew install git)")
		}
		return 0, nil, gitError(args, stderrStr, err)
	}

	count := 0
	var files []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		// Format: "<commit>:<path>:<count>" or "<path>:<count>"
		// With commit ref it's "abc123:src/file.ts:2"
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			continue
		}
		countStr := parts[len(parts)-1]
		n, err := strconv.Atoi(countStr)
		if err != nil {
			continue
		}
		// File path is between commit ref and count
		filePath := strings.Join(parts[1:len(parts)-1], ":")
		count += n
		files = append(files, filePath)
	}
	return count, files, nil
}
