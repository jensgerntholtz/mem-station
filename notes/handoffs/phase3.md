# Phase 3 handoff

- Commit: `tui: phase 3 - benchmark and burn-in runs` (this commit; `git log -1`
  has the sha)
- Goal: turn the Benchmark and Burn-in tabs into real, cancellable, observable
  runs with persisted history, so a run's configuration, result, and log path
  survive a restart.

## Delivered

New:

- `internal/benchmark/config.go` - `Options{Duration, Buffer, Workers}`,
  duration (15s/30s/1m/5m/custom), buffer (128M/256M/512M/1G) and worker
  (1..NumCPU) choices, `Validate`, `Args()` (argv slice, never a shell string),
  `Label()`, `Signature()`, the JSON `payloadOptions` form, cycling helpers, and
  the custom-duration parser.
- `internal/benchmark/history.go` - the persisted `Payload` schema, the export
  `Export` envelope, history load/append, the same-config `previous` lookup, and
  the JSON export command.
- `internal/burnin/preset.go` - `//go:embed presets/memory.job`, `Preset`,
  `Presets(customPath)` (embedded memory-heavy preset, then `--job`,
  `MEM_STATION_JOB`, `./burnin-memory.job`, or an executable-adjacent file).
  Replaces the deleted `internal/burnin/job.go`.
- `internal/burnin/presets/memory.job` - the embedded job file copied from the
  committed `burnin-memory.job`.
- `internal/burnin/config.go` - `Options{Duration, StopOnError}`, durations
  (5m/10m/30m/1h/custom), validation, label/signature, cycling helpers, and the
  custom-duration parser (bare numbers are minutes).
- `internal/burnin/history.go` - the burn-in `Payload`/`Export` schemas,
  verdict constants, history load/append, and JSON export.
- `internal/runner/handle.go` - `Handle` (Events/Cancel) and `StartFunc` so the
  run tabs can be driven by a fake job in tests.
- `internal/benchmark/benchmark_test.go`, `internal/burnin/burnin_test.go`,
  `internal/burnin/parse/burnin_test.go`, `internal/store/store_test.go`.

Changed:

- `internal/benchmark/model.go|update.go|view.go|keys.go` - config rows and
  selection, custom duration editing, run/cancel through `runner`, elapsed /
  progress / time-derived spinner, follow + scrollable live log, parsed results
  with last-vs-previous deltas and peak efficiency, a 10-run history table, JSON
  export, and distinct failed/cancelled/parse-failure states that always show the
  log path.
- `internal/burnin/model.go|update.go|view.go|keys.go` - preset + duration +
  stop-on-error config, job materialization into the state dir, live viewport
  with follow and errors-only filter, incremental error counter and banner,
  auto-cancel on the first error when stop-on-error is on, distinct
  passed/failed/cancelled verdicts, persisted results, and a history table.
- `internal/burnin/parse/burnin.go` - exported `IsErrorLine` so the live counter,
  the filter, and the parser share one rule.
- `internal/store/store.go` - `ExportJSON(name, v)` writing
  `<state>/exports/<name>.json` atomically, and the shared `sanitizeName` used by
  profiles and exports.
- `internal/ui/ui.go` - replaced the unused `NewSpinner` with
  `SpinnerFrame(t time.Time)`; the app's 500 ms elapsed tick animates it, so a
  tab needs no ticker of its own.
- `internal/app/update.go` - `ctrl+c` cancels running jobs and stays in the app
  (quitting only when nothing runs); error status banners are sticky and are
  cleared by a new run start or an explicit cancel; refreshed footer hints.
- `internal/app/app.go` - global help mentions the `ctrl+c` cancel behaviour.
- `internal/app/layout_test.go` - inactive-tab event routing, `ctrl+c` cancel
  gating, sticky error banner, and exact-size checks for both run tabs while a
  job is active at 80x24 and 120x40.

## Contracts added/changed

- `runner.Handle` / `runner.StartFunc` are new. Tabs store a `runner.Handle` and
  call an injected factory (`Model.start`) instead of `runner.Start` directly.
  Production wiring is unchanged (`*runner.Job` satisfies `Handle`).
- `store.ExportJSON(name string, v any) (string, error)` is new; exports live in
  `<state>/exports/*.json` with the same atomic-write rules as profiles.
- `ui.SpinnerFrame` replaced `ui.NewSpinner` (which nothing called).
- Run history payload schemas (stable, JSON):
  - benchmark: `{options:{duration_s,buffer,workers}, parsed, read_mbs,
    write_mbs, metrics:[{label,mbs}], parsed_duration}`.
  - burn-in: `{options:{preset,duration_s,stop_on_error}, verdict,
    success, errors, parsed_duration, stopped_on_error}`.
  - Exports wrap a `store.Record`-shaped envelope:
    `{kind, started_at, duration_ms, exit_code, error, log_path, config, result}`.
- App status semantics: `setStatus` ignores non-error statuses while an error
  banner is showing; `resetStatus` clears it on `EventStarted` and on `ctrl+c`.

