# 自定义控件渲染基座 — Flutter 对齐（统一真源）

> **版本：3.1** | 日期：2026-07-29  
> **地位：** 自定义控件 **渲染基座 + 排期 + 真窗验收** 唯一真源。  
> **并读：** [`ENGINE_UI_RENDER_BASE.md`](./ENGINE_UI_RENDER_BASE.md)（画什么 · **§20 指标族**）· [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md) · [`ENGINE_CODING_RULES.md`](./ENGINE_CODING_RULES.md)  
> **真窗统一规格（硬）：** 客户区 **1200×800** · **`RUN_SECONDS ≥ 5`** · 按主能力加长（**§2.5** 全表）。

---

## 0. 用户要求（验收与范围 · 不可删改义）

> 以下为工程约束原文级固化；实现与文档冲突时 **以本节为准**。

### 0.1 范围

| # | 要求 |
|---|------|
| U1 | 目标是 **任意自定义控件 / 自定义 RO** 可用的渲染基座，**不是** 先做官方 Kit/Ant 皮肤。 |
| U2 | **对齐 Flutter 的是帧经济学**：Layout→Paint/Record→Composite→Present、脏区、RepaintBoundary、层合成、虚拟化宿主、Overlay；**不是** 1:1 Widget/Material。 |
| U3 | **暂缓**：L3 产品控件实现、完整 IME、a11y 桥、Win/mac 平台铺开（SPI 可预留）。 |
| U4 | 「能画」几何/文/图 API 以母表 §22 为准；本文管 **怎么少画、怎么合成、怎么用真窗证明**。 |

### 0.2 验收（硬）

| # | 要求 |
|---|------|
| U5 | **每一个主能力（§R 每一行）必须单独对应一个真实窗口测试程序**（`examples/ui_wr_<id>/`，GPU Present）。 |
| U6 | **另设多主能力组合真窗口测试**（回归交互与集成，不能代替 U5 单能力窗）。 |
| U7 | 每个真窗必须同时具备：① 可 `go run` 的 GPU 窗；② **stderr + JSON 指标**，不达标 `FAIL:` + `exit 1`；③ **README 写明可见效果**（人眼或采样约定）。 |
| U8 | CPU/`NewContext` 单测可作回归，**不能单独** 将任一 §R 或 W 标为完成。 |
| U9 | 关闭某一 W / 某一 R：代码 + **对应单能力真窗绿** +（若该 W 含组合）**相关组合窗绿** + 回写本文状态。 |
| **U12** | **每个真窗必须对齐母表 §20.0 指标族 A–J**（全族必采，见 **§2.2**）；禁止只报业务字段、省略 CPU/RSS/FPS。 |
| **U13** | **流畅：动画/滚动类真窗稳态 FPS 按 60Hz 档门禁（默认 `fps_interval≥55` 或等价，见 §2.2.2）**；必须输出 `vsync_source`（fallback 禁止宣称锁 60Hz）。 |
| **U14** | **CPU + 内存硬观测：** `cpu_pct_avg` / `cpu_ui_pct` / `cpu_raster_pct`；`rss_start/end/peak/slope(/after_close)`；长跑/压力窗 slope 与 CPU 有 **FAIL 线**（§2.2.3–2.2.4）。 |
| **U15** | **真窗统一几何：客户区 `1200×800` 逻辑像素**（**每一个** 单能力窗 `ui_wr_r*` 与组合窗 `ui_wr_c*` 必须；禁止用更小窗关闭能力；调试可改尺寸但 **不得** 标 ✅ / 不得作关闭证据）。 |
| **U16** | **真窗最小运行时间 `RUN_SECONDS≥5`**（硬底线）；**按不同主能力加长** 以便观察 FPS / CPU / RSS / hitch / boundary skip 等（见 **§2 主表「推荐 RUN_SECONDS」列** 与 **§2.5 全表**）。`<5` 只允许本地调试，**不得** 用于关闭任一 R/W/C。实现侧：`<5` → `FAIL:` + `exit 1`。 |

### 0.3 施工节奏

| # | 要求 |
|---|------|
| U10 | 先 **W0 真窗门禁闭环**，再 W1…W6；禁止无真窗宣称 Retained/丝滑。 |
| U11 | 半 retained（CompositeOnly 跳过静态 + GPU Clear）禁止装省绘。 |

---

## 1. 目标分层（短）

```text
L2 RO/Layer/Boundary/Composite  ← 主施工
L1 render Present               ← 已收口，为 L2 服务
L0 Host                         ← Linux 真窗验证；三平台 SPI 预留
L3–L5 Kit                       ← 暂缓
```

---

## 2. 主能力 §R 与 **单能力真窗**（1 : 1）

> **规则（U5 + U15 + U16）：**  
> - 每一行 **必须** 有独立目录 `examples/ui_wr_<solo>/`，禁止「多项挤一个 main 就关闭多项」。  
> - **窗口大小一律 1200×800**（客户区逻辑像素）；**运行时间 ≥5s**，并按本表「推荐 RUN_SECONDS」加长以观察指标。  
> - 关闭证据：`go run` 须用 **≥ 推荐时长**（至少 ≥5）；组合窗见 **§3**，只做集成，**不替代** 本表。

