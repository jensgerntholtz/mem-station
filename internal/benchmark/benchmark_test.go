package benchmark

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"mem-station/internal/bench"
	"mem-station/internal/config"
	"mem-station/internal/msgs"
	"mem-station/internal/runner"
	"mem-station/internal/store"
	"mem-station/internal/ui"
	"mem-station/internal/uitest"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	testStart = time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	testEnd   = testStart.Add(30 * time.Second)
)

// fakeJob is a runner.Handle whose events the test feeds by hand.
type fakeJob struct {
	spec      runner.Spec
	events    chan runner.Event
	cancelled bool
}

func (f *fakeJob) Events() <-chan runner.Event { return f.events }
func (f *fakeJob) Cancel()                     { f.cancelled = true }

// testSnapshot is an applied profile with 2 channels at 3200 MT/s, so the
// theoretical peak is 51200 MB/s.
func testSnapshot() config.Snapshot {
	return config.Snapshot{
		Values: map[string]string{
			config.KeyChannels: "2",
			config.KeyFreq:     "3200",
			config.KeyCL:       "16",
		},
		Source: "applied",
	}
}

// newTestModel returns a tab wired to a fake job factory, a pinned clock, and
// an isolated state directory.
func newTestModel(t *testing.T) *Model {
	t.Helper()
	t.Setenv("MEM_STATION_STATE_DIR", t.TempDir())
	m := New(testSnapshot())
	m.now = func() time.Time { return testStart.Add(10 * time.Second) }
	m.start = func(_ context.Context, spec runner.Spec) runner.Handle {
		return &fakeJob{spec: spec, events: make(chan runner.Event, 64)}
	}
	return m
}

// startRun starts a run through the fake factory and returns the fake handle.
func startRun(t *testing.T, m *Model) *fakeJob {
	t.Helper()
	m.run()
	job, ok := m.job.(*fakeJob)
	if !ok {
		t.Fatalf("run did not start a fake job: %T", m.job)
	}
	return job
}

// event delivers one runner Event the way the app does.
func event(m *Model, ev runner.Event) {
	ev.Kind = runner.KindBenchmark
	m.Update(msgs.JobEvent{Ev: ev})
}

// started and finished are the two lifecycle events of a run.
func started(m *Model, logPath string) {
	event(m, runner.Event{Type: runner.EventStarted, At: testStart, LogPath: logPath})
}

func finished(m *Model, exitCode int, err error, logPath string) {
	event(m, runner.Event{Type: runner.EventFinished, At: testEnd, ExitCode: exitCode, Err: err, LogPath: logPath})
}

// output feeds one output line.
func output(m *Model, line string) {
	event(m, runner.Event{Type: runner.EventOutput, Line: line, At: testStart.Add(time.Second)})
}

// TestOptionsArgs checks the argv slice is built without a shell string.
func TestOptionsArgs(t *testing.T) {
	opts := Options{Duration: 45 * time.Second, Buffer: "512M", Workers: 3}
	want := []string{"--memrate", "3", "--memrate-bytes", "512M", "--metrics", "-t", "45"}
	if got := opts.Args(); !reflect.DeepEqual(got, want) {
		t.Errorf("Args() = %v, want %v", got, want)
	}
}

