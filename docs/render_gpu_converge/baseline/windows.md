# 真窗与金图清单（P0.2）

> 用途：P1.4 全窗采集、P1.6 显存快照、P2.6 行为零变化重放的**逐窗执行表**。表里每行都能直接照着跑；金图路径、容差、平台、运行方式、覆盖能力五样齐全。
>
> 生成日期：2026-10-28。仓库起点 `876b7835`。口径来源：`baseline/thresholds.md`（P0.1）+ 各窗 README/main.go 实测。

---

## 0 口径与约定

### 0.1 什么算真窗

| 类别 | 判定 | 数量 |
|---|---|---|
| 真窗 | `examples/**/main.go` 且导入 `ui/platform` 或 `ui/embedder`，会打开真实窗口 | **162** |
| 非真窗 | 只做 headless 渲染并写 PNG，不开窗 | **28**（见 §3.8） |
| 合计 | `examples/` 下全部 `main.go` | 190 |

**空白检查**：空目录（无 `main.go`）不算窗、不得标绿（`AGENTS.md`）。本表按 `find examples -name main.go` 与导入关系双向核对，162 + 28 = 190，无遗漏。

### 0.2 什么算金图

- **仓内基线**：`testdata/*_base.png`、`testdata/*golden*.png`、`golden/*.png` 中作为基线的那张。
- **运行产物**：`*_final.png`、`*_last.png`、非 `_base` 的快照（如 `c4_open.png`、`c5_final.png`）是当次输出，不是基线。
- **运行期基线**：部分窗首跑在 `/tmp/<窗名>` 产出基线、次跑起逐位对比，仓内没有提交基线。本表标 **「运行期」**，P1.4 必须把 `/tmp` 基线归档到 `baseline/windows/<窗名>/`，否则 P2.6 无法重放。

### 0.3 容差口径

| 写法 | 含义 |
|---|---|
| `0` | 同引擎金图逐位零容差（任一通道差即失败、尺寸不符直接失败），见 P0.1 §6 |
| `0*` | 仓内无基线，首跑播种、次跑起零容差（运行期基线） |
| `—` | 无金图；替代判定写在「能力标签/备注」或 §4 |

像素探针容差（不是金图容差）按窗 README 写死：engine 多为 4 或 8/255，render 基座 8/255（个别 2–4），混合类 12/255。这些不影响金图列的 `0`。

### 0.4 运行方式

| 代号 | 含义 | 命令形态 |
|---|---|---|
| `A` | `-auto-only` 自动门禁 + 无参常驻人工 | `RUN_SECONDS=N go run ./examples/<dir> -auto-only`；人工 `go run ./examples/<dir>` |
| `A+M` | 同上，另支持 `-manual-seconds` | 首个命令后再跑 `go run ./examples/<dir> -manual-seconds N` |
| `R` | 无 `-auto-only`，靠 `RUN_SECONDS` 进门禁档；不设则常驻人工 | `RUN_SECONDS=N go run ./examples/<dir>` |
| `M` | 纯人工/探针窗，无自动门禁 | `go run ./examples/<dir>`（按 README 看 PASS/FAIL 表） |

`N` 取「关闭用时长」（§2.5 档位）；下表 `A/8s` 表示自动档用 8 秒。

### 0.5 平台/后端

- 本表平台列写 **Linux X11**：这是 2026-10-28 本机实测入口（README 命令统一 `DISPLAY=:0/:1`）。
- **后端**：默认 `native`（webgpu-native）；`GPUI_BACKEND=go` 切纯 Go GL。本表不逐窗宣称 GL 已绿——**双后端覆盖由 P0.4 一致性测试台判定**，P1.4 采集时两档都跑，跑不了的档写原因。
- Wayland：所有窗都可尝试，但批量/长跑必须走隔离环境（`AGENTS.md`「GPU 真窗与压测安全」）；Wayland 专窗单列。
- Windows/macOS：`ui_pf_win32` / `ui_pf_appkit` 是平台探针，本机只能验证「明确报未实现、不崩」，不得标平台绿。

### 0.6 能力标签词表

`矢量`（路径/几何/描边）· `裁剪` · `混合`（透明/合成/MSAA）· `文本`（含字形/富文本）· `图片`（解码/纹理）· `滤镜`（模糊/颜色/掩码）· `录制`（Picture/回放）· `着色器` · `2.5D` · `显存/生命周期` · `帧时/调度` · `缓存/边界`（retained/boundary/damage）· `命中/交互` · `窗口/平台` · `IME` · `视频` · `组件`

---

## 1 汇总

| 家族 | 窗数 | 仓内金图 | 运行期金图 | 无金图 |
|---|---:|---:|---:|---:|
| 2.5D 引擎 `engine/*` | 22 | 22 | 0 | 0 |
| render 原子真窗 `ui_render_base_*` / `ui_render_graphics/*` / `ui_ant_verify` / `ui_ruler_parity` | 32 | 31 | 0 | 1（perfsoak） |
| UI 能力窗 `ui_wr_*` | 46 | 35 | 8 | 3（r12/r20? 见下） |
| UI 基座/平台 `ui_base_*` / `ui_pf_*` / `ui_l1_*` / `ui_l2_*` / `ui_text*` / `ui_ime_probe` | 20 | 8 | 0 | 12 |
| 组件 `kit_f0_*` / `kit/*` | 16 | 15 | 0 | 1（spin_min） |
| 视频 `video/*` | 22 | 1 | 0 | 21 |
| 演示/对照 | 4 | 2 | 0 | 2（pelican / zz_soak_control） |
| **合计** | **162** | **114** | **8** | **40** |

