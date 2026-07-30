---
name: gpui-wr-debug
description: §R 真窗 bug 修复调度入口——指定窗口出现 bug（闪屏 / 渲染错 / skip 失效 / 指标假绿 / 门禁偷放 等）时触发。强制复现（跑该 R 的 GPU 真窗取 §2.2 全族 JSON）→ 对照 §2.2 FAIL 线 + §2.5 关闭用时长定位 bug 所在层 → 按 AGENTS.md 分层风险规则回流：ui/rendering ui/embedder ui/scene ui/io 低风险本 skill 自修；render/ 高风险 + gpu/ 最高风险停下 question 确认后交 wr-engine；指标不诚实/门禁偷放交 metrics-audit；已关 ✅ 要升维交 wr-close 模式 2、要优化交 wr-close 模式 3；能力根本没实现交 wr-implement。当用户说「修 R3 的 bug」「debug R7」「R12 闪屏 fix」「R4 渲染错」「R11 boundary 漏 skip」「R14 slope 偷放」「R9 measure_cache 假值」「指定窗口 $Rn 出现 $现象定位修复」时触发。本 skill 是调度入口层，不替代被回流的 skill 的内部闭环。
user_invocable: true
disable_model_invocation: false
---

# gpui-wr-debug — §R 真窗 bug 修复调度入口（复现 → 定位层 → 分层回流）

> **与收敛后 5 skill 的分工：**
> - `wr-implement` = 能力实现回流（ui/ 里实现 + 单测 + 指标接线）——**本 skill 发现「能力根本没实现」时回流**
> - `wr-close` = 关 R 完整生命周期（3 模式：首次关闭 / 反攻重关 / 优化重关）——**本 skill 发现「已关 ✅ 要升维」回流模式 2 / 「要优化」回流模式 3 / 「首次关 ⬜ → 其实不该走 debug」分流**
> - `metrics-audit` = 指标族 A–J 正误审查与 bug 修复（指标层）——**本 skill 发现「指标不诚实 / 门禁偷放」时回流**
> - `wr-engine` = 底层修复回流（render / gpu / ui-scene 谨慎改 + 跨层影响面评估）——**本 skill 发现底层洞（render/gpu）时停下 question 确认后回流**
> - `wr-rewrite` = W 矩阵级调度（推翻 W<n> 重写）——**不接**（W 矩阵级重写归 wr-rewrite，本 skill 只接「指定窗口的 bug 定位 + 分层回流」）
> - **本 skill** = 指定窗口 bug 修复调度入口（强制复现 → 定位层 → 分层回流）——**调度入口层 + ui 低风险自修**
>
> **真源：** `AGENTS.md`（分层风险与修复规则）+ `docs/ENGINE_UI_WIDGET_RENDER.md` §2 主表（该 R 行的 PACKAGE / WINDOW / CLOSE_SECONDS / 指标门禁 / 状态）+ §2.2.1–§2.2.5（指标族 A–J 必采字段 + FAIL 线 + JSON 最小外壳）+ §2.5（按主能力关闭用 RUN_SECONDS 全表）+ §6 模块落点 + §10 修订表；`docs/ENGINE_UI_RENDER_BASE.md` §20.2 M-\* 全表 + §22.1 施工表（前驱序）+ §25.4 架构诚实点 6 条。若本 skill 与真源矛盾，以真源为准——发现矛盾停下报告，不要自决。

## 0. 为什么要这个 skill

历史「修指定窗口 bug」有 4 类错误回流：

| 错误回流 | 表现 | 为何危险 |
|----------|------|----------|
| **凭描述直接改 examples/ui_wr_*/main.go** | 用户说「R7 闪屏」，直接去 main.go 加防闪逻辑 | bug 可能在 ui/rendering（VirtualList）或 ui/embedder（PresentPolicy），改 examples 是绕洞 |
| **不强制复现** | 用户说「R12 指标假」，直接审 JSON | 假指标必须**重新跑真窗**取新 JSON 才能判；旧 JSON 可能已被改 |
| **render/gpu 不停下确认擅自改** | bug 定位到 `gpu/swapchain.go` 的 LoadOpLoad，直接改 | 违反 AGENTS.md：gpu/ 最高风险，必须 question 确认；可能破坏所有 R 的 Present + wgpu 升级回归 |
| **bug 定位后回流错 skill** | 「R3 boundary 文不 skip」交给 wr-close 模式 2（反攻重关） | 这是引擎洞（ui/rendering/boundary_cache.go），该交 wr-engine；wr-close 模式 2 是「已关 ✅ 升维」，不是修底层洞 |

本 skill 把「指定窗口 bug → 强制复现取 JSON → 定位 bug 所在层 → 按 AGENTS.md 分层风险规则回流对应 skill」这条回流**固化**，禁止凭描述直接改、禁止不强制复现、禁止 render/gpu 不停下确认擅自改、禁止回流错 skill。

---

## 1. 触发与输入

用户可能给：

