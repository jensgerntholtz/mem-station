# Verification Plan (Summary)

Status: **automated checks pass as of Phase 4; the manual matrix is still
unexecuted.** Phases 0-3 wrote code without running it. Phase 4 re-enabled the
automated checks (`gofmt`, `go vet`, `go build`, `go test`) and fixed what they
found; the interactive TUI, `stress-ng` runs, and the hardware matrix below have
never been executed. Run this plan in a later session before the
`tui-improvements` branch is considered merge-ready.

## 1. Scope

Verify the TUI after all four phases:

- Phase 0: streaming, cancellation, viewport sizing, key gating, bounds,
  async detection, buildability.
- Phase 1: app shell, responsive layout, keymap/help/footer, tab routing,
  job state machine.
- Phase 2: typed timing fields, detected/draft/applied, validation, apply
  modal, SPD/IMC sub-views, perf deltas.
- Phase 3: benchmark and burn-in configuration, live progress/log, history,
  persistence, profile-only apply.
- Phase 4: theming/accessibility, CLI/headless mode, exports, tests, CI.

Out of scope: writing timings to hardware, BIOS/MSR paths, benchmark validity
against external databases, stress-ng correctness itself.

## 2. Preconditions

- Linux host with the hardware described in the README (or a VM for UI-only
  checks).
- Go toolchain matching `go.mod`; no new dependencies expected.
- `stress-ng` installed for run tests; `dmidecode` and `decode-dimms` for
  detection; root/sudo optional (both paths must be tested).
- A disposable state dir:
  ```bash
  export MEM_STATION_STATE_DIR="$(mktemp -d)"
  ```
- Terminal >= 80x24; a second run at 120x40 and 160x50.
- `gofmt`, `go vet`, `go test`, `go build` available.

## 3. Static and automated checks

Run from `/home/jens/code/mem-station`:

```bash
make            # gofmt, vet, build, test (identical to CI)
make race       # go test ./... -race -count=1
make integration # go vet/test -tags integration ./internal/runner
```

The underlying commands, if you prefer them raw:

```bash
gofmt -l .
go vet ./...
go build ./...
go test ./... -count=1
go test ./... -race -count=1
go vet -tags integration ./internal/runner        # compiles the tagged tests
go test -tags integration ./internal/runner       # /bin/sh only, no stress-ng
```

Expected:
- `gofmt -l .` prints nothing.
- `go vet` clean.
- `go build ./...` succeeds (this was broken by root `mchbar.go` before).
- All tests pass; no test requires root, hardware, or `stress-ng` unless
  explicitly tagged.
- Golden view tests pass at 80x24 and 120x40 (80x24 means a 78x21 body inside
  the fixed header/tab bar/footer). Every golden goes through the shared
  `internal/uitest` harness, which strips ANSI and owns the single `-update`
  flag per test binary. The flag only exists where the harness is linked, so a
  repo-wide update needs the environment variable (this is what `make goldens`
  runs). Review the diff before committing:
  ```bash
  MEM_STATION_UPDATE_GOLDEN=1 go test ./... -run TestGolden -count=1
  git diff -- '*/testdata/*.golden'
  ```
- Parser fixtures: bench and burn-in logs parse to expected values, including
  GB/s vs MB/s, missing rates, error lines, and non-zero exits.
- Store: profile/history round-trip, JSONL append, bad trailing line tolerated,
  atomic write leaves no partial file.
- Runner: missing binary, cancel, and normal exit produce exactly one
  `EventFinished`; cancel kills the process group. Integration test may be
  tagged `//go:build integration` and skipped otherwise.
- Key gating tests: `q` while editing does not quit; plain `1/2/3` while
  editing types digits; `?` opens help; `esc` closes modal before cancelling a
  draft.
- Bubble Tea v2 specifics (manual, in a real terminal):
  - `space` now arrives as `"space"` rather than `" "`, so any space handling
    and the text inputs need a once-over by hand.
  - `shift+tab` still steps backwards through the timing fields.
  - `esc` still cancels the edit/modal (and is not eaten as an alt sequence).
  - Bracketed paste into the timing editor and the profile path prompts.
  - `--mouse` wheel scrolling, `--no-color`, and `NO_COLOR=1`.
  - Alt screen is entered and restored on quit; the window title is set.
- Command line (fang/cobra/log v2), automated where possible:
  - `mem-station help`, `mem-station <cmd> --help`, `mem-station version`,
    `mem-station --version`, `mem-station completion bash`, `mem-station man`.
  - The documented single-dash flags still work and match their double-dash
    form: `info -json -imc` must print the same document as
    `info --json --imc` (covered by a test).
  - `-json -v` keeps stdout to exactly one JSON document while the debug trace
    goes to stderr.
  - `--log-file PATH` appends the same diagnostics to a `0600` file; `-q`
    suppresses warnings such as an unreadable configured job file.
  - Exit codes: unknown command or flag → `2`; a failed bench/burn-in → `1`
    with the verdict on stdout and no extra error banner; `130` on interrupt.
