# L1 开发任务计划 — Phase 0 ~ Phase 3

> **版本：1.0** | 日期：2026-07-26  
> **约束真源：** [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md) v3.0  
> **范围：** 仅 **L1 UI 引擎**（包 `ui/`）+ 为 L1 必要的 `render`/`gpu` 增补  
> **不做：** L2 手势完备、L3 Ant 控件、旧 examples 迁移  

---

## 0. 全局约束（每阶段都遵守）

| ID | 约束 |
|----|------|
| G1 | 依赖只能 `ui → render → gpu`，**ui 禁止 import gpu** |
| G2 | 跨平台句柄：`NativeSurface{Kind, Display, Window}`；现实现 Linux X11 |
| G3 | 布局/Hit/指针 = **逻辑像素、Y-down**；光栅/Surface = **物理（×dpr）** |
| G4 | 帧管道对齐 Flutter：vsync/fallback、有限 depth、背压、pending 覆盖 |
| G5 | 功能不够改 **render 或 gpu**，不在 ui 复制 GPU 栈 |
| G6 | 新示例如 `examples/ui_l1_*`；不维护历史 examples |
| G7 | 测试：`go test ./ui/...`；动 render/gpu 时测对应包 |

### 默认假设

| 项 | 值 |
|----|-----|
| 开发 OS | Linux |
| 第一窗口 | X11；句柄注入 |
| Win/mac | `platform` 接口 + build stub |
| pipeline depth 默认 | 2 |
| vsync | 有信号用真源；否则 16.67ms，并打日志 |

### 阶段完成定义（通用）

- [ ] 本阶段「交付」全部勾完  
- [ ] 本阶段「门禁」命令通过  
- [ ] 未引入 `ui → gpu` import  
- [ ] 真源相关 F 项状态更新（可在本文件勾选）

---

# Phase 0 — 骨架 · 句柄 · 调度 · 清色 Present

**目标：** 空 `ui/` 可编译；宿主传入 X11 句柄；经 **render** 清色上屏；IDLE≈0%；帧计时可打 JSON。  
**非目标：** 控件树、Layer、动画、文字。

## 0.1 目录与包

```text
ui/
  doc.go                 // 模块说明与依赖纪律
  platform/
    host.go              // Host, Event, NativeSurface, PlatformKind
    vsync.go             // VSyncWaiter 接口
    linux_x11.go         // 或 embedder 侧最小 X11；也可靠示例自建句柄
    windows_stub.go
    darwin_stub.go
  scheduler/
    scheduler.go         // IDLE/TRANSIENT/PERSISTENT
    ticker.go            // 注册表骨架（P3 再用满）
    metrics.go           // 帧 JSON 字段
  embedder/
    app.go               // Run 循环：WaitEvents + schedule
    surface.go           // 只调 render 打开/呈现 Surface（不 import gpu）
  raster/
    loop.go              // Raster 线程：收 FramePacket，调 render Present
  // 可选：internal/depcheck 测试禁止 import gpu

render/   # 按需新增门面，例如：
  // surface_host.go  — OpenPresentTarget(NativeSurface) / 等价
  // 内部调 gpu，对外给 ui 用

examples/ui_l1_blank/
  main.go                // 建 X11 窗、注入句柄、清色循环 N 秒退出
```

## 0.2 接口草稿（实现可微调，语义锁定）

```text
// platform
type PlatformKind int // X11, Wayland, Win32, AppKit
type NativeSurface struct { Kind PlatformKind; Display, Window uintptr }
type Host interface {
  NativeSurface() NativeSurface
  Size() (w, h int)          // 逻辑
  ScaleFactor() float64      // dpr ≥ 1
  WaitEvents(timeout time.Duration) []Event
  WakeUp()
  // optional: VSyncWaiter
}

// scheduler
type FrameScheduler struct { ... }
func (s *FrameScheduler) ScheduleFrame()
func (s *FrameScheduler) SetMode(...)
func (s *FrameScheduler) Metrics() FrameMetrics

// embedder → render（名称以 render 导出为准）
// render.NewPresentTarget(ns NativeSurface, logicalW, logicalH int, scale float64) (*PresentTarget, error)
// target.PresentClear(r,g,b,a) error  // P0 最低
// target.Resize(logicalW, logicalH, scale) error
```

若 `render` 尚无「句柄→Surface」门面：本阶段任务包含 **在 render 增加门面**（内部 `gpu`），满足 G1。

## 0.3 任务列表（建议顺序）

