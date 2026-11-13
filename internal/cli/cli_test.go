package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"mem-station/internal/app"
	"mem-station/internal/benchmark"
	"mem-station/internal/burnin"
	"mem-station/internal/config"
	"mem-station/internal/memory"
	"mem-station/internal/report"
	"mem-station/internal/runner"
	"mem-station/internal/store"
)

// fakeHandle replays a scripted event stream and records cancellation.
type fakeHandle struct {
	events   chan runner.Event
	mu       sync.Mutex
	cancel   bool
	onCancel func()
}

func (f *fakeHandle) Events() <-chan runner.Event { return f.events }

func (f *fakeHandle) Cancel() {
	f.mu.Lock()
	f.cancel = true
	hook := f.onCancel
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
}

func (f *fakeHandle) cancelled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cancel
}

// fakeRunner is a runner.StartFunc that never launches a process: it records the
// spec and replays scripted events.
type fakeRunner struct {
	script []runner.Event
	specs  []runner.Spec
	jobs   []*fakeHandle

	mu sync.Mutex
	// cancelFinish makes the job emit a cancelled EventFinished from Cancel.
	cancelFinish bool
	started      chan runner.Spec
}

func (f *fakeRunner) start(_ context.Context, spec runner.Spec) runner.Handle {
	f.mu.Lock()
	f.specs = append(f.specs, spec)
	f.mu.Unlock()

	job := &fakeHandle{events: make(chan runner.Event, len(f.script)+4)}
	if f.cancelFinish {
		var once sync.Once
		job.onCancel = func() {
			once.Do(func() {
				job.events <- runner.Event{
					Kind:     spec.Kind,
					Type:     runner.EventFinished,
					At:       testStart.Add(time.Second),
					ExitCode: -1,
					Err:      runner.ErrCancelled,
					LogPath:  spec.LogPath,
				}
				close(job.events)
			})
		}
	}
	for _, ev := range f.script {
		ev.Kind = spec.Kind
		if ev.LogPath == "" {
			ev.LogPath = spec.LogPath
		}
		job.events <- ev
	}
	if !f.cancelFinish {
		close(job.events)
	}

	f.mu.Lock()
	f.jobs = append(f.jobs, job)
	f.mu.Unlock()
	if f.started != nil {
		f.started <- spec
	}
	return job
}

var testStart = time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)

// startedEvent and finishedEvent are the lifecycle events of a scripted run.
func startedEvent() runner.Event {
	return runner.Event{Type: runner.EventStarted, At: testStart}
}

func outputEvent(line string) runner.Event {
	return runner.Event{Type: runner.EventOutput, Line: line, At: testStart.Add(time.Second)}
}

func finishedEvent(exitCode int, err error) runner.Event {
	return runner.Event{Type: runner.EventFinished, At: testStart.Add(15 * time.Second), ExitCode: exitCode, Err: err}
}

// containsFold reports whether s contains substr, ignoring case. fang
// capitalizes the first word of an error message, so assertions about the
// wording are case-insensitive.
func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// testDeps builds CLI dependencies with captured streams and an isolated state
// directory.
func testDeps(t *testing.T, jobs *fakeRunner) (Deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	t.Setenv("MEM_STATION_STATE_DIR", t.TempDir())
	t.Setenv("MEM_STATION_JOB", "")

	var stdout, stderr bytes.Buffer
	deps := Deps{
		Stdout: &stdout,
		Stderr: &stderr,
		Detect: func() (map[string]string, string, *memory.SPDInfo) {
			return map[string]string{
					"Channels":     "2",
					"DRAM Freq":    "3200",
					"DRAM Voltage": "1.35V",
					"Extra Field":  "keep",
				}, "Detected 4 params from hardware.",
				&memory.SPDInfo{PartNumber: "F4-3200C16-16GVK", SizeMB: "16384"}
		},
		ReadIMC: func() ([]memory.IMCTimings, error) {
			return []memory.IMCTimings{{Channel: 0, TRAS: 38, TRP: 18, TRCD: 18, TRFC: 350, TREFI: 7800}}, nil
		},
		Start:   jobs.start,
		RunTUI:  func(app.Options) error { t.Fatal("the TUI was launched during a headless test"); return nil },
		Version: "9.9.9-test",
	}
	return deps, &stdout, &stderr
}

