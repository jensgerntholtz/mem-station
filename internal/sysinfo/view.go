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

	labelColW    = 16
	detectedColW = 9
	draftColW    = 9
	marginColW   = 8
	columnGaps   = 8 // four two-column separators
)

// View renders the tab into exactly w x h cells.
func (m *Model) View(w, h int) string {
	if w < 1 || h < 1 {
		return ""
	}
	if m.modal != modalNone {
		return m.viewApplyModal(w, h)
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
		left := ui.Panel("Timings", leftW, h, true, m.editorBody(leftW-4, h-3))
		right := ui.Panel("Guidance", rightW, h, false, m.guideBody(rightW-4))
		return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
	}

	topH := h * 64 / 100
	if topH < 12 || h-topH-1 < 6 {
		// Not enough vertical room for two stacked panels.
		topH = h
	}
	top := ui.Panel("Timings", w, topH, true, m.editorBody(w-4, topH-3))
	if topH >= h {
		return top
	}
	guide := ui.Panel("Guidance", w, h-topH-1, false, m.guideBody(w-4))
	return lipgloss.JoinVertical(lipgloss.Left, top, "", guide)
}

// viewReadOnly renders a scrollable full-size panel. Each sub-view owns its
// viewport, so the scroll offset survives tab switches and only the size (and
// therefore the wrapping) is recomputed on resize.
func (m *Model) viewReadOnly(w, h int, title string, vp *viewport.Model, lines []string) string {
	contentW := w - 4
	contentH := h - 3 // border 2 + title line
	if contentW < 1 {
		contentW = 1
	}
	if contentH < 1 {
		contentH = 1
	}
	vp.Width = contentW
	vp.Height = contentH
	// SetContent keeps the scroll offset, so re-rendering after a resize or a
	// detection update never jumps the view back to the top.
	vp.SetContent(strings.Join(lines, "\n"))
	hint := lipgloss.NewStyle().Foreground(style.ActiveTheme().Dim).
		Render(fmt.Sprintf("%s - up/down scroll (%d%%) - [/] views", title, int(vp.ScrollPercent()*100)))
	return ui.Panel(hint, w, h, false, vp.View())
}

/* --------------- Editor --------------- */

// fieldColumnWidths spreads the editor width over the five columns.
func fieldColumnWidths(width int) []int {
	safetyW := width - labelColW - detectedColW - draftColW - marginColW - columnGaps
	if safetyW < 8 {
		safetyW = 8
	}
	return []int{labelColW, detectedColW, draftColW, marginColW, safetyW}
}

// editorLine is one rendered editor row. field is the field index for a
// setting row and -1 for headers, group titles, and spacer lines.
type editorLine struct {
	text  string
	field int
}

// editorBody renders the setting table. The table scrolls so the focused field
// is always visible, even when the body is shorter than the field list.
func (m *Model) editorBody(width, height int) string {
	widths := fieldColumnWidths(width)
	issues := m.issues()
	ctx := m.context()

	lines := []editorLine{{text: m.editorHeader(widths), field: -1}}
	for gi, g := range Groups() {
		if gi > 0 {
			lines = append(lines, editorLine{field: -1})
		}
		lines = append(lines, editorLine{
			text:  ui.Truncate(style.SectionTitle.Render(string(g)), width, true),
			field: -1,
		})
		for _, i := range groupFields(g, m.fields) {
			lines = append(lines, editorLine{
				text:  ui.JoinColumns([][]string{m.fieldRow(i, widths, issues, ctx)}, widths),
				field: i,
			})
		}
	}

	// Reserve one line for the key hint at the bottom of the panel.
	visible := height - 1
	if visible < 1 {
		visible = 1
	}
	focusLine := 0
	for i, l := range lines {
		if l.field == m.focus {
			focusLine = i
			break
		}
	}
	start := 0
	if focusLine >= visible {
		start = focusLine - visible + 1
	}
	if maxStart := len(lines) - visible; start > maxStart {
		start = maxStart
	}
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(lines) {
		end = len(lines)
	}

	out := make([]string, 0, visible+1)
	for _, l := range lines[start:end] {
		out = append(out, l.text)
	}
	out = append(out, m.editorHint(end < len(lines) || start > 0))
	return strings.Join(out, "\n")
}

