// Package style holds the semantic color theme and the derived component
// styles shared by the app shell and the tab renderers.
//
// Renderers never name a color: they ask for a role (section title, muted hint,
// danger value) and this package resolves it against the active theme, so a
// light/dark switch or a NO_COLOR terminal changes every screen at once.
package style

import (
	"image/color"
	"os"

	"charm.land/lipgloss/v2"
	"charm.land/tone"
)

// Theme is the semantic token set used by the app shell and shared UI helpers.
type Theme struct {
	Bg            color.Color
	Fg            color.Color
	Muted         color.Color
	Accent        color.Color
	AccentBg      color.Color
	Border        color.Color
	BorderFocus   color.Color
	Success       color.Color
	Warning       color.Color
	Danger        color.Color
	Info          color.Color
	Label         color.Color
	Value         color.Color
	Dim           color.Color
	TabActiveFg   color.Color
	TabActiveBg   color.Color
	TabInactiveFg color.Color
	TabInactiveBg color.Color
	ButtonFg      color.Color
	ButtonBg      color.Color
	ButtonFocusFg color.Color
	ButtonFocusBg color.Color
	StatusFg      color.Color
	ErrorBg       color.Color
}

// Default returns the semantic theme for a dark or light terminal. Each token
// is one concrete color: the caller picks the palette (--dark), so nothing here
// depends on terminal background auto-detection. Bubble Tea v2 downsamples the
// rendered output to the terminal's color profile, so the same tokens degrade
// to 256/16/no color without a second code path.
func Default(dark bool) Theme {
	if dark {
		return Theme{
			Bg:            tone.Pepper,
			Fg:            tone.Salt,
			Muted:         tone.Steam,
			Accent:        tone.Hazy,
			AccentBg:      tone.Darple,
			Border:        tone.Oyster,
			BorderFocus:   tone.Hazy,
			Success:       tone.Guac,
			Warning:       tone.Mustard,
			Danger:        tone.Sriracha,
			Info:          tone.Sardine,
			Label:         tone.Anchovy,
			Value:         tone.Salt,
			Dim:           tone.Squid,
			TabActiveFg:   tone.Salt,
			TabActiveBg:   tone.Darple,
			TabInactiveFg: tone.Steam,
			TabInactiveBg: tone.Char,
			ButtonFg:      tone.Salt,
			ButtonBg:      tone.Darple,
			ButtonFocusFg: tone.Pepper,
			ButtonFocusBg: tone.Guac,
			StatusFg:      tone.Mustard,
			ErrorBg:       tone.Toast,
		}
	}
	return Theme{
		Bg:            tone.Salt,
		Fg:            tone.Pepper,
		Muted:         tone.Oyster,
		Accent:        tone.Darple,
		AccentBg:      tone.Charple,
		Border:        tone.Steep,
		BorderFocus:   tone.Darple,
		Success:       tone.Pickle,
		Warning:       tone.Tandoor,
		Danger:        tone.Pom,
		Info:          tone.Oceania,
		Label:         tone.Oceania,
		Value:         tone.Pepper,
		Dim:           tone.Squid,
		TabActiveFg:   tone.Salt,
		TabActiveBg:   tone.Darple,
		TabInactiveFg: tone.Oyster,
		TabInactiveBg: tone.Soda,
		ButtonFg:      tone.Salt,
		ButtonBg:      tone.Darple,
		ButtonFocusFg: tone.Salt,
		ButtonFocusBg: tone.Pickle,
		StatusFg:      tone.Tandoor,
		ErrorBg:       tone.Sakura,
	}
}

// Current is the active theme. It is set once in main via Set and read by the
// shell and shared UI helpers. Nil means "use the dark default".
var Current *Theme

// Set installs the active theme.
func Set(dark bool) {
	t := Default(dark)
	Current = &t
}

// ActiveTheme returns the current theme or the dark default.
func ActiveTheme() Theme {
	if Current != nil {
		return *Current
	}
	return Default(true)
}

// UsePlainOutput disables color for the rest of the process by setting the
// standard NO_COLOR/CLICOLOR variables that colorprofile.Detect honours. It must
// be called before the program starts so the detected profile is already plain
// (main does this when --no-color is given).
func UsePlainOutput() {
	os.Setenv("NO_COLOR", "1")
	os.Setenv("CLICOLOR", "0")
}

/* --------------- Derived component styles --------------- */

// Section is the style of a section heading inside a panel.
func Section() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(ActiveTheme().Accent)
}

// ColumnHeader is the style of a table header cell.
func ColumnHeader() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(ActiveTheme().Muted)
}

// FieldLabel is the style of a settings-table label cell.
func FieldLabel() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme().Label)
}

// SelectedField is the label style of the focused, not-yet-edited field.
func SelectedField() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).
		Foreground(ActiveTheme().Fg).
		Background(ActiveTheme().AccentBg)
}

// EditingField is the label style of the field whose text input owns the keys.
func EditingField() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).
		Foreground(ActiveTheme().TabActiveFg).
		Background(ActiveTheme().AccentBg)
}

// DetectedValue is the style of a hardware-detected value.
func DetectedValue() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme().Info)
}

// Muted is the style of secondary text and key hints.
func Muted() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme().Muted)
}

// Dim is the style of placeholders such as an unknown value.
func Dim() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme().Dim)
}

// ValueText is the style of a primary numeric value.
func ValueText() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme().Value)
}

// Accent is the style of an interactive key name.
func Accent() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme().Accent)
}

// Info is the style of an informational value or delta.
func Info() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme().Info)
}

// Metric is the style of a headline metric.
func Metric() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(ActiveTheme().Accent)
}

// OK is the style of a value inside its safe range.
func OK() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme().Success)
}

// Success is the style of a completed/favourable state.
func Success() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme().Success)
}

// Warning is the style of a value or state that needs attention.
func Warning() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme().Warning)
}

// Danger is the style of an unsafe value or failed state.
func Danger() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme().Danger)
}

// RunButton is the style of a primary action button.
func RunButton() lipgloss.Style {
	t := ActiveTheme()
	return lipgloss.NewStyle().Bold(true).
		Foreground(t.ButtonFocusFg).
		Background(t.ButtonFocusBg).
		Padding(0, 3)
}

// RunningButton is the style of the same button while a job is active.
func RunningButton() lipgloss.Style {
	t := ActiveTheme()
	return lipgloss.NewStyle().Bold(true).
		Foreground(t.TabActiveFg).
		Background(t.Accent).
		Padding(0, 3)
}

// TabActive is the style of the selected tab-bar label.
func TabActive() lipgloss.Style {
	t := ActiveTheme()
	return lipgloss.NewStyle().Bold(true).
		Foreground(t.TabActiveFg).
		Background(t.TabActiveBg).
		Padding(0, 2)
}

// TabInactive is the style of an unselected tab-bar label.
func TabInactive() lipgloss.Style {
	t := ActiveTheme()
	return lipgloss.NewStyle().
		Foreground(t.TabInactiveFg).
		Background(t.TabInactiveBg).
		Padding(0, 2)
}
