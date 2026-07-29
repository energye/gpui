---
name: gpui-metrics-audit
description: §2.2 指标族 A–J 的**正误审查与 bug 修复**——字段完备性、诚实性、门禁阈值防偷放、指标与代码观测一致性、降画质装绿检测。当用户说「审 R3 的指标」「metrics-audit R4」「指标族查 bug」「fps 假值」「vsource 假锁」「slope 偷放」「cpu 双 0 装绿」「降画质装绿」或怀疑任一 ui_wr_* JSON 指标不诚实/不完备/被偷放阈值时触发。可独立跑（查已有窗），也可串在 wr-close 判门禁时跑。
user_invocable: true
disable_model_invocation: false
---

# gpui-metrics-audit — 指标族正误审查与 bug 修复（防假绿）

> **与 wr-quality / wr-close 的分工：**
> - `gpui-wr-quality` = 关 R 前的**质量标准**（场景复杂度 / HUD 可见 / 实现点六维）——写代码**前**
> - `gpui-wr-close` = 关 R 的**执行流程**（建窗/跑/判门禁/回写）——写代码**后**
> - 本 skill = 指标族本身的**正误审查**——可独立跑（查已有窗的 JSON/代码），也可串在 wr-close 第 3 步判门禁时跑
>
> **真源：** `docs/ENGINE_UI_WIDGET_RENDER.md` §2.2（指标族 × 真窗义务）+ `docs/ENGINE_UI_RENDER_BASE.md` §20/§25.4（指标定义 + 架构诚实点）+ `docs/ENGINE_CODING_RULES.md`（禁降画质装绿）。本 skill 是其执行骨架。

## 0. 为什么要这个 skill

指标族 A–J 是 §2.2 的**硬门禁**，但以下 5 类「指标假绿」最常见，单纯跑 JSON 门禁抓不住：

| 假绿类 | 表现 | 单纯 JSON 门禁为何抓不到 |
|--------|------|--------------------------|
| **字段默默省略** | 族 G/H/I 的 `unavailable` 字段被悄悄不写，而非写 `null`+原因 | §2.2.1 要求「无数据填 null/0 + unavailable 原因，**禁止默默省略**」，但 JSON 解析常跳过 missing |
| **诚实性破** | `vsync_source=true` 但实为 fallback；`fps_interval` 靠静帧刷高；proxy CPU 双 0 却过 fps 门禁 | 门禁只判数值范围，不判「值与代码实际是否一致」 |
| **门禁阈值偷放** | README 声明的阈值高于文档默认（fps 门禁被放宽过 55、slope 阈值抬高过 30000、damage_ratio 松到简陋场景能过） | 门禁只判「是否过 README 声明的阈值」，不判「README 声明是否低于文档默认」 |
| **指标与代码观测不一致** | `boundary_skip` 累计数 ≠ `BoundaryCache.FrameSkip` 实际；`gpu_ops` 被误读为「每帧 submit 次数」 | 门禁只判 JSON 字段，不回代码核观测源 |
| **降画质装绿** | fps 靠静帧刷高（动帧其实掉）；damage_ratio 靠简陋场景刷低；slope 靠短时窗刷低 | 门禁只判终值，不判「值是靠什么场景刷出来的」 |

**结论：** JSON 门禁绿 ≠ 指标诚实。本 skill 是**指标层的 code review**，抓上述 5 类。

---

## 1. 触发与输入

用户可能给：
- 一个 R/C id（如 `audit R3`、`metrics-audit R4`）→ 审该窗的 JSON + 代码
- 一个具体怀疑（如「fps 假值」「vsync 假锁」「slope 偷放」「cpu 双 0 装绿」）→ 定向审该字段
- 无参数（如「指标族查 bug」）→ 审所有 `ui_wr_*` 的 JSON 一致性

从输入解析：
- `ABILITY_ID`（若给）：定位 `examples/ui_wr_<package>/`
- `SUSPECT_FIELD`（若给）：定向审该字段（如 `vsync_source`、`cpu_ui_pct`、`rss_slope_kb_per_min`）
- 若无两者：全扫 `examples/ui_wr_*/` 的最近 JSON 输出或跑一遍取 JSON

## 第 0 步：读真源 + 定位

