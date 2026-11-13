//go:build !unix

package runner

import "os/exec"

func setupProcessGroup(cmd *exec.Cmd) {}

// terminateProcess is the non-unix fallback: kill the single process.
func terminateProcess(j *Job) {
	select {
	case <-j.started:
	case <-j.exited:
		return
	}
	if j.cmd != nil && j.cmd.Process != nil {
		_ = j.cmd.Process.Kill()
	}
}