// TestRunDispatchesCommands checks the subcommand routing and usage errors.
func TestRunDispatchesCommands(t *testing.T) {
	jobs := &fakeRunner{}
	deps, stdout, stderr := testDeps(t, jobs)

	if code := Run([]string{"help"}, deps); code != ExitOK {
		t.Errorf("help exit = %d, want %d", code, ExitOK)
	}
	if !containsFold(stdout.String(), "usage") {
		t.Errorf("help output missing usage:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"nonsense"}, deps); code != ExitUsage {
		t.Errorf("unknown command exit = %d, want %d", code, ExitUsage)
	}
	if !containsFold(stderr.String(), "unknown command") {
		t.Errorf("stderr = %q, want an unknown-command error", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want nothing on stdout for a usage error", stdout.String())
	}
}

// TestRunTUIOptions checks the TUI flags reach app.Options unchanged.
func TestRunTUIOptions(t *testing.T) {
	t.Setenv("MEM_STATION_STATE_DIR", t.TempDir())
	jobs := &fakeRunner{}
	deps, stdout, _ := testDeps(t, jobs)

	var got app.Options
	launched := false
	deps.RunTUI = func(opts app.Options) error {
		launched = true
		got = opts
		return nil
	}

	code := Run([]string{
		"tui", "--tab=burn-in", "--dark=false", "--no-color", "--mouse", "--imc",
		"--state-dir", "/tmp/state", "--job", "/tmp/job", "--version=false",
	}, deps)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	if !launched {
		t.Fatal("the TUI was not launched")
	}
	if got.InitialTab != 2 {
		t.Errorf("InitialTab = %d, want the burn-in tab", got.InitialTab)
	}
	if got.Dark {
		t.Error("Dark = true, want the light theme")
	}
	if !got.Mouse {
		t.Error("Mouse = false, want mouse enabled")
	}
	if !got.IMC {
		t.Error("IMC = false, want the --imc opt-in")
	}
	if got.StateDir != "/tmp/state" || got.JobPath != "/tmp/job" {
		t.Errorf("options = %+v", got)
	}
	if got.Version != "9.9.9-test" {
		t.Errorf("Version = %q, want the injected version", got.Version)
	}
	if os.Getenv("NO_COLOR") == "" {
		t.Error("--no-color did not set NO_COLOR")
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want nothing when launching the TUI", stdout.String())
	}
}

// TestRunTUIVersionAndBadTab covers --version and an invalid --tab value.
func TestRunTUIVersionAndBadTab(t *testing.T) {
	jobs := &fakeRunner{}
	deps, stdout, stderr := testDeps(t, jobs)
	deps.RunTUI = func(app.Options) error { t.Fatal("the TUI was launched for --version"); return nil }

	if code := Run([]string{"--version"}, deps); code != ExitOK {
		t.Errorf("--version exit = %d, want %d", code, ExitOK)
	}
	if got := strings.TrimSpace(stdout.String()); got != "9.9.9-test" {
		t.Errorf("--version printed %q", got)
	}

	stdout.Reset()
	if code := Run([]string{"tui", "--tab=nope"}, deps); code != ExitUsage {
		t.Errorf("bad --tab exit = %d, want %d", code, ExitUsage)
	}
	if !containsFold(stderr.String(), "unknown tab") {
		t.Errorf("stderr = %q, want an unknown-tab error", stderr.String())
	}
}

// TestInfoJSON checks the info document is one JSON object on stdout.
func TestInfoJSON(t *testing.T) {
	jobs := &fakeRunner{}
	deps, stdout, stderr := testDeps(t, jobs)

	if code := Run([]string{"info", "-json", "-imc"}, deps); code != ExitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, ExitOK, stderr.String())
	}
	var doc infoDocument
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("info output is not one JSON document: %v\n%s", err, stdout.String())
	}
	if doc.Values["DRAM Freq"] != "3200" || doc.Message == "" {
		t.Errorf("document = %+v", doc)
	}
	if doc.SPD == nil || doc.SPD.PartNumber != "F4-3200C16-16GVK" {
		t.Errorf("SPD = %+v", doc.SPD)
	}
	if len(doc.IMC) != 1 || doc.IMC[0].TRFC != 350 {
		t.Errorf("IMC = %+v", doc.IMC)
	}
}

