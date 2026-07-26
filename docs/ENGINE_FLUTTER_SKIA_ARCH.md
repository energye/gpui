# UI 引擎架构真源 — 对齐 Flutter 管线 × Skia 光栅

> **版本：2.0** | 日期：2026-07-26  
> **状态：已批准 · 架构设计（代码未实现）**  
> **包名约定：`engine/` 新栈**（不在旧 `ui/` / `render/` 上打补丁）  
> **图示总览：** [`ENGINE_ARCH_OVERVIEW.md`](./ENGINE_ARCH_OVERVIEW.md)  
> **产品控件需求（后置）：** [`antd/`](./antd/)

---

## 怎么读本文（先看这段）

| 你想知道 | 去哪一节 |
|----------|----------|
| 整体分几层？第一目标是什么？ | **§1 四层模型** |
| 和 Flutter 哪一层对应？ | **§1.3、§2** |
| 线程 / 一帧怎么跑？ | **§3** |
| 脏区 / 滚动 / GPU 规则？ | **§4** |
| 体验要对齐 Flutter 还缺什么契约？ | **§5（F01–F18）** |
| 分几期做？何时才能开控件？ | **§6** |
| 怎么验收算过关？ | **§7** |
| 和现网旧代码什么关系？ | **§8** |

**第一目标（当前唯一主线）：**  
把 **L1 UI 引擎** 做成与 Flutter **Engine + rendering** 同级（管线、脏区、线程、合成），达到原生级丝滑。  

**明确后置：**  
**L3 产品控件**（Ant Kit）和 **L4 用户业务** —— 引擎未达标前 **禁止** 以堆控件冒充进度。

---

## 0. 一句话目标

```text
帧成本 ∝ 脏区面积与脏层数
静止 ≈ 0% CPU
局部动画不拖垮整页
输入不被 GPU 堵住
```

| 场景（桌面 60Hz，中等机器） | 门禁 |
|------------------------------|------|
| 空闲窗口 10s | UI 线程 CPU &lt; 1%，Raster 可睡眠 |
| 单 Spinner | UI CPU &lt; 3%；每帧只 re-raster **1** 个 layer；Layout=0 |
| 列表滚动（虚拟化） | 输入延迟 ≤ 1 帧；只处理可见行 |
| 复杂页 + 1 个局部动画 | 非动画子树 Paint/Raster = 0 |
| 全页主题切换 | 允许 **1 次**全量，之后回到局部 |

达不到门禁 = 当前阶段未完成，**不得**进入下一阶段（尤其不得开 L3 控件库）。

---

# §1 四层模型（架构总纲 · 必读）

## 1.1 总图

```text
┌──────────────────────────────────────────────────────────────────────────┐
│  L4  用户 / 业务层                                          【后置】      │
│  页面 · 业务流程 · 业务状态                                                │
│  只组合 L3 控件（偶尔用 L2 API），禁止碰 GPU / Raster                      │
├──────────────────────────────────────────────────────────────────────────┤
│  L3  产品控件层（Ant Kit 等）                               【后置】      │
│  Button / Table / Modal 产品 API · 视觉 Token · 控件语义                   │
│  只依赖 L2 + L1，禁止持有 Device / Texture / 私有 Present                  │
├──────────────────────────────────────────────────────────────────────────┤
│  L2  框架基础设施（Framework Shell）                    【控件前建议齐】   │
│  手势竞技 · 焦点 · Overlay 挂载壳 · 滚动视口协议 · 动画 API 完备            │
│  不是「某个 Ant 控件」，但是所有复杂控件的底座                               │
├──────────────────────────────────────────────────────────────────────────┤
│  L1  UI 引擎（第一目标 · 对齐 Flutter Engine + rendering）   【当前主线】  │
│  线程/vsync/管道 · RenderObject/Layout/Paint · Layer/Picture               │
│  合成 Present · 脏区 · 文本/图基础光栅 · dpr · 帧计时                       │
├──────────────────────────────────────────────────────────────────────────┤
│  L0  平台与 GPU 后端                                        【引擎内部】   │
│  窗口/输入/VSyncWaiter/Surface · wgpu 等（仅 Raster 线程碰 GPU）           │
└──────────────────────────────────────────────────────────────────────────┘

依赖方向（只允许向下）：

  L4 → L3 → L2 → L1 → L0

禁止：

  L3/L4 ──X──► L0 GPU
  L1 ──X──► L3（引擎不得依赖控件）
  任意层在 UI 线程 ──X──► Queue.Submit / Swapchain
```

