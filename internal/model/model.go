// Package model is the temporary pre-shell Bubble Tea model for MemStation.
// Phase 0 keeps the current layout working while removing the acute bugs and
// wiring the runner/store scaffolding; the full app split happens in Phase 1.
package model

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mem-station/internal/bench"
	"mem-station/internal/burnin"
	"mem-station/internal/memory"
	"mem-station/internal/runner"
	"mem-station/internal/store"
	"mem-station/internal/style"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Options configures the initial model.
type Options struct {
	JobPath string // --job flag value (empty means use env/defaults)
}

// --- Messages ---

// detectResultMsg carries the result of async hardware detection.
type detectResultMsg struct {
	vals map[string]string
	msg  string
	spd  *memory.SPDInfo
	imc  []memory.IMCTimings
}

// detectCmd runs hardware detection off the UI thread.
var detectCmd tea.Cmd = func() tea.Msg {
	vals, msg, spd := memory.DetectDefaults()
	imc, _ := memory.ReadMCHBARTimings() // tolerate failure (needs root)
	return detectResultMsg{vals: vals, msg: msg, spd: spd, imc: imc}
}

// jobEventMsg wraps a runner.Event for the Bubble Tea loop.
type jobEventMsg struct {
	ev runner.Event
}

// waitEventCmd blocks until the next runner event (or channel close) arrives.
func waitEventCmd(ch <-chan runner.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return jobEventMsg{ev: ev}
	}
}

// --- Types and Structs ---

type timingField struct {
	label string
	hint  string
	desc  string
	input textinput.Model
}

type fieldBounds struct {
	min, max float64
	note     string
	hasMin   bool
	hasMax   bool
}

// focusTarget identifies what is selected for activation.
type focusTarget int

const (
	focusField focusTarget = iota
	focusApply
	focusCancel
)

// Model is the root Bubble Tea model.
type Model struct {
	fields     []timingField
	focusIndex int
	focusType  focusTarget
	editing    bool // a field is being edited (input focused) vs selected
	editValue  string

	lockRatios bool
	width      int
	height     int
	status     string
	activeTab  int

	detecting bool

	benchRunning bool
	benchJob     *runner.Job
	benchEvents  <-chan runner.Event
	benchOutput  string
	benchResults *bench.BenchResults

	burnInRunning  bool
	burnInJob      *runner.Job
	burnInEvents   <-chan runner.Event
	burnInOutput   string
	burnInResults  *burnin.Results
	burnInViewport viewport.Model

	spd        *memory.SPDInfo
	imcTimings []memory.IMCTimings

	jobPath string
}

func newField(base textinput.Model, label, value, desc string) timingField {
	in := base
	in.SetValue(value)
	in.Placeholder = value
	return timingField{label: label, hint: value, desc: desc, input: in}
}

// InitialModel builds the model without blocking on hardware detection.
func InitialModel(opts Options) Model {
	baseInput := textinput.New()
	baseInput.CharLimit = 8
	baseInput.Width = 8

	fields := []timingField{
		newField(baseInput, "Channels", "2", "Detected memory channels."),
		newField(baseInput, "DRAM Ratio", "?", "Memory multiplier relative to base clock."),
		newField(baseInput, "DRAM Freq", "1600", "Effective DRAM frequency in MT/s."),
		newField(baseInput, "CAS (tCL)", "?", "CAS latency cycles."),
		newField(baseInput, "tRCD", "?", "RAS to CAS delay cycles."),
		newField(baseInput, "tRP", "?", "Row precharge delay cycles."),
		newField(baseInput, "tRAS", "?", "Active to precharge delay cycles."),
		newField(baseInput, "Command Rate", "2T", "Command rate; usually 1T or 2T."),
		newField(baseInput, "tRFC", "?", "Refresh cycle timing."),
		newField(baseInput, "DRAM Voltage", "?", "Requested memory rail voltage."),
	}

	return Model{
		fields:         fields,
		focusIndex:     0,
		focusType:      focusField,
		lockRatios:     true,
		width:          120,
		height:         40,
		status:         "Detecting hardware...",
		detecting:      true,
		burnInViewport: viewport.Model{},
		jobPath:        opts.JobPath,
	}
}

