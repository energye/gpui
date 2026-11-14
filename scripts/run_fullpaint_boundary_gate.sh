#!/usr/bin/env bash
# O10 P3: full_paint 家族边界回放覆盖 runner — 单窗串行、隔离显示、标准关闭。
# 每窗跑 -auto-only 后按 JSON 的 boundary_skip/boundary_rerecord 分类：
#   REPLAY_COVERED      skip>0（① 层回放窄化工作）
#   NO_BOUNDARY_SCENE   skip==0 && rr==0（场景无 RepaintBoundary，断言不适用）
# 用法: run_fullpaint_boundary_gate.sh [DISPLAY] [RUN_SECONDS]
set -u
REPO=$(cd "$(dirname "$0")/.." && pwd)
DISP=${1:-:3}
SECS=${2:-5}
cd "$REPO" || exit 2
OUT=${OUT:-/tmp/p0-dirty/p3family}
mkdir -p "$OUT/json"
: > "$OUT/classify.tsv"

WINDOWS=($(grep -rln "PresentPolicyFullPaint" examples/*/main.go | xargs -n1 dirname | xargs -n1 basename))
echo "windows: ${#WINDOWS[@]}"
for w in "${WINDOWS[@]}"; do
  if ! DISPLAY=$DISP xdpyinfo >/dev/null 2>&1; then
    echo -e "$w\tHEALTH_FAIL\t-\t-" >> "$OUT/classify.tsv"
    continue
  fi
  go build -o "$OUT/bin_$w" "./examples/$w" 2>/dev/null || { echo -e "$w\tBUILD_FAIL\t-\t-" >> "$OUT/classify.tsv"; continue; }
  env -u WAYLAND_DISPLAY DISPLAY=$DISP LD_LIBRARY_PATH="$REPO/lib" WGPU_NATIVE_PATH="$REPO/lib/libwgpu_native.so" \
    RUN_SECONDS=$SECS timeout 90 "$OUT/bin_$w" -auto-only > "$OUT/json/$w.json" 2>"$OUT/json/$w.log"
  rc=$?
  python3 - "$w" "$rc" "$OUT" <<'PYEOF'
import json,sys
w,rc,out=sys.argv[1],sys.argv[2],sys.argv[3]
try:
    d=json.load(open(f'{out}/json/{w}.json'))
    s,rr=d.get('boundary_skip',0),d.get('boundary_rerecord',0)
    cls='REPLAY_COVERED' if s>0 else ('NO_BOUNDARY_SCENE' if rr==0 else 'RERECORD_ONLY')
    print(f"{w}\t{cls}\tskip={s}\trr={rr}\trc={rc}")
except Exception:
    print(f"{w}\tNO_JSON\t-\t-\trc={rc}")
PYEOF
done >> "$OUT/classify.tsv"
echo ALLDONE >> "$OUT/classify.tsv"
