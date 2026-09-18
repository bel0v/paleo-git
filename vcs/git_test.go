package vcs

import (
	"context"
	"fmt"
	"testing"

	"github.com/bel0v/paleo-git/internal/testutil"
)

func TestListCommits_FirstParentReturnsLinearHistory(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	commits, err := ListCommits(context.Background(), repo, "HEAD~4", "HEAD", true, Sampling{Bucket: "commit", Every: 1})
	if err != nil {
		t.Fatalf("ListCommits error: %v", err)
	}

	// 5 commits total, HEAD~4 to HEAD inclusive = 4 commits (excludes start)
	if len(commits) < 3 {
		t.Fatalf("expected at least 3 commits, got %d", len(commits))
	}

	// Verify order: oldest first (ascending commit date)
	for i := 1; i < len(commits); i++ {
		if commits[i].CommitDate.Before(commits[i-1].CommitDate) {
			t.Errorf("commits not in ascending order at index %d", i)
		}
	}
}

func TestListCommits_SamplingStrideSkipsCommits(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	all, err := ListCommits(context.Background(), repo, "HEAD~4", "HEAD", true, Sampling{Bucket: "commit", Every: 1})
	if err != nil {
		t.Fatalf("ListCommits error: %v", err)
	}

	sampled, err := ListCommits(context.Background(), repo, "HEAD~4", "HEAD", true, Sampling{Bucket: "commit", Every: 2})
	if err != nil {
		t.Fatalf("ListCommits error: %v", err)
	}

	if len(sampled) >= len(all) {
		t.Errorf("sampled (%d) should be fewer than all (%d)", len(sampled), len(all))
	}
}

func TestGrepCount_ReturnsCorrectCount(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	// At HEAD, there are 3 files with @legacy: src/a.ts (2 lines), src/b.ts (1), test/d.test.ts (1)
	// Total lines matching: 4
	count, files, err := GrepCount(context.Background(), repo, "HEAD", "@legacy/", nil, nil)
	if err != nil {
		t.Fatalf("GrepCount error: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 matches, got %d", count)
	}
	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d: %v", len(files), files)
	}
}

func TestGrepCount_ReturnsRepoRelativeFilePaths(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	meta, err := ResolveCommit(context.Background(), repo, "HEAD")
	if err != nil {
		t.Fatalf("ResolveCommit error: %v", err)
	}

	// git grep echoes the commit back as the "<commit>:" prefix of every
	// record, whether it was given as a symbolic ref or a full SHA. Neither
	// form may leak into the returned file paths.
	for _, ref := range []string{"HEAD", meta.SHA} {
		_, files, err := GrepCount(context.Background(), repo, ref, "@legacy/", nil, nil)
		if err != nil {
			t.Fatalf("GrepCount(%s) error: %v", ref, err)
		}
		want := map[string]bool{"src/a.ts": true, "src/b.ts": true, "test/d.test.ts": true}
		if len(files) != len(want) {
			t.Fatalf("GrepCount(%s): expected %d files, got %d: %q", ref, len(want), len(files), files)
		}
		for _, f := range files {
			if !want[f] {
				t.Errorf("GrepCount(%s): unexpected file path %q", ref, f)
			}
		}
	}
}

func TestGrepCount_RespectsPathFilter(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	// Only search in src/ — should exclude test/d.test.ts
	count, files, err := GrepCount(context.Background(), repo, "HEAD", "@legacy/", []string{"src/"}, nil)
	if err != nil {
		t.Fatalf("GrepCount error: %v", err)
	}
	// src/a.ts has 2 matches, src/b.ts has 1 = 3 total
	if count != 3 {
		t.Errorf("expected 3 matches with src/ filter, got %d", count)
	}
	for _, f := range files {
		if f == "test/d.test.ts" {
			t.Error("test/d.test.ts should not be in filtered results")
		}
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}
}

