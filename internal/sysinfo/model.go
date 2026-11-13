// Package sysinfo implements the Timings tab: the timing editor, the perf
// guidance panel, and the SPD/IMC read-only sub-views.
//
// It never imports internal/app. Phase 1 keeps the Phase 0 label-keyed field
// model; Phase 2 replaces the label lookups with typed fields and an explicit
// detected/draft/applied state.
package sysinfo

import (
	"strconv"
	"strings"

	"mem-station/internal/config"
	"mem-station/internal/memory"
	"mem-station/internal/runner"
	"mem-station/internal/store"
	"mem-station/internal/ui"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// field is one editable timing value.
type field struct {
	label string // stable display label and internal id (Phase 2: FieldID)
	key   string // canonical config key used for persistence
	hint  string // last detected (or fallback) value
	desc  string
	input textinput.Model
}

// fieldBounds is a validated numeric range for a field.
type fieldBounds struct {
	min, max float64
	note     string
	hasMin   bool
	hasMax   bool
}

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

// Model is the Timings tab.
type Model struct {
	fields     []field
	focus      int
	editing    bool
	prevValue  string
	lockRatios bool

	detected  bool
	detectMsg string

	subView subView
	spd     *memory.SPDInfo
	imc     []memory.IMCTimings
	imcNote string

	applied map[string]string // label -> applied value; nil when no profile

	spdVP viewport.Model
	imcVP viewport.Model

	keys KeyMap
}

// fieldDefs is the Phase 1 field list: label, config key, fallback, help text.
var fieldDefs = []struct {
	label, key, fallback, desc string
}{
	{"Channels", config.KeyChannels, "2", "Detected memory channels."},
	{"DRAM Ratio", config.KeyRatio, "?", "Memory multiplier relative to base clock."},
	{"DRAM Freq", config.KeyFreq, "1600", "Effective DRAM frequency in MT/s."},
	{"CAS (tCL)", config.KeyCL, "?", "CAS latency in cycles."},
	{"tRCD", config.KeyRCD, "?", "RAS to CAS delay in cycles."},
	{"tRP", config.KeyRP, "?", "Row precharge delay in cycles."},
	{"tRAS", config.KeyRAS, "?", "Active to precharge delay in cycles."},
	{"Command Rate", config.KeyCR, "2T", "Command rate; usually 1T or 2T."},
	{"tRFC", config.KeyRFC, "?", "Refresh cycle time in cycles."},
	{"DRAM Voltage", config.KeyVoltage, "?", "Requested memory rail voltage."},
}

// New builds the Timings tab, loading the applied profile if one exists.
func New() *Model {
	base := textinput.New()
	base.Prompt = "" // the label column already shows which field is focused
	base.CharLimit = 8
	base.Width = 8

	applied := loadApplied()

	fields := make([]field, 0, len(fieldDefs))
	for _, def := range fieldDefs {
		value := def.fallback
		if applied != nil {
			if v, ok := applied[def.label]; ok {
				value = v
			}
		}
		in := base
		in.SetValue(value)
		in.Placeholder = def.fallback
		fields = append(fields, field{
			label: def.label,
			key:   def.key,
			hint:  def.fallback,
			desc:  def.desc,
			input: in,
		})
	}

	return &Model{
		fields:     fields,
		lockRatios: true,
		applied:    applied,
		spdVP:      viewport.New(60, 10),
		imcVP:      viewport.New(60, 10),
		keys:       DefaultKeyMap(),
	}
}

// loadApplied loads the active profile as label -> value. It returns nil when
// no profile exists or it contains no recognised values.
func loadApplied() map[string]string {
	p, ok, err := store.LoadActiveProfile()
	if err != nil || !ok {
		return nil
	}
	applied := map[string]string{}
	for _, def := range fieldDefs {
		if v := p.Values[def.key]; v != "" {
			applied[def.label] = v
		}
	}
	if len(applied) == 0 {
		return nil
	}
	return applied
}

// Init implements tea.Model; detection is owned by the root shell.
func (m *Model) Init() tea.Cmd { return nil }

// SetDetected merges a hardware detection result into the editor.
func (m *Model) SetDetected(vals map[string]string, msg string, spd *memory.SPDInfo) {
	m.detectMsg = msg
	m.spd = spd
	m.detected = len(vals) > 0
	if spd != nil {
		m.spdVP.SetContent(strings.Join(m.spdLines(), "\n"))
	}
	if !m.detected {
		return
	}
	for i := range m.fields {
		v, ok := vals[m.fields[i].label]
		if !ok || v == "" {
			continue
		}
		oldHint := m.fields[i].hint
		m.fields[i].hint = v
		m.fields[i].input.Placeholder = v
		// Only follow detection while the value is still the untouched
		// fallback/applied value; never overwrite a user edit.
		if m.fields[i].input.Value() == oldHint {
			m.fields[i].input.SetValue(v)
		}
	}
	m.spdVP.SetContent(strings.Join(m.spdLines(), "\n"))
}

// SetIMC installs the IMC register dump shown in the IMC sub-view.
func (m *Model) SetIMC(timings []memory.IMCTimings, err error) {
	m.imc = timings
	if err != nil {
		m.imcNote = err.Error()
	} else {
		m.imcNote = ""
	}
	m.imcVP.SetContent(strings.Join(m.imcLines(), "\n"))
}

// Detected reports whether hardware detection produced values.
func (m *Model) Detected() bool { return m.detected }

// DetectMessage returns the message produced by the last detection attempt.
func (m *Model) DetectMessage() string { return m.detectMsg }

// Editing reports whether a field's text input owns letter keys.
func (m *Model) Editing() bool { return m.editing }

// ModalOpen reports whether a modal is open (the apply modal lands in Phase 2).
func (m *Model) ModalOpen() bool { return false }

// Running reports whether this tab has an active job (never for Timings).
func (m *Model) Running() bool { return false }

// Wants reports whether this tab owns runner events of the given kind.
func (m *Model) Wants(runner.Kind) bool { return false }

// Dirty reports whether the draft differs from the applied values.
func (m *Model) Dirty() bool {
	for i := range m.fields {
		if m.fields[i].input.Value() != m.baseline(m.fields[i].label) {
			return true
		}
	}
	return false
}

// Blur commits any in-progress edit.
func (m *Model) Blur() { m.commitEdit() }

// Stop cancels any running job (the Timings tab owns none).
func (m *Model) Stop() {}

// Title returns the tab title.
func (m *Model) Title() string { return "Timings" }

// SubViewTitle returns the active sub-view name for the tab bar/header.
func (m *Model) SubViewTitle() string { return m.subView.String() }

// Summary returns a short hardware summary for the header.
func (m *Model) Summary() string { return m.spdSummary() }

// Snapshot returns the current applied-plus-draft values as a config snapshot
// for the run tabs and the perf projection.
func (m *Model) Snapshot() config.Snapshot {
	vals := map[string]string{}
	for i := range m.fields {
		vals[m.fields[i].key] = m.fields[i].input.Value()
	}
	return config.Snapshot{Values: vals, Source: "draft"}
}

// baseline returns the value a field reverts to: the applied value, else the
// detected value, else the fallback.
func (m *Model) baseline(label string) string {
	if m.applied != nil {
		if v, ok := m.applied[label]; ok {
			return v
		}
	}
	for i := range m.fields {
		if m.fields[i].label == label {
			return m.fields[i].hint
		}
	}
	return ""
}

/* --------------- Field helpers --------------- */

func (m *Model) fieldIndex(label string) int {
	for i := range m.fields {
		if m.fields[i].label == label {
			return i
		}
	}
	return -1
}

// parseField returns the numeric value of a field, falling back to the
// detected value when the draft is empty or unknown.
func (m *Model) parseField(label string) (float64, bool) {
	i := m.fieldIndex(label)
	if i < 0 {
		return 0, false
	}
	if v, ok := parseNumber(m.fields[i].input.Value()); ok {
		return v, true
	}
	return parseNumber(m.fields[i].hint)
}

func (m *Model) detectedField(label string) float64 {
	i := m.fieldIndex(label)
	if i < 0 {
		return 0
	}
	v, _ := parseNumber(m.fields[i].hint)
	return v
}

func (m *Model) applyDetectedValues() map[string]string {
	out := map[string]string{}
	for i := range m.fields {
		out[m.fields[i].label] = m.fields[i].input.Value()
	}
	return out
}

// parseNumber extracts the leading numeric prefix of a value ("2T" -> 2,
// "1.35V" -> 1.35, "1600" -> 1600, "?" -> not ok).
func parseNumber(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "?" {
		return 0, false
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v, true
	}
	var b strings.Builder
	dot := false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' && !dot:
			b.WriteRune(r)
			dot = true
		default:
			goto done
		}
	}
done:
	if b.Len() == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(b.String(), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func parseUnsignedInt(s string) (int, bool) {
	v, ok := parseNumber(s)
	if !ok || v <= 0 {
		return 0, false
	}
	return int(v), true
}

// message is the status message a tab wants the root shell to show.
func message(text string, level ui.Level) tea.Cmd {
	return ui.Status(text, level)
}