> 逐族核对（与 §2–§3.6 表格逐行相减，2026-10-28 脚本复核）：
> - UI 能力窗 46 = 35 仓内 + 8 运行期 + 3 无（`ui_wr_r12_metrics`、`ui_wr_c11_policy_switch`、`ui_wr_t_stress`）。
> - UI 基座/平台 20 = 8 仓内 + 12 无（4 扇 L1/L2 平台窗、5 扇 `ui_pf_*` 平台探针、`ui_ime_probe`、`ui_text_m4_virtual`）。
> - 全表 114 + 8 + 40 = 162，与 §0.1 的真窗总数一致。

---

## 2 2.5D 引擎窗（22）

> 统一：1200×800、满窗即内容、指标浮左上、Golden 裁掉指标带或按静态掩码。全部 `X11/native`。

| 窗（`examples/engine/…`） | 金图（窗内相对路径） | Golden | 运行 | 能力标签 |
|---|---|---|---|---|
| `25d_basis` | `testdata/basis_view_golden.png` | 0 | A/10s | 2.5D · 六视角基向量/排序/影子 |
| `anim` | `testdata/{fsm,lod,sk,tl}_golden.png`（4） | 0 | A/8s | 2.5D · FSM/LOD/骨骼/时间线 |
| `asset` | `testdata/asset_{large,reload}_golden.png` | 0 | A/8s | 2.5D · 资源热重载/坏文件保旧 |
| `camera` | `testdata/{camera_follow,voices}_golden.png` | 0 | A/8s | 2.5D · 相机跟随/限位/远带 |
| `dodge` | `testdata/dodge_golden.png` | 0 | A/8s | 2.5D · 精灵/碰撞/计分 |
| `fx` | `testdata/{fx_grade,fx_custom}_golden.png` | 0 | A/8s | 2.5D · 水波扭曲/后期调色 |
| `input` | `testdata/input_{remap,combo}_golden.png` | 0 | A/8s | 2.5D · 输入重映射/组合键 |
| `light` | `testdata/{light_torch,normal_dome,shadow_wall}_golden.png` | 0 | A/8s | 2.5D · 点光/方向光/法线/阴影 |
| `light2d` | `testdata/soft_{none,pcf5,pcf13}_golden.png` | 0 | A/8s | 2.5D · 2D 光/软阴影三档 |
| `nav` | `testdata/nav_golden.png` | 0 | A/8s | 2.5D · 网格烘焙/AStar/百怪分摊 |
| `particle` | `testdata/particle_{fire,trail}_golden.png` | 0 | A/8s | 2.5D · CPU 粒子/合批 |
| `particles2d` | `testdata/particles2d_boom_golden.png` | 0 | A/8s | 2.5D · 火/烟/爆三发射器 |
| `physics` | `testdata/physics_{hit,jump,large}_golden.png` | 0 | A/8s | 2.5D · 射线/碰撞/跳跃 |
| `platformer` | `testdata/platformer_golden.png` | 0 | A/8s | 2.5D · 横版手感/落地 |
| `quad` | `testdata/quad_window_golden.png` | 0 | R=8s | 2.5D · 四边形梯形对齐 |
| `save` | `testdata/save_q123_golden.png` | 0 | A | 2.5D · 存档读写 |
| `sprite` | `testdata/{sprite,anim_off}_golden.png` | 0 | A | 2.5D · 合批/图集/遮挡/帧动画 |
| `stage_chase` | `testdata/stage_chase_golden.png` | 0 | A/120s | 2.5D · 相机/排序/粒子/瓦片/流式组合 |
| `step` | `testdata/step_dirty_golden.png` | 0 | A/8s | 2.5D · 脏区（只更脏区） |
| `tex` | `testdata/tex_{far,stream}_golden.png` | 0 | A/8s | 2.5D · mipmap/流式/缺图 |
| `tilemap` | `testdata/tilemap_{map,chunk,lod,large}_golden.png` | 0 | A | 2.5D · 瓦片/分区/LOD/大世界 |
| `world` | `testdata/world_{open,large}_golden.png` | 0 | A/8s | 2.5D · 场景 JSON/多视口/挂点 |

---

## 3 逐窗表

### 3.1 render 原子真窗（32）

> 统一：1200×800（图形窗）、`X11/native`。金图掩码多为「表头+图例+卡片」；像素探针容差 8/255、个别 2–4。

