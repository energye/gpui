# P2.6「22 窗金图漂移」逐窗判定（2026-11-12）

> 结论：**21 个漂移窗判为「基线过期」，未发现渲染洞；`ui_render_base_svg` 金图有效（0 差异），不重种。**
> 用户 2026-11-12 确认后已重种 21 窗：native/go 42 轮复跑全 `0.0000%`，窗口级 1px 负向 12.9901% 红→还原绿；webp 逻辑探针同步改为 `FormatRGBAPremul`（19/19 通过）。专项收口。
> 依据：22 窗 × 双后端串行实跑（native webgpu / go gles，隔离 rootless Xwayland :3，标准自退）；每窗都有逻辑探针、像素探针、当前 PNG、基线 PNG、diff/compare 对照；代表窗金图 diff 与 C1 历史值逐字相同（dither 31.7164% / miscdraw 14.2041% / backdrop 6.3519% / shadermask 5.6476% / filterops 5.2101%），其余落在 C1 记录的 0.1880%–1.6722% 区间；native/go 的 golden pct 全部相等，内容区两后端逐字节相同。

## 1 方法与口径

- 采集：`tools/run_windows.sh`，22 窗 × {native, go} = 44 轮，单窗串行，`RUN_SECONDS=5 -auto-only`，窗口自行 `Quit/Close`；每轮前后 `xdpyinfo` 检查 :3，无重启、无硬杀。原始 stdout/stderr/rc 在 `runs/<窗>/<后端>/`。
- 分析：`tools/analyze_golden.py`，逐窗输出 golden 门禁 diff（JSON `pixel_golden_diff_pct`）、内容区 diff 计数/最大通道差/bbox/边沿占比/内部采样点/±2px 平移搜索/通道差直方图；视觉对照 `analysis/<窗>/<后端>/{diff.png,compare.png}`。
- 内容区定义：去掉 wrkit 顶栏 48px 与底部 HUD 72px；HUD 是 live FPS/phase 文本，本来就被各窗 golden mask 排除。
- 内部采样：内容区每 40px 一个采样点（RGBA 逐字节比较）；“identical”计数即逐字节相同的采样点数。
- 边沿占比：差异像素中，邻域 3×3 局部对比度 ≥16 的像素占比；1px AA 边沿漂移会得到高占比，渐变/阴影内部变化会得到低占比。
- 平移搜索：把当前图按 (dx,dy)∈[-2,2] 平移后与基线比较，最优仍为 (0,0)、增益 0 = 没有整体错位。
- 双后端：native 与 go 的内容区当前 PNG 逐字节相同（22/22，`content_diff=0`），差异只在 HUD/顶栏 live 文本；金图 pct 两档逐窗相等。

## 2 汇总表

| 窗 | 基线日期/提交 | golden diff（native/go） | 内容区 diff | 最大 Δ | 内部采样 identical | 平移最优 | 判定 |
|---|---|---|---:|---:|---|---|---|---|
| `ui_render_base_backdrop` | 2026-10-19 `8ebf5698` | 6.3519% / 6.3519% | 50,736 px（6.218%） | 21 | 501/510 | (0,0) | 基线过期 |
| `ui_render_base_cliprrect` | 2026-10-21 `1bc7f803` | 0.4986% / 0.4986% | 8,725 px（1.069%） | 17 | 510/510 | (0,0) | 基线过期 |
| `ui_render_base_dither` | 2026-10-25 `08d59211` | 31.7164% / 31.7164% | 204,509 px（25.062%） | 77 | 399/510 | (0,0) | 基线过期 |
| `ui_render_base_filterops` | 2026-10-20 `ab407ab0` | 5.2101% / 5.2101% | 38,297 px（4.693%） | 64 | 502/510 | (0,0) | 基线过期 |
| `ui_render_base_maskops` | 2026-10-19 `8db1592b` | 2.2262% / 2.2262% | 24,855 px（3.046%） | 11 | 510/510 | (0,0) | 基线过期 |
| `ui_render_base_miscdraw` | 2026-10-25 `08d59211` | 14.2041% / 14.2041% | 117,554 px（14.406%） | 106 | 458/510 | (0,0) | 基线过期 |
| `ui_render_base_pathbool` | 2026-11-02 `b93da030` | 1.8532% / 1.8532% | 22,536 px（2.762%） | 10 | 509/510 | (0,0) | 基线过期 |
| `ui_render_base_svg` | 2026-11-11 `4a41c660` | 0.0000% / 0.0000% | 0 px（0.000%） | 0 | 510/510 | (0,0) | 基线有效 |
| `ui_render_base_textlow` | 2026-10-25 `758d91f9` | 3.4169% / 3.4169% | 32,362 px（3.966%） | 13 | 500/510 | (0,0) | 基线过期 |
| `ui_render_base_textspacing` | 2026-10-25 `6a58f610` | 1.6722% / 1.6722% | 16,106 px（1.974%） | 19 | 489/510 | (0,0) | 基线过期 |
| `ui_render_base_textstroke` | 2026-10-19 `b780dea7` | 1.1950% / 1.1950% | 18,400 px（2.255%） | 22 | 510/510 | (0,0) | 基线过期 |
| `ui_render_base_verticesmesh` | 2026-10-20 `7a40b774` | 1.2079% / 1.2079% | 13,164 px（1.613%） | 8 | 510/510 | (0,0) | 基线过期 |
| `ui_render_base_webp` | 2026-10-20 `4f2f5fcd` | 2.1279% / 2.1279% | 18,932 px（2.320%） | 12 | 504/510 | (0,0) | 基线过期 |
| `ui_wr_shadermask` | 2026-10-27 `9327ff5c` | 5.6476% / 5.6476% | 46,117 px（5.652%） | 6 | 487/510 | (0,0) | 基线过期 |
| `ui_wr_p38_color_font` | 2026-11-10 `06573e5a` | 0.1880% / 0.1880% | 8,554 px（0.751%） | 10 | 740/740 | (0,0) | 基线过期 |
| `ui_wr_p38_picture` | 2026-11-07 `b028ae74` | 0.7465% / 0.7465% | 12,945 px（1.304%） | 11 | 629/629 | (0,0) | 基线过期 |
| `ui_wr_p38_svg_doc` | 2026-11-09 `972cc37c` | 0.1985% / 0.1985% | 7,934 px（0.799%） | 10 | 629/629 | (0,0) | 基线过期 |
| `ui_wr_p38_svg_filter` | 2026-11-09 `972cc37c` | 0.1996% / 0.1996% | 7,947 px（0.800%） | 10 | 629/629 | (0,0) | 基线过期 |
| `ui_wr_p38_svg_paint` | 2026-11-09 `972cc37c` | 0.1986% / 0.1986% | 7,947 px（0.800%） | 10 | 629/629 | (0,0) | 基线过期 |
| `ui_wr_p38_svg_showcase` | 2026-11-09 `d5f9af60` | 0.2801% / 0.2801% | 7,940 px（0.800%） | 9 | 629/629 | (0,0) | 基线过期 |
| `ui_wr_p38_svg_structure` | 2026-11-10 `66d08bd1` | 0.2059% / 0.2059% | 9,160 px（0.713%） | 10 | 814/814 | (0,0) | 基线过期 |
| `ui_wr_p38_svg_text` | 2026-11-09 `972cc37c` | 0.1993% / 0.1993% | 7,958 px（0.802%） | 10 | 629/629 | (0,0) | 基线过期 |

