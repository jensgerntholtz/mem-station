package sysinfo

import (
	"fmt"
	"strings"

	"mem-station/internal/style"
	"mem-station/internal/ui"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

const (
	// twoColumnMin is the body width at which the Timings sub-view switches
	// from stacked panels to side-by-side editor + guidance.
	twoColumnMin = 110

	labelColW    = 20
	detectedColW = 10
	draftColW    = 10
)

// View renders the tab into exactly w x h cells.
func (m *Model) View(w, h int) string {
	if w < 1 || h < 1 {
		return ""
	}
	switch m.subView {
	case subSPD:
		return m.viewReadOnly(w, h, "SPD Information", &m.spdVP, m.spdLines())
	case subIMC:
		return m.viewReadOnly(w, h, "IMC Timings", &m.imcVP, m.imcLines())
	default:
		return m.viewTimings(w, h)
	}
}

func (m *Model) viewTimings(w, h int) string {
	if w >= twoColumnMin {
		leftW := w * 62 / 100
		rightW := w - 2 - leftW
		if rightW < 34 {
			rightW = 34
			leftW = w - 2 - rightW
		}
		if leftW < 56 {
			leftW = 56
			rightW = w - 2 - leftW
		}
		left := ui.Panel("Timings", leftW, h, true, m.editorBody(leftW-4))
		right := ui.Panel("Guidance", rightW, h, false, m.guideBody(rightW-4))
		return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
	}

	topH := h * 64 / 100
	if topH < 12 || h-topH-1 < 6 {
		// Not enough vertical room for two stacked panels.
		topH = h
	}
	top := ui.Panel("Timings", w, topH, true, m.editorBody(w-4))
	if topH >= h {
		return top
	}
	guide := ui.Panel("Guidance", w, h-topH-1, false, m.guideBody(w-4))
	return lipgloss.JoinVertical(lipgloss.Left, top, "", guide)
}

// viewReadOnly renders a scrollable full-size panel.
func (m *Model) viewReadOnly(w, h int, title string, vp *viewport.Model, lines []string) string {
	contentW := w - 4
	contentH := h - 3 // border 2 + title line
	if contentW < 1 {
		contentW = 1
	}
	if contentH < 1 {
		contentH = 1
	}
	if vp.Width != contentW || vp.Height != contentH {
		vp.Width = contentW
		vp.Height = contentH
		vp.SetContent(strings.Join(lines, "\n"))
	}
	hint := lipgloss.NewStyle().Foreground(style.ActiveTheme().Dim).
		Render(fmt.Sprintf("%s · ↑/↓ scroll (%d%%) · [/] views", title, int(vp.ScrollPercent()*100)))
	return ui.Panel(title+"  "+hint, w, h, false, vp.View())
}

func (m *Model) editorBody(width int) string {
	boundsW := width - labelColW - detectedColW - draftColW
	if boundsW < 8 {
		boundsW = 8
	}

	lines := []string{
		style.Label.Render("Setting") +
			style.ColHeader.Width(detectedColW).Render("Detected") +
			style.ColHeader.Width(draftColW).Render("Draft") +
			style.ColHeader.Width(boundsW).Render("Safe"),
		"",
	}

	groups := []struct {
		title string
		from  int
		to    int
	}{
		{"General", 0, 3},
		{"Primary Timings", 3, 7},
		{"Secondary / Voltage", 7, len(m.fields)},
	}
	for gi, g := range groups {
		if gi > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, style.SectionTitle.Render(g.title))
		for i := g.from; i < g.to; i++ {
			lines = append(lines, m.fieldRow(i, boundsW))
		}
	}

	lock := "off"
	if m.lockRatios {
		lock = "on"
	}
	lines = append(lines, "",
		style.Hint.Render(fmt.Sprintf("ctrl+s apply · ctrl+r reset · l ratio lock [%s] · [/] views", lock)),
	)
	return strings.Join(lines, "\n")
}

func (m *Model) fieldRow(i, boundsW int) string {
	f := m.fields[i]
	draft := lipgloss.NewStyle().Width(draftColW).Render(f.input.View())
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		m.fieldLabelStyle(i).Render(f.label),
		style.OriginalValue.Render(f.hint),
		draft,
		m.renderBounds(f, m.boundsFor(f.label), boundsW),
	)
}

func (m *Model) fieldLabelStyle(i int) lipgloss.Style {
	if i != m.focus {
		return style.Label
	}
	if m.editing {
		return style.EditingLabel
	}
	return style.SelectedLabel
}

func (m *Model) guideBody(width int) string {
	f := m.fields[m.focus]
	th := style.ActiveTheme()

	lines := []string{
		style.SectionTitle.Render("Perf Projection"),
		m.perfProjection(),
		"",
		style.SectionTitle.Render("Guidance"),
		style.ResultValue.Render(f.label),
		style.Hint.Render(f.desc),
		"",
		style.SectionTitle.Render("Detection"),
	}

	switch {
	case m.detected && m.detectMsg != "":
		lines = append(lines, style.Safe.Render("ok"), style.Hint.Render(ui.Truncate(m.detectMsg, width, true)))
	case !m.detected && m.detectMsg != "":
		lines = append(lines, lipgloss.NewStyle().Foreground(th.Warning).Render("unavailable"),
			style.Hint.Render(ui.Truncate(m.detectMsg, width, true)),
			style.Hint.Render("R retries detection."))
	default:
		lines = append(lines, style.Hint.Render("waiting for detection..."))
	}

	if m.imcNote != "" {
		lines = append(lines, style.Hint.Render(ui.Truncate("IMC: "+m.imcNote, width, true)))
	}

	lines = append(lines,
		"",
		style.SectionTitle.Render("Hints"),
		style.Hint.Render("· Keep small, incremental changes."),
		style.Hint.Render("· Re-run the burn-in after each apply."),
		style.Hint.Render("· Prefer JEDEC/XMP-safe bounds."),
		"",
		style.SectionTitle.Render("Views"),
		style.Hint.Render("[ / ]  Timings · SPD · IMC"),
	)
	return strings.Join(lines, "\n")
}

func (m *Model) perfProjection() string {
	freq, okFreq := m.parseField("DRAM Freq")
	channels, okCh := m.parseField("Channels")
	cl, okCL := m.parseField("CAS (tCL)")
	if !okFreq || !okCh || freq <= 0 || channels <= 0 {
		return style.MetricMuted.Render("Enter DRAM Freq and Channels to estimate bandwidth.")
	}

	perChannel := freq * 8.0 / 1000.0 // GB/s per channel (MT/s * 8 bits / 8 / 1000)
	total := perChannel * channels
	rows := []string{
		style.Metric.Render(fmt.Sprintf("Peak:      %.2f GB/s", total)),
		style.MetricMuted.Render(fmt.Sprintf("Per chan:  %.2f GB/s", perChannel)),
		style.MetricMuted.Render(fmt.Sprintf("Sustained: %.2f GB/s (82%%)", total*0.82)),
	}
	if okCL && cl > 0 {
		rows = append(rows, style.MetricMuted.Render(fmt.Sprintf("CAS:       %.2f ns", 2000*cl/freq)))
	}
	return strings.Join(rows, "\n")
}
