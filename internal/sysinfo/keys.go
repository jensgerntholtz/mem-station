package sysinfo

import (
	"mem-station/internal/ui"

	"github.com/charmbracelet/bubbles/key"
)

// KeyMap holds the Timings tab bindings. Every binding in this package is
// declared here so help text and handling can never drift apart.
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Edit      key.Binding
	Cancel    key.Binding
	Next      key.Binding
	Prev      key.Binding
	RatioLock key.Binding
	Apply     key.Binding
	ResetAll  key.Binding
	NextView  key.Binding
	PrevView  key.Binding
	ScrollUp  key.Binding
	ScrollDn  key.Binding

	// Profile keys: named profiles and JSON import/export. Importing and
	// loading only ever change the draft; applying stays a separate confirm.
	SaveProfile   key.Binding
	LoadProfile   key.Binding
	ExportProfile key.Binding
	ImportProfile key.Binding
}

// DefaultKeyMap returns the Timings tab bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "select")),
		Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "select")),
		Edit:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit")),
		Cancel:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel/reset")),
		Next:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field")),
		Prev:      key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev field")),
		RatioLock: key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "ratio lock")),
		Apply:     key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "apply profile")),
		ResetAll:  key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "reset draft")),
		NextView:  key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "next view")),
		PrevView:  key.NewBinding(key.WithKeys("["), key.WithHelp("[", "prev view")),
		ScrollUp:  key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "scroll up")),
		ScrollDn:  key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdown", "scroll down")),

		SaveProfile:   key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "save profile")),
		LoadProfile:   key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "load profile")),
		ExportProfile: key.NewBinding(key.WithKeys("ctrl+e"), key.WithHelp("ctrl+e", "export JSON")),
		ImportProfile: key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("ctrl+o", "import JSON")),
	}
}

// Keys returns the context hints for the footer.
func (m *Model) Keys() []ui.KeyHint {
	switch {
	case m.modal != modalNone:
		return []ui.KeyHint{{Key: "enter", Action: "confirm"}, {Key: "esc", Action: "cancel"}}
	case m.editing:
		return hints(m.keys.Edit, m.keys.Cancel, m.keys.Next)
	case m.subView != subTimings:
		return hints(m.keys.ScrollUp, m.keys.ScrollDn, m.keys.PrevView, m.keys.NextView)
	default:
		return hints(m.keys.Up, m.keys.Edit, m.keys.Next, m.keys.RatioLock, m.keys.Apply, m.keys.NextView)
	}
}

// Help returns the tab's help sections.
func (m *Model) Help() []ui.HelpSection {
	global := ui.HelpSection{Title: "Global", Rows: []ui.KeyHint{
		{Key: "f1/f2/f3", Action: "tabs (or 1/2/3)"},
		{Key: "?", Action: "help"},
		{Key: "esc", Action: "close help"},
		{Key: "q", Action: "quit"},
	}}
	if m.modal != modalNone {
		return []ui.HelpSection{
			{Title: m.modal.Title(), Rows: []ui.KeyHint{
				{Key: "enter", Action: "confirm"},
				{Key: "esc", Action: "cancel - nothing is changed"},
			}},
			global,
		}
	}
	if m.editing {
		return []ui.HelpSection{
			{Title: "Editing", Rows: hints(m.keys.Edit, m.keys.Cancel, m.keys.Next, m.keys.Prev)},
			{Title: "Text", Rows: []ui.KeyHint{
				{Key: "←/→", Action: "move cursor"},
				{Key: "home/end", Action: "line start/end"},
				{Key: "backspace", Action: "delete"},
			}},
			global,
		}
	}
	tab := ui.HelpSection{Title: "Timings", Rows: hints(
		m.keys.Up, m.keys.Down, m.keys.Edit, m.keys.Cancel, m.keys.Next, m.keys.Prev,
		m.keys.RatioLock, m.keys.Apply, m.keys.ResetAll,
	)}
	apply := ui.HelpSection{Title: "Apply", Rows: []ui.KeyHint{
		{Key: "ctrl+s", Action: "review the diff, then enter to save"},
		{Key: "esc", Action: "revert the draft / cancel the review"},
	}}
	views := ui.HelpSection{Title: "Views", Rows: hints(m.keys.PrevView, m.keys.NextView, m.keys.ScrollUp, m.keys.ScrollDn)}
	profiles := ui.HelpSection{Title: "Profiles", Rows: hints(
		m.keys.SaveProfile, m.keys.LoadProfile, m.keys.ExportProfile, m.keys.ImportProfile,
	)}
	return []ui.HelpSection{tab, apply, views, profiles, global}
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
