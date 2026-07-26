# UI 引擎架构真源 — Flutter 管线 × Skia 光栅（`render`）

> **版本：3.1** | 日期：2026-07-27  
> **状态：已批准 · L1 P0–P3 已实现**（收口见 [`ENGINE_L1_CLOSEOUT.md`](./ENGINE_L1_CLOSEOUT.md)）  
> **仓库：** `ui/`（引擎）· `render/` · `gpu/`  
> **图示：** [`ENGINE_ARCH_OVERVIEW.md`](./ENGINE_ARCH_OVERVIEW.md)  
> **P0–P3 任务：** [`ENGINE_PHASE_P0_P3.md`](./ENGINE_PHASE_P0_P3.md)  
> **P4 细卡：** [`ENGINE_PHASE_P4.md`](./ENGINE_PHASE_P4.md) · **P5 细卡：** [`ENGINE_PHASE_P5.md`](./ENGINE_PHASE_P5.md)  
> **P4–P7 大纲：** [`ENGINE_PHASE_P4_P7_OUTLINE.md`](./ENGINE_PHASE_P4_P7_OUTLINE.md)  
> **控件需求（后置 P7）：** [`antd/`](./antd/)

---

## 怎么读

| 问题 | 章节 |
|------|------|
| **L1 做到哪了？** | [`ENGINE_L1_CLOSEOUT.md`](./ENGINE_L1_CLOSEOUT.md) |
| 模块怎么依赖？ | **§1** |
| 四层谁先做？ | **§2** |
| 窗口句柄怎么进 GPU？ | **§3** |
| 什么是逻辑像素 / 物理像素 / Y 轴？ | **§4**（必读） |
| 线程与一帧？ | **§5** |
| 脏区 / Layer / 滚动？ | **§6** |
| Flutter 体验契约 F01–F18？ | **§7** |
| 分期与门禁？ | **§8** + 任务/收口文 |

---

## §1 模块与依赖（硬规则）

### 1.1 仓库模块

| 路径 | 角色 | 对齐 |
|------|------|------|
| `github.com/energye/gpui/ui/...` | **L1 UI 引擎**（Flutter 管线：树、调度、Layer、帧） | Flutter Engine 调度 + rendering |
| `github.com/energye/gpui/render` | **2D 光栅与画布 API** | **Skia 式** Context / 绘制 / Present |
| `github.com/energye/gpui/gpu/...` | **GPU 设备与 Surface** | **wgpu-native（Rust wgpu）** 绑定 |

Go module 根：`github.com/energye/gpui`（见 `go.mod`）。

### 1.2 依赖方向（不可跨层）

```text
        ui
         │  只允许 import render 的公开 API
         ▼
      render
         │  只允许 import gpu 的公开 API
         ▼
        gpu
         │
         ▼
   libwgpu_native / 系统窗口句柄

禁止：
  ui  ──X──► gpu          （跨包，必须经 render）
  render ──X──► ui
  gpu ──X──► render / ui
```

| 规则 | 说明 |
|------|------|
| **ui → render** | 布局/录制结果交给 render 画；Present 走 render 封装 |
| **render → gpu** | Device、Swapchain/Surface、Submit |
| **ui 禁止 import gpu** | 编译期纪律；句柄经 render 或 ui/platform→render 规定路径进入 Surface |
| **功能不够** | 在 **`render` 或 `gpu` 对应位置** 增/删/改，不在 ui 里复制一套 GPU |

### 1.2.1 禁止 CGO（硬）

> 全文细则：[`ENGINE_CODING_RULES.md`](./ENGINE_CODING_RULES.md)

| 规则 | 说明 |
|------|------|
| **禁止 `import "C"` / `#cgo`** | `ui` / `render` / `gpu` / `examples` **全部**适用 |
| **原生 FFI 只用 purego** | `Dlopen` · `RegisterLibFunc` · `NewCallback`（X11 / Wayland / wgpu-native） |
| **`CGO_ENABLED=0` 可构建** | 不得依赖 cgo 才能编译 |

