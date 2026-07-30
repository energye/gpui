# gpui 真窗 Skill 使用手册（6 skill 收敛版）

> **版本：** 3.5 | 日期：2026-07-30
> **范围：** 本手册说明 `.grok/skills/` 下 6 个 skill 的**使用方法、触发词、串联闭环**。
> **真源：** `docs/ENGINE_UI_WIDGET_RENDER.md` v3.5（状态：§2/§3/§5/§10）+ `docs/ENGINE_UI_RENDER_BASE.md` + `AGENTS.md`。skill 是真源的执行骨架，若 skill 与真源矛盾，以真源为准。
> **配套：** 6 个 skill 的完整定义见 `.grok/skills/<skill-name>/SKILL.md`（或项目内 skill 目录）。

**真窗硬规则：**

- 状态只写 **§2 主表（R）+ §3 组合表（C）+ §5 分期（W）+ §10 修订**
- 每个 W 的**每一个主能力 R** 必须有独立 `examples/ui_wr_r*` 真窗（U5）；禁止用组合窗代替单 R
- 每个 W 的**每一个组合窗 C** 必须有独立 `examples/ui_wr_c*` 真窗（U6）；C 只做集成

**v2.0 → v3.0 升级摘要：** 新增 `gpui-wr-debug`（§R 真窗 bug 修复调度入口），解决「指定窗口出现 bug 时缺统一入口」断层。原 5 skill 职责不变，`wr-debug` 作为**调度入口层**接入，按 AGENTS.md 分层风险规则回流到对应 skill 闭环。

---

## 1. 6 个 skill 一览（收敛后）

| # | Skill | 职责 | 核心动词 | 属性 |
|---|-------|------|----------|------|
| 1 | `gpui-wr-debug` | 指定窗口 bug 修复调度入口（强制复现 → 定位层 → 分层回流） | 修 bug | **调度入口层** + ui 低风险自修 |
| 2 | `gpui-wr-implement` | 能力实现回流（ui/ 里实现 + 单测 + 指标接线） | 实现 | 能力实现层 |
| 3 | `gpui-wr-close` | 关 **R 与 C** 完整生命周期（各独立真窗；3 模式） | 关 | 关 R/C 生命周期层 |
| 4 | `gpui-metrics-audit` | 指标族 A–J 正误审查与 bug 修复（指标层） | 审 | 指标横切层 |
| 5 | `gpui-wr-engine` | 底层修复回流（render/gpu/ui-scene 谨慎改 + 跨层影响面评估） | 修底层 | 底层修复层 |
| 6 | `gpui-wr-rewrite` | W 矩阵级调度（推翻 W<n> 重写，批量调度 + W 矩阵级状态治理） | 调度 | W 矩阵调度层 |

**核心分工边界：**

- `wr-debug`：**调度入口层**——指定窗口出现 bug 时强制复现 + 定位 bug 所在层，按 AGENTS.md 分层风险规则回流对应 skill；ui/rendering / ui/embedder / ui/scene / ui/io 低风险本 skill 自修
- `wr-implement`：能力实现层（ui/ 里实现 + 单测 + 指标接线）
- `wr-close`：关 R/C 完整生命周期（3 模式；含定标准 / FAIL 回流 / 优化）
- `metrics-audit`：横切层，审指标诚实性，串在 wr-close 判门禁时
- `wr-engine`：底层修复层（render/gpu 谨慎改 + 跨层影响面评估）
- `wr-rewrite`：W 矩阵级调度层（推翻整波重写，调度上面 5 个 skill）

**收敛说明（v1.0 → v2.0）：**

| 原 skill（v1.0） | 收敛到哪（v2.0） |
|------------------|------------------|
| `wr-quality`（写前定标准） | 并入 `wr-close` 模式 1 第 1 步「定标准」 |
| `wr-rework` 类 B（门禁 FAIL 重写真窗） | 并入 `wr-close` FAIL 内置回流（第 4.2 步） |
| `wr-optimize`（已关 R 优化/增量） | 并入 `wr-close` 模式 3 优化重关 |
| `wr-rework` 类 C（引擎洞定点修） | 独立成 `wr-engine`（底层修复回流） |
| `wr-implement` / `metrics-audit` | 保持独立不变 |

**v3.0 新增 `wr-debug` 的断层背景：**

v2.0 的 5 skill 覆盖了「正向写、反向修、横向优化、底层穿透、W 矩阵级调度」，但**缺一个「指定窗口出现 bug 时的统一入口」**。用户说「修 R3 的 bug——boundary 文不 skip」时，没有 skill 直接接：

- 凭描述直接改 `examples/ui_wr_r3_boundary/main.go` = 绕洞（bug 可能在 `ui/rendering/boundary_cache.go`）
- 不强制复现就改代码 = 改完不知道修没修
- `render/` `gpu/` 不停下确认擅自改 = 违反 AGENTS.md，可能破坏所有 R 的 Present

`wr-debug` 把「指定窗口 bug → 强制复现取 JSON → 对照 §2.2 FAIL 线定位层 → 按 AGENTS.md 分层风险规则回流」这条回流**固化**，是**调度入口层** + ui 低风险自修层，不替代被回流的 skill 的内部闭环。

---

## 2. 触发词速查表

### 2.1 `gpui-wr-debug`（指定窗口 bug 修复调度入口 · v3.0 新增）

| 触发词 | 场景 | 自动回流哪个 skill |
|--------|------|--------------------|
| 「修 R3 的 bug」「debug R7」「R4 渲染错」 | 指定窗口出现 bug，定位修复 | 见下方回流决策树 |
| 「R12 闪屏 fix」「R11 boundary 漏 skip」 | 现象 + 能力点 | 同上 |
| 「R14 slope 偷放」「R9 measure_cache 假值」「R4 fps 假值」 | 指标假/偷放怀疑 | 交 `metrics-audit` |
| 「指定窗口 $Rn 出现 $现象定位修复」 | 综合请求 | 强制复现 → 定位层 → 分层回流 |

**`wr-debug` 回流决策树（按 AGENTS.md 分层风险规则）：**

