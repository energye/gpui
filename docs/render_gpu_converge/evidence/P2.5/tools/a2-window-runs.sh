#!/bin/bash
# P2.5 A2 真窗验证：旧代窗切 interval_* 契约 + fps_wall/target_hz 补齐。
# 隔离显示环境（AGENTS GPU 真窗安全）：gnome-shell headless Wayland
# --no-x11 --virtual-monitor 1600x1000@60；每轮前查桌面与 socket；
# 窗按 RUN_SECONDS 自退（标准关闭），rc=124=没自己退出必须如实登记。
# 用法: a2-window-runs.sh [输出目录]（默认本阶段 evidence 目录）
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
OUT="${1:-$ROOT/docs/render_gpu_converge/evidence/P2.5}"
export WGPU_NATIVE_PATH="$ROOT/lib/libwgpu_native.so"
export LD_LIBRARY_PATH="$ROOT/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export XDG_RUNTIME_DIR="/run/user/$(id -u)"

WL=gpui-p25
WL_LOG=/tmp/p25-headless.log
BIN=/tmp/p25-bin
mkdir -p "$BIN"

wl_alive() {
  python3 - "$XDG_RUNTIME_DIR/$WL" <<'PY'
import socket, sys
s = socket.socket(socket.AF_UNIX)
s.settimeout(2)
try:
    s.connect(sys.argv[1])
except Exception:
    sys.exit(1)
finally:
    s.close()
PY
}

runwin() {
  local bin=$1 label=$2 secs=$3 tout=$4; shift 4
  echo "## $label (RUN_SECONDS=$secs, timeout=$tout)"
  if ! wl_alive; then echo "  ABORT: 隔离 wayland socket $WL 连不上"; return 1; fi
  xdpyinfo -display "${DISPLAY:-:1}" >/dev/null 2>&1 || { echo "  ABORT: 桌面 X ${DISPLAY:-:1} 失联，停批"; return 1; }
  env "$@" WAYLAND_DISPLAY=$WL RUN_SECONDS=$secs timeout "$tout" "$BIN/$bin" \
    > "$OUT/${label}.json" 2> "$OUT/${label}.stderr"
  local rc=$?
  echo "  rc=$rc$( [ $rc -eq 124 ] && echo ' (被 timeout 杀掉：不算正常关闭)' )"
  python3 - "$OUT/${label}.json" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1]))
except Exception as e:
    print("  JSON parse FAIL:", e); raise SystemExit(1)
m = d.get("metrics", d)
new = ["interval_avg_ms", "interval_p50_ms", "interval_p95_ms", "interval_p99_ms", "fps_wall", "target_hz"]
old = ["avg_frame_interval_ms", "frame_interval_p50_ms", "frame_interval_p95_ms", "frame_interval_p99_ms"]
print("  new keys:", {k: m.get(k) for k in new if k in m})
print("  old keys present (must be none):", [k for k in old if k in m] or "NONE ✓")
PY
  echo "  stderr 尾:"; tail -3 "$OUT/${label}.stderr" | sed 's/^/    /'
  echo
}

{
  echo "# P2.5 A2 window runs (isolated headless Wayland 1600x1000@60, standard close)"
  echo "# date: $(date -Iseconds)"
  echo "# head: $(git -C "$ROOT" rev-parse --short HEAD)"

  echo; echo "## step 1: isolated gnome-shell"
  if ! wl_alive; then
    rm -f "$XDG_RUNTIME_DIR/$WL" "$XDG_RUNTIME_DIR/$WL.lock"
    XDG_RUNTIME_DIR="$XDG_RUNTIME_DIR" \
      setsid nohup dbus-run-session -- gnome-shell --headless --wayland-display=$WL \
        --virtual-monitor 1600x1000@60 --no-x11 > "$WL_LOG" 2>&1 < /dev/null &
    echo "spawned pid $!"
    for i in $(seq 1 30); do wl_alive && break; sleep 1; done
  fi
  wl_alive && echo "wayland socket alive" || { echo "HEADLESS FAILED"; tail -5 "$WL_LOG"; exit 1; }

  echo; echo "## step 2: build"
  cd "$ROOT" || exit 1
  for w in ui_l1_blank ui_render_base_perfsoak; do
    go build -o "$BIN/$w" "./examples/$w" 2>&1 | tail -2
    echo "  build $w: $([ -x "$BIN/$w" ] && echo ok || echo MISSING)"
  done
  go build -o "$BIN/kit_spin_min" "./examples/kit/spin_min" 2>&1 | tail -2
  echo "  build kit/spin_min: $([ -x "$BIN/kit_spin_min" ] && echo ok || echo MISSING)"

  echo; echo "## step 3: 3 窗 × 2 后端（native / GPUI_BACKEND=go）"
  runwin ui_l1_blank              a2_ui_l1_blank_native    20 120
  runwin ui_l1_blank              a2_ui_l1_blank_go        20 120 GPUI_BACKEND=go
  runwin kit_spin_min             a2_kit_spin_min_native   20 120
  runwin kit_spin_min             a2_kit_spin_min_go       20 120 GPUI_BACKEND=go
  runwin ui_render_base_perfsoak  a2_ui_render_base_perfsoak_native 20 120
  runwin ui_render_base_perfsoak  a2_ui_render_base_perfsoak_go    20 120 GPUI_BACKEND=go

  echo "## step 4: 桌面健康收尾"
  xdpyinfo -display "${DISPLAY:-:1}" >/dev/null 2>&1 && echo "desktop X display: alive after run" || echo "desktop X display: DEAD"
} > "$OUT/a2-window-runs.txt" 2>&1
tail -40 "$OUT/a2-window-runs.txt"
