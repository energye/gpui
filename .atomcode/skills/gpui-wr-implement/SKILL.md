---
name: gpui-wr-implement
description: §R 主能力的**能力实现回流**——在写 R 真窗**之前**，先在 `ui/rendering/`（或 `ui/embedder/` / `ui/scene/` / `ui/io/`）里实现或补全该 R 的能力，走 RENDER_BASE §0.6 三项验收的 **① 单测 + ② 指标接线**，然后才交 `gpui-wr-quality` 写真窗（③ 窗测）。当用户说「实现 R7 能力」「实现 R7」「先做 R7 能力」「把 R7 能力实现了」「implement R7」「R7 能力开发」「先实现再写真窗」时触发。与 `gpui-wr-quality`（写真窗）、`gpui-wr-close`（执行关闭）、`gpui-metrics-audit`（指标审查）、`gpui-wr-rework`（反攻修复）、`gpui-wr-optimize`（优化增量）互补，专门接「能力未实现/不完整 → 实现 + 单测 + 指标接线」这条回流。
user_invocable: true
disable_model_invocation: false
---

# gpui-wr-implement — §R 主能力实现回流（能力 → 单测 → 指标接线）

> **与现有 skill 的分工：**
> - `gpui-wr-quality` = 关 R **前**的真窗质量标准（场景矩阵 + HUD + 实现点六维）——**写真窗前**
> - `gpui-wr-close` = 关 R 的**执行流程**（建窗/跑/判门禁/回写 docs §2）——**写完真窗后**
> - `gpui-metrics-audit` = 指标族 A–J 的**正误审查与 bug 修复**——**指标层**
> - `gpui-wr-rework` = 门禁 FAIL / 指标装绿的**反攻/修复回流**——**代码层 + 引擎层**（修 bug）
> - `gpui-wr-optimize` = 已关 R（✅）的**优化/增量回流**——**性能 + 场景 + HUD + 指标**（加深）
> - **本 skill** = 写 R 真窗**之前**，先在 `ui/` 里**实现或补全**该 R 的能力，走 **① 单测 + ② 指标接线**，然后才交 `wr-quality` 写真窗——**能力实现层**
>
> **真源：** `docs/ENGINE_UI_RENDER_BASE.md` §0.6「每项能力验收（硬 · 三项全过才算完成）」+ §22.1 施工表（序 1–13 + 指标骨架）+ §22.2「收口后残项」+ §15 滚动/视口母表；`docs/ENGINE_UI_WIDGET_RENDER.md` §2 主表（该 R 行的能力定义 + 指标门禁）+ §6 模块落点。若本 skill 与真源矛盾，以真源为准——发现矛盾停下报告，不要自决。

## 0. 为什么要这个 skill

历史 `ui_wr_*` 真窗流程有一个**隐含断层**：

```
实际开发流程：
  ① 先在 ui/ 里实现 R 能力（如 VirtualList、ScrollRerecord、AsyncImage）
  ② 写单测 + 接指标字段（bind_count、scroll_rerecord、measure_cache_hit 等）
  ③ 写 examples/ui_wr_*/ 真窗，跑 GPU，判 §2.2 全族门禁，回写 docs §2

现有 skill 的覆盖：
  wr-quality + wr-close + metrics-audit + wr-rework + wr-optimize
  全都从 ③ 开始（写真窗），① ② 这段没有 skill 接
```

**断层表现：**

- 用户说「写 R7 真窗」，命中的 `wr-quality` 第 3 步「先写场景设计」**假设 R7 能力（VirtualList）已实现**
- 若 VirtualList 还没实现或不完整，`wr-quality` 没有「能力未实现该怎么办」分支
- 结果可能是：写一个**简陋真窗**（用普通列表冒充虚拟列表），跑出来 `bind_count = item_count`（虚拟化失效），门禁 FAIL，走 `wr-rework` 反攻——**绕一大圈才回到「该实现能力」这个根本问题**

本 skill 把「能力现状审查 → 实现/补全 → 单测 → 指标接线」这条回流**固化**，确保真窗写之前能力已就绪。

---

## 1. 触发与输入

用户可能给：

- 一个 R/C id + 「实现」请求（如「实现 R7 能力」「先做 R7 能力」「implement R7」）→ 能力实现回流
- 一个 R/C id + 「先实现再写真窗」请求 → 本 skill 实现 + 交 wr-quality 写真窗
- 一个「能力不完整」怀疑（如「R7 的 VirtualList 好像缺 BindCount」「R10 异步图没接 worker」）→ 定向审查 + 补全

