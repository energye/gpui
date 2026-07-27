# UI 渲染基座能力分册（施工真源）

> **版本：2.1-split** · 2026-07-28  
> 自 [`ENGINE_UI_RENDER_BASE.md`](../ENGINE_UI_RENDER_BASE.md) **按依赖组完整拆分**（母表行保留，含 D）。  
> **实现请只跟本目录**；单文件母表 = 归档全文检索。

## 怎么用

1. [00_meta](./00_meta.md) — 状态码、禁止宣称、指标并行  
2. [91_wave_order](./91_wave_order.md) — **施工顺序 + §→分册覆盖表**  
3. 打开对应分册：能力表 → 代码 → 指标 → DoD  
4. 指标定义：[90_metrics](./90_metrics.md)  
5. 改状态：分册 → 本总表一行  

## 分册总表

| 序 | 分册 | 组级状态 | Wave | 窗测/证据 |
|----|------|----------|------|-----------|
| 00 | [元信息](./00_meta.md) | 纪律 | — | §0 §1 |
| 01 | [Pipeline/脏区](./01_pipeline_dirty.md) | A/C | —/P1 | §13 §18 · S2/S4 |
| 02 | [PaintContext](./02_paint_context.md) | A ✅ | — | §11 §17 · 无 painting |
| 03 | [几何+Paint/Shader](./03_draw_geometry.md) | A/B | P0✅ | §2 §3 · geometry |
| 04 | [Path/Clip/saveLayer](./04_path_clip_savelayer.md) | A/B/C · ClipRRect✅ | P0–P2 | §4 §5 · cliplayer/geometry |
| 05 | [Transform](./05_transform.md) | A/C ✅部分 | P1 | §6 · TransformLayer |
| 06 | [图像](./06_image.md) | A/B | —/P1 | §7 · image 轴 |
| 07 | [文本/Font](./07_text.md) | C/A · ellipsis✅ | P0–P2 | §8 · 字体+maxLines/ellipsis |
| 08 | [Layer/Picture](./08_scene_picture.md) | A/C/D | —/P6 | §12 §9 |
| 09 | [调度/Vsync](./09_scheduler_vsync.md) | A/C | —/P1 | §14 |
| 10 | [滚动](./10_scroll_viewport.md) | A/D | —/P2 | §15 |
| 11 | [坐标/Hit](./11_coords_hittest.md) | A | — | §16 |
| 12 | [滤镜/阴影](./12_filter_shadow.md) | B/D | P1–P2 | §10 |
| 90 | [指标](./90_metrics.md) | 并行 | P0→P6 | §20 §21 · PerfSoak |
| 91 | [实施顺序](./91_wave_order.md) | — | — | §22–24 + 覆盖表 |
| 92 | [F01–F18](./92_f_crosswalk.md) | — | — | §19 |

## 已落地工作清单（须写入分册，已核对）

| 工作 | 分册 |
|------|------|
| 删 `ui/painting`；PaintContext；`pc.DC` | 00 · 02 |
| P0/P1 几何 + Geometry 窗测 | 03 · 91 |
| 默认**一副**系统 UI 字体；`FontResolver`/`SetDefaultFontPath`；**无 env**；`LoadMultiFace` | 07 |
| Text 多语示例 | 07 · `ui_render_base_text` |
| **Text maxLines + ellipsis** | 07 · `RenderText` / text_measure_test |
| Image 轴 async + Draw 变体 | 06 |
| TransformLayer + RenderTransform + 测 + cliplayer | 05 · 08 |
| ClipLayer 轴 Boundary/Clip/Opacity/Transform | 04 · 05 · 91 |
| **ClipRRect** UI `PushClipRRect` + `scene.ClipRRectLayer` | 04 · 02 · 08 |
| PerfSoak 轴 | 90 · 91 |
| 指标并行纪律 | 00 · 90 |
| 本拆分 2.1（全文母表入分册 + 覆盖表） | README · 91 |

## 窗测

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_render_base_geometry
go run ./examples/ui_render_base_text
go run ./examples/ui_render_base_image
go run ./examples/ui_render_base_cliplayer
go run ./examples/ui_render_base_perfsoak
go run ./examples/ui_l1_scroll
```

## 状态同步

- **A** = 证据路径 + 单测或窗测  
- 仅 render → 最高 **B**  
- 禁止分册宣称 P6 Present  
- 与归档母表冲突时：**以分册 + 代码为准**，再回写归档  

## 修订

| 版本 | 日期 | 说明 |
|------|------|------|
| 2.1-split | 2026-07-28 | 全文母表入分册；纠偏 Transform/painting；覆盖映射；已落地清单 |
| 2.0-split | 2026-07-28 | 首拆 |
