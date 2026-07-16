#!/usr/bin/env bash
# Run mem_anim_window long-soak: ONE scenario per process, sequential.
# Usage:
#   scripts/run_mem_anim_longsoak.sh              # S01-S12 @ 90s, S12 also heavy optional
#   GPUI_SOAK_SECONDS=120 scripts/run_mem_anim_longsoak.sh S03
#   scripts/run_mem_anim_longsoak.sh S01 S02 S03
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export PATH="${GO_TOOLCHAIN_PATH:-/home/yanghy/app/gopath/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.linux-amd64/bin}:$PATH"
export GOROOT="${GOROOT:-/home/yanghy/app/gopath/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.linux-amd64}"
export GOTOOLCHAIN="${GOTOOLCHAIN:-local}"
export GOCACHE="${GOCACHE:-/tmp/gpui-go-cache}"
export GOMODCACHE="${GOMODCACHE:-/home/yanghy/app/gopath/pkg/mod}"
export WGPU_NATIVE_PATH="${WGPU_NATIVE_PATH:-$ROOT/lib/libwgpu_native.so}"
export LD_LIBRARY_PATH="$ROOT/lib:${LD_LIBRARY_PATH:-}"
export DISPLAY="${DISPLAY:-:1}"
export XAUTHORITY="${XAUTHORITY:-/run/user/$(id -u)/gdm/Xauthority}"
export GPUI_SURFACE_SAMPLE_COUNT="${GPUI_SURFACE_SAMPLE_COUNT:-1}"
export GPUI_TARGET_FPS="${GPUI_TARGET_FPS:-60}"
export GPUI_ANIM_LOG_EVERY="${GPUI_ANIM_LOG_EVERY:-60}"
export GPUI_FIXED_SIZE="${GPUI_FIXED_SIZE:-1}"

BIN="${GPUI_MEM_ANIM_BIN:-/tmp/mem_anim_window}"
SEC="${GPUI_SOAK_SECONDS:-90}"
HEAVY_SEC="${GPUI_SOAK_HEAVY_SECONDS:-300}"
OUT="${GPUI_SOAK_OUT:-/tmp/mem_anim_soak_$(date +%Y%m%d_%H%M%S)}"
mkdir -p "$OUT"

echo "== build $BIN =="
go build -o "$BIN" ./examples/mem_anim_window