| 窗（`examples/…`） | 金图（窗内相对路径） | Golden | 运行 | 能力标签 |
|---|---|---|---|---|
| `ui_render_base_backdrop` | `testdata/showcase_backdrop_base.png`、`…_msaa4_base.png` | 0 | A/5s | 滤镜 · Backdrop 模糊/4x MSAA |
| `ui_render_base_cliplayer` | `testdata/showcase_cliplayer_base.png` | 0 | A/15s | 裁剪 · Boundary/Clip/Opacity/Transform |
| `ui_render_base_clipop` | `testdata/showcase_clipop_base.png` | 0 | A/5s | 裁剪 · difference/replace |
| `ui_render_base_cliprrect` | `testdata/showcase_cliprrect_base.png` | 0 | A/5s | 裁剪 · 圆角裁剪图片/文字 |
| `ui_render_base_dither` | `testdata/showcase_dither_base.png` | 0 | A/5s | 混合 · SetDither |
| `ui_render_base_emoji` | `testdata/showcase_emoji_base.png` | 0 | A/5s | 文本 · CBDT/CBLC + COLR 合成 |
| `ui_render_base_filterops` | `testdata/showcase_filterops_base.png` | 0 | A/5s | 滤镜 · BlurXY/图链/发布视图 |
| `ui_render_base_geometry` | `testdata/showcase_geometry_base.png` | 0 | A/15s | 矢量 · Rect/RRect/线/圆/渐变/Path |
| `ui_render_base_image` | `testdata/showcase_image_base.png` | 0 | A/15s | 图片 · Image 轴 |
| `ui_render_base_imagepattern` | `testdata/showcase_imagepattern_base.png` | 0 | A/5s | 图片 · 旧 Pattern/画笔 |
| `ui_render_base_maskops` | `testdata/showcase_maskops_base.png` | 0 | A/5s | 滤镜 · Set/Clear/Invert/Apply/PushMaskLayer |
| `ui_render_base_miscdraw` | `testdata/showcase_miscdraw_base.png` | 0 | A/5s | 矢量 · 杂项五 API/DrawShadow |
| `ui_render_base_pathbool` | `testdata/showcase_pathbool_base.png` | 0 | A/5s | 矢量 · Path.Op 布尔 |
| `ui_render_base_pathlow` | `testdata/showcase_pathlow_base.png` | 0 | A/5s | 矢量 · 路径低频三 API |
| `ui_render_base_pathshape` | `testdata/showcase_pathshape_base.png` | 0 | A/5s | 矢量 · Trim/WithCorners/Discrete |
| `ui_render_base_perfsoak` | — | — | R=300s | 帧时/调度 · 60fps/hitch/RSS/CPU/close（无金图：纯性能） |
| `ui_render_base_recording` | `testdata/showcase_recording_base.png` | 0 | A/5s | 录制 · Recorder/Picture 播放 |
| `ui_render_base_scene` | `testdata/showcase_scene_base.png` | 0 | A/5s | 图片 · CPU tile/GPU 场景双路 |
| `ui_render_base_strokepreset` | `testdata/showcase_strokepreset_base.png` | 0 | A/5s | 矢量 · 描边预设族 |
| `ui_render_base_svg` | `testdata/showcase_svg_base.png` | 0 | A/5s | 矢量 · SVG 图标栅格化 |
| `ui_render_base_sweepgradient` | `testdata/showcase_sweepgradient_base.png` | 0 | A/5s | 混合 · 锥形渐变 |
| `ui_render_base_text` | `testdata/showcase_text_base.png` | 0 | A/15s | 文本 · 多语言样板 |
| `ui_render_base_textlow` | `testdata/showcase_textlow_base.png` | 0 | A/5s | 文本 · 低频文本 API |
| `ui_render_base_textspacing` | `testdata/showcase_textspacing_base.png` | 0 | A/5s | 文本 · 字距/行距 |
| `ui_render_base_textstroke` | `testdata/showcase_textstroke_base.png` | 0 | A/5s | 文本 · 描边/沿路径 |
| `ui_render_base_variablefont` | `testdata/showcase_variablefont_base.png` | 0 | A/5s | 文本 · 可变字体 |
| `ui_render_base_verticesmesh` | `testdata/showcase_verticesmesh_base.png` | 0 | A/5s | 矢量 · 顶点网格 |
| `ui_render_base_webp` | `testdata/showcase_webp_base.png` | 0 | A/5s | 图片 · WebP 有损/无损/缩放 |
| `ui_render_graphics/basic` | `testdata/showcase_basic_base.png` | 0 | A/5s | 矢量 · Point/Line/Triangle |
| `ui_render_graphics/vector` | `testdata/showcase_vector_base.png` | 0 | A/5s | 矢量 · Quad/Cubic/Arc |
| `ui_ant_verify` | `testdata/ant_verify_msaa{1,4}_base.png` | 0 | A/5s | 混合 · MSAA 1x/4x |
| `ui_ruler_parity` | `testdata/golden_{flutter_list,flutter_spin,flutter_text,godot_bullets}.png` | 0 | A/60s | 文本/矢量 · Flutter/Godot 同语义对照 |

### 3.2 UI 能力窗（46）

> `R*` 单能力窗、`C*` 组合窗；1200×800、`X11/native`。金图掩码多为 header 1200×48 + legend 260×656 + body 904×656 = 821184 px。

