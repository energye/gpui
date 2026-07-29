# gpui 真窗 Skill 使用手册（5 skill 闭环）

> **版本：** 1.0 | 日期：2026-07-29
> **范围：** 本手册仅说明 `.atomcode/skills/` 下 5 个 skill 的**使用方法、触发词、串联闭环**。
> **真源：** `docs/ENGINE_UI_WIDGET_RENDER.md` v3.1 + `docs/ENGINE_UI_RENDER_BASE.md` v1.29。skill 是真源的执行骨架，若 skill 与真源矛盾，以真源为准。
> **配套：** 5 个 skill 的完整定义见 `.atomcode/skills/<skill-name>/SKILL.md`。

---

## 1. 5 个 skill 一览

| # | Skill | 职责 | 阶段 |
|---|-------|------|------|
| 1 | `gpui-wr-quality` | 关 R **前**的质量标准（场景矩阵 + HUD + 实现点六维） | 写代码前 |
| 2 | `gpui-wr-close` | 关 R 的**执行流程**（建窗/跑/判门禁/回写 docs §2） | 写完代码后 |
| 3 | `gpui-metrics-audit` | 指标族 A–J 的**正误审查与 bug 修复**（指标层） | 任意时点可独立跑，或串在 wr-close 判门禁时 |
| 4 | `gpui-wr-rework` | 门禁 FAIL / 指标装绿的**反攻/修复回流**（代码层 + 引擎层） | 反攻/重写时 |
| 5 | `gpui-wr-optimize` | 已关 R（✅）的**优化/增量回流**（性能 + 场景 + HUD + 指标） | 优化/增量时 |

**核心分工边界：**

- `wr-quality` / `wr-close` / `metrics-audit`：正向验收链路（写 → 审 → 关）
- `wr-rework`：反向修复链路（FAIL → 分类 → 修 → 重跑 → 重关）
- `wr-optimize`：横向加深链路（已关 R 优化/增量 → 重跑 → 重审 → 重关 → 回写版本）

---

## 2. 触发词速查表

### 2.1 `gpui-wr-quality`（写前定标准）

| 触发词 | 场景 |
|--------|------|
| 「写 R7 真窗」「改 R4 真窗」 | 准备写/改任一 `ui_wr_*` 真窗代码 |
| 「返工 R3」「升维真窗」 | 因质量升维返工 |
| 「wr-quality R5」 | 显式点名 |

### 2.2 `gpui-wr-close`（执行关闭流程）

| 触发词 | 场景 |
|--------|------|
| 「关闭 R7」「关掉 R10」「把 R3 收了」 | 关闭一个 §R 主能力真窗 |
| 「wr-close R4」「关这个主能力」 | 显式点名 |

### 2.3 `gpui-metrics-audit`（指标层审查）

| 触发词 | 场景 |
|--------|------|
| 「审 R3 的指标」「metrics-audit R4」「指标族查 bug」 | 指标族正误审查 |
| 「fps 假值」「vsource 假锁」「slope 偷放」「cpu 双 0 装绿」「降画质装绿」 | 怀疑任一 ui_wr_* JSON 指标不诚实/不完备/被偷放阈值 |

### 2.4 `gpui-wr-rework`（反攻/修复回流）

| 触发词 | 场景 |
|--------|------|
| 「反攻 R3」「返工 R4 真窗」「rework R3」 | 门禁 FAIL / 指标装绿识破后的反攻 |
| 「R7 装绿了改掉」「重做 R5」「R4 指标假的改真窗代码」「R7 修 bug」 | 定向反攻 |
| 「重写 R7 真窗」「重写已关的 R4」 | 推倒重写 |

### 2.5 `gpui-wr-optimize`（优化/增量回流）

| 触发词 | 场景 |
|--------|------|
| 「优化 R3」「R4 提性能」「给 R4 降 draw call」「optimize R7」 | 已关 R 的性能优化 |
| 「R7 加场景」「R5 加 HUD」「R3 加指标」「R7 加复杂度」 | 已关 R 的场景/HUD/指标增量 |
| 「已关的 R4 加新场景」 | 显式点名已关 R 增量 |

---

## 3. 完整闭环图

