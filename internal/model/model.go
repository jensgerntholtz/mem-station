package model

import (
	"fmt"
	"strconv"
	"strings"

	"mem-station/internal/bench"
	"mem-station/internal/burnin"
	"mem-station/internal/memory"
	"mem-station/internal/style"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Benchmark runner: wraps bench.RunBenchmarkCmd for Bubbletea
var runBenchmarkCmd tea.Cmd = func() tea.Msg {
	results, err := bench.RunBenchmarkCmd()
	return benchResultMsg{results: convertBenchResults(results), err: err}
}

var runBurnInCmd tea.Cmd = func() tea.Msg {
	results, err := burnin.RunBurnInCmd()
	return burnInResultMsg{results: convertBurnInResults(results), err: err}
}

// Convert burnin.BurnInResults to local burnInResults type
func convertBurnInResults(b *burnin.BurnInResults) *burnInResults {
	if b == nil {
		return nil
	}
	return &burnInResults{
		duration: b.Duration,
		errors:   b.Errors,
		success:  b.Success,
	}
}

// Convert bench.BenchResults to local benchResults type
func convertBenchResults(b *bench.BenchResults) *benchResults {
	if b == nil {
		return nil
	}
	out := &benchResults{
		readRate:  b.ReadRate,
		writeRate: b.WriteRate,
		duration:  b.Duration,
	}
	for _, m := range b.Metrics {
		out.metrics = append(out.metrics, benchMetric{label: m.Label, value: m.Value})
	}
	return out
}

// --- Helper methods for Model ---

// focusPosition returns the current focus position index.
func (m *Model) focusPosition() int {
	switch m.focusType {
	case focusField:
		return m.focusIndex
	case focusApply:
		return len(m.fields)
	default:
		return len(m.fields) + 1
	}
}

// parseNumericField parses a field value as float64.
func (m *Model) parseNumericField(label string) (float64, bool) {
	for _, f := range m.fields {
		if f.label != label {
			continue
		}
		clean := sanitizeNumeric(f.input.Value())
		if clean == "" {
			clean = sanitizeNumeric(f.hint)
		}
		if clean == "" {
			return 0, false
		}
		v, err := strconv.ParseFloat(clean, 64)
		if err != nil {
			return 0, false
		}
		return v, true
	}
	return 0, false
}

// originalNumericField returns the original (hint) value for a field as float64.
func (m *Model) originalNumericField(label string) float64 {
	for _, f := range m.fields {
		if f.label != label {
			continue
		}
		v, err := strconv.ParseFloat(sanitizeNumeric(f.hint), 64)
		if err != nil {
			return 0
		}
		return v
	}
	return 0
}

// scaleTimingField scales a timing field's value by a ratio.
func (m *Model) scaleTimingField(label string, ratio float64) {
	idx := m.fieldIndex(label)
	if idx < 0 {
		return
	}
	orig, ok := parseUnsignedInt(m.fields[idx].hint)
	if !ok {
		return
	}
	scaled := int(float64(orig)*ratio + 0.5)
	if scaled < 1 {
		scaled = 1
	}
	m.fields[idx].input.SetValue(fmt.Sprintf("%d", scaled))
}

// moveFocus moves the input focus by delta, cycling through fields and actions.
func (m *Model) moveFocus(delta int) {
	total := len(m.fields) + 2
	pos := m.focusPosition() + delta
	if pos < 0 {
		pos = total - 1
	}
	if pos >= total {
		pos = 0
	}

	for i := range m.fields {
		m.fields[i].input.Blur()
	}

	switch {
	case pos < len(m.fields):
		m.focusType = focusField
		m.focusIndex = pos
		m.fields[pos].input.Focus()
	case pos == len(m.fields):
		m.focusType = focusApply
	default:
		m.focusType = focusCancel
	}
}

// resetValues resets all fields to their original hints.
func (m *Model) resetValues() {
	for i := range m.fields {
		m.fields[i].input.SetValue(m.fields[i].hint)
	}
}

// handleFrequencyEdit auto-scales primary timings if ratio lock is enabled.
func (m *Model) handleFrequencyEdit() {
	if !m.lockRatios {
		return
	}

	newFreq, ok := m.parseNumericField("DRAM Freq")
	if !ok || newFreq <= 0 {
		return
	}

	origFreq := m.originalNumericField("DRAM Freq")
	if origFreq <= 0 {
		return
	}

	ratio := newFreq / origFreq
	m.scaleTimingField("CAS (tCL)", ratio)
	m.scaleTimingField("tRCD", ratio)
	m.scaleTimingField("tRP", ratio)
	m.scaleTimingField("tRAS", ratio)
	m.scaleTimingField("tRFC", ratio)
}

// snapshotSummary returns a short summary of current field values.
func (m Model) snapshotSummary() string {
	summary := []string{}
	for _, f := range m.fields {
		summary = append(summary, fmt.Sprintf("%s=%s", f.label, f.input.Value()))
	}
	if len(summary) > 3 {
		return strings.Join(summary[:3], ", ") + ", ..."
	}
	return strings.Join(summary, ", ")
}

func (m Model) View() string {
	if m.width == 0 {
		m.width = 120
	}

	heading := style.Heading.Render(" MemStation ") + "  " +
		style.SubHeading.Render("Memory Timing Workspace")

	tabBar := m.renderTabBar()

	var content string

	switch m.activeTab {
	case 0:
		content = m.renderSysInfoTab()
	case 1:
		content = m.renderBenchmarkTab()
	case 2:
		content = m.renderBurnInTab()
	}

	status := style.Status.Render(m.status)

	ui := lipgloss.JoinVertical(lipgloss.Left, heading, tabBar, "", content, "", status)
	return style.PaddingApp.Render(ui)
}

// Stub for sanitizeNumeric if not present
func sanitizeNumeric(s string) string { return s }

// tertiaryTimings is a placeholder for the IMC timing struct.
type tertiaryTimings struct {
	channel int
	tRDRDsg int
	tRDRDdg int
	tRDRDdr int
	tRDRDdd int
	tWRWRsg int
	tWRWRdg int
	tWRWRdr int
	tWRWRdd int
	tRDWRsg int
	tRDWRdg int
	tRDWRdr int
	tRDWRdd int
	tWRRDsg int
	tWRRDdg int
	tWRRDdr int
	tWRRDdd int
	tRAS    int
	tRCD    int
	tRP     int
	tRTP    int
	tWTR    int
	tRRDsg  int
	tRRDdg  int
	tCKE    int
	tREFI   int
	tRFC    int
	tRDPRE  int
	tWRPRE  int
	rawDBP  int
	rawRAP  int
	rawRWP  int
	rawOTH  int
	rawOTH2 int
	rawRFTP int
}

// Hardware detect: use memory.DetectDefaults
func detectDefaults() (map[string]string, string, *spdInfo) {
	vals, msg, spd := memory.DetectDefaults()
	// Convert memory.SPDInfo → local spdInfo
	var out *spdInfo
	if spd != nil {
		out = &spdInfo{
			moduleManufacturer: spd.ModuleManufacturer,
			dramManufacturer:   spd.DRAMManufacturer,
			partNumber:         spd.PartNumber,
			moduleType:         spd.ModuleType,
			memoryType:         spd.MemoryType,
			moduleSpeed:        spd.ModuleSpeed,
			sizeMB:             spd.SizeMB,
			ranks:              spd.Ranks,
			bankLayout:         spd.BankLayout,
			busWidth:           spd.BusWidth,
			deviceWidth:        spd.DeviceWidth,
			supportedCL:        spd.SupportedCL,
			voltage:            spd.Voltage,
			timingString:       spd.TimingString,
			timingsBySpeed:     spd.TimingsBySpeed,
			tWR:                spd.TWR,
			tRRD:               spd.TRRD,
			tRC:                spd.TRC,
			tWTR:               spd.TWTR,
			tRTP:               spd.TRTP,
			tFAW:               spd.TFAW,
			tCKmin:             spd.TCKmin,
			dllOff:             spd.DLLOff,
			tempRange:          spd.TempRange,
			autoSR:             spd.AutoSR,
			moduleHeight:       spd.ModuleHeight,
			refCard:            spd.RefCard,
			numDIMMs:           spd.NumDIMMs,
		}
	}
	return vals, msg, out
}
func newField(base textinput.Model, label, value, desc string) timingField {
	in := base
	in.SetValue(value)
	in.Placeholder = value
	return timingField{label: label, hint: value, desc: desc, input: in}
}
func readMCHBARTimings() ([]tertiaryTimings, error) { return nil, nil }

// --- Types and Structs ---
type timingField struct {
	label string
	hint  string
	desc  string
	input textinput.Model
}

type benchMetric struct {
	label string
	value float64
}

type benchResults struct {
	readRate  float64
	writeRate float64
	metrics   []benchMetric
	duration  string
}

type burnInResults struct {
	duration string
	errors   int
	success  bool
}

type benchResultMsg struct {
	results *benchResults
	err     error
}

type burnInResultMsg struct {
	results *burnInResults
	err     error
}

type spdInfo struct {
	moduleManufacturer string
	dramManufacturer   string
	partNumber         string
	moduleType         string
	memoryType         string
	moduleSpeed        string
	sizeMB             string
	ranks              string
	bankLayout         string
	busWidth           string
	deviceWidth        string
	supportedCL        string
	voltage            string
	timingString       string
	timingsBySpeed     []string
	tWR                string
	tRRD               string
	tRC                string
	tWTR               string
	tRTP               string
	tFAW               string
	tCKmin             string
	dllOff             string
	tempRange          string
	autoSR             string
	moduleHeight       string
	refCard            string
	numDIMMs           string
}

// --- Model struct ---
type Model struct {
	fields        []timingField
	focusIndex    int
	focusType     focusTarget
	lockRatios    bool
	width         int
	height        int
	status        string
	activeTab     int
	benchRunning  bool
	burnInRunning bool
	benchResults  *benchResults
	burnInResults *burnInResults
	spd           *spdInfo
	imcTimings    []tertiaryTimings
}

// --- Focus target for input navigation and actions ---
type focusTarget int

const (
	focusField focusTarget = iota
	focusApply
	focusCancel
)

// --- Exported constructor ---
func InitialModel() Model {
	detected, detectMsg, spd := detectDefaults()

	get := func(label, fallback string) string {
		if v, ok := detected[label]; ok && v != "" {
			return v
		}
		return fallback
	}

	baseInput := textinput.New()
	baseInput.CharLimit = 8
	baseInput.Width = 8

	fields := []timingField{
		newField(baseInput, "Channels", get("Channels", "2"), "Detected memory channels."),
		newField(baseInput, "DRAM Ratio", get("DRAM Ratio", "?"), "Memory multiplier relative to base clock."),
		newField(baseInput, "DRAM Freq", get("DRAM Freq", "1600"), "Effective DRAM frequency in MT/s."),
		newField(baseInput, "CAS (tCL)", get("CAS (tCL)", "?"), "CAS latency cycles."),
		newField(baseInput, "tRCD", get("tRCD", "?"), "RAS to CAS delay cycles."),
		newField(baseInput, "tRP", get("tRP", "?"), "Row precharge delay cycles."),
		newField(baseInput, "tRAS", get("tRAS", "?"), "Active to precharge delay cycles."),
		newField(baseInput, "Command Rate", get("Command Rate", "2T"), "Command rate; usually 1T or 2T."),
		newField(baseInput, "tRFC", get("tRFC", "?"), "Refresh cycle timing."),
		newField(baseInput, "DRAM Voltage", get("DRAM Voltage", "?"), "Requested memory rail voltage."),
	}

	fields[0].input.Focus()
	fields[0].input.PromptStyle = style.Safe
	fields[0].input.TextStyle = style.Heading

	imcTimings, _ := readMCHBARTimings()

	return Model{
		fields:     fields,
		focusIndex: 0,
		focusType:  focusField,
		lockRatios: true,
		status:     detectMsg + " | Tab/Shift+Tab to move, L toggles ratio lock.",
		spd:        spd,
		imcTimings: imcTimings,
	}
}

// --- Helper and method implementations ---

// parseUnsignedInt parses a string to a positive int, returns (0, false) if invalid.
func parseUnsignedInt(s string) (int, bool) {
	clean := sanitizeNumeric(s)
	if clean == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, false
	}
	if v <= 0 {
		return 0, false
	}
	return int(v), true
}

