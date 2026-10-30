#!/bin/bash
# P2.4b 真窗与生命周期门禁（隔离显示环境，标准关闭）。
# 1) 嵌套 gnome-shell Wayland（--no-x11；其 Xwayland 在 dbus-run-session 下起不来）
# 2) ui_l1_blank native + go 各 60s（JSON 探针）
# 3) ui_base_multiwin_cap native -auto-only RUN_SECONDS=75（8 窗生命周期）
# 4) ui_base_vram_cap ladder native -auto-only RUN_SECONDS=300（预算专项窗，峰值对账 P1.6 372MB 档）
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.4b"
export WGPU_NATIVE_PATH="$ROOT/lib/libwgpu_native.so"
export LD_LIBRARY_PATH="$ROOT/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export XDG_RUNTIME_DIR="/run/user/$(id -u)"

NESTED_WL=gpui-p24b
NESTED_LOG=/tmp/p24b-nested.log

{
  echo "# P2.4b window gates (isolated nested Wayland)"
  echo "# date: $(date -Iseconds)"
  echo "# head: $(git -C "$ROOT" rev-parse --short HEAD)"

  echo
  echo "## step 0: desktop health"
  xdpyinfo -display "${DISPLAY:-:1}" >/dev/null 2>&1 && echo "desktop X display ${DISPLAY:-:1}: alive" || echo "desktop X display: DEAD (abort)"
  echo

  echo "## step 1: nested gnome-shell (wayland display $NESTED_WL)"
  if [ ! -S "$XDG_RUNTIME_DIR/$NESTED_WL" ]; then
    dbus-run-session -- gnome-shell --nested --wayland-display=$NESTED_WL --no-x11 > "$NESTED_LOG" 2>&1 &
    echo "spawned pid $!"
    for i in $(seq 1 30); do
      [ -S "$XDG_RUNTIME_DIR/$NESTED_WL" ] && break
      sleep 1
    done
  fi
  if [ -S "$XDG_RUNTIME_DIR/$NESTED_WL" ]; then echo "nested wayland socket: alive"; else echo "NESTED FAILED"; tail -5 "$NESTED_LOG"; exit 1; fi
  echo

  for be in native go; do
    echo "## step 2: ui_l1_blank ($be) on nested Wayland $NESTED_WL"
    if [ "$be" = go ]; then export GPUI_BACKEND=go; else unset GPUI_BACKEND || true; fi
    ( cd "$ROOT" && WAYLAND_DISPLAY=$NESTED_WL RUN_SECONDS=60 timeout 200 \
      go run ./examples/ui_l1_blank ) > "$OUT/l1_blank_$be.json" 2> "$OUT/l1_blank_$be.stderr"
    echo "rc=$?"
    tail -1 "$OUT/l1_blank_$be.json" | head -c 200; echo
    echo
  done
  unset GPUI_BACKEND || true

  echo "## step 3: ui_base_multiwin_cap (native) on nested Wayland $NESTED_WL"
  ( cd "$ROOT" && WAYLAND_DISPLAY=$NESTED_WL RUN_SECONDS=75 timeout 200 \
    go run ./examples/ui_base_multiwin_cap -auto-only ) > "$OUT/multiwin_native.json" 2> "$OUT/multiwin_native.stderr"
  echo "rc=$?"
  tail -3 "$OUT/multiwin_native.stderr"
  echo

  echo "## step 4: ui_base_vram_cap ladder (native, RUN_SECONDS=300) on nested Wayland $NESTED_WL"
  ( cd "$ROOT" && WAYLAND_DISPLAY=$NESTED_WL RUN_SECONDS=300 timeout 500 \
    go run ./examples/ui_base_vram_cap -auto-only ) > "$OUT/vram_ladder_native.json" 2> "$OUT/vram_ladder_native.stderr"
  echo "rc=$?"
  tail -4 "$OUT/vram_ladder_native.stderr"
} > "$OUT/window-gates.txt" 2>&1
tail -25 "$OUT/window-gates.txt"