| 窗（`examples/…`） | 金图（窗内相对路径） | Golden | 运行 | 能力标签 |
|---|---|---|---|---|
| `ui_wr_r0_fullpaint` | `testdata/showcase_r0_fullpaint_base.png` | 0 | A/5s | 缓存/边界 · FullPaint 正确性 |
| `ui_wr_r1_layout` | 运行期 `/tmp/r1_layout/r1_final_base.png`（`R1_SNAP_DIR`） | 0* | R=30s | 缓存/边界 · 局部 NeedsLayout |
| `ui_wr_r2_paint` | `testdata/showcase_r2_paint_base.png` | 0 | A/5s | 缓存/边界 · 局部 NeedsPaint |
| `ui_wr_r3_boundary` | `testdata/showcase_r3_boundary_base.png` | 0 | A/10s | 缓存/边界 · 嵌套 Boundary |
| `ui_wr_r3b_compbits` | `testdata/showcase_r3b_compbits_base.png` | 0 | A/8s | 缓存/边界 · 合成位/边界发现 |
| `ui_wr_r4_composite` | `testdata/showcase_r4_composite_base.png` | 0 | A/15s | 缓存/边界 · 层合成分带 |
| `ui_wr_r4b_multidamage` | `testdata/r4b_multidamage_base.png` | 0 | A/15s | 缓存/边界 · DirtyLayerID/多 damage |
| `ui_wr_r5_picture` | `testdata/showcase_r5_picture_base.png` | 0 | A/5s | 录制 · Picture 录/回放 |
| `ui_wr_r6_layer_anim` | `golden/baseline_igpu.png` | 0 | R=30s | 混合 · Opacity/Transform/Clip 层动画 |
| `ui_wr_r7_virtlist` | `testdata/r7_virtlist_base.png` | 0 | A/60s | 缓存/边界 · 虚拟列表 |
| `ui_wr_r7b_scroll_reuse` | `testdata/showcase_r7b_scroll_reuse_base.png` | 0 | A/60s | 缓存/边界 · 滚动 cell 复用 |
| `ui_wr_r8_overlay` | `testdata/showcase_r8_overlay_{open,closed}_base.png` | 0 | A/15s | 混合 · 浮层独立合成双态 |
| `ui_wr_r9_text_cache` | `testdata/showcase_r9_text_cache_base.png` | 0 | A/5s | 文本/缓存 · measure 缓存 |
| `ui_wr_r10_async_image` | `testdata/showcase_r10_async_image_base.png` | 0 | A/30s | 图片 · 异步图→局部脏 |
| `ui_wr_r11_dpr` | `testdata/showcase_r11_dpr_base.png` | 0 | A/15s | 缓存/边界 · DPR/尺寸失效 |
| `ui_wr_r12_metrics` | — | — | R=5s | 帧时/调度 · A–J 指标字段完备（无金图：指标窗） |
| `ui_wr_r12b_debug_repaint` | `testdata/showcase_r12b_debug_{on,off}_base.png` | 0 | A/8s | 缓存/边界 · 重绘调试 on/off |
| `ui_wr_r13_hit` | `testdata/showcase_r13_hit_base.png` | 0 | A/5s | 命中/交互 · Hit ≡ 绘 |
| `ui_wr_r14_cache_budget` | 运行期 `/tmp/r14_cache_budget/r14_final_base.png`（`R14_SNAP_DIR`） | 0* | R=60s | 缓存/边界 · 预算/淘汰 |
| `ui_wr_r15_soak` | 运行期 `/tmp/r15_soak/r15_final_base.png`（`R15_SNAP_DIR`） | 0* | R=300s | 帧时/调度 · 全主路径长跑 |
| `ui_wr_r16_warmup` | `testdata/showcase_r16_warmup_{pre,post}_base.png` | 0 | A/5s | 窗口/平台 · 首帧/WarmUp/恢复 |
| `ui_wr_r17_bg_throttle` | `testdata/r17_bg_throttle_base.png` | 0 | A/30s | 帧时/调度 · 不可见停帧 |
| `ui_wr_r18_savelayer` | `testdata/showcase_r18_savelayer_base.png` | 0 | A/10s | 混合/滤镜 · SaveLayer 预算 |
| `ui_wr_r19_snap` | `testdata/showcase_r19_snap_base.png` | 0 | A/5s | 矢量 · 1px/设备像素对齐 |
| `ui_wr_r20_filter` | 运行期 `/tmp/r20_filter/r20_final_base.png`（`R20_SNAP_DIR`） | 0* | R=10s | 滤镜 · 模糊/灰度滤镜子树 |
| `ui_wr_r21_shell` | `testdata/r21_shell_base.png` | 0 | A/15s | 缓存/边界 · 壳/内容分层 |
| `ui_wr_r22_selection_stub` | 运行期 `/tmp/r22_selection_stub/r22_final_base.png`（`R22_SNAP_DIR`） | 0* | R=5s | 缓存/边界/文本 · 选区光标局部脏 |
| `ui_wr_c0_smoke` | `testdata/showcase_c0_smoke_{pre,post}_base.png` | 0 | A/5s | 缓存/边界/帧时 · R0+R12+R16 集成 |
| `ui_wr_c1_boundary_nest` | `testdata/showcase_c1_boundary_nest_base.png` | 0 | A/10s | 缓存/边界 · R2+R3+R3b+R12b |
| `ui_wr_c2_retained_scene` | `testdata/showcase_c2_retained_scene_base.png` | 0 | A/15s | 缓存/录制 · R3+R4+R4b+R5 |
| `ui_wr_c3_list_scroll` | `testdata/showcase_c3_list_scroll_base.png` | 0 | A/60s | 缓存/图片 · 虚拟列表+滚复用+异步图 |
| `ui_wr_c4_shell_overlay` | `golden/c4_{steady,recover}_base.png` | 0 | R=25s | 混合/缓存 · R3+R8+R21 壳浮层 |
| `ui_wr_c5_anim_over_static` | `golden/c5_final_base.png` | 0 | R=30s | 混合/缓存 · R6+R3+R4 动画盖静态 |
| `ui_wr_c6_savelayer_group` | 运行期 `/tmp/c6_savelayer_group/c6_final_base.png`（`C6_SNAP_DIR`） | 0* | R=10s | 混合/显存 · R18+R3 离屏组拒批 |
| `ui_wr_c7_resize_dpr` | `testdata/c7_resize_dpr_base.png` | 0 | A/15s | 缓存/DPR · R11+R19+R3 |
| `ui_wr_c8_hit_overlay_xf` | `testdata/c8_recover_base.png` | 0 | A/15s | 命中/混合 · R13+R6+R8 |
| `ui_wr_c9_stress_cache` | 运行期 `/tmp/c9_stress_cache/c9_final_base.png`（`C9_SNAP_DIR`） | 0* | R=60s | 缓存/显存 · R14+R3+R7 压测 |
| `ui_wr_c10_soak` | 运行期 `/tmp/c10_soak/c10_final_base.png`（`C10_SNAP_DIR`） | 0* | R=300s | 帧时/显存 · 全能力长跑 |
| `ui_wr_c11_policy_switch` | — | — | R=15s | 缓存/帧时 · full↔retained 热切换（无金图：策略窗） |
| `ui_wr_ime_r1_editor` | `testdata/showcase_ime_r1_editor_base.png` | 0 | A/5s | IME/文本 · 编辑器四元组 |
| `ui_wr_ime_r2_textlayout` | `testdata/r2_textlayout_base.png` | 0 | A | IME/文本 · 文本布局单源 |
| `ui_wr_ime_r3_channel` | `testdata/r3_channel_base.png` | 0 | A | IME/窗口 · 通道与平台 |
| `ui_wr_ime_r4_selection` | `testdata/r4_selection_base.png` | 0 | A | IME/文本 · 选区与编辑 |
| `ui_wr_ime_r5_control` | `testdata/r5_control_base.png` | 0 | A | IME/文本 · 控件与滚动 |
| `ui_wr_shadermask` | `testdata/showcase_shadermask_base.png` | 0 | A/5s | 着色器 · ShaderMask 用户空间 |
| `ui_wr_t_stress` | — | — | A/8s | 文本/帧时 · 三线压力（无金图：压力窗） |