- 一个 R/C id + bug 现象（如「修 R3 的 bug——boundary 文不 skip」「debug R7——闪屏」「R12 闪屏 fix——HUD 数字跳」）→ 定点修
- 一个 R/C id + 「指标假」怀疑（如「R14 slope 偷放」「R9 measure_cache_hit 假值」「R4 fps 假值」）→ 重新跑真窗 + 交 metrics-audit
- 一个 R/C id + 「渲染错」描述（如「R4 渲染错——retained 下壳闪」「R11 渲染错——resize 后 boundary 漏」）→ 定位层 + 回流
- 一个「指定窗口 $Rn 出现 $现象，定位修复」综合请求 → 完整回流

从输入解析：

- `ABILITY_ID`（必给）：如 `R3`、`R7`、`R12`、`C3`（原样保留大小写）
- `BUG_DESC`（若给）：bug 现象描述（如「boundary 文不 skip」「retained 下壳闪」「measure_cache_hit 假值」）
- `SUSPECT_LAYER`（若给）：用户已怀疑的层（如 `ui/rendering` / `ui/embedder` / `render` / `gpu`）
- `SUSPECT_FIELD`（若给）：用户已怀疑的指标字段（如 `measure_cache_hit` / `rss_slope_kb_per_min`）

**输入校验（硬）：**

1. `ABILITY_ID` 必给。没给 → 停下问用户「要修哪个窗口的 bug？（R<C><id>）」
2. `ABILITY_ID` 必须在 `docs/ENGINE_UI_WIDGET_RENDER.md` §2 主表或 §3 组合表里能查到。查不到 → 停下问用户「ABILITY_ID `<id>` 在主表查不到，是不是写错了？」
3. 该 R 当前状态：
   - `⬜`（未关）→ 停下问用户「R<id> 还没关（⬜），bug 修复流程的前提是「已有真窗可复现」。要先首次关闭（走 wr-close 模式 1）吗？还是该真窗根本没建？」
   - `✅` / `🔄` / `✅v2` / `✅🔄` → 继续（已关窗有 bug 走 debug；未关窗有 bug 走 wr-close 模式 1 首次关闭，不走 debug）

## 第 0 步：读真源 + 定位窗口

**必做**——每次都先读，禁止凭记忆跑流程（真源会变）。

### 0.1 读 WIDGET_RENDER §2 主表该 R 行

`read_file docs/ENGINE_UI_WIDGET_RENDER.md`，定位 §2 主表里该 id 行，取六列硬值：

- `ABILITY`（能力定义，如 R3 = 「嵌套 RenderBoundary」）
- `PACKAGE`（如 `ui_wr_r3_boundary`）
- `WINDOW` = **1200×800**（必须是这个；不是就停下，文档被改错了）
- `CLOSE_SECONDS`（§2 主表「推荐 RUN_SECONDS」列 + §2.5 关闭用时长表，**以 §2.5 为准**，如 R3=15、R7=60、R14=60）
- `指标门禁（须 FAIL）`（如 R3 = 「`boundary_skip>0`；`boundary_rerecord` 仅脏；p95/hitch；RSS」）
- 当前 `状态`（⬜ / ✅ / 🔄 / ✅v2 / ✅🔄）

### 0.2 读 WIDGET_RENDER §2.2.1–§2.2.5 + §2.5 关闭用时长表

`read_file docs/ENGINE_UI_WIDGET_RENDER.md` §2.2.1（指标族 × 真窗义务）、§2.2.2（帧时 / FPS 60+ 硬门禁）、§2.2.3（CPU 硬门禁）、§2.2.4（内存硬门禁）、§2.2.5（每个真窗 JSON 最小外壳）+ §2.5 关闭用 RUN_SECONDS 全表。

**这些是复现取 JSON + 判门禁的基准**，每次都要对照，禁止凭记忆。

### 0.3 读 RENDER_BASE §22.1 施工表（前驱序）

`read_file docs/ENGINE_UI_RENDER_BASE.md` §22.1，按该 R 能力找对应施工序，取：

- 施工序号（如 R7 → 序 12）
- 前驱序（如序 12 前驱 = 3,7）
- ① 单测 + ② 指标（验收三项的前两项）

**前驱序检查**：若该 R 的前驱序状态 `🔄` 或 `⬜`（没收口），**停下报告**：「R<id> 的前驱序 <序> 还没收口（状态 <🔄/⬜>），bug 可能源于前驱能力不完整。建议先做前驱（走 wr-implement），而不是走 debug。」让用户决定方向。

### 0.4 读 AGENTS.md 分层风险规则

`read_file AGENTS.md`，取分层风险与修复规则（关键纪律）：

| 层 | 风险 | 修复规则 |
|----|------|----------|
| `ui/rendering/` `ui/embedder/` `ui/scene/` `ui/io/` | 低 | 能力实现层，可直接按场景修 |
| `render/` | 高 | 必须 question 确认后修 |
| `gpu/` | 最高 | 必须 question 确认后修 |

+ 「禁止在示例层绕引擎洞（改 examples/ 而不改引擎层 ui/ render/ gpu/）」
+ 「JSON 门禁绿 ≠ 引擎洞已修；洞修了才能标 ✅v2」
+ 「所有 skill 的「停下报告」节点必须停，用 question 工具问用户确认方向后再继续」

