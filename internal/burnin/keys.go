package burnin

import (
	"mem-station/internal/ui"

	"github.com/charmbracelet/bubbles/key"
)

// KeyMap holds the Burn-in tab bindings.
type KeyMap struct {
	Run      key.Binding
	Cancel   key.Binding
	ScrollUp key.Binding
	ScrollDn key.Binding
}

// DefaultKeyMap returns the Burn-in tab bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Run:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "run")),
		Cancel:   key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "cancel")),
		ScrollUp: key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "scroll")),
		ScrollDn: key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdown", "scroll")),
	}
}

// Help returns the tab's help sections.
func (m *Model) Help() []ui.HelpSection {
	return []ui.HelpSection{
		{Title: "Burn-in", Rows: hints(m.keys.Run, m.keys.Cancel, m.keys.ScrollUp, m.keys.ScrollDn)},
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