### 3.3 UI 基座/平台窗（20）

| 窗（`examples/…`） | 金图（窗内相对路径） | Golden | 运行 | 平台/后端 | 能力标签 |
|---|---|---|---|---|---|
| `ui_base_batch_cap` | `testdata/batch_cap_{prims,vector}_base.png` | 0 | A/200s | X11/native | 显存/生命周期 · 合批上限 |
| `ui_base_idle` | `testdata/showcase_idle_base.png` | 0 | A/60s | X11/native | 帧时/调度 · 零脏零帧 |
| `ui_base_multiwin_cap` | `testdata/multiwin_win{1..8}_base.png` | 0 | 批量(8 窗) A/75s | X11/native | 显存/生命周期 · 多窗共享设备 |
| `ui_base_vram_cap` | `testdata/vram_win{1..8}_base.png`、`testdata/vram_oom_probe_base.png` | 0 | 批量(8 窗+OOM) A/60s | X11/native | 显存/生命周期 · 预算/水位/OOM 回落 |
| `ui_l1_blank` | — | — | R=60s | X11 或 Wayland/native | 窗口/平台 · 开窗清屏上屏 |
| `ui_l1_render_matrix` | `testdata/showcase_l1_render_matrix_base.png` | 0 | A/15s | X11/native | 缓存/边界 · L1 retained 契约烟囱 |
| `ui_l1_scroll` | — | — | R=60s | X11 或 Wayland/native | 窗口/平台 · 滚动 |
| `ui_l1_spinner` | — | — | R=60s | X11 或 Wayland/native | 帧时/调度 · 动画 |
| `ui_l2_shell` | — | — | R=120s | X11 或 Wayland/native | 窗口/平台 · 壳交互 |
| `ui_ime_probe` | — | — | M | X11 或 Wayland/native | IME/窗口 · 输入法能力探针（无金图：协议探针） |
| `ui_pf_x11` | — | — | M | X11/native | 窗口/平台 · X11 全能力 |
| `ui_pf_wayland` | — | — | M | Wayland/native | 窗口/平台 · Wayland 全能力（隔离环境） |
| `ui_pf_win32` | — | — | M | Windows/— | 窗口/平台 · Win32 占位（本机不可验） |
| `ui_pf_appkit` | — | — | M | macOS/— | 窗口/平台 · AppKit 占位（本机不可验） |
| `ui_pf_x11_device` | — | — | A | X11/native | 窗口/平台 · XInput 设备能力 |
| `ui_pf_x11_dnd` | — | — | A | X11/native | 窗口/平台 · 拖放 |
| `ui_textinput_ime` | `testdata/textinput_ime_base.png` | 0 | A | X11/native | IME/文本 · 输入框 IME 全链路 |
| `ui_text_edit_accept` | `golden/baseline_igpu.png`、`golden/f_script/s{1,2,3}.png` | 0 | R=30s | X11/native | 文本 · 单/多行输入框验收 |
| `ui_text_m4_virtual` | — | — | R=30s | X11/native | 文本/缓存 · 1e6 行虚拟化（无金图：几何/懒测量为主） |
| `ui_text_m5_anycase` | `testdata/showcase_text_m5_base.png` | 0 | A/30s | X11/native | 文本 · 多脚本/emoji/富文本/DPR |

> `ui_text_m5_anycase/golden/baseline_{igpu,m5}.png` 是 2026-09 历史参照，U21 口径以 `testdata/showcase_text_m5_base.png` 为准（README 明示）。`ui_text_edit_accept/golden/baseline_*.png` 按同机同字体零容差使用，换机只目检。

### 3.4 组件窗（16）