### 1.3 职责切分

| 包 | 做 | 不做 |
|----|----|------|
| **ui** | 帧调度、RO 树、脏区、Layer 描述、HitTest、Ticker、把帧交给 render | **cgo**、直触 gpu、自己管 Device |
| **render** | Skia 式 2D、离屏 RT、文字/图、PresentFrame*、（按需）Picture 回放 | 控件树、业务状态、**cgo** |
| **gpu** | Instance/Device/Queue、**由原生句柄建 Surface**、纹理、提交（**purego** 绑 wgpu） | UI 语义、**cgo** |

---

## §2 四层模型与第一目标

```text
L4 用户业务        【后置】
L3 产品控件 Ant    【后置 · docs/antd】
L2 框架壳（手势/焦点/Overlay…）【L1 后】
L1 UI 引擎         【★ 当前唯一主线 · 本仓库 ui/】
L0 平台句柄 + gpu  【platform SPI + gpu；经 render 使用】
```

**第一目标：** 把 **L1** 做到 Flutter 管线同级丝滑（P0–P3，见任务计划）。  
**L3 控件** 在 L1 §7.1 勾选完成前 **不开主线**。

依赖只允许向下：`L4→L3→L2→L1→L0/render/gpu`。

---

## §3 跨平台与原生句柄（Host 注入）

框架 **跨平台**：Windows / macOS / Linux；当前在 **Linux** 开发。

### 3.1 原则

| 原则 | 说明 |
|------|------|
| 引擎 **不绑定** 某一窗口库 | 宿主创建窗口/容器，把 **显示与窗口句柄** 注入 |
| 通用 SPI | 各平台实现同一接口，句柄用 `uintptr` + 平台枚举 |
| 先 Linux | X11（及后续 Wayland）；Win/mac **接口先齐、实现可 stub** |

### 3.2 概念类型（实现落在 `ui/platform`）

```text
type PlatformKind int
const (
  PlatformX11 PlatformKind = iota
  PlatformWayland
  PlatformWin32
  PlatformAppKit
  // ...
)

// NativeSurface：创建 GPU 可呈现表面所需的最小原生信息
type NativeSurface struct {
  Kind    PlatformKind
  Display uintptr  // X11 Display* / Wayland wl_display*；Win/mac 常为 0
  Window  uintptr  // X11 Window / HWND / NSView* 等「可画容器」
}

// Host：平台能力（输入/尺寸/句柄/可选 vsync）
type Host interface {
  NativeSurface() NativeSurface
  Size() (w, h int)           // 逻辑像素（见 §4）
  ScaleFactor() float64       // dpr，物理/逻辑
  WaitEvents(timeout) []Event
  // 可选：WaitVSync()；无则 Scheduler 用 16.67ms fallback
}
```

### 3.3 数据流

```text
  宿主（示例 / 应用）
    创建 OS 窗口
    得到 Display + Window（或 HWND / NSView）
         │
         ▼
  ui/platform.Host.NativeSurface()
         │
         ▼
  render 封装创建/重建 gpu Surface（内部调 gpu，ui 不 import gpu）
         │
         ▼
  Raster 线程 Present
```

**注意：** 依赖规则要求 **ui 不直接调 gpu**。  
因此「用句柄建 Surface」应暴露为 **`render` 的公开 API**（例如 `render.OpenSurface(NativeSurface)` 或等价），内部再调 `gpu`。  
`ui/platform` 只提供句柄与事件；**创建 Surface 的调用点在 ui 的 embedder 经 render 完成**。

### 3.4 各平台句柄含义（实现对照）

| 平台 | Display | Window/容器 |
|------|---------|-------------|
| Linux X11 | `Display*` | `Window` (XID) |
| Linux Wayland | `wl_display*` | `wl_surface*` |
| Windows | 0 | `HWND` |
| macOS | 0 | `NSView*`（或 CAMetalLayer 指针，按 gpu 绑定约定） |