// fieldIndex returns the index of a field with the given label, or -1 if not found.
func (m *Model) fieldIndex(label string) int {
	for i, f := range m.fields {
		if f.label == label {
			return i
		}
	}
	return -1
}

func (m Model) renderTabBar() string {
	tabs := []string{"System info", "Benchmark", "Burn-in"}
	var rendered []string
	for i, tab := range tabs {
		if m.activeTab == i {
			rendered = append(rendered, style.ActiveTab.Render(tab))
		} else {
			rendered = append(rendered, style.InactiveTab.Render(tab))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

func (m Model) renderSysInfoTab() string {
	leftWidth := (m.width - 8) * 2 / 3
	rightWidth := m.width - leftWidth - 8
	if leftWidth < 50 {
		leftWidth = 50
	}
	if rightWidth < 32 {
		rightWidth = 32
	}

	leftPanel := style.Panel.Width(leftWidth).Render(m.renderTimingEditor(leftWidth - 4))
	rightPanel := style.Panel.Width(rightWidth).Render(m.renderGuidePanel(rightWidth - 4))

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)

	parts := []string{topRow}

	fullWidth := m.width - 6
	if fullWidth < 60 {
		fullWidth = 60
	}

	if m.spd != nil {
		parts = append(parts, "", style.InfoPanel.Width(fullWidth).Render(m.renderSPDInfoPanel(fullWidth-4)))
	}

	if len(m.imcTimings) > 0 {
		parts = append(parts, "", style.InfoPanel.Width(fullWidth).Render(m.renderIMCPanel()))
	}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m Model) renderBurnInTab() string {
	fullWidth := m.width - 8
	if fullWidth < 60 {
		fullWidth = 60
	}

	var sections []string

	if m.burnInRunning {
		sections = append(sections, style.RunningBtn.Render(" Running... (30s) "))
	} else {
		sections = append(sections, style.RunBtn.Render(" Enter — Run Burn-in "))
	}
	sections = append(sections, "")

	return style.Panel.Width(fullWidth).Render(strings.Join(sections, "\n"))
}

func (m Model) renderBenchmarkTab() string {
	fullWidth := m.width - 8
	if fullWidth < 60 {
		fullWidth = 60
	}

	var sections []string

	if m.benchRunning {
		sections = append(sections, style.RunningBtn.Render(" Running... (30s) "))
	} else {
		sections = append(sections, style.RunBtn.Render(" Enter — Run Benchmark "))
	}
	sections = append(sections, "")

	if m.benchResults == nil {
		sections = append(sections,
			style.Hint.Render("Run a stress-ng memrate benchmark to measure actual memory throughput."),
			style.Hint.Render("Results will be compared against theoretical peak bandwidth."),
			"",
			style.Hint.Render("Command: stress-ng --memrate 1 --memrate-bytes 256M -t 30 --metrics"),
		)
	} else {
		sections = append(sections, m.renderBenchResults()...)
	}

	return style.Panel.Width(fullWidth).Render(strings.Join(sections, "\n"))
}

// Ensure Model implements tea.Model
func (m Model) Init() tea.Cmd {
	return nil // or textinput.Blink if desired
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case benchResultMsg:
		m.benchRunning = false
		if msg.err != nil {
			m.status = fmt.Sprintf("Benchmark failed: %v", msg.err)
		} else {
			m.benchResults = msg.results
			m.status = "Benchmark complete. Results shown below."
		}
		return m, nil
	case burnInResultMsg:
		m.burnInRunning = false
		if msg.err != nil {
			m.status = fmt.Sprintf("Burn-in failed: %v", msg.err)
		} else {
			m.burnInResults = msg.results
			m.status = "Burn-in complete. Results shown below."
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "f1":
			m.activeTab = 0
			m.status = "Tab/Shift+Tab to move, L toggles ratio lock, Enter to apply, Esc to cancel changes."
			return m, nil
		case "f2":
			m.activeTab = 1
			if m.benchResults != nil {
				m.status = "Benchmark results shown. Press Enter to re-run."
			} else {
				m.status = "Press Enter to run memrate benchmark (30s)."
			}
			return m, nil
		case "f3":
			m.activeTab = 2
			if m.burnInResults != nil {
				m.status = "Burn-in results shown. Press Enter to re-run."
			} else {
				m.status = "Press Enter to run burn-in test (30s)."
			}
			return m, nil
		case "l":
			if m.activeTab == 0 {
				m.lockRatios = !m.lockRatios
				if m.lockRatios {
					m.status = "Ratio lock enabled. Frequency edits auto-scale primary timings."
				} else {
					m.status = "Ratio lock disabled. Timings can be tuned independently."
				}
			}
			return m, nil
		case "tab":
			if m.activeTab == 0 {
				m.moveFocus(1)
			}
			return m, nil
		case "shift+tab":
			if m.activeTab == 0 {
				m.moveFocus(-1)
			}
			return m, nil
		case "esc":
			if m.activeTab == 0 {
				m.resetValues()
				m.status = "Changes canceled."
			}
			return m, nil
		case "enter":
			if m.activeTab == 1 {
				if !m.benchRunning {
					m.benchRunning = true
					m.status = "Running memrate benchmark (30s)..."
					return m, runBenchmarkCmd
				}
				return m, nil
			}
			if m.focusType == focusApply {
				m.status = "Applied (mock): " + m.snapshotSummary()
				return m, nil
			}
			if m.focusType == focusCancel {
				m.resetValues()
				m.status = "Changes canceled."
				return m, nil
			}
		}
	}

	if m.activeTab == 0 && m.focusType == focusField {
		idx := m.focusIndex
		before := m.fields[idx].input.Value()
		updated, cmd := m.fields[idx].input.Update(msg)
		m.fields[idx].input = updated
		after := m.fields[idx].input.Value()

		if m.fields[idx].label == "DRAM Freq" && before != after {
			m.handleFrequencyEdit()
		}

		return m, cmd
	}

	return m, nil
}

/* --------------- Render Methods --------------- */

func (m Model) renderTimingEditor(width int) string {
	tableHeader := lipgloss.JoinHorizontal(
		lipgloss.Left,
		style.Label.Render("Setting"),
		style.ColHeader.Width(10).Render("Original"),
		style.ColHeader.Width(10).Render("Current"),
		style.ColHeader.Width(14).Render("Safe Range"),
	)

	sections := []string{
		tableHeader,
		"",
		style.SectionTitle.Render("General"),
		m.renderFields(0, 3),
		"",
		style.SectionTitle.Render("Primary Timings"),
		m.renderFields(3, 7),
		"",
		style.SectionTitle.Render("Secondary / Voltage"),
		m.renderFields(7, len(m.fields)),
		"",
		m.renderButtons(),
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(sections, "\n"))
}

func (m Model) renderFields(start, end int) string {
	rows := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		bounds := m.safeBounds(m.fields[i].label)
		inputWidth := lipgloss.NewStyle().Width(10)
		row := lipgloss.JoinHorizontal(
			lipgloss.Left,
			style.Label.Render(m.fields[i].label),
			style.OriginalValue.Render(m.fields[i].hint),
			inputWidth.Render(m.fields[i].input.View()),
			m.renderBounds(m.fields[i], bounds),
		)
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
}

type fieldBounds struct {
	min    int
	max    int
	note   string
	hasMin bool
	hasMax bool
}

func (m Model) safeBounds(label string) fieldBounds {
	freq, okF := m.parseNumericField("DRAM Freq")
	if !okF || freq <= 0 {
		freq = 1600
	}
	tclVal, _ := m.parseNumericField("CAS (tCL)")
	trcdVal, _ := m.parseNumericField("tRCD")
	switch label {
	case "CAS (tCL)":
		minCL := ceilDiv(int(freq)*10, 2000)
		if minCL < 5 {
			minCL = 5
		}
		return fieldBounds{min: minCL, max: 18, hasMin: true, hasMax: true}
	case "tRCD":
		minRCD := ceilDiv(int(freq)*10, 2000)
		if minRCD < 5 {
			minRCD = 5
		}
		return fieldBounds{min: minRCD, max: 18, hasMin: true, hasMax: true}
	case "tRP":
		minRP := ceilDiv(int(freq)*10, 2000)
		if minRP < 5 {
			minRP = 5
		}
		return fieldBounds{min: minRP, max: 18, hasMin: true, hasMax: true}
	case "tRAS":
		minFormula := int(tclVal + trcdVal)
		minNs := ceilDiv(int(freq)*35, 2000)
		minRAS := minFormula
		if minNs > minRAS {
			minRAS = minNs
		}
		if minRAS < 15 {
			minRAS = 15
		}
		return fieldBounds{min: minRAS, max: 63, hasMin: true, hasMax: true, note: "≥CL+tRCD"}
	case "tRFC":
		minRFC := ceilDiv(int(freq)*160, 2000)
		return fieldBounds{min: minRFC, max: 511, hasMin: true, hasMax: true, note: "≥160ns"}
	case "Command Rate":
		return fieldBounds{min: 1, max: 2, hasMin: true, hasMax: true, note: "1T/2T"}
	case "DRAM Voltage":
		return fieldBounds{min: 1, max: 1, hasMin: true, hasMax: true, note: "1.35-1.65V"}
	default:
		return fieldBounds{}
	}
}

func (m Model) renderBounds(f timingField, b fieldBounds) string {
	if !b.hasMin && !b.hasMax {
		return style.Bounds.Render("")
	}
	var rangeStr string
	if f.label == "DRAM Voltage" || f.label == "Command Rate" {
		rangeStr = b.note
	} else if b.hasMin && b.hasMax {
		rangeStr = fmt.Sprintf("%d-%d", b.min, b.max)
		if b.note != "" {
			rangeStr += " " + b.note
		}
	} else if b.hasMin {
		rangeStr = fmt.Sprintf("≥%d", b.min)
	}
	val, ok := parseUnsignedInt(f.input.Value())
	if !ok {
		return style.Bounds.Render(rangeStr)
	}
	switch {
	case b.hasMin && val < b.min:
		return style.Unsafe.Width(14).Render(rangeStr)
	case b.hasMax && val > b.max:
		return style.Warn.Width(14).Render(rangeStr)
	default:
		return style.Safe.Width(14).Render(rangeStr)
	}
}

func ceilDiv(a, b int) int {
	return (a + b - 1) / b
}

func (m Model) renderButtons() string {
	apply := style.Btn.Render("Apply")
	cancel := style.Btn.Render("Cancel")
	lock := style.LockOff.Render("Ratio Lock OFF")
	if m.lockRatios {
		lock = style.LockOn.Render("Ratio Lock ON")
	}
	if m.focusType == focusApply {
		apply = style.BtnFocus.Render("Apply")
	}
	if m.focusType == focusCancel {
		cancel = style.BtnFocus.Render("Cancel")
	}
	controls := lipgloss.JoinHorizontal(lipgloss.Top, apply, " ", cancel, "   ", lock)
	return style.SectionTitle.Render("Actions") + "\n" + controls
}

func (m Model) renderGuidePanel(width int) string {
	focused := m.fields[m.focusIndex]
	projection := m.renderPerfProjection()
	guide := []string{
		style.SectionTitle.Render("Perf Projection"),
		projection,
		"",
		style.SectionTitle.Render("Guidance"),
		style.ResultValue.Render(focused.label),
		focused.desc,
		"",
		style.SectionTitle.Render("Hints"),
		style.Hint.Render("- Keep small, incremental changes."),
		style.Hint.Render("- Test stability after each apply."),
		style.Hint.Render("- Prefer JEDEC/XMP-safe bounds."),
		"",
		style.SectionTitle.Render("Keys"),
		style.Hint.Render("F1 / F2 / F3       Switch tabs"),
		style.Hint.Render("Tab / Shift+Tab    Move focus"),
		style.Hint.Render("Enter              Activate action"),
		style.Hint.Render("L                  Toggle ratio lock"),
		style.Hint.Render("Esc                Cancel edits"),
		style.Hint.Render("Q                  Quit"),
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(guide, "\n"))
}

func (m Model) renderPerfProjection() string {
	freqMT, okFreq := m.parseNumericField("DRAM Freq")
	channels, okCh := m.parseNumericField("Channels")
	tcl, okCL := m.parseNumericField("CAS (tCL)")
	if !okFreq || !okCh {
		return style.MetricMuted.Render("Enter numeric DRAM Freq and Channels to estimate bandwidth.")
	}
	perChannelGBs := freqMT * 8.0 / 1000.0
	totalGBs := perChannelGBs * channels
	realisticGBs := totalGBs * 0.82
	rows := []string{
		style.Metric.Render(fmt.Sprintf("Peak Bandwidth: %.2f GB/s", totalGBs)),
		style.MetricMuted.Render(fmt.Sprintf("Per Channel:    %.2f GB/s", perChannelGBs)),
		style.MetricMuted.Render(fmt.Sprintf("Est. Sustained: %.2f GB/s (82%% eff.)", realisticGBs)),
	}
	if okCL && freqMT > 0 {
		casNs := (2000.0 * tcl) / freqMT
		rows = append(rows, style.MetricMuted.Render(fmt.Sprintf("CAS Latency:    %.2f ns", casNs)))
	}
	return strings.Join(rows, "\n")
}

func (m Model) renderBenchResults() []string {
	r := m.benchResults
	peakMBs := m.theoreticalPeakMBs()
	header := lipgloss.JoinHorizontal(lipgloss.Left,
		style.ResultLabel.Render("Metric"),
		style.ResultValue.Render("Measured"),
		style.ResultTheo.Render("Theoretical"),
		style.ColHeader.Width(12).Render("Efficiency"),
	)
	rows := []string{
		style.SectionTitle.Render("Summary"),
		header,
		m.renderResultRow("Read Rate (avg)", r.readRate, peakMBs),
		m.renderResultRow("Write Rate (avg)", r.writeRate, peakMBs),
		"",
	}
	if len(r.metrics) > 0 {
		rows = append(rows, style.SectionTitle.Render("Detailed Rates (MB/s)"))
		for _, metric := range r.metrics {
			rows = append(rows, m.renderResultRow(metric.label, metric.value, peakMBs))
		}
		rows = append(rows, "")
	}
	rows = append(rows,
		style.SectionTitle.Render("Theoretical Basis"),
		style.MetricMuted.Render(fmt.Sprintf("Peak Bandwidth: %.0f MB/s (%.2f GB/s)", peakMBs, peakMBs/1000)),
	)
	return rows
}

func (m Model) renderResultRow(label string, measured, theoretical float64) string {
	eff := 0.0
	if theoretical > 0 {
		eff = (measured / theoretical) * 100
	}
	effStr := fmt.Sprintf("%.1f%%", eff)
	var styledEff string
	switch {
	case eff >= 70:
		styledEff = style.EffGood.Width(12).Render(effStr)
	case eff >= 40:
		styledEff = style.EffOk.Width(12).Render(effStr)
	default:
		styledEff = style.EffLow.Width(12).Render(effStr)
	}
	return lipgloss.JoinHorizontal(lipgloss.Left,
		style.ResultLabel.Render(label),
		style.ResultValue.Render(fmt.Sprintf("%.0f", measured)),
		style.ResultTheo.Render(fmt.Sprintf("%.0f", theoretical)),
		styledEff,
	)
}

func (m Model) theoreticalPeakMBs() float64 {
	freq, okF := m.parseNumericField("DRAM Freq")
	ch, okC := m.parseNumericField("Channels")
	if !okF || !okC || freq <= 0 || ch <= 0 {
		return 0
	}
	return freq * 8.0 * ch
}

func (m Model) renderIMCPanel() string {
	heading := style.SectionTitle.Render("IMC Tertiary Timings (Haswell)") + "  " +
		style.Hint.Render("(read-only, from MCHBAR registers)")
	var chSections []string
	for _, t := range m.imcTimings {
		chSections = append(chSections, renderChannelIMC(t))
	}
	return lipgloss.JoinVertical(lipgloss.Left, heading, "", strings.Join(chSections, "\n\n"))
}

func (m Model) renderSPDInfoPanel(width int) string {
	lines := []string{
		style.SectionTitle.Render("SPD Information"),
		style.InfoLabel.Render("Module:") + " " + style.InfoValue.Render(m.spd.moduleManufacturer+" "+m.spd.partNumber),
		style.InfoLabel.Render("DRAM:") + " " + style.InfoValue.Render(m.spd.dramManufacturer+" "+m.spd.memoryType),
		style.InfoLabel.Render("Size:") + " " + style.InfoValue.Render(m.spd.sizeMB+" MB ("+m.spd.ranks+" ranks)"),
		style.InfoLabel.Render("Speed:") + " " + style.InfoValue.Render(m.spd.moduleSpeed),
		style.InfoLabel.Render("Timings:") + " " + style.InfoValue.Render(m.spd.timingString),
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func renderChannelIMC(t tertiaryTimings) string {
	matHdr := style.ColHeader.Render(fmt.Sprintf("  %-10s %4s %4s %4s %4s", "", "sg", "dg", "dr", "dd"))
	matRow := func(name string, sg, dg, dr, dd int) string {
		return fmt.Sprintf("  %-10s %4d %4d %4d %4d", name, sg, dg, dr, dd)
	}
	imcRow := func(label string, val int) string {
		return fmt.Sprintf("  %-12s %d", label, val)
	}
	left := strings.Join([]string{
		style.SectionTitle.Render(fmt.Sprintf("Channel %d — Turnaround", t.channel)),
		matHdr,
		matRow("tRDRD", t.tRDRDsg, t.tRDRDdg, t.tRDRDdr, t.tRDRDdd),
		matRow("tWRWR", t.tWRWRsg, t.tWRWRdg, t.tWRWRdr, t.tWRWRdd),
		matRow("tRDWR", t.tRDWRsg, t.tRDWRdg, t.tRDWRdr, t.tRDWRdd),
		matRow("tWRRD", t.tWRRDsg, t.tWRRDdg, t.tWRRDdr, t.tWRRDdd),
	}, "\n")
	right := strings.Join([]string{
		style.SectionTitle.Render("IMC Register Values"),
		imcRow("tRAS", t.tRAS),
		imcRow("tRCD", t.tRCD),
		imcRow("tRP", t.tRP),
		imcRow("tRTP", t.tRTP),
		imcRow("tWTR", t.tWTR),
		imcRow("tRRD sg/dg", t.tRRDsg) + " / " + strconv.Itoa(t.tRRDdg),
		imcRow("tCKE", t.tCKE),
		imcRow("tREFI", t.tREFI),
		imcRow("tRFC", t.tRFC),
	}, "\n")
	leftStyled := lipgloss.NewStyle().Width(38).Render(left)
	rightStyled := lipgloss.NewStyle().Width(30).Render(right)
	return lipgloss.JoinHorizontal(lipgloss.Top, leftStyled, "  ", rightStyled)
}
