package benchmark

import (
	"encoding/json"
	"fmt"
	"time"

	"mem-station/internal/bench"
	"mem-station/internal/config"
	"mem-station/internal/runner"
	"mem-station/internal/store"
	"mem-station/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

// Metric is one parsed memrate sub-metric in the persisted schema.
type Metric struct {
	Label string  `json:"label"`
	MBs   float64 `json:"mbs"`
}

// Payload is the stable JSON schema of a benchmark result.
type Payload struct {
	Options        payloadOptions `json:"options"`
	Parsed         bool           `json:"parsed"`
	ReadMBs        float64        `json:"read_mbs"`
	WriteMBs       float64        `json:"write_mbs"`
	Metrics        []Metric       `json:"metrics,omitempty"`
	ParsedDuration string         `json:"parsed_duration,omitempty"`
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

// payloadOf converts the parsed results of one run into the stored schema.
func payloadOf(opts Options, results *bench.BenchResults) Payload {
	p := Payload{Options: opts.payload()}
	if results == nil {
		return p
	}
	p.Parsed = results.ReadRate > 0 || results.WriteRate > 0 || len(results.Metrics) > 0
	p.ReadMBs = results.ReadRate
	p.WriteMBs = results.WriteRate
	p.ParsedDuration = results.Duration
	for _, mt := range results.Metrics {
		p.Metrics = append(p.Metrics, Metric{Label: mt.Label, MBs: mt.Value})
	}
	return p
}

// optionsFor returns the configuration a stored payload ran with.
func (p Payload) optionsFor() Options { return p.Options.options() }

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

// recordFor builds the history record for the finished run.
func (m *Model) recordFor(p Payload) store.Record {
	data, err := json.Marshal(p)
	if err != nil {
		data = nil
	}
	id := fmt.Sprintf("%d", m.startedAt.UnixNano())
	return store.Record{
		ID:         id,
		Kind:       runner.KindBenchmark,
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
// history. A store failure is surfaced in the history footer, never faked.
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
	recs, err := store.RecentHistory(runner.KindBenchmark, historyLimit)
	if err != nil {
		m.historyErr = "history unavailable: " + err.Error()
		return
	}
	m.runs = make([]runRecord, 0, len(recs))
	for _, rec := range recs {
		m.runs = append(m.runs, runRecord{Record: rec, Result: decodePayload(rec.Payload)})
	}
}

// previous returns the newest parsed run with the given configuration
// signature, excluding the run identified by excludeID.
func (m *Model) previous(sig, excludeID string) (Payload, bool) {
	for _, rec := range m.runs {
		if rec.Record.ID == excludeID || !rec.Result.Parsed {
			continue
		}
		if rec.Result.optionsFor().Signature() == sig {
			return rec.Result, true
		}
	}
	return Payload{}, false
}

// export writes the most recent completed run to the state dir as JSON.
func (m *Model) export() tea.Cmd {
	if len(m.runs) == 0 {
		return status("No benchmark run to export yet.", ui.LevelWarn)
	}
	rec := m.runs[0]
	name := "benchmark-" + rec.Record.StartedAt.UTC().Format("20060102T150405")
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