// Init kicks off async hardware detection.
func (m Model) Init() tea.Cmd {
	return detectCmd
}

/* --------------- Update --------------- */

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case detectResultMsg:
		m.detecting = false
		m.spd = msg.spd
		m.imcTimings = msg.imc
		m.applyDetected(msg.vals)
		if len(msg.vals) == 0 {
			m.status = msg.msg + " Press R to retry."
		} else {
			m.status = msg.msg + " | Tab/Shift+Tab to move, L toggles ratio lock."
		}
		return m, nil

	case jobEventMsg:
		return m.handleJobEvent(msg.ev)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeBurnInViewport()
		return m, nil

	case tea.KeyMsg:
		if m.editing {
			return m.handleEditingKey(msg)
		}
		return m.handleKey(msg)
	}

	// Non-key messages (e.g. textinput.Blink) reach the active input only
	// while a field is being edited.
	if m.editing && m.activeTab == 0 && m.focusType == focusField {
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

func (m Model) handleEditingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.fields[m.focusIndex].input.SetValue(m.editValue)
		m.fields[m.focusIndex].input.Blur()
		m.editing = false
		m.status = "Edit cancelled."
		return m, nil

	case "enter":
		m.commitFieldEdit()
		return m, nil

	case "tab":
		m.commitFieldEdit()
		m.moveFocus(1)
		return m, nil

	case "shift+tab":
		m.commitFieldEdit()
		m.moveFocus(-1)
		return m, nil

	case "ctrl+c":
		return m, tea.Quit
	}

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

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.cancelJobs()
		return m, tea.Quit

	case "r", "R":
		m.detecting = true
		m.status = "Detecting hardware..."
		return m, detectCmd

	case "f1":
		return m.switchTab(0)
	case "f2":
		return m.switchTab(1)
	case "f3":
		return m.switchTab(2)

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
		switch m.activeTab {
		case 1:
			return m.startBenchmark()
		case 2:
			return m.startBurnIn()
		}

		// System info tab: selected field / apply / cancel.
		switch m.focusType {
		case focusField:
			m.editing = true
			m.editValue = m.fields[m.focusIndex].input.Value()
			cmd := m.fields[m.focusIndex].input.Focus()
			return m, cmd
		case focusApply:
			m.status = "Profile only - not written to hardware. (Apply flow arrives in Phase 2.)"
			return m, nil
		case focusCancel:
			m.resetValues()
			m.status = "Changes canceled."
			return m, nil
		}
	}

	return m, nil
}

func (m Model) handleJobEvent(ev runner.Event) (tea.Model, tea.Cmd) {
	switch ev.Kind {
	case runner.KindBenchmark:
		return m.handleBenchEvent(ev)
	case runner.KindBurnIn:
		return m.handleBurnInEvent(ev)
	}
	return m, nil
}