从输入解析：

- `ABILITY_ID`（必给）：如 `R7`、`R7b`、`R10`、`C3`
- `SUSPECT_GAP`（若给）：定向审查的能力缺口（如「BindCount 没接」「scroll_rerecord 没字段」）

## 第 0 步：读真源 + 定位 + 能力清单提取

**必做**——每次都先读，禁止凭记忆跑流程（真源会变）。

### 0.1 读 WIDGET_RENDER §2 主表该 R 行

`read_file docs/ENGINE_UI_WIDGET_RENDER.md`，定位 §2 主表里该 id 行，取五列硬值：

- `ABILITY`（能力定义，如 R7 = 「虚拟化宿主」）
- `PACKAGE`（如 `ui_wr_r7_virtlist`）
- `WINDOW` = **1200×800**（必须是这个；不是就停下，文档被改错了）
- `CLOSE_SECONDS`（如 R7=60；§2.5 关闭用时长表与此不一致以 §2.5 为准）
- `指标门禁（须 FAIL）`（如 R7 = 「`bind_count≪item_count`；p95/hitch；RSS」）
- `WAVE`（波次，如 W3）
- 当前 `状态`（⬜ / ✅ / 🔄）

### 0.2 读 RENDER_BASE §22.1 施工表该 R 对应的序

`read_file docs/ENGINE_UI_RENDER_BASE.md`，定位 §22.1 施工表，按该 R 能力找对应序（如 R7 虚拟化宿主 → 序 12 滚动/视口）：

| 列 | 取什么 |
|----|--------|
| 序 | 该 R 对应的施工序号（1–13） |
| 能力（母表） | 该序对应的能力族（如序 12 = §15 滚动/视口） |
| 前驱 | 该序的前驱序（如序 12 前驱 = 3,7）——前驱没做完不要跳 |
| ① 单测 | 该序要写的单测（如序 12 = `index↔offset；ScrollToIndex；S5`） |
| ② 指标 | 该序要接的指标（如序 12 = `bind 上界；layout 不风暴`） |
| ③ 窗测 | 该序要写的窗测（如序 12 = `scroll`）——本 skill **不做** ③，交 wr-quality |

### 0.3 读 RENDER_BASE §22.2 收口后残项（若该序已收口）

若 §22.1 该序状态是 `✅/🔄`（已收口或收口中），读 §22.2 该序的「残项」列——确认本 R 要实现的能力是否在残项范围内（如序 12 残项 = 「multi-sliver；BouncingPhysics」）。

**若残项超出本波范围**（如 multi-sliver 属 W6）：停下问用户，这是远期 D 行，不属本波。

### 0.4 读 RENDER_BASE 对应母表 §（能力清单）

按该 R 能力找对应母表节：

| R 能力 | 对应母表节 | 能力清单 |
|--------|------------|----------|
| R7 虚拟化宿主 | §15 滚动/视口 | VirtualList + BindCount + ItemCount + OffsetOfIndex + IndexAtOffset + ScrollToIndex + CacheExtent |
| R7b 滚动少重录 cell | §15 + §13 | ScrollRerecord 上限 + 静 cell 保持 + 新入视口才重录 |
| R10 图异步→局部脏 | §7 图像 + §13 | AsyncImage + worker 解码 + 出图后 rerecord 仅一格 |
| R6 层动画 | §6 Transform + §10 滤镜 | OpacityLayer + TransformLayer + ClipLayer 动画 + hitch |
| R8 Overlay 独立合成 | §11 PaintingContext + Overlay | Overlay 独立 band + 主树 paint_count 不涨 |
| R14 缓存预算/淘汰 | §10 SaveLayer + Budget | cache_entries + RSS slope FAIL + 超预算仍正确 |
| R15 UI/raster 所有权·长跑 | §14 调度 + §20 指标 | soak 300s + 无崩 + 无读回 + hitch/CPU/RSS |
| R17 不可见降频 | §14 调度 | 后台 interval 明显变大 + 最小化段 |
| R20 Filter 层 | §10 滤镜 | 子树灰/糊 + 外不变 + 局部 rerecord |
| R21 壳/内容分层 | §11 + §13 | 顶栏 rerecord=0 + 体滚 + 壳分层 |
| R22 选区/光标局部脏预留 | §8 文本 | stub API + 字段可 0 + 防将来全窗刷 |

