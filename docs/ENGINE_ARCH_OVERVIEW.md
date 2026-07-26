# UI 架构总览图 — 四层模型 × Flutter 管线

> **版本：2.0** | 日期：2026-07-26  
> **状态：已批准 · 图示总览**  
> **文字真源（条款与门禁）：** [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md)  
> 本文用图回答三件事：**分几层、第一目标是谁、一帧怎么跑。**

---

## 0. 三十秒看懂

```text
第一目标：L1 UI 引擎  =  Flutter 的 Engine + rendering（管线丝滑）
其后：    L2 框架壳    =  手势/焦点/Overlay/滚动视口（控件底座，不是 Ant 控件）
再后：    L3 产品控件  =  Ant Kit（docs/antd）
最后：    L4 用户业务  =  页面组合控件

帧成本 ∝ 脏区，不是控件个数
静止 ≈ 0% CPU · 输入不堵在 GPU 上
```

| 问题 | 答案 |
|------|------|
| 控件依赖谁？ | **L2 + L1**（禁止直接 GPU） |
| 用户功能靠什么？ | **主要组合 L3 控件** |
| 滚动算控件吗？ | **协议在 L1**；List/Table 产品在 L3 |
| 何时开 Ant 控件？ | **L1 雏形勾选全过之后**（真源 §5.1） |

---

## 1. 四层总图（最重要）

```text
┌─────────────────────────────────────────────────────────────────────────┐
│ L4  用户 / 业务                                              【后置】    │
│     页面 · 流程 · 业务状态 · 组合控件完成功能                             │
├─────────────────────────────────────────────────────────────────────────┤
│ L3  产品控件（Ant Kit）                                      【后置】    │
│     Button / Table / Modal… · Token/Skin · docs/antd                    │
├─────────────────────────────────────────────────────────────────────────┤
│ L2  框架基础设施                                              【控件前】  │
│     手势竞技 · 焦点 · Overlay 挂载壳 · 滚动视口 · 动画 API                │
│     （不是某个 Ant 控件，但是所有复杂控件的底座）                           │
├─────────────────────────────────────────────────────────────────────────┤
│ L1  UI 引擎  ★ 当前唯一主线                                   【第一目标】│
│     vsync/管道 · RO/Layout/Paint · Layer/Picture · 合成 · 脏区          │
│     文本/图基础 · dpr · 帧计时 · 滚动协议 · 最小 Ticker                  │
├─────────────────────────────────────────────────────────────────────────┤
│ L0  平台 + GPU 后端（仅引擎内部）                                         │
│     窗口/输入/VSyncWaiter/Surface · Device 仅 Raster 线程                │
└─────────────────────────────────────────────────────────────────────────┘

        L4 → L3 → L2 → L1 → L0     （只允许向下依赖）
        L3/L4 禁止直触 GPU
        L1 禁止依赖 L3
```

### 1.1 和 Flutter 怎么对应

```text
  Flutter                              本架构
  ─────────────────────────────        ────────────
  Engine + rendering + scheduler  →    L1 + L0
  gestures / Focus / Overlay 机制 →    L2
  Material 组件                   →    L3（我们用 Ant，不是 Material）
  业务 App                        →    L4
```

### 1.2 各层「有 / 没有」

| 层 | 有 | 没有 |
|----|----|------|
| **L1** | 树、布局、绘制录制、分层、栅格、Present、脏区、基础字/图、滚动**协议** | Ant Button、业务页 |
| **L2** | 手势、焦点、Overlay**壳**、动画 API、视口机制 | 带皮肤的 Modal 产品 API |
| **L3** | 具名产品控件、设计 Token | GPU 句柄 |
| **L4** | 业务组合 | 自建渲染管线 |

---

## 2. L1 内部：双线程（引擎心脏）

```text
     ┌──────────────┐
     │ VSyncWaiter  │ 真显示信号（fallback 仅兜底）
     │ Scheduler    │ IDLE / 按需 / 管道背压 / 帧计时
     └──────┬───────┘
            │
            ▼
╔════════════════ UI THREAD ════════════════╗
║  输入 · Layout · Paint→Picture · HitTest  ║
║  可变 RenderObject 树                     ║
║  Submit(FramePacket)                      ║
║  ✗ 禁止 GPU Submit                        ║
╚═══════════════════╤═══════════════════════╝
                    │ COW + mutations
                    │ 有限管道 · 背压 · pending 可覆盖
                    ▼
╔══════════════ RASTER THREAD ══════════════╗
║  脏 Layer → RT · 静态层复用纹理            ║
║  blit 合成 → Present                      ║
║  ✗ 禁止读可变 RO 树                       ║
╚═══════════════════════════════════════════╝

  （加深）IO Thread：解码图片，不进 UI 热路径
```

