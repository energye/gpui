#!/bin/bash
# P2.4b negative control: mutating one declaration-table number must turn
# the pin test red; reverting restores green with zero residue.
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.4b/negative.txt"
BUDGET="$ROOT/render/budget.go"
cd "$ROOT" || exit 1

{
  echo "# P2.4b negative control: budget table mutation"
  echo "# date: $(date -Iseconds)"
  echo
  echo "# step 1: clean pin test green"
  go test ./render/ -run 'TestDefaultBudget_PinnedToPreP24bValues' -count=1 2>&1 | tail -1
  echo
  echo "# step 2: mutate BudgetImageCache.MaxBytes 64MiB -> 32MiB (temporary patch)"
  sed -i 's/return Budget{MaxEntries: 128, MaxBytes: 64 << 20, MinEntries: 16, MinBytes: 8 << 20}/return Budget{MaxEntries: 128, MaxBytes: 32 << 20, MinEntries: 16, MinBytes: 8 << 20}/' "$BUDGET"
  go test ./render/ -run 'TestDefaultBudget_PinnedToPreP24bValues' -count=1 > /tmp/p24b_neg.out 2>&1
  rc=$?
  tail -3 /tmp/p24b_neg.out
  echo "# test exit: $rc  (expect 1: pin caught the drift)"
  echo
  echo "# step 3: revert, pin test green again"
  sed -i 's/return Budget{MaxEntries: 128, MaxBytes: 32 << 20, MinEntries: 16, MinBytes: 8 << 20}/return Budget{MaxEntries: 128, MaxBytes: 64 << 20, MinEntries: 16, MinBytes: 8 << 20}/' "$BUDGET"
  go test ./render/ -run 'TestDefaultBudget_PinnedToPreP24bValues' -count=1 2>&1 | tail -1
  echo
  echo "# step 4: residue grep (expect no 32 MiB image budget left)"
  grep -n "MaxBytes: 32 << 20, MinEntries: 16" "$BUDGET"
  echo "# grep exit: $?  (1 = zero residue)"
} > "$OUT" 2>&1
cat "$OUT"