1. `read_file docs/ENGINE_UI_WIDGET_RENDER.md` §2.2.1（族 × 真窗义务表）+ §2.2.2/§2.2.3/§2.2.4（FAIL 线）+ §2.2.5（JSON 最小外壳）
2. `read_file docs/ENGINE_UI_RENDER_BASE.md` §20.2（M-\* 全表，字段定义）+ §25.4（架构诚实点 6 条）+ §20.3（采集注意）
3. `read_file ui/scheduler/metrics.go` 的 `FrameMetrics` struct——**字段名真源**，禁止自创字段名
4. 若给 `ABILITY_ID`：`read_file examples/ui_wr_<package>/main.go` + `README.md`，定位 JSON 输出代码与门禁阈值声明

## 第 1 步：字段完备性审查（族 A–J 默默省略）

对照 §2.2.1 族表，**逐族**查 JSON 输出代码：

| 族 | 必采字段（§2.2.1） | 审查点 |
|----|---------------------|--------|
| **A 帧时** | `fps_wall`、`interval_avg_ms`、`interval_p50_ms`、`interval_p95_ms`、`interval_p99_ms`、`hitch_count`、`hitch_rate_per_min`、`vsync_source`、`target_hz` | 每个字段在 JSON 里**必须出现**；缺数据写 `null`/`0` + `"unavailable_reason"`，**禁止默默省略** |
| **B 管线** | `frame_build_ms`、`frame_raster_ms`、`pipeline_depth`、`pipeline_max` | 同上 |
| **C 脏区** | `layout_count`、`paint_count`、`damage_area_px`、`damage_ratio`、`present_mode`、`present_policy` + 能力专用 | 同上；能力专用字段（R3 `boundary_skip`/`boundary_rerecord`、R7 `bind_count`、R4b `dirty_layer_ids` 等）必须按该 R §2 主表门禁列出现 |
| **D CPU** | `cpu_pct_avg`、`cpu_ui_pct`、`cpu_raster_pct` | 三者**全采**；缺则写 `cpu_unavailable=true` + 原因（Linux 长窗禁止长期 unavailable） |
| **E 内存** | `rss_start_kb`、`rss_end_kb`、`rss_peak_kb`、`rss_slope_kb_per_min`、`rss_after_close_kb`（能采则采） | 前 4 个 Linux 必采；缺 → FAIL；`after_close` 能采则采，不采要标原因 |
| **F GPU** | `gpu_ops`、`cpu_fallback_ops`、`last_cpu_fallback`、`frame_flushes` | 4 个全采；热路径 `cpu_fallback_ops` 无故暴涨要解释 |
| **G 图/文** | R9/R10 强制 `measure_cache_hit`；其它窗可 `g_metrics=skipped` + 原因 | R9/R10 缺 `measure_cache_hit` → FAIL；其它窗没标 `skipped` 原因 → FAIL |
| **H 启动** | `warmup`、`time_to_first_present_ms`（有则采） | R0/R16/C0 首帧有内容硬；`time_to_first_present_ms` 超预算 FAIL（预算写 README，默认建议 <2000ms） |
| **I 回归** | 支持 `BASELINE_JSON` / `SAVE_BASELINE_JSON` | 可选开；发布/合入关键窗建议开；超 `DefaultBaselineTolerance` → FAIL |
| **J 正确性** | `ui` 无 import gpu、无 cgo、本窗不降画质 | 跑 `go test ./ui -run TestNoGPUImport` + grep `import "C"` + 查 README 声明 |

**输出：** 每族列出 `PRESENT` / `MISSING（原因）` / `NULL_WITHOUT_REASON`（即写了 null 但没标 unavailable 原因，违反 §2.2.1）。

**bug 修复：** 若 `MISSING` 或 `NULL_WITHOUT_REASON`，回 `main.go` 补字段输出 + 原因标注。

## 第 2 步：诚实性审查（值与代码实际是否一致）

逐族查「JSON 填的值」vs「代码实际观测」是否一致：

