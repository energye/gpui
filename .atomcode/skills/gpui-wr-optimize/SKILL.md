---
name: gpui-wr-optimize
description: §R 真窗的**优化/增量回流**——对已关闭的 R（状态 ✅）做性能优化、场景增量、HUD 增强、指标新增，然后重跑重审重关 + 回写 docs §2 状态列 + §10 修订表。当用户说「优化 R3」「R4 提性能」「R7 加场景」「R5 加 HUD」「给 R4 降 draw call」「optimize R7」「R3 加指标」「R7 加复杂度」「已关的 R4 加新场景」时触发。与 gpui-wr-quality（写前定标准）、gpui-wr-close（执行关闭流程）、gpui-metrics-audit（指标层审查）、gpui-wr-rework（反攻/修复回流）互补，专门接「已关 R 的优化/增量 → 重跑 → 重审 → 重关 → 回写版本」这条回流。
user_invocable: true
disable_model_invocation: false
---

# gpui-wr-optimize — §R 真窗优化/增量回流（已关 R 的加深）

> **与现有 skill 的分工：**
> - `gpui-wr-quality` = 关 R **前**的质量标准（场景矩阵 + HUD + 实现点六维）——**写代码前**
> - `gpui-wr-close` = 关 R 的**执行流程**（建窗/跑/判门禁/回写）——**写完代码后**
> - `gpui-metrics-audit` = 指标族 A–J 的**正误审查与 bug 修复**——**指标层**
> - `gpui-wr-rework` = 门禁 FAIL / 指标装绿的**反攻/修复回流**——**代码层 + 引擎层**（修 bug）
> - **本 skill** = 已关 R 的**优化/增量回流**——**性能优化 + 场景增量 + HUD 增强 + 指标新增**（不修 bug，是加深）
>
> **真源：** `docs/ENGINE_UI_RENDER_BASE.md` §22.2「收口后残项」+ §25.5「建议的下一步」（产品驱动开残项、可选架构、可观测加深）；`docs/ENGINE_UI_WIDGET_RENDER.md` §10 修订表 + §9 文档与状态治理。若本 skill 与真源矛盾，以真源为准——发现矛盾停下报告，不要自决。

## 0. 为什么要这个 skill

基座收口后（RENDER_BASE §25），已关 R 的优化/增量没有 skill 接住，常见 3 类错误回流：

| 错误回流 | 表现 | 为何危险 |
|----------|------|----------|
| **优化后不重审** | 降了 draw call 但没重跑 metrics-audit，可能引入指标回归 | 违反 wr-close §3「每族必判」+ metrics-audit §2「诚实性」 |
| **增量后不重关** | 加了新场景/新 HUD 但 docs §2 状态列没回写 | 违反 WIDGET_RENDER §9「文档与状态治理」+ U9 |
| **优化当修 bug** | 把「想提性能」当「门禁 FAIL 反攻」走 wr-rework | 路径错；优化是加深，不是修洞 |

本 skill 把「已关 R 优化/增量 → 重跑 → 重审 → 重关 → 回写版本」这条回流**固化**，确保优化不引入回归、增量不漂移 docs 状态。

---

## 1. 触发与输入

用户可能给：

- 一个已关 R/C id + 优化目标（如「优化 R3」「R4 降 draw call」「optimize R7」）→ 优化回流
- 一个已关 R/C id + 增量目标（如「R7 加场景」「R5 加 HUD」「R3 加指标」「已关的 R4 加新场景」）→ 增量回流
- 一个混合请求（如「R7 优化 + 加滚动场景」）→ 两路都走

从输入解析：

- `ABILITY_ID`（必给）：如 `R7`、`C3`、`R3b`
- `OPT_TARGET`（若给）：优化目标，如 `降 draw call`、`提 fps`、`清冗余`、`降内存`
- `INCR_TARGET`（若给）：增量目标，如 `加场景`、`加 HUD`、`加指标`、`加复杂度`

## 第 0 步：读真源 + 定位 + 前置校验

**必做**——每次都先读，禁止凭记忆跑流程（真源会变）。

1. `read_file docs/ENGINE_UI_WIDGET_RENDER.md`：
   - §2 主表该 id 行（取 `PACKAGE`、`WINDOW=1200×800`、`CLOSE_SECONDS`、`WAVE`、当前「状态」列）
   - §2.2.1 指标族 × 真窗义务表 + §2.2.2/§2.2.3/§2.2.4 FAIL 线（重审基准）
   - §9「文档与状态治理」+ §10 修订表（优化/增量时状态标注 + 版本回写）
