#!/bin/bash
# P2.1 negative control: re-adding deleted imports/symbols must break the build,
# removing the probe must restore green with zero residue.
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.1/negative.txt"
PROBE="$ROOT/gpu/shader/zz_p21_negative_probe.go"
cd "$ROOT" || exit 1

{
  echo "# P2.1 negative control: deleted packages/symbols must be unreachable"
  echo "# date: $(date -Iseconds)"
  echo
  echo "# step 1: clean tree builds green"
  go build ./gpu/shader/... ./gpu/gwgpu/gles/... ./gpu/gwgpu/metal/... >/dev/null 2>&1; echo "# exit: $?"
  echo
  echo "# step 2: probe re-imports gpu/shader/spirv + gpu/gwgpu/naga (deleted trees)"
  cat > "$PROBE" <<'EOF'
package shader

import (
	_ "github.com/energye/gpui/gpu/gwgpu/naga"
	_ "github.com/energye/gpui/gpu/shader/spirv"
)
EOF
  go build ./gpu/shader 2>&1
  echo "# exit: $?"
  echo
  echo "# step 3: probe references removed root symbols (Compile / GenerateSPIRV / CompileWithOptions)"
  cat > "$PROBE" <<'EOF'
package shader

var (
	_ = Compile
	_ = CompileWithOptions
	_ = GenerateSPIRV
)
EOF
  go build ./gpu/shader 2>&1
  echo "# exit: $?"
  echo
  echo "# step 4: probe calls removed render/internal/gpu.CompileShaderToSPIRV"
  rm -f "$PROBE"
  PROBE2="$ROOT/render/internal/gpu/zz_p21_negative_probe.go"
  cat > "$PROBE2" <<'EOF'
//go:build !nogpu

package gpu

var _ = CompileShaderToSPIRV
EOF
  go build ./render/internal/gpu 2>&1
  echo "# exit: $?"
  rm -f "$PROBE2"
  echo
  echo "# step 5: probes removed, rebuild green"
  go build ./gpu/shader/... ./gpu/gwgpu/gles/... ./gpu/gwgpu/metal/... ./render/internal/gpu >/dev/null 2>&1; echo "# exit: $?"
  echo
  echo "# step 6: residual probe check"
  for p in "$PROBE" "$PROBE2"; do
    if [ -e "$p" ]; then echo "$p STILL PRESENT"; else echo "$(basename "$p"): removed clean"; fi
  done
} > "$OUT" 2>&1

cat "$OUT"
