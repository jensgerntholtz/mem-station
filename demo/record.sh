#!/usr/bin/env bash
#
# Record the demo GIFs locally.
#
#   ./demo/record.sh              # every tape in demo/
#   ./demo/record.sh bench.tape   # just one
#
# This is a developer convenience, not CI. It builds the TUI, seeds a throwaway
# state directory, puts demo/bin first on PATH (a deterministic stress-ng
# stand-in), and runs the tapes through VHS. Nothing outside demo/.build,
# demo/.state, demo/.tools and the .gif files is touched.
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT=$PWD
TOOLS="$ROOT/demo/.tools"
BUILD="$ROOT/demo/.build"
STATE="$ROOT/demo/.state"
BIN="$BUILD/mem-station"

# Demo tooling lives under demo/.tools so no system install is needed.
export PATH="$ROOT/demo/bin:$TOOLS:$PATH"

need() {
	command -v "$1" >/dev/null 2>&1 || {
		echo "demo: $1 is required" >&2
		return 1
	}
}

if ! need vhs; then
	cat >&2 <<EOF
Install it into demo/.tools (no system change):

  GOBIN="$TOOLS" go install github.com/charmbracelet/vhs@latest
EOF
	exit 1
fi

if ! need ttyd; then
	cat >&2 <<EOF
vhs needs ttyd. Install it with your package manager, or fetch the static
binary into demo/.tools:

  case "\$(uname -m)" in
    x86_64)  asset=ttyd.x86_64 ;;
    aarch64) asset=ttyd.aarch64 ;;
  esac
  curl -sSL -o "$TOOLS/ttyd" \\
    "https://github.com/tsl0922/ttyd/releases/latest/download/\$asset"
  chmod +x "$TOOLS/ttyd"
EOF
	exit 1
fi

if ! need ffmpeg; then
	echo "demo: ffmpeg is required" >&2
	exit 1
fi

echo "==> building $BIN"
mkdir -p "$BUILD"
go build -o "$BIN" ./cmd/mem-station

echo "==> seeding $STATE"
rm -rf "$STATE"
MEM_STATION_STATE_DIR="$STATE" go run ./demo/seed

export MEM_STATION_STATE_DIR="$STATE"
# The tapes launch the binary from the repo root, so the repository's
# burnin-memory.job is discovered as a selectable preset.
unset MEM_STATION_JOB

if [[ $# -eq 0 ]]; then
	set -- demo/*.tape
fi

for tape in "$@"; do
	[[ -f $tape ]] || tape="demo/$tape"
	if [[ ! -f $tape ]]; then
		echo "demo: no such tape: $tape" >&2
		exit 1
	fi
	echo "==> recording $tape"
	vhs "$tape"
done

echo "==> done"
ls -1 "$ROOT"/demo/*.gif 2>/dev/null || true