| ID | 能力 | 单能力真窗包名 | **窗口** | **推荐 RUN_SECONDS** | 指标门禁（须 FAIL） | 可见效果（README 必写） | 波次 | 状态 |
|----|------|----------------|----------|----------------------|--------------------|-------------------------|------|------|
| **R0** | **FullPaint 正确性**（静+动同屏，防 Clear 丢静态） | `ui_wr_r0_fullpaint` | **1200×800** | **5**（观察 15） | **§2.2 全族** + policy + 静/动存在；持续 tick 则 **fps 门禁** | 静色块始终在；动块持续变 | **W0** | **✅** |
| **R1** | 局部 NeedsLayout | `ui_wr_r1_layout` | **1200×800** | **5** | `layout_count` 符合「只脏子树」约定 | 仅目标子节点高度变，邻域不抖 | W1+ | ⬜ |
| **R2** | 局部 NeedsPaint | `ui_wr_r2_paint` | **1200×800** | **5** | `paint_count`/visits 可解释 | 仅目标节点变色 | **W1** | **✅** |
| **R3** | Boundary 真缓存 | `ui_wr_r3_boundary` | **1200×800** | **10** | `boundary_rerecord` 仅脏；**`boundary_skip>0`** | 静 boundary 不动；脏每帧变 | **W1** | **✅** |
| **R3b** | Compositing bits / 边界发现 | `ui_wr_r3b_compbits` | **1200×800** | **8** | `boundary_count`；合成链深度 | 嵌套 boundary 只重约定层 | **W1** | **✅** |
| **R4** | 层 Composite Present | `ui_wr_r4_composite` | **1200×800** | **15** | `present_policy`；`damage_ratio` 门禁 | Retained 下静在、damage≪全屏 | **W2** | **✅** |
| **R4b** | DirtyLayerID + 多 damage | `ui_wr_r4b_multidamage` | **1200×800** | **15** | `dirty_layer_ids`；rects/并集 | 两远离脏点更新，中间静在 | **W2** | **✅** |
| **R5** | Picture 录/回放 | `ui_wr_r5_picture` | **1200×800** | **5** | `picture_op_count`；可选像素差 | 回放区≡直绘区 | **W1** | **✅** |
| **R6** | Opacity/Transform/Clip **层**动画 | `ui_wr_r6_layer_anim` | **1200×800** | **30** | `paint_count` 稳；`hitch_rate`；**fps≥55** | 转/淡/裁流畅；静背景不闪 | **W5** | ⬜ |
| **R7** | 虚拟化宿主 | `ui_wr_r7_virtlist` | **1200×800** | **60** | **`bind_count≪item_count`**；p95/hitch；RSS | 仅视口 cell；快滑约定 | **W3** | ⬜ |
| **R7b** | 滚动少重录 cell | `ui_wr_r7b_scroll_reuse` | **1200×800** | **60** | **`scroll_rerecord` 上限**；fps | 静 cell 保持；新入视口才重录 | **W3** | ⬜ |
| **R8** | Overlay 独立合成 | `ui_wr_r8_overlay` | **1200×800** | **15** | 开浮层后主树 `paint_count` 不涨 | 面板盖上；底静仍在 | **W4** | ⬜ |
| **R9** | 文本 measure 缓存 | `ui_wr_r9_text_cache` | **1200×800** | **5** | `measure_cache_hit`（可先打桩再严） | 同文同 style 宽高稳、不抖 | **W1** | **✅** |
| **R10** | 图异步→局部脏 | `ui_wr_r10_async_image` | **1200×800** | **30** | 出图后 rerecord **仅一格** | 占位→图仅该格变 | **W3** | ⬜ |
| **R11** | DPR/尺寸缓存失效 | `ui_wr_r11_dpr` | **1200×800** | **15** | 变更后 rerecord **一波**再回稳 | 无残影、不错位 | **W2** | **✅** |
| **R12** | 帧指标字段完备 | `ui_wr_r12_metrics` | **1200×800** | **5** | **公共字段全集存在**否则 FAIL | stderr/JSON 可读 | 全程 | ⬜ |
| **R12b** | 重绘调试可视化 | `ui_wr_r12b_debug_repaint` | **1200×800** | **8** | `debug_repaint=1` 时有叠加标志 | **人眼见谁在重绘** | **W1** | **✅** |
| **R13** | Hit ≡ 绘 | `ui_wr_r13_hit` | **1200×800** | **5** | 点击→命中 ID（脚本或日志断言） | 点哪高亮哪 | **W2** | **✅** |
| **R14** | 缓存预算/淘汰 | `ui_wr_r14_cache_budget` | **1200×800** | **60** | `cache_entries`、**RSS slope FAIL** | 超预算仍正确 | **W6** | ⬜ |
| **R15** | UI/raster 所有权·长跑 | `ui_wr_r15_soak` | **1200×800** | **300** | 时长、无崩、无读回；hitch/CPU/RSS | soak 不挂 | W2+ | ⬜ |
| **R16** | 首帧/WarmUp/恢复 | `ui_wr_r16_warmup`（W0 由 C0 覆盖子集） | **1200×800** | **5** | 首帧 full；policy | 首帧有内容；恢复不黑 | **W0** 子集 ✅ · 完整窗 ⬜ |
| **R17** | 不可见降频 | `ui_wr_r17_bg_throttle` | **1200×800** | **30** | 后台 interval 明显变大 | 后置 | 后置 | ⬜ |
| **R18** | SaveLayer+预算 | `ui_wr_r18_savelayer` | **1200×800** | **10** | `savelayer_count`/reject | 组内半透明对；超预算可观测 | **W2** | **✅** |
| **R19** | 1px/设备像素对齐 | `ui_wr_r19_snap` | **1200×800** | **5** | 约定 scale 下采样或截图门禁 | 1px 线清晰不糊 | W1–W2 | ⬜ |
| **R20** | Filter 层（可选） | `ui_wr_r20_filter` | **1200×800** | **10** | 局部 rerecord | 子树灰/糊，外不变 | 可选 | ⬜ |
| **R21** | 壳/内容分层 | `ui_wr_r21_shell` | **1200×800** | **15** | 滚体时顶栏 `rerecord=0` | 顶栏静、体滚 | W2–W4 | ⬜ |
| **R22** | 选区/光标局部脏预留 | `ui_wr_r22_selection_stub` | **1200×800** | **5** | 字段可 0；API 存在 | stub 可空跑；防将来全窗刷 | 预留 | ⬜ |

