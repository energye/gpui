#!/bin/bash
# P2.3 per-test-file regression for render/internal/gpu (per AGENTS: one file at a time).
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.3/per-file-tests.txt"
cd "$ROOT/render/internal/gpu" || exit 1

{
  echo "# P2.3 per-test-file run: ./render/internal/gpu"
  echo "# date: $(date -Iseconds)"
  echo "# head: $(git -C "$ROOT" rev-parse --short HEAD)"
  echo "# ruler: for each *_test.go, go test -run '^(TestA|TestB|...)$' (benchmarks excluded)"
  echo
  pass=0; fail=0; skip=0; none=0
  for f in *_test.go; do
    names=$(grep -oE '^func Test[A-Za-z0-9_]*\(' "$f" | sed 's/^func //; s/($//' | paste -sd '|')
    if [ -z "$names" ]; then
      out=$(go test -count=1 -run='XXXNONE' . 2>&1); rc=$?
      echo "$f: NO_TEST_FUNCS compile-only rc=$rc"
      [ $rc -eq 0 ] && none=$((none+1)) || fail=$((fail+1))
      continue
    fi
    out=$(go test -count=1 -timeout 10m -run="^($names)\$" . 2>&1); rc=$?
    status=PASS
    [ $rc -ne 0 ] && status=FAIL
    if [ $rc -eq 0 ] && echo "$out" | grep -q '^ok.*no tests to run'; then status=SKIP_MISSING; fi
    echo "$f: $status rc=$rc"
    if [ "$status" = FAIL ]; then
      fail=$((fail+1)); echo "$out" | tail -20 | sed 's/^/    /'
    elif [ "$status" = SKIP_MISSING ]; then
      skip=$((skip+1))
    else
      pass=$((pass+1))
    fi
  done
  echo
  echo "SUMMARY: PASS=$pass NO_TEST=$none SKIP_MISSING=$skip FAIL=$fail"
} > "$OUT" 2>&1

tail -3 "$OUT"
