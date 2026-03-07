package vcs

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type CommitMeta struct {
	SHA        string
	AuthorDate time.Time
	Order      int
}

// ListCommits returns commits in the range (start, end] in oldest-first order.
// If firstParent is true, only follows first parents (linear history).
// The every parameter controls sampling stride: 1 = every commit, 2 = every other, etc.
func ListCommits(repoPath, start, end string, firstParent bool, every int) ([]CommitMeta, error) {
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

	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git rev-list: %w", err)
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

// ListChangedFiles returns file paths changed in the given commit vs its parent.
func ListChangedFiles(repoPath, commit string) ([]string, error) {
	if err := validateRepoPath(repoPath); err != nil {
		return nil, err
	}
	if err := validateRef(commit, "commit"); err != nil {
		return nil, err
	}

	out, err := exec.Command("git", "-C", repoPath, "--no-pager", "diff-tree", "--no-commit-id", "-r", "--name-only", commit).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff-tree: %w", err)
	}

	var files []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// GrepCount counts lines matching a pattern at a given commit.
// If paths is non-empty, only files under those paths are searched.
// Returns the match count and the list of files that contained matches.
func GrepCount(repoPath, commit, pattern string, paths []string) (int, []string, error) {
	if err := validateRepoPath(repoPath); err != nil {
		return 0, nil, err
	}
	if err := validateRef(commit, "commit"); err != nil {
		return 0, nil, err
	}
	if err := validatePattern(pattern); err != nil {
		return 0, nil, err
	}

	args := []string{"-C", repoPath, "--no-pager", "grep", "-c", "-e", pattern, commit}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}

	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		// git grep exits 1 when no matches found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return 0, nil, nil
		}
		return 0, nil, fmt.Errorf("git grep: %w", err)
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
