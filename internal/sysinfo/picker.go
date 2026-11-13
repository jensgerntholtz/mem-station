package sysinfo

import (
	"fmt"
	"io"
	"strings"

	"mem-station/internal/store"
	"mem-station/internal/style"
	"mem-station/internal/ui"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// profileItem is one saved profile in the load picker.
type profileItem struct{ name string }

func (p profileItem) FilterValue() string { return p.name }
func (p profileItem) Title() string       { return p.name }

// profileDelegate renders one single-line profile row. The stock default
// delegate reserves two lines per item, which is more than a profile name
// needs inside a dialog.
type profileDelegate struct{}

func (profileDelegate) Height() int  { return 1 }
func (profileDelegate) Spacing() int { return 0 }

func (profileDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (profileDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	profile, ok := item.(profileItem)
	if !ok {
		return
	}
	th := style.ActiveTheme()
	title := ui.Truncate(profile.Title(), max(1, m.Width()-2), true)
	if index == m.Index() {
		fmt.Fprint(w, lipgloss.NewStyle().
			Bold(true).Foreground(th.TabActiveFg).Background(th.AccentBg).
			Padding(0, 1).Render(title))
		return
	}
	fmt.Fprint(w, lipgloss.NewStyle().
		Foreground(th.Label).Padding(0, 1).Render(title))
}

// newProfilePicker builds the load-profile list. The list chrome (title, help,
// status bar, pagination) is off because the dialog frame supplies it.
func newProfilePicker() list.Model {
	picker := list.New(nil, profileDelegate{}, 0, 0)
	picker.SetShowTitle(false)
	picker.SetShowHelp(false)
	picker.SetShowStatusBar(false)
	picker.SetShowPagination(false)
	picker.SetShowFilter(true)
	picker.SetFilteringEnabled(true)

	// The picker is a modal, not a screen: it must never quit the program.
	// Enter, esc and q are handled by updatePicker, and "/" starts filtering.
	km := list.DefaultKeyMap()
	km.Quit = key.NewBinding()
	km.ForceQuit = key.NewBinding()
	picker.KeyMap = km
	return picker
}

// refreshProfiles reloads the saved profile names and clears any stale filter.
func (m *Model) refreshProfiles() {
	m.pickerErr = ""
	names, err := store.ListProfiles()
	if err != nil {
		m.pickerErr = err.Error()
	}
	items := make([]list.Item, 0, len(names))
	for _, name := range names {
		items = append(items, profileItem{name: name})
	}
	m.picker.ResetFilter()
	m.picker.SetItems(items)
	m.picker.Select(0)
}

// updatePicker routes a message while the load picker owns the input.
func (m *Model) updatePicker(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && !m.picker.SettingFilter() {
		switch keyMsg.String() {
		case "enter":
			return m.loadSelectedProfile()
		case "esc", "q":
			m.closeModal()
			return message("Load cancelled - nothing was changed.", ui.LevelInfo)
		}
	}
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	return cmd
}

// loadSelectedProfile copies the highlighted profile into the draft.
func (m *Model) loadSelectedProfile() tea.Cmd {
	item, ok := m.picker.SelectedItem().(profileItem)
	if !ok {
		return message("No profile selected - ctrl+p saves the draft first.", ui.LevelWarn)
	}
	m.closeModal()
	return m.loadNamedProfile(item.name)
}

// pickerDialog renders the load dialog: the list plus a key hint. The shell
// centres and composites it over the tab.
func (m *Model) pickerDialog(w, h int) string {
	modalW := modalWidth(w)
	innerW := modalW - 4
	rows := len(m.picker.Items())
	if rows == 0 {
		rows = 1 // the list's "No items." line
	}
	listH := min(rows+1, max(3, h-8))
	m.picker.SetSize(max(8, innerW), listH)

	lines := strings.Split(m.picker.View(), "\n")
	body := make([]string, 0, len(lines)+3)
	body = append(body, lines...)
	if m.pickerErr != "" {
		body = append(body, style.Danger().Render(ui.Truncate(m.pickerErr, innerW, true)))
	}
	body = append(body, "", style.Muted().Render(ui.Truncate(
		"↑/↓ select · / filter · enter load · esc cancel", innerW, true)))
	return ui.Modal("Load profile", body, modalW)
}