## 3 逐窗证据

### ui_render_base_backdrop

- 基线：`examples/ui_render_base_backdrop/testdata/showcase_backdrop_base.png`（2026-10-19 `8ebf5698`）
- golden：6.3519% over 627,200 px（native 与 go 同值；容差 0）
- 内容区：50,736 / 816,000 px 不同（6.218%），最大通道 Δ=21，bbox=[0, 48, 1200, 728]，边沿占比 44.9%
- 内部采样：501/510 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：(482,290) base=(54, 84, 131, 255) current=(42, 74, 124, 255)；(522,290) base=(40, 111, 101, 255) current=(28, 103, 92, 255)；(842,290) base=(121, 122, 122, 255) current=(121, 121, 122, 255)
- 视觉对照：`analysis/ui_render_base_backdrop/native/compare.png`（上=基线，下=当前）、`analysis/ui_render_base_backdrop/native/diff.png`（红=差异）
- 判定：基线过期。差异沿 backdrop/blur 卡边界与柔和渐变（edge 44.9%，内部 501/510 同），最大 21 在卡边过渡；基线早于 P4.2/P4.3 blur 对齐与 2026-11-11 SDF 覆盖 LUT。窗口 20/20 像素探针通过。

### ui_render_base_cliprrect

- 基线：`examples/ui_render_base_cliprrect/testdata/showcase_cliprrect_base.png`（2026-10-21 `1bc7f803`）
- golden：0.4986% over 627,200 px（native 与 go 同值；容差 0）
- 内容区：8,725 / 816,000 px 不同（1.069%），最大通道 Δ=17，bbox=[0, 48, 1200, 728]，边沿占比 23.0%
- 内部采样：510/510 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：无（全部采样点逐字节相同）
- 视觉对照：`analysis/ui_render_base_cliprrect/native/compare.png`（上=基线，下=当前）、`analysis/ui_render_base_cliprrect/native/diff.png`（红=差异）
- 判定：基线过期。内部 510/510 同，差异仅在圆角/裁剪边沿（edge 23.0%），最大 17；基线早于 2026-11-11 SDF 覆盖 LUT。

### ui_render_base_dither

- 基线：`examples/ui_render_base_dither/testdata/showcase_dither_base.png`（2026-10-25 `08d59211`）
- golden：31.7164% over 627,200 px（native 与 go 同值；容差 0）
- 内容区：204,509 / 816,000 px 不同（25.062%），最大通道 Δ=77，bbox=[0, 48, 1200, 728]，边沿占比 7.5%
- 内部采样：399/510 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：(322,170) base=(59, 65, 78, 255) current=(57, 63, 77, 255)；(362,170) base=(59, 65, 78, 255) current=(57, 63, 77, 255)；(402,170) base=(59, 65, 78, 255) current=(57, 63, 77, 255)
- 视觉对照：`analysis/ui_render_base_dither/native/compare.png`（上=基线，下=当前）、`analysis/ui_render_base_dither/native/diff.png`（红=差异）
- 判定：基线过期。差异集中在扫掠/径向渐变卡：dither 把底层 1 LSB 覆盖精度变化放大成图案翻转；内部 399/510 同，差异点均为 1–3 LSB 邻域；卡 C 当前为连续平滑梯度、基线为旧量化带，最大 77 在渐变内部，不是缺内容。基线早于 SDF LUT。逻辑 16/16、像素 29/29 通过。

