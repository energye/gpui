# 自定义控件渲染基座 — Flutter 对齐（统一真源）

> **版本：初始重置** | 日期：2026-08-01  
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
| **R0** | **FullPaint 正确性**（静+动同屏，防 Clear 丢静态） | `ui_wr_r0_fullpaint` | **1200×800** | **5**（观察 15） | **§2.2 全族** + policy + 静/动存在；持续 tick 则 **fps 门禁** | 静网+文+动；LiveHUD；相位 | **W0** | **✅** |
| **R1** | 局部 NeedsLayout | `ui_wr_r1_layout` | **1200×800** | **5** | `layout_count` 符合「只脏子树」约定 | 仅目标子节点高度变，邻域不抖 | W1+ | ⬜ |
| **R2** | 局部 NeedsPaint | `ui_wr_r2_paint` | **1200×800** | **5** | `paint_count`/visits 可解释 | 仅目标节点变色 | **W1** | **✅** |
| **R3** | Boundary 真缓存 | `ui_wr_r3_boundary` | **1200×800** | **10** | `boundary_rerecord` 仅脏；**`boundary_skip>0`** | 静 boundary 不动；脏每帧变 | **W1** | **✅** |
| **R3b** | Compositing bits / 边界发现 | `ui_wr_r3b_compbits` | **1200×800** | **8** | `boundary_count`；合成链深度 | 嵌套 boundary 只重约定层 | **W1** | **✅** |
| **R4** | 层 Composite Present | `ui_wr_r4_composite` | **1200×800** | **15** | `present_policy`；`damage_ratio` 门禁 | Retained 下静在、damage≪全屏 | **W2** | **✅** |
| **R4b** | DirtyLayerID + 多 damage | `ui_wr_r4b_multidamage` | **1200×800** | **15** | `dirty_layer_ids`；rects/并集 | 两远离脏点更新，中间静在 | **W2** | **✅** |
| **R5** | Picture 录/回放 | `ui_wr_r5_picture` | **1200×800** | **5** | `picture_op_count`；可选像素差 | 回放区≡直绘区 | **W1–W2** | **✅v2** |
| **R6** | Opacity/Transform/Clip **层**动画 | `ui_wr_r6_layer_anim` | **1200×800** | **30** | `paint_count` 稳；`hitch_rate`；**fps≥55** | 转/淡/裁流畅；静背景不闪 | **W5** | ⬜ |
| **R7** | 虚拟化宿主 | `ui_wr_r7_virtlist` | **1200×800** | **60** | **`bind_count≪item_count`**；p95/hitch；RSS | 仅视口 cell；快滑约定 | **W3** | **⬜** |
| **R7b** | 滚动少重录 cell | `ui_wr_r7b_scroll_reuse` | **1200×800** | **60** | **`scroll_rerecord` 上限**；fps | 静 cell 保持；新入视口才重录 | **W3** | **⬜** |
| **R8** | Overlay 独立合成 | `ui_wr_r8_overlay` | **1200×800** | **15** | 开浮层后主树 `paint_count` 不涨 | 面板盖上；底静仍在 | **W4** | ⬜ |
| **R9** | 文本 measure 缓存 | `ui_wr_r9_text_cache` | **1200×800** | **5** | `measure_cache_hit`（可先打桩再严） | 同文同 style 宽高稳、不抖 | **W1** | **✅** |
| **R10** | 图异步→局部脏 | `ui_wr_r10_async_image` | **1200×800** | **30** | 出图后 rerecord **仅一格** | 占位→图仅该格变 | **W3** | **⬜** |
| **R11** | DPR/尺寸缓存失效 | `ui_wr_r11_dpr` | **1200×800** | **15** | 变更后 rerecord **一波**再回稳 | 无残影、不错位 | **W2** | **✅** |
| **R12** | 帧指标字段完备 | `ui_wr_r12_metrics` | **1200×800** | **5** | **公共字段全集存在**否则 FAIL（可 `schema_only` 主判，仍须真窗 Present） | stderr/JSON 可读；字段齐 | **W0** | **✅** |
| **R12b** | 重绘调试可视化 | `ui_wr_r12b_debug_repaint` | **1200×800** | **8** | `debug_repaint=1` 时有叠加标志 | **人眼见谁在重绘** | **W1** | **✅** |
| **R13** | Hit ≡ 绘 | `ui_wr_r13_hit` | **1200×800** | **5** | 点击→命中 ID（脚本或日志断言） | 点哪高亮哪 | **W2** | **✅** |
| **R14** | 缓存预算/淘汰 | `ui_wr_r14_cache_budget` | **1200×800** | **60** | `cache_entries`、**RSS slope FAIL** | 超预算仍正确 | **W6** | ⬜ |
| **R15** | UI/raster 所有权·长跑 | `ui_wr_r15_soak` | **1200×800** | **300** | 时长、无崩、无读回；hitch/CPU/RSS | soak 不挂 | W2+ | ⬜ |
| **R16** | 首帧/WarmUp/恢复 | `ui_wr_r16_warmup` | **1200×800** | **5** | 首帧 full；`warmup:true`；policy；首帧有内容 | 首帧有内容；恢复不黑 | **W0** | **✅** |
| **R17** | 不可见降频 | `ui_wr_r17_bg_throttle` | **1200×800** | **30** | 后台 interval 明显变大 | 后置 | 后置 | ⬜ |
| **R18** | SaveLayer+预算 | `ui_wr_r18_savelayer` | **1200×800** | **10** | `savelayer_count`/reject | 组内半透明对；超预算可观测 | **W2** | **✅** |
| **R19** | 1px/设备像素对齐 | `ui_wr_r19_snap` | **1200×800** | **5** | 约定 scale 下采样或截图门禁 | 1px 线清晰不糊 | W1–W2 | **✅** |
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

### 2.6 每个 R 窗口复杂度标准（U17/U18/U20 质量条 · 硬）

> **原则：每个 R 窗口必须复杂到能支撑后续真实控件实现**，不是最小验证色块。  
> **对标：** R0 `ui_wr_r0_fullpaint` 为质量标杆（wrkit Shell + LiveHUD + PhaseClock + 多区域视觉 + 能力专用动画 + 详尽 Extra）。  
> **禁止：** 裸 box + 简单 sin 脉冲 + 无 HUD + 无 Legend 就宣称关闭。

#### 2.6.1 所有 R 窗口通用基线（必须）

| 项 | 要求 | 说明 |
|----|------|------|
| **wrkit.NewShell** | TopBar + Legend(≥5行色块) + Body + LiveHUD | 每个 R 必须有完整壳；禁止裸 root |
| **布局驱动（Flutter 对齐）** | 动态热点/能力元素定位一律走 **布局驱动**，禁止手算固定坐标 + clamp | 对标 R4：`shell.Body.Align(hot, ax, ay)`（`RenderAlignBox`，Flutter Align/FractionallySizedBox 语义——Layout 按 (父−子)×比例求 offset，resize 自动重算、无跳变）；相位切换用 `SetAlignment` 而非改坐标。**W2–W6 全部 R 窗口必用**（R4/R4b 已落地）；静态壳元素（色格阵/标签）可 `Place`。**内容不变：窗口展示的仍是该 R 能力自己的场景**（如 R7 虚拟列表、R10 异步图），本行只约束「定位手段」，不改变「能力展示内容」 |
| **LiveHUD** | 实时 fps/p95/policy/paint/presents/core | `wrkit.NewLiveHUD` + `hud.Update(Snap{...})` |
| **PhaseClock** | Steady→Spike→Recover 三阶段 | 能力行为随阶段变化（不只是动画频率） |
| **EnsureUIFace** | 字体加载 + 文字标签可见 | `wrkit.EnsureUIFace()` 在 buildScene 前调用 |
| **多区域 Panel** | TopBar/Legend/能力区/HUD 四区以上 | `wrkit.NewPanel` 布局，禁止两个裸 box |
| **Legend 内容** | 色块 + 文字说明能力含义、颜色编码、预期行为 | ≥5 行，每行有 ColorAt + LabelAt |
| **Extra 描述** | `impl_correctness`/`impl_dirty`/`impl_cache`/`impl_edge`/`impl_fail`/`impl_visible` | 详尽描述实现策略、边界、失败条件 |
| **wrgate.BuildReport** | §2.2 全族 A–J JSON | 通过 `wrgate.BuildReport` 自动填充 |
| **wrgate.EvaluateGates** | 能力专用门禁 | 每个 R 有独立的 GateOptions + 自定义检查 |
| **1200×800 + RUN_SECONDS≥5** | U15/U16 | `wrkit.RunSeconds` + `wrkit.RequireMinRun` |

#### 2.6.2 各 R 窗口复杂度要求（支撑控件）

| R | 包名 | **必须包含的视觉内容** | **必须包含的交互/动画** | **必须证明的边界** | **控件支撑意义** |
|---|------|----------------------|------------------------|-------------------|-----------------|
| **R0** | `ui_wr_r0_fullpaint` | 密集 4×4 静态色块网格 + ≥8 文字标签 + 热脉冲 box + 侧边静态条 | PhaseClock 驱动脉冲速率变化（Spike 加速） | 全树 Paint 下静态内容不被 Clear 丢弃 | 证明任意复杂静态控件树每帧存活 |
| **R1** | `ui_wr_r1_layout` | 深层嵌套容器树（≥5层）+ 多个不同尺寸子节点 + 约束传递可视化 | 定时改变单个子节点高度/宽度 → 只触发子树 Layout | `layout_count` 只脏子树，邻域不抖 | 证明控件尺寸变更不触发全局重布局 |
| **R2** | `ui_wr_r2_paint` | 多个 RepaintBoundary 区域（≥3个）+ 不同颜色/内容类型（色块+文字+渐变） | 多个独立热点以不同频率 MarkNeedsPaint | 静态 boundary 的 `NeedsPaint()` 始终为 false | 证明控件局部重绘不影响兄弟节点 |
| **R3** | `ui_wr_r3_boundary` | 多级嵌套 boundary（≥3层）+ 各层有文字/图形混排内容 + 脏热区 | 外层脏→只外层 rerecord；内层脏→只内层 rerecord | `boundary_skip>0`（干净层 Replay）+ `boundary_rerecord>0`（脏层重录） | 证明嵌套控件缓存独立，内脏不外溢 |
| **R3b** | `ui_wr_r3b_compbits` | 嵌套 boundary 链（≥3层）+ 各层 NeedsCompositing 状态可视化 | 定时触发不同层级脏 → 验证合成链深度 | `boundary_count≥3` + `boundary_max_depth≥2` + `NeedsCompositing` 正确传播 | 证明控件树合成位发现正确 |
| **R4** | `ui_wr_r4_composite` | retained 模式：多 boundary 区域 + 局部动画 + 大面积静态背景 | 动画区持续变化，静态区不重绘 | `damage_ratio≤0.35`（远小于全屏）+ `present_mode` 非 full | 证明控件场景下 retained 合成省绘 |
| **R4b** | `ui_wr_r4b_multidamage` | 两个远离脏点（左上+右下）+ 中间大面积静态 + 不同内容类型 | 两点独立脏，中间不脏 | `dirty_layer_id_max≥2` + `damage_multi_frames≥1` | 证明多脏区独立 scissor，中间区域不重绘 |
| **R5** | `ui_wr_r5_picture` | 复杂 Picture（路径填充/描边 + 文字 + 色块 + 变换）+ 直绘 vs 回放对比区 | 每帧 Replay 回放 vs 直绘并排 | `picture_op_count≥3` + 回放像素 ≡ 直绘 | 证明 Picture 显示列表完全等价于直绘 |
| **R6** | `ui_wr_r6_layer_anim` | Opacity 淡入淡出 + Transform 旋转/缩放 + Clip 裁剪动画 + 大面积静态背景 | 三种层动画同时运行，PhaseClock 控制节奏 | `fps≥55` + `hitch_rate` 合理 + 静背景不闪 | 证明控件层动画不影响静态缓存 |
| **R7** | `ui_wr_r7_virtlist` | 1000+ 项虚拟列表 + 异构 item（文字行+色块+图片混排）+ 滚动条指示器 | 快速滚动 + 惯性滚动 + 变高行 | `bind_count≪item_count`（如 1000 项只绑 30）+ RSS slope 合理 | 证明控件长列表虚拟化正确 |
| **R7b** | `ui_wr_r7b_scroll_reuse` | 滚动复用场景：大量 cell + 滚动时静态 cell 不重录 | 持续滚动 60s | `scroll_rerecord` 有上限 + `fps≥55` | 证明滚动时控件 cell Picture 缓存复用 |
| **R8** | `ui_wr_r8_overlay` | 复杂主树内容 + 多层浮层叠加 + 浮层内有交互元素 | 开/关浮层 → 主树 `paint_count` 不涨 | 开 overlay 后主树不重绘 | 证明弹窗/菜单/Tooltip 不触发底层重绘 |
| **R9** | `ui_wr_r9_text_cache` | 多种文字样式（字号/颜色/粗细）+ 重复布局触发 | 每 tick 强制 `MarkNeedsLayout` 但文字不变 | `measure_cache_hit≥1` + `hits≥miss` | 证明控件文字度量缓存有效 |
| **R10** | `ui_wr_r10_async_image` | 多个异步图片占位符 + 图片加载完成切换 + 局部脏区 | 图片逐个加载 → 每次只脏一格 | 出图后 rerecord 仅一格 | 证明控件图片加载不触发全局重绘 |
| **R11** | `ui_wr_r11_dpr` | 多个 boundary 区域 + DPR/尺寸变更 + 缓存失效可视化 | 两次尺寸变更（~3s 和 ~6s）→ rerecord 波 → 回到 skip | `cache_invalidations≥1` + `boundary_rerecord≥1` + `boundary_skip≥1` | 证明控件 DPR 变更后缓存正确重建 |
| **R12** | `ui_wr_r12_metrics` | §2.2 全族字段 schema 展示 + 字段存在性验证 | N/A（schema_only） | 所有 RequiredSchemaKeys 存在 | 证明指标采集完备 |
| **R12b** | `ui_wr_r12b_debug_repaint` | 多个区域 + 可开关 debug 重绘色（magenta 叠加） | `DEBUG_REPAINT=1` → 脏区域闪烁；`0` → 无叠加 | debug on → `draws≥1`；off → `draws==0` | 证明控件重绘可视化可用 |
| **R13** | `ui_wr_r13_hit` | 多个不同颜色/形状的 hit 目标 + 空白区域 + 变换/裁剪下的目标 | 脚本化点击探针（≥4个）+ 实时指针点击 | `scripted_ok == scripted_total`（全部命中） | 证明控件命中测试 ≡ 绘制身份 |
| **R14** | `ui_wr_r14_cache_budget` | 大量 boundary 区域 + 持续滚动产生新缓存 + 超预算淘汰可视化 | 长时间滚动 → 缓存超预算 → 淘汰 | `cache_entries` 有上限 + `RSS slope` 不失控 | 证明控件缓存淘汰机制正确 |
| **R15** | `ui_wr_r15_soak` | 全主路径组合（静态+动画+滚动+浮层）+ 长跑 300s | 持续运行，PhaseClock 周期变化 | 无崩 + `hitch_rate` 合理 + CPU/RSS 稳定 | 证明控件长时间运行不泄漏不崩溃 |
| **R16** | `ui_wr_r16_warmup` | 首帧可见内容（非黑屏）+ 渐入动画 | `WarmUp: true` → 首帧同步 Present | `warmup=true` + `first_present<2000ms` | 证明控件首帧有内容 |
| **R17** | `ui_wr_r17_bg_throttle` | 前台正常内容 + 后台降频验证 | 窗口最小化/不可见 → interval 明显变大 | 后台 `interval` > 前台 2x | 证明控件后台不浪费 CPU |
| **R18** | `ui_wr_r18_savelayer` | SaveLayer 半透明组 + 超预算拒批可视化 | 两个 SaveLayer：第一个允许，第二个拒批 | `savelayer_allow≥1` + `savelayer_reject≥1` | 证明控件离屏合成走预算 |
| **R19** | `ui_wr_r19_snap` | 1px 线条网格 + 不同 DPR 下的清晰度对比 | DPR 变更 → 线条清晰度验证 | 约定 scale 下 1px 线不糊 | 证明控件 1px 边框/分割线清晰 |
| **R20** | `ui_wr_r20_filter` | 模糊/灰度滤镜子树 + 外部不变内容 | 滤镜范围变化 → 只脏子树 | 局部 rerecord，外不变 | 证明控件滤镜效果局部化 |
| **R21** | `ui_wr_r21_shell` | 顶栏（标题+按钮）+ 可滚动体内容 | 体内容滚动 → 顶栏 `rerecord=0` | 顶栏 Picture 缓存不被滚动触发 | 证明控件壳/内容分层正确 |
| **R22** | `ui_wr_r22_selection_stub` | 选区/光标 stub API + 防全窗刷预留 | stub 空跑 | API 存在 + 字段可 0 | 证明控件选区 API 预留就绪 |

