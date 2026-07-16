#!/usr/bin/env bash
# Run particle kitchen-sink tiers L0–L4 with JSON evidence.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export PATH="${PATH:-}:/home/yanghy/app/energy/go/bin"
export WGPU_NATIVE_PATH="${WGPU_NATIVE_PATH:-$ROOT/lib/libwgpu_native.so}"
export LD_LIBRARY_PATH="$ROOT/lib:${LD_LIBRARY_PATH:-}"
export DISPLAY="${DISPLAY:-:1}"
export XAUTHORITY="${XAUTHORITY:-/run/user/1000/gdm/Xauthority}"
export GOCACHE="${GOCACHE:-/tmp/gpui-go-cache}"
export GOMODCACHE="${GOMODCACHE:-/home/yanghy/app/gopath/pkg/mod}"
export GOWORK=off

SECONDS_EACH="${GPUI_ANIM_SECONDS:-8}"
OUTDIR="${GPUI_PKS_OUT:-/tmp/pks_run}"
BIN="${PKS_BIN:-/tmp/pks_bin}"
mkdir -p "$OUTDIR"
if [[ ! -x "$BIN" ]]; then
  go build -o "$BIN" ./examples/particle_kitchen_sink
fi

tiers=(L0 L1 L2 L3 L4)
pass=0; fail=0
for t in "${tiers[@]}"; do
  echo "==== $t ===="
  if GPUI_TIER="$t" GPUI_ANIM_SECONDS="$SECONDS_EACH" \
      GPUI_RESULT_FILE="$OUTDIR/${t}.json" "$BIN" >"$OUTDIR/${t}.log" 2>&1; then
    pass=$((pass+1))
  else
    fail=$((fail+1))
    echo "FAIL $t"
  fi
  if [[ -f "$OUTDIR/${t}.json" ]]; then
    python3 -c "import json;d=json.load(open('$OUTDIR/${t}.json'));print(d['tier'],d['status'],'fps',round(d['fps_avg'],1),'cpu',round(d['cpu_avg'],1),'fb',d['cpu_fallback_ops'],'n',d['particle_n'])"
  fi
done

{
  echo "# particle kitchen sink"
  echo
  echo "| tier | status | fps_avg | cpu% | cpu_fb | n | features |"
  echo "|------|--------|---------|------|--------|---|----------|"
  for t in "${tiers[@]}"; do
    jf="$OUTDIR/${t}.json"
    if [[ -f "$jf" ]]; then
      python3 - <<PY
import json
d=json.load(open("$jf"))
print(f"| {d['tier']} | {d['status']} | {d['fps_avg']:.1f} | {d['cpu_avg']:.1f} | {d['cpu_fallback_ops']} | {d['particle_n']} | {d['features']} |")
PY
    else
      echo "| $t | MISSING | | | | | |"
    fi
  done
} >"$OUTDIR/SUMMARY.md"
echo "==== SUMMARY pass=$pass fail=$fail out=$OUTDIR ===="
cat "$OUTDIR/SUMMARY.md"
[[ "$fail" -eq 0 ]]

# For isolation axes + bisect, use: scripts/run_pks_matrix.sh