### ui_render_base_filterops

- 基线：`examples/ui_render_base_filterops/testdata/showcase_filterops_base.png`（2026-10-20 `ab407ab0`）
- golden：5.2101% over 627,200 px（native 与 go 同值；容差 0）
- 内容区：38,297 / 816,000 px 不同（4.693%），最大通道 Δ=64，bbox=[0, 48, 1200, 728]，边沿占比 47.9%
- 内部采样：502/510 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：(1002,290) base=(42, 43, 37, 255) current=(42, 42, 37, 255)；(1042,290) base=(42, 43, 37, 255) current=(42, 42, 37, 255)；(322,570) base=(210, 67, 32, 255) current=(209, 60, 31, 255)
- 视觉对照：`analysis/ui_render_base_filterops/native/compare.png`（上=基线，下=当前）、`analysis/ui_render_base_filterops/native/diff.png`（红=差异）
- 判定：基线过期。差异沿滤镜卡边缘与 diff 面板分布（diff 面板 ×64 放大 1 LSB；max 64 在 diff 面板），内部 502/510 同；基线早于 P4.2 blur 对齐、P4.3 filter 源清理与 SDF LUT。像素探针全过。

### ui_render_base_maskops

- 基线：`examples/ui_render_base_maskops/testdata/showcase_maskops_base.png`（2026-10-19 `8db1592b`）
- golden：2.2262% over 627,200 px（native 与 go 同值；容差 0）
- 内容区：24,855 / 816,000 px 不同（3.046%），最大通道 Δ=11，bbox=[0, 48, 1200, 728]，边沿占比 32.6%
- 内部采样：510/510 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：无（全部采样点逐字节相同）
- 视觉对照：`analysis/ui_render_base_maskops/native/compare.png`（上=基线，下=当前）、`analysis/ui_render_base_maskops/native/diff.png`（红=差异）
- 判定：基线过期。内部 510/510 同，差异只在 mask 边沿（edge 32.6%），最大 11；基线早于 P4.3 mask 池化（应零行为变化）与 SDF LUT。

### ui_render_base_miscdraw

- 基线：`examples/ui_render_base_miscdraw/testdata/showcase_miscdraw_base.png`（2026-10-25 `08d59211`）
- golden：14.2041% over 737,760 px（native 与 go 同值；容差 0）
- 内容区：117,554 / 816,000 px 不同（14.406%），最大通道 Δ=106，bbox=[0, 48, 1200, 728]，边沿占比 25.7%
- 内部采样：458/510 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：(322,170) base=(201, 204, 209, 255) current=(226, 230, 235, 255)；(362,170) base=(39, 86, 162, 255) current=(59, 130, 245, 255)；(402,170) base=(44, 98, 184, 255) current=(59, 130, 245, 255)
- 视觉对照：`analysis/ui_render_base_miscdraw/native/compare.png`（上=基线，下=当前）、`analysis/ui_render_base_miscdraw/native/diff.png`（红=差异）
- 判定：基线过期。差异集中在 DrawShadow 卡与字形 AA 边沿：投影颜色整体变亮（P4.1 DrawShadow 设备空间/spot 模型变更），内部 458/510 同，最大 106 在投影内部但内容结构完整；基线早于 P4.1/P4.3。逻辑/像素探针全过。

### ui_render_base_pathbool

- 基线：`examples/ui_render_base_pathbool/testdata/showcase_pathbool_base.png`（2026-11-02 `b93da030`）
- golden：1.8532% over 627,200 px（native 与 go 同值；容差 0）
- 内容区：22,536 / 816,000 px 不同（2.762%），最大通道 Δ=10，bbox=[0, 48, 1200, 728]，边沿占比 25.6%
- 内部采样：509/510 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：(442,490) base=(90, 97, 110, 255) current=(88, 95, 109, 255)
- 视觉对照：`analysis/ui_render_base_pathbool/native/compare.png`（上=基线，下=当前）、`analysis/ui_render_base_pathbool/native/diff.png`（红=差异）
- 判定：基线过期。内部 509/510 同，差异沿布尔路径边沿（edge 25.6%），最大 10；基线早于 2026-11-11 SDF LUT。

### ui_render_base_svg

- 基线：`examples/ui_render_base_svg/testdata/showcase_svg_base.png`（2026-11-11 `4a41c660`）
- golden：0.0000% over 627,200 px（native 与 go 同值；容差 0）
- 内容区：0 / 816,000 px 不同（0.000%），最大通道 Δ=0，bbox=None，边沿占比 0.0%
- 内部采样：510/510 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：无（全部采样点逐字节相同）
- 视觉对照：`analysis/ui_render_base_svg/native/compare.png`（上=基线，下=当前）、`analysis/ui_render_base_svg/native/diff.png`（红=差异）
- 判定：基线有效，不重种。0 差异；基线就是 2026-11-11 SDF LUT 批重种的图。对照组：同一引擎下唯一在 SDF LUT 后重种的窗正好 0。基线有效，不重种。