// TestInfoHuman prints every detected value, including unknown future keys.
func TestInfoHuman(t *testing.T) {
	jobs := &fakeRunner{}
	deps, stdout, _ := testDeps(t, jobs)

	if code := Run([]string{"info", "-imc"}, deps); code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	out := stdout.String()
	for _, want := range []string{"Hardware", "DRAM Freq", "Extra Field", "F4-3200C16-16GVK", "channel 0"} {
		if !strings.Contains(out, want) {
			t.Errorf("info output missing %q:\n%s", want, out)
		}
	}
}

// TestInfoDoesNotReadIMCByDefault checks the safe default: raw /dev/mem access
// must be an explicit opt-in.
func TestInfoDoesNotReadIMCByDefault(t *testing.T) {
	jobs := &fakeRunner{}
	deps, stdout, _ := testDeps(t, jobs)
	deps.ReadIMC = func() ([]memory.IMCTimings, error) {
		t.Fatal("info read IMC without -imc")
		return nil, nil
	}

	if code := Run([]string{"info", "-json"}, deps); code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	var doc infoDocument
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("info output is not one JSON document: %v", err)
	}
	if len(doc.IMC) != 0 {
		t.Errorf("IMC = %+v, want no raw read by default", doc.IMC)
	}
	if !strings.Contains(doc.IMCError, "-imc") {
		t.Errorf("IMCError = %q, want the opt-in hint", doc.IMCError)
	}
}

// TestBenchJSONSuccess runs a scripted benchmark and checks JSON, argv, history,
// and exit code.
func TestBenchJSONSuccess(t *testing.T) {
	jobs := &fakeRunner{script: []runner.Event{
		startedEvent(),
		outputEvent("memrate: read rate 21400.00 MB per sec"),
		outputEvent("memrate: write rate 18100.00 MB per sec"),
		outputEvent("memrate: read64 20000.00 MB per sec"),
		outputEvent("stress-ng: info: completed in 15.02 secs"),
		finishedEvent(0, nil),
	}}
	deps, stdout, stderr := testDeps(t, jobs)

	code := Run([]string{"bench", "-duration", "15s", "-buffer", "256M", "-workers", "2", "-json"}, deps)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, ExitOK, stderr.String())
	}

	var doc benchmark.Export
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("bench output is not one JSON document: %v\n%s", err, stdout.String())
	}
	if doc.Result.ReadMBs != 21400 || doc.Result.WriteMBs != 18100 {
		t.Errorf("result = %+v", doc.Result)
	}
	if doc.Kind != string(runner.KindBenchmark) || doc.LogPath == "" {
		t.Errorf("document = %+v", doc)
	}
	if !doc.Result.Parsed {
		t.Error("parsed = false for a successful parse")
	}

	if len(jobs.specs) != 1 {
		t.Fatalf("jobs launched = %d, want 1", len(jobs.specs))
	}
	spec := jobs.specs[0]
	if spec.Name != "stress-ng" {
		t.Errorf("job name = %q, want stress-ng", spec.Name)
	}
	want := []string{"--memrate", "2", "--memrate-bytes", "256M", "--metrics", "-t", "15"}
	if strings.Join(spec.Args, " ") != strings.Join(want, " ") {
		t.Errorf("args = %v, want %v", spec.Args, want)
	}

	recs, err := store.RecentHistory(runner.KindBenchmark, 5)
	if err != nil {
		t.Fatalf("RecentHistory: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("history records = %d, want 1", len(recs))
	}
	if recs[0].DurationMS != 15000 {
		t.Errorf("recorded duration = %d ms, want 15000", recs[0].DurationMS)
	}
	if recs[0].Config.Source != "none" {
		t.Errorf("recorded config source = %q, want none without an applied profile", recs[0].Config.Source)
	}
}

