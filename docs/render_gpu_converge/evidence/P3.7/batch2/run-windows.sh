#!/bin/bash
# P3.7 批 2 真窗复验：一轮一窗、-auto-only 自检后自管关闭、串行不并发（AGENTS GPU 安全条）。
# 每轮前后检查显示会话（本机 Wayland + XWayland :0，无 Xorg）；六~十轮 rc 记录在此。
set -u
ROOT=/home/yanghy/app/projects/gogpu/gpui
OUT="$ROOT/docs/render_gpu_converge/evidence/P3.7/batch2/windows-summary.txt"
cd "$ROOT" || exit 1
export CGO_ENABLED=1 LD_LIBRARY_PATH="$ROOT/lib" WGPU_NATIVE_PATH="$ROOT/lib/libwgpu_native.so"
export DISPLAY="${DISPLAY:-:0}"

: > "$OUT"
{
  echo "# 批 2 真窗复验汇总（$(date -Iseconds)）"
  echo "# 头：HEAD=$(git rev-parse --short HEAD) dirty=$(git status --porcelain | wc -l) paths（批 2 工作树未提交）"
  echo "# 跑法：一轮一窗、-auto-only、串行；每轮前后 pgrep -c Xwayland；无 SIGTERM 兜底、无 kill -9。"
} >> "$OUT"

wins=(engine/25d_basis engine/stage_chase engine/sprite engine/world engine/camera)
fail=0
for w in "${wins[@]}"; do
  for be in native go; do
    bin="/tmp/p37b2_$(echo "$w" | tr / _)_$be"
    if ! GPUI_BACKEND=$be go build -o "$bin" "./examples/$w" 2>>"$OUT"; then
      echo "窗 $w / $be: BUILD-FAIL" >> "$OUT"; fail=1; continue
    fi
    before=$(pgrep -c Xwayland || true)
    # stage_chase 的 auto 段本身就跑 120s（main.go:1034 runSeconds(120)），护栏要给足余量。
    guard=200
    case "$w" in engine/stage_chase) guard=300 ;; esac
    timeout "$guard" "$bin" -auto-only > "/tmp/p37b2_$(echo "$w" | tr / _)_$be.log" 2>&1
    rc=$?
    if [ "$rc" -ne 0 ]; then echo "窗 $w / $be: rc=$rc（timeout 护栏 $guard s，见日志）" >> "$OUT"; fail=1; fi
    after=$(pgrep -c Xwayland || true)
    key=$(grep -oE '"golden_changed_pct":[0-9.]+|"parity_changed_pct":[0-9.]+|golden=true\(wrote=[a-z]+ changed=[0-9]+\)|probes ok=(true|false)|"gate":[a-z]+' "/tmp/p37b2_$(echo "$w" | tr / _)_$be.log" | tr '\n' ' ')
    {
      echo "窗 $w / $be: rc=$rc xwayland_before=$before after=$after"
      echo "    $key"
    } >> "$OUT"
    rm -f "$bin"
  done
done
{
  echo "----"
  echo "全部窗口跑完：fail=$fail（0=全 rc 0）"
  echo "显示会话健康：$(pgrep -c Xwayland || echo 0) Xwayland 进程"
} >> "$OUT"
echo "done fail=$fail"
