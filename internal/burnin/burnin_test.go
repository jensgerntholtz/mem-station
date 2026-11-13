package burnin

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"mem-station/internal/burnin/parse"
	"mem-station/internal/config"
	"mem-station/internal/msgs"
	"mem-station/internal/runner"
	"mem-station/internal/store"
	"mem-station/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files")

var (
	testStart = time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	testEnd   = testStart.Add(5 * time.Minute)
)

// fakeJob is a runner.Handle whose events the test feeds by hand.
type fakeJob struct {
	spec      runner.Spec
	events    chan runner.Event
	cancelled bool
}

func (f *fakeJob) Events() <-chan runner.Event { return f.events }
func (f *fakeJob) Cancel()                     { f.cancelled = true }

func testSnapshot() config.Snapshot {
	return config.Snapshot{
		Values: map[string]string{
			config.KeyChannels: "2",
			config.KeyFreq:     "3200",
		},
		Source: "applied",
	}
}

// newTestModel returns a tab wired to a fake job factory, a pinned clock, and
// an isolated state directory.
func newTestModel(t *testing.T) *Model {
	t.Helper()
	t.Setenv("MEM_STATION_STATE_DIR", t.TempDir())
	m := New("", testSnapshot())
	m.now = func() time.Time { return testStart.Add(time.Minute) }
	m.start = func(_ context.Context, spec runner.Spec) runner.Handle {
		return &fakeJob{spec: spec, events: make(chan runner.Event, 64)}
	}
	return m
}

func startRun(t *testing.T, m *Model) *fakeJob {
	t.Helper()
	m.run()
	job, ok := m.job.(*fakeJob)
	if !ok {
		t.Fatalf("run did not start a fake job: %T", m.job)
	}
	return job
}

func event(m *Model, ev runner.Event) {
	ev.Kind = runner.KindBurnIn
	m.Update(msgs.JobEvent{Ev: ev})
}

func started(m *Model, logPath string) {
	event(m, runner.Event{Type: runner.EventStarted, At: testStart, LogPath: logPath})
}

func output(m *Model, line string) {
	event(m, runner.Event{Type: runner.EventOutput, Line: line, At: testStart.Add(time.Second)})
}

func finished(m *Model, exitCode int, err error, logPath string) {
	event(m, runner.Event{Type: runner.EventFinished, At: testEnd, ExitCode: exitCode, Err: err, LogPath: logPath})
}

// TestPresetsIncludeEmbeddedPreset checks the built-in job file ships with the
// binary and is the default choice.
func TestPresetsIncludeEmbeddedPreset(t *testing.T) {
	t.Setenv("MEM_STATION_STATE_DIR", t.TempDir())
	t.Setenv("MEM_STATION_JOB", "")
	presets, warn := Presets("")
	if warn != "" {
		t.Errorf("unexpected warning: %s", warn)
	}
	if len(presets) != 1 {
		t.Fatalf("presets = %d, want only the embedded one", len(presets))
	}
	p := presets[0]
	if p.Path != "" {
		t.Errorf("embedded preset path = %q, want empty", p.Path)
	}
	if len(p.Content) == 0 || !strings.Contains(string(p.Content), "run parallel") {
		t.Errorf("embedded preset content missing:\n%s", p.Content)
	}
}

// TestPresetsCustomPath checks an explicit job file is offered alongside the
// embedded preset, and that a missing explicit file is reported.
func TestPresetsCustomPath(t *testing.T) {
	t.Setenv("MEM_STATION_STATE_DIR", t.TempDir())
	t.Setenv("MEM_STATION_JOB", "")

	path := filepath.Join(t.TempDir(), "custom.job")
	if err := os.WriteFile(path, []byte("timeout 1m\nnull 0\n"), 0o600); err != nil {
		t.Fatalf("write job: %v", err)
	}
	presets, warn := Presets(path)
	if warn != "" {
		t.Errorf("unexpected warning: %s", warn)
	}
	if len(presets) != 2 {
		t.Fatalf("presets = %d, want embedded plus custom", len(presets))
	}
	if presets[1].Path != path || !strings.Contains(string(presets[1].Content), "null 0") {
		t.Errorf("custom preset = %+v", presets[1])
	}

	presets, warn = Presets(filepath.Join(t.TempDir(), "missing.job"))
	if len(presets) != 1 {
		t.Errorf("presets with a missing custom file = %d, want the embedded one", len(presets))
	}
	if !strings.Contains(warn, "unavailable") {
		t.Errorf("warning = %q, want it to mention the missing file", warn)
	}
}

