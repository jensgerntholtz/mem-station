package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// main.go now only contains the CLI entrypoint and legacy CLI logic.
// The TUI model and helpers have been moved to internal/model.

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

// fieldIndex returns index of field with label, or -1 if not found.
func (m model) fieldIndex(label string) int {
	for i, f := range m.fields {
		if f.label == label {
			return i
		}
	}
	return -1
}

// snapshotSummary returns a short summary of current field values.
func (m model) snapshotSummary() string {
	summary := []string{}
	for _, f := range m.fields {
		summary = append(summary, fmt.Sprintf("%s=%s", f.label, f.input.Value()))
	}
	if len(summary) > 3 {
		return strings.Join(summary[:3], ", ") + ", ..."
	}
	return strings.Join(summary, ", ")
}

// View implements tea.Model interface for model.
func (m model) View() string {
	if m.width == 0 {
		m.width = 120
	}

	heading := StyleHeading.Render(" MemStation ") + "  " +
		StyleSubHeading.Render("Memory Timing Workspace")

	tabBar := m.renderTabBar()

	var content string
	if m.activeTab == 0 {
		content = m.renderSysInfoTab()
	} else {
		content = m.renderBenchmarkTab()
	}

	status := StyleStatus.Render(m.status)

	ui := lipgloss.JoinVertical(lipgloss.Left, heading, tabBar, "", content, "", status)
	return StylePaddingApp.Render(ui)
}

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

