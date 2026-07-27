# 91 · 实施顺序 · Wave · 窗测 · 缺口 · 覆盖映射

> **按依赖序推进**；指标始终并行。修订：2026-07-28

## 1. 分册施工顺序（打勾用）

| 序 | 分册 | 组状态 | 下一动作 |
|----|------|--------|----------|
| 0 | [00_meta](./00_meta.md) | 纪律 | 遵守 |
| 1 | [01_pipeline_dirty](./01_pipeline_dirty.md) | A/C | 加深 compositing |
| 2 | [02_paint_context](./02_paint_context.md) | A ✅ | 维持无 painting |
| 3 | [11_coords_hittest](./11_coords_hittest.md) | A | 逆 CTM hit 后置 |
| 4 | [09_scheduler_vsync](./09_scheduler_vsync.md) | A/C | **真 VSync** |
| 5 | [03_draw_geometry](./03_draw_geometry.md) | A/B ✅窗测 | P2 vertices |
| 6 | [04_path_clip_savelayer](./04_path_clip_savelayer.md) | A/B/C | ClipRRect ✅ · metrics/nest 文档 |
| 7 | [08_scene_picture](./08_scene_picture.md) | A/C/D | P6 picture/damage |
| 8 | [05_transform](./05_transform.md) | A/C ✅部分 | 逆 hit · 真合成 |
| 9 | [06_image](./06_image.md) | A/B · Dispose✅ | Atlas/JSON 解码字段 |
| 10 | [07_text](./07_text.md) | C/A · ellipsis✅ | Paragraph · BiDi |
| 11 | [10_scroll_viewport](./10_scroll_viewport.md) | A/D | 可变高 · base 轴 |
| 12 | [12_filter_shadow](./12_filter_shadow.md) | B/D | 按需 |
| ∞ | [90_metrics](./90_metrics.md) | 并行 | 每 PR |
| P6 | Picture/damage/HUD | ⬜ | 有数据再拆 |

## 2. 母表 § → 分册（覆盖必须完整）

| 原 § | 内容 | 分册 |
|------|------|------|
| §0 | 元信息/并行指标 | [00](./00_meta.md) |
| §1 | 架构对照 | [00](./00_meta.md) |
| §2 | Canvas | [03](./03_draw_geometry.md)（SCALE/ROTATE 亦见 [05](./05_transform.md)） |
| §3 | Paint/Shader | [03](./03_draw_geometry.md) |
| §4 | Path | [04](./04_path_clip_savelayer.md) |
| §5 | Clip/saveLayer | [04](./04_path_clip_savelayer.md) |
| §6 | Transform | [05](./05_transform.md) |
| §7 | Image | [06](./06_image.md) |
| §8 | Text | [07](./07_text.md) |
| §9 | Picture | [08](./08_scene_picture.md) |
| §10 | Filter/Shadow | [12](./12_filter_shadow.md) |
| §11 | PaintingContext | [02](./02_paint_context.md) |
| §12 | Layer 类型 | [08](./08_scene_picture.md) |
| §13 | RO/Pipeline | [01](./01_pipeline_dirty.md) |
| §14 | Scheduler | [09](./09_scheduler_vsync.md) |
| §15 | Scroll | [10](./10_scroll_viewport.md) |
| §16 | Coord/Hit | [11](./11_coords_hittest.md) |
| §17 | 绘制入口现状 | [02](./02_paint_context.md) |
| §18 | RO/scene 现状 | [01](./01_pipeline_dirty.md)（§18 全文）+ [08](./08_scene_picture.md) |
| §19 | F01–F18 | [92](./92_f_crosswalk.md) |
| §20–§21 | 指标/门禁 | [90](./90_metrics.md) |
| §22–§24 | 缺口/Wave/窗测 | 本文件 |
| §25–§26 | 完备性/历史 | [README](./README.md) · 归档母表 |

## 3. Wave 摘要

| Wave | 内容 | 2026-07-28 |
|------|------|------------|
| P0 | 几何/指标/Geometry | ✅ |
| P1 | wrap/path/layer/**TransformLayer**/Image·Text 轴 · **ellipsis/maxLines** | 主路径 ✅；VSync/Paragraph 后置 |
| P2 | path metrics、可变高、Filter 层 | ⬜ |
| P3 | soak 体系/baseline 库 | 窗测有 · 库未自动化 |
| P6 | damage Present · Picture 录制 · HUD | ⬜ |
| P7/远期 | Ant · PlatformView · IME | ⬜ |

## 4. 缺口注册表

| 优先级 | 缺口 | Wave |
|--------|------|------|
| P1 | exhost 真 VSync | P1 |
| P1 | ParagraphBuilder（富文本） | P1 |
| P1 | ~~ellipsis / maxLines~~ ✅ | — |
| P1 | ~~ClipRRect UI/层~~ ✅ | — |
| P1 | ~~Image dispose 规范~~ ✅ | — |
| P2 | path metrics · 可变行高 · Backdrop Layer | P2 |
| P6 | Picture 显示列表 · dirty Present · HUD | P6 |
| 远期 | PlatformView · FragmentShader · IME | 远期 |

## 5. 窗测轴

| 轴 | 示例 | 状态 |
|----|------|------|
| Geometry | `ui_render_base_geometry` | ✅ |
| Text | `ui_render_base_text` | ✅ |
| Image | `ui_render_base_image` | ✅ |
| ClipLayer | `ui_render_base_cliplayer` | ✅ |
| PerfSoak | `ui_render_base_perfsoak` | ✅ |
| Scroll | `ui_l1_scroll` | 部分（可升 base 轴） |

**每轴 = 能力 + 指标 JSON。**

## 6. 依赖简图

```text
00_meta
 ├─ 01_pipeline ─ 02_paint ─┬─ 03_draw ─ 04_path_clip ─ 12_filter
 │                          ├─ 06_image
 │                          └─ 07_text
 │            └─ 08_scene ─ 05_transform
 │            └─ 10_scroll
 ├─ 09_scheduler ∥ 90_metrics（贯穿）
 └─ 11_coords
      P6 ← 08 picture/damage
```
