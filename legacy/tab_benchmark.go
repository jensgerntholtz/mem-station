package main

import (
	"strings"
)

func (m model) renderBenchmarkTab() string {
	fullWidth := m.width - 8
	if fullWidth < 60 {
		fullWidth = 60
	}

	var sections []string

	if m.benchRunning {
		sections = append(sections, StyleRunningBtn.Render(" Running... (30s) "))
	} else {
		sections = append(sections, StyleRunBtn.Render(" Enter \u2014 Run Benchmark "))
	}
	sections = append(sections, "")

	if m.benchResults == nil {
		sections = append(sections,
			StyleHint.Render("Run a stress-ng memrate benchmark to measure actual memory throughput."),
			StyleHint.Render("Results will be compared against theoretical peak bandwidth."),
			"",
			StyleHint.Render("Command: stress-ng --memrate 1 --memrate-bytes 256M -t 30 --metrics"),
		)
	} else {
		sections = append(sections, m.renderBenchResults()...)
	}

	return StylePanel.Width(fullWidth).Render(strings.Join(sections, "\n"))
}
