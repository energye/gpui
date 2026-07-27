# 10 · 滚动 / 视口 / 虚拟化

> **状态（组级）：** A（固定行高）/ D（可变高·Physics） · **Wave：** — / P2 · 修订：2026-07-28  
> 拆自 `ENGINE_UI_RENDER_BASE`；状态码见 [00_meta](./00_meta.md)  
> **施工真源分册** — 改状态先改本文件，再同步 [README](./README.md)

## 1. 目标 / 非目标

**目标：** 视口裁剪+滚动偏移 paint-only；固定行高 VirtualList；嵌套滚动协议。  
**非目标：** 可变行高 sliver；弹簧 Physics（P2）。  
**已落地：** Viewport/VirtualList/Scrollable；S5/S6 测；`ui_l1_scroll`；nested_scroll 测。

## 2. 依赖（交叉关联）

| 方向 | 分册 / 说明 |
|------|-------------|
| **前置** | [01_pipeline_dirty](./01_pipeline_dirty.md) · [04_path_clip_savelayer](./04_path_clip_savelayer.md) |
| **同级可并行** | 见 [91_wave_order](./91_wave_order.md) |
| **解锁** | 长列表产品 · 可选 `ui_render_base_scroll` 轴 |

## 3. 能力表（母表全文）

## §15 滚动 / 视口 / 虚拟化母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FScroll-OFFSET | 滚动偏移 | Viewport offset | 列表 | SetScrollOffset/ScrollBy | — | Viewport+Offset | A | viewport.go | 默认只 MarkNeedsPaint | — |
| FScroll-CLIP | 视口裁剪 | clip canvas | 离屏不画 | PushClip 于 Viewport.Paint | — | — | A | viewport.go | — | — |
| FScroll-VIRTUAL | 虚拟化 | SliverChild | 长列表 | VirtualList 固定行高 | — | — | A | virtual_list.go | 可变高 D | P2 |
| FScroll-CACHE | cacheExtent | cacheExtent | 预创建窗外 | CacheExtent | — | VirtualList | A | virtual_list.go | — | — |
| FScroll-NEST | 嵌套滚动 | NestedScroll/竞技 | 父子列表 | Scrollable.Parent 移交 | — | gestures+scrollable | A | nested_scroll_test.go | P5 | — |
| FScroll-VAR-EXTENT | 可变行高 | Sliver 可变 | 聊天列表 | — | — | — | D | — | — | P2 |
| FScroll-PHYSICS | 物理回弹 | ScrollPhysics | iOS/Android 感 | — | — | — | D | — | 产品感后置 | P2 |

---



## 4. 代码落点

| 层 | 路径 |
|----|------|
| Viewport / VirtualList / Scrollable | `ui/rendering/*.go` |
| 示例 | `examples/ui_l1_scroll` |
| 测 | nested_scroll · virtual · S5/S6 |

## 5. 指标挂钩

| 指标 | 通过方向 |
|------|----------|
| M-BIND | ≪ itemCount |
| M-LAYOUT-COUNT | 拖拽无 layout 风暴 |
| M-FPS / hitch | 滚动跟手 |

完整定义 → [90_metrics](./90_metrics.md)。**无 baseline 的优化无效**（[00_meta](./00_meta.md)）。

## 6. 验收 DoD

- [x] 固定行高虚拟列表  
- [x] 嵌套滚动测  
- [ ] 可变行高（D/P2）  
- [ ] 升格 `ui_render_base_scroll` + JSON 门禁  
- [ ] ScrollPhysics |

## 7. 风险与非宣称

滚动默认应 MarkNeedsPaint 而非 layout（已有路径需保持）。

## 8. 相关分册

- [总集 README](./README.md) · [实施顺序 91](./91_wave_order.md) · [指标 90](./90_metrics.md) · [F 交叉 92](./92_f_crosswalk.md)
