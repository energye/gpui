---
name: gpui-wr-close
description: 关 R 完整生命周期——定标准（场景矩阵 + HUD + 实现点六维）→ 写代码 → 跑 GPU 取 JSON → 判 §2.2 全族门禁 → 回写 docs/ENGINE_UI_WIDGET_RENDER.md §2 状态列。3 种模式：模式 1 首次关闭（R 当前 ⬜）/ 模式 2 反攻重关（R 当前 ✅ 要升维，或 🔄 质量返工中）/ 模式 3 优化重关（R 当前 ✅ 要优化/增量）。当用户说「关闭 R7」「关掉 R10」「把 R3 收了」「反攻 R3」「重写 R4」「返工 R5」「优化 R4」「R7 加场景」「R4 提性能」时触发。FAIL 内置回流：第 3 步判门禁 FAIL → 自动回第 1 步重写（按定标准），不需另起 skill。
user_invocable: true
disable_model_invocation: false
---

# gpui-wr-close — 关 R 完整生命周期（3 模式）

> **与收敛后 5 skill 的分工：**
> - `wr-implement` = 能力实现回流（ui/ 里实现 + 单测 + 指标接线）——**本 skill 的前置**
> - **本 skill** = 关 R 完整生命周期（3 模式：首次关闭 / 反攻重关 / 优化重关）
> - `metrics-audit` = 指标族 A–J 正误审查与 bug 修复（指标层）——**被本 skill 串接**
> - `wr-engine` = 底层修复回流（render/gpu/ui-scene 谨慎改 + 跨层影响面评估）——**本 skill 发现底层不满足时调用**
> - `wr-rewrite` = W 矩阵级调度（推翻 W<n> 重写）——**本 skill 被 wr-rewrite 调度**
>
> **真源：** `docs/ENGINE_UI_WIDGET_RENDER.md`（v3.1）§0 U5/U7/U9/U12–U16 + §2.2 + §2.5 + §9 文档与状态治理 + §10 修订表；`docs/ENGINE_UI_RENDER_BASE.md` §20.2 M-\* 全表 + §25.4 架构诚实点 6 条。本 skill 是其执行骨架。若本 skill 与真源矛盾，以真源为准——发现矛盾停下报告，不要自决。

## 0. 为什么要这个 skill

历史 `ui_wr_*` 真窗形成了「门禁假绿」模式：场景 = 1–3 个 `RenderColorBox` + `sin` 变色，指标只在**结束时 stdout JSON**，人眼看不到 FPS/damage/skip，`damage_ratio<0.35` 在简陋场景下轻松通过却**不证明**复杂 UI 下正确。本 skill 把验收标准从「JSON 绿」升维到「复杂场景 + 窗内可见 + 实现点全面」。

**本 skill 收敛了原 3 个 skill 的职责：**

| 原 skill | 收敛到本 skill 的哪 |
|----------|----------------------|
| `wr-quality`（写前定标准） | 模式 1 第 0 步「定标准」+ 模式 2 第 2 步「按定标准重写」 |
| `wr-rework` 类 B（门禁 FAIL 重写真窗） | FAIL 内置回流：第 3 步判门禁 FAIL → 自动回第 1 步重写 |
| `wr-optimize`（已关 R 优化/增量） | 模式 3 优化重关 |

**禁止的关闭证据（任一出现即不得标 ✅）：**

- 纯色块双节点关闭主能力
- 只有 JSON 知道、人眼看不出对应字段
- 简陋场景下过门禁（如 `damage_ratio` 在「绿静+红热」两色块下 ≪0.35）
- 无 HUD 运行中可见指标
- 实现点清单缺项（见模式 1 第 0 步「实现点六维」）

---

## 输入

用户会给一个 R id（如 `R7`、`R10`、`R3b`），也可能给组合窗 C id（如 `C3`）。本 skill 同时支持 `R` 与 `C`。

从输入解析：

- `ABILITY_ID`：如 `R7`、`C3`、`R3b`（原样保留大小写）
- `PACKAGE`：对应包名，从主表查（若用户没给，从 §2 / §3 主表按 id 查；查不到就**停下问用户**，不要猜）
- `MODE`：根据该 R 当前状态自动选（见「模式选择」）

## 模式选择

`read_file docs/ENGINE_UI_WIDGET_RENDER.md` 定位 §2 主表该 R 行的「状态」列，按当前状态选模式：

| 当前状态 | 触发词示例 | 选哪个模式 |
|----------|------------|------------|
| `⬜`（未关） | 「关闭 R7」「关掉 R10」 | **模式 1 首次关闭** |
| `✅`（已关，要升维重写） | 「反攻 R3」「重写 R4」「返工 R5」 | **模式 2 反攻重关** |
| `✅`（已关，要优化/增量） | 「优化 R4」「R7 加场景」「R4 提性能」 | **模式 3 优化重关** |
| `🔄`（质量返工中） | 「继续反攻 R3」 | **模式 2 反攻重关**（继续重关） |

