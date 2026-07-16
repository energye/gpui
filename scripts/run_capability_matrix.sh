#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export WGPU_NATIVE_PATH="${WGPU_NATIVE_PATH:-$ROOT/lib/libwgpu_native.so}"
export LD_LIBRARY_PATH="$ROOT/lib:${LD_LIBRARY_PATH:-}"
export DISPLAY="${DISPLAY:-:0}"
SECONDS_EACH="${GPUI_ANIM_SECONDS:-12}"
OUTDIR="${GPUI_CAP_OUT:-/tmp/capability_matrix_run}"
mkdir -p "$OUTDIR"
BIN="${CAPABILITY_MATRIX_BIN:-/tmp/capability_matrix}"
if [[ ! -x "$BIN" ]]; then
  go build -o "$BIN" ./examples/capability_matrix
fi
ids=(C01 C02 C03 C04 C05 C06 C07 C08 C09 C10 C11 C12 C13 C14 C15 C16 C17 C18 C19 C20)
pass=0; fail=0
for id in "${ids[@]}"; do
  echo "==== $id ===="
  if GPUI_SCENARIO="$id" GPUI_ANIM_SECONDS="$SECONDS_EACH" \
      GPUI_RESULT_FILE="$OUTDIR/${id}.json" "$BIN"; then
    pass=$((pass+1))
  else
    fail=$((fail+1))
    echo "FAIL $id"
  fi
done
echo "==== SUMMARY pass=$pass fail=$fail out=$OUTDIR ===="
[[ "$fail" -eq 0 ]]
