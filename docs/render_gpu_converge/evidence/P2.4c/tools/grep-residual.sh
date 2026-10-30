#!/bin/bash
# P2.4c residual grep: the three bare deferred-release queues must be gone;
# the registry is the only deferred-release outlet.
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.4c/grep-residual.txt"
cd "$ROOT" || exit 1

{
  echo "# P2.4c residual grep"
  echo "# date: $(date -Iseconds)"
  echo
  echo "## 1) 旧队列/旧释放口符号（期望 0 命中；docs/ 与基线快照不算）"
  for pat in 'pendingTexRetire' 'pendingBufRetire' 'ReleaseEphemeral' 'OnSubmissionFinished' 'EphemeralPending' 'EphemeralPending:'; do
    n=$(grep -rn --include='*.go' -- "$pat" render/ gpu/ ui/ 2>/dev/null | grep -v '_test.go' | wc -l)
    echo "$pat: $n 命中"
    grep -rn --include='*.go' -- "$pat" render/ gpu/ ui/ 2>/dev/null | grep -v '_test.go' | sed 's/^/    /'
  done
  echo
  echo "## 2) 裸 Drain()（期望 0 命中：释放只在 SubmitDone 走 registry）"
  n=$(grep -rn --include='*.go' '\.Drain()' render/internal/gpu 2>/dev/null | grep -v '_test.go' | wc -l)
  echo ".Drain(): $n 命中"
  grep -rn --include='*.go' '\.Drain()' render/internal/gpu 2>/dev/null | grep -v '_test.go' | sed 's/^/    /'
  echo
  echo "## 3) 延迟释放唯一入口调用点"
  echo "retireNative 调用点:"
  grep -rn --include='*.go' 'retireNative(' render/internal/gpu | grep -v '_test.go' | sed 's/^/    /'
  echo "SubmitDone 同步点:"
  grep -rn --include='*.go' 'SubmitDone()' render/internal/gpu | grep -v '_test.go' | sed 's/^/    /'
  echo
  echo "## 4) 分层：render 非测试代码中的后端特有类型（期望 0；创建入口除外）"
  grep -rnE --include='*.go' '\bwebgpu\.|\bgwgpu/|\bgles\.' render/*.go 2>/dev/null |
    grep -v '_test.go' | grep -v 'present_swapchain' | grep -v 'CreateInstance' | grep -v '_ "' | sed 's/^/    /'
  echo "(空 = 合规；注意 'rectangles.' 这类英文词不是 gles 引用，用词边界排除)"
} > "$OUT"
cat "$OUT"