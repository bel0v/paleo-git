package config

import "testing"

func TestMetricHash_StableAcrossRepeatedCalls(t *testing.T) {
	m := Metric{
		ID:        "legacy-imports",
		Traversal: "default",
		Paths:     Paths{Include: []string{"src/**/*.ts"}},
		Runner: RunnerRef{
			Builtin: "git_grep_count",
			Config:  map[string]any{"pattern": "from '@legacy/"},
		},
	}
	h1 := MetricHash(m)
	h2 := MetricHash(m)
	if h1 != h2 {
		t.Errorf("hash not stable: %s != %s", h1, h2)
	}
	if len(h1) != 32 {
		t.Errorf("expected 32 hex chars, got %d: %s", len(h1), h1)
	}
}

func TestMetricHash_ChangesWhenRunnerConfigChanges(t *testing.T) {
	m1 := Metric{
		ID:        "legacy-imports",
		Traversal: "default",
		Paths:     Paths{Include: []string{"src/**/*.ts"}},
		Runner: RunnerRef{
			Builtin: "git_grep_count",
			Config:  map[string]any{"pattern": "from '@legacy/"},
		},
	}
	m2 := Metric{
		ID:        "legacy-imports",
		Traversal: "default",
		Paths:     Paths{Include: []string{"src/**/*.ts"}},
		Runner: RunnerRef{
			Builtin: "git_grep_count",
			Config:  map[string]any{"pattern": "from '@new/"},
		},
	}
	if MetricHash(m1) == MetricHash(m2) {
		t.Error("hash should differ when runner config changes")
	}
}

func TestMetricHash_ChangesWhenPathsChange(t *testing.T) {
	m1 := Metric{
		ID:        "test",
		Traversal: "default",
		Paths:     Paths{Include: []string{"src/**/*.ts"}},
		Runner:    RunnerRef{Builtin: "git_grep_count"},
	}
	m2 := Metric{
		ID:        "test",
		Traversal: "default",
		Paths:     Paths{Include: []string{"lib/**/*.ts"}},
		Runner:    RunnerRef{Builtin: "git_grep_count"},
	}
	if MetricHash(m1) == MetricHash(m2) {
		t.Error("hash should differ when paths change")
	}
}

func TestMetricHash_ChangesWhenRunnerTypeChanges(t *testing.T) {
	m1 := Metric{
		ID:        "test",
		Traversal: "default",
		Paths:     Paths{Include: []string{"src/**"}},
		Runner:    RunnerRef{Builtin: "git_grep_count"},
	}
	m2 := Metric{
		ID:        "test",
		Traversal: "default",
		Paths:     Paths{Include: []string{"src/**"}},
		Runner:    RunnerRef{Exec: []string{"node", "check.js"}},
	}
	if MetricHash(m1) == MetricHash(m2) {
		t.Error("hash should differ when runner type changes")
	}
}
