# R1 · widget 应用层 + 测试验收体系诚实性审查

> 审查人：QA 架构师（独立审查线）。日期：2026-08-22。
> 方法：快审 `ui/` 主要链路 + 抽查 §2/§3 标 ✅ 条目的真窗与门禁代码 + `go vet`。未跑全量 test。

## 一句直话

**这套「文档 + 真窗 + JSON 门禁」体系基本诚实可信——26 个 `ui_wr_*` 目录全部有 main.go，抽查 10 条 ✅ 无一造假，指标是真实测量、除零有保护；但它有两个软肋：① C2 的 damage 门禁用的是「按场景公式估算的 sumRatio」而不是实测像素和，属于半自证；② fps 门禁用平均帧间隔推 fps，长跑里几帧超长卡顿会被平均值稀释。widget 层离「工业级组件库」还差一层：手势竞技场没接进统一事件路由主链路（只有 scrollable 在自己接）、InputRouter 命中只派发给单个目标而非整条命中路径、仓库里还躺着编译不过的 tmp 目录拖脏 `go vet`。**

---

## A. ui/ 应用层快审

### A1. 事件 → 手势 → 重绘链路

- 链路骨架完整：platform.Event → `ui/embedder/input_router.go:96 RoutePlatform` → `input.FromPlatform` 归一化 → 按 Kind 分派 pointer/key/text/IME；pointer 走 hit-test 后回调 `OnPointer`，重绘靠 `ScheduleFrame`（`ui/application/app.go:343`）进入 demand-driven 帧循环。
- **问题 P2：InputRouter 只把事件给「命中的那一个」目标**。`ui/embedder/input_router.go:139-149` 里 `band`/`entry` 被 `_ =` 显式丢弃，注释宣称「Deliver to every control on the hit path」，实际只对 `target` 单点调 `OnPointer`——没有父链冒泡（Flutter 的 hit path 是一条祖先链逐个分发），滚动容器套按钮这类组合会漏收 move/up 事件。目前靠 `OnPointer` 回调手工补，示例能跑但架构上没闭环。
- **问题 P2：手势竞技场是「孤岛」**。`ui/gestures/arena.go`（Flutter GestureArena 子集，实现质量不错）+ `dispatcher.go` 存在且 `ui/rendering/scrollable.go:49-68` 在用 PanGestureRecognizer，但 InputRouter 主链路完全不认识 arena——每个想参与手势的组件要自己 new Manager。工业级组件库要求 tap/pan/long-press 在同一次 down 上公平竞争，现在做不到全局仲裁。
- application 层（`ui/application/app.go`，382 行）职责干净：NewWindow/SetRoot/SetInput/ScheduleFrame 都是薄封装，无越权逻辑。✅

### A2. 调度器帧率控制

- `ui/scheduler/scheduler.go` 是 Flutter 式三态（Idle/Transient/Persistent）+ VsyncWaiter 回调模型：监听协程只盖时间戳不排帧，需求全来自事件/ticker（on-demand 渲染）；vblank 挂死时时间戳过期自动回落软件间隔（`scheduler.go:30-60` 注释与实现一致）。
- FrameDue 区分快 vsync 源（120Hz 每 stamp 出一帧）与慢/批量源，防过渲染。设计对齐 Flutter，✅。
- 小瑕疵 P3：`DefaultAnimTick=16ms` 硬编码，高刷屏软件回退档会锁 60fps（有 vsync 时不受影响）。

### A3. 软件/GPU 路径一致性

- 路由可观测性做得好：`render/context.go:143-171 RenderPathStats` 记 gpu_ops / cpu_fallback_ops / 最后一次回退原因，并接入 FrameMetrics JSON——GPU 偷偷回退 CPU 是藏不住的，这点诚实。
- `render/software.go`（1510 行）是解析式 AA 扫描线光栅器，带 tile 光栅器自适应分档（`shouldUseTileRasterizer` software.go:228）和 clip/mask coverage 合成，不是玩具；pixmap/pixmap_pool 有 GenerationID 脏标记和池化。
- **问题 P3：一致性没有自动对照门禁**。同一场景 GPU 输出 vs SoftwareRenderer 输出的像素差对照只在零散测试里有，没有纳入 wr 真窗门禁族；CPU 回退时画质是否等价只能靠人眼。
- **问题 P3：仓库垃圾拖累构建面**。`tmp_vlprobe/main.go` 编译不过（引用不存在的 `rendering.PipelineOwner.LayerCache`）、`render/tmp_ref_stroke/wind/sqrtest.go` main 重定义——见 go vet 结果。临时探针目录不该留在包路径里。

---

## B. 测试与验收体系诚实性（重点）

