// Package msgs holds the Bubble Tea message types shared by the app shell and
// the tab packages. Keeping them in a leaf package avoids an app <-> tab import
// cycle.
package msgs

import (
	"time"

	"mem-station/internal/runner"

	tea "charm.land/bubbletea/v2"
)

// JobEvent delivers one runner event. The app routes it to the owning tab by
// Ev.Kind, so events are never lost when a tab is inactive.
type JobEvent struct {
	Ev runner.Event
}

// Tick is emitted by the app's elapsed-time ticker while a job is running.
type Tick struct {
	At time.Time
}

// WaitEvent reads one event from a job's stream and returns it as a message.
func WaitEvent(ch <-chan runner.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return JobEvent{Ev: ev}
	}
}

// TickEvery schedules a Tick after d.
func TickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return Tick{At: t} })
}