### 0.5 读 §10 修订表 + §6 模块落点

`read_file docs/ENGINE_UI_WIDGET_RENDER.md` §10 修订表（取当前最新版本号，如 3.1）+ §6 模块落点表（确认 bug 该落哪个包——Present/policy → ui/embedder；脏 layout/VirtualList → ui/rendering；Picture/Layer/Composite → ui/scene；指标 → ui/scheduler）。

## 第 1 步：强制复现——跑该 R 真窗取 §2.2 全族 JSON

**禁止跳过**——这是 debug 的硬前提。不跑真窗，没新 JSON，无法判 bug 在哪层。

### 1.1 复现命令

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=<CLOSE_SECONDS> go run ./examples/<PACKAGE>
```

- `RUN_SECONDS` 取 §2.5 关闭用值（如 R3=15、R7=60、R14=60），**不可缩短**
- 窗口必须 1200×800（U15）
- 若环境无 GPU/X11：**停下**，告诉用户「需要 GPU 真窗环境（needs_gpu_window），debug 无法继续」，禁止用 CPU stub 或单测复现

### 1.2 取 JSON + stderr

跑完取：

- stderr（含 `FAIL:` 原因，若 `exit 1`）
- stdout JSON（§2.2.5 最小外壳，全族 A–J 字段）

**关键：** 若用户给的 `BUG_DESC` 是「指标假」（如「R14 slope 偷放」「R9 measure_cache_hit 假值」），**必须重新跑真窗**取新 JSON，**禁止用历史 JSON**——历史 JSON 可能已被改，不是 bug 现场。

### 1.3 复现确认

| 复现结果 | 处理 |
|----------|------|
| bug 复现（看到 FAIL / JSON 字段异常 / 现象重现） | 进第 2 步定位层 |
| bug 未复现（跑出来全绿） | 停下报告用户「本次复现未重现 bug——可能是间歇性 / 已被其他 skill 修复 / 现象描述与代码不符。建议：① 加长 RUN_SECONDS 再跑；② 描述更具体的复现场景；③ 怀疑已被修，跑 `git log` 查最近改动」 |
| 跑不起来（编译错 / GPU 不可用） | 停下报告用户「复现失败：<原因>。需先解决环境问题」 |

## 第 2 步：定位 bug 所在层（核心步骤）

**必做**——按 JSON 族 FAIL 线 + §6 模块落点定位 bug 所在层。禁止凭 `BUG_DESC` 直接猜层。

### 2.1 判 JSON 族 FAIL 线（对照 §2.2.2–§2.2.4）

按该 R 的 §2 主表「指标门禁」列 + §2.2.2（FPS）/ §2.2.3（CPU）/ §2.2.4（内存）逐条判：

| 族 | FAIL 线 | bug 可能所在层 |
|----|---------|----------------|
| **A 帧时** | `fps_interval<55` 或 `interval_p95_ms>22`（动画/滚动窗） | `ui/scheduler`（VSync）+ `ui/embedder`（PresentPolicy）+ `ui/rendering`（脏 layout 风暴） |
| **B 管线** | `pipeline_depth` 持续 > 配置上限 | `ui/embedder`（PipelineApp）+ `render`（画布 submit） |
| **C 脏区** | 能力专用字段异常（如 R3 `boundary_skip=0` 但场景有静文） | `ui/rendering/boundary_cache.go` + `ui/scene`（Picture 录回放） |
| **D CPU** | `cpu_pct_avg>85%` 且持续（长窗） | `ui/rendering`（layout 风暴）+ `ui/scheduler`（采样） |
| **E 内存** | `rss_slope_kb_per_min>30000`（压力窗） | `ui/rendering`（缓存不淘汰）+ `ui/scene`（Layer 常驻） |
| **F GPU** | `cpu_fallback_ops` 无故暴涨 | `gpu`（device/swapchain）+ `render`（submit 路径） |
| **G 图/文** | `measure_cache_hit` 假值 / R9/R10 强制字段缺失 | `ui/rendering`（measure cache）+ `ui/io`（decode worker） |
| **H 启动** | `time_to_first_present_ms` 超预算 | `ui/embedder`（首帧 Present） |
| **J 正确性** | `import "C"` 命中 / ui import gpu / 本窗降画质 | 构建期 hook（不改代码层） |

### 2.2 按现象 + §6 模块落点定位层

| 现象 / FAIL 族 | 定位层 | 模块落点（§6） |
|----------------|--------|----------------|
| BoundaryCache 文/嵌套静区不 skip（R3/R3b/C1） | `ui/rendering/boundary_cache.go` | ui/rendering |
| 壳在 retained 下闪/残（R4/R4b/C2） | `ui/embedder/pipeline_app.go` | ui/embedder |
| DirtyLayerIDs 每帧重建，R4b id 不稳 | `ui/rendering/` 或 `ui/embedder/` | ui/rendering + embedder |
| render Picture 不支持回放（R5/R21） | `ui/scene/picture.go` 或 `render/` | ui/scene + render |
| gpu/swapchain.go 的 LoadOpLoad 不对（R4 retained） | `gpu/swapchain.go` | gpu |
| VirtualList bind_count=item_count（R7 虚拟化失效） | `ui/rendering/virtual_list.go` | ui/rendering |
| scroll_rerecord 字段缺失（R7b） | `ui/rendering/virtual_list.go` 或 `scrollable.go` | ui/rendering |
| AsyncImage worker 缺（R10） | `ui/io/decode.go` | ui/io |
| measure_cache_hit 假值（R9） | `ui/rendering/`（measure cache）+ `ui/scheduler/metrics.go`（字段接线） | ui/rendering + scheduler |
| overlay_paint_count 涨（R8） | `ui/overlay/` + `ui/embedder` | ui/overlay + embedder |
| cache_entries 淘汰失效（R14） | `ui/rendering/`（SaveLayerBudget）+ `ui/scheduler`（字段接线） | ui/rendering + scheduler |
| 指标字段名不一致 / null 无原因 / JSON marshal 错 | 指标层（`ui/scheduler/metrics.go` + `ui/embedder/pipeline_app.go` 接线） | metrics-audit 范围 |
| 门禁阈值偷放 / slope_gate=off 滥用 / 降画质装绿 | 指标审查层 | metrics-audit 范围 |

### 2.3 代码定位（按 §2.2 定位层）

按定位层 `grep` / `read_symbol` / `list_symbols` 看缺陷代码：

```bash
# 例：BoundaryCache 文不 skip
grep -n "FrameSkip\|record\|OnPaint" ui/rendering/boundary_cache.go
read_symbol ui/rendering/boundary_cache.go FrameSkip

