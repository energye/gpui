# capability_matrix 运行使用文档

`capability_matrix` 是 Linux/X11 **真窗口** 能力矩阵示例，用来在真实 present 路径上验收 Skia 级 2D 画布能力：

```text
render.Context → gpu/webgpu → gpu/rwgpu → libwgpu_native → swapchain present
```

对照文档：

| 文档 | 内容 |
| --- | --- |
| `docs/SKIA_2D_CAPABILITY_MATRIX.md` | Skia 2D 能力真源（矩阵 ID） |
| `docs/CAPABILITY_MATRIX_WINDOW.md` | 窗口验收目标、场景与门禁设计 |
| 本文件 | **怎么编译、运行、读结果、回归、golden** |

程序 **没有命令行 flag**，全部用 `GPUI_*` / `WGPU_*` / `DISPLAY` 等环境变量控制。  
约定：**一个进程只跑一个场景**（`GPUI_SCENARIO=C0x`）。

---

## 1. 前置条件

- **OS / 构建**：Linux，构建标签 `linux && !nogpu`
- **显示**：可用 X11 `DISPLAY`（Wayland 下需 XWayland / 可用 X11 会话）
- **GPU**：本机可加载 WebGPU native（`libwgpu_native.so`）
- **工作目录**：建议在仓库根目录执行（相对路径 `lib/`、脚本路径都按根目录设计）

未设置时程序会尝试：

| 项 | 默认行为 |
| --- | --- |
| `WGPU_NATIVE_PATH` | 若存在 `lib/libwgpu_native.so` 则自动指向它 |
| `GPUI_SURFACE_SAMPLE_COUNT` | 默认设为 `1` |
| `GPUI_SCENARIO` | 未设置或未知 → **C01** |
| `GPUI_ANIM_SECONDS` | 默认 **0**（不限时，关窗口或信号才退出） |
| `GPUI_TARGET_FPS` | 默认 **60**（内部夹在 15–120） |

推荐显式环境（交互 / 脚本通用）：

```bash
cd /path/to/gpui

export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/lib:${LD_LIBRARY_PATH:-}
export DISPLAY=${DISPLAY:-:0}
# 若 X 需要授权：
# export XAUTHORITY=/run/user/$(id -u)/gdm/Xauthority
export GOCACHE=${GOCACHE:-/tmp/gpui-go-cache}
```

---

## 2. 快速开始

### 2.1 交互运行（默认 C01，关窗口退出）

```bash
go run ./examples/capability_matrix
```

窗口标题类似：`gpui capability C01 窗口呈现/清屏`。  
日志会打印场景期望（「应看到: …」），HUD 顶部有 FPS / CPU / RSS。

### 2.2 指定场景 + 定时退出

```bash
GPUI_SCENARIO=C06 GPUI_ANIM_SECONDS=12 go run ./examples/capability_matrix
```

| 变量 | 作用 |
| --- | --- |
| `GPUI_SCENARIO` | `C01` … `C32` |
| `GPUI_ANIM_SECONDS` | `>0` 时到达秒数后 `exit_reason=timeout` 并写结果；`0`/未设则一直跑 |

### 2.3 编译后运行

```bash
go build -o /tmp/capability_matrix ./examples/capability_matrix

GPUI_SCENARIO=C20 GPUI_ANIM_SECONDS=12 /tmp/capability_matrix
```

### 2.4 写 JSON 结果（自动化常用）

```bash
GPUI_SCENARIO=C20 \
GPUI_ANIM_SECONDS=12 \
GPUI_RESULT_FILE=/tmp/cap_C20.json \
go run ./examples/capability_matrix
```

会生成：

```text
/tmp/cap_C20.json       # 完整 JSON
/tmp/cap_C20.json.line  # 单行摘要
```

进程退出码：`PASS` → `0`，`FAIL` → `1`。

---

## 3. 运行时行为说明

### 3.1 主循环