func (m Model) handleBenchEvent(ev runner.Event) (tea.Model, tea.Cmd) {
	switch ev.Type {
	case runner.EventStarted:
		m.status = fmt.Sprintf("Benchmark running. Log: %s", ev.LogPath)
		return m, waitEventCmd(m.benchEvents)
	case runner.EventOutput:
		m.benchOutput += ev.Line + "\n"
		return m, waitEventCmd(m.benchEvents)
	case runner.EventFinished:
		m.benchRunning = false
		if ev.Err != nil {
			m.status = fmt.Sprintf("Benchmark failed: %v", ev.Err)
		} else {
			m.benchResults = bench.ParseBenchOutput(m.benchOutput)
			m.status = "Benchmark complete. Results shown below."
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleBurnInEvent(ev runner.Event) (tea.Model, tea.Cmd) {
	switch ev.Type {
	case runner.EventStarted:
		m.status = fmt.Sprintf("Burn-in running. Log: %s", ev.LogPath)
		return m, waitEventCmd(m.burnInEvents)
	case runner.EventOutput:
		m.burnInOutput += ev.Line + "\n"
		m.burnInViewport.SetContent(m.burnInOutput)
		m.burnInViewport.GotoBottom()
		return m, waitEventCmd(m.burnInEvents)
	case runner.EventFinished:
		m.burnInRunning = false
		res := burnin.ParseBurnInOutput(m.burnInOutput, ev.ExitCode, ev.Err)
		res.LogPath = ev.LogPath
		m.burnInResults = res
		if res.Success {
			m.status = "Burn-in passed."
		} else {
			m.status = fmt.Sprintf("Burn-in failed: %d error(s), exit code %d.", res.Errors, res.ExitCode)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) startBenchmark() (tea.Model, tea.Cmd) {
	if m.benchRunning {
		return m, nil
	}
	logPath := store.LogPath(runner.KindBenchmark, time.Now())
	spec := runner.Spec{
		Kind:    runner.KindBenchmark,
		Name:    "stress-ng",
		Args:    []string{"stress-ng", "--memrate", "1", "--memrate-bytes", "256M", "-t", "30", "--metrics"},
		LogPath: logPath,
	}
	job := runner.Start(context.Background(), spec)
	m.benchJob = job
	m.benchEvents = job.Events()
	m.benchRunning = true
	m.benchOutput = ""
	m.benchResults = nil
	m.status = "Running memrate benchmark (30s)..."
	return m, waitEventCmd(m.benchEvents)
}

func (m Model) startBurnIn() (tea.Model, tea.Cmd) {
	if m.burnInRunning {
		return m, nil
	}
	srcPath, err := resolveJobPath(m.jobPath)
	if err != nil {
		m.status = fmt.Sprintf("Cannot start burn-in: %v", err)
		return m, nil
	}
	preset, err := os.ReadFile(srcPath)
	if err != nil {
		m.status = fmt.Sprintf("Cannot read job file: %v", err)
		return m, nil
	}
	duration := 30 * time.Minute
	jobPath, err := store.MaterializeJob(preset, duration)
	if err != nil {
		m.status = fmt.Sprintf("Cannot materialize job: %v", err)
		return m, nil
	}
	logPath := store.LogPath(runner.KindBurnIn, time.Now())
	spec := runner.Spec{
		Kind:     runner.KindBurnIn,
		Name:     "stress-ng",
		Args:     []string{"stress-ng", "--job", jobPath},
		LogPath:  logPath,
		Duration: duration,
	}
	job := runner.Start(context.Background(), spec)
	m.burnInJob = job
	m.burnInEvents = job.Events()
	m.burnInRunning = true
	m.burnInOutput = ""
	m.burnInResults = nil
	m.resizeBurnInViewport()
	m.status = "Running burn-in (30m)..."
	return m, waitEventCmd(m.burnInEvents)
}

func (m Model) cancelJobs() {
	if m.burnInJob != nil {
		m.burnInJob.Cancel()
	}
	if m.benchJob != nil {
		m.benchJob.Cancel()
	}
}

func (m Model) switchTab(tab int) (tea.Model, tea.Cmd) {
	m.activeTab = tab
	switch tab {
	case 0:
		m.status = "Tab/Shift+Tab to move, L toggles ratio lock, Enter to edit, Esc to cancel."
	case 1:
		if m.benchResults != nil {
			m.status = "Benchmark results shown. Press Enter to re-run."
		} else {
			m.status = "Press Enter to run memrate benchmark (30s)."
		}
	case 2:
		m.resizeBurnInViewport()
		if m.burnInResults != nil {
			m.status = "Burn-in results shown. Press Enter to re-run."
		} else {
			m.status = "Press Enter to run burn-in test (30m)."
		}
	}
	return m, nil
}

func (m *Model) resizeBurnInViewport() {
	w, h := m.burnInViewportSize()
	m.burnInViewport.Width = w
	m.burnInViewport.Height = h
	m.burnInViewport.SetContent(m.burnInOutput)
}

// burnInViewportSize derives the burn-in log viewport from the window, keeping
// the footer visible at 80x24 and 120x40.
func (m Model) burnInViewportSize() (int, int) {
	w := m.width - 12
	if w < 20 {
		w = 20
	}
	h := m.height - 24
	if h < 3 {
		h = 3
	}
	return w, h
}

func (m *Model) applyDetected(vals map[string]string) {
	for i := range m.fields {
		if v, ok := vals[m.fields[i].label]; ok && v != "" {
			m.fields[i].hint = v
			m.fields[i].input.SetValue(v)
		}
	}
}

// resolveJobPath resolves the burn-in job source: --job flag, MEM_STATION_JOB,
// then ./burnin-memory.job, then next to the executable.
func resolveJobPath(flagValue string) (string, error) {
	var candidates []string
	switch {
	case flagValue != "":
		candidates = append(candidates, flagValue)
	case os.Getenv("MEM_STATION_JOB") != "":
		candidates = append(candidates, os.Getenv("MEM_STATION_JOB"))
	default:
		candidates = append(candidates, "burnin-memory.job")
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "burnin-memory.job"))
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			if abs, err := filepath.Abs(c); err == nil {
				return abs, nil
			}
			return c, nil
		}
	}
	return "", fmt.Errorf("burn-in job file not found (set MEM_STATION_JOB or pass --job)")
}

/* --------------- Field helpers --------------- */

func (m *Model) commitFieldEdit() {
	if !m.editing {
		return
	}
	m.editing = false
	m.fields[m.focusIndex].input.Blur()
}

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
	m.editing = false

	switch {
	case pos < len(m.fields):
		m.focusType = focusField
		m.focusIndex = pos
	case pos == len(m.fields):
		m.focusType = focusApply
	default:
		m.focusType = focusCancel
	}
}

func (m *Model) resetValues() {
	for i := range m.fields {
		m.fields[i].input.SetValue(m.fields[i].hint)
	}
}

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
	m.fields[idx].input.SetValue(strconv.Itoa(scaled))
}