```
指定窗口 bug
  ↓
第 1 步：强制复现——跑该 R 真窗取 §2.2 全族 JSON（不跑就凭描述改 = 绕洞）
  ↓
第 2 步：定位 bug 所在层（对照 §2.2 FAIL 线 + §6 模块落点，不凭 BUG_DESC 猜层）
  ↓
第 3 步：分层回流
  ├─ ui/rendering / ui/embedder / ui/scene / ui/io（低风险）+ 能力实现缺陷
  │   → 本 skill 第 3.2 步 ui 低风险自修（edit_file + 单测 + 重跑真窗验证）
  │
  ├─ render/（高风险）
  │   → 停下 question 确认（AGENTS.md：render/ 必须 question 确认后修）
  │   └─ 用户确认 → 交 wr-engine
  │
  ├─ gpu/（最高风险）
  │   → 停下 question 确认（AGENTS.md：gpu/ 最高风险，必须 question 确认后修）
  │   └─ 用户确认 → 交 wr-engine
  │
  ├─ 指标层（不诚实 / 门禁偷放 / 字段名不一致 / null 无原因）
  │   → 交 metrics-audit 审指标诚实性
  │
  ├─ 已关 ✅ 真窗代码层 bug（U17 场景简陋 / HUD 不见 / 相位缺 / 实现点缺）
  │   → 交 wr-close 模式 2（反攻重关）
  │
  ├─ 已关 ✅ 性能 / 场景 / HUD / 指标 优化增量
  │   → 交 wr-close 模式 3（优化重关）
  │
  └─ 能力根本没实现（如 R7 VirtualList 文件不存在）
      → 交 wr-implement（能力实现回流）
```

**`wr-debug` 输入校验（硬）：**

1. `ABILITY_ID` 必给。没给 → 停下问用户「要修哪个窗口的 bug？（R<C><id>）」
2. 该 R 当前状态：
   - `⬜`（未关）→ 停下问用户「R<id> 还没关，bug 修复前提是「已有真窗可复现」。要先首次关闭（走 wr-close 模式 1）吗？」
   - `✅` / `🔄` / `✅v2` / `✅🔄` → 继续

**`wr-debug` 不接的范围：**

- W 矩阵级批量调度（归 `wr-rewrite`）
- 状态升维（§2 主表该 R 行「状态」列升维归被回流的 skill 改，本 skill 只回写 §10 修订表）

### 2.2 `gpui-wr-implement`（能力实现回流）

| 触发词 | 场景 |
|--------|------|
| 「实现 R7 能力」「实现 R7」「先做 R7 能力」 | 写真窗前的能力实现 |
| 「把 R7 能力实现了」「implement R7」「R7 能力开发」 | 显式点名 |
| 「先实现再写真窗」「R7 不完整」 | 串接或能力现状审查 |

### 2.2 `gpui-wr-close`（关 R 完整生命周期，3 模式）

| 触发词 | 场景 | 自动选哪个模式 |
|--------|------|----------------|
| 「关闭 R7」「关掉 R10」「把 R3 收了」 | 关闭一个 §R 主能力真窗 | 模式 1 首次关闭（R 当前 ⬜） |
| 「反攻 R3」「重写 R4」「返工 R5」 | 门禁 FAIL / 推翻重写升维 | 模式 2 反攻重关（R 当前 ✅ 或 🔄） |
| 「优化 R4」「R7 加场景」「R4 提性能」 | 已关 R 优化/增量 | 模式 3 优化重关（R 当前 ✅） |

**模式选择规则：** `read_file docs/ENGINE_UI_WIDGET_RENDER.md` §2 主表该 R 行的「状态」列，按当前状态自动选模式。状态与触发词冲突时停下问用户。

### 2.3 `gpui-metrics-audit`（指标层审查）

| 触发词 | 场景 |
|--------|------|
| 「审 R3 的指标」「metrics-audit R4」「指标族查 bug」 | 指标族正误审查 |
| 「fps 假值」「vsource 假锁」「slope 偷放」「cpu 双 0 装绿」「降画质装绿」 | 怀疑任一 ui_wr_* JSON 指标不诚实/不完备/被偷放阈值 |

### 2.4 `gpui-wr-engine`（底层修复回流）

| 触发词 | 场景 |
|--------|------|
| 「修底层」「render 不满足」「gpu 谨慎改」 | 底层修复回流 |
| 「定点修引擎」「类 C 引擎洞」「底层依赖不满足」 | 显式点名 |
| 「BoundaryCache 文不 skip 修哪」「retained 下壳闪修 pipeline_app」 | 定向修底层 |

### 2.5 `gpui-wr-rewrite`（W 矩阵级调度）

| 触发词 | 场景 |
|--------|------|
| 「推翻 W2 重写」「W0–W6 全部重写」「重写整个 W3」 | W 矩阵级推翻重写调度 |
| 「rewrite W4」「批量重写 R7/R7b/R10」「重写整波」 | 显式点名或指定 R 集合 |

---

## 3. 完整闭环图