## 1.2 每层一句话 + 包不包含

### L1 — UI 引擎（第一目标）

| | |
|--|--|
| **是什么** | 让「一棵界面树」能按 Flutter 方式 **布局、绘制、分层、异步栅格、按需呈现** 的核心 |
| **对齐 Flutter** | `Engine`（Raster/提交/vsync）+ `package:flutter/rendering`（RO/Layer/Pipeline）+ `scheduler` 帧模型 |
| **包含** | 见 §1.4 清单 |
| **不包含** | Ant Button、业务页面、Material 皮肤 |
| **完成标志** | §6.3 / §1.8.3 式门禁 + §7 场景 S0/S2/S4 等 |

### L2 — 框架基础设施

| | |
|--|--|
| **是什么** | 控件会用到的 **通用机制**，但 **不是** 某个产品控件 |
| **对齐 Flutter** | `gestures`、`Focus`、`Overlay` 机制、`Scrollable`/`Viewport` 协议、完整 `AnimationController` |
| **包含** | 手势竞技、焦点遍历、Overlay 挂载、滚动视口+虚拟化协议、动画 API |
| **不包含** | `kit.Button`、带 Ant 视觉的 Modal 产品 API |
| **何时做** | L1 雏形后、L3 之前（强烈建议齐，否则控件会各自发明手势/焦点） |

### L3 — 产品控件层

| | |
|--|--|
| **是什么** | 给业务用的 **具名控件** 与设计体系 |
| **对齐** | 需求见 `docs/antd`（Ant Design）；**不是**复刻 Flutter Material API |
| **包含** | Button、Table、Form、DatePicker… Token/Skin |
| **依赖** | 只许 L2+L1 |
| **何时做** | **L1 达标 + L2 基本齐** 之后（P7） |

### L4 — 用户 / 业务层

| | |
|--|--|
| **是什么** | 具体 App：页面、流程、业务状态 |
| **做法** | 组合 L3 控件；需要深度定制时可用 L2/L1 积木 |
| **禁止** | 直接 GPU、绕过脏区模型刷全树 |

### L0 — 平台与 GPU（引擎内部）

| | |
|--|--|
| **是什么** | 窗口、输入、真 vsync、Swapchain；GPU Device 仅在 Raster 线程 |
| **谁用** | 仅 L1 embedder / raster backend |
| **控件与业务** | **零直接依赖** |

## 1.3 和 Flutter 目录的对应（一看便知）

| Flutter | 本架构层级 | 本仓库阶段 |
|---------|------------|------------|
| Engine（IO/UI/Raster 协作、GPU、呈现） | **L1 + L0** | 第一目标 |
| `rendering`（RenderObject、Layer、PipelineOwner） | **L1** | 第一目标 |
| `scheduler`（帧、Ticker） | **L1** | 第一目标 |
| `painting`（画布、文本绘制接口） | **L1** | 第一目标（深度分期） |
| `gestures` | **L2** | 控件前 |
| `widgets` 里 Focus / Overlay / Scrollable 机制 | **L2**（机制） | 控件前 |
| Material / Cupertino **组件** | **L3**（我们用 Ant） | 后置 |
| 业务 App | **L4** | 后置 |

## 1.4 L1 UI 引擎：详细清单（第一目标范围）

下列全部属于 **引擎**，不是控件：

| 模块 | 职责 | 非目标 |
|------|------|--------|
| **FrameScheduler** | IDLE / 按需 / vsync；管道深度；背压；帧计时 | 业务定时器逻辑 |
| **Ticker / 最小 Animation** | 驱动动画帧；完成即卸 | Ant 风格动效皮肤 |
| **RenderObject + Constraints** | 布局、paint 分 phase、HitTest | Button 产品 API |
| **RelayoutBoundary / RepaintBoundary** | 截断 layout/paint 脏 | — |
| **Layer 树 + Picture/DisplayList** | retained 场景；COW+变更集 | — |
| **PaintingContext** | 只录制 draw op，不碰 GPU | — |
| **Raster 线程 + Compositor** | 脏层栅格、blit present | — |
| **GPU Backend 热路径** | rect/rrect/path/text atlas/image blit/clip/batch | 完整 Skia 冷门 API |
| **坐标 / dpr** | 逻辑布局、物理光栅 | — |
| **VSyncWaiter / Surface 生命周期** | 真 vsync、遮挡停帧 | — |
| **滚动视口协议（引擎侧）** | OffsetLayer + 虚拟化 **接口** | Material ListView 产品 |
| **基础文本/图** | 能画字、能贴图、atlas | 完整富文本编辑器产品 |

