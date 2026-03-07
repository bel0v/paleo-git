package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/bel0v/paleo-git/runner"
)

type ExecRunner struct {
	argv []string
}

func New(argv []string) *ExecRunner {
	return &ExecRunner{argv: argv}
}

type output struct {
	Value int      `json:"value"`
	Files []string `json:"files,omitempty"`
}

func (e *ExecRunner) Run(ctx context.Context, req runner.RunRequest) (*runner.RunResult, error) {
	cmd := exec.CommandContext(ctx, e.argv[0], e.argv[1:]...)
	setProcAttr(cmd)
	cmd.WaitDelay = 500 * time.Millisecond

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	env := cmd.Environ()
	env = append(env,
		"PALEO_COMMIT="+req.Commit,
		"PALEO_REPO_PATH="+req.RepoPath,
	)

	if req.Config != nil {
		configJSON, err := json.Marshal(req.Config)
		if err != nil {
			return nil, fmt.Errorf("exec runner: failed to marshal config: %w", err)
		}
		env = append(env, "PALEO_RUNNER_CONFIG="+string(configJSON))
	}

	if len(req.PathsInclude) > 0 {
		includeJSON, _ := json.Marshal(req.PathsInclude)
		env = append(env, "PALEO_PATHS_INCLUDE="+string(includeJSON))
	}
	if len(req.PathsExclude) > 0 {
		excludeJSON, _ := json.Marshal(req.PathsExclude)
		env = append(env, "PALEO_PATHS_EXCLUDE="+string(excludeJSON))
	}

	cmd.Env = env

	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("exec runner: timeout: %w", ctx.Err())
	}
	if err != nil {
		return nil, fmt.Errorf("exec runner: command failed (exit %v): %s", err, stderr.String())
	}

	var out output
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("exec runner: invalid JSON output: %w (stdout: %s)", err, stdout.String())
	}

	return &runner.RunResult{Value: out.Value, Files: out.Files}, nil
}
