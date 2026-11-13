# MemStation TUI Implementation Brief

Read `notes/tui-plan.md` first, then this file, then the previous phase handoff
(`notes/handoffs/phaseN.md`). You are one worker in a sequential chain. The
parent owns acceptance; you own your phase and its commit.

## 0. Ground rules (all phases)

1. **Do not run verification.** By explicit owner instruction, do not run
   `go build`, `go test`, `go vet`, `go run`, the TUI, `stress-ng`, or any
   runtime smoke check in this session. Use `gofmt -w` on changed Go files
   (formatting/parse check) and `git` only. Do not "quickly check" a build.
2. **No new third-party dependencies.** Use the Go standard library and the
   Charm packages already in `go.mod` (`bubbletea`, `bubbles`, `lipgloss`).
   Do not change `go.mod`/`go.sum` unless a compile-critical import is missing;
   if unavoidable, explain it in the handoff.
3. **Never write to stdout/stderr from library packages.** Only
   `cmd/mem-station` may print (headless subcommands). TUI logs go to files.
4. **Preserve local work.** `burnin-memory.job` has an uncommitted local change
   (`timeout 30m`); do not revert or commit it. `burnin.log` is untracked and
   owned by root; do not touch or commit it. Never commit binaries or logs.
5. **One writer.** All phases run sequentially in
   `/home/jens/code/mem-station` on branch `tui-improvements`. Do not create
   worktrees, switch branches, merge, or rebase.
6. **Commit each phase.** `git add` only the paths you changed, then
   `git commit -m "tui: phase N - <summary>"`. Include
   `notes/handoffs/phaseN.md` in the commit. Do not `git add -A`.
7. **Leave a coherent state.** If you run out of room, finish the current file,
   commit what is coherent, and write the handoff with the exact remaining work.
   Do not leave half-applied renames.
8. **Read before editing.** Preserve existing behavior that this brief does not
   ask you to change. Prefer small, explicit functions over clever shortcuts.
9. **No fake data.** Do not render invented values, do not claim success from a
   stub, and do not label something "applied" unless it was persisted.
10. **Safety text.** Any apply/commit path must say "Profile only - not written
    to hardware". Never add code that writes MSRs/BIOS/hardware timings.

## 1. Shared contracts

### 1.1 `internal/config` (leaf)

```go
package config

const (
    KeyChannels = "channels"
    KeyRatio    = "dram_ratio"
    KeyFreq     = "dram_freq"
    KeyCL       = "tCL"
    KeyRCD      = "tRCD"
    KeyRP       = "tRP"
    KeyRAS      = "tRAS"
    KeyCR       = "command_rate"
    KeyRFC      = "tRFC"
    KeyVoltage  = "dram_voltage"
)

type Snapshot struct {
    Values     map[string]string `json:"values"`      // raw display values
    CapturedAt time.Time         `json:"captured_at"`
    Source     string            `json:"source"`      // detected|applied|draft
}

type Profile struct {
    Name       string            `json:"name"`
    Values     map[string]string `json:"values"`
    UpdatedAt  time.Time         `json:"updated_at"`
    Source     string            `json:"source,omitempty"`
}

func (s Snapshot) Float(key string) (float64, bool)
func (s Snapshot) FreqMT() float64
func (s Snapshot) Channels() int
func (s Snapshot) PeakMBs() float64       // freq * 8 * channels
func (s Snapshot) CASNs() float64         // 2000*CL/freq, 0 if unknown
func (s Snapshot) Clone() Snapshot
```

Keep values as strings so the UI can preserve `2T`, `1.35`, and placeholders.
Parsing lives in helpers, not every renderer.

### 1.2 `internal/runner`

```go
package runner

type Kind string
const (
    KindBenchmark Kind = "benchmark"
    KindBurnIn    Kind = "burnin"
)

type EventType int
const (
    EventStarted EventType = iota
    EventOutput
    EventFinished
)

type Event struct {
    Kind     Kind
    Type     EventType
    Line     string
    At       time.Time
    ExitCode int
    Err      error
    LogPath  string
}

type Spec struct {
    Kind     Kind
    Name     string
    Args     []string
    Dir      string
    Env      []string
    LogPath  string
    Duration time.Duration
}

type Job struct { /* unexported */ }

func Start(ctx context.Context, spec Spec) *Job
func (j *Job) Events() <-chan Event
func (j *Job) Cancel()
func (j *Job) Wait() error
var ErrCancelled = errors.New("job cancelled")
```

