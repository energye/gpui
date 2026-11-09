# P3.9b 正式方案：1x/4x 真实窗口动画性能基准（2026-11-09，方向已拍板）

> 依据：`assessment.md` 全量勘察 + 2026-11-09 用户三项拍板（满屏 1080p 留 4K 档 / 混合通用动画+mesh / 当前桌面单窗串行）。
> 状态：方案稿 → 实现。零引擎改动、零新增公开 API。

## 0 直话

- 交付一个**满窗即内容**的混合通用动画真窗：文字、圆角、曲线路径、线性/径向渐变、图片、半透明层、旋转/缩放、粒子点阵，加上 **mesh gradient 网格**一起每帧动；指标行浮在画面左上角。
- 分辨率可配（默认满屏 1920×1080，留 3840×2160 档本机不跑）；MSAA 可配（开窗前 `SetMSAASampleCount`）。
- 三种模式：`-auto-only` 冻结相位做门禁三证据、`-measure` 动画推进测性能、无参数常驻人工窗。
- 当前 Wayland 桌面 1x/4x × native/go 共 4 轮串行测量，出显存/首帧/帧时/CPU 对照表。

## 1 拍板结果（2026-11-09 用户确认）

| 项 | 决定 |
|---|---|
| 分辨率 | **满屏 1920×1080 出真数据**；代码 `GPUI_P39B_W/H` 可配，留 3840×2160 档给大显存机器，本机不跑并登记原因 |
| 场景 | **混合通用动画 + mesh**：文字/圆角/曲线/渐变/图片/半透明层/旋转缩放/粒子 + mesh gradient 网格一起每帧动 |
| 测量环境 | **当前 Wayland 桌面单窗串行**：一轮一窗、程序自己正常退出、每轮前查显示连接、异常即停 |

## 2 窗口契约

### 2.1 可配项（env，全部在 `platform.Open` 前读）

| env | 默认 | 说明 |
|---|---|---|
| `GPUI_P39B_W` / `GPUI_P39B_H` | 1920 / 1080 | 逻辑窗口尺寸；所有场景坐标按 `w,h` 归一化，4K 档自动铺满 |
| `GPUI_P39B_MSAA` | `1` | `1` 或 `4`；`4` 时在开窗前调 `render.SetMSAASampleCount(render.MSAASampleCount4)`，否则显式设 1 |
| `GPUI_BACKEND` | native | 引擎读取；`go` = gwgpu-gles |
| `GPUI_P39B_BREAK` | 空 | `freeze` 人为冻结动画相位，用于「相位推进」逻辑探针的负向对照 |
| `GPUI_ACCEPT_GOLDEN` | 空 | 金图 seed（仅门禁模式） |

### 2.2 三种运行模式

- **`-auto-only`（门禁/三证据）**：相位冻结为常量（画面稳定），跑 6s，出快照 → 逻辑探针 + 像素探针 + 金图逐位（容差 0）→ `EvaluateGates` 判 PASS/FAIL → 出 JSON。
- **`-measure`（性能测量）**：相位 = `time.Since(start).Seconds()` 持续推进，跑 `RUN_SECONDS`（默认 18s），每帧 `ScheduleFrame` + `PresentPolicyFullPaint` 全量重绘，出性能 JSON（逻辑探针 + fps/p95/hitch/显存/首帧/CPU/RSS）。**不做金图比对**（动画画面不稳定）。
- **无参数（常驻人工窗）**：动画持续，`ModePersistent`，`OnEvent` 收真事件打日志并在 HUD 改状态文本，关窗（`EventCloseRequested`→`Quit`→`app.Close`→`win.Close`）后出汇总 + JSON。

### 2.3 场景构成（满窗，指标浮层除外全是内容）

按 `w,h` 归一化；以下每帧随 `phase` 变化：

| # | 图元 | API | 为什么放进来 |
|---|---|---|---|
| 1 | mesh gradient 背景网格 3×3 | `MeshGradient` + `Context.DrawMeshGradient`（CatmullRom，顶点随相位波动/颜色流动） | 被测新能力；大面积曲线边，1x/4x 差异可见 |
| 2 | 线性渐变条 + 径向渐变区 | `rendering.FillLinearGradient/FillRadialGradient` | 通用渐变填充 |
| 3 | 三条三次贝塞尔曲线 | `render.NewPath` + `CubicTo` + `StrokePath` | 路径解析 AA 图元（1x 本就平滑） |
| 4 | 四张旋转圆角卡片 | `pc.PushTransform` + `FillRoundRect` + 圆/文字 + `PopTransform` | 旋转/缩放动画 + 圆角边缘 |
| 5 | 图片旋转缩放 | `image.NewRGBA` 生成 → `ImageBufFromImage` → `DrawImageEx` | 贴图/插值路径 |
| 6 | 半透明叠加层大圆 | `pc.SaveLayer/RestoreLayer` + `FillCircle` | layer pass + blend |
| 7 | 粒子点阵 ×40 | `FillCircle` | 大量小图元 + 每帧变动 |
| 8 | 固定纯色锚点卡 ×2 | `FillRect`（不随相位） | 像素探针稳定锚点 |
| 9 | 指标行（浮层） | 左上角半透明条 + `DrawString` | 帧率/mode/msaa/phase 浮在内容上；金图比对排除此行 |

### 2.4 动画与全量重绘

- 相位无共享状态：OnPaint 内由 `time.Since(startTime)` 或冻结常量计算，避免 raster/UI 线程竞争。
- ticker 每帧 `app.ScheduleFrame()`；`PresentPolicyFullPaint` 保证每帧全量重绘，4x 的光栅化/带宽代价才测得出来。
- 场景绘制计数、mesh 三角数用 `atomic.Int64` 累计，供逻辑探针与 `-race`。