type benchResultMsg struct {
	results *benchResults
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

type model struct {
	fields       []timingField
	focusIndex   int
	focusType    focusTarget
	lockRatios   bool
	width        int
	height       int
	status       string
	activeTab    int
	benchRunning bool
	benchResults *benchResults
	spd          *spdInfo
	imcTimings   []tertiaryTimings
}

// detectDefaults detects default values from hardware.
func detectDefaults() (map[string]string, string, *spdInfo) {
	vals := make(map[string]string)

	out, err := exec.Command("dmidecode", "--type", "17", "--type", "4").CombinedOutput()
	if err != nil {
		return vals, "Auto-detect failed (try: sudo ./mem-station)", nil
	}

	dmi := string(out)

	// Parse populated memory devices from type 17 blocks
	blocks := strings.Split(dmi, "Memory Device")
	chSet := make(map[string]bool)
	confSpeedRe := regexp.MustCompile(`Configured Memory Speed:\s+(\d+)\s+MT/s`)
	speedRe := regexp.MustCompile(`\bSpeed:\s+(\d+)\s+MT/s`)
	voltRe := regexp.MustCompile(`Configured Voltage:\s+([\d.]+)\s+V`)
	locRe := regexp.MustCompile(`Locator:\s+(\S+)`)

	for _, block := range blocks[1:] {
		if strings.Contains(block, "No Module Installed") {
			continue
		}

		// Frequency: prefer Configured Memory Speed, fall back to Speed
		if _, ok := vals["DRAM Freq"]; !ok {
			if m := confSpeedRe.FindStringSubmatch(block); len(m) > 1 {
				vals["DRAM Freq"] = m[1]
			} else if m := speedRe.FindStringSubmatch(block); len(m) > 1 {
				vals["DRAM Freq"] = m[1]
			}
		}

		// Voltage (skip "Unknown")
		if _, ok := vals["DRAM Voltage"]; !ok {
			if m := voltRe.FindStringSubmatch(block); len(m) > 1 {
				vals["DRAM Voltage"] = m[1]
			}
		}

		// Channel from Locator (ChannelA-DIMM0, DIMM_A1, A0, etc.)
		if m := locRe.FindStringSubmatch(block); len(m) > 1 {
			loc := strings.ToUpper(m[1])
			for _, ch := range []string{"A", "B", "C", "D"} {
				if strings.Contains(loc, "CHANNEL"+ch) ||
					strings.Contains(loc, "_"+ch) ||
					strings.HasPrefix(loc, ch) {
					chSet[ch] = true
					break
				}
			}
		}
	}

	if len(chSet) > 0 {
		vals["Channels"] = strconv.Itoa(len(chSet))
	}

	// BCLK and DRAM ratio from Processor Information (type 4)
	bclkRe := regexp.MustCompile(`External Clock:\s+(\d+)\s+MHz`)
	if m := bclkRe.FindStringSubmatch(dmi); len(m) > 1 {
		bclk, _ := strconv.Atoi(m[1])
		if bclk > 0 {
			if freqStr, ok := vals["DRAM Freq"]; ok {
				freq, _ := strconv.Atoi(freqStr)
				if freq > 0 {
					vals["DRAM Ratio"] = strconv.Itoa(freq / bclk)
				}
			}
		}
	}

	// Try decode-dimms for SPD timing data
	var spd *spdInfo
	if spdOut, spdErr := exec.Command("decode-dimms").CombinedOutput(); spdErr == nil {
		parseSPDTimings(string(spdOut), vals)
		spd = parseSPDInfo(string(spdOut))
	}

	count := len(vals)
	return vals, fmt.Sprintf("Detected %d params from hardware.", count), spd
}

func parseSPDTimings(spd string, vals map[string]string) {
	freqStr, ok := vals["DRAM Freq"]
	if !ok {
		return
	}
	freqMT, err := strconv.ParseFloat(freqStr, 64)
	if err != nil || freqMT <= 0 {
		return
	}
	freqMHz := freqMT / 2.0

	timingPatterns := []struct {
		field string
		re    *regexp.Regexp
	}{
		{"CAS (tCL)", regexp.MustCompile(`(?i)(?:CAS Latency|Minimum CAS Latency|tAA)\D+?(\d+\.?\d*)\s*ns`)},
		{"tRCD", regexp.MustCompile(`(?i)(?:RAS.to.CAS|tRCD)\D+?(\d+\.?\d*)\s*ns`)},
		{"tRP", regexp.MustCompile(`(?i)(?:Row.Precharge|tRP)\D+?(\d+\.?\d*)\s*ns`)},
		{"tRAS", regexp.MustCompile(`(?i)(?:Active.to.Precharge|tRAS)\D+?(\d+\.?\d*)\s*ns`)},
		{"tRFC", regexp.MustCompile(`(?i)(?:Refresh.Recovery|tRFC)\D+?(\d+\.?\d*)\s*ns`)},
	}

	for _, tp := range timingPatterns {
		if m := tp.re.FindStringSubmatch(spd); len(m) > 1 {
			ns, err := strconv.ParseFloat(m[1], 64)
			if err != nil || ns <= 0 {
				continue
			}
			cycles := int(math.Ceil(ns * freqMHz / 1000.0))
			vals[tp.field] = strconv.Itoa(cycles)
		}
	}
}

func initialModel() model {
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
	fields[0].input.PromptStyle = StyleSafe
	fields[0].input.TextStyle = StyleHeading

	imcTimings, _ := readMCHBARTimings()

	return model{
		fields:     fields,
		focusIndex: 0,
		focusType:  focusField,
		lockRatios: true,
		status:     detectMsg + " | Tab/Shift+Tab to move, L toggles ratio lock.",
		spd:        spd,
		imcTimings: imcTimings,
	}
}

func newField(base textinput.Model, label, value, desc string) timingField {
	in := base
	in.SetValue(value)
	in.Placeholder = value
	return timingField{
		label: label,
		hint:  value,
		desc:  desc,
		input: in,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m *model) moveFocus(delta int) {
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

func (m model) focusPosition() int {
	switch m.focusType {
	case focusField:
		return m.focusIndex
	case focusApply:
		return len(m.fields)
	default:
		return len(m.fields) + 1
	}
}

func (m *model) resetValues() {
	for i := range m.fields {
		m.fields[i].input.SetValue(m.fields[i].hint)
	}
}

func (m *model) handleFrequencyEdit() {
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

func (m *model) scaleTimingField(label string, ratio float64) {
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
	m.fields[idx].input.SetValue(strconv.Itoa(scaled))
}

func (m model) originalNumericField(label string) float64 {
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

func (m model) renderTimingEditor(width int) string {
	tableHeader := lipgloss.JoinHorizontal(
		lipgloss.Left,
		StyleLabel.Render("Setting"),
		StyleColHeader.Width(10).Render("Original"),
		StyleColHeader.Width(10).Render("Current"),
		StyleColHeader.Width(14).Render("Safe Range"),
	)

	sections := []string{
		tableHeader,
		"",
		StyleSectionTitle.Render("General"),
		m.renderFields(0, 3),
		"",
		StyleSectionTitle.Render("Primary Timings"),
		m.renderFields(3, 7),
		"",
		StyleSectionTitle.Render("Secondary / Voltage"),
		m.renderFields(7, len(m.fields)),
		"",
		m.renderButtons(),
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(sections, "\n"))
}

func (m model) renderFields(start, end int) string {
	rows := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		bounds := m.safeBounds(m.fields[i].label)
		inputWidth := lipgloss.NewStyle().Width(10)
		row := lipgloss.JoinHorizontal(
			lipgloss.Left,
			StyleLabel.Render(m.fields[i].label),
			StyleOriginalValue.Render(m.fields[i].hint),
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

func (m model) safeBounds(label string) fieldBounds {
	freq, okF := m.parseNumericField("DRAM Freq")
	if !okF || freq <= 0 {
		freq = 1600
	}

	tclVal, _ := m.parseNumericField("CAS (tCL)")
	trcdVal, _ := m.parseNumericField("tRCD")

	switch label {
	case "CAS (tCL)":
		// JEDEC DDR3: tCL(ns) ≥ 10ns → cycles = ceil(10 * freq / 2000)
		minCL := ceilDiv(int(freq)*10, 2000)
		if minCL < 5 {
			minCL = 5
		}
		return fieldBounds{min: minCL, max: 18, hasMin: true, hasMax: true}
	case "tRCD":
		// JEDEC: tRCD(ns) ≥ 10ns
		minRCD := ceilDiv(int(freq)*10, 2000)
		if minRCD < 5 {
			minRCD = 5
		}
		return fieldBounds{min: minRCD, max: 18, hasMin: true, hasMax: true}
	case "tRP":
		// JEDEC: tRP(ns) ≥ 10ns
		minRP := ceilDiv(int(freq)*10, 2000)
		if minRP < 5 {
			minRP = 5
		}
		return fieldBounds{min: minRP, max: 18, hasMin: true, hasMax: true}
	case "tRAS":
		// JEDEC: tRAS ≥ tCL + tRCD; also tRAS(ns) ≥ 35ns
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
		// DDR3 4Gbit: tRFC ≥ 260ns; 2Gbit: ≥160ns. Use 160ns as safe floor.
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

func (m model) renderBounds(f timingField, b fieldBounds) string {
	if !b.hasMin && !b.hasMax {
		return StyleBounds.Render("")
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

	// Color based on whether current value is in range
	val, ok := parseUnsignedInt(f.input.Value())
	if !ok {
		return StyleBounds.Render(rangeStr)
	}

	switch {
	case b.hasMin && val < b.min:
		return StyleUnsafe.Width(14).Render(rangeStr)
	case b.hasMax && val > b.max:
		return StyleWarn.Width(14).Render(rangeStr)
	default:
		return StyleSafe.Width(14).Render(rangeStr)
	}
}

func ceilDiv(a, b int) int {
	return (a + b - 1) / b
}

func (m model) renderButtons() string {
	apply := StyleBtn.Render("Apply")
	cancel := StyleBtn.Render("Cancel")
	lock := StyleLockOff.Render("Ratio Lock OFF")
	if m.lockRatios {
		lock = StyleLockOn.Render("Ratio Lock ON")
	}

	if m.focusType == focusApply {
		apply = StyleBtnFocus.Render("Apply")
	}
	if m.focusType == focusCancel {
		cancel = StyleBtnFocus.Render("Cancel")
	}

	controls := lipgloss.JoinHorizontal(lipgloss.Top, apply, " ", cancel, "   ", lock)
	return StyleSectionTitle.Render("Actions") + "\n" + controls
}

func (m model) renderGuidePanel(width int) string {
	focused := m.fields[m.focusIndex]
	projection := m.renderPerfProjection()
	guide := []string{
		StyleSectionTitle.Render("Perf Projection"),
		projection,
		"",
		StyleSectionTitle.Render("Guidance"),
		StyleResultValue.Render(focused.label),
		focused.desc,
		"",
		StyleSectionTitle.Render("Hints"),
		StyleHint.Render("- Keep small, incremental changes."),
		StyleHint.Render("- Test stability after each apply."),
		StyleHint.Render("- Prefer JEDEC/XMP-safe bounds."),
		"",
		StyleSectionTitle.Render("Keys"),
		StyleHint.Render("F1 / F2            Switch tabs"),
		StyleHint.Render("Tab / Shift+Tab    Move focus"),
		StyleHint.Render("Enter              Activate action"),
		StyleHint.Render("L                  Toggle ratio lock"),
		StyleHint.Render("Esc                Cancel edits"),
		StyleHint.Render("Q                  Quit"),
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(guide, "\n"))
}

func (m model) renderPerfProjection() string {
	freqMT, okFreq := m.parseNumericField("DRAM Freq")
	channels, okCh := m.parseNumericField("Channels")
	tcl, okCL := m.parseNumericField("CAS (tCL)")

	if !okFreq || !okCh {
		return StyleMetricMuted.Render("Enter numeric DRAM Freq and Channels to estimate bandwidth.")
	}

	perChannelGBs := freqMT * 8.0 / 1000.0
	totalGBs := perChannelGBs * channels
	realisticGBs := totalGBs * 0.82

	rows := []string{
		StyleMetric.Render(fmt.Sprintf("Peak Bandwidth: %.2f GB/s", totalGBs)),
		StyleMetricMuted.Render(fmt.Sprintf("Per Channel:    %.2f GB/s", perChannelGBs)),
		StyleMetricMuted.Render(fmt.Sprintf("Est. Sustained: %.2f GB/s (82%% eff.)", realisticGBs)),
	}

	if okCL && freqMT > 0 {
		casNs := (2000.0 * tcl) / freqMT
		rows = append(rows, StyleMetricMuted.Render(fmt.Sprintf("CAS Latency:    %.2f ns", casNs)))
	}

	return strings.Join(rows, "\n")
}

func (m model) parseNumericField(label string) (float64, bool) {
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

func sanitizeNumeric(s string) string {
	b := strings.Builder{}
	hasDot := false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' && !hasDot:
			hasDot = true
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (m model) renderBenchResults() []string {
	r := m.benchResults
	peakMBs := m.theoreticalPeakMBs()

	header := lipgloss.JoinHorizontal(lipgloss.Left,
		StyleResultLabel.Render("Metric"),
		StyleResultValue.Render("Measured"),
		StyleResultTheo.Render("Theoretical"),
		StyleColHeader.Width(12).Render("Efficiency"),
	)

	rows := []string{
		StyleSectionTitle.Render("Summary"),
		header,
		m.renderResultRow("Read Rate (avg)", r.readRate, peakMBs),
		m.renderResultRow("Write Rate (avg)", r.writeRate, peakMBs),
		"",
	}

	if len(r.metrics) > 0 {
		rows = append(rows, StyleSectionTitle.Render("Detailed Rates (MB/s)"))
		for _, metric := range r.metrics {
			rows = append(rows, m.renderResultRow(metric.label, metric.value, peakMBs))
		}
		rows = append(rows, "")
	}

	rows = append(rows,
		StyleSectionTitle.Render("Theoretical Basis"),
		StyleMetricMuted.Render(fmt.Sprintf("Peak Bandwidth: %.0f MB/s (%.2f GB/s)", peakMBs, peakMBs/1000)),
	)

	return rows
}

func (m model) renderResultRow(label string, measured, theoretical float64) string {
	eff := 0.0
	if theoretical > 0 {
		eff = (measured / theoretical) * 100
	}

	effStr := fmt.Sprintf("%.1f%%", eff)
	var styledEff string
	switch {
	case eff >= 70:
		styledEff = StyleEffGood.Width(12).Render(effStr)
	case eff >= 40:
		styledEff = StyleEffOk.Width(12).Render(effStr)
	default:
		styledEff = StyleEffLow.Width(12).Render(effStr)
	}

	return lipgloss.JoinHorizontal(lipgloss.Left,
		StyleResultLabel.Render(label),
		StyleResultValue.Render(fmt.Sprintf("%.0f", measured)),
		StyleResultTheo.Render(fmt.Sprintf("%.0f", theoretical)),
		styledEff,
	)
}

func (m model) theoreticalPeakMBs() float64 {
	freq, okF := m.parseNumericField("DRAM Freq")
	ch, okC := m.parseNumericField("Channels")
	if !okF || !okC || freq <= 0 || ch <= 0 {
		return 0
	}
	return freq * 8.0 * ch
}

func runBenchmarkCmd() tea.Msg {
	cmd := exec.Command("stress-ng", "--memrate", "1", "--memrate-bytes", "256M", "-t", "30", "--metrics")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if err != nil {
		return benchResultMsg{err: fmt.Errorf("%v: %s", err, buf.String())}
	}
	return benchResultMsg{results: parseBenchOutput(buf.String())}
}

var (
	readRateRe  = regexp.MustCompile(`read rate\s+([\d.]+)\s+MB per sec`)
	writeRateRe = regexp.MustCompile(`write rate\s+([\d.]+)\s+MB per sec`)
	metricRe    = regexp.MustCompile(`memrate\s+([\d.]+)\s+(\S+)\s+MB per sec`)
	durationRe  = regexp.MustCompile(`completed in\s+([\d.]+\s+secs)`)
)

func parseBenchOutput(output string) *benchResults {
	r := &benchResults{}

	if m := readRateRe.FindStringSubmatch(output); len(m) > 1 {
		r.readRate, _ = strconv.ParseFloat(m[1], 64)
	}
	if m := writeRateRe.FindStringSubmatch(output); len(m) > 1 {
		r.writeRate, _ = strconv.ParseFloat(m[1], 64)
	}
	if m := durationRe.FindStringSubmatch(output); len(m) > 1 {
		r.duration = m[1]
	}

	interesting := map[string]bool{
		"read64": true, "read128": true, "read256": true,
		"write64": true, "write128": true, "write64stoq": true,
		"read64pf": true, "read128pf": true,
		"memset": true,
	}

	for _, match := range metricRe.FindAllStringSubmatch(output, -1) {
		if len(match) < 3 {
			continue
		}
		label := match[2]
		if !interesting[label] {
			continue
		}
		val, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			continue
		}
		r.metrics = append(r.metrics, benchMetric{label: label, value: val})
	}

	return r
}

func parseSPDInfo(spd string) *spdInfo {
	info := &spdInfo{}

	get := func(pattern string) string {
		re := regexp.MustCompile(pattern)
		if m := re.FindStringSubmatch(spd); len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
		return ""
	}

	info.moduleManufacturer = get(`Module Manufacturer\s+(.+)`)
	info.dramManufacturer = get(`DRAM Manufacturer\s+(.+)`)
	info.partNumber = get(`Part Number\s+(.+)`)
	info.moduleType = get(`Module Type\s+(.+)`)
	info.memoryType = get(`Fundamental Memory type\s+(.+)`)
	info.moduleSpeed = get(`Maximum module speed\s+(.+)`)
	info.sizeMB = get(`Size\s+(\d+\s*MB)`)
	info.ranks = get(`Ranks\s+(\d+)`)
	info.bankLayout = get(`Banks x Rows x Columns x Bits\s+(.+)`)
	info.busWidth = get(`Primary Bus Width\s+(.+)`)
	info.deviceWidth = get(`SDRAM Device Width\s+(.+)`)
	info.supportedCL = get(`Supported CAS Latencies.*?\s+([\dT,\s]+)`)
	info.voltage = get(`Operable voltages\s+(.+)`)
	info.timingString = get(`tCL-tRCD-tRP-tRAS\s+(.+)`)
	info.tWR = get(`Write Recovery.*?\(tWR\)\s+([\d.]+\s*ns)`)
	info.tRRD = get(`Row Active to Row Active.*?\(tRRD\)\s+([\d.]+\s*ns)`)
	info.tRC = get(`Active to Auto-Refresh.*?\(tRC\)\s+([\d.]+\s*ns)`)
	info.tWTR = get(`Write to Read.*?\(tWTR\)\s+([\d.]+\s*ns)`)
	info.tRTP = get(`Read to Pre-charge.*?\(tRTP\)\s+([\d.]+\s*ns)`)
	info.tFAW = get(`Four Activate Window.*?\(tFAW\)\s+([\d.]+\s*ns)`)
	info.tCKmin = get(`Minimum Cycle Time.*?\(tCK\)\s+([\d.]+\s*ns)`)
	info.dllOff = get(`DLL-Off Mode supported\?\s+(.+)`)
	info.tempRange = get(`Operating temperature range\s+(.+)`)
	info.autoSR = get(`Auto Self-Refresh\?\s+(.+)`)
	info.moduleHeight = get(`Module Height\s+(.+)`)
	info.refCard = get(`Module Reference Card\s+(.+)`)
	info.numDIMMs = get(`Number of SDRAM DIMMs detected.*?:\s+(\d+)`)

	// Timings at standard speeds
	speedRe := regexp.MustCompile(`tCL-tRCD-tRP-tRAS as (DDR\d+-\d+)\s+([\d-]+)`)
	for _, m := range speedRe.FindAllStringSubmatch(spd, -1) {
		if len(m) > 2 {
			info.timingsBySpeed = append(info.timingsBySpeed, fmt.Sprintf("%-12s %s", m[1], m[2]))
		}
	}

	return info
}

func (m model) renderSPDInfoPanel(width int) string {
	s := m.spd

	row := func(label, value string) string {
		if value == "" {
			value = "N/A"
		}
		return lipgloss.JoinHorizontal(lipgloss.Left,
			StyleInfoLabel.Render(label),
			StyleInfoValue.Render(value),
		)
	}

	colWidth := (width - 4) / 2
	if colWidth < 40 {
		colWidth = 40
	}

	// Left column: Module identity + physical
	leftRows := []string{
		StyleSectionTitle.Render("Module Identity"),
		row("Module Manufacturer", s.moduleManufacturer),
		row("DRAM Manufacturer", s.dramManufacturer),
		row("Part Number", s.partNumber),
		row("Module Type", s.moduleType),
		row("Memory Type", s.memoryType),
		row("Max Module Speed", s.moduleSpeed),
		"",
		StyleSectionTitle.Render("Physical"),
		row("Size", s.sizeMB),
		row("Ranks", s.ranks),
		row("Bank Layout", s.bankLayout),
		row("Bus Width", s.busWidth),
		row("Device Width", s.deviceWidth),
		row("Module Height", s.moduleHeight),
		row("Reference Card", s.refCard),
	}
	if s.numDIMMs != "" {
		leftRows = append(leftRows, row("DIMMs Detected", s.numDIMMs))
	}

	// Right column: OC-relevant data
	rightRows := []string{
		StyleSectionTitle.Render("Overclocking Reference"),
		row("Supported CAS", s.supportedCL),
		row("SPD Timings", s.timingString),
		row("Min Cycle Time (tCK)", s.tCKmin),
		row("Operable Voltage", s.voltage),
		row("DLL-Off Support", s.dllOff),
		"",
		StyleSectionTitle.Render("Secondary Timings (ns)"),
		row("tWR  (Write Recovery)", s.tWR),
		row("tRRD (Row-to-Row)", s.tRRD),
		row("tRC  (Active-to-Refresh)", s.tRC),
		row("tWTR (Write-to-Read)", s.tWTR),
		row("tRTP (Read-to-Precharge)", s.tRTP),
		row("tFAW (4-Activate Window)", s.tFAW),
		"",
		StyleSectionTitle.Render("Features"),
		row("Temp Range", s.tempRange),
		row("Auto Self-Refresh", s.autoSR),
	}

	if len(s.timingsBySpeed) > 0 {
		rightRows = append(rightRows, "", StyleSectionTitle.Render("Timings by Speed"))
		for _, t := range s.timingsBySpeed {
			rightRows = append(rightRows, StyleHint.Render(t))
		}
	}

	left := lipgloss.NewStyle().Width(colWidth).Render(strings.Join(leftRows, "\n"))
	right := lipgloss.NewStyle().Width(colWidth).Render(strings.Join(rightRows, "\n"))

	heading := StyleSectionTitle.Render("SPD / Module Information") + "  " +
		StyleHint.Render("(read-only, from decode-dimms)")

	content := lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
	return lipgloss.JoinVertical(lipgloss.Left, heading, "", content)
}

func (m model) renderIMCPanel() string {
	heading := StyleSectionTitle.Render("IMC Tertiary Timings (Haswell)") + "  " +
		StyleHint.Render("(read-only, from MCHBAR registers)")

	var chSections []string
	for _, t := range m.imcTimings {
		chSections = append(chSections, renderChannelIMC(t))
	}

	return lipgloss.JoinVertical(lipgloss.Left, heading, "", strings.Join(chSections, "\n\n"))
}

func renderChannelIMC(t tertiaryTimings) string {
	matHdr := StyleColHeader.Render(fmt.Sprintf("  %-10s %4s %4s %4s %4s", "", "sg", "dg", "dr", "dd"))
	matRow := func(name string, sg, dg, dr, dd int) string {
		return fmt.Sprintf("  %-10s %4d %4d %4d %4d", name, sg, dg, dr, dd)
	}
	imcRow := func(label string, val int) string {
		return fmt.Sprintf("  %-12s %d", label, val)
	}

	left := strings.Join([]string{
		StyleSectionTitle.Render(fmt.Sprintf("Channel %d — Turnaround", t.channel)),
		matHdr,
		matRow("tRDRD", t.tRDRDsg, t.tRDRDdg, t.tRDRDdr, t.tRDRDdd),
		matRow("tWRWR", t.tWRWRsg, t.tWRWRdg, t.tWRWRdr, t.tWRWRdd),
		matRow("tRDWR", t.tRDWRsg, t.tRDWRdg, t.tRDWRdr, t.tRDWRdd),
		matRow("tWRRD", t.tWRRDsg, t.tWRRDdg, t.tWRRDdr, t.tWRRDdd),
	}, "\n")

	right := strings.Join([]string{
		StyleSectionTitle.Render("IMC Register Values"),
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

// Focus target for input navigation and actions
//go:generate stringer -type=focusTarget

type focusTarget int

const (
	focusField focusTarget = iota
	focusApply
	focusCancel
)

func (m model) renderTabBar() string {
	tabs := []string{"System Info", "Benchmark"}
	var rendered []string
	for i, tab := range tabs {
		if m.activeTab == i {
			rendered = append(rendered, StyleActiveTab.Render(tab))
		} else {
			rendered = append(rendered, StyleInactiveTab.Render(tab))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--mchbar" {
		dumpMCHBAR()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--mchbar-raw" {
		dumpMCHBARRaw()
		return
	}
	// No-op. TUI moved. Exit.
}

func dumpMCHBAR() {
	base, err := getMCHBARBase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ getMCHBARBase: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ MCHBAR base: 0x%X\n\n", base)

	timings, err := readMCHBARTimings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ readMCHBARTimings: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ %d active channel(s)\n\n", len(timings))

	for _, t := range timings {
		fmt.Printf("── Channel %d ─────────────────────────────\n\n", t.channel)
		fmt.Println("  Raw Registers:")
		fmt.Printf("    TC_DBP   = 0x%08X\n", t.rawDBP)
		fmt.Printf("    TC_RAP   = 0x%08X\n", t.rawRAP)
		fmt.Printf("    TC_RWP   = 0x%08X\n", t.rawRWP)
		fmt.Printf("    TC_OTH   = 0x%08X\n", t.rawOTH)
		fmt.Printf("    TC_OTH2  = 0x%08X\n", t.rawOTH2)
		fmt.Printf("    TC_RFTP  = 0x%08X\n\n", t.rawRFTP)
		fmt.Println("  Primary (TC_DBP):")
		fmt.Printf("    tRAS=%d  tRCD=%d  tRP=%d  tRDPRE=%d  tWRPRE=%d\n\n",
			t.tRAS, t.tRCD, t.tRP, t.tRDPRE, t.tWRPRE)
		fmt.Println("  Secondary (TC_RAP):")
		fmt.Printf("    tRRDsg=%d  tRRDdg=%d  tRTP=%d  tCKE=%d  tWTR=%d\n\n",
			t.tRRDsg, t.tRRDdg, t.tRTP, t.tCKE, t.tWTR)
		fmt.Println("  Turnaround:")
		fmt.Printf("    %-10s %4s %4s %4s %4s\n", "", "sg", "dg", "dr", "dd")
		fmt.Printf("    %-10s %4d %4d %4d %4d\n", "tRDRD", t.tRDRDsg, t.tRDRDdg, t.tRDRDdr, t.tRDRDdd)
		fmt.Printf("    %-10s %4d %4d %4d %4d\n", "tWRWR", t.tWRWRsg, t.tWRWRdg, t.tWRWRdr, t.tWRWRdd)
		fmt.Printf("    %-10s %4d %4d %4d %4d\n", "tRDWR", t.tRDWRsg, t.tRDWRdg, t.tRDWRdr, t.tRDWRdd)
		fmt.Printf("    %-10s %4d %4d %4d %4d\n\n", "tWRRD", t.tWRRDsg, t.tWRRDdg, t.tWRRDdr, t.tWRRDdd)
		fmt.Println("  Refresh (TC_RFTP):")
		fmt.Printf("    tREFI=%d  tRFC=%d\n\n", t.tREFI, t.tRFC)
	}
}
