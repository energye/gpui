#!/bin/bash
# P2.4a 真窗与生命周期门禁（隔离显示环境，标准关闭）。
# 1) 起嵌套 gnome-shell（带 X11，独立 Wayland display gpui-p24a）→ 其内 Xwayland 为隔离 X 显示
# 2) ui_l1_blank native + go 各 60s（JSON 探针 + stderr 归档；无金图为设计内，替代判定=JSON/快照计数）
# 3) ui_base_multiwin_cap go 档 -auto-only RUN_SECONDS=75（8 窗批量生命周期，借共享不变式）
# 健康检查 + 标准退出（门禁窗自走 RUN_SECONDS 退出；不 kill）。
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.4a"
export WGPU_NATIVE_PATH="$ROOT/lib/libwgpu_native.so"
export LD_LIBRARY_PATH="$ROOT/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export XDG_RUNTIME_DIR="/run/user/$(id -u)"

NESTED_WL=gpui-p24a
NESTED_LOG=/tmp/p24a-nested.log

{
  echo "# P2.4a window gates (isolated nested display)"
  echo "# date: $(date -Iseconds)"
  echo "# head: $(git -C "$ROOT" rev-parse --short HEAD)"

  # ---- step 0: current desktop health
  echo
  echo "## step 0: desktop health"
  xdpyinfo -display "${DISPLAY:-:1}" >/dev/null 2>&1 && echo "desktop X display ${DISPLAY:-:1}: alive" || echo "desktop X display: DEAD (abort)"
  echo

  # ---- step 1: nested compositor (Wayland-only isolation; its Xwayland cannot
# start under dbus-run-session — user systemd unavailable — so windows run
# Wayland-native, which is an accepted platform row in baseline/windows.md)
  if [ ! -S "$XDG_RUNTIME_DIR/$NESTED_WL" ]; then
    dbus-run-session -- gnome-shell --nested --wayland-display=$NESTED_WL --no-x11 > "$NESTED_LOG" 2>&1 &
    echo "spawned nested gnome-shell pid $!"
    for i in $(seq 1 30); do
      [ -S "$XDG_RUNTIME_DIR/$NESTED_WL" ] && break
      sleep 1
    done
  fi
  if [ -S "$XDG_RUNTIME_DIR/$NESTED_WL" ]; then echo "nested wayland socket: alive"; else echo "NESTED FAILED"; tail -5 "$NESTED_LOG"; exit 1; fi
  echo

  # ---- step 2: ui_l1_blank both backends (Wayland-native in the nested session)
  for be in native go; do
    echo "## step 2: ui_l1_blank ($be) on nested Wayland $NESTED_WL"
    if [ "$be" = go ]; then export GPUI_BACKEND=go; else unset GPUI_BACKEND || true; fi
    ( cd "$ROOT" && WAYLAND_DISPLAY=$NESTED_WL RUN_SECONDS=60 timeout 90       go run ./examples/ui_l1_blank ) > "$OUT/l1_blank_$be.json" 2> "$OUT/l1_blank_$be.stderr"
    echo "rc=$?"
    tail -1 "$OUT/l1_blank_$be.json" | head -c 200; echo
    echo
  done
  unset GPUI_BACKEND || true

  # ---- step 3: multiwin cap both backends (lifecycle, 8 windows, borrow invariant)
  for be in native go; do
    echo "## step 3: ui_base_multiwin_cap ($be) on nested Wayland $NESTED_WL"
    if [ "$be" = go ]; then export GPUI_BACKEND=go; else unset GPUI_BACKEND || true; fi
    ( cd "$ROOT" && WAYLAND_DISPLAY=$NESTED_WL RUN_SECONDS=75 timeout 150       go run ./examples/ui_base_multiwin_cap -auto-only ) > "$OUT/multiwin_$be.json" 2> "$OUT/multiwin_$be.stderr"
    echo "rc=$?"
    tail -3 "$OUT/multiwin_$be.stderr"
    echo
  done
  unset GPUI_BACKEND || true
} > "$OUT/window-gates.txt" 2>&1
tail -20 "$OUT/window-gates.txt"
