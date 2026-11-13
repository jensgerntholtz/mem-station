package burnin

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
	vpH := contentH - used - 1
	if vpH < 2 {
		vpH = 2
	}
	m.resizeViewport(w-4, vpH)

	parts := []string{controls}
	if len(results) > 0 {
		parts = append(parts, "", strings.Join(results, "\n"))
	}
	parts = append(parts, "", m.vp.View())
	return ui.Panel("Burn-in", w, h, false, strings.Join(parts, "\n"))
}

func (m *Model) controls(w int) string {
	th := style.ActiveTheme()
	var button string
	switch {
	case m.Running():
		button = style.RunningBtn.Render(" Running ")
	case m.state == runner.StateIdle:
		button = style.RunBtn.Render(" Enter - Run Burn-in ")
	default:
		button = style.RunBtn.Render(" Enter - Run Again ")
	}

	elapsed := ui.DurationLabel(m.elapsed())
	total := ui.DurationLabel(m.duration)
	bar := ui.Progress(m.progress(), min(28, max(12, w/4)))
	line := lipgloss.JoinHorizontal(lipgloss.Top,
		button, "  ",
		lipgloss.NewStyle().Foreground(th.Muted).Render(elapsed+" / "+total), "  ",
		lipgloss.NewStyle().Foreground(th.Accent).Render(bar), "  ",
		m.stateLabel(),
	)

	lines := []string{line}
	if m.presetSource != "" || m.jobPath != "" {
		src := m.presetSource
		if src == "" {
			src = m.jobPath
		}
		lines = append(lines, style.Hint.Render("preset: "+src))
	}
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
		return lipgloss.NewStyle().Foreground(th.Success).Render("passed")
	case runner.StateFailed:
		return lipgloss.NewStyle().Foreground(th.Danger).Render("failed")
	case runner.StateCancelled:
		return lipgloss.NewStyle().Foreground(th.Danger).Render("cancelled")
	default:
		return lipgloss.NewStyle().Foreground(th.Muted).Render("idle")
	}
}

// resultLines renders the burn-in verdict, or nil when there is no result.
func (m *Model) resultLines() []string {
	th := style.ActiveTheme()
	r := m.results
	if r == nil {
		if m.state == runner.StateFailed && m.errText != "" {
			return []string{lipgloss.NewStyle().Foreground(th.Danger).Render("error: " + m.errText)}
		}
		return nil
	}

	verdict := lipgloss.NewStyle().Bold(true).Foreground(th.Danger).Render("FAIL")
	if r.Success {
		verdict = lipgloss.NewStyle().Bold(true).Foreground(th.Success).Render("PASS")
	}

	label := func(s string) string { return style.ResultLabel.Render(s) }
	value := func(s string) string { return style.ResultValue.Render(s) }

	return []string{
		style.SectionTitle.Render("Result"),
		label("Verdict") + value(" ") + verdict,
		label("Duration") + value(r.Duration),
		label("Error lines") + value(fmt.Sprint(r.Errors)),
		label("Exit code") + value(fmt.Sprint(r.ExitCode)),
	}
}
