#!/bin/bash
# P2.4d -race（本步新增的共享状态：设备级 registry owner 分账 / res.Cache 桶与 lru /
# InflightGate 的 hold-resolve / directSubmit 的 pump-drain-invalidate / 探针跨锁快照 / 池 purge）
# 每个包先 -list 把入选用例名打出来：正则拼错 = 选中 0 条 = 沉默通过，这里让它当场可见。
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
export WGPU_NATIVE_PATH="$ROOT/lib/libwgpu_native.so"
export LD_LIBRARY_PATH="$ROOT/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.4d/race.txt"

RES_RE='^(TestGate_.*|TestCache_.*|TestRegistry_.*|TestSubmission.*|TestSessionRetireSequence_GatedBySubmitDone|TestIdleCallback_.*|TestReleaseSequenceTiming)$'
GPU_RE='^(TestSurfaceCache_.*|TestTextureSet_.*|TestOffscreenCache_.*|TestDualTexOutPool_.*|TestDirectSubmit_.*|TestSubmitGated_.*|TestGPUShared_.*|TestSessionRegistry_.*|TestRegTransientView_.*|TestRegBorrowed_.*|TestP6_.*|TestImageCache_.*|TestS67_ImageCache_.*|TestSessionFrameBufferShrink|TestSessionShrinkViaBeginFrame|TestShrinkDue_Matrix|TestTexturedQuadPipeline_.*|TestDepthClipMaskCache_.*)$'
RENDER_RE='^(TestGPUPoolStats_.*|TestRegisterGPUPoolStatsCollector_.*|TestPoolDomainStats_.*|TestPurgeEvictables_.*|TestIsGPUOutOfMemory)$'
WRGATE_RE='^(TestBuildReport_PoolSectionFromProbe|TestEvaluatePoolGates)$'

sel() { # sel <pkg> <regex>
  local n
  n=$(go test -list "$2" "./$1" 2>/dev/null | grep -cE '^Test')
  echo "# 入选用例 $n 条:"
  go test -list "$2" "./$1" 2>/dev/null | grep -E '^Test' | sed 's/^/    /'
}

{
  echo "# P2.4d -race"
  echo "# date: $(date -Iseconds)"
  echo "# head: $(git -C "$ROOT" rev-parse --short HEAD)"
  echo "# env: CGO_ENABLED=1 WGPU_NATIVE_PATH set"
  echo
  echo "## render/internal/gpu/res（registry 槽位 + owner 分账 / Cache 桶与 lru / Submission / InflightGate）"
  sel render/internal/gpu/res "$RES_RE"
  CGO_ENABLED=1 go test -race -count=1 -timeout 20m ./render/internal/gpu/res/ -run "$RES_RE"
  echo "# exit: $?"
  echo
  echo "## render/internal/gpu（设备级池：surface trio / offscreen / dualtex / filter / directSubmit / purge 链 / 探针）"
  sel render/internal/gpu "$GPU_RE"
  CGO_ENABLED=1 go test -race -count=1 -timeout 20m ./render/internal/gpu/ -run "$GPU_RE"
  echo "# exit: $?"
  echo
  echo "## render（探针注册表并发）"
  sel render "$RENDER_RE"
  CGO_ENABLED=1 go test -race -count=1 -timeout 20m ./render/ -run "$RENDER_RE"
  echo "# exit: $?"
  echo
  echo "## examples/wrgate（pool 段与门禁）"
  sel examples/wrgate "$WRGATE_RE"
  CGO_ENABLED=1 go test -race -count=1 -timeout 20m ./examples/wrgate/ -run "$WRGATE_RE"
  echo "# exit: $?"
} > "$OUT" 2>&1
tail -60 "$OUT"
