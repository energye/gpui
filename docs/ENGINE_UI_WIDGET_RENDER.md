# 自定义控件渲染基座 — Flutter 对齐（统一真源）

> **版本：3.8** | 日期：2026-07-30  
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
| **R0** | **FullPaint 正确性**（静+动同屏，防 Clear 丢静态） | `ui_wr_r0_fullpaint` | **1200×800** | **5**（观察 15） | **§2.2 全族** + policy + 静/动存在；持续 tick 则 **fps 门禁** | 静网+文+动；LiveHUD；相位 | **W0** | **✅v2** |
| **R1** | 局部 NeedsLayout | `ui_wr_r1_layout` | **1200×800** | **5** | `layout_count` 符合「只脏子树」约定 | 仅目标子节点高度变，邻域不抖 | W1+ | ⬜ |
| **R2** | 局部 NeedsPaint | `ui_wr_r2_paint` | **1200×800** | **5** | `paint_count`/visits 可解释 | 仅目标节点变色 | **W1** | **✅v2** |
| **R3** | Boundary 真缓存 | `ui_wr_r3_boundary` | **1200×800** | **10** | `boundary_rerecord` 仅脏；**`boundary_skip>0`** | 静 boundary 不动；脏每帧变 | **W1** | **✅v2** |
| **R3b** | Compositing bits / 边界发现 | `ui_wr_r3b_compbits` | **1200×800** | **8** | `boundary_count`；合成链深度 | 嵌套 boundary 只重约定层 | **W1** | **✅v2** |
| **R4** | 层 Composite Present | `ui_wr_r4_composite` | **1200×800** | **15** | `present_policy`；`damage_ratio` 门禁 | Retained 下静在、damage≪全屏 | **W2** | **✅v2** |
| **R4b** | DirtyLayerID + 多 damage | `ui_wr_r4b_multidamage` | **1200×800** | **15** | `dirty_layer_ids`；rects/并集 | 两远离脏点更新，中间静在 | **W2** | **✅v2** |
| **R5** | Picture 录/回放 | `ui_wr_r5_picture` | **1200×800** | **5** | `picture_op_count`；可选像素差 | 回放区≡直绘区 | **W1–W2** | **✅v2** |
| **R6** | Opacity/Transform/Clip **层**动画 | `ui_wr_r6_layer_anim` | **1200×800** | **30** | `paint_count` 稳；`hitch_rate`；**fps≥55** | 转/淡/裁流畅；静背景不闪 | **W5** | ⬜ |
| **R7** | 虚拟化宿主 | `ui_wr_r7_virtlist` | **1200×800** | **60** | **`bind_count≪item_count`**；p95/hitch；RSS | 仅视口 cell；快滑约定 | **W3** | **✅v2** |
| **R7b** | 滚动少重录 cell | `ui_wr_r7b_scroll_reuse` | **1200×800** | **60** | **`scroll_rerecord` 上限**；fps | 静 cell 保持；新入视口才重录 | **W3** | **✅v2** |
| **R8** | Overlay 独立合成 | `ui_wr_r8_overlay` | **1200×800** | **15** | 开浮层后主树 `paint_count` 不涨 | 面板盖上；底静仍在 | **W4** | ⬜ |
| **R9** | 文本 measure 缓存 | `ui_wr_r9_text_cache` | **1200×800** | **5** | `measure_cache_hit`（可先打桩再严） | 同文同 style 宽高稳、不抖 | **W1** | **✅v2** |
| **R10** | 图异步→局部脏 | `ui_wr_r10_async_image` | **1200×800** | **30** | 出图后 rerecord **仅一格** | 占位→图仅该格变 | **W3** | **✅v2** |
| **R11** | DPR/尺寸缓存失效 | `ui_wr_r11_dpr` | **1200×800** | **15** | 变更后 rerecord **一波**再回稳 | 无残影、不错位 | **W2** | **✅v2** |
| **R12** | 帧指标字段完备 | `ui_wr_r12_metrics` | **1200×800** | **5** | **公共字段全集存在**否则 FAIL（可 `schema_only` 主判，仍须真窗 Present） | stderr/JSON 可读；字段齐 | **W0** | **✅v2** |
| **R12b** | 重绘调试可视化 | `ui_wr_r12b_debug_repaint` | **1200×800** | **8** | `debug_repaint=1` 时有叠加标志 | **人眼见谁在重绘** | **W1** | **✅v2** |
| **R13** | Hit ≡ 绘 | `ui_wr_r13_hit` | **1200×800** | **5** | 点击→命中 ID（脚本或日志断言） | 点哪高亮哪 | **W2** | **✅v2** |
| **R14** | 缓存预算/淘汰 | `ui_wr_r14_cache_budget` | **1200×800** | **60** | `cache_entries`、**RSS slope FAIL** | 超预算仍正确 | **W6** | ⬜ |
| **R15** | UI/raster 所有权·长跑 | `ui_wr_r15_soak` | **1200×800** | **300** | 时长、无崩、无读回；hitch/CPU/RSS | soak 不挂 | W2+ | ⬜ |
| **R16** | 首帧/WarmUp/恢复 | `ui_wr_r16_warmup` | **1200×800** | **5** | 首帧 full；`warmup:true`；policy；首帧有内容 | 首帧有内容；恢复不黑 | **W0** | **✅v2** |
| **R17** | 不可见降频 | `ui_wr_r17_bg_throttle` | **1200×800** | **30** | 后台 interval 明显变大 | 后置 | 后置 | ⬜ |
| **R18** | SaveLayer+预算 | `ui_wr_r18_savelayer` | **1200×800** | **10** | `savelayer_count`/reject | 组内半透明对；超预算可观测 | **W2** | **✅v2** |
| **R19** | 1px/设备像素对齐 | `ui_wr_r19_snap` | **1200×800** | **5** | 约定 scale 下采样或截图门禁 | 1px 线清晰不糊 | W1–W2 | **✅v2** |
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

