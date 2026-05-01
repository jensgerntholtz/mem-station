package main

import "github.com/charmbracelet/lipgloss"

func (m model) renderSysInfoTab() string {
	leftWidth := (m.width - 8) * 2 / 3
	rightWidth := m.width - leftWidth - 8
	if leftWidth < 50 {
		leftWidth = 50
	}
	if rightWidth < 32 {
		rightWidth = 32
	}

	leftPanel := StylePanel.Width(leftWidth).Render(m.renderTimingEditor(leftWidth - 4))
	rightPanel := StylePanel.Width(rightWidth).Render(m.renderGuidePanel(rightWidth - 4))

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)

	parts := []string{topRow}

	fullWidth := m.width - 6
	if fullWidth < 60 {
		fullWidth = 60
	}

	// if m.spd != nil {
	// 	parts = append(parts, "", StyleInfoPanel.Width(fullWidth).Render(m.renderSPDInfoPanel(fullWidth-4)))
	// }

	if len(m.imcTimings) > 0 {
		parts = append(parts, "", StyleInfoPanel.Width(fullWidth).Render(m.renderIMCPanel()))
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