### ui_render_base_textlow

- 基线：`examples/ui_render_base_textlow/testdata/showcase_textlow_base.png`（2026-10-25 `758d91f9`）
- golden：3.4169% over 627,200 px（native 与 go 同值；容差 0）
- 内容区：32,362 / 816,000 px 不同（3.966%），最大通道 Δ=13，bbox=[0, 48, 1200, 728]，边沿占比 48.4%
- 内部采样：500/510 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：(322,570) base=(32, 36, 44, 255) current=(31, 34, 43, 255)；(362,570) base=(32, 36, 44, 255) current=(31, 34, 43, 255)；(402,570) base=(32, 36, 44, 255) current=(31, 34, 43, 255)
- 视觉对照：`analysis/ui_render_base_textlow/native/compare.png`（上=基线，下=当前）、`analysis/ui_render_base_textlow/native/diff.png`（红=差异）
- 判定：基线过期。差异沿字形 AA 边沿，内部 500/510 同，最大 13；基线早于 2026-11-11 SDF LUT。

### ui_render_base_textspacing

- 基线：`examples/ui_render_base_textspacing/testdata/showcase_textspacing_base.png`（2026-10-25 `6a58f610`）
- golden：1.6722% over 627,200 px（native 与 go 同值；容差 0）
- 内容区：16,106 / 816,000 px 不同（1.974%），最大通道 Δ=19，bbox=[0, 48, 1200, 728]，边沿占比 61.8%
- 内部采样：489/510 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：(442,210) base=(41, 38, 35, 255) current=(29, 31, 34, 255)；(882,210) base=(41, 38, 35, 255) current=(29, 31, 34, 255)；(922,210) base=(41, 38, 35, 255) current=(29, 31, 34, 255)
- 视觉对照：`analysis/ui_render_base_textspacing/native/compare.png`（上=基线，下=当前）、`analysis/ui_render_base_textspacing/native/diff.png`（红=差异）
- 判定：基线过期。差异沿字形边沿/字距边界，内部 489/510 同，最大 19；基线早于 2026-11-11 SDF LUT。

### ui_render_base_textstroke

- 基线：`examples/ui_render_base_textstroke/testdata/showcase_textstroke_base.png`（2026-10-19 `b780dea7`）
- golden：1.1950% over 627,200 px（native 与 go 同值；容差 0）
- 内容区：18,400 / 816,000 px 不同（2.255%），最大通道 Δ=22，bbox=[0, 48, 1200, 728]，边沿占比 9.2%
- 内部采样：510/510 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：无（全部采样点逐字节相同）
- 视觉对照：`analysis/ui_render_base_textstroke/native/compare.png`（上=基线，下=当前）、`analysis/ui_render_base_textstroke/native/diff.png`（红=差异）
- 判定：基线过期。内部 510/510 同，差异只在描边边沿（edge 9.2% 因描边带宽），最大 22；基线早于 2026-11-11 SDF LUT。

### ui_render_base_verticesmesh

- 基线：`examples/ui_render_base_verticesmesh/testdata/showcase_verticesmesh_base.png`（2026-10-20 `7a40b774`）
- golden：1.2079% over 627,200 px（native 与 go 同值；容差 0）
- 内容区：13,164 / 816,000 px 不同（1.613%），最大通道 Δ=8，bbox=[0, 48, 1200, 728]，边沿占比 52.8%
- 内部采样：510/510 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：无（全部采样点逐字节相同）
- 视觉对照：`analysis/ui_render_base_verticesmesh/native/compare.png`（上=基线，下=当前）、`analysis/ui_render_base_verticesmesh/native/diff.png`（红=差异）
- 判定：基线过期。内部 510/510 同，差异在网格三角形边沿（edge 52.8%），最大 8；基线早于 2026-11-11 SDF LUT。

### ui_render_base_webp

- 基线：`examples/ui_render_base_webp/testdata/showcase_webp_base.png`（2026-10-20 `4f2f5fcd`）
- golden：2.1279% over 627,200 px（native 与 go 同值；容差 0）
- 内容区：18,932 / 816,000 px 不同（2.320%），最大通道 Δ=12，bbox=[0, 48, 1200, 728]，边沿占比 65.1%
- 内部采样：504/510 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：(362,570) base=(77, 87, 106, 255) current=(80, 90, 110, 255)；(402,570) base=(77, 87, 106, 255) current=(80, 90, 110, 255)；(442,570) base=(77, 87, 106, 255) current=(80, 90, 110, 255)
- 视觉对照：`analysis/ui_render_base_webp/native/compare.png`（上=基线，下=当前）、`analysis/ui_render_base_webp/native/diff.png`（红=差异）
- 判定：基线过期。内部 504/510 同，差异在图片边缘/缩放采样（edge 65.1%），最大 12；基线早于 2026-11-11 SDF LUT。另有逻辑探针 format 期望过期（§6）。

