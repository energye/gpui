# 窗口证据对账（2026-10-21）

> 本文只盘点证据，不改能力状态；能力状态仍以 `docs/RENDER_FEATURE_COVERAGE.md` §1/§2/§3/§5 为准。
> 扫描对象：`examples/**/main.go` 目录；WIN=真窗/窗口入口，OFF=离屏示例。
> 三证据口径：逻辑探针 + 像素断言 + Golden 逐位（`docs/UI_PIXEL_ASSERTION_STANDARD.md`）。抓屏 vs 快照是辅助证据，不替代三件套。

## 0. 先看结论

- **基数钉死**：183 个 `main.go` 目录 = **155 个 WIN + 28 个 OFF**，与总账 §1 的 155/28 一致；此前「计数落后」的判断收回。
- **证据现状**：155 个 WIN 里，**94 扇同时有 Golden + 像素**，**19 扇只有像素**，**42 扇两样都没有**，**0 扇只有 Golden**。
- **缺口**：61 扇缺 Golden（19 + 42），42 扇缺像素；其中真正要补三证据的**画面窗 = 42 扇**（26 扇 neither + 16 扇 pixel-only），另外 **17 扇 E1 控制/平台/性能/行为探针 + 2 扇 E2 动画演示**需要书面豁免或补固定帧。
- **首批**：先补 10 扇（§5），覆盖 4 个 render_base 轴、2 个 graphics 窗、3 个 R 窗、1 个组合窗；其余按 §3/§4 分档排期。
- **抓屏**：只有 20 扇能扫到抓屏信号，其中 16 扇同时有 Golden+像素；78 扇双证窗没有抓屏信号。抓屏是辅助项，但「真窗自测 + 人工二合一」仍要逐窗补。
- **旧说法**：review 的「27 个真窗全为逻辑指标单证」是过期/子集口径，不能当当前全量结论；以本文和总账 §7 更正为准。

## 1. 口径与扫描方法

- `WIN`：目录下有 `main.go` 且入口是窗口/真窗；`OFF`：离屏示例（`render_*` 等）。
- `G`：扫到 `testdata/*_base.png` / `golden` 基线；`P`：扫到像素探针/像素断言；`S`：扫到 `xwd`/`x11grab`/抓屏关键字；`J`：扫到 JSON 输出。
- 这是**静态信号扫描**，不是逐窗运行结果：`P` 命中不代表探针真的断言了「画面对」，`G` 命中也不代表基线是最新；P1 执行时必须逐窗 `-auto-only` 复核并看 JSON 的 `logic_probes_*` / `pixel_probes_*` / `pixel_golden_*`。
- 附表的 `双证` 只表示 Golden+像素两样信号都在；三证据还差逻辑探针这一项，要按窗读 JSON/日志，不能拿 `J` 直接当逻辑证据。

## 2. 总表

| 家族 | 真窗 | Golden+像素 | 仅像素 | 两样都无 | 说明 |
|---|---:|---:|---:|---:|---|
| engine | 22 | 22 | 0 | 0 | 2.5D 窗全部双证 |
| kit | 16 | 15 | 0 | 1 | 组件窗；`spin_min` 是唯一缺口（动画演示） |
| video | 21 | 20 | 0 | 1 | 视频窗；`video_player` 是点击 demo，不是门禁窗 |
| ui | 95 | 37 | 19 | 39 | 主缺口：37 双证 / 19 仅像素 / 39 两样都无 |
| 其他 | 1 | 0 | 0 | 1 | `zz_soak_control` 对照窗 |
| **合计** | **155** | **94** | **19** | **42** | |

补充：Golden 命中 94、像素命中 113（94+19）、JSON 输出 137、README 132、抓屏信号 20；`S` 同时有 G+P 的只有 16 扇。

## 3. 42 扇「pixel + Golden 两样都没有」分档

### 3.1 E1：控制/平台/性能/行为探针（14 扇，无需画面三证据）

