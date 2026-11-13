// Package store manages the XDG state directory, profiles, run history, and
// materialized job files.
package store

import (
	"bufio"
	"bytes"
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

// Record is a single persisted run history entry.
type Record struct {
	ID         string          `json:"id"`
	Kind       runner.Kind     `json:"kind"`
	StartedAt  time.Time       `json:"started_at"`
	DurationMS int64           `json:"duration_ms"`
	ExitCode   int             `json:"exit_code"`
	Error      string          `json:"error,omitempty"`
	LogPath    string          `json:"log_path,omitempty"`
	Config     config.Snapshot `json:"config"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

var jobTimeoutRe = regexp.MustCompile(`(?m)^[ \t]*timeout[ \t]+.*$`)

// Dir resolves and creates the state directory:
// MEM_STATION_STATE_DIR, else $XDG_STATE_HOME/mem-station, else
// ~/.local/state/mem-station.
func Dir() (string, error) {
	if d := os.Getenv("MEM_STATION_STATE_DIR"); d != "" {
		return ensureDir(d)
	}
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return ensureDir(filepath.Join(d, "mem-station"))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return ensureDir(filepath.Join(home, ".local", "state", "mem-station"))
}

// LogPath returns the path where a run of the given kind should be logged.
// The directory is not created here; the runner creates it when writing.
func LogPath(kind runner.Kind, t time.Time) string {
	dir, err := Dir()
	if err != nil {
		dir = filepath.Join(os.TempDir(), "mem-station")
	}
	name := fmt.Sprintf("%s-%s.log", kind, t.UTC().Format("20060102T150405.000Z"))
	return filepath.Join(dir, "logs", name)
}

// LoadActiveProfile loads the active profile. The bool is false when no
// profile exists yet.
func LoadActiveProfile() (config.Profile, bool, error) {
	dir, err := Dir()
	if err != nil {
		return config.Profile{}, false, err
	}
	return loadProfile(filepath.Join(dir, "profiles", "active.json"))
}

// SaveActiveProfile atomically persists the active profile.
func SaveActiveProfile(p config.Profile) error {
	p.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	dir, err := Dir()
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, "profiles", "active.json"), data, 0600)
}

// LoadNamedProfile loads a named profile by its display name.
func LoadNamedProfile(name string) (config.Profile, bool, error) {
	if strings.TrimSpace(name) == "" {
		return config.Profile{}, false, fmt.Errorf("profile name is required")
	}
	dir, err := Dir()
	if err != nil {
		return config.Profile{}, false, err
	}
	return loadProfile(filepath.Join(dir, "profiles", profileFileName(name)))
}

// SaveNamedProfile atomically persists a named profile.
func SaveNamedProfile(p config.Profile) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("profile name is required")
	}
	p.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	dir, err := Dir()
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, "profiles", profileFileName(p.Name)), data, 0600)
}

// ListProfiles returns sorted profile names, excluding the active profile.
func ListProfiles() ([]string, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(dir, "profiles"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || e.Name() == "active.json" || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".json"))
	}
	sort.Strings(names)
	return names, nil
}

// AppendHistory appends a record to the JSONL history file.
func AppendHistory(r Record) error {
	if r.ID == "" {
		r.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if r.StartedAt.IsZero() {
		r.StartedAt = time.Now()
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	dir, err := Dir()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "history.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

// RecentHistory returns up to n most-recent records of the given kind,
// newest first. An empty kind matches all records.
func RecentHistory(kind runner.Kind, n int) ([]Record, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(dir, "history.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			continue // tolerate bad trailing lines
		}
		if kind == "" || r.Kind == kind {
			out = append(out, r)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out, nil
}

// ExportJSON writes v as indented JSON to <state>/exports/<name>.json and
// returns the absolute path. Writes are atomic so an export never leaves a
// partial file behind.
func ExportJSON(name string, v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "exports", sanitizeName(name)+".json")
	if err := writeFileAtomic(path, append(data, '\n'), 0600); err != nil {
		return "", err
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs, nil
	}
	return path, nil
}

// MaterializeJob writes a copy of preset into the state dir with its timeout
// line replaced by the given duration (appended if absent). The repo job file
// is never modified. It returns the absolute path to the materialized job.
func MaterializeJob(preset []byte, duration time.Duration) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	jobsDir := filepath.Join(dir, "jobs")
	if err := os.MkdirAll(jobsDir, 0700); err != nil {
		return "", err
	}

	timeoutLine := "timeout " + formatDuration(duration)
	content := string(preset)
	replaced := jobTimeoutRe.ReplaceAllString(content, timeoutLine)
	if replaced == content {
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		replaced = content + timeoutLine + "\n"
	}

	path := filepath.Join(jobsDir, "burnin-memory.job")
	if err := writeFileAtomic(path, []byte(replaced), 0600); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}
	return abs, nil
}

func ensureDir(d string) (string, error) {
	if err := os.MkdirAll(d, 0700); err != nil {
		return "", err
	}
	return d, nil
}

func loadProfile(path string) (config.Profile, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return config.Profile{}, false, nil
	}
	if err != nil {
		return config.Profile{}, false, err
	}
	var p config.Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return config.Profile{}, false, fmt.Errorf("parse profile %s: %w", path, err)
	}
	return p, true, nil
}

func profileFileName(name string) string {
	s := sanitizeName(name)
	if s == "" {
		s = "profile"
	}
	return s + ".json"
}

// sanitizeName reduces a display name to a safe file stem.
func sanitizeName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d <= 0 {
		return "0s"
	}
	switch {
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	default:
		return fmt.Sprintf("%ds", d/time.Second)
	}
}
