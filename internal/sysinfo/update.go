package sysinfo

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"mem-station/internal/config"
	"mem-station/internal/store"
	"mem-station/internal/ui"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// Update handles messages for the Timings tab and returns any follow-up
// command. Status text is reported to the root shell as ui.StatusMsg.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	if m.modal != modalNone {
		return m.updateModal(msg)
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		// Cursor blink and paste messages belong to the focused input.
		if m.editing {
			return m.updateInput(msg)
		}
		return nil
	}
	return m.handleKey(keyMsg)
}

func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
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
	case key.Matches(msg, m.keys.Inc):
		if m.subView == subTimings {
			return m.adjust(1)
		}
	case key.Matches(msg, m.keys.Dec):
		if m.subView == subTimings {
			return m.adjust(-1)
		}
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
		if m.subView == subTimings && !m.startEditing() {
			f := m.fields[m.focus]
			if opts := m.presetValues(f); len(opts) > 0 {
				return message(f.Label+": use ←/→ to choose "+strings.Join(opts, ", "), ui.LevelInfo)
			}
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
	case key.Matches(msg, m.keys.SaveProfile):
		return m.openSaveProfile()
	case key.Matches(msg, m.keys.LoadProfile):
		return m.openLoadProfile()
	case key.Matches(msg, m.keys.ExportProfile):
		return m.openExportProfile()
	case key.Matches(msg, m.keys.ImportProfile):
		return m.openImportProfile()
	}
	return nil
}

/* --------------- Editing --------------- */