| 目录 | 理由 |
|---|---|
| `examples/ui_ime_probe` | IME 平台输入探针；没有独立画面断言目标。 |
| `examples/ui_l1_blank` | L1 空白 clear/present 控制窗；没有内容可断言。 |
| `examples/ui_l1_scroll` | VirtualList 1000 行自动滚动 demo；动画/性能窗，逐位 Golden 不适用，保留 JSON/性能证据。 |
| `examples/ui_l1_spinner` | spinner + JSON 指标；动画/性能门禁，不是画面正确性窗。 |
| `examples/ui_l2_shell` | Tap/Pan、Focus、Overlay 行为 smoke；JSON 行为证据，可见内容少；若要补只补 overlay 像素。 |
| `examples/ui_pf_appkit` | macOS 平台能力探针（事件/输入/窗口），不是渲染画面窗。 |
| `examples/ui_pf_wayland` | Wayland 平台能力探针（事件/输入/窗口），不是渲染画面窗。 |
| `examples/ui_pf_win32` | Win32 平台能力探针（事件/输入/窗口），不是渲染画面窗。 |
| `examples/ui_pf_x11` | X11 平台能力探针（事件/输入/窗口），不是渲染画面窗。 |
| `examples/ui_render_base_perfsoak` | PerfSoak 性能/资源窗；帧率、hitch、RSS、CPU、关闭资源，不做逐位画面。 |
| `examples/ui_text_m4_virtual` | 1e6 行虚拟化日志；JSON/性能窗，滚动内容不适合逐位 Golden。 |
| `examples/ui_wr_r12_metrics` | 帧指标 schema/字段完备窗；行为/字段证据，没有独立画面。 |
| `examples/video/video_player` | 点击播放 demo；README 明确「not a gate window」。 |
| `examples/zz_soak_control` | 单进程三窗小损伤动画对照窗；诊断保留，不是能力窗。 |

### 3.2 E2：动画/演示窗（2 扇，Golden 需冻结帧或书面豁免）

| 目录 | 理由 |
|---|---|
| `examples/kit/spin_min` | 单动态图标 demo/性能控制；若要三证据，必须固定帧 + Golden，否则只能书面豁免。 |
| `examples/ui_render_jumprope` | 2D 协调动画演示；需固定时间/seed 的冻结帧 + Golden，或书面豁免。 |

### 3.3 P1：画面窗缺口（26 扇，要补像素 + Golden + 逻辑）

| 目录 | JSON | 抓屏 | 可见内容/缺口 |
|---|---:|---:|---|
| `examples/ui_l1_render_matrix` | — | — | M1–M5 多轴渲染烟囱；是可见画面 smoke，但优先级低于 render_base/wr 主能力。 |
| `examples/ui_render_base_cliplayer` | — | — | ClipLayer 轴四卡（Boundary / Clip / Opacity / Transform）。 |
| `examples/ui_render_base_geometry` | — | — | Geometry 轴四卡（矩形/圆/圆角/路径/变换）。 |
| `examples/ui_render_base_image` | — | — | Image 轴四卡（采样/变换/裁剪/透明度）。 |
| `examples/ui_render_base_text` | — | — | Text 轴四卡（多语言/字距/描边/换行）。 |
| `examples/ui_render_graphics/basic` | — | — | 光栅基元窗（线/矩形/圆/多边形/混合）。 |
| `examples/ui_render_graphics/vector` | — | — | 2D 矢量路径窗（贝塞尔/虚线/填充规则/变换）。 |
| `examples/ui_text_m5_anycase` | ✅ | — | 多脚本/emoji/组合字符/富文本 run/多 DPR 的文本 close-out 窗。 |
| `examples/ui_wr_c0_smoke` | ✅ | — | W0 C0 组合窗（R0+R12+R16 集成）；组合窗也要独立证据。 |
| `examples/ui_wr_c1_boundary_nest` | ✅ | — | W1 C1 组合窗（嵌套 boundary）。 |
| `examples/ui_wr_c3_list_scroll` | ✅ | — | W3 C3 组合窗（列表滚动）；需确定性冻结帧。 |
| `examples/ui_wr_ime_r1_editor` | ✅ | — | IME R1 Editor 四元组；可见编辑器内容。 |
| `examples/ui_wr_r0_fullpaint` | ✅ | — | R0 FullPaint 正确性。 |
| `examples/ui_wr_r10_async_image` | ✅ | — | R10 图片异步→局部脏。 |
| `examples/ui_wr_r11_dpr` | ✅ | — | R11 DPR / size cache。 |
| `examples/ui_wr_r12b_debug_repaint` | ✅ | — | R12b debug repaint 可视化；debug overlay 本身可像素断言。 |
| `examples/ui_wr_r13_hit` | ✅ | — | R13 命中测试 ≡ 绘制身份；命中是行为，但「≡ 绘制」需要像素身份证据。 |
| `examples/ui_wr_r16_warmup` | ✅ | — | R16 首帧/WarmUp/恢复；首帧内容要像素断言。 |
| `examples/ui_wr_r18_savelayer` | ✅ | ✅ | R18 SaveLayer + budget；已有抓屏信号，缺像素与 Golden。 |
| `examples/ui_wr_r2_paint` | ✅ | — | R2 局部 NeedsPaint 隔离；脏区外像素必须不变。 |
| `examples/ui_wr_r3_boundary` | ✅ | — | R3 嵌套 RepaintBoundary cache。 |
| `examples/ui_wr_r3b_compbits` | ✅ | — | R3b compositing bits / boundary discovery。 |
| `examples/ui_wr_r5_picture` | ✅ | — | R5 Picture record/replay；README 标注当前模式 2 返工，Golden 要等返工稳定后再种。 |
| `examples/ui_wr_r7b_scroll_reuse` | ✅ | — | R7b 滚动少重录 cell；需确定性冻结帧。 |
| `examples/ui_wr_r8_overlay` | ✅ | — | R8 overlay 独立脏带；浮层开/关两态像素。 |
| `examples/ui_wr_r9_text_cache` | ✅ | — | R9 文本 measure cache；缓存命中/失效后的像素一致。 |