2. `read_file docs/ENGINE_UI_RENDER_BASE.md` §22.2「收口后残项」+ §25.5「建议的下一步」——确认该优化/增量在「产品驱动开残项」范围内，不是无目标清 D
3. `read_file .atomcode/skills/gpui-wr-quality/SKILL.md` §3 场景矩阵该 R 行 + §4 实现点六维——增量后必须重新达 U17/U18/U20
4. `read_file examples/ui_wr_<package>/main.go` + `README.md`——定位现有真窗代码与门禁阈值声明

### 前置校验（硬）

| 校验项 | 不通过的处理 |
|--------|--------------|
| 该 R 当前状态必须 `✅`（已关） | 若 `⬜` 未关 → 交 wr-quality + wr-close 走首次关闭；若 `🔄` 质量返工中 → 交 wr-rework 走反攻；**不**走本 skill |
| 优化/增量目标必须在 RENDER_BASE §22.2/§25.5 范围内 | 若超出（如要加 PlatformView）→ 停下问用户，这是远期 D 行，不属本波 |
| 该 R 的 §2 主表门禁不能因优化放宽 | 若优化方案需降门禁阈值 → 停下报告，门禁是硬的不许放 |

**前置校验通过 → 状态标注（WIDGET_RENDER §9）：**

1. `read_file docs/ENGINE_UI_WIDGET_RENDER.md` 定位 §2 主表该 R 行的「状态」列
2. 用 `edit_file` 把 `✅` 改成 `✅🔄 优化中` 或 `✅🔄 增量中`（**保留 ✅**，因为基础门禁曾绿；**加 🔄** 标注当前在优化/增量回流中）
3. §10 修订表先加占位行：`<版本号待定> | R<id> 优化/增量：<目标摘要>`

## 第 1 步：优化回流（OPT_TARGET 类）

> **优化目标：** 降 draw call、提 fps、清冗余、降内存、降 CPU、降 hitch、降 raster ms 等。

### 1.1 定位优化点

根据 `OPT_TARGET`，定位 `examples/ui_wr_<package>/main.go` 的优化点：

| 优化目标 | 定位 | 常见优化动作 |
|----------|------|--------------|
| 降 draw call | 数 `RenderPathStats` 的 `gpu_ops` 累计 + 每帧 `DC` 调用 | 合批同 shader、用 Picture 录回放避免重录、静态走 BoundaryCache |
| 提 fps | 查 `interval_p95_ms` 尖峰帧 | 减装饰、静态走 BoundaryCache、避免每帧 layout |
| 清冗余 | 查死代码、未用 import、未用变量 | 删 |
| 降内存 | 查 `rss_peak_kb` 来源 | 复用 buffer、提前 dispose、避免大 Picture 常驻 |
| 降 CPU | 查 `cpu_ui_pct`/`cpu_raster_pct` 尖峰帧 | 静态 skip、避免每帧 measure、走 BoundaryCache |
| 降 hitch | 查 `hitch_count` 集中帧 | 减长 build/raster 帧、避免 GC 尖峰 |
| 降 raster ms | 查 `frame_raster_ms` 尖峰帧 | 减 overdraw、走 Picture 缓存、避免大 SaveLayer |

### 1.2 优化纪律

- **不**改门禁阈值（门禁是硬的）
- **不**删场景降复杂度（违反 wr-quality U17）
- **不**降画质装绿（违反 metrics-audit §5）
- **只**改 `examples/ui_wr_<package>/main.go` 的实现细节（合批、缓存、复用）
- 若优化触及引擎（如要加 multi-RT 合批）→ 停下报告，这是 W6 架构，不属本波

### 1.3 优化完 → 交第 3 步重跑

## 第 2 步：增量回流（INCR_TARGET 类）

> **增量目标：** 加场景、加 HUD、加指标、加复杂度、加相位、加实现点维度等。

### 2.1 重新对照 wr-quality 场景矩阵

`read_file .atomcode/skills/gpui-wr-quality/SKILL.md` §3 场景矩阵该 R 行，确认增量后是否仍达 U17 五项 + U18 HUD + U20 实现点六维。

### 2.2 按增量目标改代码

| 增量目标 | 改哪 | 改什么 |
|----------|------|--------|
| 加场景 | `examples/ui_wr_<package>/main.go` | 加多区域布局 / 静态密集 / 动态热点 / 能力专属压力（按 wr-quality §1 U17） |
| 加 HUD | `examples/ui_wr_<package>/main.go` | 加 wrkit LiveHUD 的字段（按 wr-quality §2 U18） |
| 加指标 | `examples/ui_wr_<package>/main.go` | JSON 输出加该指标字段（字段名取自 `ui/scheduler/metrics.go` 的 `FrameMetrics` struct） |
| 加复杂度 | `examples/ui_wr_<package>/main.go` | 加嵌套深度 / 子树数量 / 动画节点数 |
| 加相位 | `examples/ui_wr_<package>/main.go` | 加 Steady/Spike/Recover 相位切换（按 wr-quality §1 U17 第 5 项） |
| 加实现点维度 | `examples/ui_wr_<package>/README.md` | 补六维清单缺项（正确性/脏区/缓存/边界条件/失败模式/窗内如何看出） |