| 窗（`examples/…`） | 金图（窗内相对路径） | Golden | 运行 | 能力标签 |
|---|---|---|---|---|
| `kit_f0_content` | `testdata/showcase_content_base.png` | 0 | A/15s | 组件/文本/图片 · 单/多行/省略/富文本/图标/图片三态 |
| `kit_f0_decor` | `testdata/showcase_decor_base.png` | 0 | A/5s | 组件/混合 · 装饰/透明度公式 |
| `kit_f0_interact` | `testdata/showcase_interact_base.png` | 0 | A/15s | 组件/命中 · hover/禁用/浮层 |
| `kit_f0_layout` | `testdata/showcase_layout_base.png` | 0 | A/5s | 组件 · 行/叠放/换行布局 |
| `kit_f0_motion` | `testdata/showcase_motion_base.png` | 0 | A/30s | 组件/帧时 · 动效 |
| `kit_f0_scope` | `testdata/showcase_scope_base.png` | 0 | A/5s | 组件/混合 · 主题作用域 |
| `kit_f0_theme` | `testdata/showcase_theme_{default,skinned,compact}_base.png` | 0 | A/10s | 组件 · 三主题 |
| `kit/color-model` | `testdata/showcase_color_model_base.png` | 0 | A/5s | 组件 · 颜色值 |
| `kit/date-engine` | `testdata/showcase_date_engine_base.png` | 0 | A/5s | 组件 · 日期计算 |
| `kit/icon` | `testdata/showcase_icon_base.png` | 0 | A/5s | 组件 · 图标 |
| `kit/modal-host` | `testdata/showcase_modal_host_base.png` | 0 | A/5s | 组件/命中 · 弹窗栈 |
| `kit/notice-queue` | `testdata/showcase_notice_queue_base.png` | 0 | A/5s | 组件 · 通知队列 |
| `kit/qrcode` | `testdata/showcase_qrcode_base.png` | 0 | A/5s | 组件 · 二维码 |
| `kit/spin_min` | — | — | A/60s | 组件/帧时 · 旋转图标（无金图：只出 stderr 指标+JSON） |
| `kit/tour-mask` | `testdata/showcase_tour_mask_base.png` | 0 | A/5s | 组件/裁剪/混合 · 引导遮罩 |
| `kit/upload` | `testdata/showcase_upload_base.png` | 0 | A/5s | 组件 · 上传列表 |

### 3.5 视频窗（22）

> 全部 `X11/native`；视频素材/码流来自 `testdata/`。除 `video_retained_golden` 外都靠逻辑探针判定，无金图（画面每帧在变，做固定点金图必假绿——见 P0.1 §1 原则）。

| 窗（`examples/video/…`） | 金图 | Golden | 运行 | 能力标签 |
|---|---|---|---|---|
| `video_retained_golden` | `testdata/showcase_retained_golden_dpr1_base.png` | 0 | A | 视频/录制 · 固定帧纹理层零容差 |
| `video_player` | — | — | R=15s | 视频 · 用法小样（非门禁） |
| `video_2k_play` | — | — | R=30s | 视频 · 2K 直传 |
| `video_4k_play` | — | — | R=60s | 视频 · 4K 直传/同源双绘 |
| `video_a2_sync` | — | — | A/15s | 视频 · 音画对齐 |
| `video_a4_sink` | — | — | A/15s | 视频 · 宿主出声 |
| `video_b1_frag` | — | — | A/15s | 视频 · 分段 MP4 |
| `video_s8_index` | — | — | A/15s | 视频 · 跳转索引 |
| `video_vc0_fullplay` | — | — | R=30s | 视频 · 全链路组合 |
| `video_vc1_seek_fault` | — | — | R=30s | 视频 · 边播边跳+坏文件 |
| `video_vc2_soak` | — | — | R=120s | 视频 · 长跑组合 |
| `video_vc3_embed_extend` | — | — | R=30s | 视频 · 内嵌+注册+播放 |
| `video_vr0_demux` | — | — | R=5s | 视频 · MP4 拆盒 |
| `video_vr1_params` | — | — | R=5s | 视频 · 参数与切分 |
| `video_vr2_decode` | — | — | R=15s | 视频 · H.264 解码 |
| `video_vr3_color` | — | — | R=5s | 视频 · 颜色转换 |
| `video_vr4_play` | — | — | R=30s | 视频 · 播放管线 |
| `video_vr5_seek` | — | — | R=15s | 视频 · 跳进度 |
| `video_vr6_fault` | — | — | R=15s | 视频 · 容错 |
| `video_vr7_perf` | — | — | R=60s | 视频/帧时 · 性能极限 |
| `video_vr8_embed` | — | — | R=30s | 视频 · 真窗内嵌 |
| `video_vr9_registry` | — | — | R=15s | 视频 · 扩展注册 |

### 3.6 演示/对照窗（4）

| 窗（`examples/…`） | 金图 | Golden | 运行 | 能力标签 |
|---|---|---|---|---|
| `ui_render_jumprope` | `testdata/jumprope_base.png` | 0 | A/15s | 矢量/帧时 · 双人摇绳动画 |
| `ui_render_particle` | `testdata/particle_base.png` | 0 | A/15s | 混合/帧时 · 粒子特效 |
| `ui_render_pelican` | — | — | R=15s | 帧时/窗口 · 整窗动画（无金图：整窗全动，见 C5 对照） |
| `zz_soak_control` | — | — | 批量(3 窗) R=180s | 帧时/窗口 · 多窗小损伤对照 |

### 3.7 非真窗例子（28，headless）

> 不开窗口，只渲染 PNG 到 `tmp/` 或 `out/`。**不列入 P1.4 真窗采集**；其能力验证靠单测与这些输出图，P2 重构时按文件逐跑。

| 例子 | 产物 | 说明 |
|---|---|---|
| `render_basic`、`render_shapes`、`render_clipping`、`render_bezier_test`、`render_pixel_perfect`、`render_sdf`、`render_gpu` | `tmp/*.png` | render 2D 基础/形状/裁剪/贝塞尔/像素/SDF/GPU 头less 输出 |
| `render_text`、`render_cjk_text`、`render_text_fallback`、`render_text_transform`、`render_variable_font`、`render_color_emoji` | `tmp/*.png` | 文本/回退/变换/VF/彩色 emoji 头less |
| `render_images`、`render_scene`、`render_scene_gpu`、`render_recording` | `tmp/*.png` | 图片/场景 CPU-GPU/录制头less |
| `render_visual_basic`、`render_visual_shapes`、`render_visual_clipping`、`render_visual_gpu`、`render_visual_images`、`render_visual_scene`、`render_visual_scene_gpu`、`render_visual_text`、`render_visual_cjk_text`、`render_visual_text_transform` | `tmp/*.png` | 视觉对照版头less |
| `text_langs_png` | `out/samples_*.png` | 全语言文本大图 |