## 1.5 依赖与禁止（硬规则）

```text
允许：
  L4  import L3, L2, L1（推荐只用 L3）
  L3  import L2, L1
  L2  import L1
  L1  import L0（platform / gpu backend 内部）

禁止：
  L1 import L2/L3/L4
  L2/L3/L4 直接调用 GPU Device / Swapchain / Queue.Submit
  控件在 Tick 里 MarkNeedsLayout 刷整树（无 boundary）
  产品 UI 默认 Continuous 死循环刷帧
```

**编译期目标：** `engine/ui` ↛ `engine/raster` 的反向依赖；  
`kit` ↛ `gpu` / `raster/backend`。

## 1.6 目标优先级（写进排期）

| 优先级 | 内容 | 对应 |
|--------|------|------|
| **P0 主线** | L1 UI 引擎达到 Flutter 体验雏形 | 本文 §6 Phase 0–3 |
| **P1 主线** | L1 加深（滚动门禁、文本/图 IO）+ L2 框架壳 | Phase 4–5 |
| **P2 后置** | L3 产品控件（Ant） | Phase 7 + `docs/antd` |
| **P3 后置** | L4 业务 | 产品工程 |

---

# §2 与 Flutter / Skia 对齐策略

## 2.1 总判断

| 层面 | 策略 |
|------|------|
| **性能关键路径** | **不简化** — 与 Flutter/Skia 生产模型同构 |
| **产品表面积** | **有意缩小** — 不做完整 Material、不做全平台嵌入 |
| **实现深度** | **分期** — 先 L1 正确，再加深 |
| **语言** | Go + wgpu 路线，不是 Dart VM |

**一句话：**  
管线 = Flutter 生产级同构；控件与生态 = 不做成 Flutter 克隆。

## 2.2 必须同构（L1 禁止再砍）

| # | Flutter / Skia | 本引擎 |
|---|----------------|--------|
| 1 | UI 线程 ≠ Raster 线程 | 同 |
| 2 | 不共享可变 Element/RO 树 | 只传 `FramePacket`（COW+变更集） |
| 3 | RO layout / paint 分 phase | 同 |
| 4 | Layer + RepaintBoundary | 同 |
| 5 | markNeedsPaint 止于 paint boundary | 同 |
| 6 | Paint → Picture，非直接 GPU | 同 |
| 7 | Present 以纹理合成为主 | blit-only present |
| 8 | Scheduler idle / vsync | FrameScheduler |
| 9 | Ticker 完成即卸 | 同 |
| 10 | 有限管道 + 背压 + pending 覆盖 + miss vsync | 同 §3.3 |
| 11 | Layer 增量，禁每帧全树深拷贝 | 同 §4.2 |
| 12 | RelayoutBoundary + compositing 传播 | 同 §4.1 / §4.3 |
| 13 | Scroll offset 在 Layer + 虚拟化 | 同 §4.5（引擎协议） |
| 14 | 真 vsync · dpr · 帧计时 | 同 §3 / §7 |

## 2.3 有意不做（避免误解「整体 = 复刻 Flutter」）

| 不做 | 原因 |
|------|------|
| 完整声明式 Widget 强制生态 | 性能不依赖声明式；L1 可用命令式挂 RO |
| Material/Cupertino 组件库 | L3 走 Ant（`docs/antd`） |
| iOS/Android/Web 全嵌入 | 桌面优先 |
| Dart Isolate 通用模型 | 固定 UI + Raster（+ IO） |
| 完整 DevTools / 热重载 | 仅帧 JSON + 后期 HUD |
| 完整 Skia 文档/PDF/冷门 API | UI 2D 子集 |

---

# §3 运行时：线程 · 一帧 · 掉帧（L1 核心）

## 3.1 线程与所有权

