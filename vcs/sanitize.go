package vcs

import (
	"fmt"
	"strings"
)

// validateRef checks that a git ref doesn't look like a flag.
func validateRef(ref, fieldName string) error {
	if ref == "" {
		return fmt.Errorf("%s: must not be empty", fieldName)
	}
	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("%s: must not start with '-' (got %q)", fieldName, ref)
	}
	return nil
}

// validatePattern checks that a grep pattern doesn't look like a flag.
func validatePattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("pattern: must not be empty")
	}
	if strings.HasPrefix(pattern, "-") {
		return fmt.Errorf("pattern: must not start with '-' (got %q)", pattern)
	}
	return nil
}

// validateRepoPath checks that a repo path doesn't look like a flag.
func validateRepoPath(path string) error {
	if path == "" {
		return fmt.Errorf("repo path: must not be empty")
	}
	if strings.HasPrefix(path, "-") {
		return fmt.Errorf("repo path: must not start with '-' (got %q)", path)
	}
	return nil
}
