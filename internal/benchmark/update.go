package benchmark

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"mem-station/internal/bench"
	"mem-station/internal/msgs"
	"mem-station/internal/runner"
	"mem-station/internal/store"
	"mem-station/internal/ui"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// maxOutputLines bounds the retained live output.
const maxOutputLines = 2000

// Update handles messages for the Benchmark tab.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case msgs.JobEvent:
		if !m.Wants(msg.Ev.Kind) {
			return nil
		}
		return m.handleEvent(msg.Ev)
	}
	return nil
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		if m.Running() {
			return m.cancel()
		}
	case key.Matches(msg, m.keys.Run):
		if !m.Running() {
			return m.run()
		}
	case key.Matches(msg, m.keys.ScrollUp):
		m.vp.LineUp(1)
	case key.Matches(msg, m.keys.ScrollDn):
		m.vp.LineDown(1)
	}
	return nil
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
		return msgs.WaitEvent(m.events)

	case runner.EventFinished:
		m.finishedAt = ev.At
		m.exitCode = ev.ExitCode
		m.logPath = ev.LogPath
		switch {
		case errors.Is(ev.Err, runner.ErrCancelled):
			m.state = runner.StateCancelled
			m.errText = ""
			return status("Benchmark cancelled.", ui.LevelWarn)
		case ev.Err != nil || ev.ExitCode != 0:
			m.state = runner.StateFailed
			if ev.Err != nil {
				m.errText = ev.Err.Error()
			} else {
				m.errText = fmt.Sprintf("exit code %d", ev.ExitCode)
			}
			return status("Benchmark failed: "+m.errText+" (log: "+m.logPath+")", ui.LevelError)
		default:
			m.state = runner.StateDone
			m.errText = ""
			m.results = bench.ParseBenchOutput(m.output)
			return status("Benchmark complete. Results shown above the log.", ui.LevelInfo)
		}
	}
	return nil
}

func (m *Model) run() tea.Cmd {
	spec := runner.Spec{
		Kind: runner.KindBenchmark,
		Name: "stress-ng",
		Args: []string{
			"--memrate", "1",
			"--memrate-bytes", "256M",
			"-t", strconv.Itoa(int(m.duration.Seconds())),
			"--metrics",
		},
		LogPath:  store.LogPath(runner.KindBenchmark, time.Now()),
		Duration: m.duration,
	}

	m.job = runner.Start(context.Background(), spec)
	m.events = m.job.Events()
	m.state = runner.StateRunning
	m.startedAt = time.Now()
	m.finishedAt = time.Time{}
	m.output = ""
	m.results = nil
	m.errText = ""
	m.exitCode = 0
	m.vp.SetContent("")

	return tea.Batch(
		msgs.WaitEvent(m.events),
		status("Running memrate benchmark ("+ui.DurationLabel(m.duration)+")...", ui.LevelInfo),
	)
}

func (m *Model) cancel() tea.Cmd {
	if m.job == nil {
		return nil
	}
	m.job.Cancel()
	return status("Cancelling benchmark...", ui.LevelWarn)
}

// elapsed returns the run time so far (or the final duration).
func (m *Model) elapsed() time.Duration {
	if m.startedAt.IsZero() {
		return 0
	}
	if m.Running() {
		return time.Since(m.startedAt)
	}
	if !m.finishedAt.IsZero() {
		return m.finishedAt.Sub(m.startedAt)
	}
	return 0
}

func (m *Model) progress() float64 {
	if m.duration <= 0 {
		return 0
	}
	pct := float64(m.elapsed()) / float64(m.duration) * 100
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
	if m.vp.Width != width || m.vp.Height != height {
		m.vp.Width = width
		m.vp.Height = height
		m.vp.SetContent(m.output)
	}
}

func status(text string, level ui.Level) tea.Cmd { return ui.Status(text, level) }