Requirements:
- Launch with `exec.CommandContext`, merge stdout+stderr through one scanner
  with a large buffer (>=1 MiB), tee every line to the log file.
- Emit exactly one `EventStarted`, zero or more `EventOutput`, exactly one
  `EventFinished`; close the channel after Finished.
- Started event carries `LogPath`.
- Finished carries `ExitCode`, `Err` (nil on success, `ErrCancelled` on cancel,
  `exec.ErrNotFound`/lookup error on missing binary, `*exec.ExitError` mapped to
  a useful error).
- `Cancel` is idempotent: SIGTERM the process group, wait up to 3 s, SIGKILL.
  Use `SysProcAttr{Setpgid: true}` on unix (`proc_unix.go` with
  `//go:build unix`) and `Process.Kill` fallback (`proc_other.go`).
- Create the log file (0600) and parent dirs; if creation fails, emit Finished
  with the error instead of panicking.
- Never block `Start`; the caller consumes events.

Tea bridge (app/tab side):
- A tab stores `job *runner.Job`, `events <-chan runner.Event`.
- `waitEventCmd(ch)` returns one `jobEventMsg`; after `EventOutput` return it
  again, after `EventFinished` stop.
- A 200 ms `tea.Tick` ticker drives elapsed/progress while running.
- Cancel calls `job.Cancel()` and keeps consuming until Finished.

### 1.3 `internal/store`

- `Dir()` resolution: `MEM_STATION_STATE_DIR`, else `$XDG_STATE_HOME/mem-station`,
  else `~/.local/state/mem-station`; create 0700.
- `LoadActiveProfile() (config.Profile, bool, error)`, `SaveActiveProfile`.
- `LoadNamedProfile(name)`, `SaveNamedProfile`, `ListProfiles`.
- `AppendHistory(Record) error`, `RecentHistory(kind runner.Kind, n int)`.
- `Record{ID, Kind, StartedAt, DurationMS, ExitCode, Error, LogPath,
  Config config.Snapshot, Payload json.RawMessage}`.
- History is JSONL, append-only, 0600; tolerate bad trailing lines.
- Atomic writes: temp file in the same dir + rename.
- `MaterializeJob(preset []byte, duration time.Duration) (path string, err error)`:
  write to `jobs/`, replace a line matching `^\s*timeout\s+.*$` with
  `timeout <duration>`, append the line if absent. Return absolute path.
- `LogPath(kind runner.Kind, t time.Time) string` under `logs/`.

### 1.4 `internal/style`

- Keep the existing color constants as sensible dark defaults.
- Add a semantic `Theme`:
  `Bg, Fg, Muted, Accent, AccentBg, Border, BorderFocus, Success, Warning,
  Danger, Info, Label, Value, Dim, TabActiveFg/Bg, TabInactiveFg/Bg,
  ButtonFg/Bg, ButtonFocusFg/Bg, StatusFg, ErrorBg`.
- `style.Default(dark bool) Theme` using `lipgloss.AdaptiveColor`; call once in
  `main` and pass down (package-level current theme is acceptable if documented).
- Respect `NO_COLOR` / `CLICOLOR=0` by using the ASCII color profile or plain
  styles.
- Reuse styles instead of hardcoded widths/colors in renderers. Do not delete
  widely used exported vars unless you update every use.

### 1.5 `internal/ui`

Small, reusable helpers (keep this package dependency-light):

- `ClipLines(s string, h int) string` - hard height limit.
- `Truncate(s string, w int, ellipsis bool) string` using `lipgloss.Width`.
- `PadRight(s string, w int) string`.
- `JoinColumns(rows [][]string, widths []int) string` - aligned text tables.
- `Panel(title string, width, height int, focused bool, body string) string` -
  rounded border, title inserted in the top border, body clipped/padded.
- `Footer(status string, level Level, hints []KeyHint, width int) string` -
  left status, right keys, one line.
- `HelpOverlay(width, height int, sections []HelpSection) string`.
- `Modal(title string, body []string, width int) string`.
- `Progress(pct float64, width int) string` and `Spinner` (bubbles/spinner ok).
- `KeyHint{Key, Action string}`, `HelpSection{Title string, Rows []KeyHint}`.