**命名纪律：** 包名 `ui_wr_<id>_*` 与上表 **一一对应**；新增 R 必须同步新增一行 + 一个包 + **窗口/时长** 两列，禁止复用他包关闭新 R。

### 2.1 Present 策略

| 策略 | 含义 | 默认 |
|------|------|------|
| `full_paint` | 每帧全树 paint | **全局默认**（W0 正确性；防 Clear 丢静态） |
| `retained` | 稳态 CompositeOnly + PresentWithAuto damage | **W2 窗可选**（`SetPresentPolicy`）；W6 再作默认 |
| `hybrid` | 可选 | — |

禁止：CompositeOnly 跳过静态 + **GPU 全幅 Clear** 并存（无 LoadOpLoad/damage 时）。  
允许：`retained` 下 CompositeOnly + **damage Present（LoadOpLoad）** — 静态靠未脏区保留。

### 2.2 真窗硬性指标（对齐母表 §20.0 · **此前未写全，v2.4 补齐**）

> 母表定义见 [`ENGINE_UI_RENDER_BASE.md` §20.0–§20.2](./ENGINE_UI_RENDER_BASE.md)。  
> **每个** `ui_wr_*` 真窗结束时必须输出 **一族不少的 JSON 块**（无数据填 `null`/`0` + `unavailable` 原因，**禁止默默省略**）。  
> 下表 **「硬 FAIL」** 列：不满足则 `exit 1`（除非 README 声明本窗为 `gate=schema_only` 且仅用于 R12 字段存在性——**仅 R12 允许**）。

#### 2.2.1 指标族 × 真窗义务（§20.0）

| 族 | 回答什么 | 真窗 **必采字段**（JSON） | 母表 M-*（对照） | **硬 FAIL 默认**（可按窗收紧，不可无故放宽不写） |
|----|----------|---------------------------|------------------|--------------------------------------------------|
| **A 帧时** | 稳不稳、卡在哪 | `fps_wall`、`interval_avg_ms`、`interval_p50_ms`、`interval_p95_ms`、`interval_p99_ms`、`hitch_count`、`hitch_rate_per_min`、`vsync_source`、`target_hz` | M-FPS-WALL、M-INTERVAL-*、M-HITCH*、M-VSYNC-SOURCE、M-TARGET-HZ | 见 **§2.2.2 FPS/流畅** |
| **B 管线** | 是否堵 Present | `frame_build_ms`、`frame_raster_ms`、`pipeline_depth`、`pipeline_max` | M-BUILD-MS、M-RASTER-MS、M-PIPE-* | 深度持续 > 配置上限 → FAIL；build/raster 尖峰记入 p95 观察（有 p99 后升硬） |
| **C 脏区** | 成本 ∝ 脏？ | `layout_count`、`paint_count`、`damage_area_px`、`damage_ratio`、`present_mode`、`present_policy`；能力相关 `bind_count`/`boundary_*`/`dirty_layer_ids` | M-LAYOUT/PAINT、M-DAMAGE-AREA、M-BIND… | **按该 R 专用门禁**（如 R3 skip>0、R7 bind≪N）；FullPaint 下 `damage_ratio` 可接近 1（**不得**用其冒充 Retained） |
| **D CPU** | 谁吃 CPU | **`cpu_pct_avg`**（进程）、**`cpu_ui_pct`**、**`cpu_raster_pct`**（路径 proxy） | M-CPU-PROCESS、M-CPU-UI、M-CPU-RASTER | 见 **§2.2.3 CPU** |
| **E 内存** | 漏不漏 | **`rss_start_kb`、`rss_end_kb`、`rss_peak_kb`、`rss_slope_kb_per_min`、`rss_after_close_kb`**（能采 after_close 则采） | M-RSS-* | 见 **§2.2.4 内存** |
| **F GPU** | 提交/回退 | `gpu_ops`、`cpu_fallback_ops`、`last_cpu_fallback`、`frame_flushes` | M-GPU-SUBMIT/FALLBACK* | 热路径 `cpu_fallback_ops` 无故暴涨 → FAIL（阈值写在窗 README；默认 soak 可 >0 但须解释） |
| **G 图/文** | 缓存/解码 | 有则：`measure_cache_hit`、解码不在 UI 的证明字段/日志 | M-ATLAS-*、M-IMG-DECODE | R9/R10 窗强制；其它窗可 `g_metrics=skipped` |
| **H 启动** | 首帧 | `warmup`、`time_to_first_present_ms`（有则） | M-WARMUP、M-TIME-TO-FIRST-PRESENT | R0/R16/C0：**首帧有内容**；`time_to_first_present_ms` 超预算 FAIL（预算写 README，默认建议 &lt;2000ms 冷启同机） |
| **I 回归** | 变差？ | 支持 `BASELINE_JSON` / `SAVE_BASELINE_JSON`（与 perfsoak 同） | M-BASELINE-DELTA | 可选开；**发布/合入关键窗建议开**；超 `DefaultBaselineTolerance` → FAIL |
| **J 正确性** | 假绿？ | 构建/CI：`ui` 无 import gpu；无 cgo；本窗不降画质 | M-DEP-*、M-NO-QUALITY-CHEAT | 违规直接拒合入 |