func (m Model) focusPosition() int {
	switch m.focusType {
	case focusField:
		return m.focusIndex
	case focusApply:
		return len(m.fields)
	default:
		return len(m.fields) + 1
	}
}

func (m Model) fieldIndex(label string) int {
	for i, f := range m.fields {
		if f.label == label {
			return i
		}
	}
	return -1
}

// parseNumericField parses a field value (input first, then hint) as float64.
func (m Model) parseNumericField(label string) (float64, bool) {
	for _, f := range m.fields {
		if f.label != label {
			continue
		}
		if v, ok := parseNumber(f.input.Value()); ok {
			return v, true
		}
		return parseNumber(f.hint)
	}
	return 0, false
}

func (m Model) originalNumericField(label string) float64 {
	for _, f := range m.fields {
		if f.label != label {
			continue
		}
		v, _ := parseNumber(f.hint)
		return v
	}
	return 0
}

// parseNumber extracts the leading numeric prefix of a value ("2T" -> 2,
// "1.35V" -> 1.35, "1600" -> 1600, "?" -> not ok).
func parseNumber(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	i := 0
	for i < len(s) {
		c := s[i]
		if (c >= '0' && c <= '9') || c == '.' || c == '+' || c == '-' {
			i++
			continue
		}
		break
	}
	if i == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func parseUnsignedInt(s string) (int, bool) {
	v, ok := parseNumber(s)
	if !ok || v <= 0 {
		return 0, false
	}
	return int(v), true
}

func ceilDiv(a, b int) int {
	return (a + b - 1) / b
}

func formatBounds(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

/* --------------- View --------------- */

func (m Model) View() string {
	if m.width == 0 {
		m.width = 120
	}
	if m.height == 0 {
		m.height = 40
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
	switch {
	case m.burnInRunning:
		sections = append(sections, style.RunningBtn.Render(" Running... (30m) "))
	case m.burnInResults != nil:
		r := m.burnInResults
		if r.Success {
			sections = append(sections, style.Safe.Render("✓ Burn-in passed")+style.Hint.Render(fmt.Sprintf("  errors=%d exit=%d", r.Errors, r.ExitCode)))
		} else {
			sections = append(sections, style.Unsafe.Render("✗ Burn-in failed")+style.Hint.Render(fmt.Sprintf("  errors=%d exit=%d", r.Errors, r.ExitCode)))
		}
	default:
		sections = append(sections, style.RunBtn.Render(" Enter — Run Burn-in "))
	}
	sections = append(sections, "")

	bottomPanel := style.Panel.Width(fullWidth).Render(m.burnInViewport.View())

	mainContent := style.Panel.Width(fullWidth).Render(strings.Join(sections, "\n"))

	return lipgloss.JoinVertical(lipgloss.Top, mainContent, bottomPanel)
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

func (m Model) safeBounds(label string) fieldBounds {
	freq, okF := m.parseNumericField("DRAM Freq")
	if !okF || freq <= 0 {
		freq = 1600
	}
	tclVal, _ := m.parseNumericField("CAS (tCL)")
	trcdVal, _ := m.parseNumericField("tRCD")
	switch label {
	case "CAS (tCL)":
		minCL := float64(ceilDiv(int(freq)*10, 2000))
		if minCL < 5 {
			minCL = 5
		}
		return fieldBounds{min: minCL, max: 18, hasMin: true, hasMax: true}
	case "tRCD":
		minRCD := float64(ceilDiv(int(freq)*10, 2000))
		if minRCD < 5 {
			minRCD = 5
		}
		return fieldBounds{min: minRCD, max: 18, hasMin: true, hasMax: true}
	case "tRP":
		minRP := float64(ceilDiv(int(freq)*10, 2000))
		if minRP < 5 {
			minRP = 5
		}
		return fieldBounds{min: minRP, max: 18, hasMin: true, hasMax: true}
	case "tRAS":
		minFormula := tclVal + trcdVal
		minNs := float64(ceilDiv(int(freq)*35, 2000))
		minRAS := minFormula
		if minNs > minRAS {
			minRAS = minNs
		}
		if minRAS < 15 {
			minRAS = 15
		}
		return fieldBounds{min: minRAS, max: 63, hasMin: true, hasMax: true, note: "≥CL+tRCD"}
	case "tRFC":
		minRFC := float64(ceilDiv(int(freq)*160, 2000))
		return fieldBounds{min: minRFC, max: 511, hasMin: true, hasMax: true, note: "≥160ns"}
	case "Command Rate":
		return fieldBounds{min: 1, max: 2, hasMin: true, hasMax: true, note: "1T/2T"}
	case "DRAM Voltage":
		return fieldBounds{min: 1.35, max: 1.65, hasMin: true, hasMax: true, note: "1.35-1.65V"}
	default:
		return fieldBounds{}
	}
}

func (m Model) renderBounds(f timingField, b fieldBounds) string {
	if !b.hasMin && !b.hasMax {
		return style.Bounds.Render("")
	}
	var rangeStr string
	switch f.label {
	case "DRAM Voltage", "Command Rate":
		rangeStr = b.note
	default:
		if b.hasMin && b.hasMax {
			rangeStr = formatBounds(b.min) + "-" + formatBounds(b.max)
			if b.note != "" {
				rangeStr += " " + b.note
			}
		} else if b.hasMin {
			rangeStr = "≥" + formatBounds(b.min)
		} else {
			rangeStr = "≤" + formatBounds(b.max)
		}
	}

	v, ok := parseNumber(f.input.Value())
	if !ok {
		// Unknown ("?") values get the range without a severity color.
		return style.Bounds.Render(rangeStr)
	}

	if f.label == "DRAM Voltage" {
		if (b.hasMin && v < b.min) || (b.hasMax && v > b.max) {
			return style.Unsafe.Width(14).Render(rangeStr)
		}
		return style.Safe.Width(14).Render(rangeStr)
	}

	switch {
	case b.hasMin && v < b.min:
		return style.Unsafe.Width(14).Render(rangeStr)
	case b.hasMax && v > b.max:
		return style.Warn.Width(14).Render(rangeStr)
	default:
		return style.Safe.Width(14).Render(rangeStr)
	}
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
		style.Hint.Render("Enter              Edit / run / apply"),
		style.Hint.Render("L                  Toggle ratio lock"),
		style.Hint.Render("Esc                Cancel edit (2x resets)"),
		style.Hint.Render("R                  Re-detect hardware"),
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
		m.renderResultRow("Read Rate (avg)", r.ReadRate, peakMBs),
		m.renderResultRow("Write Rate (avg)", r.WriteRate, peakMBs),
		"",
	}
	if len(r.Metrics) > 0 {
		rows = append(rows, style.SectionTitle.Render("Detailed Rates (MB/s)"))
		for _, metric := range r.Metrics {
			rows = append(rows, m.renderResultRow(metric.Label, metric.Value, peakMBs))
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
		style.InfoLabel.Render("Module:") + " " + style.InfoValue.Render(m.spd.ModuleManufacturer+" "+m.spd.PartNumber),
		style.InfoLabel.Render("DRAM:") + " " + style.InfoValue.Render(m.spd.DRAMManufacturer+" "+m.spd.MemoryType),
		style.InfoLabel.Render("Size:") + " " + style.InfoValue.Render(m.spd.SizeMB+" MB ("+m.spd.Ranks+" ranks)"),
		style.InfoLabel.Render("Speed:") + " " + style.InfoValue.Render(m.spd.ModuleSpeed),
		style.InfoLabel.Render("Timings:") + " " + style.InfoValue.Render(m.spd.TimingString),
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func renderChannelIMC(t memory.IMCTimings) string {
	matHdr := style.ColHeader.Render(fmt.Sprintf("  %-10s %4s %4s %4s %4s", "", "sg", "dg", "dr", "dd"))
	matRow := func(name string, sg, dg, dr, dd int) string {
		return fmt.Sprintf("  %-10s %4d %4d %4d %4d", name, sg, dg, dr, dd)
	}
	imcRow := func(label string, val int) string {
		return fmt.Sprintf("  %-12s %d", label, val)
	}
	left := strings.Join([]string{
		style.SectionTitle.Render(fmt.Sprintf("Channel %d — Turnaround", t.Channel)),
		matHdr,
		matRow("tRDRD", t.TRDRDsg, t.TRDRDdg, t.TRDRDdr, t.TRDRDdd),
		matRow("tWRWR", t.TWRWRsg, t.TWRWRdg, t.TWRWRdr, t.TWRWRdd),
		matRow("tRDWR", t.TRDWRsg, t.TRDWRdg, t.TRDWRdr, t.TRDWRdd),
		matRow("tWRRD", t.TWRRDsg, t.TWRRDdg, t.TWRRDdr, t.TWRRDdd),
	}, "\n")
	right := strings.Join([]string{
		style.SectionTitle.Render("IMC Register Values"),
		imcRow("tRAS", t.TRAS),
		imcRow("tRCD", t.TRCD),
		imcRow("tRP", t.TRP),
		imcRow("tRTP", t.TRTP),
		imcRow("tWTR", t.TWTR),
		imcRow("tRRD sg/dg", t.TRRDsg) + " / " + strconv.Itoa(t.TRRDdg),
		imcRow("tCKE", t.TCKE),
		imcRow("tREFI", t.TREFI),
		imcRow("tRFC", t.TRFC),
	}, "\n")
	leftStyled := lipgloss.NewStyle().Width(38).Render(left)
	rightStyled := lipgloss.NewStyle().Width(30).Render(right)
	return lipgloss.JoinHorizontal(lipgloss.Top, leftStyled, "  ", rightStyled)
}
