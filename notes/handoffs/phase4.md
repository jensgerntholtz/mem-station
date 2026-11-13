# Phase 4 handoff

- Commit: (this commit) `tui: phase 4 - theme, CLI, profiles, tests, CI`
- Goal: make the branch reviewable and operationally complete: one semantic
  theme everywhere, a real command line (TUI + headless), profile import/export,
  a shared golden harness, CI, and docs. The manual verification matrix is still
  unexecuted.

## Delivered

New:

- `internal/cli/` - the command line. `cli.go` (dispatch, exit codes, injected
  `Deps`, usage), `tui.go` (TUI flags, `--tab/--dark/--no-color/--mouse/
  --state-dir/--job/--version`, `parseTab`), `info.go` (detection/SPD/IMC, human
  and JSON), `bench.go` (+`-duration -buffer -workers -json -out`), `burnin.go`
  (+`-duration -preset -job -json -out`), `run.go` (event drain with
  context-cancel, bounded output buffer, `jobResult`), `record.go` (record
  building, applied-profile config snapshot, best-effort history append),
  `cli_test.go` (fake `runner.StartFunc`, no process launched).
- `cmd/mem-station/main.go` - now three lines: `os.Exit(cli.Run(...))`.
- `internal/uitest/uitest.go` - shared golden harness: `StripANSI`, one
  `-update` flag per test binary, `Golden`, `GoldenView` (exact-size + golden).
- `internal/app/view_test.go` + `internal/app/testdata/*.golden` - shell goldens:
  timings at 80x24 and 120x40, help overlay at both sizes, apply review, profile
  prompt, too-small screen.
- `internal/app/keys_test.go` - global key-gating table (idle/editing/modal/help)
  and mouse-wheel gating.
- `internal/runner/runner_integration_test.go` (`//go:build integration`) -
  output+log tee, exit code, missing binary, cancel kills the process group.
  Uses `/bin/sh` only. Not run in this phase (compiled with
  `go vet -tags integration`).
- `.github/workflows/ci.yml` - gofmt, `go vet ./...`, `go build ./...`,
  `go test ./... -count=1` on `ubuntu-latest`; no root, no hardware.

Changed:

- `internal/style/style.go` - rewritten around `Theme` only. The legacy exported
  style vars (and their dead color constants) are gone; renderers now call
  `style.Section()`, `ColumnHeader()`, `Muted()`, `Dim()`, `FieldLabel()`,
  `SelectedField()`, `EditingField()`, `DetectedValue()`, `ValueText()`,
  `Accent()`, `Info()`, `Metric()`, `OK()`, `Success()`, `Warning()`,
  `Danger()`, `RunButton()`, `RunningButton()`, `TabActive()`, `TabInactive()`.
  New `UsePlainOutput()` sets `NO_COLOR`/`CLICOLOR=0` (called by `--no-color`).
- `internal/ui/ui.go` - `ui.MarkOK/MarkInfo/MarkWarn/MarkError`,
  `ui.SeverityMark(Level)`; the footer prefixes non-info statuses with its
  marker; hint/help/modal text uses the theme helpers.
- `internal/benchmark/view.go`, `internal/burnin/view.go`,
  `internal/sysinfo/view.go|spd.go|imc.go`, `internal/app/view.go` - migrated off
  the legacy style vars; run state labels and the burn-in error banner carry
  `ok`/`!`/`x` markers next to their words.
- `internal/benchmark/history.go` - `payloadOf` is now the exported
  `NewPayload(opts, results)` so the headless command shares the schema.
- `internal/burnin/history.go` - verdicts exported (`VerdictPassed/Failed/
  Cancelled`) and `NewPayload(...)` exported for the same reason.
- `internal/sysinfo/{model,update,view,keys}.go` - profile prompts: `ctrl+p` save
  the draft as a named profile, `ctrl+l` load a named profile into the draft,
  `ctrl+e` export the active profile to a typed path, `ctrl+o` import a validated
  profile file into the draft. One prompt modal (`modalSaveProfile` etc.) with
  hint text, inline errors that keep the modal open, and `esc` as a no-op exit.
  `Stop()` closes any modal.
- `internal/config/config.go` - `config.Keys` and `config.IsKey` (import
  allow-list).
- `internal/store/store.go` - `ExportPath`, `ExportFile(path, v)` (atomic, 0600,
  creates parents), `ImportProfile(path)` (parses, requires values, rejects
  unknown keys, derives a name from the file stem).
- `internal/app/update.go` - `tea.MouseMsg` handling: wheel up/down map to
  `pgup`/`pgdown` for the active tab (and scroll help), only when `--mouse` is
  set and no editor/modal owns the keys. Clicks are ignored.
- `internal/benchmark/benchmark_test.go`, `internal/burnin/burnin_test.go` - use
  `internal/uitest`; local `checkGolden`/`ansiRe`/`-update` duplicates removed.
- `internal/{benchmark,burnin}/testdata/*.golden`, new
  `internal/app/testdata/*.golden` - regenerated for the severity markers and the
  new Profiles help section.
- `internal/store/store_test.go`, `internal/sysinfo/sysinfo_test.go` - import/
  export round-trips, validation rejections, named-profile save/load, prompt
  errors.
- `README.md` - rewritten: features, requirements, flags, every key, headless
  commands and exit codes, state-directory layout, job-file rules, safety text,
  development commands.