---

## 4 PLAN §5/§6 能力 → 窗映射

> 规则：每项能力至少一扇窗；没有窗的写明「现无窗」+ 替代验证 + 计划窗名。P3/P4 建窗后回填本节。

### 4.1 PLAN §5.1 任意图形

| 能力 | 现有窗 | 缺口与替代 |
|---|---|---|
| 自定义/运行时着色器 | `ui_wr_shadermask`（ShaderMask 单点） | 通用自定义 shader API 无窗；P3.2 建窗，现靠单测 |
| 色彩管理 HDR/广色域/ICC | 无 | P3.1 建 `ui_wr_hdr`；现无替代（先红后绿） |
| MaskFilter | 无专属 | `maskops`/`filterops` 只覆盖邻接路径；P3.4 建窗 |
| ColorFilter 家族 | 无专属 | `filterops` 覆盖 ImageFilter 图链（Blur/Grayscale/Invert），ColorFilter 家族 P3.4 |
| ImageFilter 家族 | `ui_render_base_filterops`、`ui_wr_r20_filter` | erode/dilate/lighting/offset/tile/merge/displacement/alpha-threshold 待 P3.4 |
| conic 路径段 | 无 | `pathshape` 只覆盖 Trim/WithCorners/Discrete；P3.5 单测+金图 |
| 四角独立圆角 | `ui_render_base_pathshape`（统一半径） | 四角独立半径 P3.5 |
| 4×4 透视矩阵 | 无 | `ui_l1_render_matrix` 是 retained 契约窗，不是矩阵窗；P3.5 |
| mesh gradient | 无 | `sweepgradient` 不是 mesh；P3.5 |
| GPU mipmap 链 | `engine/tex` | 已覆盖 |
| 图片格式 GIF/APNG/动图 WebP/AVIF/HEIF/EXIF | `ui_render_base_webp`（静态 WebP）、`ui_render_base_emoji`（CBDT/CBLC） | 动图/AVIF/HEIF/EXIF 无窗；P3.6 |
| 路径效果 1D/2D/Sum/Compose | 无 | `strokepreset`/`pathlow` 只覆盖描边/低频；P3.5 |
| 布尔保留曲线 | `ui_render_base_pathbool` | 保曲线精度验收在 P3.5 补 |
| SVG 完整语义 | `ui_render_base_svg`（图标栅格化） | defs/use/clipPath/text/filter 待 P3.8 |

### 4.2 PLAN §5.2 UI 组件库

| 能力 | 现有窗 | 缺口与替代 |
|---|---|---|
| render 层逐段富文本样式 | `kit_f0_content`、`ui_text_m5_anycase`（组件层富文本 run） | render 层逐 run API 待 P3.3 |
| 两端对齐 Justify | 无 | `textspacing` 是字距；P3.3 单测+金图 |
| ellipsis/maxLines 下沉 | `kit_f0_content`（组件层） | 下沉到 render 待 P3.3 |
| COLRv1 完整 | `ui_render_base_emoji`（COLR 三层） | paint graph 完整层 P4.4 |
| 可复用嵌套 Picture | `ui_render_base_recording`、`ui_wr_r5_picture` | 嵌套引用深度验收待 P3.8 |

### 4.3 PLAN §5.3 2.5D 引擎

| 能力 | 现有窗 | 缺口与替代 |
|---|---|---|
| 透视相机/投影矩阵 | `engine/25d_basis`（六视角基向量）、`engine/stage_chase` | 真透视矩阵待 P3.7 |
| 世界高度/精灵 z 序 | `engine/world`、`engine/tilemap`、`engine/sprite`、`engine/stage_chase` | 已覆盖 |
| 自定义材质 | 无 | 依赖 P3.2；P3.7 建窗 |
| HDR/泛光 | `engine/fx`（grade 调色） | 不是 HDR；P3.7 |
| GPU mipmap | `engine/tex` | 已覆盖 |

### 4.4 PLAN §6 半成品

| 能力 | 现有窗 | 缺口与替代 |
|---|---|---|
| 线性空间混合 | 无 | `decor`/`motion` 的混合公式是 sRGB；P3.1 |
| MSAA>1 even-odd 裁剪 | `ui_ant_verify`（MSAA 1/4）、`ui_render_base_clipop` | even-odd × MSAA>1 组合 P4.1 |
| 非整数裁剪矩形 | `ui_render_base_cliprrect`、`ui_render_base_clipop` | 分数 scissor 验收 P4.1 |
| 顶点网格 alpha/blend | `ui_render_base_verticesmesh` | P4.1 |
| DrawShadow | `ui_render_base_miscdraw` | 环境光/透视剪影 P4.1 |
| GPU/CPU 模糊一致 | `ui_render_base_filterops`（6/255、1.5/255） | P4.2 逐位/统一算法 |
| mask 临时纹理池化 | `ui_render_base_maskops`、`ui_wr_r8_overlay` | 预算/帧时 P4.3 |
| backdrop 全表面快照 | `ui_render_base_backdrop` | 损伤区裁剪 P4.3 |
| 抖动整图+回读 | `ui_render_base_dither` | 只输出路径/避免回读 P4.3 |
| 覆盖率在 CPU | 无 | P3.0 性能项目，无窗 |
| COLRv1 基础层 | `ui_render_base_emoji` | P4.4 |
| DrawParagraph 单色 | `ui_render_base_text`、`kit_f0_content` | 逐 run 样式 P3.3 |

---