```
                ┌─── [wr-implement] 实现能力 ──┐
                │   (ui/rendering/ + 单测 + 指标接线)  │
                │                                    │
   新 R  ──────►│                                    ▼
                │                            [wr-close] 关 R 完整生命周期
                │                            (3 模式：首次关闭 / 反攻重关 / 优化重关)
                │                                    │
                │                                    ▼
                │                            跑GPU取JSON (wr-close 第 3 步)
                │                                    │
                │                            ┌───────┴───────┐
                │                            ▼               ▼
                │                    底层不满足？        判门禁+审指标
                │                    ├─ 是 → [wr-engine] 修底层
                │                    │       └─ 修完回 wr-close 第 3 步重跑
                │                    └─ 否 → 继续
                │                                    │
                │                            ┌───────┴───────┐
                │                            ▼               ▼
                │                        FAIL 内置回流    真绿
                │                        (回第 2 步重写)    │
                │                                    │       │
                │                                    └──► 重跑
                │                                            │
                │                                            ▼
                │                                    回写 docs §2 + §10
                │                                            │
                │                                            ▼
                │                                    R 关闭 ✅ / ✅v2
                │
                │   ┌── [wr-rewrite] W 矩阵级调度 ─────────────────┐
                │   │   (推翻 W<n> 重写，批量调度下面 5 个 skill)    │
   已关 R bug ─►│   │   调度 wr-implement / wr-close 3 模式 /         │
                │   │           wr-engine / metrics-audit             │
                │   └─────────────────────────────────────────────────┘
                │
                │   ┌── [wr-debug] 指定窗口 bug 修复调度入口 ────────┐
                │   │   (强制复现 → 定位层 → 按 AGENTS.md 分层回流)   │
        bug  ──►│   │   ui/* 低风险自修 / render+gpu 停下确认交       │
                │   │   wr-engine / 指标层交 metrics-audit /          │
                │   │   已关 ✅ 交 wr-close 模式 2/3 / 能力没实现     │
                │   │   交 wr-implement                                │
                │   └─────────────────────────────────────────────────┘

横切层：[metrics-audit] 审指标（串在 wr-close 第 4.3 步判门禁时 / wr-debug 指标层回流时）
```

**6 个 skill 各管一段：bug 修复调度入口 → 能力实现 → 关 R 生命周期 → 审指标 → 修底层 → W 矩阵级调度，全闭环。**

---

## 4. 典型串联场景

### 4.1 场景 A：W3 阶段关 R7（未关，完整闭环 — 含能力实现）

```
你: 实现 R7 能力（或「先实现再写真窗」）
  → wr-implement §1 能力现状审查
    - ui/rendering/virtual_list.go 是否有 BindCount/ScrollToIndex/OffsetOfIndex
    - ui/rendering/virtual_list_test.go 单测是否覆盖
    - ui/scheduler/metrics.go 的 FrameMetrics struct 是否有 bind_count
  → wr-implement §2 实现/补全能力（只动 ui/rendering/）
  → wr-implement §3 跑单测：go test ./ui/rendering -run TestVirtualList -count=1
  → wr-implement 第 4 步 指标接线：bind_count 接 metrics.go + pipeline_app.go
  → wr-implement §5 能力就绪 → 交 wr-close 模式 1

你: 关闭 R7（wr-implement 自动串接，或你显式说）
  → wr-close 模式 1 第 1 步定标准（U17 场景复杂度 + U18 HUD + U20 实现点六维）
  → wr-close 模式 1 第 2 步写真窗代码
  → wr-close 第 3 步跑 GPU 取 JSON
    ├─ 发现底层不满足 → 交 wr-engine 修底层 → 修完回第 3 步重跑
    └─ 底层 OK → 继续
  → wr-close 第 4 步判门禁 + 串 metrics-audit 审指标
    ├─ 任一 FAIL → FAIL 内置回流（回第 2 步重写）
    │   ├─ 类 A 指标层 bug → 交 metrics-audit 修 JSON marshal
    │   ├─ 类 B 真窗代码层 bug → wr-close 第 2 步重写
    │   └─ 类 C 引擎洞 → 交 wr-engine 定点修
    └─ 全绿 → wr-close 第 6 步回写 docs §2 R7 行 ⬜ → ✅ + §10 修订表

下一步：同理关 R7b、R10、C3
```

### 4.2 场景 B：已关的 R4 想优化降 draw call

```
你: 优化 R4，降 draw call
  → wr-close 自动选模式 3（R4 当前 ✅）
  → wr-close 模式 3 §2.3.1 前置校验（R4 ✅，优化目标在范围内，不降门禁）
  → wr-close 模式 3 §2.3.2 状态标注 ✅ → ✅🔄 优化中
  → wr-close 模式 3 §2.3.3 改 main.go（合批/Picture 缓存）
  → wr-close 第 3 步重跑 GPU + 第 4 步串 metrics-audit 验没回归
  → wr-close 第 6 步回写 docs §2 + §10
  → 状态升维 ✅🔄 → ✅v2-optimized + §10 修订表加版本
```

### 4.3 场景 C：已关的 R3 场景太简陋，推倒重写升维

```
你: 重写已关的 R3 真窗，升维
  → wr-close 自动选模式 2（R3 当前 ✅，要升维重写）
  → wr-close 模式 2 §2.2.1 状态降级 ✅ → 🔄 质量返工
  → wr-close 模式 2 §2.2.2 第 1 步定标准（U17/U18/U20）→ 第 2 步重写
  → wr-close 第 3 步重跑 + 第 4 步重审真绿 + 第 6 步重关
  → 状态升维 🔄 → ✅v2（质量条）+ §10 修订表
```

### 4.4 场景 D：怀疑 R7 指标装绿，定向审查

```
你: 审 R7 的指标，怀疑 fps 假值
  → metrics-audit 第 1–5 步全套审查
  → 第 1 步：字段完备性（族 A–J 不默默省略）
  → 第 2 步：诚实性（vsync_source 不假锁、fps_interval 不靠静帧刷高）
  → 第 3 步：门禁阈值防偷放（README 阈值不高于文档默认）
  → 第 4 步：指标与代码观测一致性（JSON 累计数 = 代码实际）
  → 第 5 步：降画质装绿检测（场景达 U17、RUN_SECONDS ≥ 关闭用值）
  → 若 SUSPECT/FAKE → 交 wr-close 模式 2 反攻重关
  → 若 HONEST/EARNED → 报告「指标诚实」，无需反攻
```

### 4.5 场景 E：已关的 R7 想加新滚动场景 + 新 HUD 字段

```
你: R7 加场景 + 加 HUD
  → wr-close 自动选模式 3（R7 当前 ✅，要增量）
  → wr-close 模式 3 §2.3.1 前置校验（R7 ✅）
  → wr-close 模式 3 §2.3.2 状态标注 ✅ → ✅🔄 增量中
  → wr-close 模式 3 §2.3.3 改 main.go（按 U17 加场景 + U18 加 HUD）
  → wr-close 模式 3 §2.3.4 改 README（Visible effect + Gates 加新行）
  → wr-close 第 3 步重跑 + 第 4 步串 metrics-audit 验没回归 + 第 6 步重关
  → 状态升维 ✅🔄 → ✅v2-extended + §10 修订表加版本
```