# Default matrix: S01-S12; pass S15-S21 / S13 / S14 explicitly for gap+stress
if [[ $# -gt 0 ]]; then
  SCENARIOS=("$@")
else
  SCENARIOS=(S01 S02 S03 S04 S05 S06 S07 S08 S09 S10 S11 S12)
fi

SUMMARY="$OUT/SUMMARY.md"
{
  echo "# mem_anim long-soak SUMMARY"
  echo
  echo "- out: \`$OUT\`"
  echo "- seconds_default: $SEC"
  echo "- one scenario per process: YES"
  echo
  echo "| Scenario | Status | Frames | FPS ema/avg | CPU% | RSS ΔKB | Steady ΔKB | cpu_fb | Notes |"
  echo "|----------|--------|--------|-------------|------|---------|------------|--------|-------|"
} > "$SUMMARY"

fail=0
for sc in "${SCENARIOS[@]}"; do
  sc=$(echo "$sc" | tr '[:lower:]' '[:upper:]')
  dur="$SEC"
  case "$sc" in
    S12|S14|S21) dur="$HEAVY_SEC" ;;
  esac
  # timeout slightly above anim seconds for teardown
  kill_after=$((dur + 30))
  dir="$OUT/$sc"
  mkdir -p "$dir"
  echo "==== RUN $sc duration=${dur}s (single process) ====" | tee "$dir/stdout.log"

  # Clean env of FEAT overrides so scenario mapping is pure.
  # Only inject GPUI_DENSITY for S13 — global density must not pollute S12/S14.
  dens_args=()
  case "$sc" in
    S13)
      if [[ -n "${GPUI_DENSITY:-}" ]]; then
        dens_args+=(GPUI_DENSITY="$GPUI_DENSITY")
      fi
      ;;
  esac
  env -u GPUI_FEAT_ALL -u GPUI_FEAT_BG -u GPUI_FEAT_GLOW -u GPUI_FEAT_CARDS \
      -u GPUI_FEAT_PATHS -u GPUI_FEAT_DASH -u GPUI_FEAT_CLIP -u GPUI_FEAT_LAYER \
      -u GPUI_FEAT_BACKDROP -u GPUI_FEAT_MASK -u GPUI_FEAT_IMAGE -u GPUI_FEAT_TEXT \
      -u GPUI_FEAT_FILTER -u GPUI_FEAT_TRANSFORM -u GPUI_FEAT_BLEND -u GPUI_FEAT_VERTICES \
      -u GPUI_FEAT_PIXELS -u GPUI_FEAT_POLYGON -u GPUI_FEAT_GRADIENT -u GPUI_FEAT_PATTERN \
      -u GPUI_FEAT_ADVBLEND -u GPUI_FEAT_RRECTCLIP -u GPUI_FEAT_TEXTLCD -u GPUI_FEAT_DAMAGE \
      -u GPUI_FEAT_SCROLL -u GPUI_FEAT_HUD -u GPUI_STRESS -u GPUI_PERF_LITE \
      -u GPUI_DENSITY \
    GPUI_SCENARIO="$sc" \
    GPUI_ANIM_SECONDS="$dur" \
    GPUI_METRICS_FILE="$dir/metrics.csv" \
    GPUI_RESULT_FILE="$dir/result.json" \
    "${dens_args[@]}" \
    /usr/bin/time -f 'elapsed=%e cpu=%P maxrss=%MKB' \
      timeout -s INT "${kill_after}s" "$BIN" >>"$dir/stdout.log" 2>&1 \
    || true

  # Parse result
  status="UNKNOWN"
  line=""
  if [[ -f "$dir/result.json.line" ]]; then
    line=$(cat "$dir/result.json.line")
    if echo "$line" | grep -q 'status=PASS'; then status=PASS; else status=FAIL; fail=1; fi
  elif grep -q 'status=PASS' "$dir/stdout.log"; then
    status=PASS
    line=$(grep 'done scenario=' "$dir/stdout.log" | tail -1)
  else
    status=FAIL
    fail=1
    line=$(tail -3 "$dir/stdout.log" | tr '\n' ' ')
  fi

  # Extract fields roughly
  fps_ema=$(echo "$line" | sed -n 's/.*fps_ema=\([0-9.]*\).*/\1/p'); fps_ema=${fps_ema:-?}
  fps_avg=$(echo "$line" | sed -n 's/.*fps_avg=\([0-9.]*\).*/\1/p'); fps_avg=${fps_avg:-?}
  cpu=$(echo "$line" | sed -n 's/.*cpu_avg=\([0-9.]*\).*/\1/p'); cpu=${cpu:-?}
  frames=$(echo "$line" | sed -n 's/.*frames=\([0-9]*\).*/\1/p'); frames=${frames:-?}
  dlt=$(echo "$line" | sed -n 's/.*rss_delta_kb=\([-0-9]*\).*/\1/p'); dlt=${dlt:-?}
  sdlt=$(echo "$line" | sed -n 's/.*rss_steady_delta_kb=\([-0-9]*\).*/\1/p'); sdlt=${sdlt:-?}
  cfb=$(echo "$line" | sed -n 's/.*cpu_fb=\([0-9]*\).*/\1/p'); cfb=${cfb:-?}

  echo "| $sc | $status | $frames | $fps_ema / $fps_avg | $cpu | $dlt | $sdlt | $cfb | ${dur}s |" >> "$SUMMARY"
  echo "==== DONE $sc $status ===="
  # gap between scenarios for GPU reclaim
  sleep 2
done

echo >> "$SUMMARY"
echo "fail=$fail" >> "$SUMMARY"
echo "SUMMARY -> $SUMMARY"
echo "DONE fail=$fail"
exit $fail