# 例：retained 下壳闪
grep -n "useRetained\|CompositeOnly\|LoadOpLoad\|PresentPolicy" ui/embedder/pipeline_app.go

# 例：VirtualList bind_count 失效
grep -n "BindCount\|bind_count" ui/rendering/virtual_list.go ui/scheduler/metrics.go
```

### 2.4 定位结论输出

```
📋 R<id> bug 定位

ABILITY_ID：<R3 等>
BUG_DESC：<boundary 文不 skip 等>
当前状态：<⬜ / ✅ / 🔄 / ✅v2 / ✅🔄>

复现：
  命令：RUN_SECONDS=<CLOSE_SECONDS> go run ./examples/<PACKAGE>
  JSON 路径：stdout
  bug 复现：<是 / 否>
  复现证据：<FAIL 行 / 异常字段值 / 现象截图描述>

JSON 族 FAIL 线判定：
  族 A 帧时：<PASS / FAIL（fps_interval=XX <55）>
  族 B 管线：<PASS / FAIL>
  族 C 脏区：<PASS / FAIL（boundary_skip=0 但场景有静文）>
  族 D CPU ：<PASS / FAIL>
  族 E 内存：<PASS / FAIL>
  族 F GPU ：<PASS / FAIL>
  族 G 图/文：<PASS / FAIL / N/A>
  族 H 启动：<PASS / FAIL / N/A>
  族 J 正确：<PASS / FAIL>

