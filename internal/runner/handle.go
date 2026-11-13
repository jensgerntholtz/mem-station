package runner

import "context"

// Handle is the subset of Job the TUI tabs use: consume the event stream and
// cancel the process group. *Job satisfies it; the run tabs take a factory so
// tests can inject a fake stream without launching a process.
type Handle interface {
	Events() <-chan Event
	Cancel()
}

// StartFunc launches a job and returns its handle. Tests replace it with a fake.
type StartFunc func(ctx context.Context, spec Spec) Handle
