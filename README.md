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
  root
- Raw IMC registers are optional. They are read only with `--imc` or the `I`
  key, need root, and may need `iomem=relaxed` on the kernel command line.
  Normal detection uses DMI/SPD and does not open `/dev/mem`.
- A terminal of at least 80x24

Detection failing is not fatal: the app starts immediately, shows what it could
not read, and allows manual entry. When detection needs root, press `E` in the
TUI to re-run it through `sudo` without restarting: the program releases the
terminal, `sudo` prompts for a password, and the privileged result is loaded
back into the Timings tab. `R` retries unprivileged detection.

### Raw IMC access and `iomem=relaxed`

Normal detection never opens `/dev/mem`: it uses `dmidecode` and
`decode-dimms` SPD data. Raw IMC/tertiary timing registers are the only
feature that needs direct MMIO, so they are opt-in (`--imc`, `I`, or
`info -imc`). That path still needs root and, on strict kernels, the
`iomem=relaxed` boot parameter. If you only need primary/secondary timings,
remove `iomem=relaxed` and leave raw IMC access off. There is currently no
unprivileged kernel interface that exposes those memory-controller registers.

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
| `--imc` | read raw IMC registers at startup (root; may need `iomem=relaxed`) |
| `--state-dir DIR` | state directory, overrides `MEM_STATION_STATE_DIR` |
| `--job PATH` | burn-in job file, overrides `MEM_STATION_JOB` |
| `--version` | print the version and exit |

These apply to every command, not just the launcher:

| Flag | Meaning |
| --- | --- |
| `--log-file PATH` | append diagnostics to a file as well as stderr |
| `-v`, `--verbose` | log debug diagnostics (start options, run results) |
| `-q`, `--quiet` | log errors only, e.g. suppress a missing-job warning |

`NO_COLOR=1` and `CLICOLOR=0` are honoured without any flag. State is never
conveyed by color alone: severities and verdicts carry `ok`, `?`, `!`, and `x`
markers.