### 4.6 场景 F：推翻 W2 整波重写（W 矩阵级调度）

```
你: 推翻 W2 重写
  → wr-rewrite §0 读真源 + W 矩阵定位
    - §5 分期表 W2 行（当前 ✅）
    - §2 主表 W2 涉及的 R（R4/R4b/R11/R13/R18）
        - §10 修订表当前版本号
  → wr-rewrite §1 W 矩阵级状态降级
    - §5 分期表 W2 行：✅ → 🔄 W2 推翻重写
    - §2 主表 W2 所有 R 行：✅ → 🔄
    - §10 修订表：加占位行
  → wr-rewrite §2 逐个 R 回流（调度 wr-implement / wr-close 3 模式 / wr-engine）
    for each R in [R4, R4b, R11, R13, R18]:
      ├─ R 能力是否就绪？
      │   ├─ 否 → 调度 wr-implement 实现能力
      │   │       └─ wr-implement 发现底层不满足 → 调度 wr-engine 修底层
      │   └─ 是 → 进下一步
      ├─ R 真窗是否要写/重写？
      │   ├─ R 当前 ⬜ + 真窗未建 → 调度 wr-close 模式 1（首次关闭）
      │   ├─ R 当前 ✅ + 要推翻重写升维 → 调度 wr-close 模式 2（反攻重关）
      │   ├─ R 当前 ✅ + 要优化/增量 → 调度 wr-close 模式 3（优化重关）
      │   └─ R 当前 🔄 + 重写中 → 调度 wr-close 模式 2 继续重关
      ├─ 跑 GPU 时发现底层不满足？
      │   └─ 是 → 调度 wr-engine 修底层
      └─ 跑完 + 判门禁时审指标？
          └─ 是 → 调度 metrics-audit 审指标诚实性
  → wr-rewrite §2.3 底层洞批量收集（HOLE_LIST 去重）
  → wr-rewrite §2.4 底层洞批量定点修（逐个调度 wr-engine）
  → wr-rewrite §2.5 组合窗回流（C2/C7）
  → wr-rewrite §3 批量重跑验证
    - 批量重跑 W2 所有 R 真窗
    - 批量重跑 W2 组合窗
    - 批量审指标（调度 metrics-audit 逐个 R 审）
  → wr-rewrite 第 4 步 W 矩阵级状态升维
    - §5 分期表 W2 行：🔄 → ✅v2（推翻重写后）
    - §2 主表 W2 所有 R 行：🔄 → ✅v2
    - §10 修订表：占位行改为正式版本说明
```

### 4.7 场景 G：已关的 R3 出现「boundary 文不 skip」bug（v3.0 新增）

```
你: 修 R3 的 bug——boundary 文不 skip
  → wr-debug §0 读真源（AGENTS.md + §2 R3 行 + §2.2 FAIL 线 + §2.5 关闭用时长 + §22.1 序 3 前驱）
  → wr-debug §1 强制复现
    RUN_SECONDS=15 go run ./examples/ui_wr_r3_boundary
    取 stderr + JSON
  → wr-debug §2 定位 bug 所在层
    族 C 脏区 FAIL（boundary_skip=0 但场景有静文）
    对照 §6 模块落点 → ui/rendering/boundary_cache.go
    风险等级：低（ui/rendering 是能力实现层）
  → wr-debug §3.2 ui 低风险自修
    read_symbol ui/rendering/boundary_cache.go FrameSkip
    edit_file 定点修（扩 record 类型支持文/自定义 OnPaint 静区）
    跑改动单测：go test ./ui/rendering -run TestBoundaryCache -count=1
    跑全 ui 包回归：go test ./ui/... -count=1
    重跑该 R 真窗验证 bug 已修：RUN_SECONDS=15 go run ./examples/ui_wr_r3_boundary
  → wr-debug 第 4 步 回写 docs/ENGINE_UI_WIDGET_RENDER.md §10 修订表
    「<版本号> | R3 bug 修复：boundary 文不 skip → 扩 record 类型 in ui/rendering/boundary_cache.go」
```

**关键：** `wr-debug` 不改 §2 主表 R3 行的「状态」列——状态升维归被回流的 skill 改。本 skill 只回写 §10 修订表。若 R3 当前 ✅，bug 修完后状态保持 ✅（不升维，只是 bug 修了）；若要升维到 ✅v2，那归 `wr-close` 模式 2 反攻重关，不归 `wr-debug`。

---

## 5. 6 个 skill 的分工边界

### 5.1 谁管什么

| 动作 | 归哪个 skill |
|------|--------------|
| 指定窗口 bug 修复（强制复现 → 定位层 → 分层回流） | `wr-debug` |
| 写真窗**前**的能力实现（ui/ 里实现 + 单测 + 指标接线） | `wr-implement` |
| 关 R 完整生命周期（3 模式：首次关闭 / 反攻重关 / 优化重关） | `wr-close` |
| 指标族 A–J 字段完备性/诚实性/阈值防偷放/观测一致性/降画质检测 | `metrics-audit` |
| 底层修复回流（render/gpu/ui-scene 谨慎改 + 跨层影响面评估） | `wr-engine` |
| W 矩阵级调度（推翻 W<n> 重写，批量调度 + W 矩阵级状态治理） | `wr-rewrite` |

### 5.2 谁不管什么

| 动作 | 不归哪个 skill | 该归谁 |
|------|----------------|--------|
| 能力实现（ui/rendering/ 里写 VirtualList 等） | `wr-close`（只关 R 生命周期） | `wr-implement` |
| 关 R 的建窗/跑/判门禁/回写 docs | `wr-implement`（只实现能力） | `wr-close` |
| 指标层 JSON marshal bug | `wr-close`（只判门禁） | `metrics-audit` |
| 底层修复（render/gpu/ui-scene） | `wr-close`（发现底层不满足交出去） | `wr-engine` |
| W 矩阵级批量调度 | `wr-close`（只管单 R） | `wr-rewrite` |
| 单 R 的首次关闭 | `wr-rewrite`（只管 W 矩阵级） | `wr-implement` → `wr-close` 模式 1 |

