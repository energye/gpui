# S5.5 — 控件层开工入口条件

> 版本：1.2 | 日期：2026-07-21  
> 状态：**S5.5 关闭 / S5 全线关闭** — **允许**开控件层主线  
> 引擎缺口（开工后仍要跟）：[`ENGINE_GAPS.md`](./ENGINE_GAPS.md)

---

## 1. 允许开始「类 Ant Design 控件层」之前必须满足

| # | 条件 | 证据 | 状态 |
|---|------|------|------|
| 1 | Skia 主路径 P0 绘制能力无阻塞缺口 | `S5_SKIA_UI_GAP` / 能力矩阵 | ✅ |
| 2 | Present / damage / 帧模型可用 | `PresentFrame*` · `FlushGPUWithViewDamage` | ✅ |
| 3 | 主路径 GPU 优先、可测 | `GPU_FIRST_ROUTING` · GPUOps 门禁 | ✅ |
| 4 | 回归：能力/组合/复杂 UI 抽样绿 | `TestP1_*` · capability_matrix | ✅ |
| 5 | **控件实现不得另起光栅化**；必须走 `render` | 架构约束 | ✅ |
| 6 | Surface / device 生命周期可恢复 | `SURFACE_LIFECYCLE_*` · device_lost 文档 | ✅ |

---

## 2. 控件层启动后仍禁止

- silent CPU 冒充 GPU  
- 绕过 `PresentFrame*` 自管 swapchain 像素语义却宣称引擎能力  
- 把 R.02 PDF/SVG、真 multiplanar YUV 当作控件阻塞依赖  
- 把 **布局 / HitTest / IME** 当作引擎缺口（见 `ENGINE_GAPS` §3）

---

## 3. 引擎侧开工后仍须跟进（非控件实现）

| 优先级 | 缺口 | 文档 |
|--------|------|------|
| 主 | G1 文本（CFF / shaping / 长文） | `ENGINE_GAPS` |
| 主 | G2 矢量脏区效率 | 同上 |
| 主 | G3 重场景 + lifecycle soak | 同上 · surface · mem 护栏 |

**控件层可开工 ≠ 引擎生产级零缺口。**

---

## 4. 建议的控件层第一批（非 S5 范围）

形态：Button / Input / Modal / List row / Table cell — **仅组件 API**，绘制全走 `Context`。

完整框架（分层 / 插件 / 跨平台 / M0–M6）→ [`UI_FRAMEWORK_MAP.md`](./UI_FRAMEWORK_MAP.md)。  
目标包：`ui/core` + `ui/kit` + `ui/skin/*` + `ui/platform`（**无**默认 `ui/antd`）。

---

## 5. 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | S5.5 关闭 |
| 2026-07-21 | 1.1 | 对齐现网 lifecycle；挂 ENGINE_GAPS |
| 2026-07-21 | 1.2 | 挂合并后的 UI_FRAMEWORK_MAP |