func (m *Model) editorHeader(widths []int) string {
	th := style.ActiveTheme()
	cells := []string{
		lipgloss.NewStyle().Bold(true).Foreground(th.Label).Render("Setting"),
		lipgloss.NewStyle().Bold(true).Foreground(th.Muted).Render("Detected"),
		lipgloss.NewStyle().Bold(true).Foreground(th.Muted).Render("Draft"),
		lipgloss.NewStyle().Bold(true).Foreground(th.Muted).Render("Delta"),
		lipgloss.NewStyle().Bold(true).Foreground(th.Muted).Render("Safety"),
	}
	return ui.JoinColumns([][]string{cells}, widths)
}

func (m *Model) fieldRow(i int, widths []int, issues []Issue, ctx ValidationContext) []string {
	f := m.fields[i]
	return []string{
		m.fieldLabelStyle(i).Width(widths[0]).Render(f.Label),
		m.detectedCell(f, widths[1]),
		m.draftCell(i, widths[2]),
		m.marginCell(f, ctx, widths[3]),
		m.safetyCell(f, issues, ctx, widths[4]),
	}
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

// detectedCell shows the hardware value, or "?" when detection has not
// produced one. Unknown values are never rendered as a number.
func (m *Model) detectedCell(f Field, width int) string {
	th := style.ActiveTheme()
	v, ok := m.detected[f.ID]
	if !ok || strings.TrimSpace(v) == "" {
		return lipgloss.NewStyle().Foreground(th.Dim).Render("?")
	}
	return style.OriginalValue.Width(width).Render(ui.Truncate(formatValue(f, v), width, true))
}

// draftCell shows the editable value; the focused field renders the live text
// input so the cursor is visible while editing.
func (m *Model) draftCell(i, width int) string {
	f := m.fields[i]
	if m.editing && i == m.focus {
		return ui.PadRight(ui.Truncate(m.input.View(), width, false), width)
	}
	th := style.ActiveTheme()
	raw := strings.TrimSpace(m.draft[f.ID])
	if raw == "" || raw == "?" {
		return lipgloss.NewStyle().Foreground(th.Dim).Render("?")
	}
	if i == m.focus {
		return lipgloss.NewStyle().Bold(true).Foreground(th.Value).
			Render(ui.Truncate(formatValue(f, raw), width, true))
	}
	return ui.Truncate(formatValue(f, raw), width, true)
}

// marginCell shows the signed margin to the nearest safe bound, in the field's
// unit. "?" means there is no known range to measure against yet.
func (m *Model) marginCell(f Field, ctx ValidationContext, width int) string {
	th := style.ActiveTheme()
	if f.Bounds == nil {
		return ""
	}
	b := f.Bounds(ctx)
	if !b.Known() {
		return ""
	}
	v, ok := ctx.Float(f.ID)
	if !ok {
		return lipgloss.NewStyle().Foreground(th.Dim).Render("?")
	}
	mg, ok := margin(b, v)
	if !ok {
		return ""
	}
	text := signedMargin(mg) + marginUnit(f)
	st := lipgloss.NewStyle().Foreground(th.Success)
	switch {
	case b.HasMin && v < b.Min:
		st = lipgloss.NewStyle().Foreground(th.Danger)
	case b.HasMax && v > b.Max:
		if b.severityAbove() == SeverityDanger {
			st = lipgloss.NewStyle().Foreground(th.Danger)
		} else {
			st = lipgloss.NewStyle().Foreground(th.Warning)
		}
	}
	return st.Render(ui.Truncate(text, width, true))
}

// safetyCell shows the worst validation finding for the field, or "ok" when the
// value is inside its known range.
func (m *Model) safetyCell(f Field, issues []Issue, ctx ValidationContext, width int) string {
	th := style.ActiveTheme()
	iss, ok := worstIssue(issues, f.ID)
	if !ok {
		if f.Bounds == nil {
			return ""
		}
		if b := f.Bounds(ctx); !b.Known() {
			return ""
		}
		return lipgloss.NewStyle().Foreground(th.Success).Render("ok")
	}
	var styleFn lipgloss.Style
	var symbol string
	switch iss.Severity {
	case SeverityDanger:
		styleFn, symbol = lipgloss.NewStyle().Foreground(th.Danger), "x "
	case SeverityWarn:
		styleFn, symbol = lipgloss.NewStyle().Foreground(th.Warning), "! "
	case SeverityInfo:
		styleFn, symbol = lipgloss.NewStyle().Foreground(th.Dim), "? "
	default:
		return ""
	}
	return styleFn.Render(ui.Truncate(symbol+iss.Message, width, true))
}

// signedMargin formats a margin with an explicit sign.
func signedMargin(v float64) string {
	if v > 0 {
		return "+" + formatNumber(v)
	}
	return formatNumber(v)
}

// marginUnit returns the compact unit suffix for the margin column.
func marginUnit(f Field) string {
	switch f.Unit {
	case "V", "T":
		return f.Unit
	default:
		return ""
	}
}

// editorHint renders the last editor line: the ratio lock state plus a scroll
// indicator when the table is taller than the panel.
func (m *Model) editorHint(scrollable bool) string {
	lock := "off"
	if m.lockRatios {
		lock = "on"
	}
	text := fmt.Sprintf("ctrl+s apply - ctrl+r reset - l lock[%s]", lock)
	if scrollable {
		text = "more fields - " + text
	}
	return style.Hint.Render(text)
}

/* --------------- Guidance --------------- */

func (m *Model) guideBody(width int) string {
	th := style.ActiveTheme()
	ctx := m.context()
	issues := m.issues()

	f := m.fields[m.focus]
	lines := []string{
		style.SectionTitle.Render("Perf Projection"),
	}
	lines = append(lines, m.perfProjection()...)

	lines = append(lines, "", style.SectionTitle.Render("Derived"))
	lines = append(lines, m.derivedLines()...)

	lines = append(lines, "", style.SectionTitle.Render(f.Label), style.Hint.Render(ui.Truncate(f.Desc, width, true)))
	if f.Bounds != nil {
		if label := f.Bounds(ctx).Label(); label != "" {
			lines = append(lines, style.Hint.Render("safe: "+ui.Truncate(label, max(1, width-6), true)))
		}
	}

	lines = append(lines, "", style.SectionTitle.Render("Safety"))
	danger, warn, info := countIssues(issues)
	lines = append(lines,
		lipgloss.NewStyle().Foreground(th.Success).Render(fmt.Sprintf("ok     %d fields", len(m.fields)-danger-warn-info)),
		lipgloss.NewStyle().Foreground(th.Dim).Render(fmt.Sprintf("? info %d", info)),
		lipgloss.NewStyle().Foreground(th.Warning).Render(fmt.Sprintf("! warn %d", warn)),
		lipgloss.NewStyle().Foreground(th.Danger).Render(fmt.Sprintf("x danger %d", danger)),
	)

	lines = append(lines, "", style.SectionTitle.Render("Detection"))
	switch {
	case m.Detected() && m.detectMsg != "":
		lines = append(lines, style.Safe.Render("ok"), style.Hint.Render(ui.Truncate(m.detectMsg, width, true)))
	case m.detectMsg != "":
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
		style.SectionTitle.Render("Views"),
		style.Hint.Render("[ / ]  Timings - SPD - IMC"),
	)
	return strings.Join(lines, "\n")
}

// perfProjection renders the theoretical bandwidth, per-channel bandwidth, CAS
// latency, and the delta against the applied profile.
func (m *Model) perfProjection() []string {
	freq, okFreq := parseNumber(m.draft[FieldFreq])
	channels, okCh := parseNumber(m.draft[FieldChannels])
	if !okFreq || !okCh || freq <= 0 || channels <= 0 {
		return []string{style.MetricMuted.Render("Enter DRAM Freq and Channels to estimate bandwidth.")}
	}

	perChannel := freq * 8.0 / 1000.0 // GB/s per channel (MT/s * 8 bits / 8 / 1000)
	total := perChannel * channels
	rows := []string{
		style.Metric.Render(fmt.Sprintf("Peak:      %.2f GB/s", total)),
		style.MetricMuted.Render(fmt.Sprintf("Per chan:  %.2f GB/s", perChannel)),
		style.MetricMuted.Render(fmt.Sprintf("Sustained: %.2f GB/s (82%% eff)", total*0.82)),
	}
	if cl, ok := parseNumber(m.draft[FieldCL]); ok && cl > 0 {
		rows = append(rows, style.MetricMuted.Render(fmt.Sprintf("CAS:       %.2f ns", cyclesToNs(cl, freq))))
	}

	// Compare against the applied profile so a regression is visible before it
	// is saved. Only computed when both sides are known.
	aFreq, okAF := parseNumber(m.baseline(FieldFreq))
	aCh, okAC := parseNumber(m.baseline(FieldChannels))
	if okAF && okAC && aFreq > 0 && aCh > 0 {
		appliedTotal := aFreq * 8.0 / 1000.0 * aCh
		delta := total - appliedTotal
		if delta != 0 {
			rows = append(rows, deltaLine(delta, appliedTotal))
		}
	}
	return rows
}

// deltaLine renders the bandwidth delta against the applied profile.
func deltaLine(delta, applied float64) string {
	th := style.ActiveTheme()
	pct := 0.0
	if applied > 0 {
		pct = delta / applied * 100
	}
	text := fmt.Sprintf("vs applied: %+.2f GB/s (%+.1f%%)", delta, pct)
	if delta < 0 {
		return lipgloss.NewStyle().Foreground(th.Warning).Render(text)
	}
	return lipgloss.NewStyle().Foreground(th.Success).Render(text)
}

// derivedLines renders the derived timing metrics (row cycle and refresh
// window) in nanoseconds.
func (m *Model) derivedLines() []string {
	freq, ok := parseNumber(m.draft[FieldFreq])
	if !ok || freq <= 0 {
		return []string{style.MetricMuted.Render("Needs DRAM Freq to convert cycles to ns.")}
	}
	var out []string
	if ras, ok := parseNumber(m.draft[FieldRAS]); ok {
		if rp, ok := parseNumber(m.draft[FieldRP]); ok {
			out = append(out, style.MetricMuted.Render(
				fmt.Sprintf("tRC:  %.1f ns (tRAS+tRP)", cyclesToNs(ras+rp, freq))))
		}
	}
	if rfc, ok := parseNumber(m.draft[FieldRFC]); ok {
		out = append(out, style.MetricMuted.Render(
			fmt.Sprintf("tRFC: %.1f ns (min 160 ns)", cyclesToNs(rfc, freq))))
	}
	if len(out) == 0 {
		return []string{style.MetricMuted.Render("Needs tRAS/tRP/tRFC to derive row timings.")}
	}
	return out
}

// countIssues tallies findings by severity.
func countIssues(issues []Issue) (danger, warn, info int) {
	for _, iss := range issues {
		switch iss.Severity {
		case SeverityDanger:
			danger++
		case SeverityWarn:
			warn++
		case SeverityInfo:
			info++
		}
	}
	return danger, warn, info
}

/* --------------- Apply review modal --------------- */

// viewApplyModal renders the review dialog centred in the body.
func (m *Model) viewApplyModal(w, h int) string {
	modalW := w - 8
	if modalW > 64 {
		modalW = 64
	}
	if minW := w - 2; modalW > minW {
		modalW = minW
	}
	if modalW < 20 {
		modalW = 20
	}

	budget := h - 6 // borders, title, and padding
	if budget < 3 {
		budget = 3
	}
	body := m.applyReviewLines(modalW - 4)
	if len(body) > budget {
		body = body[:budget]
	}
	modal := ui.Modal("Apply profile", body, modalW)
	return ui.ClipLines(lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, modal), h)
}