#### 2.6.3 质量检查清单（每个 R 窗口提交前）

```text
□ wrkit.NewShell 完整壳（TopBar + Legend + Body + LiveHUD）
□ 动态热点/能力元素用布局驱动（Panel.Align + SetAlignment，Flutter 对齐；禁止固定坐标+clamp）
□ PhaseClock 三阶段驱动能力行为变化
□ EnsureUIFace 字体加载 + 文字标签可见
□ ≥5 行 Legend（色块 + 文字说明）
□ 多区域 Panel 布局（≥4 区）
□ 能力专用动画/交互（不是简单 sin）
□ 详尽 Extra（impl_correctness/dirty/cache/edge/fail/visible）
□ wrgate.BuildReport 输出 §2.2 全族 A–J
□ wrgate.EvaluateGates + 能力专用自定义检查
□ 1200×800 + RUN_SECONDS≥5 + wrkit.RequireMinRun
□ 边界情况验证（脏传播止步、缓存命中/失效、预算拒批等）
□ 控件支撑意义明确（Extra 中记录）
```

### 2.3 作者纪律

视觉→Paint；尺寸→Layout；热点→Boundary；长列表→VirtualList；OnPaint 禁 IO/改树；动画优先层；浮层→Overlay；海量图→独立层；SaveLayer 走预算。

> **API 维护义务（硬）：** 任何涉及 `render/` 公开 API 的改动（增/改/删符号、改签名、变接线状态）都必须**同步更新 `docs/RENDER_API_CATALOG.md`**（该文档是 render 公开 API 总账，含状态表），并在 §10 修订表追加一行记录。禁止只改代码不更新 API 目录。

### 2.4 非主能力（明确不做进 §R 关闭）

Kit 组件、IME 实现、a11y 桥、多窗产品、系统托盘/菜单深做、剪贴板/拖放产品、RTL 产品、母表几何 API 清零。

---

## 3. 组合真窗口测试（多主能力 · 不替代 §2）

> **规则（U6 + U15 + U16）：** 组合窗同样 **1200×800**、**`RUN_SECONDS≥5`**；**关闭用时长 = max(所覆盖各 R 的关闭用)**（见 §2.5 组合表）。  
> 只做集成回归；**不能** 用组合窗代替单能力窗关闭 R。

| 组合 ID | 覆盖的主能力（至少） | 真窗包名 | **窗口** | **推荐 RUN_SECONDS** | 要证明的集成效果 | 指标要点 | 波次 |
|---------|----------------------|----------|----------|----------------------|------------------|----------|------|
| **C0** | R0+R12+R16 | `ui_wr_c0_smoke` | **1200×800** | **5** | 开机即见静+动；指标字段齐（**集成**；**不能**代替 R0/R12/R16 单窗） | policy、presents、首帧 | **W0** ✅ |
| **C1** | R2+R3+R3b+R12b | `ui_wr_c1_boundary_nest` | **1200×800** | **10** | 嵌套 boundary + 可开关 debug 重绘色 | rerecord/skip/boundary_count | **W1** ✅ |
| **C2** | R3+R4+R4b+R5 | `ui_wr_c2_retained_scene` | **1200×800** | **15** | Retained 整场景：多 boundary + Picture | damage_ratio、dirty_layers | **W2** ⬜ |
| **C3** | R4+R7+R7b+R10 | `ui_wr_c3_list_scroll` | **1200×800** | **60** | 虚拟列表 + 滚复用 + 异步图格 | bind、scroll_rerecord、p95 | **W3** ⬜ |
| **C4** | R3+R8+R21 | `ui_wr_c4_shell_overlay` | **1200×800** | **15** | 顶栏静 + 体内容 + 浮层面板 | 顶栏 rerecord=0；开 overlay 主 paint | **W4** |
| **C5** | R6+R3+R4 | `ui_wr_c5_anim_over_static` | **1200×800** | **30** | 层动画盖在静态缓存上 | hitch；静不闪 | **W5** |
| **C6** | R18+R3 | `ui_wr_c6_savelayer_group` | **1200×800** | **10** | 离屏组 + boundary | savelayer_* | W2/W5 |
| **C7** | R11+R19+R3 | `ui_wr_c7_resize_dpr` | **1200×800** | **15** | 改尺寸/DPR 后缓存与 1px 线 | 一波 rerecord；线清晰 | **W2** ⬜ 组合窗 |
| **C8** | R13+R6+R8 | `ui_wr_c8_hit_overlay_xf` | **1200×800** | **15** | 变换/浮层下命中 | 命中 ID | W4+ |
| **C9** | R14+R3+R7 | `ui_wr_c9_stress_cache` | **1200×800** | **60** | 多 boundary + 列表压缓存 | cache_entries、RSS | **W6** |
| **C10** | R15+全主路径 | `ui_wr_c10_soak` | **1200×800** | **300** | 长跑组合 | 无崩；hitch 可报 | W2+ |
| **C11** | R0→R4 策略切换 | `ui_wr_c11_policy_switch` | **1200×800** | **15** | full_paint↔retained 切换正确 | policy 字段；切换后静不丢 | **W6** |

### 3.1 每个 C 窗口复杂度标准（与 R 同级 · 硬）

> **原则：C 窗口是多主能力的集成压测**，必须比单 R 窗口更复杂 — 多能力同时在线、相互作用、暴露跨能力边界问题。  
> **禁止：** 把多个 R 的色块拼在一起就宣称集成。必须有**能力间的交互场景**。

#### 3.1.1 所有 C 窗口通用基线（与 §2.6.1 一致 + 集成加强）

| 项 | 要求 | 说明 |
|----|------|------|
| **wrkit.NewShell** | TopBar + Legend(≥8行) + Body + LiveHUD | C 窗 Legend 要比 R 更多行（覆盖多能力说明） |
| **LiveHUD** | 实时 fps/p95/policy/paint/presents/core | core 字段展示所有覆盖能力的关键指标 |
| **PhaseClock** | Steady→Spike→Recover | 阶段变化必须同时影响多个能力（不只是一个） |
| **EnsureUIFace** | 字体加载 + 文字标签可见 | 同 §2.6.1 |
| **多区域 Panel** | ≥6 区（比 R 更多：每个能力至少一个专属区域） | 能力区域必须**共存同屏**，不是切换显示 |
| **Legend 内容** | 每个覆盖能力的色块+说明 + 集成效果说明 | ≥8 行 |
| **Extra 描述** | `covers` + 每个能力的集成验证 + `impl_interaction` | 必须描述能力间如何交互、为什么集成有意义 |
| **wrgate.BuildReport** | §2.2 全族 A–J JSON | 同 R |
| **wrgate.EvaluateGates** | 所有覆盖能力的门禁取并集 | C 的 GateOptions = ∪(各 R 门禁) |
| **1200×800 + RUN_SECONDS≥5** | U15/U16 | 同 R |

#### 3.1.2 各 C 窗口复杂度要求（集成压测）

| C | 包名 | **覆盖能力** | **必须包含的集成场景** | **必须同时在线的能力** | **必须证明的跨能力边界** | **控件集成意义** |
|---|------|-------------|----------------------|---------------------|----------------------|----------------|
| **C0** | `ui_wr_c0_smoke` | R0+R12+R16 | 密集静态网格 + 热脉冲 + 首帧 WarmUp + 指标 schema 验证 | FullPaint 静态存活 + 首帧有内容 + 字段齐 | 首帧内容不黑 + 指标全族存在 + 静态每帧存活 | 证明控件开机即完整可用 |
| **C1** | `ui_wr_c1_boundary_nest` | R2+R3+R3b+R12b | 多层嵌套 boundary（≥3层）+ 内层脏不外溢 + 外层脏不内传 + debug 重绘色可视化 | boundary skip/rerecord + NeedsPaint 隔离 + compositing bits + debug overlay | 内脏→只内 rerecord；外脏→只外 rerecord；debug 色只在脏区闪 | 证明控件嵌套缓存+脏隔离+调试可视化同时工作 |
| **C2** | `ui_wr_c2_retained_scene` | R3+R4+R4b+R5 | retained 模式下：多 boundary + Picture 回放 + 远离脏点 + damage present | boundary 缓存 + retained composite + multi-damage + Picture replay | damage ratio≪1 + dirty_layer_id≥2 + Picture 回放正确 + boundary skip>0 | 证明控件 retained 场景完整闭环 |
| **C3** | `ui_wr_c3_list_scroll` | R4+R7+R7b+R10 | 虚拟列表（1000+项）+ 滚动复用 + 异步图片格 + retained damage | VirtualList 绑定 + scroll reuse + async image + composite | bind≪N + scroll_rerecord 有上限 + 图片只脏一格 + damage≪全屏 | 证明控件长列表+图片+滚动+retained 共存 |
| **C4** | `ui_wr_c4_shell_overlay` | R3+R8+R21 | 顶栏（壳）+ 可滚动体内容 + 浮层面板叠加 | boundary 缓存 + overlay 独立合成 + 壳/内容分层 | 顶栏 rerecord=0 + 开 overlay 后主 paint 不涨 + 体滚不影响顶栏 | 证明控件壳+内容+浮层三层独立 |
| **C5** | `ui_wr_c5_anim_over_static` | R6+R3+R4 | 层动画（Opacity/Transform/Clip）盖在静态 boundary 缓存上 | 层动画 + boundary 缓存 + retained composite | 静背景不闪 + fps≥55 + hitch 合理 + damage 只在动画区 | 证明控件动画层不影响静态缓存 |
| **C6** | `ui_wr_c6_savelayer_group` | R18+R3 | SaveLayer 离屏组 + boundary 缓存 + 超预算拒批 | SaveLayer 预算 + boundary 缓存 | savelayer_allow≥1 + savelayer_reject≥1 + boundary skip>0 | 证明控件离屏合成+缓存+预算共存 |
| **C7** | `ui_wr_c7_resize_dpr` | R11+R19+R3 | 两次尺寸变更 + boundary 缓存失效/重建 + 1px 线清晰度 | DPR 缓存失效 + boundary rerecord/snap + 1px 对齐 | 一波 rerecord 后回 skip + 1px 线不糊 + 缓存正确重建 | 证明控件 DPR 变更后完整恢复 |
| **C8** | `ui_wr_c8_hit_overlay_xf` | R13+R6+R8 | 变换/裁剪下的 hit 目标 + 浮层叠加 + 层动画 | hit 测试 + overlay + transform | 变换下命中正确 + 浮层下命中正确 + 动画不影响命中 | 证明控件交互在变换+浮层下正确 |
| **C9** | `ui_wr_c9_stress_cache` | R14+R3+R7 | 大量 boundary + 虚拟列表滚动 + 缓存预算压测 | 缓存预算 + boundary 缓存 + VirtualList | cache_entries 有上限 + RSS slope 不失控 + bind≪N | 证明控件缓存+虚拟化压力下不泄漏 |
| **C10** | `ui_wr_c10_soak` | R15+全主路径 | 全能力组合长跑（静态+动画+滚动+浮层+图片）300s | 所有主能力同时在线 | 无崩 + hitch_rate 合理 + CPU/RSS 稳定 + 无渐进退化 | 证明控件全能力长跑不泄漏不崩溃 |
| **C11** | `ui_wr_c11_policy_switch` | R0→R4 | full_paint↔retained 策略切换 + 静态内容不丢失 | 策略切换 + 静态存活 + damage present | 切换后静态不丢 + policy 字段正确 + 切换无闪烁 | 证明控件策略热切换正确 |

