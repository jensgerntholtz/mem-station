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
