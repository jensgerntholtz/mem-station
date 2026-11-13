package sysinfo

import (
	"os"
	"strings"

	"mem-station/internal/style"
	"mem-station/internal/ui"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// newImportPicker builds the JSON profile file browser. It starts in the
// process working directory, shows names only, and lets esc close the dialog
// instead of walking up a directory (h/←/backspace still go up).
func newImportPicker() filepicker.Model {
	fp := filepicker.New()
	fp.DirAllowed = false
	fp.FileAllowed = true
	fp.ShowHidden = false
	fp.ShowPermissions = false
	fp.ShowSize = false
	fp.AutoHeight = true
	fp.AllowedTypes = []string{".json"}

	km := filepicker.DefaultKeyMap()
	km.Back = key.NewBinding(key.WithKeys("h", "backspace", "left"), key.WithHelp("h", "up"))
	fp.KeyMap = km
	return fp
}

// openImportPicker starts the file browser and returns its directory read.
func (m *Model) openImportPicker() tea.Cmd {
	m.commitEdit()
	m.promptErr = ""
	if dir, err := os.Getwd(); err == nil {
		m.importPicker.CurrentDirectory = dir
	}
	m.modal = modalImportProfile
	return m.importPicker.Init()
}

// updateImportPicker routes a message while the import browser owns the input.
// A failed validation keeps the dialog open with the reason, so a wrong file is
// never imported silently.
func (m *Model) updateImportPicker(msg tea.Msg) tea.Cmd {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && keyMsg.String() == "esc" {
		m.closeModal()
		return message("Import cancelled - nothing was changed.", ui.LevelInfo)
	}

	var cmd tea.Cmd
	m.importPicker, cmd = m.importPicker.Update(msg)
	if ok, path := m.importPicker.DidSelectFile(msg); ok {
		m.promptErr = ""
		load := m.importProfile(path)
		if m.promptErr == "" {
			m.closeModal()
			return tea.Batch(cmd, load)
		}
	}
	return cmd
}

// importDialog renders the file browser inside the shared modal frame.
func (m *Model) importDialog(w, h int) string {
	modalW := modalWidth(w)
	innerW := modalW - 4
	clip := func(s string) string { return ui.Truncate(s, innerW, true) }

	// Nine rows of chrome: borders, title, directory, two blanks and the hint.
	m.importPicker.SetHeight(max(3, h-9))
	body := []string{style.Muted().Render(clip(m.importPicker.CurrentDirectory)), ""}
	for _, line := range strings.Split(m.importPicker.View(), "\n") {
		body = append(body, clip(line))
	}
	if m.promptErr != "" {
		body = append(body, "", style.Danger().Render(clip(m.promptErr)))
	}
	body = append(body, "", style.Muted().Render(clip(
		"enter open/select · h/← up · esc cancel · *.json only")))
	return ui.Modal("Import profile JSON", body, modalW)
}
