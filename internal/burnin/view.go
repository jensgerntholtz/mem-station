package burnin

import (
	"fmt"
	"strings"
	"time"

	"mem-station/internal/runner"
	"mem-station/internal/style"
	"mem-station/internal/ui"

	"charm.land/lipgloss/v2"
)

// historyColumnWidths are the aligned column widths of the history table.
var historyColumnWidths = []int{12, 9, 5, 6, 24}

// maxHistoryRows bounds the history rows shown above the log.
const maxHistoryRows = 4

// View renders the tab into exactly w x h cells.
func (m *Model) View(w, h int) string {
	contentH := h - 3 // border 2 + title 1
	if contentH < 1 {
		contentH = 1
	}

	controls := m.controls(w)
	results := m.resultLines(w)
	history := m.historyLines()

	// The fixed block is the controls plus the blank line and the live-log
	// title above the viewport. History only appears when the remaining space
	// can hold it above a readable log.
	fixed := len(controls) + 2
	if len(results) > 0 {
		fixed += 1 + len(results)
	}
	vpH := contentH - fixed
	histUsed := 0
	if len(history) > 0 && vpH-len(history)-1 >= 3 {
		histUsed = len(history) + 1
		vpH -= histUsed
	}
	if vpH < 3 {
		vpH = 3
	}
	m.resizeViewport(w-4, vpH)

	parts := []string{strings.Join(controls, "\n")}
	if len(results) > 0 {
		parts = append(parts, "", strings.Join(results, "\n"))
	}
	parts = append(parts, "", m.logView())
	if histUsed > 0 {
		parts = append(parts, "", strings.Join(history, "\n"))
	}
	return ui.Panel("Burn-in", w, h, false, strings.Join(parts, "\n"))
}

// logView renders the live output viewport with its follow/filter markers.
func (m *Model) logView() string {
	filter := "filter: all"
	if m.errorsOnly {
		filter = "filter: errors"
	}
	follow := "follow: off"
	if m.follow {
		follow = "follow: on"
	}
	title := style.Muted().Render("Live log (" + follow + ", " + filter + ")")
	return title + "\n" + m.vp.View()
}

func (m *Model) controls(w int) []string {
	lines := []string{m.configLine()}

	var button string
	switch {
	case m.Running():
		button = style.RunningButton().Render(" Running ")
	case m.state == runner.StateIdle:
		button = style.RunButton().Render(" Enter - Run Burn-in ")
	default:
		button = style.RunButton().Render(" Enter - Run Again ")
	}

	elapsed := ui.DurationLabel(m.elapsed())
	total := ui.DurationLabel(m.options().Duration)
	if m.runOptions.Duration > 0 {
		total = ui.DurationLabel(m.runOptions.Duration)
	}
	bar := ui.Progress(m.progress(), min(28, max(10, w/4)))
	state := m.stateLabel()
	if m.Running() {
		state = ui.SpinnerFrame(m.now()) + " " + state
	}
	lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top,
		button, "  ",
		style.Muted().Render(elapsed+" / "+total), "  ",
		style.Accent().Render(bar), "  ",
		state,
	))

	if m.errors > 0 {
		lines = append(lines, m.errorBanner())
	}
	if m.jobPath != "" && m.Running() {
		lines = append(lines, style.Muted().Render("job: "+m.jobPath))
	}
	if m.logPath != "" && m.Running() {
		lines = append(lines, style.Muted().Render("log: "+m.logPath))
	}
	if m.presetWarn != "" {
		lines = append(lines, style.Warning().Render(m.presetWarn))
	}
	if m.historyErr != "" {
		lines = append(lines, style.Warning().Render(m.historyErr))
	}
	return lines
}

// errorBanner is the prominent error line shown as soon as errors appear.
func (m *Model) errorBanner() string {
	th := style.ActiveTheme()
	text := fmt.Sprintf("%s %d error line(s)", ui.MarkError, m.errors)
	if m.firstError != "" {
		text += " · first: " + m.firstError
	}
	return lipgloss.NewStyle().Bold(true).Foreground(th.Danger).Render(text)
}

// configLine renders the editable preset/duration/stop-on-error row.
func (m *Model) configLine() string {
	th := style.ActiveTheme()
	field := func(label, value string, selected bool) string {
		l := style.Muted().Render(label + " ")
		if selected {
			return l + lipgloss.NewStyle().Bold(true).Foreground(th.Accent).Render("[ "+value+" ]")
		}
		return l + lipgloss.NewStyle().Foreground(th.Value).Render("["+value+"]")
	}

	durationValue := durationLabel(m.durationIndex, m.custom)
	if m.editing {
		durationValue = m.input.View()
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		field("Preset", m.preset().Name, m.row == rowPreset),
		"  ",
		field("Duration", durationValue, m.row == rowDuration),
		"  ",
		field("Stop on error", onOff(m.stopOnError), m.row == rowStopOnError),
	)
}

