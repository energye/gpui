P3.7 批 6 · 窗 ① 透视批绘（`examples/engine/p37_persp_batch`）交付证据（2026-11-05）
================================================================================

收口能力行：批 1a（透视合批）/ 1b（腿染色 + 采样核）/ 1c（批绘腿吃四角+权重，经引擎出口）/
1d（Canvas 级 4×4 投影层四动词）。四行交付前都是 ☑，交付原因见下「窗内证据」。

一、这扇窗为什么必须新建（不能改用既有窗）
------------------------------------------
`ui_wr_p35_persp` 每张贴透视 quad 后面紧跟一笔 `StrokePath`（H12），图像与路径交错 ⇒ 合并腿
从没在窗内跑过；且它不碰 Tint、不碰图集腿、全仓 `Set/Push/PopProjection` 调用数为 0。
新窗把「合并段」与「夹描边段」画在同一路画面里做正反对照片，并首次消费投影栈。

二、五臂落点与实测（15 枚逻辑探针 + 语义像素 + 双后端金图）
--------------------------------------------------------
| 臂 | 画法 | 实测 |
|---|---|---|
| a 合并段 | 六张同透视 quad 连排，中间只有图像命令 | `merged_run_one_draw`：**PerspDraws=1 / MaxPerspRun=6** |
| a 反面 | 同六张，每张**之前**先画一条落在间隙里的分隔刻度 | `interleaved_path_breaks_run`：**PerspDraws=6 / MaxPerspRun=1** |
| a 语义 | 两种命令序在面板落点区必须逐位相同（合批不改画面） | `merge_keeps_picture_bitwise`：**diff=0 px**（容差写死 0） |
| b 染色 | 同段六张，前三张 `Tint=(1,0.62,0.28)`，后三张零值 | `tint_lights_projective_leg`：tinted=(191,118,54,255) ≡ plain(191)×系数，容差 6 档 |
| c 采样核 | 透视+bicubic 具名拒绝、非法权重具名拒绝、仿射+bicubic 合法、透视+最近邻正常出图 | 三枚探针各就各位（`perspective_bicubic_refused`/`bad_weights_refused`/`affine_bicubic_still_draws`） |
| c 眼后 | 深度压到焦点之后 ⇒ 投影出口整块拒绝（ok=false），该格保持底色 | `behind_eye_refused` + `behind_eye_painted_nothing`（探针点 (420,756) 实测 (26,28,41)≈底色） |
| d 图集腿 | `camera.NewProjector` + `ProjectSpriteQuad` 产四角与权重 → **DrawAtlasEx**（老口按批 1c 的 D-4 契约不读精灵自带透视） | `atlas_engine_leg_merges`：**PerspDraws=1 / MaxPerspRun=5 / Drawn=5 / Degraded=false** |
| e 投影层 | `PushProjection(Matrix4)` + `ClipRoundRect` 画远处告示牌，`Pop` 后 `Projection()` 回恒等 | `projection_stack_push_restore`：**pushed=true / restored=true**，附 `ProjectionStats` 读数 |
| e 附加 fail-closed | 同一段里投影态下多写一笔文字、`Pop` 之后的恒等段再写同一串字（2026-11-06 补，见 §七） | `text_under_projection_failclosed`：**投影段墨差=0**（整笔缺席 + `TextSkips++`）、**恒等段墨差>40**（同一串字在场） |
| 等权≡仿射 | 同图同角，`Perspective:true + Weights 全 1` vs `Perspective:false` | `equal_weights_equal_affine`：**diff=0 px** |
| 真透视≠仿射 | 梯形权重参与 1/w 有理插值 | `honest_perspective_differs`：**diff>0 px** |
| 引擎出口 fail-closed | 深度给 NaN | `engine_export_rejects_nonfinite`：**ok=false** |
| 窗内合并（H12） | 逐帧从活 Context 读合批账再清零 | `window_frame_max_run=7 ≥ 2` 且 `window_leg_merged=true`（门禁硬断言，不达标直接 exit 1） |

三、H15（末帧可复现）与 H16（金图有牙齿）
------------------------------------------
H15 做法沿用 game_fx：门禁相 `MaxFrames=240` 定帧停机 + 相位只随已完成帧数走 + 过
`goldenFreezeAt=120` 定格；常驻相不限帧。实测：

- 门禁相两轮：`golden=0.0000% / 921600 px`（`presents=240` 与 `241` 两类只差在 HUD 文案行，
  指标带 y<32 被 mask 掉，比对区零差）；
- 常驻相（9s）：与冻结基线差 **101473 px** ⇒ 画面真在动，定格才是可复现的原因；
- H16-d 把 `goldenFreezeAt` 120→90：金图立刻报 **1.8793%** ⇒ 相位确实进画面。

H16 四处变异（各自还原复绿）：

| 变异 | 结果 |
|---|---|
| H16-a 焦距 520→560（所有投影落点动） | 金图 **5.7130%** |
| H16-b 合并段改走仿射腿 | `merged_run_one_draw` 红（draws=0 max_run=0） |
| H16-c 染色 G 0.62→0.45 | `tint_lights_projective_leg` 红（tinted=(189,85,53) want (189,117,53)） |
| H16-d 定格帧 120→90 | 金图 **1.8793%** |
| H16-e 恒等段那笔文字挪到 `Pop` 之前（= 也画在投影下） | `text_under_projection_failclosed` 红（`text_skips=2`、恒等段墨差 0 want>40）+ 金图 **0.0966%** |

双后端：`native` 与 `GPUI_BACKEND=go`（纯 Go GL）各播种一次并各复验 `0.0000%`；
两张基线互比 **整窗零差（含指标带）** ⇒ 同一份合并腿与同一份投影数学在两后端上出同一张图。