1. 处理 X11 事件（关窗 → 退出）
2. 可选：`GPUI_ANIM_SECONDS` 超时退出
3. `dc.BeginFrame()` 绘制场景 + 可选 HUD
4. `sc.BeginFrame()` 获取 surface 纹理
5. `PresentFrameFull` / `PresentFrameDamage` 提交
6. 采集 FPS EMA、CPU%、RSS、GPU ops / CPU fallback
7. 按 `GPUI_TARGET_FPS` 做帧 pacing

### 3.2 Device lost / 最小化

底层已按 **Flutter / Skia** 模型接入：

- `WGPUDeviceLostCallback` 标记设备丢失
- `Swapchain.EnableAutoRecover` 在下一帧重建 device 并 reconfigure surface
- 示例 **不会** 因 device lost 直接 `SIGABRT` 或进程退出；lost 帧会 skip，恢复后继续画

因此：

- 窗口被其他窗口挡住一部分、失焦：**继续动画与 present**
- 最小化 / 长时间遮挡后再恢复：可能出现短暂 skip 或 recover 日志，应自动恢复

日志示例：

```text
BeginFrame: device lost/recovering (recoveries=N) — skip
GPU device recovered (recoveries=N) — continue rendering
```

### 3.3 如何退出

| 方式 | `exit_reason` |
| --- | --- |
| 点窗口关闭 | `window_close` |
| `Ctrl+C` / `SIGTERM` | `signal` |
| `GPUI_ANIM_SECONDS>0` 到期 | `timeout` |

---

## 4. 场景一览（C01–C32）

一进程一场景：`GPUI_SCENARIO=Cxx`。未知 ID 会回退默认 C01 并打日志。

| ID | 名称 | 中文 | 主要验收点（摘要） |
| --- | --- | --- | --- |
| C01 | SurfacePresentClear | 窗口呈现/清屏 | 稳定 clear + present |
| C02 | TransformStack | 变换栈 | 平移/旋转/缩放栈 |
| C03 | PathFillStroke | 路径填充+描边 | Path fill/stroke |
| C04 | HairlineDash | Hairline+虚线 | Hairline / dash |
| C05 | ClipRectRRect | 矩形/圆角裁剪 | Clip rect / rrect |
| C06 | GradientPattern | 渐变+图案 | Linear/radial/sweep + pattern |
| C07 | BlendModes | 可分离混合 | Multiply/Screen/Overlay 等 |
| C08 | LayerOpacity | 半透明图层 | saveLayer / opacity |
| C09 | ImageWritePixels | 贴图+写像素 | Image + WritePixels |
| C10 | TextCJKDecor | 中英文+装饰 | CJK 文本与装饰线 |
| C11 | FilterBlurShadow | 模糊/阴影滤镜 | Blur / shadow / color filter |
| C12 | MeshVertices | 顶点网格 | drawVertices / mesh |
| C13 | EvenOddPath | EvenOdd 填充 | EvenOdd vs NonZero |
| C14 | MaskLayer | 蒙版图层 | Alpha mask layer |
| C15 | BackdropBlur | 背景采样层 | Backdrop + blur |
| C16 | DamagePresent | 局部 Damage Present | 局部 dirty present |
| C17 | AdvBlendPanel | 高级混合 | SoftLight/Diff/HSL 等 |
| C18 | TextLCDShape | LCD 子像素文本 | LCD / shaping |
| C19 | RRectXYRadii | 独立圆角半径 | 四角独立 rrect |
| C20 | CompositeUI | 多能力 UI 合成 | 多能力同屏 |
| C21 | PorterDuffBoard | PorterDuff 混合板 | PorterDuff 模式 |
| C22 | ClipPathDiff | 路径/Difference 裁剪 | Path clip / difference |
| C23 | GradientTileLocal | 渐变 tile/局部矩阵 | Repeat/Reflect + local matrix |
| C24 | ImageAdvanced | 图高级采样 | mip / 九宫 / 旋转半透明 |
| C25 | TextShapeEmoji | 文本 shaping/混排 | MultiFace / emoji 稳定性 |
| C26 | PathAdvanced | 路径进阶 | 弧 / boolean / path effect |
| C27 | TransformAdvanced | 变换进阶 | 非均匀缩放 / 透视 / miter |
| C28 | LayerFilterGraph | 层混合+滤镜链 | layer + filter graph |
| C29 | QualityMSAAAA | 质量 MSAA/AA | AA / hairline / dither |
| C30 | AtlasPicture | Atlas+Picture 录制 | DrawAtlas + Picture 回放 |
| C31 | PathRasterFast | 路径光栅快径 | 凸/非凸路径快径 |
| C32 | CompositeRegression | 合成压力回归 | 轻量多能力回归 |

