#!/bin/bash
# P2.6-c 隔离显示环境：gnome-shell headless Wayland + rootless Xwayland :3。
# P1.4/P1.6 同款口径（DISPLAY=:3，rootless，-noreset -accessx -core）；
# 不碰当前桌面 :1。健康检查用 socket connect（僵尸 socket 不算活）。
set -u
export LC_ALL=C
XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
WL=gpui-p26
XDISPLAY=:3

wl_alive() {
  python3 - "$XDG_RUNTIME_DIR/$WL" <<'PY'
import socket, sys
s = socket.socket(socket.AF_UNIX); s.settimeout(2)
try:
    s.connect(sys.argv[1])
except Exception:
    sys.exit(1)
finally:
    s.close()
PY
}
x_alive() { xdpyinfo -display "$XDISPLAY" >/dev/null 2>&1; }

case "${1:-start}" in
  start)
    if ! wl_alive; then
      rm -f "$XDG_RUNTIME_DIR/$WL" "$XDG_RUNTIME_DIR/$WL.lock"
      XDG_RUNTIME_DIR="$XDG_RUNTIME_DIR" \
        setsid nohup dbus-run-session -- gnome-shell --headless --wayland-display=$WL \
          --virtual-monitor 1600x1000@60 --no-x11 > /tmp/p26-headless.log 2>&1 < /dev/null &
      for i in $(seq 1 30); do wl_alive && break; sleep 1; done
    fi
    wl_alive || { echo "HEADLESS FAILED"; tail -5 /tmp/p26-headless.log; exit 1; }
    echo "wayland $WL: alive"
    if ! x_alive; then
      # rootless Xwayland 挂到 headless 合成器上（P1.4 同款参数）
      WAYLAND_DISPLAY=$WL XDG_RUNTIME_DIR="$XDG_RUNTIME_DIR" \
        setsid nohup Xwayland $XDISPLAY -rootless -noreset -accessx -core \
          > /tmp/p26-xwayland.log 2>&1 < /dev/null &
      for i in $(seq 1 20); do x_alive && break; sleep 1; done
    fi
    x_alive && echo "X display $XDISPLAY: alive" || { echo "XWAYLAND FAILED"; tail -5 /tmp/p26-xwayland.log; exit 1; }
    ;;
  check)
    wl_alive && x_alive && echo OK || { echo "ENV DEAD (wl=$(wl_alive && echo y || echo n) x=$(x_alive && echo y || echo n))"; exit 1; }
    ;;
  stop)
    # 只杀本会话名下的合成器与 Xwayland（gpui-p26 / :3），不碰桌面。
    pkill -f "gnome-shell --headless --wayland-display=$WL" 2>/dev/null
    pkill -f "Xwayland $XDISPLAY" 2>/dev/null
    sleep 1
    echo stopped
    ;;
esac