#### 2.2.2 帧时 / **FPS 60+** 硬门禁（族 A）

| 窗类型 | `target_hz` | **硬 FAIL** | 说明 |
|--------|-------------|-------------|------|
| **动画 / 滚动 / 持续 tick**（R6、R7、R7b、C3、C5、C10 等） | 60 | **有效 FPS &lt; 55**（优先 `fps_interval`，否则 `fps_wall`）或 **`interval_p95_ms > 22`** | 目标 60fps 档；须 **§2.5 推荐时长** 再判 |
| **同上 · 长 soak**（表内 ≥60s） | 60 | 另：**`hitch_rate_per_min` 超 README 预算**（默认建议 ≤5） | 与母表 hitch 一致 |
| **正确性 + 持续 tick**（R0 等） | 60 | 运行 **≥5s** 后：有效 FPS ≥55（同 60 档） | 短于 5s **不得**关 R |
| **任意窗** | — | **必须输出 `vsync_source`** | `fallback` 禁止宣称锁 60Hz |

```text
fps_wall     = present_count / elapsed_sec   // 含开关窗，仅参考
fps_interval = 1000 / interval_avg_ms        // 稳态帧率，门禁优先
窗口默认     = 1200×800 逻辑像素（U15）
最小 RUN_SECONDS = 5（U16）；更长见 §2.5
```

#### 2.2.3 CPU 硬门禁（族 D）

| 指标 | 来源 | **硬 FAIL 默认** |
|------|------|------------------|
| `cpu_pct_avg` | ProcessTracker | **短窗（&lt;30s）**：只 **必采**；尖峰记日志。**长窗/soak（≥60s）**：`cpu_pct_avg > 单核折算预算` → FAIL（默认建议 **&gt;85%** 且持续，具体写 README；无 /proc 则 `cpu_unavailable=true`，Linux 真窗 **禁止** 长期 unavailable） |
| `cpu_ui_pct` / `cpu_raster_pct` | 路径 proxy | **必采**（无 build/raster note 时可为 0）；动画窗二者应可解释（禁止双 0 却 fps 门禁靠降画质） |

> proxy ≠ OS 线程 CPU（母表已说明）；真窗仍须输出，便于回归。

#### 2.2.4 内存硬门禁（族 E）

| 指标 | **硬 FAIL 默认** |
|------|------------------|
| `rss_start_kb` / `rss_end_kb` / `rss_peak_kb` | **必采**（Linux）；缺失 → FAIL |
| `rss_slope_kb_per_min` | **soak/压力窗（≥60s 或 R14/C9）**：slope **&gt; 预算** → FAIL（默认建议 **&gt;30000 KB/min** 极端爬升，场景可收紧到更低）；短功能窗必采、默认只告警可在 README 写 `slope_gate=off` **仅短于 15s 的正确性窗** |
| `rss_after_close_kb` | 能采则采；关闭后相对 peak **无故不降** 记 WARN，soak 可升 FAIL |

#### 2.2.5 每个真窗 JSON 最小外壳（实现模板）

```json
{
  "ability_id": "R3",
  "scenario": "ui_wr_r3_boundary",
  "client_px": "1200x800",
  "run_seconds": 10,
  "present_policy": "full_paint",
  "target_hz": 60,
  "fps_wall": 58.2,
  "interval_p50_ms": 16.7,
  "interval_p95_ms": 18.1,
  "interval_p99_ms": 20.0,
  "hitch_count": 0,
  "hitch_rate_per_min": 0,
  "vsync_source": "fallback",
  "frame_build_ms": 0.4,
  "frame_raster_ms": 1.2,
  "pipeline_depth": 0,
  "layout_count": 12,
  "paint_count": 120,
  "damage_area_px": 0,
  "damage_ratio": 0,
  "present_mode": "full",
  "cpu_pct_avg": 12.5,
  "cpu_ui_pct": 40.0,
  "cpu_raster_pct": 60.0,
  "rss_start_kb": 100000,
  "rss_end_kb": 102000,
  "rss_peak_kb": 105000,
  "rss_slope_kb_per_min": 120.0,
  "gpu_ops": 1000,
  "cpu_fallback_ops": 0,
  "ability_extra": {}
}
```