### 1.6 `internal/memory` (hardware leaf)

- Keep `DetectDefaults`, `ParseSPDTimings`, `ParseSPDInfo` signatures.
- Move root `mchbar.go` into `internal/memory/imc_linux.go` (`//go:build linux`),
  package `memory`, exported:
  ```go
  type IMCTimings struct { Channel int; /* exported fields for all registers */ }
  func ReadMCHBARTimings() ([]IMCTimings, error)
  ```
- Add `imc_stub.go` (`//go:build !linux`) returning `nil, nil` or an
  unsupported error. Do not leave `package main` files at the repo root.
- Keep `dmidecode`/`decode-dimms` calls but never print; return errors/messages.

### 1.7 App shell

- `internal/app` is package `app`; rename the old `internal/model` directory.
- `app.Options{InitialTab int; JobPath string; StateDir string; Dark bool;
  Mouse bool; Version string; Detect func() (map[string]string, string,
  *memory.SPDInfo) }`; defaults via `app.DefaultOptions()`.
- Root `Model` fields include: active tab, width/height, status/level, help
  open flag, modal state, running indicators, state store, and the three tab
  models (`sysinfo.Model`, `benchmark.Model`, `burnin.Model`).
- `Init()` returns `tea.Batch(detectCmd, ...)`. `detectCmd` calls the injected
  detect func (default `memory.DetectDefaults`) and returns `detectResultMsg`.
  Do not call detection in the constructor before first paint.
- Global key handling consults the active tab:
  `Editing() bool` (text input owns letter keys), `ModalOpen() bool`.
  When either is true, only `ctrl+c`, `esc`, `enter`, `tab`, `shift+tab` may
  be interpreted globally, and only where the tab does not consume them.
- Route `WindowSizeMsg` to all tabs. Route `jobEventMsg` to the owning tab by
  `Kind`, even if that tab is not active. Route `tickMsg` similarly.
- Status messages: `statusMsg{text string; level ui.Level}`; root displays them.
  A tab can return `tea.Cmd` producing `statusMsg`; root may auto-clear after
  5 s via `clearStatusMsg` (do not clear errors).
- Layout: compute header/tabbar/footer heights, pass the remaining body size to
  the active tab. Never render more lines than the body height. At
  `width < 80 || height < 24`, render the too-small screen instead of tabs.
- Two-column breakpoint at width >= 110; below, stack panels.
- Tab keys: F1-F3, Ctrl+1..3 always; plain 1/2/3 only when not editing.
- `?` toggles help; `esc` closes help/modal before anything else.
- `q` quits only when not editing and no modal/help is open.
- No `WithMouseAllMotion` by default. If mouse is enabled, use cell motion and
  only handle wheel events for viewports.

## 2. Phase 0 - Stabilize (worker 1)

Goal: remove the acute bugs and create the runner/store/scaffolding that later
phases use, without the full app split. Keep the current layout working.

Tasks:
1. Move root `mchbar.go` to `internal/memory/imc_linux.go` with build tags and
   exported `IMCTimings`/`ReadMCHBARTimings`; add the non-linux stub. Update the
   temporary model code to use it. Delete the root file. There must be no
   `package main` outside `cmd/`.
2. Create `internal/runner` exactly as specified. Use it for both benchmark and
   burn-in. Remove the nested-cmd streaming hack. Ensure the success path clears
   `burnInRunning` and emits a result.
3. Create `internal/store` with profile/history/log/materialize support. For
   Phase 0, use it for the burn-in log location and state dir. Do not yet build
   the full UI history.
4. Implement real burn-in parsing:
   `burnin.ParseBurnInOutput(output string, exitCode int, err error)
   *burnin.Results` (adjust existing API carefully). Remove `fmt.Println`.
   `Success` requires exit code 0 and zero error lines. Include
   `Duration`, `Errors`, `ExitCode`, `LogPath`, `RawOutput` (bounded, e.g. last
   2000 lines).
5. Fix the viewport: on `WindowSizeMsg`, set width/height from the body area;
   on tab switch, resize. Burn-in output must be visible at 120x40 and 80x24.
