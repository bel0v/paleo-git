package builtins

import "testing"

func TestNew_KnownAndUnknown(t *testing.T) {
	for _, name := range Names() {
		r, ok := New(name)
		if !ok || r == nil {
			t.Errorf("New(%q) should return a runner", name)
		}
	}
	if _, ok := New("nope"); ok {
		t.Error("New(\"nope\") should not resolve")
	}
}

func TestNames_IsSortedAndComplete(t *testing.T) {
	names := Names()
	want := []string{"git_file_count", "git_grep_count"}
	if len(names) != len(want) {
		t.Fatalf("expected %v, got %v", want, names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, names)
		}
	}
}