### ui_wr_shadermask

- 基线：`examples/ui_wr_shadermask/testdata/showcase_shadermask_base.png`（2026-10-27 `9327ff5c`）
- golden：5.6476% over 912,000 px（native 与 go 同值；容差 0）
- 内容区：46,117 / 816,000 px 不同（5.652%），最大通道 Δ=6，bbox=[0, 48, 1200, 728]，边沿占比 62.5%
- 内部采样：487/510 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：(82,50) base=(134, 148, 176, 255) current=(134, 149, 176, 255)；(162,50) base=(99, 110, 129, 255) current=(99, 109, 130, 255)；(202,50) base=(130, 144, 171, 255) current=(130, 144, 170, 255)
- 视觉对照：`analysis/ui_wr_shadermask/native/compare.png`（上=基线，下=当前）、`analysis/ui_wr_shadermask/native/diff.png`（红=差异）
- 判定：基线过期。差异广泛但幅度极小（max 6，内部 487/510 同，差异多为 1 LSB），符合 P3.8 premul/alpha 精度变更 + SDF LUT；基线 2026-10-27。

### ui_wr_p38_color_font

- 基线：`examples/ui_wr_p38_color_font/golden/showcase_color_font_base.png（go 用 _go_base）`（2026-11-10 `06573e5a`）
- golden：0.1880% over 1,075,440 px（native 与 go 同值；容差 0）
- 内容区：8,554 / 1,138,800 px 不同（0.751%），最大通道 Δ=10，bbox=[0, 48, 1460, 828]，边沿占比 18.4%
- 内部采样：740/740 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：无（全部采样点逐字节相同）
- 视觉对照：`analysis/ui_wr_p38_color_font/native/compare.png`（上=基线，下=当前）、`analysis/ui_wr_p38_color_font/native/diff.png`（红=差异）
- 判定：基线过期。内部 740/740 同，差异只在字形/卡片边沿（edge 18.4%），最大 10；基线在 P4.4 重种，但早于 2026-11-11 SDF LUT。

### ui_wr_p38_picture

- 基线：`examples/ui_wr_p38_picture/golden/showcase_picture_base.png（go 用 _go_base）`（2026-11-07 `b028ae74`）
- golden：0.7465% over 912,640 px（native 与 go 同值；容差 0）
- 内容区：12,945 / 992,800 px 不同（1.304%），最大通道 Δ=11，bbox=[0, 48, 1460, 728]，边沿占比 37.5%
- 内部采样：629/629 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：无（全部采样点逐字节相同）
- 视觉对照：`analysis/ui_wr_p38_picture/native/compare.png`（上=基线，下=当前）、`analysis/ui_wr_p38_picture/native/diff.png`（红=差异）
- 判定：基线过期。内部 629/629 同，差异在图片/卡片边沿（edge 37.5%），最大 11；基线早于 P3.8 后续与 SDF LUT。

### ui_wr_p38_svg_doc

- 基线：`examples/ui_wr_p38_svg_doc/golden/showcase_svg_doc_base.png`（2026-11-09 `972cc37c`）
- golden：0.1985% over 917,440 px（native 与 go 同值；容差 0）
- 内容区：7,934 / 992,800 px 不同（0.799%），最大通道 Δ=10，bbox=[0, 48, 1460, 728]，边沿占比 18.8%
- 内部采样：629/629 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：无（全部采样点逐字节相同）
- 视觉对照：`analysis/ui_wr_p38_svg_doc/native/compare.png`（上=基线，下=当前）、`analysis/ui_wr_p38_svg_doc/native/diff.png`（红=差异）
- 判定：基线过期。内部 629/629 同，差异在 SVG 图形边沿（edge 18.8%），最大 10；基线早于 2026-11-11 SDF LUT。

### ui_wr_p38_svg_filter

- 基线：`examples/ui_wr_p38_svg_filter/golden/showcase_svg_filter_base.png`（2026-11-09 `972cc37c`）
- golden：0.1996% over 912,640 px（native 与 go 同值；容差 0）
- 内容区：7,947 / 992,800 px 不同（0.800%），最大通道 Δ=10，bbox=[0, 48, 1460, 728]，边沿占比 19.0%
- 内部采样：629/629 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：无（全部采样点逐字节相同）
- 视觉对照：`analysis/ui_wr_p38_svg_filter/native/compare.png`（上=基线，下=当前）、`analysis/ui_wr_p38_svg_filter/native/diff.png`（红=差异）
- 判定：基线过期。内部 629/629 同，差异在 SVG 滤镜/图形边沿（edge 19.0%），最大 10；基线早于 2026-11-11 SDF LUT。

### ui_wr_p38_svg_paint

- 基线：`examples/ui_wr_p38_svg_paint/golden/showcase_svg_doc_base.png`（2026-11-09 `972cc37c`）
- golden：0.1986% over 917,440 px（native 与 go 同值；容差 0）
- 内容区：7,947 / 992,800 px 不同（0.800%），最大通道 Δ=10，bbox=[0, 48, 1460, 728]，边沿占比 19.0%
- 内部采样：629/629 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：无（全部采样点逐字节相同）
- 视觉对照：`analysis/ui_wr_p38_svg_paint/native/compare.png`（上=基线，下=当前）、`analysis/ui_wr_p38_svg_paint/native/diff.png`（红=差异）
- 判定：基线过期。内部 629/629 同，差异在 SVG paint-server 图形边沿（edge 19.0%），最大 10；基线早于 2026-11-11 SDF LUT。