### 0.5 读 WIDGET_RENDER §6 模块落点

`read_file docs/ENGINE_UI_WIDGET_RENDER.md` §6 模块落点表，确认该 R 能力该落哪个包：

| 能力 | 包 |
|------|-----|
| Present / PaintPresentTree / policy | `ui/embedder` |
| 脏 layout/paint、VirtualList | `ui/rendering` |
| Picture、Layer、Composite | `ui/scene` |
| Overlay | `ui/overlay` |
| 指标 | `ui/scheduler` |
| **全部真窗** | `examples/ui_wr_*` only |

**关键边界：** 能力实现落 `ui/`，真窗落 `examples/`。本 skill 只动 `ui/`，**不**动 `examples/`（真窗归 wr-quality）。

## 第 1 步：能力现状审查

### 1.1 定位能力文件

按 §0.5 模块落点 + §0.4 能力清单，`glob` / `grep` 定位现有能力文件：

```bash
# 例：R7 虚拟化宿主
ls ui/rendering/virtual_list.go ui/rendering/viewport.go ui/rendering/scrollable.go 2>/dev/null
grep -n "BindCount\|ItemCount\|OffsetOfIndex\|IndexAtOffset\|ScrollToIndex\|CacheExtent" ui/rendering/virtual_list.go
```

### 1.2 能力清单缺口审查

对照 §0.4 能力清单，逐项查是否已实现：

| 检查项 | 审查方法 |
|--------|----------|
| 能力文件是否存在 | `glob ui/rendering/<file>.go` |
| 关键函数/字段是否存在 | `grep -n "<func>/<field>" ui/rendering/<file>.go` |
| 关键函数签名是否完整 | `read_symbol ui/rendering/<file>.go <func>` |
| 单测是否存在 | `glob ui/rendering/<file>_test.go` |
| 单测是否覆盖关键路径 | `grep -n "Test<Func>" ui/rendering/<file>_test.go` |
| 指标字段是否已接 metrics.go | `grep -n "<field>" ui/scheduler/metrics.go` |
| 指标字段是否在 FrameMetrics struct | `read_symbol ui/scheduler/metrics.go FrameMetrics` |

### 1.3 输出能力现状报告

```
📋 R<id> 能力现状审查

能力定义：<WIDGET_RENDER §2 该 R 行 ABILITY 列>
施工序：<RENDER_BASE §22.1 该 R 对应的序>
前驱序：<序>（状态：✅/🔄/⬜）
模块落点：<ui/rendering / ui/embedder / ui/scene / ui/overlay>

能力清单（§0.4）缺口审查：
  [能力 1]：<已实现 / 缺函数 / 缺字段 / 完全缺失>
  [能力 2]：<已实现 / 缺函数 / 缺字段 / 完全缺失>
  ...

单测现状：
  <file>_test.go：<存在 / 不存在>
  覆盖关键路径：<是 / 否——缺 Test<Func>>

指标接线现状：
  <field>：<已在 FrameMetrics struct / 缺字段 / 字段名不一致>
  ...

缺口清单：
  1. <缺口 1——如 BindCount 字段已在但没接 metrics.go>
  2. <缺口 2——如 ScrollToIndex 单测缺>
  3. <缺口 3——如 scroll_rerecord 字段完全缺失>
```

### 1.4 前驱序检查（硬）

§22.1 该序的「前驱」列，逐个查前驱序是否已收口：

- 前驱序状态 `✅` → 通过
- 前驱序状态 `🔄` 或 `⬜` → **停下报告**：「该 R 的前驱序 <序> 还没收口（状态 <🔄/⬜>），先做前驱。」

**禁止跳前驱。** RENDER_BASE §22.0 明确「前驱未完成不要跳」。

## 第 2 步：实现或补全能力

### 2.1 实现策略选择

按 §1.3 缺口清单的严重程度选实现策略：

| 缺口严重度 | 策略 |
|------------|------|
| 完全缺失（能力文件不存在） | 从 §0.4 能力清单逐项实现，新建 `<file>.go` |
| 部分缺失（文件在但函数/字段缺） | `edit_file` 补缺的函数/字段，不动现有已实现部分 |
| 单测缺 | 新建 `<file>_test.go` 或 `edit_file` 补 `Test<Func>` |
| 指标字段缺 | 交第 4 步指标接线 |
| 字段名与 metrics.go 不一致 | 交第 4 步对齐字段名 |