## 4. 19 扇「只有像素、没有 Golden」分档

### 4.1 E1：平台/压测探针（3 扇，无需逐位 Golden）

| 目录 | 理由 |
|---|---|
| `examples/ui_pf_x11_device` | X11 设备热插拔平台探针；像素用于设备信息/状态，不做逐位 Golden。 |
| `examples/ui_pf_x11_dnd` | X11 XDND 拖放平台探针；像素用于拖放反馈，不做逐位 Golden。 |
| `examples/ui_wr_t_stress` | T 阶段线程压力窗；性能/稳定性，不是画面正确性。 |

### 4.2 需补 Golden（16 扇）

| 目录 | 抓屏 | 现状/补法 |
|---|---:|---|
| `examples/ui_ant_verify` | ✅ | AA 专项窗（圆角/斜边/细边框），缺 Golden 价值最高；优先补。 |
| `examples/ui_render_particle` | — | 粒子动画演示；需固定 seed/冻结帧才能种 Golden，否则只能书面豁免。 |
| `examples/ui_textinput_ime` | — | IME 测试窗；补 Golden（候选/输入框稳定态）。 |
| `examples/ui_wr_c2_retained_scene` | — | W2 C2 retained 整场景（多 boundary + Picture 回放 + 远离脏点 + damage present）；首批。 |
| `examples/ui_wr_c7_resize_dpr` | — | W2 C7 两次尺寸/DPR 失效波 + 1px 对齐 + 嵌套 boundary；按 1x/2x 两态补 Golden。 |
| `examples/ui_wr_c8_hit_overlay_xf` | — | W4+ C8 变换/浮层/层动画下的命中集成；补 Golden。 |
| `examples/ui_wr_ime_r2_textlayout` | — | IME R2 TextLayout 单一真源；补 Golden。 |
| `examples/ui_wr_ime_r3_channel` | — | IME R3 channel & platform；补 Golden。 |
| `examples/ui_wr_ime_r4_selection` | — | IME R4 selection & editing；补 Golden。 |
| `examples/ui_wr_ime_r5_control` | — | IME R5 control & scroll；补 Golden（稳定态）。 |
| `examples/ui_wr_r17_bg_throttle` | — | R17 前台/后台节流；前台稳定态可种 Golden，后台状态用逻辑。 |
| `examples/ui_wr_r19_snap` | — | R19 1px / device-pixel alignment；Golden 价值高。 |
| `examples/ui_wr_r21_shell` | ✅ | R21 shell/content layering；已有抓屏信号，补 Golden。 |
| `examples/ui_wr_r4b_multidamage` | — | R4b DirtyLayerID + 多 damage；补 Golden。 |
| `examples/ui_wr_r4_composite` | ✅ | R4 层 Composite Present（Retained）；已有抓屏信号，补 Golden。 |
| `examples/ui_wr_r7_virtlist` | — | R7 虚拟化宿主 VirtualList；补 Golden（稳定态）。 |

