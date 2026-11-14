#!/usr/bin/env bash
# O11 现有窗回归 runner（串行、隔离 :3、标准自退、不硬杀）。
# 用法: bash run_regression_windows.sh <bin 目录> <输出目录>
# 每轮前 xdpyinfo 健康检查；窗以 RUN_SECONDS 自退，timeout 仅为兜底（标准关闭优先）。
# 已知存量红窗（r4b/r21 金图、r8 fps）预期 rc!=0 —— 记录 diff/fps 与登记值对照。
set -u
ROOT="$(cd "$(dirname "$0")/../../../.." && pwd)"
BINDIR="${1:?bin dir}"
OUT="${2:?out dir}"
mkdir -p "$OUT"
: > "$OUT/summary.tsv"
echo -e "window\tstatus\trc\tfps\tnote" >> "$OUT/summary.tsv"

run_one() {
  local win="$1" secs="$2" extra="${3:-}"
  DISPLAY=:3 xdpyinfo >/dev/null 2>&1 || { echo -e "$win\tDISPLAY_DEAD\t-\t-\t隔离显示失联，停批" >> "$OUT/summary.tsv"; exit 2; }
  echo "== $win (RUN_SECONDS=$secs $extra) =="
  ( cd "$ROOT" && env -u WAYLAND_DISPLAY DISPLAY=:3 \
      LD_LIBRARY_PATH="$ROOT/lib" WGPU_NATIVE_PATH="$ROOT/lib/libwgpu_native.so" \
      RUN_SECONDS="$secs" timeout 300 "$BINDIR/$win" $extra \
      >"$OUT/$win.stdout.txt" 2>"$OUT/$win.stderr.txt" )
  local rc=$?
  local fps; fps=$(grep -o '"fps_interval":[0-9.]*' "$OUT/$win.stdout.txt" 2>/dev/null | head -1 | cut -d: -f2)
  echo -e "$win\trc=$rc\trc=$rc\tfps=$fps\tsee stderr" >> "$OUT/summary.tsv"
}

run_one ui_base_idle 30
run_one ui_wr_r4b_multidamage 15
run_one ui_wr_r8_overlay 30
run_one ui_wr_r21_shell 15
for k in kit_f0_content kit_f0_decor kit_f0_interact kit_f0_layout kit_f0_motion kit_f0_scope kit_f0_theme; do
  run_one "$k" 20
done
echo "done"
