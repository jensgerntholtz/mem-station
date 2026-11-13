// Package app is the root Bubble Tea shell: layout, global keys, tab routing,
// status/footer, and the help overlay. It owns the tab models but never reaches
// into their internals.
package app

import (
	"os"

	"mem-station/internal/benchmark"
	"mem-station/internal/burnin"
	"mem-station/internal/config"
	"mem-station/internal/memory"
	"mem-station/internal/runner"
	"mem-station/internal/style"
	"mem-station/internal/sysinfo"
	"mem-station/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

// Version is the default version string shown in the header.
const Version = "0.2.0-dev"

// Tab is the contract every tab implements. Tabs may import config, runner,
// store, style, ui and msgs, but never this package.
type Tab interface {
	// Init returns startup commands for the tab.
	Init() tea.Cmd
	// Update handles a message and returns any follow-up command.
	Update(tea.Msg) tea.Cmd
	// View renders the tab into exactly w x h cells.
	View(w, h int) string
	// Title is the tab label.
	Title() string
	// Keys are the context key hints shown in the footer.
	Keys() []ui.KeyHint
	// Help are the help sections for the help overlay.
	Help() []ui.HelpSection
	// Editing reports whether a text input owns letter keys.
	Editing() bool
	// ModalOpen reports whether a modal owns all input.
	ModalOpen() bool
	// Running reports whether the tab has an active job.
	Running() bool
	// Dirty reports unsaved draft changes.
	Dirty() bool
	// Wants reports whether the tab owns runner events of a kind.
	Wants(runner.Kind) bool
	// Blur commits/ends any in-progress edit.
	Blur()
	// Stop cancels any running job (called on quit).
	Stop()
}

// Options configures the root model.
type Options struct {
	InitialTab int
	JobPath    string
	StateDir   string
	Dark       bool
	Mouse      bool
	Version    string
	Detect     func() (map[string]string, string, *memory.SPDInfo)
}

// DefaultOptions returns the production defaults.
func DefaultOptions() Options {
	return Options{
		Version: Version,
		Dark:    true,
		Detect:  memory.DetectDefaults,
	}
}

// Model is the root applicaton model.
type Model struct {
	opts Options

	width, height int
	active        int

	sys   *sysinfo.Model
	bench *benchmark.Model
	burn  *burnin.Model
	tabs  []Tab

	keys KeyMap

	status    string
	level     ui.Level
	statusSeq int

	helpOpen   bool
	helpOffset int

	tickScheduled bool
}

// New builds the root model.
func New(opts Options) *Model {
	if opts.Detect == nil {
		opts.Detect = memory.DetectDefaults
	}
	if opts.Version == "" {
		opts.Version = Version
	}
	// The store reads MEM_STATION_STATE_DIR; honour an explicit StateDir.
	if opts.StateDir != "" {
		os.Setenv("MEM_STATION_STATE_DIR", opts.StateDir)
	}

	style.Set(opts.Dark)

	sys := sysinfo.New()
	snapshot := sys.Snapshot()
	bench := benchmark.New(snapshot)
	burn := burnin.New(opts.JobPath, snapshot)

	m := &Model{
		opts:  opts,
		sys:   sys,
		bench: bench,
		burn:  burn,
		tabs:  []Tab{sys, bench, burn},
		keys:  DefaultKeyMap(),
		level: ui.LevelInfo,
	}
	if opts.InitialTab >= 0 && opts.InitialTab < len(m.tabs) {
		m.active = opts.InitialTab
	}
	m.status = m.defaultStatus()
	return m
}

// snapshot returns the Timings tab's current configuration snapshot.
func (m *Model) snapshot() config.Snapshot { return m.sys.Snapshot() }

// syncSnapshot forwards the current configuration to the run tabs.
func (m *Model) syncSnapshot() {
	s := m.snapshot()
	m.bench.SetSnapshot(s)
	m.burn.SetSnapshot(s)
}

// anyRunning reports whether any tab has an active job.
func (m *Model) anyRunning() bool {
	for _, t := range m.tabs {
		if t.Running() {
			return true
		}
	}
	return false
}

// HelpSections returns the global help plus the active tab's sections.
func (m *Model) HelpSections() []ui.HelpSection {
	global := ui.HelpSection{
		Title: "Global",
		Rows: []ui.KeyHint{
			{Key: "f1/f2/f3", Action: "switch tabs (or 1/2/3)"},
			{Key: "?", Action: "toggle help"},
			{Key: "esc", Action: "close help"},
			{Key: "R", Action: "retry detection"},
			{Key: "ctrl+c", Action: "cancel a running job, else quit"},
			{Key: "q", Action: "quit"},
		},
	}
	return append([]ui.HelpSection{global}, m.tabs[m.active].Help()...)
}
