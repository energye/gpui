#!/usr/bin/env bash
set -u
REPO=/home/yanghy/app/projects/gogpu/gpui
OUT=$REPO/docs/render_gpu_converge/evidence/P2.6/o1-rss-plateau/o6c-2h
PROBE=$REPO/docs/render_gpu_converge/evidence/P2.6/o1-rss-plateau/tools/probe.py
export DISPLAY=:3
unset WAYLAND_DISPLAY
export LD_LIBRARY_PATH=$REPO/lib
export WGPU_NATIVE_PATH=$REPO/lib/libwgpu_native.so
export GODEBUG=gctrace=1
health(){ DISPLAY=:3 xdpyinfo >/dev/null 2>&1; }
summary(){
  local d=$1 name=$2
  python3 - "$d" "$name" >> "$OUT/summary.txt" <<'PY'
import csv,json,os,sys
d,name=sys.argv[1],sys.argv[2]
pts=[(float(r['t_sec']),int(r['VmRSS'])) for r in csv.DictReader(open(os.path.join(d,'rss.tsv')),delimiter='\t') if int(r['VmRSS'])>0]
if not pts:
    print(name,'NO_RSS'); raise SystemExit
end=max(0,pts[-1][0]-2)
pts=[p for p in pts if p[0]<=end]
def at(t):
    c=[p for p in pts if p[0]>=t]
    return c[0][1] if c else pts[-1][1]
s60=at(60); send=pts[-1][1]; peak=max(p[1] for p in pts)
print(f"{name}\tt_end={end:.0f}s rss60={s60/1024:.1f}MB end={send/1024:.1f}MB peak={peak/1024:.1f}MB growth60_end={(send-s60)/max(1,s60)*100:.2f}%")
PY
}
echo "=== O6c start $(date -Is) ===" >> "$OUT/runner.log"
if ! health; then echo "HEALTH_FAIL before m4" >> "$OUT/runner.log"; exit 3; fi
mkdir -p "$OUT/m4_go_2h"
python3 "$PROBE" --bin /tmp/o6c/bin/ui_text_m4_virtual --out "$OUT/m4_go_2h" --secs 7200 --smaps-at 600,1800,3600,5400,7000 \
  --env RUN_SECONDS=7200 --env GPUI_BACKEND=go --env GODEBUG=gctrace=1 \
  --env LD_LIBRARY_PATH=$REPO/lib --env WGPU_NATIVE_PATH=$REPO/lib/libwgpu_native.so >> "$OUT/runner.log" 2>&1
echo "m4_go_2h done rc=$? $(date -Is)" >> "$OUT/runner.log"
summary "$OUT/m4_go_2h" m4_go_2h
if ! health; then echo "HEALTH_FAIL before video" >> "$OUT/runner.log"; exit 4; fi
mkdir -p "$OUT/video_native_2h"
python3 "$PROBE" --bin /tmp/o6c/bin/video_vc2_soak --out "$OUT/video_native_2h" --secs 7200 --smaps-at 600,1800,3600,5400,7000 \
  --env RUN_SECONDS=7200 --env GODEBUG=gctrace=1 \
  --env LD_LIBRARY_PATH=$REPO/lib --env WGPU_NATIVE_PATH=$REPO/lib/libwgpu_native.so >> "$OUT/runner.log" 2>&1
echo "video_native_2h done rc=$? $(date -Is)" >> "$OUT/runner.log"
summary "$OUT/video_native_2h" video_native_2h
echo "=== O6c finish $(date -Is) ===" >> "$OUT/runner.log"