### B1. 真窗存在性核查（§2 主表 R / §3 组合表 C 抽查）

统计：`examples/ui_wr_*` 共 **26 个目录，全部含 main.go（0 个空目录）**。

抽查 10 条标 ✅ 条目逐一对照：

| 文档条目 | example 目录 | main.go | 结论 |
|---|---|---|---|
| §2 R0 ✅ `ui_wr_r0_fullpaint` | 有 | 有 | ✅ 属实 |
| §2 R2 ✅ `ui_wr_r2_paint` | 有 | 有 | ✅ 属实 |
| §2 R3 ✅ `ui_wr_r3_boundary` | 有 | 有 | ✅ 属实 |
| §2 R4b ✅ `ui_wr_r4b_multidamage` | 有 | 有 | ✅ 属实 |
| §2 R5 ✅v2 `ui_wr_r5_picture` | 有 | 有 | ✅ 属实 |
| §2 R7 ✅ `ui_wr_r7_virtlist` | 有 | 有 | ✅ 属实 |
| §2 R9 ✅ `ui_wr_r9_text_cache` | 有 | 有 | ✅ 属实 |
| §2 R21 ✅ `ui_wr_r21_shell` | 有 | 有 | ✅ 属实 |
| §3 C3 ✅ `ui_wr_c3_list_scroll` | 有 | 有 | ✅ 属实 |
| §3 C7 ✅ `ui_wr_c7_resize_dpr` | 有 | 有 | ✅ 属实 |

未标 ✅ 的 R14/R15/R17/R20/R6/C5 等 ⬜ 条目确实没有对应目录——**状态标注与磁盘事实一致，没有发现空目录充数或虚标 ✅**。§10 修订表的关窗记录写得极细（连 FAIL 教训、union vs sum 口径分歧都如实写），这是好信号。

### B2. 指标 JSON 产出代码假绿嫌疑审查

看了三处产出代码：

**① `ui/scheduler/metrics.go`（FrameMetrics/MetricsStore）— 干净。**
- 除零全部有保护：avg interval 用 `intervalN` 计数后才除（metrics.go:214）；percentiles 空 ring 返回 0（metrics.go:592）；hitch rate 对 elapsed≤1e-9 显式归 0 不出 inf（metrics.go:629）；pathCPU 对 total≤1e-12 归 0（metrics.go:333）。
- 没有任何固定值/常量填充；所有字段来自 NoteXxx 实测采样。p50/p95/p99 是真排序（插入排序 n≤256，够用）。

**② `examples/wrgate/report.go` BuildReport — 基本干净，两处小嫌疑。**
- 除零保护齐：elapsed clamp 到 ≥0.001（report.go:95-98）、AvgFrameIntervalMs>1e-6 才算 fpsInterval、SurfaceAreaPx≤0 时 damage_ratio=0。
- **嫌疑 1（P2）：`FPSWall = PresentCount/ElapsedSec`（report.go:99）**，present 次数当帧数在 idle 窗口会把「偶尔 present」美化成高 fps；好在各窗硬门禁实际用的是 fps_interval，此字段仅参考。
- **嫌疑 2（P3）：report.go:169-172 一段空的 if 体**——注释说"可能合法为零"，判断后什么都不做，是写了一半的诚实性检查（cpu_unavailable 从未置位），应删或补完。
- VSyncSource 为空填 "unknown"、RSS 三零显式标 RSSUnavailable——不可得就承认不可得，方向正确。

**③ `examples/ui_wr_c2_retained_scene/main.go` — 发现本次审查最大问题。**
- **P1：damage_ratio_sum 门禁值是公式估算，不是实测**。`sumRatio()`（main.go:63-66）直接写死场景几何常数：`100²+3·hotSize²+2·picBox²+1200×72` 再除以窗口面积——即「我设计的场景最坏该多重」的理论值拿去过 ≤0.35 的门禁，而真正实测的 union bbox 口径（main.go:366-368, 404-406 由 `app.DamageStats()` 采样）只作为观测展示。这不算凭空造假（注释坦白了口径、引擎层 frame.go 的 sum-of-rects 决策逻辑是真的），但**门禁数字不来自测量，改场景参数门禁自动跟着变绿**——这正是「降难度凑门禁」的温和形态。修法：从 render/frame.go 把每帧 rects 面积和实测出来喂给 ability_extra。
- 对比之下 R7（virtlist）的门禁全是实测硬断言：bind_count/item_count 来自 MetricsStore、fps<55 exit(1)、p95>22 exit(1)、hitch 与 rss_slope 超预算 exit(1)（main.go:300-347），vsync 缺失也 FAIL 且 fallback 必须如实标注——这个窗的诚实性无可挑剔。
- **P3：fps 口径偏乐观（体系性）**。fps_interval = 1000/AvgFrameIntervalMs（r7 main.go:298-299 同款），平均值会被长尾稀释：600 帧 16.7ms 里混 10 帧 100ms 卡顿，均值仍 ≈17.9ms→55.9fps 过线。建议门禁改用 p95 或 max 双判（p95 已有，max 未进门禁）。

