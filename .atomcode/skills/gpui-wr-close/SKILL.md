---
name: gpui-wr-close
description: 关闭一个 §R 主能力真窗的端到端流程——建真窗代码、跑 GPU 真窗取 JSON、判 §2.2 全族门禁、回写 docs/ENGINE_UI_WIDGET_RENDER.md §2 状态列。当用户说「关闭 R7」「关掉 R10」「把 R3 收了」「wr-close R4」「关这个主能力」时触发。
user_invocable: true
disable_model_invocation: false
---

# gpui-wr-close — 关闭一个 §R 主能力真窗

> 本 skill 是 `docs/ENGINE_UI_WIDGET_RENDER.md`（v3.1）§0 U5/U7/U9/U12–U16 + §2.2 + §2.5 的**执行骨架**。每次关闭一个 R 都要把这 5 件事做全，禁止只做代码就标 ✅。
>
> **⚠ 串接纪律（硬）：** 本 skill 是**执行流程**（建窗/跑/判门禁/回写），不管**开发质量标准**（复杂场景 + 窗内 HUD + 实现点清单）。
> - 写/改任一 `examples/ui_wr_*` 代码**之前**，必须先加载 `gpui-wr-quality`，按其场景矩阵 + 实现点六维设计场景与 HUD
> - 本 skill 的第 3 步判门禁时，**同时**要判 wr-quality 的 U17（场景复杂度）/ U18（HUD 可见）/ U20（实现点清单）——JSON 族门禁绿但 U17/U18/U20 任一未过 → 综合 FAIL，**不得**标 ✅
> - 顺序：先 `wr-quality`（定场景与 HUD 设计）→ 写代码 → 再本 skill（执行 + 判门禁 + 回写）
> - 若该 R 是因质量升维返工：§5 回写时状态 `✅ → 🔄 质量返工`，升维绿后 `🔄 → ✅v2（质量条）`

## 输入

用户会给一个 R id（如 `R7`、`R10`、`R3b`），也可能给组合窗 C id（如 `C3`）。本 skill 同时支持 `R` 与 `C`。

从输入里解析出：
- `ABILITY_ID`：如 `R7`、`C3`、`R3b`（原样保留大小写）
- `PACKAGE`：对应包名，从主表查（若用户没给，从 §2 / §3 主表按 id 查；查不到就**停下问用户**，不要猜）

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

**若任何一份文档与代码现状矛盾**（如主表说 R 状态 ⬜ 但 `examples/ui_wr_<id>/` 已有 main.go），**停下向用户报告矛盾**，不要自动决定信哪边。

## 第 1 步：建真窗代码（`examples/ui_wr_<package>/`）

> 若目录已存在且含 main.go，跳到第 2 步。空目录（0 文件）视为未建，继续本步。

### 1.1 `main.go` 硬骨架（对照已通过的 R0/R4 main.go）

必须坐实的硬约束（来自 §0 U15/U16 + §2.2.5）：

| 约束 | 实现 |
|------|------|
| **窗口 1200×800** | `const winW, winH = 1200, 800`；JSON `"client_px": "1200x800"` |
| **RUN_SECONDS≥5** | `runSeconds(<CLOSE_SECONDS>)` 默认取关闭用值；`RUN_SECONDS` 环境变量覆盖；`<5` → `fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close <ABILITY_ID> (U16)")` + `os.Exit(1)` |
| **GPU 真窗** | 经 `examples/exhost` 开 X11 �窗，`export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so`；无 GPU 环境 → `FAIL: window open (needs_gpu_window)` + exit 1（禁止 CPU stub 关 R） |
| **stderr + JSON** | 结束时输出一族 JSON（§2.2.5 最小外壳）到 stdout/stderr；不达标字段写 `null`/`0` + `unavailable` 原因，**禁止默默省略** |
| **不达标 FAIL** | 各门禁检查失败 → `fmt.Fprintf(os.Stderr, "FAIL: <原因>")` + `os.Exit(1)` |
| **可见效果** | 在窗里画**能肉眼看到**该 R 能力的内容（如 R7 虚拟列表：视口内 cell + 快滑；R3 boundary：静块始终在 + 脏块每帧变） |

### 1.2 §2.2 全族 A–J 必采字段（写入 JSON）

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

### 1.3 `README.md`

必含四段：
1. **Run**：`RUN_SECONDS=<CLOSE_SECONDS> go run ./examples/<package>`（+ LD_LIBRARY_PATH/WGPU_NATIVE_PATH 那两行 export）
2. **Window / Close duration**：`1200×800` + `<CLOSE_SECONDS>s` + `RUN_SECONDS<5 → FAIL`
3. **Visible effect**：表格列「Region / Expectation」，写人眼能见的预期（如「绿块 @ (48,48) 静止不动」「红块 @ (700,280) 每帧变色」）
4. **Gates**：逐条列该 R 的 §2 主表门禁 + §2.2 全族 FAIL 线（如 R3：`boundary_skip≥1`、`boundary_rerecord 仅脏`、`present_count≥1`、`fps_interval≥55`（持续 tick 时））

