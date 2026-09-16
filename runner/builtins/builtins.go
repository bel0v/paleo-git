// Package builtins is the registry of runners addressable by name from a
// metric's `runner.builtin` field. Add new built-in runners here.
package builtins

import (
	"sort"

	"github.com/bel0v/paleo-git/runner"
	"github.com/bel0v/paleo-git/runner/gitfilecount"
	"github.com/bel0v/paleo-git/runner/gitgrep"
)

var registry = map[string]func() runner.Runner{
	"git_grep_count": func() runner.Runner { return gitgrep.New() },
	"git_file_count": func() runner.Runner { return gitfilecount.New() },
}

// New returns a fresh instance of the named builtin runner.
func New(name string) (runner.Runner, bool) {
	factory, ok := registry[name]
	if !ok {
		return nil, false
	}
	return factory(), true
}

// Names lists the registered builtin runner names, sorted.
func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