### 2.2 实现纪律

- **只动 `ui/`，不**动 `examples/`（真窗归 wr-quality）
- **禁止 ui→gpu 依赖**（CODING_RULES §5：架构 vs 示例边界 + §0.4 工程纪律）
- **禁止 CGO**（purego）
- 字段名取自 `ui/scheduler/metrics.go` 的 `FrameMetrics` struct，**禁止自创字段名**
- 实现 VirtualList 等能力时，按 RENDER_BASE §22.1 该序的「① 单测」列提前规划单测覆盖
- 若实现触及引擎架构（如要加 multi-RT 层纹理 compositor）→ 停下报告，这是 W6 架构，不属本波

### 2.3 实现动作

按缺口清单逐项实现：

```go
// 例：R7 VirtualList 补 BindCount 接线
// ui/rendering/virtual_list.go
type VirtualList struct {
    ItemCount  int
    BindCount  int  // 已存在——但需确认接线到 metrics.go
    // ...
}
```

```go
// 例：R7b 补 scroll_rerecord 字段
// ui/rendering/virtual_list.go 或 scrollable.go
type ScrollRerecordStats struct {
    ScrollRerecord int  // 滚动时重录的 cell 数
}
```

```go
// 例：R10 异步图 worker 解码
// ui/io/decode.go
func DecodeFileAsync(path string, cb func(ImageBuf, error)) { ... }
```

### 2.4 实现完 → 交第 3 步跑单测

## 第 3 步：跑单测（RENDER_BASE §0.6 ① 单测）

### 3.1 跑该 R 相关包的单测

```bash
# 例：R7 VirtualList
go test ./ui/rendering -run 'TestVirtualList|TestViewport|TestScroll' -count=1 -v

# 例：R10 异步图
go test ./ui/io -run 'TestDecode|TestAsyncImage' -count=1 -v

# 例：R6 层动画
go test ./ui/scene -run 'TestTransform|TestOpacity|TestClip' -count=1 -v
```

### 3.2 单测结果处理

| 结果 | 处理 |
|------|------|
| 全绿 | 交第 4 步指标接线 |
| 编译错 | `read_file` 看错，`edit_file` 修，重跑 |
| 单测 FAIL | `read_file` 看失败的单测，定位是实现 bug 还是单测期望错——**实现 bug 修实现；单测期望错修单测**（但若单测期望是「能力该有」的，不许改单测降低期望） |
| 单测缺（该跑的 Test<Func> 不存在） | 回第 2 步补单测，再跑 |

### 3.3 跑全 ui 包回归

```bash
go test ./ui/... -count=1
```

**全绿才能进第 4 步。** 若有回归（之前绿的测现在红了），回第 2 步排查实现是否破坏了已有能力。

## 第 4 步：指标接线（RENDER_BASE §0.6 ② 指标）

### 4.1 定位指标字段

`read_file ui/scheduler/metrics.go`，`read_symbol FrameMetrics`——确认该 R 能力专用指标字段是否已在 struct 里：

| R 能力 | 能力专用字段 | 来源 |
|--------|--------------|------|
| R7 虚拟化宿主 | `bind_count` | `VirtualList.BindCount` |
| R7b 滚动复用 | `scroll_rerecord` | `VirtualList.ScrollRerecord` 或 `Scrollable.ScrollRerecord` |
| R10 异步图 | `img_decode_ms` / `img_upload_n` | `ui/io/decode.go` worker |
| R6 层动画 | `hitch_count` / `hitch_rate_per_min` / `fps_interval` | `MetricsStore` 帧间隔环 |
| R8 Overlay | `overlay_paint_count` / 主树 `paint_count` | `Overlay` 独立 band 统计 |
| R14 缓存预算 | `cache_entries` / `rss_slope_kb_per_min` | `SaveLayerBudget` + `ProcessTracker` |
| R15 soak | `hitch_rate_per_min` / `cpu_pct_avg` / `rss_*` | `MetricsStore` + `ProcessTracker` |
| R17 后台降频 | `interval_avg_ms`（后台 vs 前台对比） | `MetricsStore` 帧间隔环 |
| R20 Filter 层 | `filter_layer_count` / `raster_layer_count` | `ImageFilterLayer` 统计 |
| R21 壳分层 | `shell_rerecord` / `body_rerecord` | 壳层 vs 体层 rerecord 统计 |
| R22 选区 stub | `selection_dirty_layer_ids`（stub 可 0） | 选区局部脏预留字段 |