仓库内 golden 参考图：`examples/capability_matrix/golden/C01.png` …（用于像素回归，不是运行必需）。

---

## 5. 环境变量完整表

### 5.1 程序本身

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `GPUI_SCENARIO` | `C01` | 场景 ID：`C01`–`C32`（大小写不敏感） |
| `GPUI_ANIM_SECONDS` | `0` | 自动退出秒数；`0` = 不限时 |
| `GPUI_RESULT_FILE` | 空 | 写 JSON 结果路径；同时写 `路径.line` |
| `GPUI_ANIM_LOG_EVERY` | `60` | 每 N 帧打运行日志；`0` 可关（实现上 `N<=0` 不打周期日志） |
| `GPUI_TARGET_FPS` | `60` | 目标帧率（15–120），影响 pacing 与 FPS 门禁 |
| `GPUI_DETERMINISTIC` | off | `1/true`：固定 60Hz 时间线 `t=frame/60`（golden） |
| `GPUI_FIXED_T` | 空 | 固定绘制时间 `t`（覆盖 deterministic） |
| `GPUI_GOLDEN_NO_HUD` | off | `1`：不画 HUD（capture/golden） |
| `GPUI_CAPTURE_DIR` | 空 | 非空时在指定帧写 `${SCENARIO}.png` |
| `GPUI_CAPTURE_FRAME` | `90` | capture 帧号 |
| `GPUI_SURFACE_SAMPLE_COUNT` | 启动时默认 `1` | 表面采样数 |
| `WGPU_NATIVE_PATH` | 自动探测 `lib/libwgpu_native.so` | native 动态库路径 |
| `LD_LIBRARY_PATH` | — | 通常需包含仓库 `lib/` |
| `DISPLAY` | — | X11 display，如 `:0` / `:1` |
| `XAUTHORITY` | — | X 授权文件（无头机/GDM 常见） |

布尔变量接受常见真值：`1` / `true` / `yes` / `on`（见 `util.go`）。

### 5.2 批量脚本额外变量

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `GPUI_CAP_OUT` | 脚本内见下 | 输出目录 |
| `CAPABILITY_MATRIX_BIN` | `/tmp/capability_matrix` 或 `/tmp/cap_l0_bin` | 预编译二进制 |
| `GPUI_P6_MODE` | `regress` | P6：`regress` / `capture-golden` / `compare` |
| `GPUI_GOLDEN_DIR` | `examples/capability_matrix/golden` | golden 目录 |
| `GPUI_CAPTURE_DIR` | `$GPUI_CAP_OUT/capture` | capture 目录（P6） |
| `GPUI_MAX_RMSE` | `0.08` | golden RMSE 上限 |
| `GPUI_MIN_SSIM` | `0.90` | golden SSIM 下限 |

---

## 6. 结果与判定

### 6.1 日志

周期日志示例：

```text
C06 frame=120 fps=59.8 cpu=22.0% rss=180000KB gpu_ops=… cpu_fb=0 probe=true
```

结束日志示例：

```text
DONE C06 status=PASS fps_ema=… fps_avg=… cpu=… cpu_fb=0 gpu_ops=… probe=true reason= exit=timeout
```

### 6.2 JSON 字段（`GPUI_RESULT_FILE`）