6. Fix key handling minimally in the existing model until Phase 1:
   - Track whether a field is editing (input focused) vs selected.
   - While editing, letters/digits go to the input; `q` must not quit.
   - `esc` first exits editing/restores the previous draft value; a second esc
     resets the draft to applied (or detected if no applied).
   - Global keys only when not editing.
7. Bounds correctness: parse voltage as float; represent bounds as `float64`;
   below-min timing = danger, above-max timing = warning/info, voltage outside
   range = danger. Do not color ranges for unknown (`?`) values.
8. Async detection: `Init()` returns the detect command; show a loading status
   and retry key `R`. Do not block construction.
9. Remove `WithMouseAllMotion`; use no mouse for now. Remove unused stubs
   (`sanitizeNumeric` no-op, unused stream functions) and dead imports.
10. Job path/log handling: resolve the job path from `MEM_STATION_JOB` or
    `--job` if present, then `./burnin-memory.job`, then executable-adjacent;
    otherwise return a clear error. Materialize duration into the state dir
    instead of mutating the repo file. Default burn-in duration: 30m (matching
    the local job file); never claim 30s.
11. Optional but useful: a `SIGINT`/quit path cancels any running job before
    exiting.

Do not build the new app shell yet. Do not add tests that require running.

Handoff: exact files moved/created, runner event semantics, remaining Phase 1
work, any behavior intentionally left.

## 3. Phase 1 - App shell, layout, keymap, tab packages (worker 2)

Goal: split the app and make the shell responsive and modal.

Tasks:
1. Rename `internal/model` to `internal/app` (package `app`), update
   `cmd/mem-station/main.go`. Split into `model.go`, `update.go`, `view.go`,
   `keys.go`, `messages.go`, `layout.go`. No 1000-line file.
2. Create `internal/ui` helpers and `internal/style` theme as specified. Use
   them in the shell and tabs; delete hardcoded layout widths where replaced.
3. Extract the three tabs into packages:
   - `internal/sysinfo`: current timing editor + guide panel + SPD panel + IMC
     panel (behavior may remain close to Phase 0 for now).
   - `internal/benchmark`: current benchmark tab.
   - `internal/burnin`: current burn-in tab + Phase 0 streaming.
   Each exposes `Update`, `View(w,h)`, `Editing()`, `ModalOpen()`,
   `Running()`, `Title()`, and `Keys() []ui.KeyHint`.
   Avoid import cycles: tab packages may import config/runner/store/style/ui,
   but never `app`.
4. Root app:
   - Ownership of global keys and routing; no duplicate key strings across
     files.
   - Header with host summary and dirty/running markers.
   - Numbered tab bar with active state and key hints.
   - Fixed footer with status/level and context keys.
   - `?` help overlay; `esc` closes overlays; `q` gated.
   - Too-small screen (<80x24); body exact height via `ui.ClipLines`.
   - Two-column at >=110, stacked below; no content pushing the footer.
   - Resize all viewports on `WindowSizeMsg`.
5. Keymap:
   - `bubbles/key` bindings in one `KeyMap` type; sections for Global,
     Timings, Benchmark, Burn-in.
   - Plain number tab keys only when not editing.
   - Field navigation: Up/Down when selected; Enter to edit; Esc to cancel;
     Tab/Shift+Tab commits and moves. Document that arrows edit text while
     editing.
   - Ratio lock `l`; apply `ctrl+s`; reset draft `ctrl+r`; subviews `[`/`]`.
   - Jobs: Enter run, `x` cancel, `f` follow, `e` errors-only, PgUp/PgDn scroll.
6. Job state machine shared by the run tabs: Idle, Running, Done, Failed,
   Cancelled; elapsed ticker; exactly one transition to terminal state; notify
   root for the running marker; preserve results after switching tabs.
7. Detection: move to async `Init` command, loading UI, error banner with
   retry `R`, and manual editing allowed with a warning. No panics when SPD or
   IMC is nil.
8. Remove all hardcoded "(30s)" strings; durations come from tab config.
9. Keep Phase 0 behavior and fixes. Do not implement Phase 2/3 features yet.

Handoff: new package tree, routing/message contracts, keymap table, layout
math, remaining work for Phase 2/3.

## 4. Phase 2 - Timings workspace (worker 3)

