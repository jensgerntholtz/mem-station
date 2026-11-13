package sysinfo

import (
	"testing"

	"mem-station/internal/config"
	"mem-station/internal/store"

	tea "github.com/charmbracelet/bubbletea"
)

// newTestModel builds a Timings tab with an isolated state directory.
func newTestModel(t *testing.T) *Model {
	t.Helper()
	t.Setenv("MEM_STATION_STATE_DIR", t.TempDir())
	return New()
}

// TestParseNumber covers the value parser used for every numeric field.
func TestParseNumber(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"1600", 1600, true},
		{"2T", 2, true},
		{"1.35V", 1.35, true},
		{"?", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := parseNumber(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseNumber(%q) = %v,%v want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

// TestBoundsForFrequency checks the nanosecond-derived minimums and the fixed
// voltage window.
func TestBoundsForFrequency(t *testing.T) {
	m := newTestModel(t)
	m.fields[m.fieldIndex("DRAM Freq")].input.SetValue("3200")

	// tRFC >= 160 ns at 3200 MT/s = 3200*160/2000 = 256 cycles.
	if b := m.boundsFor("tRFC"); b.min != 256 {
		t.Errorf("tRFC min = %v, want 256", b.min)
	}
	// CAS minimum is max(5, 10 ns) = 16 cycles at 3200 MT/s.
	if b := m.boundsFor("CAS (tCL)"); b.min != 16 {
		t.Errorf("tCL min = %v, want 16", b.min)
	}
	v := m.boundsFor("DRAM Voltage")
	if v.min != 1.35 || v.max != 1.65 {
		t.Errorf("voltage bounds = %v-%v, want 1.35-1.65", v.min, v.max)
	}
}

// TestEditApplyRoundTrip checks draft editing, the dirty flag, and that apply
// persists the active profile without touching hardware.
func TestEditApplyRoundTrip(t *testing.T) {
	m := newTestModel(t)

	if m.Dirty() {
		t.Fatal("a fresh model must not be dirty")
	}

	// Edit the first field: select it, start editing, type a digit.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // commit

	if !m.Dirty() {
		t.Fatal("editing a field must mark the draft dirty")
	}

	if cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS}); cmd == nil {
		t.Fatal("apply must return a status command")
	}
	if m.Dirty() {
		t.Error("apply must clear the dirty flag")
	}

	p, ok, err := store.LoadActiveProfile()
	if err != nil || !ok {
		t.Fatalf("active profile missing after apply: ok=%v err=%v", ok, err)
	}
	if got := p.Values[config.KeyChannels]; got == "" {
		t.Errorf("applied profile has no channels value: %+v", p.Values)
	}
}

// TestCancelEditRestoresValue checks esc while editing only reverts the edit.
func TestCancelEditRestoresValue(t *testing.T) {
	m := newTestModel(t)
	before := m.fields[0].input.Value()

	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("9")})
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if got := m.fields[0].input.Value(); got != before {
		t.Errorf("esc while editing: value = %q, want %q", got, before)
	}
	if m.Dirty() {
		t.Error("cancelled edit must not leave the draft dirty")
	}
}

// TestSnapshotUsesCanonicalKeys checks the snapshot handed to the run tabs.
func TestSnapshotUsesCanonicalKeys(t *testing.T) {
	m := newTestModel(t)
	snap := m.Snapshot()
	for _, key := range []string{
		config.KeyChannels, config.KeyFreq, config.KeyCL, config.KeyRCD,
		config.KeyRP, config.KeyRAS, config.KeyCR, config.KeyRFC, config.KeyVoltage,
	} {
		if _, ok := snap.Values[key]; !ok {
			t.Errorf("snapshot missing key %q", key)
		}
	}
}