## 5. P1 首批 10 扇

顺序按「先主能力轴、后组合窗」排；每扇的验收口径见 §5.1。

| # | 目录 | 现状 | 补法 |
|---:|---|---|---|
| 1 | `examples/ui_render_base_text` | 无像素、无 Golden、无 JSON | 四卡多语言/字距/描边/换行；补像素 + 固定字符串 Golden；顺带复核 `MeasureWidth == TextLayout`。 |
| 2 | `examples/ui_render_base_image` | 无像素、无 Golden、无 JSON | 图像轴四卡；补像素 + Golden（采样/变换/裁剪/透明度）。 |
| 3 | `examples/ui_render_base_geometry` | 无像素、无 Golden、无 JSON | 几何轴四卡；补像素 + Golden（矩形/圆/圆角/路径/变换）。 |
| 4 | `examples/ui_render_base_cliplayer` | 无像素、无 Golden、无 JSON | Boundary / Clip / Opacity / Transform 四卡；补像素 + Golden。 |
| 5 | `examples/ui_render_graphics/basic` | 无像素、无 Golden、无 JSON | 光栅基元（线/矩形/圆/多边形/混合）；补像素 + Golden。 |
| 6 | `examples/ui_render_graphics/vector` | 无像素、无 Golden、无 JSON | 2D 路径（贝塞尔/虚线/填充规则/变换）；补像素 + Golden。 |
| 7 | `examples/ui_wr_r2_paint` | 无像素、无 Golden，有 JSON | 局部 NeedsPaint 隔离；补「脏区外不变」像素 + Golden。 |
| 8 | `examples/ui_wr_r5_picture` | 无像素、无 Golden，有 JSON；README 标注返工中 | Picture record/replay；先等模式 2 返工稳定，再补像素 + Golden，禁止提前种基线装绿。 |
| 9 | `examples/ui_wr_r18_savelayer` | 无像素、无 Golden，有 JSON + 抓屏信号 | SaveLayer + budget；已有抓屏信号，补像素 + Golden。 |
| 10 | `examples/ui_wr_c2_retained_scene` | 有像素、无 Golden，有 JSON | 已有像素；补 Golden（多 boundary + Picture 回放 + 远离脏点 + damage present）。 |

### 5.1 单窗验收口径（硬）

1. `-auto-only` 退出码 0，JSON 里 `logic_probes_ok == total`、`pixel_probes_ok == total`、`pixel_golden_diff_pct == 0`。
2. Golden 连跑两次都是 `0.0000%`；首次播种后必须再跑一次，不接受「播种那次算通过」。
3. 有抓屏信号的窗补「抓屏 vs 快照」同区对比，差值为 0（容差按窗声明，不允许无声明放水）。
4. 像素探针必须覆盖「画面对」的形态/颜色/位置/空白，不允许只数墨点；Golden 要盖住指标行，只比内容。
5. 动画窗先定冻结帧（固定时间/seed/手动暂停），否则只能书面豁免，不能拿「跑一次碰巧一样」当 Golden。

## 6. 对总账 §7 的更正

- §7 第 1 条引用的「27 个真窗全为逻辑指标单证」是旧 review 的子集口径，**不是当前 155 扇的全量结论**；按本树实扫，94 扇已有 Golden+像素，19 扇仅像素，42 扇两样都无。U21 仍未清零，缺口按本文 §3/§4 分档。
- §7 第 4 条「金图现状两口径待并表」本次已完成静态并表：A2 金图资产「存在但未带进本树」与 review「逻辑单证」的口径，以本文附录 A + 总账 §11 为准；逐窗复跑仍在 P1。
- 155/28 的计数正确；「计数落后」不再作为缺口。

## 7. 扫描局限