Goal: make the Timings tab the real overclocking workspace.

Tasks:
1. Implement typed fields in `internal/sysinfo/fields.go`:
   `FieldID`, `Field{ID, Label, Unit, Group, Kind, Options, Default, Desc,
   Step, Scale, Bounds func(ValidationContext) Bounds, Format}`.
   Groups: General, Primary, Secondary, Voltage. Add at minimum: channels,
   ratio, freq, tCL, tRCD, tRP, tRAS, CR, tRFC, voltage.
2. Replace label-string lookups (`parseNumericField("DRAM Freq")`) with IDs and
   a values map. Keep display formatting as strings in the profile.
3. Implement detected/draft/applied:
   - detected from hardware (or empty placeholders with warnings).
   - applied loaded from `store.LoadActiveProfile`; fall back to detected.
   - draft initialized from applied; edits update draft only.
   - `Dirty()` compares draft vs applied.
   - Profile save on apply only.
4. Validation in `internal/sysinfo/validate.go`:
   - `Issue{FieldID, Severity, Message, Delta float64}`.
   - timing minimums from frequency and JEDEC-ish rules (existing formulas are a
     starting point; keep them documented and correct them where obviously
     wrong).
   - voltage parsed as float and checked against 1.35-1.65 V; CR in {1T,2T};
     unknown values produce info "waiting for detection", not a fake range.
   - derived rules: tRAS >= tCL+tRCD, tRFC ns, tRC if available.
   - margins in cycles/ns/V; render symbol + margin.
5. Editing UX:
   - selected vs editing modes per field as in the shared contract.
   - Enter edits; Esc restores the pre-edit draft value and exits editing;
     Tab/Shift+Tab commits and moves; Up/Down moves when selected.
   - disable global bindings while editing.
   - ratio lock scales primary timings from the previous frequency on commit;
     clamp to valid minimums; do not rescale on every keystroke.
   - `Ctrl+R` resets draft to applied; per-field Esc reverts.
6. Apply modal:
   - `Ctrl+S` opens review with diff (applied -> draft), warnings, and the
     "Profile only - not written to hardware" disclaimer.
   - Enter confirms (save active profile, applied = draft, status), Esc cancels.
   - No-op apply when not dirty; show a message instead.
7. Renderer:
   - Aligned columns: Setting, Detected, Draft, Delta, Safety.
   - Fixed widths computed from the panel width; truncate notes.
   - Live derived panel: peak/sustained bandwidth, per-channel, CAS ns, deltas
     vs applied; highlight regressions.
   - Dirty marker in the tab/header via root.
8. Sub-views Timings / SPD / IMC with `[`/`]`:
   - SPD: all `memory.SPDInfo` fields, grouped, scrollable viewport; no silent
     truncation.
   - IMC: all channels from `memory.IMCTimings`, matrices, scrollable.
   - Preserve scroll position per sub-view; resize viewports.
9. Expose `Snapshot() config.Snapshot` to root so benchmark/burn-in know the
   current applied config. Root forwards updates after apply.
10. Write (do not run) validation and view tests.

Handoff: field list/rules, apply flow, snapshot API, remaining Phase 3 work.

## 5. Phase 3 - Benchmark and Burn-in runs (worker 4)

Goal: real, cancellable, observable runs with history.

Tasks:
1. Benchmark tab:
   - Config: duration (15s/30s/1m/5m/custom), buffer (128M/256M/512M/1G),
     workers (1..N), validated; build argv, never a shell string.
   - Run via runner; disable config while running; ticker for elapsed and
     progress bar; spinner; cancel `x`; `ctrl+c` also cancels and stays in app.
   - Stream output to a scrollable viewport with follow toggle; show the last
     error prominently.
   - Parse with `bench.ParseBenchOutput`; on finish persist a history record
     with config snapshot, result payload, exit code, log path.
   - Results: last vs previous run delta, theoretical/effective; history table
     for the last 10 runs (timestamp, read, write, duration, config summary).
   - Export selected/last run JSON to the state dir; show path.
   - Handle missing stress-ng / non-zero exit / parse failure with the raw log.
