package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem-station/internal/config"
	"mem-station/internal/runner"
)

// newTestDir isolates the state directory for one test.
func newTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("MEM_STATION_STATE_DIR", dir)
	return dir
}

func record(kind runner.Kind, at time.Time, logPath string) Record {
	return Record{
		Kind:       kind,
		StartedAt:  at,
		DurationMS: 1000,
		ExitCode:   0,
		LogPath:    logPath,
		Config:     config.Snapshot{Values: map[string]string{config.KeyFreq: "3200"}, Source: "applied"},
		Payload:    json.RawMessage(`{"ok":true}`),
	}
}

// TestHistoryRoundTrip checks appends, ordering, filtering, and the n limit.
func TestHistoryRoundTrip(t *testing.T) {
	newTestDir(t)
	base := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if err := AppendHistory(record(runner.KindBenchmark, base.Add(time.Duration(i)*time.Minute), "bench.log")); err != nil {
			t.Fatalf("AppendHistory: %v", err)
		}
	}
	if err := AppendHistory(record(runner.KindBurnIn, base.Add(10*time.Minute), "burn.log")); err != nil {
		t.Fatalf("AppendHistory: %v", err)
	}

	recs, err := RecentHistory(runner.KindBenchmark, 10)
	if err != nil {
		t.Fatalf("RecentHistory: %v", err)
	}
	if len(recs) != 3 {
		t.Fatalf("benchmark records = %d, want 3", len(recs))
	}
	if !recs[0].StartedAt.After(recs[1].StartedAt) {
		t.Errorf("records are not newest first: %v", recs)
	}
	if recs[0].LogPath != "bench.log" || string(recs[0].Payload) != `{"ok":true}` {
		t.Errorf("record round-trip lost data: %+v", recs[0])
	}
	if recs[0].ID == "" {
		t.Error("AppendHistory did not assign an ID")
	}

	limited, err := RecentHistory(runner.KindBenchmark, 2)
	if err != nil {
		t.Fatalf("RecentHistory: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("limited records = %d, want 2", len(limited))
	}

	all, err := RecentHistory("", 10)
	if err != nil {
		t.Fatalf("RecentHistory: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("all records = %d, want 4", len(all))
	}
}

// TestHistoryToleratesBadTrailingLine checks a truncated write does not hide the
// earlier records.
func TestHistoryToleratesBadTrailingLine(t *testing.T) {
	dir := newTestDir(t)
	if err := AppendHistory(record(runner.KindBurnIn, time.Now(), "burn.log")); err != nil {
		t.Fatalf("AppendHistory: %v", err)
	}
	path := filepath.Join(dir, "history.jsonl")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open history: %v", err)
	}
	if _, err := f.WriteString(`{"kind":"burnin"` + "\n"); err != nil {
		t.Fatalf("write bad line: %v", err)
	}
	f.Close()

	recs, err := RecentHistory(runner.KindBurnIn, 10)
	if err != nil {
		t.Fatalf("RecentHistory: %v", err)
	}
	if len(recs) != 1 {
		t.Errorf("records = %d, want the one good record", len(recs))
	}
}

// TestRecentHistoryMissingFile checks a fresh state dir has no history.
func TestRecentHistoryMissingFile(t *testing.T) {
	newTestDir(t)
	recs, err := RecentHistory(runner.KindBenchmark, 5)
	if err != nil {
		t.Fatalf("RecentHistory: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("records = %d, want none", len(recs))
	}
}

// TestExportJSON checks the export lands in the state dir with 0600.
func TestExportJSON(t *testing.T) {
	dir := newTestDir(t)
	path, err := ExportJSON("Benchmark Run 42", map[string]string{"hello": "world"})
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}
	if want := filepath.Join(dir, "exports", "benchmark-run-42.json"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat export: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o, want 600", perm)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	var doc map[string]string
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("export json: %v", err)
	}
	if doc["hello"] != "world" {
		t.Errorf("export document = %v", doc)
	}
}

