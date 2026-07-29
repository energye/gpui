# gpui 真窗 Skill 使用手册（5 skill 收敛版）

> **版本：** 2.0 | 日期：2026-07-29
> **范围：** 本手册说明 `.atomcode/skills/` 下 5 个 skill 的**使用方法、触发词、串联闭环**。
> **真源：** `docs/ENGINE_UI_WIDGET_RENDER.md` v3.1 + `docs/ENGINE_UI_RENDER_BASE.md` v1.29。skill 是真源的执行骨架，若 skill 与真源矛盾，以真源为准。
> **配套：** 5 个 skill 的完整定义见 `.atomcode/skills/<skill-name>/SKILL.md`。

---

## 1. 5 个 skill 一览（收敛后）

| # | Skill | 职责 | 核心动词 |
|---|-------|------|----------|
| 1 | `gpui-wr-implement` | 能力实现回流（ui/ 里实现 + 单测 + 指标接线） | 实现 |
| 2 | `gpui-wr-close` | 关 R 完整生命周期（3 模式：首次关闭 / 反攻重关 / 优化重关） | 关 |
| 3 | `gpui-metrics-audit` | 指标族 A–J 正误审查与 bug 修复（指标层） | 审 |
| 4 | `gpui-wr-engine` | 底层修复回流（render/gpu/ui-scene 谨慎改 + 跨层影响面评估） | 修底层 |
| 5 | `gpui-wr-rewrite` | W 矩阵级调度（推翻 W<n> 重写，批量调度 + W 矩阵级状态治理） | 调度 |

**核心分工边界：**

- `wr-implement`：能力实现层（ui/ 里实现 + 单测 + 指标接线）
- `wr-close`：关 R 完整生命周期（3 模式吸收了原 wr-quality + wr-rework 类 B + wr-optimize）
- `metrics-audit`：横切层，审指标诚实性，串在 wr-close 判门禁时
- `wr-engine`：底层修复层（render/gpu 谨慎改 + 跨层影响面评估）
- `wr-rewrite`：W 矩阵级调度层（推翻整波重写，调度上面 4 个 skill）

**收敛说明（v1.0 → v2.0）：**

| 原 skill（v1.0） | 收敛到哪（v2.0） |
|------------------|------------------|
| `wr-quality`（写前定标准） | 并入 `wr-close` 模式 1 第 1 步「定标准」 |
| `wr-rework` 类 B（门禁 FAIL 重写真窗） | 并入 `wr-close` FAIL 内置回流（第 4.2 步） |
| `wr-optimize`（已关 R 优化/增量） | 并入 `wr-close` 模式 3 优化重关 |
| `wr-rework` 类 C（引擎洞定点修） | 独立成 `wr-engine`（底层修复回流） |
| `wr-implement` / `metrics-audit` | 保持独立不变 |

---

## 2. 触发词速查表

### 2.1 `gpui-wr-implement`（能力实现回流）

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
                │   │   (推翻 W<n> 重写，批量调度下面 4 个 skill)    │
                └──►│   调度 wr-implement / wr-close 3 模式 /         │
                    │           wr-engine / metrics-audit             │
                    └─────────────────────────────────────────────────┘

横切层：[metrics-audit] 审指标（串在 wr-close 第 4.3 步判门禁时）
```

**5 个 skill 各管一段：能力实现 → 关 R 生命周期 → 审指标 → 修底层 → W 矩阵级调度，全闭环。**

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
  → wr-implement §4 指标接线：bind_count 接 metrics.go + pipeline_app.go
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
    - §4c 关闭清单
    - §10 修订表当前版本号
  → wr-rewrite §1 W 矩阵级状态降级
    - §5 分期表 W2 行：✅ → 🔄 W2 推翻重写
    - §4c 关闭清单：✅ → 🔄 推翻重写中
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
  → wr-rewrite §4 W 矩阵级状态升维
    - §5 分期表 W2 行：🔄 → ✅v2（推翻重写后）
    - §4c 关闭清单：🔄 → ✅v2
    - §2 主表 W2 所有 R 行：🔄 → ✅v2
    - §10 修订表：占位行改为正式版本说明
```

---

## 5. 5 个 skill 的分工边界

### 5.1 谁管什么

| 动作 | 归哪个 skill |
|------|--------------|
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
| §4/§4b/§4c 该 W 关闭清单 | `✅` | `🔄 推翻重写中` |
| §2 主表该 W 所有 R 行 | `✅` | `🔄` |
| §10 修订表 | — | 加占位行 `<版本号> | W<n> 推翻重写：<原因摘要>（进行中）` |

### 8.2 逐个 R 回流（第 2 步，调度现有 4 个 skill）

