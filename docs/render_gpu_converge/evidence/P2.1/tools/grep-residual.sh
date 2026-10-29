#!/bin/bash
# P2.1 residual grep: no Go code may reference the deleted trees or symbols.
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.1/grep-residual.txt"
cd "$ROOT" || exit 1

{
  echo "# P2.1 residual grep (Go code + shell/test data, excluding /docs/ evidence)"
  echo "# date: $(date -Iseconds)"
  echo
  echo "## deleted import paths"
  for p in 'gpu/gwgpu/naga' 'gpu/shader/spirv' 'gpu/shader/hlsl' 'gpu/shader/dxil' \
           'gpu/shader/internal/backend' 'gpu/shader/internal/dxcvalidator'; do
    hits=$(grep -rn --include=*.go "$p" . | grep -v '/docs/' | grep -v '^./.p21' | wc -l)
    printf "%-34s hits=%s\n" "$p" "$hits"
    [ "$hits" != "0" ] && grep -rn --include=*.go "$p" . | grep -v '/docs/' | sed 's/^/    /'
  done
  echo
  echo "## deleted root symbols (P2.1 D3) and old package qualifier"
  for s in CompileShaderToSPIRV CompileWithOptions GenerateSPIRV dxcvalidator; do
    hits=$(grep -rnF --include=*.go "$s" . | grep -v '/docs/' | grep -v '^./.p21' | wc -l)
    printf "%-26s hits=%s\n" "$s" "$hits"
    [ "$hits" != "0" ] && grep -rnF --include=*.go "$s" . | grep -v '/docs/' | sed 's/^/    /'
  done
  hits=$(grep -rnE --include=*.go '\bnaga\.' . | grep -v '/docs/' | grep -v '^./.p21' | wc -l)
  printf "%-26s hits=%s\n" 'naga.' "$hits"
  [ "$hits" != "0" ] && grep -rnE --include=*.go '\bnaga\.' . | grep -v '/docs/' | sed 's/^/    /'
  echo
  echo "## note: the single 'naga.' hit is a code comment in"
  echo "##       gpu/shader/wgsl/internal/lower/lower.go:9329 ('matches Rust naga'), not a qualifier."
  echo "## note: root shader.Compile is intentionally gone (SPIR-V only);"
  echo "##       msl.DefaultOptions / glsl.Compile remain and are the live API."
  echo
  echo "## deleted dirs on disk"
  for d in gpu/gwgpu/naga gpu/shader/spirv gpu/shader/hlsl gpu/shader/dxil \
           gpu/shader/internal/backend gpu/shader/internal/dxcvalidator; do
    if [ -e "$d" ]; then echo "$d: STILL PRESENT"; else echo "$d: absent"; fi
  done
  echo
  echo "## temporary snapshot work dirs (must be absent before commit)"
  for d in .p21_tmp .p21_tmp_after; do
    if [ -e "$d" ]; then echo "$d: STILL PRESENT"; else echo "$d: absent"; fi
  done
  echo
  echo "## live importers of the merged package"
  grep -rln --include=*.go '"github.com/energye/gpui/gpu/shader' . | grep -v '/docs/' | sort
} > "$OUT" 2>&1

sed -n '1,60p' "$OUT"
