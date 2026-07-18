#!/usr/bin/env bash
# Memory leak + positive performance guard (render engine).
# Authority: docs/MEM_LEAK_PERF_GUARD_PLAN.md
#
#   ./scripts/run_mem_guard.sh quick
#   ./scripts/run_mem_guard.sh daily
#   ./scripts/run_mem_guard.sh deep
#
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Match run_full_unit_tests.sh toolchain (go.mod requires >=1.25)
export GOROOT="${GOROOT:-/home/yanghy/app/gopath/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.linux-amd64}"
if [[ -x "$GOROOT/bin/go" ]]; then
  export PATH="$GOROOT/bin:$PATH"
fi
export GOTOOLCHAIN="${GOTOOLCHAIN:-local}"
export GOWORK="${GOWORK:-off}"


MODE="${1:-quick}"
export WGPU_NATIVE_PATH="${WGPU_NATIVE_PATH:-$ROOT/lib/libwgpu_native.so}"
export GOCACHE="${GOCACHE:-$ROOT/tmp/go-cache}"
export LD_LIBRARY_PATH="$ROOT/lib:${LD_LIBRARY_PATH:-}"
export DISPLAY="${DISPLAY:-:1}"
export XAUTHORITY="${XAUTHORITY:-/run/user/$(id -u)/gdm/Xauthority}"
export GPUI_SURFACE_SAMPLE_COUNT="${GPUI_SURFACE_SAMPLE_COUNT:-1}"

TS="$(date +%Y%m%d_%H%M%S)"
OUT="${GPUI_MEM_GUARD_OUT:-$ROOT/tmp/mem_guard_${MODE}_${TS}}"
mkdir -p "$OUT/pks"
SUMMARY="$OUT/SUMMARY.md"
: >"$SUMMARY"

log() { echo "$*" | tee -a "$SUMMARY"; }
fail=0

run_pks() {
  local probe="$1"
  local sec="$2"
  local json="$OUT/pks/${probe}_${sec}s.json"
  local plog="$OUT/pks_${probe}_${sec}s.log"
  log ">> PKS $probe seconds=$sec"
  set +e
  GPUI_PROBE="$probe" GPUI_ANIM_SECONDS="$sec" GPUI_RESULT_FILE="$json" \
    go run ./examples/particle_kitchen_sink >"$plog" 2>&1
  local rc=$?
  set -e
  if [[ $rc -ne 0 ]]; then
    log "FAIL $probe process_exit=$rc (see $plog)"
    fail=1
    return 0
  fi
  if [[ ! -f "$json" ]]; then
    log "FAIL $probe missing json"
    fail=1
    return 0
  fi
  local line
  line="$(python3 -c "
import json
r=json.load(open('${json}'))
print('status=%s fps_ema=%s cpu_avg=%s rss_steady_delta_kb=%s reason=%s' % (
  r.get('status'), r.get('fps_ema'), r.get('cpu_avg'), r.get('rss_steady_delta_kb'), r.get('fail_reason') or r.get('reason','')))
" 2>/dev/null || echo "json_parse_error")"
  log "   $line"
  if echo "$line" | grep -qi 'status=FAIL'; then
    fail=1
  fi
}

log "# mem_guard mode=$MODE"
log "out=$OUT"
log "date=$(date -Iseconds 2>/dev/null || date)"
log ""

case "$MODE" in
  quick)
    log "## 1) TestMem leak tiers (COUNT=${GPUI_MEM_COUNT:-1})"
    set +e
    GPUI_MEM_LOG="$OUT/mem_leak.log" GPUI_MEM_COUNT="${GPUI_MEM_COUNT:-1}" ./scripts/run_mem_leak_tests.sh
    rc=$?
    set -e
    if [[ $rc -ne 0 ]]; then log "FAIL run_mem_leak_tests"; fail=1; else log "OK run_mem_leak_tests"; fi
    log "## 2) P_MEM_SOAK 60s"
    run_pks P_MEM_SOAK 60
    log "## 3) Perf sample P_SOLID 30s"
    run_pks P_SOLID 30
    ;;
  daily)
    if [[ "${GPUI_MEM_GUARD_SKIP_UNIT:-0}" != "1" ]]; then
      log "## 0) Full unit"
      set +e
      ./scripts/run_full_unit_tests.sh
      rc=$?
      set -e
      if [[ $rc -ne 0 ]]; then log "FAIL full unit"; fail=1; else log "OK full unit"; fi
      cp -a tmp/full_unit/summary.txt "$OUT/full_unit_summary.txt" 2>/dev/null || true
    else
      log "## 0) skip unit (GPUI_MEM_GUARD_SKIP_UNIT=1)"
    fi
    log "## 1) TestMem COUNT=${GPUI_MEM_COUNT:-3}"
    set +e
    GPUI_MEM_LOG="$OUT/mem_leak.log" GPUI_MEM_COUNT="${GPUI_MEM_COUNT:-3}" ./scripts/run_mem_leak_tests.sh
    rc=$?
    set -e
    if [[ $rc -ne 0 ]]; then log "FAIL run_mem_leak_tests"; fail=1; else log "OK run_mem_leak_tests"; fi
    log "## 2) mem matrix"
    set +e
    GPUI_PKS_FILTER=mem GPUI_PKS_OUT="$OUT/pks_matrix" ./scripts/run_pks_matrix.sh
    rc=$?
    set -e
    if [[ $rc -ne 0 ]]; then log "FAIL pks mem matrix"; fail=1; else log "OK pks mem matrix"; fi
    log "## 3) P_MEM_LONG 180s"
    run_pks P_MEM_LONG 180
    log "## 4) Perf guards"
    run_pks P_SOLID 60
    run_pks P_BLEND_LAYER 60
    ;;
  deep)
    log "## 1) TestMem COUNT=${GPUI_MEM_COUNT:-3}"
    set +e
    GPUI_MEM_LOG="$OUT/mem_leak.log" GPUI_MEM_COUNT="${GPUI_MEM_COUNT:-3}" ./scripts/run_mem_leak_tests.sh
    rc=$?
    set -e
    [[ $rc -eq 0 ]] || { log "FAIL TestMem"; fail=1; }
    log "## 2) mem matrix"
    set +e
    GPUI_PKS_FILTER=mem GPUI_PKS_OUT="$OUT/pks_matrix" ./scripts/run_pks_matrix.sh
    rc=$?
    set -e
    [[ $rc -eq 0 ]] || { log "FAIL matrix"; fail=1; }
    log "## 3) P_MEM_LONG 600s"
    run_pks P_MEM_LONG 600
    log "## 4) Perf guards"
    run_pks P_SOLID 60
    run_pks P_BLEND_LAYER 60
    run_pks P_L1 60
    ;;
  *)
    echo "usage: $0 {quick|daily|deep}" >&2
    exit 2
    ;;
esac

log ""
log "DONE fail=$fail out=$OUT"
echo "SUMMARY: $SUMMARY"
exit $fail