- 只扫关键字，`S` 可能漏掉用自建 helper/外部脚本抓屏的窗；`G` 可能漏掉非 `testdata` 基线；`P` 可能把「打印像素」误当断言。逐窗以 README + `-auto-only` JSON 为准。
- 本次没有逐窗跑 155 扇（只跑了 P0.5 涉及的 4 扇：strokepreset / textlow / textspacing / textstroke）；P1 执行时逐扇跑，不拿静态扫描当门禁。
- 工作区里 `clipop/dither/maskops/textspacing/textstroke/verticesmesh` 的 PNG 是别线未提交改动；本文与 P0.5 回归没有把它们纳入提交清单。

## 附录 A：155 个 WIN 全景（自动扫描信号）

> `G`=Golden 信号，`P`=像素信号，`S`=抓屏信号，`J`=JSON 输出；分档：`双证` / `缺 Golden` / `缺像素+Golden` / `E1` / `E2` / `P1`。

### A.1 engine（22）

| 目录 | G | P | S | J | 分档 |
|---|---:|---:|---:|---:|---|
| `examples/engine/25d_basis` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/anim` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/asset` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/camera` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/dodge` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/fx` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/input` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/light` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/light2d` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/nav` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/particle` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/particles2d` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/physics` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/platformer` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/quad` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/save` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/sprite` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/stage_chase` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/step` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/tex` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/tilemap` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/engine/world` | ✅ | ✅ | — | ✅ | 双证 |

### A.2 kit（16）

| 目录 | G | P | S | J | 分档 |
|---|---:|---:|---:|---:|---|
| `examples/kit/color-model` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/kit/date-engine` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/kit/icon` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/kit/modal-host` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/kit/notice-queue` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/kit/qrcode` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/kit/spin_min` | — | — | — | — | E2 |
| `examples/kit/tour-mask` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/kit/upload` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/kit_f0_content` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/kit_f0_decor` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/kit_f0_interact` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/kit_f0_layout` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/kit_f0_motion` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/kit_f0_scope` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/kit_f0_theme` | ✅ | ✅ | — | ✅ | 双证 |

### A.3 video（21）

| 目录 | G | P | S | J | 分档 |
|---|---:|---:|---:|---:|---|
| `examples/video/video_2k_play` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_4k_play` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_a2_sync` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_a4_sink` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_b1_frag` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_player` | — | — | — | — | E1 |
| `examples/video/video_s8_index` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_vc0_fullplay` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_vc1_seek_fault` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_vc2_soak` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_vc3_embed_extend` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_vr0_demux` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_vr1_params` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_vr2_decode` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_vr3_color` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_vr4_play` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_vr5_seek` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_vr6_fault` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_vr7_perf` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_vr8_embed` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/video/video_vr9_registry` | ✅ | ✅ | — | ✅ | 双证 |

### A.4 ui（95）