### ui_wr_p38_svg_showcase

- 基线：`examples/ui_wr_p38_svg_showcase/golden/showcase_showcase_base.png`（2026-11-09 `d5f9af60`）
- golden：0.2801% over 647,620 px（native 与 go 同值；容差 0）
- 内容区：7,940 / 992,800 px 不同（0.800%），最大通道 Δ=9，bbox=[0, 48, 1460, 728]，边沿占比 19.0%
- 内部采样：629/629 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：无（全部采样点逐字节相同）
- 视觉对照：`analysis/ui_wr_p38_svg_showcase/native/compare.png`（上=基线，下=当前）、`analysis/ui_wr_p38_svg_showcase/native/diff.png`（红=差异）
- 判定：基线过期。内部 629/629 同，差异在 SVG 图形边沿（edge 19.0%），最大 9；基线在 P3.8 premul 批重种，但早于 SDF LUT。

### ui_wr_p38_svg_structure

- 基线：`examples/ui_wr_p38_svg_structure/golden/showcase_svg_structure_base.png`（2026-11-10 `66d08bd1`）
- golden：0.2059% over 1,076,640 px（native 与 go 同值；容差 0）
- 内容区：9,160 / 1,284,800 px 不同（0.713%），最大通道 Δ=10，bbox=[0, 48, 1460, 928]，边沿占比 17.3%
- 内部采样：814/814 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：无（全部采样点逐字节相同）
- 视觉对照：`analysis/ui_wr_p38_svg_structure/native/compare.png`（上=基线，下=当前）、`analysis/ui_wr_p38_svg_structure/native/diff.png`（红=差异）
- 判定：基线过期。内部 814/814 同，差异在 marker/结构图形边沿（edge 17.3%），最大 10；基线早于 2026-11-11 SDF LUT。

### ui_wr_p38_svg_text

- 基线：`examples/ui_wr_p38_svg_text/golden/showcase_svg_text_base.png`（2026-11-09 `972cc37c`）
- golden：0.1993% over 912,640 px（native 与 go 同值；容差 0）
- 内容区：7,958 / 992,800 px 不同（0.802%），最大通道 Δ=10，bbox=[0, 48, 1460, 728]，边沿占比 19.6%
- 内部采样：629/629 逐字节相同；平移最优 (0,0)，增益 0.00%
- 内部差异样点：无（全部采样点逐字节相同）
- 视觉对照：`analysis/ui_wr_p38_svg_text/native/compare.png`（上=基线，下=当前）、`analysis/ui_wr_p38_svg_text/native/diff.png`（红=差异）
- 判定：基线过期。内部 629/629 同，差异在 SVG 文字/图形边沿（edge 19.6%），最大 10；基线早于 2026-11-11 SDF LUT。

## 4 根因相关性与「渲染洞」排除

1. **同一引擎下唯一 0 差异的窗正好是 SDF LUT 重种窗**：`ui_render_base_svg` 基线提交 `4a41c660`（2026-11-11「SDF 覆盖线实现批…6 扇金图重种」），本批 0.0000%；其余 21 窗基线均早于该提交（最晚 2026-11-10），全部出现 0.1880%–31.7164% 漂移。这个对照把「SDF 覆盖 LUT 改变 AA 边沿几个 LSB」与「基线过期」直接对上。
2. **两后端内容逐字节相同**：22/22 窗 native 与 go 的内容区当前 PNG `content_diff=0`、max Δ=0；golden pct 逐窗相等。排除后端特有洞；也排除“某档跑错”的可能。
3. **无整体错位**：22 窗平移搜索最优全部 (0,0)、增益 0；不是布局/相机/视口平移。
4. **内容结构完整**：逐窗 compare 对照均为同一画面；差异集中在 AA 边沿、1px 过渡、渐变/阴影内部精度、diff 面板放大（filterops ×64、dither ×64）等位置。Dither 卡 C 当前是连续平滑梯度、基线是旧量化带，属于精度改善而非缺失。
5. **像素探针全过**：21 个漂移窗的窗口内像素探针全绿（webp 的 24/24 像素探针也全绿；唯一红的是逻辑探针的格式期望，见 §6），说明当前画面满足各窗写死的颜色/几何/边界断言。
6. **已知变更与基线日期吻合（未逐提交 bisect，按时间窗与失败形状归因）**：`4a41c660` SDF 覆盖 LUT（2026-11-11）、P4.1 DrawShadow/GLES（11-10）、P4.2 blur 对齐（11-10）、P4.3 阴影/滤镜源清理（11-10）、P3.8 premul 修复（`d5f9af60`）、M5 格式参数化（11-11，声明 8bit 行为零变化）。这些提交之后重种的窗（svg 11-11、color_font 11-10、p38 svg 11-09/10）漂移幅度正好落在 0–0.2% 的 SDF 边沿量级；更老的窗（10-19–10-27）叠加 P4.1/P4.2/P4.3，幅度更大。

