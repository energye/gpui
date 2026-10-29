#!/bin/bash
# P2.1 API snapshot re-check: rerun the P1.5 collector over the baseline
# 352-package list, then diff against baseline/api with the 30 packages that
# P2.1 deleted (naga tree + shader spirv/hlsl/dxil/internal backends) removed
# from the comparison set. Allowed remaining differences:
#   - gpu/shader* : old shader tree replaced by the naga implementation
#   - render/internal/gpu : CompileShaderToSPIRV removed (D3)
#   - render/render : absence inherited from P2.2 (recorded by P2.3)
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
BASE="$ROOT/docs/render_gpu_converge/baseline/api"
EV="$ROOT/docs/render_gpu_converge/evidence/P2.1"
TMP=/tmp/p21_api_post
BASE_FILTERED=/tmp/p21_api_base_filtered
OUT="$EV/api-diff.txt"
REMOVED="$EV/tools/removed-packages.txt"

rm -rf "$TMP" "$BASE_FILTERED"
mkdir -p "$BASE_FILTERED"

# post package list = baseline list minus P2.1-deleted packages
comm -23 <(sort "$BASE/packages.txt") <(sort "$REMOVED") > /tmp/p21_post_packages.txt
echo "post packages: $(wc -l < /tmp/p21_post_packages.txt) (baseline $(wc -l < "$BASE/packages.txt") minus $(wc -l < "$REMOVED"))"

sed -e "s#^ROOT = .*#ROOT = \"$ROOT\"#" \
    -e "s#^EVID = .*#EVID = \"/tmp/p21_post_packages.txt\"#" \
    -e "s#^OUT = .*#OUT = \"$TMP\"#" \
    "$BASE/tools/gen_api.py" > /tmp/p21_gen_api.py
python3 /tmp/p21_gen_api.py > /tmp/p21_gen_api.log 2>&1
rc=$?
tail -n 3 /tmp/p21_gen_api.log

# filtered baseline: drop deleted packages from doc/ and test_counts.tsv
cp -a "$BASE/doc" "$BASE_FILTERED/doc"
while read -r imp; do
  rel=${imp#github.com/energye/gpui/}
  rm -f "$BASE_FILTERED/doc/$rel.txt"
done < "$REMOVED"
# prune only the deleted trees; other empty dirs mirror go doc's mkdir side effects
for prefix in gpu/gwgpu/naga gpu/shader/dxil gpu/shader/hlsl gpu/shader/spirv \
              gpu/shader/internal/dxcvalidator gpu/shader/internal/backend; do
  find "$BASE_FILTERED/doc/$prefix" -type d -empty -delete 2>/dev/null
done
awk -F'\t' 'NR==1{print;next} {print}' "$BASE/test_counts.tsv" | \
  awk -F'\t' -v OFS='\t' 'NR==FNR{skip[$1]=1;next} !($1 in skip)' \
      "$REMOVED" - > "$BASE_FILTERED/test_counts.tsv"

{
  echo "# P2.1 API snapshot diff vs baseline/api (P1.5)"
  echo "# date: $(date -Iseconds)"
  echo "# method: baseline packages minus the 30 P2.1-deleted packages, full gen_api.py rerun into $TMP"
  echo "#         baseline side filtered the same way: $BASE_FILTERED"
  echo

  echo "## doc/ byte diff (-rq)"
  diff -rq "$BASE_FILTERED/doc" "$TMP/doc"
  echo "# rc=$?"
  echo

  echo "## test_counts.tsv diff (filtered baseline vs post)"
  diff "$BASE_FILTERED/test_counts.tsv" "$TMP/test_counts.tsv"
  echo "# rc=$?"
  echo

  echo "## totals (filtered baseline vs post)"
  python3 - "$BASE_FILTERED/test_counts.tsv" "$TMP/test_counts.tsv" <<'PY'
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
for name, path in (('filtered-baseline', sys.argv[1]), ('post', sys.argv[2])):
    pkgs, files, funcs, kinds = total(path)
    print(f"{name}: packages={pkgs} files={files} funcs={funcs} " +
          "(" + " / ".join(f"{k} {v}" for k, v in kinds.items()) + ")")
PY
  echo

  echo "## empty-main whitelist diff"
  diff "$BASE/doc_empty_main.txt" "$TMP/doc_empty_main.txt"
  echo "# rc=$?"
  echo

  echo "## gpu/shader root symbol-set diff"
  python3 - "$BASE_FILTERED/doc/gpu/shader.txt" "$TMP/doc/gpu/shader.txt" <<'PY'
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

  echo "## render/internal/gpu symbol-set diff (expect only CompileShaderToSPIRV)"
  python3 - "$BASE_FILTERED/doc/render/internal/gpu.txt" "$TMP/doc/render/internal/gpu.txt" <<'PY'
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
} > "$OUT" 2>&1

echo "--- unexpected diffs (must be empty) ---"
fail=0
u=$(diff -rq "$BASE_FILTERED/doc" "$TMP/doc" |
    grep -v '^Files .*/doc/gpu/shader\.txt' | grep -v '^Files .*/doc/gpu/shader/' |
    grep -v 'render/internal/gpu\.txt' | grep -v 'Only in .*doc/render: render.txt')
[ -n "$u" ] && { echo "$u"; fail=1; }
u=$(diff "$BASE_FILTERED/test_counts.tsv" "$TMP/test_counts.tsv" |
    grep -vE '^[0-9,]+[acd][0-9,]+$' | grep -v '^---$' |
    grep -v 'gpu/shader' | grep -v 'render/internal/gpu' | grep -v 'render/render')
[ -n "$u" ] && { echo "counts:"; echo "$u"; fail=1; }
u=$(diff "$BASE/doc_empty_main.txt" "$TMP/doc_empty_main.txt")
[ -n "$u" ] && { echo "empty-main diff:"; echo "$u"; fail=1; }
if [ $fail -eq 0 ]; then
  msg="RESULT: PASS (only gpu/shader* and render/internal/gpu changed; render/render absence inherited from P2.2)"
else
  msg="RESULT: FAIL"
fi
{ echo "## whitelist check"; echo "$msg"; } >> "$OUT"
echo "$msg"
echo "written: $OUT"
exit $fail
