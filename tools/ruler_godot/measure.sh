#!/usr/bin/env bash
# Same /proc + X11 sampler as the Flutter ruler, applied to bullet_shower.
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../.." && pwd)
WORK=${GODOT_WORK:-/tmp/opencode/godot_ruler}
VERSION=${GODOT_VERSION:-4.5.1-stable}
BIN=${GODOT_BIN:-$WORK/Godot_v${VERSION}_linux.x86_64}
DEMO=${GODOT_DEMO:-$WORK/bullet_shower}
BULLETS=${GODOT_BULLETS:-500}
DURATION=${DURATION:-60}
WARMUP=${WARMUP:-5}
OUT=${1:-${GODOT_OUT:-/tmp/opencode/ruler_runs/godot_${BULLETS}}}

if [[ ! -x "$BIN" || ! -f "$DEMO/project.godot" ]]; then
  echo "ruler not prepared; run $HERE/setup.sh first" >&2
  exit 2
fi

exec python3 "$ROOT/tools/ruler_flutter/scripts/measure.py" \
  --app "$BIN" \
  --arg=--path --arg="$DEMO" \
  --arg=--resolution --arg=1200x800 \
  --arg=--rendering-method --arg=gl_compatibility \
  --env "GODOT_BULLETS=$BULLETS" \
  --window-match "Bullet Shower" \
  --duration "$DURATION" --warmup "$WARMUP" --out "$OUT"