2. Burn-in tab:
   - Presets: embedded/default "memory-heavy" copied from
     `burnin-memory.job` into `internal/burnin/presets/memory.job`, plus an
     optional custom path. Duration options 5m/10m/30m/1h/custom; stop-on-error
     toggle.
   - Materialize job with chosen timeout in state dir; never modify the repo
     job at runtime; log to state dir.
   - Run via runner; live viewport with follow/filter (all/errors), PgUp/PgDn;
     error counter; prominent banner when errors > 0; auto-cancel if
     stop-on-error.
   - Terminal states: pass only when exit 0 and zero parsed errors; cancelled
     and failed are distinct.
   - Persist result + log path; show summary, elapsed, errors, exit code, and
     "view log" hint after finish.
   - Cancel kills the process group; verify (by code inspection) no orphan path.
3. History/store integration:
   - Append records atomically; load recent records on tab init.
   - Show comparative deltas and configuration labels; never mix runs from
     different configs without labeling.
   - Export JSON with a stable schema.
4. Root integration:
   - Running marker in header/tab bar; events route even when the tab is not
     active; returning to the tab shows current progress/log.
   - Status messages do not overwrite an active error banner.
5. Write (do not run) parser fixtures, history round-trip tests, and golden
   views for running/done/cancelled states.

Handoff: config options, history schema, cancel semantics, remaining Phase 4.

## 6. Phase 4 - Polish, CLI, docs, tests, CI (worker 5)

Goal: make the branch reviewable and operationally complete without running
verification.

Tasks:
1. Theming/accessibility:
   - Apply semantic theme across all tabs; adaptive light/dark and NO_COLOR.
   - Severity uses symbols + text, not color alone; consistent focus styling.
   - Minimum contrast and no hardcoded colors left in renderers.
2. CLI (`cmd/mem-station`):
   - No-arg / `tui` launches the TUI; flags `--tab`, `--dark`, `--no-color`,
     `--mouse`, `--state-dir`, `--job`, `--version`.
   - Headless `info [-json]`, `bench [-duration] [-buffer] [-workers] [-json]`,
     `burnin [-duration] [-preset] [-job] [-json]`. Reuse hw/runner/store.
   - SIGINT cancels cleanly; JSON is one object/stream on stdout, human output
     goes to stdout, errors to stderr; exit codes meaningful.
   - Do not add a shell dependency for headless runs.
3. Profiles/export:
   - Named profile save/load/list; export active profile JSON and last run
     JSON to a user-specified path; import profile JSON with validation.
   - TUI keys for export/import (document them) and status confirmations.
4. Docs:
   - Update `README.md`: features, keys, headless commands, state dir, safety
     (profile-only apply), required tools, job path.
   - Finalize `notes/verification-plan.md` with the exact commands and manual
     matrix (this is a summary; do not execute it).
   - Keep the plan and handoffs accurate.
5. Tests (write, do not run):
   - Parser fixtures (`bench`, `burnin`), store round-trips/atomicity,
     validation tables, key gating, runner cancel (integration-tagged if it
     needs a shell), golden views at 80x24 and 120x40 for all three tabs and
     overlays.
   - Golden test update flag `-update`; strip ANSI; no hardware calls.
6. CI: `.github/workflows/ci.yml` with gofmt check, `go vet ./...`,
   `go test ./...`, `go build ./...` on ubuntu-latest; no root/hardware.
7. Final consistency pass by reading code:
   - no `fmt.Println` in library packages;
   - no `Applied (mock)` or stub success;
   - no unused exported stubs/dead files;
   - no root `package main`;
   - imports resolve within the target tree;
   - `go.mod` unchanged unless justified.
8. Final commit and handoff.

## 7. Handoff format

Write `notes/handoffs/phaseN.md` with:

```md
# Phase N handoff

- Commit: <sha> <subject>
- Goal: ...
- Delivered: bullet list with file paths
- Contracts added/changed: ...
- Deviations from brief: ...
- Known risks / unverified areas: ...
- Exact remaining work for the next phase: ...
- Commands intentionally not run: ...
```

Keep it short and factual. The next worker reads it before touching code.

## 8. Final reminders

- The owner's instruction "skip all verification in this session" is explicit.
  Do not build, test, run, or profile. A clean `gofmt` is the only automated
  check.
- If you must choose between a large risky refactor and a smaller coherent
  improvement, choose the smaller one and document the follow-up.
- Do not claim the branch is ready to merge. Verification is deferred.
