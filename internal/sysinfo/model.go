// Package sysinfo implements the Timings tab: the timing editor, the perf and
// guidance panel, the apply review modal, and the SPD/IMC read-only sub-views.
//
// Values are keyed by FieldID (see fields.go) and kept as strings so "2T" and
// "1.35" survive a round-trip through the profile store. Three maps hold the
// tab state: detected (hardware), applied (the active profile, or the detected
// values when no profile exists), and draft (what the user is editing).
//
// It never imports internal/app.
package sysinfo

import (
	"strings"
	"time"

	"mem-station/internal/config"
	"mem-station/internal/memory"
	"mem-station/internal/runner"
	"mem-station/internal/store"
	"mem-station/internal/ui"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// subView selects which read-only or editable panel the tab shows.
type subView int

const (
	subTimings subView = iota
	subSPD
	subIMC
)

func (s subView) String() string {
	switch s {
	case subSPD:
		return "SPD"
	case subIMC:
		return "IMC"
	default:
		return "Timings"
	}
}

// modalState tracks the open modal, if any.
type modalState int

const (
	modalNone modalState = iota
	modalApply
	modalSaveProfile
	modalLoadProfile
	modalExportProfile
	modalImportProfile
)

// Title names the modal for the dialog and the help overlay.
func (s modalState) Title() string {
	switch s {
	case modalApply:
		return "Apply review"
	case modalSaveProfile:
		return "Save profile as"
	case modalLoadProfile:
		return "Load profile"
	case modalExportProfile:
		return "Export profile JSON"
	case modalImportProfile:
		return "Import profile JSON"
	default:
		return "Dialog"
	}
}

// prompt reports whether the modal asks for a typed value instead of a yes/no
// decision.
func (s modalState) prompt() bool {
	switch s {
	case modalSaveProfile, modalLoadProfile, modalExportProfile, modalImportProfile:
		return true
	}
	return false
}

// Model is the Timings tab.
type Model struct {
	fields []Field

	draft    map[FieldID]string
	applied  map[FieldID]string
	detected map[FieldID]string

	focus      int
	editing    bool
	editPrev   string
	input      textinput.Model
	lockRatios bool

	detectMsg string

	subView subView
	spd     *memory.SPDInfo
	imc     []memory.IMCTimings
	imcNote string

	spdVP viewport.Model
	imcVP viewport.Model

	modal modalState
	// prompt is the text input of a profile-name/path modal; promptHint and
	// promptErr explain what is expected and why the last confirm failed.
	prompt     textinput.Model
	promptHint string
	promptErr  string

	keys KeyMap
}

// New builds the Timings tab, loading the applied profile if one exists. The
// draft starts at the applied values, or at the field placeholders until
// detection arrives.
func New() *Model {
	applied := loadApplied()
	draft := make(map[FieldID]string, len(fields))
	for _, f := range fields {
		if v, ok := applied[f.ID]; ok {
			draft[f.ID] = v
			continue
		}
		draft[f.ID] = f.Default
	}

	in := textinput.New()
	in.Prompt = "" // the label column already shows which field is focused
	in.CharLimit = 8
	in.Width = draftColW - 1

	prompt := textinput.New()
	prompt.Prompt = "> "
	prompt.CharLimit = 512
	prompt.Width = 60

	return &Model{
		fields:     Fields(),
		draft:      draft,
		applied:    applied,
		detected:   map[FieldID]string{},
		input:      in,
		prompt:     prompt,
		lockRatios: true,
		imcNote:    "Raw MCHBAR registers are opt-in; press I to read them.",
		spdVP:      viewport.New(60, 10),
		imcVP:      viewport.New(60, 10),
		keys:       DefaultKeyMap(),
	}
}

// loadApplied loads the active profile as FieldID -> value. It returns nil when
// no profile exists or it contains no recognised values.
func loadApplied() map[FieldID]string {
	p, ok, err := store.LoadActiveProfile()
	if err != nil || !ok {
		return nil
	}
	out := map[FieldID]string{}
	for _, f := range fields {
		if v := strings.TrimSpace(p.Values[f.Key]); v != "" {
			out[f.ID] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Init implements tea.Model; detection is owned by the root shell.
func (m *Model) Init() tea.Cmd { return nil }

// SetDetected installs a hardware detection result. Detection keys may be field
// IDs, config keys, or the labels used by memory.DetectDefaults; all three are
// resolved here so the shell never needs to know field names.
func (m *Model) SetDetected(vals map[string]string, msg string, spd *memory.SPDInfo) {
	m.detectMsg = msg
	m.spd = spd
	for _, f := range m.fields {
		v := detectedValue(vals, f)
		if v == "" {
			continue
		}
		previous := m.baseline(f.ID)
		m.detected[f.ID] = v
		if _, hasApplied := m.applied[f.ID]; hasApplied {
			continue // an applied profile wins over detection
		}
		if m.editing && m.fields[m.focus].ID == f.ID {
			continue // never overwrite a value being typed
		}
		if m.draft[f.ID] == previous {
			m.draft[f.ID] = v
		}
	}
}

// detectedValue resolves one detected value for a field.
func detectedValue(vals map[string]string, f Field) string {
	for _, key := range []string{string(f.ID), f.Key, f.DetectLabel} {
		if key == "" {
			continue
		}
		if v := strings.TrimSpace(vals[key]); v != "" {
			return v
		}
	}
	return ""
}

// SetIMC installs the IMC register dump shown in the IMC sub-view.
func (m *Model) SetIMC(timings []memory.IMCTimings, err error) {
	m.imc = timings
	if err != nil {
		m.imcNote = err.Error()
	} else {
		m.imcNote = ""
	}
}

// Detected reports whether hardware detection produced values.
func (m *Model) Detected() bool { return len(m.detected) > 0 }

// DetectMessage returns the message produced by the last detection attempt.
func (m *Model) DetectMessage() string { return m.detectMsg }

// Editing reports whether a field's text input owns letter keys.
func (m *Model) Editing() bool { return m.editing }

// ModalOpen reports whether the apply review modal is open.
func (m *Model) ModalOpen() bool { return m.modal != modalNone }

// Running reports whether this tab has an active job (never for Timings).
func (m *Model) Running() bool { return false }

// Wants reports whether this tab owns runner events of the given kind.
func (m *Model) Wants(runner.Kind) bool { return false }

// Dirty reports whether the draft differs from the applied values.
func (m *Model) Dirty() bool {
	for _, f := range m.fields {
		if m.draft[f.ID] != m.baseline(f.ID) {
			return true
		}
	}
	return false
}

// Blur commits any in-progress edit.
func (m *Model) Blur() { m.commitEdit() }

// Stop closes any modal and cancels any running job (the Timings tab owns none).
func (m *Model) Stop() {
	m.closeModal()
	m.commitEdit()
}

// Title returns the tab title.
func (m *Model) Title() string { return "Timings" }

// SubViewTitle returns the active sub-view name for the tab bar/header.
func (m *Model) SubViewTitle() string { return m.subView.String() }

// Summary returns a short hardware summary for the header.
func (m *Model) Summary() string { return m.spdSummary() }

// Snapshot returns the configuration the run tabs should attribute a run to:
// the applied profile when one exists, otherwise the detected values.
func (m *Model) Snapshot() config.Snapshot {
	source := "applied"
	if m.applied == nil {
		source = "detected"
	}
	vals := make(map[string]string, len(m.fields))
	for _, f := range m.fields {
		vals[f.Key] = m.baseline(f.ID)
	}
	return config.Snapshot{Values: vals, CapturedAt: time.Now(), Source: source}
}

// baseline returns the value a field reverts to: the applied value, else the
// detected value, else the field placeholder.
func (m *Model) baseline(id FieldID) string {
	if v, ok := m.applied[id]; ok {
		return v
	}
	if v, ok := m.detected[id]; ok {
		return v
	}
	if f, ok := LookupField(id); ok {
		return f.Default
	}
	return ""
}

// context returns a validation context over the draft values.
func (m *Model) context() ValidationContext {
	return NewValidationContext(m.draft)
}

// issues validates the draft.
func (m *Model) issues() []Issue { return Validate(m.context()) }

// fieldIndex returns the position of id in the field list, or -1.
func (m *Model) fieldIndex(id FieldID) int {
	for i := range m.fields {
		if m.fields[i].ID == id {
			return i
		}
	}
	return -1
}

// message is the status message a tab wants the root shell to show.
func message(text string, level ui.Level) tea.Cmd {
	return ui.Status(text, level)
}
