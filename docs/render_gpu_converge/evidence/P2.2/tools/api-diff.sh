#!/bin/bash
# P2.2 API snapshot re-check: rerun the P1.5 snapshot collector with
# render/render -> render/hdr, then diff against baseline/api.
# Full post snapshot goes to /tmp (5.5MB duplicate of the 350 unchanged docs
# is not committed); only the changed artifacts live in evidence/P2.2/api-post/.
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
BASE="$ROOT/docs/render_gpu_converge/baseline/api"
TMP=/tmp/p22_api_post
PKGS=/tmp/p22_packages.txt

python3 - "$BASE/packages.txt" "$PKGS" <<'PY'
import sys
src, dst = sys.argv[1], sys.argv[2]
M = "github.com/energye/gpui"
pkgs = [x.strip() for x in open(src, encoding="utf-8") if x.strip()]
assert M + "/render/render" in pkgs
assert M + "/render/hdr" not in pkgs
pkgs = [p for p in pkgs if p != M + "/render/render"] + [M + "/render/hdr"]
open(dst, "w", encoding="utf-8").write("\n".join(sorted(pkgs)) + "\n")
PY

rm -rf "$TMP"
sed -e "s#^ROOT = .*#ROOT = \"$ROOT\"#" \
    -e "s#^EVID = .*#EVID = \"$PKGS\"#" \
    -e "s#^OUT = .*#OUT = \"$TMP\"#" \
    "$BASE/tools/gen_api.py" > /tmp/p22_gen_api.py
python3 /tmp/p22_gen_api.py | tail -n 3

fail=0
extra=$(diff -rq "$BASE/doc" "$TMP/doc" | grep -vE 'hdr\.txt|render\.txt')
if [ -n "$extra" ]; then echo "UNEXPECTED doc diff:"; echo "$extra"; fail=1; fi
extra=$(diff "$BASE/test_counts.tsv" "$TMP/test_counts.tsv" | grep -vE '^[0-9,]+[acd][0-9,]+$' | grep -vE 'render/hdr|render/render')
if [ -n "$extra" ]; then echo "UNEXPECTED test_counts diff:"; echo "$extra"; fail=1; fi
extra=$(diff "$BASE/doc_empty_main.txt" "$TMP/doc_empty_main.txt")
if [ -n "$extra" ]; then echo "UNEXPECTED empty-main whitelist diff:"; echo "$extra"; fail=1; fi

echo "--- expected whitelist diff ---"
diff -rq "$BASE/doc" "$TMP/doc"
diff "$BASE/test_counts.tsv" "$TMP/test_counts.tsv"
if [ "$fail" -ne 0 ]; then echo "RESULT: FAIL"; exit 1; fi
echo "RESULT: PASS (only render/render removed, render/hdr added)"
