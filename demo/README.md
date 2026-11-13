# Demo recordings

VHS tapes for the README GIFs, plus a local script that records them.

This is a developer convenience. Nothing here runs in CI.

```bash
./demo/record.sh              # record every tape
./demo/record.sh bench.tape   # record one
```

Output lands in `demo/*.gif`. The GIFs are generated files and gitignored by
default; the one exception is `demo/frontpage.gif`, which the README embeds, so
re-recording `frontpage.tape` changes a tracked file. Publish any other demo
with `git add -f demo/<name>.gif`.

## What the tapes show

| Tape | Content |
| --- | --- |
| `frontpage.tape` | The README hero: Timings workspace into the apply review, launched while hidden so the first frame is the app. Committed as `demo/frontpage.gif`. |
| `timings.tape` | Semantic arrow-key steps, the ratio lock rescaling timings, the apply review overlay, the SPD/IMC views, the help overlay |
| `bench.tape` | Benchmark run: live log, parsed results with deltas, history |
| `burnin.tape` | Preset cycling, a burn-in run, the pass verdict and error counter |
| `pickers.tape` | `ctrl+l` profile picker with fuzzy filtering, `ctrl+o` JSON browser |

## Requirements

- Go (to build the TUI and the seeder)
- `ffmpeg`
- `vhs` and `ttyd`

Both demo tools can live in `demo/.tools/` (gitignored) so nothing is installed
system-wide; the script puts that directory on `PATH`:

```bash
GOBIN="$PWD/demo/.tools" go install github.com/charmbracelet/vhs@latest

case "$(uname -m)" in
  x86_64)  asset=ttyd.x86_64 ;;
  aarch64) asset=ttyd.aarch64 ;;
  *) echo "pick a ttyd asset for $(uname -m)"; exit 1 ;;
esac
curl -sSL -o "$PWD/demo/.tools/ttyd" \
  "https://github.com/tsl0922/ttyd/releases/latest/download/$asset"
chmod +x "$PWD/demo/.tools/ttyd"
```

If `vhs`/`ttyd` are already on your `PATH`, the script uses those instead.

## How it works

`record.sh`:

1. builds the TUI to `demo/.build/mem-station`;
2. wipes `demo/.state` and reseeds it with `demo/seed` (one applied profile and
   three named profiles), so every recording starts from the same values;
3. puts `demo/bin` and `demo/.tools` first on `PATH`;
4. runs each tape through `vhs`.

`demo/bin/stress-ng` is a stand-in for the real `stress-ng`. A real burn-in runs
for 30 minutes and a real memrate benchmark needs the actual binary, so the
stand-in prints the same line shapes the parsers expect, paced over ~3.5s, and
exits 0. It exists only on the demo `PATH`.

The tapes run from the repository root, so the repository's
`burnin-memory.job` is discovered as a selectable preset.

## Caveats

- Detection needs root (`dmidecode`/`decode-dimms`), so the unprivileged demo
  shows "Detection unavailable" with the `E` elevate hint. Everything else —
  the applied profile, margins, guidance, benchmark, burn-in — is real.
- The tapes fix the terminal grid with `Set Columns 120` / `Set Rows 40`, which
  triggers the two-column layout. Change those to record a narrow demo; the TUI
  needs at least 80x24.
- `Set PlaybackSpeed 1.2` compresses idle time. Raise it if a tape feels slow.
