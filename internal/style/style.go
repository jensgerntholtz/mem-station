package style

import "github.com/charmbracelet/lipgloss"

// Color palette (migrate from styles.go)
const (
	ColorFgMain      = "230"
	ColorBgMain      = "62"
	ColorFgSub       = "153"
	ColorSection     = "111"
	ColorBorderPanel = "60"
	ColorLabel       = "109"
	ColorHint        = "102"
	ColorColHeader   = "189"
	ColorOrigVal     = "245"
	ColorBounds      = "144"
	ColorSafe        = "42"
	ColorWarn        = "220"
	ColorUnsafe      = "196"
	ColorTabActiveFg = "230"
	ColorTabActiveBg = "62"
	ColorTabInactFg  = "245"
	ColorTabInactBg  = "238"
	ColorRunBtnFg    = "230"
	ColorRunBtnBg    = "28"
	ColorRunningBg   = "100"
	ColorEffGood     = "42"
	ColorEffOk       = "220"
	ColorEffLow      = "196"
	ColorResultLbl   = "109"
	ColorResultVal   = "230"
	ColorResultTheo  = "153"
	ColorInfoLbl     = "109"
	ColorInfoVal     = "230"
	ColorInfoPanel   = "60"
	ColorStatus      = "220"
	ColorBtnFg       = "230"
	ColorBtnBg       = "62"
	ColorBtnFocusFg  = "230"
	ColorBtnFocusBg  = "28"
	ColorLockOnFg    = "230"
	ColorLockOnBg    = "28"
	ColorLockOffFg   = "230"
	ColorLockOffBg   = "62"
	ColorMetric      = "111"
	ColorMetricMute  = "245"
)

// Style presets (migrate from styles.go, map StyleX → X)
var (
	PaddingApp     = lipgloss.NewStyle().Padding(1, 2)
	PaddingPanel   = lipgloss.NewStyle().Padding(1, 1)
	PaddingBtn     = lipgloss.NewStyle().Padding(0, 2)
	PaddingBtnWide = lipgloss.NewStyle().Padding(0, 3)
	PaddingLock    = lipgloss.NewStyle().Padding(0, 1)
	Heading        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorFgMain)).Background(lipgloss.Color(ColorBgMain)).Padding(0, 1)
	SubHeading     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorFgSub))
	SectionTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorSection))
	Panel          = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(ColorBorderPanel)).Padding(1, 1)
	Label          = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorLabel)).Width(20)
	SelectedLabel  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorLabel)).Background(lipgloss.Color("236")).Width(20)
	EditingLabel   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorFgMain)).Background(lipgloss.Color("24")).Width(20)
	Hint           = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorHint))
	ColHeader      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorColHeader))
	OriginalValue  = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorOrigVal)).Width(10)
	Bounds         = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBounds)).Width(14)
	Safe           = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSafe))
	Warn           = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarn))
	Unsafe         = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorUnsafe))
	ActiveTab      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorTabActiveFg)).Background(lipgloss.Color(ColorTabActiveBg)).Padding(0, 2)
	InactiveTab    = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTabInactFg)).Background(lipgloss.Color(ColorTabInactBg)).Padding(0, 2)
	RunBtn         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorRunBtnFg)).Background(lipgloss.Color(ColorRunBtnBg)).Padding(0, 3)
	RunningBtn     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorRunBtnFg)).Background(lipgloss.Color(ColorRunningBg)).Padding(0, 3)
	EffGood        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorEffGood))
	EffOk          = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorEffOk))
	EffLow         = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorEffLow))
	ResultLabel    = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorResultLbl)).Width(24)
	ResultValue    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorResultVal)).Width(14)
	ResultTheo     = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorResultTheo)).Width(14)
	InfoLabel      = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorInfoLbl)).Width(28)
	InfoValue      = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorInfoVal))
	InfoPanel      = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(ColorInfoPanel)).Padding(1, 1)
	Status         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorStatus))
	Btn            = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBtnFg)).Background(lipgloss.Color(ColorBtnBg)).Padding(0, 2)
	BtnFocus       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorBtnFocusFg)).Background(lipgloss.Color(ColorBtnFocusBg)).Padding(0, 2)
	LockOn         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorLockOnFg)).Background(lipgloss.Color(ColorLockOnBg)).Padding(0, 1)
	LockOff        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorLockOffFg)).Background(lipgloss.Color(ColorLockOffBg)).Padding(0, 1)
	Metric         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorMetric))
	MetricMuted    = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMetricMute))
	ConsoleLine    = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMetric))
)