四、本轮抓到并修掉的两个自身缺陷（如实记，不当引擎洞）
------------------------------------------------------
1. **只画一帧的静窗**：`content.OnPaint` 里写了 `pc.DC.(*render.Context)` 断言、取不到就 return。
   生产路径每帧是 `recordFullFrame` 录进 `*recording.Recorder` 再回放，只有 warm-up 那一笔是
   `*render.Context` ⇒ 第二帧起整窗静止，合批账也停在 warm-up。改为：绘制一律走 `render.Canvas`
   （投影四动词/图集 Ex/裁剪都在其上），活 Context 只用于读合批账（`ImageBatchStats` 不在 Canvas 上）。
   修后实测：常驻相与冻结相逐位不同（动画回来了），窗内合批读数来自真回放帧。
2. **`plate()` 把屏幕落点同时当投影中心** ⇒ 移动 cx 只是移动坐标系原点，画面不动
   （放大振幅到 180 才在比对区看出 10898 px，且只有图集带在动）。改为固定视口中心、
   按中段深度把屏幕落点折回世界系（与图集腿同一把尺子）。

五、还欠的
----------
- Metal 后端本机不可验，按跨平台标记纪律记「待验」；
- 3e-1/3e-2/3e-3（scene 投影口 / quad+权重通道 / item 级材质编码）**不在本窗收口**：
  本窗是 engine 窗，而批 3e-2 已把「engine/、ui/ 对 `render/scene` 零引用」钉成结构约束
  ⇒ 这三行只能由 scene 侧窗（`examples/ui_render_base_scene`）收口，归属修正见
  `batch6-window-plan.md` §5；
- 帧耗时/p95 类性能门按同日拍板不排在批 6 之前（只报数不判绿）。

六、门禁账与配套证据
--------------------
- 门禁逐项（build/vet/gofmt/逐文件 408 文件/apidoc/`-race`/脚本口径）：`window-1-gates.md`。
  触及 5 包 **387 PASS / 6 FAIL / 15 SKIP_TAGGED**，6 条逐条对在册红名单，零新增红；
  apidoc rc=0（新口入 §3.17，计数按编译视图重测 157 类型 / 228 Context 方法）。
- 原始跑数（每轮命令、每轮金图百分比、JSON `ability_extra` 全块、播种即非过、双后端互比、
  四处变异的原始行）：`window-1-runs.md`。
- **新洞 H19 由本窗收尾时挖出并登记**（整包 `go test ./render` 跑到 `TestP37ProjectionCounters`
  被 wgpu-native 硬中止；单测隔离绿、回退 HEAD 复现 ⇒ 存量洞，与本窗无关；连带回收批 5
  「整包 `-race` rc=0」那条管道假绿）。用户拍板：**先修 H19，再开 ②**。
- 两枚提交：交付枚 `6f6d3d92`（窗 + 合批账本公开口 + 测试 + 目录 §3.17），簿记枚=本文件所在提交。

七、2026-11-06 补证：投影下文字 fail-closed 臂（能力×真窗完备性审计 缺口 1）
----------------------------------------------------------------------------
**为什么补**：批 6 收官后做「每条能力都有真窗吗」的全仓审计（结论见台账 §7 补证行），
`render` 的文字腿在投影层之下的行为——**整笔拒画 + `TextSkips` 计数、且 `Pop` 后同一串字必须
正常在场**——四扇窗里没有任何一扇验过：`p37_persp_batch` 的臂 e 只画 quad，全仓 `PushProjection`
调用点里没有一笔 `DrawString`。这是一条「框架实现了、窗没用上」的真缺口。引擎侧并不缺底：
`render/p37_projection_stances_test.go:499` 把「投影下 0 像素 + `TextSkips` +1」逐入口钉到文字族
九个公开动词、`render/p37_projection_test.go:505` 另有离屏导出对照——按 AGENTS 口径这些都只是
**原子验证**，缺的是**呈现帧上的真窗证据 + 双后端逐位金图**这一枚。

**臂的形态**：`paintProjectionLayer(dc, tex, drawText)` 在 `PushProjection` + `ClipRoundRect`
段内多写一笔「投影下文字应整笔缺席」，`Pop` 之后的恒等段再写同一串「恒等段文字在场」；
探针从两枚离屏 Context（`drawText=true` 的投影段 / 控制段的恒等段）各取一条 210×22 的条带，
与不带字的同帧图逐像素比墨差：投影段条带 **必须 0**（配合 `TextSkips ≥ 1` 才叫"拒画"而非"碰巧没墨"），
恒等段条带 **必须 >40**（同一串字在同一坐标尺子下正常在场）。字体取
`text.LoadMultiFace(16)` → `rendering.TryLoadDefaultFace(16)` 兜底，两笔都缺席即探针红，不静默放行。

**实测（native）**：`text_skips=1`、投影段墨差 `0`、恒等段墨差 `826` ⇒ `text_under_projection_failclosed` 绿，
15/15 探针全绿。

**金图重录（既有基线必然报警，按删-播-验流程重录）**：先 `rm` 两枚基线，`GPUI_ACCEPT_GOLDEN=1`
各自播种（播种跑本身判红 `rc=1`，不算过），再各复验：native `0.0000%`、go `0.0000%`，
另加两轮 native 稳定轮 `0.0000% / 0.0000%`，两张基线互比 **整窗零差**。原始行：`/tmp/w1_batch.log`。

**H16 有牙齿**：见上表 H16-e —— 把恒等段那笔文字挪进投影段，探针与金图同时报警
（`text_skips=2`、恒等段墨差 0、金图 0.0966%），还原后复绿 15/15 / `0.0000%`。

**本窗探针数 14 → 15**（README、§二 表、门禁 JSON `probes=15/15` 同步）。