定位层：<ui/rendering / ui/embedder / ui/scene / ui/io / render / gpu / 指标层 / 能力未实现>
定位文件：<file.go>
定位函数/字段：<func / field>
风险等级（按 AGENTS.md）：<低 ui/* / 高 render / 最高 gpu>
```

## 第 3 步：分层回流（按 AGENTS.md 分层风险规则）

**核心步骤**——按定位层 + 风险等级回流对应 skill。本 skill 严格按 AGENTS.md「所有 skill 的『停下报告』节点必须停，用 question 工具问用户确认方向后再继续」执行。

### 3.1 回流决策树

```
按 §2.4 定位结论：

├─ 定位层 = ui/rendering / ui/embedder / ui/scene / ui/io（低风险能力实现层）
│   ├─ bug 是「能力实现缺陷」（如 boundary_cache.go 的 skip 逻辑错）
│   │   → 本 skill 第 3.2 步「ui 低风险自修」
│   └─ bug 是「能力根本没实现」（如 R7 VirtualList 文件不存在）
│       → 交 wr-implement（能力实现回流）
│       └─ 告知 wr-implement：ABILITY_ID / SUSPECT_GAP / SOURCE_SKILL=wr-debug
│
├─ 定位层 = render/（高风险）
│   → 停下 question 确认（AGENTS.md：render/ 必须 question 确认后修）
│   └─ 用户确认 → 交 wr-engine
│       └─ 告知 wr-engine：HOLE_DESC / SUSPECT_LAYER=render / SUSPECT_FILE / SOURCE_SKILL=wr-debug
│
├─ 定位层 = gpu/（最高风险）
│   → 停下 question 确认（AGENTS.md：gpu/ 最高风险，必须 question 确认后修）
│   └─ 用户确认 → 交 wr-engine
│       └─ 告知 wr-engine：HOLE_DESC / SUSPECT_LAYER=gpu / SUSPECT_FILE / SOURCE_SKILL=wr-debug
│
├─ 定位层 = 指标层（指标不诚实 / 门禁偷放 / 字段名不一致 / null 无原因）
│   → 交 metrics-audit（指标层正误审查 + bug 修复）
│   └─ 告知 metrics-audit：ABILITY_ID / SUSPECT_FIELD / 任务=审该 R 的 JSON 指标诚实性
│
├─ 定位层 = 「已关 ✅ 真窗代码层 bug」（U17 场景简陋 / HUD 不见 / 相位缺 / 实现点缺）
│   └─ 该 R 当前 ✅ → 交 wr-close 模式 2（反攻重关）
│       └─ 告知 wr-close：ABILITY_ID / 模式=2 / SOURCE_SKILL=wr-debug / bug 摘要
│
└─ 定位层 = 「已关 ✅ 性能 / 场景 / HUD / 指标 优化增量」
    └─ 该 R 当前 ✅ → 交 wr-close 模式 3（优化重关）
        └─ 告知 wr-close：ABILITY_ID / 模式=3 / SOURCE_SKILL=wr-debug / 优化目标
```

### 3.2 ui 低风险自修（本 skill 自己改 ui/ 低风险层）

**适用条件**（全部满足才走本步）：

- 定位层 ∈ {`ui/rendering/`, `ui/embedder/`, `ui/scene/`, `ui/io/`}
- bug 是「能力实现缺陷」（不是「能力根本没实现」）
- 改动局部（单函数 / 单字段）
- 不触及引擎架构（如 multi-RT 层纹理 compositor 属 W6，不属本波）

**修复动作：**

1. `read_symbol` 看缺陷函数完整代码
2. `edit_file` 定点修（改最小集，不大重构）
3. 跑改动单测：
   ```bash
   # 例：改了 ui/rendering/boundary_cache.go
   go test ./ui/rendering -run TestBoundaryCache -count=1 -v
   ```
4. 跑全 ui 包回归：
   ```bash
   go test ./ui/... -count=1
   ```
5. **重跑该 R 真窗验证 bug 已修**（必须重跑，禁止只靠单测绿就说修好）：
   ```bash
   RUN_SECONDS=<CLOSE_SECONDS> go run ./examples/<PACKAGE>
   ```

**修复纪律（硬）：**

- **禁止在示例层绕引擎洞**（AGENTS.md）——该改 ui/rendering 或 render，不改 examples/ui_wr_*/main.go 绕洞
- **禁止降画质装绿**——不在 ui/rendering / render / gpu 层放宽门禁
- **禁止 ui→gpu 依赖**（CODING_RULES §0.4）
- **禁止 CGO**（purego）
- 改动局部化，不破坏现有 API 契约（若破坏，必须同步改上游调用 + 跑全回归）

**自修失败回流：**

| 自修结果 | 处理 |
|----------|------|
| 单测绿 + 真窗重跑 bug 已修 | 进第 4 步回写 docs |
| 单测 FAIL | 定位是实现 bug 还是单测期望错——实现 bug 修实现；单测期望错修单测（但不许降单测期望） |
| 真窗重跑 bug 仍在 | 停下报告：「ui/ 自修后真窗重跑 bug 仍在，可能定位层错了或 bug 涉及多层。建议：① 重新定位层（回第 2 步）；② 怀疑底层洞，交 wr-engine」 |
| 改了 ui/embedder/pipeline_app.go 引发全回归 | 停下报告：「改 pipeline_app.go 影响所有 R 的 Present，建议先跑全回归确认影响面」 |

### 3.3 render/ 停下 question 确认 → 交 wr-engine

**触发条件：** §2.4 定位层 = `render/`（高风险，AGENTS.md：render/ 必须 question 确认后修）。

**停下 question（必须用 request_user_input）：**

告知用户：
- bug 定位到 `render/`（画布/绘制 API 基座），风险等级：高
- 改 render/ 影响所有 R 的绘制基座
- 评估要点：改的是哪个 API？改了影响哪些上游（ui/rendering 调 render / ui/scene 调 render）？改了影响哪些下游（gpu 的 RT 隔离 / submit 路径）？

问用户：
- 选项 A：确认改 render/ → 交 wr-engine（wr-engine 走第 1 步跨层影响面评估 + 第 2 步定点修 + 第 3 步跑回归）
- 选项 B：不改 render/，尝试在 ui/rendering 层绕过（降风险）
- 选项 C：停下不修（该 R 保持 🔄 或 ⬜）

**用户选 A：** 交 wr-engine，告知：

```
use_skill gpui-wr-engine
告知 wr-engine：
  - HOLE_DESC：<bug 描述，如「render Picture 不支持回放」>
  - SUSPECT_LAYER：render
  - SUSPECT_FILE：<具体文件，如 render/picture.go>
  - SOURCE_SKILL：wr-debug
  - ABILITY_ID：<R id>
  - 修复完后交回 wr-debug 第 3.4 步「交回 wr-debug 跑真窗验证」
```

**用户选 B：** 本 skill 第 3.2 步「ui 低风险自修」尝试在 ui/rendering 层绕过（但**禁止在 examples 里绕**）。

**用户选 C：** 停下不修，输出报告说明 bug 未修原因。

