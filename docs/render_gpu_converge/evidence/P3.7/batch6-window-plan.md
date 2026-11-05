# P3.7 批 6 · 五扇真窗收口清单（2026-11-05 钉版，用户拍板「先钉批 6 窗清单」）

名字与硬要求取自 `design.md` §7（五扇表 + 三证据 + 金图分后端 + 双后端一轮一窗）；
本文件的增量是把**批 1–3 的 13 行 ☑ 逐行指派收口窗**，并写清每扇窗**必须画到哪一臂**才算像素收口
（口径见 `tasks.md` §4.1：✅=新能力本身画面被窗金图/探针断言过；「存量零变化」不算收口）。

前置事实（全部实测，出处 `doc-audit-20261105.md`）：13 行 ☑ 之所以是 ☑，是因为这些臂在现有任何窗里
都没被画过或没被测过——`SetProjection`/`PushProjection`/`.Material`/`SetImageAt`/`Scene.DrawImageQuad`
在 `render/` 外调用数均为 0，`ProjectSpriteQuad` 仅定义+单测，全仓唯一画透视 quad 的
`ui_wr_p35_persp` 每张 quad 后紧跟一笔 `StrokePath`（main.go:562,586）⇒ 合批路从未在窗内发生（H12）。

---

## 1 五扇窗 × 必画臂 × 必带探针

| 窗（归位 `examples/engine/`） | 收口哪几行 ☑ | 必须画到的臂（缺一即仍 ☑） | 必带探针（JSON 里要有键） | 已知结构障碍 / 注意事项 |
|---|---|---|---|---|
| **① 透视批绘窗** | 批 1a、1b、1c、1d | (a) **同段连续 ≥2 张同透视 quad，中间不夹任何路径/图层操作**（H1a/H12 硬要求）；(b) `QuadDrawOptions.Tint` 非零值（1b）；(c) bicubic 透视采样（1b，现有窗全 Nearest）；(d) `AtlasDrawOptions` 走批绘腿吃四角+权重（1c，经 `camera.ProjectSpriteQuad` 出口）；(e) `SetProjection`/`PushProjection`/`PopProjection` 栈（1d，现全仓零调用） | `draw_call`/`uniform_slab` 计数（`LastImageBatchStats`，须断言 N 张一段 → 1 次）、透视校正 vs 仿射并排差值、眼后整块拒绝计数、投影栈层级与链序 | 现 `ui_wr_p35_persp` **不能**直接改用来收口——它的 per-quad 描边天然阻断合批；新窗必须把描边挪到独立图层/独立段，或另设「纯 quad 段」 |
| **② 高度与 z 序窗** | 批 2 | (a) `YSort25D` z 桶分组与 ±4096 CLAMP；(b) 层内键按 `is_equal_approx` 判等 + DFS 破平票（平票不闪需两轮帧对比断言）；(c) `Transform.Height`/世界高度参与的盖序变化；(d) 非有限输入 fail-closed；(e) 跨 tier 提交序=盖序不变式 | 盖序随高度与 y 变的断言值、平票稳定（连续 N 帧同一顺序）、`LandShadow` 落点、非有限拒绝计数 | 批 2 当时的五窗证据全是 `changed=0`（无回归），**没有任何一帧断言过新盖序**；且 `engine/world/cull_test.go` 的 `TestCullBudgetLarge` 是浮动计时门（整包红、逐文件绿），本窗不得拿它当收口证据 |
| **③ 材质窗** | 批 3a、3b、3c、3d、3e-3 | (a) 精灵批 + 透视腿挂 `Material`（3b 三入口）；(b) **半透明（alpha<255）材质填充**——现 p32 两卡 alpha=1，premul 修复的画面从未出现在任何窗（3a）；(c) `SetImageAt` 多纹理对第二张参与出图（3c，现零调用者）；(d) **描边吃 shader**（3d，现 p32 全文无 Stroke，Δ≥60 只有离屏钉）；(e) `Scene.SetMaterial` item 级粘滞经 scene 流出图（3e-3，现零调用者）；(f) 坏 WGSL 具名拒绝态在窗内可见（3d 七哨兵） | GPU op 数不随材质张数线性炸（无读回断言）、`MaterialSkips`/`ShaderBackendFails` 计数、恒等材质 ≡ 无材质逐位、描边 shader 上下沿 Δ 断言、`ErrQuadMaterialUnsupported` 拒绝计数 | 这是唯一能验 `ErrShaderBackendCompile` 真触发（gles 腿首绘）的载体（批 3d 已登记「真触发随批 6 材质窗 gles 腿承载」）；双后端各一轮，metal 本机不可验按标记写明 |
| **④ mip 出口窗** | 批 4（未开工） | 引擎出口档位映射：`tex.Filter`↔`sprite.AtlasFilter`→`DrawImageOptions.Mipmap`（批 4 交付后才存在） | 采样键对账（段间字节差）、档位→屏幕变化、**摘掉传递必须回退档**（负向） | 依赖批 4 先行；形态承接 `ui_wr_p36_mipmap` 三档 |
| **⑤ 泛光窗** | 批 5（未开工） | 阈值/半径/强度扫参、亮部提取→降采样模糊→叠加合成链（RGBA8 近似） | 亮部区域标准差与能量、**显式 LDR 声明探针**（三处声明「非 HDR」）、fps 与 p95 | 依赖批 5 先行；开工前须按完整流程先修 M6（`ExportImageBuf` R/B 互换） |