// TestBenchExportToPath checks -out writes the same JSON document to a chosen
// destination without polluting stdout.
func TestBenchExportToPath(t *testing.T) {
	jobs := &fakeRunner{script: []runner.Event{
		startedEvent(),
		outputEvent("memrate: read rate 21400.00 MB per sec"),
		finishedEvent(0, nil),
	}}
	deps, stdout, stderr := testDeps(t, jobs)
	path := filepath.Join(t.TempDir(), "nested", "bench.json")

	if code := Run([]string{"bench", "-out", path}, deps); code != ExitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, ExitOK, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Saved") {
		t.Errorf("human output does not mention the saved file:\n%s", stdout.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	var doc benchmark.Export
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("export is not valid JSON: %v", err)
	}
	if doc.Result.ReadMBs != 21400 {
		t.Errorf("exported read = %v, want 21400", doc.Result.ReadMBs)
	}
	if stdout.Len() == 0 || strings.Contains(stdout.String(), "\"kind\"") {
		t.Errorf("stdout = %q, want human output only without -json", stdout.String())
	}
}

// TestBenchFailureExit checks a non-zero stress-ng exit is reported and fails.
func TestBenchFailureExit(t *testing.T) {
	jobs := &fakeRunner{script: []runner.Event{
		startedEvent(),
		outputEvent("stress-ng: error: memrate: out of memory"),
		finishedEvent(2, errors.New("stress-ng exited with status 2")),
	}}
	deps, stdout, _ := testDeps(t, jobs)

	if code := Run([]string{"bench", "-json"}, deps); code != ExitFailed {
		t.Fatalf("exit = %d, want %d", code, ExitFailed)
	}
	var doc benchmark.Export
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("bench output is not one JSON document: %v", err)
	}
	if doc.ExitCode != 2 || doc.Error == "" {
		t.Errorf("document = %+v, want exit 2 and an error", doc)
	}
}

// TestBenchCancelExit checks a cancelled run exits 130 and is labelled cancelled.
func TestBenchCancelExit(t *testing.T) {
	jobs := &fakeRunner{script: []runner.Event{
		startedEvent(),
		finishedEvent(-1, runner.ErrCancelled),
	}}
	deps, stdout, _ := testDeps(t, jobs)
	deps.Ctx = context.Background()

	if code := Run([]string{"bench", "-json"}, deps); code != ExitSigint {
		t.Fatalf("exit = %d, want %d", code, ExitSigint)
	}
	var doc benchmark.Export
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("bench output is not one JSON document: %v", err)
	}
	if doc.Error != "cancelled" {
		t.Errorf("error = %q, want cancelled", doc.Error)
	}
}