- `notes/verification-plan.md` - status updated honestly (automated checks pass,
  manual matrix unexecuted), exact command list incl. `-tags integration` and
  golden `-update`, profile/export and color checks added to the manual matrix,
  stale "code was never built" risk rewritten.

## Contracts added/changed

- `internal/cli.Deps{Stdout, Stderr, Detect, ReadIMC, Start, RunTUI, Ctx,
  Version}` + `cli.Run(args, deps) int` with `ExitOK/ExitFailed/ExitUsage/
  ExitSigint` (0/1/2/130). `app.Options` is the TUI contract; `Deps.RunTUI` takes
  it, so tests never open a terminal.
- Headless JSON reuses the tab export schemas: `benchmark.Export`/`Payload` and
  `burnin.Export`/`Payload` (`benchmark.NewPayload`, `burnin.NewPayload`,
  `burnin.Verdict*`).
- `store.ExportPath`, `store.ExportFile`, `store.ImportProfile`,
  `config.IsKey`/`config.Keys`.
- `style` API is now theme-derived functions only; there are no exported style
  vars left. `style.UsePlainOutput()` is the only color switch.
- `ui.Mark*` + `ui.SeverityMark`.
- `internal/uitest` is the only golden harness: `-update` is registered once per
  test binary by that package.
- `runner.Handle`/`runner.StartFunc` unchanged from Phase 3; the CLI injects
  `runner.Start` through `Deps.Start`.

## Deviations from brief

- Printing lives in `internal/cli`, not `cmd/mem-station` (§0 rule 3). All writes
  go to injected `io.Writer`s (`Deps.Stdout/Stderr`), the library packages still
  never touch the process's streams, and `cmd/mem-station/main.go` is a three-line
  wrapper. This is what makes the CLI testable without a process.
- Headless `-out PATH` was added to `bench`/`burnin` so "export the last run JSON
  to a user-specified path" exists. The TUI's `e`/`s` keys still export into
  `<state>/exports/`; adding a path prompt to both run tabs was left out to keep
  this phase reviewable (the profile export does prompt for a path).
- `--tab` accepts names as well as 0-2; `-preset` accepts a name or index of the
  embedded/explicit presets rather than a path (paths use `-job`).
- Headless runs record the applied profile as their config and never run hardware
  detection (`info` is the detection command); the record says
  `config.source: "none"` when no profile has been applied.
- `--no-color` is implemented by setting `NO_COLOR`/`CLICOLOR=0` (what
  lipgloss/termenv read when they resolve the color profile) rather than
  `lipgloss.SetColorProfile(termenv.Ascii)`, which would have made
  `muesli/termenv` a direct dependency. `go.mod`/`go.sum` are unchanged.
- The run tabs' panels keep the unfocused border: `Tab.View(w, h)` has no
  focus argument, so panel focus highlighting still only distinguishes the
  Timings editor from its guidance panel.
- Golden harness lives in `internal/uitest` (a normal package, imported only by
  `_test.go` files) because three packages need the same comparison.

## Known risks / unverified areas

- No interactive TUI run, no `stress-ng` invocation, no hardware detection, and
  no integration-tag run in this phase (the integration test only compiles).
- The new CLI has never been executed against a real `stress-ng`; argv, the
  `--job <materialized>` invocation, signal handling and exit codes are only
  covered by fake-runner tests.
- Contrast of the light palette and of the new `RunningButton`/`SelectedField`
  colors is unverified by eye; `NO_COLOR` support relies on termenv reading the
  environment (asserted only as "the flag sets the variables").
- The `--mouse` path maps wheel events to page keys; smooth scrolling and
  terminal-specific wheel behavior are untested.
- Headless `burnin` always runs with stop-on-error on (the TUI can toggle it);
  the CLI has no flag for it.
- Profile import accepts any value string for a known key and leaves semantic
  validation to the editor's Safety column; an imported `<state>/profiles/
  active.json`-style file with `"source":"apply"` loads as a draft, not applied.
- CI workflow is committed but has never run (first run happens on push).
- `history.jsonl` still has no rotation; the UI loads only the last N.

## Exact remaining work for the next phase

There is no Phase 5 in `notes/impl-brief.md`; this was the last planned phase.
Remaining work is verification and follow-up, in this order:

1. Run `notes/verification-plan.md` §3 (static/automated) including
   `go test -tags integration ./internal/runner` and `go test ./... -race`.
2. Run §4 manually: TUI at 80x24/120x40/160x50, key modality, apply/import/
   export flows, a real 15s benchmark, a 5m burn-in with cancel, `NO_COLOR` and
   `--dark=false`, and the headless commands against a real `stress-ng`.
3. Fix whatever §3/§4 find; regenerate goldens with
   `go test ./... -run TestGolden -update` only for intentional view changes.
4. If desired later: a path prompt for run-tab exports, a `-stop-on-error` flag
   for headless `burnin`, mouse-wheel smoothness, and history rotation.

## Commands intentionally not run

- The TUI was never launched; no benchmark, burn-in, `stress-ng`, or hardware
  detection ran; no MSR/BIOS/hardware write path exists.
- `go test -tags integration ./internal/runner` was not executed (it launches
  `/bin/sh`); only `go vet -tags integration` was run.
- No `go run` smoke test of the CLI; its behaviour is covered by the fake-runner
  tests only.
- Run in this phase: `gofmt -l .`, `go build ./...`, `go vet ./...`,
  `go vet -tags integration ./internal/runner`, `go test ./... -count=1`,
  `go test ./... -race -count=1`.
