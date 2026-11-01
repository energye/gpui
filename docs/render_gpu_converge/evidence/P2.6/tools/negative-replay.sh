#!/bin/bash
# P2.6-f 负向对照重放（P1.8 三破坏，口径照 baseline/negative/summary.md「复现」节）。
# 每项破坏→必红→当场还原→复绿留档。破坏 3 说明：P1.8 的 K1 上界收紧旋钮
# （solid_gradient.json 131072→0）在 P2.4c 修复 K1 后上界已是 0/0，改用
# 「临时摘掉 TexturedQuadPipeline.destroyPipeline 的 bicubic 两管线+shader 销毁」
# 注入真实少释放（P2.4c K1 的原洞），一致性台 resource 断言必须红。
set -u
export LC_ALL=C
ROOT=/home/yanghy/app/projects/gogpu/gpui
cd "$ROOT"
EV=docs/render_gpu_converge/evidence/P2.6
export WGPU_NATIVE_PATH="$ROOT/lib/libwgpu_native.so"
export LD_LIBRARY_PATH="$ROOT/lib:${LD_LIBRARY_PATH:-}"
mkdir -p "$EV/f-negative"
CONF='go test ./render -run ^TestConformanceDualBackend$ -count=1'

{
  echo "# P2.6-f 负向对照重放"
  echo "# date: $(date -Iseconds)  head: $(git rev-parse --short HEAD)"

  echo; echo "## 绿基线（无破坏）"
  $CONF > "$EV/f-negative/green-baseline.txt" 2>&1; echo "rc=$? (expect 0)"

  echo; echo "## 破坏 1：画面偏移 1px（GPUI_NEG_SHIFT_1PX=1，patch.diff 打入）"
  if git apply --check baseline/negative/patch.diff 2>/dev/null; then
    git apply baseline/negative/patch.diff
    GPUI_NEG_SHIFT_1PX=1 $CONF > "$EV/f-negative/shift1px-red.txt" 2>&1
    echo "red rc=$? (expect 1)"; grep -E "coverage|≠|FAIL" "$EV/f-negative/shift1px-red.txt" | head -3
    GPUI_NEG_SHIFT_1PX= $CONF > "$EV/f-negative/shift1px-green.txt" 2>&1
    echo "env 关 rc=$? (expect 0)"
    git apply -R baseline/negative/patch.diff
    echo "patch 还原: $(git status --porcelain render/internal/conformance render/conformance_test.go | wc -l) 处残留 (expect 0)"
  else
    echo "patch.diff 无法干净应用（conformance 代码在 P2 有变）——按 patch 内容手工重建破坏后重试"
    echo "patch check: $(git apply --check baseline/negative/patch.diff 2>&1 | head -3)"
  fi

  echo; echo "## 破坏 2：跳过一次绘制（GPUI_NEG_SKIP_DRAW=1，同补丁）"
  if git apply --check baseline/negative/patch.diff 2>/dev/null; then
    git apply baseline/negative/patch.diff
    GPUI_NEG_SKIP_DRAW=1 $CONF > "$EV/f-negative/skipdraw-red.txt" 2>&1
    echo "red rc=$? (expect 1)"
    git apply -R baseline/negative/patch.diff
  fi

  echo; echo "## 破坏 3：少释放一次（临时注释 bicubic 两管线+shader 销毁，P2.4c K1 原洞）"
  python3 - <<'PY'
p='render/internal/gpu/image_pipeline.go'
s=open(p).read()
old="""\tif p.pipelineWithBicubicDepthClip != nil {
\t\tp.pipelineWithBicubicDepthClip.Destroy()
\t\tp.pipelineWithBicubicDepthClip = nil
\t}
\tif p.pipelineWithBicubic != nil {
\t\tp.pipelineWithBicubic.Destroy()
\t\tp.pipelineWithBicubic = nil
\t}"""
assert old in s, "bicubic destroy block not found"
open('/tmp/p26f-orig.go','w').write(s)
s=s.replace(old, """\t// P2.6-f negative: K1 leak injection (destroy calls temporarily disabled)
\t_ = p.pipelineWithBicubicDepthClip
\t_ = p.pipelineWithBicubic""")
open(p,'w').write(s)
print("K1 leak injected")
PY
  $CONF > "$EV/f-negative/skipforget-red.txt" 2>&1
  echo "red rc=$? (expect 1)"; grep -E "resource|steady|FAIL" "$EV/f-negative/skipforget-red.txt" | head -3
  cp /tmp/p26f-orig.go "$(grep -rln 'func.*destroyPipeline' render/internal/gpu/ | head -1)"
  git diff --stat render/internal/gpu | tail -1
  $CONF > "$EV/f-negative/skipforget-green.txt" 2>&1
  echo "还原 rc=$? (expect 0)"

  echo; echo "## §8.2 阈值变异对（extractor 读 thresholds.md）"
  python3 docs/render_gpu_converge/baseline/perf/tools/extract_perf.py > "$EV/f-negative/threshold-normal.txt" 2>&1
  echo "正常态 p95_fail=$(grep -c 'p95_fail' "$EV/f-negative/threshold-normal.txt" || true)（对照基线 4）"
} > "$EV/f-negative/negative-replay.txt" 2>&1
tail -30 "$EV/f-negative/negative-replay.txt"
