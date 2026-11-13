//go:build integration

// Integration tests for the runner's process handling. They launch /bin/sh and
// are therefore excluded from the default test run: `go test -tags integration
// ./internal/runner`. They never run stress-ng or a benchmark/burn-in.
package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// collect drains a job's events and returns them in order.
func collect(t *testing.T, job *Job) []Event {
	t.Helper()
	var events []Event
	deadline := time.After(10 * time.Second)
	for {
		select {
		case ev, ok := <-job.Events():
			if !ok {
				return events
			}
			events = append(events, ev)
		case <-deadline:
			t.Fatal("timed out waiting for the job's event stream to close")
		}
	}
}

// TestIntegrationOutputAndLog checks output streaming and log teeing.
func TestIntegrationOutputAndLog(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "logs", "run.log")
	job := Start(context.Background(), Spec{
		Kind:    KindBenchmark,
		Name:    "/bin/sh",
		Args:    []string{"-c", "printf 'first\\nsecond\\n'"},
		LogPath: logPath,
	})
	events := collect(t, job)

	if len(events) < 3 {
		t.Fatalf("events = %d, want started, output, finished", len(events))
	}
	if events[0].Type != EventStarted || events[0].LogPath != logPath {
		t.Errorf("first event = %+v, want EventStarted with the log path", events[0])
	}
	if last := events[len(events)-1]; last.Type != EventFinished || last.ExitCode != 0 || last.Err != nil {
		t.Errorf("last event = %+v, want a successful finish", last)
	}
	var lines []string
	for _, ev := range events {
		if ev.Type == EventOutput {
			lines = append(lines, ev.Line)
		}
	}
	if len(lines) != 2 || lines[0] != "first" || lines[1] != "second" {
		t.Errorf("output lines = %q", lines)
	}
	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if string(logged) != "first\nsecond\n" {
		t.Errorf("log content = %q", logged)
	}
	if err := job.Wait(); err != nil {
		t.Errorf("Wait() = %v, want nil", err)
	}
}

// TestIntegrationExitCode checks a non-zero exit is reported with its code.
func TestIntegrationExitCode(t *testing.T) {
	job := Start(context.Background(), Spec{
		Kind:    KindBenchmark,
		Name:    "/bin/sh",
		Args:    []string{"-c", "exit 3"},
		LogPath: filepath.Join(t.TempDir(), "run.log"),
	})
	events := collect(t, job)
	last := events[len(events)-1]
	if last.Type != EventFinished || last.ExitCode != 3 || last.Err == nil {
		t.Errorf("finished event = %+v, want exit code 3 and an error", last)
	}
}

// TestIntegrationMissingBinary checks a missing binary fails the job instead of
// hanging or panicking.
func TestIntegrationMissingBinary(t *testing.T) {
	job := Start(context.Background(), Spec{
		Kind:    KindBenchmark,
		Name:    "mem-station-no-such-binary",
		LogPath: filepath.Join(t.TempDir(), "run.log"),
	})
	events := collect(t, job)
	if len(events) != 2 {
		t.Fatalf("events = %d, want started and finished", len(events))
	}
	if last := events[1]; last.Type != EventFinished || last.Err == nil {
		t.Errorf("finished event = %+v, want a lookup error", last)
	}
}

// TestIntegrationCancelKillsProcessGroup checks Cancel terminates the process
// group and reports ErrCancelled exactly once.
func TestIntegrationCancelKillsProcessGroup(t *testing.T) {
	job := Start(context.Background(), Spec{
		Kind:    KindBurnIn,
		Name:    "/bin/sh",
		Args:    []string{"-c", "sleep 30"},
		LogPath: filepath.Join(t.TempDir(), "run.log"),
	})

	started := make(chan struct{})
	done := make(chan []Event, 1)
	go func() {
		var events []Event
		for ev := range job.Events() {
			if ev.Type == EventStarted {
				close(started)
			}
			events = append(events, ev)
		}
		done <- events
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the job never started")
	}

	finished := time.Now()
	job.Cancel()
	job.Cancel() // idempotent

	select {
	case events := <-done:
		if elapsed := time.Since(finished); elapsed > 5*time.Second {
			t.Errorf("cancellation took %s", elapsed)
		}
		if len(events) == 0 {
			t.Fatal("no events recorded")
		}
		last := events[len(events)-1]
		if last.Type != EventFinished || !errors.Is(last.Err, ErrCancelled) {
			t.Errorf("finished event = %+v, want ErrCancelled", last)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the cancelled job did not finish")
	}

	if err := job.Wait(); !errors.Is(err, ErrCancelled) {
		t.Errorf("Wait() = %v, want ErrCancelled", err)
	}
}
