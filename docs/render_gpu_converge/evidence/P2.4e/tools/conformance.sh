#!/bin/bash
# P2.4d conformance 双后端（native + go）重跑：D7 要求「池改造后画面逐字节不变」，
# 金图替代判定 = conformance 三场景 frame.png 与 P0.4 基线一致 + 资源上界 0/0 不回退。
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.4e/conformance"
mkdir -p "$OUT"
export WGPU_NATIVE_PATH="$ROOT/lib/libwgpu_native.so"
export LD_LIBRARY_PATH="$ROOT/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
cd "$ROOT" || exit 1

{
  echo "# P2.4d conformance dual backend"
  echo "# date: $(date -Iseconds)"
  echo "# head: $(git rev-parse --short HEAD)"
  echo "# 上界口径：P2.4c 已把 solid_gradient/text_image 的 max_steady_count_delta/bytes 收紧到 0/0"
  echo
  echo "## 双后端（GPUI_CONFORMANCE_BACKENDS 默认 native+go）"
  go test -count=1 -timeout 30m -v -run '^TestConformanceDualBackend$' ./render 2>&1
  echo "# exit: $?"
  echo
  echo "## 三场景逐字节对账（frame.png vs 提交前基线）"
  git status --porcelain render/testdata/conformance | sed 's/^/    /'
  echo "    （上面只应出现 json 的上界/说明改动，不该出现 frame.png 改动）"
  for d in render/testdata/conformance/*/; do
    echo "    $d: $(git status --porcelain "$d" | wc -l) 项变更"
  done
} > "$OUT/rerun-after-pool-convergence.txt" 2>&1
tail -20 "$OUT/rerun-after-pool-convergence.txt"