`ability_extra`：该 R 专用字段（boundary_skip、bind_count…）。

#### 2.2.6 与旧「公共字段」关系

**U12–U16 + §2.2.1–2.2.5 + §2.5** 为硬要求；§2 各 R 行「指标」列 = **在全族必采之上的附加门禁**。

### 2.5 真窗几何与运行时长（U15 / U16）

> **每个主能力 / 组合真窗测试必须遵守：**  
> 1. 客户区 **固定 1200×800**（逻辑像素）；  
> 2. **`RUN_SECONDS ≥ 5`**（硬底线，实现侧 `<5` → FAIL）；  
> 3. **按能力使用下表「关闭用」时长**（可更长、关闭时不可更短），以便稳定观察 FPS / CPU / RSS / hitch / 能力专用字段。

#### 统一规格

| 项 | 值 | 说明 |
|----|-----|------|
| **客户区大小** | **1200 × 800** 逻辑像素 | **全部** `ui_wr_r*` / `ui_wr_c*`；HiDPI 下物理 = 1200×scale × 800×scale |
| **最小运行** | **`RUN_SECONDS ≥ 5`** | 硬底线；环境变量可加大；**不可**用 &lt;5 关闭能力 |
| **关闭用时长** | 见下表「关闭用 RUN_SECONDS」 | 至少取该列；本地观察 CPU/RSS 可再加长 |
| **采样** | 全程 `ProcessTracker.Sample` + 帧间隔环 | 结束时打 §2.2 JSON（全族 A–J） |
| **标题** | 含 ability id | 如 `gpui ui_wr_r3_boundary` |
| **JSON 外壳** | 建议含 `client_px":"1200x800"`、`run_seconds` | 便于门禁与 baseline 对齐 |

```bash
# 标准跑法（所有 ui_wr_*）
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 硬底线（所有能力）
RUN_SECONDS=5 go run ./examples/ui_wr_<id>

# 关闭用：取 §2 主表 / 下表「关闭用」列（例）
RUN_SECONDS=10 go run ./examples/ui_wr_r3_boundary      # Boundary
RUN_SECONDS=30 go run ./examples/ui_wr_r6_layer_anim    # 动画
RUN_SECONDS=60 go run ./examples/ui_wr_r7_virtlist      # 虚拟列表
RUN_SECONDS=300 go run ./examples/ui_wr_r15_soak        # soak
```

#### 按主能力 **关闭用时长全表**（可加长，勿缩短关闭）

| ID | 窗口 | **关闭用 RUN_SECONDS** | 本地加长建议 | 观察重点（指标） |
|----|------|------------------------|--------------|------------------|
| R0 | 1200×800 | **5** | 15 | policy、静/动、fps_interval、§2.2 全族 |
| R1 | 1200×800 | **5** | 10 | layout_count 只脏子树 |
| R2 | 1200×800 | **5** | 10 | paint_count / visits |
| R3 | 1200×800 | **10** | 30 | **boundary_skip>0**、rerecord 仅脏、CPU |
| R3b | 1200×800 | **8** | 15 | boundary_count、合成深度 |
| R4 | 1200×800 | **15** | 30 | present_policy、damage_ratio |
| R4b | 1200×800 | **15** | 30 | dirty_layer_ids、多 damage 并集 |
| R5 | 1200×800 | **5** | 10 | picture_op_count、回放≡直绘 |
| R6 | 1200×800 | **30** | 60 | **fps≥55**、p95、hitch、静背景 |
| R7 | 1200×800 | **60** | 120 | bind≪N、p95、hitch、**RSS slope** |
| R7b | 1200×800 | **60** | 120 | scroll_rerecord 上限、fps |
| R8 | 1200×800 | **15** | 30 | 开 overlay 后主 paint 不涨 |
| R9 | 1200×800 | **5** | 15 | measure_cache_hit |
| R10 | 1200×800 | **30** | 60 | 出图后仅一格 rerecord |
| R11 | 1200×800 | **15** | 30 | resize/DPR 一波 rerecord 再回稳 |
| R12 | 1200×800 | **5** | 10 | 字段 schema 全集 |
| R12b | 1200×800 | **8** | 15 | debug 重绘可视化 |
| R13 | 1200×800 | **5** | 10 | 命中 ID |
| R14 | 1200×800 | **60** | 120 | cache_entries、**RSS slope FAIL** |
| R15 | 1200×800 | **300** | 600 | 无崩、hitch_rate、CPU/RSS |
| R16 | 1200×800 | **5** | 15 | 首帧 full、warmup、恢复不黑 |
| R17 | 1200×800 | **30** | 60 | 后台 interval 变大（含最小化段） |
| R18 | 1200×800 | **10** | 20 | savelayer_count / reject |
| R19 | 1200×800 | **5** | 10 | 1px 对齐采样 |
| R20 | 1200×800 | **10** | 20 | 局部 rerecord、滤镜范围 |
| R21 | 1200×800 | **15** | 30 | 顶栏 rerecord=0、体滚 |
| R22 | 1200×800 | **5** | 10 | stub API / 字段可 0 |