| 字段 | 含义 |
| --- | --- |
| `scenario` / `name` / `matrix_ids` | 场景信息 |
| `seconds` / `frames` / `presents` | 时长与帧统计 |
| `fps_ema` / `fps_avg` | 瞬时 EMA / 稳态平均 FPS |
| `cpu_avg` | 平均 CPU%（进程侧采样） |
| `rss_start_kb` / `rss_end_kb` / `rss_steady_delta_kb` | 内存 |
| `gpu_ops` / `cpu_fallback_ops` / `last_fb` | GPU 优先路径统计 |
| `probe_ok` / `probe_note` | 场景语义 probe |
| `status` / `fail_reason` | `PASS` / `FAIL` 与原因 |
| `exit_reason` | `window_close` / `signal` / `timeout` 等 |
| `allow_low_fps` | 是否放宽 FPS 门禁（场景属性） |

### 6.3 PASS / FAIL 门禁（`judgeResult`）

| 条件 | 失败原因示例 |
| --- | --- |
| `frames < 30` | `too_few_frames` |
| `cpu_fallback_ops > 0` | `cpu_fallback_ops=N last=…` |
| `gpu_ops <= 0` | `gpu_ops=0` |
| probe 失败 | `probe_fail:…` |
| 非 low-FPS 场景：`fps_ema < target-5` | `fps_low_steady …` |
| 非 low-FPS 场景：`fps_avg < target-12` | `fps_low_avg …` |
| 稳态 RSS 增量 `> 512MiB` | `rss_steady_delta_kb=…` |

说明：

- 稳态 FPS 会跳过约 1s / 前 45 帧热身，避免首帧 pipeline 编译拖垮 avg
- 交互手动关窗若帧数过少会 FAIL（自动化请设 `GPUI_ANIM_SECONDS` 足够长）

---

## 7. 批量回归

### 7.1 L0：C01–C20

```bash
scripts/run_capability_matrix.sh
```

默认：

- 每场景 `GPUI_ANIM_SECONDS=12`
- 输出 `/tmp/capability_matrix_run/${ID}.json`
- `DISPLAY` 默认 `:0`

自定义：

```bash
GPUI_ANIM_SECONDS=12 \
GPUI_CAP_OUT=/tmp/capability_matrix_run \
scripts/run_capability_matrix.sh
```

使用已编译二进制：

```bash
go build -o /tmp/capability_matrix ./examples/capability_matrix
CAPABILITY_MATRIX_BIN=/tmp/capability_matrix \
GPUI_CAP_OUT=/tmp/capability_matrix_run \
scripts/run_capability_matrix.sh
```

### 7.2 P6 全量：C01–C32

```bash
scripts/run_capability_matrix_p6.sh
```

默认：

- 每场景 8s
- 输出 `/tmp/cap_p6_run`
- `DISPLAY` 默认 `:1`
- 生成日志、JSON，并汇总 `PERF.md` 等

```bash
GPUI_CAP_OUT=/tmp/cap_p6_run \
GPUI_ANIM_SECONDS=8 \
scripts/run_capability_matrix_p6.sh
```

---

## 8. Golden 像素捕获与比较

用于确定性像素回归（关 HUD + 固定时间线）。

### 8.1 批量 capture

```bash
GPUI_P6_MODE=capture-golden \
GPUI_CAP_OUT=/tmp/cap_p6_run \
GPUI_CAPTURE_DIR=/tmp/cap_p6_run/capture \
GPUI_CAPTURE_FRAME=90 \
scripts/run_capability_matrix_p6.sh
```

### 8.2 与仓库 golden 比较

```bash
GPUI_P6_MODE=compare \
GPUI_CAP_OUT=/tmp/cap_p6_run \
GPUI_CAPTURE_DIR=/tmp/cap_p6_run/capture \
GPUI_GOLDEN_DIR=examples/capability_matrix/golden \
scripts/run_capability_matrix_p6.sh
```

或直接：

```bash
python3 scripts/cap_compare_golden.py \
  --got /tmp/cap_p6_run/capture \
  --golden examples/capability_matrix/golden \
  --diff-dir /tmp/cap_p6_run/diff \
  --report /tmp/cap_p6_run/golden_report.json
```

