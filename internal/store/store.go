// Package store owns persistence under the XDG state directory: profiles,
// run history, log paths, and burn-in job materialization.
//
// The state dir resolves in order: MEM_STATION_STATE_DIR, then
// $XDG_STATE_HOME/mem-station, then ~/.local/state/mem-station.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"mem-station/internal/config"
	"mem-station/internal/runner"
)

// Dir resolves (and creates, 0700) the state directory.
func Dir() (string, error) {
	if v := os.Getenv("MEM_STATION_STATE_DIR"); v != "" {
		return ensureDir(v)
	}
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return ensureDir(filepath.Join(v, "mem-station"))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return ensureDir(filepath.Join(home, ".local", "state", "mem-station"))
}

func ensureDir(p string) (string, error) {
	if err := os.MkdirAll(p, 0700); err != nil {
		return "", err
	}
	return p, nil
}

// --- Profiles ---

// LoadActiveProfile returns the active profile and whether it exists.
func LoadActiveProfile() (config.Profile, bool, error) {
	p, err := activeProfilePath()
	if err != nil {
		return config.Profile{}, false, err
	}
	return readProfile(p)
}

// SaveActiveProfile atomically writes the active profile.
func SaveActiveProfile(prof config.Profile) error {
	p, err := activeProfilePath()
	if err != nil {
		return err
	}
	return writeProfile(p, prof)
}

// LoadNamedProfile returns a named profile and whether it exists.
func LoadNamedProfile(name string) (config.Profile, bool, error) {
	if name == "" {
		return config.Profile{}, false, fmt.Errorf("store: empty profile name")
	}
	p, err := namedProfilePath(name)
	if err != nil {
		return config.Profile{}, false, err
	}
	return readProfile(p)
}

// SaveNamedProfile atomically writes a named profile.
func SaveNamedProfile(prof config.Profile) error {
	if prof.Name == "" {
		return fmt.Errorf("store: empty profile name")
	}
	p, err := namedProfilePath(prof.Name)
	if err != nil {
		return err
	}
	return writeProfile(p, prof)
}

// ListProfiles returns the names of saved profiles (excluding "active").
func ListProfiles() ([]string, error) {
	d, err := Dir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(d, "profiles")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		if name == "active" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func activeProfilePath() (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "profiles", "active.json"), nil
}

func namedProfilePath(name string) (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "profiles", name+".json"), nil
}

func readProfile(path string) (config.Profile, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return config.Profile{}, false, nil
	}
	if err != nil {
		return config.Profile{}, false, err
	}
	var prof config.Profile
	if err := json.Unmarshal(data, &prof); err != nil {
		return config.Profile{}, false, err
	}
	return prof, true, nil
}

func writeProfile(path string, prof config.Profile) error {
	data, err := json.MarshalIndent(prof, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, data)
}

// --- History ---

// Record is one append-only history entry (JSONL).
type Record struct {
	ID         string          `json:"id"`
	Kind       runner.Kind     `json:"kind"`
	StartedAt  time.Time       `json:"started_at"`
	DurationMS int64           `json:"duration_ms"`
	ExitCode   int             `json:"exit_code"`
	Error      string          `json:"error,omitempty"`
	LogPath    string          `json:"log_path,omitempty"`
	Config     config.Snapshot `json:"config,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

// AppendHistory appends one record to the JSONL history file (0600).
func AppendHistory(r Record) error {
	d, err := Dir()
	if err != nil {
		return err
	}
	if r.ID == "" {
		r.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	path := filepath.Join(d, "history.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// RecentHistory returns the most recent n records of the given kind (newest
// first). It tolerates corrupt trailing lines.
func RecentHistory(kind runner.Kind, n int) ([]Record, error) {
	d, err := Dir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(d, "history.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var all []Record
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rec Record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue // tolerate bad trailing lines
		}
		if kind != "" && rec.Kind != kind {
			continue
		}
		all = append(all, rec)
	}
	if n > 0 && len(all) > n {
		all = all[len(all)-n:]
	}
	// Newest first.
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	return all, nil
}

// --- Logs and job materialization ---

// LogPath returns the absolute-ish log path for a run under logs/.
func LogPath(kind runner.Kind, t time.Time) string {
	d, err := Dir()
	if err != nil {
		d = os.TempDir()
	}
	return filepath.Join(d, "logs", fmt.Sprintf("%s-%s.log", string(kind), t.Format("20060102-150405.000")))
}

var timeoutLineRe = regexp.MustCompile(`(?m)^\s*timeout\s+.*$`)

// MaterializeJob writes preset (the raw job file bytes) to a fresh file under
// jobs/, replacing (or appending) the timeout line with the given duration.
// The returned path is absolute. The source preset is never mutated.
func MaterializeJob(preset []byte, duration time.Duration) (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	jobsDir := filepath.Join(d, "jobs")
	if err := os.MkdirAll(jobsDir, 0700); err != nil {
		return "", err
	}

	replacement := "timeout " + formatJobTimeout(duration)
	content := string(preset)
	if timeoutLineRe.MatchString(content) {
		content = timeoutLineRe.ReplaceAllString(content, replacement)
	} else {
		content = strings.TrimRight(content, "\n") + "\n" + replacement + "\n"
	}

	path := filepath.Join(jobsDir, fmt.Sprintf("burnin-%d.job", time.Now().UnixNano()))
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return "", err
	}
	return filepath.Abs(path)
}

func formatJobTimeout(d time.Duration) string {
	switch {
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	case d%time.Second == 0:
		return fmt.Sprintf("%ds", int(d/time.Second))
	default:
		return fmt.Sprintf("%dms", int(d/time.Millisecond))
	}
}

// atomicWrite writes data to path via a temp file + rename in the same dir.
func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