## 5 执行与采集规则（交给 P1.4/P1.6）

1. **顺序**：按 §2 → §3.1 → §3.2 → §3.3 → §3.4 → §3.5 → §3.6 逐窗跑，一个跑完再跑下一个；批量窗（multiwin/vram/zz_soak）单独排最后。
2. **每窗产物**：JSON 探针、稳态 PNG、Recover PNG、金图 diff 计数、`p95/p99`、`vsync_source`、显存账本；落 `baseline/windows/<窗名>/`。
3. **运行期金图**（`0*` 8 扇）：先带 `<族>_SNAP_DIR=baseline/windows/<窗名>` 跑首遍播种，再跑第二遍确认 `diff=0`；不把 `/tmp` 当永久基线。环境变量逐窗为 `R1_SNAP_DIR`/`R14_SNAP_DIR`/`R15_SNAP_DIR`/`R20_SNAP_DIR`/`R22_SNAP_DIR`/`C6_SNAP_DIR`/`C9_SNAP_DIR`/`C10_SNAP_DIR`（表内已逐行标注）。
4. **两后端**：默认 `native` 跑一遍；`GPUI_BACKEND=go` 再跑一遍，失败档记录「后端不支持/未实现 + 现象」，不静默跳过。
5. **平台例外**：`ui_pf_win32` / `ui_pf_appkit` 本机只验「明确拒绝、不崩」；`ui_pf_wayland` 与 Wayland 抽验走隔离环境。
6. **无金图窗**：按 §4 的替代判定跑（逻辑探针/像素断言/性能门禁），并在 `baseline/windows/<窗名>/README.md` 写清「为什么无金图」。
7. **安全**：批量与长跑遵守 `AGENTS.md` 隔离显示、标准 `WM_DELETE` 关闭、`WaitIdle` 顺序；当前桌面只做单窗验收。

---

## 6 自校验

```bash
# 1) 真窗/非真窗全集（应输出 windows=162 headless=28 total=190）
python3 - <<'PY'
import os, subprocess
dirs=[os.path.dirname(x) for x in subprocess.check_output(['find','examples','-name','main.go'],text=True).split()]
def win(d):
    s=open(os.path.join(d,'main.go'),encoding='utf-8',errors='ignore').read()
    return 'ui/platform' in s or 'ui/embedder' in s
w=[d for d in dirs if win(d)]
print("windows=%d headless=%d total=%d"%(len(w),len(dirs)-len(w),len(dirs)))
PY

# 2) 表内行数、分族计数、仓内金图路径存在性（应输出 rows=162 inrepo=114 runtime=8 none=40 且 missing=0，
#    分族计数须与 §1 汇总表逐行相等）
python3 - <<'PY'
import os,re
pre={'2':'examples/engine/','3.1':'examples/','3.2':'examples/','3.3':'examples/','3.4':'examples/','3.5':'examples/video/','3.6':'examples/'}
cur=None; rows={}
for l in open('docs/render_gpu_converge/baseline/windows.md').read().splitlines():
    m=re.match(r'^### (3\.\d) ',l)
    if m: cur=m.group(1); continue
    m=re.match(r'^## (\d) ',l)
    if m: cur='2' if m.group(1)=='2' else None; continue
    if cur in pre and l.startswith('| `'):
        c=[x.strip() for x in l.strip().strip('|').split('|')]
        rows.setdefault(cur,[]).append((c[0].strip('`'),c[1],c[2].strip('` ')))
def exp(s):
    m=re.search(r'\{([^{}]*)\}',s)
    if not m: return [s]
    out=[]
    for a in m.group(1).split(','):
        if '..' in a:
            lo,hi=a.split('..'); out+=[x for i in range(int(lo),int(hi)+1) for x in exp(s[:m.start()]+str(i)+s[m.end():])]
        else: out+=exp(s[:m.start()]+a+s[m.end():])
    return out
miss=[]; n=0; stat={}
for sec,rs in rows.items():
    for win,cell,tol in rs:
        n+=1; st=stat.setdefault(sec,[0,0,0,0]); st[0]+=1
        st[1 if tol=='0' else 2 if tol=='0*' else 3]+=1
        if tol=='0':
            for part in re.split(r'、',cell):
                for q0 in re.findall(r'`([^`]*)`',part):
                    if '…' in q0: continue
                    for q in exp(q0):
                        if not os.path.exists(os.path.join(pre[sec]+win,q)): miss.append((win,q))
print("rows=%d inrepo=%d runtime=%d none=%d"%(n,sum(v[1] for v in stat.values()),sum(v[2] for v in stat.values()),sum(v[3] for v in stat.values())))
for sec in sorted(stat): print(sec, stat[sec])
print("missing=%d"%len(miss))
for w,q in miss: print("  MISS",w,q)
PY
```

`0*` 行的 `/tmp` 文件名已对代码逐个核过（2026-10-28），不随仓校验；P1.4 归档到 `baseline/windows/` 后改回 `0` 并纳入脚本。

---

## 7 修订记录

| 日期 | 修订 |
|---|---|
| 2026-10-28 | 首版：162 真窗 + 28 非真窗全量清点；金图 114 仓内 / 8 运行期 / 40 无；能力标签与 PLAN §5/§6 逐项映射；P1.4 采集规则与自校验命令 |
| 2026-10-28 | 复核修正：汇总表基座族 8/12（原误写 10/10）；r1 运行期基线改 `r1_final_base.png`；8 扇 `0*` 补精确 `/tmp` 路径与环境变量；OOM 金图补 `testdata/` 前缀；§6 换为可执行脚本（行数/分族/存在性一次核完，实测 162 行、114/8/40、缺图 0） |
