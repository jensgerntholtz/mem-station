// Package burnin implements the Burn-in tab: it runs a stress-ng job file
// through internal/runner and summarizes the parsed outcome. Parsing lives in
// the leaf package burnin/parse.
package burnin

import (
	"time"

	"mem-station/internal/burnin/parse"
	"mem-station/internal/config"
	"mem-station/internal/runner"
	"mem-station/internal/ui"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// defaultDuration is the Phase 1 burn-in duration. Phase 3 makes it
// configurable (5m/10m/30m/1h/custom).
const defaultDuration = 30 * time.Minute

// Model is the Burn-in tab.
type Model struct {
	state        runner.State
	job          *runner.Job
	events       <-chan runner.Event
	jobPath      string // user-specified preset path ("" = discovery)
	duration     time.Duration
	snapshot     config.Snapshot
	presetSource string

	output     string
	results    *parse.Results
	logPath    string
	exitCode   int
	errText    string
	startedAt  time.Time
	finishedAt time.Time

	vp   viewport.Model
	keys KeyMap
}

// New builds an idle burn-in tab. jobPath may be empty to use discovery.
func New(jobPath string, snapshot config.Snapshot) *Model {
	return &Model{
		state:    runner.StateIdle,
		jobPath:  jobPath,
		duration: defaultDuration,
		snapshot: snapshot,
		vp:       viewport.New(60, 10),
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

// Running reports whether a burn-in job is active.
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
func (m *Model) Title() string { return "Burn-in" }

// State returns the current job state.
func (m *Model) State() runner.State { return m.state }

// Wants reports whether this tab owns runner events of the given kind.
func (m *Model) Wants(kind runner.Kind) bool { return kind == runner.KindBurnIn }

// Keys returns the footer hints for the current state.
func (m *Model) Keys() []ui.KeyHint {
	if m.Running() {
		return hints(m.keys.Cancel, m.keys.ScrollUp, m.keys.ScrollDn)
	}
	return hints(m.keys.Run, m.keys.ScrollUp, m.keys.ScrollDn)
}