| 字段 | 诚实性审查 | 假绿模式 |
|------|------------|----------|
| `vsync_source` | 查 `ui/scheduler` 的 `WaitFramePace`——`true` 仅在成功 `WaitVSync` 后；fallback 时必须填 `"fallback"`，**禁止** `true` 或空 | fallback 装锁 60Hz（U13 反宣称） |
| `fps_interval` vs `fps_wall` | `fps_interval = 1000/interval_avg_ms`（稳态帧率）；`fps_wall = present_count/elapsed_sec`（含开关窗，仅参考）。门禁优先 `fps_interval`。查是否用 `fps_wall` 冒充稳态 | 静帧刷高 `fps_wall` 装稳态 |
| `cpu_ui_pct` / `cpu_raster_pct` | 查 `metrics.go` 的 `pathCPULocked()`——proxy = build/raster 路径工作份额，**非 OS 线程**。动画窗二者应可解释；**双 0 却 fps 门禁靠降画质过** → FAIL | 双 0 假象（无 build/raster note 时为 0，但 fps 门禁若靠静帧过 = 装绿） |
| `cpu_pct_avg` | 查 `ProcessTracker`；无 /proc 时 `cpu_unavailable=true`。Linux 真窗**禁止**长期 unavailable | stub=0 装绿 |
| `rss_slope_kb_per_min` | 查 `ProcessTracker.Apply`；`(end-start)/elapsed_minutes`。零 when wall span missing/negligible。soak 长窗 slope 应 ≈0，非 Linux stub=0 | stub=0 装稳；短时窗刷低 slope |
| `gpu_ops` | 查 `render.Context.RenderPathStats`——**累计**非每帧。禁止误读为「每帧 GPU submit 次数」（母表 §25.4 已警告） | 累计数被当成每帧数误判 |
| `cpu_fallback_ops` | 热路径应趋 0；暴涨要解释（阈值写 README） | 无故暴涨不解释 |
| `damage_ratio` | FullPaint 下可接近 1（**不得**用其冒充 Retained）；Retained 下应 ≪1。查 `present_mode` 与 `present_policy` 是否与实际路径一致 | FullPaint 的 damage_ratio≈1 装成 Retained 的脏区小 |
| `boundary_skip` / `boundary_rerecord` | 查 `BoundaryCache.FrameSkip` / `FrameRerecord` 累计——JSON 累计数应与代码实际一致 | 累计数与实际不符 |
| `present_policy` | 查 `MetricsStore.PresentPolicy()` + `PipelineApp.useRetained`——默认 `full_paint`，retained 需 `SetPresentPolicy` 切。查是否宣称 retained 但实走 full_paint | full_paint 装 retained |
| `hitch_rate_per_min` | 查 `hitchRatePerMinLocked()`——`hitch_count/elapsed_minutes`。零 when wall span missing。soak 应低 | 短时窗刷低 hitch |

**输出：** 每个审字段标 `HONEST` / `SUSPECT（具体）` / `FAKE（具体假绿模式）`。

**bug 修复：** `SUSPECT`/`FAKE` 回代码修——通常是修 JSON 输出逻辑（如实填 `fallback`、标 `unavailable`），或修观测源（如 `ProcessTracker` stub 该降级而非填 0）。

## 第 3 步：门禁阈值防偷放审查

对照 §2.2.2/§2.2.3/§2.2.4 的**文档默认 FAIL 线**，查各窗 README 声明的阈值是否**低于**文档默认（即偷放宽）：

| 指标 | 文档默认 FAIL 线 | 偷放模式 |
|------|------------------|----------|
| `fps_interval` 或 `fps_wall` | 动画/滚动/持续 tick 窗 **<55** 或 `interval_p95_ms>22` → FAIL | README 把 fps 阈值放宽到 50/45/30 |
| `hitch_rate_per_min` | soak 长窗超 README 预算（默认建议 ≤5） → FAIL | README 把 hitch 预算抬高到 20/50 |
| `cpu_pct_avg` | 长窗/soak（≥60s）`>85%` 且持续 → FAIL | README 把 CPU 阈值抬高到 200%（单核折算）或关掉 |
| `rss_slope_kb_per_min` | soak/压力窗 `>30000` 极端爬升（场景可收紧到更低） → FAIL | README 把 slope 阈值抬高到 100000+ 或写 `slope_gate=off` 滥用 |
| `damage_ratio` | Retained 下应 ≪1（R4 `MaxDamageRatio=0.35`） | README 把 damage 阈值松到 0.8/0.9（简陋场景下必过） |
| `time_to_first_present_ms` | 默认建议 <2000ms 冷启同机 | README 把首帧预算抬高到 10000+ |
| `cpu_fallback_ops` | 热路径趋 0；暴涨要解释（soak 可 >0 但须解释） | README 把 fallback 阈值抬高或关掉 |

**审查方法：**
1. `grep` README 的 `Gates` / `FAIL` / `阈值` / `gate` / `Max` 段，提取声明的阈值
2. 对照文档默认——**声明的阈值高于文档默认** = 偷放
3. 特别查 `slope_gate=off`、`fps_gate=off` 等**关闭门禁**的声明——§2.2.4 仅允许「短于 15s 的正确性窗」写 `slope_gate=off`，其它滥用 → FAIL

**输出：** 每个阈值标 `STRICT（声明确 ≤ 文档默认）` / `AT_DEFAULT` / `LOOSE（偷放，具体高出多少）` / `GATE_OFF（滥用关闭）`。