#### 3.1.3 C 窗口质量检查清单

```text
□ §2.6.3 所有 R 基线项（Shell/LiveHUD/PhaseClock/字体/Legend/Panel/Extra/BuildReport/EvaluateGates）
□ Legend ≥8 行（每个覆盖能力有独立说明 + 集成效果说明）
□ ≥6 区 Panel 布局（每个能力至少一个专属区域）
□ 能力间交互场景（不是简单拼接）
□ PhaseClock 同时影响多个能力
□ Extra.covers 明确列出覆盖能力
□ Extra.impl_interaction 描述能力间交互方式
□ GateOptions = ∪(各 R 门禁取并集)
□ 集成边界验证（跨能力的脏传播/缓存/合成不互相干扰）
□ 控件集成意义明确（Extra 中记录）
```

---

## 5. 分期（W 关闭 = 单窗全集 + 组合窗）

| W | 状态 | 必须绿的 **单能力窗** | 必须绿的 **组合窗** |
|---|------|----------------------|---------------------|
| **W0** | **✅** | **R0✅ · R12✅ · R16✅**（各独立 `ui_wr_*` 真窗） | C0✅（仅集成） |
| **W1** | **✅** | R2✅ R3✅ R3b✅ R5✅ R9✅ R12b✅ R19✅（各独立 `ui_wr_*` 真窗） | C1✅ |
| **W2** | ⬜ | **R4✅ R4b✅ R5✅v2 R11✅ R13✅ R18✅** · R21(可) R19(可) | **C2⬜ C7⬜ 组合窗** |
| **W3** | **⬜** | R7⬜ · R7b⬜ · R10⬜（各独立 `ui_wr_*` 真窗） | C3⬜ |
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
| Q2 | R12 与每个窗的指标重复 | **R12 必须有独立真窗**测「字段 schema」；各窗仍测业务门禁；C0 只做集成 |
| Q3 | 无显示 CI | `needs_gpu_window`；禁止假绿 |

---

## 10. 修订

