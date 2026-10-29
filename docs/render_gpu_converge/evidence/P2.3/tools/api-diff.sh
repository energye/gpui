#!/bin/bash
# P2.3 API snapshot re-check: rerun the P1.5 collector over the unchanged
# 352-package list, then diff against baseline/api. Only render/internal/gpu is
# allowed to differ (10 source + 10 test files removed, 4 Stub*ID types moved).
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
BASE="$ROOT/docs/render_gpu_converge/baseline/api"
TMP=/tmp/p23_api_post
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.3/api-diff.txt"

rm -rf "$TMP"
sed -e "s#^ROOT = .*#ROOT = \"$ROOT\"#" \
    -e "s#^EVID = .*#EVID = \"$BASE/packages.txt\"#" \
    -e "s#^OUT = .*#OUT = \"$TMP\"#" \
    "$BASE/tools/gen_api.py" > /tmp/p23_gen_api.py
python3 /tmp/p23_gen_api.py > /tmp/p23_gen_api.log 2>&1
rc=$?
tail -n 3 /tmp/p23_gen_api.log

{
  echo "# P2.3 API snapshot diff vs baseline/api (P1.5)"
  echo "# date: $(date -Iseconds)"
  echo "# method: packages.txt unchanged (352), full gen_api.py rerun into $TMP"
  echo

  echo "## doc/ byte diff (-rq)"
  diff -rq "$BASE/doc" "$TMP/doc"
  echo "# rc=$?"
  echo

  echo "## doc/ symbol-set diff (render/internal/gpu only)"
  python3 - "$BASE/doc/render/internal/gpu.txt" "$TMP/doc/render/internal/gpu.txt" <<'PY'
import re, sys
R = re.compile(r'^(func|type|const|var)\s+(?:\([^)]*\)\s+)?([A-Za-z_][A-Za-z0-9_]*)')
def syms(path):
    out = set()
    for line in open(path, encoding='utf-8'):
        m = R.match(line)
        if m:
            out.add((m.group(1), m.group(2)))
    return out
a, b = syms(sys.argv[1]), syms(sys.argv[2])
removed = sorted(a - b)
added = sorted(b - a)
print(f"baseline symbols: {len(a)}")
print(f"post symbols:     {len(b)}")
print(f"removed: {len(removed)}")
for k, n in removed:
    print(f"  - {k} {n}")
print(f"added: {len(added)}")
for k, n in added:
    print(f"  + {k} {n}")
PY
  echo

  echo "## test_counts.tsv diff"
  diff "$BASE/test_counts.tsv" "$TMP/test_counts.tsv"
  echo "# rc=$?"
  echo

  echo "## totals"
  python3 - "$BASE/test_counts.tsv" "$TMP/test_counts.tsv" <<'PY'
import sys
def total(path):
    files = funcs = 0
    kinds = {'Test':0,'Benchmark':0,'Example':0,'Fuzz':0}
    for line in list(open(path, encoding='utf-8'))[1:]:
        p = line.rstrip('\n').split('\t')
        if len(p) < 6:
            continue
        files += int(p[1]); funcs += sum(int(x) for x in p[2:6])
        for k, i in zip(kinds, range(2, 6)):
            kinds[k] += int(p[i])
    pkgs = sum(1 for _ in open(path, encoding='utf-8')) - 1
    return pkgs, files, funcs, kinds
for name, path in (('baseline', sys.argv[1]), ('post', sys.argv[2])):
    pkgs, files, funcs, kinds = total(path)
    print(f"{name}: packages={pkgs} files={files} funcs={funcs} " +
          "(" + " / ".join(f"{k} {v}" for k, v in kinds.items()) + ")")
PY
  echo

  echo "## empty-main whitelist diff"
  diff "$BASE/doc_empty_main.txt" "$TMP/doc_empty_main.txt"
  echo "# rc=$?"
} > "$OUT" 2>&1

echo "--- unexpected diffs (must be empty) ---"
fail=0
u=$(diff -rq "$BASE/doc" "$TMP/doc" | grep -v 'render/internal/gpu\.txt' | grep -v 'doc/render: render.txt')
[ -n "$u" ] && { echo "$u"; fail=1; }
u=$(diff "$BASE/test_counts.tsv" "$TMP/test_counts.tsv" |
    grep -vE '^[0-9,]+[acd][0-9,]+$' | grep -v '^---$' |
    grep -v 'render/internal/gpu' | grep -v 'render/render')
[ -n "$u" ] && { echo "$u"; fail=1; }
u=$(diff "$BASE/doc_empty_main.txt" "$TMP/doc_empty_main.txt")
[ -n "$u" ] && { echo "empty-main diff:"; echo "$u"; fail=1; }
if [ $fail -eq 0 ]; then
  msg="RESULT: PASS (only render/internal/gpu changed; render/render absence inherited from P2.2)"
else
  msg="RESULT: FAIL"
fi
{ echo; echo "## whitelist check"; echo "$msg"; } >> "$OUT"
echo "$msg"
echo "written: $OUT"
exit $fail