- Dialogs and pickers (P3):
  - Help, the apply review and every prompt float over the tab instead of
    replacing it; the frame stays exactly 80x24 and 120x40.
  - `ctrl+l`: the picker lists saved profiles, `/` filters, ↑/↓ selects, enter
    loads into the draft, esc/q close. It must never quit the program.
  - `ctrl+o`: the browser starts in the working directory, `*.json` files are
    selectable, `h`/`backspace`/`←` go up, esc closes. An invalid file keeps
    the dialog open with the reason inside it.
  - `ctrl+e` in Burn-in: the embedded preset explains that `--job PATH` is
    needed; a custom job file opens in `$EDITOR` and is reloaded on return.
  - The terminal progress bar appears only while a bench/burn-in is running.
- `report`:
  - `mem-station report` on a terminal renders with glamour; `NO_COLOR=1` gives
    plain rendered text.
  - `mem-station report > report.md` writes raw Markdown (starts with
    `# MemStation report`, no ANSI) that GitHub/pandoc can read.
  - `--style dark` forces styling even when piped; `--style raw` writes the
    source on a terminal.
  - `report --json` parses as one document whose `hardware` section matches
    `info --json`; `--json --markdown` and `--history 0` exit `2`.
  - A machine with no SPD, no applied profile and no runs still produces every
    section with an explanation instead of an empty document.

CI (`.github/workflows/ci.yml`) runs gofmt, `go vet ./...`, `go build ./...`,
and `go test ./... -count=1` on `ubuntu-latest`. CI does not run hardware, the
tagged integration tests, or long burn-in tests; the push to this branch is the
first real CI run of that workflow (it has never executed).

## 4. Manual TUI matrix

Launch:
```bash
go run ./cmd/mem-station
```

### 4.1 Layout and resize

| Size | Expected |
| --- | --- |
| 160x50 | Two columns; header, tab bar, body, footer all visible |
| 120x40 | Two columns; no clipping/wrapping that hides controls |
| 100x30 | Stacked single column; scrollable body; footer visible |
| 80x24 | Minimum supported; footer visible; body scrolls |
| 70x20 | "Terminal too small" screen, no panic, resize recovers |
| Resize while running a job | Viewports resize, footer stays, progress/log remains |

Check on each tab: Timings, SPD, IMC, Benchmark, Burn-in, Help overlay, Apply
modal, Error banner.

### 4.2 Keys and modality

- `q` while a timing field is editing inserts `q`; it does not quit.
- `q` on a tab with no editor quits.
- Plain `1/2/3` while editing type digits; `Ctrl+1..3` and `F1..F3` switch
  tabs from anywhere.
- `Tab`/`Shift+Tab` commit and move focus; `Up`/`Down` move when selected;
  `Left`/`Right` decrease/increase the selected numeric value by its `Step`,
  or cycle preset values; `Enter` starts editing (except preset-only fields);
  `Esc` cancels the edit; second `Esc` reverts the field (or the draft, per the
  final behavior documented in the handoff).
- `?` toggles help; `Esc` closes it; help scrolls if longer than the screen.
- Mouse disabled by default; if `--mouse` is enabled, wheel scrolls viewports
  and does not break text selection or flood events.

### 4.3 Timings and validation

- With detection unavailable (non-root), the app paints immediately, shows a
  loading/error state with the sudo hint, and allows manual entry with a
  warning. `R` retries.
- With detection available, detected values populate; SPD/IMC sub-views show
  all parsed fields; `[`/`]` switch views and preserve state.
- Enter a frequency change with ratio lock on: primary timings scale once from
  the previous frequency; they do not compound on each keystroke. The same
  applies when `Left`/`Right` changes the frequency.
- Numeric fields accept digits only (plus one decimal point for voltage); try
  typing letters and pasting an invalid value. `Command Rate` cycles `1T`/`2T`
  with `Left`/`Right` and does not open a text editor. With SPD data, the
  frequency field cycles the detected speed list while still allowing a custom
  value through `Enter`.
- Set tCL below the minimum: red `x` with margin text; above soft max: warning
  `!`; valid: `check`. Verify at least one case each, plus voltage 1.30/1.35/
  1.70 V and CR 1T/2T.
- Derived panel updates live (peak/sustained, CAS ns, deltas). `tRAS <
  tCL+tRCD` warns.
- `Ctrl+R` restores applied values; dirty marker clears.
- `Ctrl+S` opens the review modal with diff and "Profile only - not written to
  hardware"; Enter saves the active profile, Esc does nothing. Restart the TUI
  and confirm applied values reload. No hardware write occurs.

### 4.4 Benchmark

- Missing `stress-ng` (use a PATH shim or uninstall in a container): clear error
  with the attempted command, no panic, no stuck "running" state.
- 15 s run: progress/elapsed updates, log streams, results show measured vs
  previous vs theoretical, history gains one record, log file exists under the
  state dir.
- Run twice and confirm the second run shows deltas vs the first and that the
  history table labels timestamps/configs.
- Cancel with `x` mid-run: process group dies (check `pgrep -a stress-ng`),
  state becomes Cancelled, log retained, app remains responsive.
- Switch tabs while running; return and confirm progress/log are current.
- Invalid config input is rejected before launch (no shell injection).

