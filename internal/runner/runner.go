// Package runner executes external jobs (benchmarks, burn-in runs) with
// streamed output, cancellation via process groups, and tee'd log files.
//
// Start never blocks: it returns a *Job whose Events channel delivers exactly
// one EventStarted, zero or more EventOutput, and exactly one EventFinished
// (after which the channel is closed). Callers must consume events until
// EventFinished; Wait blocks until the job has reached its terminal state.
package runner

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Kind identifies the class of job.
type Kind string

const (
	KindBenchmark Kind = "benchmark"
	KindBurnIn    Kind = "burnin"
)

// EventType enumerates the lifecycle events emitted by a Job.
type EventType int

const (
	EventStarted EventType = iota
	EventOutput
	EventFinished
)

// Event is a single lifecycle event from a Job.
type Event struct {
	Kind     Kind
	Type     EventType
	Line     string // populated for EventOutput
	At       time.Time
	ExitCode int   // populated for EventFinished
	Err      error // populated for EventFinished (nil on success)
	LogPath  string
}

// Spec describes a job to run.
type Spec struct {
	Kind     Kind
	Name     string // fallback binary name when Args is empty
	Args     []string
	Dir      string
	Env      []string
	LogPath  string
	Duration time.Duration
}

// ErrCancelled is returned (or carried in EventFinished.Err) when a job is
// cancelled via Cancel or a cancelled parent context.
var ErrCancelled = errors.New("job cancelled")

// Job is a running (or finished) job. Its fields are unexported; interact
// through Events, Cancel, and Wait.
type Job struct {
	events  chan Event
	done    chan struct{}
	waitErr error
	cmd     *exec.Cmd
	cancel  context.CancelFunc
}

// Start launches the job described by spec. The returned Job is non-nil and
// Start returns immediately.
func Start(ctx context.Context, spec Spec) *Job {
	j := &Job{
		events: make(chan Event, 64),
		done:   make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(ctx)
	j.cancel = cancel

	j.cmd = buildCommand(spec)
	setProcessGroup(j.cmd)

	go j.run(ctx, spec)
	return j
}

func buildCommand(spec Spec) *exec.Cmd {
	if len(spec.Args) > 0 {
		cmd := exec.Command(spec.Args[0], spec.Args[1:]...)
		cmd.Dir = spec.Dir
		cmd.Env = spec.Env
		return cmd
	}
	cmd := exec.Command(spec.Name)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	return cmd
}

// Events returns the channel of lifecycle events. It is closed after
// EventFinished is delivered.
func (j *Job) Events() <-chan Event { return j.events }

// Cancel requests cancellation. It is idempotent and safe to call before,
// during, or after the job finishes. The process group receives SIGTERM and,
// if it does not exit within 3 seconds, SIGKILL.
func (j *Job) Cancel() {
	j.cancel()
}

// Wait blocks until the job reaches its terminal state and returns the final
// error (nil on success, ErrCancelled on cancellation).
func (j *Job) Wait() error {
	<-j.done
	return j.waitErr
}

func (j *Job) run(ctx context.Context, spec Spec) {
	defer close(j.done)
	defer close(j.events)

	logFile, err := openLog(spec.LogPath)
	if err != nil {
		j.waitErr = err
		j.events <- Event{Kind: spec.Kind, Type: EventFinished, At: time.Now(), Err: err, LogPath: spec.LogPath}
		return
	}
	defer logFile.Close()

	if ctx.Err() != nil {
		j.waitErr = ErrCancelled
		j.events <- Event{Kind: spec.Kind, Type: EventFinished, At: time.Now(), Err: ErrCancelled}
		return
	}

	// Merge stdout+stderr through a single pipe and tee every line to the log
	// file while forwarding it as an EventOutput.
	pr, pw := io.Pipe()
	j.cmd.Stdout = pw
	j.cmd.Stderr = pw

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		defer pr.Close()
		scanner := bufio.NewScanner(pr)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			_, _ = logFile.WriteString(line + "\n")
			select {
			case j.events <- Event{Kind: spec.Kind, Type: EventOutput, Line: line, At: time.Now()}:
			case <-ctx.Done():
				return
			}
		}
	}()

	j.events <- Event{Kind: spec.Kind, Type: EventStarted, At: time.Now(), LogPath: spec.LogPath}

	if err := j.cmd.Start(); err != nil {
		_ = pw.Close()
		<-readDone
		j.waitErr = mapStartError(err)
		j.events <- Event{Kind: spec.Kind, Type: EventFinished, At: time.Now(), ExitCode: -1, Err: j.waitErr, LogPath: spec.LogPath}
		return
	}

	procDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			j.terminate()
		case <-procDone:
		}
	}()

	waitErr := j.cmd.Wait()
	close(procDone)
	_ = pw.Close()
	<-readDone

	exitCode := 0
	var finErr error
	switch {
	case ctx.Err() != nil:
		finErr = ErrCancelled
		exitCode = exitCodeOf(waitErr)
	case waitErr != nil:
		finErr = mapWaitError(waitErr)
		exitCode = exitCodeOf(waitErr)
	}

	j.waitErr = finErr
	j.events <- Event{Kind: spec.Kind, Type: EventFinished, At: time.Now(), ExitCode: exitCode, Err: finErr, LogPath: spec.LogPath}
}

// terminate signals the process group with SIGTERM and escalates to SIGKILL
// after 3 seconds if it is still running. It is only called from the
// context-cancellation watcher, so it runs at most once per job.
func (j *Job) terminate() {
	signalTerm(j.cmd)
	go func() {
		time.Sleep(3 * time.Second)
		signalKill(j.cmd)
	}()
}

func openLog(path string) (*os.File, error) {
	if path == "" {
		return nil, errors.New("runner: empty log path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
}

func exitCodeOf(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

func mapStartError(err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("%w: command not found", exec.ErrNotFound)
	}
	return err
}

func mapWaitError(err error) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return fmt.Errorf("process exited with status %d", ee.ExitCode())
	}
	return err
}
