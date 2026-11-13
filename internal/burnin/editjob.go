package burnin

import (
	"mem-station/internal/ui"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/editor"
)

// editJobMsg carries the editor's exit status back into the tab.
type editJobMsg struct{ err error }

// editJob opens the selected preset's job file in $EDITOR, handing the terminal
// to the editor and reloading the presets when it returns.
func (m *Model) editJob() tea.Cmd {
	preset := m.preset()
	if preset.Path == "" {
		return status("The built-in preset has no file to edit - start with --job PATH to use your own job file.", ui.LevelWarn)
	}
	cmd, err := editor.Command("mem-station", preset.Path)
	if err != nil {
		return status("Cannot open the editor: "+err.Error(), ui.LevelError)
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return editJobMsg{err: err} })
}

// handleEditJob reloads the presets so the next run uses the edited file. A
// running job is unaffected: it already holds its own materialized copy.
func (m *Model) handleEditJob(msg editJobMsg) tea.Cmd {
	presets, warn := Presets(m.customJob)
	m.presets = presets
	m.presetWarn = warn
	if m.presetIndex >= len(m.presets) {
		m.presetIndex = 0
	}
	switch {
	case msg.err != nil:
		return status("Editor exited with an error: "+msg.err.Error(), ui.LevelWarn)
	case warn != "":
		return status(warn, ui.LevelWarn)
	default:
		return status("Reloaded the job file - the next run uses the edited version.", ui.LevelInfo)
	}
}
