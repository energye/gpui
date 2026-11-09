#!/usr/bin/env bash
# P3.9b 1x/4x serial measurement driver.
#
# Safety (AGENTS.md "GPU 真窗与压测安全"): ONE window at a time, strictly
# serial, no background jobs, no concurrency. Each round runs the app with
# RUN_SECONDS set so the program exits by itself via its normal close path
# (app.Run -> app.Close -> win.Close). No kill -9 / pkill / killall, ever.
# Before every round: verify the compositor/session is alive and the previous
# round left no crash signature. On any anomaly: STOP the batch and report.
#
# Known-invalid combo (2026-11-09 finding, see findings-go4x-black.md):
#   go/GL + 4x MSAA renders an all-black window. The window_not_black logic
#   probe makes this combo go RED (exit 1) on purpose. It is recorded as
#   INVALID and NEVER counted as green; its numbers are excluded from the
#   1x/4x conclusion. It does NOT stop the batch.
set -u

REPO=/home/yanghy/app/projects/gogpu/gpui
BIN=/tmp/p39b_bin
EVID="$REPO/docs/render_gpu_converge/evidence/P3.9b"
cd "$REPO" || exit 1
mkdir -p "$EVID/gate" "$EVID/negative" "$EVID/measure" "$EVID/window" "$EVID/gates"

export WGPU_NATIVE_PATH="$REPO/lib/libwgpu_native.so"
export LD_LIBRARY_PATH="$REPO/lib"
SOCK=/run/user/1000/wayland-0

STAGE="${1:-all}"   # gate | negative | measure | all
RUNS=0
PREV_LOG=""

health() {
  local label="$1"
  if ! pgrep -x gnome-shell >/dev/null 2>&1; then
    echo "STOP-BATCH [$label]: gnome-shell gone (session disappeared)"; return 1
  fi
  if ! python3 -c "import socket;s=socket.socket(socket.AF_UNIX);s.settimeout(2);s.connect('$SOCK');s.close()" >/dev/null 2>&1; then
    echo "STOP-BATCH [$label]: wayland-0 socket not connectable"; return 1
  fi
  if [ -n "$PREV_LOG" ] && [ -f "$PREV_LOG" ]; then
    if grep -qiE 'nvidia_drv\.so|Invalid MIT-MAGIC-COOKIE|segmentation fault|SIGSEGV|Xorg.*fail' "$PREV_LOG"; then
      echo "STOP-BATCH [$label]: crash signature in $PREV_LOG"; return 1
    fi
  fi
  return 0
}

# run_round <name> <backend> <msaa> <secs> <modeflag> [extra env...]
# Writes <name>.json (last JSON line), <name>.txt, <name>.rc. Returns rc.
run_round() {
  local name="$1" backend="$2" msaa="$3" secs="$4" modeflag="$5"; shift 5
  local extra=("$@")
  health "$name" || return 2
  local out="$EVID/$name.json" log="$EVID/$name.txt"
  echo "=== [$name] backend=$backend msaa=$msaa secs=$secs mode=$modeflag extra=${extra[*]:-} ==="
  env "RUN_SECONDS=$secs" "GPUI_BACKEND=$backend" "GPUI_P39B_MSAA=$msaa" "${extra[@]}" \
      timeout --signal=TERM "$((secs+45))s" "$BIN" "$modeflag" >"$out" 2>"$log"
  local rc=$?
  grep '^{' "$out" | tail -1 > "$out.tmp" && mv "$out.tmp" "$out"
  echo "$rc" > "$EVID/$name.rc"
  PREV_LOG="$log"; RUNS=$((RUNS+1))
  echo "    exit=$rc json_lines=$(grep -c '^{' "$out")"
  return "$rc"
}

