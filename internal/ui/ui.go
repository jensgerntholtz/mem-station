// Package ui provides small, reusable Bubble Tea presentation helpers shared
// by the app shell and the tab packages. It is dependency-light: it depends
// only on the style leaf package, lipgloss, and the bubbles spinner.
package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"mem-station/internal/style"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Level classifies a status/error message for display.
type Level int

const (
	LevelInfo Level = iota
	LevelWarn
	LevelError
)

// StatusMsg is produced by Status and displayed by the root shell.
type StatusMsg struct {
	Text  string
	Level Level
}

// Status returns a command that emits a StatusMsg.
func Status(text string, level Level) tea.Cmd {
	return func() tea.Msg {
		return StatusMsg{Text: text, Level: level}
	}
}

// Severity markers used next to values and run verdicts. Every severity cue
// pairs a marker with text, so state is never conveyed by color alone (and
// stays legible with NO_COLOR).
const (
	MarkOK    = "ok"
	MarkInfo  = "?"
	MarkWarn  = "!"
	MarkError = "x"
)

// SeverityMark returns the marker for a status level.
func SeverityMark(l Level) string {
	switch l {
	case LevelWarn:
		return MarkWarn
	case LevelError:
		return MarkError
	default:
		return MarkInfo
	}
}

// KeyHint describes one key binding for footers and the help overlay.
type KeyHint struct {
	Key    string
	Action string
}

// HelpSection groups key hints under a title for the help overlay.
type HelpSection struct {
	Title string
	Rows  []KeyHint
}

// ClipLines hard-limits s to at most h lines.
func ClipLines(s string, h int) string {
	if h <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= h {
		return s
	}
	return strings.Join(lines[:h], "\n")
}

// Truncate truncates s to at most w visual columns, appending an ellipsis
// when ellipsis is true and truncation occurred.
func Truncate(s string, w int, ellipsis bool) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	// ansi.Truncate keeps escape sequences intact and counts only printable
	// cells, so truncating styled text cannot emit broken SGR sequences.
	tail := ""
	if ellipsis {
		tail = "…"
	}
	return ansi.Truncate(s, w, tail)
}

// PadRight pads s with spaces to w visual columns (no-op if s is wider).
func PadRight(s string, w int) string {
	if pad := w - lipgloss.Width(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// JoinColumns renders aligned text table rows using the given column widths.
func JoinColumns(rows [][]string, widths []int) string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		cells := make([]string, 0, len(row))
		for i, cell := range row {
			w := 0
			if i < len(widths) {
				w = widths[i]
			}
			cells = append(cells, PadRight(Truncate(cell, w, true), w))
		}
		out = append(out, strings.Join(cells, "  "))
	}
	return strings.Join(out, "\n")
}

// Panel renders a bordered box that occupies exactly width x height cells,
// with an optional title on its first content line. The body is clipped to the
// interior width and height, so a panel never overflows its box.
func Panel(title string, width, height int, focused bool, body string) string {
	th := style.ActiveTheme()
	borderColor := th.Border
	if focused {
		borderColor = th.BorderFocus
	}
	if width < 4 {
		width = 4
	}
	if height < 3 {
		height = 3
	}

	innerW := width - 4 // two border columns + one padding column per side
	contentH := height - 2
	if innerW < 1 {
		innerW = 1
	}
	if contentH < 1 {
		contentH = 1
	}
	if title != "" {
		contentH--
	}

	content := clipBlock(body, innerW, contentH)
	if title != "" {
		titleLine := style.Section().Render(Truncate(title, innerW, true))
		content = titleLine + "\n" + content
	}

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(width).
		Height(height)
	return border.Render(ClipLines(content, height-2))
}

// Footer renders a one-line status bar: status text on the left (colored by
// level) and as many key hints on the right as fit. Hints are dropped from the
// end before the status is squeezed, so the status message survives narrow
// terminals. The result is always exactly width cells wide.
func Footer(status string, level Level, hints []KeyHint, width int) string {
	if width <= 0 {
		return ""
	}
	th := style.ActiveTheme()
	statusColor := th.Fg
	switch level {
	case LevelWarn:
		statusColor = th.Warning
	case LevelError:
		statusColor = th.Danger
	}

	// Keep a readable minimum for the status; the hints give way first.
	reserve := width / 3
	if reserve > 24 {
		reserve = 24
	}
	if reserve < 12 {
		reserve = 12
	}
	if width < reserve+8 {
		reserve = 0
	}

	for len(hints) > 0 {
		right := renderHints(hints)
		if lipgloss.Width(right) <= width-reserve {
			break
		}
		hints = hints[:len(hints)-1]
	}

	// Warn/error statuses carry a marker so the level is not color-only.
	if level != LevelInfo {
		status = SeverityMark(level) + " " + status
	}

	right := renderHints(hints)
	rightW := lipgloss.Width(right)
	if rightW > width {
		return Truncate(right, width, false)
	}

	leftW := width - rightW
	left := lipgloss.NewStyle().Foreground(statusColor).
		Render(PadRight(Truncate(status, leftW, true), leftW))
	return left + right
}

