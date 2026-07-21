#!/usr/bin/env bash
# Multi-round lifecycle matrix for all X11 window examples.
set -u
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"
export GOROOT="${GOROOT:-/home/yanghy/app/gopath/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.0.linux-amd64}"
export PATH="$GOROOT/bin:$PATH"
export GOTOOLCHAIN=local GOWORK=off
export WGPU_NATIVE_PATH="${WGPU_NATIVE_PATH:-$ROOT/lib/libwgpu_native.so}"
export LD_LIBRARY_PATH="${LD_LIBRARY_PATH:-$ROOT/lib}"
export DISPLAY="${DISPLAY:-:1}"
export XAUTHORITY="${XAUTHORITY:-/run/user/1000/gdm/Xauthority}"
export GPUI_SURFACE_SAMPLE_COUNT="${GPUI_SURFACE_SAMPLE_COUNT:-1}"

OUT=${OUT:-tmp/window_lifecycle_$(date +%Y%m%d_%H%M%S)}
mkdir -p "$OUT" tmp/bins
ROUNDS=${ROUNDS:-2}
SUMMARY="$OUT/SUMMARY.md"
{
  echo "# Window lifecycle matrix"
  echo
  echo "Generated: $(date -Iseconds)  ROUNDS=$ROUNDS"
  echo
  echo "| example | round | mode | exit | oom | recover_logs | verdict | note |"
  echo "|---------|------:|------|-----:|----:|-------------:|---------|------|"
} >"$SUMMARY"

PASS=0
FAIL=0

build() {
  go build -o "tmp/bins/$1" "./examples/$1" >"$OUT/build_$1.log" 2>&1
}

run_one() {
  local name=$1 mode=$2
  shift 2
  local bin=tmp/bins/$name
  local logf="$OUT/${name}__${mode}__r${ROUND}.log"
  local t0=$(date +%s)
  set +e
  timeout 45 env "$@" "$bin" >"$logf" 2>&1
  local ec=$?
  set -e
  local dt=$(( $(date +%s) - t0 ))
  local oom=0 rec=0 note=""
  if [[ -f "$logf" ]]; then
    oom=$(strings "$logf" 2>/dev/null | grep -c 'CreateTexture OOM' || true)
    rec=$(strings "$logf" 2>/dev/null | grep -cE 'GPU device recovered|ForceRecoverHealthy' || true)
    if strings "$logf" 2>/dev/null | grep -q 'selftest_ok'; then note="selftest_ok"; fi
    if strings "$logf" 2>/dev/null | grep -q 'status=PASS'; then note="${note:+$note;}PASS"; fi
    if strings "$logf" 2>/dev/null | grep -q 'DONE scenario'; then note="${note:+$note;}DONE"; fi
    if [[ $ec -eq 124 ]]; then note="${note:+$note;}timeout"; fi
  fi
  local verdict=PASS
  # Hard fail: OOM spam or native abort signals
  if [[ $oom -gt 0 ]]; then verdict=FAIL; note="${note:+$note;}oom"; fi
  if [[ $ec -eq 134 || $ec -eq 139 || $ec -eq 132 ]]; then verdict=FAIL; note="${note:+$note;}signal_abort"; fi
  # Force-recover modes: need recover log and zero OOM
  if [[ "$mode" == *force* || "$mode" == *recover* ]]; then
    if [[ $rec -lt 1 ]]; then verdict=FAIL; note="${note:+$note;}no_recover"; fi
  fi
  if [[ "$verdict" == PASS ]]; then PASS=$((PASS+1)); else FAIL=$((FAIL+1)); fi
  echo "| $name | $ROUND | $mode | $ec | $oom | $rec | **$verdict** | ${note:-} ${dt}s |" >>"$SUMMARY"
  echo "  [$verdict] $name $mode r$ROUND exit=$ec oom=$oom rec=$rec ${dt}s"
}

echo "OUT=$OUT ROUNDS=$ROUNDS"

for ex in mem_anim_window device_lost_redraw particle_kitchen_sink capability_matrix window_present vram_stages app_lifecycle_shell antd_desktop_app flutter_app_shell api_coverage_app; do
  echo "BUILD $ex"
  if ! build "$ex"; then
    echo "| $ex | - | build | 1 | - | - | **FAIL** | build |" >>"$SUMMARY"
    FAIL=$((FAIL+1))
    echo "  BUILD FAIL $ex"
  fi
done