### 2.3 同步改 README

`examples/ui_wr_<package>/README.md` 同步加：

- **Visible effect** 表加新场景/新 HUD 的预期行
- **Gates** 表加新指标的 §2.2 全族 FAIL 线（**阈值不偷放**）

### 2.4 增量完 → 交第 3 步重跑

## 第 3 步：重跑 GPU 真窗取新 JSON

**禁止跳过**——优化/增量的核心是「重跑验没回归」。

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=<CLOSE_SECONDS> go run ./examples/<package>
```

- 若环境无 GPU/X11：**停下**，告诉用户「需要 GPU 真窗环境（needs_gpu_window）」
- 跑完取 stderr + JSON。若 JSON 缺族字段，回第 1/2 步补 main.go
- 若 `exit 1` 且 `FAIL:` 原因是门禁不达标 → **优化/增量引入了回归**，回第 1/2 步排查

## 第 4 步：重跑 metrics-audit 验没回归

`use_skill` 加载 `gpui-metrics-audit`，对**新 JSON** 跑 §1–§5 全套审查：

- 字段完备性（族 A–J 不默默省略，**新加指标字段也在**）
- 诚实性（vsync_source 不假锁、fps_interval 不靠静帧刷高）
- 门禁阈值防偷放（README 阈值不高于文档默认）
- 指标与代码观测一致性（JSON 累计数 = 代码实际）
- 降画质装绿检测（场景达 U17、RUN_SECONDS ≥ 关闭用值）

### 没回归判定

| metrics-audit 输出 | 动作 |
|--------------------|------|
| 全 PASS（EARNED + HONEST + CONSISTENT + AT_DEFAULT + PRESENT） | 交第 5 步重关 |
| 任一 FAIL | **优化/增量引入了回归**，回第 1/2 步排查；若是引擎洞（类 C）交 wr-rework 第 3 步 |

**关键：本步不许降门禁。** 若优化后 fps 真低就回退优化（回第 1 步），**禁止**把 fps 门禁从 55 放宽到 45。

### 同时对比优化前后指标（若 OPT_TARGET 类）

若用户给了 `OPT_TARGET`（如「R4 降 draw call」），对比优化前后的关键指标：

| 指标 | 优化前 | 优化后 | 判定 |
|------|--------|--------|------|
| `gpu_ops`（累计） | X | Y | Y < X ? 优化生效 : 没优化 |
| `fps_interval` | X | Y | Y ≥ 55 ? 没回归 : 回退 |
| `rss_peak_kb` | X | Y | Y ≤ X ? 降内存生效 : 没降 |
| `cpu_pct_avg` | X | Y | Y ≤ X ? 降 CPU 生效 : 没降 |

**若优化没生效**（如降 draw call 但 `gpu_ops` 没降）：报告用户，可能优化方向错，回第 1 步重新定位优化点。

## 第 5 步：交 wr-close 重关 + 状态升维 + 版本回写

`use_skill` 加载 `gpui-wr-close`，走其完整流程（第 0–5 步）重关该 R：

- 第 0 步：读真源 + 定位包（取当前 `CLOSE_SECONDS`、`WAVE`）
- 第 1 步：跳过（真窗代码已在第 1/2 步改完）
- 第 2 步：重跑 GPU 取新 JSON（与第 3 步一致，wr-close 自己再跑一遍确认）
- 第 3 步：判门禁（逐族 FAIL 线）
- 第 4 步：若该 R 有组合窗，跑组合窗
- 第 5 步：回写 docs §2 状态列

### 状态升维（WIDGET_RENDER §9 + §10）

按 wr-close 第 5 步回写时，**优化/增量场景**的状态转换：

| 优化/增量前状态 | 优化/增量后状态 | §10 修订表 |
|-----------------|-----------------|------------|
| `✅🔄 优化中` | `✅v2-optimized` | `<版本> \| R<id> 优化生效：<优化前 → 优化后 指标对比>` |
| `✅🔄 增量中` | `✅v2-extended` | `<版本> \| R<id> 增量生效：<加了什么 + 新指标值>` |
| `✅`（直接优化，未标 🔄） | `✅v2-optimized` | 同上 |

`edit_file` 回写 `docs/ENGINE_UI_WIDGET_RENDER.md`：

1. §2 主表该 R 行「状态」列：`✅🔄 优化中 → ✅v2-optimized` 或 `✅🔄 增量中 → ✅v2-extended`
2. §10 修订表把占位行改为正式版本说明
3. 重新 `read_file` 确认 edit 落点对、没破坏表格 markdown

## 输出格式

完成后给用户一份结构化报告：

```
✅ R<id> 优化/增量收口

