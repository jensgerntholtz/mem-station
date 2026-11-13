package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// TestFooterTruncationKeepsAGap checks a status too long for its column ends in
// an ellipsis followed by a space, so it never runs into the key hints.
func TestFooterTruncationKeepsAGap(t *testing.T) {
	hints := []KeyHint{
		{Key: "↑/k", Action: "select"},
		{Key: "←", Action: "decrease"},
		{Key: "→", Action: "increase"},
		{Key: "enter", Action: "edit"},
		{Key: "tab", Action: "next field"},
	}
	// Long enough to be truncated at every width under test.
	status := strings.Repeat("Detection needs root · press E to elevate with sudo. ", 3)
	for _, width := range []int{118, 100, 78} {
		out := Footer(status, LevelError, hints, width)
		if got := lipgloss.Width(out); got != width {
			t.Errorf("width %d: footer is %d cells", width, got)
		}
		plain := StripANSIForTest(out)
		if !strings.Contains(plain, "… ") {
			t.Errorf("width %d: no gap after the ellipsis: %q", width, plain)
		}
	}
}

// StripANSIForTest removes SGR sequences so assertions read visible text.
func StripANSIForTest(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case inEscape:
			if r == 'm' {
				inEscape = false
			}
		case r == 0x1b:
			inEscape = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