**结论：21 窗为基线过期，不是渲染洞。** 不建议改引擎；建议按 §7 清单重种基线。

## 5 每窗理由（摘要）

- `ui_render_base_backdrop`：差异沿 backdrop/blur 卡边界与柔和渐变（edge 44.9%，内部 501/510 同），最大 21 在卡边过渡；基线早于 P4.2/P4.3 blur 对齐与 2026-11-11 SDF 覆盖 LUT。窗口 20/20 像素探针通过。
- `ui_render_base_cliprrect`：内部 510/510 同，差异仅在圆角/裁剪边沿（edge 23.0%），最大 17；基线早于 2026-11-11 SDF 覆盖 LUT。
- `ui_render_base_dither`：差异集中在扫掠/径向渐变卡：dither 把底层 1 LSB 覆盖精度变化放大成图案翻转；内部 399/510 同，差异点均为 1–3 LSB 邻域；卡 C 当前为连续平滑梯度、基线为旧量化带，最大 77 在渐变内部，不是缺内容。基线早于 SDF LUT。逻辑 16/16、像素 29/29 通过。
- `ui_render_base_filterops`：差异沿滤镜卡边缘与 diff 面板分布（diff 面板 ×64 放大 1 LSB；max 64 在 diff 面板），内部 502/510 同；基线早于 P4.2 blur 对齐、P4.3 filter 源清理与 SDF LUT。像素探针全过。
- `ui_render_base_maskops`：内部 510/510 同，差异只在 mask 边沿（edge 32.6%），最大 11；基线早于 P4.3 mask 池化（应零行为变化）与 SDF LUT。
- `ui_render_base_miscdraw`：差异集中在 DrawShadow 卡与字形 AA 边沿：投影颜色整体变亮（P4.1 DrawShadow 设备空间/spot 模型变更），内部 458/510 同，最大 106 在投影内部但内容结构完整；基线早于 P4.1/P4.3。逻辑/像素探针全过。
- `ui_render_base_pathbool`：内部 509/510 同，差异沿布尔路径边沿（edge 25.6%），最大 10；基线早于 2026-11-11 SDF LUT。
- `ui_render_base_svg`：0 差异；基线就是 2026-11-11 SDF LUT 批重种的图。对照组：同一引擎下唯一在 SDF LUT 后重种的窗正好 0。基线有效，不重种。
- `ui_render_base_textlow`：差异沿字形 AA 边沿，内部 500/510 同，最大 13；基线早于 2026-11-11 SDF LUT。
- `ui_render_base_textspacing`：差异沿字形边沿/字距边界，内部 489/510 同，最大 19；基线早于 2026-11-11 SDF LUT。
- `ui_render_base_textstroke`：内部 510/510 同，差异只在描边边沿（edge 9.2% 因描边带宽），最大 22；基线早于 2026-11-11 SDF LUT。
- `ui_render_base_verticesmesh`：内部 510/510 同，差异在网格三角形边沿（edge 52.8%），最大 8；基线早于 2026-11-11 SDF LUT。
- `ui_render_base_webp`：内部 504/510 同，差异在图片边缘/缩放采样（edge 65.1%），最大 12；基线早于 2026-11-11 SDF LUT。另有逻辑探针 format 期望过期（§6）。
- `ui_wr_shadermask`：差异广泛但幅度极小（max 6，内部 487/510 同，差异多为 1 LSB），符合 P3.8 premul/alpha 精度变更 + SDF LUT；基线 2026-10-27。
- `ui_wr_p38_color_font`：内部 740/740 同，差异只在字形/卡片边沿（edge 18.4%），最大 10；基线在 P4.4 重种，但早于 2026-11-11 SDF LUT。
- `ui_wr_p38_picture`：内部 629/629 同，差异在图片/卡片边沿（edge 37.5%），最大 11；基线早于 P3.8 后续与 SDF LUT。
- `ui_wr_p38_svg_doc`：内部 629/629 同，差异在 SVG 图形边沿（edge 18.8%），最大 10；基线早于 2026-11-11 SDF LUT。
- `ui_wr_p38_svg_filter`：内部 629/629 同，差异在 SVG 滤镜/图形边沿（edge 19.0%），最大 10；基线早于 2026-11-11 SDF LUT。
- `ui_wr_p38_svg_paint`：内部 629/629 同，差异在 SVG paint-server 图形边沿（edge 19.0%），最大 10；基线早于 2026-11-11 SDF LUT。
- `ui_wr_p38_svg_showcase`：内部 629/629 同，差异在 SVG 图形边沿（edge 19.0%），最大 9；基线在 P3.8 premul 批重种，但早于 SDF LUT。
- `ui_wr_p38_svg_structure`：内部 814/814 同，差异在 marker/结构图形边沿（edge 17.3%），最大 10；基线早于 2026-11-11 SDF LUT。
- `ui_wr_p38_svg_text`：内部 629/629 同，差异在 SVG 文字/图形边沿（edge 19.6%），最大 10；基线早于 2026-11-11 SDF LUT。

