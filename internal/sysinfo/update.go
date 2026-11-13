package sysinfo

import (
	"fmt"
	"strconv"
	"time"

	"mem-station/internal/config"
	"mem-station/internal/store"
	"mem-station/internal/ui"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles messages for the Timings tab and returns any follow-up
// command. Status text is reported to the root shell as ui.StatusMsg.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		// Cursor blink and paste messages belong to the focused input.
		if m.editing {
			return m.updateInput(msg)
		}
		return nil
	}
	return m.handleKey(keyMsg)
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	// While editing, only the edit-control keys are interpreted here; every
	// other key goes to the text input (so "q" types a letter).
	if m.editing {
		switch {
		case key.Matches(msg, m.keys.Cancel):
			m.cancelEdit()
			return message("Edit cancelled.", ui.LevelInfo)
		case key.Matches(msg, m.keys.Edit):
			m.commitEdit()
			return message("Value committed.", ui.LevelInfo)
		case key.Matches(msg, m.keys.Next):
			m.commitEdit()
			m.moveFocus(1)
			return nil
		case key.Matches(msg, m.keys.Prev):
			m.commitEdit()
			m.moveFocus(-1)
			return nil
		case key.Matches(msg, m.keys.Up), key.Matches(msg, m.keys.Down):
			return m.updateInput(msg)
		default:
			return m.updateInputWithRatios(msg)
		}
	}

	switch {
	case key.Matches(msg, m.keys.Cancel):
		if m.subView != subTimings {
			m.subView = subTimings
			return nil
		}
		m.resetDraft()
		return message("Draft reset to the applied values.", ui.LevelInfo)
	case key.Matches(msg, m.keys.Up):
		if m.subView == subTimings {
			m.moveFocus(-1)
		} else {
			m.scrollActive(-1)
		}
	case key.Matches(msg, m.keys.Down):
		if m.subView == subTimings {
			m.moveFocus(1)
		} else {
			m.scrollActive(1)
		}
	case key.Matches(msg, m.keys.Edit):
		if m.subView == subTimings {
			m.startEditing()
		}
	case key.Matches(msg, m.keys.Next):
		if m.subView == subTimings {
			m.moveFocus(1)
		}
	case key.Matches(msg, m.keys.Prev):
		if m.subView == subTimings {
			m.moveFocus(-1)
		}
	case key.Matches(msg, m.keys.RatioLock):
		m.lockRatios = !m.lockRatios
		if m.lockRatios {
			return message("Ratio lock on: frequency edits rescale primary timings.", ui.LevelInfo)
		}
		return message("Ratio lock off: timings are edited independently.", ui.LevelInfo)
	case key.Matches(msg, m.keys.Apply):
		return m.apply()
	case key.Matches(msg, m.keys.ResetAll):
		m.resetDraft()
		return message("Draft reset to the applied values.", ui.LevelInfo)
	case key.Matches(msg, m.keys.NextView):
		m.cycleView(1)
	case key.Matches(msg, m.keys.PrevView):
		m.cycleView(-1)
	case key.Matches(msg, m.keys.ScrollUp):
		m.scrollActive(-1)
	case key.Matches(msg, m.keys.ScrollDn):
		m.scrollActive(1)
	}
	return nil
}

/* --------------- Editing --------------- */

func (m *Model) startEditing() {
	i := m.focus
	m.editing = true
	m.prevValue = m.fields[i].input.Value()
	m.fields[i].input.CursorEnd()
	m.fields[i].input.Focus()
}

func (m *Model) commitEdit() {
	if !m.editing {
		return
	}
	m.fields[m.focus].input.Blur()
	m.editing = false
}

func (m *Model) cancelEdit() {
	if !m.editing {
		return
	}
	m.fields[m.focus].input.SetValue(m.prevValue)
	m.fields[m.focus].input.Blur()
	m.editing = false
}

func (m *Model) moveFocus(delta int) {
	n := len(m.fields)
	m.focus = ((m.focus+delta)%n + n) % n
}

func (m *Model) resetDraft() {
	for i := range m.fields {
		m.fields[i].input.SetValue(m.baseline(m.fields[i].label))
	}
}

// updateInput forwards a message to the focused text input.
func (m *Model) updateInput(msg tea.Msg) tea.Cmd {
	if !m.editing {
		return nil
	}
	i := m.focus
	updated, cmd := m.fields[i].input.Update(msg)
	m.fields[i].input = updated
	return cmd
}

// updateInputWithRatios forwards a message to the focused input and, when the
// edited field is the DRAM frequency, rescales the primary timings.
func (m *Model) updateInputWithRatios(msg tea.Msg) tea.Cmd {
	i := m.focus
	before := m.fields[i].input.Value()
	cmd := m.updateInput(msg)
	if m.fields[i].label == "DRAM Freq" && m.fields[i].input.Value() != before {
		m.rescaleFromFrequency()
	}
	return cmd
}

// rescaleFromFrequency scales the primary timing drafts from their detected
// values using the current frequency draft. It is a no-op unless the ratio
// lock is enabled.
func (m *Model) rescaleFromFrequency() {
	if !m.lockRatios {
		return
	}
	newFreq, ok := m.parseField("DRAM Freq")
	if !ok || newFreq <= 0 {
		return
	}
	baseFreq := m.detectedField("DRAM Freq")
	if baseFreq <= 0 {
		return
	}
	ratio := newFreq / baseFreq
	for _, label := range []string{"CAS (tCL)", "tRCD", "tRP", "tRAS", "tRFC"} {
		i := m.fieldIndex(label)
		if i < 0 {
			continue
		}
		base, ok := parseUnsignedInt(m.fields[i].hint)
		if !ok {
			continue
		}
		scaled := int(float64(base)*ratio + 0.5)
		if scaled < 1 {
			scaled = 1
		}
		m.fields[i].input.SetValue(strconv.Itoa(scaled))
	}
}

/* --------------- Apply --------------- */

// apply persists the draft as the active profile. Hardware writes are not
// implemented; the status message says so explicitly.
func (m *Model) apply() tea.Cmd {
	if !m.Dirty() {
		return message("Nothing to apply - the draft matches the applied profile.", ui.LevelInfo)
	}
	values := map[string]string{}
	for i := range m.fields {
		values[m.fields[i].key] = m.fields[i].input.Value()
	}
	profile := config.Profile{
		Name:      "active",
		Values:    values,
		UpdatedAt: time.Now(),
		Source:    "apply",
	}
	if err := store.SaveActiveProfile(profile); err != nil {
		return message("Apply failed: "+err.Error(), ui.LevelError)
	}
	m.applied = m.applyDetectedValues()
	return message(fmt.Sprintf("Profile only - saved %d values to the state dir; hardware was not written.", len(values)), ui.LevelWarn)
}

/* --------------- Sub-views --------------- */

func (m *Model) cycleView(delta int) {
	if m.editing {
		m.commitEdit()
	}
	n := 3
	v := (int(m.subView) + delta%n + n) % n
	m.subView = subView(v)
}

func (m *Model) scrollActive(delta int) {
	switch m.subView {
	case subSPD:
		m.spdVP.LineDown(delta)
	case subIMC:
		m.imcVP.LineDown(delta)
	}
}