| # | 任务 | 包 | DoD |
|---|------|-----|-----|
| P0.1 | 写 `ui/doc.go` 依赖纪律注释 | ui | 文档写明禁止 import gpu |
| P0.2 | 定义 `NativeSurface` / `Host` / `Event` | ui/platform | 单测 stub Host |
| P0.3 | Win/mac stub 文件编译通过 | ui/platform | `GOOS=windows/darwin` 编译 platform |
| P0.4 | `render`：NativeSurface→可 Present 目标门面 | render (+gpu) | 单测或 Linux 真窗；ui 仍不 import gpu |
| P0.5 | `scheduler`：IDLE WaitEvents(-1)；有 schedule 则 vsync/fallback | ui/scheduler | 单测假时钟 |
| P0.6 | `FrameMetrics` 字段：frame 间隔、missed_vsync、pipeline_depth 占位 | ui/scheduler | JSON 可 Marshal |
| P0.7 | `raster.Loop`：独立 goroutine+LockOSThread；队列 depth=2；背压 | ui/raster | 单测：满队列不无限涨 |
| P0.8 | `embedder.App`：事件循环 + 清色 Present 一帧 | ui/embedder | 逻辑通 |
| P0.9 | 示例 `examples/ui_l1_blank`：X11 窗、注入句柄、清色、跑 3s 退出码 0 | examples | 有 DISPLAY 时人工/脚本可跑 |
| P0.10 | 依赖守卫：`go test` 失败若 ui 源文件 import gpu | ui | 测试或 `go list` 脚本 |
| P0.11 | 文档：P0 完成记录（命令与结果可贴 PR） | docs | — |

## 0.4 门禁

```bash
go test ./ui/...
# 有 DISPLAY 时：
go run ./examples/ui_l1_blank   # 清色窗，无崩溃，退出 0
```

| 指标 | 标准 |
|------|------|
| 空闲（无 ScheduleFrame） | CPU 近 0（WaitEvents 阻塞） |
| ui import gpu | **0 处** |
| 左上角（可选） | 若画标记，应在客户区左上（为 P1 铺路） |

## 0.5 F 项

- [x] 开工即遵守 G1–G7  
- [ ] F06 接口存在（真 vsync 可先 fallback）  
- [ ] F14 指标结构存在  

---

# Phase 1 — RenderObject · Picture · 坐标

**目标：** 可布局、可 HitTest、paint 只录制/调用 render 画简单几何；**逻辑/物理/Y-down** 单测绿。  
**非目标：** Layer 缓存、动画、双线程脏层优化。

## 1.1 目录增量

```text
ui/rendering/
  node.go            // RenderObject 接口
  pipeline.go        // PipelineOwner：layout/paint flush
  constraints.go
  box.go             // 最小 RenderBox
  boundary.go        // RelayoutBoundary 标记
ui/painting/
  context.go         // PaintingContext：画 rect 等 → render
ui/scene/
  picture.go         // 最小 DisplayList 或「直接 recording 到 render.Context」过渡
```

**过渡策略：** P1 允许 paint 时同步写入 `render.Context`（仍仅 UI 或单线程），但 **API 形状** 按「先录制再栅格」预留，P2/P3 再拆线程。

## 1.2 任务列表

| # | 任务 | DoD |
|---|------|-----|
| P1.1 | `RenderObject`：layout/paint/hit、parent/child、needsLayout/needsPaint | 单测树 |
| P1.2 | `Constraints` + 最小 `RenderBox`（固定宽高、padding 可选） | 单测 |
| P1.3 | `markNeedsLayout` 冒泡至 **RelayoutBoundary** | 单测截断 |
| P1.4 | `markNeedsPaint` 冒泡（P2 再截断于 RepaintBoundary） | 单测 |
| P1.5 | `PipelineOwner.FlushLayout/FlushPaint` | 二次 flush 无工作量计数 |
| P1.6 | `PaintingContext` 画填充 rect（逻辑坐标）经 render | 像素或 mock |
| P1.7 | **坐标契约测试**：逻辑 Size、scale=2 时物理缓冲关系 | 与 render DeviceScale 一致 |
| P1.8 | **Y-down 测试**：原点标记在「上」；HitTest 与画一致 | 单测/小图 |
| P1.9 | HitTest 逻辑坐标 | 单测 |
| P1.10 | 示例扩展或 headless：画一个逻辑左上色块 | 可选真窗 |

## 1.3 门禁

```bash
go test ./ui/...
go test ./render/...   # 若改了 scale/Y
```

| 指标 | 标准 |
|------|------|
| 无脏时 flush | layout_count=0, paint_count=0 |
| F07 | 逻辑/物理/Y-down 测绿 |

## 1.4 F 项

- [ ] F03 RelayoutBoundary  
- [ ] F07 §4 坐标  

---

# Phase 2 — Layer · RepaintBoundary · COW

**目标：** retained Layer；paint 止于 RepaintBoundary；FramePacket 增量语义；禁止全树 deep copy。  
**非目标：** 完整动画、滚动虚拟化门禁。

## 2.1 目录增量

```text
ui/scene/
  layer.go           // Offset/Opacity/Clip/Picture 层（最小集）
  boundary_layer.go  // RepaintBoundary → 独立 layer 槽
  packet.go          // FramePacket: COW root + mutations + dirty ids
  compositing.go     // needsCompositing 传播
ui/rendering/
  // paint 时 push/pop layer
```

## 2.2 任务列表

