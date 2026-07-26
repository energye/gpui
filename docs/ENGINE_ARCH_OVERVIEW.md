# UI 架构总览 — 一看就懂

> **版本：3.0** | 日期：2026-07-26  
> **真源：** [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md)  
> **P0–P3 任务：** [`ENGINE_PHASE_P0_P3.md`](./ENGINE_PHASE_P0_P3.md)  
> **P4–P7 大纲 · L1 手感验收：** [`ENGINE_PHASE_P4_P7_OUTLINE.md`](./ENGINE_PHASE_P4_P7_OUTLINE.md)

---

## 0. 三十秒

```text
依赖：  ui  →  render(Skia 式)  →  gpu(wgpu)
        禁止 ui 直接调 gpu

第一目标：L1 = ui/ 里 Flutter 式管线（丝滑调度与脏区）
后置：    L2 框架壳 → L3 Ant 控件 → L4 业务

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
│ render  │  Skia 式 2D · 离屏 · Present 封装
└────┬────┘
     │ 只 import gpu
     ▼
┌─────────┐
│   gpu   │  wgpu-native · Surface(原生句柄) · Submit
└─────────┘
```

功能不够 → 改 **render** 或 **gpu**，不在 ui 复制 GPU。

---

## 2. 四层（谁先做）

```text
L4 业务          后置
L3 Ant 控件      后置（docs/antd）
L2 手势/焦点壳   L1 之后
L1 UI 引擎       ★ 现在（包 ui/）
L0 句柄+gpu      platform + gpu（经 render）
```

---

## 3. 句柄怎么进画面

```text
应用创建 OS 窗口
    → Host.NativeSurface{ Kind, Display, Window }
    → render 建 Surface（内部用 gpu）
    → Raster 线程 Present
ui 永不 import gpu
```

| 平台 | 典型句柄 |
|------|----------|
| Linux X11 | Display* + Window |
| Wayland | wl_display* + wl_surface* |
| Win | HWND |
| macOS | NSView* 等 |

---

## 4. 逻辑像素 / 物理像素 / Y 轴（白话）

### 4.1 两种「长度」

| 名字 | 干什么用 | 类比 |
|------|----------|------|
| **逻辑像素** | 写布局、算点击：按钮宽 100 | 设计稿上的 px |
| **物理像素** | 屏幕真实点、GPU 纹理大小 | 视网膜屏上更密的点 |
| **dpr/scale** | 物理 ≈ 逻辑 × scale | 2x 屏上 1 逻辑 = 2 物理 |

```text
ui 里永远用逻辑数字
到 render 画的时候带上 ScaleFactor
swapchain 按物理分辨率配置
```

### 4.2 Y 轴

| | |
|--|--|
| **Y-down** | Y 越大越靠**下**；原点在**左上**（Flutter / 常见 GUI） |
| **Y-up** | Y 越大越靠**上**（部分 3D/GPU 习惯） |

**本引擎 ui：Y-down。**  
若 GPU 某路径是 Y-up，只在 render/gpu 边界翻一次，不要让 ui 树两套坐标。

### 4.3 怎么验收没搞反

- 在逻辑 (0,0) 画点 → 应在窗口**左上**  
- 点击左上 → 命中左上控件  
- scale=2 时画面清晰，按钮逻辑宽不变、像素更密  

---

## 5. 一帧（L1）

```text
vsync → Tick → layout(脏) → paint(脏) → FramePacket
     → Raster 调 render 画脏层 → Present
```

掉帧：miss vsync / pending 覆盖 / 管道背压（同 Flutter）。

---

## 6. 脏区直觉

```text
整页干净，只有 Spinner 在 Boundary 里转
→ 只 paint/raster 这一块，其余层复用纹理
```

---

## 7. 分期

| 阶段 | 做什么 |
|------|--------|
| P0 | ui 骨架 · Host 句柄 · 调度 · 清色 Present · 计时 |
| P1 | RO · Picture · 坐标/Y 测 |
| P2 | Layer · Boundary · COW |
| P3 | 双线程管道 · 脏层 · Spinner 门禁 → **L1 雏形达标** |

详单：[`ENGINE_PHASE_P0_P3.md`](./ENGINE_PHASE_P0_P3.md)

---

## 8. 目录（重建）

```text
ui/platform|scheduler|rendering|painting|scene|embedder|...
render/     （已有，按需改）
gpu/        （已有，按需改）
examples/ui_l1_blank|ui_l1_spinner  （新建验收）
```

---

## 9. 修订

| 版本 | 说明 |
|------|------|
| 3.0 | 清仓后：ui>render>gpu；句柄；逻辑/物理/Y 白话；任务计划链接 |
