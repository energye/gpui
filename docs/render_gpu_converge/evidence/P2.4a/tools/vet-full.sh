#!/bin/bash
# P2.4a full-repo go vet (default / -tags nogpu), same ruler as P2.1/P2.3:
# P1.2b package list minus ui_wr_t_stress, render/render (P2.2) and the 30
# packages P2.1 deleted. P2.4a changed no package set (symbols only), so the
# expectation is 61 / 58 diagnostics, byte-identical to P2.3/P2.1.
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
EV="$ROOT/docs/render_gpu_converge/evidence/P2.4a"
P12="$ROOT/docs/render_gpu_converge/evidence/P1.2b"
REMOVED="$ROOT/docs/render_gpu_converge/evidence/P2.1/tools/removed-packages.txt"
OUT="$EV/vet-full.txt"
cd "$ROOT" || exit 1

rm -f /tmp/p24a_exclude.txt
grep '^github.com/energye/gpui/render/render$' "$P12/packages-default.txt" >> /tmp/p24a_exclude.txt
cat "$REMOVED" >> /tmp/p24a_exclude.txt
sort -u /tmp/p24a_exclude.txt -o /tmp/p24a_exclude.txt

grep -v 'ui_wr_t_stress' "$P12/packages-default.txt" | sort > /tmp/p24a_pkgs_default_all.txt
comm -23 /tmp/p24a_pkgs_default_all.txt /tmp/p24a_exclude.txt > /tmp/p24a_pkgs_default.txt

grep -v 'ui_wr_t_stress' "$P12/packages-nogpu.txt" | sort > /tmp/p24a_pkgs_nogpu_all.txt
comm -23 /tmp/p24a_pkgs_nogpu_all.txt /tmp/p24a_exclude.txt > /tmp/p24a_pkgs_nogpu.txt

{
  echo "# P2.4a full-repo go vet, default tags (P1.2b list minus t_stress/render.render/P2.1 deletions)"
  echo "# pkgs: $(wc -l < /tmp/p24a_pkgs_default.txt)"
  echo "# date: $(date -Iseconds)"
  go vet $(cat /tmp/p24a_pkgs_default.txt) 2>&1
  echo "# exit: $?"
  echo
  echo "# P2.4a full-repo go vet, -tags nogpu"
  echo "# pkgs: $(wc -l < /tmp/p24a_pkgs_nogpu.txt)"
  go vet -tags nogpu $(cat /tmp/p24a_pkgs_nogpu.txt) 2>&1
  echo "# exit: $?"
} > "$OUT"
echo "vet done: $(grep -c 'unreachable code\|possible misuse' "$OUT") diagnostics-ish lines; see $OUT"