func TestGrepCount_RespectsExcludeFilter(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	// Search all files but exclude test/
	count, files, err := GrepCount(context.Background(), repo, "HEAD", "@legacy/", nil, []string{"test/"})
	if err != nil {
		t.Fatalf("GrepCount error: %v", err)
	}
	// src/a.ts has 2 matches, src/b.ts has 1 = 3 total (test/d.test.ts excluded)
	if count != 3 {
		t.Errorf("expected 3 matches with test/ excluded, got %d", count)
	}
	for _, f := range files {
		if f == "test/d.test.ts" {
			t.Error("test/d.test.ts should not be in excluded results")
		}
	}
}

func TestListFiles_ReturnsAllFilesUnderPathspec(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	files, err := ListFiles(context.Background(), repo, "HEAD", nil, nil)
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	want := map[string]bool{"src/a.ts": true, "src/b.ts": true, "src/c.tsx": true, "test/d.test.ts": true}
	if len(files) != len(want) {
		t.Fatalf("expected %d files, got %d: %q", len(want), len(files), files)
	}
	for _, f := range files {
		if !want[f] {
			t.Errorf("unexpected file path %q", f)
		}
	}
}

func TestListFiles_RespectsIncludeExcludeAndGlobs(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	files, err := ListFiles(context.Background(), repo, "HEAD", []string{"src/"}, nil)
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	if len(files) != 3 {
		t.Errorf("include src/: expected 3 files, got %d: %q", len(files), files)
	}

	files, err = ListFiles(context.Background(), repo, "HEAD", nil, []string{"test/"})
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	if len(files) != 3 {
		t.Errorf("exclude test/: expected 3 files, got %d: %q", len(files), files)
	}

	files, err = ListFiles(context.Background(), repo, "HEAD", []string{":(glob)**/*.tsx"}, nil)
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	if len(files) != 1 || files[0] != "src/c.tsx" {
		t.Errorf("glob *.tsx: expected [src/c.tsx], got %q", files)
	}
}

func TestListFiles_NoFilesReturnsEmpty(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	files, err := ListFiles(context.Background(), repo, "HEAD", []string{"nonexistent/"}, nil)
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected no files, got %q", files)
	}
}

func TestListFiles_IncludesEmptyFiles(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	testutil.CommitFile(t, repo, "src/empty.ts", "")

	files, err := ListFiles(context.Background(), repo, "HEAD", []string{"src/empty.ts"}, nil)
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	if len(files) != 1 || files[0] != "src/empty.ts" {
		t.Errorf("expected [src/empty.ts], got %q", files)
	}
}

func TestResolveCommit_ReturnsMetadata(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	meta, err := ResolveCommit(context.Background(), repo, "HEAD")
	if err != nil {
		t.Fatalf("ResolveCommit error: %v", err)
	}
	if len(meta.SHA) != 40 {
		t.Errorf("expected 40-char SHA, got %q", meta.SHA)
	}
	if meta.CommitDate.IsZero() {
		t.Error("expected non-zero CommitDate")
	}
}

func TestResolveCommit_RejectsFlagLikeRef(t *testing.T) {
	_, err := ResolveCommit(context.Background(), "/tmp/repo", "--exec=bad")
	if err == nil {
		t.Fatal("expected error for flag-like ref")
	}
}

