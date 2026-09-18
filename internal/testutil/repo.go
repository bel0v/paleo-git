package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// CreateFixtureRepo creates a temporary git repo with a known commit history.
// Returns the repo path. The repo is cleaned up when the test finishes.
//
// Commit history (oldest first):
//
//	commit 1: adds src/a.ts with "import { foo } from '@legacy/utils'"
//	commit 2: adds src/b.ts with "import { bar } from '@legacy/components'"
//	commit 3: adds src/c.tsx with "import { baz } from '@new/components'"
//	commit 4: modifies src/a.ts to add a second legacy import line
//	commit 5: adds test/d.test.ts with "import { foo } from '@legacy/utils'"
func CreateFixtureRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	run := func(args ...string) { t.Helper(); runGit(t, repoDir, args...) }
	write := func(relPath, content string) { t.Helper(); writeFile(t, repoDir, relPath, content) }

	run("git", "init")
	run("git", "checkout", "-b", "main")

	// Commit 1
	write("src/a.ts", "import { foo } from '@legacy/utils'\nconsole.log(foo)\n")
	run("git", "add", ".")
	run("git", "commit", "-m", "Add src/a.ts with legacy import")

	// Commit 2
	write("src/b.ts", "import { bar } from '@legacy/components'\nconsole.log(bar)\n")
	run("git", "add", ".")
	run("git", "commit", "-m", "Add src/b.ts with legacy import")

	// Commit 3
	write("src/c.tsx", "import { baz } from '@new/components'\nconsole.log(baz)\n")
	run("git", "add", ".")
	run("git", "commit", "-m", "Add src/c.tsx with new import")

	// Commit 4
	write("src/a.ts", "import { foo } from '@legacy/utils'\nimport { qux } from '@legacy/helpers'\nconsole.log(foo, qux)\n")
	run("git", "add", ".")
	run("git", "commit", "-m", "Add second legacy import to src/a.ts")

	// Commit 5
	write("test/d.test.ts", "import { foo } from '@legacy/utils'\ndescribe('test', () => {})\n")
	run("git", "add", ".")
	run("git", "commit", "-m", "Add test file with legacy import")

	return repoDir
}

// CreateDatedRepo creates a repo whose only commits are one file each, made
// at the given RFC3339 author/committer dates in order. Unlike
// CreateFixtureRepo it has no "now"-dated commits, so tests about dates see
// exactly the history they describe.
func CreateDatedRepo(t *testing.T, dates ...string) string {
	t.Helper()
	repoDir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGit(t, repoDir, "git", "init")
	runGit(t, repoDir, "git", "checkout", "-b", "main")
	for i, date := range dates {
		CommitFileAt(t, repoDir, fmt.Sprintf("dated/%d.ts", i), date, date)
	}
	return repoDir
}

// CommitFile writes a file into an existing fixture repo and commits it on
// top of HEAD.
func CommitFile(t *testing.T, repoDir, relPath, content string) {
	t.Helper()
	writeFile(t, repoDir, relPath, content)
	runGit(t, repoDir, "git", "add", ".")
	runGit(t, repoDir, "git", "commit", "-m", "Add "+relPath)
}

// CommitFileAt is CommitFile with the author and committer date both set to
// the given RFC3339 timestamp, for tests that depend on commit dates.
func CommitFileAt(t *testing.T, repoDir, relPath, content, date string) {
	t.Helper()
	CommitFileAtDates(t, repoDir, relPath, content, date, date)
}

// CommitFileAtDates is CommitFile with separate author and committer dates,
// the shape a rebased commit has: written at authorDate, landed at
// committerDate.
func CommitFileAtDates(t *testing.T, repoDir, relPath, content, authorDate, committerDate string) {
	t.Helper()
	writeFile(t, repoDir, relPath, content)
	runGit(t, repoDir, "git", "add", ".")
	runGitEnv(t, repoDir, []string{"GIT_AUTHOR_DATE=" + authorDate, "GIT_COMMITTER_DATE=" + committerDate},
		"git", "commit", "-m", "Add "+relPath+" at "+committerDate)
}

func runGit(t *testing.T, repoDir string, args ...string) {
	t.Helper()
	runGitEnv(t, repoDir, nil, args...)
}

func runGitEnv(t *testing.T, repoDir string, extraEnv []string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	cmd.Env = append(cmd.Env, extraEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %v failed: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, repoDir, relPath, content string) {
	t.Helper()
	absPath := filepath.Join(repoDir, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
