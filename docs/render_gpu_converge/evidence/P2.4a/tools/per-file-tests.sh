#!/bin/bash
# P2.4a per-test-file regression (AGENTS: one *_test.go at a time).
# Scope: the three touched packages (render root, render/gpu, render/internal/gpu).
# Env: native wgpu lib + display available (window tests run per P1.3 discipline;
# one file = one process, no full-suite batch).
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
export WGPU_NATIVE_PATH="$ROOT/lib/libwgpu_native.so"
export LD_LIBRARY_PATH="$ROOT/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.4a/per-file-tests.txt"

PKGS=(
  render
  render/gpu
  render/internal/gpu
)

{
  echo "# P2.4a per-test-file run"
  echo "# date: $(date -Iseconds)"
  echo "# head: $(git -C "$ROOT" rev-parse --short HEAD)"
  echo "# env: WGPU_NATIVE_PATH set, DISPLAY=${DISPLAY:-unset}"
  echo "# ruler: for each *_test.go, go test -run '^(TestA|TestB|...)$' (benchmarks excluded);"
  echo "#        build-tag-excluded files show as SKIP_TAGGED"
  echo
  pass=0; fail=0; skip=0; none=0
  total_pass=0; total_fail=0; total_skip=0; total_none=0
  for pkg in "${PKGS[@]}"; do
    cd "$ROOT/$pkg" || { echo "MISSING PACKAGE $pkg"; exit 1; }
    echo "## $pkg"
    for f in *_test.go; do
      names=$(grep -oE '^func Test[A-Za-z0-9_]*\(' "$f" | sed 's/^func //; s/($//' | paste -sd '|')
      if [ -z "$names" ]; then
        out=$(go test -count=1 -run='XXXNONE' . 2>&1); rc=$?
        echo "$f: NO_TEST_FUNCS compile-only rc=$rc"
        [ $rc -eq 0 ] && none=$((none+1)) || { fail=$((fail+1)); echo "$out" | tail -20 | sed 's/^/    /'; }
        continue
      fi
      out=$(go test -count=1 -timeout 20m -run="^($names)\$" . 2>&1); rc=$?
      status=PASS
      [ $rc -ne 0 ] && status=FAIL
      if [ $rc -eq 0 ] && echo "$out" | grep -Eq 'no tests to run|\[no test files\]'; then status=SKIP_TAGGED; fi
      if [ $rc -eq 0 ] && ! echo "$out" | grep -q '^ok' && [ "$status" = PASS ]; then status=FAIL; fi
      echo "$f: $status rc=$rc"
      if [ "$status" = FAIL ]; then
        fail=$((fail+1)); echo "$out" | tail -30 | sed 's/^/    /'
      elif [ "$status" = SKIP_TAGGED ]; then
        skip=$((skip+1))
      else
        pass=$((pass+1))
      fi
    done
    echo
    echo "  subtotal: PASS=$pass NO_TEST=$none SKIP_TAGGED=$skip FAIL=$fail"
    total_pass=$((total_pass+pass)); total_fail=$((total_fail+fail))
    total_skip=$((total_skip+skip)); total_none=$((total_none+none))
    pass=0; fail=0; skip=0; none=0
  done
  echo "SUMMARY: PASS=$total_pass NO_TEST=$total_none SKIP_TAGGED=$total_skip FAIL=$total_fail"
} > "$OUT" 2>&1

tail -3 "$OUT"
