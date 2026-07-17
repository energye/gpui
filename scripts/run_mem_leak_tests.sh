#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export WGPU_NATIVE_PATH="${WGPU_NATIVE_PATH:-$ROOT/lib/libwgpu_native.so}"
export GOCACHE="${GOCACHE:-$ROOT/tmp/go-cache}"
export LD_LIBRARY_PATH="$ROOT/lib:${LD_LIBRARY_PATH:-}"
export DISPLAY="${DISPLAY:-:1}"
export XAUTHORITY="${XAUTHORITY:-/run/user/$(id -u)/gdm/Xauthority}"
export GPUI_SURFACE_SAMPLE_COUNT="${GPUI_SURFACE_SAMPLE_COUNT:-1}"
COUNT="${GPUI_MEM_COUNT:-3}"
TIMEOUT="${GPUI_MEM_TIMEOUT:-180s}"
LOG="${GPUI_MEM_LOG:-$ROOT/tmp/gpui_mem_leak_tests.log}"
: > "$LOG"

echo "== MEM leak suite count=$COUNT (process-isolated tiers) ==" | tee -a "$LOG"
fail=0
for run in $(seq 1 "$COUNT"); do
  echo "---- pass $run/$COUNT ----" | tee -a "$LOG"
  for pat in \
    'TestMem_T0_' \
    'TestMem_T1_' \
    'TestMem_T2_' \
    'TestMem_T3_ComplexOffscreen_EscalatingLevels$' \
    'TestMem_T3_ComplexOffscreen_SizeChurn$' \
    'TestMem_T4_'
  do
    echo ">> $pat" | tee -a "$LOG"
    if ! timeout "$TIMEOUT" go test -count=1 ./render -run "$pat" -timeout "$TIMEOUT" >>"$LOG" 2>&1; then
      echo "FAIL $pat pass=$run" | tee -a "$LOG"
      fail=1
    else
      echo "OK $pat" | tee -a "$LOG"
    fi
    sleep 1.0
  done
done
echo "DONE fail=$fail" | tee -a "$LOG"
exit $fail