回流类别：<优化 / 增量 / 混合>
优化/增量前状态：<✅ / ✅🔄 优化中>

优化目标（若 OPT_TARGET）：<降 draw call / 提 fps / ...>
增量目标（若 INCR_TARGET）：<加场景 / 加 HUD / 加指标 / ...>

改了哪些文件：
  - examples/<package>/main.go（<优化/增量摘要>）
  - examples/<package>/README.md（<Visible effect / Gates 更新摘要>）

重跑验证：
  GPU 真窗：RUN_SECONDS=<CLOSE_SECONDS> PASS
  metrics-audit 重审：全 PASS（没回归）
  wr-close 重关：族 A–J 全绿

优化前后对比（若 OPT_TARGET）：
  gpu_ops（累计）：X → Y（<降了 / 没降 / 升了>）
  fps_interval：X → Y（<没回归 / 回退>）
  ...

状态升维：
  docs §2 R<id> 行：<✅ → ✅v2-optimized> 或 <✅ → ✅v2-extended>
  §10 修订表：加版本说明

下一步：<W<n+1> 剩 R<下个 id>> 或 <优化完成，无后续>
```

若有任一 FAIL：

```
⚠️ R<id> 优化/增量未达标

当前回流类别：<优化 / 增量 / 混合>
FAIL 原因：<优化没生效 / 增量引入回归 / 引擎洞（交 wr-rework）/ 门禁不过>

下一步建议：
  - 优化没生效 → 回第 1 步重新定位优化点
  - 增量引入回归 → 回第 2 步排查增量代码
  - 引擎洞 → 交 wr-rework 第 3 步定点修
  - 门禁不过 → **不许降门禁**，回第 1/2 步修代码
```

## 禁令自检（每次结束前过一遍）

- [ ] 前置校验做了（该 R 当前 ✅ 才走本 skill；⬜ 交 wr-close；🔄 交 wr-rework）
- [ ] 优化/增量目标在 RENDER_BASE §22.2/§25.5 范围内（不是无目标清 D）
- [ ] **没降门禁阈值过优化**（门禁是硬的）
- [ ] **没删场景降复杂度**（违反 wr-quality U17）
- [ ] **没降画质装绿**（违反 metrics-audit §5）
- [ ] 优化只改了 `examples/ui_wr_*/main.go` 的实现细节，**没**触及引擎架构（如 multi-RT）
- [ ] 增量后重新达 wr-quality U17/U18/U20
- [ ] 重跑用了 `RUN_SECONDS ≥ 关闭用值`，没靠短时窗刷低 slope/hitch
- [ ] 重跑 metrics-audit 验了没回归（第 4 步）
- [ ] 优化前后指标对比做了（若 OPT_TARGET 类）
- [ ] 状态标注做了（✅ → ✅🔄 优化中 / 增量中）
- [ ] 状态升维做了（✅🔄 → ✅v2-optimized / ✅v2-extended）
- [ ] §10 修订表加了版本说明（占位行改为正式行）

任一项未过 → 回对应步骤修。

---

## 附：与 wr-quality / wr-close / metrics-audit / wr-rework 的接口

| 接口方向 | 内容 |
|----------|------|
| **入：用户优化/增量请求** | 已关 R id + OPT_TARGET / INCR_TARGET → 第 0 步前置校验 |
| **出：交 metrics-audit 重审** | 第 3 步取新 JSON 后，交 metrics-audit 跑 §1–§5 验没回归（第 4 步） |
| **出：交 wr-close 重关** | 第 4 步验没回归后，交 wr-close 走第 5 步回写 docs §2 状态列 + §10 修订表（第 5 步） |
| **不接：未关 R 的首次关闭** | ⬜ 状态 → 交 wr-quality + wr-close 走首次关闭 |
| **不接：门禁 FAIL 的反攻修复** | 🔄 质量返工中 / 门禁 FAIL → 交 wr-rework 走反攻回流 |
| **不接：类 A 指标层 bug** | JSON marshal bug → 交 metrics-audit 修 |
| **不接：类 C 引擎洞** | 复杂场景下引擎炸 → 交 wr-rework 第 3 步定点修 |

**关键边界：** 本 skill 只接**已关 R（✅）的优化/增量回流**。未关 R 的首次关闭归 wr-close；门禁 FAIL 的反攻修复归 wr-rework；指标层 bug 归 metrics-audit；写代码前的标准定归 wr-quality。5 个 skill 各管一段，正向写、反向修、横向优化都有 skill 接，这才闭环。