### 3.4 gpu/ 停下 question 确认 → 交 wr-engine

**触发条件：** §2.4 定位层 = `gpu/`（最高风险，AGENTS.md：gpu/ 最高风险，必须 question 确认后修）。

**停下 question（必须用 request_user_input）：**

告知用户：
- bug 定位到 `gpu/`（直接对接 GPU 后端 wgpu native + 系统句柄），风险等级：最高
- 改 gpu/ 可能破坏所有 R 的 Present + wgpu 版本升级回归
- 评估要点：改的是哪个 API（Device / Swapchain / Queue / BindGroup / libwgpu_native 绑定）？GPU 驱动兼容性（Mesa / AMD / Intel / NVIDIA）？wgpu 版本兼容性？是否有替代方案（改 render 层绕过）？影响面是否超阈值？

问用户：
- 选项 A：确认改 gpu/ → 交 wr-engine（wr-engine 强制停下报告 GPU 驱动兼容性 + wgpu 版本兼容性 + 替代方案 + 影响面）
- 选项 B：不改 gpu/，尝试在 render 层或 ui/rendering 层绕过（降风险）
- 选项 C：停下不修（该 R 保持 🔄 或 ⬜）

**用户选 A：** 交 wr-engine，告知：

```
use_skill gpui-wr-engine
告知 wr-engine：
  - HOLE_DESC：<bug 描述，如「gpu/swapchain.go 的 LoadOpLoad 不对」>
  - SUSPECT_LAYER：gpu
  - SUSPECT_FILE：<具体文件，如 gpu/swapchain.go>
  - SOURCE_SKILL：wr-debug
  - ABILITY_ID：<R id>
  - 修复完后交回 wr-debug 第 3.5 步「交回 wr-debug 跑真窗验证」
```

**用户选 B：** 本 skill 第 3.2 步「ui 低风险自修」或第 3.3 步「render/ 停下 question」。

**用户选 C：** 停下不修。

### 3.5 交回 wr-debug 跑真窗验证（wr-engine 修完后）

wr-engine 修完底层 + 跑回归全绿后，交回本 skill 跑真窗验证 bug 已修：

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=<CLOSE_SECONDS> go run ./examples/<PACKAGE>
```

**验证通过**（bug 已修 + §2.2 全族门禁绿）→ 进第 4 步回写 docs。

**验证未通过**（bug 仍在或新 FAIL）→ 停下报告：「底层修完后真窗重跑 bug 仍在 / 新 FAIL，可能：① bug 涉及多层，需再定位；② 底层修复不完整，需交回 wr-engine 补修」。

### 3.6 交 metrics-audit（指标不诚实 / 门禁偷放）

**触发条件：** §2.4 定位层 = 指标层（字段缺/null 无原因/JSON 字段名不一致/门禁阈值偷放/slope_gate=off 滥用/降画质装绿）。

交 metrics-audit：

```
use_skill gpui-metrics-audit
告知 metrics-audit：
  - ABILITY_ID：<R id>
  - SUSPECT_FIELD：<若定向审某字段，如 measure_cache_hit / rss_slope_kb_per_min>
  - 任务：审该 R 的 JSON 指标诚实性（字段完备性 / 诚实性 / 阈值防偷放 / 观测一致性 / 降画质检测）
  - SOURCE_SKILL：wr-debug
```

metrics-audit 输出全 PASS（EARNED + HONEST + CONSISTENT + AT_DEFAULT + PRESENT）→ bug 已修，进第 4 步。

metrics-audit 输出任一 SUSPECT/FAKE/LOOSE/GATE_OFF/SCENE_CHEAT/TIME_CHEAT/IDLE_CHEAT → 指标层 bug 未修完，按 metrics-audit 的修复建议继续（类 A 指标层 bug 交 metrics-audit 修 JSON marshal；类 B 真窗代码层 bug 本 skill 第 3.2 步自修或交 wr-close 模式 2；类 C 引擎洞交 wr-engine）。

### 3.7 交 wr-close 模式 2（已关 ✅ 要升维）

**触发条件：** §2.4 定位层 = 「已关 ✅ 真窗代码层 bug」（U17 场景简陋 / HUD 不见 / 相位缺 / 实现点缺），该 R 当前 ✅。

交 wr-close 模式 2：

```
use_skill gpui-wr-close
告知 wr-close：
  - ABILITY_ID：<R id>
  - 模式：2（反攻重关）
  - SOURCE_SKILL：wr-debug
  - bug 摘要：<如「U17 场景简陋——只有两色块，不满足多区域+静态密集+动态热点」>
  - wr-close 模式 2 内部走：状态降级（✅ → 🔄 质量返工）→ 重写真窗（按 U17/U18/U20）→ 跑 GPU → 判门禁 → 回写 docs
```

### 3.8 交 wr-close 模式 3（已关 ✅ 要优化）

**触发条件：** §2.4 定位层 = 「已关 ✅ 性能 / 场景 / HUD / 指标 优化增量」，该 R 当前 ✅。

交 wr-close 模式 3：

```
use_skill gpui-wr-close
告知 wr-close：
  - ABILITY_ID：<R id>
  - 模式：3（优化重关）
  - SOURCE_SKILL：wr-debug
  - 优化目标：<如「降 draw call」「提 fps」「加场景」>