```text
╔══════════════════════════ UI THREAD ══════════════════════════╗
║  职责：输入 · 布局 · 命中 · Ticker · 录制 Picture · 提交帧      ║
║  拥有：RenderObject 树（可变）· 焦点/手势状态（L2 也在此线程）  ║
║  禁止：Device · Swapchain · Queue.Submit                      ║
╚══════════════════════════════╤════════════════════════════════╝
                               │ FramePacket（COW + mutations）
                               │ vsync 节奏 · 有限管道 · 背压
                               ▼
╔════════════════════════ RASTER THREAD ════════════════════════╗
║  职责：消费已提交帧 · 栅格脏 Layer · blit 合成 · Present       ║
║  拥有：GPU · Layer 纹理 · Compositor                          ║
║  禁止：读可变 RO 树 · 业务状态机 · 布局                        ║
╚═══════════════════════════════════════════════════════════════╝

（L1 加深 / L2 前）IO Thread：图片解码等，不进 UI/Raster 热路径
```

**铁律：**

1. 可变 UI 树 **永不** 跨线程共享  
2. 跨线程只传 **不可变语义** 的 `FramePacket`（共享/COW，**禁止**默认全树 deep copy）  
3. GPU **仅** Raster 线程  
4. UI **不** Wait Present；管道满走 **背压**  
5. HitTest 用 UI 侧 **逻辑像素** 几何，不等 GPU  
6. 帧节奏 = Flutter Animator 同构（§3.3）

## 3.2 正常一帧（对齐 Flutter）

```text
vsync（或 IDLE 脏唤醒后等下一 vsync）
  → UI: beginFrame
  → UI: Ticker.tick
  → UI: layout(dirty only)     // 止于 RelayoutBoundary
  → UI: paint(dirty only)      // 止于 RepaintBoundary → 写 Picture
  → UI: Submit(FramePacket)    // 受管道深度约束
  → Raster: 取待处理帧
  → Raster: dirty layers → 离屏 RT
  → Raster: blit-only composite → Present
  → FrameCompleted → 回收管道槽
```

## 3.3 落后与掉帧（禁止「无界 coalesce」）

| 情况 | 行为（与 Flutter 相同） |
|------|-------------------------|
| 正常 | 一拍 vsync → 一帧 → Raster 画该帧 |
| UI 忙 | **miss vsync**，帧率下降 |
| Raster 忙、pending 未开始 | 新包 **覆盖** pending（最新状态） |
| 管道满 | **背压**，禁止无界队列 |
| 正在栅格中的帧 | 画完；不与半帧错误合并 |

```text
  UI Submit ──► [ in-flight … | pending? ] ──► Raster
                    │                │
                    │ 满→背压        │ 未开始可被更新覆盖
                    └────────────────┘
```

**vsync：** 优先 `platform.VSyncWaiter` 真信号；16.67ms **仅**无信号兜底。

## 3.4 帧调度模式

| 模式 | 条件 | 行为 |
|------|------|------|
| **IDLE** | 无 ticker、无 dirty、无 pending | 阻塞等事件，≈0% CPU |
| **TRANSIENT** | 短脏/短动画 | vsync 画到干净 |
| **PERSISTENT** | 持续 Ticker | 每 vsync 一帧 |
| **CUSTOM** | 游戏/粒子 | 可选；**产品 UI 默认禁用** |

---

# §4 场景图 · 脏区 · 滚动 · GPU（L1 细则）

## 4.1 三棵概念树

```text
  （可选）Widget 配置
        ↓
  （可选）Element 生命周期
        ↓
  RenderObject 树  ──paint──►  Layer 树  ──raster──►  Texture
  (layout / hit)              (retained)              (GPU)
```

| 脏标记 | 行为 |
|--------|------|
| `markNeedsLayout` | 向上到 **RelayoutBoundary** 停止 |
| `markNeedsPaint` | 向上到 **RepaintBoundary** 停止 |
| `markNeedsCompositing` | 属性可只合成；并 **向上传播** needsCompositing |

```text
帧成本 ≈ O(脏 Layout) + O(脏 Paint) + O(脏 Layer 像素) + O(合成涉及层)
      ≠ O(整棵控件树) ≠ O(整屏像素)（除非全脏）
```

## 4.2 FramePacket（增量 · 非全树拷贝）

```text
FramePacket {
  frame_id, dpr, viewport,
  layer_root,              // 共享 / COW 引用
  mutations[],             // 本帧结构/属性/picture 变更
  dirty_layer_ids[],       // 必须 re-raster
  compositor_dirty_ids[],  // 仅合成（opacity/transform 等）
}
```

| 允许 | 禁止 |
|------|------|
| 未脏节点跨帧复用 | 每帧 deep copy 全树为默认 |
| 仅脏层重录 Picture | 全树每帧重录 |
| 属性动画只写 compositor_dirty | 属性动画整页 markNeedsPaint |

