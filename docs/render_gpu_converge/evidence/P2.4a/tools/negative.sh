#!/bin/bash
# P2.4a negative control: the deleted mirror/dead symbols must be unreachable
# (compile red), and the funnel must be the only webgpu.CreateInstance call.
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.4a/negative.txt"
PROBE="$ROOT/render/internal/gpu/zz_p24a_negative_probe.go"
cd "$ROOT" || exit 1

{
  echo "# P2.4a negative control"
  echo "# date: $(date -Iseconds)"
  echo
  echo "# step 1: clean tree builds green"
  go build ./render/... >/dev/null 2>&1; echo "# exit: $?"
  echo
  echo "# step 2: probe references deleted mirror/dead symbols (must be compile red)"
  cat > "$PROBE" <<'GOEOF'
//go:build !nogpu

package gpu

import "github.com/energye/gpui/gpu/hal"

func zzP24aProbe() {
	var _ func(string) *hal.DeviceDescriptor = renderDeviceDescriptor
	_ = createDevice
	_ = getDeviceQueue
	_ = releaseAdapter
	var _ GPUInfo
}
GOEOF
  go build ./render/internal/gpu 2>&1
  echo "# exit: $?  (expect non-zero: symbols deleted)"
  echo
  echo "# step 3: remove probe, rebuild green"
  rm -f "$PROBE"
  go build ./render/internal/gpu 2>&1
  echo "# exit: $?  (expect 0)"
  echo
  echo "# step 4: residue grep after revert (expect empty)"
  grep -rn "zz_p24a_negative_probe\|zzP24aProbe" --include="*.go" "$ROOT/render" 2>/dev/null
  echo "# grep exit: $?  (1 = zero residue)"
  echo
  echo "# step 5: funnel guard — webgpu.CreateInstance outside backend.go (expect only comment/doc hits)"
  grep -rn "webgpu.CreateInstance" --include="*.go" "$ROOT/render" | grep -v _test.go | grep -v "^.*backend.go" | grep -v "^\s*//" | grep -v "// "
  echo "# grep exit: $?  (1 = only the single funnel sentence remains)"
} > "$OUT" 2>&1
cat "$OUT"
