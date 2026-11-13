# MemStation

A Linux terminal workspace for memory overclocking enthusiasts: inspect timings,
sketch a profile, measure bandwidth, and burn in a memory configuration.

**Profile only.** MemStation never writes MSRs, BIOS, or hardware timings. An
apply saves a profile file; the command line says so, and the apply review says
so.

**Note:** this project is in early development. Verification of the TUI and of
real `stress-ng` runs is pending; see `notes/verification-plan.md`.

Current build is only verified on

OS: CachyOS x86_64  
Kernel: Linux 7.0.1-1-cachyos  
CPU: Intel(R) Core(TM) i5-4670K (4) @ 4.00 GHz  

<img width="1282" height="719" alt="image" src="https://github.com/user-attachments/assets/171b24f0-f9ce-48af-8bba-4a35eba3275e" />

## Requirements

- Linux (the IMC reader is `//go:build linux`)
- Go toolchain matching `go.mod`
- `stress-ng` for benchmark and burn-in runs (the TUI and the headless commands
  report a clear error when it is missing)
- `dmidecode` and/or `decode-dimms` for hardware detection; both usually need
  root, and the IMC register view additionally needs `iomem=relaxed` on the
  kernel command line
- A terminal of at least 80x24

Detection failing is not fatal: the app starts immediately, shows what it could
not read, and allows manual entry. When detection needs root, press `E` in the
TUI to re-run it through `sudo` without restarting: the program releases the
terminal, `sudo` prompts for a password, and the privileged result is loaded
back into the Timings tab. `R` retries unprivileged detection.

## Build and run

```bash
go build -o mem-station ./cmd/mem-station
./mem-station              # terminal UI, Timings tab
./mem-station --tab burn-in
```

`go run ./cmd/mem-station` works too.

### TUI flags

| Flag | Meaning |
| --- | --- |
| `--tab NAME\|N` | initial tab: `timings` (0), `benchmark` (1), `burn-in` (2) |
| `--dark[=false]` | dark theme (default `true`); `--dark=false` selects the light palette |
| `--no-color` | disable color (sets `NO_COLOR`/`CLICOLOR=0` before the first paint) |
| `--mouse` | enable mouse wheel scrolling (cell motion only) |
| `--state-dir DIR` | state directory, overrides `MEM_STATION_STATE_DIR` |
| `--job PATH` | burn-in job file, overrides `MEM_STATION_JOB` |
| `--version` | print the version and exit |

`NO_COLOR=1` and `CLICOLOR=0` are honoured without any flag. State is never
conveyed by color alone: severities and verdicts carry `ok`, `?`, `!`, and `x`
markers.

## Keys

Global keys work whenever no text input or modal owns the keyboard.

| Key | Action |
| --- | --- |
| `F1`/`F2`/`F3`, `ctrl+1..3` | switch tabs from anywhere |
| `1`/`2`/`3` | switch tabs when not editing |
| `?` | help overlay; `esc` closes it |
| `R` | retry hardware detection (unprivileged) |
| `E` | elevate: re-run detection through `sudo`, then reload it |
| `ctrl+c` | cancel a running job, otherwise quit |
| `q` | quit when not editing and no overlay is open |

### Timings

| Key | Action |
| --- | --- |
| `↑`/`↓`, `j`/`k` | move between settings |
| `enter` | edit the selected value |
| `tab`/`shift+tab` | commit and move to the next/previous setting |
| `esc` | cancel the edit; outside an edit it falls back through the views |
| `l` | ratio lock: a frequency change rescales the primary timings once |
| `ctrl+s` | review the diff, then `enter` saves the active profile |
| `ctrl+r` | reset the draft to the applied values |
| `[`/`]` | Timings, SPD, and IMC views |
| `ctrl+p` | save the draft as a named profile |
| `ctrl+l` | load a named profile into the draft |
| `ctrl+e` | export the active profile JSON to a path you type |
| `ctrl+o` | import a profile JSON file into the draft (validated) |

Named profiles, imports, and exports only ever change the draft. Applying is the
only step that writes the active profile, and it never touches hardware.

### Benchmark

| Key | Action |
| --- | --- |
| `enter` | run (or run again) |
| `x` | cancel the run (kills the process group) |
| `↑`/`↓` | select duration, buffer, or workers |
| `←`/`→` | change the selected value; "custom" opens a duration input |
| `f` | toggle log follow |
| `e` | export the last run JSON into the state dir |
| `pgup`/`pgdown` | scroll the log |

### Burn-in

| Key | Action |
| --- | --- |
| `enter` | run (or run again) |
| `x` | cancel the run (kills the process group) |
| `↑`/`↓` | select preset, duration, or stop-on-error |
| `←`/`→` | change the selected value |
| `e` | toggle the errors-only log filter |
| `f` | toggle log follow |
| `s` | export the last result JSON into the state dir |
| `pgup`/`pgdown` | scroll the log |

## Headless commands

The same hardware/runner/store code powers scriptable commands. Human output goes
to stdout, diagnostics to stderr, and `-json` prints exactly one JSON document on
stdout.

```bash
mem-station info                       # detected values, SPD, IMC
mem-station info -json

mem-station bench -duration 30s -buffer 256M -workers 1
mem-station bench -duration 30s -json
mem-station bench -duration 30s -json -out ./bench.json

mem-station burnin -duration 30m -preset memory-heavy
mem-station burnin -duration 5m -job ./my.job -json -out ./burnin.json
```

Exit codes: `0` success (a passed run for `burnin`, a parsed run for `bench`),
`1` the run failed, `2` usage error, `130` interrupted.

`SIGINT`/`SIGTERM` cancel the running job's process group and still report the
runner's terminal state, so a script sees `cancelled` rather than a hang.

Headless runs record the *applied* profile as their configuration and never run
hardware detection; use `mem-station info` for detection. `bench` and `burnin`
print the log path they wrote; `-out PATH` additionally writes the same JSON
document to a path you choose.

## State directory

Resolved in this order: `MEM_STATION_STATE_DIR`, `$XDG_STATE_HOME/mem-station`,
`~/.local/state/mem-station`. The directory is created `0700`; files are `0600`.

```
<state>/
  profiles/active.json     applied profile
  profiles/<name>.json     named profiles (ctrl+p)
  history.jsonl            append-only run history (TUI and headless)
  exports/*.json           exported profiles and runs
  jobs/burnin-memory.job   materialized burn-in job with the chosen timeout
  logs/                    full output of every run
```

The repository's `burnin-memory.job` is never modified: the TUI and the headless
command write a copy with the chosen `timeout` into the state directory. The
embedded preset (`internal/burnin/presets/memory.job`) is the canonical job for
TUI runs; `--job`/`MEM_STATION_JOB` adds a custom job as a second preset.

## Development

```bash
gofmt -l .          # must print nothing
go vet ./...
go build ./...
go test ./...
go test -tags integration ./internal/runner   # launches /bin/sh only
go test ./... -run TestGolden -update         # rewrite golden views
```

CI runs gofmt, vet, build, and the default test suite on `ubuntu-latest` with no
root and no hardware.

Layout: `internal/app` is the shell (layout, global keys, routing); tabs live in
`internal/sysinfo`, `internal/benchmark`, and `internal/burnin` and never import
`internal/app`. Shared leaves: `config`, `runner`, `store`, `style`, `ui`,
`msgs`, `memory`; `internal/cli` implements the command line.
