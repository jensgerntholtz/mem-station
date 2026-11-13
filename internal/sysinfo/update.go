package sysinfo

import (
	"fmt"
	"math"
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
	if m.modal != modalNone {
		return m.updateModal(msg)
	}
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
			return nil
		case key.Matches(msg, m.keys.Next):
			m.commitEdit()
			m.moveFocus(1)
			return nil
		case key.Matches(msg, m.keys.Prev):
			m.commitEdit()
			m.moveFocus(-1)
			return nil
		default:
			return m.updateInput(msg)
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
			return message("Ratio lock on: committing a DRAM freq edit rescales the primary timings.", ui.LevelInfo)
		}
		return message("Ratio lock off: timings are edited independently.", ui.LevelInfo)
	case key.Matches(msg, m.keys.Apply):
		return m.openApply()
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
	f := m.fields[i]
	m.editing = true
	m.editPrev = m.draft[f.ID]
	m.input.SetValue(m.editPrev)
	m.input.CursorEnd()
	m.input.Focus()
}

// commitEdit ends editing and applies the frequency ratio lock, if the edited
// field was the DRAM frequency. Rescaling happens on commit, never per
// keystroke, so intermediate values never rewrite the sibling drafts.
func (m *Model) commitEdit() {
	if !m.editing {
		return
	}
	f := m.fields[m.focus]
	m.input.Blur()
	m.editing = false
	if f.ID == FieldFreq {
		m.rescaleFromFrequency(m.editPrev)
	}
}

func (m *Model) cancelEdit() {
	if !m.editing {
		return
	}
	f := m.fields[m.focus]
	m.draft[f.ID] = m.editPrev
	m.input.SetValue(m.editPrev)
	m.input.Blur()
	m.editing = false
}

func (m *Model) moveFocus(delta int) {
	n := len(m.fields)
	m.focus = ((m.focus+delta)%n + n) % n
}

// resetDraft restores every field to its baseline (applied, detected, default).
func (m *Model) resetDraft() {
	for _, f := range m.fields {
		m.draft[f.ID] = m.baseline(f.ID)
	}
}

// updateInput forwards a message to the focused text input and mirrors the
// input value into the draft so the renderer and Dirty() stay live.
func (m *Model) updateInput(msg tea.Msg) tea.Cmd {
	if !m.editing {
		return nil
	}
	updated, cmd := m.input.Update(msg)
	m.input = updated
	m.draft[m.fields[m.focus].ID] = m.input.Value()
	return cmd
}

// rescaleFromFrequency scales the frequency-bound drafts by the ratio between
// the previous and the new frequency, then clamps them to their safe minimum.
// It is a no-op unless the ratio lock is enabled.
func (m *Model) rescaleFromFrequency(prevRaw string) {
	if !m.lockRatios {
		return
	}
	ctx := m.context()
	newFreq, ok := ctx.Float(FieldFreq)
	if !ok || newFreq <= 0 {
		return
	}
	prevFreq, ok := parseNumber(prevRaw)
	if !ok || prevFreq <= 0 || prevFreq == newFreq {
		return
	}
	ratio := newFreq / prevFreq

	for _, f := range m.fields {
		if f.ID == FieldFreq || f.Scale == 0 {
			continue
		}
		cur, ok := parseNumber(m.draft[f.ID])
		if !ok || cur <= 0 {
			continue
		}
		scaled := cur * ratio * f.Scale
		if f.Bounds != nil {
			if b := f.Bounds(ctx); b.HasMin && scaled < b.Min {
				scaled = b.Min
			}
		}
		m.draft[f.ID] = formatScaled(f, scaled)
	}
}

// formatScaled snaps a rescaled value to the field's step.
func formatScaled(f Field, v float64) string {
	step := f.Step
	if step <= 0 {
		step = 1
	}
	v = math.Round(v/step) * step
	if step >= 1 {
		return strconv.Itoa(int(math.Round(v)))
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

/* --------------- Apply --------------- */

// openApply opens the review modal. Applying is a two-step flow: review, then
// confirm.
func (m *Model) openApply() tea.Cmd {
	if m.subView != subTimings {
		return message("Switch to the Timings view to apply the draft.", ui.LevelInfo)
	}
	m.commitEdit()
	if !m.Dirty() {
		return message("Nothing to apply - the draft matches the applied profile.", ui.LevelInfo)
	}
	m.modal = modalApply
	return nil
}

// updateModal routes keys while the apply review modal owns the input.
func (m *Model) updateModal(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch keyMsg.String() {
	case "enter":
		return m.confirmApply()
	case "esc", "q":
		m.modal = modalNone
		return message("Apply cancelled - nothing was saved.", ui.LevelInfo)
	}
	return nil
}

// confirmApply persists the draft as the active profile. Hardware writes are
// not implemented and the confirmation says so explicitly.
func (m *Model) confirmApply() tea.Cmd {
	if m.modal != modalApply {
		return nil
	}
	m.modal = modalNone

	values := make(map[string]string, len(m.fields))
	for _, f := range m.fields {
		values[f.Key] = m.draft[f.ID]
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
	applied := make(map[FieldID]string, len(m.draft))
	for id, v := range m.draft {
		applied[id] = v
	}
	m.applied = applied
	return message(fmt.Sprintf("Profile only - not written to hardware. Saved %d values to the active profile.", len(values)), ui.LevelWarn)
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