### 4.2 字段接线三种情况

| 情况 | 处理 |
|------|------|
| 字段已在 FrameMetrics struct 且已接 PipelineApp | 通过——进第 5 步 |
| 字段已在 struct 但没接 PipelineApp（JSON 不输出） | `edit_file ui/embedder/pipeline_app.go` 补接线（在 Present 后采样） |
| 字段完全不在 struct | **停下报告**：「该 R 能力专用字段 `<field>` 不在 `FrameMetrics` struct，需先加字段。这是指标层变更，建议交 `metrics-audit` 审查字段名一致性后再加。」 |

### 4.3 字段名一致性审查（交 metrics-audit）

若第 4.2 步要新增字段，**禁止自创字段名**。交 `metrics-audit` 第 4 步「指标与代码观测一致性审查」：

- 字段名必须与 `FrameMetrics` struct 一致
- JSON marshal 必须用 struct 名
- 累计 vs 每帧必须正确（如 `bind_count` 是当前已挂子节点数，非累计）

**metrics-audit 审通过** → `edit_file ui/scheduler/metrics.go` 加字段，`edit_file ui/embedder/pipeline_app.go` 接线，交第 5 步。

## 第 5 步：交 wr-quality 写真窗

### 5.1 能力就绪判定

本 skill 完成判定（三项全过）：

| 判定项 | 标准 |
|--------|------|
| ① 能力实现 | §0.4 能力清单逐项已实现，`grep` 关键函数/字段全命中 |
| ① 单测绿 | `go test ./ui/rendering -run Test<Func>` 绿 + `go test ./ui/...` 全绿无回归 |
| ② 指标接线 | §4.1 能力专用字段已在 `FrameMetrics` struct 且已接 `pipeline_app.go` |

**三项全过 → 能力就绪，交 wr-quality 写真窗。**

### 5.2 交接 wr-quality

`use_skill` 加载 `gpui-wr-quality`，告知：

- 该 R 能力已实现（`ui/rendering/<file>.go` 的 `<func>` 已就绪）
- 单测已绿（`Test<Func>` 全过）
- 指标字段已接（`<field>` 已在 `FrameMetrics` struct 且已接 `pipeline_app.go`）
- **请按 wr-quality §5 流程写 `examples/ui_wr_<package>/` 真窗（③ 窗测）**

wr-quality 写完真窗后，交 `wr-close` 跑 GPU + 判门禁 + 回写 docs §2。

### 5.3 若用户请求是「先实现再写真窗」

本 skill 走完第 1–4 步，**自动串接** wr-quality 写真窗（§5.2），再串 wr-close 关闭——完整一条龙：

```
wr-implement（能力实现 + 单测 + 指标接线）
  → wr-quality（写真窗 + 场景矩阵 + HUD + 实现点六维）
  → wr-close（跑 GPU + 判门禁 + 回写 docs §2）
  → metrics-audit（审指标诚实性，串在 wr-close 判门禁时）
  → 若 FAIL → wr-rework（反攻修复）
```

## 输出格式

完成后给用户一份结构化报告：

```
✅ R<id> 能力实现收口

能力定义：<WIDGET_RENDER §2 该 R 行 ABILITY 列>
施工序：<RENDER_BASE §22.1 序>（前驱序 <序> 状态 ✅）

能力实现：
  ui/rendering/<file>.go（<实现摘要>）
  关键函数/字段：<逐项列出——已实现>

单测：
  ui/rendering/<file>_test.go（<单测摘要>）
  go test ./ui/rendering -run Test<Func>  PASS
  go test ./ui/...  PASS（无回归）

指标接线：
  <field>：已在 FrameMetrics struct + 已接 pipeline_app.go
  ...

能力就绪：三项全过（能力实现 + 单测绿 + 指标接线）
→ 交 wr-quality 写真窗

下一步：wr-quality 写 examples/ui_wr_<package>/ 真窗
```

若有任一 FAIL：

