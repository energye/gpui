#!/bin/bash
# P2.5 -race：本步触及的共享状态（hal 释放开关 hook、scheduler MetricsStore targetHz/fps_wall、
# scene refuseChanged 损伤联合、render/internal/gpu 池族回归）
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
export WGPU_NATIVE_PATH="$ROOT/lib/libwgpu_native.so"
export LD_LIBRARY_PATH="$ROOT/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.5/race.txt"
GPU_RE='^(TestSurfaceCache_.*|TestTextureSet_.*|TestOffscreenCache_.*|TestDualTexOutPool_.*|TestDirectSubmit_.*|TestGPUShared_.*|TestSessionRegistry_.*|TestImageCache_.*|TestShrinkDue_Matrix|TestShrinkRerecordDamagesClearedTail|TestCaretTransparentRerecordClearsStale)$'
{
  echo "# P2.5 -race"
  echo "# date: $(date -Iseconds)"
  echo "# head: $(git -C "$ROOT" rev-parse --short HEAD)"
  echo
  echo "## gpu/hal（释放开关 hook + 全包）"
  CGO_ENABLED=1 go test -race -count=1 -timeout 20m ./gpu/hal/ 2>&1 | tail -4
  echo "# exit: ${PIPESTATUS[0]}"
  echo
  echo "## ui/scheduler（MetricsStore 并发记帧 + targetHz/fps_wall 快照）"
  CGO_ENABLED=1 go test -race -count=1 -timeout 20m ./ui/scheduler/ 2>&1 | tail -4
  echo "# exit: ${PIPESTATUS[0]}"
  echo
  echo "## ui/scene（refuseChanged 损伤联合 + caret/stable 尾巴）"
  CGO_ENABLED=1 go test -race -count=1 -timeout 20m ./ui/scene/ 2>&1 | tail -4
  echo "# exit: ${PIPESTATUS[0]}"
  echo
  echo "## render/internal/gpu（池族回归集）"
  CGO_ENABLED=1 go test -race -count=1 -timeout 20m ./render/internal/gpu/ -run "$GPU_RE" 2>&1 | tail -4
  echo "# exit: ${PIPESTATUS[0]}"
} > "$OUT" 2>&1
tail -30 "$OUT"
