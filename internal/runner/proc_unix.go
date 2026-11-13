//go:build unix

package runner

import (
	"os/exec"
	"syscall"
)

// setProcessGroup puts the child in its own process group so the whole group
// can be signalled on cancel (kills stress-ng and any descendants).
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func signalTerm(cmd *exec.Cmd) {
	signalGroup(cmd, syscall.SIGTERM)
}

func signalKill(cmd *exec.Cmd) {
	signalGroup(cmd, syscall.SIGKILL)
}

func signalGroup(cmd *exec.Cmd, sig syscall.Signal) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	// Negative pid targets the entire process group.
	_ = syscall.Kill(-cmd.Process.Pid, sig)
}