默认门槛：`RMSE <= 0.08`，`SSIM >= 0.90`。

### 8.3 单场景 PNG

```bash
GPUI_SCENARIO=C06 \
GPUI_ANIM_SECONDS=3 \
GPUI_DETERMINISTIC=1 \
GPUI_GOLDEN_NO_HUD=1 \
GPUI_CAPTURE_DIR=/tmp/cap_capture \
GPUI_CAPTURE_FRAME=90 \
go run ./examples/capability_matrix
```

输出：`/tmp/cap_capture/C06.png`。

---

## 9. 常用命令速查

```bash
# 交互默认 C01
go run ./examples/capability_matrix

# 指定场景 12 秒 + 结果
GPUI_SCENARIO=C12 GPUI_ANIM_SECONDS=12 \
  GPUI_RESULT_FILE=/tmp/cap_C12.json \
  go run ./examples/capability_matrix

# 编译
go build -o /tmp/capability_matrix ./examples/capability_matrix

# L0 回归 C01-C20
scripts/run_capability_matrix.sh

# 全量 C01-C32
scripts/run_capability_matrix_p6.sh
```

---

## 10. 故障排查

| 现象 | 排查 |
| --- | --- |
| `XOpenDisplay failed` | 检查 `DISPLAY` / `XAUTHORITY`；本机可先 `echo $DISPLAY` |
| 找不到 `libwgpu_native` | 设 `WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so`，并加 `LD_LIBRARY_PATH=$PWD/lib` |
| 编译失败 / `nogpu` | 确认 Linux 且未强开 `nogpu` tag |
| `status=FAIL too_few_frames` | 交互过早关窗，或 `GPUI_ANIM_SECONDS` 太短 |
| `cpu_fallback_ops>0` | 该场景走了 CPU 回落，查 `last_fb` 与对应 render 路径 |
| `fps_low_*` | 机器负载/驱动；可临时提高 `GPUI_ANIM_SECONDS` 或查是否热身不足 |
| 最小化/切窗后曾崩溃 | 当前库 + 示例已 auto-recover；若仍 abort，保留完整栈与 `recoveries` 日志 |
| golden 全红 | 先确认 `GPUI_DETERMINISTIC=1` + `GPUI_GOLDEN_NO_HUD=1`，再比驱动/字体差异 |

调试建议：

```bash
# 更密日志
GPUI_SCENARIO=C01 GPUI_ANIM_SECONDS=5 GPUI_ANIM_LOG_EVERY=30 \
  go run ./examples/capability_matrix
```

---

## 11. 源码入口

| 文件 | 作用 |
| --- | --- |
| `main.go` | 设备 / swapchain / 主循环 / auto-recover / 结果写出 |
| `scenarios.go` | C01–C32 定义 |
| `probes.go` | 场景绘制与 probe |
| `metrics.go` | JSON / PASS·FAIL 判定 |
| `x11.go` | 最小 X11 窗口 |
| `util.go` | env 解析等 |
| `golden/` | 参考 PNG |

相关脚本：

- `scripts/run_capability_matrix.sh` — C01–C20
- `scripts/run_capability_matrix_p6.sh` — C01–C32 + golden/perf
- `scripts/cap_compare_golden.py` — PNG 比较

---

## 12. 与其它示例的关系

| 示例 | 定位 |
| --- | --- |
| `capability_matrix` | **能力正确性**（矩阵 ID、probe、GPU-first、golden） |
| `mem_anim_window` | **长时间动画 soak**（FPS/RSS/复合场景 Sxx） |
| `particle_kitchen_sink` | 粒子/厨房水槽压测 |
| `window_present` | 最短 present 冒烟 |
| `mem_window_stress` | **无窗口** offscreen 资源 churn |

device lost 策略在上述**有窗口**示例中一致：库层 callback + swapchain auto-recover，应用层 skip 帧并 rebind accelerator，不退出进程。