**模式冲突处理**：若用户说「优化 R4」但 R4 当前 ⬜ → 停下问用户「R4 还没关，要先首次关闭吗？」。若用户说「关闭 R7」但 R7 当前 ✅ → 停下问用户「R7 已关，要反攻重关（模式 2）还是优化重关（模式 3）？」。

---

## 第 0 步：读真源 + 定位包

**必做**——每次都先读，禁止凭记忆跑流程（真源会变）。

1. `read_file docs/ENGINE_UI_WIDGET_RENDER.md`，定位 §2 主表里该 id 行（`R<id>` 或 `C<id>`），取四列硬值：
   - `PACKAGE`（如 `ui_wr_r7_virtlist`）
   - `WINDOW` = **1200×800**（必须是这个；不是就停下，文档被改错了）
   - `CLOSE_SECONDS`（§2 主表「推荐 RUN_SECONDS」列，如 R7=60；若 §2.5 关闭用时长表与此不一致，以 §2.5 为准）
   - `WAVE`（波次，如 W3）
2. 若 `ABILITY_ID` 是组合窗（C*），读 §3 组合表 + §2.5「按组合窗关闭用时长」表，`CLOSE_SECONDS = max(所覆盖各 R 的关闭用)`。
3. 读 §2.2.1 指标族 × 真窗义务表 + §2.2.2/§2.2.3/§2.2.4 的 FAIL 线——**这些是判定基准**，每次都要对照，禁止凭记忆判门禁。
4. 读 `docs/ENGINE_UI_RENDER_BASE.md` §20.2 M-\* 全表补字段语义（若该 R 有能力专用字段，如 R3 的 `boundary_skip`、R7 的 `bind_count`）。
5. 读 §9 文档与状态治理 + §10 修订表——状态转换的硬规则。

**若任何一份文档与代码现状矛盾**（如主表说 R 状态 ⬜ 但 `examples/ui_wr_<id>/` 已有 main.go），**停下向用户报告矛盾**，不要自动决定信哪边。

---

## 第 1 步：定标准（U17 场景复杂度 + U18 HUD 可见 + U20 实现点六维）

**必做**——关 R 前的质量基准，禁止只看 JSON 绿就标 ✅。

### 1.1 U17 复杂场景（每个主能力窗硬要求）

每个 `ui_wr_r*` / `ui_wr_c*` 必须**同时**具备五项，缺一即质量不达标：

| # | 要求 | 落地 |
|---|------|------|
| 1 | **多区域布局** | ≥ 壳层（TopBar 标题/相位/政策 + 主内容 + 侧栏/图例 + 底栏 HUD），或等价复杂度；禁止「整屏一坨」 |
| 2 | **静态密集内容** | ≥ 8 个文字标签 或 4×4 色格 + 嵌套 boundary；**不能只靠纯色块**（纯色块阵 = 假绿） |
| 3 | **动态热点** | ≥1 处持续动画/交互，与静态同屏；人眼能区分「静/动」 |
| 4 | **能力专属压力** | 见 §1.4 场景矩阵该 R 行；**禁止**用他能力场景冒充（如 R3 不许只测色块 skip，要测嵌套 + 文） |
| 5 | **相位脚本** | ≥2 相：稳态（Steady）/ 突变（Spike）/ 可选恢复（Recover）；人眼与门禁都能区分相位切换 |

### 1.2 U18 窗内可见指标（运行中硬要求 · LiveHUD）

**禁止**只在结束时 stdout JSON。运行中窗内必须能人眼读出：

| 必须在窗内可见 | 实现 |
|----------------|------|
| 能力 ID + 当前相位 | TopBar 标题条 |
| 实时 FPS / interval p95 | 滚动数字（每帧或每 N 帧刷新） |
| 本能力核心计数器 | 如 `boundary_skip` / `damage_ratio` / `dirty_layer_ids` / `bind_count` / `paint_count` |
| PASS/FAIL 预览色 | 绿=当前达门禁趋势，红=已破线（结束仍以 JSON 为准，HUD 只是人眼预警） |
| 图例 | 静态区 / 脏区 / boundary 边界用色或描边约定 |

**HUD 实现纪律（防止 HUD 污门禁）：**

