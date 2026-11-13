// Package runner executes external jobs, streams their merged output, and
// exposes a cancellable event stream to the TUI.
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
	"sync"
	"time"
)

// Kind identifies the logical job type so the app can route events.
type Kind string

const (
	KindBenchmark Kind = "benchmark"
	KindBurnIn    Kind = "burnin"
)

// EventType is the phase of a job event.
type EventType int

const (
	EventStarted EventType = iota
	EventOutput
	EventFinished
)

// Event is a single lifecycle or output event emitted by a running job.
type Event struct {
	Kind     Kind
	Type     EventType
	Line     string
	At       time.Time
	ExitCode int
	Err      error
	LogPath  string
}

// Spec describes a job to run.
type Spec struct {
	Kind     Kind
	Name     string
	Args     []string
	Dir      string
	Env      []string
	LogPath  string
	Duration time.Duration
}

// ErrCancelled is returned for a job that was cancelled via Cancel.
var ErrCancelled = errors.New("job cancelled")

// Job is a running (or finished) external job. Its fields are unexported on
// purpose; consumers interact through Events, Cancel, and Wait.
type Job struct {
	ctx    context.Context
	cancel context.CancelFunc
	cmd    *exec.Cmd

	events  chan Event
	done    chan struct{}
	started chan struct{}
	exited  chan struct{}

	exitedOnce sync.Once
	cancelOnce sync.Once

	mu        sync.Mutex
	cancelled bool
	resultErr error
}

// Start launches spec in a goroutine and returns immediately. The caller must
// consume Events to avoid blocking the job.
func Start(ctx context.Context, spec Spec) *Job {
	ctx, cancel := context.WithCancel(ctx)
	j := &Job{
		ctx:     ctx,
		cancel:  cancel,
		events:  make(chan Event),
		done:    make(chan struct{}),
		started: make(chan struct{}),
		exited:  make(chan struct{}),
	}

	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = append(os.Environ(), spec.Env...)
	setupProcessGroup(cmd)
	j.cmd = cmd

	go j.run(spec)
	return j
}

// Events returns the receive side of the job's event stream.
func (j *Job) Events() <-chan Event { return j.events }

// Cancel terminates the job's process group. It is idempotent.
func (j *Job) Cancel() {
	j.cancelOnce.Do(func() {
		j.mu.Lock()
		j.cancelled = true
		j.mu.Unlock()

		// Signal the process group first so children are reaped, then cancel
		// the CommandContext as a fallback.
		terminateProcess(j)
		j.cancel()
	})
}

// Wait blocks until the job finishes and returns its final error.
func (j *Job) Wait() error {
	<-j.done
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.resultErr
}

func (j *Job) run(spec Spec) {
	defer close(j.done)
	defer j.closeExited()
	defer close(j.events)

	j.emit(Event{Kind: spec.Kind, Type: EventStarted, At: time.Now(), LogPath: spec.LogPath})

	logFile, err := createLog(spec.LogPath)
	if err != nil {
		j.finish(spec, -1, err)
		return
	}
	defer logFile.Close()

	// Cancelled before the process started.
	if j.ctx.Err() != nil {
		j.finish(spec, -1, ErrCancelled)
		return
	}

	pr, pw := io.Pipe()
	defer pw.Close()
	sw := &syncWriter{w: io.MultiWriter(logFile, pw)}
	j.cmd.Stdout = sw
	j.cmd.Stderr = sw

	if err := j.cmd.Start(); err != nil {
		j.finish(spec, -1, err)
		return
	}
	close(j.started)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(pr)
		scanner.Buffer(make([]byte, 64*1024), 1<<20)
		for scanner.Scan() {
			j.emit(Event{Kind: spec.Kind, Type: EventOutput, Line: scanner.Text(), At: time.Now()})
		}
	}()

	waitErr := j.cmd.Wait()
	pw.Close()
	wg.Wait()

	// Signal cancellation waiters as soon as the process has been reaped,
	// without waiting for any buffered Finished event to be consumed.
	j.closeExited()

	exitCode := 0
	if waitErr != nil {
		if j.isCancelled() {
			waitErr = ErrCancelled
			exitCode = -1
		} else {
			var ee *exec.ExitError
			if errors.As(waitErr, &ee) {
				exitCode = ee.ExitCode()
				waitErr = fmt.Errorf("%s exited with status %d (log: %s)", spec.Name, exitCode, spec.LogPath)
			}
		}
	}
	j.finish(spec, exitCode, waitErr)
}

func (j *Job) emit(ev Event) {
	j.events <- ev
}

func (j *Job) finish(spec Spec, exitCode int, err error) {
	j.mu.Lock()
	j.resultErr = err
	j.mu.Unlock()

	j.emit(Event{
		Kind:     spec.Kind,
		Type:     EventFinished,
		At:       time.Now(),
		ExitCode: exitCode,
		Err:      err,
		LogPath:  spec.LogPath,
	})
}

func (j *Job) isCancelled() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.cancelled
}

func (j *Job) closeExited() {
	j.exitedOnce.Do(func() { close(j.exited) })
}

func createLog(path string) (*os.File, error) {
	if path == "" {
		return nil, fmt.Errorf("empty log path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
}

type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}