// TestBenchContextCancelCancelsJob checks an interrupt reaches Cancel and the
// command still reports the runner's terminal event.
func TestBenchContextCancelCancelsJob(t *testing.T) {
	jobs := &fakeRunner{script: []runner.Event{startedEvent()}, cancelFinish: true, started: make(chan runner.Spec, 1)}
	deps, _, _ := testDeps(t, jobs)

	ctx, cancel := context.WithCancel(context.Background())
	deps.Ctx = ctx

	done := make(chan int, 1)
	go func() { done <- Run([]string{"bench", "-json"}, deps) }()

	<-jobs.started // the run is live
	cancel()

	select {
	case code := <-done:
		if code != ExitSigint {
			t.Errorf("exit = %d, want %d", code, ExitSigint)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the cancelled command did not finish")
	}

	if len(jobs.jobs) != 1 || !jobs.jobs[0].cancelled() {
		t.Error("cancelling the context did not cancel the job")
	}
}

// TestBurninPresetAndMaterialize checks the burn-in command materializes the job
// in the state dir with the requested timeout and reports a verdict.
func TestBurninPresetAndMaterialize(t *testing.T) {
	jobs := &fakeRunner{script: []runner.Event{
		startedEvent(),
		outputEvent("stress-ng: info: successful run completed in 300.00 secs"),
		finishedEvent(0, nil),
	}}
	deps, stdout, stderr := testDeps(t, jobs)

	if code := Run([]string{"burnin", "-duration", "5m", "-json"}, deps); code != ExitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, ExitOK, stderr.String())
	}
	var doc burnin.Export
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("burnin output is not one JSON document: %v\n%s", err, stdout.String())
	}
	if doc.Result.Verdict != burnin.VerdictPassed || !doc.Result.Success {
		t.Errorf("result = %+v, want passed", doc.Result)
	}
	if doc.Result.Options.DurationSeconds != 300 {
		t.Errorf("recorded duration = %d s, want 300", doc.Result.Options.DurationSeconds)
	}

	if len(jobs.specs) != 1 || len(jobs.specs[0].Args) != 2 || jobs.specs[0].Args[0] != "--job" {
		t.Fatalf("job specs = %+v", jobs.specs)
	}
	jobFile := jobs.specs[0].Args[1]
	data, err := os.ReadFile(jobFile)
	if err != nil {
		t.Fatalf("read materialized job: %v", err)
	}
	if !strings.Contains(string(data), "timeout 5m") {
		t.Errorf("materialized job has no 5m timeout:\n%s", data)
	}
	if filepath.Dir(jobFile) == "." {
		t.Errorf("job path %q is relative, want inside the state dir", jobFile)
	}
}

// TestBurninFailureVerdict checks error lines fail the run even with exit 0.
func TestBurninFailureVerdict(t *testing.T) {
	jobs := &fakeRunner{script: []runner.Event{
		startedEvent(),
		outputEvent("stress-ng: error: [42] vm: failed to allocate"),
		finishedEvent(0, nil),
	}}
	deps, stdout, _ := testDeps(t, jobs)

	if code := Run([]string{"burnin", "-json"}, deps); code != ExitFailed {
		t.Fatalf("exit = %d, want %d", code, ExitFailed)
	}
	var doc burnin.Export
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("burnin output is not one JSON document: %v", err)
	}
	if doc.Result.Verdict != burnin.VerdictFailed || doc.Result.Errors != 1 {
		t.Errorf("result = %+v, want failed with one error line", doc.Result)
	}
}

