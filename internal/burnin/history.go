package burnin

import (
	"encoding/json"
	"fmt"
	"time"

	"mem-station/internal/config"
	"mem-station/internal/runner"
	"mem-station/internal/store"
	"mem-station/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

// Verdict values persisted with a run.
const (
	verdictPassed    = "passed"
	verdictFailed    = "failed"
	verdictCancelled = "cancelled"
)

// Payload is the stable JSON schema of a burn-in result.
type Payload struct {
	Options        payloadOptions `json:"options"`
	Verdict        string         `json:"verdict"`
	Success        bool           `json:"success"`
	Errors         int            `json:"errors"`
	ParsedDuration string         `json:"parsed_duration,omitempty"`
	StoppedOnError bool           `json:"stopped_on_error,omitempty"`
}

// Export is the stable JSON schema the export command writes to disk.
type Export struct {
	Kind       string          `json:"kind"`
	StartedAt  time.Time       `json:"started_at"`
	DurationMS int64           `json:"duration_ms"`
	ExitCode   int             `json:"exit_code"`
	Error      string          `json:"error,omitempty"`
	LogPath    string          `json:"log_path"`
	Config     config.Snapshot `json:"config"`
	Result     Payload         `json:"result"`
}

// runRecord is one history entry together with its decoded payload.
type runRecord struct {
	Record store.Record
	Result Payload
}

func decodePayload(raw json.RawMessage) Payload {
	var p Payload
	if len(raw) == 0 {
		return p
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return Payload{}
	}
	return p
}

// payloadOf converts the finished run into the stored schema.
func (m *Model) payloadOf() Payload {
	p := Payload{
		Options:        m.runOptions.payload(m.runPreset.ShortName()),
		Errors:         m.errors,
		StoppedOnError: m.stoppedOnErr,
		Success:        m.state == runner.StateDone,
		Verdict:        verdictPassed,
	}
	if m.results != nil {
		p.ParsedDuration = m.results.Duration
	}
	switch m.state {
	case runner.StateDone:
		p.Verdict = verdictPassed
	case runner.StateCancelled:
		p.Verdict = verdictCancelled
	default:
		p.Verdict = verdictFailed
	}
	return p
}

// recordFor builds the history record for the finished run.
func (m *Model) recordFor(p Payload) store.Record {
	data, err := json.Marshal(p)
	if err != nil {
		data = nil
	}
	return store.Record{
		ID:         fmt.Sprintf("%d", m.startedAt.UnixNano()),
		Kind:       runner.KindBurnIn,
		StartedAt:  m.startedAt,
		DurationMS: m.elapsed().Milliseconds(),
		ExitCode:   m.exitCode,
		Error:      m.errText,
		LogPath:    m.logPath,
		Config:     m.snapshot.Clone(),
		Payload:    data,
	}
}

// appendHistory persists the finished run and prepends it to the in-memory
// history. A store failure is surfaced in the tab, never faked.
func (m *Model) appendHistory(p Payload) {
	rec := m.recordFor(p)
	if err := store.AppendHistory(rec); err != nil {
		m.historyErr = "history not saved: " + err.Error()
	} else {
		m.historyErr = ""
	}
	m.lastID = rec.ID
	m.runs = append([]runRecord{{Record: rec, Result: p}}, m.runs...)
	if len(m.runs) > historyLimit {
		m.runs = m.runs[:historyLimit]
	}
}

// loadHistory reads the recent runs from the store.
func (m *Model) loadHistory() {
	recs, err := store.RecentHistory(runner.KindBurnIn, historyLimit)
	if err != nil {
		m.historyErr = "history unavailable: " + err.Error()
		return
	}
	m.runs = make([]runRecord, 0, len(recs))
	for _, rec := range recs {
		m.runs = append(m.runs, runRecord{Record: rec, Result: decodePayload(rec.Payload)})
	}
}

// export writes the most recent completed run to the state dir as JSON.
func (m *Model) export() tea.Cmd {
	if len(m.runs) == 0 {
		return status("No burn-in run to export yet.", ui.LevelWarn)
	}
	rec := m.runs[0]
	name := "burnin-" + rec.Record.StartedAt.UTC().Format("20060102T150405")
	path, err := store.ExportJSON(name, exportFor(rec))
	if err != nil {
		return status("Export failed: "+err.Error(), ui.LevelError)
	}
	return status("Exported "+path, ui.LevelInfo)
}

// exportFor builds the export document for one stored run.
func exportFor(rec runRecord) Export {
	return Export{
		Kind:       string(rec.Record.Kind),
		StartedAt:  rec.Record.StartedAt,
		DurationMS: rec.Record.DurationMS,
		ExitCode:   rec.Record.ExitCode,
		Error:      rec.Record.Error,
		LogPath:    rec.Record.LogPath,
		Config:     rec.Record.Config,
		Result:     rec.Result,
	}
}
