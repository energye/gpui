# docs/ — 文档索引

> 日期：2026-07-26  
> **仓库：** `ui/`（L1 待建）· `render/`（Skia 式）· `gpu/`（wgpu）  
> **依赖：** `ui → render → gpu`（禁止跨层）

---

## 阅读顺序

| 顺序 | 文档 | 内容 |
|------|------|------|
| 1 | [`ENGINE_ARCH_OVERVIEW.md`](./ENGINE_ARCH_OVERVIEW.md) | 总览图 · 逻辑/物理/Y 白话 · 四层 |
| 2 | [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md) | **架构真源** v3 |
| 3 | [`ENGINE_PHASE_P0_P3.md`](./ENGINE_PHASE_P0_P3.md) | **P0–P3 详细任务计划** |
| 4 | [`ENGINE_PHASE_P4_P7_OUTLINE.md`](./ENGINE_PHASE_P4_P7_OUTLINE.md) | **P4–P7 大纲** + **L1 原生手感如何验收** |
| — | [`antd/`](./antd/) | L3 控件需求（后置） |

---

## 关键约定（摘要）

| 项 | 约定 |
|----|------|
| 第一目标 | L1 UI 引擎（Flutter 管线丝滑） |
| 包 | `github.com/energye/gpui/ui` |
| 光栅 | `render`（不够就改 render） |
| GPU | `gpu` + wgpu-native（不够就改 gpu）；**ui 不 import gpu** |
| 窗口 | 宿主建窗，注入 `NativeSurface` 句柄 |
| 坐标 | 布局/点击=逻辑像素 Y-down；纹理=物理像素 × dpr |
| 示例 | 新建 `examples/ui_l1_*` |

---

## 维护

1. 架构变更 → 真源 + 总览  
2. 阶段任务 → `ENGINE_PHASE_P0_P3.md`（P4+ 另文）  
3. 控件需求 → `antd/`  
4. L1 §7.1 未勾完 → 不开 L3 控件主线  