```
                ┌─── [wr-quality] 定标准 ────┐
                │                              │
   新 R  ──────►│                              ├──► 写代码 ──► 跑GPU取JSON ──► [metrics-audit] 审指标
                │                              │                                    │
                │                              │                                    ├─ 装绿/假值 ─► [wr-rework] 反攻改真窗/修引擎洞 ─► 回到重跑
                │                              │                                    │
                │                              │                                    └─ 真绿 ─► [wr-close] 判门禁+回写docs
                │                              │                                                    │
                │                              │                                                    ├─ 门禁不过 ─► [wr-rework] ─► 回到重跑
                │                              │                                                    │
                │                              │                                                    └─ 门禁过 ─► R 关闭 ✅
                │                              │
                │                              │
                └── 已关闭的 R 想优化/加场景 ──► [wr-optimize] 改代码 ─► 重跑 ─► 重审验没回归 ─► 重关 ─► 回写docs §2/§10
```

**5 个 skill 各管一段，正向写、反向修、横向优化都有 skill 接，闭环。**

---

## 4. 典型串联场景

### 4.1 场景 A：W3 阶段关 R7（未关，完整闭环）

```
你: 写 R7 真窗
  → wr-quality 定标准 + 写代码

你: 关闭 R7
  → wr-close 跑 GPU + 判门禁 + 串 metrics-audit 审指标 + 回写 docs §2

若门禁 FAIL:
你: 反攻 R7
  → wr-rework 分类（A 指标层 / B 真窗代码层 / C 引擎洞）
  → 修 → 重跑 → 重审 → 重关
```

### 4.2 场景 B：已关的 R4 想优化降 draw call

```
你: 优化 R4，降 draw call
  → wr-optimize 前置校验（R4 当前 ✅）
  → 状态标注 ✅ → ✅🔄 优化中
  → 改 main.go（合批/Picture 缓存）
  → 重跑 GPU + metrics-audit 验没回归 + wr-close 重关
  → 状态升维 ✅🔄 → ✅v2-optimized + §10 修订表加版本
```

### 4.3 场景 C：已关的 R3 场景太简陋，推倒重写升维

```
你: 重写已关的 R3 真窗，升维
  → wr-rework 状态降级 ✅ → 🔄 质量返工
  → 第 2 步重写（按 wr-quality 场景矩阵 + 实现点六维）
  → 重跑 + 重审真绿 + 重关
  → 状态升维 🔄 → ✅v2（质量条）+ §10 修订表
```

### 4.4 场景 D：怀疑 R7 指标装绿，定向审查

```
你: 审 R7 的指标，怀疑 fps 假值
  → metrics-audit 第 1–5 步全套审查
  → 若 SUSPECT/FAKE → 交 wr-rework 反攻改真窗/修引擎洞
  → 若 HONEST/EARNED → 报告「指标诚实」，无需反攻
```

### 4.5 场景 E：已关的 R7 想加新滚动场景 + 新 HUD 字段

```
你: R7 加场景 + 加 HUD
  → wr-optimize 前置校验（R7 当前 ✅）
  → 状态标注 ✅ → ✅🔄 增量中
  → 改 main.go（按 wr-quality U17 加场景 + U18 加 HUD）
  → 改 README（Visible effect + Gates 加新行）
  → 重跑 + metrics-audit 验没回归 + wr-close 重关
  → 状态升维 ✅🔄 → ✅v2-extended + §10 修订表加版本
```

---

## 5. 5 个 skill 的分工边界

### 5.1 谁管什么

| 动作 | 归哪个 skill |
|------|--------------|
| 写代码前的场景矩阵 + HUD + 实现点六维标准 | `wr-quality` |
| 写完代码后的建窗/跑/判门禁/回写 docs §2 | `wr-close` |
| 指标族 A–J 字段完备性/诚实性/阈值防偷放/观测一致性/降画质检测 | `metrics-audit` |
| 门禁 FAIL / 指标装绿识破后的反攻/修复（代码层 + 引擎层） | `wr-rework` |
| 已关 R 的优化/增量（性能 + 场景 + HUD + 指标） | `wr-optimize` |

### 5.2 谁不管什么

