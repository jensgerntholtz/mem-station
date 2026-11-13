// Package burnin implements the Burn-in tab: it materializes a stress-ng job
// file with the chosen timeout into the state dir, runs it through
// internal/runner, and summarizes the parsed outcome. Parsing lives in the leaf
// package burnin/parse.
package burnin

import (
	"context"
	"time"

	"mem-station/internal/burnin/parse"
	"mem-station/internal/config"
	"mem-station/internal/runner"
	"mem-station/internal/ui"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// configRow identifies one row of the run configuration.
type configRow int

const (
	rowPreset configRow = iota
	rowDuration
	rowStopOnError
	rowCount
)

// historyLimit is how many recent runs the tab loads and keeps.
const historyLimit = 10

// defaultStart launches the real stress-ng process; tests replace it through
// Model.start.
func defaultStart(ctx context.Context, spec runner.Spec) runner.Handle {
	return runner.Start(ctx, spec)
}

// Model is the Burn-in tab.
type Model struct {
	state    runner.State
	start    runner.StartFunc
	job      runner.Handle
	events   <-chan runner.Event
	snapshot config.Snapshot
	// now is the clock used for elapsed time; tests pin it.
	now func() time.Time

	presets       []Preset
	presetWarn    string
	presetIndex   int
	durationIndex int
	custom        time.Duration
	stopOnError   bool
	row           configRow

	editing  bool
	input    textinput.Model
	editPrev int

	output       string
	errorOutput  string
	errors       int
	firstError   string
	follow       bool
	errorsOnly   bool
	stoppedOnErr bool

	results  *parse.Results
	logPath  string
	jobPath  string
	exitCode int
	errText  string

	startedAt  time.Time
	finishedAt time.Time
	// runOptions and runPreset are what the active/last run used, so a
	// finished record is never attributed to settings changed mid-run.
	runOptions Options
	runPreset  Preset

	runs       []runRecord
	historyErr string
	lastID     string

	vp   viewport.Model
	keys KeyMap
}

// New builds an idle burn-in tab. jobPath may be empty; the embedded preset is
// always available.
func New(jobPath string, snapshot config.Snapshot) *Model {
	in := textinput.New()
	in.Prompt = ""
	in.CharLimit = 8
	in.Width = 8

	presets, warn := Presets(jobPath)

	return &Model{
		state:         runner.StateIdle,
		start:         defaultStart,
		snapshot:      snapshot,
		now:           time.Now,
		presets:       presets,
		presetWarn:    warn,
		durationIndex: durationIndexDefault,
		custom:        time.Hour,
		stopOnError:   true,
		input:         in,
		follow:        true,
		vp:            viewport.New(60, 10),
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

// Running reports whether a burn-in job is active.
func (m *Model) Running() bool { return m.state == runner.StateRunning }

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
func (m *Model) Title() string { return "Burn-in" }

// State returns the current job state.
func (m *Model) State() runner.State { return m.state }

// Wants reports whether this tab owns runner events of the given kind.
func (m *Model) Wants(kind runner.Kind) bool { return kind == runner.KindBurnIn }

// Keys returns the footer hints for the current state.
func (m *Model) Keys() []ui.KeyHint {
	switch {
	case m.editing:
		return []ui.KeyHint{{Key: "enter", Action: "commit duration"}, {Key: "esc", Action: "cancel edit"}}
	case m.Running():
		return hints(m.keys.Cancel, m.keys.Filter, m.keys.Follow, m.keys.ScrollUp, m.keys.ScrollDn)
	default:
		return hints(m.keys.Run, m.keys.Up, m.keys.Prev, m.keys.Filter, m.keys.Follow)
	}
}

// options returns the configuration the next run would use.
func (m *Model) options() Options {
	return Options{
		Duration:    durationFor(m.durationIndex, m.custom),
		StopOnError: m.stopOnError,
	}
}

// preset returns the selected preset. The embedded preset is always index 0.
func (m *Model) preset() Preset {
	if m.presetIndex < 0 || m.presetIndex >= len(m.presets) {
		return m.presets[0]
	}
	return m.presets[m.presetIndex]
}

// filtered returns the log lines the viewport shows: everything, or only the
// lines that look like errors.
func (m *Model) filtered() string {
	if m.errorsOnly {
		return m.errorOutput
	}
	return m.output
}
