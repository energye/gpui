---
name: gpui-wayland-nested
description: X11 机器上跑 Wayland 真窗 GPU 测试——起嵌套 GNOME 合成器（软件渲染）→ 验套接字真活 → 带 WAYLAND_DISPLAY 跑 ui_wr_* 窗 → 以 backend=wayland 验明正身 → 清场。触发词：「wayland 真窗」「x11 测 wayland」「嵌套合成器跑窗」「wayland 证据」「/gpui-wayland-nested」。
---

# gpui-wayland-nested：X11 环境跑 Wayland 真窗

在只有 X11 会话、无 Wayland 合成器、无 root 装包的机器上，
用免安装的 GNOME 嵌套模式给 `examples/ui_wr_*` 真窗提供 Wayland 后端证据。
细则与坑的完整版见 `docs/ENGINE_WAYLAND_NESTED_TEST.md`（本 skill 是可执行手顺，文档是备查）。

## 输入

- 窗包：`examples/ui_wr_*`（须已在 X11 下跑通过，嵌套只换后端、不修窗）
- 时长：该窗关闭用 `RUN_SECONDS`（如 R17=30）

## 步骤

### 0. 前置

确认 `gnome-shell` 存在（`which gnome-shell`）且当前是 X11 会话。
确认没有残留套接字：`ls /run/user/1000/gpui-nested` 应不存在，存在先删
（崩掉的合成器会留僵尸套接字，文件在但连不上）。

### 1. 起合成器（软件渲染必选）

裸驱 EGL 在嵌套下会起后崩，必须强制 mesa 软件栈，后台起：

```bash
export DISPLAY=:0 XDG_RUNTIME_DIR=/run/user/1000
export LIBGL_ALWAYS_SOFTWARE=1
export __EGL_VENDOR_LIBRARY_FILENAMES=/usr/share/glvnd/egl_vendor.d/50_mesa.json
dbus-run-session -- gnome-shell --nested --wayland-display=gpui-nested --no-x11 > /tmp/nested.log 2>&1
```

`--wayland-display` 定名防串真会话；`dbus-run-session` 隔离总线。

### 2. 验活：必须实际连上

只看文件存在会被僵尸骗，用真连接判定，不 `ALIVE` 就看 `/tmp/nested*.log`：

```bash
python3 -c "import socket;s=socket.socket(socket.AF_UNIX);s.settimeout(2);s.connect('/run/user/1000/gpui-nested');print('ALIVE');s.close()"
```

### 3. 跑窗（后台，时长按关闭档）

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export DISPLAY=:0 WAYLAND_DISPLAY=gpui-nested XDG_RUNTIME_DIR=/run/user/1000
RUN_SECONDS=<关闭秒数> go run ./examples/<window> > /tmp/<run>.json 2> /tmp/<run>.stderr
```

`platform.Open` 看到 `WAYLAND_DISPLAY` 自动选 Wayland 后端，无需改窗代码。

### 4. 验明正身 + 判门禁归属

1. 先看 JSON `ability_extra.backend`——**必须是 `wayland`**，否则这次跑不算 Wayland 证据。
2. 再看该窗自己的门禁（exit 码 + FAIL 行），嵌套下全绿才算过。
3. 以下**只记录、不判 FAIL**（嵌套+软件栈代价，R17 实测）：前台 p95 ~60ms、
   `vsync=fallback`、CPU 回退 ops 上万、CPU% 高。别拿这组数做性能结论；
   性能门禁只认原生后端。

### 5. 清场（必做）

杀自己起的合成器进程、删套接字、`pgrep` 确认无残留。
注意：`ding.js` 桌面图标进程可能已被真会话收养——归属活会话的进程别杀，
只清合成器本体和套接字。

## 失败模式对照（R17 踩过）

| 现象 | 根因 | 动作 |
|------|------|------|
| `wl_display_connect failed`，套接字文件存在 | 合成器已崩（僵尸套接字）或没立住 | 删套接字→看 log→重启合成器→重跑，应用侧零改动 |
| 上一把绿、下一把启动失败 | 嵌套 Shell 自身 JS GC 不稳定，自崩 | 同上，重启取对子即可 |
| 窗能跑但恢复/交互语义怪 | 嵌套也是 Mutter 系；程序化恢复本就不是可移植原语 | 按该窗既定口径（止于最小化/恢复走单测），不为嵌套改门禁 |

## 输出

- Wayland 证据 JSON（`backend=wayland` + 该窗门禁 PASS，X11 同窗至少一跑对照）。
- 报告写清：嵌套 GNOME（软件渲染）+ 两遍数据 + 上表命中的异常（如有）+ 清场确认。
- 关 R 时回写真源：X11 证据与 Wayland 证据分开写，不混。

## 禁令自检

- 没 `backend=wayland` = 没证据，禁止标 Wayland 绿。
- 禁止为让嵌套跑通而降该窗门禁、改窗代码迁就合成器。
- 合成器/套接字残留 = 没做完，清场是本 skill 的一部分。