## 4.3 Compositing 传播（对齐 Flutter）

| 规则 | 说明 |
|------|------|
| 子需合成 → 父 `needsCompositing` | 同 `updateCompositingBits` |
| `alwaysNeedsCompositing` | opacity / clip / transform / filter 等 |
| `isRepaintBoundary` | 独立 paint/raster 单元 |

## 4.4 合成契约

| 阶段 | 允许 | 禁止 |
|------|------|------|
| Layer Raster | 矢量、AA、clip | 写 Swapchain |
| Present | 纹理 blit、简单 opacity/filter | 默认全屏矢量重画 |
| L1 雏形起 | 脏层 re-raster；**静态层纹理复用** | 每帧无条件重传全部层 |
| 后期增强 | 更细 dirty rect、OS damage | 把「静态层复用」拖到最后才做 |
| saveLayer | 小面积 + 独立 Boundary + 预算 | 大表/全屏 saveLayer 默认 |

## 4.5 滚动：引擎协议（L1），不是 List 控件（L3）

| 规则 | 说明 |
|------|------|
| Offset 在 **OffsetLayer / TransformLayer** | 优先 compositor-only |
| 虚拟化 | 只挂载可见（+缓存窗）RO；每帧 bind 有上界 |
| 拖拽 | UI 线程即时消费；move 可合并，**用最新点** |
| 禁止 | 1k 行全 mount；滚动默认全链 markNeedsLayout |

**L3 的 Table/List** 只是消费该协议，**协议本身属于引擎/框架**。

## 4.6 GPU 最小热路径（L1 雏形门禁）

| 必有 | |
|------|--|
| 实心 rect / rrect | |
| 路径填充（可分期 stroke） | |
| 纹理 blit | |
| Glyph atlas + 画字 | 无字不成 UI |
| Clip rect（rrect 更佳） | |
| 批处理 / 限制 submit 次数 | |
| Shader/pipeline warm-up | 降首帧 jank |
| RT/纹理池 | |

## 4.7 坐标与输入

| 项 | 契约 |
|----|------|
| 布局 / HitTest / 指针 | **逻辑像素** |
| 光栅 / Layer RT | **物理** = 逻辑 × dpr（规则固定可测） |
| 事件 | UI 线程处理，**不等** Present |
| Pointer move | 可 coalesce 为最新；Down/Up/滚轮保序 |

---

# §5 Flutter 体验硬契约（F01–F18）

> 下列曾是文档缺口；现为 **L1（及标明的 L2）硬契约**，不是 P6 可选项。

| ID | 能力 | 层级 | 最晚阶段 |
|----|------|------|----------|
| **F01** | Layer COW+mutations，禁全树 deep copy | L1 | P2 |
| **F02** | 静态层纹理复用；脏层才 re-raster | L1 | P3 |
| **F03** | RelayoutBoundary 与 paint 同等 | L1 | P1–P2 |
| **F04** | compositing bit 向上传播 | L1 | P2 |
| **F05** | Scroll offset Layer + 虚拟化协议 | L1（协议） | P3 草案 · P4 门禁 |
| **F06** | 真 VSyncWaiter | L1/L0 | P0 |
| **F07** | 逻辑/物理坐标 | L1 | P0–P1 |
| **F08** | 输入即时、最新点跟手 | L1 | P0/P3 |
| **F09** | 动画默认 compositor-only | L1 最小 · L2 完备 | P3/P5 |
| **F10** | GPU 热路径 + 少 submit | L1 | P3 |
| **F11** | warm-up | L1 | P3 |
| **F12** | IO 解码线程 | L1 | P4 |
| **F13** | Overlay **band 机制**（非 Modal 产品） | L1 预留 · L2 机制 | P2/P5 |
| **F14** | 帧计时 / jank 字段 | L1 | P0 |
| **F15** | 遮挡停帧与恢复 | L1/L0 | P3 |
| **F16** | saveLayer 预算 | L1 | P4–P6 |
| **F17** | 最小 Ticker/Animation（非私门路 Spinner） | L1 | P3 |
| **F18** | Present 节拍策略可文档化 | L1/L0 | P3 |

### 纠正误解