| 目录 | G | P | S | J | 分档 |
|---|---:|---:|---:|---:|---|
| `examples/ui_ant_verify` | — | ✅ | ✅ | — | 缺 Golden |
| `examples/ui_ime_probe` | — | — | — | — | E1 |
| `examples/ui_l1_blank` | — | — | — | — | E1 |
| `examples/ui_l1_render_matrix` | — | — | — | — | P1 |
| `examples/ui_l1_scroll` | — | — | — | — | E1 |
| `examples/ui_l1_spinner` | — | — | — | — | E1 |
| `examples/ui_l2_shell` | — | — | — | ✅ | E1 |
| `examples/ui_pf_appkit` | — | — | — | ✅ | E1 |
| `examples/ui_pf_wayland` | — | — | — | ✅ | E1 |
| `examples/ui_pf_win32` | — | — | — | ✅ | E1 |
| `examples/ui_pf_x11` | — | — | — | ✅ | E1 |
| `examples/ui_pf_x11_device` | — | ✅ | — | ✅ | E1 |
| `examples/ui_pf_x11_dnd` | — | ✅ | — | ✅ | E1 |
| `examples/ui_render_base_backdrop` | ✅ | ✅ | ✅ | ✅ | 双证 |
| `examples/ui_render_base_cliplayer` | — | — | — | — | P1 |
| `examples/ui_render_base_clipop` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_render_base_cliprrect` | ✅ | ✅ | ✅ | ✅ | 双证 |
| `examples/ui_render_base_dither` | ✅ | ✅ | ✅ | ✅ | 双证 |
| `examples/ui_render_base_emoji` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_render_base_filterops` | ✅ | ✅ | ✅ | ✅ | 双证 |
| `examples/ui_render_base_geometry` | — | — | — | — | P1 |
| `examples/ui_render_base_image` | — | — | — | — | P1 |
| `examples/ui_render_base_imagepattern` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_render_base_maskops` | ✅ | ✅ | ✅ | ✅ | 双证 |
| `examples/ui_render_base_miscdraw` | ✅ | ✅ | ✅ | ✅ | 双证 |
| `examples/ui_render_base_pathbool` | ✅ | ✅ | ✅ | ✅ | 双证 |
| `examples/ui_render_base_pathlow` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_render_base_pathshape` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_render_base_perfsoak` | — | — | — | ✅ | E1 |
| `examples/ui_render_base_recording` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_render_base_scene` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_render_base_strokepreset` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_render_base_svg` | ✅ | ✅ | ✅ | ✅ | 双证 |
| `examples/ui_render_base_sweepgradient` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_render_base_text` | — | — | — | — | P1 |
| `examples/ui_render_base_textlow` | ✅ | ✅ | ✅ | ✅ | 双证 |
| `examples/ui_render_base_textspacing` | ✅ | ✅ | ✅ | ✅ | 双证 |
| `examples/ui_render_base_textstroke` | ✅ | ✅ | ✅ | ✅ | 双证 |
| `examples/ui_render_base_variablefont` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_render_base_verticesmesh` | ✅ | ✅ | ✅ | ✅ | 双证 |
| `examples/ui_render_base_webp` | ✅ | ✅ | ✅ | ✅ | 双证 |
| `examples/ui_render_graphics/basic` | — | — | — | — | P1 |
| `examples/ui_render_graphics/vector` | — | — | — | — | P1 |
| `examples/ui_render_jumprope` | — | — | — | — | E2 |
| `examples/ui_render_particle` | — | ✅ | — | — | 缺 Golden |
| `examples/ui_render_pelican` | ✅ | ✅ | — | — | 双证 |
| `examples/ui_text_edit_accept` | ✅ | ✅ | ✅ | ✅ | 双证 |
| `examples/ui_text_m4_virtual` | — | — | — | ✅ | E1 |
| `examples/ui_text_m5_anycase` | — | — | — | ✅ | P1 |
| `examples/ui_textinput_ime` | — | ✅ | — | ✅ | 缺 Golden |
| `examples/ui_wr_c0_smoke` | — | — | — | ✅ | P1 |
| `examples/ui_wr_c10_soak` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_wr_c11_policy_switch` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_wr_c1_boundary_nest` | — | — | — | ✅ | P1 |
| `examples/ui_wr_c2_retained_scene` | — | ✅ | — | ✅ | 缺 Golden |
| `examples/ui_wr_c3_list_scroll` | — | — | — | ✅ | P1 |
| `examples/ui_wr_c4_shell_overlay` | ✅ | ✅ | ✅ | ✅ | 双证 |
| `examples/ui_wr_c5_anim_over_static` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_wr_c6_savelayer_group` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_wr_c7_resize_dpr` | — | ✅ | — | ✅ | 缺 Golden |
| `examples/ui_wr_c8_hit_overlay_xf` | — | ✅ | — | ✅ | 缺 Golden |
| `examples/ui_wr_c9_stress_cache` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_wr_ime_r1_editor` | — | — | — | ✅ | P1 |
| `examples/ui_wr_ime_r2_textlayout` | — | ✅ | — | ✅ | 缺 Golden |
| `examples/ui_wr_ime_r3_channel` | — | ✅ | — | ✅ | 缺 Golden |
| `examples/ui_wr_ime_r4_selection` | — | ✅ | — | ✅ | 缺 Golden |
| `examples/ui_wr_ime_r5_control` | — | ✅ | — | ✅ | 缺 Golden |
| `examples/ui_wr_r0_fullpaint` | — | — | — | ✅ | P1 |
| `examples/ui_wr_r10_async_image` | — | — | — | ✅ | P1 |
| `examples/ui_wr_r11_dpr` | — | — | — | ✅ | P1 |
| `examples/ui_wr_r12_metrics` | — | — | — | ✅ | E1 |
| `examples/ui_wr_r12b_debug_repaint` | — | — | — | ✅ | P1 |
| `examples/ui_wr_r13_hit` | — | — | — | ✅ | P1 |
| `examples/ui_wr_r14_cache_budget` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_wr_r15_soak` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_wr_r16_warmup` | — | — | — | ✅ | P1 |
| `examples/ui_wr_r17_bg_throttle` | — | ✅ | — | ✅ | 缺 Golden |
| `examples/ui_wr_r18_savelayer` | — | — | ✅ | ✅ | P1 |
| `examples/ui_wr_r19_snap` | — | ✅ | — | ✅ | 缺 Golden |
| `examples/ui_wr_r1_layout` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_wr_r20_filter` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_wr_r21_shell` | — | ✅ | ✅ | ✅ | 缺 Golden |
| `examples/ui_wr_r22_selection_stub` | ✅ | ✅ | — | ✅ | 双证 |
| `examples/ui_wr_r2_paint` | — | — | — | ✅ | P1 |
| `examples/ui_wr_r3_boundary` | — | — | — | ✅ | P1 |
| `examples/ui_wr_r3b_compbits` | — | — | — | ✅ | P1 |
| `examples/ui_wr_r4_composite` | — | ✅ | ✅ | ✅ | 缺 Golden |
| `examples/ui_wr_r4b_multidamage` | — | ✅ | — | ✅ | 缺 Golden |
| `examples/ui_wr_r5_picture` | — | — | — | ✅ | P1 |
| `examples/ui_wr_r6_layer_anim` | ✅ | ✅ | ✅ | ✅ | 双证 |
| `examples/ui_wr_r7_virtlist` | — | ✅ | — | ✅ | 缺 Golden |
| `examples/ui_wr_r7b_scroll_reuse` | — | — | — | ✅ | P1 |
| `examples/ui_wr_r8_overlay` | — | — | — | ✅ | P1 |
| `examples/ui_wr_r9_text_cache` | — | — | — | ✅ | P1 |
| `examples/ui_wr_t_stress` | — | ✅ | — | ✅ | E1 |

### A.5 其他（1）

| 目录 | G | P | S | J | 分档 |
|---|---:|---:|---:|---:|---|
| `examples/zz_soak_control` | — | — | — | — | E1 |

## 附录 B：28 个 OFF（离屏示例，不算真窗）

| # | 目录 |
|---:|---|
| 1 | `examples/render_basic` |
| 2 | `examples/render_bezier_test` |
| 3 | `examples/render_cjk_text` |
| 4 | `examples/render_clipping` |
| 5 | `examples/render_color_emoji` |
| 6 | `examples/render_gpu` |
| 7 | `examples/render_images` |
| 8 | `examples/render_pixel_perfect` |
| 9 | `examples/render_recording` |
| 10 | `examples/render_scene` |
| 11 | `examples/render_scene_gpu` |
| 12 | `examples/render_sdf` |
| 13 | `examples/render_shapes` |
| 14 | `examples/render_text` |
| 15 | `examples/render_text_fallback` |
| 16 | `examples/render_text_transform` |
| 17 | `examples/render_variable_font` |
| 18 | `examples/render_visual_basic` |
| 19 | `examples/render_visual_cjk_text` |
| 20 | `examples/render_visual_clipping` |
| 21 | `examples/render_visual_gpu` |
| 22 | `examples/render_visual_images` |
| 23 | `examples/render_visual_scene` |
| 24 | `examples/render_visual_scene_gpu` |
| 25 | `examples/render_visual_shapes` |
| 26 | `examples/render_visual_text` |
| 27 | `examples/render_visual_text_transform` |
| 28 | `examples/text_langs_png` |

---

对账人：OpenCode（DeepSeek V4.1 Flash Free）｜日期：2026-10-21｜数据源：`/tmp/opencode/win_evidence.csv`（本次扫描）
