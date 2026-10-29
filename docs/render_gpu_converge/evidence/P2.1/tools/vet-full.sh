#!/bin/bash
# P2.1 full-repo go vet (default / -tags nogpu), same ruler as P1.2b/P2.3:
# P1.2b package list minus known-broken ui_wr_t_stress and minus render/render
# (already deleted by P2.2; kept in the P1.2b list), further minus the 30
# packages P2.1 deleted. Expect 61 / 58 diagnostics, byte-identical to P2.3.
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
EV="$ROOT/docs/render_gpu_converge/evidence/P2.1"
P12="$ROOT/docs/render_gpu_converge/evidence/P1.2b"
REMOVED="$EV/tools/removed-packages.txt"
cd "$ROOT" || exit 1

# packages absent from the tree: P2.1 deletions + render/render (P2.2)
rm -f /tmp/p21_exclude.txt
grep '^github.com/energye/gpui/render/render$' "$P12/packages-default.txt" >> /tmp/p21_exclude.txt
cat "$REMOVED" >> /tmp/p21_exclude.txt
sort -u /tmp/p21_exclude.txt -o /tmp/p21_exclude.txt

grep -v 'ui_wr_t_stress' "$P12/packages-default.txt" | sort > /tmp/p21_pkgs_default_all.txt
comm -23 /tmp/p21_pkgs_default_all.txt /tmp/p21_exclude.txt > /tmp/p21_pkgs_default.txt

grep -v 'ui_wr_t_stress' "$P12/packages-nogpu.txt" | sort > /tmp/p21_pkgs_nogpu_all.txt
comm -23 /tmp/p21_pkgs_nogpu_all.txt /tmp/p21_exclude.txt > /tmp/p21_pkgs_nogpu.txt

{
  echo "# P2.1 full-repo go vet, default tags (P1.2b package list minus t_stress, render/render (P2.2) and 30 P2.1-deleted packages)"
  echo "# pkgs: $(wc -l < /tmp/p21_pkgs_default.txt)"
  echo "# date: $(date -Iseconds)"
  go vet $(cat /tmp/p21_pkgs_default.txt) 2>&1
  echo "# exit: $?"
} > "$EV/vet-full-default.txt" 2>&1

{
  echo "# P2.1 full-repo go vet, -tags nogpu (P1.2b package list minus t_stress, render/render (P2.2) and 30 P2.1-deleted packages)"
  echo "# pkgs: $(wc -l < /tmp/p21_pkgs_nogpu.txt)"
  echo "# date: $(date -Iseconds)"
  go vet -tags nogpu $(cat /tmp/p21_pkgs_nogpu.txt) 2>&1
  echo "# exit: $?"
} > "$EV/vet-full-nogpu.txt" 2>&1

echo "default diagnostics: $(grep -cE '^[^#].*:[0-9]+:[0-9]+: ' "$EV/vet-full-default.txt") (P2.3: 61)"
echo "nogpu diagnostics:   $(grep -cE '^[^#].*:[0-9]+:[0-9]+: ' "$EV/vet-full-nogpu.txt") (P2.3: 58)"
echo "--- diagnostic-line diff vs P2.3 (must be empty) ---"
diff <(grep -E '^[^#].*:[0-9]+:[0-9]+: ' "$ROOT/docs/render_gpu_converge/evidence/P2.3/vet-full-default.txt") \
     <(grep -E '^[^#].*:[0-9]+:[0-9]+: ' "$EV/vet-full-default.txt")
diff <(grep -E '^[^#].*:[0-9]+:[0-9]+: ' "$ROOT/docs/render_gpu_converge/evidence/P2.3/vet-full-nogpu.txt") \
     <(grep -E '^[^#].*:[0-9]+:[0-9]+: ' "$EV/vet-full-nogpu.txt")