// Theme is the semantic token set used by the app shell and shared UI helpers.
// Individual tab renderers may keep using the exported style vars above until
// Phase 4 migrates them fully onto these tokens.
type Theme struct {
	Bg            lipgloss.AdaptiveColor
	Fg            lipgloss.AdaptiveColor
	Muted         lipgloss.AdaptiveColor
	Accent        lipgloss.AdaptiveColor
	AccentBg      lipgloss.AdaptiveColor
	Border        lipgloss.AdaptiveColor
	BorderFocus   lipgloss.AdaptiveColor
	Success       lipgloss.AdaptiveColor
	Warning       lipgloss.AdaptiveColor
	Danger        lipgloss.AdaptiveColor
	Info          lipgloss.AdaptiveColor
	Label         lipgloss.AdaptiveColor
	Value         lipgloss.AdaptiveColor
	Dim           lipgloss.AdaptiveColor
	TabActiveFg   lipgloss.AdaptiveColor
	TabActiveBg   lipgloss.AdaptiveColor
	TabInactiveFg lipgloss.AdaptiveColor
	TabInactiveBg lipgloss.AdaptiveColor
	ButtonFg      lipgloss.AdaptiveColor
	ButtonBg      lipgloss.AdaptiveColor
	ButtonFocusFg lipgloss.AdaptiveColor
	ButtonFocusBg lipgloss.AdaptiveColor
	StatusFg      lipgloss.AdaptiveColor
	ErrorBg       lipgloss.AdaptiveColor
}

// Default returns the semantic theme for a dark or light terminal. Colors are
// AdaptiveColor values; the global lipgloss/termenv color profile degrades or
// strips them when NO_COLOR/CLICOLOR=0 is set. The dark flag selects the
// palette; each token uses the same color for light and dark backgrounds so
// the chosen appearance is stable regardless of terminal auto-detection.
func Default(dark bool) Theme {
	if dark {
		return Theme{
			Bg:            solid("234"),
			Fg:            solid("230"),
			Muted:         solid("245"),
			Accent:        solid("111"),
			AccentBg:      solid("62"),
			Border:        solid("60"),
			BorderFocus:   solid("111"),
			Success:       solid("42"),
			Warning:       solid("220"),
			Danger:        solid("196"),
			Info:          solid("153"),
			Label:         solid("109"),
			Value:         solid("230"),
			Dim:           solid("102"),
			TabActiveFg:   solid("230"),
			TabActiveBg:   solid("62"),
			TabInactiveFg: solid("245"),
			TabInactiveBg: solid("238"),
			ButtonFg:      solid("230"),
			ButtonBg:      solid("62"),
			ButtonFocusFg: solid("230"),
			ButtonFocusBg: solid("28"),
			StatusFg:      solid("220"),
			ErrorBg:       solid("52"),
		}
	}
	return Theme{
		Bg:            solid("231"),
		Fg:            solid("16"),
		Muted:         solid("240"),
		Accent:        solid("25"),
		AccentBg:      solid("39"),
		Border:        solid("250"),
		BorderFocus:   solid("25"),
		Success:       solid("28"),
		Warning:       solid("172"),
		Danger:        solid("160"),
		Info:          solid("24"),
		Label:         solid("24"),
		Value:         solid("16"),
		Dim:           solid("244"),
		TabActiveFg:   solid("231"),
		TabActiveBg:   solid("25"),
		TabInactiveFg: solid("240"),
		TabInactiveBg: solid("253"),
		ButtonFg:      solid("231"),
		ButtonBg:      solid("25"),
		ButtonFocusFg: solid("231"),
		ButtonFocusBg: solid("28"),
		StatusFg:      solid("166"),
		ErrorBg:       solid("224"),
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

func solid(c string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: c, Dark: c}
}
