package app

import (
	"reflect"
	"strings"
	"testing"

	"mem-station/internal/memory"
	"mem-station/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// testModel builds a root model with detection stubbed out and an isolated
// state directory, so tests never touch the real profile store.
func testModel(t *testing.T) *Model {
	t.Helper()
	t.Setenv("MEM_STATION_STATE_DIR", t.TempDir())

	opts := DefaultOptions()
	opts.Detect = func() (map[string]string, string, *memory.SPDInfo) {
		return map[string]string{
				"Channels":     "2",
				"DRAM Ratio":   "16",
				"DRAM Freq":    "3200",
				"CAS (tCL)":    "16",
				"tRCD":         "18",
				"tRP":          "18",
				"tRAS":         "38",
				"Command Rate": "1T",
				"tRFC":         "350",
				"DRAM Voltage": "1.35V",
			}, "Detected 2 channels at 3200 MT/s",
			&memory.SPDInfo{
				PartNumber:   "F4-3200C16-16GVK",
				SizeMB:       "16384",
				ModuleSpeed:  "3200 MT/s",
				Ranks:        "2",
				TimingString: "16-18-18-38",
			}
	}
	return New(opts)
}

// sizeModel applies a window size and returns the rendered view.
func sizeModel(m *Model, w, h int) string {
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m.View()
}

// TestViewIsExactlyTerminalSize guards the fixed-footer contract: the shell
// must fill the terminal exactly, never scrolling the footer off screen.
func TestViewIsExactlyTerminalSize(t *testing.T) {
	sizes := []struct{ w, h int }{
		{80, 24}, {100, 30}, {110, 32}, {120, 40}, {160, 50},
	}
	for _, size := range sizes {
		m := testModel(t)
		out := sizeModel(m, size.w, size.h)

		if got := lipgloss.Height(out); got != size.h {
			t.Errorf("%dx%d: height = %d, want %d", size.w, size.h, got, size.h)
		}
		if got := lipgloss.Width(out); got != size.w {
			t.Errorf("%dx%d: width = %d, want %d", size.w, size.h, got, size.w)
		}
		if strings.Contains(out, "Terminal too small") {
			t.Errorf("%dx%d: unexpected too-small screen", size.w, size.h)
		}
	}
}

// TestTooSmallScreen checks the <80x24 fallback still fills the terminal.
func TestTooSmallScreen(t *testing.T) {
	m := testModel(t)
	out := sizeModel(m, 70, 20)
	if !strings.Contains(out, "Terminal too small") {
		t.Fatalf("expected too-small notice, got:\n%s", out)
	}
	if got := lipgloss.Height(out); got != 20 {
		t.Errorf("height = %d, want 20", got)
	}
	if got := lipgloss.Width(out); got != 70 {
		t.Errorf("width = %d, want 70", got)
	}
}

// TestHelpOverlayFillsBody checks "?" replaces the body at the same size.
func TestHelpOverlayFillsBody(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	out := m.View()
	if !strings.Contains(out, "Global") {
		t.Fatalf("help overlay missing global section:\n%s", out)
	}
	if got := lipgloss.Height(out); got != 30 {
		t.Errorf("height = %d, want 30", got)
	}
	if got := lipgloss.Width(out); got != 100 {
		t.Errorf("width = %d, want 100", got)
	}
}

// TestTwoColumnBreakpoint checks the Timings tab switches layouts at 110 body
// columns: side-by-side above, stacked below.
func TestTwoColumnBreakpoint(t *testing.T) {
	narrow := testModel(t)
	outNarrow := sizeModel(narrow, 100, 32)
	if !strings.Contains(outNarrow, "Guidance") {
		t.Fatalf("narrow layout missing guidance panel:\n%s", outNarrow)
	}

	wide := testModel(t)
	outWide := sizeModel(wide, 120, 40)
	sideBySide := false
	for _, line := range strings.Split(outWide, "\n") {
		if strings.Contains(line, "Timings") && strings.Contains(line, "Guidance") {
			sideBySide = true
			break
		}
	}
	if !sideBySide {
		t.Errorf("wide layout is not side-by-side:\n%s", outWide)
	}
}

// TestTabBarAndFooter checks the tab bar and footer are the first and last
// body rows.
func TestTabBarAndFooter(t *testing.T) {
	m := testModel(t)
	out := sizeModel(m, 100, 30)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if !strings.Contains(lines[1], "Timings") || !strings.Contains(lines[1], "Burn-in") {
		t.Errorf("row 2 is not the tab bar: %q", lines[1])
	}
	footer := lines[len(lines)-1]
	if !strings.Contains(footer, "apply") {
		t.Errorf("last row is not the footer: %q", footer)
	}
}

// TestAllTabsAreExactSize checks each tab, including sub-views, fills the body
// exactly so the footer never moves.
func TestAllTabsAreExactSize(t *testing.T) {
	for _, tab := range []int{0, 1, 2} {
		m := testModel(t)
		m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
		m.switchTab(tab)
		out := m.View()
		if got := lipgloss.Height(out); got != 30 {
			t.Errorf("tab %d: height = %d, want 30", tab, got)
		}
		if got := lipgloss.Width(out); got != 100 {
			t.Errorf("tab %d: width = %d, want 100", tab, got)
		}
	}
}

// TestTimingsSubViewsAreExactSize checks the SPD and IMC sub-views.
func TestTimingsSubViewsAreExactSize(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	for i := 0; i < 2; i++ {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
		out := m.View()
		if got := lipgloss.Height(out); got != 30 {
			t.Errorf("sub-view %d: height = %d, want 30", i+1, got)
		}
		if got := lipgloss.Width(out); got != 100 {
			t.Errorf("sub-view %d: width = %d, want 100", i+1, got)
		}
	}
}

// TestEditingOwnsLetterKeys guards the Phase 0 fix: while a field is being
// edited, letters type instead of triggering global actions, and digits do not
// switch tabs.
func TestEditingOwnsLetterKeys(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // start editing Channels

	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); isQuit(cmd) {
		t.Errorf("typing q while editing quit the program")
	}
	if m.active != 0 {
		t.Errorf("typing q while editing switched tab to %d", m.active)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	if m.active != 0 {
		t.Errorf("digit while editing switched tab to %d", m.active)
	}
	if out := m.View(); !strings.Contains(out, "2q") {
		t.Errorf("typed characters missing from the draft field")
	}
}

// TestFooterKeepsStatus checks the footer keeps the status message visible at
// the minimum terminal width.
func TestFooterKeepsStatus(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.setStatus("Burn-in failed: 12 error lines", ui.LevelError)
	out := m.View()

	lines := strings.Split(out, "\n")
	footer := ""
	for _, line := range lines {
		if strings.Contains(line, "Burn-in failed") {
			footer = line
		}
	}
	if footer == "" {
		t.Fatalf("status text missing from the footer:\n%s", out)
	}
	if got := lipgloss.Width(footer); got != 80 {
		t.Errorf("footer width = %d, want 80", got)
	}
}

// isQuit reports whether cmd is tea.Quit.
func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	return reflect.ValueOf(cmd).Pointer() == reflect.ValueOf(tea.Quit).Pointer()
}

// TestQuitGating checks q quits only when no editor or overlay owns the keys.
func TestQuitGating(t *testing.T) {
	m := testModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")}); isQuit(cmd) {
		t.Fatal("help toggle quit the program")
	}
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); isQuit(cmd) {
		t.Error("q with help open should close help, not quit")
	}
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); !isQuit(cmd) {
		t.Error("q with no overlay should quit")
	}
}
