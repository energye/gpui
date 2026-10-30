#!/bin/bash
# P2.4c -race（触及共享状态机的改动：registry 槽位 / submission / cache idle 回池 / 会话延迟释放）
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
export WGPU_NATIVE_PATH="$ROOT/lib/libwgpu_native.so"
export LD_LIBRARY_PATH="$ROOT/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.4c/race.txt"

{
  echo "# P2.4c -race"
  echo "# date: $(date -Iseconds)"
  echo "# env: CGO_ENABLED=1 WGPU_NATIVE_PATH set"
  echo
  echo "## render/internal/gpu/res（registry/submission/cache 状态机 + idle 回调）"
  CGO_ENABLED=1 go test -race -count=1 ./render/internal/gpu/res/ \
    -run '^(TestSessionRetireSequence_GatedBySubmitDone|TestIdleCallback_|TestSubmission.*|TestCache_.*|TestRegistry_.*|TestReleaseSequenceTiming)$'
  echo "# exit: $?"
  echo
  echo "## render/internal/gpu（会话 Retire/Restore + ImageCache retire 钩子 + 帧缓冲收缩）"
  echo "# 既有红排除：TestTextureSet_SurfaceToOffscreenSameSize 等 stencil 分配断言在 HEAD 2204ad5a 干净树同样失败（环境，非本步回归），见 per-file-tests-notes.md"
  CGO_ENABLED=1 go test -race -count=1 -timeout 20m ./render/internal/gpu/ \
    -run '^(TestP6_.*|TestImageCache_.*|TestTextureSet_DestroyWithRetireFnDefersRelease|TestTextureSet_DestroyWithoutRetireFnReleasesImmediately|TestTextureSet_PruneStencilPoolExcept.*|TestTextureSet_FailFastLatch|TestTextureSet_DegradedLatch|TestS67_ImageCache_.*|TestRegTransientView_.*|TestRegBorrowed_RetireSkipsNativeRelease|TestSessionFrameBufferShrink|TestSessionShrinkViaBeginFrame|TestShrinkDue_Matrix|TestTexturedQuadPipeline_DestroyReleasesAllOwnedResources)$'
  echo "# exit: $?"
} > "$OUT" 2>&1
tail -6 "$OUT"