## 第 2 步：跑真窗取 JSON

**禁止跳过**——这是 U5/U7 的核心。

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=<CLOSE_SECONDS> go run ./examples/<package>
```

- 若环境无 GPU/X11：**停下**，告诉用户「需要 GPU 真窗环境（needs_gpu_window）」，禁止用 CPU 单测或 stub 关 R。
- 跑完取 stderr + JSON。若 JSON 缺族字段，回到第 1 步补 main.go。
- 若 `exit 1` 且 `FAIL:` 原因是门禁不达标，**不要**自动改门禁值——报告给用户，门禁是硬的不许放。

## 第 3 步：判门禁（逐族 FAIL 线）

对照 §2.2.2 / §2.2.3 / §2.2.4 + 该 R §2 主表「指标门禁」列，逐条判：

- **族 A**：若该窗是动画/滚动/持续 tick 类（R6/R7/R7b/C3/C5 等），`fps_interval≥55` 或 `interval_p95_ms≤22`；长 soak 另查 `hitch_rate_per_min` 超 README 预算；**必须**有 `vsync_source`（`fallback` 禁止宣称锁 60Hz）
- **族 B**：`pipeline_depth` 持续 > 配置上限 → FAIL
- **族 C**：该 R 专用门禁（如 R3 `boundary_skip>0`、R7 `bind_count≪item_count`、R4 `damage_ratio`≪全屏）；FullPaint 下 `damage_ratio` 可接近 1 但**不得**用其冒充 Retained
- **族 D**：`cpu_pct_avg`；长窗/soak（≥60s）`>85%` 且持续 → FAIL；`cpu_ui_pct`/`cpu_raster_pct` 必采（动画窗禁双 0 却靠降画质过 fps 门禁）
- **族 E**：`rss_start/end/peak_kb` 必采（Linux）；soak/压力窗 `rss_slope_kb_per_min` 超预算（默认 `>30000` 极端，场景可收紧）→ FAIL
- **族 F**：热路径 `cpu_fallback_ops` 无故暴涨 → FAIL（阈值写 README）
- **族 H**：R0/R16/C0 首帧有内容；`time_to_first_present_ms` 超预算 → FAIL
- **族 J**：`go test ./ui -run TestNoGPUImport` 绿 + grep `import "C"` 无命中 + 本窗没降画质

**每条判**记录：`PASS` / `FAIL（原因）` / `UNAVAILABLE（原因）`。任一 FAIL → 不许标该 R 为 ✅，回到第 1 步修代码。

## 第 4 步：若该 R 有组合窗，跑组合窗

查 §3 组合表，若该 R 被某组合窗覆盖（如 R7 被 C3 覆盖）：
- 组合窗**不能**代替单能力窗关 R（U5），但**也要绿**（U6 + U9）
- 组合窗同样 1200×800 + `RUN_SECONDS≥max(覆盖各 R)` + §2.2 全族
- 组合窗只做集成回归，不重判单能力门禁

若组合窗目录为空（如 C3 在 W3 未建），**停下问用户**：是现在建组合窗，还是该 R 不带组合窗收口。

## 第 5 步：回写 `docs/ENGINE_UI_WIDGET_RENDER.md` 状态列

**必做**——U9 的硬要求，禁止代码绿但文档仍 ⬜。

1. `read_file docs/ENGINE_UI_WIDGET_RENDER.md` 定位 §2 主表该 R 行的「状态」列（最右列）
2. 用 `edit_file` 把 `⬜` 改成 `✅`（或 `✅ GPU PASS` + 简要证据，对齐 W0/W1/W2 已有行的写法）
3. 若该 R 使某波次 W 关闭清单（§4/§4b/§4c）需要新增条目，同步加
4. 若该 R 是某波次最后一个，更新 §5 分期表该 W 的状态（`⬜` → `✅`）+ §11 一句话末尾的「下一步」表述
5. §10 修订表加一行新版本说明（如 `3.2 | W3 ✅ 核心路径：R7/R7b/R10 真窗 + C3`）

**回写后**重新 `read_file` 确认 edit 落点对、没破坏表格 markdown。

## 第 6 步：单测（回归 · 不单独关 R）

U8：CPU/`NewContext` 单测可作回归，**不能单独**将任一 §R 标完成。但若该 R 有对应单测（如 R3 的 `boundary_cache_test.go`），跑一遍 `go test ./ui/rendering -run TestBoundaryCache -count=1` 确认绿。

## 输出格式

完成后给用户一份结构化报告：

```
✅ R<id> 收口完成

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
文档：docs/ENGINE_UI_WIDGET_RENDER.md §2 R<id> 行 ⬜ → ✅
单测：go test ./ui/rendering -run TestXX  PASS

下一步：W<n+1> 剩 R<下个 id> / R<下个 id>
```

若有任一 FAIL：

```
⚠️ R<id> 未达标

FAIL 原因：<族><具体>
下一步建议：<修代码还是改门禁（门禁是硬的不许改）>
```

## 禁令自检（每次结束前过一遍）

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

任一项未过 → 回到对应步骤修。
