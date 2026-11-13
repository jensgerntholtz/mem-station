# MemStation TUI Improvement Plan

Status: approved for implementation on branch `tui-improvements`.
Implementation is split across sequential subagent phases. Verification is
deferred by request; see `notes/verification-plan.md`.

## 1. Current state (assessed 2026-11)

- `internal/model/model.go` is a 1080-line file mixing types, detection mapping,
  update logic, rendering, validation, and styling.
- The burn-in streaming pipeline is broken in several ways: nested `tea.Cmd`
  messages, a new process per line if naively fixed, a success path that never
  clears `burnInRunning`, a zero-sized viewport, and `fmt.Println` in the
  parser corrupting the alt screen.
- Global keys are handled while typing, so `q` quits from a text field and `l`
  toggles the ratio lock while editing.
- `m.height` is stored but never used. At 80x24 the app renders roughly 45
  lines; the footer is invisible and panels wrap badly.
- Apply is `Applied (mock)`; there is no detected/draft/applied model.
- Hardware detection blocks the first paint; `stress-ng` jobs cannot be
  cancelled and always claim 30 s while the job file says 30 min.
- Bounds parsing treats voltage as an integer and inverts severity for min/max
  violations. `sanitizeNumeric` and `readMCHBARTimings` are stubs.
- Root `mchbar.go` is `package main` without `main`, so `go build ./...` fails.
- No persistence, no run history, no profile export, no tests, no CI.

## 2. Product decisions

These are fixed for the implementation unless the owner changes them:

1. **Apply is profile-only.** The TUI never writes timings to hardware. Apply
   commits the draft to the active profile and the UI says
   "Profile only - not written to hardware". Export is available.
2. **Minimum terminal is 80x24.** Below that, show a "terminal too small"
   screen. Two-column layout at >=110 columns, single-column below.
3. **Modality is explicit.** A field is either *selected* or *editing*.
   Global letter shortcuts are inactive while editing. `q` quits only when not
   editing and no modal is open.
4. **System Info has sub-views:** Timings, SPD, IMC, switched with `[`/`]`.
5. **Tabs are 1 Timings, 2 Benchmark, 3 Burn-in.** F1-F3 and Ctrl+1..3 always
   work; plain 1/2/3 only when not editing.
6. **Safety is communicated without color alone:** `check`, `!`, `x` symbols
   plus text margins, in addition to color.
7. **Async by default:** detection happens after first paint; jobs stream
   events and are cancellable.
8. **State lives in XDG state dirs** (`$XDG_STATE_HOME/mem-station` or
   `~/.local/state/mem-station`), overridable with `MEM_STATION_STATE_DIR`.

## 3. Target experience

### 3.1 Shell

```
+ mem-station ---------------------------------------------------------------+
| 1 Timings   2 Benchmark   3 Burn-in        Z97M Pro4 - i5-4670K - 2x8 GB  |
+----------------------------------------------------------------------------+
|                                                                            |
|   tab body (exact height, clipped, scrollable where needed)                |
|                                                                            |
+----------------------------------------------------------------------------+
| Profile only - press Ctrl+S to review & apply        ? help  q quit        |
+----------------------------------------------------------------------------+
```

- Header: product, active host summary, dirty marker (`*`) when draft differs
  from applied, running marker (`running: bench`) when a job is active.
- Tab bar: numbered tabs with key hints; active tab is visually distinct.
- Body: receives exact `width x height`; never lets content push the footer off
  screen.
- Footer: status/error text on the left, context keys on the right, truncated.
- `?` opens a scrollable help overlay with all bindings and safety notes.
- Resize reflows immediately; viewports are resized, not recreated.

### 3.2 Timings tab

Sub-views: `Timings | SPD | IMC`, selected with `[`/`]`.

```
TIMINGS                     detected   draft     delta   safety
General
  Channels                  2          2          -       ok
  DRAM Ratio                12         12         -       ok
  DRAM Freq (MT/s)          1600       1800       +200    ok
Primary
  tCL                       11         10         -1      ! below JEDEC 11
  tRCD                      11         11         -       ok
  tRP                       9          8          -1      x min 9 (JEDEC)
  tRAS                      28         28         -       ok
Secondary / voltage
  Command Rate              2T         2T         -       ok
  tRFC                      160        160        -       ok
  DRAM Voltage (V)          1.35       1.35       -       ok
```

- Columns are fixed-width and aligned; long notes truncate with an ellipsis.
- `detected` comes from hardware/SPD once, `draft` is editable, `applied` is
  loaded from the state dir. `delta` is draft minus detected, with units.
- Validation returns issues with severity, message, and margin:
  - danger: timing below minimum, voltage outside safe range.
  - warning: above soft maximum, near minimum, derived-rule violation.
  - info: no action.
- Derived values are live: ns for every timing, `tRAS >= tCL + tRCD`, tRFC ns,
  peak/sustained bandwidth, CAS latency, deltas vs applied.
- Ratio lock scales primary timings once per frequency commit, from the
  previous frequency, not from the original hint on every keystroke.
- Apply (`Ctrl+S`) opens a review modal with the diff, warnings, and the
  profile-only disclaimer. Enter confirms; Esc cancels.
- `Ctrl+R` resets draft to applied; per-field Esc reverts the selected field.
- SPD view lists every parsed field, grouped and scrollable. No invented data.
- IMC view renders every detected channel's register-derived timings.
- Empty/error states are explicit (no silent `?` pretending to be data).

