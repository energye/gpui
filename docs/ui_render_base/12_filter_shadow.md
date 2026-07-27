# 12 · 阴影 / 滤镜 / Blend / Mask

> **状态（组级）：** B/D 为主 · **Wave：** P1–P2 · 修订：2026-07-28  
> 拆自 `ENGINE_UI_RENDER_BASE`；状态码见 [00_meta](./00_meta.md)  
> **施工真源分册** — 改状态先改本文件，再同步 [README](./README.md)

## 1. 目标 / 非目标

**目标：** 盘点滤镜/阴影能力；按需接到 UI；F16 预算。  
**非目标：** 未接场景层前宣称毛玻璃产品完成。  
**组内顺序：** path shadow 近似 → blend/saveLayer → blur → backdrop/filter **Layer 类型**。

## 2. 依赖（交叉关联）

| 方向 | 分册 / 说明 |
|------|-------------|
| **前置** | [04_path_clip_savelayer](./04_path_clip_savelayer.md) · [08_scene_picture](./08_scene_picture.md) |
| **同级可并行** | 见 [91_wave_order](./91_wave_order.md) |
| **解锁** | Material 海拔 · 导航毛玻璃 |

## 3. 能力表（母表全文）

## §10 阴影 / 滤镜 / Blend / Mask 母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FF-SHADOW-PATH | 路径阴影 | Canvas.drawShadow | 卡片海拔 | — | ApplyDropShadow 近似 | — | B | filter_ops.go | — | P1 |
| FF-BLUR | 高斯模糊 | ImageFilter.blur | 毛玻璃 | — | ApplyBlur/ApplyBlurXY | 无 Backdrop Layer 场景 | B | filter_ops.go | — | P2 |
| FF-COLOR-MATRIX | 颜色矩阵 | ColorFilter.matrix | 置灰 | — | ApplyColorMatrix | — | B | filter_ops.go | — | P2 |
| FF-BLEND-LAYER | 混合合成 | saveLayer+blend | 叠色 | — | SetBlendMode/PushLayer | — | B | context_layer.go | — | P1 |
| FF-MASK | 遮罩层 | ShaderMask/saveLayer mask | 渐变淡出 | — | PushMaskLayer | — | B | context_layer.go | — | P2 |
| FF-BACKDROP | 背景滤镜 | BackdropFilter | 毛玻璃导航 | — | PushBackdropLayer | 无 FL-BACKDROP 场景层 | B/D | m4_extensions.go | — | P2 |
| FF-BUDGET | saveLayer 预算 | 工程纪律 | 防滥用 | SaveLayerBudget | — | — | C | paint_context.go · viewport | F16 | P1 |

---



## 4. 代码落点

| 层 | 路径 |
|----|------|
| render | `filter_ops.go` · layer blend/mask/backdrop |
| UI | `SaveLayerBudget`；无完整 Filter RO/Layer |

## 5. 指标挂钩

优先 M-GPU-FALLBACK、layer pool、RSS；避免主路径全幅模糊。

完整定义 → [90_metrics](./90_metrics.md)。**无 baseline 的优化无效**（[00_meta](./00_meta.md)）。

## 6. 验收 DoD

- [ ] render filter 单测清单核对  
- [ ] UI 场景层仍 D 时保持状态诚实  
- [ ] 与 P6 层 RT 边界写清

## 7. 风险与非宣称

Backdrop/Filter **场景层**多为 D。不是 P6 Present，但是重负载。

## 8. 相关分册

- [总集 README](./README.md) · [实施顺序 91](./91_wave_order.md) · [指标 90](./90_metrics.md) · [F 交叉 92](./92_f_crosswalk.md)