| 错 | 对 |
|----|-----|
| Raster 旁路听所有 dirty | UI **主动** Submit；Raster **任务驱动** |
| 无界 coalesce = Flutter | vsync + 有限管道 + 背压 + pending 覆盖 |
| 有 Boundary 就丝滑 | 还要增量 Layer、合成复用、滚动协议、GPU 热路径 |
| 线程跑通 = 引擎完成 | 必须 §5.1 勾选表 |

### 5.1 L1「Flutter 体验雏形」完成勾选（P3 关闭条件）

```text
□ F06 真 vsync（或平台限制书面声明 + fallback）
□ F01 无默认全树 deep copy（大树+1 动画 Submit 成本不随 N 线性爆）
□ F02 每帧 re-raster 层数 = dirty 集；静态层复用
□ F03/F04 layout/paint/compositing 单测绿
□ F07 dpr 契约测绿
□ F08 Raster 忙时 pointer 仍处理，拖拽最新点
□ F10 最小热路径 + 无每图元 Submit
□ F11 warm-up 或首帧 jank 预算内
□ F14 帧计时 JSON 齐全
□ F17 官方 Ticker 路径 Spinner（S2）
□ §3.3 管道/背压/pending 测绿
□ §0 / §7 中 S0、S2、S4 数字门禁达标
```

**未全勾：不得宣称引擎完成，不得开 L3 控件库。**

---

# §6 分期实施

## 6.1 原则

1. **先 L1，再 L2，再 L3** — 禁止并行堆 Ant 控件冲进度  
2. 每阶段可演示 + 可测；未过门禁不进下一阶段  
3. 不为单场景写私有 GPU 快路径  
4. 滚动/文本基础算 **L1**，不算「等控件层再做」

## 6.2 阶段与层级对照

```text
Phase 0–3  ──►  L1 UI 引擎（第一目标 · Flutter 体验雏形）
Phase 4    ──►  L1 加深（滚动门禁、文本/图 IO）
Phase 5    ──►  L2 框架壳为主（手势/焦点/Overlay/动画完备）
Phase 6    ──►  L1 增强（细 damage、多窗、HUD）
Phase 7    ──►  L3 产品控件（Ant）
之后       ──►  L4 业务
```

## 6.3 各阶段交付与门禁

### Phase 0 — 契约骨架（L1）

| 交付 | 门禁 |
|------|------|
| 双线程壳 · FramePacket 形状 · Scheduler（VSyncWaiter+管道+背压）· 清色 Present · 帧计时 F14 · 跨线程访问 fail | 空闲 CPU&lt;1%；UI 不 Submit GPU；管道满有背压；计时字段可观测 |

### Phase 1 — RenderObject + Picture（L1）

| 交付 | 门禁 |
|------|------|
| RO · PipelineOwner · Box/Flex · Picture · HitTest · RelayoutBoundary · dpr | 二次 layout/paint=0；boundary 截断 layout；golden |

### Phase 2 — Layer 增量（L1）

| 交付 | 门禁 |
|------|------|
| Layer 类型 · Boundary · COW/mutations · compositing 传播 · overlay **band 预留** | 10k 节点+1 动画仅 paint boundary；无全树 deep copy；opacity 0 次重录 Picture |

### Phase 3 — 异步 Raster + GPU 热路径（L1 雏形关闭）

| 交付 | 门禁 |
|------|------|
| Raster 循环 · §3.3 · §4.6 热路径 · 静态层复用 · 最小 Ticker/Animation · warm-up · 输入跟手 · surface 停帧 | **§5.1 全勾**；S0/S2/S4 达标 |

### Phase 4 — L1 加深

滚动硬门禁（F05）· IO 解码 · 文本/图加深 · clip。S5/S6 绿。

### Phase 5 — L2 框架壳

手势竞技 · 焦点 · Overlay **机制填满** · 动画 API 完备 · Semantics 最小。  
**仍不是** Ant Modal 产品控件。

### Phase 6 — L1 增强

细 dirty rect · RasterCache · 多窗 · HUD。

### Phase 7 — L3 控件

`docs/antd`；动画走 Controller；动效默认 Boundary；禁止控件持 GPU。

## 6.4 节奏参考

| 阶段 | 量级 | 标志 |
|------|------|------|
| P0 | 2–3 周 | vsync+计时+空窗 |
| P1 | 3–5 周 | RO 语义 |
| P2 | 3–4 周 | Layer 增量 |
| P3 | 5–7 周 | **§5.1 全勾 = L1 雏形完成** |
| P4 | 3–6 周 | 滚动/IO |
| P5 | 3–6 周 | L2 壳 |
| P6 | 持续 | 增强 |
| P7 | 产品并行 | L3 |