| 动作 | 不归哪个 skill | 该归谁 |
|------|----------------|--------|
| 未关 R 的首次关闭 | `wr-optimize`（只接已关 R） | `wr-quality` + `wr-close` |
| 门禁 FAIL 的反攻修复 | `wr-optimize`（只接优化/增量） | `wr-rework` |
| 指标层 JSON marshal bug | `wr-rework`（只接代码层 + 引擎层） | `metrics-audit` |
| 写代码前的标准定 | `wr-close`（只接执行流程） | `wr-quality` |
| 类 C 引擎洞 | `wr-optimize`（优化不修洞） | `wr-rework` 第 3 步 |

### 5.3 状态转换矩阵（docs §2 + §10）

| 场景 | 状态转换 | §10 修订表 |
|------|----------|------------|
| 未关 R 首次关闭 | `⬜ → ✅` | `<版本> \| W<n> ✅ <R id> 真窗 + C<组合>` |
| 已关 R 反攻升维 | `✅ → 🔄 质量返工 → ✅v2（质量条）` | `<版本> \| R<id> 质量升维 ✅v2：<反攻摘要>` |
| 已关 R 优化 | `✅ → ✅🔄 优化中 → ✅v2-optimized` | `<版本> \| R<id> 优化生效：<优化前 → 优化后 指标对比>` |
| 已关 R 增量 | `✅ → ✅🔄 增量中 → ✅v2-extended` | `<版本> \| R<id> 增量生效：<加了什么 + 新指标值>` |

---

## 6. `wr-rework` 的 3 类分类决策树

反攻/修复时，FAIL 原因必须先分类，不同类走不同路径：

```
FAIL 原因 → 分类
├─ 「JSON 字段缺/无名/null 无原因」
│   → 类 A 指标层 bug
│   → 交回 metrics-audit 第 1/4 步修 JSON marshal
│   → 不走 wr-rework
│
├─ 「场景简陋/HUD 不见/相位缺/实现点缺/阈值偷放」
│   → 类 B 真窗代码层 bug
│   → wr-rework 第 2 步：重写 examples/ui_wr_*/main.go
│   → 按 wr-quality 场景矩阵 + 实现点六维重写
│   → 禁止删场景保门禁、禁止降阈值
│
└─ 「复杂场景下引擎炸/BoundaryCache 不 skip/壳 retained 闪」
    → 类 C 引擎洞
    → wr-rework 第 3 步：定点修 ui/rendering/* 或 ui/embedder/*
    → 按 wr-quality §5「高概率洞表」定位
    → 禁止在示例里绕洞
    → 修完跑 go test ./ui/... 确认没回归
```

---

## 7. `wr-optimize` 的前置校验

优化/增量前必须过前置校验，不过则不进本 skill：

```
已关 R（✅）想优化/增量
│
├─ 校验 1：该 R 当前状态必须 ✅（已关）
│   ├─ ⬜ 未关 → 交 wr-quality + wr-close 走首次关闭
│   ├─ 🔄 质量返工中 → 交 wr-rework 走反攻
│   └─ ✅ → 通过
│
├─ 校验 2：优化/增量目标必须在 RENDER_BASE §22.2/§25.5 范围内
│   ├─ 超出（如要加 PlatformView）→ 停下问用户，这是远期 D 行
│   └─ 在范围内 → 通过
│
└─ 校验 3：该 R 的 §2 主表门禁不能因优化放宽
    ├─ 优化方案需降门禁阈值 → 停下报告，门禁是硬的不许放
    └─ 不降门禁 → 通过
```

---

## 8. 共享接口约定

5 个 skill 之间的数据传递接口：

| 接口方向 | 内容 |
|----------|------|
| `metrics-audit` → `wr-rework` | 审查报告：FAIL 字段 + 假绿模式（SUSPECT/FAKE/LOOSE/GATE_OFF/SCENE_CHEAT/TIME_CHEAT/IDLE_CHEAT） |
| `wr-close` 第 3 步 FAIL → `wr-rework` | 门禁不过的族 + 原因 |
| `wr-rework` → `wr-close` | 反攻修完 + 重跑 + 重审真绿后，交 wr-close 走第 5 步回写 docs §2 状态列 |
| `wr-rework` → `metrics-audit` | 反攻修完取新 JSON 后，交 metrics-audit 跑 §1–§5 验真绿 |
| `wr-optimize` → `metrics-audit` | 优化/增量改完取新 JSON 后，交 metrics-audit 跑 §1–§5 验没回归 |
| `wr-optimize` → `wr-close` | 优化/增量验没回归后，交 wr-close 走第 5 步回写 docs §2 + §10 |
| `wr-quality` → 所有 | 场景矩阵 + HUD 字段清单 + 实现点六维作为**验收基准** |

