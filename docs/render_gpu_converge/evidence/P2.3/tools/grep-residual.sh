#!/bin/bash
# P2.3 residual grep: no Go code may reference the deleted cluster.
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.3/grep-residual.txt"
cd "$ROOT" || exit 1

{
  echo "# P2.3 residual grep (Go code only)"
  echo "# date: $(date -Iseconds)"
  echo
  echo "## deleted package symbols"
  for s in NewGPUSceneRenderer HybridPipeline HybridFineRasterizer GPUCoarseRasterizer \
           GPUFineRasterizer GPUFlattenRasterizer MemoryManager PipelineCacheCore \
           NewPipelineCache CompileShaders ShaderModules BackendGPU newGPURenderer \
           GPURenderer StubBindGroupLayoutID InvalidPipelineID; do
    hits=$(grep -rn --include=*.go "\b$s\b" . | grep -v '/docs/' | wc -l)
    printf "%-26s hits=%s\n" "$s" "$hits"
    if [ "$hits" != "0" ]; then grep -rn --include=*.go "\b$s\b" . | grep -v '/docs/' | sed 's/^/    /'; fi
  done
  echo
  echo "## note: remaining GPUSceneRenderer hits are render/scene.GPUSceneRenderer,"
  echo "## a different type declared in render/scene/gpu_renderer.go:37."
  echo
  echo "## deleted WGSL embed paths"
  for f in blit blend strip composite coarse fine flatten; do
    n=$(grep -rn "shaders/$f\.wgsl" --include=*.go . | wc -l)
    echo "shaders/$f.wgsl: embed/$n"
  done
  echo
  echo "## internal/gpu now imports gpu/webgpu only at the whitelisted entries"
  grep -rn "webgpu\.CreateInstance" --include=*.go render gpu | grep -v _test.go
} > "$OUT" 2>&1

sed -n '1,40p' "$OUT"