// TestOptionsValidate rejects values that would build a bogus invocation.
func TestOptionsValidate(t *testing.T) {
	cases := []struct {
		name string
		opts Options
		ok   bool
	}{
		{"default", DefaultOptions(), true},
		{"custom duration", Options{Duration: 90 * time.Second, Buffer: "1G", Workers: 1}, true},
		{"too short", Options{Duration: 0, Buffer: "256M", Workers: 1}, false},
		{"too long", Options{Duration: 3 * time.Hour, Buffer: "256M", Workers: 1}, false},
		{"bad buffer", Options{Duration: time.Minute, Buffer: "64T", Workers: 1}, false},
		{"no workers", Options{Duration: time.Minute, Buffer: "256M", Workers: 0}, false},
		{"too many workers", Options{Duration: time.Minute, Buffer: "256M", Workers: maxWorkers() + 1}, false},
	}
	for _, tc := range cases {
		err := tc.opts.Validate()
		if tc.ok && err != nil {
			t.Errorf("%s: unexpected error %v", tc.name, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("%s: expected an error", tc.name)
		}
	}
}

// TestOptionsLabelAndSignature checks history labels never mix configurations.
func TestOptionsLabelAndSignature(t *testing.T) {
	opts := Options{Duration: 30 * time.Second, Buffer: "256M", Workers: 2}
	if got, want := opts.Label(), "30s · 256M · 2 workers"; got != want {
		t.Errorf("Label() = %q, want %q", got, want)
	}
	if got, want := opts.Signature(), "30s/256M/2"; got != want {
		t.Errorf("Signature() = %q, want %q", got, want)
	}
}

// TestConfigCycling checks the selector clamps and opens the custom editor.
func TestConfigCycling(t *testing.T) {
	m := newTestModel(t)

	m.row = rowBuffer
	for i := 0; i < len(buffers)+2; i++ {
		m.cycle(1)
	}
	if got := buffers[m.bufferIndex]; got != "1G" {
		t.Errorf("buffer after cycling past the end = %q, want 1G", got)
	}
	for i := 0; i < len(buffers)+2; i++ {
		m.cycle(-1)
	}
	if got := buffers[m.bufferIndex]; got != "128M" {
		t.Errorf("buffer after cycling before the start = %q, want 128M", got)
	}

	m.row = rowWorkers
	m.cycle(-1)
	if m.workers != 1 {
		t.Errorf("workers below 1: got %d", m.workers)
	}

	m.row = rowDuration
	for i := 0; i < customDurationIndex; i++ {
		m.cycle(1)
	}
	if m.durationIndex != customDurationIndex || !m.Editing() {
		t.Fatalf("custom duration option did not open the input (index %d, editing %v)", m.durationIndex, m.Editing())
	}
	m.input.SetValue("90s")
	if cmd := m.handleEditKey(uitest.Code(tea.KeyEnter)); cmd != nil {
		t.Fatal("a valid custom duration produced an error message")
	}
	if m.Editing() {
		t.Error("committing the custom duration left the input open")
	}
	if got := m.options().Duration; got != 90*time.Second {
		t.Errorf("custom duration = %s, want 1m30s", got)
	}
}

// TestCustomDurationEscReverts checks Esc restores the previous option.
func TestCustomDurationEscReverts(t *testing.T) {
	m := newTestModel(t)
	m.row = rowDuration
	for m.durationIndex != customDurationIndex-1 {
		m.cycle(1)
	}
	prev := m.durationIndex
	m.cycle(1) // the custom entry opens the input
	if !m.Editing() {
		t.Fatal("expected the custom input to be open")
	}
	m.handleEditKey(uitest.Code(tea.KeyEsc))
	if m.Editing() {
		t.Error("esc did not close the custom input")
	}
	if m.durationIndex != prev {
		t.Errorf("duration index = %d, want the previous %d", m.durationIndex, prev)
	}
}

// TestInvalidCustomDurationIsRejected checks a bad typed value keeps the input
// open and reports an error instead of running with a bogus duration.
func TestInvalidCustomDurationIsRejected(t *testing.T) {
	m := newTestModel(t)
	m.row = rowDuration
	for m.durationIndex != customDurationIndex {
		m.cycle(1)
	}
	m.input.SetValue("nonsense")
	cmd := m.handleEditKey(uitest.Code(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("invalid duration produced no status message")
	}
	if !m.Editing() {
		t.Error("invalid duration closed the input")
	}
}

// TestRunBuildsSpecAndPersistsHistory drives a full successful run.
func TestRunBuildsSpecAndPersistsHistory(t *testing.T) {
	m := newTestModel(t)
	m.row = rowDuration
	m.cycle(1) // 1m
	m.row = rowBuffer
	m.cycle(1) // 512M
	m.row = rowWorkers
	m.cycle(1)
	m.cycle(1) // 3 workers

	job := startRun(t, m)
	if m.State() != runner.StateRunning {
		t.Errorf("state after run = %v, want running", m.State())
	}
	if got, want := job.spec.Name, "stress-ng"; got != want {
		t.Errorf("spec name = %q, want %q", got, want)
	}
	wantArgs := []string{"--memrate", "3", "--memrate-bytes", "512M", "--metrics", "-t", "60"}
	if !reflect.DeepEqual(job.spec.Args, wantArgs) {
		t.Errorf("spec args = %v, want %v", job.spec.Args, wantArgs)
	}

	started(m, job.spec.LogPath)
	output(m, "stress-ng: info: running 3 memrate workers")
	output(m, "memrate: read rate 21400.00 MB per sec")
	output(m, "memrate: write rate 18100.00 MB per sec")
	output(m, "memrate: read64 20000.00 MB per sec")
	finished(m, 0, nil, job.spec.LogPath)

	if m.State() != runner.StateDone {
		t.Fatalf("state = %v (%s), want done", m.State(), m.errText)
	}
	if m.results == nil || m.results.ReadRate != 21400 || m.results.WriteRate != 18100 {
		t.Fatalf("results not parsed: %+v", m.results)
	}

	recs, err := store.RecentHistory(runner.KindBenchmark, 10)
	if err != nil {
		t.Fatalf("RecentHistory: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("history records = %d, want 1", len(recs))
	}
	rec := recs[0]
	if rec.LogPath != job.spec.LogPath {
		t.Errorf("record log path = %q, want %q", rec.LogPath, job.spec.LogPath)
	}
	if rec.DurationMS != 30000 {
		t.Errorf("record duration = %d ms, want 30000", rec.DurationMS)
	}
	if got := rec.Config.Values[config.KeyFreq]; got != "3200" {
		t.Errorf("record snapshot freq = %q, want 3200", got)
	}
	var payload Payload
	if err := json.Unmarshal(rec.Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if !payload.Parsed || payload.ReadMBs != 21400 {
		t.Errorf("payload = %+v", payload)
	}
	if payload.Options.Buffer != "512M" || payload.Options.Workers != 3 || payload.Options.DurationSeconds != 60 {
		t.Errorf("payload options = %+v", payload.Options)
	}
	if len(m.runs) != 1 || m.runs[0].Record.ID == "" {
		t.Errorf("in-memory history not updated: %+v", m.runs)
	}
}

// TestFailurePaths covers a missing binary, a non-zero exit, and output the
// parser cannot use.
func TestFailurePaths(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		exitCode   int
		output     string
		wantInText string
	}{
		{"missing binary", errors.New(`exec: "stress-ng": executable file not found in $PATH`), -1, "", "not found"},
		{"non-zero exit", errors.New("stress-ng exited with status 2"), 2, "bogus output", "status 2"},
		{"unparseable", nil, 0, "no rates here", "could not parse"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(t)
			job := startRun(t, m)
			started(m, job.spec.LogPath)
			if tc.output != "" {
				output(m, tc.output)
			}
			finished(m, tc.exitCode, tc.err, job.spec.LogPath)

			if m.State() != runner.StateFailed {
				t.Fatalf("state = %v, want failed", m.State())
			}
			if !strings.Contains(m.errText, tc.wantInText) {
				t.Errorf("error text = %q, want it to contain %q", m.errText, tc.wantInText)
			}
			out := m.View(78, 21)
			if !strings.Contains(out, "failed") {
				t.Errorf("view does not report the failure:\n%s", out)
			}
			recs, err := store.RecentHistory(runner.KindBenchmark, 10)
			if err != nil {
				t.Fatalf("RecentHistory: %v", err)
			}
			if len(recs) != 1 || recs[0].Error == "" {
				t.Errorf("failed run not recorded with its error: %+v", recs)
			}
		})
	}
}

// TestCancelKeepsConsuming checks x cancels the handle and the terminal state
// still comes from the runner's Finished event.
func TestCancelKeepsConsuming(t *testing.T) {
	m := newTestModel(t)
	job := startRun(t, m)
	started(m, job.spec.LogPath)

	m.handleKey(uitest.Key("x"))
	if !job.cancelled {
		t.Fatal("x did not cancel the job")
	}
	if m.State() != runner.StateRunning {
		t.Errorf("state right after cancel = %v, want running until Finished", m.State())
	}

	finished(m, -1, runner.ErrCancelled, job.spec.LogPath)
	if m.State() != runner.StateCancelled {
		t.Errorf("state = %v, want cancelled", m.State())
	}
	recs, err := store.RecentHistory(runner.KindBenchmark, 10)
	if err != nil {
		t.Fatalf("RecentHistory: %v", err)
	}
	if len(recs) != 1 || recs[0].Error != "cancelled" {
		t.Errorf("cancelled run record = %+v", recs)
	}
}

// TestConfigIsLockedWhileRunning checks settings cannot change mid-run.
func TestConfigIsLockedWhileRunning(t *testing.T) {
	m := newTestModel(t)
	job := startRun(t, m)
	started(m, job.spec.LogPath)

	before := m.options()
	m.row = rowBuffer
	m.cycle(1)
	m.row = rowDuration
	m.handleKey(uitest.Code(tea.KeyLeft))
	if got := m.options(); got != before {
		t.Errorf("configuration changed while running: %+v -> %+v", before, got)
	}
	if m.Editing() {
		t.Error("duration editing opened while a run was active")
	}
}

// TestFollowToggle checks follow drives the viewport to the bottom and scrolling
// up turns it off.
func TestFollowToggle(t *testing.T) {
	m := newTestModel(t)
	job := startRun(t, m)
	started(m, job.spec.LogPath)
	m.vp.SetWidth(60)
	m.vp.SetHeight(5)
	for i := 0; i < 20; i++ {
		output(m, "line")
	}
	if !m.follow || !m.vp.AtBottom() {
		t.Fatal("follow did not keep the viewport at the bottom")
	}
	m.handleKey(uitest.Code(tea.KeyPgUp))
	if m.follow {
		t.Error("pgup left follow enabled")
	}
	m.handleKey(uitest.Key("f"))
	if !m.follow || !m.vp.AtBottom() {
		t.Error("f did not restore follow")
	}
}

// TestPreviousRunDelta checks deltas only compare runs with the same config.
func TestPreviousRunDelta(t *testing.T) {
	m := newTestModel(t)
	m.runs = []runRecord{
		{Record: store.Record{ID: "new"}, Result: Payload{Options: Options{Duration: 30 * time.Second, Buffer: "256M", Workers: 1}.payload(), Parsed: true, ReadMBs: 21000}},
		{Record: store.Record{ID: "other"}, Result: Payload{Options: Options{Duration: 30 * time.Second, Buffer: "512M", Workers: 1}.payload(), Parsed: true, ReadMBs: 9000}},
		{Record: store.Record{ID: "same"}, Result: Payload{Options: Options{Duration: 30 * time.Second, Buffer: "256M", Workers: 1}.payload(), Parsed: true, ReadMBs: 20000}},
	}
	prev, ok := m.previous(Options{Duration: 30 * time.Second, Buffer: "256M", Workers: 1}.Signature(), "new")
	if !ok {
		t.Fatal("previous run not found")
	}
	if prev.ReadMBs != 20000 {
		t.Errorf("previous read = %v, want the same-config 20000", prev.ReadMBs)
	}
}

// TestExportWritesJSON checks e writes the last run to the state dir.
func TestExportWritesJSON(t *testing.T) {
	m := newTestModel(t)
	job := startRun(t, m)
	started(m, job.spec.LogPath)
	output(m, "memrate: read rate 21400.00 MB per sec")
	output(m, "memrate: write rate 18100.00 MB per sec")
	finished(m, 0, nil, job.spec.LogPath)

	cmd := m.handleKey(uitest.Key("e"))
	if cmd == nil {
		t.Fatal("export produced no status message")
	}
	msg, ok := cmd().(ui.StatusMsg)
	if !ok {
		t.Fatalf("export message = %T", cmd())
	}
	if msg.Level != ui.LevelInfo || !strings.Contains(msg.Text, "Exported") {
		t.Errorf("export status = %+v", msg)
	}

	dir, err := store.Dir()
	if err != nil {
		t.Fatalf("store.Dir: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "exports"))
	if err != nil {
		t.Fatalf("exports dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("exported files = %d, want 1", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(dir, "exports", entries[0].Name()))
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	var doc Export
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("export json: %v", err)
	}
	if doc.Kind != "benchmark" || doc.Result.ReadMBs != 21400 || doc.LogPath == "" {
		t.Errorf("export document = %+v", doc)
	}
}

// TestNoExportBeforeRun checks e is harmless without history.
func TestNoExportBeforeRun(t *testing.T) {
	m := newTestModel(t)
	cmd := m.handleKey(uitest.Key("e"))
	if cmd == nil {
		t.Fatal("expected a status message")
	}
	msg, ok := cmd().(ui.StatusMsg)
	if !ok || !strings.Contains(msg.Text, "No benchmark run") {
		t.Errorf("status = %#v", cmd())
	}
}

// TestHistoryLoadsOnInit checks recent runs are loaded when the tab starts.
func TestHistoryLoadsOnInit(t *testing.T) {
	t.Setenv("MEM_STATION_STATE_DIR", t.TempDir())
	payload, _ := json.Marshal(Payload{
		Options:  Options{Duration: time.Minute, Buffer: "1G", Workers: 2}.payload(),
		Parsed:   true,
		ReadMBs:  21000,
		WriteMBs: 18000,
	})
	if err := store.AppendHistory(store.Record{
		ID:        "seed",
		Kind:      runner.KindBenchmark,
		StartedAt: testStart,
		LogPath:   "/tmp/log",
		Payload:   payload,
	}); err != nil {
		t.Fatalf("AppendHistory: %v", err)
	}

	m := New(testSnapshot())
	m.Init()
	if len(m.runs) != 1 {
		t.Fatalf("loaded runs = %d, want 1", len(m.runs))
	}
	out := m.View(118, 37)
	if !strings.Contains(out, "History") || !strings.Contains(out, "1G") {
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
			uitest.GoldenView(t, "testdata", tc.name+".golden", tc.w, tc.h, m.View(tc.w, tc.h))
		})
	}
}

// stateModel builds a tab in a fixed state with deterministic content, so it can
// be rendered into a golden file or an exact-size check.
func stateModel(t *testing.T, state runner.State) *Model {
	t.Helper()
	t.Setenv("MEM_STATION_STATE_DIR", t.TempDir())
	m := New(testSnapshot())
	m.now = func() time.Time { return testStart.Add(10 * time.Second) }
	m.durationIndex = 1
	m.bufferIndex = 1
	m.workers = 1
	m.logPath = "/var/lib/mem-station/logs/benchmark.log"
	m.runOptions = Options{Duration: 30 * time.Second, Buffer: "256M", Workers: 1}

	switch state {
	case runner.StateIdle:
		return m
	case runner.StateRunning:
		m.state = runner.StateRunning
		m.startedAt = testStart
		m.results = nil
	case runner.StateDone:
		m.state = runner.StateDone
		m.startedAt = testStart
		m.finishedAt = testEnd
		m.output = "stress-ng: info: dispatching memrate workers\nmemrate: read rate 21400.00 MB per sec\nmemrate: write rate 18100.00 MB per sec"
		m.results = &bench.BenchResults{
			ReadRate:  21400,
			WriteRate: 18100,
			Metrics: []bench.BenchMetric{
				{Label: "read64", Value: 20000},
				{Label: "write64", Value: 17000},
			},
			Duration: "30.02 secs",
		}
	case runner.StateFailed:
		m.state = runner.StateFailed
		m.startedAt = testStart
		m.finishedAt = testEnd
		m.exitCode = 2
		m.errText = "stress-ng exited with status 2 (log: /var/lib/mem-station/logs/benchmark.log)"
	case runner.StateCancelled:
		m.state = runner.StateCancelled
		m.startedAt = testStart
		m.finishedAt = testStart.Add(5 * time.Second)
		m.errText = "cancelled"
		m.output = "stress-ng: info: dispatching memrate workers"
	}
	return m
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
