# Phase 0 handoff

- Commit: (see git log) tui: phase 0 - stabilize streaming, detection, bounds, and build
- Goal: remove acute bugs and add the runner/store/config scaffolding without
  the full app split; keep the current layout working.

## Delivered

Created:
- `internal/config/config.go` — leaf `Snapshot`/`Profile` types, canonical field
  keys, `Float/FreqMT/Channels/PeakMBs/CASNs/Clone` helpers.
- `internal/runner/runner.go` — `Start/Events/Cancel/Wait`, `Spec`, `Kind`,
  `EventType`, `Event`, `ErrCancelled`. Merged stdout+stderr via one scanner
  (1 MiB buffer), tees every line to a 0600 log file, emits exactly one
  Started, N Output, one Finished, then closes the channel.
- `internal/runner/proc_unix.go` (`//go:build unix`) — `Setpgid` process groups,
  SIGTERM then SIGKILL after 3 s.
- `internal/runner/proc_other.go` (`//go:build !unix`) — `Process.Signal/Kill`
  fallback.
- `internal/store/store.go` — XDG state dir (`MEM_STATION_STATE_DIR`,
  `$XDG_STATE_HOME/mem-station`, `~/.local/state/mem-station`), profiles
  (active + named, atomic writes), JSONL history (tolerates bad trailing
  lines), `LogPath`, `MaterializeJob` (duration substitution into a fresh
  `jobs/` file, never mutates the source preset).
- `internal/memory/imc.go` — shared exported `IMCTimings` struct (no build tag).
- `internal/memory/imc_linux.go` (`//go:build linux`) — `ReadMCHBARTimings` and
  helpers moved from root `mchbar.go` (the `dumpMCHBARRaw` print helper was
  dropped; library packages must not write stdout/stderr).
- `internal/memory/imc_stub.go` (`//go:build !linux`) — returns `nil, nil`.

Modified:
- `internal/burnin/burnin.go` — rewritten: `ParseBurnInOutput(output, exitCode,
  err) *Results` with `Duration/Errors/ExitCode/LogPath/RawOutput/Success`.
  `fmt.Println` and the blocking `RunBurnInCmd`/`StreamBurnInCmd` removed.
  `Success` = no runner error + exit 0 + zero error lines. `RawOutput` bounded
  to last 2000 lines.
- `internal/model/model.go` — rewritten around Phase 0 fixes (below).
- `cmd/mem-station/main.go` — `--job` flag, removed `WithMouseAllMotion`,
  passes `model.Options{JobPath}`.

Deleted:
- root `mchbar.go` (now `package main` exists only under `cmd/`).

## Phase 0 fixes applied

- Viewport: `resizeBurnInViewport` sizes the burn-in viewport from the window
  on `WindowSizeMsg` and on tab switch (`burnInViewportSize`: width = m.width-12,
  height = m.height-24, min 3).
- Keys: explicit selected-vs-editing. `editing bool` + `editValue`. While
  editing, letters/digits/arrows go to the input (`q` does not quit); `esc`
  restores the pre-edit value and exits editing; a second `esc` (selected mode)
  resets to detected values. Global keys (f1-f3, l, tab, enter, esc, r, q,
  ctrl+c) only run when not editing. `q`/`ctrl+c` cancels any running job first.
- Bounds: `fieldBounds` is now `float64`; voltage 1.35-1.65 V parsed as float;
  below-min timing = danger, above-max = warning; voltage outside range =
  danger; unknown (`?`) values render the range with no severity color.
- Async detection: `InitialModel` no longer blocks; `Init()` returns
  `detectCmd` (dmidecode/decode-dimms + IMC read). `R` retries. Loading status
  shown until the result message lands.
- Jobs: benchmark and burn-in both run through `runner.Start` with a
  `waitEventCmd` bridge; `burnInRunning` is cleared on `EventFinished` and a
  real `burnin.Results` is produced. Apply now says "Profile only - not written
  to hardware" instead of "Applied (mock)".
- Job path: `MEM_STATION_JOB` → `--job` → `./burnin-memory.job` →
  executable-adjacent; materialized into the state dir; default duration 30m
  (labels say 30m, not 30s).

## Contracts added/changed

- `burnin.ParseBurnInOutput` signature changed (was `ParseBurnInOutput(string)
  *BurnInResults`); `BurnInResults` renamed to `Results` with added
  `ExitCode/LogPath/RawOutput`.
- `model.InitialModel` now takes `model.Options` (Phase 1 will replace this with
  `app.Options`).
- `store` and `runner` match the shared contracts in `notes/impl-brief.md` §1.2
  and §1.3.

## Deviations from brief

- Split `IMCTimings` into a shared `imc.go` (no build tag) so the non-linux
  stub and the model can reference the full struct without duplicating ~40
  fields; the function bodies are in `imc_linux.go` as specified.
- `applyDetected` overwrites field values on every detection result (including
  `R` retry) rather than tracking user edits; acceptable for Phase 0, document
  in Phase 2 when draft/applied state lands.
- Kept `bench.RunBenchmarkCmd` (functional blocking helper, still usable by the
  Phase 4 headless CLI); only the TUI path was moved to `runner`.

## Known risks / unverified areas

- No build/test/vet/run performed (owner instruction). Only `gofmt -w` was run
  and it passed.
- Burn-in error-line counting is a heuristic regex (`error|errors|failed|
  failure|fatal|aborted|abort|segfault|out of memory`); a verbose success log
  that happens to contain e.g. "0 errors" would inflate the count. Verify
  against a real stress-ng verbose run later.
- `burnInViewportSize` chrome math (height-24) is approximate; verify the
  footer stays visible at 80x24 and 120x40 in the acceptance pass.
- Runner requires callers to consume events until `EventFinished` (documented);
  `Wait()` without draining events could block on a full buffer.
- IMC read still requires root + `iomem=relaxed`; failures are tolerated (nil).

## Exact remaining work for Phase 1

1. Rename `internal/model` → `internal/app` (package `app`), update
   `cmd/mem-station/main.go`; split into `model.go/update.go/view.go/keys.go/
   messages.go/layout.go`.
2. Create `internal/ui` helpers and the semantic `style.Theme`; replace
   hardcoded layout widths where replaced.
3. Extract `internal/sysinfo`, `internal/benchmark`, `internal/burnin` tab
   packages exposing `Update/View(w,h)/Editing()/ModalOpen()/Running()/
   Title()/Keys()`. Keep `internal/burnin` parser leaf (do not self-import).
4. Root shell: global key ownership, header with dirty/running markers,
   numbered tab bar (F1-F3, Ctrl+1..3 always; plain 1/2/3 only when not
   editing), fixed footer, `?` help overlay, too-small screen (<80x24), exact
   body height, two-column at >=110 cols, viewport resizing.
5. Single `KeyMap`; document field navigation and editing semantics.
6. Shared job state machine (Idle/Running/Done/Failed/Cancelled), elapsed
   ticker, cross-tab event routing.
7. Move detection UI into the async `Init` command path with loading/error
   banner + retry `R`; no panic when SPD/IMC is nil.
8. Remove hardcoded "(30s)" strings; durations come from tab config.
9. Keep all Phase 0 behavior and fixes.

## Commands intentionally not run

`go build`, `go test`, `go vet`, `go run`, the TUI, and `stress-ng` were all
skipped per the owner's explicit instruction. Only `gofmt -w` was run on the
changed Go files.
