package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"mem-station/internal/memory"
	"mem-station/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

// elevationDocument mirrors the stable `mem-station info -json` payload (see
// internal/cli/info.go). The shell decodes it instead of importing the CLI
// package, so the dependency direction stays app -> tabs only.
type elevationDocument struct {
	Values   map[string]string   `json:"values"`
	Message  string              `json:"message"`
	SPD      *memory.SPDInfo     `json:"spd"`
	IMC      []memory.IMCTimings `json:"imc"`
	IMCError string              `json:"imc_error"`
}

// elevateMsg carries the result of an elevated detection run.
type elevateMsg struct {
	doc elevationDocument
	err error
}

// elevateCmd re-runs hardware detection with root privileges without leaving
// the TUI: the program hands the terminal to `sudo <exe> info -json` (so sudo
// can prompt for a password), captures the JSON on stdout, and decodes it when
// the TUI resumes. No state is shared through files.
func (m *Model) elevateCmd() tea.Cmd {
	if isRoot() {
		return m.setStatus("Running as root already - detection is unrestricted.", ui.LevelInfo)
	}
	exe, err := os.Executable()
	if err != nil {
		return m.setStatus("Cannot elevate: "+err.Error(), ui.LevelError)
	}

	args := []string{exe, "info", "-json"}
	if m.opts.IMC || m.imcRequested {
		args = append(args, "-imc")
	}
	var out bytes.Buffer
	cmd := exec.Command("sudo", args...)
	// Capture stdout; stdio is otherwise wired to the released terminal, so the
	// sudo password prompt still works while the JSON never hits the screen.
	cmd.Stdout = &out

	return tea.ExecProcess(cmd, func(runErr error) tea.Msg {
		if runErr != nil {
			return elevateMsg{err: runErr}
		}
		var doc elevationDocument
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			snippet := strings.TrimSpace(out.String())
			if len(snippet) > 160 {
				snippet = snippet[:160] + "..."
			}
			return elevateMsg{err: fmt.Errorf("parse privileged info output: %v (%q)", err, snippet)}
		}
		return elevateMsg{doc: doc}
	})
}

// handleElevation applies an elevated detection result to the tabs.
func (m *Model) handleElevation(msg elevateMsg) tea.Cmd {
	if msg.err != nil {
		return m.setStatus("Elevation cancelled or failed: "+msg.err.Error(), ui.LevelError)
	}

	doc := msg.doc
	m.sys.SetDetected(doc.Values, doc.Message, doc.SPD)

	var imcErr error
	if doc.IMCError != "" {
		imcErr = errors.New(doc.IMCError)
	}
	m.sys.SetIMC(doc.IMC, imcErr)
	m.syncSnapshot()

	if len(doc.Values) == 0 {
		return m.setStatus("Elevated detection returned no values: "+doc.Message, ui.LevelError)
	}
	return m.setStatus(fmt.Sprintf("Elevated detection: %d values, SPD %s, IMC %s",
		len(doc.Values), availability(doc.SPD != nil), availability(len(doc.IMC) > 0)), ui.LevelInfo)
}

func availability(ok bool) string {
	if ok {
		return "ok"
	}
	return "unavailable"
}
