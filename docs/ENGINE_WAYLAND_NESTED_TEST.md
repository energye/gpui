# X11 环境下测 Wayland 真窗（嵌套 GNOME 合成器）

> 实测通过（2026-09-13，R17 `ui_wr_r17_bg_throttle` 30s 两连 PASS，`backend=wayland` 确认）。
> 适用：只有 X11 会话、无 Wayland 合成器、无 root 装包的机器。

## 1. 起合成器（软件渲染是必选项）

裸驱 EGL 在嵌套下会起后崩，必须强制 mesa 软件栈：

```bash
export DISPLAY=:0 XDG_RUNTIME_DIR=/run/user/1000
export LIBGL_ALWAYS_SOFTWARE=1
export __EGL_VENDOR_LIBRARY_FILENAMES=/usr/share/glvnd/egl_vendor.d/50_mesa.json
dbus-run-session -- gnome-shell --nested --wayland-display=gpui-nested --no-x11 > /tmp/nested.log 2>&1 &
```

说明：`gnome-shell` 系统自带免安装；`--wayland-display` 定套接字名防串会话；
`dbus-run-session` 隔离总线，不污染真会话。

## 2. 验活：必须实际连上，不能只看文件存在

崩掉的合成器会留下僵尸套接字（文件在，连上报 connection refused），所以：

```bash
python3 -c "import socket;s=socket.socket(socket.AF_UNIX);s.settimeout(2);s.connect('/run/user/1000/gpui-nested');print('ALIVE');s.close()"
```

打印 `ALIVE` 才算立住，否则看 `/tmp/nested.log` 找死因。

## 3. 跑窗

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export DISPLAY=:0 WAYLAND_DISPLAY=gpui-nested XDG_RUNTIME_DIR=/run/user/1000
RUN_SECONDS=30 go run ./examples/<your_window>
```

`platform.Open` 看到 `WAYLAND_DISPLAY` 自动选 Wayland 后端。

## 4. 验后端身份

JSON 里 `ability_extra.backend` 必须是 `wayland`。
只有它是 wayland，这次跑才算 Wayland 证据，否则就是连失败回落或跑错地方。

## 5. 已知坑（都会再遇到，先写下来）

1. **嵌套 Shell 自身不稳定**：它的 JS GC 在嵌套+软件下会崩，表现是"上一把绿、下一把启动报
   `wl_display_connect failed`"——这是合成器死了，不是被测应用的问题。重启合成器重跑即可，
   应用侧零改动。
2. **嵌套环境前台慢且 jank 是正常的**：R17 实测 p95 ~60ms、`vsync=fallback`、CPU 回退 2 万多 ops，
   全是嵌套+软件栈代价。门禁不含流畅项的窗照常判，JSON 如实记录；别拿这组数做性能结论。
3. **最小化恢复别指望程序化**：Mutter（含嵌套）无视裸 `XMapWindow` 恢复，Wayland 协议本身就没有
   恢复原语。需要恢复语义的窗，脚本止于最小化，恢复走单测 + 用户驱动路径。
4. **收尾必做**：杀合成器进程、删僵尸套接字、再 `pgrep` 确认。注意 `ding.js` 桌面图标进程可能被
   真会话收养——归属活会话的进程别杀，只清自己起的合成器和套接字。
