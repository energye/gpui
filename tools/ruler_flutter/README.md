# Flutter 对照尺（P3）

同机、同客户区尺寸、同动画语义的 Flutter 对照臂，供
`docs/RENDER_BASE_RESOURCE_MATRIX.md` 的 §5.5 / 矩阵行 2、3、12 使用。

## 三个场景

| `RULER_SCENE` | 客户区 | 内容 | 对位 gpui 真窗 |
|---|---|---|---|
| `spin` | 480×320 | 一个 64px 图标持续旋转 | `examples/kit/spin_min` |
| `list` | 1200×800 | 1200 项变高列表（44/92/60/76），四类行型 + 24 张图，Steady/Spike/Recover 连续滚动；右侧静态 4×4 区 | `examples/ui_wr_c3_list_scroll` |
| `text` | 1200×800 | `testdata` 同源多脚本/emoji/富文本/多字号密集页，首帧后静止 | `examples/ui_text_m5_anycase` |

窗口尺寸由 `linux/runner/my_application.cc` 按 `RULER_SCENE` 设置；不使用
GTK header bar，避免 header 吃掉客户区高度。三个场景都铺满客户区，没有
额外装饰窗。

## 构建

本机 Flutter SDK：`/home/yanghy/app/projects/gogpu/flutter`（stable 3.47.8）。
当前机器没有 `clang`，用 `g++` 包装脚本过 `flutter doctor` 的 Linux 工具链
检查：

```bash
mkdir -p /tmp/opencode/flutter_toolchain
printf '#!/bin/sh\nexec g++ "$@"\n' > /tmp/opencode/flutter_toolchain/clang++
printf '#!/bin/sh\nexec gcc "$@"\n' > /tmp/opencode/flutter_toolchain/clang
chmod +x /tmp/opencode/flutter_toolchain/clang++
chmod +x /tmp/opencode/flutter_toolchain/clang
export PATH=/tmp/opencode/flutter_toolchain:$PATH

cd tools/ruler_flutter
/home/yanghy/app/projects/gogpu/flutter/bin/flutter doctor -v
/home/yanghy/app/projects/gogpu/flutter/bin/flutter build linux --release
```

## 运行

> **安全（硬）**：批量测量和长循环压测必须在隔离显示里跑（Wayland/XWayland、独立 Xorg 或嵌套 X server）；单窗验收/测量可走当前桌面，但必须先通过隔离验证，按“一轮一窗、跑完查 Xorg”执行。当前桌面只用于单窗验收/测量和看结果。

```bash
export RULER_SCENE=spin       # spin | list | text | full | grid | boundary | overlay | resize | min | cp
export RULER_SECONDS=0        # 0 = 常驻；>0 自退出
./build/linux/x64/release/bundle/ruler_flutter
```

## P4 对照臂（2026-10-27）

矩阵 A 步新增四类能力，全部不依赖 xdotool/wmctrl：

1. **七个新场景**（对应矩阵行 4/5/6/7/8/9/13，与 gpui 对位窗同内容同规模）：
   `full`（全幅重建）、`grid`（20 格异步解码）、`boundary`（16 格边界热轮转）、
   `overlay`（t=1.2s 开盖层保持）、`resize`（响应式布局，GPU 侧两波失效）、
   `min`（FG 面板 + 静格，t=22 最小化）、`cp`（100 条动画贝塞尔）。
2. **runner 动作** `RULER_ACTIONS="<spec>@<t>[,...]"`：GTK 定时执行，
   `minimize` = iconify（行 9），`res:WxH` = 改尺寸（行 7）。时间相对**进程启动**，
   采样器 geo 序列时间相对**测量窗起点**（≈启动+warmup+3s），比对时注意口径
   （实例定位 `RULER_INSTANCE_INDEX`：标题 `RULER_<SCENE>_<i>`，摆位与 gpui
   multiwin 相同 col=i%4 / row=i/4 / X=col*480 / Y=row*380）。
3. **采数器扩展** `measure.py`：`--scene` 七场景、`--instances N`（行 10 多实例臂，
   合计+逐例 CPU/RSS，多窗并发禁止在桌面跑，走隔离显示）、`map_ms`（行 16
   首帧代理 = 进程启动→X 客户区 mapped，0.2s 扫描粒度）、`geo_series`
   （1Hz 几何+隐藏态序列，resize/min 臂的生效铁证）。
4. **诊断开关**（定成本源用，不影响正式臂语义）：`RULER_GRID_NOIMG=1`
   （grid 占位不换图，解码照跑）、`RULER_OVERLAY_NOBARRIER=1`（去全屏调光层）。

