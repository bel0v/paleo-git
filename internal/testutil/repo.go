package testutil

import (
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

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	write := func(relPath, content string) {
		t.Helper()
		absPath := filepath.Join(repoDir, relPath)
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

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
