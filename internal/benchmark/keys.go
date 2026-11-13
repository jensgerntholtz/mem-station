package benchmark

import (
	"mem-station/internal/ui"

	"charm.land/bubbles/v2/key"
)

// KeyMap holds the Benchmark tab bindings.
type KeyMap struct {
	Run      key.Binding
	Cancel   key.Binding
	Up       key.Binding
	Down     key.Binding
	Prev     key.Binding
	Next     key.Binding
	Follow   key.Binding
	Export   key.Binding
	ScrollUp key.Binding
	ScrollDn key.Binding
}

// DefaultKeyMap returns the Benchmark tab bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Run:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "run")),
		Cancel:   key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "cancel")),
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "config row")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "config row")),
		Prev:     key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "change value")),
		Next:     key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "change value")),
		Follow:   key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "follow log")),
		Export:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "export JSON")),
		ScrollUp: key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "scroll")),
		ScrollDn: key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdown", "scroll")),
	}
}

// Help returns the tab's help sections.
func (m *Model) Help() []ui.HelpSection {
	return []ui.HelpSection{
		{
			Title: "Benchmark",
			Rows: []ui.KeyHint{
				{Key: "enter", Action: "run benchmark"},
				{Key: "x", Action: "cancel run"},
				{Key: "↑/↓", Action: "select config row"},
				{Key: "←/→", Action: "change duration/buffer/workers"},
				{Key: "f", Action: "toggle log follow"},
				{Key: "e", Action: "export last run JSON"},
				{Key: "pgup/pgdown", Action: "scroll log"},
			},
		},
	}
}

func hints(bindings ...key.Binding) []ui.KeyHint {
	out := make([]ui.KeyHint, 0, len(bindings))
	for _, b := range bindings {
		if !b.Enabled() {
			continue
		}
		h := b.Help()
		out = append(out, ui.KeyHint{Key: h.Key, Action: h.Desc})
	}
	return out
}
