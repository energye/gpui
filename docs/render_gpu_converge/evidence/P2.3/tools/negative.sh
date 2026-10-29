#!/bin/bash
# P2.3 negative control: re-adding deleted symbols must break the build.
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.3/negative.txt"
PROBE="$ROOT/render/internal/gpu/zz_p23_negative_probe.go"
cd "$ROOT" || exit 1

{
  echo "# P2.3 negative control: deleted symbols must be undefined"
  echo "# date: $(date -Iseconds)"
  echo
  echo "# step 1: clean tree builds green"
  go build ./render/... >/dev/null 2>&1; echo "# exit: $?"
  echo
  echo "# step 2: add probe referencing NewBackend / newGPURenderer / NewGPUSceneRenderer"
  cat > "$PROBE" <<'EOF'
//go:build !nogpu

package gpu

var (
	_ = NewBackend
	_ = newGPURenderer
	_ = NewGPUSceneRenderer
)
EOF
  go build ./render/internal/gpu 2>&1
  echo "# exit: $?"
  echo
  echo "# step 3: remove probe, rebuild"
  rm -f "$PROBE"
  go build ./render/... >/dev/null 2>&1; echo "# exit: $?"
  echo
  echo "# step 4: residual probe check"
  if [ -e "$PROBE" ]; then echo "PROBE STILL PRESENT"; else echo "probe removed: clean"; fi
} > "$OUT" 2>&1

cat "$OUT"