**关键：** 5 个 skill 通过 docs §2 状态列 + §10 修订表 + JSON 文件传递状态，不通过共享内存或全局变量。

---

## 9. 禁令总表（5 个 skill 共享）

以下禁令在 5 个 skill 中都适用，违反任一即 FAIL：

| # | 禁令 | 出处 |
|---|------|------|
| 1 | 不删场景保门禁 | wr-quality U17 + §5 |
| 2 | 不降 README 阈值过门禁 | metrics-audit §3 |
| 3 | 不在示例里绕引擎洞 | wr-rework 第 3 步 |
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
| 16 | 优化只改 examples/ui_wr_*/main.go 实现细节，不触及引擎架构 | wr-optimize 第 1.2 步 |
| 17 | 引擎洞修完跑 go test ./ui/... 确认没回归 | wr-rework 第 3.2 步 |

---

## 10. 开发实战速查

### 10.1 W3 阶段（你现在所在）的推荐流程

```
1. 你: 写 R7 真窗
   → wr-quality 定场景矩阵 + HUD + 实现点六维
   → 写 examples/ui_wr_r7_virtlist/main.go + README.md

2. 你: 关闭 R7
   → wr-close 跑 GPU 取 JSON
   → 判 §2.2 全族门禁（族 A–J）
   → 串 metrics-audit 审指标诚实性
   → 若全绿：回写 docs §2 R7 行 ⬜ → ✅ + §10 修订表
   → 若 FAIL：进第 3 步

3. 你: 反攻 R7
   → wr-rework 分类 FAIL 原因
   → 类 A：交 metrics-audit 修 JSON marshal
   → 类 B：重写 main.go（按 wr-quality 场景矩阵）
   → 类 C：定点修 ui/rendering/* 或 ui/embedder/*
   → 重跑 + 重审真绿 + 重关

4. 同理关 R7b、R10、C3
```

### 10.2 已关 R 的优化/增量流程

```
1. 你: 优化 R4，降 draw call
   → wr-optimize 前置校验（R4 ✅）
   → 状态标注 ✅ → ✅🔄 优化中
   → 改 main.go（合批/Picture 缓存）
   → 重跑 GPU + metrics-audit 验没回归
   → wr-close 重关 + 状态升维 ✅🔄 → ✅v2-optimized
   → §10 修订表加版本

2. 你: R7 加场景 + 加 HUD
   → wr-optimize 前置校验（R7 ✅）
   → 状态标注 ✅ → ✅🔄 增量中
   → 改 main.go（按 wr-quality U17 加场景 + U18 加 HUD）
   → 改 README（Visible effect + Gates 加新行）
   → 重跑 + metrics-audit 验没回归 + wr-close 重关
   → 状态升维 ✅🔄 → ✅v2-extended + §10 修订表
```

### 10.3 怀疑指标装绿的审查流程

```
1. 你: 审 R7 的指标，怀疑 fps 假值
   → metrics-audit 第 1–5 步全套审查
   → 第 1 步：字段完备性（族 A–J 不默默省略）
   → 第 2 步：诚实性（vsync_source 不假锁、fps_interval 不靠静帧刷高）
   → 第 3 步：门禁阈值防偷放（README 阈值不高于文档默认）
   → 第 4 步：指标与代码观测一致性（JSON 累计数 = 代码实际）
   → 第 5 步：降画质装绿检测（场景达 U17、RUN_SECONDS ≥ 关闭用值）
   → 若 SUSPECT/FAKE：交 wr-rework 反攻
   → 若 HONEST/EARNED：报告「指标诚实」，无需反攻
```

---

## 11. 一句话总结

> **5 个 skill 各管一段：wr-quality（写前定标准）→ wr-close（执行关闭）→ metrics-audit（指标层审查）→ wr-rework（反攻/修复回流）→ wr-optimize（优化/增量回流）。正向写、反向修、横向优化都有 skill 接，docs §2 状态列 + §10 修订表贯穿全流程，闭环。**
