//go:build unix

package runner

import (
	"os/exec"
	"syscall"
	"time"
)

func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateProcess signals the process group with SIGTERM, waits up to 3s for
// the process to exit, then escalates to SIGKILL.
func terminateProcess(j *Job) {
	select {
	case <-j.started:
	case <-j.exited:
		return
	case <-time.After(3 * time.Second):
		return
	}

	if j.cmd == nil || j.cmd.Process == nil {
		return
	}
	pid := j.cmd.Process.Pid

	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		_ = j.cmd.Process.Signal(syscall.SIGTERM)
	}

	select {
	case <-j.exited:
		return
	case <-time.After(3 * time.Second):
	}

	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		_ = j.cmd.Process.Kill()
	}
}
