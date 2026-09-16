package main

import (
	"bytes"
	"testing"
)

// runCLI runs the CLI with the given args and returns stdout, stderr and
// the execution error, without touching the process streams.
func runCLI(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errBuf bytes.Buffer
	cmd := buildRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errBuf.String(), err
}

// captureOutput runs the CLI and returns stdout, failing the test on error.
func captureOutput(t *testing.T, args ...string) string {
	t.Helper()
	stdout, stderr, err := runCLI(t, args...)
	if err != nil {
		t.Fatalf("CLI error: %v\nstderr: %s", err, stderr)
	}
	return stdout
}
