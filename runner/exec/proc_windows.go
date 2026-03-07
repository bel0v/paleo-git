//go:build windows

package exec

import "os/exec"

func setProcAttr(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		return cmd.Process.Kill()
	}
}