具体以 `gpu/webgpu` 现有 `CreateSurfaceFrom*` 为准，不足则在 **gpu** 层补。

---

## §4 逻辑像素 · 物理像素 · Y 轴（说明「E」）

你问的 **「Y 轴 / 逻辑-物理像素」** 不是玄学，是两件独立的事：

### 4.1 逻辑像素 vs 物理像素（HiDPI / 缩放）

| 概念 | 含义 | 谁用 |
|------|------|------|
| **逻辑像素（logical / CSS px / dp）** | 布局、HitTest、指针坐标用的单位；**与控件宽高数字一致** | **ui 树全程** |
| **物理像素（physical / 设备像素）** | 屏幕上真实点；GPU 纹理、swapchain 尺寸 | **render 光栅 / gpu Surface** |
| **dpr / deviceScale** | `物理 ≈ 逻辑 × scale`（scale 常 1、1.5、2…） | Host.ScaleFactor() → render.WithDeviceScale |

**例子：**

```text
窗口客户区逻辑大小 800×600，系统缩放 200%（scale=2）

布局：按钮宽 100 逻辑 px
HitTest：鼠标 (50, 20) 也是逻辑坐标
纹理/swapchain：约 1600×1200 物理 px
画按钮：render 在物理缓冲上画宽约 200 物理 px 的区域
```

| 若搞反 | 后果 |
|--------|------|
| 用物理尺寸去做布局 | 控件巨大或点不中 |
| 用逻辑尺寸去配 swapchain 却 scale=2 | 糊、发虚 |
| 指针用物理、布局用逻辑且不换算 | 点击偏移 |

**L1 契约（对齐 Flutter）：**

```text
ui：  Size / Offset / HitTest / 指针  → 一律逻辑像素
render：绘制与 damage 可按 deviceScale 转到物理
gpu：  Surface/纹理尺寸 → 物理像素
Host.Size() → 逻辑；Host.ScaleFactor() → dpr
```

`render` 已有 `WithDeviceScale`：逻辑尺寸建 Context，内部按 scale 处理物理分辨率。L1 必须 **统一走这套**，不要自己另发明一套坐标。

### 4.2 Y 轴方向（Y-down vs Y-up）

| 体系 | Y 增大方向 | 原点常见位置 |
|------|------------|--------------|
| **UI / 多数 2D GUI（含 Flutter 布局）** | **向下**（Y-down） | 左上角 |
| **部分 GPU / 数学/部分图像 API** | **向上**（Y-up） | 左下角 |

若 ui 按「上到下」排控件，render/gpu 某条路径却按「下到上」采样，会出现：

- 整窗上下颠倒  
- 文字/图片翻转  
- HitTest 与画面不一致  

**L1 契约：**

```text
ui 布局与 HitTest：Y-down，原点在内容区左上
交给 render 的绘制坐标：与 render.Context 文档一致（以 render 为准，当前按 2D UI 惯例 Y-down）
若某条 GPU 路径是 Y-up：只在 render/gpu 边界做一次翻转，禁止泄漏到 ui 树
```

**验收：** 在 (0,0) 画一个标记应出现在 **窗口客户区左上**；点击左上命中最上层控件。

### 4.3 一句话记住

```text
逻辑像素 = 量布局、点鼠标
物理像素 = 量屏幕点、GPU 纹理
Y-down     = 界面从上往下排（和 Flutter 一致）
scale      = 物理/逻辑，只在 ui→render 边界乘
```

---

## §5 运行时：线程与一帧（L1）

### 5.1 线程

```text
UI 线程：  输入 · layout · paint→Picture/录制 · Submit 帧描述
           禁止：gpu Submit / 直接操作 Device

Raster 线程（可 LockOSThread）：
           调 render 栅格脏层 · Present
           禁止：读可变 RO 树

（P4）IO 线程：图片解码等
```

