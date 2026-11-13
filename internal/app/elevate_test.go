package app

import (
	"errors"
	"strings"
	"testing"

	"mem-station/internal/memory"
	"mem-station/internal/ui"
	"mem-station/internal/uitest"

	tea "charm.land/bubbletea/v2"
)

// TestHandleElevationAppliesResult checks that the privileged `info -json`
// payload lands in the Timings tab and the header summary.
func TestHandleElevationAppliesResult(t *testing.T) {
	m := testModel(t)

	msg := elevateMsg{doc: elevationDocument{
		Values: map[string]string{
			"Channels":  "2",
			"DRAM Freq": "3200",
			"CAS (tCL)": "16",
		},
		Message: "Detected 3 params",
		SPD: &memory.SPDInfo{
			PartNumber:  "F4-3200C16-16GVK",
			SizeMB:      "8192 MB",
			ModuleSpeed: "3200 MT/s",
		},
		IMC: []memory.IMCTimings{{Channel: 0, TRAS: 32}},
	}}

	if cmd := m.handleElevation(msg); cmd == nil {
		t.Fatal("successful elevation should return a status command")
	}
	if m.level != ui.LevelInfo {
		t.Errorf("status level = %v, want info", m.level)
	}
	if !strings.Contains(m.status, "Elevated detection: 3 values") {
		t.Errorf("status = %q, want an elevated-detection summary", m.status)
	}
	if !strings.Contains(m.status, "IMC ok") {
		t.Errorf("status = %q, want IMC availability", m.status)
	}
	if !m.sys.Detected() {
		t.Error("Timings tab did not receive the elevated values")
	}

	summary := m.sys.Summary()
	if !strings.Contains(summary, "8192 MB") {
		t.Errorf("header summary = %q, want the module size", summary)
	}
	if strings.Contains(summary, "MB MB") {
		t.Errorf("header summary = %q, duplicated size unit", summary)
	}
}

// TestHandleElevationFailure checks that a failed sudo run is reported as an
// error instead of silently doing nothing.
func TestHandleElevationFailure(t *testing.T) {
	m := testModel(t)

	m.handleElevation(elevateMsg{err: errors.New("exit status 1")})

	if m.level != ui.LevelError {
		t.Errorf("status level = %v, want error", m.level)
	}
	if !strings.Contains(m.status, "Elevation cancelled or failed") {
		t.Errorf("status = %q, want an elevation failure message", m.status)
	}
	if m.sys.Detected() {
		t.Error("a failed elevation must not claim detected values")
	}
}

// TestDetectionFailureMentionsElevation checks the unprivileged hint names the
// key that fixes it.
func TestDetectionFailureMentionsElevation(t *testing.T) {
	if isRoot() {
		t.Skip("running as root: detection has nothing to elevate")
	}
	m := testModel(t)
	m.detectionFailed("Auto-detect failed")

	if m.level != ui.LevelError {
		t.Errorf("status level = %v, want error", m.level)
	}
	if !strings.Contains(m.status, "press E") || !strings.Contains(m.status, "sudo") {
		t.Errorf("status = %q, want an elevate-with-sudo hint", m.status)
	}
}

// TestElevateKeyStartsElevation checks that E returns the terminal-handoff
// command. The command is never executed here: it would run sudo.
func TestElevateKeyStartsElevation(t *testing.T) {
	if isRoot() {
		t.Skip("running as root: no elevation path")
	}
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	_, cmd := m.Update(uitest.Key("E"))
	if cmd == nil {
		t.Fatal("E did not start an elevated detection run")
	}
}

// TestElevateKeyIsIgnoredWhileEditing checks the editing gate: E must not
// start a sudo run while a numeric field owns the keyboard.
func TestElevateKeyIsIgnoredWhileEditing(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.Update(uitest.Code(tea.KeyEnter)) // start editing the first field

	_, cmd := m.Update(uitest.Key("E"))
	if cmd != nil {
		t.Error("E started elevation while a field was being edited")
	}
	if strings.Contains(m.View().Content, "2E") {
		t.Error("numeric field accepted a letter")
	}
}