func renderHints(hints []KeyHint) string {
	if len(hints) == 0 {
		return ""
	}
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		key := style.Accent().Render(h.Key)
		action := style.Muted().Render(h.Action)
		parts = append(parts, key+" "+action)
	}
	return strings.Join(parts, "  ")
}

// HelpOverlay renders a scrollable-at-offset-0 help box.
func HelpOverlay(width, height int, sections []HelpSection) string {
	return HelpOverlayAt(width, height, sections, 0)
}

// HelpOverlayAt renders a help box with the given scroll offset (in lines).
func HelpOverlayAt(width, height int, sections []HelpSection, offset int) string {
	th := style.ActiveTheme()
	if width < 4 {
		width = 4
	}
	if height < 2 {
		height = 2
	}

	var lines []string
	for _, sec := range sections {
		lines = append(lines, style.Section().Render(sec.Title))
		for _, row := range sec.Rows {
			key := style.Accent().Render(row.Key)
			action := style.Muted().Render(row.Action)
			lines = append(lines, "  "+key+"  "+action)
		}
		lines = append(lines, "")
	}

	if offset < 0 {
		offset = 0
	}
	if offset >= len(lines) {
		offset = max(0, len(lines)-1)
	}

	innerW := width - 4
	innerH := height - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	// The footer occupies the last content line.
	body := clipBlock(strings.Join(lines[offset:], "\n"), innerW, max(1, innerH-1))
	footer := style.Dim().Render("↑/↓ scroll · esc close")

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.BorderFocus).
		Padding(0, 1).
		Width(width).
		Height(height)
	return border.Render(ClipLines(body+"\n"+footer, height-2))
}

// Modal renders a bordered dialog with a title and body lines.
func Modal(title string, body []string, width int) string {
	th := style.ActiveTheme()
	if width < 4 {
		width = 4
	}
	lines := []string{style.Section().Render(title), ""}
	lines = append(lines, body...)
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.BorderFocus).
		Padding(0, 1).
		Width(width)
	return border.Render(strings.Join(lines, "\n"))
}

// Progress renders an ASCII progress bar of the given total width.
func Progress(pct float64, width int) string {
	if width < 3 {
		width = 3
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	inner := width - 2
	filled := int(math.Round(pct / 100 * float64(inner)))
	if filled > inner {
		filled = inner
	}
	if filled < 0 {
		filled = 0
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", inner-filled) + "]"
}

// DurationLabel formats a duration compactly: 30s, 5m, 1h, 90s.
func DurationLabel(d time.Duration) string {
	d = d.Round(time.Second)
	switch {
	case d <= 0:
		return "0s"
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	default:
		return fmt.Sprintf("%ds", d/time.Second)
	}
}

// AppendLine appends line to existing, keeping at most max trailing lines.
func AppendLine(existing, line string, max int) string {
	if max <= 0 {
		return line
	}
	if existing == "" {
		return line
	}
	combined := existing + "\n" + line
	lines := strings.Split(combined, "\n")
	if len(lines) > max {
		lines = lines[len(lines)-max:]
	}
	return strings.Join(lines, "\n")
}

// spinnerFrames are the run-state spinner glyphs, indexed by elapsed time so a
// tab never needs its own ticker beyond the app's 500 ms elapsed tick.
var spinnerFrames = []string{"|", "/", "-", "\\"}

// SpinnerFrame returns the spinner glyph for the given instant.
func SpinnerFrame(t time.Time) string {
	ms := t.UnixMilli()
	return spinnerFrames[ms/200%int64(len(spinnerFrames))]
}

func clipBlock(s string, w, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, line := range lines {
		lines[i] = Truncate(line, w, true)
	}
	return strings.Join(lines, "\n")
}