跨线程只传 **不可变语义** 的 `FramePacket`（COW + 变更集），**禁止**默认全树 deep copy。

### 5.2 正常一帧

```text
vsync（真信号优先；无则 16.67ms fallback）
  → Ticker → layout(脏) → paint(脏) → Submit(FramePacket)
  → Raster：脏 Layer → render 离屏/绘制 → blit 合成 → Present
  → FrameCompleted → 回收管道槽
```

### 5.3 掉帧（对齐 Flutter，禁止无界 coalesce）

| 情况 | 行为 |
|------|------|
| UI 忙 | miss vsync |
| pending 未开始栅格 | 新包可覆盖 pending |
| 管道满 | **背压**，禁止无限排队 |
| 正在画的帧 | 画完 |

### 5.4 调度模式

| 模式 | 行为 |
|------|------|
| IDLE | 无事阻塞，≈0% CPU |
| TRANSIENT | 短动画/脏，vsync 直到干净 |
| PERSISTENT | 持续 Ticker，每 vsync 一帧 |
| CUSTOM | 仅游戏；产品 UI 禁用 |

---

## §6 场景图与性能模型（L1）

### 6.1 脏区

| 标记 | 停止于 |
|------|--------|
| markNeedsLayout | RelayoutBoundary |
| markNeedsPaint | RepaintBoundary |
| 仅 transform/opacity | compositor_dirty（可不重录 Picture） |

```text
帧成本 ∝ 脏 Layout + 脏 Paint + 脏 Layer 像素 + 合成涉及层
```

### 6.2 FramePacket

```text
frame_id, dpr, viewport(逻辑),
layer_root (COW),
mutations[],
dirty_layer_ids[],      // re-raster
compositor_dirty_ids[]  // 只合成
```

### 6.3 合成

| | 允许 | 禁止 |
|--|------|------|
| Layer 离屏 | 矢量/字/图（经 render） | 直接写 swapchain |
| Present | 纹理 blit 等 | 默认全屏矢量当唯一路径 |
| P3 起 | 静态层纹理复用 | 每帧无条件重传所有层 |

### 6.4 滚动协议（L1，不是 List 控件）

- offset 在 Offset/Transform Layer  
- 虚拟化只挂可见 RO  
- 拖拽 UI 线程最新点  

### 6.5 render 最小热路径（P3，不够就改 render）

rect/rrect、path、text atlas、image blit、clip、batch submit、scale、离屏 RT、Present。

---

## §7 Flutter 体验契约 F01–F18

| ID | 内容 | 最晚 |
|----|------|------|
| F01 | Layer COW，禁全树 deep copy | P2 |
| F02 | 静态层复用；脏层才 re-raster | P3 |
| F03 | RelayoutBoundary | P1–P2 |
| F04 | compositing 传播 | P2 |
| F05 | 滚动协议 | P3 草案 · P4 门禁 |
| F06 | 真 vsync 接口（可 fallback） | P0 |
| F07 | **逻辑/物理 + Y-down（§4）** | P0–P1 |
| F08 | 输入不等 Present | P0/P3 |
| F09 | 动画默认 compositor-only | P3/P5 |
| F10 | GPU/render 热路径 | P3 |
| F11 | warm-up | P3 |
| F12 | IO 解码 | P4 |
| F13 | Overlay band 预留 | P2 |
| F14 | 帧计时 JSON | P0 |
| F15 | 遮挡停帧 | P3 |
| F16 | saveLayer 预算 | P4–P6 |
| F17 | 最小 Ticker/Animation | P3 |
| F18 | Present 策略可文档化 | P3 |

### 7.1 L1 雏形完成勾选（P3 关门 = 原生级 **引擎** 手感）

