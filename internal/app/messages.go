package app

import (
	"mem-station/internal/memory"

	tea "charm.land/bubbletea/v2"
)

// detectResultMsg carries a finished hardware detection attempt.
type detectResultMsg struct {
	values map[string]string
	msg    string
	spd    *memory.SPDInfo
}

// imcResultMsg carries the IMC register read (or its error).
type imcResultMsg struct {
	timings []memory.IMCTimings
	err     error
}

// clearStatusMsg clears the status line if it is still the one that scheduled
// the clear.
type clearStatusMsg struct {
	seq int
}

// detectCmd runs hardware detection off the UI goroutine.
func (m *Model) detectCmd() tea.Cmd {
	detect := m.opts.Detect
	return func() tea.Msg {
		vals, msg, spd := detect()
		return detectResultMsg{values: vals, msg: msg, spd: spd}
	}
}

// imcCmd reads the IMC register dump off the UI goroutine.
func imcCmd() tea.Cmd {
	return func() tea.Msg {
		timings, err := memory.ReadMCHBARTimings()
		return imcResultMsg{timings: timings, err: err}
	}
}