应用 stdout 每 2s 打一行：

```json
{"event":"RULER_METRIC","scene":"list","t_s":"10.0","frames":122,"fps":61.0,"p95_ms":16.67,"raster_p95_ms":10.01,"hitch":1}
```

`fps` / `p95_ms` 来自 Flutter `FrameTiming` 的 vsync 间隔；`hitch` 是间隔
>33ms 的帧数；`raster_p95_ms` 是 raster 线程耗时。

## 采数

`scripts/measure.py` 是 Flutter/Godot/gpui 共用的采数器：

- 等真实 X11 窗口出现（不是只等进程）；
- 按 `xwininfo` 的客户区几何截屏，检查不是全黑；
- 预热后从 `/proc/<pid>/stat` 取 utime+stime、从 `/proc/<pid>/status` 取
  VmRSS，测量窗口内算 CPU% / RSS 峰值/斜率；
- 从应用 stdout 摘取 `RULER_METRIC`（Flutter）或 `GODOT_FPS`（Godot）。

```bash
python3 scripts/measure.py --scene spin --window-match RULER_SPIN \
  --duration 60 --warmup 5 --out /tmp/ruler/spin_1
python3 scripts/measure.py --scene list --window-match RULER_LIST \
  --duration 60 --warmup 20 --out /tmp/ruler/list_1
python3 scripts/measure.py --scene text --window-match RULER_TEXT \
  --duration 60 --warmup 5 --out /tmp/ruler/text_1
```

每个 `out` 目录有 `report.json`、`shot.png`、`shot_root.png`（整屏原图）、
`app.stdout`、`app.stderr`。
`--duration` 之外另加 15s 余量给应用自退出（`RULER_SECONDS`）；采数结束先发
WM_DELETE_WINDOW 走应用正常关闭，5s 未退再 SIGTERM，10s 仍未退默认报错收尾
（只有隔离显示可显式 `GPUI_ALLOW_GPU_HARD_KILL=1` 用 SIGKILL 兜底）。

注意事项：

- 口径是**整个进程** CPU（含内核态），不含 Xorg/合成器；与 gpui 的 `/proc`
  口径一致。
- 列表预热用 20s：前 10s 有 shader/图集预热，且列表每 9s 一个
  Steady→Spike→Recover 周期，60s 窗口覆盖约 6.6 个周期。
- Flutter 文本页首帧后没有 ticker，CPU 应该接近 0；这验证的是按需模型
  的静态 idle，不是文本整形成本。与 m5 的 CPU 对比时必须把 m5 的
  HUD 60fps tick 税分开说。
- `shot.png` 只证明画面非黑/内容可见，不是 Golden；资源矩阵不把对照尺
  截图当引擎像素门禁。

### Wayland 会话（行 10 隔离环境）的取证口径

行 10 多实例批量按安全口径必须跑在隔离显示；当前隔离环境是「登录真 GNOME
Wayland 会话 + `GDK_BACKEND=x11` 走 XWayland」。该环境机器抓合成像素三条路
都不通（2026-10-27 实测）：

- GNOME Shell 截图 D-Bus（`org.gnome.Shell.Screenshot.Screenshot`）把外部
  调用拒于白名单之外（`AccessDenied: Screenshot is not allowed`）；
- 系统未装 Shell 白名单内的官方工具 `gnome-screenshot`；
- rootless XWayland 的 root 帧缓冲没有合成内容：`ffmpeg x11grab` 抓根窗
  全黑（非应用故障）；对 GL 呈现的窗口本体 `XGetImage` 返回 `BadMatch`
  （像素在 GPU 侧缓冲，X server 无拷贝）。

因此多实例臂里 `measure.py` 把截图记为 `"source":"unavailable"` 并注明
原因，**不炸轮**；行 10 的机器证据 =
「进程存活 + X11 窗映射（标题/几何）+ 应用侧逐窗 `RULER_METRIC` 帧统计」，
像素证据按「自测 + 人工二合一」转人工目击：正账入表前人工目击确认
（窗口在屏上可见旋转），目录中该轮截图为黑图占位不作为内容证据。
桌面 Xorg 会话的单窗轮仍走 x11grab 截图证据，口径不变。

## 依赖

- Flutter stable（本机 3.47.8，engine `cf636b1dfb`）
- Linux desktop 工具链：GTK3、cmake、ninja、pkg-config、g++ 11.4
- Python 3 + Pillow + python-xlib（采数/截屏）
- `ffmpeg`（`x11grab` 客户区截图）、`xwininfo`