## 2 ☑ 行 → 收口窗 反查表（确保无一行无归属）

| §7 行 | 批 | 收口窗 | 备注 |
|---|---|---|---|
| 739 批 1a 透视合批 | 1a | ① | H12 主要承接者 |
| 740 批 1b 腿染色 | 1b | ① | tint 与 bicubic 两条臂 |
| 741 批 1c 批绘吃四角+权重 | 1c | ① | 经 `ProjectSpriteQuad` 引擎出口 |
| 742 批 1d Canvas 4×4 投影层 | 1d | ① | 全仓零调用，必须由窗首次消费 |
| 743 批 2 z 序与世界高度 | 2 | ② | 五窗十轮全是无回归证据 |
| 744 批 3 开工（探针） | — | 过程项 | 无需像素收口 |
| 745 批 3e-1 scene 投影口 | 3e-1 | ① 或 ③（scene 流） | `Scene.SetProjection` 走 scene 流，建议随 ① 的 scene 段 |
| 746 批 3e-2 quad+权重通道 | 3e-2 | ① | `Scene.DrawImageQuad` 目前无窗调用 |
| 747 批 3a 材质骨架+premul | 3a | ③ | 必须有 alpha<255 的材质填充 |
| 748 批 3b 三入口挂材质 | 3b | ③ | 三入口各画一臂 |
| 749 批 3c 多纹理与预算 | 3c | ③ | `SetImageAt` 第二张参与出图 |
| 750 批 3d 描边洞+七哨兵 | 3d | ③ | 描边 shader + 具名拒绝态 |
| 751 批 3e-3 item 级材质编码 | 3e-3 | ③ | `Scene.SetMaterial` 经 scene 流出图 |
| 752 批 3 收尾 | — | 过程项 | 观测量非画面 |
| 753 文档纠偏+fail-closed | — | 过程项 | 本轮已把四窗 fail-open 修成 fail-closed |
| 754 口径落地 | — | 过程项 | 两态口径本身 |

⇒ **能力行 11 条全部有归属**（①4 条 + ②1 条 + ③5 条 + ①/③共享 1 条），过程行 5 条不需要像素收口。

## 3 每扇窗的统一硬要求（缺一项即不得改回 ✅）

1. 三证据合证：JSON 逻辑探针 + 语义像素断言 + Golden 逐位；容差按 P0.1 默认 0 并**显式声明**。
2. 金图按后端分文件 `_native` / `_go`；**fail-closed 已是标准**（本轮四窗已改：缺基线判红，
   显式 `GPUI_ACCEPT_GOLDEN=1` 播种那一跑也不算过）——新窗必须沿用 `wrkit.GoldenGate`，禁止自写播种即绿。
3. 双后端各一轮；`examples/engine/` 归位；`-auto-only` 自动档 + 常驻人工窗二合一（一次性验收窗不代替人工窗）。
4. 像素断言坐标从「画面所用同一份投影/材质输出」反推，不从屏幕反推结论（P3.5 教训）。
5. 浮动计时门（`engine/world cull`、`s6_9 heavy budget`）**一律不作收口证据**；若窗要带性能门，
   必须先重定标（同日基线 + 容差声明），否则只报数不判绿。
6. 收口后逐行把 §7 状态从 ☑ 改回 ✅，并附窗名 + 差异值 + 提交号；未收口的行不许留在 ✅。

## 4 开工顺序建议（待拍板）

①③② 三扇不依赖未开工批次，可先行；④ 依赖批 4、⑤ 依赖批 5（且 ⑤ 依赖 M6 先修）。
H8 已随本轮扩容解决（engine/video 进门禁）；剩余待拍板：**性能门重定标**要不要排在批 6 之前。

---

## 5 交付进展与一处归属修正（2026-11-05）

