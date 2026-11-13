package cli

import (
	"context"
	"errors"
	"strings"
	"time"

	"mem-station/internal/runner"
)

// maxRetainedLines bounds the output a headless run keeps in memory. The full
// stream always lands in the runner's log file, so parsing only needs the tail.
const maxRetainedLines = 2000

// jobResult is the collected outcome of one headless run.
type jobResult struct {
	Output     string
	ExitCode   int
	Err        error
	LogPath    string
	StartedAt  time.Time
	FinishedAt time.Time
}

// cancelled reports whether the run was stopped by a signal.
func (r jobResult) cancelled() bool { return errors.Is(r.Err, runner.ErrCancelled) }

// errText is the diagnostic string stored with a run record.
func (r jobResult) errText() string {
	switch {
	case r.Err == nil:
		return ""
	case r.cancelled():
		return "cancelled"
	default:
		return r.Err.Error()
	}
}

// duration returns the measured wall time of the run.
func (r jobResult) duration() time.Duration {
	if r.StartedAt.IsZero() || r.FinishedAt.IsZero() {
		return 0
	}
	return r.FinishedAt.Sub(r.StartedAt)
}

// runJob starts spec and drains its event stream until EventFinished, cancelling
// the process group when ctx is cancelled. The runner owns process cleanup, so
// no child is left behind when the command is interrupted.
func runJob(ctx context.Context, start runner.StartFunc, spec runner.Spec) jobResult {
	job := start(ctx, spec)
	events := job.Events()
	cancel := ctx.Done()

	res := jobResult{LogPath: spec.LogPath}
	out := lineBuffer{max: maxRetainedLines}
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				res.Output = out.String()
				return res
			}
			switch ev.Type {
			case runner.EventStarted:
				res.StartedAt = ev.At
				if ev.LogPath != "" {
					res.LogPath = ev.LogPath
				}
			case runner.EventOutput:
				out.Add(ev.Line)
			case runner.EventFinished:
				// EventFinished is terminal: the runner closes the stream right
				// after it, and stopping here also covers a stream that does not.
				res.FinishedAt = ev.At
				res.ExitCode = ev.ExitCode
				res.Err = ev.Err
				if ev.LogPath != "" {
					res.LogPath = ev.LogPath
				}
				res.Output = out.String()
				return res
			}
		case <-cancel:
			// Cancel once, then keep draining: the runner emits its own
			// EventFinished, and stopping here would lose the exit status.
			job.Cancel()
			cancel = nil
		}
	}
}

// lineBuffer keeps the last max lines of a stream.
type lineBuffer struct {
	lines []string
	max   int
}

// Add appends one line, dropping the oldest when the buffer is full.
func (b *lineBuffer) Add(line string) {
	if b.max <= 0 {
		return
	}
	if len(b.lines) == b.max {
		copy(b.lines, b.lines[1:])
		b.lines[len(b.lines)-1] = line
		return
	}
	b.lines = append(b.lines, line)
}

// String returns the retained lines.
func (b *lineBuffer) String() string { return strings.Join(b.lines, "\n") }