| # | 任务 | DoD |
|---|------|-----|
| P2.1 | Layer 类型最小集 + 遍历 | 单测 |
| P2.2 | paint 路径构建 Layer 树 | 单测 |
| P2.3 | **RepaintBoundary**：markNeedsPaint 不冒泡出 boundary | 单测 |
| P2.4 | FramePacket：**共享/COW** + mutations；基准：大树+1 脏不 copy 全节点 | 基准或单测计数 |
| P2.5 | dirty_layer_ids / compositor_dirty_ids 填充 | 单测 |
| P2.6 | compositing bits 向上传播 | 单测 F04 |
| P2.7 | Overlay **band 预留**（可空实现） | API 存在 |
| P2.8 | embedder：Submit packet 到 raster 队列（仍可单线程执行栅格） | 联通 |
| P2.9 | render：按需 **离屏 RT + 纹理 blit** API（若缺则本阶段改 render） | 单测或示例 |

## 2.3 门禁

| 指标 | 标准 |
|------|------|
| 10k 空节点 + 1 boundary 脏 | paint 只进 boundary 子树 |
| 全树 deep copy | 默认路径 **不存在** |
| F01 F03 F04 F13 | 测绿 |

## 2.4 F 项

- [ ] F01 F02 准备（栅格复用可在 P3 闭环）  
- [ ] F04 F13  

---

# Phase 3 — 双线程管道 · 脏层栅格 · 体验雏形

**目标：** §7.1 勾选；S0/S2/S4 数字门禁；官方 Ticker 驱动 Spinner；静态层不每帧 re-raster。  
**非目标：** Ant 控件、完整手势竞技。

## 3.1 目录增量

```text
ui/animation/
  controller.go      // 最小 AnimationController + Ticker
ui/rendering/ 或 scene/
  // Spinner 测试用最小 RenderObject（非 kit）
examples/ui_l1_spinner/
  main.go
```

## 3.2 任务列表

| # | 任务 | DoD |
|---|------|-----|
| P3.1 | Raster 线程与 UI 线程 **正式分离**；UI 不 Wait Present | 注入慢 Present，UI 仍处理事件 |
| P3.2 | 管道 depth=2、背压、pending 覆盖单测 | F 管道 |
| P3.3 | 仅 dirty_layer_ids **re-raster**；静态层复用纹理 | 计数断言 |
| P3.4 | render 热路径缺口补齐：rrect/text 基础/clip/batch（按需改 render） | 能画 Spinner |
| P3.5 | warm-up：首用 pipeline 预热或 jank 预算记录 | 日志/指标 |
| P3.6 | `AnimationController` + Ticker；完成自动卸 | 单测 |
| P3.7 | 测试用 Spinner（RepaintBoundary + 旋转或相位） | 非 kit 包 |
| P3.8 | 示例 `ui_l1_spinner` + 可选 JSON 指标输出 | 真窗 |
| P3.9 | S0：blank 空闲 CPU 抽样 | 文档记录 |
| P3.10 | S2：单 Spinner re-raster 层数=1；layout=0 | 自动或半自动 |
| P3.11 | S4：静态复杂树+1 Spinner，静态层 re-raster=0 | 计数 |
| P3.12 | 遮挡/不可 present：停产帧（能测则测） | F15 基础 |
| P3.13 | §7.1 清单全部勾选，写 `docs` 完成节或 PR 说明 | 关门 |

## 3.3 门禁（数字目标）

| 场景 | 标准（中等机器，可按硬件微调但需记录） |
|------|----------------------------------------|
| S0 空闲 | UI CPU &lt; 1% |
| S2 Spinner | UI CPU &lt; 3%；`raster_layer_count==1`；layout_count==0 |
| S4 | 非 Spinner 层 re-raster==0 |
| 依赖 | ui 无 import gpu |

```bash
go test ./ui/...
go test ./render/...   # 若有改动
go run ./examples/ui_l1_blank
go run ./examples/ui_l1_spinner
```

## 3.4 F 项（P3 关门）

- [ ] F01 F02 F06 F07 F08 F10 F11 F14 F17  
- [ ] F03 F04（P2 应已绿）  
- [ ] §7.1 全勾  

---

## 阶段依赖图

```text
P0 句柄+调度+清色
 └─ P1 RO+坐标
     └─ P2 Layer+COW
         └─ P3 管道+脏层+Spinner ──► L1 体验雏形达标
                                      之后才 P4 滚动门禁 / L2 / L3
```

---

## 每阶段 PR 建议

| 阶段 | PR 标题示例 | 必须贴出 |
|------|-------------|----------|
| P0 | `ui: P0 host surface clear present` | test 输出、blank 运行说明 |
| P1 | `ui: P1 render object and metrics` | 坐标/Y 测 |
| P2 | `ui: P2 layer repaint boundary` | dirty 计数测 |
| P3 | `ui: P3 pipeline spinner gate` | §7.1 勾选表 |

---

## 修订

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-07-26 | 首版：在 ui>render>gpu、句柄注入、清仓仓库约束下的 P0–P3 任务卡 |