```
⚠️ R<id> 能力实现未达标

当前阶段：<能力实现 / 单测 / 指标接线>
FAIL 原因：<具体>
下一步建议：
  - 单测 FAIL → 修实现或修单测（不许降单测期望）
  - 指标字段缺 → 交 metrics-audit 审字段名后加
  - 前驱序未收口 → 先做前驱
```

## 禁令自检（每次结束前过一遍）

- [ ] **只动了 `ui/`，没动 `examples/`**（真窗归 wr-quality）
- [ ] **没引入 ui→gpu 依赖**（CODING_RULES §5）
- [ ] **没用 CGO**（purego）
- [ ] 字段名取自 `FrameMetrics` struct，**没自创字段名**
- [ ] 前驱序已收口（没跳前驱）
- [ ] 单测绿 + `go test ./ui/...` 全绿无回归
- [ ] 指标字段已在 `FrameMetrics` struct 且已接 `pipeline_app.go`
- [ ] 若要新增字段，交了 metrics-audit 审字段名一致性
- [ ] 没触及引擎架构（如 multi-RT 层纹理 compositor 属 W6）
- [ ] 能力实现按 §0.4 能力清单逐项覆盖，没漏
- [ ] 若残项超出本波范围（如 multi-sliver 属 W6），停下问了用户

任一项未过 → 回对应步骤修。

---

## 附：与其他 5 个 skill 的接口

| 接口方向 | 内容 |
|----------|------|
| **入：用户实现请求** | R id + 「实现」/「先实现再写真窗」/「能力不完整」 → 第 0 步 |
| **出：交 wr-quality 写真窗** | 第 5 步能力就绪后，交 wr-quality 按 §5 流程写 `examples/ui_wr_*/` 真窗 |
| **出：交 metrics-audit 审字段名** | 第 4 步要新增字段时，交 metrics-audit 第 4 步审字段名一致性 |
| **不接：真窗测试** | 真窗的写/跑/判门禁归 wr-quality + wr-close |
| **不接：指标层 JSON marshal bug** | 字段缺/null 无原因/JSON 字段名不一致 → 交 metrics-audit |
| **不接：门禁 FAIL 反攻修复** | 门禁 FAIL 反攻归 wr-rework（本 skill 是「写真窗前」的能力实现，wr-rework 是「写完真窗后门禁 FAIL」的反攻） |
| **不接：已关 R 优化/增量** | 已关 R（✅）优化/增量归 wr-optimize |

**关键边界：** 本 skill 只接「**写 R 真窗之前**的能力实现 + 单测 + 指标接线」。6 个 skill 各管一段：能力实现（wr-implement）→ 写真窗（wr-quality）→ 跑指标（wr-close 串 metrics-audit）→ 反攻修复（wr-rework）→ 优化增量（wr-optimize），全闭环。

---

## 附：RENDER_BASE §22.1 施工表序 → R 映射速查

| 施工序 | 能力族 | 对应 R | 模块落点 |
|--------|--------|--------|----------|
| 1 | 坐标/DPR/Hit §16 | R19（1px snap） | `ui/rendering` + embedder |
| 2 | Pipeline/脏区/RO §13 | R0/R1/R2/R12/R12b | `ui/rendering` + embedder |
| 3 | PaintContext 基础 §11 | 所有 R 前驱 | `ui/rendering` |
| 4 | 调度/VSync §14 | R17（后台降频） | `ui/scheduler` |
| 5 | 几何+Paint/Shader §2§3 | R0/R19 | `ui/rendering` |
| 6 | Path §4 | R0/R19 | `ui/rendering` |
| 7 | Clip/saveLayer §5 | R18/R20/R22 | `ui/rendering` |
| 8 | Transform 层 §6 | R6/R8 | `ui/scene` + rendering |
| 9 | 图像 §7 | R10 | `ui/io` + rendering |
| 10 | 文本/Paragraph §8 | R9/R22 | `ui/rendering` |
| 11 | 滤镜/阴影层 §10§12 | R20 | `ui/scene` + rendering |
| 12 | 滚动/视口 §15 | **R7/R7b** | `ui/rendering` |
| 13 | Picture/局部 Present §9§18.3 | R5/R10/R21 | `ui/scene` + rendering |
| 指标骨架 | §20 | 所有 R | `ui/scheduler` + embedder |

**本 skill 适用所有施工序**——只要该 R 能力还没实现或不完整，都走本 skill 实现 + 单测 + 指标接线，再交 wr-quality 写真窗。