## 6 独立发现：webp 逻辑探针 format 期望过期（不是金图漂移）

`ui_render_base_webp` 的 golden 漂移按 §3 判为基线过期；但该窗同时有一条逻辑探针红：

```
webp probe webp_lossless_format FAIL: format=RGBAPremul want RGBA8
FAIL: logic probes 18/19 bad=[webp_lossless_format]
```

- P1.4 基线（2026-10-29）该探针为 pass；P3.8 `d5f9af60`「WebP 真修 FromStdImage NRGBA 漏预乘」把 `FromStdImage` 的存储格式统一为 `FormatRGBAPremul`（`render/internal/image/io.go:313-355`）。
- 同窗的像素探针 24/24 全过：`webp_lossless_exact_vs_png` 逐字节 0 差、alpha 直方图 379/64/5701 全对；失败只来自“格式标签”期望仍是旧的 `FormatRGBA8`。
- 定性：**测试 oracle 过期，不是渲染洞**。已按用户 2026-11-12 确认把探针期望改为 `FormatRGBAPremul`（`examples/ui_render_base_webp/probes.go`），重跑 logic 19/19、pixel 24/24、golden 0.0000%。

## 7 重种清单与执行结果

**重种 21 窗**（`ui_render_base_svg` 不重种）：

- `ui_render_base_backdrop`
- `ui_render_base_cliprrect`
- `ui_render_base_dither`
- `ui_render_base_filterops`
- `ui_render_base_maskops`
- `ui_render_base_miscdraw`
- `ui_render_base_pathbool`
- `ui_render_base_textlow`
- `ui_render_base_textspacing`
- `ui_render_base_textstroke`
- `ui_render_base_verticesmesh`
- `ui_render_base_webp`
- `ui_wr_shadermask`
- `ui_wr_p38_color_font`
- `ui_wr_p38_picture`
- `ui_wr_p38_svg_doc`
- `ui_wr_p38_svg_filter`
- `ui_wr_p38_svg_paint`
- `ui_wr_p38_svg_showcase`
- `ui_wr_p38_svg_structure`
- `ui_wr_p38_svg_text`

执行（2026-11-12，用户确认「全部重种 + 修 webp 探针」）：

1. 21 窗当前图（`runs/<窗>/native|go/current.png`）复制到各自 `*_base.png`；旧基线归档 `old-bases/`，逐项哈希见 `reseed.txt`。`ui_wr_p38_color_font` / `picture` 另写 `_go_base.png`（go 快照名不同）。
2. `ui_render_base_webp` 探针期望改为 `FormatRGBAPremul`（`examples/ui_render_base_webp/probes.go`），重跑 logic 19/19。
3. 21 窗 × 双后端复跑 42 轮：**42/42 rc=0、42/42 `pixel_golden_diff_pct=0.0000%`、21/21 窗 native/go 内容区逐字节相同**；明细 `verify/SUMMARY.md`、`verify/run-index.tsv`。
4. 窗口级 1px 负向（`ui_render_base_backdrop` + `SNAP_DIR` 临时目录）：平移基线 → `diff=12.9901%`、rc=1；还原 → `0.0000%`、rc=0；原文件未动。
5. `ui_base_vram_cap` 按既有 i5-v2 证据同批处理：8 窗重种；native Mailbox + go/gles 各 300s cascade，8/8 `0.0000%`、像素 16/16、rc=0；见 `i5-vram/closeout.md`。首轮 win1 snapshot 未写出竞态登记 TASKS §2.2 N3。

## 8 证据索引

- 运行索引：`run-index.tsv`（44 行，窗/后端/rc/golden pct/total px/logic/backend/screen/snapshot/base）
- 逐窗原始输出：`runs/<窗>/<后端>/{stdout.json,stderr.log,rc,elapsed_sec,current.png,base.png}`
- 逐窗分析 JSON：`analysis.jsonl`（44 行）
- 逐窗视觉对照：`analysis/<窗>/<后端>/{diff.png,compare.png}`
- 分析器自检：`selftest.txt`（identical=0；1px 平移=非零且检出；单点改色=1 px；PASS）
- 采集/分析脚本：`tools/run_windows.sh`、`tools/analyze_golden.py`、`tools/run_analysis.py`、`tools/selftest_analysis.py`、`tools/gen_assessment.py`；重种/复跑：`tools/reseed.py`、`verify/`、`reseed.txt`

## 9 重种后验收汇总

| 项 | 结果 |
|---|---|
| 21 窗 × 双后端复跑 | 42/42 rc=0；42/42 golden 0.0000% |
| 跨后端内容 | 21/21 窗 native vs go 内容区 `content_diff=0`、max Δ=0 |
| webp 逻辑探针 | `logic_pass=true`、19/19、pixel 24/24、golden 0.0000% |
| 1px 窗口级负向 | 平移基线 `12.9901%` → rc=1；还原 `0.0000%` → rc=0 |
| I5 `ui_base_vram_cap` | native 300s + go 300s 各 8/8 窗 `0.0000%`、像素 16/16、rc=0 |

**状态：P2.6 22 窗金图漂移专项 ✅；I5 金图债 ✅。**