// TestRunMaterializesJobInStateDir checks the chosen duration is written to a
// copy in the state dir and the repo job file is never touched.
func TestRunMaterializesJobInStateDir(t *testing.T) {
	m := newTestModel(t)
	m.row = rowDuration
	m.cycle(-1) // 10m
	m.cycle(-1) // 5m
	m.row = rowStopOnError
	m.cycle(-1) // off

	job := startRun(t, m)
	if m.State() != runner.StateRunning {
		t.Errorf("state = %v, want running", m.State())
	}
	if len(job.spec.Args) != 2 || job.spec.Args[0] != "--job" {
		t.Fatalf("args = %v, want --job <path>", job.spec.Args)
	}
	dir, err := store.Dir()
	if err != nil {
		t.Fatalf("store.Dir: %v", err)
	}
	jobPath := job.spec.Args[1]
	if !strings.HasPrefix(jobPath, dir) {
		t.Errorf("job path %q is outside the state dir %q", jobPath, dir)
	}
	data, err := os.ReadFile(jobPath)
	if err != nil {
		t.Fatalf("read materialized job: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "timeout 5m") {
		t.Errorf("materialized job has no timeout 5m:\n%s", content)
	}
	if !strings.Contains(content, "memthrash") {
		t.Errorf("materialized job lost the preset stressors:\n%s", content)
	}
	if m.runOptions.Duration != 5*time.Minute || m.runOptions.StopOnError {
		t.Errorf("run options = %+v", m.runOptions)
	}
}

// TestSuccessfulRunPersistsHistory drives a passing run.
func TestSuccessfulRunPersistsHistory(t *testing.T) {
	m := newTestModel(t)
	job := startRun(t, m)
	started(m, job.spec.LogPath)
	output(m, "stress-ng: info: dispatching stressors")
	output(m, "stress-ng: info: successful run completed in 300.00 secs (0 errors)")
	finished(m, 0, nil, job.spec.LogPath)

	if m.State() != runner.StateDone {
		t.Fatalf("state = %v (%s), want done", m.State(), m.errText)
	}
	if m.results == nil || !m.results.Success {
		t.Fatalf("results = %+v, want success", m.results)
	}
	if m.errors != 0 {
		t.Errorf("error count = %d, want 0", m.errors)
	}
	out := m.View(78, 21)
	for _, want := range []string{"PASS", "Result"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q:\n%s", want, out)
		}
	}

	recs, err := store.RecentHistory(runner.KindBurnIn, 10)
	if err != nil {
		t.Fatalf("RecentHistory: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("history = %d records, want 1", len(recs))
	}
	if recs[0].LogPath != job.spec.LogPath || recs[0].DurationMS != int64(testEnd.Sub(testStart)/time.Millisecond) {
		t.Errorf("record = %+v", recs[0])
	}
	var payload Payload
	if err := json.Unmarshal(recs[0].Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if payload.Verdict != verdictPassed || !payload.Success || payload.Options.PresetName == "" {
		t.Errorf("payload = %+v", payload)
	}
	if payload.Options.DurationSeconds != 1800 {
		t.Errorf("payload duration = %d, want the default 1800", payload.Options.DurationSeconds)
	}
}

// TestErrorLinesFailTheRunEvenWithExitZero checks success needs zero parsed
// errors, not just exit 0.
func TestErrorLinesFailTheRunEvenWithExitZero(t *testing.T) {
	m := newTestModel(t)
	job := startRun(t, m)
	started(m, job.spec.LogPath)
	output(m, "stress-ng: info: starting")
	output(m, "stress-ng: error: [123] memthrash: memory compare failed")
	finished(m, 0, nil, job.spec.LogPath)

	if m.State() != runner.StateFailed {
		t.Fatalf("state = %v, want failed", m.State())
	}
	if m.errors != 1 {
		t.Errorf("error count = %d, want 1", m.errors)
	}
	if !strings.Contains(m.errText, "error lines") {
		t.Errorf("error text = %q", m.errText)
	}
	out := m.View(78, 21)
	if !strings.Contains(out, "error line(s)") {
		t.Errorf("error banner missing:\n%s", out)
	}
	recs, _ := store.RecentHistory(runner.KindBurnIn, 10)
	if len(recs) != 1 {
		t.Fatalf("history = %d records, want 1", len(recs))
	}
	var payload Payload
	_ = json.Unmarshal(recs[0].Payload, &payload)
	if payload.Verdict != verdictFailed || payload.Errors != 1 {
		t.Errorf("payload = %+v", payload)
	}
}

// TestStopOnErrorCancels checks the first error line cancels the process group
// and the run is reported as failed, not cancelled.
func TestStopOnErrorCancels(t *testing.T) {
	m := newTestModel(t)
	if !m.stopOnError {
		t.Fatal("stop on error should default to on")
	}
	job := startRun(t, m)
	started(m, job.spec.LogPath)
	output(m, "stress-ng: error: [42] malloc: out of memory")
	if !job.cancelled {
		t.Fatal("stop on error did not cancel the job")
	}
	if m.State() != runner.StateRunning {
		t.Errorf("state after cancel = %v, want running until Finished", m.State())
	}
	finished(m, -1, runner.ErrCancelled, job.spec.LogPath)
	if m.State() != runner.StateFailed {
		t.Errorf("state = %v, want failed", m.State())
	}
	if !strings.Contains(m.errText, "stopped on first error") {
		t.Errorf("error text = %q", m.errText)
	}
	var payload Payload
	recs, _ := store.RecentHistory(runner.KindBurnIn, 10)
	if len(recs) != 1 {
		t.Fatalf("history = %d records, want 1", len(recs))
	}
	_ = json.Unmarshal(recs[0].Payload, &payload)
	if payload.Verdict != verdictFailed || !payload.StoppedOnError {
		t.Errorf("payload = %+v", payload)
	}
}

// TestUserCancelIsDistinctFromFailure checks cancelled runs are not reported as
// failures and keep their own verdict.
func TestUserCancelIsDistinctFromFailure(t *testing.T) {
	m := newTestModel(t)
	job := startRun(t, m)
	started(m, job.spec.LogPath)
	output(m, "stress-ng: info: running")

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if !job.cancelled {
		t.Fatal("x did not cancel the job")
	}
	finished(m, -1, runner.ErrCancelled, job.spec.LogPath)

	if m.State() != runner.StateCancelled {
		t.Fatalf("state = %v, want cancelled", m.State())
	}
	out := m.View(78, 21)
	if !strings.Contains(out, "CANCELLED") {
		t.Errorf("view does not show the cancelled verdict:\n%s", out)
	}
	recs, _ := store.RecentHistory(runner.KindBurnIn, 10)
	if len(recs) != 1 {
		t.Fatalf("history = %d records, want 1", len(recs))
	}
	var payload Payload
	_ = json.Unmarshal(recs[0].Payload, &payload)
	if payload.Verdict != verdictCancelled || payload.Success {
		t.Errorf("payload = %+v", payload)
	}
}

// TestErrorsOnlyFilter checks e filters the log down to error lines.
func TestErrorsOnlyFilter(t *testing.T) {
	m := newTestModel(t)
	job := startRun(t, m)
	started(m, job.spec.LogPath)
	output(m, "stress-ng: info: starting")
	output(m, "stress-ng: error: one")
	output(m, "stress-ng: info: running")
	output(m, "stress-ng: error: two")

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if !m.errorsOnly {
		t.Fatal("e did not switch to the errors-only filter")
	}
	if got := m.filtered(); strings.Contains(got, "info:") || !strings.Contains(got, "error: one") || !strings.Contains(got, "error: two") {
		t.Errorf("filtered log = %q", got)
	}
	out := m.View(118, 37)
	if !strings.Contains(out, "filter: errors") {
		t.Errorf("view does not report the filter:\n%s", out)
	}
}

// TestExportWritesJSON checks s writes the last run to the state dir.
func TestExportWritesJSON(t *testing.T) {
	m := newTestModel(t)
	job := startRun(t, m)
	started(m, job.spec.LogPath)
	output(m, "stress-ng: info: successful run completed in 300.00 secs (0 errors)")
	finished(m, 0, nil, job.spec.LogPath)

	cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if cmd == nil {
		t.Fatal("export produced no status message")
	}
	msg, ok := cmd().(ui.StatusMsg)
	if !ok || msg.Level != ui.LevelInfo || !strings.Contains(msg.Text, "Exported") {
		t.Errorf("export status = %#v", cmd())
	}
	dir, err := store.Dir()
	if err != nil {
		t.Fatalf("store.Dir: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "exports"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("exports = %v (%v)", entries, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "exports", entries[0].Name()))
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	var doc Export
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("export json: %v", err)
	}
	if doc.Kind != "burnin" || doc.Result.Verdict != verdictPassed || doc.LogPath == "" {
		t.Errorf("export document = %+v", doc)
	}
}

// TestHistoryLoadsOnInit checks recent runs are loaded when the tab starts.
func TestHistoryLoadsOnInit(t *testing.T) {
	t.Setenv("MEM_STATION_STATE_DIR", t.TempDir())
	payload, _ := json.Marshal(Payload{
		Options: payloadOptions{PresetName: "memory-heavy (embedded)", DurationSeconds: 600},
		Verdict: verdictFailed,
		Errors:  4,
	})
	if err := store.AppendHistory(store.Record{
		ID:        "seed",
		Kind:      runner.KindBurnIn,
		StartedAt: testStart,
		Payload:   payload,
	}); err != nil {
		t.Fatalf("AppendHistory: %v", err)
	}

	m := New("", testSnapshot())
	m.Init()
	if len(m.runs) != 1 {
		t.Fatalf("loaded runs = %d, want 1", len(m.runs))
	}
	out := m.View(118, 37)
	if !strings.Contains(out, "History") || !strings.Contains(out, verdictFailed) {
		t.Errorf("history table missing:\n%s", out)
	}
}

// TestViewIsExactSizeInEveryState checks the exact-size layout contract for all
// run states at the minimum and a wide terminal.
func TestViewIsExactSizeInEveryState(t *testing.T) {
	sizes := []struct{ w, h int }{{78, 21}, {118, 37}}
	for _, size := range sizes {
		for _, state := range []runner.State{runner.StateIdle, runner.StateRunning, runner.StateDone, runner.StateFailed, runner.StateCancelled} {
			m := stateModel(t, state)
			out := m.View(size.w, size.h)
			if got := lipgloss.Height(out); got != size.h {
				t.Errorf("%v %dx%d: height = %d, want %d", state, size.w, size.h, got, size.h)
			}
			if got := lipgloss.Width(out); got != size.w {
				t.Errorf("%v %dx%d: width = %d, want %d", state, size.w, size.h, got, size.w)
			}
		}
	}
}

// TestGoldenViews pins the rendered running/done/cancelled views.
func TestGoldenViews(t *testing.T) {
	cases := []struct {
		name  string
		state runner.State
		w, h  int
	}{
		{"running_78x21", runner.StateRunning, 78, 21},
		{"done_78x21", runner.StateDone, 78, 21},
		{"cancelled_78x21", runner.StateCancelled, 78, 21},
		{"done_118x37", runner.StateDone, 118, 37},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := stateModel(t, tc.state)
			checkGolden(t, tc.name+".golden", m.View(tc.w, tc.h))
		})
	}
}

// stateModel builds a tab in a fixed state with deterministic content.
func stateModel(t *testing.T, state runner.State) *Model {
	t.Helper()
	t.Setenv("MEM_STATION_STATE_DIR", t.TempDir())
	m := New("", testSnapshot())
	m.now = func() time.Time { return testStart.Add(time.Minute) }
	m.runOptions = Options{Duration: 30 * time.Minute, StopOnError: true}
	m.runPreset = m.presets[0]
	m.jobPath = "/var/lib/mem-station/jobs/burnin-memory.job"
	m.logPath = "/var/lib/mem-station/logs/burnin.log"

	switch state {
	case runner.StateIdle:
		return m
	case runner.StateRunning:
		m.state = runner.StateRunning
		m.startedAt = testStart
		m.output = "stress-ng: info: dispatching stressors\nstress-ng: info: vm: setting up"
		m.errorOutput = "stress-ng: error: [42] vm: failed to allocate"
		m.errors = 1
		m.firstError = "stress-ng: error: [42] vm: failed to allocate"
	case runner.StateDone:
		m.state = runner.StateDone
		m.startedAt = testStart
		m.finishedAt = testEnd
		m.output = "stress-ng: info: dispatching stressors\nstress-ng: info: successful run completed in 300.00 secs"
		m.results = parsedResults(0, 0, true)
	case runner.StateFailed:
		m.state = runner.StateFailed
		m.startedAt = testStart
		m.finishedAt = testEnd
		m.errors = 2
		m.firstError = "stress-ng: error: [42] vm: failed to allocate"
		m.errText = "2 error lines"
		m.output = "stress-ng: error: [42] vm: failed to allocate\nstress-ng: error: [43] memthrash: mismatch"
		m.results = parsedResults(0, 2, false)
	case runner.StateCancelled:
		m.state = runner.StateCancelled
		m.startedAt = testStart
		m.finishedAt = testStart.Add(time.Minute)
		m.errText = "cancelled"
		m.output = "stress-ng: info: dispatching stressors"
		m.results = parsedResults(-1, 0, false)
	}
	return m
}

// parsedResults builds a parse.Results summary for a fixed state.
func parsedResults(exitCode, errors int, success bool) *parse.Results {
	return &parse.Results{Duration: "5 mins", ExitCode: exitCode, Errors: errors, Success: success}
}

// ansiRe strips SGR sequences so goldens do not depend on the color profile.
var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	got = strings.TrimRight(ansiRe.ReplaceAllString(got, ""), "\n")
	path := filepath.Join("testdata", name)
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run with -update): %v", path, err)
	}
	if got != strings.TrimRight(string(want), "\n") {
		t.Errorf("golden %s mismatch:\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

// TestStopCancelsRunningJob checks the app's quit/cancel path reaches the
// runner process group.
func TestStopCancelsRunningJob(t *testing.T) {
	m := newTestModel(t)
	job := startRun(t, m)
	started(m, job.spec.LogPath)
	m.Stop()
	if !job.cancelled {
		t.Error("Stop did not cancel the running job")
	}
}
