package runner

// State is the TUI-visible lifecycle of a job, shared by the benchmark and
// burn-in tabs. The runner itself only emits events; tabs derive this state
// from EventStarted/EventFinished.
type State int

const (
	StateIdle State = iota
	StateRunning
	StateDone
	StateFailed
	StateCancelled
)

// String returns a short human-readable name for a state.
func (s State) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateRunning:
		return "running"
	case StateDone:
		return "done"
	case StateFailed:
		return "failed"
	case StateCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}