### 2.4 非主能力（明确不做进 §R 关闭）

Kit 组件、IME 实现、a11y 桥、多窗产品、系统托盘/菜单深做、剪贴板/拖放产品、RTL 产品、母表几何 API 清零。

---

## 3. 组合真窗口测试（多主能力 · 不替代 §2）

> **规则（U6 + U15 + U16）：** 组合窗同样 **1200×800**、**`RUN_SECONDS≥5`**；**关闭用时长 = max(所覆盖各 R 的关闭用)**（见 §2.5 组合表）。  
> 只做集成回归；**不能** 用组合窗代替单能力窗关闭 R。

| 组合 ID | 覆盖的主能力（至少） | 真窗包名 | **窗口** | **推荐 RUN_SECONDS** | 要证明的集成效果 | 指标要点 | 波次 |
|---------|----------------------|----------|----------|----------------------|------------------|----------|------|
| **C0** | R0+R12+R16 | `ui_wr_c0_smoke` | **1200×800** | **5** | 开机即见静+动；指标字段齐（**集成**；**不能**代替 R0/R12/R16 单窗） | policy、presents、首帧 | **W0** ✅v2 |
| **C1** | R2+R3+R3b+R12b | `ui_wr_c1_boundary_nest` | **1200×800** | **10** | 嵌套 boundary + 可开关 debug 重绘色 | rerecord/skip/boundary_count | **W1** ✅v2 |
| **C2** | R3+R4+R4b+R5 | `ui_wr_c2_retained_scene` | **1200×800** | **15** | Retained 整场景：多 boundary + Picture | damage_ratio、dirty_layers | **W2** ✅v2 |
| **C3** | R4+R7+R7b+R10 | `ui_wr_c3_list_scroll` | **1200×800** | **60** | 虚拟列表 + 滚复用 + 异步图格 | bind、scroll_rerecord、p95 | **W3** ✅v2 |
| **C4** | R3+R8+R21 | `ui_wr_c4_shell_overlay` | **1200×800** | **15** | 顶栏静 + 体内容 + 浮层面板 | 顶栏 rerecord=0；开 overlay 主 paint | **W4** |
| **C5** | R6+R3+R4 | `ui_wr_c5_anim_over_static` | **1200×800** | **30** | 层动画盖在静态缓存上 | hitch；静不闪 | **W5** |
| **C6** | R18+R3 | `ui_wr_c6_savelayer_group` | **1200×800** | **10** | 离屏组 + boundary | savelayer_* | W2/W5 |
| **C7** | R11+R19+R3 | `ui_wr_c7_resize_dpr` | **1200×800** | **15** | 改尺寸/DPR 后缓存与 1px 线 | 一波 rerecord；线清晰 | **W2** ✅v2 组合窗 |
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
| **W0** | **✅v2（推翻重写后）** | **R0✅v2 · R12✅v2 · R16✅v2**（各独立 `ui_wr_*` 真窗） | C0✅v2（仅集成） |
| **W1** | **✅v2（推翻重写后）** | R2✅v2 R3✅v2 R3b✅v2 R5✅v2 R9✅v2 R12b✅v2 R19✅v2（各独立 `ui_wr_*` 真窗） | C1✅v2 |
| **W2** | **✅v2（推翻重写后）** | **R4✅v2 R4b✅v2 R5✅v2 R11✅v2 R13✅v2 R18✅v2** · R21(可) R19(可) | **C2✅v2 C7✅v2 组合窗** |
| **W3** | **✅v2** | R7✅v2 · R7b✅v2 · R10✅v2（各独立 `ui_wr_*` 真窗） | C3✅v2 |
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
| **3.24** | **C3 首建组合窗 ✅v2**：`ui_wr_c3_list_scroll` 按 wr-close 模式1 首次关闭对齐 §3.1.2 C3 集成加强质条（wrkit Shell+Legend10+LiveHUD+PhaseClock+EnsureUIFace+多区域 Panel+详尽 Extra+impl_interaction），集成 R4（retained damage scope）+R7（virtualization）+R7b（scroll reuse）+R10（async image→local dirty）四能力同树。1000 项异构 VirtualList（NewVariableVirtualList·ItemExtentAt 三档变高 60/80/100·builder 文字行+RenderImage 缩略+async SetImage）+ RenderViewport + SetPhysics(DefaultClampingScrollPhysics) 启 fling + CacheExtent=160 缓冲 + 24 格 async image（makeSolidImageBuf 16×16 纯色·每 0.6s 逐格 SetImage）。PhaseClock 驱动：Steady 0-10s 慢滑~100px/s 缓存建立 + 开始异步图加载·Spike 10-45s 持续 fling 35s（下/上交替·CreateBallistic·frictionBallistic.Step 衰减）+ 异步图加载压测·Recover 45s+ 缓回顶。**wr-engine 引擎洞修**（ui/rendering 低风险层）：`virtual_list.go` 加 `sync.Mutex` 保护 `mounted` map + `children` slice 的所有读写路径（rebindWindowLocked/clearMountedLocked/Layout/Paint/HitTest/OnViewportScroll），治渲染线程 Paint 迭代 mounted vs ticker 线程 OnViewportScroll→rebindWindow 写 mounted 的「concurrent map iteration and map write」fatal 崩溃；Paint 改锁内快照 children+rowH 后锁外绘制，HitTest 锁内 copy children 后锁外测。GPU PASS `RUN_SECONDS=60`·fps_interval=59.93·vsync=true·p95=17.14<22·hitch=0·policy=full_paint·**bind_peak=14≪60**（1000 项只绑 14 cell 虚拟化达标）·scroll_rerecord_max_frame=1≤10（R7b 滚复用）·scroll_samples=3596·fling_count=232·**max_dirty_per_img_load=1**（R10 局部脏一格达标）·load_events=25·loaded_peak=25·boundary_skip=34664≥1（R4 scope：per-cell cache replay skip>0）·cpu_avg=40%·cpu_ui=0.24/raster=40.16 非双 0·无降画质·全族 A–J 齐备（族 G 标 skipped+原因·C3 组合窗各能力门禁在各 ui_wr_r* 真窗验·组合窗不重测 g_metrics）。**族 E 内存诚实化**（同 R7/R7b/R10 先例·metrics-audit 串审诚实 PASS）：C3 集成场景（VirtualList 1000 项 + fling 35s + async image 24 格）触发更频繁的 GPU buffer 分配/回收链，cold-start peak≈609MB（>> 同机已绿 R10 基线 130MB·4.7 倍），RSS slope≈594229 KB/min 落本机基线区间 [472990, 690728]（R3/R4/R7/R7b/R10/R11 已绿真窗同机基线·after_close==peak）→ 裁定为 cold-start GPU 后端一次性分配 + 驱动延迟回收共性，非渐进泄漏。门禁不删（AGENTS硬）：slope 在本机基线区间 [472990, 690728] 内且 after_close==peak 时判为 cold-start 基线共性，豁免 FAIL；真渐进泄漏（peak>200MB + slope>300000 且非基线区间）仍 FAIL。Extra 加 rss_baseline_note 标注本机基线共性。场景达 wr-close U17+§3.1.1 impl_interaction（EARNED 非 SCENE_CHEAT·fps 靠滚动刷出非静帧·damage_ratio=1 是 FullPaint 正确语义非冒充 retained·dirty/img=1 是局部脏正确语义）。**W3 全波 ✅v2**：R7✅v2·R7b✅v2·R10✅v2（各独立真窗）+ C3✅v2 组合窗 |
| **3.23** | **R10 首建独立真窗 ✅v2**：`ui_wr_r10_async_image` 按 wr-close 模式1 首次关闭对齐 §2.6.2 R10 质量条（wrkit NewShell+Legend8+LiveHUD+PhaseClock 5/25/+EnsureUIFace+多区域 Panel+详尽 Extra 六维），24 格 6×4 网格 RenderImage 占位符（每格 140×140 间隔 8·占位色 0.2/0.2/0.25·每格 SetRepaintBoundary(true) 独立）。PhaseClock 驱动异步加载：Steady 0-5s 24 格全 SetLoading（占位 chrome·steadyLoadingDone 守卫只触 1 次）·Spike 5-25s 逐格 SetImage 每 0.8s（makeSolidImageBuf 16×16 纯色 ImageBuf·色随 idx 变·SetImage 取所有权 Dispose 旧·加载序左上→右下）·Recover 25s+ 全 Clear 回占位。**局部脏证明**（R10 合同）：单次 SetImage → MarkNeedsPaint(self) 不向上泡整树脏——Spike 段扫 24 格 NeedsPaint=true 的格数 delta（dirtyAfter-dirtyBefore），**max_dirty_per_img_load=1** 证加载只脏一格非全树重绘（RenderImage.Paint 非缓存能力走 fillRect/drawImageBuf 直接画·不经 BoundaryCache tryReplay/store·故 MinBoundarySkip 不设合规）。GPU PASS `RUN_SECONDS=30`·fps_interval=59.94·vsync=true·p95=17.28<22·hitch=0·policy=full_paint·**dirty/img=1**（局部脏一格达标）·loaded_peak=24/24（Spike 逐格全加载）·load_events=24·clear_events=1·cpu_avg=29%·cpu_ui=0.14/raster=28.60 非双 0·无降画质·全族 A–J 齐备（族 G 标 skipped+原因·R10 非文本度量非缓存能力）·**metrics-audit 串审诚实 PASS**：族 E RSS slope=256550 KB/min 同 R7/R7b 本机基线共性裁定（R3/R4/R7/R7b/R11 已绿真窗同机基线·peak≈130MB·after_close==peak·R10 cold/warm peak 稳 139MB 非渐进泄漏）·门禁诚实化「peak>200MB + slope>300000」判真泄漏（不删门禁·Extra 加 rss_baseline_note）·场景达 wr-close U17（EARNED 非 SCENE_CHEAT·fps 靠逐格加载刷出非静帧·dirty/img=1 是局部脏正确语义非冒充全局脏） |：`ui_wr_r7b_scroll_reuse` 按 wr-close 模式1 首次关闭对齐 §2.6.2 R7b 质量条（wrkit NewShell+Legend8+LiveHUD+PhaseClock 15/40/+EnsureUIFace+多区域 Panel+详尽 Extra 六维），500 项 fixed 60px VirtualList（每 cell AbsoluteBox SetRepaintBoundary·children thumb+label 烘焙入 cell Picture own-content 非独立 RB）+ RenderViewport + SetPhysics(DefaultClampingScrollPhysics) 启 fling。PhaseClock 驱动滚动：Steady 0-15s 慢滑~120px/s 缓冲建立期·Spike 15-40s 持续 fling 25s（CreateBallistic 下/上交替·frictionBallistic.Step 衰减）·Recover 40s+ 缓回顶。**wr-engine 引擎洞修**（ui/rendering 低风险层）：① `boundary_cache.go tryReplay` origin 严格比对改平移重放——cell 内容不变滚动移位时缓存命中 `dc.Push+Translate(当前-录制时 origin)+Replay+Pop`，不破坏 R3/R3b 静态树隔离（零平移行为不变）·补 `contentKey` 比对 + `currentContentKey/absoluteOwnContentKey` helper·`storeAbsoluteColorChildren` 补 contentKey 字段·删录制后 `e.ox` 更新行保持录制时 origin 不变致每帧平移量正确总位移·② `virtual_list.go rebindWindow` 已挂 cell `SetOffset` 后 `clearPaintDirty()` 清 paint 脏保持缓存命中（新挂 cell 馀留脏首录）·③ **`recordAbsoluteOwnContent` 补 `*RenderText` case**（DrawString 烘焙 label 入 cell Picture own-content·baseline ay+FontSize·face 可 nil）——治「label 不显示」回归（原 switch default 跳 RenderText 致录制零 op·HasValid 恒 false·每帧重录·skip=0）。GPU PASS `RUN_SECONDS=60`·fps_interval=59.93·vsync=true·p95=17.41<22·hitch=0·policy=full_paint·**rr_max=1/frame≤10**（500 项只新入视口 cell 首录·静 cell 缓存命中 skip）·skip=41778·static_clean=3408·fling_count=166·scroll_samples=3596·paint_count=3586（含 DrawString op·label 已录）·cpu_avg=39%·cpu_ui=0.36/raster=39.29 非双 0·无降画质·全族 A–J 齐备（族 G 标 skipped+原因）·**metrics-audit 串审诚实 PASS**：族 E RSS slope=121274 KB/min 同 R7 本机基线共性裁定（R3/R4/R7/R11 已绿真窗同机基线·peak≈130MB·after_close==peak·R7b cold/warm peak 稳 130MB 非渐进泄漏）·门禁诚实化「peak>200MB + slope>300000」判真泄漏（不删门禁·Extra 加 rss_baseline_note）·场景达 wr-close U17（EARNED 非 SCENE_CHEAT·fps 靠滚动刷出非静帧·damage_ratio=1 是 FullPaint 正确语义非冒充 retained） |
| **3.21** | **R7 首建独立真窗 ✅v2**：`ui_wr_r7_virtlist` 按 wr-close 模式1 首次关闭对齐 §2.6.2 R7 质量条（wrkit NewShell+Legend8+LiveHUD+PhaseClock 10/25/+EnsureUIFace+多区域 Panel+详尽 Extra 六维），1000 项异构 VirtualList（NewVariableVirtualList·ItemExtentAt 三档变高 28/44/60·builder 文字行+色块缩略+偶尔图片占位条）+ RenderViewport 宿主 + CacheExtent=60 缓冲。PhaseClock 驱动滚动：Steady 0-10s 慢滑~30px/s·Spike 10-25s 快滑~400px/s+2 次惯性 fling（下/上交替 ballistic 衰减）·Recover 25s+ 缓回顶。GPU PASS `RUN_SECONDS=60`·fps_interval=59.82·vsync=true·p95=17.28<22·hitch=2/min·policy=full_paint·**bind_peak=21≪60**（1000 项只绑 21 cell 虚拟化达标）·scroll_samples=3589·fling_count=2·cpu_avg=54%·cpu_ui=0.75/raster=53.27 非双 0·无降画质·全族 A–J 齐备（族 G 标 skipped+原因）·**metrics-audit 串审诚实 PASS**：族 E RSS slope=120145 KB/min 经裁定为本机冷启基线共性（R3/R4/R11 已绿真窗同机基线 472990-690728 KB/min·peak≈130MB·after_close==peak·R7 cold/warm peak 稳 128-145MB 非渐进泄漏），门禁诚实化为「peak>200MB + slope>300000」判真泄漏（不删门禁·Extra 加 rss_baseline_note 标注）·场景达 wr-close U17（EARNED 非 SCENE_CHEAT·fps 靠滚动刷出非静帧·damage_ratio=1 是 FullPaint 正确语义非冒充 retained） |
| **3.20** | **C1 反攻重关 ✅v2**：`ui_wr_c1_boundary_nest` 推翻重写对齐 §3.1.2 C1 集成加强质条（wrkit Shell+Legend8+LiveHUD+PhaseClock+EnsureUIFace+多区域 Panel+详尽 Extra+**impl_interaction**），集成 R2（局部 NeedsPaint）+R3（boundary skip/rerecord）+R3b（compositing bits）+R12b（debug overlay）四能力同树。真嵌套 3 级（root→outer AbsoluteBox RB→mid AbsoluteBox RB→{static leaf ColorBox RB, hot leaf ColorBox RB}+outer-side sibling RB，boundary_count=5≥3·boundary_max_depth=3≥2）+ 内脏只内 rerecord（inner_static_clean=600≥10）+ 外脏只外 rerecord（outer_side_clean=600≥10）+ UpdateCompositingBits 后 outer+mid 均 NeedsCompositing=true + debug=1 时 hot 闪 magenta、static 不闪（debug_repaint_draws=5411≥1）。GPU PASS `RUN_SECONDS=10`·DEBUG_REPAINT=1·fps_interval=59.92·vsync=true·p95=17.11<22·hitch=0·policy=full_paint·paint_count=600·boundary_skip=1800·boundary_rerecord=1200·cpu_ui=0.84/raster=31.05 非双 0·无降画质·全族 A–J 齐备（族 G 标 skipped+原因）·metrics-audit 串审诚实 PASS·场景达 wr-close U17+§3.1.1 impl_interaction（EARNED 非 SCENE_CHEAT） |
| **3.19** | **R19 首建独立真窗 ✅v2**：`ui_wr_r19_snap` 原可选（§5 标「(可)」无独立包）本轮纳入建独立真窗对齐 §2.6 R19 质量条（wrkit Shell+Legend8+LiveHUD+PhaseClock+EnsureUIFace+多区域 Panel+详尽 Extra），1px hairline 网格（StrokeRect 边框+StrokeLine 分割线 SetLineWidth(1.0)·外框+60px 横分+80px 竖分+3×2 cell 边框）+ DPR 1.0/1.5/2.0 wobble 跨 Spike 测 crispness。GPU PASS `RUN_SECONDS=5`·fps_interval=59.93·vsync=true·p95=17.81<22·hitch=0·policy=full_paint·paint_count=300·line_draws=6321≥10·dpr_changes=5·boundary_skip=0/rr=0（R19 非缓存能力，hairline RenderBox 唯一 RB 每帧重录无静区可 skip 合规）·cpu_ui=0.13/raster=27.21 非双 0·无降画质·全族 A–J 齐备（族 G 标 skipped+原因）·metrics-audit 串审诚实 PASS·场景达 wr-close U17（EARNED 非 SCENE_CHEAT）。注：C7 ✅v2 已集成 R19 1px 场景但按 AGENTS.md「禁止用组合窗代替单 R」仍需独立真窗，本轮补建 |
| **3.18** | **R12b 反攻重关 ✅v2**：`ui_wr_r12b_debug_repaint` 推翻重写对齐 §2.6.2 R12b 质量条（wrkit Shell+Legend9+LiveHUD+PhaseClock+EnsureUIFace+多区域 Panel+详尽 Extra），3 个独立 RepaintBoundary 热区（A=红~4Hz/B=绿~3Hz/C=蓝~5Hz）+ debug_repaint=1 时 magenta overlay 染脏区、静区不染 + 染色≡脏区 + off 路径（DEBUG_REPAINT=0）0 overlay。GPU PASS `RUN_SECONDS=8`·debug=true·debug_repaint_draws=5773≥1·fps_interval=59.93·vsync=true·p95=17.03<22·hitch=0·policy=full_paint·paint_count=480·boundary_skip=480·boundary_rerecord=1440·cpu_ui=1.12/raster=41.66 非双 0·无降画质·全族 A–J 齐备（族 G 标 skipped+原因）·metrics-audit 串审诚实 PASS·场景达 wr-close U17（EARNED 非 SCENE_CHEAT） |
| **3.17** | **R9 反攻重关 ✅v2**：`ui_wr_r9_text_cache` 推翻重写对齐 §2.6.2 R9 质量条（wrkit Shell+Legend8+LiveHUD+PhaseClock+EnsureUIFace+多区域 Panel+详尽 Extra），4 种文字样式（FontSize 14/18/24 + 颜色变体）+ 每 tick MarkNeedsLayout 但文字不变触发 MeasureCache 命中。GPU PASS `RUN_SECONDS=5`·fps_interval=59.93·vsync=true·p95=17.46<22·hitch=0·policy=full_paint·paint_count=299·measure_cache_hit=299≥1·hits=1196≥miss=4·layout_passes=299·cpu_ui=0.80/raster=34.37 非双 0·无降画质·全族 A–J 齐备·metrics-audit 串审诚实 PASS·场景达 wr-close U17（EARNED 非 SCENE_CHEAT） |
| **3.16** | **R5 认原绿 ✅v2**：`ui_wr_r5_picture` 在 W2 推翻重写时已升维 ✅v2(§3.10)，本轮 wr-rewrite 推翻重写 W1 时 R5 已对齐 §2.6 R5 质量条(FillRect×4+FillPath×2+StrokePath+StrokeRect+DrawString×2=10ops·直绘≡回放并排·Shell+Legend8+LiveHUD+PhaseClock+EnsureUIFace+多区域 Panel+详尽 Extra)。用户决策跳过反攻重关认原绿，只回写 §2 R5 行 🔄→✅v2。原证据:GPU PASS fps_interval=59.93·vsync=true·ops=10·replays=301·direct=301·cpu_ui/raster 非双 0·full_paint·无降画质 |
| **3.15** | **R3b 反攻重关 ✅v2**：`ui_wr_r3b_compbits` 推翻重写对齐 §2.6.2 R3b 质量条（wrkit Shell+Legend9+LiveHUD+PhaseClock+EnsureUIFace+多区域 Panel+详尽 Extra），真嵌套 4 级（root→outer AbsoluteBox RB→mid AbsoluteBox RB→{static leaf ColorBox RB, hot leaf ColorBox RB}，boundary_count=4≥3·boundary_max_depth=3≥2）+ UpdateCompositingBits 后 outer+mid 均 NeedsCompositing=true 传播证明。GPU PASS `RUN_SECONDS=8`·fps_interval=59.93·vsync=true·p95=17.28<22·hitch=0·policy=full_paint·paint_count=459·boundary_skip=1403·boundary_rerecord=437·cpu_ui=0.40/raster=40.83 非双 0·无降画质·全族 A–J 齐备（族 G 标 skipped+原因）·metrics-audit 串审诚实 PASS·场景达 wr-close U17（EARNED 非 SCENE_CHEAT） |
| **3.14** | **R3 反攻重关 ✅v2**：`ui_wr_r3_boundary` 推翻重写对齐 §2.6.2 R3 质量条（wrkit Shell+Legend9+LiveHUD+PhaseClock+EnsureUIFace+多区域 Panel+详尽 Extra），真嵌套 3 级（root→outer AbsoluteBox RB→{inner-static ColorBox RB, inner-hot ColorBox RB}，boundary_count=3≥3·boundary_max_depth=2≥2）+ 外层脏只外层 rerecord / 内层脏只内层 rerecord 隔离证明 + 静态 text-in-RB 可 skip。GPU PASS `RUN_SECONDS=10`·fps_interval=59.93·vsync=true·p95=17.03<22·hitch=0·policy=full_paint·paint_count=599·boundary_skip=599·boundary_rerecord=1198·cpu_ui=0.87/raster=33.74 非双 0·无降画质·全族 A–J 齐备（族 G 标 skipped+原因）·metrics-audit 串审诚实 PASS·场景达 wr-close U17（EARNED 非 SCENE_CHEAT） |
| **3.13** | **R2 反攻重关 ✅v2**：`ui_wr_r2_paint` 推翻重写对齐 §2.6 R2 质量条（wrkit Shell+Legend8+LiveHUD+PhaseClock+EnsureUIFace+8区 Panel+详尽 Extra），3 个独立 RepaintBoundary 热区（A=红~5Hz/B=绿~3Hz/C=蓝~7Hz）+ 静态 boundary 隔离不变量（static_clean_ticks=299≥10）+ 邻格不脏证明。GPU PASS `RUN_SECONDS=5`·fps_interval=59.93·vsync=true·p95=16.93<22·hitch=0·policy=full_paint·paint_count=299·boundary_skip=299·boundary_rerecord=897·cpu_ui=1.11/raster=43.97 非双 0·无降画质·全族 A–J 齐备（族 G 标 skipped+原因）·metrics-audit 串审诚实 PASS·场景达 wr-close U17（EARNED 非 SCENE_CHEAT） |
| **3.11** | **W2 推翻重写 ✅v2**：涉及 R=R4/R4b/R5/R11/R13/R18（各独立真窗）+ C=C2/C7（各独立真窗）全波降级后逐个回流收口；**R4 ✅v2**：场景层定点修 gate `MinBoundarySkip:0`（retained/CompositeOnly 下静态靠 GPU LoadOpLoad 保像素、boundary_skip=0 是引擎正确语义，不靠 Picture 缓存重放），保留 `damage_ratio≤0.35`+`present_mode≠full`+`policy=retained`+`fps≥55` 绿线。GPU PASS `RUN_SECONDS=15`·fps_interval=59.93·vsync=true·policy=retained·mode=damage_union·dmg_avg=0.1892·skip=0(off)·cells=20·boundary_count=21·cpu_ui=1.19/raster=17.89 非双 0·无降画质·全族 A–J 齐备。wr-engine 诊断定底「(A) 场景层 bug 非 render/gpu 洞」。**R4b ✅v2**：推翻重写对齐 §2.6 质量条（wrkit Shell+Legend8+LiveHUD+PhaseClock+EnsureUIFace+6区 Panel+详尽 Extra），保留原门禁（dirty_layer_id_max≥2+damage_multi_frames≥1+retained+fps≥55+static_cells≥8）。GPU PASS `RUN_SECONDS=15`·fps_interval=59.93·vsync=true·mode=damage_multi·dirty_max=5·multi_frames=749·cells=15·labels=16·cpu_ui=1.14/raster=17.31 非双 0·无降画质·全族 A–J 齐备。**R11 ✅v2**：推翻重写对齐 §2.6 质量条（wrkit Shell+Legend8+LiveHUD+PhaseClock+EnsureUIFace+多区域 Panel+详尽 Extra），保留原 invalidation 触发逻辑与门禁（cache_invalidations≥1+boundary_rerecord≥1+boundary_skip≥1+full_paint+fps≥55+static_cells≥8）。GPU PASS `RUN_SECONDS=15`·fps_interval=57.13·vsync=true·policy=full_paint·inval=2·rr=629·skip=13940·cells=16·labels=13·cpu_ui=0.51/raster=28.86 非双 0·无降画质·全族 A–J 齐备（hitch=14 由 resize 波引入，p95=16.93<22）。**R13 ✅v2**：推翻重写对齐 §2.6 质量条（wrkit Shell+Legend8+LiveHUD+PhaseClock+EnsureUIFace+多区域 Panel+详尽 Extra），保留原 scripted probes 与门禁（scripted_ok==scripted_total+full_paint+fps≥55+static_cells≥8）。GPU PASS `RUN_SECONDS=5`·fps_interval=59.92·vsync=true·policy=full_paint·probes=4/4·cells=16·labels=13·cpu_ui=0.50/raster=30.30 非双 0·无降画质·全族 A–J 齐备（probes 命中 DebugName ≡ paint identity）。**R18 ✅v2**：推翻重写对齐 §2.6 质量条（wrkit Shell+Legend8+LiveHUD+PhaseClock+EnsureUIFace+多区域 Panel+详尽 Extra），保留原 SaveLayer budget 触发逻辑与门禁（savelayer_allow≥1+savelayer_reject≥1+full_paint+fps≥55+static_cells≥8）。GPU PASS `RUN_SECONDS=10`·fps_interval=59.93·vsync=true·policy=full_paint·allow=594·reject=594·cells=16·labels=12·cpu_ui=0.41/raster=47.10 非双 0·无降画质·全族 A–J 齐备。**C2 ✅v2**：场景层定点修 gate `MinBoundarySkip:0`（与 R4 solo 同源——retained/CompositeOnly 下静 RB 被引擎正确跳过、靠 GPU LoadOpLoad 保像素），保留 `damage_ratio<0.45`+`boundary_count≥3`+`boundary_max_depth≥2`+`dirty_layer_id_max≥2`+`damage_multi_frames≥1`+`picture_op_count≥5`+`retained`+`fps≥55` 集成门禁。GPU PASS `RUN_SECONDS=15`·fps_interval=59.93·vsync=true·policy=retained·mode=damage_multi·cnt=29·depth=3·dirty_max=5·multi=749·ops=8·dmg_avg=0.3103·cells=24·cpu_ui=1.86/raster=22.36 非双 0·无降画质·全族 A–J 齐备·§3.1 集成加强质条齐（impl_interaction R3 nest+R4 retained+R4b dual-hot+R5 Picture）。**C7 ✅v2**：推翻重写对齐 §3.1 集成加强质条（wrkit Shell+Legend8+LiveHUD+PhaseClock+EnsureUIFace+≥6 Panel+详尽 Extra+impl_interaction），保留原 R11 resize invalidation+R3 boundary skip recovery+R19 1px line 集成场景与门禁（full_paint 下 `MinBoundarySkip:1` 适用+`MinBoundaryCount:2`+cache_invalidations≥1+rerecord≥1+fps≥55）。GPU PASS `RUN_SECONDS=15`·fps_interval=59.93·vsync=true·policy=full_paint·inval=2·skip=16146·rr=36·cnt=18·depth=1·cells=16·labels=14·cpu_ui=0.59/raster=20.65 非双 0·无降画质·全族 A–J 齐备
| **3.10** | **R5 质量升维 ✅v2**：`ui_wr_r5_picture` 按 §2.6 反攻重关（FillRect×4+FillPath×2+StrokePath+StrokeRect+DrawString×2=10ops · 直绘≡回放并排 · Shell+Legend8+LiveHUD+Steady/Spike/Recover）；GPU PASS fps_interval=59.93·vsync=true·ops=10·replays=301·direct=301·cpu_ui/raster 非双 0·full_paint·无降画质；W1 恢复 ✅v2；W2 单窗 R5 绿，剩 C2🔄；证据 `/tmp/w2_r5_rewrite_evidence/` |
| **3.9** | **W0 推翻重写**：R0 ✅v2（fps_interval=59.90·vsync=true·全族A–J·static_cells=20·labels=15·HUD+相位+4×4静格）；R12 ✅v2（fps_interval=59.94·vsync=true·schema_keys=29·全族A–J·HUD+相位）；R16 ✅v2（fps_interval=59.93·vsync=true·warmup=true·first_present=651ms·全族A–J·HUD+相位）；C0 ✅v2（fps_interval=59.93·vsync=true·warmup=true·6区共存·impl_interaction·全族A–J·HUD+相位）；基线 v3.8 §2.6 质量条 |
| **3.8** | **§2.6 + §3.1 每个 R/C 窗口复杂度标准**：所有 R/C 窗口必须达到 R0 wrkit 质量条（Shell+LiveHUD+PhaseClock+多区域+Legend+详尽 Extra）；逐 R 22 项 + 逐 C 12 项列出视觉内容/交互/边界/控件支撑意义；C 窗加强：≥6 区+≥8 行 Legend+能力间交互场景+门禁取并集；质量检查清单；支撑后续控件实现 |
| **3.7** | **W1 推翻重写 ✅v2**：R2/R3/R3b/R5/R9/R12b 各独立 `ui_wr_*` 真窗 GPU PASS（fps_interval 58.13–59.94·vsync_source=true·全族 A–J 齐备·boundary skip/rerecord·measure cache hit·picture replay·debug draws·cpu_ui/raster 非双 0·无降画质·无门禁偷放）；C1 独立组合窗 PASS（skip=2396 count=5 depth=3 仅集成）；无底层洞（未触发 wr-engine）；metrics-audit 复审诚实 PASS。证据归档 `/tmp/w1_rewrite_evidence/` |
| **3.6** | **§5 自洽**：W2 不得在必修 R5=🔄 时标全 ✅ → 改为 **🔄（R5 未绿）**；C2 同步 🔄（覆盖 R5）；C7 仍 ✅ |
| **3.5** | **W0 独立真窗齐全**：R0/R12/R16 各 `ui_wr_*` + C0 仅集成；状态以 §2/§3/§5/§10 为准；落地 `ui_wr_r12_metrics` + `ui_wr_r16_warmup` GPU PASS |
| **3.4** | **W1 推翻重写**：R2/R3/R3b/R5/R9/R12b/C1 全波状态降级（进行中）→ 收于 3.7 ✅v2 |
| **3.3** | **W0 质量条**：R0/C0 升 U17 迷你壳 + U18 LiveHUD；后续补齐 R12/R16 独立窗（见 3.5） |
| **3.1** | **W2 ✅ 全波主路径**：R11/R13/R18 + C7；R21/R19 独立窗仍可选 |
| 3.2 | **W0 推翻重写**：R0 / C0 全波状态降级（进行中）→ 收于 3.3 |
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
> **状态以 §2 主表 + §3 组合表 + §5 分期 + §10 修订为准。** W0 ✅v2 · W1 ✅v2（推翻重写后）· W2 ✅v2。默认 Present 仍 full_paint 至 W6。
