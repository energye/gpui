#!/bin/bash
# P2.4d 真窗与生命周期门禁（隔离显示环境，标准关闭）。
# 0) 预编译被测窗为独立二进制（编译时间不占 RUN_SECONDS；信号直接打在进程上，
#    不经过 `go run` 的中间进程）
# 1) 隔离 gnome-shell headless Wayland（--no-x11 --virtual-monitor 1600x1000@60）。
#    不用 --nested：nested 的 stage 固定 800x600（X11 size hints min=max），
#    1200x800 的 R20 窗会被 WM 压成 748x541，像素探针坐标与滤镜负载全失真
#    （第一轮证据见 window-gates-run1-bad-env.txt 的失败形态）。headless 的
#    gbm renderer 走真 GPU，窗口可以保持请求尺寸。
# 2) ui_l1_blank native 60s（JSON 探针，含 pool 段）
# 3) ui_base_multiwin_cap native -auto-only RUN_SECONDS=75（8 窗生命周期 + pool 段）
# 4) ui_wr_r20_filter native RUN_SECONDS=30 两遍（首遍播种金图，第二遍确认 diff=0；
#    filter-graph + device-surface + layer-offscreen 三域探针）
# 5) ui_render_base_scene native -auto-only RUN_SECONDS=10（dualtex-snap 域唯一
#    现有真窗负载：scene Multiply layer；该窗的 X11 screen grab 在 Wayland 下必失败，
#    rc=1 如实登记，pool 段仍有效）
# 6) ui_base_vram_cap ladder native -auto-only RUN_SECONDS=300（峰值对账 P1.6 372MB 档）
# 每轮前查桌面与 Wayland socket；rc=124 = 没自己退出（被 timeout 杀），必须如实登记不算 PASS。
set -u
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/../../../../.." && pwd)
OUT="$ROOT/docs/render_gpu_converge/evidence/P2.4d"
export WGPU_NATIVE_PATH="$ROOT/lib/libwgpu_native.so"
export LD_LIBRARY_PATH="$ROOT/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export XDG_RUNTIME_DIR="/run/user/$(id -u)"

WL=gpui-p24d
WL_LOG=/tmp/p24d-headless.log
BIN=/tmp/p24d-bin
mkdir -p "$BIN"

WINDOWS=(ui_l1_blank ui_base_multiwin_cap ui_wr_r20_filter ui_render_base_scene ui_base_vram_cap)

# wl_alive：连接一次套接字。崩掉的合成器会留下僵尸 socket（文件在、connect 拒绝），
# 只看 -S 会把死环境当成活环境（docs/ENGINE_WAYLAND_NESTED_TEST.md §2）。
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

# runwin <binary> <label> <secs> <timeout> <args...>：跑二进制，记录 rc，124 单独标注。
# 输出固定为 $OUT/<label>.json / .stderr，避免同一二进制多遍跑互相覆盖。
runwin() {
  local bin=$1 label=$2 secs=$3 tout=$4; shift 4
  echo "## $label (native, RUN_SECONDS=$secs, timeout=$tout)"
  if ! wl_alive; then echo "  ABORT: 隔离 wayland socket $WL 连不上（僵尸 socket 或合成器已崩）"; return 1; fi
  xdpyinfo -display "${DISPLAY:-:1}" >/dev/null 2>&1 || { echo "  ABORT: 桌面 X ${DISPLAY:-:1} 失联，停批（AGENTS GPU 真窗安全）"; return 1; }
  WAYLAND_DISPLAY=$WL RUN_SECONDS=$secs timeout "$tout" "$BIN/$bin" "$@" \
    > "$OUT/${label}.json" 2> "$OUT/${label}.stderr"
  local rc=$?
  echo "  rc=$rc$( [ $rc -eq 124 ] && echo ' (被 timeout 杀掉：窗没有自己退出，不算正常关闭)' )"
  echo "  pool 段: $(grep -o '"pool":\[[^]]*\]' "$OUT/${label}.json" | head -1 | cut -c1-400)"
  echo "  stderr 尾:"; tail -4 "$OUT/${label}.stderr" | sed 's/^/    /'
  echo
}

{
  echo "# P2.4d window gates (isolated headless Wayland, 1600x1000@60)"
  echo "# date: $(date -Iseconds)"
  echo "# head: $(git -C "$ROOT" rev-parse --short HEAD)"

  echo
  echo "## step 0: desktop health"
  xdpyinfo -display "${DISPLAY:-:1}" >/dev/null 2>&1 && echo "desktop X display ${DISPLAY:-:1}: alive" || echo "desktop X display: DEAD (abort)"
  echo

  echo "## step 1: isolated gnome-shell (headless wayland display $WL, virtual monitor 1600x1000@60)"
  if ! wl_alive; then
    # 僵尸 socket 文件会让新合成器起不来；先删再起（只动本会话自己的名字）。
    rm -f "$XDG_RUNTIME_DIR/$WL" "$XDG_RUNTIME_DIR/$WL.lock"
    XDG_RUNTIME_DIR="$XDG_RUNTIME_DIR" \
      setsid nohup dbus-run-session -- gnome-shell --headless --wayland-display=$WL \
        --virtual-monitor 1600x1000@60 --no-x11 > "$WL_LOG" 2>&1 < /dev/null &
    echo "spawned pid $!"
    for i in $(seq 1 30); do
      wl_alive && break
      sleep 1
    done
  fi
  if ! wl_alive; then echo "HEADLESS FAILED"; tail -5 "$WL_LOG"; exit 1; fi
  echo "wayland socket: alive (connect ok)"
  echo

  echo "## step 2: build binaries (信号直接打在被测进程上)"
  cd "$ROOT" || exit 1
  for w in "${WINDOWS[@]}"; do
    go build -o "$BIN/$w" "./examples/$w" 2>&1 | tail -3
    echo "  build $w: rc=${PIPESTATUS[0]} $([ -x "$BIN/$w" ] && echo ok || echo MISSING)"
  done
  echo

  runwin ui_l1_blank ui_l1_blank_native 60 200
  runwin ui_base_multiwin_cap ui_base_multiwin_cap_native 75 200 -auto-only
  # r20 走 wrkit 的 RUN_SECONDS 自动退出（该窗没有 -auto-only 开关）；池段来自 wrgate.Report.Pool。
  # 首遍播种 /tmp 金图，第二遍必须 diff=0；两遍都落盘，第二遍是收口数字。
  rm -rf "$OUT/r20_snapshots"; mkdir -p "$OUT/r20_snapshots"
  export R20_SNAP_DIR="$OUT/r20_snapshots"
  runwin ui_wr_r20_filter ui_wr_r20_filter_native-seed 30 300
  runwin ui_wr_r20_filter ui_wr_r20_filter_native 30 300
  unset R20_SNAP_DIR
  # dualtex-snap 唯一现有真窗负载：scene Multiply layer。X11 screen grab 在 Wayland
  # 必失败（rc=1），pool 段有效；该窗自己的门禁不是 P2.4d 的判定对象。
  runwin ui_render_base_scene ui_render_base_scene_native 10 200 -auto-only
  runwin ui_base_vram_cap ui_base_vram_cap_native 300 500 -auto-only

  echo "## step 6: 桌面健康收尾"
  xdpyinfo -display "${DISPLAY:-:1}" >/dev/null 2>&1 && echo "desktop X display: alive after run" || echo "desktop X display: DEAD"
  dmesg 2>/dev/null | tail -3 | sed 's/^/    /' || true
} > "$OUT/window-gates.txt" 2>&1
tail -60 "$OUT/window-gates.txt"