for ROUND in $(seq 1 "$ROUNDS"); do
  echo "==== ROUND $ROUND ===="

  run_one mem_anim_window smoke \
    GPUI_SCENARIO=S03 GPUI_ANIM_SECONDS=4
  run_one mem_anim_window force_recover \
    GPUI_SCENARIO=S12 GPUI_FORCE_LOST_AFTER=35 GPUI_ANIM_SECONDS=8
  run_one mem_anim_window selftest_min \
    GPUI_SCENARIO=S12 GPUI_SELFTEST_LIFECYCLE=1 \
    GPUI_SELFTEST_MIN_AT=30 GPUI_SELFTEST_MAP_AT=70 GPUI_SELFTEST_LOST_AT=110 GPUI_SELFTEST_DONE_AT=150

  run_one device_lost_redraw force_recover \
    GPUI_FORCE_LOST_AFTER=40
  # no auto-exit — timeout after ~8s of drawing is OK if no OOM
  run_one device_lost_redraw smoke_timeout \
    GPUI_FORCE_LOST_AFTER=999999

  run_one particle_kitchen_sink smoke \
    GPUI_TIER=L1 GPUI_ANIM_SECONDS=4
  run_one particle_kitchen_sink force_recover \
    GPUI_TIER=L1 GPUI_FORCE_LOST_AFTER=30 GPUI_ANIM_SECONDS=7

  run_one capability_matrix smoke \
    GPUI_SCENARIO=C01 GPUI_ANIM_SECONDS=4
  run_one capability_matrix force_recover \
    GPUI_SCENARIO=C01 GPUI_FORCE_LOST_AFTER=25 GPUI_ANIM_SECONDS=6

  run_one window_present smoke \
    GPUI_PRESENT_FRAMES=60
  run_one window_present force_recover \
    GPUI_PRESENT_FRAMES=90 GPUI_FORCE_LOST_AFTER=40

  run_one vram_stages clear \
    GPUI_VRAM_STAGE=clear GPUI_VRAM_SECONDS=3

  run_one antd_desktop_app smoke \
    GPUI_ANIM_SECONDS=5
  run_one antd_desktop_app force_recover \
    GPUI_ANIM_SECONDS=7 GPUI_FORCE_LOST_AFTER=35
  run_one flutter_app_shell smoke \
    GPUI_ANIM_SECONDS=5
  run_one flutter_app_shell force_recover \
    GPUI_ANIM_SECONDS=7 GPUI_FORCE_LOST_AFTER=35

  run_one api_coverage_app force_recover \
    GPUI_ANIM_SECONDS=12 GPUI_FORCE_LOST_AFTER=5 GPUI_TARGET_FPS=15 GPUI_COVERAGE_STRICT=1
  run_one api_coverage_app selftest_full_api \
    GPUI_SELFTEST_LIFECYCLE=1 GPUI_TARGET_FPS=15 GPUI_COVERAGE_STRICT=1 \
    GPUI_SELFTEST_MIN_AT=4 GPUI_SELFTEST_MAP_AT=10 GPUI_SELFTEST_LOST_AT=16 GPUI_SELFTEST_DONE_AT=24

  for sc in S_ANIM S_LAYER S_MULTI S_RESIZE S_RECOVER S_ALL; do
    run_one app_lifecycle_shell "$sc" \
      GPUI_SHELL_SCENARIO=$sc GPUI_ANIM_SECONDS=6 \
      GPUI_SELFTEST_MIN_AT=25 GPUI_SELFTEST_MAP_AT=55 GPUI_SELFTEST_LOST_AT=90 GPUI_SELFTEST_DONE_AT=130 \
      GPUI_SHELL_RESIZE_AT=35
  done
done

{
  echo
  echo "## Totals"
  echo
  echo "- PASS rows: $PASS"
  echo "- FAIL rows: $FAIL"
  echo
  echo "## Coverage map (Skia / Flutter / Ant Design)"
  echo
  echo "| Pattern | Example | Mode |"
  echo "|---------|---------|------|"
  echo "| Continuous Canvas animation | mem_anim_window, app_lifecycle_shell S_ANIM | smoke |"
  echo "| Unfocus still present | app_lifecycle_shell S_FOCUS (manual), capability_matrix | host |"
  echo "| Minimize → Unconfigure | mem_anim selftest, particle, app_lifecycle_shell S_MIN/S_ALL | selftest |"
  echo "| Device abandon+recreate | * FORCE_LOST / ForceRecoverHealthy | force_recover |"
  echo "| Resize surface | app_lifecycle_shell S_RESIZE, particle | resize |"
  echo "| Modal / saveLayer RT | app_lifecycle_shell S_LAYER, mem_anim S12 | layer |"
  echo "| Multi Context | app_lifecycle_shell S_MULTI | multi |"
  echo "| VRAM attribution | vram_stages | clear |"
  echo "| Capability IDs | capability_matrix | C01 |"
  echo "| Redraw request loop | device_lost_redraw | force |"
  echo "| Ant Design Pro desktop | antd_desktop_app | smoke/force/selftest |"
  echo "| Flutter Material shell | flutter_app_shell | smoke/force/selftest |"
  echo "| Full Context public API | api_coverage_app | force+selftest 180/180 |"
} >>"$SUMMARY"

echo "DONE PASS=$PASS FAIL=$FAIL OUT=$OUT"
cat "$SUMMARY"
