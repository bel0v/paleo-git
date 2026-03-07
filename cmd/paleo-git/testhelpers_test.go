package main

import (
	"bytes"
	"os"
	"testing"
)

// captureOutput captures stdout during fn execution.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

// runCLI executes the CLI with the given args in-process.
func runCLI(t *testing.T, args ...string) {
	t.Helper()
	cmd := buildRootCmd()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("CLI error: %v", err)
	}
}
