package main

import (
	"bytes"
	"testing"
)

// captureOutput runs the CLI with the given args and returns stdout.
// Uses cobra's SetOut to capture output without touching os.Stdout.
func captureOutput(t *testing.T, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	cmd := buildRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("CLI error: %v", err)
	}
	return buf.String()
}