```text
□ F01 F02 F03 F04 F06 F07 F08 F10 F11 F14 F17
□ §5.3 管道行为测绿
□ S0 空闲 · S2 单 Spinner · S4 复杂页+1 动画 数字门禁
```

未勾完 **不开 L3 控件主线**。  
「雏形」= 相对 L2/L3 的第一完成线，**不是**「手感可以很差」——门禁按原生引擎体验定。

---

## §8 分期（L1）

| 阶段 | 内容 | 详单 |
|------|------|------|
| **P0** | 包骨架、Host 句柄、Scheduler、经 render 清色 Present、帧计时 | [`ENGINE_PHASE_P0_P3.md`](./ENGINE_PHASE_P0_P3.md) |
| **P1** | RenderObject、Picture 录制、RelayoutBoundary、§4 坐标测 | 同上 |
| **P2** | Layer、Boundary、COW、compositing、overlay band 预留 | 同上 |
| **P3** | 真双线程管道、脏层栅格、静态层复用、Ticker、S2/S4 | 同上 |
| P4+ | 滚动门禁、IO、L2… | 后补 |

新验收示例（不碰旧 examples 包历史）：

```text
examples/ui_l1_blank/      P0
examples/ui_l1_spinner/    P3
```

测试范围建议：

```text
go test ./ui/...
go test ./render/...   # 改 render 时
go test ./gpu/...      # 改 gpu 时
# 真窗：go run ./examples/ui_l1_*
```

---

## §9 包目录规划（重建 `ui/`）

```text
ui/
  platform/     # Host、NativeSurface、linux/windows/darwin
  scheduler/    # FrameScheduler、Ticker、帧计时
  rendering/    # RenderObject、PipelineOwner、constraints
  painting/     # PaintingContext → 调 render 录制/绘制接口
  animation/    # 最小 AnimationController（P3）
  scene/        # Layer、Picture 描述、dirty 集
  embedder/     # 粘合 Host + Scheduler + Raster 循环
  raster/       # 仅编排「调 render Present」；禁止 import gpu
  testutil/     # 帧计数、门禁辅助

# 禁止 ui 下出现：直接 import github.com/energye/gpui/gpu
```

`render` / `gpu` 保持现有树；缺 Surface-from-handle 的 ui 友好封装时，**在 render 增加门面**。

---

## §10 风险与修订

| 风险 | 缓解 |
|------|------|
| render 偏 immediate | P2–P3 在 render 补离屏 RT/层 blit；ui 坚持 retained 语义 |
| 误 ui→gpu | 依赖检查脚本 / 代码评审 |
| 坐标/Y 轴混乱 | §4 单测 + 左上角标记场景 |
| 无真 vsync | 接口预留；fallback 可测抖动 |

| 版本 | 说明 |
|------|------|
| 3.0 | 适配清仓后仓库：`ui>render>gpu`；句柄 SPI；§4 逻辑/物理/Y 轴；任务计划外链；废弃 engine/ 命名 |
| 3.1 | L1 P0–P3 实现后：状态改为已实现；挂链 ENGINE_L1_CLOSEOUT |
| 3.2 | 挂链 P4/P5 细卡；下一步改为 P5 |

---

## §11 状态与下一步

**已完成：** L1 P0–P3（见 [`ENGINE_L1_CLOSEOUT.md`](./ENGINE_L1_CLOSEOUT.md)）。

**下一步（可选）：**

1. 存真窗 JSON 为 baseline  
2. P4 ✅ → [`ENGINE_PHASE_P4.md`](./ENGINE_PHASE_P4.md)  
3. **P5 L2 细卡** → [`ENGINE_PHASE_P5.md`](./ENGINE_PHASE_P5.md)（手势/焦点/Overlay；排除 antd）  
4. L3 Ant 仍后置（P7）

**验收：**

```bash
go test ./ui/... -count=1
go run ./examples/ui_l1_blank
go run ./examples/ui_l1_spinner
```

