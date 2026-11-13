// Package benchmark implements the Benchmark tab: it configures and runs the
// stress-ng memrate benchmark through internal/runner, parses the output, and
// keeps the recent run history in internal/store.
package benchmark

import (
	"context"
	"time"

	"mem-station/internal/bench"
	"mem-station/internal/config"
	"mem-station/internal/runner"
	"mem-station/internal/ui"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// configRow identifies one row of the run configuration.
type configRow int

const (
	rowDuration configRow = iota
	rowBuffer
	rowWorkers
	rowCount
)

// durationIndexDefault selects the 30s option.
const durationIndexDefault = 1

// historyLimit is how many recent runs the tab loads and keeps.
const historyLimit = 10

// defaultStart launches the real stress-ng process; tests replace it through
// Model.start.
func defaultStart(ctx context.Context, spec runner.Spec) runner.Handle {
	return runner.Start(ctx, spec)
}

// Model is the Benchmark tab.
type Model struct {
	state    runner.State
	start    runner.StartFunc
	job      runner.Handle
	events   <-chan runner.Event
	snapshot config.Snapshot
	// now is the clock used for elapsed time; tests pin it.
	now func() time.Time

	durationIndex int
	custom        time.Duration
	bufferIndex   int
	workers       int
	row           configRow

	editing  bool
	input    textinput.Model
	editPrev int

	output   string
	follow   bool
	results  *bench.BenchResults
	parseMsg string
	logPath  string
	exitCode int
	errText  string

	startedAt  time.Time
	finishedAt time.Time
	// runOptions is the configuration the active/last run used, so a finished
	// record is never attributed to settings the user changed mid-run.
	runOptions Options

	runs       []runRecord
	historyErr string
	lastID     string

	vp   viewport.Model
	keys KeyMap
}

// New builds an idle benchmark tab.
func New(snapshot config.Snapshot) *Model {
	in := textinput.New()
	in.Prompt = ""
	in.CharLimit = 8
	in.SetWidth(8)

	return &Model{
		state:         runner.StateIdle,
		start:         defaultStart,
		snapshot:      snapshot,
		now:           time.Now,
		durationIndex: durationIndexDefault,
		custom:        2 * time.Minute,
		bufferIndex:   defaultBufferIndex,
		workers:       1,
		input:         in,
		follow:        true,
		vp:            viewport.New(viewport.WithWidth(60), viewport.WithHeight(8)),
		keys:          DefaultKeyMap(),
	}
}

// Init loads the recent run history; runs are user initiated.
func (m *Model) Init() tea.Cmd {
	m.loadHistory()
	return nil
}

// SetSnapshot updates the applied configuration the next run is attributed to.
func (m *Model) SetSnapshot(s config.Snapshot) { m.snapshot = s }

// Editing reports whether the custom duration input owns letter keys.
func (m *Model) Editing() bool { return m.editing }

// ModalOpen reports whether a modal is open (never here).
func (m *Model) ModalOpen() bool { return false }

// Running reports whether a benchmark job is active.
func (m *Model) Running() bool { return m.state == runner.StateRunning }

// Progress reports the running job's completion in 0..1, or -1 when idle.
func (m *Model) Progress() float64 {
	if !m.Running() {
		return -1
	}
	return m.progress() / 100
}

// Dirty reports whether unattributed changes exist (never here).
func (m *Model) Dirty() bool { return false }

// Blur ends any in-progress edit (run tabs have no persistent editors).
func (m *Model) Blur() {
	if m.editing {
		m.cancelEdit()
	}
}

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
	switch {
	case m.editing:
		return []ui.KeyHint{{Key: "enter", Action: "commit duration"}, {Key: "esc", Action: "cancel edit"}}
	case m.Running():
		return hints(m.keys.Cancel, m.keys.Follow, m.keys.ScrollUp, m.keys.ScrollDn)
	default:
		return hints(m.keys.Run, m.keys.Up, m.keys.Prev, m.keys.Export, m.keys.Follow)
	}
}

// options returns the configuration the next run would use.
func (m *Model) options() Options {
	return Options{
		Duration: durationFor(m.durationIndex, m.custom),
		Buffer:   buffers[m.bufferIndex],
		Workers:  m.workers,
	}
}