#### 按组合窗 **关闭用时长**

| ID | 窗口 | **关闭用 RUN_SECONDS** | 覆盖最重能力 | 观察重点 |
|----|------|------------------------|--------------|----------|
| C0 | 1200×800 | **5** | R0 | policy、presents、首帧、schema |
| C1 | 1200×800 | **10** | R3 | nest skip/rerecord、debug 色 |
| C2 | 1200×800 | **15** | R4 | damage、dirty_layers、Picture |
| C3 | 1200×800 | **60** | R7 | bind、scroll_rerecord、p95、RSS |
| C4 | 1200×800 | **15** | R8 | 顶栏静、overlay 主 paint |
| C5 | 1200×800 | **30** | R6 | hitch、静不闪、fps |
| C6 | 1200×800 | **10** | R18 | savelayer_* |
| C7 | 1200×800 | **15** | R11 | 一波 rerecord、1px |
| C8 | 1200×800 | **15** | R8/R6 | 命中 ID under xform |
| C9 | 1200×800 | **60** | R14 | cache_entries、RSS slope |
| C10 | 1200×800 | **300** | R15 | 无崩、hitch、CPU/RSS |
| C11 | 1200×800 | **15** | R0↔R4 | policy 切换后静不丢 |

#### 类别速查（与上表一致）

| 类别 | 适用 ID | **关闭用** | 观察重点 |
|------|---------|------------|----------|
| 正确性 / schema / 命中 / snap | R0–R2, R5, R9, R12, R13, R16, R19, R22, C0 | **5** | 内容对、policy、schema、fps_interval |
| Boundary / 合成位 / debug | R3, R3b, R12b, C1 | **8–10** | skip/rerecord 稳定 |
| 层 Present / 多 damage / DPR / 壳 | R4, R4b, R11, R21, C2, C4, C7, C11 | **15** | damage、双脏、resize |
| SaveLayer / Filter | R18, R20, C6 | **10** | 层计数、拒批 |
| **动画层** | R6, C5 | **30** | fps≥55、p95、hitch |
| **虚拟列表 / 滚复用 / 异步图** | R7, R7b, R10, C3 | **30–60** | bind、scroll_rerecord、RSS |
| Overlay / 命中组合 | R8, C8 | **15** | 主 paint 不涨、命中 |
| **预算 / 压力** | R14, C9 | **60** | cache、RSS slope FAIL |
| **Soak 长跑** | R15, C10 | **300** | 无崩、hitch_rate、CPU/RSS |
| 后台降频 | R17 | **30** | interval 变大 |

**原则：**  
- **窗口永远 1200×800**；时长 **永远 ≥5**，关闭时取「关闭用」列。  
- 需要看 **RSS slope / hitch_rate / 滚动复用 / 动画稳态** 的，**禁止** 用 5s 蒙混关闭。  
- 实现：`Width/Height = 1200/800`；`runSeconds` 默认取关闭用值；`RUN_SECONDS` 环境变量可覆盖但 **&lt;5 → FAIL**。

### 2.3 作者纪律

视觉→Paint；尺寸→Layout；热点→Boundary；长列表→VirtualList；OnPaint 禁 IO/改树；动画优先层；浮层→Overlay；海量图→独立层；SaveLayer 走预算。

### 2.4 非主能力（明确不做进 §R 关闭）

Kit 组件、IME 实现、a11y 桥、多窗产品、系统托盘/菜单深做、剪贴板/拖放产品、RTL 产品、母表几何 API 清零。

---

## 3. 组合真窗口测试（多主能力 · 不替代 §2）

> **规则（U6 + U15 + U16）：** 组合窗同样 **1200×800**、**`RUN_SECONDS≥5`**；**关闭用时长 = max(所覆盖各 R 的关闭用)**（见 §2.5 组合表）。  
> 只做集成回归；**不能** 用组合窗代替单能力窗关闭 R。