**掉帧（对齐 Flutter，不是无界 coalesce）：**

```text
UI 忙        → miss vsync
pending 未画 → 新帧可覆盖 pending
管道满       → 背压，禁止无限排队
```

---

## 3. L1 内部：一帧流水线

```text
  vsync
    → Ticker
    → layout(仅脏，止于 RelayoutBoundary)
    → paint(仅脏，止于 RepaintBoundary) → Picture
    → Submit(FramePacket)
    → Raster 仅 dirty layers
    → blit Present
    → FrameDone（回收管道槽）
```

**成本：**

```text
∝ 脏 Layout + 脏 Paint + 脏 Layer 像素 + 合成涉及层
≠ 整棵控件树
```

---

## 4. L1 内部：脏区直觉

```text
  Root
  └─ Page（干净）
     ├─ Header（干净）—— 不 paint、不 raster
     └─ RepaintBoundary · Spinner（脏）
            只 paint 这里 · 只 raster 这一层 RT
```

| 标记 | 停在哪 |
|------|--------|
| markNeedsLayout | RelayoutBoundary |
| markNeedsPaint | RepaintBoundary |
| 仅 opacity/transform | compositor_dirty（可不重录 Picture） |

---

## 5. 滚动：协议在 L1，列表产品在 L3

```text
  L1 引擎协议                     L3 产品控件（后置）
  ─────────────────               ────────────────
  OffsetLayer 上的 offset    ←──  kit.Table / List 去改 offset
  虚拟化挂载可见 RO           ←──  控件提供 row builder
  拖拽最新点跟手             ←──  控件不自写 16ms 刷全树
```

**禁止：** 把「会不会滚」留到写 Table 控件时才设计协议。

---

## 6. 合成：离屏矢量 vs 屏上 blit

```text
  脏 Layer RT          允许：path / text / image / clip
        │
        ▼ 纹理
  Present 帧           只允许：texture blit · 简单透明度
                       禁止：默认全屏矢量重画
```

---

## 7. 包目录 ↔ 层级

```text
engine/
  platform/              L0
  scheduler/             L1
  ui/rendering|painting  L1
  ui/animation           L1 最小 → L2 完备
  ui/gestures|focus      L2
  scene/                 L1
  raster/                L1 + L0 GPU
  embedder/              粘合

kit/                     L3（后置）
app/                     L4（后置）
```

```text
kit ──► engine 公开 API
kit ──X──► gpu / raster backend
```

---

## 8. 分期：先引擎，后壳，最后控件

```text
  P0─P3   L1  UI 引擎  →  Flutter 体验雏形（真源 §5.1 全勾）
    │
  P4      L1  加深（滚动门禁 · 文本/图 IO）
    │
  P5      L2  框架壳（手势 · 焦点 · Overlay · 动画完备）
    │
  P6      L1  增强（细 damage · 多窗 · HUD）
    │
  P7      L3  Ant 控件
    │
  之后    L4  业务
```

| 阶段 | 你在做哪一层 | 完成标志 |
|------|--------------|----------|
| P0–P3 | **L1 第一目标** | 真源 §5.1 勾选表 |
| P4 | L1 加深 | S5/S6 滚动门禁 |
| P5 | **L2** | 手势/焦点/Overlay 可测 |
| P6 | L1 增强 | 工具与多窗 |
| P7 | **L3** | antd 控件迁入 |

---

## 9. F 契约挂在哪一层（摘要）

| ID | 一句话 | 层 |
|----|--------|-----|
| F01–F04, F06–F08, F10–F11, F14–F18 | 管线/脏区/GPU/vsync/计时 | **L1** |
| F05 | 滚动协议 | **L1** |
| F09, F13, 手势/焦点 | 动画完备、Overlay 机制 | **L1 预留 / L2 完备** |
| F12 | IO 解码 | **L1** |
| Ant 控件 | Button… | **L3** |

全文表：真源 **§5**。

---

## 10. 单 Spinner（引擎层示例）

```text
  Ticker → markNeedsPaint → 停在 RepaintBoundary
  Layout = 0
  Paint 仅 Spinner 子树 → Picture
  Raster 仅 1 个 RT
  其余层纹理复用 + blit
  → CPU 低，整页不重布局
```

这是 **L1** 能力；Spinner **产品外观** 属于以后的 L3。

---

## 11. 修订

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.x | 2026-07-26 | 线程/流水线/F 摘要 |
| **2.0** | **2026-07-26** | **以四层模型为第一章；明确第一目标=L1；滚动协议≠控件；分期挂层级** |