// TestBurninUnknownPreset is a usage error, not a run failure.
func TestBurninUnknownPreset(t *testing.T) {
	jobs := &fakeRunner{}
	deps, stdout, stderr := testDeps(t, jobs)

	if code := Run([]string{"burnin", "-preset", "nope"}, deps); code != ExitUsage {
		t.Fatalf("exit = %d, want %d", code, ExitUsage)
	}
	if !containsFold(stderr.String(), "unknown preset") {
		t.Errorf("stderr = %q, want an unknown-preset error", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want nothing", stdout.String())
	}
	if len(jobs.specs) != 0 {
		t.Error("a job was launched for an unknown preset")
	}
}

// TestNormalizeArgs checks the documented single-dash flags are rewritten for
// pflag without touching shorthands, unknown flags, or a bare "--" tail.
func TestNormalizeArgs(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{[]string{"bench", "-json"}, []string{"bench", "--json"}},
		{[]string{"-duration", "15s"}, []string{"--duration", "15s"}},
		{[]string{"-json=false"}, []string{"--json=false"}},
		{[]string{"-out=/tmp/x"}, []string{"--out=/tmp/x"}},
		{[]string{"--json"}, []string{"--json"}},
		{[]string{"-v"}, []string{"-v"}},
		{[]string{"-q"}, []string{"-q"}},
		{[]string{"-h"}, []string{"-h"}},
		{[]string{"-unknown"}, []string{"-unknown"}},
		{[]string{"info", "--", "-json"}, []string{"info", "--", "-json"}},
	}
	for _, tc := range cases {
		if got := normalizeArgs(tc.in); strings.Join(got, " ") != strings.Join(tc.want, " ") {
			t.Errorf("normalizeArgs(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestDoubleDashFlagsMatchSingleDash checks the canonical pflag form produces
// the same document as the documented single-dash form.
func TestDoubleDashFlagsMatchSingleDash(t *testing.T) {
	jobs := &fakeRunner{}
	deps, stdout, stderr := testDeps(t, jobs)
	if code := Run([]string{"info", "-json", "-imc"}, deps); code != ExitOK {
		t.Fatalf("single-dash exit = %d (stderr: %s)", code, stderr.String())
	}
	single := stdout.String()

	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"info", "--json", "--imc"}, deps); code != ExitOK {
		t.Fatalf("double-dash exit = %d (stderr: %s)", code, stderr.String())
	}
	if stdout.String() != single {
		t.Errorf("--json differs from -json:\n%s\n---\n%s", stdout.String(), single)
	}
}

// TestLogFileCapturesDiagnostics checks --log-file plus -v keep stdout pure
// while the debug trace lands in the file and on stderr.
func TestLogFileCapturesDiagnostics(t *testing.T) {
	jobs := &fakeRunner{script: []runner.Event{
		startedEvent(),
		outputEvent("memrate: read rate 21400.00 MB per sec"),
		finishedEvent(0, nil),
	}}
	deps, stdout, stderr := testDeps(t, jobs)
	path := filepath.Join(t.TempDir(), "diag.log")

	code := Run([]string{
		"bench", "-duration", "15s", "-json", "--log-file", path, "-v",
	}, deps)
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, ExitOK, stderr.String())
	}

	// stdout is still exactly one JSON document, even with debug logging on.
	var doc benchmark.Export
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, stdout.String())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	for _, want := range []string{"starting benchmark", "benchmark finished"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("log file missing %q:\n%s", want, data)
		}
	}
	if !strings.Contains(stderr.String(), "starting benchmark") {
		t.Errorf("stderr missing the debug trace:\n%s", stderr.String())
	}
}

// TestQuietSuppressesWarnings checks -q keeps a non-error diagnostic out of
// stderr without changing the exit code or the result.
func TestQuietSuppressesWarnings(t *testing.T) {
	jobs := &fakeRunner{script: []runner.Event{
		startedEvent(),
		outputEvent("stress-ng: info: completed in 5.00 secs"),
		finishedEvent(0, nil),
	}}
	deps, _, stderr := testDeps(t, jobs)
	missing := filepath.Join(t.TempDir(), "missing.job")

	if code := Run([]string{"burnin", "-duration", "5m", "-json", "-job", missing}, deps); code != ExitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, ExitOK, stderr.String())
	}
	if !containsFold(stderr.String(), "unavailable") {
		t.Fatalf("stderr = %q, want the missing-job warning", stderr.String())
	}

	stderr.Reset()
	if code := Run([]string{"burnin", "-duration", "5m", "-json", "-job", missing, "-q"}, deps); code != ExitOK {
		t.Fatalf("quiet exit = %d, want %d (stderr: %s)", code, ExitOK, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want nothing with -q", stderr.String())
	}
}

