#!/usr/bin/env bash
# Re-run FAIL shard tests from a summary file, one Test* per process.
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export GOROOT="${GOROOT:-/home/yanghy/app/gopath/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.5.linux-amd64}"
export PATH="$GOROOT/bin:$PATH"
export GOCACHE="${GOCACHE:-$ROOT/tmp/go-cache}"
export GOWORK=off
export GOTOOLCHAIN=local
export WGPU_NATIVE_PATH="${WGPU_NATIVE_PATH:-$ROOT/lib/libwgpu_native.so}"
export LD_LIBRARY_PATH="$ROOT/lib:${LD_LIBRARY_PATH:-}"
export DISPLAY="${DISPLAY:-:1}"
export GPUI_SURFACE_SAMPLE_COUNT="${GPUI_SURFACE_SAMPLE_COUNT:-1}"
SUMMARY="${1:-$ROOT/tmp/full_unit/summary.txt}"
OUT="${2:-$ROOT/tmp/full_unit/rerun_fails}"
mkdir -p "$OUT"
LIST="$OUT/tests_to_rerun.txt"
python3 - "$SUMMARY" "$LIST" <<'PY'
import re, sys
from pathlib import Path
summary, out_path = sys.argv[1], sys.argv[2]
text = Path(summary).read_text(errors="replace")
items = []
for line in text.splitlines():
    if not line.startswith("FAIL "):
        continue
    parts = line.split(None, 2)
    if len(parts) < 2:
        continue
    pkg = parts[1]
    rest = parts[2] if len(parts) > 2 else ""
    rest = re.sub(r"\s*\(rc=\d+\)\s*$", "", rest).strip()
    if rest.startswith("^(") and rest.endswith(")$"):
        for name in rest[2:-2].split("|"):
            name = name.strip()
            if name.startswith("Test"):
                items.append((pkg, name))
    elif rest == "":
        items.append((pkg, ""))
    else:
        name = rest.strip("^$")
        if name.startswith("Test"):
            items.append((pkg, name))
seen = set()
uniq = []
for p, n in items:
    k = (p, n)
    if k in seen:
        continue
    seen.add(k)
    uniq.append(k)
Path(out_path).write_text("".join(f"{p}\t{n}\n" for p, n in uniq))
print(f"parsed {len(uniq)} tests from {summary}", flush=True)
PY

: > "$OUT/summary.txt"
fail=0
pass=0
while IFS=$'\t' read -r pkg tname; do
  [[ -z "${pkg:-}" ]] && continue
  if [[ -z "${tname:-}" ]]; then
    safe=$(echo "$pkg" | tr '/.' '__')
    log="$OUT/${safe}_ALL.log"
    echo "=== $pkg ALL $(date +%H:%M:%S) ===" | tee -a "$OUT/summary.txt"
    if timeout 900s go test -count=1 -p 1 -parallel 1 "$pkg" -timeout 900s >"$log" 2>&1; then
      echo "PASS $pkg ALL" | tee -a "$OUT/summary.txt"
      pass=$((pass + 1))
    else
      echo "FAIL $pkg ALL" | tee -a "$OUT/summary.txt"
      fail=$((fail + 1))
    fi
  else
    safe=$(echo "${pkg}_${tname}" | tr '/.' '__')
    log="$OUT/${safe}.log"
    echo "=== $pkg $tname $(date +%H:%M:%S) ===" | tee -a "$OUT/summary.txt"
    if timeout 180s go test -count=1 -p 1 -parallel 1 "$pkg" -run "^${tname}$" -timeout 180s >"$log" 2>&1; then
      echo "PASS $pkg $tname" | tee -a "$OUT/summary.txt"
      pass=$((pass + 1))
    else
      echo "FAIL $pkg $tname" | tee -a "$OUT/summary.txt"
      strings "$log" 2>/dev/null | rg -n '^--- FAIL:|panic:|Not enough memory|Error' | tail -20 >>"$OUT/summary.txt" || true
      fail=$((fail + 1))
    fi
  fi
  sleep 2
done < "$LIST"

echo "DONE pass=$pass fail=$fail $(date +%H:%M:%S)" | tee -a "$OUT/summary.txt"
echo "$fail" > "$OUT/exit_code.txt"
exit "$fail"
