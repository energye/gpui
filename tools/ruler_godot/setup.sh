#!/usr/bin/env bash
# Prepare the Godot 2D comparison ruler without touching the upstream demo.
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../.." && pwd)
WORK=${GODOT_WORK:-/tmp/opencode/godot_ruler}
VERSION=${GODOT_VERSION:-4.5.1-stable}
LINUX=linux.x86_64
DEMO_ROOT=${GODOT_DEMO_ROOT:-$ROOT/../godot-demo-projects}
DEMO=$WORK/bullet_shower

if [[ ! -f "$DEMO_ROOT/2d/bullet_shower/project.godot" ]]; then
  echo "missing upstream demo: $DEMO_ROOT/2d/bullet_shower" >&2
  echo "set GODOT_DEMO_ROOT to a checkout of godotengine/godot-demo-projects" >&2
  exit 2
fi

mkdir -p "$WORK"

if [[ -n "${GODOT_BIN:-}" ]]; then
  BIN=$GODOT_BIN
else
  BIN=$WORK/Godot_v${VERSION}_${LINUX}
  if [[ ! -x "$BIN" ]]; then
    ZIP=$WORK/Godot_v${VERSION}_${LINUX}.zip
    URL="https://github.com/godotengine/godot/releases/download/${VERSION}/Godot_v${VERSION}_${LINUX}.zip"
    echo "downloading $URL"
    curl -L --fail --retry 3 -o "$ZIP" "$URL"
    unzip -q -o "$ZIP" -d "$WORK"
    chmod +x "$BIN"
  fi
fi
if [[ ! -x "$BIN" ]]; then
  echo "Godot binary not executable: $BIN" >&2
  exit 2
fi

if [[ "$VERSION" == "4.5.1-stable" ]]; then
  EXPECTED=db07cae7de644278a1884d4552bdf2bca3f5d30131b18faf3a0c4d730080b199
  GOT=$(sha256sum "$BIN" | awk '{print $1}')
  if [[ "$GOT" != "$EXPECTED" ]]; then
    echo "sha256 mismatch for $BIN" >&2
    echo "expected $EXPECTED" >&2
    echo "got      $GOT" >&2
    exit 2
  fi
fi

rm -rf "$DEMO"
cp -a "$DEMO_ROOT/2d/bullet_shower" "$DEMO"
cp "$HERE/fps_probe.gd" "$DEMO/fps_probe.gd"
python3 "$HERE/patch_demo.py" "$DEMO"

echo "importing $DEMO (headless)"
"$BIN" --headless --import --path "$DEMO" >"$WORK/import.log" 2>&1
if [[ ! -d "$DEMO/.godot/imported" ]]; then
  echo "import failed; see $WORK/import.log" >&2
  tail -30 "$WORK/import.log" >&2
  exit 1
fi

cat <<EOF
Godot ruler ready.
  binary: $BIN
  demo:   $DEMO
  import: $WORK/import.log

Run:
  $HERE/run.sh
Measure (60s after 5s warmup, 500 bullets):
  $HERE/measure.sh /tmp/opencode/ruler_runs/godot_500
EOF
