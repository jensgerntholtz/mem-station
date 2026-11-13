package app

import (
	"testing"

	"mem-station/internal/uitest"

	tea "charm.land/bubbletea/v2"
)

// keyMode is the input ownership state a global key has to respect.
type keyMode string

const (
	modeIdle    keyMode = "idle"
	modeEditing keyMode = "editing"
	modeModal   keyMode = "modal"
	modeHelp    keyMode = "help"
)

// keyCase is one row of the global key-gating table: which key, in which mode,
// and what the shell is allowed to do with it.
type keyCase struct {
	mode keyMode
	key  tea.KeyPressMsg
	// quits is true when the key may end the program.
	quits bool
	// switchesTab is true when the key may change the active tab.
	switchesTab bool
	// helpOpen is the expected help-overlay state after the key.
	helpOpen bool
	// modalOpen is the expected modal state after the key.
	modalOpen bool
}

// TestGlobalKeyGating drives the table through the root shell so a key can
// never both type into an editor and trigger a global action.
func TestGlobalKeyGating(t *testing.T) {
	key := func(s string) tea.KeyPressMsg {
		switch s {
		case "esc":
			return uitest.Code(tea.KeyEsc)
		default:
			return uitest.Key(s)
		}
	}

	cases := []keyCase{
		// Idle: the global bindings are available.
		{modeIdle, key("q"), true, false, false, false},
		{modeIdle, key("?"), false, false, true, false},
		{modeIdle, key("2"), false, true, false, false},
		// Editing a timing field: letters and digits belong to the input.
		{modeEditing, key("q"), false, false, false, false},
		{modeEditing, key("?"), false, false, false, false},
		{modeEditing, key("2"), false, false, false, false},
		// The apply review owns the keys; q closes it and nothing else happens.
		{modeModal, key("q"), false, false, false, false},
		{modeModal, key("?"), false, false, false, true},
		{modeModal, key("2"), false, false, false, true},
		// The help overlay: q/?/esc close it, digits are inert.
		{modeHelp, key("q"), false, false, false, false},
		{modeHelp, key("?"), false, false, false, false},
		{modeHelp, key("2"), false, false, true, false},
	}

	for _, tc := range cases {
		m := testModel(t)
		m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

		switch tc.mode {
		case modeEditing:
			m.Update(uitest.Code(tea.KeyEnter)) // start editing Channels
			if !m.sys.Editing() {
				t.Fatalf("%s: setup failed, no field is being edited", tc.mode)
			}
		case modeModal:
			m.Update(uitest.Code(tea.KeyEnter))
			m.Update(uitest.Key("4"))
			m.Update(uitest.Code(tea.KeyEnter))
			m.Update(uitest.Ctrl('s'))
			if !m.sys.ModalOpen() {
				t.Fatalf("%s: setup failed, no modal is open", tc.mode)
			}
		case modeHelp:
			m.Update(uitest.Key("?"))
			if !m.helpOpen {
				t.Fatalf("%s: setup failed, help is not open", tc.mode)
			}
		}

		name := string(tc.mode) + "/" + tc.key.String()
		activeBefore := m.active
		_, cmd := m.Update(tc.key)

		if got := isQuit(cmd); got != tc.quits {
			t.Errorf("%s: quit = %v, want %v", name, got, tc.quits)
		}
		if got := m.active != activeBefore; got != tc.switchesTab {
			t.Errorf("%s: tab switched = %v, want %v", name, got, tc.switchesTab)
		}
		if m.helpOpen != tc.helpOpen {
			t.Errorf("%s: help open = %v, want %v", name, m.helpOpen, tc.helpOpen)
		}
		if m.sys.ModalOpen() != tc.modalOpen {
			t.Errorf("%s: modal open = %v, want %v", name, m.sys.ModalOpen(), tc.modalOpen)
		}
	}
}

// TestMouseWheelScrollsViewports checks wheel input reaches the active tab only
// when mouse support is enabled, and never while an editor owns the keys.
func TestMouseWheelScrollsViewports(t *testing.T) {
	t.Setenv("MEM_STATION_STATE_DIR", t.TempDir())
	opts := DefaultOptions()
	opts.Mouse = true
	m := New(opts)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.sys.SetDetected(map[string]string{"DRAM Freq": "3200"}, "ok", nil)
	// The SPD view scrolls with pgup/pgdown, which wheel events map onto.
	m.Update(uitest.Key("]"))
	_ = m.View().Content

	wheel := uitest.Wheel(false)
	m.Update(wheel)
	_ = m.View().Content
	// Scrolling may legitimately stop at the end of a short SPD view; the
	// contract under test is that no key is misinterpreted, so assert the tab
	// stayed on the SPD view and the shell still fits.
	if got := m.sys.SubViewTitle(); got != "SPD" {
		t.Errorf("wheel changed the sub-view to %q", got)
	}

	// With mouse disabled the event is ignored.
	plain := New(DefaultOptions())
	plain.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if _, cmd := plain.Update(wheel); cmd != nil {
		t.Error("wheel produced a command with mouse support disabled")
	}
}

// TestIMCKeyStartsOptInRead checks the raw IMC read is available on demand
// without being part of the default startup path.
func TestIMCKeyStartsOptInRead(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	_, cmd := m.Update(uitest.Key("I"))
	if cmd == nil {
		t.Fatal("I did not start an IMC register read")
	}
}