// TestMaterializeJobReplacesTimeout checks the repository job file is copied
// with only its timeout rewritten.
func TestMaterializeJobReplacesTimeout(t *testing.T) {
	dir := newTestDir(t)
	preset := []byte("run parallel\ntimeout 30s\nnull 0\n")
	path, err := MaterializeJob(preset, 90*time.Minute)
	if err != nil {
		t.Fatalf("MaterializeJob: %v", err)
	}
	if !strings.HasPrefix(path, dir) {
		t.Errorf("path = %q, want it under %q", path, dir)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read materialized job: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "timeout 90m") {
		t.Errorf("timeout not replaced:\n%s", content)
	}
	if strings.Contains(content, "timeout 30s") {
		t.Errorf("old timeout survived:\n%s", content)
	}
	if !strings.Contains(content, "null 0") {
		t.Errorf("preset content lost:\n%s", content)
	}
	if string(preset) != "run parallel\ntimeout 30s\nnull 0\n" {
		t.Errorf("preset was mutated: %q", preset)
	}
}

// TestMaterializeJobAppendsMissingTimeout checks a job file without a timeout
// line gets one.
func TestMaterializeJobAppendsMissingTimeout(t *testing.T) {
	newTestDir(t)
	path, err := MaterializeJob([]byte("run sequential\nnull 0"), 5*time.Minute)
	if err != nil {
		t.Fatalf("MaterializeJob: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read materialized job: %v", err)
	}
	if !strings.Contains(string(data), "timeout 5m") {
		t.Errorf("missing timeout not appended:\n%s", data)
	}
}

// TestExportFileToChosenPath checks a user-chosen destination is created with
// private permissions and returns the absolute path.
func TestExportFileToChosenPath(t *testing.T) {
	newTestDir(t)
	path := filepath.Join(t.TempDir(), "nested", "profile.json")

	written, err := ExportFile(path, config.Profile{Name: "active", Values: map[string]string{config.KeyFreq: "3200"}})
	if err != nil {
		t.Fatalf("ExportFile: %v", err)
	}
	if written != path {
		t.Errorf("ExportFile returned %q, want %q", written, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat export: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("export mode = %o, want 600", perm)
	}
	if _, err := ExportFile("", config.Profile{}); err == nil {
		t.Error("ExportFile accepted an empty path")
	}
}

// TestImportProfileValidation rejects malformed files and accepts a profile
// written by ExportFile.
func TestImportProfileValidation(t *testing.T) {
	newTestDir(t)
	dir := t.TempDir()

	valid := filepath.Join(dir, "valid.json")
	if _, err := ExportFile(valid, config.Profile{
		Name:   "valid",
		Values: map[string]string{config.KeyFreq: "3200", config.KeyVoltage: "1.35"},
	}); err != nil {
		t.Fatalf("ExportFile: %v", err)
	}
	profile, err := ImportProfile(valid)
	if err != nil {
		t.Fatalf("ImportProfile(valid): %v", err)
	}
	if profile.Name != "valid" || profile.Values[config.KeyFreq] != "3200" {
		t.Errorf("imported profile = %+v", profile)
	}

	cases := []struct {
		name    string
		content string
	}{
		{"broken json", `{"values":`},
		{"no values", `{"name":"empty","values":{}}`},
		{"unknown key", `{"name":"typo","values":{"dram_feq":"3200"}}`},
	}
	for _, tc := range cases {
		path := filepath.Join(dir, strings.ReplaceAll(tc.name, " ", "-")+".json")
		if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
			t.Fatalf("write %s: %v", tc.name, err)
		}
		if _, err := ImportProfile(path); err == nil {
			t.Errorf("%s: ImportProfile accepted an invalid file", tc.name)
		}
	}

	// A nameless profile takes its name from the file stem.
	nameless := filepath.Join(dir, "daily-3200.json")
	if err := os.WriteFile(nameless, []byte(`{"values":{"dram_freq":"3200"}}`), 0o600); err != nil {
		t.Fatalf("write nameless profile: %v", err)
	}
	profile, err = ImportProfile(nameless)
	if err != nil {
		t.Fatalf("ImportProfile(nameless): %v", err)
	}
	if profile.Name != "daily-3200" {
		t.Errorf("derived name = %q, want daily-3200", profile.Name)
	}

	if _, err := ImportProfile(filepath.Join(dir, "missing.json")); err == nil {
		t.Error("ImportProfile accepted a missing file")
	}
}

// TestExportPathIsAbsolute checks the default export location is inside the
// state dir and absolute, so it can prefill a "save as" prompt.
func TestExportPathIsAbsolute(t *testing.T) {
	dir := newTestDir(t)
	path, err := ExportPath("Profile 1")
	if err != nil {
		t.Fatalf("ExportPath: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("ExportPath returned a relative path %q", path)
	}
	if want := filepath.Join(dir, "exports", "profile-1.json"); path != want {
		t.Errorf("ExportPath = %q, want %q", path, want)
	}
}