```

### 3.9 交 wr-implement（能力根本没实现）

**触发条件：** §2.4 定位层 = 「能力根本没实现」（如 R7 VirtualList 文件不存在 / 关键函数缺失）。

交 wr-implement：

```
use_skill gpui-wr-implement
告知 wr-implement：
  - ABILITY_ID：<R id>
  - SUSPECT_GAP：<能力缺口，如「VirtualList 文件不存在」「BindCount 没接 metrics.go」>
  - SOURCE_SKILL：wr-debug
  - wr-implement 内部走：能力现状审查 → 实现/补全 → 单测 → 指标接线 → 交 wr-close 模式 1 写真窗
```

## 第 4 步：回写 `docs/ENGINE_UI_WIDGET_RENDER.md` §10 修订表

**必做**——bug 修完后（无论本 skill 自修还是被回流 skill 修完交回），回写 docs。

### 4.1 §10 修订表加行

`read_file docs/ENGINE_UI_WIDGET_RENDER.md` 定位 §10 修订表最后一行，用 `edit_file` 加：

```
| <下一版本号> | R<id> bug 修复：<bug 摘要 + 修复层 + 修复函数/字段> |
```

**注：** §2 主表该 R 行的「状态」列**不**由本 skill 改——状态升维归被回流的 skill（wr-close 模式 2 升 ✅v2 / wr-engine 修底层后交原 skill 升维 / metrics-audit 修指标后交原 skill 升维）。

### 4.2 回写后验证

`read_file` 重新确认 edit 落点对、没破坏表格 markdown。

## 输出格式

完成后给用户一份结构化报告：

```
✅ R<id> bug 修复收口

ABILITY_ID：<R3 等>
BUG_DESC：<boundary 文不 skip 等>
原状态：<⬜ / ✅ / 🔄 / ✅v2 / ✅🔄>

复现：
  命令：RUN_SECONDS=<CLOSE_SECONDS> go run ./examples/<PACKAGE>
  bug 复现：是
  复现证据：<FAIL 行 / 异常字段值 / 现象截图描述>

定位层：<ui/rendering / ui/embedder / ui/scene / ui/io / render / gpu / 指标层 / 能力未实现>
定位文件：<file.go>
定位函数/字段：<func / field>
风险等级：<低 / 高 / 最高>

回流 skill：<本 skill 自修 / wr-engine / metrics-audit / wr-close 模式 2 / wr-close 模式 3 / wr-implement>
回流结果：
  本 skill 自修：单测 PASS + 真窗重跑 bug 已修
  wr-engine：底层修完 + 跑回归全绿 + 交回本 skill 重跑真窗 bug 已修
  metrics-audit：全 PASS（EARNED + HONEST + CONSISTENT + AT_DEFAULT + PRESENT）
  wr-close 模式 2：状态降级 ✅ → 🔄 → 重写真窗 → 跑 GPU → 判门禁全绿 → 状态升维 ✅v2
  wr-close 模式 3：优化生效 → ✅v2-optimized 或 ✅v2-extended
  wr-implement：能力实现 + 单测 + 指标接线 → 交 wr-close 模式 1 写真窗

§2.2 全族门禁重判：
  族 A 帧时：PASS  (fps_interval=XX, hitch=X/min, vsync=fallback)
  族 B 管线：PASS
  族 C 脏区：PASS  (能力专用字段 XX)
  族 D CPU ：PASS  (cpu_pct_avg=XX%)
  族 E 内存：PASS  (rss_slope=XX KB/min)
  族 F GPU ：PASS  (fallback=0)
  族 G 图/文：PASS / N/A
  族 H 启动：PASS / N/A
  族 J 正确：PASS  (depcheck 绿)

文档：docs/ENGINE_UI_WIDGET_RENDER.md §10 修订表加行「<版本号> | R<id> bug 修复：<摘要>」

下一步：<该 R 还要优化？还是关下一个 R？>
```

若有任一 FAIL 或停下报告：

```
⚠️ R<id> bug 修复未达标 / 需用户确认

当前阶段：<第 1 步复现 / 第 2 步定位层 / 第 3 步分层回流 / 第 4 步回写 docs>
FAIL 原因：<具体>

若第 1 步复现未复现 bug：
  本次复现未重现 bug——可能是间歇性 / 已被其他 skill 修复 / 现象描述与代码不符
  建议：① 加长 RUN_SECONDS 再跑；② 描述更具体的复现场景；③ 怀疑已被修，跑 git log 查最近改动

若第 3.3 步 render/ 停下 question 确认：
  bug 定位到 render/（高风险），需用户确认改不改
  选项 A：确认改 → 交 wr-engine
  选项 B：不改 render/，尝试在 ui/rendering 层绕过
  选项 C：停下不修