// TestReportJSON checks the report reuses the info and profile schemas instead
// of inventing new ones.
func TestReportJSON(t *testing.T) {
	jobs := &fakeRunner{}
	deps, stdout, stderr := testDeps(t, jobs)
	if err := store.SaveActiveProfile(config.Profile{
		Name:   "active",
		Values: map[string]string{"tCL": "16", "tRCD": "18"},
	}); err != nil {
		t.Fatalf("SaveActiveProfile: %v", err)
	}

	if code := Run([]string{"report", "-json"}, deps); code != ExitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, ExitOK, stderr.String())
	}
	var doc report.Document
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("report output is not one JSON document: %v\n%s", err, stdout.String())
	}
	if got := doc.Hardware.Values["DRAM Freq"]; got != "3200" {
		t.Errorf("hardware values = %v, want the injected detection result", doc.Hardware.Values)
	}
	if doc.Version != "9.9.9-test" {
		t.Errorf("version = %q", doc.Version)
	}
	if doc.Profile == nil || doc.Profile.Values["tCL"] != "16" {
		t.Errorf("profile = %+v, want the applied profile", doc.Profile)
	}
	if len(doc.History) != 0 {
		t.Errorf("history = %v, want none", doc.History)
	}
	if !strings.Contains(doc.Hardware.IMCError, "-imc") {
		t.Errorf("IMCError = %q, want the opt-in hint", doc.Hardware.IMCError)
	}
	if doc.GeneratedAt.IsZero() {
		t.Error("generated_at is zero")
	}
}

// TestReportMarkdownIsPlainWhenPiped checks the default style degrades to plain
// text, and that an explicit style still asks for colour.
func TestReportMarkdownIsPlainWhenPiped(t *testing.T) {
	jobs := &fakeRunner{}
	deps, stdout, stderr := testDeps(t, jobs)

	if code := Run([]string{"report", "--markdown"}, deps); code != ExitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, ExitOK, stderr.String())
	}
	out := stdout.String()
	if strings.Contains(out, "\x1b[") {
		t.Errorf("piped report contains ANSI escapes:\n%q", out)
	}
	if !strings.HasPrefix(out, "# MemStation report") {
		t.Errorf("piped report is not the Markdown source:\n%s", out)
	}
	for _, want := range []string{"MemStation report", "Hardware", "SPD", "Profile only"} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q:\n%s", want, out)
		}
	}

	stdout.Reset()
	if code := Run([]string{"report", "--style", "dark"}, deps); code != ExitOK {
		t.Fatalf("dark exit = %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\x1b[") {
		t.Error("an explicit --style dark did not emit any styling")
	}

	stdout.Reset()
	t.Setenv("NO_COLOR", "1")
	if code := Run([]string{"report"}, deps); code != ExitOK {
		t.Fatalf("NO_COLOR exit = %d", code)
	}
	if strings.Contains(stdout.String(), "\x1b[") {
		t.Errorf("NO_COLOR report contains ANSI escapes:\n%q", stdout.String())
	}
}

// TestReportFlagValidation covers the mutually exclusive output flags and the
// history bound.
func TestReportFlagValidation(t *testing.T) {
	jobs := &fakeRunner{}
	deps, stdout, stderr := testDeps(t, jobs)

	for _, args := range [][]string{
		{"report", "--json", "--markdown"},
		{"report", "--history", "0"},
	} {
		if code := Run(args, deps); code != ExitUsage {
			t.Errorf("%v exit = %d, want %d", args, code, ExitUsage)
		}
	}
	if stdout.Len() != 0 {
		t.Errorf("usage errors wrote to stdout: %q", stdout.String())
	}
	if !containsFold(stderr.String(), "exclusive") && !containsFold(stderr.String(), "at least 1") {
		t.Errorf("stderr = %q, want an explanation", stderr.String())
	}
}