# ---- gate: 3 valid combos must verify diff=0; go4x recorded INVALID --------
gate() {
  # Valid combos: native 1x/4x, go 1x. (Baselines seeded earlier with the same
  # scene; the added window_not_black probe does not change the image.)
  for combo in "native 1" "native 4" "go 1"; do
    set -- $combo; local be=$1 ms=$2 tag="${1}_msaa${2}"
    local rc
    run_round "gate/${tag}_verify" "$be" "$ms" 6 -auto-only ; rc=$?
    if [ "$rc" -ne 0 ]; then echo "STOP-BATCH: gate verify $tag rc=$rc"; return 2; fi
    python3 - "$EVID/gate/${tag}_verify.json" <<'PY' || { echo "STOP-BATCH: gate verify $tag diff != 0"; return 2; }
import json,sys
e=json.load(open(sys.argv[1]))["ability_extra"]
d=e["pixel_golden_diff_pct"]; s=e["pixel_golden_first_run"]
print("    verify golden_diff=%.6f%% first_run=%s logic=%d/%d nonblack=%d/%d" % (
    d,s,e["logic_probes_ok"],e["logic_probes_total"],e.get("window_nonblack",-1),e.get("window_samples",-1)))
sys.exit(0 if (d==0 and not s) else 1)
PY
  done
  # go/GL 4x: known-invalid. Run once (frozen phase), expect RED via
  # window_not_black probe; record as INVALID, never green, never a stop.
  local rc
  run_round "gate/go_msaa4_invalid" "go" 4 6 -auto-only ; rc=$?
  if [ "$rc" -eq 1 ]; then
    python3 - "$EVID/gate/go_msaa4_invalid.json" <<'PY'
import json,sys
e=json.load(open(sys.argv[1]))["ability_extra"]
print("    go4x INVALID (expected RED): nonblack=%d/%d logic=%d/%d" % (
    e.get("window_nonblack",-1),e.get("window_samples",-1),e["logic_probes_ok"],e["logic_probes_total"]))
PY
    echo "INVALID: go/GL+4x all-black window (see findings-go4x-black.md); excluded from green & conclusions" \
      > "$EVID/gate/go_msaa4.INVALID"
  else
    echo "STOP-BATCH: go4x unexpectedly rc=$rc (expected 1 = black-window red)"; return 2
  fi
}

# ---- negative control: freeze phase in measure mode -> phase_advanced RED ---
negative() {
  local rc
  run_round "negative/freeze" "native" 1 6 -measure "GPUI_P39B_BREAK=freeze" ; rc=$?
  if [ "$rc" -ne 1 ]; then echo "STOP-BATCH: freeze did NOT go red (rc=$rc)"; return 2; fi
  python3 - "$EVID/negative/freeze.json" <<'PY'
import json,sys
e=json.load(open(sys.argv[1]))["ability_extra"]
print("    freeze RED (expected): phase_advanced=%s logic=%d/%d" % (
    e.get("logic_05_phase_advanced"),e["logic_probes_ok"],e["logic_probes_total"]))
PY
}

# ---- measure: 3 valid combos green + go4x INVALID, 18s each ----------------
measure() {
  for combo in "native 1" "native 4" "go 1"; do
    set -- $combo; local be=$1 ms=$2 tag="${1}_msaa${2}" rc
    run_round "measure/${tag}" "$be" "$ms" 18 -measure ; rc=$?
    if [ "$rc" -ne 0 ]; then echo "STOP-BATCH: measure $tag rc=$rc"; return 2; fi
  done
  local rc
  run_round "measure/go_msaa4_invalid" "go" 4 18 -measure ; rc=$?
  if [ "$rc" -eq 1 ]; then
    echo "INVALID: go/GL+4x all-black window; numbers excluded from 1x/4x conclusion" \
      > "$EVID/measure/go_msaa4.INVALID"
    echo "    go4x INVALID (expected RED), recorded"
  else
    echo "STOP-BATCH: measure go4x unexpectedly rc=$rc"; return 2
  fi
}

case "$STAGE" in
  gate) gate ;;
  negative) negative ;;
  measure) measure ;;
  all) gate && negative && measure ;;
  *) echo "usage: $0 gate|negative|measure|all"; exit 1 ;;
esac
rc=$?
echo "=== DONE stage=$STAGE rounds=$RUNS rc=$rc ==="
exit "$rc"
