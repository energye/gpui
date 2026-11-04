#!/bin/bash
# P3.7 批 3e-2 真窗复验：scene 流新增 TagImageQuad + CPU 瓦片腿链序修复，
# 验证不碰 quad 动词的存量窗画面逐位不变。一轮一窗、-auto-only 自检后自管关闭、串行（AGENTS GPU 安全条）。
set -u
ROOT=/home/yanghy/app/projects/gogpu/gpui
OUT="$ROOT/docs/render_gpu_converge/evidence/P3.7/batch3e/windows-3e2.txt"
cd "$ROOT" || exit 1
export CGO_ENABLED=1 LD_LIBRARY_PATH="$ROOT/lib" WGPU_NATIVE_PATH="$ROOT/lib/libwgpu_native.so"
export DISPLAY="${DISPLAY:-:0}"

: > "$OUT"
{
  echo "# 批 3e-2 真窗复验汇总（$(date -Iseconds)）"
  echo "# 头：HEAD=$(git rev-parse --short HEAD) dirty=$(git status --porcelain | wc -l) paths（3e-2 工作树未提交）"
  echo "# 跑法：一轮一窗、-auto-only、串行；每轮前后 pgrep -c Xwayland；无 kill -9、无 pkill。"
  echo "# 对照正本：batch3e/windows-summary.txt（3e-1 同一窗同一 golden）"
} >> "$OUT"

wins=(ui_render_base_scene)
fail=0
for w in "${wins[@]}"; do
  for be in native go; do
    bin="/tmp/p37_3e2_${w}_${be}"
    if ! GPUI_BACKEND=$be go build -o "$bin" "./examples/$w" 2>>"$OUT"; then
      echo "窗 $w / $be: BUILD-FAIL" >> "$OUT"; fail=1; continue
    fi
    before=$(pgrep -c Xwayland || true)
    timeout 200 "$bin" -auto-only > "/tmp/p37_3e2_${w}_$be.log" 2>&1
    rc=$?
    after=$(pgrep -c Xwayland || true)
    key=$(grep -oE 'logic=[0-9]+/[0-9]+ point=[0-9]+/[0-9]+ ink=[0-9]+/[0-9]+ parity=[0-9.]+% golden=[0-9.]+%|golden [a-z_]+\.png: diff=[0-9.]+% over [0-9]+ px|"pixel_golden_first_run":[a-z]+' "/tmp/p37_3e2_${w}_$be.log" | tr '\n' ' ')
    {
      echo "窗 $w / $be: rc=$rc xwayland_before=$before after=$after"
      echo "    $key"
    } >> "$OUT"
    [ "$rc" -ne 0 ] && fail=1
    rm -f "$bin"
  done
done
{
  echo "----"
  echo "全部窗口跑完：fail=$fail（0=全 rc 0）"
  echo "显示会话健康：$(pgrep -c Xwayland || echo 0) Xwayland 进程"
} >> "$OUT"
echo "done fail=$fail"
