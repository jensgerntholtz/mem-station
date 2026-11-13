package benchmark

import (
	"context"
	"errors"
	"fmt"
	"time"

	"mem-station/internal/bench"
	"mem-station/internal/msgs"
	"mem-station/internal/runner"
	"mem-station/internal/store"
	"mem-station/internal/ui"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// maxOutputLines bounds the retained live output.
const maxOutputLines = 2000

// Update handles messages for the Benchmark tab.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case msgs.JobEvent:
		if !m.Wants(msg.Ev.Kind) {
			return nil
		}
		return m.handleEvent(msg.Ev)
	}
	return nil
}

func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.editing {
		return m.handleEditKey(msg)
	}
	switch {
	case key.Matches(msg, m.keys.Cancel):
		if m.Running() {
			return m.cancel("Benchmark cancelled.")
		}
	case key.Matches(msg, m.keys.Run):
		if !m.Running() {
			return m.run()
		}
	case key.Matches(msg, m.keys.Up):
		if !m.Running() && m.row > 0 {
			m.row--
		}
	case key.Matches(msg, m.keys.Down):
		if !m.Running() && m.row < rowCount-1 {
			m.row++
		}
	case key.Matches(msg, m.keys.Prev):
		return m.cycle(-1)
	case key.Matches(msg, m.keys.Next):
		return m.cycle(1)
	case key.Matches(msg, m.keys.Follow):
		m.follow = !m.follow
		if m.follow {
			m.vp.GotoBottom()
		}
	case key.Matches(msg, m.keys.Export):
		return m.export()
	case key.Matches(msg, m.keys.ScrollUp):
		m.follow = false
		m.vp.PageUp()
	case key.Matches(msg, m.keys.ScrollDn):
		m.vp.PageDown()
		if m.vp.AtBottom() {
			m.follow = true
		}
	}
	return nil
}

// handleEditKey drives the custom-duration input.
func (m *Model) handleEditKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "enter", "tab", "shift+tab":
		if err := m.commitEdit(); err != nil {
			return status(err.Error(), ui.LevelError)
		}
		if msg.String() != "enter" {
			m.row = (m.row + 1) % rowCount
		}
		return nil
	case "esc":
		m.cancelEdit()
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

// cycle moves the selected configuration value. The custom duration option
// opens the input for a typed value.
func (m *Model) cycle(dir int) tea.Cmd {
	if m.Running() {
		return nil
	}
	switch m.row {
	case rowDuration:
		prev := m.durationIndex
		if !cycleDuration(&m.durationIndex, dir) {
			return nil
		}
		if m.durationIndex == customDurationIndex {
			m.startEdit(prev)
			return nil
		}
	case rowBuffer:
		cycleBuffer(&m.bufferIndex, dir)
	case rowWorkers:
		cycleWorkers(&m.workers, dir)
	}
	return nil
}

// startEdit opens the custom duration input with the current value.
func (m *Model) startEdit(prevIndex int) {
	m.editing = true
	m.editPrev = prevIndex
	m.input.SetValue(ui.DurationLabel(m.custom))
	m.input.CursorEnd()
	m.input.Focus()
}

// commitEdit parses and applies the typed duration.
func (m *Model) commitEdit() error {
	d, err := parseDurationInput(m.input.Value())
	if err != nil {
		return err
	}
	if d < minDuration || d > maxDuration {
		return fmt.Errorf("duration must be between %s and %s", ui.DurationLabel(minDuration), ui.DurationLabel(maxDuration))
	}
	m.custom = d
	m.editing = false
	m.input.Blur()
	return nil
}

// cancelEdit restores the duration option that was selected before the input
// opened.
func (m *Model) cancelEdit() {
	m.editing = false
	m.input.Blur()
	m.durationIndex = m.editPrev
}

func (m *Model) handleEvent(ev runner.Event) tea.Cmd {
	switch ev.Type {
	case runner.EventStarted:
		m.state = runner.StateRunning
		m.startedAt = ev.At
		m.logPath = ev.LogPath
		return msgs.WaitEvent(m.events)

	case runner.EventOutput:
		m.output = ui.AppendLine(m.output, ev.Line, maxOutputLines)
		m.vp.SetContent(m.output)
		if m.follow {
			m.vp.GotoBottom()
		}
		return msgs.WaitEvent(m.events)

	case runner.EventFinished:
		return m.finish(ev)
	}
	return nil
}

