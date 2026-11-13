//go:build !unix

package runner

import (
	"os"
	"os/exec"
)

func setProcessGroup(cmd *exec.Cmd) {}

func signalTerm(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Signal(os.Interrupt)
	}
}

func signalKill(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