- HUD 走 `examples/wrkit` 共享 `LiveHUD`（Overlay 顶层 或 root 上**非 Boundary** 的 HUD 节点，自脏每帧）
- 数据源：`app.Metrics().Snapshot()` / `LastBoundaryFrame()` / `DamageStats` / `LastPresentMode` / `LastDirtyLayerIDs` / 场景自报 `ability_extra`——**不新增 ui→gpu 依赖**
- 绘制：`DrawString` + 半透明底
- **关键：HUD 自身 dirty 不污染主能力门禁**——二选一：
  - (a) 门禁统计排除 HUD 层（content-only damage）
  - (b) HUD 独立 band，damage 只计 content band
- 默认每窗开 HUD；`WR_HUD=0` 可关（CI 仍可开）

### 1.3 U20 实现点清单（每 R 关 R 前自检六维）

每个 R 在 `docs/ENGINE_UI_WIDGET_RENDER.md` §2 主表行须有**「实现点」子表**（或在 README 里），覆盖六项：

| 维度 | 要回答 | 示例（R3） |
|------|--------|------------|
| **正确性** | 能力语义对了吗 | 嵌套 RB 的子树脏不重录父 Picture |
| **脏区** | 成本 ∝ 脏吗 | skip 累加、rerecord 仅脏、damage 不含静区 |
| **缓存** | 命中/失效正确吗 | 文/嵌套静区能 skip；size 变 invalidate 后一波重录再 skip |
| **边界条件** | 极端不炸吗 | 深嵌套、空子树、同帧多次 invalidate |
| **失败模式** | 挂了怎么表现 | boundary 数=0、cache miss 全路径、超预算 reject |
| **窗内如何看出** | HUD/图例对应字段 | skip 数字、rerecord debug 描边、静区色稳定 |

**缺任一项 → 不得标 ✅**（即使 JSON 门禁绿）。

### 1.4 场景矩阵（每 R 关 R 前必达）

> 共同：从「两色块」→「迷你应用壳」。每窗至少：TopBar + 左图例 + Body 静态 dense（≥8 标签或 4×4 色格 + 嵌套 boundary）+ Body 动态（能力专属）+ 底栏 HUD。

| ID | 当前简陋点 | 复杂场景要求 | 窗内必见 | 引擎若挂必修 |
|----|------------|--------------|----------|--------------|
| **R0** | 两色块 | 静网 + 动块 + 文；Clear 后静不丢 | policy、静格存在 | FullPaint 路径 |
| **R2** | 一静一热 | 9 宫静 + 1 热；邻格不变色 | paint_count、仅热 debug 闪 | 局部 NeedsPaint |
| **R3** | 两 RB | 嵌套 3 层 RB + 静文 + 热；skip 累加可见 | skip↑ rerecord 仅热 | BoundaryCache 文/嵌套 |
| **R3b** | 计数 | 深嵌套 + 旁路 boundary；depth 可见 | boundary_count/depth | compositing bits |
| **R4** | 绿+红 | **retained 壳**：顶栏静、体热、侧栏静；damage≪1 | policy、dmg%、mode | CompositeOnly+LoadOpLoad |
| **R4b** | 两角色块 | 四角热 + 中心 dense 静 + 双远端同帧脏 | dirty_ids≥2、multi 次数 | multi damage |
| **R5** | 直绘/回放两区 | 同构图左右对照 + 多 op（path/文/图） | op_count、两侧一致 | Picture 完备 |
| **R9** | 单文本 | 多样式重复 measure + 热改一字 | hit/miss 数字 | measure cache |
| **R11** | 单 box 改 size | 周期改 client/逻辑 scale；文+线+网 | 一波 rerecord 再 skip | InvalidateBoundaryCache |
| **R12b** | 两色块 | debug 开：多节点，人眼见谁闪 | debug 描边计数 | DebugRepaint |
| **R13** | 脚本点 4 点 | 变换/重叠/裁剪下可点；高亮+HUD 命中名 | hit id 大字 | Hit≡Paint |
| **R18** | 预算 allow/reject | 组半透明 + 超预算拒绝可观测 | allow/reject 计数 | SaveLayer 预算 |
| **C1/C2/C7** | 略好但仍色块 | 组合：嵌套+retained+resize 同壳 | 各 R 字段同屏 | 集成回归 |

**R1 / R19 / R21 / R16 完整窗**：若本次返工顺带，按同标准；否则明确 ⬜ 不占 ✅。

### 1.5 高概率引擎洞表（复杂场景后会炸）

若第 2 步跑 GPU 时引擎在复杂场景下炸，对照此表定点修（实际修复交 `wr-engine`）：