### 4.5 Burn-in

- Start a short run (5m or a 1m custom duration) and confirm:
  - the materialized job under the state dir has the chosen `timeout`;
  - the repo `burnin-memory.job` is unchanged;
  - progress/elapsed/error counter update;
  - follow/filter/scroll work;
  - cancel kills the process group;
  - the result is labelled Pass/Fail/Cancelled correctly;
  - the summary and log path remain after completion.
- Inject an error line in a fake job/output (or use a failing command): banner
  appears, error count increments, stop-on-error cancels.
- Leave the tab and return: state/log preserved.
- Long-run sanity (optional, 10 min): no memory growth from unbounded log
  buffers; log viewport caps retained lines; no leaked goroutines/processes.
- Quit while running: job cancelled; no `stress-ng` left behind.

### 4.6 Persistence, export, CLI

- State dir override works; all files are 0600/0700 and no writes land in CWD.
- Quit/restart: active profile and history reload; history comparison still
  valid.
- Export profile and run JSON; validate schema and that no secrets/paths are
  surprising. `ctrl+e` in the Timings tab prompts for a destination path and
  writes mode 0600 (also for a path that does not exist yet).
- `ctrl+p` saves the draft as a named profile; `ctrl+l` lists the saved names in
  the prompt hint and loads one into the draft. Neither changes the applied
  profile.
- Import a profile with `ctrl+o`: broken JSON, an empty `values` map, and an
  unknown key all keep the modal open with the reason; a valid file merges as a
  draft, not hardware.
- Headless:
  ```bash
  go run ./cmd/mem-station info --json
  go run ./cmd/mem-station info --json -imc   # opt-in raw IMC read
  go run ./cmd/mem-station bench --duration 15s --buffer 256M --json
  go run ./cmd/mem-station burnin --duration 1m --json
  ```
  JSON is valid, exits are meaningful, stderr is separate, SIGINT cancels.
- `--version`, `--tab`, `--no-color`, `--dark`, `--imc` behave as documented.
- `info` without `-imc` never opens `/dev/mem`; `-imc` reports a clear
  root/iomem error on an unsupported or unprivileged system.

### 4.7 Theme and accessibility

- `NO_COLOR=1` and `--no-color`: output remains legible; severity still shows
  `ok`/`?`/`!`/`x` markers and every verdict keeps its word (PASS/FAIL/
  CANCELLED), so nothing depends on color.
- Light and dark backgrounds (`--dark` vs `--dark=false`): labels/values/status
  meet reasonable contrast; focus (selected field, active tab, focused panel
  border) stays distinguishable from the inactive state.
- 256-color and 16-color terminals degrade without losing status text.
- ANSI escapes are not printed raw; no `fmt.Println` output leaks into the alt
  screen.

## 5. Acceptance criteria

Merge-ready only if all are true:

1. `gofmt`, `go vet`, `go build ./...`, and `go test ./...` pass locally and in
   CI.
2. The app renders correctly at 80x24, 120x40, and 160x50 with the footer
   visible on every tab.
3. No key both types and triggers a global action; `q` cannot quit from an
   editor.
4. Burn-in streams continuously, cancels cleanly, never leaves children, and
   reports Pass/Fail/Cancelled truthfully.
5. Benchmark cancel and history comparison work; logs persist under the state
   dir.
6. Apply is clearly profile-only, persists, and never writes hardware.
7. Missing hardware tools and missing `stress-ng` fail gracefully.
8. Detection never blocks first paint; retry works.
9. No stdout/stderr writes from library packages; no root `package main`.
10. Golden views and parser/store/validation tests pass.

## 6. Known risks and things to watch

- **The manual matrix is unverified.** Phases 0-3 wrote code without running it;
  Phase 4 made gofmt/vet/build/test pass, but nobody has yet looked at the TUI,
  run `stress-ng`, or checked the hardware detections. Treat the automated pass
  as "it compiles and the unit tests agree with it", nothing more.
- **Import cycles** from extracting tabs. Check `go list`/build first.
- **Golden tests** can be brittle with lipgloss version differences; strip ANSI
  and assert size/content, not raw color codes.
- **Process groups**: `Setpgid` is unix-only; ensure the non-unix stub builds
  even though the app targets Linux.
- **Job file duplication**: the embedded preset and the repo job can drift.
  Treat the preset as canonical for TUI runs and note it in the README.
- **Long burn-in**: the 30m default can hide leaks in a short smoke test; run
  at least one 10+ minute test with a memory check (e.g. `go tool pprof` or
  `ps -o rss`).
- **dmidecode/decode-dimms** output varies by distro/version; parser fixtures
  should cover the sample in `notes/outputs/` plus a missing-field case.
- **History volume**: JSONL is append-only; add a cap/rotation if repeated runs
  grow the file. Verify the UI only loads the last N.

## 7. Deferred-work record

This plan was written before (and not executed during) implementation. The
phase handoffs in `notes/handoffs/` list what each worker intentionally left
unverified. The first verification session should start with Section 3, fix
build/test failures, then proceed through Section 4. Do not merge on the basis
of a clean `gofmt` alone.
