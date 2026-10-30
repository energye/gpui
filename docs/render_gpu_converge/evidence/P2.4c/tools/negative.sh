#!/bin/bash
# P2.4c negative control: remove the registry in-flight gate (the single gate
# that every deferred release now depends on) and prove the state-machine
# tests turn red, then restore and prove green. Conformance-level negative
# control (one Release skipped → steady count/bytes delta red) is recorded in
# negative/conformance-negative.txt from the K1 trace work.
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
export WGPU_NATIVE_PATH="$ROOT/lib/libwgpu_native.so"
export LD_LIBRARY_PATH="$ROOT/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.4c/negative/registry-gate-negative.txt"
F="$ROOT/render/internal/gpu/res/registry.go"
BAK=$(mktemp /tmp/registry.go.XXXXXX)
cp "$F" "$BAK"
mkdir -p "$(dirname "$OUT")"

restore() { cp "$BAK" "$F"; rm -f "$BAK"; }
trap restore EXIT

{
  echo "# P2.4c negative control: registry in-flight gate"
  echo "# date: $(date -Iseconds)"
  echo "# target test: TestSessionRetireSequence_GatedBySubmitDone + TestRegistry_InflightBlocksReleaseUntilSubmitDone"
  echo

  echo "## step 1: clean state green"
  go test -count=1 ./render/internal/gpu/res -run '^(TestSessionRetireSequence_GatedBySubmitDone|TestRegistry_InflightBlocksReleaseUntilSubmitDone)$'
  echo "# exit: $? (expect 0)"
  echo

  echo "## step 2: remove the inflight gate in removeIfReady (temporary patch)"
  sed -i 's/if s == nil || !s.retire || s.refs != 0 || s.inflight != 0 {/if s == nil || !s.retire || s.refs != 0 {/' "$F"
  grep -n 'func (r \*Registry) removeIfReady' -A 2 "$F" | sed 's/^/    /'
  go test -count=1 ./render/internal/gpu/res -run '^(TestSessionRetireSequence_GatedBySubmitDone|TestRegistry_InflightBlocksReleaseUntilSubmitDone)$'
  echo "# exit: $? (expect 1: release escaped the submission gate)"
  echo

  restore
  trap - EXIT

  echo "## step 3: restored, green again"
  go test -count=1 ./render/internal/gpu/res -run '^(TestSessionRetireSequence_GatedBySubmitDone|TestRegistry_InflightBlocksReleaseUntilSubmitDone)$'
  echo "# exit: $? (expect 0)"
  echo

  echo "## step 4: residue grep (expect 0 hits of the patched form)"
  grep -n 'if s == nil || !s.retire || s.refs != 0 {' "$F" | wc -l
} > "$OUT" 2>&1
cat "$OUT"