### 5.3 状态转换矩阵（docs §2 + §10）

| 场景 | 状态转换 | §10 修订表 |
|------|----------|------------|
| 未关 R 首次关闭（wr-close 模式 1） | `⬜ → ✅` | `<版本> \| W<n> ✅ <R id> 真窗 + C<组合>` |
| 已关 R 反攻升维（wr-close 模式 2） | `✅ → 🔄 质量返工 → ✅v2（质量条）` | `<版本> \| R<id> 质量升维 ✅v2：<反攻摘要>` |
| 已关 R 优化（wr-close 模式 3） | `✅ → ✅🔄 优化中 → ✅v2-optimized` | `<版本> \| R<id> 优化生效：<优化前 → 优化后 指标对比>` |
| 已关 R 增量（wr-close 模式 3） | `✅ → ✅🔄 增量中 → ✅v2-extended` | `<版本> \| R<id> 增量生效：<加了什么 + 新指标值>` |
| W 矩阵级推翻重写（wr-rewrite） | `✅ → 🔄 W<n> 推翻重写 → ✅v2（推翻重写后）` | `<版本> \| W<n> 推翻重写 ✅v2：<重写摘要 + 涉及的 R 列表>` |

**注意：** `wr-implement` 是**能力实现**，不动 docs §2 状态列（状态列归 wr-close 回写）。`wr-implement` 完成后 docs §2 该 R 行状态仍是 ⬜，直到 wr-close 走完第 6 步才 ⬜ → ✅。

---

## 6. `wr-close` 的 3 种模式分流

`wr-close` 是关 R 完整生命周期的核心 skill，支持 3 种模式：

| 模式 | 触发 | R 当前状态 | 流程 |
|------|------|------------|------|
| **模式 1 首次关闭** | 「关闭 R7」 | `⬜`（未关） | 第 1 步定标准 → 第 2 步写代码 → 第 3 步跑 GPU → 第 4 步判门禁+审指标 → 第 6 步回写 ⬜ → ✅ |
| **模式 2 反攻重关** | 「反攻 R3」「重写 R4」 | `✅`（已关，要升维）或 `🔄`（质量返工中） | 第 2.2.1 步状态降级 ✅ → 🔄 → 第 2.2.2 步重写 → 重跑 → 重审 → 重关 🔄 → ✅v2 |
| **模式 3 优化重关** | 「优化 R4」「R7 加场景」 | `✅`（已关，要优化/增量） | 第 2.3.1 步前置校验 → 第 2.3.2 步状态标注 ✅ → ✅🔄 → 第 2.3.3 步改代码 → 重跑 → 验没回归 → 重关 ✅🔄 → ✅v2-optimized |

**FAIL 内置回流（核心）：** 任一模式跑 GPU 判门禁 FAIL，自动回第 2 步重写（按第 1 步定标准），不需另起 skill。这是原 `wr-rework` 类 B 的职责。

**模式选择规则：** `read_file docs/ENGINE_UI_WIDGET_RENDER.md` §2 主表该 R 行的「状态」列，按当前状态自动选模式。状态与触发词冲突时停下问用户。

---

## 7. `wr-engine` 的底层修复回流（render/gpu 谨慎改）

`wr-engine` 专门处理底层修复，**render/ 和 gpu/ 直接对接 GPU 后端，最谨慎改**：

### 7.1 修复边界 + 风险等级

| 层 | 文件示例 | 风险 | 修复纪律 |
|----|----------|------|----------|
| `ui/rendering/` | `boundary_cache.go` / `virtual_list.go` | 低 | 能力实现层——**可改** |
| `ui/embedder/` | `pipeline_app.go` | 中 | Present 路径——**可改**（改动局部 + 有单测覆盖） |
| `ui/scene/` | `picture.go` / `composite.go` / `layer.go` | 中 | Picture/Layer/Composite——**可改**（同上） |
| `render/` | `context.go` / `draw.go` / `path.go` / `image.go` | 高 | 画布/绘制 API——**谨慎改**（跨层影响面评估 + 评估对 gpu/ 下游 + ui/rendering 上游的影响） |
| `gpu/` | `device.go` / `swapchain.go` / `libwgpu_native` 绑定 | 最高 | wgpu 后端对接——**最谨慎改**（强制停下报告用户：GPU 驱动兼容性 + wgpu 版本兼容性 + 替代方案 + 影响面超阈值） |
| `ui/io/` | `decode.go` | 低 | 异步图 worker——**可改** |

### 7.2 跨层影响面评估（核心步骤）

改底层前必须评估影响面：

```
底层洞定位（哪一层？）
  ↓
跨层影响面评估
  ├─ ui/rendering/ 改动（低风险）→ 通过 → 定点修
  ├─ ui/embedder/ 改动（中风险）→ 改动局部 + 有单测覆盖 → 通过 → 定点修
  ├─ ui/scene/ 改动（中风险）→ 同上
  ├─ render/ 改动（高风险）→ 评估对 gpu/ 下游 + ui/rendering 上游的影响
  │   ├─ 影响面可控 → 通过 → 定点修
  │   └─ 影响面超阈值 → 停下报告用户
  └─ gpu/ 改动（最高风险）→ 强制停下报告用户
      ├─ 用户确认改 → 定点修（必须加 gpu 单测 + 全回归）
      └─ 用户不确认 / 建议绕过 → 改 render 层绕过（降风险），或停下不修（该 R 保持 🔄）
```

---

## 8. `wr-rewrite` 的 W 矩阵级调度

`wr-rewrite` 专门处理 W 矩阵级推翻重写调度，**是调度层，不直接写代码**：

### 8.1 W 矩阵级状态降级（第 1 步）

推翻 W<n> 重写时，该 W 的所有状态都要降级：