| 高概率洞（复杂场景后会炸） | 修哪 |
|----------------------------|------|
| BoundaryCache 仅 Color/Absolute MVP，文/自定义 OnPaint 静区无法 skip | `ui/rendering/boundary_cache.go` 扩 record 类型或强制静区走可录路径 |
| 壳在 retained 下闪/残；force/clear 与 CompositeOnly 交界；HUD damage 污染 | `ui/embedder/pipeline_app.go` |
| DirtyLayerIDs 每帧重建，R4b id 不稳 | 稳定 cacheID 绑定 |
| 无像素采样门禁，纯 ratio 假绿 | 可选 `SAMPLE_PX` 探针（逻辑坐标读回或约定截图像素） |

**禁止为过门禁删场景降复杂度。** 引擎挂了报告洞 + 定点修（交 wr-engine），不许删场景保门禁。

**不做（本波）：** 全量 multi-RT 层纹理 compositor、默认全局 retained（仍属 W6）。

---

## 第 2 步：写代码（按第 1 步定标准）

### 2.1 模式 1 首次关闭：建真窗代码

> 若目录已存在且含 main.go，跳到第 3 步跑真窗。空目录（0 文件）视为未建，继续本步。

#### 2.1.1 `main.go` 硬骨架（对照已通过的 R0/R4 main.go）

必须坐实的硬约束（来自 §0 U15/U16 + §2.2.5）：

| 约束 | 实现 |
|------|------|
| **窗口 1200×800** | `const winW, winH = 1200, 800`；JSON `"client_px": "1200x800"` |
| **RUN_SECONDS≥5** | `runSeconds(<CLOSE_SECONDS>)` 默认取关闭用值；`RUN_SECONDS` 环境变量覆盖；`<5` → `fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close <ABILITY_ID> (U16)")` + `os.Exit(1)` |
| **GPU 真窗** | 经 `examples/exhost` 开 X11 真窗，`export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so`；无 GPU 环境 → `FAIL: window open (needs_gpu_window)` + exit 1（禁止 CPU stub 关 R） |
| **stderr + JSON** | 结束时输出一族 JSON（§2.2.5 最小外壳）到 stdout/stderr；不达标字段写 `null`/`0` + `unavailable` 原因，**禁止默默省略** |
| **不达标 FAIL** | 各门禁检查失败 → `fmt.Fprintf(os.Stderr, "FAIL: <原因>")` + `os.Exit(1)` |
| **可见效果** | 在窗里画**能肉眼看到**该 R 能力的内容（如 R7 虚拟列表：视口内 cell + 快滑；R3 boundary：静块始终在 + 脏块每帧变） |

#### 2.1.2 §2.2 全族 A–J 必采字段（写入 JSON）

从 `ui/scheduler/metrics.go` 的 `FrameMetrics` struct 取字段名（禁止自创字段名）。至少：

- **A 帧时**：`fps_wall`、`interval_avg_ms`、`interval_p50_ms`、`interval_p95_ms`、`interval_p99_ms`、`hitch_count`、`hitch_rate_per_min`、`vsync_source`、`target_hz`
- **B 管线**：`frame_build_ms`、`frame_raster_ms`、`pipeline_depth`、`pipeline_max`
- **C 脏区**：`layout_count`、`paint_count`、`damage_area_px`、`damage_ratio`、`present_mode`、`present_policy` + **能力专用**（R3 `boundary_skip`/`boundary_rerecord`、R7 `bind_count`、R4b `dirty_layer_ids` 等）
- **D CPU**：`cpu_pct_avg`、`cpu_ui_pct`、`cpu_raster_pct`
- **E 内存**：`rss_start_kb`、`rss_end_kb`、`rss_peak_kb`、`rss_slope_kb_per_min`、`rss_after_close_kb`（能采则采）
- **F GPU**：`gpu_ops`、`cpu_fallback_ops`、`last_cpu_fallback`、`frame_flushes`
- **G 图/文**：R9/R10 强制 `measure_cache_hit`；其它窗 `g_metrics=skipped` + 原因
- **H 启动**：`warmup`、`time_to_first_present_ms`（有则采；R0/R16/C0 首帧有内容硬）
- **I 回归**：支持 `BASELINE_JSON` / `SAVE_BASELINE_JSON`（可选开，发布/合入关键窗建议开）
- **J 正确性**：构建期 `ui` 无 import gpu、无 cgo、本窗不降画质（本步靠 hook/depcheck，JSON 里标 `"depcheck": "passed"`）

族字段在 JSON 里**全部出现**，无数据写 `null` 或 `0` + `"unavailable_reason"`，禁止默默省略。

#### 2.1.3 `README.md`

必含四段：

