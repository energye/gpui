#!/usr/bin/env bash
# Run the Godot 2D ruler window. Extra args are passed to Godot.
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
WORK=${GODOT_WORK:-/tmp/opencode/godot_ruler}
VERSION=${GODOT_VERSION:-4.5.1-stable}
BIN=${GODOT_BIN:-$WORK/Godot_v${VERSION}_linux.x86_64}
DEMO=${GODOT_DEMO:-$WORK/bullet_shower}

if [[ ! -x "$BIN" || ! -f "$DEMO/project.godot" ]]; then
  echo "ruler not prepared; run $HERE/setup.sh first" >&2
  exit 2
fi

exec "$BIN" --path "$DEMO" --resolution 1200x800 \
  --rendering-method gl_compatibility "$@"