若第 3.4 步 gpu/ 停下 question 确认：
  bug 定位到 gpu/（最高风险），需用户确认改不改
  选项 A：确认改 → 交 wr-engine（强制停下报告 GPU 驱动兼容性 + wgpu 版本兼容性 + 替代方案 + 影响面）
  选项 B：不改 gpu/，尝试在 render 层或 ui/rendering 层绕过
  选项 C：停下不修

若第 3.5 步底层修完后真窗重跑 bug 仍在：
  可能：① bug 涉及多层，需再定位；② 底层修复不完整，需交回 wr-engine 补修
```

## 禁令自检（每次结束前过一遍）

### 通用禁令

- [ ] **第 1 步强制复现做了**（不跑真窗就凭描述改代码 = 绕洞）
- [ ] **第 2 步定位层对照 §2.2 FAIL 线 + §6 模块落点，没凭 BUG_DESC 直接猜层**
- [ ] **第 3.1 步回流决策树按 AGENTS.md 分层风险规则走**
- [ ] **render/ 改动停下 question 确认了**（AGENTS.md：render/ 必须 question 确认后修）
- [ ] **gpu/ 改动停下 question 确认了**（AGENTS.md：gpu/ 最高风险，必须 question 确认后修）
- [ ] **禁止在示例层绕引擎洞**（该改 ui/rendering 或 render，不改 examples/ui_wr_*/main.go 绕洞）
- [ ] **禁止降画质装绿**（不在 ui/rendering / render / gpu 层放宽门禁）
- [ ] **禁止 ui→gpu 依赖**（CODING_RULES §0.4）
- [ ] **禁止 CGO**（purego）
- [ ] **第 4 步回写 docs §10 修订表了**
- [ ] **状态升维不由本 skill 改**（§2 主表该 R 行「状态」列归被回流的 skill 改）

### 自修专属禁令

- [ ] **第 3.2 步 ui 低风险自修只在 ui/rendering / ui/embedder / ui/scene / ui/io 范围内改**
- [ ] **自修后重跑该 R 真窗验证 bug 已修**（不只靠单测绿就说修好）
- [ ] **改了 ui/embedder/pipeline_app.go 引发全回归时停下报告**

### 分层回流专属禁令

- [ ] **回流 wr-implement 仅在「能力根本没实现」时**（不是「能力实现缺陷」——后者本 skill 自修）
- [ ] **回流 wr-close 模式 2 仅在「已关 ✅ 真窗代码层 bug」时**（不是「底层洞」——后者交 wr-engine）
- [ ] **回流 wr-close 模式 3 仅在「已关 ✅ 优化增量」时**（不是「bug 修复」——bug 修复走模式 2 或本 skill 自修）
- [ ] **回流 wr-engine 仅在「底层洞（render/gpu）」时**（不是「指标不诚实」——后者交 metrics-audit）
- [ ] **回流 metrics-audit 仅在「指标层 bug」时**（不是「引擎洞」——后者交 wr-engine）

任一项未过 → 回对应步骤修，或停下报告用户。

---

## 附：与其他 5 个 skill 的接口

| 接口方向 | 内容 |
|----------|------|
| **入：用户直接触发** | 「修 R3 的 bug」「debug R7」「R12 闪屏 fix」「R4 渲染错」→ 第 0 步读真源 + 第 1 步强制复现 |
| **出：交 wr-engine（render/ 停下确认）** | 第 3.3 步，bug 定位到 render/ 高风险层，停下 question 确认后交 wr-engine |
| **出：交 wr-engine（gpu/ 停下确认）** | 第 3.4 步，bug 定位到 gpu/ 最高风险层，停下 question 确认后交 wr-engine |
| **出：交 wr-engine 后交回** | 第 3.5 步，wr-engine 修完底层 + 跑回归全绿，交回本 skill 重跑真窗验证 bug 已修 |
| **出：交 metrics-audit** | 第 3.6 步，bug 定位到指标层（不诚实 / 偷放），交 metrics-audit 审指标诚实性 |
| **出：交 wr-close 模式 2** | 第 3.7 步，bug 是「已关 ✅ 真窗代码层 bug」，交 wr-close 模式 2 反攻重关 |
| **出：交 wr-close 模式 3** | 第 3.8 步，bug 是「已关 ✅ 优化增量」，交 wr-close 模式 3 优化重关 |
| **出：交 wr-implement** | 第 3.9 步，bug 是「能力根本没实现」，交 wr-implement 实现能力 |
| **不接：W 矩阵级调度** | 推翻 W<n> 重写批量调度归 wr-rewrite（本 skill 只接「指定窗口的 bug 定位 + 分层回流」） |
| **不接：状态升维** | §2 主表该 R 行「状态」列升维归被回流的 skill 改（本 skill 只回写 §10 修订表） |

**关键边界：** 本 skill 只接「指定窗口 bug → 强制复现 → 定位层 → 分层回流」。5 个 skill 各管一段：实现能力（wr-implement）→ 关 R 生命周期（wr-close 3 模式）→ 审指标（metrics-audit）→ 修底层（wr-engine）→ W 矩阵级调度（wr-rewrite），全闭环。本 skill 是调度入口层 + ui 低风险自修层，不替代被回流的 skill 的内部闭环。