1. **Run**：`RUN_SECONDS=<CLOSE_SECONDS> go run ./examples/<package>`（+ LD_LIBRARY_PATH/WGPU_NATIVE_PATH 那两行 export）
2. **Window / Close duration**：`1200×800` + `<CLOSE_SECONDS>s` + `RUN_SECONDS<5 → FAIL`
3. **Visible effect**：表格列「Region / Expectation」，写人眼能见的预期（如「绿块 @ (48,48) 静止不动」「红块 @ (700,280) 每帧变色」）
4. **Gates**：逐条列该 R 的 §2 主表门禁 + §2.2 全族 FAIL 线（如 R3：`boundary_skip≥1`、`boundary_rerecord 仅脏`、`present_count≥1`、`fps_interval≥55`（持续 tick 时））

### 2.2 模式 2 反攻重关：状态降级 + 重写真窗

#### 2.2.1 状态降级（WIDGET_RENDER §9）

1. `read_file docs/ENGINE_UI_WIDGET_RENDER.md` 定位 §2 主表该 R 行的「状态」列
2. 用 `edit_file` 把 `✅` 改成 `🔄 质量返工`（保留「基础门禁曾绿」备注，但**不得宣称关闭**）
3. §4/§4b/§4c 关闭清单（若该 R 在列）：改为「基础门禁曾通 / 质量条重写中」
4. §10 修订表加一行：`<版本号> | R<id> 质量返工：<FAIL 原因摘要>`

若该 R 当前 ⬜（未关），跳过状态降级，直接走第 2.2.2 步重写（这是「重写未关 R」场景）。

若该 R 当前 🔄（已在质量返工中），继续本步重写，不重复降级。

#### 2.2.2 重写真窗代码（按第 1 步定标准）

`edit_file` / `write_file` 重写 `examples/ui_wr_<package>/main.go` + `README.md`，按第 1 步定的 U17 场景复杂度 + U18 HUD 可见 + U20 实现点六维重写。

**重写纪律：**

- **禁止删场景保门禁**（违反 U17）
- **禁止降 README 阈值过门禁**（门禁是硬的不许放）
- **禁止在示例里绕引擎洞**（该走第 5 步交 wr-engine 定点修 ui/）
- 重写完 → 交第 3 步跑真窗

### 2.3 模式 3 优化重关：状态标注 + 改代码

#### 2.3.1 前置校验（硬）

| 校验项 | 不通过的处理 |
|--------|--------------|
| 该 R 当前状态必须 `✅`（已关） | 若 `⬜` 未关 → 交模式 1 走首次关闭；若 `🔄` 质量返工中 → 交模式 2 走反攻；**不**走模式 3 |
| 优化/增量目标必须在 RENDER_BASE §22.2/§25.5 范围内 | 若超出（如要加 PlatformView）→ 停下问用户，这是远期 D 行，不属本波 |
| 该 R 的 §2 主表门禁不能因优化放宽 | 若优化方案需降门禁阈值 → 停下报告，门禁是硬的不许放 |

#### 2.3.2 状态标注（WIDGET_RENDER §9）

`edit_file docs/ENGINE_UI_WIDGET_RENDER.md`：

1. §2 主表该 R 行「状态」列：`✅` → `✅🔄 优化中` 或 `✅🔄 增量中`（**保留 ✅**，因为基础门禁曾绿；**加 🔄** 标注当前在优化/增量回流中）
2. §10 修订表加占位行：`<版本号待定> | R<id> 优化/增量：<目标摘要>`

#### 2.3.3 改代码

| 优化目标 | 改哪 | 改什么 |
|----------|------|--------|
| 降 draw call | `examples/ui_wr_<package>/main.go` | 合批同 shader、用 Picture 录回放避免重录、静态走 BoundaryCache |
| 提 fps | `examples/ui_wr_<package>/main.go` | 减装饰、静态走 BoundaryCache、避免每帧 layout |
| 清冗余 | `examples/ui_wr_<package>/main.go` | 删死代码、未用 import、未用变量 |
| 降内存 | `examples/ui_wr_<package>/main.go` | 复用 buffer、提前 dispose、避免大 Picture 常驻 |
| 加场景 | `examples/ui_wr_<package>/main.go` | 加多区域布局 / 静态密集 / 动态热点 / 能力专属压力（按 §1.1 U17） |
| 加 HUD | `examples/ui_wr_<package>/main.go` | 加 wrkit LiveHUD 的字段（按 §1.2 U18） |
| 加指标 | `examples/ui_wr_<package>/main.go` | JSON 输出加该指标字段（字段名取自 `ui/scheduler/metrics.go` 的 `FrameMetrics` struct） |
| 加复杂度 | `examples/ui_wr_<package>/main.go` | 加嵌套深度 / 子树数量 / 动画节点数 |
| 加相位 | `examples/ui_wr_<package>/main.go` | 加 Steady/Spike/Recover 相位切换（按 §1.1 U17 第 5 项） |
| 加实现点维度 | `examples/ui_wr_<package>/README.md` | 补六维清单缺项（正确性/脏区/缓存/边界条件/失败模式/窗内如何看出） |