### B3. `go vet ./...`

```
vet: render/tmp_ref_stroke/wind/sqrtest.go:10:6: main redeclared in this block
vet: tmp_vlprobe/main.go:76:14: owner.LayerCache undefined (PipelineOwner 无此方法)
ui/platform/*: 19 处 possible misuse of unsafe.Pointer（wayland 各文件）
ui/rendering/text.go:592:4: self-assignment of s to s
```
- P2：两个 tmp 目录直接让 `go vet ./...` 非绿，CI 若跑 vet 必红。
- P3：wayland 平台层的 unsafe.Pointer 大多是 syscall 参数转换惯例，属已知风格问题；`text.go:592 self-assignment` 是真 bug 痕迹（ellipsis 二分前忘了剥旧省略号，行为碰巧被后续循环兜住）。

---

## 两个问题的回答

**Q1：这套「文档+真窗+JSON 门禁」的验收体系是否诚实可信？**

**基本诚实，可信度中上（B+/A−）。** 正面证据：26 个真窗全部实存且有 main.go，抽查 10 条 ✅ 与磁盘完全一致；⬜ 条目老实不建目录；指标层除零保护齐全、无固定值造假；不可得指标显式标 unavailable/unknown 而非填默认值；修订表连失败教训都记录。扣分项：C2 的 damage 门禁用设计公式替代实测（P1，唯一实质性折扣）；fps 平均值口径对长尾卡顿偏宽容；wrgate 有一段写一半的死检查。结论：体系骨架是真验收，个别门禁口径需要从「自称」升级为「实测」才算全闭环。

**Q2：widget 层能否支撑工业级组件库？**

**地基合格，上层建筑缺两根梁，暂不能。** 已具备：demand-driven 调度器对齐 Flutter、hit≡绘有 11 个 RO 的 HitTest、虚拟列表/边界缓存/damage present 等性能件齐全且有真窗背书。缺的：① 手势系统未进主路由——竞技场孤岛意味着多手势组件（可拖拽卡片、嵌套滚动）无法正确仲裁，这是工业组件库的硬门槛；② InputRouter 只发单目标不发命中路径，事件冒泡/捕获缺失，复合组件（下拉框=按钮+浮层+滚动）接不起来；③ 组件库本体尚薄（rendering 里以基础 RO 为主，无成熟 Selection/TextField/Table 级控件）。先把 P1/P2 修掉再谈组件库扩张。

---

## 评分表

| 维度 | 得分 | 说明 |
|---|---|---|
| 正确性 | 7.5/10 | 链路可用、单测覆盖广；扣分：事件只发单目标、手势孤岛、text.go 自赋值痕迹 |
| 性能 | 8.5/10 | 调度器/on-demand 渲染/damage present/缓存淘汰都对齐 Flutter，真窗实测数据扎实 |
| 资源 | 8/10 | RSS 斜率稳态最小二乘、代际逐出、pixmap 池化；扣 tmp 目录与 CPU 回退无对照门禁 |
| 可读性 | 8/10 | 注释讲约束不讲流水账、JSON 字段稳定；扣 wrgate 死 if、tmp 垃圾、C2 公式门禁绕远 |
| **合计** | **32/40** | |

## TOP5 必修清单

1. **[P1] C2 damage_ratio_sum 改为实测**：从 render/frame.go present 决策处导出每帧 rects 面积和进 ability_extra，废掉 `ui_wr_c2_retained_scene/main.go:63-66` 的公式估算；R4b 同口径一并核。
2. **[P2] InputRouter 补命中路径分发**：`ui/embedder/input_router.go:130-150` 沿 hit path 祖先链逐个派发 OnPointer（含 enter/leave），兑现注释承诺。
3. **[P2] 手势竞技场接入主路由**：InputRouter.OnPointer 统一喂 GestureArenaManager，tap/pan 全局仲裁，scrollable 改走统一入口。
4. **[P2] 清理 vet 红灯**：删 `tmp_vlprobe/`、`render/tmp_ref_stroke/wind/sqrtest.go`（或移出包路径），修 `ui/rendering/text.go:592` 自赋值；目标 `go vet ./...` 全绿。
5. **[P3] 门禁口径加固**：fps 门禁加 max_frame_interval 或 hitch 双判防长尾稀释；删 `examples/wrgate/report.go:169-172` 空 if；补 GPU vs Software 像素一致性抽样门禁。