func (m *Model) stateLabel() string {
	switch m.state {
	case runner.StateRunning:
		return style.Warning().Render("running")
	case runner.StateDone:
		return style.Success().Render(ui.MarkOK + " passed")
	case runner.StateFailed:
		return style.Danger().Render(ui.MarkError + " failed")
	case runner.StateCancelled:
		return style.Danger().Render(ui.MarkWarn + " cancelled")
	default:
		return style.Muted().Render("idle")
	}
}

// resultLines renders the verdict summary after a terminal state.
func (m *Model) resultLines(w int) []string {
	th := style.ActiveTheme()
	if m.startedAt.IsZero() || m.Running() || m.results == nil {
		return nil
	}
	verdict := VerdictFailed
	verdictStyle := lipgloss.NewStyle().Bold(true).Foreground(th.Danger).Render("FAIL")
	switch m.state {
	case runner.StateDone:
		verdict = VerdictPassed
		verdictStyle = lipgloss.NewStyle().Bold(true).Foreground(th.Success).Render("PASS")
	case runner.StateCancelled:
		verdict = VerdictCancelled
		verdictStyle = lipgloss.NewStyle().Bold(true).Foreground(th.Warning).Render("CANCELLED")
	}

	label := func(s string) string { return style.Muted().Render(s + " ") }
	value := func(s string) string { return lipgloss.NewStyle().Foreground(th.Value).Render(s) }

	summary := lipgloss.JoinHorizontal(lipgloss.Top,
		label("Verdict"), verdictStyle, "   ",
		label("Elapsed"), value(ui.DurationLabel(m.elapsed())), "   ",
		label("Errors"), value(fmt.Sprint(m.errors)), "   ",
		label("Exit"), value(fmt.Sprint(m.exitCode)),
	)
	lines := []string{
		style.Section().Render("Result · " + verdict),
		ui.Truncate(summary, max(20, w-4), true),
	}
	if m.errText != "" && m.state != runner.StateDone {
		st := style.Danger()
		if m.state == runner.StateCancelled {
			st = style.Warning()
		}
		lines = append(lines, st.Render("reason: "+m.errText))
	}
	if m.logPath != "" {
		lines = append(lines, style.Muted().Render("log: "+m.logPath+"  (full run: view the log file)"))
	}
	return lines
}

// historyLines renders the recent-run table, newest first.
func (m *Model) historyLines() []string {
	if len(m.runs) == 0 {
		return nil
	}
	th := style.ActiveTheme()
	rows := [][]string{{
		style.ColumnHeader().Render("When"),
		style.ColumnHeader().Render("Verdict"),
		style.ColumnHeader().Render("Err"),
		style.ColumnHeader().Render("Dur"),
		style.ColumnHeader().Render("Config"),
	}}
	shown := 0
	for _, rec := range m.runs {
		if shown >= maxHistoryRows {
			break
		}
		shown++
		verdict := rec.Result.Verdict
		if verdict == "" {
			verdict = "-"
		}
		st := style.Muted()
		switch verdict {
		case VerdictPassed:
			st = style.Success()
		case VerdictFailed:
			st = style.Danger()
		case VerdictCancelled:
			st = style.Warning()
		}
		opts := rec.Result.Options.options()
		config := ui.DurationLabel(opts.Duration)
		if name := rec.Result.Options.PresetName; name != "" {
			config += " · " + name
		}
		rows = append(rows, []string{
			style.Muted().Render(rec.Record.StartedAt.Local().Format("01-02 15:04")),
			st.Render(verdict),
			lipgloss.NewStyle().Foreground(th.Value).Render(fmt.Sprint(rec.Result.Errors)),
			style.Muted().Render(ui.DurationLabel(time.Duration(rec.Record.DurationMS) * time.Millisecond)),
			lipgloss.NewStyle().Foreground(th.Label).Render(config),
		})
	}
	out := []string{style.Section().Render(fmt.Sprintf("History (last %d, newest first)", len(m.runs)))}
	out = append(out, strings.Split(ui.JoinColumns(rows, historyColumnWidths), "\n")...)
	return out
}