```
for each R in 该 W 的 R 列表:
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
```

**底层洞批量收集（关键）：** 逐个 R 回流过程中，收集所有底层洞到 `HOLE_LIST`（去重）。多个 R 可能撞同一个底层洞（如 R4/R11/R18 都需要 BoundaryCache 文 skip），批量收集后一次修，避免重复修。

**底层洞批量定点修：** `HOLE_LIST` 收集完后，逐个底层洞调度 wr-engine 修。修完底层后，重新跑该 W 所有相关 R 的真窗，确认底层修复后这些 R 都能过门禁。

### 8.3 批量重跑验证（第 3 步）

```
批量重跑该 W 所有 R 的真窗
批量重跑该 W 的组合窗
批量审指标（调度 metrics-audit 逐个 R 审）
```

### 8.4 W 矩阵级状态升维（第 4 步）

W 矩阵级重写完真绿，docs 状态升维：

| docs 章节 | 原状态 | 升维后 |
|-----------|--------|--------|
| §5 分期表该 W 行 | `🔄 W<n> 推翻重写` | `✅v2（推翻重写后）` |
| §4/§4b/§4c 该 W 关闭清单 | `🔄 推翻重写中` | `✅v2` |
| §2 主表该 W 所有 R 行 | `🔄` | `✅v2（推翻重写后）` |
| §10 修订表 | 占位行 | 改为正式版本说明 |

---

## 9. 共享接口约定

5 个 skill 之间的数据传递接口：

| 接口方向 | 内容 |
|----------|------|
| `wr-implement` → `wr-close` | 能力已实现 + 单测绿 + 指标字段已接，交 wr-close 模式 1 写真窗 |
| `wr-close` → `wr-engine` | 跑 GPU 发现底层不满足，交 wr-engine 修底层 |
| `wr-engine` → `wr-close` | 底层修完 + 跑回归全绿，交回 wr-close 第 3 步重跑真窗 |
| `wr-close` → `metrics-audit` | 判门禁时，串接 metrics-audit 审指标诚实性 |
| `wr-rewrite` → `wr-implement` | W 矩阵重写时，该 W 的 R 能力不满足，调度 wr-implement 实现 |
| `wr-rewrite` → `wr-close` 模式 1/2/3 | W 矩阵重写时，调度 wr-close 对应模式 |
| `wr-rewrite` → `wr-engine` | W 矩阵重写时，HOLE_LIST 收集完后，逐个底层洞调度 wr-engine 修 |
| `wr-rewrite` → `metrics-audit` | W 矩阵重写批量审指标时，调度 metrics-audit 审 JSON |
| `wr-implement` → `wr-engine` | 实现能力时发现底层不满足，交 wr-engine 修底层 |

**关键：** 5 个 skill 通过 docs §2 状态列 + §10 修订表 + JSON 文件 + FrameMetrics struct 传递状态，不通过共享内存或全局变量。

---

## 10. 禁令总表（5 个 skill 共享）

以下禁令在 5 个 skill 中都适用，违反任一即 FAIL：

| # | 禁令 | 出处 |
|---|------|------|
| 1 | 不删场景保门禁 | wr-close 模式 1 §1.1 U17 |
| 2 | 不降 README 阈值过门禁 | metrics-audit §3 |
| 3 | 不在示例里绕引擎洞 | wr-close 模式 2 §2.2.2 + wr-engine |
| 4 | 不用 CPU stub 或单测单独关 R | wr-close U8 |
| 5 | 不用组合窗代替单能力窗关 R | wr-close U5 |
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
   → wr-implement §4 指标接线：bind_count 接 metrics.go + pipeline_app.go
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
     - §4c 关闭清单 + §10 修订表
   → wr-rewrite §1 W 矩阵级状态降级
     - §5 分期表 W2 行：✅ → 🔄 W2 推翻重写
     - §4c 关闭清单：✅ → 🔄 推翻重写中
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
   → wr-rewrite §4 W 矩阵级状态升维
     - §5 分期表 W2 行：🔄 → ✅v2（推翻重写后）
     - §4c 关闭清单：🔄 → ✅v2
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
   → wr-engine §4 交回 wr-close 第 3 步重跑真窗
```

---

## 12. 一句话总结

> **5 个 skill 各管一段：wr-implement（能力实现）→ wr-close（关 R 完整生命周期，3 模式）→ metrics-audit（指标层审查）→ wr-engine（底层修复回流，render/gpu 谨慎改）→ wr-rewrite（W 矩阵级调度，推翻整波重写）。正向写、反向修、横向优化、底层穿透、W 矩阵级调度都有 skill 接，docs §2 状态列 + §10 修订表贯穿全流程，闭环。**
