#!/usr/bin/env bash
set -u
REPO=/home/yanghy/app/projects/gogpu/gpui
OUT=$REPO/docs/render_gpu_converge/evidence/P2.6/o1-rss-plateau/o7-reseed
O6C=$REPO/docs/render_gpu_converge/evidence/P2.6/o1-rss-plateau/o6c-2h
mkdir -p "$OUT"
echo "=== O7 reseed watcher start $(date -Is) pid=$$ ===" >> "$OUT/watcher.log"
while kill -0 423570 2>/dev/null; do sleep 60; done
if ! grep -q "=== O6c finish" "$O6C/runner.log" 2>/dev/null; then
  echo "ABORT: O6c did not finish cleanly $(date -Is)" >> "$OUT/watcher.log"
  echo "O6c not finished cleanly" > "$OUT/ABORT"
  exit 1
fi
if ! DISPLAY=:3 xdpyinfo >/dev/null 2>&1; then
  echo "ABORT: display :3 dead $(date -Is)" >> "$OUT/watcher.log"
  echo "display :3 dead" > "$OUT/ABORT"
  exit 2
fi
echo "=== O7 reseed start $(date -Is) ===" >> "$OUT/watcher.log"
python3 "$OUT/tools/reseed.py" >> "$OUT/reseed.log" 2>&1
rc=$?
echo "reseed_rc=$rc $(date -Is)" >> "$OUT/watcher.log"