### 3.3 Benchmark tab

```
Run    Duration [30s]   Buffer [256M]   Workers [1]    [Enter] run  [x] cancel
       Running 00:18 / 00:30   read 21.4 GB/s   write 18.1 GB/s
Results
  Metric        Last      Prev      Delta     Theory    Eff     Bar
  read64        21400     20800     +2.9%     25600     83.6%   ####----
  ...
History (last 10)  [h] toggle   [e] export selected
```

- Configurable duration and buffer; arguments are validated and passed as an
  argv slice, never a shell string.
- Live elapsed/remaining, spinner, streamed log (follow toggle, scroll).
- Results compare last vs previous run and against theoretical peak.
- Every completed run appends to history with a config snapshot, timestamp,
  exit code, parsed result, and raw log path.
- Missing `stress-ng`, non-zero exit, and parse failure are surfaced with the
  raw log, not a bare status string.

### 3.4 Burn-in tab

```
Preset [memory-heavy]  Duration [30m]  Stop on error [on]   [Enter] run  [x] cancel
RUNNING  elapsed 12:34 / 30:00   errors 3 (!)   log: ~/.local/state/.../log
[################--------------------] 41%
Live log (follow: on, filter: all)
  ...
```

- Preset + duration materialize a job file in the state dir with the `timeout`
  line replaced; the repo job file is never modified at runtime.
- Live log is a real viewport with follow/filter/scroll.
- Error count is parsed from output and exit status; success requires exit 0
  and zero detected errors.
- Errors trigger a prominent banner and optionally cancel the run.
- Cancel kills the whole process group; no orphaned `stress-ng`.
- Result and log path persist in history; the summary remains visible after
  completion.

## 4. Target architecture

```
cmd/mem-station/main.go          flags, headless subcommands, program setup
internal/
  app/                           root model, routing, keymap, layout, messages
    model.go  update.go  view.go  keys.go  messages.go  layout.go
  ui/                            panel, table, footer, help, modal, progress, clip
  sysinfo/                       timing editor, fields, validation, SPD, IMC, perf
  benchmark/                     benchmark tab, config, results, history view
  burnin/                        burn-in tab, presets, live log, results
  config/                        Snapshot, Profile, field keys (leaf)
  runner/                        process execution, streaming, cancel, events
  store/                         XDG state, profiles, history, job materialization
  memory/                        dmidecode/decode-dimms/SPD + MCHBAR (linux build tag)
  bench/                         stress-ng memrate parser (leaf)
  burninparse/                   rename of old burnin parser (leaf) OR keep burnin/
  style/                         semantic theme tokens and shared styles
```

Notes:
- The old `internal/model` package is renamed to `internal/app`.
- The old `internal/burnin` parser stays as a leaf package; the TUI tab lives in
  `internal/burnin` only if that does not import itself. If needed, rename the
  parser to `internal/burnin/parse` and keep the tab at `internal/burnin`.
- `internal/memory` absorbs root `mchbar.go` so `go build ./...` works.
- The import graph is acyclic:
  `style, config, bench, burnin/parse, memory` are leaves;
  `store` imports config;
  `sysinfo, benchmark, burnin` import config/store/runner/style/ui;
  `app` imports the tabs, runner, memory, style, ui.

## 5. Cross-cutting improvements

- Central keymap with editing/modal gating and a real help overlay.
- Responsive layout with exact-height bodies and viewport scrolling.
- Async detection with loading, error, retry, and sudo guidance.
- Shared job runner: streaming, progress ticks, cancellation, process groups,
  raw log files, exactly-one terminal event.
- Persistence: active profile, named profiles, run history, logs, exports.
- Theme: adaptive dark/light, `NO_COLOR`, semantic tokens, non-color symbols.
- Tests (written, not executed in the implementation session): parser
  fixtures, validation tables, store round-trips, key gating, golden views at
  80x24 and 120x40, runner cancel integration.
- CI workflow for gofmt, vet, tests, and `go build ./...`.

## 6. Phases

| Phase | Goal | Main deliverables |
| --- | --- | --- |
| 0 | Stabilize | Fix streaming/viewport/keys/quit, async detect, bounds, build; introduce runner and store skeleton |
| 1 | App shell | Rename to app, split files, responsive layout, keymap/help/footer, tab packages, job state machine |
| 2 | Timings | Typed fields, detected/draft/applied, validation, apply modal, SPD/IMC subviews, perf deltas |
| 3 | Runs | Bench and burn-in config, live progress/log, cancel, history comparison, persistence |
| 4 | Polish | Theming/accessibility, CLI subcommands/export, README, tests, CI, final consistency pass |

Each phase is implemented by a separate worker on this branch, commits its
work, and writes `notes/handoffs/phaseN.md`. Later phases build on the previous
handoff. Verification commands are intentionally not run.

## 7. Non-goals

- Writing timings to hardware, MSR, or BIOS.
- New third-party dependencies (stdlib + already-vendored Charm packages only).
- Rewriting `stress-ng` job content beyond duration substitution.
- Bubble Tea v2 migration.
- Network features, telemetry, or accounts.
- Changing benchmark math to unsupported platform-specific models.

## 8. Follow-up

Verification and acceptance are described in `notes/verification-plan.md` and
must be executed in a later session before merge. The branch is not expected to
be merge-ready without that pass.