| docs 章节 | 原状态 | 降级后 |
|-----------|--------|--------|
| §5 分期表该 W 行 | `✅` | `🔄 W<n> 推翻重写` |
| §3 组合表该 W 的 C | `✅` | `🔄 推翻重写中` |
| §2 主表该 W 所有 R 行 | `✅` | `🔄` |
| §10 修订表 | — | 加占位行 `<版本号> | W<n> 推翻重写：<原因摘要>（进行中）` |

### 8.2 逐个 R **与** C 回流（第 2 步）

**硬：** §5 该 W 列出的 **每一个 R**、**每一个 C** 都必须有独立 `examples/ui_wr_*` 包；目录空 = 模式 1 建窗。

```
for each R in 该 W 的 R 列表:   # 各 ui_wr_r* 独立真窗
  ├─ 能力就绪？否 → wr-implement（必要时 wr-engine）
  ├─ ⬜/空目录 → wr-close 模式 1；✅/🔄 推翻 → 模式 2；优化 → 模式 3
  ├─ GPU 洞 → wr-engine；指标 → metrics-audit
  └─ **禁止**用任何 C 的绿代替本 R

for each C in 该 W 的 C 列表:   # 各 ui_wr_c* 独立真窗（不可省）
  ├─ 空目录/未关 → wr-close 模式 1 建组合窗
  ├─ 推翻 → wr-close 模式 2
  ├─ 只做集成，不重判所覆盖 R 的单能力门禁
  └─ **绝不**因 C 绿而把未建/未绿的 R 标 ✅
```

**底层洞：** 回流中收集 `HOLE_LIST` 去重 → 逐个 wr-engine → 修完重跑该 W **全部 R 与 C**。

### 8.3 批量重跑验证（第 3 步）

```
批量重跑该 W **全部** R 的独立真窗
批量重跑该 W **全部** C 的独立真窗
批量 metrics-audit（各 R；C 按集成字段）
任一缺包或 FAIL → 不得 W 升维
```

### 8.4 W 矩阵级状态升维（第 4 步）

W 矩阵级重写完真绿，docs 状态升维：

| docs 章节 | 原状态 | 升维后 |
|-----------|--------|--------|
| §5 分期表该 W 行 | `🔄 W<n> 推翻重写` | `✅v2（推翻重写后）` |
| §3 组合表该 W 的 C | `🔄` | `✅v2` |
| §2 主表该 W 所有 R 行 | `🔄` | `✅v2（推翻重写后）` |
| §10 修订表 | 占位行 | 改为正式版本说明 |

---

## 9. 共享接口约定

6 个 skill 之间的数据传递接口：

| 接口方向 | 内容 |
|----------|------|
| `wr-debug` → `wr-engine` | bug 定位到 render/gpu，停下 question 确认后交 wr-engine 修底层 |
| `wr-debug` → `metrics-audit` | bug 定位到指标层（不诚实/偷放/字段名不一致），交 metrics-audit 审指标诚实性 |
| `wr-debug` → `wr-close` 模式 2 | bug 是「已关 ✅ 真窗代码层 bug」，交 wr-close 模式 2 反攻重关 |
| `wr-debug` → `wr-close` 模式 3 | bug 是「已关 ✅ 优化增量」，交 wr-close 模式 3 优化重关 |
| `wr-debug` → `wr-implement` | bug 是「能力根本没实现」，交 wr-implement 实现能力 |
| `wr-debug` → `wr-debug` §3.5 | wr-engine 修完底层 + 跑回归全绿，交回 wr-debug 重跑真窗验证 bug 已修 |
| `wr-implement` → `wr-close` | 能力已实现 + 单测绿 + 指标字段已接，交 wr-close 模式 1 写真窗 |
| `wr-close` → `wr-engine` | 跑 GPU 发现底层不满足，交 wr-engine 修底层 |
| `wr-engine` → `wr-close` | 底层修完 + 跑回归全绿，交回 wr-close 第 3 步重跑真窗 |
| `wr-close` → `metrics-audit` | 判门禁时，串接 metrics-audit 审指标诚实性 |
| `wr-rewrite` → `wr-implement` | W 矩阵重写时，该 W 的 R 能力不满足，调度 wr-implement 实现 |
| `wr-rewrite` → `wr-close` 模式 1/2/3 | W 矩阵重写时，调度 wr-close 对应模式 |
| `wr-rewrite` → `wr-engine` | W 矩阵重写时，HOLE_LIST 收集完后，逐个底层洞调度 wr-engine 修 |
| `wr-rewrite` → `metrics-audit` | W 矩阵重写批量审指标时，调度 metrics-audit 审 JSON |
| `wr-implement` → `wr-engine` | 实现能力时发现底层不满足，交 wr-engine 修底层 |

**关键：** 6 个 skill 通过 docs §2 状态列 + §10 修订表 + JSON 文件 + FrameMetrics struct 传递状态，不通过共享内存或全局变量。`wr-debug` 不改 §2 主表「状态」列（升维归被回流的 skill），只回写 §10 修订表。

---

## 10. 禁令总表（5 个 skill 共享）

以下禁令在 5 个 skill 中都适用，违反任一即 FAIL：

