# Phase 1 handoff

- Commit: (this commit) `tui: phase 1 - app shell, tabs, keymap, layout`
- Goal: split the app from the tab packages, make the shell responsive and
  modal, and give every tab the same lifecycle contract.

## Delivered

New packages:

- `internal/app/` - root shell, package `app`:
  - `app.go`: `Options`/`DefaultOptions`, `Tab` interface, `Model`, `New`.
  - `update.go`: global key ownership, detection/status/job-event/tick routing,
    5 s status auto-clear (errors persist), `ensureTick` (500 ms elapsed ticker).
  - `view.go`: header (version, SPD summary, dirty/running markers), numbered
    tab bar with markers, help overlay, too-small screen, fixed footer.
  - `layout.go`: `frame()` geometry, `minWidth/minHeight`, `twoColumnMin`.
  - `keys.go`: global `KeyMap` (`q`, `?`, `R`, `f1-f3`/`ctrl+1..3`).
  - `messages.go`: `detectResultMsg`, `imcResultMsg`, `clearStatusMsg` and the
    async `detectCmd`/`imcCmd` constructors.
  - `layout_test.go`: exact-size, too-small, help overlay, two-column, all-tab,
    sub-view, editing-gate, quit-gate, and footer-status tests.
- `internal/msgs/` - `JobEvent`, `Tick`, `WaitEvent`, `TickEvery`. Shared by the
  shell and tabs so no app <-> tab import cycle exists.
- `internal/sysinfo/` - Timings tab: `model.go` (state, detection merge,
  snapshot, baseline), `update.go` (keys, editing, ratio rescale, apply, views),
  `view.go` (editor, guidance, read-only panel), `bounds.go` (safe ranges),
  `spd.go` (all SPD fields), `imc.go` (turnaround matrix + registers),
  `keys.go`, `sysinfo_test.go`.
- `internal/benchmark/` - Benchmark tab: `model.go`, `update.go` (run/cancel via
  `runner`, event handling, state machine), `view.go` (controls, progress,
  results vs theoretical peak, live log), `keys.go`.
- `internal/burnin/` (package `burnin`, now the tab) - `model.go`, `update.go`,
  `view.go`, `keys.go`, `job.go` (preset discovery: `--job`,
  `MEM_STATION_JOB`, `./burnin-memory.job`, executable-adjacent).
- `internal/burnin/parse/` - leaf parser (already present; error counting now
  ignores "0 errors"/"no errors detected" phrasing).
- `internal/ui/` - `ClipLines`, ANSI-safe `Truncate` (uses
  `charmbracelet/x/ansi`), `PadRight`, `JoinColumns`, `Panel`, `Footer`,
  `HelpOverlay(At)`, `Modal`, `Progress`, `NewSpinner`, `DurationLabel`,
  `AppendLine`, `Level`/`Status`/`KeyHint`/`HelpSection`.
- `internal/style/` - semantic `Theme`, `Default(dark)`, `Set`, `ActiveTheme`.
- `internal/runner/state.go` - `State` (Idle/Running/Done/Failed/Cancelled).

Changed:

- `cmd/mem-station/main.go` - flags `-job`, `-tab`, `-dark`, `-version`;
  `style.Set` in `app.New`; mouse only via `Options.Mouse`.
- `internal/model/` - **deleted** (replaced by `internal/app` + tabs).
- `internal/burnin/burnin.go` - **deleted** (parser moved to
  `internal/burnin/parse`).

## Contracts added/changed

- `app.Tab`: `Init`, `Update(tea.Msg) tea.Cmd`, `View(w,h int) string`,
  `Title`, `Keys() []ui.KeyHint`, `Help() []ui.HelpSection`, `Editing`,
  `ModalOpen`, `Running`, `Dirty`, `Wants(runner.Kind)`, `Blur`, `Stop`.
- `sysinfo.Model` no longer owns detection; the shell calls `SetDetected(vals,
  msg, spd)` and `SetIMC(timings, err)`. Detection now passes
  `*memory.SPDInfo` directly; the duplicated `spdInfo` struct is gone.
- Tabs report status through `ui.Status`/`ui.StatusMsg` only.
- `ui.Panel`/`HelpOverlay` render **exactly** `w x h` cells: panel width/height
  include the border, title occupies one interior row.
- Layout math: `bodyH = height - 3` (header 1, tab bar 1, footer 1); panels are
  sized to the body, so the footer can never be pushed off screen.

## Deviations from brief

- `[`/`]` sub-views (Timings/SPD/IMC) landed already in Phase 1 instead of
  Phase 2, because stacking SPD+IMC under the editor clipped panel borders at
  80x24.
- Apply is immediate (`ctrl+s` saves the active profile) instead of the Phase 2
  review modal. The focusable Apply/Cancel buttons were dropped in favour of
  `ctrl+s`/`ctrl+r`, matching the Phase 1 keymap in the brief.
- Fields are still label-keyed (`"CAS (tCL)"` -> `config.KeyCL`); the typed
  `FieldID` model is Phase 2.
- The two-column breakpoint uses body width (window width - 2) >= 110.
- Verification was re-enabled for this phase at the owner's request: `go build
  ./...`, `go vet ./...`, `go test ./...` pass and `gofmt -l .` is clean. The
  TUI was not launched interactively and `stress-ng` was not executed.

## Known risks / unverified areas

- Rendering is verified headlessly (exact cell counts, content presence) in
  `internal/app/layout_test.go`; no human has looked at it in a real terminal.
- Benchmark/burn-in streaming, cancel, and viewport scrolling are untested
  against a real `stress-ng` run.
- Burn-in error counting is still a regex heuristic (mitigated for "0 errors").
- Runner requires draining events until `EventFinished`; the shell routes
  events to the owning tab only, by `Kind`.
- IMC read still needs root + `iomem=relaxed`; failures show in the IMC view.
- The elapsed ticker runs only while a tab reports `Running()`.

## Exact remaining work for Phase 2

Per `notes/impl-brief.md` §4: typed fields (`internal/sysinfo/fields.go` with
`FieldID`/`Field`/`Groups`), explicit detected/draft/applied state,
`validate.go` with `Issue{FieldID,Severity,Message,Delta}`, derived rules
(tRAS >= tCL+tRCD, tRFC ns, tRC), margins in cycles/ns/V, apply review modal
with diff, renderer with Detected/Draft/Delta/Safety columns, derived perf
panel, SPD/IMC scroll position retention, and view/validation tests.

## Commands intentionally not run

The TUI itself (no interactive terminal session) and `stress-ng` (never
executed). No hardware writes exist in the codebase.