| 组合 ID | 覆盖的主能力（至少） | 真窗包名 | **窗口** | **推荐 RUN_SECONDS** | 要证明的集成效果 | 指标要点 | 波次 |
|---------|----------------------|----------|----------|----------------------|------------------|----------|------|
| **C0** | R0+R12+R16 | `ui_wr_c0_smoke` | **1200×800** | **5** | 开机即见静+动；指标字段齐 | policy、presents、首帧 | **W0** |
| **C1** | R2+R3+R3b+R12b | `ui_wr_c1_boundary_nest` | **1200×800** | **10** | 嵌套 boundary + 可开关 debug 重绘色 | rerecord/skip/boundary_count | **W1** ✅ 组合窗（不单独关 R2/R12b） |
| **C2** | R3+R4+R4b+R5 | `ui_wr_c2_retained_scene` | **1200×800** | **15** | Retained 整场景：多 boundary + Picture | damage_ratio、dirty_layers | **W2** ✅ 组合窗 |
| **C3** | R4+R7+R7b+R10 | `ui_wr_c3_list_scroll` | **1200×800** | **60** | 虚拟列表 + 滚复用 + 异步图格 | bind、scroll_rerecord、p95 | **W3** |
| **C4** | R3+R8+R21 | `ui_wr_c4_shell_overlay` | **1200×800** | **15** | 顶栏静 + 体内容 + 浮层面板 | 顶栏 rerecord=0；开 overlay 主 paint | **W4** |
| **C5** | R6+R3+R4 | `ui_wr_c5_anim_over_static` | **1200×800** | **30** | 层动画盖在静态缓存上 | hitch；静不闪 | **W5** |
| **C6** | R18+R3 | `ui_wr_c6_savelayer_group` | **1200×800** | **10** | 离屏组 + boundary | savelayer_* | W2/W5 |
| **C7** | R11+R19+R3 | `ui_wr_c7_resize_dpr` | **1200×800** | **15** | 改尺寸/DPR 后缓存与 1px 线 | 一波 rerecord；线清晰 | **W2** ✅ 组合窗 |
| **C8** | R13+R6+R8 | `ui_wr_c8_hit_overlay_xf` | **1200×800** | **15** | 变换/浮层下命中 | 命中 ID | W4+ |
| **C9** | R14+R3+R7 | `ui_wr_c9_stress_cache` | **1200×800** | **60** | 多 boundary + 列表压缓存 | cache_entries、RSS | **W6** |
| **C10** | R15+全主路径 | `ui_wr_c10_soak` | **1200×800** | **300** | 长跑组合 | 无崩；hitch 可报 | W2+ |
| **C11** | R0→R4 策略切换 | `ui_wr_c11_policy_switch` | **1200×800** | **15** | full_paint↔retained 切换正确 | policy 字段；切换后静不丢 | **W6** |

---

## 4. W0 关闭清单（2026-07-28 落地）

### 4.1 结论

| 项 | 状态 |
|----|------|
| 稳态全树 paint 代码 | ✅ |
| `present_policy=full_paint` → Metrics/JSON | ✅ `scheduler` + `NewPipelineApp` |
| CPU 单测（不单独关闭 W0） | ✅ 已标注 |
| **`ui_wr_r0_fullpaint` 真窗门禁** | ✅ PASS（GPU X11；§2.2 JSON + policy + presents + fps_interval） |
| **`ui_wr_c0_smoke` 组合门禁** | ✅ PASS（schema + warmup + policy） |
| 共享 `examples/wrgate` 门禁/报告 | ✅ 含 unit 测 FAIL 路径 |

**状态：✅ W0 真窗门禁闭环**（R16 独立完整窗仍可后补；C0 已覆盖 WarmUp 子集）。

### 4.2 关闭项对照

| # | 内容 | 状态 |
|---|------|------|
| W0.1 | Metrics：`present_policy=full_paint` | ✅ |
| W0.2 | **`examples/ui_wr_r0_fullpaint`** | ✅ |
| W0.3 | **`examples/ui_wr_c0_smoke`** | ✅ |
| W0.4 | 两窗：§2.2 全族 + FPS/CPU/RSS + README | ✅ |
| W0.5 | 首帧/WarmUp：C0 `warmup:true` + R0 WarmUp | ✅ 子集 |
| W0.6 | 单测注明不单独关闭 W0 | ✅ |
| W0.7 | JSON：`fps_wall`/`fps_interval`/`cpu_*`/`rss_*`/`present_policy` | ✅ |

---

## 4b. W1 关闭清单（2026-07-28）

| 项 | 状态 |
|----|------|
| `BoundaryCache` own-content + 嵌套 walk | ✅ |
| `CountRepaintBoundaries` + compositing bits | ✅ |
| Metrics `boundary_*` + wrgate 门禁 | ✅ |
| **R2** `ui_wr_r2_paint` | ✅ GPU PASS（static clean ticks + skip） |
| **R3** `ui_wr_r3_boundary` | ✅ GPU PASS |
| **R3b** `ui_wr_r3b_compbits` | ✅ GPU PASS |
| **R5** `ui_wr_r5_picture` | ✅ GPU PASS（ops≥3, replay_frames） |
| **R9** `ui_wr_r9_text_cache` | ✅ GPU PASS（measure hits≫miss） |
| **R12b** `ui_wr_r12b_debug_repaint` | ✅ GPU PASS（debug draws≥1） |
| **C1** `ui_wr_c1_boundary_nest` | ✅ 组合窗 |
| R19 1px snap | 可选，未关（不挡 W1） |

**状态：✅ W1 主能力真窗闭环**（R19 可选后置）。

---

## 4c. W2 关闭清单（2026-07-28/29）