// applyReviewLines renders the applied -> draft diff, the validation warnings,
// and the safety disclaimer.
func (m *Model) applyReviewLines(innerW int) []string {
	th := style.ActiveTheme()
	clip := func(s string) string { return ui.Truncate(s, innerW, true) }

	lines := []string{
		lipgloss.NewStyle().Foreground(th.Muted).Render(clip("Review the differences, then confirm.")),
		"",
	}
	changes := 0
	for _, f := range m.fields {
		from := m.baseline(f.ID)
		to := m.draft[f.ID]
		if from == to {
			continue
		}
		changes++
		line := fmt.Sprintf("%-14s %s -> %s", f.Label, formatValue(f, from), formatValue(f, to))
		lines = append(lines, lipgloss.NewStyle().Foreground(th.Info).Render(clip(line)))
	}
	if changes == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(th.Dim).Render(clip("(no differences)")))
	}

	issues := m.issues()
	var warnings []string
	for _, iss := range issues {
		if iss.Severity != SeverityWarn && iss.Severity != SeverityDanger {
			continue
		}
		label := string(iss.FieldID)
		if f, ok := LookupField(iss.FieldID); ok {
			label = f.Label
		}
		symbol := "!"
		st := lipgloss.NewStyle().Foreground(th.Warning)
		if iss.Severity == SeverityDanger {
			symbol = "x"
			st = lipgloss.NewStyle().Foreground(th.Danger)
		}
		warnings = append(warnings, st.Render(clip(symbol+" "+label+": "+iss.Message)))
	}
	if len(warnings) > 0 {
		lines = append(lines, "", style.SectionTitle.Render("Warnings"))
		lines = append(lines, warnings...)
	}
	if _, _, info := countIssues(issues); info > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(th.Dim).
			Render(clip(fmt.Sprintf("%d fields are still waiting for detection.", info))))
	}

	lines = append(lines, "",
		lipgloss.NewStyle().Bold(true).Foreground(th.Warning).Render(clip("Profile only - not written to hardware")),
		lipgloss.NewStyle().Foreground(th.Muted).Render(clip("enter confirm - esc cancel")),
	)
	return lines
}