## 3 MSAA 生效的可观测证据（不靠猜）

| 信号 | 来源 | 判据 |
|---|---|---|
| `render.MSAASampleCount()` | `sample_count.go:53` | 等于请求值 |
| `device-surface` 池字节 | `render.GPUPoolStats()` → report `pool` | 4x 显著大于 1x（P3.9a 静态实测 5.67×） |
| `MSAA downgraded` 日志 | `gpu_shared.go:771` OOM 降级 | 必须为空；非空 = 该档撑不住，如实登记 |
| `gpu_fallbacks` / `cpu_fallback_ops` | report | 应为 0 |

这几项写进每轮 JSON 的 `ability_extra`，对照表直接引用。

## 4 验证载体（§4.1 / §4.2）

- **逻辑探针**：`scene_primitives > 0`、`mesh_triangles > 0`、`msaa_setting == msaa_requested`、`device_surface_bytes > 0`、`phase_advanced == (mode==measure)`、`cpu_fallback == 0`、请求/生效 MSAA 一致、分辨率记录。
- **像素探针**（仅门禁冻结相位）：`anchor_magenta`（固定卡 want 255,0,255，tol 6）、`anchor_cyan`（want 0,255,255，tol 6）、`background_not_clear`（非清屏色）。
- **金图**：`testdata/goldens/p39b_anim_<backend>_msaa<1|4>.png`，容差 0；按 backend×msaa 各一份（4x 边缘与 1x 不同，不共用）；比对区 `GoldenRect{0, 44, w, h-44}` 排除指标行，用 `GoldenGateW(cur, base, w, rects)`。
- **双后端**：native 1x/4x、go 1x/4x 四轮；metal 不可验（无 macOS 真机）。
- **负向对照**：① `GPUI_P39B_BREAK=freeze` 在 measure 模式下相位不推进 → 逻辑探针红；② 4x 请求时 device-surface 字节若不大于 1x → 判「4x 未生效/被降级」红。

## 5 测量与安全流程（AGENTS 硬规则）

- 当前 GNOME Wayland 桌面（前序 P3.9/P3.9a 同路径真窗均正常退出，有先例）。
- **一轮一窗、串行**：`RUN_SECONDS=18 go run ./examples/ui_wr_p39b_msaa_anim -measure`，4 组合各一次；程序 `RunFor` 到点自己退出，不并发、不加速循环、不 `kill -9`/`pkill`/`killall`。
- 每轮前健康检查：gnome-shell / Xwayland 存活、`wayland-0` 可连、无 `nvidia_drv.so` 段错误 / `Invalid MIT-MAGIC-COOKIE-1`；异常立即停批报告。
- 门禁轮（`-auto-only`）先 seed 4 份基线（`GPUI_ACCEPT_GOLDEN=1`）再各 verify 一次 diff=0。
- 4K 档本机不跑（1 GiB 显存会 OOM 降级/驱动崩溃风险）。

## 6 门禁命令

```
# 编译/vet（无 GPU）
go build ./examples/ui_wr_p39b_msaa_anim
go vet ./examples/ui_wr_p39b_msaa_anim

# 门禁 seed（4 组合，冻结相位）
GPUI_P39B_MSAA=1 GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=6 go run ./examples/ui_wr_p39b_msaa_anim -auto-only
GPUI_P39B_MSAA=4 GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=6 go run ./examples/ui_wr_p39b_msaa_anim -auto-only
GPUI_BACKEND=go ... (同上两组)

# 门禁 verify（4 组合，diff 应为 0）
GPUI_P39B_MSAA=1 RUN_SECONDS=6 go run ./examples/ui_wr_p39b_msaa_anim -auto-only
...

# 测量（4 组合，动画 18s）
RUN_SECONDS=18 go run ./examples/ui_wr_p39b_msaa_anim -measure
GPUI_P39B_MSAA=4 RUN_SECONDS=18 go run ./examples/ui_wr_p39b_msaa_anim -measure
GPUI_BACKEND=go ... (同上两组)

# 常驻人工窗
go run ./examples/ui_wr_p39b_msaa_anim
```

## 7 影响面与收敛边界

- 只新增 `examples/ui_wr_p39b_msaa_anim/`（main.go + testdata + goldens）与 `evidence/P3.9b/`，改 TASKS/PLAN 文档。
- **不改**：引擎、`gpu/hal`、WGSL、MSAA 默认值、`render.Mesh*`、`ui/kit`、`docs/RENDER_API_CATALOG.md`（无新公开 API，`apidoc` 应仍 rc=0）。
- 复用：P3.9/P3.9a 的 `MeshGradient`/`DrawMeshGradient`、`wrkit`、`wrgate`、`rendering` draw helpers、`ProcessTracker`（RSS/CPU）。
- 不在示例层实现引擎行为；示例只调公开 API + 生成演示数据。

## 8 证据目录

```
evidence/P3.9b/
  assessment.md  design.md  summary.md
  golden-diff.txt  compare.md（1x/4x 对照表）
  gate/            # 门禁 seed/verify 日志（4 组合）
  measure/         # 四轮 JSON + 原始日志
  window/          # 冻结相位 PNG + 4 份金图
  negative-*.txt   # 负向对照
  gates/           # vet / apidoc / build
```

## 9 完成判定

- ✅ = 四轮真窗正常退出、各拿到有效性能 JSON、逻辑探针全绿、1x/4x 显存差异可观测（或如实登记降级）、对照表与结论落盘、三证据齐。
- 若 1080p 4x 也 OOM 降级：如实登记，不粉饰改绿。
- 三证据缺一不标绿；搞不定如实说没修好。
