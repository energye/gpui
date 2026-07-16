#!/usr/bin/env bash
# P6: full C01–C32 window regression + optional golden capture/compare + perf table.
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
OUTDIR="${GPUI_CAP_OUT:-/tmp/cap_p6_run}"
BIN="${CAPABILITY_MATRIX_BIN:-/tmp/cap_l0_bin}"
MODE="${GPUI_P6_MODE:-regress}" # regress | capture-golden | compare
GOLDEN_DIR="${GPUI_GOLDEN_DIR:-$ROOT/examples/capability_matrix/golden}"
CAPTURE_DIR="${GPUI_CAPTURE_DIR:-$OUTDIR/capture}"

mkdir -p "$OUTDIR"
if [[ ! -x "$BIN" ]]; then
  go build -o "$BIN" ./examples/capability_matrix
fi

ids=(C01 C02 C03 C04 C05 C06 C07 C08 C09 C10 C11 C12 C13 C14 C15 C16 C17 C18 C19 C20
     C21 C22 C23 C24 C25 C26 C27 C28 C29 C30 C31 C32)

pass=0; fail=0
declare -a lines

run_one() {
  local id="$1"
  local extra_env=()
  if [[ "$MODE" == "capture-golden" || "$MODE" == "capture" ]]; then
    extra_env+=(
      GPUI_DETERMINISTIC=1
      GPUI_GOLDEN_NO_HUD=1
      GPUI_CAPTURE_DIR="$CAPTURE_DIR"
      GPUI_CAPTURE_FRAME="${GPUI_CAPTURE_FRAME:-90}"
    )
  fi
  echo "==== $id mode=$MODE ===="
  if env "${extra_env[@]}" \
      GPUI_SCENARIO="$id" GPUI_ANIM_SECONDS="$SECONDS_EACH" \
      GPUI_RESULT_FILE="$OUTDIR/${id}.json" \
      "$BIN" >"$OUTDIR/${id}.log" 2>&1; then
    st=$(python3 -c "import json;print(json.load(open('$OUTDIR/${id}.json')).get('status','?'))" 2>/dev/null || echo '?')
    if [[ "$st" == "PASS" ]]; then
      pass=$((pass+1))
    else
      fail=$((fail+1))
    fi
    lines+=("$id $st")
  else
    fail=$((fail+1))
    lines+=("$id FAIL_RUN")
  fi
}

if [[ "$MODE" == "compare" ]]; then
  python3 "$ROOT/scripts/cap_compare_golden.py" \
    --got "${GPUI_CAPTURE_DIR:-$CAPTURE_DIR}" \
    --golden "$GOLDEN_DIR" \
    --diff-dir "$OUTDIR/diff" \
    --report "$OUTDIR/golden_report.json" \
    --max-rmse "${GPUI_MAX_RMSE:-0.08}" \
    --min-ssim "${GPUI_MIN_SSIM:-0.90}"
  exit $?
fi

for id in "${ids[@]}"; do
  run_one "$id"
done

# perf markdown
{
  echo "# capability_matrix P6 perf baseline"
  echo
  echo "| ID | status | fps_ema | fps_avg | cpu% | cpu_fb | gpu_ops | rss_end_kb |"
  echo "|----|--------|---------|---------|------|--------|---------|------------|"
  for id in "${ids[@]}"; do
    jf="$OUTDIR/${id}.json"
    if [[ -f "$jf" ]]; then
      python3 - <<PY
import json
d=json.load(open("$jf"))
print(f"| {d.get('scenario')} | {d.get('status')} | {d.get('fps_ema',0):.1f} | {d.get('fps_avg',0):.1f} | {d.get('cpu_avg',0):.1f} | {d.get('cpu_fallback_ops',0)} | {d.get('gpu_ops',0)} | {d.get('rss_end_kb',0)} |")
PY
    else
      echo "| $id | MISSING | | | | | | |"
    fi
  done
} >"$OUTDIR/PERF.md"

echo "==== SUMMARY pass=$pass fail=$fail out=$OUTDIR ===="
cat "$OUTDIR/PERF.md"
[[ "$fail" -eq 0 ]]
