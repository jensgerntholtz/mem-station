package benchmark

import (
	"fmt"
	"strings"
	"time"

	"mem-station/internal/runner"
	"mem-station/internal/style"
	"mem-station/internal/ui"

	"github.com/charmbracelet/lipgloss"
)

// resultColumnWidths are the aligned column widths of the result table.
var resultColumnWidths = []int{12, 9, 9, 8, 9, 6}

// historyColumnWidths are the aligned column widths of the history table.
var historyColumnWidths = []int{12, 8, 8, 6, 22}

// maxResultMetrics bounds the metric rows so the log keeps room at 80x24.
const maxResultMetrics = 4

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
	history := m.historyLines(w)

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
	return ui.Panel("Benchmark", w, h, false, strings.Join(parts, "\n"))
}

// logView renders the live output viewport with its follow marker.
func (m *Model) logView() string {
	follow := "follow: off"
	if m.follow {
		follow = "follow: on"
	}
	title := style.Muted().Render("Live log (" + follow + ")")
	return title + "\n" + m.vp.View()
}

func (m *Model) controls(w int) []string {
	lines := []string{m.configLine()}

	var button string
	switch {
	case m.Running():
		button = style.RunningButton().Render(" Running ")
	case m.state == runner.StateIdle:
		button = style.RunButton().Render(" Enter - Run Benchmark ")
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
		lipgloss.NewStyle().Foreground(style.ActiveTheme().Accent).Render(bar), "  ",
		state,
	))
	if m.logPath != "" {
		lines = append(lines, style.Muted().Render("log: "+m.logPath))
	}
	if m.historyErr != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(style.ActiveTheme().Warning).Render(m.historyErr))
	}
	return lines
}

// configLine renders the editable duration/buffer/workers row.
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
		field("Duration", durationValue, m.row == rowDuration),
		"  ",
		field("Buffer", buffers[m.bufferIndex], m.row == rowBuffer),
		"  ",
		field("Workers", fmt.Sprint(m.workers), m.row == rowWorkers),
	)
}

func (m *Model) stateLabel() string {
	switch m.state {
	case runner.StateRunning:
		return style.Warning().Render("running")
	case runner.StateDone:
		return style.Success().Render(ui.MarkOK + " done")
	case runner.StateFailed:
		return style.Danger().Render(ui.MarkError + " failed (exit " + fmt.Sprint(m.exitCode) + ")")
	case runner.StateCancelled:
		return style.Danger().Render(ui.MarkWarn + " cancelled")
	default:
		return style.Muted().Render("idle")
	}
}

// resultLines renders the parsed result table (last vs previous run) or the
// failure reason. It returns nil when there is nothing to show.
func (m *Model) resultLines(w int) []string {
	if m.startedAt.IsZero() || m.Running() {
		return nil
	}
	if m.results == nil || !NewPayload(m.runOptions, m.results).Parsed {
		msg := m.errText
		if msg == "" {
			msg = m.parseMsg
		}
		if msg == "" {
			return nil
		}
		st := style.Danger()
		if m.state == runner.StateCancelled {
			st = style.Warning()
		}
		return []string{st.Render("error: " + ui.Truncate(msg, max(8, w-8), true))}
	}

	opts := m.runOptions
	prev, hasPrev := m.previous(opts.Signature(), m.lastID)
	peak := m.snapshot.PeakMBs()

	header := style.ColumnHeader().Render
	rows := [][]string{{
		header("Metric"), header("Last"), header("Prev"), header("Delta"), header("Peak"), header("Eff"),
	}}
	rows = append(rows, resultRow("Read (avg)", m.results.ReadRate, prev.ReadMBs, hasPrev, peak))
	rows = append(rows, resultRow("Write (avg)", m.results.WriteRate, prev.WriteMBs, hasPrev, peak))
	for i, mt := range m.results.Metrics {
		if i >= maxResultMetrics {
			break
		}
		var prevVal float64
		for _, pm := range prev.Metrics {
			if pm.Label == mt.Label {
				prevVal = pm.MBs
				break
			}
		}
		rows = append(rows, resultRow(mt.Label, mt.Value, prevVal, hasPrev, peak))
	}

	out := []string{style.Section().Render("Results · " + opts.Label())}
	out = append(out, strings.Split(ui.JoinColumns(rows, resultColumnWidths), "\n")...)
	return out
}

// resultRow renders one measured metric against the previous run and the
// theoretical peak.
func resultRow(label string, last, prev float64, hasPrev bool, peak float64) []string {
	th := style.ActiveTheme()
	muted := style.Muted().Render
	prevStr := "-"
	deltaStr := "-"
	if hasPrev && prev > 0 {
		prevStr = fmt.Sprintf("%.0f", prev)
		delta := (last - prev) / prev * 100
		st := style.Success()
		if delta < 0 {
			st = style.Danger()
		}
		deltaStr = st.Render(fmt.Sprintf("%+.1f%%", delta))
	}
	eff := "-"
	if peak > 0 {
		eff = fmt.Sprintf("%.1f%%", last/peak*100)
	}
	return []string{
		lipgloss.NewStyle().Foreground(th.Label).Render(label),
		lipgloss.NewStyle().Foreground(th.Value).Render(fmt.Sprintf("%.0f", last)),
		muted(prevStr),
		deltaStr,
		muted(fmt.Sprintf("%.0f", peak)),
		muted(eff),
	}
}

// historyLines renders the recent-run table, newest first.
func (m *Model) historyLines(w int) []string {
	if len(m.runs) == 0 {
		return nil
	}
	th := style.ActiveTheme()
	rows := [][]string{{
		style.ColumnHeader().Render("When"),
		style.ColumnHeader().Render("Read"),
		style.ColumnHeader().Render("Write"),
		style.ColumnHeader().Render("Dur"),
		style.ColumnHeader().Render("Config"),
	}}
	shown := 0
	for _, rec := range m.runs {
		if shown >= maxHistoryRows {
			break
		}
		shown++
		read, write := "-", "-"
		if rec.Result.Parsed {
			read = fmt.Sprintf("%.0f", rec.Result.ReadMBs)
			write = fmt.Sprintf("%.0f", rec.Result.WriteMBs)
		}
		dur := ui.DurationLabel(time.Duration(rec.Record.DurationMS) * time.Millisecond)
		rows = append(rows, []string{
			style.Muted().Render(rec.Record.StartedAt.Local().Format("01-02 15:04")),
			lipgloss.NewStyle().Foreground(th.Value).Render(read),
			lipgloss.NewStyle().Foreground(th.Value).Render(write),
			style.Muted().Render(dur),
			lipgloss.NewStyle().Foreground(th.Label).Render(rec.Result.optionsFor().Label()),
		})
	}
	out := []string{style.Section().Render(fmt.Sprintf("History (last %d, newest first)", len(m.runs)))}
	out = append(out, strings.Split(ui.JoinColumns(rows, historyColumnWidths), "\n")...)
	return out
}