**bug 修复：** `LOOSE`/`GATE_OFF` 回 README 把阈值降回文档默认，或回代码修场景（如 fps 真低就修引擎/减装饰，**禁止**降阈值保门禁）。

## 第 4 步：指标与代码观测一致性审查

回代码核 JSON 字段的**观测源**是否与字段语义一致：

| 字段 | 观测源（代码） | 一致性审查 |
|------|----------------|------------|
| `boundary_skip` / `boundary_rerecord` | `rendering.BoundaryCache.FrameSkip` / `FrameRerecord`（累加，跨帧） | JSON 累计数应 = `BoundaryCache` 实际；若每帧重置会偏低 |
| `bind_count` | `rendering.VirtualList.BindCount`（当前已挂子节点数） | JSON 应 ≪ `ItemCount`；若 = `ItemCount` 则虚拟化失效 |
| `gpu_ops` | `render.Context.RenderPathStats`（累计路由计数） | JSON 累计数应 = Context 实际；**禁止**当每帧 submit |
| `cpu_ui_pct` / `cpu_raster_pct` | `MetricsStore.pathCPULocked()`（build/raster 份额 × 进程 CPU） | proxy ≠ OS 线程；当 `cpu_pct_avg>0` 时 UI+Raster≈进程 CPU |
| `rss_slope_kb_per_min` | `ProcessTracker.Apply`（`(end-start)/elapsed_min`） | 零 when span missing/negligible；非 stub=0 |
| `damage_area_px` / `damage_ratio` | `PipelineApp` Present 后 `DamageStats` | Retained 下应 ≪全屏；FullPaint 下可接近全屏 |
| `dirty_layer_ids` | `LastDirtyLayerIDs` | R4b 复杂场景 id 应稳；每帧重建 = 不稳 |
| `present_mode` / `present_policy` | `MetricsStore.PresentPolicy()` + `PipelineApp.useRetained` | 宣称 retained 但 `useRetained=false` = 假 |

**审查方法：**
1. `grep` `ui/scheduler/metrics.go` 取字段定义 + Note* 方法
2. `grep` `ui/embedder/pipeline_app.go` 取字段接线
3. `grep` 该窗 `main.go` 的 JSON marshal，对照字段名是否与 `FrameMetrics` struct 一致（禁止自创字段名）
4. 若字段在 JSON 里与 struct 名不一致 → bug（要么修 JSON 用 struct 名，要么 struct 加 alias）

**输出：** 每个字段标 `CONSISTENT` / `MISREAD（误读模式）` / `MISWIRED（接线错）` / `STALE_DATA（累加 vs 每帧错）`。

**bug 修复：** 回 `main.go` 或 `pipeline_app.go` 修接线。

## 第 5 步：降画质装绿检测

查「门禁值是靠什么场景刷出来的」——若靠**降场景**过门禁，即装绿：

| 降画质模式 | 检测 |
|------------|------|
| fps 靠静帧刷高 | 查窗是否有持续 tick（动画/滚动）；若仅静帧却 fps>55 = 静帧刷高（动帧实际掉） |
| damage_ratio 靠简陋场景刷低 | 查场景是否「两色块」级（wr-quality §3 场景矩阵会拦）；简陋场景下 damage 自然≪1 = 不证明复杂 UI 下正确 |
| slope 靠短时窗刷低 | 查 `RUN_SECONDS` 是否 ≥该 R 关闭用值；短于关闭用值刷低 slope = 假稳 |
| hitch 靠短时窗刷低 | 同上；hitch_rate 按 /min，短时窗 hitch_count=0 自然 hitch_rate=0 |
| cpu 靠 idle 刷低 | 查窗是否有动画；若 idle 空转 cpu=0 = 不证明动画窗 CPU 可控 |
| boundary_skip 靠纯色块刷高 | 查静区是否纯 `RenderColorBox`（wr-quality 禁止）；色块 skip 容易，文/嵌套 skip 才是真 |

**审查方法：**
1. 查 wr-quality 的场景矩阵该 R 行——若场景未达 U17 复杂度，**所有**门禁值都存疑（降场景刷出来的）
2. 查 `RUN_SECONDS` 实跑值 vs §2.5 关闭用值——短于关闭用值 = 刷低时窗类指标
3. 查是否有持续 tick——无 tick 却 fps 门禁过 = 静帧刷高

**输出：** 每类标 `EARNED（靠复杂场景+关闭用时长刷出来的）` / `SCENE_CHEAT（靠简陋场景）` / `TIME_CHEAT（靠短时窗）` / `IDLE_CHEAT（靠空转）`。

