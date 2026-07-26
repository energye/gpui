# UI 架构总览 — 一看就懂

> **版本：3.2** | 日期：2026-07-27  
> **L1 收口（状态 / 命令）：** [`ENGINE_L1_CLOSEOUT.md`](./ENGINE_L1_CLOSEOUT.md)  
> **架构真源：** [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md)  
> **P0–P3 任务：** [`ENGINE_PHASE_P0_P3.md`](./ENGINE_PHASE_P0_P3.md)（已完成）  
> **P4 细卡：** [`ENGINE_PHASE_P4.md`](./ENGINE_PHASE_P4.md)  
> **P4+ 大纲：** [`ENGINE_PHASE_P4_P7_OUTLINE.md`](./ENGINE_PHASE_P4_P7_OUTLINE.md)

---

## 0. 三十秒

```text
依赖：  ui  →  render(Skia 式)  →  gpu(wgpu)
        禁止 ui 直接调 gpu

L1（ui/）P0–P3  ✅  体验雏形可验收
后置：  P4 滚动/IO → L2 框架壳 → L3 Ant → L4 业务

坐标：布局/点击 = 逻辑像素，Y 向下；GPU 纹理 = 物理像素（× dpr）
窗口：宿主创建，把 Display/Window 等句柄注入 Host
```

---

## 1. 模块依赖

```text
┌─────────┐
│   ui    │  Flutter 管线：树 · 帧 · Layer · 调度
└────┬────┘
     │ 只 import render
     ▼
┌─────────┐
│ render  │  Skia 式 2D · Present 封装（PresentTarget）
└────┬────┘
     │ 只 import gpu
     ▼
┌─────────┐
│   gpu   │  wgpu-native · Surface(原生句柄) · Submit
└─────────┘
```

功能不够 → 改 **render** 或 **gpu**，不在 ui 复制 GPU。

---

## 2. 四层

```text
L4 业务          后置
L3 Ant 控件      后置（docs/antd）
L2 手势/焦点壳   后置（P5）
L1 UI 引擎       ✅ P0–P3 已落地（包 ui/）
L0 句柄+gpu      platform + gpu（经 render）
```

---

## 3. 句柄怎么进画面

```text
应用创建 OS 窗口
    → Host.NativeSurface{ Kind, Display, Window }
    → render.NewPresentTarget（内部用 gpu）
    → Raster 线程 Present
ui 永不 import gpu
```

| 平台 | 典型句柄 |
|------|----------|
| Linux X11 | Display* + Window |
| Wayland | wl_display* + wl_surface* |
| Win | HWND |
| macOS | NSView* / CAMetalLayer* 等 |

---

## 4. 逻辑像素 / 物理像素 / Y 轴（白话）

| 名字 | 干什么用 |
|------|----------|
| **逻辑像素** | 布局、点击 |
| **物理像素** | GPU 纹理、swapchain |
| **dpr** | 物理 ≈ 逻辑 × scale |
| **Y-down** | 原点左上，Y 向下（Flutter 同） |

验收：逻辑 (0,0) 画点在窗口左上；点左上命中左上控件。

---

## 5. 一帧（L1）

```text
vsync/fallback → Tick → layout(脏) → paint(脏) → FramePacket
              → Raster（SubmitLatest，不堵 UI）→ render Present
```

掉帧：miss vsync / pending 覆盖 / 管道背压。

---

## 6. 脏区直觉

```text
只有 Spinner 在 RepaintBoundary 里转
→ 只 paint/计脏该层；静态子树 CompositeOnly 跳过
```

---

## 7. 分期状态

| 阶段 | 内容 | 状态 |
|------|------|------|
| P0–P3 | L1 体验雏形 | ✅ |
| P4 | 滚动 / 文本 / IO | ✅ |
| P5–P7 | L2 / 增强 / Ant | ⬜ |

**验收命令与限制 →** [`ENGINE_L1_CLOSEOUT.md`](./ENGINE_L1_CLOSEOUT.md)

---

## 8. 目录

```text
ui/platform|scheduler|raster|embedder|rendering|painting|scene|animation
render/present_target.go
examples/ui_l1_blank | ui_l1_spinner
```

---

## 9. 修订

| 版本 | 说明 |
|------|------|
| 3.0–3.1 | 清仓架构；P0–P3 状态 |
| **3.2** | **L1 文档收尾：总览与收口对齐；L1 标完成** |