#### 2.3.4 同步改 README

`examples/ui_wr_<package>/README.md` 同步加：

- **Visible effect** 表加新场景/新 HUD 的预期行
- **Gates** 表加新指标的 §2.2 全族 FAIL 线（**阈值不偷放**）

#### 2.3.5 改完 → 交第 3 步跑真窗

---

## 第 3 步：跑真窗取 JSON

**禁止跳过**——这是 U5/U7 的核心。

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=<CLOSE_SECONDS> go run ./examples/<package>
```

- 若环境无 GPU/X11：**停下**，告诉用户「需要 GPU 真窗环境（needs_gpu_window）」，禁止用 CPU 单测或 stub 关 R。
- 跑完取 stderr + JSON。若 JSON 缺族字段，回到第 2 步补 main.go。
- 若 `exit 1` 且 `FAIL:` 原因是门禁不达标，**不要**自动改门禁值——报告给用户，门禁是硬的不许放。

**跑真窗时发现底层不满足？**（如 BoundaryCache 文不 skip、retained 下壳闪、render Picture 不支持回放、gpu/swapchain.go 的 LoadOpLoad 不对）：

→ 交 `wr-engine`（底层修复回流）

```
use_skill gpui-wr-engine
告知 wr-engine：
  - HOLE_DESC：<底层洞描述，如「BoundaryCache 文/嵌套静区不 skip」>
  - SUSPECT_LAYER：<ui/rendering / ui/embedder / render / gpu 等>
  - SUSPECT_FILE：<具体文件>
  - SOURCE_SKILL：wr-close
```

wr-engine 修完底层 + 跑回归全绿，交回本 skill 第 3 步重新跑真窗。

---

## 第 4 步：判门禁（逐族 FAIL 线）

对照 §2.2.2 / §2.2.3 / §2.2.4 + 该 R §2 主表「指标门禁」列，逐条判：

- **族 A**：若该窗是动画/滚动/持续 tick 类（R6/R7/R7b/C3/C5 等），`fps_interval≥55` 或 `interval_p95_ms≤22`；长 soak 另查 `hitch_rate_per_min` 超 README 预算；**必须**有 `vsync_source`（`fallback` 禁止宣称锁 60Hz）
- **族 B**：`pipeline_depth` 持续 > 配置上限 → FAIL
- **族 C**：该 R 专用门禁（如 R3 `boundary_skip>0`、R7 `bind_count≪item_count`、R4 `damage_ratio`≪全屏）；FullPaint 下 `damage_ratio` 可接近 1 但**不得**用其冒充 Retained
- **族 D**：`cpu_pct_avg`；长窗/soak（≥60s）`>85%` 且持续 → FAIL；`cpu_ui_pct`/`cpu_raster_pct` 必采（动画窗禁双 0 却靠降画质过 fps 门禁）
- **族 E**：`rss_start/end/peak_kb` 必采（Linux）；soak/压力窗 `rss_slope_kb_per_min` 超预算（默认 `>30000` 极端，场景可收紧）→ FAIL
- **族 F**：热路径 `cpu_fallback_ops` 无故暴涨 → FAIL（阈值写 README）
- **族 H**：R0/R16/C0 首帧有内容；`time_to_first_present_ms` 超预算 → FAIL
- **族 J**：`go test ./ui -run TestNoGPUImport` 绿 + grep `import "C"` 无命中 + 本窗没降画质

**每条判**记录：`PASS` / `FAIL（原因）` / `UNAVAILABLE（原因）`。

### 4.1 同时判 U17/U18/U20（质量基准）

本 skill 第 1 步定的 U17（场景复杂度）/ U18（HUD 可见）/ U20（实现点清单）也要同时判：

- JSON 族门禁绿但 U17/U18/U20 任一未过 → 综合 FAIL，**不得**标 ✅
- 此时回第 2 步重写（按第 1 步定标准），不许降标准

### 4.2 FAIL 内置回流（核心）

任一 FAIL → 不许标该 R 为 ✅，回到第 2 步修代码（按第 1 步定标准重写）。

**这是原 `wr-rework` 类 B 的职责**：门禁 FAIL 后，按第 1 步的 U17/U18/U20 标准重写 `examples/ui_wr_*/main.go`，**禁止**：

- 删场景降复杂度保门禁
- 降 README 阈值过门禁
- 在示例里绕引擎洞（该走第 3 步交 wr-engine）

重写完 → 回第 3 步重新跑真窗 → 再判门禁。循环直到全绿或停下报告。

### 4.3 串接 metrics-audit 审指标诚实性

判门禁时，**同时**调度 metrics-audit 审指标诚实性（字段完备性 / 诚实性 / 阈值防偷放 / 观测一致性 / 降画质检测）：

```
use_skill gpui-metrics-audit
告知 metrics-audit：
  - ABILITY_ID：<R id>
  - SUSPECT_FIELD：<若定向审某字段>
  - 任务：审该 R 的 JSON 指标族诚实性
