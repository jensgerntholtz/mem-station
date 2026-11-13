package benchmark

import (
	"fmt"
	"strings"

	"mem-station/internal/runner"
	"mem-station/internal/style"
	"mem-station/internal/ui"

	"github.com/charmbracelet/lipgloss"
)

// View renders the tab into exactly w x h cells.
func (m *Model) View(w, h int) string {
	contentH := h - 3 // border 2 + title 1
	if contentH < 1 {
		contentH = 1
	}
	controls := m.controls(w)
	results := m.resultLines()

	used := len(strings.Split(controls, "\n"))
	if len(results) > 0 {
		used += 1 + len(results)
	}
	vpH := contentH - used - 1 // one blank line before the log
	if vpH < 2 {
		vpH = 2
	}
	m.resizeViewport(w-4, vpH)

	parts := []string{controls}
	if len(results) > 0 {
		parts = append(parts, "", strings.Join(results, "\n"))
	}
	parts = append(parts, "", m.vp.View())
	return ui.Panel("Benchmark", w, h, false, strings.Join(parts, "\n"))
}

func (m *Model) controls(w int) string {
	th := style.ActiveTheme()
	var button string
	switch {
	case m.Running():
		button = style.RunningBtn.Render(" Running ")
	case m.state == runner.StateIdle:
		button = style.RunBtn.Render(" Enter - Run Benchmark ")
	default:
		button = style.RunBtn.Render(" Enter - Run Again ")
	}

	elapsed := ui.DurationLabel(m.elapsed())
	total := ui.DurationLabel(m.duration)
	bar := ui.Progress(m.progress(), min(28, max(12, w/4)))
	statusLine := lipgloss.JoinHorizontal(lipgloss.Top,
		button,
		"  ",
		lipgloss.NewStyle().Foreground(th.Muted).Render(elapsed+" / "+total),
		"  ",
		lipgloss.NewStyle().Foreground(th.Accent).Render(bar),
		"  ",
		m.stateLabel(),
	)

	config := style.Hint.Render(fmt.Sprintf(
		"memrate 1 · 256M buffer · duration %s · workers 1", total))
	if s := m.snapshot; s.Values != nil {
		if peak := s.PeakMBs(); peak > 0 {
			config += style.Hint.Render(fmt.Sprintf("  ·  theoretical peak %.0f MB/s", peak))
		}
	}

	lines := []string{statusLine, config}
	if m.logPath != "" {
		lines = append(lines, style.Hint.Render("log: "+m.logPath))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) stateLabel() string {
	th := style.ActiveTheme()
	switch m.state {
	case runner.StateRunning:
		return lipgloss.NewStyle().Foreground(th.Warning).Render("running")
	case runner.StateDone:
		return lipgloss.NewStyle().Foreground(th.Success).Render("done")
	case runner.StateFailed:
		return lipgloss.NewStyle().Foreground(th.Danger).Render("failed (exit " + fmt.Sprint(m.exitCode) + ")")
	case runner.StateCancelled:
		return lipgloss.NewStyle().Foreground(th.Danger).Render("cancelled")
	default:
		return lipgloss.NewStyle().Foreground(th.Muted).Render("idle")
	}
}

// resultLines renders the parsed result table, or nil when there is no result.
func (m *Model) resultLines() []string {
	if m.results == nil {
		if m.state == runner.StateFailed && m.errText != "" {
			return []string{lipgloss.NewStyle().Foreground(style.ActiveTheme().Danger).Render("error: " + m.errText)}
		}
		return nil
	}
	r := m.results
	peak := m.peakMBs()

	header := style.ResultLabel.Render("Metric") +
		style.ResultValue.Render("Measured") +
		style.ResultTheo.Render("Peak") +
		style.ColHeader.Width(12).Render("Efficiency")

	lines := []string{
		style.SectionTitle.Render("Summary"),
		header,
		resultRow("Read (avg)", r.ReadRate, peak),
		resultRow("Write (avg)", r.WriteRate, peak),
	}
	if len(r.Metrics) > 0 {
		lines = append(lines, style.SectionTitle.Render("Rates (MB/s)"))
		for _, mt := range r.Metrics {
			lines = append(lines, resultRow(mt.Label, mt.Value, peak))
		}
	}
	return lines
}

func resultRow(label string, measured, peak float64) string {
	eff := 0.0
	if peak > 0 {
		eff = measured / peak * 100
	}
	effStr := fmt.Sprintf("%.1f%%", eff)
	styleEff := style.EffLow
	switch {
	case eff >= 70:
		styleEff = style.EffGood
	case eff >= 40:
		styleEff = style.EffOk
	}
	return style.ResultLabel.Render(label) +
		style.ResultValue.Render(fmt.Sprintf("%.0f", measured)) +
		style.ResultTheo.Render(fmt.Sprintf("%.0f", peak)) +
		styleEff.Width(12).Render(effStr)
}

func (m *Model) peakMBs() float64 { return m.snapshot.PeakMBs() }