---

# §7 验收

## 7.1 场景矩阵

| ID | 场景 | 主要验证 | 主要层级 |
|----|------|----------|----------|
| S0 | 空窗 IDLE | CPU≈0 | L1 |
| S1 | 低频闪点 | 非闪烁不 raster | L1 |
| S2 | 单 Spinner boundary | 1 layer/frame | L1 |
| S3 | 10 Spinner | 禁止全树 paint | L1 |
| S4 | 复杂静态页+1 Spinner | 静态层 0 re-raster | L1 |
| S5 | 列表滚动 | 虚拟化上界 | L1 协议 |
| S6 | 拖拽 | 输入延迟 | L1 |
| S7 | Modal 淡入 | overlay band | L2 机制（后 L3 产品） |
| S8 | resize | 全量后恢复局部 | L1 |
| S9 | 遮挡/恢复 | 无泄漏 | L1/L0 |
| S10 | 主题切换 | 一次全量可接受 | L1+L3 后 |

## 7.2 JSON 指标（P0 起）

| 字段 | 含义 |
|------|------|
| `ui_cpu` / `raster_cpu` | 分线程 |
| `layout_count` / `paint_count` | 本帧触及 |
| `raster_layer_count` | **re-raster** 层数（≈dirty） |
| `composite_layer_count` | 参与合成层数 |
| `frame_build_ms` / `frame_raster_ms` | 分轨耗时 |
| `frame_time_p50` / `p99` | 帧间隔 |
| `missed_vsync` | 掉拍 |
| `pipeline_depth` | 在途帧 |
| `gpu_submit_count` | 防碎提交（P3+） |

---

# §8 与现网代码 · 包结构

## 8.1 策略

| | |
|--|--|
| 新路径 | `engine/` |
| 可借用 | wgpu、字体、shader 算法、测试资源 |
| 不可借用 | 旧 Tree 脏语义、同步 Present hop、控件直触 Context |
| 旧栈 | 可暂存产品；新栈达标再切默认 |
| 控件迁移 | 仅 P7+；不保证旧 API 兼容 |

## 8.2 包结构（与层级对应）

```text
engine/
  platform/           # L0  窗口 · 输入 · VSyncWaiter · surface
  scheduler/          # L1  帧调度 · Ticker · 帧计时
  ui/rendering/       # L1  RenderObject · PipelineOwner · compositing
  ui/painting/        # L1  PaintingContext
  ui/animation/       # L1 最小 · L2 完备
  ui/gestures/        # L2  手势竞技
  ui/focus/           # L2  焦点
  scene/layer/        # L1  Layer · COW · overlay band
  scene/picture/      # L1  DisplayList
  raster/backend/     # L0/L1 GPU
  raster/compositor/  # L1  合成 present
  raster/text|image/  # L1  加深
  embedder/           # 粘合
  test/

kit/                  # L3  后置 · 依赖 engine 公开 API
app/                  # L4  业务
```

```text
embedder → scheduler, ui, raster, platform
ui(rendering) → scene
raster → scene + backend
kit → engine（公开面）
kit ↛ backend/gpu
```

---

# §9 风险 · 下一步 · 修订

## 9.1 风险

| 风险 | 缓解 |
|------|------|
| 每帧全树打包 | F01 强制 COW+变更集 |
| 过早堆控件 | §1.6 / §5.1 门禁 |
| 滚动当控件才做 | §4.5 算 L1 |
| 手势糊进各控件 | Phase 5 做 L2 |
| GPU 过载 | 背压 + 静态层复用 + 预算 |
| Snapshot/Picture 分配 | arena、脏层才录 |

## 9.2 立即下一步

1. 以本文 v2.0 为真源（四层模型为准）  
2. 建 `engine/` 空模块 + 依赖边界检查  
3. 开工 **Phase 0**（VSyncWaiter + 管道 + 计时 + 清色窗）  
4. **§5.1 未全勾前禁止 L3 控件主线**

## 9.3 修订

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.x | 2026-07-26 | 初版方案、F 清单、帧落后对齐 Flutter |
| **2.0** | **2026-07-26** | **重写：四层模型（L0–L4）为总纲；第一目标=L1；L2/L3/L4 边界写死；分期与 F 项挂到层级** |