```

metrics-audit 输出全 PASS（EARNED + HONEST + CONSISTENT + AT_DEFAULT + PRESENT）→ 真绿，进第 5 步回写 docs。

metrics-audit 输出任一 SUSPECT/FAKE/LOOSE/GATE_OFF/SCENE_CHEAT/TIME_CHEAT/IDLE_CHEAT → 综合 FAIL：

- **类 A 指标层 bug**（字段缺/null 无原因/JSON 字段名不一致）→ 交 metrics-audit 第 1/4 步修 JSON marshal
- **类 B 真窗代码层 bug**（场景简陋/HUD 不见/相位缺/实现点缺/阈值偷放）→ 本 skill 第 2 步重写
- **类 C 引擎洞**（复杂场景下引擎炸）→ 交 wr-engine 定点修

---

## 第 5 步：若该 R 有组合窗，跑组合窗

查 §3 组合表，若该 R 被某组合窗覆盖（如 R7 被 C3 覆盖）：

- 组合窗**不能**代替单能力窗关 R（U5），但**也要绿**（U6 + U9）
- 组合窗同样 1200×800 + `RUN_SECONDS≥max(覆盖各 R)` + §2.2 全族
- 组合窗只做集成回归，不重判单能力门禁

若组合窗目录为空（如 C3 在 W3 未建），**停下问用户**：是现在建组合窗，还是该 R 不带组合窗收口。

---

## 第 6 步：回写 `docs/ENGINE_UI_WIDGET_RENDER.md` 状态列

**必做**——U9 的硬要求，禁止代码绿但文档仍 ⬜。

### 6.1 按模式选状态转换

| 模式 | 原状态 | 升维后 | §10 修订表 |
|------|--------|--------|------------|
| 模式 1 首次关闭 | `⬜` | `✅` 或 `✅ GPU PASS` | `<版本> \| W<n> ✅ <R id> 真窗 + C<组合>` |
| 模式 2 反攻重关 | `✅`（曾关，质量返工中 🔄） | `✅v2（质量条）` | `<版本> \| R<id> 质量升维 ✅v2：<反攻摘要>` |
| 模式 3 优化重关 | `✅`（优化/增量中 ✅🔄） | `✅v2-optimized` 或 `✅v2-extended` | `<版本> \| R<id> 优化生效：<优化前 → 优化后 指标对比>` 或 `<版本> \| R<id> 增量生效：<加了什么 + 新指标值>` |

### 6.2 edit_file 回写

1. `read_file docs/ENGINE_UI_WIDGET_RENDER.md` 定位 §2 主表该 R 行的「状态」列（最右列）
2. 用 `edit_file` 改状态（按 §6.1 表）
3. 若该 R 使某波次 W 关闭清单（§4/§4b/§4c）需要新增条目，同步加
4. 若该 R 是某波次最后一个，更新 §5 分期表该 W 的状态（`⬜` → `✅`）+ §11 一句话末尾的「下一步」表述
5. §10 修订表加一行新版本说明（按 §6.1 表）

**回写后**重新 `read_file` 确认 edit 落点对、没破坏表格 markdown。

---

## 第 7 步：单测（回归 · 不单独关 R）

U8：CPU/`NewContext` 单测可作回归，**不能单独**将任一 §R 标完成。但若该 R 有对应单测（如 R3 的 `boundary_cache_test.go`），跑一遍 `go test ./ui/rendering -run TestBoundaryCache -count=1` 确认绿。

---

## 输出格式

完成后给用户一份结构化报告：

```
✅ R<id> <模式名>收口完成

模式：<模式 1 首次关闭 / 模式 2 反攻重关 / 模式 3 优化重关>
真窗：examples/<package>/   （1200×800 · <CLOSE_SECONDS>s · GPU PASS）
组合窗：examples/<c_package>/   （若涉及）

门禁：
  族 A 帧时：PASS  (fps_interval=XX, hitch=X/min, vsync=fallback)
  族 B 管线：PASS
  族 C 脏区：PASS  (能力专用字段 XX)
  族 D CPU ：PASS  (cpu_pct_avg=XX%)
  族 E 内存：PASS  (rss_slope=XX KB/min)
  族 F GPU ：PASS  (fallback=0)
  族 H 启动：PASS  (首帧 XX ms)
  族 J 正确：PASS  (depcheck 绿)

