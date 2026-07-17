#!/usr/bin/env bash
# Full unit suite for gpu + render core packages after bottom-layer changes.
# Process-isolated per package; heavy GPU packages sharded to avoid iGPU OOM.
set -u
trap '' HUP
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export GOROOT="${GOROOT:-/home/yanghy/app/gopath/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.linux-amd64}"
export PATH="$GOROOT/bin:$PATH"
export GOCACHE="${GOCACHE:-$ROOT/tmp/go-cache}"
export GOWORK=off
export GOTOOLCHAIN=local
export WGPU_NATIVE_PATH="${WGPU_NATIVE_PATH:-$ROOT/lib/libwgpu_native.so}"
export LD_LIBRARY_PATH="$ROOT/lib:${LD_LIBRARY_PATH:-}"
export DISPLAY="${DISPLAY:-:1}"
export XAUTHORITY="${XAUTHORITY:-/run/user/$(id -u)/gdm/Xauthority}"
export GPUI_SURFACE_SAMPLE_COUNT="${GPUI_SURFACE_SAMPLE_COUNT:-1}"
# Lower host pressure for many sequential Device creates
export GOMAXPROCS="${GOMAXPROCS:-2}"

OUT="${GPUI_FULL_UNIT_OUT:-$ROOT/tmp/full_unit}"
mkdir -p "$OUT"
TIMEOUT="${GPUI_FULL_UNIT_TIMEOUT:-900s}"
BATCH="${GPUI_FULL_UNIT_BATCH:-2}"   # tests per process for heavy pkgs
: > "$OUT/summary.txt"
: > "$OUT/runner.log"
date | tee "$OUT/start.txt" | tee -a "$OUT/runner.log"

LIGHT_PACKAGES=(
  ./gpu/context
  ./gpu/types
  ./gpu/webgpu/internal/thread
  ./render/internal/color
  ./render/internal/blend
  ./render/internal/clip
  ./render/internal/cache
  ./render/internal/stroke
  ./render/internal/filter
  ./render/internal/parallel
  ./render/internal/raster
  ./render/internal/wide
  ./render/internal/gpu/tilecompute
  ./render/text/cache
  ./render/filters
  ./render/gpu
  ./render/recording
  ./render/scene
  ./render/surface
  ./render/raster
)

# Heavy: many RequestDevice / GPU sessions per package
HEAVY_PACKAGES=(
  ./gpu/rwgpu
  ./gpu/webgpu
  ./render/internal/gpu
  ./render/text
  ./render
)

fail=0
pass=0
skip=0

run_pkg_once() {
  local pkg="$1"
  local runpat="${2:-}"
  local safe tag log rc
  safe=$(echo "$pkg" | sed 's|[./]|_|g')
  if [[ -n "$runpat" ]]; then
    tag="${safe}_$(echo "$runpat" | tr -c 'A-Za-z0-9' '_' | cut -c1-48)"
  else
    tag="$safe"
  fi
  log="$OUT/${tag}.log"
  if [[ -n "$runpat" ]]; then
    echo "=== $pkg -run $runpat $(date +%H:%M:%S) ===" | tee -a "$OUT/summary.txt" | tee -a "$OUT/runner.log"
    timeout "$TIMEOUT" go test -count=1 -p 1 -parallel 1 "$pkg" -run "$runpat" -timeout "$TIMEOUT" >"$log" 2>&1
    rc=$?
  else
    echo "=== $pkg $(date +%H:%M:%S) ===" | tee -a "$OUT/summary.txt" | tee -a "$OUT/runner.log"
    timeout "$TIMEOUT" go test -count=1 -p 1 -parallel 1 "$pkg" -timeout "$TIMEOUT" >"$log" 2>&1
    rc=$?
  fi
  if [[ $rc -eq 0 ]]; then
    if grep -qE 'no test files|\[no tests to run\]' "$log" 2>/dev/null; then
      echo "SKIP $pkg ${runpat:-}" | tee -a "$OUT/summary.txt" | tee -a "$OUT/runner.log"
      skip=$((skip+1))
    else
      echo "PASS $pkg ${runpat:-}" | tee -a "$OUT/summary.txt" | tee -a "$OUT/runner.log"
      pass=$((pass+1))
    fi
  else
    if grep -qE 'no test files|build constraints exclude all' "$log" 2>/dev/null; then
      echo "SKIP $pkg ${runpat:-}" | tee -a "$OUT/summary.txt" | tee -a "$OUT/runner.log"
      skip=$((skip+1))
    else
      echo "FAIL $pkg ${runpat:-} (rc=$rc)" | tee -a "$OUT/summary.txt" | tee -a "$OUT/runner.log"
      # Keep last FAIL lines for triage
      rg -n '^--- FAIL:|FAIL\t|Error|panic|Not enough memory' "$log" 2>/dev/null | tail -40 >> "$OUT/runner.log" || tail -25 "$log" >> "$OUT/runner.log" || true
      fail=$((fail+1))
    fi
  fi
  # Allow iGPU/driver reclaim between processes
  sleep 3
}

run_heavy_sharded() {
  local pkg="$1"
  local list tmp i n names batch pat raw
  tmp=$(mktemp)
  raw=$(mktemp)
  # List tests into a file (avoid pipefail/rg pipeline false-negatives that force
  # whole-package runs and iGPU OOM under batch stress).
  if ! go test -count=1 -list '.*' "$pkg" >"$raw" 2>"$raw.err"; then
    echo "WARN list tests failed for $pkg; running whole package" | tee -a "$OUT/runner.log"
    echo "list_err: $(head -c 400 "$raw.err" 2>/dev/null)" | tee -a "$OUT/runner.log"
    run_pkg_once "$pkg" ""
    rm -f "$tmp" "$raw" "$raw.err"
    return
  fi
  # Keep only Test* names (drop Example*, ok footer).
  if command -v rg >/dev/null 2>&1; then
    rg '^Test' "$raw" >"$tmp" || true
  else
    grep -E '^Test' "$raw" >"$tmp" || true
  fi
  rm -f "$raw" "$raw.err"
  n=$(wc -l <"$tmp" | tr -d ' ')
  if [[ "$n" -eq 0 ]]; then
    echo "SKIP $pkg (no tests listed)" | tee -a "$OUT/summary.txt"
    skip=$((skip+1))
    rm -f "$tmp"
    return
  fi
  echo "--- shard $pkg tests=$n batch=$BATCH ---" | tee -a "$OUT/summary.txt" | tee -a "$OUT/runner.log"
  i=0
  batch=()
  while IFS= read -r name; do
    [[ -z "$name" ]] && continue
    batch+=("$name")
    if [[ ${#batch[@]} -ge $BATCH ]]; then
      pat="^($(IFS='|'; echo "${batch[*]}"))$"
      run_pkg_once "$pkg" "$pat"
      batch=()
      i=$((i+1))
    fi
  done <"$tmp"
  if [[ ${#batch[@]} -gt 0 ]]; then
    pat="^($(IFS='|'; echo "${batch[*]}"))$"
    run_pkg_once "$pkg" "$pat"
  fi
  rm -f "$tmp"
}

for pkg in "${LIGHT_PACKAGES[@]}"; do
  run_pkg_once "$pkg" ""
done

for pkg in "${HEAVY_PACKAGES[@]}"; do
  run_heavy_sharded "$pkg"
done

echo "DONE pass=$pass fail=$fail skip=$skip $(date +%H:%M:%S)" | tee -a "$OUT/summary.txt" | tee -a "$OUT/runner.log"
echo "$fail" > "$OUT/exit_code.txt"
date | tee "$OUT/end.txt"
exit $fail
