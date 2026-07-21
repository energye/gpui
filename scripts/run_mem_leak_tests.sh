#!/usr/bin/env bash
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

# Prefer absolute go from GOROOT so bare `go` never falls back to system 1.21.
if [[ -x "${GOROOT}/bin/go" ]]; then
  GO_BIN="${GOROOT}/bin/go"
elif command -v go >/dev/null 2>&1; then
  GO_BIN="$(command -v go)"
else
  echo "error: go not found (set GOROOT to go1.25+ toolchain)" >&2
  exit 127
fi
export GO_BIN
# Prepend so child scripts / timeout see the same toolchain first.
export PATH="$(dirname "$GO_BIN"):$PATH"
if ! "$GO_BIN" version 2>/dev/null | grep -qE 'go1\.(2[5-9]|[3-9][0-9])'; then
  echo "error: need go >= 1.25, got: $("$GO_BIN" version 2>&1)" >&2
  exit 127
fi


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
    if ! timeout "$TIMEOUT" "$GO_BIN" test -count=1 ./render -run "$pat" -timeout "$TIMEOUT" >>"$LOG" 2>&1; then
      echo "FAIL $pat pass=$run" | tee -a "$LOG"
      fail=1
      # Give driver time to reclaim after hard OOM/abort.
      sleep 3.0
    else
      echo "OK $pat" | tee -a "$LOG"
    fi
    # Extra settle after heavy VRAM tiers so the next process is not starved.
    case "$pat" in
      *SizeChurn*) sleep 5.0 ;;
      *T3*) sleep 2.0 ;;
      *T4*) sleep 2.0 ;;
      *) sleep 1.0 ;;
    esac
  done
done
echo "DONE fail=$fail" | tee -a "$LOG"
exit $fail