质量基准（U17/U18/U20）：
  U17 场景复杂度：PASS（多区域 / 静态密集 / 动态热点 / 能力专属 / 相位脚本）
  U18 HUD 可见：PASS（能力 ID + 相位 + FPS + 核心计数器 + 预览色 + 图例）
  U20 实现点六维：PASS（正确性 / 脏区 / 缓存 / 边界条件 / 失败模式 / 窗内如何看出）

metrics-audit 串审：PASS（EARNED + HONEST + CONSISTENT + AT_DEFAULT + PRESENT）

文档：docs/ENGINE_UI_WIDGET_RENDER.md §2 R<id> 行 <原状态> → <升维后>
单测：go test ./ui/rendering -run TestXX  PASS

下一步：W<n+1> 剩 R<下个 id> / R<下个 id>
```

若有任一 FAIL：

```
⚠️ R<id> <模式名>未达标

FAIL 原因：<族><具体> 或 <U17/U18/U20 未过> 或 <metrics-audit SUSPECT/FAKE>
分类：
  - 类 A 指标层 bug → 交 metrics-audit 修 JSON marshal
  - 类 B 真窗代码层 bug → 本 skill 第 2 步重写
  - 类 C 引擎洞 → 交 wr-engine 定点修
下一步建议：<修代码还是改门禁（门禁是硬的不许改）>
```

---

## 禁令自检（每次结束前过一遍）

### 通用禁令（3 模式共享）

- [ ] 没用 CPU stub 或单测单独关 R（U8）
- [ ] 没用组合窗代替单能力窗关 R（U5）
- [ ] 窗口是 1200×800 不是更小（U15）
- [ ] RUN_SECONDS ≥ 推荐关闭用值，至少 ≥5（U16）
- [ ] JSON 含全族 A–J 字段，无默默省略（U12）
- [ ] 动画/滚动窗 `fps_interval≥55` 或 `interval_p95≤22`（U13）
- [ ] 输出了 `vsync_source`，fallback 没宣称锁 60Hz（U13）
- [ ] `cpu_pct_avg` / `rss_*` 等硬观测采到了（U14）
- [ ] README 写了可见效果 + 门禁（U7）
- [ ] 文档 §2 状态列回写了（U9）
- [ ] 没降画质装绿（如把 fps 门禁放宽过 55、把 slope 阈值抬高过默认）
- [ ] 没用 CompositeOnly+GPU Clear 装省绘（U11）

### 模式 1 首次关闭专属

- [ ] 第 1 步定了 U17/U18/U20 标准后才写代码
- [ ] 第 4.1 步同时判了 U17/U18/U20，不只看 JSON 绿

### 模式 2 反攻重关专属

- [ ] 状态降级做了（✅ → 🔄 质量返工）
- [ ] 重写没删场景保门禁
- [ ] 重写没降 README 阈值
- [ ] 引擎洞交了 wr-engine，没在示例里绕

### 模式 3 优化重关专属

- [ ] 前置校验做了（该 R 当前 ✅ 才走模式 3）
- [ ] 优化/增量目标在 RENDER_BASE §22.2/§25.5 范围内
- [ ] 没降门禁阈值过优化
- [ ] 优化只改 examples/ui_wr_*/main.go 实现细节，不触及引擎架构（如 multi-RT）
- [ ] 状态标注做了（✅ → ✅🔄 优化中 / 增量中）

任一项未过 → 回对应步骤修。

---

## 附：与其他 4 个 skill 的接口

| 接口方向 | 内容 |
|----------|------|
| **入：wr-implement 转交** | wr-implement 第 5 步能力就绪后，交本 skill 模式 1 写真窗 |
| **入：wr-rewrite 调度** | wr-rewrite 第 2 步逐 R 回流时，调度本 skill 模式 1/2/3 |
| **入：用户直接触发** | 「关闭 R7」「反攻 R3」「优化 R4」→ 按该 R 当前状态自动选模式 |
| **出：交 wr-engine 修底层** | 第 3 步跑真窗发现底层不满足 → 交 wr-engine 修底层 |
| **出：交 metrics-audit 审指标** | 第 4.3 步判门禁时，串接 metrics-audit 审指标诚实性 |
| **不接：能力实现** | 能力实现（ui/rendering/）归 wr-implement |
| **不接：W 矩阵级调度** | W 矩阵级调度归 wr-rewrite（本 skill 是被调度者，不是调度者） |

**关键边界：** 本 skill 只接「关 R 完整生命周期（3 模式）」。5 个 skill 各管一段：实现能力（wr-implement）→ 关 R 生命周期（wr-close 3 模式）→ 审指标（metrics-audit）→ 修底层（wr-engine）→ W 矩阵级调度（wr-rewrite），全闭环。