// finish derives the terminal state, parses the output, and persists the run.
func (m *Model) finish(ev runner.Event) tea.Cmd {
	m.finishedAt = ev.At
	m.exitCode = ev.ExitCode
	m.logPath = ev.LogPath
	m.results = bench.ParseBenchOutput(m.output)
	m.parseMsg = ""

	switch {
	case errors.Is(ev.Err, runner.ErrCancelled):
		m.state = runner.StateCancelled
		m.errText = "cancelled"
	case ev.Err != nil:
		m.state = runner.StateFailed
		m.errText = ev.Err.Error()
	case ev.ExitCode != 0:
		m.state = runner.StateFailed
		m.errText = fmt.Sprintf("exit code %d", ev.ExitCode)
	case m.results.ReadRate == 0 && m.results.WriteRate == 0 && len(m.results.Metrics) == 0:
		m.state = runner.StateFailed
		m.parseMsg = "could not parse memrate output"
		m.errText = m.parseMsg + " (log: " + m.logPath + ")"
	default:
		m.state = runner.StateDone
	}

	payload := NewPayload(m.runOptions, m.results)
	m.appendHistory(payload)

	switch m.state {
	case runner.StateCancelled:
		return status("Benchmark cancelled.", ui.LevelWarn)
	case runner.StateFailed:
		return status("Benchmark failed: "+m.errText, ui.LevelError)
	default:
		return status(fmt.Sprintf("Benchmark complete: read %.0f MB/s, write %.0f MB/s (log: %s)",
			m.results.ReadRate, m.results.WriteRate, m.logPath), ui.LevelInfo)
	}
}

func (m *Model) run() tea.Cmd {
	opts := m.options()
	if err := opts.Validate(); err != nil {
		return status("Cannot run: "+err.Error(), ui.LevelError)
	}
	m.runOptions = opts
	spec := runner.Spec{
		Kind:     runner.KindBenchmark,
		Name:     "stress-ng",
		Args:     opts.Args(),
		LogPath:  store.LogPath(runner.KindBenchmark, time.Now()),
		Duration: opts.Duration,
	}

	m.job = m.start(context.Background(), spec)
	m.events = m.job.Events()
	m.state = runner.StateRunning
	m.startedAt = m.now()
	m.finishedAt = time.Time{}
	m.output = ""
	m.results = nil
	m.parseMsg = ""
	m.errText = ""
	m.exitCode = 0
	m.logPath = spec.LogPath
	m.vp.SetContent("")

	return tea.Batch(
		msgs.WaitEvent(m.events),
		status("Running memrate benchmark ("+opts.Label()+")...", ui.LevelInfo),
	)
}

// cancel asks the runner to kill the process group; events keep flowing until
// EventFinished arrives.
func (m *Model) cancel(message string) tea.Cmd {
	if m.job == nil {
		return nil
	}
	m.job.Cancel()
	return status(message, ui.LevelWarn)
}

// elapsed returns the run time so far (or the final duration).
func (m *Model) elapsed() time.Duration {
	if m.startedAt.IsZero() {
		return 0
	}
	if m.Running() {
		return m.now().Sub(m.startedAt)
	}
	if !m.finishedAt.IsZero() {
		return m.finishedAt.Sub(m.startedAt)
	}
	return 0
}

func (m *Model) progress() float64 {
	total := m.runOptions.Duration
	if total <= 0 {
		total = m.options().Duration
	}
	if total <= 0 {
		return 0
	}
	pct := float64(m.elapsed()) / float64(total) * 100
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

// resizeViewport sizes the live-output viewport to the panel interior.
func (m *Model) resizeViewport(width, height int) {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	if m.vp.Width() != width || m.vp.Height() != height {
		m.vp.SetWidth(width)
		m.vp.SetHeight(height)
		m.vp.SetContent(m.output)
		if m.follow {
			m.vp.GotoBottom()
		}
	}
}

func status(text string, level ui.Level) tea.Cmd { return ui.Status(text, level) }