The command line is built with [fang](https://github.com/charmbracelet/fang), so
`mem-station help`, per-command `--help`, shell completions
(`mem-station completion bash`) and man pages (`mem-station man`) come for free.
Both `--json` and the documented single-dash `-json` work; long flags are
rewritten before parsing.

## Keys

Global keys work whenever no text input or modal owns the keyboard.

| Key | Action |
| --- | --- |
| `F1`/`F2`/`F3`, `ctrl+1..3` | switch tabs from anywhere |
| `1`/`2`/`3` | switch tabs when not editing |
| `?` | help overlay; `esc` closes it |
| `R` | retry hardware detection (unprivileged) |
| `E` | elevate: re-run detection through `sudo`, then reload it |
| `I` | read raw IMC registers on demand (opt-in; root; may need `iomem=relaxed`) |
| `ctrl+c` | cancel a running job, otherwise quit |
| `q` | quit when not editing and no overlay is open |

### Timings

| Key | Action |
| --- | --- |
| `↑`/`↓`, `j`/`k` | move between settings |
| `←`/`→` | decrease/increase the selected numeric value, or cycle preset values |
| `enter` | edit the selected numeric value; preset-only fields use `←`/`→` |
| `tab`/`shift+tab` | commit and move to the next/previous setting |
| `esc` | cancel the edit; outside an edit it falls back through the views |
| `l` | ratio lock: a frequency change rescales the primary timings once |
| `ctrl+s` | review the diff, then `enter` saves the active profile |
| `ctrl+r` | reset the draft to the applied values |
| `[`/`]` | Timings, SPD, and IMC views |
| `ctrl+p` | save the draft as a named profile |
| `ctrl+l` | pick a saved profile (↑/↓, `/` filters, enter loads) |
| `ctrl+e` | export the active profile JSON to a path you type |
| `ctrl+o` | browse for a profile JSON file to import into the draft (validated) |

Named profiles, imports, and exports only ever change the draft. Applying is the
only step that writes the active profile, and it never touches hardware.

`ctrl+l` and `ctrl+o` open pickers rather than asking you to type a name or an
absolute path; `esc` closes either one without changing the draft. The built-in
burn-in preset is compiled in, so `ctrl+e` only has a file to edit when you
start with `--job PATH` (or `MEM_STATION_JOB`).

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
| `ctrl+e` | edit the selected job file in `$EDITOR` (custom jobs only) |
| `pgup`/`pgdown` | scroll the log |

## Headless commands

The same hardware/runner/store code powers scriptable commands. Human output goes
to stdout, diagnostics to stderr, and `--json` prints exactly one JSON document on
stdout. The older single-dash forms (`-json`, `-duration`, `-out`, ...) still
work.

The direct IMC register reader has verified layouts for Intel Sandy Bridge, Ivy
Bridge, Haswell, and Broadwell. Other CPUs still get primary/secondary timings
from DMI/SPD; the app reports the IMC register view as unsupported rather than
guessing at an unknown register layout.

```bash
mem-station info                       # detected values and SPD (no raw registers)
mem-station info -json
mem-station info -json -imc            # opt-in raw IMC registers (root; may need iomem=relaxed)

mem-station bench -duration 30s -buffer 256M -workers 1
mem-station bench -duration 30s -json
mem-station bench -duration 30s -json -out ./bench.json

mem-station burnin -duration 30m -preset memory-heavy
mem-station burnin -duration 5m -job ./my.job -json -out ./burnin.json

mem-station report                     # hardware + profile + history, styled
mem-station report > report.md         # the Markdown source, piped
mem-station report --json              # the same content as one JSON document
mem-station report --imc --history 5   # opt-in registers, five recent runs
```

`report` renders Markdown for a terminal and prints the Markdown source when
stdout is not a terminal, so redirecting it produces a file GitHub or pandoc can
read. `--style` forces a rendering (`raw`, `dark`, `light`, `notty`, `ascii`,
`pink`, `dracula`, `tokyo-night`); `--json` embeds the same documents `info
--json`, the profile store and `history.jsonl` already use, rather than a new
schema. Detection failures are reported in the document instead of failing the
command.

Exit codes: `0` success (a passed run for `burnin`, a parsed run for `bench`),
`1` the run failed, `2` usage error, `130` interrupted. A failed run exits `1`
without an extra error banner, because the verdict is already on stdout; usage
errors are printed by fang on stderr.

`SIGINT`/`SIGTERM` cancel the running job's process group and still report the
runner's terminal state, so a script sees `cancelled` rather than a hang.

`bench` and `burnin` record the *applied* profile as their configuration and
never run hardware detection; `info` and `report` are the detection commands.
`bench` and `burnin` print the log path they wrote; `-out PATH` additionally
writes the same JSON document to a path you choose.

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

`make` is the single entry point; CI runs the same targets.

```bash
make            # gofmt, vet, build, test (what CI runs)
make race       # the suite under -race
make integration # tagged runner tests (launches /bin/sh only)
make goldens    # rewrite golden views, then print the diff summary
```

The raw commands remain available:

```bash
gofmt -l .          # must print nothing
go vet ./...
go build ./...
go test ./...
go test -tags integration ./internal/runner   # launches /bin/sh only
go test ./... -run TestGolden -update         # rewrite golden views
go test ./internal/app -run TestGolden -update  # the same, scoped to one package
```

The `-update` flag only exists in test binaries that link `internal/uitest`, so
`go test ./... -update` fails in packages without golden tests. Use the
`MEM_STATION_UPDATE_GOLDEN=1` environment variable (or `make goldens`) for a
repo-wide update; review the diff before committing.

CI runs gofmt, vet, build, the default test suite and the race suite on
`ubuntu-latest` with no root and no hardware.

`./demo/record.sh` records the demo GIFs locally with VHS; see
[`demo/README.md`](demo/README.md). It is a developer convenience and is not
part of CI.

Layout: `internal/app` is the shell (layout, global keys, routing); tabs live in
`internal/sysinfo`, `internal/benchmark`, and `internal/burnin` and never import
`internal/app`. Shared leaves: `config`, `runner`, `store`, `style`, `ui`,
`msgs`, `memory`; `internal/cli` implements the command line.
