package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"mem-station/internal/config"
	"mem-station/internal/runner"
	"mem-station/internal/store"
)

// configSnapshot returns the applied profile as the configuration a headless run
// is attributed to. Headless commands never call hardware detection (that is
// what `info` is for); an unapplied host records an explicit "none" source
// rather than a guessed configuration.
func configSnapshot() config.Snapshot {
	profile, ok, err := store.LoadActiveProfile()
	if err != nil || !ok {
		return config.Snapshot{Source: "none"}
	}
	return config.Snapshot{Values: profile.Values, CapturedAt: profile.UpdatedAt, Source: "applied"}
}

// newRecord builds the history record for a finished headless run. It uses the
// same schema the TUI writes, so `RecentHistory` shows both kinds of run.
func newRecord(kind runner.Kind, res jobResult, payload any) store.Record {
	data, err := json.Marshal(payload)
	if err != nil {
		data = nil
	}
	started := res.StartedAt
	if started.IsZero() {
		started = time.Now()
	}
	id := fmt.Sprintf("%d-%s", started.UnixNano(), kind)
	return store.Record{
		ID:         id,
		Kind:       kind,
		StartedAt:  started,
		DurationMS: res.duration().Milliseconds(),
		ExitCode:   res.ExitCode,
		Error:      res.errText(),
		LogPath:    res.LogPath,
		Config:     configSnapshot(),
		Payload:    data,
	}
}

// persistRecord appends the run to the history file. A store failure is reported
// on stderr but never turns a finished run into a failed command: the log file
// and stdout already carry the result.
func persistRecord(deps Deps, rec store.Record) {
	if err := store.AppendHistory(rec); err != nil {
		fmt.Fprintln(deps.Stderr, "mem-station: history not saved:", err)
	}
}