| # | 禁令 | 出处 |
|---|------|------|
| 1 | 不删场景保门禁 | wr-close 模式 1 §1.1 U17 |
| 2 | 不降 README 阈值过门禁 | metrics-audit §3 |
| 3 | 不在示例里绕引擎洞 | wr-close 模式 2 §2.2.2 + wr-engine |
| 4 | 不用 CPU stub 或单测单独关 R | wr-close U8 |
| 5 | 每个 R/C 独立 ui_wr_* 包；不用组合窗代替单 R | wr-close U5/U6 |
| 5b | 状态只写 §2/§3/§5/§10 | wr-rewrite / wr-close |
| 6 | 窗口必须 1200×800 不是更小 | wr-close U15 |
| 7 | RUN_SECONDS ≥ 推荐关闭用值，至少 ≥5 | wr-close U16 |
| 8 | JSON 含全族 A–J 字段，无默默省略 | wr-close U12 |
| 9 | 动画/滚动窗 `fps_interval≥55` 或 `interval_p95≤22` | wr-close U13 |
| 10 | 必须输出 `vsync_source`，fallback 不宣称锁 60Hz | wr-close U13 |
| 11 | `cpu_pct_avg` / `rss_*` 等硬观测采到了 | wr-close U14 |
| 12 | README 写了可见效果 + 门禁 | wr-close U7 |
| 13 | 文档 §2 状态列回写了 | wr-close U9 |
| 14 | 没降画质装绿 | metrics-audit §5 |
| 15 | 没用 CompositeOnly+GPU Clear 装省绘 | wr-close U11 |
| 16 | 能力实现只动 ui/，不动 examples/（真窗归 wr-close） | wr-implement §2.2 |
| 17 | 不引入 ui→gpu 依赖、不用 CGO（purego） | wr-implement §2.2 + CODING_RULES §0.4 |
| 18 | 字段名取自 FrameMetrics struct，禁止自创字段名 | wr-implement §2.2 + metrics-audit 第 4 步 |
| 19 | 前驱序未收口不跳（禁止跳前驱） | wr-implement §1.4 + RENDER_BASE §22.0 |
| 20 | render/ 改动必须跨层影响面评估 | wr-engine §1.2 |
| 21 | gpu/ 改动强制停下报告用户 | wr-engine §1.2 |
| 22 | wr-rewrite 是调度层，不直接写代码 | wr-rewrite §0 |
| 23 | wr-rewrite 底层洞批量收集（去重，避免重复修） | wr-rewrite §2.3 |
| 24 | 引擎洞修完跑 go test ./ui/... 确认没回归 | wr-engine 第 3 步 |
| 25 | wr-debug 强制复现——不跑真窗就凭描述改代码 = 绕洞 | wr-debug §1 |
| 26 | wr-debug 不凭 BUG_DESC 猜层——对照 §2.2 FAIL 线 + §6 模块落点定位 | wr-debug §2 |
| 27 | wr-debug ui 低风险自修后必须重跑该 R 真窗验证 bug 已修（不只靠单测绿） | wr-debug §3.2 |
| 28 | wr-debug 不改 §2 主表「状态」列（升维归被回流的 skill），只回写 §10 修订表 | wr-debug 第 4 步 |

---

## 11. 开发实战速查

### 11.1 W3 阶段（你现在所在）的推荐流程

```
1. 你: 实现 R7 能力（或「先实现再写真窗」）
   → wr-implement §1 能力现状审查
     - ui/rendering/virtual_list.go 是否有 BindCount/ScrollToIndex/OffsetOfIndex
     - ui/rendering/virtual_list_test.go 单测是否覆盖
     - ui/scheduler/metrics.go 的 FrameMetrics struct 是否有 bind_count
   → wr-implement §2 实现/补全（只动 ui/rendering/）
   → wr-implement §3 跑单测：go test ./ui/rendering -run TestVirtualList -count=1
   → wr-implement 第 4 步 指标接线：bind_count 接 metrics.go + pipeline_app.go
   → wr-implement §5 能力就绪 → 交 wr-close 模式 1

2. 你: 关闭 R7（wr-implement 自动串接，或你显式说）
   → wr-close 模式 1 第 1 步定标准（U17/U18/U20）
   → wr-close 模式 1 第 2 步写真窗代码
   → wr-close 第 3 步跑 GPU 取 JSON
     ├─ 发现底层不满足 → 交 wr-engine 修底层 → 修完回第 3 步重跑
     └─ 底层 OK → 继续
   → wr-close 第 4 步判门禁 + 串 metrics-audit 审指标
     ├─ 任一 FAIL → FAIL 内置回流（回第 2 步重写）
     │   ├─ 类 A 指标层 bug → 交 metrics-audit 修 JSON marshal
     │   ├─ 类 B 真窗代码层 bug → wr-close 第 2 步重写
     │   └─ 类 C 引擎洞 → 交 wr-engine 定点修
     └─ 全绿 → wr-close 第 6 步回写 docs §2 R7 行 ⬜ → ✅ + §10 修订表

3. 同理关 R7b、R10、C3
   - R7b：wr-implement（补 scroll_rerecord 字段）→ wr-close 模式 1
   - R10：wr-implement（补 async image worker）→ wr-close 模式 1
   - C3：wr-close 模式 1（组合窗跑 GPU + 判门禁）
```

### 11.2 已关 R 的优化/增量流程

```
1. 你: 优化 R4，降 draw call
   → wr-close 自动选模式 3（R4 当前 ✅）
   → wr-close 模式 3 前置校验（R4 ✅）
   → 状态标注 ✅ → ✅🔄 优化中
   → 改 main.go（合批/Picture 缓存）
   → 重跑 GPU + 串 metrics-audit 验没回归
   → wr-close 第 6 步回写 + 状态升维 ✅🔄 → ✅v2-optimized + §10 修订表

2. 你: R7 加场景 + 加 HUD
   → wr-close 自动选模式 3（R7 当前 ✅，要增量）
   → 前置校验 + 状态标注 ✅ → ✅🔄 增量中
   → 改 main.go（按 U17 加场景 + U18 加 HUD）
   → 改 README（Visible effect + Gates 加新行）
   → 重跑 + 串 metrics-audit 验没回归 + 回写 + 状态升维 ✅🔄 → ✅v2-extended
```

### 11.3 怀疑指标装绿的审查流程

```
1. 你: 审 R7 的指标，怀疑 fps 假值
   → metrics-audit 第 1–5 步全套审查
   → 第 1 步：字段完备性（族 A–J 不默默省略）
   → 第 2 步：诚实性（vsync_source 不假锁、fps_interval 不靠静帧刷高）
   → 第 3 步：门禁阈值防偷放（README 阈值不高于文档默认）
   → 第 4 步：指标与代码观测一致性（JSON 累计数 = 代码实际）
   → 第 5 步：降画质装绿检测（场景达 U17、RUN_SECONDS ≥ 关闭用值）
   → 若 SUSPECT/FAKE → 交 wr-close 模式 2 反攻重关
   → 若 HONEST/EARNED → 报告「指标诚实」，无需反攻
```