| 版本 | 说明 |
|------|------|
| 默认后端=当前会话后端 + Wayland 手动 resize | **①默认后端改为当前系统实际后端（ui/platform/detect_linux.go）**：`DetectDisplayBackend` 由「DISPLAY 存在即 X11 优先」改为 **WAYLAND_DISPLAY 存在即 Wayland 优先**（GNOME/KDE Wayland 会话同时导出 DISPLAY=Xwayland，X11 即 Xwayland，带交换链重配卡顿/无 sync-request resize 的局限；原生 Wayland 才是实际合成器）。验证（2026-08-18）：`tmp_resize_diag` Auto 路径 `opened backend=wayland`。**②Wayland 手动调整窗口大小支持**：链路（CSD 8 向手柄 hitTest → `xdg_toplevel.resize` → 合成器 configure → `EventResize` → 重排+呈现）已完整，补齐两块——**(a) `render/present_target.go` applyPendingSwapchainLocked 按平台分流**：X11/Xwayland 维持延迟重配（BeginFrame outdated-retry，风暴 +70%）；**Wayland 等平台恢复同步重配**（wgpu 表面尺寸变化**不报 outdated**，延迟重配会导致交换链永远卡在旧尺寸——实测 retry-only 下 Wayland sc.frame 恒 400x400、内容错位；原生 Wayland Configure 仅 ~1–5ms，同步无妨）；**(b) `ui/platform/wayland_linux.go` 跟踪 `resizing` configure 状态**（`wlTopConfigure` 解码后原本丢弃，现写入 `wlWin.resizing`，供应用/测试确认交互式 resize 握手）。验证（NVIDIA Vulkan，2026-08-18）：Wayland 初始 configure 400→408×436（CSD 边框）、1600×1000→1868×1053（Mutter 贴合工作区）均 EventResize→同步重配→sc.frame 跟随（323/连续帧）；`TestWaylandRealWindowOpen/Events/Controls` PASS；render `TestPresentResize_*` PASS；X11 风暴 fps 12.5/10.0/10.0（延迟路径保持 +70%）；`wayland_interact_test.go` 的 `w.resizing` 编译依赖已满足。**残余**：真·鼠标拖拽握手（合成器接受 `xdg_toplevel.resize`）需真实输入，本环境无注入权限（uinput 仅 root），未能自动化端到端验证——需人工拖拽确认。 |
| resize拖拽延迟（swapchain 重配延迟化） | **拖拽 resize 期间"内容不跟手/越大越卡"的可测上限再提升（引擎优化，`tmp_resize_diag` DIAG_STORM=1）**：**机制实测**——风暴期每帧栅格耗时 30–390ms（峰值 2.9s）而稳态仅 1–8ms；相位计时定位：acquire 失败瞬时（0ms，"surface outdated"），**wgpu `Surface.Configure`（X11/Xwayland+NVIDIA）本身 26–383ms 且随尺寸超线性增长**（520²→33ms、1120×1016→86ms、1440×1016→280ms、1560×1016→383ms），重配期间窗口继续移动 → 重配完的 swapchain 立刻又过期 → 重试 acquire 再次 outdated → 帧丢弃 → 拖拽呈现被钉死在 3–15fps，窗口越大越差（用户观感"越不跟手"）。此前同步路径（`applyPendingSwapchainLocked` 直接 `sc.Resize`）每步都撞这个 Configure；"先 DiscardTexture 再重配"实测无效（已回滚）。**修复（对齐 Skia 的 acquire 失败重建语义，X11/Xwayland 路径）**：`applyPendingSwapchainLocked` 改 **record-only**（不再同步 Configure），交换链重配**延迟到 BeginFrame 的 outdated-retry**（失败 acquire → 探活窗尺寸 → Configure → 重试 acquire），已存在的重试路径天然带 DiscardTexture；postResizeFull=3 全写预算与风暴窗口保留（重配那帧 + 后续 2 帧写满 3 缓冲，全写语义不变）；**Wayland 等其他平台仍走同步重配**（wgpu 表面尺寸变化不报 outdated，见「默认后端=当前会话后端」行）。**验证（GPU NVIDIA Vulkan 580.173.02，2026-08-18）**：风暴 fps 均值 6.9→11.7（**+70%**，11.4/6.7/8.0/1.5 → 14.4/9.0/12.0/11.5）；稳态 60fps 不受影响；`TestPresentResize_FullRecoveryWritesEveryBuffer`/`StormWindowCoversBudgetGap` PASS；全屏启动抓帧 fill=OK；R4 组合窗 1200×800 fill=OK。**残余上限=环境**：Xwayland 下 Vulkan swapchain 重配 26–383ms 无法从框架侧消除（Flutter/GTK 用 EGL 窗口面、尺寸跟随窗口无需重配，是原生丝滑的架构差异）；同一代码在原生 Wayland/Xorg 上重配约 1–5ms。 |
| resize启动尺寸同步（tmp_resize_diag） | **窗口打开即被 WM 改尺寸（最大化/贴合工作区）时渲染内容与窗口不一致修复（引擎洞，`tmp_resize_diag` DIAG_FULLSCREEN=1 复现：请求 1600×1000，GNOME 地图时最大化到 1868×1016）**：根因=**启动期尺寸缺口**——X11 `x11Create` 在 `XMapWindow`+`XFlush` 后直接以**请求尺寸**初始化 `st.w/st.h`，而 WM 在 map 期间的调整只发 ConfigureNotify、若在应用首个 WaitEvents 之前到达则被起始事件批次吞掉；尺寸一旦在探针侧先行置位，`setSize` 又报"无变化"不发 `EventResize` → host/swapchain/viewport 均为实际尺寸而**场景树停在初始布局**（实测 1868×1016 窗口内容 1600×1000：标题栏右缘 1599、状态栏 999，整会话错位，表现为"部分图形不跟随窗口"）。**修复（对齐 Flutter/Skia 启动初始 metrics 语义）**：`ui/platform/x11_linux.go`——①`x11Create` map 后先排空 map 噪声（50ms），创建 `st` 后**自适应等待客户端几何稳定**（20ms 采样、双采样一致即止、500ms 上限，期间继续排空 ConfigureNotify 防陈旧批次），`x11GetGeometry` 探测**实际客户端尺寸**写入 `st.w/st.h`；②尺寸异于请求时置 `initResizePending`，`drainX` 首个 drain 投递一次 `EventResize(实际尺寸)`（`setSize` 对已置位尺寸报"无变化"，正常 ConfigureNotify 路径不会补发）→ 应用随首个事件循环批次 relayout 到实际尺寸（`pipeline_app.go` 帧边界读 host 尺寸，swapchain==内容==窗口）。**验证（GPU NVIDIA Vulkan，2026-08-18）**：DIAG_FULLSCREEN=1 全屏运行 xwd 抓帧标题栏右缘 1867/1868、状态栏底 1015/1016（此前 1599/999），`EventResize 1868x1016` t=0.4s 送达、18s 内 1090 presents 无 BeginFrame 错误；风暴回归 fps 7.0–9.5 无回归；R4 组合窗 1200×800 正常（title_right=1199；WM 未改尺寸时走原路径、无 `initResizePending`）；`go build ./ui/platform` 绿（platform 测试包此前被另一线未提交 `wayland_interact_test.go` 的 `w.resizing` 编译错误阻断，该字段已在「默认后端=当前会话后端」行补齐）。 |
| resize渲染同步（tmp_resize_diag） | **拖动窗口大小时渲染内容不跟随窗口同步（`tmp_resize_diag` 复现，引擎洞修复）**：**根因 4 层**——①**swapchain Configure 在 UI 线程**（render/present_target.go `Resize()` 直接 `sc.Resize`→wgpu `Surface.Configure`，llvmpipe 每次 ~40–100ms，resize 风暴时 UI 线程每帧被它阻塞）；②**UI 线程 Configure 与 raster 线程 present 竞态**（"SurfaceOutput must be dropped" 语义，raster 前一帧未 EndFrame 时 UI 的 Configure 失败、错误被 `_ =` 吞掉 → swapchain 卡死在旧尺寸，实测风暴中 1560×1016 定格 415 帧）；③**内容尺寸与 swapchain 尺寸不一致**（内容按 `host.Size()` 布局、swapchain 按事件尺寸配置，拖动期窗口永远超前 → 每次 acquire 报 "surface outdated" → 整帧丢弃 → 拖动期几乎零呈现 → 内容冻结在旧尺寸）；④**X11 尺寸探针潜藏 null deref**（`XGetGeometry` root_return 传 0，Xlib 无条件写 → 地址 0 崩溃，ui/platform 同款潜伏）。**修复（对齐 Skia/Flutter：raster 线程拥有 surface）**——①⑦ `present_target.go`：`Resize` 改 **record-only**（逻辑尺寸+dc+`swapchainPending` 标记，无 sc.Resize），`present()` 在 raster 线程 `applyPendingSwapchainLocked` 应用（与 BeginFrame/EndFrame 经 mu 串行，无 UI 阻塞、无竞态；`InFullRecovery` 纳入 `swapchainPending` 保持全写语义）；②⑦ `pipeline_app.go`：帧边界改读**当前 host 尺寸**（与内容布局/包构建同源）应用 Resize + 强制 relayout——swapchain 尺寸==内容尺寸==窗口；③⑦ `gpu/webgpu`：`Surface.WindowSize()`（X11 XGetGeometry，XLockDisplay 包裹）+ BeginFrame 的 outdated 重试先探活窗尺寸再 Configure（重试 acquire 命中当前窗口，不再循环丢弃）；④⑦ `NotifyFrameDrawn`（X11 `_NET_WM_SYNC_REQUEST` 计数器）从 submit 改到 **present 完成后**推进（raster job 内，err==nil 时）——合成器只在真帧落幕后才解除拉伸；⑤⑦ `x11GetGeometry`（ui/platform）root_return 改 &rootRet 修复潜伏崩溃；⑥ tmp_resize_diag：`Resize()` 补 `status.PlaceOn(root,0,h-statusH)`（status bar 原来钉死在初始 400×400 的 y=370，窗口越大越明显“不跟随”）。**验证（GPU llvmpipe，2026-08-18）**：风暴 45s（400↔1600，40px/12ms）presented 尺寸 **96% 帧在 ±1 step（40px）内跟踪窗口**（旧：swapchain 冻结 1560 定格、outdated 全丢弃、内容冻结）；storm fps 3.5–7→4–9.4（余量为 llvmpipe 全屏矢量+MSAA 固有的渲染成本，非编排问题）；稳态 60fps 不受影响；R4 保留真窗 3 连跑 55.1/55.1/55.4（基线 55.0–55.2 **零回归**）；render `TestPresentResize_*` 隔离 PASS；gpu/webgpu 仅既有 TestS68 失败（stash 基线一致）；apidoc 绿。 |
| CFF 正向优化（bounds 缓存） | **CFF/CFF2 字形 bounds 缓存（正向优化，续 CFF 解析缓存）**：命中态 pprof 显示 `glyphBoundsCFF` 每次做完整 CFF 轮廓提取（`extractCFFOutline`→`sf.LoadGlyph`+`fdSelect.lookup`，CID 字体 FDSelect 查找贵），且 `face.Glyphs`（shaping）每串每字形都调 → 修复前 noto CFF 命中态 0.28ms/串 vs glyf 0.06ms/串。修复=`ownParsedFont` 新增按 (gid, sizeQ4) 的 `cffBounds` 缓存（map + mutex，上限 4096 溢出重建），`glyphBoundsCFF` 命中直接返回，仅首次提取轮廓。**验证（2026-08-17）**：noto CFF 命中态 60 串 16.9→5.2ms（与 glyf 5.4ms 持平，0.087ms/串）；CFF/CFF2 outline+bounds 测试、TestGlyphBounds 全绿（缓存前后一致）；ui 三包 + render/text + hint 全绿；sweep 84/84；R4 真窗 fps≈59.9、cpu_raster 24.4%、skip=52982 全绿。 |
| CFF 解析缓存 + UI 默认思源黑体 SC | **CFF/CFF2 字体 light 光栅性能根因修复 + 示例默认字体切为思源黑体 SC（微软雅黑风格）**：① 性能根因（用户报告 CJK 优先链 R4 真窗 fps 29–51 崩、raster 1 秒/帧、CPU 96%）：`render/text/hint` `LightHintVar` 每次光栅一个 CFF 字形都执行 `cffParseAll` 全表解析（pprof `readIndex` 63-100% 热点），且 glyph atlas 对空格等空字形不缓存 → 每串动态文本（HUD）都穿透到全表解析（每串 4-16ms/首串 120ms）；glyf 字体有 hint 缓存所以不受影响。修复=`hint/cff_parse_cache.go` 新增 per-font CFF/CFF2 表解析缓存（key=raw 数据指针+faceIdx，LRU 上限 8 防 RSS 膨胀），`LightHintVar` 改走 `cffParseCached/cff2ParseCached`（同一 FontSource 每字体只解析一次，空字形光栅从 ~5ms 降至 µs）；2 新单测（同字体复用同一解析+与直解析一致 / 解析错误不缓存）。② 示例默认字体：`wrkit` UI 链改为**思源黑体（Noto Sans CJK SC）优先**——系统 `NotoSansCJK-Regular.ttc` 默认集合索引 0 是 JP（日式字形），`loadUIChain` 用 `SetSystemFontPaths(FontRoleCJK, 用户目录 SC OTF, 系统 TTC)` + `LoadDefaultFaceFor(CJK 优先链)`（雅黑风格黑体 + 简体字形，Latin 也走思源西文）。**验证（2026-08-17）**：SC 链 R4 真窗 15s GPU 门禁全绿（fps≈59.8、首帧 36ms（原 823ms）、cpu_raster 27.7%、skip=52923、mode=damage_multi）；fbench 相位扫描 CFF 30 串 226→17.8ms / round2 164→10ms；hint 包 light_api/psh_light/cffcs/TestScanCJK3000（3000 字 × 6 字号 vs ftexp）/M2 verify 全绿；ui/scene|rendering|embedder + render/text 相关测试全绿；sweep 84/84 + 系统 6 字体 × 12 字号 0 失败；build/vet/apidoc 绿。 |
| render/text hinting 存量修复 | **render/text hinting 存量不一致修复（UI 文字对齐生产 light 模式）**：存量问题=① `wrkit.FaceAt` 注释意图 HintingNone（R21 时期，当时 light 未实现），但 `MultiFace.Source()` 恒 nil 使 `src.Face(points, WithHinting(None))` 分支永不执行 → UI 文字实际以 face 默认 **HintingFull** 渲染；② `MultiFace.AtSize` 重建成员 face 时**丢弃全部渲染选项**（`src.Face(size)` 无 opts）→ 即使显式设置了 hinting 也在一轮字号重导后回到引擎默认 Full。修复（render/text 层 + wrkit 验证使用）：① `AtSize` 改 `src.Face(size, faceOptionsOf(f)...)`——保留 direction/hinting/features/variations/language；② 新增导出方法 `MultiFace.WithHinting(h)`（按当前尺寸重建全部成员并钉死 hinting）；③ `wrkit.FaceAt` 改用 `WithHinting(HintingVertical)`（= **FT light**，Skia slight 等价语义，生产 GPU/CPU 单一配置）＋`AtSize(points)`，注释同步更新；④ 新增 3 单测（AtSize 保留 hinting / WithHinting / 链式）。**验证（2026-08-17）**：`FaceAt` 8–48pt 全字号产出 HintingVertical；覆盖扫描 84/84 逐像素一致 + 系统 6 字体 × 12 字号 0 失败（含 uming@8pt 残余——Full hinting 下 11px 差异在 light 下归零）；ui/scene|rendering|embedder + render/text multi/face/options 测试全绿；R4 真窗 15s GPU 门禁全绿（fps≈59.9、dmg_avg=0.058、skip=52982、mode=damage_multi）。相关文档：`ENGINE_TEXT_HINT_LIGHT_PLAN.md` §2 例窗行更新；RENDER_API_CATALOG.md text 族补 `WithHinting`。 |
| R4 小字号文本缺笔划 | **R4 真窗左侧 legend 小字号中文顶部横笔划缺失修复（live 纹理合成路径）**：用户报告「局/屏/重/景/损」等字顶部横笔划不完整、PNG 快照正常 → 差异在 retained 纹理路径（`CompositeFramePacketTextured` 把文字层录进 bounds 大小本地纹理再 blit）vs 直绘快照。根因=ui/scene `PictureTextureCache.measureTextBounds` 用字体指标/advance 估算文字层边界，而实际墨迹超出估算：① **CJK 墨迹比 MultiFace 首个字体（DejaVu）ascent 高 ~1.6px**（局/屏/重/景/损 顶部横正好在估算界之上 1px）→ 本地纹理顶部整行被视口裁掉；② **引擎逐字推进是 `pen += round(adv)`（设备像素网格 snap）**，估算用未取整 pen，长串末字墨迹右移 ~5px（12pt 下每个数字 7.64→8），右缘 AA 列被裁。修复=`measureTextBounds` 重构：按 MultiFace run 用各 run 字体自身度量做下限 + 逐字形 glyf/CFF 墨迹 bbox（Y-down、相对基线）+ **复刻引擎逐字 round 推进** + 2px 边距（对齐 Flutter per-run 行高 / Skia glyph bounds+1）。**验证（GPU llvmpipe，2026-08-17）**：覆盖扫描 84/84 逐像素一致（7 类文本 × 8–48pt：混合拉丁+CJK、CJK 顶部横字、纯 CJK、拉丁下行/重音、泰文、天城文、数字，textured vs 直绘）**+ 系统 6 个额外字体 × 12 字号 0 失败**（AR PL UMing 明体/UKai 楷体/Liberation Sans/FreeSerif/Droid Sans Fallback/Ubuntu；唯一残余 uming@8pt 11px 已证明为 Full hinting 小字号 mask 撑宽所致——同字体换 HintingNone 即 0 失败，属既有 AtSize hinting 不一致项，未列入本次修复）；ui/scene|rendering|embedder 测试全绿（含 GPU 环境）；R4 真窗 15s GPU 门禁全绿（fps≈59.7、dmg_avg=0.058、skip=52864、mode=damage_union）。附带：examples/ui_wr_r4_composite 增 `SnapshotPath`（关闭时保存 GPU 回读 PNG 供像素验证，env `R4_SNAPSHOT` 可改、空禁用）。 |
| 示例裁剪修复 | **examples/render_clipping 图形2/图形5 裁剪框内未完全填充修复（示例层）**：根因=示例绘制范围小于裁剪框——图形2（ClipRect 300,50,160,160）格子循环仅 200×200（覆盖 x250-450/y0-200，裁剪框到 x460/y210，右下角 10px 白底）；图形5（Nested Clips 300,350,160,160）背景条纹 `y<200` 步进 5 高 3（覆盖到 y498，裁剪框到 y510，底部 12px 白底）。引擎裁剪本身正确（左/上超出均被正确裁掉，CPU/GPU 行为一致）。修复=循环范围扩大至覆盖裁剪框四周（图形2 `y<240`/`x<240`、图形5 `y<240`），绘制内容盖满裁剪框并超出。**验证（CPU+GPU，2026-08-16）**：修复后 CPU/GPU 渲染像素级一致（0 差异采样点）、图形2 裁剪框内白像素 0、图形4 无连续空白带。 |
| MSAA默认1x | **引擎默认 MSAA 从 4x 改为 1x（对齐 Skia kCoverage：analytic fringe 抗锯齿代替多采样）+ API 重命名**：原因=默认 4x 依赖 MSAA 离散覆盖率（4 级）产生斜线阶梯，且多采样开销高；analytic fringe（cover_aa）在 1 采样下提供连续覆盖率（与 CPU scanline 匹配）。改动=①`render/sample_count.go`：`SetDefaultSampleCount`/`DefaultSampleCount` 更名为 MSAA 标准名 `SetMSAASampleCount`/`MSAASampleCount`；`MSAASampleCount()` 未设置时直接返回 1（调用方无兜底）；②`gpu_shared.go`：`resolveSampleCount` 不再探测设备 4x，直接返回 `render.MSAASampleCount()`（默认 1）；external/standalone 分支统一走它；`SampleCount()` 默认 1；③同步全部调用方与测试断言（render/ui/examples）。**真机验收（llvmpipe，2026-08-15）**：默认 1x 下 `render_text_transform` 变换文本边缘过渡连续（GPU `186 46 31 46 185` vs CPU `196 40 30 40 193`）、无锯齿无毛边；变换格 GPU/CPU diff 0.94-1.78%；TestMSAASampleCount*/TestResolveSampleCount* 全绿；`go build ./...` OK。 |
| 路径AA对齐Skia | **stencil+cover 路径填充在任意采样数下叠加 analytic fringe（对齐 Skia kCoverage 连续覆盖率）+ CPU hairline 语义对齐**：原因=变换文本/图形路径在 4x MSAA 下只走二进制 cover，斜线边缘仅 4 级离散覆盖率 → 阶梯锯齿（CPU scanline 为连续覆盖率）；≤1px 细线 GPU 已按 hairline 1px、CPU 却按 0.5px 半透明。修复=①`gpu_render_context.go` `useAA` 与 `SampleCount()==1` 解耦（所有采样数生成 AA band 数据）；②`stencil_renderer.go` AA band/内带绘制条件去掉 `sampleCount==1`（MSAA 下也画 fringe，硬件 resolve + 连续边缘距离双保险）；③`stroke_user_expand.go` hairline 判定从 `width<=0` 改为"设备空间宽度 ≤1px"（Skia SkPaint lineWidth≤1→hairline 1px）；④`tessellate.go`+`cover_aa.wgsl` fringe 半宽 0.75→0.35px（0.75 在 4x MSAA 叠加下过渡过宽 → 毛边；0.35 与 CPU scanline 过渡宽一致，边缘硬朗）。**真机验收（llvmpipe，2026-08-15）**：`render_text_transform` 斜体/旋转文本斜线边缘过渡 `186 46 31 46 185` 与 CPU `196 40 30 40 193` 同模式（连续、窄过渡、无阶梯），字形形态与 CPU 逐行一致；0.5px 十字线 GPU hairline snap 实线 vs CPU AA 淡线（Skia CPU/GPU 各自语义）；render/gpu/ui 回归无新增失败；`go build ./...` OK。 |
| 变换文本质量门禁 | **render_text_transform 示例 GPU 变换文本质量修复（对齐 Skia：变换文本走矢量路径）**：原因=TextModeAuto 下旋转/切变/非均匀缩放的文本仍走 MSDF（固定 pxRange=4@64px 参考），边缘过渡带随放大线性变宽（24px 显示≈1.5px，×3 放大≈4.5px≈笔画半宽）→ 字形膨胀糊实、斜体重影；CPU 侧同场景走 Tier2 矢量 outline 故正确。修复=`selectTextStrategy()` 加变换质量门禁 `needsOutlineTransform()`（`B≠0∨D≠0`=旋转/切变，`|A|≠|E|`=非均匀缩放）→ 返回 `TextModeVector`（`drawStringAsOutlines`→`doFill`→GPU stencil+cover 或 CPU Tier2，与 CPU 完全同路径）；显式 textMode/forceTextMode 不受影响；轴对齐均匀缩放/平移/identity 维持 glyph-mask/MSDF 管线。**真机验收（llvmpipe，2026-08-15）**：`go run ./render/examples/render_text_transform`（GPU）第 5-9 格（Scale3,1/Rot30/Rot45/Shear/Scale+Rot）与 CPU diff 从 1.7-4.6% 降至 0.8-1.7%（剩余为 stencil vs scanline AA 算法差），字形从实心糊块恢复清晰线条；新测试 `TestTextTransformAutoRoutesOutlines`（GPU：auto==vector、auto≠MSDF）+ `TestNeedsOutlineTransform` 全绿；render/ui 文本族回归全绿（含 `TestTextModeAutoPreservesBehavior`/`TestTextTransformGolden`）。 |
| 洞4 计算管线裁剪 | **Vello 计算管线 GPU 裁剪集成（compute_clip GPU vs CPU 一致性）**：原因=GPU 侧缺 clip_leaf 阶段（EndClip draw monoid 修正）+ 带裁剪入口，着色器 coarse/fine 裁剪逻辑早已写好但从未被驱动。改动：①新增 `clip_leaf.wgsl`（CPU `clipLeafScan` 栈式配对 1:1 移植，单 workgroup 串行）；②`draw_leaf.wgsl` 写 ClipInp（BeginClip 正 path_ix/EndClip 补码）；③`vello_compute.go` 插 `VelloStageClipLeaf`（draw_leaf 后、coarse 前）+ ClipInp 缓冲 + 绑定/派发；④`vello_accelerator.go` 新增 `RenderSceneComputeDef`（收 `[]SceneElement`，`EncodeSceneDef` 编码，复用 `buildPathMetadata`，抽出共享 `dispatchSceneCore`）；⑤新增 `scene_auto.go` **自动切换入口 `RenderSceneAuto`**（一套场景代码，GPU 计算可用走 `gpu-compute`、否则回退 CPU 参考 `cpu-reference`，示例零分支）；⑥示例 `compute_clip` 改为一套场景一次调用，输出单图 `tmp/compute_clip.png`。**顺带修复 CPU 参考的样式索引错位**：`extractPathFillRules` 原按 `StyleBase+pathIx` 直取，EndClip 哑路径吃掉一个 path 索引却不产生样式 → EndClip 后的路径读错样式、末路径 even-odd 丢失（示例右星中心被非零填充）；改为按 draw tag 流 + draw monoid PathIx 重建映射（与 GPU `buildPathMetadata` 逐元素映射一致）。**真机验收（llvmpipe，2026-08-15）**：`TestVelloComputeClipGolden` 单/嵌套裁剪 0.00% diff（GPU==CPU）；`TestVelloComputeGolden` 7 项既有 golden 0.00%（HEAD 基线对照确认非回归）；`TestRenderSceneAuto` 双后端 0 差异；`go run ./render/examples/compute_clip` GPU/CPU 后端输出一致且诊断像素正确（两星 even-odd 空心）；宿主侧测试（阶段布局/绑定/元数据/回归 even-odd-after-EndClip）全绿；doc 见 `RENDER_2D_ALIGNMENT_PLAN.md` 洞4。 |
| 采样数配置 | **render 公开 API（render/sample_count.go）**：`SetMSAASampleCount(n)` / `MSAASampleCount()` + 常量 `MSAASampleCount1`(1x)/`MSAASampleCount4`(4x)。**引擎默认 1x MSAA**（边缘由 analytic fringe coverage 抗锯齿，Skia kCoverage 语义；窗口/离屏一致，CPU scanline 匹配）。**全局控制仅此 API（env `GPUI_SURFACE_SAMPLE_COUNT` 已移除，2026-08）**：Set(4)=显式 4x MSAA（窗口 chrome/高质量配置）、Set(0)=重置引擎默认（1x）；未设置时 `MSAASampleCount()` 直接返回 1（调用方无兜底分支）。特效离屏（SetEffectSurface）恒 1x 不受影响。单测 TestMSAASampleCount*/TestResolveSampleCount* 全绿；目录文档 §1/§9 已同步。 |
| 描边角点AA | **stencil cover 内带（interior band）尖角楔形修复 + 直边内带恢复（render/internal/gpu/tessellate.go，ui_render_graphics/basic 图形六描边三角两轮修复）**：第 1 轮用户报三闭合处空像素+刺尖；第 2 轮报三线条阶梯锯齿（沿三角形长直边）。根因=①1x analytic fringe 内带（Replace 覆写）在**尖角楔形**（峰顶 miter 50°/底角 65°、孔洞 V 49.8°、孔洞正交 90°）把本应满覆盖的像素 Replace 成 0.5-0.85 → 闭合处空心；②长直边 just-inside 若被内带正确渐变则与**其它图形（③线⑤三角走 convex 快路径）的双侧平滑过渡一致**，此前为对齐 CPU 参考把长直边内带整体屏蔽 → 直边退化为二进制内侧（0.37→1.0 硬跳）→ 阶梯感。修复=`TessellateAA` Pass 2 内带发射规则：**尖角短段（≤4px、转角命中 {40-60° 孔洞V / 85-95° 正交孔洞 / ≥110° 外 miter}）整段抑制内带**；**长直边内带恢复**（just-inside 部分覆盖，与 convex 路径同形态）；**尖角段与相邻短装饰段的长边端按 aa·(1+5/sin(φ/2)) 侵蚀**（含「装饰端侵蚀」：长边端部转角与 miter 装饰共线时为连带楔形按装饰尖角侵蚀）；内带重建保留原 ±aa 切向延伸（无侵蚀时几何不变）。眼睛尖端（75°）与圆帽弧（22.5°/段）不触发——保留内部渐变。**验证（2026-08-17）**：活窗 xwd 抓帧左边缘 41/47 行双侧平滑过渡（0.81→1.0→0.51 与 convex ③线 0.71→1.0→0.64 同形态）、峰顶楔形实心+AA 边、凹口 y=406 0.39、两底角满覆盖；`render_clipping` SavePNG 与 CPU 差异 12 像素（ex1/ex3 0；ex5 眼睛尖端 6 处 1px AA 差 + 6 处 ±1 取整）；单测 `TestTessellateAA_ConvexBandUnaffected`（内带存在断言）；render/internal/gpu 全量 PASS；顶 render 包失败集与基线一致；apidoc 绿。 |
| CPU模式真窗口 | **GOGPU_RENDER_MODE=cpu 接通真窗口呈现（render/accelerator.go + context.go + text.go + gpu_render_context.go + present_target.go，CPU 模式黑窗口修复）**：用户报 CPU 模式跑不出黑窗口。根因=`renderMode()` 只被 `AcceleratorCanRenderDirect()` 消费（示例调试打印用），运行时 doFill/doStroke/文字/窗口呈现全部照走 GPU 会话——CPU 光栅输出只在离屏/SNAP 路径生效，窗口内容 100% 依赖 GPU 渲染管线；用户机器 GPU 会话链路失败 → swapchain 空白 → 黑窗口。修复=让 CPU 模式真正生效：①新增公开 API `render.CPUOnlyMode()`（`GOGPU_RENDER_MODE=cpu` 判定）；②`doFill`/`doStroke` 分支加 CPU 模式门控（形状走 SoftwareRenderer）；③`selectTextStrategy` CPU 模式返回 TextModeBitmap（文字走 CPU）；④视图刷新（Flush pending==0 + view 非空 + CPUOnlyMode）走 `uploadPixmapToView`（CPU pixmap 直传 swapchain）——顺带修复其 256 字节行对齐 + 补空命令缓冲提交（WriteTexture 需 submit 才执行，否则 swapchain 呈现未初始化纹理仍黑屏）+ 命令缓冲 Release；⑤swapchain 纹理 usage 加 CopyDst。**验证（2026-08-17）**：CPU 模式活窗 xwd 抓帧与真 CPU 光栅快照**画布区 0 像素差异**（⑥ 粉色 944==944、棋盘格/峰顶楔形一致；此前窗口 = GPU 会话渲染，峰顶 0.45 vs CPU 1.0）；GPU 模式活窗不受影响（⑥ 954、楔形 0.45 保持）；render/internal/gpu 全量 PASS；顶 render 包失败集与基线一致（5 既有+S69 缺基线）；apidoc 绿（目录补 CPUOnlyMode）。 |
| API收敛 | **render 重复 API 收敛（按 §7.5 决定 + 扩展性评估）**：① `ClipRect`→委托 `ClipRectOp(Intersect)`（SkClipOp 默认，单一实现）；② `PathBuilder.Rect/Circle/Ellipse`→委托 `Path.Rectangle/Circle/Ellipse`（同实现；RoundRect 保留 skia 标准系数 k=0.5522847498）；③ `CustomBrush.Linear/Radial/Horizontal/VerticalGradient` 标 `Deprecated:`（数据式画刷取代，零生产消费者）；④ `Set*` 注释标注为 `Solid+SetFillBrush` 兼容别名；⑤ **扩展性保留**：recording（PDF/SVG 导出）、surface（第三方后端 RFC#46）、ColorFunc 机制不删。`go build/vet + 受影响测试（TestPath/PathBuilder/CustomBrush/Solid/ClipRect 族）全绿`（TestP12_ClipRectDifferenceGPU 为无 GPU 环境预存在失败，stash 基线复现一致）。详见 docs/RENDER_API_CATALOG.md §7.5.1。 |
| API目录标准化 | **RENDER_API_CATALOG.md 三标准化 + 规则入整体**：① 3 轮源码级核对（主包顶层 0 缺失 + 方法级 0 缺失；scene/recording/surface/svg 子包 100% 覆盖，text 族级归纳）；② 全文档 API 行统一「功能说明 + 精简」两列格式（§1–§6/§8 重写，§8 增公开类型导出方法速查表，§9 常量总表，§11 附录入子包枚举成员）；③ 增 `scripts/apidoc` 一致性检查器（go/ast 提取全部导出符号对照目录文档，可挂 CI），同步义务写入 AGENTS.md + CLAUDE.md（API 文档同步纪律章节，合入前必跑 `go run ./scripts/apidoc`）。 |
| API目录同步 | **新增 `docs/RENDER_API_CATALOG.md`（render 公开 API 总账）**：主包 94 类型 + 109 顶层函数 + Context 181 导出方法 + const/var，以及 `render/text`(226)/scene(132)/recording(78)/surface(47)/svg(15)/filters(0)/raster(0)/gpu(16) 子包，按功能域分类 + 接线状态标注。**已实现未接线**（🔌）：render/svg 整包、render/surface 整包、render/recording 整包、render/raster 独立注册、路径布尔 BooleanPath/PathOp*、ClipOpDifference/Replace、Path Trim/WithCorners/Discrete、SetDither/DrawImageQuad、Pattern/ImagePattern 旧接口、Painter 族、文本描边 StrokeString/TextPath 等。**半成品 GPU 未生效**（⚠️）：任意路径裁剪 `Clip()`/`PushClipPath`（代码接线存在、CPU 软路径正常，GPU 真窗 2026-08 复测画穿，待 wr-debug/wr-engine 定位）；render/gpu 设备生命周期 API 未进 ui/embedder。**从此硬规则（§2.3 + AGENTS.md）**：每次增/改/删 render 公开 API 或接线状态必须同步更新该文档并在此追加记录。 |
| （初始） | 修订记录已清除；全部 R/C/W 状态回退初始 ⬜。待彻查收敛后重新关闭。 |
| （重写中） | W1–W3 推翻重写（进行中）：ui 层 R 能力代码按文档全部删除重实现，对齐 skia/flutter 工业级控件基座。涉及 R= R2/R3/R3b/R4/R4b/R5/R7/R7b/R9/R10/R11/R12b/R13/R18/R19/R21，C= C1/C2/C3/C7。逐波执行：W1 → W2 → W3。 |
| W1 波收口 | **W1 整波 GPU PASS**：ui 层重写 R2（局部 NeedsPaint + paint_visits）/ R3（BoundaryCache 通用录制 + non-cacheable 门禁 + tryReplay 位移）/ R3b（compositing bits 增量传播 + boundary_count/max_depth）/ R5（CountPictureOps + picture_op_count）/ R9（measure_cache_hit/miss 接线）/ R12b（debug repaint 接线）/ R19（snap.go 1px 对齐）；7 独立真窗 + C1 组合窗 GPU 全绿（fps≈60、skip/rerecord、hits≥miss、debug on/off）。 |
| R0 真窗 | **R0 FullPaint 正确性 独立真窗 GPU PASS**（`ui_wr_r0_fullpaint` 1200×800·5s）：6×4 静色格 + 12 静文 + 相位驱动动块同屏；fps_interval=59.9（≥55）、paint_count≈presents（每帧全树）、damage_ratio=1（full_paint 语义非 Retained）、warmup=true、全族 A–J 字段齐。 |
| R0 指标审计 | **metrics-audit R0 通过**：族 A–E/H/I 字段全 PRESENT；诚实性全绿（vsync_source=true 真实、cpu_ui+raster≈cpu_pct_avg、rss 短窗 slope 已声明）；无阈值偷放；无降画质装绿。修复 wrgate 指标层 1 处：`last_cpu_fallback` 空值被 omitempty 静默省略（违反 §2.2.1 禁止默默省略）→ 已去 omitempty 并重跑确认。 |
| R12 真窗 | **R12 帧指标字段完备 独立真窗 GPU PASS**（`ui_wr_r12_metrics` 1200×800·5s）：`gate=schema_only`（R12 唯一允许）+ 真窗 Present=301（静态 4×3 色格 + 8 文 + 相位动块）；wrgate 30 RequiredSchemaKeys + §2.2.1 A–J 全族 42 键逐一核对全存在；fps_interval=59.9 如实输出不做硬门禁；顺带修复 wrgate `measure_cache_hit` 同款 omitempty 省略。 |
| R16 能力 | **R16 H 族观测接线**（wr-implement）：FrameMetrics 加 `warmup`/`time_to_first_present_ms`/`first_present_paint_count`（取自 §2.2.1 H 族 M-WARMUP/M-TIME-TO-FIRST-PRESENT）；PipelineApp `recordFirstPresent` 首帧一次性发布（WarmUp presentSyncFull 或主循环首帧，互斥首次赢）；wrgate BuildReport 观测优先于旧 Warmup 兜底；单测 `TestMetrics_FirstPresent_*` + `TestBuildReport_FirstPresentObservations` 绿 + `go test ./ui/...` 无回归。 |
| R16 真窗 | **R16 首帧/WarmUp/恢复 独立真窗 GPU PASS**（`ui_wr_r16_warmup` 1200×800·5s）：WarmUp=true 首帧全清全画（warmup 观测=true）；首帧有内容 first_present_paint_count>0；time_to_first_present_ms=89.6ms（Open→首 Present 墙钟观测）；fps_interval=59.9（≥55 持续 tick）；中段 InvalidateBoundaryCache 模拟遮挡恢复 → recovery_repaint_ok=true（恢复后全树重画继续、不黑）；policy=full_paint。 |
| W0 三窗视觉区分 | **wr-close 模式 3 视觉优化**（R0/R12/R16 肉眼可区分）：R0 加相位横幅大字（PHASE: STEADY/SPIKE/RECOVER 随相位变字变色，相位驱动可见）；R12 加 SCHEMA 大字实时滚动核对字段名（`SCHEMA: 42 FIELDS · <key>`，schema 核对可视化）；R16 恢复瞬间动块大跳到右下变白 + 大字横幅 `RECOVERY OK`（约 1s），恢复动作肉眼可见。三窗重跑 GPU 全绿（R0 fps=59.9 / R12 schema_only OK / R16 recovery=true·t2f=101ms）。 |
| C0 真窗 | **C0 R0+R12+R16 集成窗 GPU PASS**（`ui_wr_c0_smoke` 1200×800·5s）：开机即见静+动（6×4 色格 + 12 文 + 相位动块 + SCHEMA 滚动大字 + 首帧横幅）；policy=full_paint、首帧不黑（warmup=true + first_present_paint_count=1）、静态每帧存活（paint=300≈presents=301）、§2.2.1 全族 42 键 + wrgate 30 键全在、fps_interval=59.9、t2f=110ms。集成窗不代单窗。 |
| **W0 波收口** | **W0 整波 GPU PASS**：R0（FullPaint 正确性）/ R12（指标字段完备）/ R16（首帧/WarmUp/恢复）3 独立真窗 + C0 集成窗全绿；顺带 wrgate 指标层修复（last_cpu_fallback / measure_cache_hit 禁止 omitempty 静默省略）+ H 族观测接线（warmup / time_to_first_present_ms / first_present_paint_count）+ R0 指标审计通过 + 三窗视觉区分。默认 Present 仍 full_paint 至 W6。 |
| W2 R4 修 bug | **R4 拖拽 resize 停止渲染修复**（GPU 层）：① offscreen 纹理池释放路径取消 pool 归还（`gpu_render_context.go`），release 回调直接 `view.Release(); tex.Release()`；② 新增 `PictureTextureCache.allocEntry`：脏层每次重录分配全新 offscreen 纹理并延迟 2 帧释放旧视图——前一帧提交可能仍在采样旧视图（blit RESOURCE）而重录取其 COLOR_TARGET，wgpu 拒绝同 usage scope 冲突（残留 TXFLUSHERR 883 次/30s → 0）；真窗多轮干净运行全绿（fps≈59.9、dmg_avg≤0.04、present_mode=damage_union/damage_multi、boundary_skip>0）；resize 后像素验证动画存活（热块相位不同位置）+ 几何正确（格阵 304–1183）。 |
| R4 错位排查 | **「画面错位」结论为像素解析假象，引擎无此 bug**：XWD 像素解析脚本头部字节序错误（big-endian 头按 LE 读）导致坐标错读（误判格阵在 0–751）；修正解析后全部历史截图（pre/ctrl/c2/c6/c7/c9/c13/b1/b2/c1 等）格阵均在 x=304–1183、热块相位位置精确（SPIKE(1044,620)黄 / RECOVER(984,700)蓝），与 `ui_wr_r4_composite` 期望布局完全一致；引擎坐标路径（顶点构建/CTM/uniform/viewport/绘制 pass）本就正确，无需修底层。随附诊断全清：移除全部 WR_DIAG/WR_FULLDAMAGE/WR_PROBE/WR_FORCEFULL 分支与打印及 Diag* 计数器（稳态偶发 blit nil 为纹理缓存 miss→向量重放兜底，视觉无影响，非 bug），仅保留 TXFLUSHERR 真实错误日志。 |
| R4 resize 黑帧 | **R4 min/max 风暴黑帧根治（对齐 Skia swapchain recreate 语义，洞 A–D 全收口）**：① 洞 A（Idle 不配对 BeginFrame→永久黑屏）先前已修（present() Idle 分支 DiscardFrame + `TestP14` 双向验证）；② **洞 B/C/D 统一进 `render/present_target.go`**：新增 `postResizeFull` 状态机——`Resize()` 物理尺寸真变化置 3（覆盖双/三缓冲），`present()` 内 `postResizeFull>0` 强制全量路径，**仅 EndFrame 成功才递减**（BeginFrame timeout 帧不消耗预算，天然修复「timeout 吃额度」）；③ embedder 删 `forceFullPresent` 跨线程计数器与 resize 分支 Store(3)，`compositeOnly` 改由 `target.InFullRecovery()` 决定（同锁查询无竞态），app.go 旧路径自动受益；④ 新增 GPU 单测 `TestPresentResize_FullRecoveryWritesEveryBuffer`（render 包，X11+wgpu，Resize→3 帧 full→稳态 idle→再 Resize 重武装，禁用状态机必 FAIL 已双向验证）；⑤ **render 测试包断链修复**：旧架构 `p1_composition_matrix_*`（依赖已删 standardtest + 已删 P1 文档）删除，`compMakeImage` 迁入 s5 helpers，`go vet ./render/` 恢复干净。**像素回归**：6 循环 260×170↔1200×800 风暴中 2 次截图均 0.0% 黑（修复前 mm4 36.1% 黑）；日志 0 in-flight、每次 apply 后连续 3 帧 full、timeout 帧跳过且不消耗预算、恢复 retained 稳态；全量 `go build ./...` + `go test ./ui/...`（14 包）+ 双 GPU 单测绿；R4DIAG/diagf 探针移除。 |
| R4 首次关闭 | **R4 层 Composite Present 独立真窗首次关闭（`ui_wr_r4_composite` 1200×800·15s·GPU PASS）**：① 示例层 Hot 块改 **Flutter 式布局驱动**（新增 `ui/rendering/align.go` `RenderAlignBox`——Align 语义，Layout 按 (父−子)×比例求 offset，resize 自动跟随、无固定坐标/clamp；`wrkit.Panel.Align` 暴露；单测 `TestAlignBox_*` 绿）；② **引擎洞风暴窗口**：`postResizeFull` 3 帧固定预算在拖边风暴中 step 间隙 >3 帧时耗尽→新尺寸 retained 帧 LoadOpLoad 半写缓冲→黑/错乱；新增 **storm-aware 窗口**（render/present_target.go：`resizeStormWindow=300ms`，Resize 记 `lastResizeAt`，`InFullRecovery`/`present()` 风暴活跃期强制全程 full；新 GPU 单测 `TestPresentResize_StormWindowCoversBudgetGap`）；③ 真窗 JSON：fps_interval=59.9（≥55）、damage_ratio_avg=0.057（≤0.35）、present_mode=damage_union 非 full、boundary_skip=52982>0、vsync_source=true、hitch=1/min、slope=短窗 off（§2.2.4 <15s 允许）、全族 A–J 字段齐。§2 R4 ⬜→✅；§5 W2 行 R4✅（整波仍 ⬜ 待 R4b/R11/R13/R18）。 |
| R4b 首次关闭 | **R4b DirtyLayerID + 多 damage 独立真窗首次关闭（`ui_wr_r4b_multidamage` 1200×800·15s·GPU PASS）**：① **wr-implement 能力审查**——能力层已就绪（`ui/scene/build.go` PushBoundary(paintDirty)→DirtyBoundaryIDs 收集、BuildPacket→DirtyLayerIDs、`rasterize.go` 仅 id 命中重录、`render/frame.go` waste-ratio 1.35→PresentModeDamageMulti）；补齐 **wrgate 门禁接线**（`EvaluateRetainedExtras`：`dirty_layer_id_max`+`damage_multi_frames` 从 ability_extra 判定，`EvaluateGates` 自动调用；单测 `TestEvaluateRetainedExtras_DirtyLayerIDGates` 三态）；② **真窗场景**：左上+右下两远离脏点（90×90）每帧同帧变脏 → 两独立 dirty layer id，中央 6×5 色格+8 标签大面积静态（boundary 不重绘），相位脚本 STEADY/SPIKE/RECOVER；HUD 实时 ids/multi/dmg/mode/skip；③ **门禁**：dirty_layer_id_max=10≥2、damage_multi_frames=886≥1、present_mode=damage_multi 非 full、boundary_skip=48114>0、damage_ratio_sum=0.026（**sum of rects 真实重绘像素**——两对角脏点 union bbox 0.49 为几何必然，frame.go 升格决策用 sum，README 已注诚实边界）、fps_interval=59.4≥55、p95=17.0≤22、vsync_source=true、fallback=0、slope_gate=off（<15s 正确性窗 §2.2.4）；metrics-audit 串审全 PASS（10 族 PRESENT、诚实性 11 项 HONEST、阈值 AT_DEFAULT/STRICT、一致性 CONSISTENT、降画质 EARNED）。§2 R4b ⬜→✅；§5 W2 行 R4b✅（整波仍 ⬜ 待 R5/R11/R13/R18）。 |
| 布局驱动规则（§2.6.1） | **新增 U 规则：W2–W6 全部 R 窗口动态热点/能力元素定位一律 Flutter 式布局驱动**（`Panel.Align` + `SetAlignment`，`RenderAlignBox` 语义：offset=(父−子)×比例，resize 自动重算无跳变；禁止固定坐标+clamp，对标 R4）；§2.6.1 通用基线表加「布局驱动」行 + §2.6.3 检查清单加项。**R4b 同步改造**：两远离热点由 `Place(20,20)/(W-150,H-150)` 改为 `Body.Align(0.0246,0.0353)/(0.926,0.894)`（几何精确还原：offset=(20.0,20.0)/(753.8,506.0)，resize 自动跟随），相位切换 SetAlignment 微移保持对角远离；重跑真窗全绿（dirty_layer_id_max=10、damage_multi_frames=888、mode=damage_multi、skip=48222、fps=59.5、fallback=0）。 |
| R5 质量升维 ✅v2 | **R5 Picture 录/回放 质量返工关窗（模式 2 反攻重关 `ui_wr_r5_picture` 1200×800·5s·GPU PASS）**：① **ui 层能力推翻重写**（`ui/scene/picture.go` 对齐 skia/flutter——Picture 不可变显示列表、录制期 paint 规范化 clamp01、严格 SkPaint alpha 语义（删 `a==0→a=1` 历史 hack）、**Bounds 记入 path/image 几何**（旧实现漏记，damage rect 缺陷）、StrokePath 按线宽外扩、float64 内部 bounds + ceil 保守取整；API 零破坏 36 调用点全兼容）；② **render 层洞修复**（`render/path.go` `Path.Clone` 不复制增量 bounds → 补 5 字段，clone 后 Bounds 不再恒空；wr-engine 影响面评估：上游 picture/paint_context、下游 gpu 不受影响，无行为反转，改动前后 render FAIL 集合 IDENTICAL 零回归）；③ **单测**：`picture_test.go` 原 10 + 新增 4（Bounds_IncludesPathAndImage / Bounds_StrokeInflates / Replay_ZeroAlphaDrawsNothing / Recorder_ClampsPaint）全绿，`./ui/...` 14 包无回归 + `CGO_ENABLED=0 go build ./...` OK；④ **真窗升维**（U17/U18/U20）：8 op 直绘 vs 回放同构图对照（2 path + 2 rect + 2 image + 2 text，补 DrawImage 1:1+缩放）、§2.6.1 Align 布局驱动、HUD 加 picture_bounds 行（实测 (18,20 164x210) = 全几何 op union + stroke inflate）、README 六维 + slope_gate=off 诚实声明（5s 正确性窗 §2.2.4）；⑤ **门禁**：picture_op_count=8≥3、fps_interval=59.9≥55、p95=17.8≤22、presents=301、policy=full_paint、vsync_source=true、fallback=0、hitch=0；metrics-audit 串审全 PASS（10 族 PRESENT、11 项 HONEST、阈值 AT_DEFAULT、观测 CONSISTENT、降画质 EARNED）。§2 R5 🔄→✅v2；§5 W2 行 R5✅v2（整波仍 ⬜ 待 R11/R13/R18 + C2）。 |
| R11 首次关闭 | **R11 DPR/尺寸缓存失效 独立真窗首次关闭（`ui_wr_r11_dpr` 1200×800·15s·GPU PASS，3 连跑确定性）**：① **wr-implement 能力补全**——`PipelineApp` 增 `cacheInvalidations atomic.Int64`（`InvalidateBoundaryCache` 失效时自增）+ getter `CacheInvalidations()`；wrgate `GateOptions.MinCacheInvalidations` + `EvaluateRetainedExtras` 加 `cache_invalidations` 判定（ability_extra 机制，R4b 先例）；单测 `TestBoundaryCache_DPRInvalidateReRecords`（steady→Clear→一波 rerecord→回 skip 闭环）+ `TestEvaluateRetainedExtras_CacheInvalidationGate`（pass/fail/missing/off 四态）全绿，`./ui/...` 14 包 + wrgate 无回归，`CGO_ENABLED=0 go build ./...` OK；② **真窗场景**（U17 全达标）：RESIZE-A（Align 0.04/0.06 190×140→250×170 布局驱动尺寸变更 ~3s）、DPR-B（2×2 色格全量 `InvalidateBoundaryCache` ~6s）、NEST-C（3 层嵌套）、DENSE-D（4×4 色格+8 标签+40×40 图 `src.Clone` 浅拷贝）、HOT（**live paint 非 boundary** 每帧变色——零 rerecord 噪声，稳态帧 `FrameRerecord=0` 使波计数确定性；初始 HOT 为 boundary 时 run3 峰帧被双 paint 漏采 → 改 live-paint + peak 降为观测字段）；③ **门禁**：cache_invalidations=1≥1、boundary_rerecord=12≥1、boundary_skip=9877≥1、wave1=1≥1（RESIZE 变更后 0.8s 内 `BoundaryRerecord` 增量）、wave2=11≥1（DPR 同法）、steady_rr_frame=0（回稳后单帧采样）、fps_interval=59.9≥55、p95=17.3≤22、presents=900、policy=full_paint（正确性窗全屏损伤 1，不冒充 retained）、vsync_source=true、hitch=0、fallback=0、slope_gate=off（15s 正确性窗 §2.2.4，同 R4b/R5 先例）；`resize/dpr_wave_peak_frame` 仅观测不设门禁（tick/paint 交错可能漏采峰帧）；④ metrics-audit 串审全 PASS（10 族 PRESENT、诚实性 HONEST、阈值 AT_DEFAULT、观测 CONSISTENT、降画质 EARNED）。§2 R11 ⬜→✅；§5 W2 行 R11✅（整波仍 ⬜ 待 R13/R18 + C2/C7）。 |
| R13 首次关闭 | **R13 Hit ≡ 绘 独立真窗首次关闭（`ui_wr_r13_hit` 1200×800·5s·GPU PASS，4 连跑确定性 probes=8 ok=8）**：① **wr-implement 能力审查**——能力层已就绪（`RenderObject.HitTest` 接口 + 11 个 RO 实现、`PipelineApp.HitTestPointer` overlay 优先、`overlay.HitTestStack`、`RenderTransform.HitTest` 反变换 `T(c)·S⁻¹·R(-θ)·T(-c)`）；补缺 `RenderClipRRect.HitTest` 专门单测 2 个（裁剪内命中/裁剪外拒绝、溢出子只在裁剪内可命中），原有 7 个 hit 单测（transform 旋转/缩放/组合、overlay TopFirst/Block、box offset）全绿，`./ui/...` 14 包无回归 + `CGO_ENABLED=0 go build ./...` OK；② **真窗场景**（U17 全达标）：8 探针 7 类目标全 Align 布局驱动——A/B 普通色块、C 圆角裁剪（50×50 r=12）、D 旋转 30° 反变换命中、E 裁剪溢出（40×40 clip 内命中 / 探针 (50,50) 裁剪外拒绝）、F/G 重叠取最上层（target-top）、H 空白区无命中；静态密集 4×4 色格+8 标签、HOT live paint 每帧、相位 Steady→Spike（高亮发橙）→Recover（还原）、HIT 横幅+LiveHUD 窗内可见、实时 X11 指针 `EventPointer+PointerDown` 走同一 `HitTestPointer`（stderr 日志 `ptr click @(x,y) hit="name"`）；③ **门禁**：scripted_ok=8==scripted_total=8（§2 主表硬门禁）、scripted_hit=6≥4（§2.6 探针数）、命中身份逐探针 DebugName 断言、presents=301、policy=full_paint（正确性窗 damage=1 不冒充 retained）、fps_interval=59.9≥55、p95=16.9≤22、hitch=0、vsync_source=true、fallback=0、cpu_ui=1.6/cpu_raster=17.6 非双 0、slope_gate=off（5s 正确性窗 §2.2.4，R4b/R5/R11 先例）；探针坐标=引擎布局 offset 链（body+align+容器+target）实测非臆造，空白/裁剪外拒绝方向也计数；④ metrics-audit 串审全 PASS（10 族 PRESENT、诚实性 HONEST、阈值 AT_DEFAULT、观测 CONSISTENT、降画质 EARNED）。§2 R13 ⬜→✅；§5 W2 行 R13✅（整波仍 ⬜ 待 R18 + C2/C7）。 |
| R18 首次关闭 | **R18 SaveLayer+预算 独立真窗首次关闭（`ui_wr_r18_savelayer` 1200×800·10s·GPU PASS，3 连跑确定性 allow=600/601/596）**：① **wr-implement 能力补全**——能力半成品（`SaveLayer/RestoreLayer/SaveLayerBudget` 已存在但无计数、无 FrameMetrics 字段、OnPaint 的 `pc.LayerBudget=nil` 预算永不生效）按用户选定方案 A 引擎接线：`paint_context.go` 增 `SaveLayerStats{Allow,Reject atomic.Int64}`，`SaveLayer()` 预算拒批→`Reject.Add(1)`、`PushLayerIsolated` 成功→`Allow.Add(1)`；`pipeline_app.go` `PipelineOptions` 增 `SaveLayerMaxOps/MaxArea`（0=不限，默认其它窗零影响）+ `PipelineApp` 持 `saveStats/saveBudget` 每帧 `budget.Reset()` 注入 `paintPresentTreeWithOpts`（主循环 + presentSyncFull 两调用点），主循环采样转每帧增量 `NoteSaveLayer`（累计值直转会重复计数）；`metrics.go` FrameMetrics 增 `savelayer_allow/savelayer_reject`（omitempty，字段名与 §2.6 门禁一致）+ `NoteSaveLayer`；单测新增 `TestSaveLayer_StatsCountsAllowReject`（MaxOps=1 → Allow=1/Reject=1）+ `TestSaveLayer_StatsAllowNoBudget`（无预算全 Allow）+ `TestWithOrigin_SharesLayerStats`（洞回归），既有 6 个 SaveLayer 测试保持绿，`./ui/...` 14 包无回归 + `CGO_ENABLED=0 go build ./...` OK；② **真窗场景**（U17 全达标）：A 组 1 `SaveLayer(190×120, 0.55)` 半透明组每帧首调用→**允许**（两色块叠加半透明混合=离屏合成可见）、B 组 2 同帧第二调用→**拒批**（无离屏合成直绘 + 红框「BUDGET REJECT」可视）、C 组 3 Spike 相（5–8s）出现追加第三调用→拒批 2/帧（HUD reject 跳动）、D 静态密集 4×4 色格+8 标签、HOT live paint 每帧、相位 Steady→Spike→Recover、HUD `SL: allow=n reject=n (MaxOps=1)` 实时行；③ **门禁**：savelayer_allow=600≥1 且 savelayer_reject=780≥1（§2 主表 + §2.6「两个 SaveLayer 第一个允许第二个拒批」；reject=780=Steady/Recover 420 帧×1+Spike 180 帧×2 与相位脚本数学吻合，allow=600=每帧 1×600 帧）、presents=600、policy=full_paint（正确性窗 damage=1 不冒充 retained）、fps_interval=59.93≥55、p95=18.9≤22、hitch=0、vsync_source=true、fallback=0、cpu_raster=29.4%（每帧离屏合成真实开销非假绿）、slope_gate=off（10s 正确性窗 §2.2.4，R4b/R5/R11/R13 先例）；**洞修复**：首跑 allow=0/reject=0 → 定位 `PaintContext.WithOrigin` 漏复制 `LayerStats`（`RenderAlignBox.Paint` 经 `pc.WithOrigin` 传子 pc，Align 子树下 SaveLayer 计数静默丢失）→ 单行补 `LayerStats: pc.LayerStats` + 回归单测（用户确认直接修，ui/rendering 低风险层）；④ metrics-audit 串审全 PASS（10 族 PRESENT、诚实性 HONEST、阈值 AT_DEFAULT、观测 CONSISTENT、降画质 EARNED）。§2 R18 ⬜→✅；§5 W2 行 R18✅（W2 单能力窗全绿，剩 C2/C7 组合窗）。 |
| R21 Bug2 修 | **R21 文字断笔/笔划粗 引擎洞修复（render/ 高风险，经用户确认自研优化方向）**：① **根因**——像素对照（开发期 libfreetype 度量衡，`/tmp/opencode/ftexp` + `rastercmp`）证明光栅器自身无断笔（NO_HINTING 同轮廓下自研与 FreeType 连通块全一致）；断笔在 **placement 层**：hinted 文本带小数 X 进光栅 → 1px 垂直 CJK 笔划劈成两个半覆盖列（FreeType 自身 fracX=0.5 时同断，实测 comps 2→3）；引擎 `snapX := hinting==HintingFull && !useLCD`（`render/internal/gpu/glyph_mask_engine.go:414`）漏掉 CJK Vertical 路径；② **修复**——snapX 改 `hinting != HintingNone`（CJK Vertical 纳入整数 X 放置，LCD 保留 RGB 相位，HintingNone 保留小数）；CPU 软件路径 `render/text/draw.go` drawGlyphs/drawGlyphsVariable 同步：hinted 时 subpixelX/Y=0 + round-advance 网格；③ **验证**——新单测 `glyph_placement_test.go` 5 项合同（CJK-V snapX/Full snapX/None 小数/LCD X 相位/snapXGrid 单调整数）全 PASS；CJK 对照矩阵 8 字×11/12/16px×hint×frac：修复后 own-v0=FT-l0 无断笔（标16 comps=2），own-v0.5 与 FT-n0.5 同断（机制共证）；`go test ./render/...` 失败集与 stash 基线完全相同（零新增）；真窗 15s 全绿 + **90s 温和 resize（40 次）回归**：无崩溃、shell_rerecord_scroll=0（Bug1 门禁不回退）、fps=59.54、fallback=0、skip=97722；④ **边界记录**——own Full hint @16 标 comps=3 为 ADR-027 已知（CJK 不走 Full，selectGlyphMaskHinting 已规避）；历史「自研 no-hint 12px 标=5 连通块」与当前代码不符（当前=2）。§2 R21 状态仍 ⬜（升维归 wr-close）；本行 = wr-debug 修复记录。 |
| 2D对齐三洞 | **按 Skia/Flutter 对齐标准修复三个 2D 能力洞（计划 `docs/RENDER_2D_ALIGNMENT_PLAN.md`）**。① **任意路径裁剪 Clip() GPU 激活（GPU-CLIP-003a）**：根因 = `s.depthClipPipeline` 从未初始化（`NewDepthClipPipeline` 仅测试调用，1538 守卫恒 false，stencil+depth 两阶段整条死代码）→ `NewGPURenderSession` 构造时初始化 + `ensureStagePipelines` 防御重建；新增会话级单测（无 GPU skip）。**待 GPU 真窗复测**（`examples/render_clipping`）。② **Bicubic GPU 化（I.03，对齐 Skia SkCubicResampler）**：新增 `textured_quad_bicubic.wgsl`（16-tap Catmull-Rom B=0,C=0.5，与 CPU SampleBicubic 像素一致；负权重跳过 bug 已修）；`ImageDrawCommand.Bicubic` + 管线双变体（stencil/depth-clip）+ `QueueImageDraw(..., bicubic ...bool)` 可变参数后向兼容；context_image 统一 GPU 优先、CPU 兜底；单测 = shader 算法 Go 镜像 vs CPU 逐像素对照 ≤2/255。③ **UI 排版接通 render/text（对齐 Flutter SkParagraph UAX#14）**：`fitRunPrefix` 换 `text.WrapText` 词边界优先（lossless 切分 + 行尾空格 trim 归还）；`drawTextWrapped` 接 `WrapText` 逐行 DrawString（保留 DrawStringWrapped 无字体兜底）；单测 = 词边界不断词 + 超宽词字符兜底。验证：`go build ./...` OK；`go test ./render ./render/internal/gpu ./ui/...` 失败集 8 个与 stash baseline 完全一致（零新增回归）；新增单测全 PASS。文档同步：RENDER_API_CATALOG §3.2/§3.5/§7.3 更新（Clip 待 GPU 复测不标 ✅）。 |
| API目录同步 | **RENDER_API_CATALOG.md 与源码一致性修正（2026-08-15 复查，纯文档无代码改动）**：① **虚报清理**——Path 方法实为 `QuadraticTo`（非 QuadTo）；Paint 无 `SetColor/SetLineWidth`；Pixmap 无 `Image/Stride` 方法（实为 ImageView/ToImage；Stride 是 image.RGBA 字段）；SoftwareRenderer 无 `Flush/Render/Clear/Capabilities`；Point 无 `Approx`（属 Vec2）；§9 移除不属于主包的 `TextureUsage` 族 5 常量与 `BlendSourceOver`（主包 BlendMode=intImage 别名、无该常量）；§1 Rect 行移除 recording 专属 `NewRectFromPoints`；§6.1 text 族表 `ShapeResult`/`GlyphMaskFlags`/`LoadFontFace*` 三处不实（实际为 ShapedGlyph / GlyphMaskFlagAliased、LCD、LCDBGR / LoadDefaultFace、LoadMultiFace、NewFontSourceFromFile）；附录 B 移除 scene 虚报 `BlendModulate`。② **接线状态修正**——SelectPipeline 由「仅测试」改 ✅（生产 `render/internal/gpu/gpu_render_context.go:2820`，Auto 模式每帧）；DrawMesh 由 🔗 改 🧪（仅测试）；LayerPoolStats/ResetLayerPoolStats 由 🔗 改 🧪（仅测试，注释自述）。③ **数量快照刷新**——主包 顶层函数 109→111、常量 91→120；text 226→239、scene 132→143、recording 78→85、surface 47→53、gpu 16→18（go doc 符号段口径，2026-08-15）；附录 A 补 22 个 const 块首行并按 go doc 原生缩进重建；附录 B 补 scene/recording/surface 全部枚举成员组。`go run ./scripts/apidoc` 绿；§8/§9/附录 A/B 双向（代码↔文档）复核全过。 |
| API目录同步 | **新增 `GPUTransformMaskTextAccelerator` 接口（kTransformedMask 语义：旋转/变换文本整串 CPU 光栅 alpha mask → glyph-mask atlas quad，GPU 与 CPU Skia-AAA 逐像素同源）**：`render/accelerator.go` 新接口 + `render/text.go` dispatchText TextModeVector 分支接入（显式 Vector 模式保持纯轮廓）+ `render/internal/gpu/transformed_mask_text.go` 实现（AnalyticFiller 直连光栅，绕过 SoftwareRenderer Auto 分支的 AdaptiveFiller 注册分流；`glyph_mask_engine.go` PutTransformMask + transformMaskKey 内容指纹缓存；新单测 `transform_mask_raster_test.go` 旋转/轴对齐矩形墨量断言）。RENDER_API_CATALOG §2 能力接口群补符号；`go run ./scripts/apidoc` 绿。实测（render_text_transform 9 格）：旋转格 big(≥160) 130→0、maxd 211→≤74、quad 与 mask 三向校验一致（光栅±5 级 / 混合±4 级）；残余少量亚像素差（4-5 级，CPU side clip/光栅舍入）待下一轮专项收敛。 |
| Clip 洞修复收口 | **任意路径裁剪 GPU 画穿修复收口（render/internal/gpu，GPU-CLIP-003a 复测通过）**：① **根因**——`DepthClipPipeline.ensurePipeline()`（编译 shader + 创建 `uniformBGL` + stencil-fill/cover-to-depth 两管线）在生产路径从未被调用：`NewGPURenderSession` 只构造结构体，`uniformBGL` 恒 nil → `BuildClipResources` 的 `CreateBindGroup` 全部失败（`layout is nil`）→ 每个 ScissorGroup 拿不到 `depthClipRes` → `Clip()/ClipPreserve()` 深度裁剪从未生效，`examples/render_clipping` 圆（ex1）/星（ex3）/复杂路径（ex5）条纹整片画穿（圆外泄漏 19919/19919=100%）。② **修复**——`render/internal/gpu/render_session.go` `ensureStagePipelines(needDepthClip)` 分支补调 `s.depthClipPipeline.ensurePipeline()`（防御性，幂等）。③ **验证**——新回归单测 `TestDepthClip_ProductionPath_NoManualEnsure`（经生产 `RenderFrameGrouped` + ClipPath，不手动 ensurePipeline：clip 方外 0 像素越界）；render-session 隔离诊断：square/circle/SDF 裁剪全对（凸路径），star 非凸路径 fan 三角形外溢为既有次要缺陷（另行立案）；示例复测 `examples/render_clipping`：ex1 圆外泄漏 100%→0.6%（仅边缘 AA）、ex3 星外条纹 744→0、ex5 复杂路径 big(≥160) 52 个集中在 clip 边界 AA；gpu 包全量 ok、`go build ./...` OK、Clip 相关测试全绿。§7.3 Clip 状态 ⚠️→✅。 |
| Clip 边缘 AA 补全 | **任意路径裁剪 GPU 边缘锯齿修复（render/internal/gpu，Skia kCoverage / Flutter ClipMask 模式）**：① **根因**——sampleCount==1（无 MSAA，llvmpipe 软 Vulkan）下 `DepthClipPipeline` 的 stencil+depth 两阶段是二值掩码：stencil（fan 三角形）判定 inside/outside 后在 cover 阶段写 depth=0.0/1.0，内容管线 `DepthCompare=GreaterEqual` 要么全过要么全拒→ clip 边缘像素无部分覆盖 → 圆（ex1）外边框、星形（ex3）内凹/外边锯齿；CPU 软渲染走分析 AA（`ClipStack.Coverage`），GPU 与 CPU 结果不一致（复跑像素证据：ex1 圆边缘带 [r≈80] 过渡像素 GPU=0 / CPU=367，星形外带 GPU=0 / CPU=27）。② **修复**——① `clipPathCoverageMask` 用 CPU 分析 AA（`render/internal/clip.MaskClipper`，与软件后端同源：4x Y 超采样 + 边缘分数覆盖）生成全帧 R8 coverage mask；② `DepthClipPipeline.BuildClipMask` 上传 mask 纹理 + bind group（复用 L.06 `@group(2)` mask 通道 + `MaskParams{Enabled:1}`）；③ Phase1.5 `stencilExpandPipeline`（仅 sampleCount==1 创建）：用 `TessellateAA` 生成的 bandVerts 外带网格 stencil `Replace(1)` 扩充深度通过区到 mask 渐变带——否则边界像素在深度测试就被拒，mask 没机会调制；④ `recordGroupDraws` 内容管线（SDF/convex/stencil-cover）mask 参数从 `frameMaskBindGroup()` 换成 per-group `depthClipRes.maskBG`。③ **验证**——新单测 `TestClipPathCoverageMask_AnalyticAA`（圆：中心 255/角落 0/边缘带含渐变）+ `TestBuildClipMask_SampleCount1`（管线集成建出 mask 三件套）+ `TestClipPathCoverageMask_EmptyPath`；readback 诊断确认 mask 纹理内容与 CPU 数组逐字节一致；真窗复测 `examples/render_clipping`：ex1 圆边缘过渡像素 0→418（CPU 367）、ex3 星形外带 0→25（CPU 27）、ex4 嵌套圆 clip 0→276（CPU 332），GPU 与 CPU 边缘渐变一致、ex2/ex5 与修复前 diff=0 零回归；gpu 包全量 + DepthClip 族测试全绿、`go build ./...` OK。§7.3 Clip 行补 AA 证据。 |
| Clip 描边内侧 AA | **任意路径描边（stroke）内侧锯齿修复（render/internal/gpu，`FanTessellator.TessellateAA` 孔洞轮廓 band 方向）**：① **根因**——`ClipPreserve`/普通描边走 `StrokePath`→expand 成环形 fill（外轮廓+内轮廓两 contour）→ stencil-then-cover。`TessellateAA` 按**单轮廓面积符号**决定 band 外法线方向（面积负→orient=-1），但环形路径的内轮廓（孔洞，面积负）winding 与主轮廓相反——它的"外侧"应朝向孔洞（stencil==0 区），却被当成普通 CW 轮廓处理：exterior band 朝向描边带、interior band 朝向孔洞，描边内侧（ex5 眼睛形顶部弧线 y≈69）边缘 AA 缺失 → 内侧整行纯黑硬边（复跑：GPU y66..69 全黑 vs CPU 3 行黑 + 1 行灰 191，「描边内侧有锯齿」）。② **修复**——`TessellateAA` Pass 2 按主轮廓符号（最大面积轮廓）判定孔洞：面积符号与主符号相反的 contour 翻转 orient（exterior band 朝向孔洞）+ 孔洞轮廓外带外扩 `aaCoverBandPad=0.5px`（覆盖像素中心，过渡带保持不变——内带不扩，防过渡带比 CPU 宽）；③ **验证**——真窗复测 `examples/render_clipping` ex5（眼睛形复杂路径描边）：x=675 顶弧描边内侧 y69 GPU=181（CPU 191）平滑过渡，原 GPU=0 全黑；星形（ex3）/圆（ex1）边缘零回归（diff 与修复前逐像素一致）；render 包失败集与 stash 基线完全一致（零新增）、gpu 包全量 PASS、`go build ./...` OK。 |
| RSS泄漏修复 | **GPU 资源登记表只进不出修复（render/internal/gpu，R4 真窗 RSS 持续增长根因）**：① **根因**——`viewToResView`/`ResolveCommandView`（Raw 回退）每次纹理绘制（HUD blit、base layer）都向 session 资源登记表 `res.Registry` 注册一个临时 view 条目，命令消费后只 `Release`（引用数归零）**从不 `Retire`**（生产代码 `Retire` 零调用、`Submission.Track` 从未接线）→ `removeIfReady` 的删除条件（`retire==true`）永不满足 → slot+map entry+原生句柄只增不减，90s 实测 Go 存活堆 +32MB、稳态 RSS 约 +0.68MB/s 无限增长。② **修复**——`Registry` 加 `RegisterBorrowed`（借用注册：跟踪但不拥有、删除时不销毁原生，视图生命周期归 PictureTextureCache 等外部 owner）；`viewToResView`/Raw 回退改走借用注册；`GPURenderSession` 加 `pendingViewRetires`，注册的临时 ref 在**帧完成点**（BeginFrame vsync/drainQueue、readback Map 同步点）统一 `Release`+`Retire`——销毁严格发生在命令执行完之后，借用视图不被销毁（第一次尝试直接 Retire 导致持久纹理视图被销毁 → present 退化 full，改用借用语义后恢复）。③ **验证**——新单测 `TestRegTransientView_LifecycleClosesAtFrameBoundary`/`TestRegBorrowed_RetireSkipsNativeRelease`/`TestRegTransientView_RawResolvePathRetired` 全绿（含借用不销毁断言）；res 包全量 PASS；GPU 真窗 90s 复测：present_mode=damage_union（无渲染退化）、cpu_fallback_ops=0、0 错误；**稳态 RSS 增长 +41MB/60s→+10MB/60s（-75%）、GC live 增长 +32MB→+8MB（-75%）**；render/internal/gpu 相关测试（P5/Opt40/view_raw）无回归（Opt27/Opt28 为既有失败，stash 基线一致）。 |
| 渲染停止修复 | **"运行一会自动停止渲染、调整窗口大小才恢复"三层根因修复，最终对齐 Flutter VsyncWaiter 非阻塞回调模型（ui/scheduler + gpu/webgpu，R4 长测复现：无周期活动时 ~40s 冻结）**：① **vsync 挂起**（第一层）——`WaitFramePace` 原阻塞调 `WaitVSync`（DRM vblank wait），vblank 源停止即卡死主循环。**修复（对齐 Flutter VsyncWaiter 回调语义，实现/修复流程纪律：先核查 → Flutter/Skia 原生为回调非阻塞 → 重构而非补丁；最初 100ms 超时+闩锁方案经审计判定机制不对齐后推翻重构）**：`FrameScheduler` 加 `ensureVsyncListener`——独立 goroutine 循环 `WaitVSync`，成功打时间戳 + `ScheduleFrame`（主循环从不阻塞等 vblank）；`FrameDue()` 非阻塞帧节奏门——vsync 信号新鲜（33ms 窗口）或距上帧 ≥ animTick 才放行渲染；`WaitFramePace` 退化为纯记账。vblank 挂起 = listener goroutine 卡住一次 + 时间戳停更 = **自动**降级软件节奏（无超时/闩锁状态机）。② **swapchain acquire 挂起**（第二层）——`BeginFrame` 的 `GetCurrentTexture` 无超时，llvmpipe 下图像耗尽后永久阻塞并持有 PresentTarget 锁 → 主 goroutine 连带冻结。**修复（先核查 → 复用存量 Skia OUT_OF_DATE 重建路径，750-785 行）**：`acquireSurfaceTexture` 250ms 超时 + `acquireHung` 闩锁（挂起后跳过 cgo）+ 复用 DiscardTexture+Configure 重建，成功复位。③ **全阻塞时 runtime timer 停摆**（第三层，Go 运行时 + 软 Vulkan 环境实证，**非 Skia/Flutter 机制、已如实标注**）——进程全阻塞后 `time.After`/`Sleep` 不触发（TICK 诊断 goroutine 存在则健康、移除则 ~40s 冻结，稳定复现）。修复：`startBeat` 帧心跳 goroutine（16ms 周期持续活动）。**真窗验收（llvmpipe，2026-08-17）**：非阻塞重构后 **5/5 次 90s 全程 fps 65.9~72.1（≥55）、present_mode=damage_union、presents 5900+ 零中断**；vblank 挂时阻塞版与非阻塞版同表现为慢速兜底（对照证明为环境状态、引擎不冻结）；单测 14 个（FramePace 信号驱动/error 计 miss/error 重试节流/挂起不阻塞/信号停止后软件节奏降级/心跳等）**全部 `-race` 全绿**；swapchain acquire 单测 5 个 `-race` 全绿；scheduler/embedder 包全量 PASS（既有 Opt27/28/TestS68 失败与本次无关，stash 基线一致）。 |
| 架构对齐收口 | **真窗迁框架原生窗口 + 事件循环内核 poll + 心跳删除 + 可见性停帧（2026-08-17，对齐 Flutter/Skia 桌面 GUI 模型）**：① **真窗窗口宿主从 examples/exhost 迁移到 ui/platform 框架原生窗口**——30 个 ui_wr_*/ui_l1_*/ui_render_* 真窗 `exhost.Open(exhost.Options{...})` → `platform.Open(platform.Options{...})`（产品层已有 Backend.Create/Options/Open，本轮补 `Window.Backend()` 公开方法）；**examples/exhost 目录整个删除**（示例层宿主移除，真窗统一用框架窗口能力）。② **事件循环内核 poll（取代并删除心跳）**——ui/platform x11 `WaitEvents` 从 `time.After` 轮询改为 `unix.Poll`（XConnectionNumber fd + self-pipe，WakeUp 写 pipe）——Flutter/Skia 事件循环模型（内核级等待、无 runtime timer 依赖）；wayland 原本就是 poll。**`startBeat` 帧心跳删除**（其补偿的 timer 停摆问题随 poll 改造消失：所有宿主事件循环都不再依赖 runtime timer）；`drainWakePipe` 修复（阻塞 Read 挂起 → 非阻塞单次读）。③ **可见性停帧（对齐 Flutter lifecycle paused → 停帧）**——embedder 消费 `EventOccluded`（x11 VisibilityNotify 已产生）：窗口全遮挡/最小化 → `occluded` 闩锁 → 渲染门关闭（不渲染、不调度）；可见恢复 → 重新调度。**真窗验收（llvmpipe，2026-08-17）**：R4 框架窗口 + poll + occluded 门 90s fps=61.3、present_mode=damage_multi、presents=5523 零中断；scheduler/embedder/platform 全量 + `-race` 全绿；`go build ./...` + apidoc 绿。 |


---

## 11. 一句话

> **每个主能力真窗：1200×800 · RUN_SECONDS≥5 · 按能力加长观察指标（§2 主表 + §2.5 全表）。**  
> **状态以 §2 主表 + §3 组合表 + §5 分期 + §10 修订为准。** W0 ✅ · W1 ✅ · W2–W3 ⬜。默认 Present 仍 full_paint 至 W6。