**bug 修复：** 降画质装绿 = 回 wr-quality 升场景，**不许**降门禁。本步与 wr-quality 的 §5「引擎若挂必修」联通——若复杂场景下指标炸，报引擎洞定点修。

## 第 6 步：串接 wr-close / wr-quality

本 skill 可独立跑（查已有窗），也可串在 wr-close 第 3 步：

- **串 wr-close：** wr-close 判 JSON 族门禁时，同时跑本 skill 的第 1–5 步。任一假绿类发现 → 综合 FAIL，不得标 ✅，回 wr-quality 升场景或回代码修观测
- **串 wr-quality：** 本 skill 第 5 步的降画质检测，若场景未达 U17，直接交 wr-quality 升场景
- **独立跑：** 用户给「audit R3」即审已有窗，输出审查报告 + bug 清单

## 输出格式

完成后给一份审查报告：

```
📋 R<id> 指标族审查

第 1 步 字段完备性：
  族 A：PRESENT（9/9 字段）
  族 B：PRESENT（4/4）
  族 C：MISSING — `dirty_layer_ids` 未输出（R4b 专用，该窗为 R4 不强制）→ N/A
  族 D：NULL_WITHOUT_REASON — `cpu_ui_pct=0` 未标 unavailable_reason → FAIL
  族 E：PRESENT（5/5）
  族 F：PRESENT
  族 G：g_metrics=skipped（非 R9/R10，合规）
  族 H：PRESENT
  族 I：可选未开（合规）
  族 J：depcheck 绿

第 2 步 诚实性：
  vsync_source：HONEST（fallback）
  fps_interval vs fps_wall：HONEST（门禁用 fps_interval）
  cpu_ui/raster：SUSPECT — 双 0 但 fps>55，疑静帧刷高 → 验第 5 步
  gpu_ops：HONEST（累计，README 已注明）
  present_policy：HONEST（full_paint，未宣称 retained）

第 3 步 门禁阈值：
  fps≥55：AT_DEFAULT
  hitch≤5/min：AT_DEFAULT
  cpu>85%：LOOSE — README 声明 150%，文档默认 85% → FAIL（偷放）
  slope>30000：AT_DEFAULT

第 4 步 观测一致性：
  boundary_skip：CONSISTENT（= BoundaryCache.FrameSkip 累加）
  gpu_ops：CONSISTENT
  cpu_ui/raster：CONSISTENT（pathCPULocked 接线对）

第 5 步 降画质检测：
  fps>55：IDLE_CHEAT — 窗无持续 tick，静帧刷高 → FAIL
  damage_ratio≪1：SCENE_CHEAT — 场景仅两色块（wr-quality §3 未达）→ FAIL

综合：FAIL（3 处需修）
  1. 族 D `cpu_ui_pct=0` 补 unavailable_reason 或修观测
  2. README cpu 阈值降回 85%（偷放）
  3. 场景升维加持续 tick（fps 靠静帧刷高）→ 交 wr-quality
```

若全 PASS：

```
✅ R<id> 指标族诚实
  字段完备：10 族全 PRESENT 或合规 N/A
  诚实性：11 字段全 HONEST
  阈值：7 项全 AT_DEFAULT 或 STRICT
  观测一致性：8 字段全 CONSISTENT
  降画质：6 项全 EARNED
```

## 禁令自检（每次结束前过一遍）

- [ ] 族 A–J 字段逐族查过，没漏族
- [ ] `unavailable` 字段有原因标注，没默默省略
- [ ] `vsync_source` 与 `WaitFramePace` 实际一致，fallback 没装锁 60Hz
- [ ] `fps_interval`（非 `fps_wall`）做门禁主判
- [ ] `cpu_ui_pct`/`cpu_raster_pct` 双 0 时查是否静帧刷高 fps
- [ ] README 声明的阈值全不高于文档默认（没偷放）
- [ ] `slope_gate=off`/`fps_gate=off` 仅短于 15s 正确性窗用（没滥用）
- [ ] `gpu_ops` 当累计读（没误读每帧 submit）
- [ ] JSON 字段名与 `FrameMetrics` struct 一致（没自创）
- [ ] 场景达 wr-quality U17（没靠简陋场景刷 damage/skip）
- [ ] `RUN_SECONDS` ≥ 该 R 关闭用值（没靠短时窗刷 slope/hitch）
- [ ] 没为过门禁删场景降复杂度（降画质装绿）

任一项未过 → 报告 bug，回代码或回 wr-quality 修。