| 项 | 状态 |
|----|------|
| `SetPresentPolicy(retained)` → CompositeOnly + PresentWithAuto | ✅ |
| Damage / DirtyLayerIDs 指标 | ✅ |
| **R4** `ui_wr_r4_composite` | ✅ |
| **R4b** `ui_wr_r4b_multidamage` | ✅ |
| **R5**（W1） | ✅ |
| **R11** `ui_wr_r11_dpr` | ✅ size 变更 + `InvalidateBoundaryCache` → rerecord 再 skip |
| **R13** `ui_wr_r13_hit` | ✅ 脚本探针 4/4 + DebugName |
| **R18** `ui_wr_r18_savelayer` | ✅ allow+reject 预算 |
| **C2** / **C7** | ✅ 组合窗 |
| R21 壳分层 | ⬜ 可选后置（W2–W4） |
| R19 1px snap 独立窗 | ⬜ 可选（C7 含 hairline 子集） |

**状态：✅ W2 主能力真窗闭环**（R21/R19 独立窗可选后置）。全局默认 Present 仍 `full_paint` 至 W6。

---

## 5. 分期（W 关闭 = 单窗全集 + 组合窗）

| W | 状态 | 必须绿的 **单能力窗** | 必须绿的 **组合窗** |
|---|------|----------------------|---------------------|
| **W0** | **✅** | R0、R12（经 C0 schema）、R16 子集 | C0 |
| **W1** | **✅** | **R2✅ R3✅ R3b✅ R5✅ R9✅ R12b✅** · R19(可) | **C1✅** |
| **W2** | **✅** | **R4✅ R4b✅ R5✅ R11✅ R13✅ R18✅** · R21(可) R19(可) | **C2✅ C7✅** |
| **W3** | ⬜ | R7、R7b、R10 | C3 |
| **W4** | ⬜ | R8、R21(若未做) | C4、C8(可) |
| **W5** | ⬜ | R6、R20(可) | C5、C6(可) |
| **W6** | ⬜ | R14、默认 retained | C9、C11；回归 C0–C5 |

```text
W0 → W1 → W2 → W6
         ↘ W3
    W2 → W4 → W5
```

**90 天：** 0–15d W0 真窗；15–45d W1 单窗+ C1；45–75d W2+C2；75–90d W2 收口或 W3。

**熔断：** 无单能力真窗标 ✅；用组合窗代替单窗关闭 R；CompositeOnly+Clear 装省绘；列表万 RO。

---

## 6. 模块落点

| 能力 | 包 |
|------|-----|
| Present / PaintPresentTree / policy | `ui/embedder` |
| 脏 layout/paint、VirtualList | `ui/rendering` |
| Picture、Layer、Composite | `ui/scene` |
| Overlay | `ui/overlay` |
| 指标 | `ui/scheduler` |
| **全部真窗** | `examples/ui_wr_*` only |

---

## 7. 域 / L0（非施工单）

G0–G17 / X 横切：需求地图。L0 三平台：预留；真窗本阶段 Linux GPU。

---

## 8. 宣称

```text
允许：某 R/C 在对应 ui_wr_* 真窗 + §2.2 全族指标（含 FPS/CPU/RSS）证据下描述已达标
禁止：无单能力真窗关闭 R；省略 CPU/RSS/FPS 只报业务字段；
      fallback 宣称锁 60Hz；FullPaint 冒充 Retained；无 baseline 谈更顺
```

---

## 9. 开放问题

| ID | 问题 | 倾向 |
|----|------|------|
| Q1 | 单能力窗数量多，CI 如何跑 | 标签 `wr_solo` / `wr_combo`；PR 必跑当前 W 相关 |
| Q2 | R12 与每个窗的指标重复 | R12 窗测「字段 schema」；各窗测业务门禁 |
| Q3 | 无显示 CI | `needs_gpu_window`；禁止假绿 |

---

## 10. 修订

| 版本 | 说明 |
|------|------|
| **3.1** | **W2 ✅ 全波主路径**：R11/R13/R18 + C7；R21/R19 独立窗仍可选 |
| 3.0 | **W2 核心**：R4/R4b/C2 retained Present（CompositeOnly + damage_multi） |
| 2.9 | **W1 ✅ 全波**：R2/R5/R9/R12b 真窗 + measure cache + debug repaint；R19 仍可选 |
| 2.8 | **W1 核心**：R3/R3b/C1；`BoundaryCache` own-content 嵌套 |
| 2.7 | **主表落地时长**：§2 / §3 每行 **窗口 1200×800** + **推荐 RUN_SECONDS**；§2.5 关闭用全表 |
| 2.6 | **真窗规格**：U15 **1200×800**；U16 **RUN_SECONDS≥5** + §2.5 分类加长；W0 示例同步 |
| 2.5 | **W0 ✅ 闭环**：`present_policy`；r0 + c0；wrgate |
| 2.4 | 真窗硬指标 §20.0 / FPS/CPU/RSS（U12–U14） |
| 2.3 | U1–U11；每 R 独立真窗；组合 C0–C11 |
| 2.2 | R3b…R22 补强 |
| 2.1 | 真窗硬规则；W0 降 🔄 |
| 2.0 | 四册合一 |

---

## 11. 一句话

> **每个主能力真窗：1200×800 · RUN_SECONDS≥5 · 按能力加长观察指标（§2 主表 + §2.5 全表）。**  
> **W0 ✅ · W1 ✅ · W2 ✅**（retained + hit + DPR 失效 + SaveLayer）。**下一步 W3** 虚拟列表。默认 Present 仍 full_paint 至 W6。
