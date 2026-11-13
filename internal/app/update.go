package app

import (
	"time"

	"mem-station/internal/msgs"
	"mem-station/internal/runner"
	"mem-station/internal/ui"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// tickInterval is the elapsed-time refresh while a job runs.
const tickInterval = 500 * time.Millisecond

// clearStatusAfter is how long an informational status stays on screen.
const clearStatusAfter = 5 * time.Second

// Init starts async detection plus any tab startup commands.
func (m *Model) Init() tea.Cmd {
	cmd := tea.Batch(
		m.detectCmd(),
		imcCmd(),
		m.tabs[0].Init(),
		m.tabs[1].Init(),
		m.tabs[2].Init(),
	)
	m.status = "Detecting hardware..."
	m.level = ui.LevelInfo
	return cmd
}

// Update routes messages: size, keys, detection, status, job events, and ticks.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m, m.handleKey(msg)

	case detectResultMsg:
		m.sys.SetDetected(msg.values, msg.msg, msg.spd)
		m.syncSnapshot()
		if len(msg.values) == 0 {
			return m, m.setStatus(msg.msg+" · press R to retry", ui.LevelWarn)
		}
		return m, m.setStatus(msg.msg, ui.LevelInfo)

	case imcResultMsg:
		m.sys.SetIMC(msg.timings, msg.err)
		return m, nil

	case msgs.JobEvent:
		return m, m.routeJobEvent(msg)

	case msgs.Tick:
		m.tickScheduled = false
		cmds := make([]tea.Cmd, 0, 2)
		for _, t := range m.tabs {
			if c := t.Update(msg); c != nil {
				cmds = append(cmds, c)
			}
		}
		return m, tea.Batch(append(cmds, m.ensureTick())...)

	case ui.StatusMsg:
		return m, m.setStatus(msg.Text, msg.Level)

	case clearStatusMsg:
		if msg.seq == m.statusSeq {
			m.status = m.defaultStatus()
			m.level = ui.LevelInfo
		}
		return m, nil
	}

	// Everything else (cursor blink, paste, ...) belongs to the active tab.
	if c := m.tabs[m.active].Update(msg); c != nil {
		return m, c
	}
	return m, nil
}

// handleKey implements global key ownership, then forwards to the active tab.
func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	// ctrl+c cancels a running job and stays in the app; with nothing running
	// it quits.
	if msg.String() == "ctrl+c" {
		if m.anyRunning() {
			for _, t := range m.tabs {
				t.Stop()
			}
			m.resetStatus()
			return m.setStatus("Cancelling the running job...", ui.LevelWarn)
		}
		return m.quit()
	}

	if m.helpOpen {
		switch msg.String() {
		case "?", "esc", "q":
			m.helpOpen = false
		case "up", "k":
			if m.helpOffset > 0 {
				m.helpOffset--
			}
		case "down", "j":
			m.helpOffset++
		case "pgup":
			m.helpOffset = max(0, m.helpOffset-10)
		case "pgdown":
			m.helpOffset += 10
		}
		return nil
	}

	active := m.tabs[m.active]

	// A modal or an active text input owns every key except the three handled
	// above; the tab decides what esc/enter/tab mean.
	if active.ModalOpen() || active.Editing() {
		cmd := active.Update(msg)
		if m.active == 0 {
			// A confirmed apply inside the modal changes the applied profile.
			m.syncSnapshot()
		}
		return cmd
	}

	switch {
	case key.Matches(msg, m.keys.Help):
		m.helpOpen = true
		m.helpOffset = 0
		return nil
	case key.Matches(msg, m.keys.Quit):
		return m.quit()
	case key.Matches(msg, m.keys.RetryDetect):
		return m.retryDetection()
	case key.Matches(msg, m.keys.Tab1):
		m.switchTab(0)
		return nil
	case key.Matches(msg, m.keys.Tab2):
		m.switchTab(1)
		return nil
	case key.Matches(msg, m.keys.Tab3):
		m.switchTab(2)
		return nil
	}

	// Plain digits switch tabs only when no text input owns the keyboard.
	if s := msg.String(); len(s) == 1 && s[0] >= '1' && s[0] <= '3' {
		m.switchTab(int(s[0] - '1'))
		return nil
	}

	cmd := active.Update(msg)
	if m.active == 0 {
		m.syncSnapshot()
	}
	return cmd
}

func (m *Model) retryDetection() tea.Cmd {
	m.sys.Blur()
	return tea.Batch(m.detectCmd(), m.setStatus("Retrying hardware detection...", ui.LevelInfo))
}

func (m *Model) switchTab(i int) {
	if i < 0 || i >= len(m.tabs) || i == m.active {
		return
	}
	m.tabs[m.active].Blur()
	m.active = i
	m.syncSnapshot()
}

func (m *Model) quit() tea.Cmd {
	for _, t := range m.tabs {
		t.Stop()
	}
	return tea.Quit
}

// routeJobEvent delivers a runner event to the owning tab, then keeps the
// elapsed ticker alive while any job runs. A new run supersedes an error banner
// left over from an earlier run.
func (m *Model) routeJobEvent(msg msgs.JobEvent) tea.Cmd {
	if msg.Ev.Type == runner.EventStarted {
		m.resetStatus()
	}
	cmds := make([]tea.Cmd, 0, 3)
	for _, t := range m.tabs {
		if !t.Wants(msg.Ev.Kind) {
			continue
		}
		if c := t.Update(msg); c != nil {
			cmds = append(cmds, c)
		}
	}
	if msg.Ev.Type == runner.EventFinished {
		m.syncSnapshot()
	}
	return tea.Batch(append(cmds, m.ensureTick())...)
}

// ensureTick schedules the next elapsed tick at most once.
func (m *Model) ensureTick() tea.Cmd {
	if m.tickScheduled || !m.anyRunning() {
		return nil
	}
	m.tickScheduled = true
	return msgs.TickEvery(tickInterval)
}

// setStatus publishes a status line and schedules its auto-clear. Error banners
// stay until another error, a new run, or an explicit cancel replaces them, so
// background info messages never hide a failure.
func (m *Model) setStatus(text string, level ui.Level) tea.Cmd {
	if m.level == ui.LevelError && level != ui.LevelError {
		return nil
	}
	m.status = text
	m.level = level
	m.statusSeq++
	if level == ui.LevelError {
		return nil
	}
	seq := m.statusSeq
	return tea.Tick(clearStatusAfter, func(time.Time) tea.Msg { return clearStatusMsg{seq: seq} })
}

// resetStatus drops an active error banner, restoring the context hint.
func (m *Model) resetStatus() {
	if m.level != ui.LevelError {
		return
	}
	m.level = ui.LevelInfo
	m.status = m.defaultStatus()
	m.statusSeq++
}

func (m *Model) defaultStatus() string {
	switch m.active {
	case 0:
		return "Timings · enter edit · ctrl+s apply · ctrl+r reset · [/] views · ? help"
	case 1:
		return "Benchmark · enter run · ↑/↓ rows · ←/→ values · f follow · e export"
	default:
		return "Burn-in · enter run · ↑/↓ rows · ←/→ values · e filter · f follow · s save"
	}
}
