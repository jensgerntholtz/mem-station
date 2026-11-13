package app

import (
	"fmt"
	"strings"

	"mem-station/internal/style"
	"mem-station/internal/ui"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// View renders the whole shell: header, tab bar, body, footer. The result is
// exactly the terminal size minus nothing - the body absorbs all slack.
//
// Bubble Tea v2 takes terminal modes from the view instead of program options,
// so the alternate screen, the window title, and mouse capture are declared
// here rather than at Program construction.
func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "MemStation"
	if m.opts.Mouse {
		v.MouseMode = tea.MouseModeCellMotion
	}
	return v
}

// render builds the view content. Tests compare it against golden files, and
// View wraps it with the terminal modes above.
func (m *Model) render() string {
	if m.tooSmall() {
		return m.tooSmallView()
	}

	f := m.frame()
	parts := []string{
		m.renderHeader(f),
		m.renderTabBar(f),
	}

	if m.helpOpen {
		parts = append(parts, ui.HelpOverlayAt(f.contentW, f.bodyH, m.HelpSections(), m.helpOffset))
	} else {
		parts = append(parts, m.tabs[m.active].View(f.contentW, f.bodyH))
	}

	parts = append(parts, ui.Footer(m.status, m.level, m.tabs[m.active].Keys(), f.contentW))

	body := lipgloss.JoinVertical(lipgloss.Left, parts...)
	body = ui.ClipLines(body, f.height)
	return lipgloss.NewStyle().Padding(0, 1).Render(body)
}

// renderHeader draws the title, host summary, and dirty/running markers.
func (m *Model) renderHeader(f frame) string {
	th := style.ActiveTheme()

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(th.TabActiveFg).
		Background(th.TabActiveBg).
		Padding(0, 1).
		Render("MemStation")
	version := style.Muted().Render(m.opts.Version)

	segments := []string{title, version}

	if s := m.sys.Summary(); s != "" {
		segments = append(segments, style.Info().Render(s))
	}
	if m.sys.Dirty() {
		segments = append(segments, style.Warning().Render("* unsaved changes"))
	}
	for _, t := range m.tabs {
		if t.Running() {
			label := "* " + strings.ToLower(t.Title()) + " running"
			segments = append(segments, style.Success().Render(label))
		}
	}

	line := lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(segments, "  "))
	return ui.Truncate(line, f.contentW, true)
}

// renderTabBar draws the numbered tab bar with active state and markers.
func (m *Model) renderTabBar(f frame) string {
	tabs := make([]string, 0, len(m.tabs))
	for i, t := range m.tabs {
		label := fmt.Sprintf("%d %s", i+1, t.Title())
		if t.Running() {
			label += " *"
		} else if t.Dirty() {
			label += " ~"
		}
		if i == m.active {
			tabs = append(tabs, style.TabActive().Render(label))
		} else {
			tabs = append(tabs, style.TabInactive().Render(label))
		}
	}
	line := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	return ui.Truncate(line, f.contentW, false)
}

// tooSmallView replaces the shell when the terminal cannot host it.
func (m *Model) tooSmallView() string {
	th := style.ActiveTheme()
	w, h := m.size()

	message := lipgloss.JoinVertical(lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Foreground(th.Accent).Render("MemStation"),
		"",
		style.Warning().Render(fmt.Sprintf("Terminal too small: %dx%d", w, h)),
		style.Muted().Render(fmt.Sprintf("Minimum size is %dx%d", minWidth, minHeight)),
		"",
		style.Dim().Render("Resize the window, or press q to quit."),
	)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, message)
}
