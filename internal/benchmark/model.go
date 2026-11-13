// Package benchmark implements the Benchmark tab: it configures and runs the
// stress-ng memrate benchmark through internal/runner and renders the parsed
// results.
package benchmark

import (
	"time"

	"mem-station/internal/bench"
	"mem-station/internal/config"
	"mem-station/internal/runner"
	"mem-station/internal/ui"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// defaultDuration is the Phase 1 benchmark duration. Phase 3 makes it
// configurable from the tab.
const defaultDuration = 30 * time.Second

// Model is the Benchmark tab.
type Model struct {
	state    runner.State
	job      *runner.Job
	events   <-chan runner.Event
	duration time.Duration
	snapshot config.Snapshot

	output     string
	results    *bench.BenchResults
	logPath    string
	exitCode   int
	errText    string
	startedAt  time.Time
	finishedAt time.Time

	vp   viewport.Model
	keys KeyMap
}

// New builds an idle benchmark tab.
func New(snapshot config.Snapshot) *Model {
	return &Model{
		state:    runner.StateIdle,
		duration: defaultDuration,
		snapshot: snapshot,
		vp:       viewport.New(60, 8),
		keys:     DefaultKeyMap(),
	}
}

// Init implements the tab lifecycle; runs are user initiated.
func (m *Model) Init() tea.Cmd { return nil }

// SetSnapshot updates the applied configuration the next run is attributed to.
func (m *Model) SetSnapshot(s config.Snapshot) { m.snapshot = s }

// Editing reports whether a text input owns letter keys (never here).
func (m *Model) Editing() bool { return false }

// ModalOpen reports whether a modal is open (never here).
func (m *Model) ModalOpen() bool { return false }

// Running reports whether a benchmark job is active.
func (m *Model) Running() bool { return m.state == runner.StateRunning }

// Dirty reports whether unattributed changes exist (never here).
func (m *Model) Dirty() bool { return false }

// Blur ends any in-progress edit (run tabs have no editors).
func (m *Model) Blur() {}

// Stop cancels a running job, if any.
func (m *Model) Stop() {
	if m.job != nil && m.state == runner.StateRunning {
		m.job.Cancel()
	}
}

// Title returns the tab title.
func (m *Model) Title() string { return "Benchmark" }

// State returns the current job state.
func (m *Model) State() runner.State { return m.state }

// Wants reports whether this tab owns runner events of the given kind.
func (m *Model) Wants(kind runner.Kind) bool { return kind == runner.KindBenchmark }

// Keys returns the footer hints for the current state.
func (m *Model) Keys() []ui.KeyHint {
	if m.Running() {
		return hints(m.keys.Cancel, m.keys.ScrollUp, m.keys.ScrollDn)
	}
	return hints(m.keys.Run, m.keys.ScrollUp, m.keys.ScrollDn)
}