### 11.4 推翻整波重写流程（W 矩阵级调度）

```
1. 你: 推翻 W2 重写
   → wr-rewrite §0 读真源 + W 矩阵定位
     - §5 分期表 W2 行（当前 ✅）
     - §2 主表 W2 涉及的 R（R4/R4b/R11/R13/R18）
     - §3 组合表 + §10 修订表
   → wr-rewrite §1 W 矩阵级状态降级
     - §5 分期表 W2 行：✅ → 🔄 W2 推翻重写
      - §2 主表 W2 所有 R 行：✅ → 🔄
     - §10 修订表：加占位行
   → wr-rewrite §2 逐个 R 回流（调度 wr-implement / wr-close 3 模式 / wr-engine / metrics-audit）
     - R4：wr-close 模式 2（反攻重关）→ ✅v2
     - R4b：wr-implement（补 scroll_rerecord）→ wr-close 模式 1 → ✅
     - R11：wr-close 模式 2 → ✅v2
     - R13：wr-close 模式 2 → ✅v2
     - R18：wr-close 模式 2 → ✅v2
     - 底层洞批量收集（HOLE_LIST 去重）
     - 底层洞批量定点修（逐个调度 wr-engine）
     - 组合窗回流（C2/C7）
   → wr-rewrite §3 批量重跑验证
     - 批量重跑 W2 所有 R 真窗
     - 批量重跑 W2 组合窗
     - 批量审指标（调度 metrics-audit 逐个 R 审）
   → wr-rewrite 第 4 步 W 矩阵级状态升维
     - §5 分期表 W2 行：🔄 → ✅v2（推翻重写后）
      - §2 主表 W2 所有 R 行：🔄 → ✅v2
     - §10 修订表：占位行改为正式版本说明
```

### 11.5 底层不满足时修底层流程

```
1. 跑真窗时发现底层不满足
   → wr-close 第 3 步交 wr-engine
   → wr-engine §0 读真源 + 定位底层洞
   → wr-engine §1 跨层影响面评估
     ├─ ui/rendering/ 改动（低风险）→ 通过 → 定点修
     ├─ ui/embedder/ / ui/scene/ 改动（中风险）→ 改动局部 + 有单测覆盖 → 通过 → 定点修
     ├─ render/ 改动（高风险）→ 评估对 gpu/ 下游 + ui/rendering 上游的影响
     │   ├─ 影响面可控 → 通过 → 定点修
     │   └─ 影响面超阈值 → 停下报告用户
     └─ gpu/ 改动（最高风险）→ 强制停下报告用户
         ├─ 用户确认改 → 定点修（必须加 gpu 单测 + 全回归）
         └─ 用户不确认 / 建议绕过 → 改 render 层绕过，或停下不修
   → wr-engine §2 定点修底层
   → wr-engine §3 跑回归（go test ./ui/... + 若改 gpu/ 跑 gpu 单测）
   → wr-engine 第 4 步 交回 wr-close 第 3 步重跑真窗
```

### 11.6 指定窗口出现 bug 的修复流程（v3.0 新增）

```
你: 修 R3 的 bug——boundary 文不 skip
  → wr-debug §0 读真源
    - AGENTS.md 分层风险规则
    - docs/ENGINE_UI_WIDGET_RENDER.md §2 R3 行（PACKAGE / WINDOW / CLOSE_SECONDS / 状态）
    - §2.2 全族 FAIL 线 + §2.5 关闭用时长 + §6 模块落点 + §10 修订表
    - docs/ENGINE_UI_RENDER_BASE.md §22.1 序 3 前驱
  → wr-debug §1 强制复现
    RUN_SECONDS=15 go run ./examples/ui_wr_r3_boundary
    取 stderr + JSON（不跑就凭描述改 = 绕洞）
  → wr-debug §2 定位 bug 所在层
    族 C 脏区 FAIL（boundary_skip=0 但场景有静文）
    对照 §6 模块落点 → ui/rendering/boundary_cache.go
    风险等级：低（ui/rendering 是能力实现层）
  → wr-debug §3 分层回流
    ├─ 定位层 = ui/rendering（低风险）+ 能力实现缺陷
    │   → wr-debug §3.2 ui 低风险自修
    │     - read_symbol ui/rendering/boundary_cache.go FrameSkip
    │     - edit_file 定点修（扩 record 类型支持文/自定义 OnPaint 静区）
    │     - 跑改动单测：go test ./ui/rendering -run TestBoundaryCache -count=1
    │     - 跑全 ui 包回归：go test ./ui/... -count=1
    │     - 重跑该 R 真窗验证 bug 已修
    │
    ├─ 若定位层 = render/（高风险）→ 停下 question 确认 → 交 wr-engine
    ├─ 若定位层 = gpu/（最高风险）→ 停下 question 确认 → 交 wr-engine
    ├─ 若定位层 = 指标层 → 交 metrics-audit 审指标诚实性
    ├─ 若定位层 = 已关 ✅ 真窗代码层 bug → 交 wr-close 模式 2
    ├─ 若定位层 = 已关 ✅ 优化增量 → 交 wr-close 模式 3
    └─ 若定位层 = 能力根本没实现 → 交 wr-implement
  → wr-debug 第 4 步 回写 docs §10 修订表
    「<版本号> | R3 bug 修复：boundary 文不 skip → 扩 record 类型」
```

---

## 12. 一句话总结

> **6 个 skill 各管一段：wr-debug（指定窗口 bug 修复调度入口，强制复现 → 定位层 → 分层回流）→ wr-implement（能力实现）→ wr-close（关 R 完整生命周期，3 模式）→ metrics-audit（指标层审查）→ wr-engine（底层修复回流，render/gpu 谨慎改）→ wr-rewrite（W 矩阵级调度，推翻整波重写）。bug 修复入口、正向写、反向修、横向优化、底层穿透、W 矩阵级调度都有 skill 接，docs §2 状态列 + §10 修订表贯穿全流程，闭环。**
