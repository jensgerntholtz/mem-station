package app

import (
	"mem-station/internal/ui"

	"charm.land/bubbles/v2/key"
)

// KeyMap holds the global bindings. Tab bindings live in their tab packages.
type KeyMap struct {
	Quit        key.Binding
	Help        key.Binding
	RetryDetect key.Binding
	Elevate     key.Binding
	IMC         key.Binding
	Tab1        key.Binding
	Tab2        key.Binding
	Tab3        key.Binding
}

// DefaultKeyMap returns the global bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit:        key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		RetryDetect: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "retry detection")),
		Elevate:     key.NewBinding(key.WithKeys("E"), key.WithHelp("E", "elevate (sudo)")),
		IMC:         key.NewBinding(key.WithKeys("I"), key.WithHelp("I", "read IMC registers")),
		Tab1:        key.NewBinding(key.WithKeys("f1", "ctrl+1"), key.WithHelp("f1/1", "timings")),
		Tab2:        key.NewBinding(key.WithKeys("f2", "ctrl+2"), key.WithHelp("f2/2", "benchmark")),
		Tab3:        key.NewBinding(key.WithKeys("f3", "ctrl+3"), key.WithHelp("f3/3", "burn-in")),
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