## Deviations from brief

- Tests drive the tabs through a fake `runner.Handle` instead of executing
  stress-ng. That required the `Handle`/`StartFunc` seam above.
- Spinner is a time-derived glyph rather than `bubbles/spinner`; a
  spinner-owned tick would stall whenever the tab is inactive (the app only
  routes unknown messages to the active tab).
- `ctrl+c` changed from "always quit" to "cancel a running job, else quit"
  (brief §5 task 1) at the app shell, not inside the tab.
- Burn-in export is bound to `s` ("save JSON"); `e` is the errors filter and
  `f` follow in both tabs. The brief did not assign burn-in keys.
- Benchmark history also records cancelled runs (`error: "cancelled"`,
  `parsed: false`); they render with `-` rates and are excluded from
  last-vs-previous deltas.
- Custom duration ranges are bounded (benchmark 1s..2h, burn-in 1m..8h) and bare
  numbers are seconds for benchmark, minutes for burn-in.
- Golden views are rendered for running/done/cancelled in each tab plus a wide
  done view (idle/failed are covered by exact-size and content assertions).
  Each tab package has its own tiny `-update` golden helper; Phase 4 owns a
  shared harness.
- The history table is dropped when the body cannot hold it above a readable log
  (typical at 80x24 with results); nothing is silently truncated mid-row.
- `store.MaterializeJob` keeps its single `jobs/burnin-memory.job` output path;
  presets overwrite each other rather than keeping per-preset copies.

## Known risks / unverified areas

- No interactive TUI run and no `stress-ng` execution in this phase. The argv
  (`--memrate <workers> --memrate-bytes <buffer> --metrics -t <secs>`) and the
  `--job <materialized path>` invocation are unverified against an installed
  stress-ng.
- Buffers/workers are offered as-is; nothing validates them against system RAM.
- Error detection is the existing regex heuristic. A summary line such as
  "unsuccessful run completed ... (N errors)" also matches, so the live counter
  can be one higher than the number of distinct failures (conservative: the run
  is still marked failed). `0 errors`/`no errors` phrasing is excluded.
- Viewport paging, follow behaviour, and spinner smoothness are untested by eye.
- History timestamps render in local time; tests avoid goldening them.
- `history.jsonl` appends are a single write but not atomic by design
  (append-only JSONL tolerates a truncated trailing line, which is tested).
- Golden files encode the current rendering; any styling change needs
  `go test ./internal/benchmark ./internal/burnin -run TestGoldenViews -update`.
- The benchmark tab's live log retains the last 2000 lines; longer runs lose the
  head in the UI (the full log is on disk at the shown path).

## Exact remaining work for Phase 4

Per `notes/impl-brief.md` §6:

1. Theming/accessibility: migrate the new run-tab renderers off the legacy
   `style.RunBtn`/`RunningBtn`/`SectionTitle`/`ColHeader` vars onto `Theme`
   tokens; add symbols/text to every severity cue; honour `NO_COLOR` /
   `CLICOLOR=0` across tabs; verify contrast.
2. CLI: `cmd/mem-station` flags `--tab`, `--dark`, `--no-color`, `--mouse`,
   `--state-dir`, `--job`, `--version`; headless
   `info [-json]`, `bench [-duration] [-buffer] [-workers] [-json]`,
   `burnin [-duration] [-preset] [-job] [-json]` reusing `runner`/`store`;
   SIGINT cancels cleanly; stable JSON on stdout and errors on stderr.
3. Profiles/export: named profiles, import with validation, and export paths
   chosen by the user (today `ExportJSON` always writes into the state dir). The
   benchmark `e` / burn-in `s` keys can stay as the "export to state dir"
   defaults.
4. Docs: README (keys, headless commands, state dir layout, safety text
   "Profile only - not written to hardware"), finalize
   `notes/verification-plan.md` with the manual matrix; keep handoffs current.
5. Tests: shared golden harness with ANSI stripping and `-update`; key-gating
   tables; store atomicity/round-trip consolidation; a runner cancel
   integration test (`//go:build integration`); golden views for overlays and
   the Timings tab at 80x24/120x40.
6. CI: `.github/workflows/ci.yml` with gofmt check, `go vet ./...`,
   `go test ./...`, `go build ./...`; no root/hardware.
7. Final consistency pass: no `fmt.Println` in library packages, no dead exports,
   no stale "(30s)" claims, and re-check the exact-size layout tests.
8. Optional cleanup noted here: the duration-option cycling helpers are
   duplicated between `internal/benchmark/config.go` and
   `internal/burnin/config.go`; consolidating them is safe but not required.

## Commands intentionally not run

The TUI was never launched interactively. No benchmark, burn-in, `stress-ng`, or
hardware detection run was executed, and no MSR/BIOS/hardware write path exists.
`gofmt -l .`, `go build ./...`, `go vet ./...`, `go test ./...`, and
`go test ./... -race` were run (verification is re-enabled since Phase 1).