**① 透视批绘窗已交付**：`examples/engine/p37_persp_batch`（14 枚逻辑探针 + 语义像素 + 双后端
`_native`/`_go` 金图逐位 0.0000%，H15/H16 各有实测正反面），证据
`batch6/window-1-persp-batch.md`。§2 反查表里 739/740/741/742 四行（批 1a/1b/1c/1d）
因此从 ☑ 改回 ✅。为拿窗内证据另加一枚渲染层观测口 `Context.ImageBatchStats`（见目录 §3.17），
因为 H12 要的「N 张一段 → 1 次 draw」在 `render/internal/gpu` 外读不到。

**归属修正（不照 §2 原表执行）**：3e-1（scene 4×4 投影口）、3e-2（scene 四角+权重 quad 通道）、
3e-3（scene item 级材质编码）三行原写「① 或 ③（scene 流）」——**engine 窗收不了它们**。
批 3e-2 已把「`engine/`、`ui/` 对 `render/scene` 零引用」钉成结构约束（Godot 2D 命令面不带投影参数，
2.5D 走的是引擎自己算好再喂），所以这三行只能由 scene 侧窗 `examples/ui_render_base_scene`
（或另立 scene 真窗）收口。本批把它列为**第 ⑥ 扇**（scene 流收口窗），不硬塞进 ①/③。

**第 ⑥ 扇范围**：`Scene.SetProjection/PushProjection/PopProjection` 出图、`Scene.DrawImageQuad`
（梯形 + 权重）出图、`Scene.SetMaterial` item 级粘滞出图 + `MaterialSkips` 计数可见 +
三条既有 3e 钉的窗内像素版；三证据与 fail-closed 金图同 §3。

---

**② 高度与 z 序窗已交付**（2026-11-06，交付枚见 §7 批 6 ② 行）：`examples/engine/p37_height_zsort`
五臂一屏全部新建载体，22 枚逻辑/像素探针 + 双后端金图逐位 0.0000%（两基线逐位相同）+
H15 六轮恒 0 + H16 四变异三重报警（0.25%/0.31%/0.96%/1.11%），证据 `batch6/window-2-height-zsort.md`。
§2 反查表 743 行（批 2）☑→✅。零新增公开 API。

## 6 ① 的门禁账与新排的队（2026-11-05）

**① 已按 §3 六条硬要求跑齐**（逐条落点见 `batch6/window-1-persp-batch.md`，原始跑数见
`batch6/window-1-runs.md`，门禁见 `batch6/window-1-gates.md`）：三证据合证且容差显式写死
（合批≡拆开 `diff=0`、染色 6 档）、金图按后端分文件 `_native`/`_go` 且 fail-closed（缺基线红、
播种不算过，h15 三轮实测留档）、双后端各一轮且两基线互比整窗零差、`examples/engine/` 归位、
`-auto-only` 门禁相 + `-manual-seconds` 常驻人工相二合一、像素坐标从窗内同一份投影输出反推。
逐文件回归触及 5 包 408 文件 = 387 PASS / 6 FAIL（6/6 在册零新增红）/ 15 SKIP_TAGGED；
apidoc rc=0；build/vet rc=0（按「先落文件再取 `$?`」复测）。

**队变了（用户拍板）**：批 6 ① 收尾时挖出 **H19**——整包 `go test ./render` 跑到
`TestP37ProjectionCounters` 被 wgpu-native `invalid bind group layout` 非展开 panic 硬中止
（带不带 `-race` 都一样、`DATA RACE` 零命中、单测隔离绿、移开本批改动作回退 HEAD 复现 ⇒ 存量洞，
定位 `gpu_shared.go:863` → `sdf_render.go:326/373-379`）。它让「整包 `-race`」这道门在本仓不可用，
而逐文件口径看不见它。**先修 H19（资源生命周期线，走完整修复流程：影响面评估 + 复现钉 +
对齐 Skia/Flutter 共享句柄释放口径 + 停下报告节点），修完再开 ②**。

**③ 材质窗的范围补一句**：§1 表里 3d 那臂（描边吃 shader + 七枚具名拒绝态 + `ErrShaderBackendCompile`
gles 真触发）**代码侧早已交付**（`bfdd853d`，台账 750 行是 ☑），③ 窗只欠把它的画面断出来并跑 gles 腿，
不是重新实现；目录 §13 那句「⬜ 未开工：3d 描边洞+哨兵」的陈旧措辞已随本批修正。

**待跑（批 6 收尾统一补，口径按批 3 收尾登记）**：五目标交叉构建 {默认,nogpu} 10 段、
64 包全量 sweep 对在册只减不增、conformance 双后端一轮，并回填 §7 与 H19 修复后的整包 `-race`。