func (m *Model) startEditing() bool {
	i := m.focus
	f := m.fields[i]
	if f.Kind == KindEnum {
		// Enum fields are preset-only: ←/→ cycles their allowed values.
		return false
	}
	m.editing = true
	m.editPrev = m.draft[f.ID]
	m.input.SetValue(m.editPrev)
	m.input.CursorEnd()
	m.input.Focus()
	return true
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

// adjust changes the selected value. Fields with allowed values (enum fields
// and the SPD frequency list) cycle through them; other numeric fields move by
// their declared Step. Frequency changes honour the ratio lock.
func (m *Model) adjust(delta int) tea.Cmd {
	f := m.fields[m.focus]
	previous := strings.TrimSpace(m.draft[f.ID])

	if opts := m.presetValues(f); len(opts) > 0 {
		next := cyclePreset(opts, previous, delta)
		if next == "" || next == previous {
			return nil
		}
		m.draft[f.ID] = next
		if f.ID == FieldFreq {
			m.rescaleFromFrequency(previous)
		}
		return nil
	}
	if f.Kind == KindEnum {
		return nil
	}

	value, ok := parseNumber(previous)
	if !ok {
		if delta < 0 {
			return nil
		}
		value, _ = parseNumber(f.Default)
	}
	step := f.Step
	if step <= 0 {
		step = 1
	}
	base := math.Round(value/step) * step
	next := base + float64(delta)*step
	if next < 0 {
		next = 0
	}
	if !ok && next == 0 {
		next = step
	}
	m.draft[f.ID] = formatScaled(f, next)
	if f.ID == FieldFreq {
		m.rescaleFromFrequency(previous)
	}
	return nil
}

// presetValues returns the allowed/suggested values for a field. Enum fields
// are strict; the frequency list comes from the detected SPD profiles and is
// advisory because a custom frequency can still be typed with Enter.
func (m *Model) presetValues(f Field) []string {
	if f.Kind == KindEnum {
		return f.Options
	}
	if f.ID == FieldFreq && m.spd != nil {
		return m.spd.SupportedFrequencies()
	}
	return nil
}

// cyclePreset moves through options by delta, wrapping at the ends. A custom
// current value (one not in the list) moves to the next higher or lower option
// rather than jumping to the first entry.
func cyclePreset(options []string, current string, delta int) string {
	if len(options) == 0 {
		return ""
	}
	current = strings.TrimSpace(current)
	index := -1
	for i, opt := range options {
		if strings.EqualFold(opt, current) {
			index = i
			break
		}
	}
	if index >= 0 {
		next := ((index+delta)%len(options) + len(options)) % len(options)
		return options[next]
	}
	if value, ok := parseNumber(current); ok {
		best := -1
		bestDistance := math.Inf(1)
		for i, opt := range options {
			n, ok := parseNumber(opt)
			if !ok {
				continue
			}
			distance := n - value
			if delta > 0 && distance <= 0 {
				continue
			}
			if delta < 0 && distance >= 0 {
				continue
			}
			if math.Abs(distance) < bestDistance {
				best, bestDistance = i, math.Abs(distance)
			}
		}
		if best >= 0 {
			return options[best]
		}
	}
	if delta < 0 {
		return options[len(options)-1]
	}
	return options[0]
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
	f := m.fields[m.focus]
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.Text != "" {
		// Bubble Tea v2 carries printable input as a string, so the field's
		// character filter rewrites Text (and Code when it is a single rune)
		// instead of the v1 rune slice.
		text := string(filterInputRunes(f, []rune(keyMsg.Text), m.input.Value()))
		if text == "" {
			return nil
		}
		keyMsg.Text = text
		if r := []rune(text); len(r) == 1 {
			keyMsg.Code = r[0]
		}
		msg = keyMsg
	}
	updated, cmd := m.input.Update(msg)
	if sanitized := filterInputText(f, updated.Value()); sanitized != updated.Value() {
		updated.SetValue(sanitized)
		updated.CursorEnd()
	}
	m.input = updated
	m.draft[f.ID] = m.input.Value()
	return cmd
}

// filterInputRunes rejects characters a numeric field cannot store. Integers
// accept digits only; floats additionally accept one decimal point.
func filterInputRunes(f Field, runes []rune, existing string) []rune {
	if f.Kind != KindInt && f.Kind != KindFloat {
		return runes
	}
	out := make([]rune, 0, len(runes))
	dot := strings.Contains(existing, ".")
	for _, r := range runes {
		switch {
		case r >= '0' && r <= '9':
			out = append(out, r)
		case r == '.' && f.Kind == KindFloat && !dot:
			out = append(out, r)
			dot = true
		}
	}
	return out
}

// filterInputText applies the same rule to an already-inserted value (for
// example a paste) without changing valid characters.
func filterInputText(f Field, value string) string {
	if f.Kind != KindInt && f.Kind != KindFloat {
		return value
	}
	return string(filterInputRunes(f, []rune(value), ""))
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

// updateModal routes messages while a modal owns the input: the apply review
// confirms on enter, the profile prompts additionally forward typing.
func (m *Model) updateModal(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		if m.modal.prompt() {
			var cmd tea.Cmd
			m.prompt, cmd = m.prompt.Update(msg)
			return cmd
		}
		return nil
	}

	if m.modal == modalApply {
		switch keyMsg.String() {
		case "enter":
			return m.confirmApply()
		case "esc", "q":
			m.modal = modalNone
			return message("Apply cancelled - nothing was saved.", ui.LevelInfo)
		}
		return nil
	}

	switch keyMsg.String() {
	case "enter":
		return m.confirmPrompt()
	case "esc", "q":
		title := m.modal.Title()
		m.closeModal()
		return message(title+" cancelled - nothing was changed.", ui.LevelInfo)
	}
	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(keyMsg)
	return cmd
}

// closeModal closes the open modal and drops any prompt error.
func (m *Model) closeModal() {
	m.modal = modalNone
	m.promptErr = ""
	m.promptHint = ""
	m.prompt.Blur()
}

/* --------------- Profiles --------------- */

// openSaveProfile prompts for a display name for the current draft.
func (m *Model) openSaveProfile() tea.Cmd {
	m.commitEdit()
	m.openPrompt(modalSaveProfile, "", "name for the current draft, e.g. daily-3200")
	return nil
}

// openLoadProfile prompts for the name of a saved profile. The hint lists what
// is available so the tab can be driven without guessing.
func (m *Model) openLoadProfile() tea.Cmd {
	m.commitEdit()
	names, err := store.ListProfiles()
	hint := "name of a saved profile"
	switch {
	case err != nil:
		hint = "profile list unavailable: " + err.Error()
	case len(names) == 0:
		hint = "no saved profiles yet - ctrl+p saves the draft"
	default:
		hint = "saved: " + strings.Join(names, ", ")
	}
	m.openPrompt(modalLoadProfile, "", hint)
	return nil
}

// openExportProfile prompts for a destination path, prefilled with the default
// export location inside the state dir.
func (m *Model) openExportProfile() tea.Cmd {
	m.commitEdit()
	path, err := store.ExportPath("profile-" + time.Now().UTC().Format("20060102T150405"))
	if err != nil {
		path = ""
	}
	m.openPrompt(modalExportProfile, path, "file to write the active profile to")
	return nil
}

// openImportProfile prompts for a profile JSON file to load into the draft.
func (m *Model) openImportProfile() tea.Cmd {
	m.commitEdit()
	m.openPrompt(modalImportProfile, "", "path to a profile JSON file")
	return nil
}

// openPrompt opens a text-prompt modal with a prefilled value.
func (m *Model) openPrompt(state modalState, value, hint string) {
	m.modal = state
	m.promptErr = ""
	m.promptHint = hint
	m.prompt.SetValue(value)
	m.prompt.CursorEnd()
	m.prompt.Focus()
}

// confirmPrompt runs the prompt the user confirmed. A failure keeps the modal
// open with the reason, so a typo never closes the dialog silently.
func (m *Model) confirmPrompt() tea.Cmd {
	value := strings.TrimSpace(m.prompt.Value())
	if value == "" {
		m.promptErr = "a value is required"
		return nil
	}
	state := m.modal
	m.promptErr = ""
	var cmd tea.Cmd
	switch state {
	case modalSaveProfile:
		cmd = m.saveNamedProfile(value)
	case modalLoadProfile:
		cmd = m.loadNamedProfile(value)
	case modalExportProfile:
		cmd = m.exportProfile(value)
	case modalImportProfile:
		cmd = m.importProfile(value)
	default:
		return nil
	}
	if m.promptErr != "" {
		return nil
	}
	m.closeModal()
	return cmd
}

// saveNamedProfile persists the draft under a display name. It never touches the
// active profile: applying stays a separate, explicit step.
func (m *Model) saveNamedProfile(name string) tea.Cmd {
	profile := config.Profile{Name: name, Values: m.draftValues(), Source: "draft"}
	if err := store.SaveNamedProfile(profile); err != nil {
		m.promptErr = err.Error()
		return nil
	}
	return message(fmt.Sprintf("Saved the draft as profile %q - ctrl+s applies it.", name), ui.LevelInfo)
}

// loadNamedProfile copies a saved profile into the draft.
func (m *Model) loadNamedProfile(name string) tea.Cmd {
	profile, ok, err := store.LoadNamedProfile(name)
	if err != nil {
		m.promptErr = err.Error()
		return nil
	}
	if !ok {
		m.promptErr = fmt.Sprintf("no profile named %q", name)
		return nil
	}
	loaded := m.applyValuesToDraft(profile.Values)
	return message(fmt.Sprintf("Loaded %d values from profile %q into the draft - ctrl+s applies them.", loaded, name), ui.LevelInfo)
}

// exportProfile writes the applied profile to a user-chosen path.
func (m *Model) exportProfile(path string) tea.Cmd {
	written, err := store.ExportFile(path, m.activeProfile())
	if err != nil {
		m.promptErr = err.Error()
		return nil
	}
	return message("Profile only - not written to hardware. Exported the active profile to "+written, ui.LevelInfo)
}

// importProfile validates a profile JSON file and loads it into the draft.
func (m *Model) importProfile(path string) tea.Cmd {
	profile, err := store.ImportProfile(path)
	if err != nil {
		m.promptErr = err.Error()
		return nil
	}
	loaded := m.applyValuesToDraft(profile.Values)
	return message(fmt.Sprintf("Imported %d values from %s into the draft - ctrl+s applies them.", loaded, profile.Name), ui.LevelInfo)
}

// draftValues returns the draft in canonical-key form.
func (m *Model) draftValues() map[string]string {
	values := make(map[string]string, len(m.fields))
	for _, f := range m.fields {
		values[f.Key] = m.draft[f.ID]
	}
	return values
}

// applyValuesToDraft copies known canonical keys into the draft and reports how
// many fields changed.
func (m *Model) applyValuesToDraft(values map[string]string) int {
	changed := 0
	for _, f := range m.fields {
		v := strings.TrimSpace(values[f.Key])
		if v == "" || m.draft[f.ID] == v {
			continue
		}
		m.draft[f.ID] = v
		changed++
	}
	return changed
}

// activeProfile builds the profile form of what the run tabs are attributed to:
// the applied profile, else the detected values.
func (m *Model) activeProfile() config.Profile {
	snap := m.Snapshot()
	return config.Profile{Name: "active", Values: snap.Values, Source: snap.Source}
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

// scrollActive scrolls the visible read-only sub-view by delta lines. The
// direction-specific methods are used because v2's ScrollDown/ScrollUp
// short-circuit at their respective ends, so a negative ScrollDown would not
// scroll up from the bottom.
func (m *Model) scrollActive(delta int) {
	var vp *viewport.Model
	switch m.subView {
	case subSPD:
		vp = &m.spdVP
	case subIMC:
		vp = &m.imcVP
	}
	if vp == nil {
		return
	}
	switch {
	case delta > 0:
		vp.ScrollDown(delta)
	case delta < 0:
		vp.ScrollUp(-delta)
	}
}