func TestListCommits_RespectsContextCancellation(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := ListCommits(ctx, repo, "HEAD~4", "HEAD", true, Sampling{Bucket: "commit", Every: 1})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// datedRepo has five commits with controlled commit dates: two on Wed
// 2025-01-01, then 01-02 (Thu), 01-05 (Sun) and 01-06 (Mon, the next ISO
// week).
var datedRepoDates = []string{
	"2025-01-01T09:00:00Z", "2025-01-01T17:00:00Z", "2025-01-02T10:00:00Z",
	"2025-01-05T10:00:00Z", "2025-01-06T10:00:00Z",
}

func datedRepo(t *testing.T) string {
	return testutil.CreateDatedRepo(t, datedRepoDates...)
}

// rebasedTipRepo is datedRepo plus a tip that was written on 2024-06-01 but
// landed (committer date) on 2025-01-07, the shape a rebase-merge leaves.
func rebasedTipRepo(t *testing.T) string {
	repo := datedRepo(t)
	testutil.CommitFileAtDates(t, repo, "dated/rebased.ts", "old", "2024-06-01T10:00:00Z", "2025-01-07T10:00:00Z")
	return repo
}

func days(commits []CommitMeta) []string {
	out := make([]string, len(commits))
	for i, c := range commits {
		out[i] = c.CommitDate.UTC().Format("2006-01-02T15")
	}
	return out
}

func TestListCommits_DayBucketKeepsLatestCommitPerDay(t *testing.T) {
	repo := datedRepo(t)
	got, err := ListCommits(context.Background(), repo, "", "HEAD", true, Sampling{Bucket: "day", Every: 1})
	if err != nil {
		t.Fatalf("ListCommits error: %v", err)
	}
	want := []string{"2025-01-01T17", "2025-01-02T10", "2025-01-05T10", "2025-01-06T10"}
	if fmt.Sprint(days(got)) != fmt.Sprint(want) {
		t.Errorf("day buckets: got %v, want %v", days(got), want)
	}
}

func TestListCommits_WeekBucketFollowsISOWeeks(t *testing.T) {
	repo := datedRepo(t)
	got, err := ListCommits(context.Background(), repo, "", "HEAD", true, Sampling{Bucket: "week", Every: 1})
	if err != nil {
		t.Fatalf("ListCommits error: %v", err)
	}
	// Wed 01-01 .. Sun 01-05 are one ISO week; Mon 01-06 starts the next.
	want := []string{"2025-01-05T10", "2025-01-06T10"}
	if fmt.Sprint(days(got)) != fmt.Sprint(want) {
		t.Errorf("week buckets: got %v, want %v", days(got), want)
	}
}

func TestListCommits_BucketStrideIsAnchoredToEpochNotRangeStart(t *testing.T) {
	repo := datedRepo(t)
	// Day numbers since the epoch: 2025-01-01 = 20089 (odd), 01-02 = 20090,
	// 01-05 = 20093, 01-06 = 20094. every: 2 keeps even days; the tip is
	// 01-06 and is even anyway.
	want := []string{"2025-01-02T10", "2025-01-06T10"}
	for _, start := range []string{"", "HEAD~4"} {
		got, err := ListCommits(context.Background(), repo, start, "HEAD", true, Sampling{Bucket: "day", Every: 2})
		if err != nil {
			t.Fatalf("ListCommits error: %v", err)
		}
		if fmt.Sprint(days(got)) != fmt.Sprint(want) {
			t.Errorf("start %s: got %v, want %v (picks must not move with the start)", start, days(got), want)
		}
	}
}

func TestListCommits_BucketAlwaysIncludesTip(t *testing.T) {
	repo := datedRepo(t)
	// The week of 01-06 has index (20094+3)/7 = 2871, odd, so with every: 2
	// it would be dropped without the tip rule.
	got, err := ListCommits(context.Background(), repo, "", "HEAD", true, Sampling{Bucket: "week", Every: 2})
	if err != nil {
		t.Fatalf("ListCommits error: %v", err)
	}
	if last := got[len(got)-1].CommitDate.UTC().Format("2006-01-02"); last != "2025-01-06" {
		t.Errorf("tip must always be included, last was %s", last)
	}
}

func TestListCommits_RejectsUnknownBucket(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	if _, err := ListCommits(context.Background(), repo, "HEAD~1", "HEAD", true, Sampling{Bucket: "fortnight", Every: 1}); err == nil {
		t.Fatal("expected error for unknown bucket")
	}
	if _, err := ListCommits(context.Background(), repo, "HEAD~1", "HEAD", true, Sampling{Every: 1}); err == nil {
		t.Fatal("expected error for empty bucket: it must be explicit")
	}
}

func TestResolveStart_DateStartsTraversalAtFirstCommitOnOrAfterIt(t *testing.T) {
	repo := datedRepo(t)
	sha, err := ResolveStart(context.Background(), repo, "2025-01-05", "HEAD", true)
	if err != nil {
		t.Fatalf("ResolveStart error: %v", err)
	}
	meta, err := ResolveCommit(context.Background(), repo, sha)
	if err != nil {
		t.Fatal(err)
	}
	if got := meta.CommitDate.UTC().Format("2006-01-02T15"); got != "2025-01-02T10" {
		t.Errorf("start before 2025-01-05 should be the 01-02 commit, got %s", got)
	}
	// The traversal then begins with the first commit on/after the date.
	commits, err := ListCommits(context.Background(), repo, sha, "HEAD", true, Sampling{Bucket: "commit", Every: 1})
	if err != nil {
		t.Fatal(err)
	}
	if first := days(commits)[0]; first != "2025-01-05T10" {
		t.Errorf("traversal should start at 2025-01-05, got %s", first)
	}
}

func TestResolveStart_UsesLandingDateNotAuthorDate(t *testing.T) {
	repo := rebasedTipRepo(t)
	// By author date the tip (2024-06-01) would be "the newest commit before
	// 2025-01-05" and the traversal would be empty. By committer date the
	// first commit on/after the day is 01-05, so the start is the 01-02
	// commit before it.
	sha, err := ResolveStart(context.Background(), repo, "2025-01-05", "HEAD", true)
	if err != nil {
		t.Fatalf("ResolveStart error: %v", err)
	}
	meta, err := ResolveCommit(context.Background(), repo, sha)
	if err != nil {
		t.Fatal(err)
	}
	if got := meta.CommitDate.UTC().Format("2006-01-02T15"); got != "2025-01-02T10" {
		t.Errorf("start should be the 01-02 commit, got %s", got)
	}
}

func TestListCommits_BucketsByLandingDate(t *testing.T) {
	repo := rebasedTipRepo(t)
	got, err := ListCommits(context.Background(), repo, "", "HEAD", true, Sampling{Bucket: "day", Every: 1})
	if err != nil {
		t.Fatalf("ListCommits error: %v", err)
	}
	// The rebased tip lands on 01-07 and is bucketed there, not on its
	// 2024-06-01 author date; every day has exactly one pick.
	want := []string{"2025-01-01T17", "2025-01-02T10", "2025-01-05T10", "2025-01-06T10", "2025-01-07T10"}
	if fmt.Sprint(days(got)) != fmt.Sprint(want) {
		t.Errorf("got %v, want %v", days(got), want)
	}
}

func TestListCommits_RejectsNonPositiveStride(t *testing.T) {
	repo := testutil.CreateFixtureRepo(t)
	if _, err := ListCommits(context.Background(), repo, "HEAD~1", "HEAD", true, Sampling{Bucket: "commit", Every: 0}); err == nil {
		t.Fatal("expected error for every: 0")
	}
}

func TestResolveStart_PassesRevisionsThroughAndHandlesOutOfRangeDates(t *testing.T) {
	repo := datedRepo(t)
	if got, err := ResolveStart(context.Background(), repo, "HEAD~3", "HEAD", true); err != nil || got != "HEAD~3" {
		t.Errorf("revision should pass through, got %q, %v", got, err)
	}
	if got, err := ResolveStart(context.Background(), repo, "1990-01-01", "HEAD", true); err != nil || got != "" {
		t.Errorf("a date before all commits should select the whole history (empty start), got %q, %v", got, err)
	}
	if _, err := ResolveStart(context.Background(), repo, "2999-01-01", "HEAD", true); err == nil {
		t.Error("expected error when no commit is on or after the date")
	}
}
