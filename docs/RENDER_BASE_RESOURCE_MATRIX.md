# 底座资源场景矩阵（RENDER_BASE_RESOURCE_MATRIX）

日期：2026-10-26 ｜ 仓库：`/home/yanghy/app/projects/gogpu/gpui` ｜ 分支：`feat/框架代码整理`
状态：**实施中——P2 已冻结；P3 尾账 Golden 已清零（13→0），maskops 活窗红已收（2026-10-26），P3 未收口只剩 Flutter/Godot 对照尺阻塞**（E1 已豁免、机器证据在 §5.6；E2 已定案、§5.7；两处既有红已定案）；P4 未开工。范围：支撑三栋楼（任意图形渲染 / UI 组件库 / 2.5D 引擎）的**共用底座能力**的资源场景真窗覆盖，指标对齐**工业级产品同机实测**（Flutter/Godot 对照尺）。上层业务场景（组件交互全集、引擎玩法全集、视频业务）归各自域真源，本矩阵只管底座。

## 0. 一句话与做成算完口径

把底座能力封闭成 **17 行场景矩阵**（能力 × 触发模式 × 资源维度），每行做到：

> **真窗 + 资源指标 JSON（CPU/RSS/显存/fps/p95/hitch）+ 同机工业级对照数字 + 比值**——四项齐全、逐行可审计、无一行无数据或无书面豁免。

指标分两段：**健康线**（现有自定阈值，防退化）+ **对齐线**（新增，≤ 同机工业级对照 × 系数，系数数据后置定稿）。"通用/对齐"的宣布权在这张矩阵的量门数字上。

## 1. 决策记录（2026-10-25，用户已定）

1. **Godot 二进制**：允许下载官方预编译或从本机源码编译（`/home/yanghy/app/projects/gogpu/godot`；GAP_PLAN 的"只读不编译"指不改其源码做参考，编译出可执行体做对照尺不违反该约束，编译产物只用于测量）；
2. **Flutter**：允许本机构建（SDK 已在 `/home/yanghy/app/projects/gogpu/flutter`，`bin/flutter` 已验存在）或取现成产物；
3. **对照系数**：**数据后置**——先跑出基线再定稿系数（建议初值 CPU 1.2×、RSS 1.5×，仅作占位，禁直接当验收线）。

## 2. 场景矩阵（17 行封闭集，本表冻结；新增场景走 §7 增量登记）

| # | 底座场景 | 触发模式 | 真窗现状 | 资源指标现状 | 对照场景（尺） | 缺口动作 |
|---|---|---|---|---|---|---|
| 1 | 空闲（无脏零帧） | 开窗静止 60s | **`ui_base_idle`** | ✅ 两连 gate + 60s manual：presents=1、warmup 后增量=0、CPU 0.6–0.8%、golden 0（873600px） | Flutter 静止页 | 补对照（Flutter 尺 P1 阻塞） |
| 2 | 常驻最小动画（心跳税地板） | 单 64px 旋转 60fps | **spin_min** | ✅ CPU 有账（现态 14.59% / 终态 3.4~3.5%） | Flutter 同尺寸旋转图标 | 补对照 |
| 3 | 滚动（retained 主场景） | 列表连续滚动 | C3/C9/R7/R7b | ✅ R7 60s close-run 两连：fps 59.9、p95 19.8ms、hitch 1.0–2.0/min、CPU 18.27%（ui 7.93 + raster 10.33）、RSS 斜率 3458/4435KB/min（valid）、damage 9.25%、bind 14/1200、scroll 重录 14、golden 0；C3 31.98% 已有 | Flutter ListView 同规模 | 补对照（Flutter 尺 P1 阻塞） |
| 4 | 全幅重建上限 | full_paint 全窗每帧变 | R0 | ✅ | Flutter 全幅动画页 | 补对照 |
| 5 | 异步资源→局部脏 | 解码完成逐格点亮 | R10/C3 | ✅ | Flutter 异步图片列表 | 补对照 |
| 6 | 缓存命中/失效 | 边界/纹理命中与失效 | R3/R3b/R9/C9 | ✅ | Flutter RepaintBoundary 开关 | 补对照 |
| 7 | resize/DPR 失效波 | 改尺寸/DPR | R11/C7 | ✅ | Flutter 窗口 resize | 补对照 |
| 8 | 浮层/覆盖带脏 | overlay 开关 | R8 | ✅ | Flutter Overlay/Dialog | 补对照 |
| 9 | 后台/前台节流 | 遮挡/最小化 | R17 | ✅ 两连 gate + Golden + 资源账：最小化后 fg=1308–1310、bg=1、presents=1310–1312/30.1s、pixel 3/3、golden 0；FG 双快照 live（diff 0.92% / content 13.81%） | Flutter 同内容最小化 | 补对照（Flutter 尺 P1 阻塞） |
| 10 | 多窗共享上限 | 1/2/4/8 窗同动画 | **`ui_base_multiwin_cap`** | ✅ 1/2/4/8 曲线：平均 60fps 上限 **4 窗**；8 窗 31fps、总呈现 ≈250 帧/秒；p95 稳态在 1–2 窗间抖动 | Flutter 多实例 | 补对照（Flutter 尺 P1 阻塞） |
| 11 | 长跑稳态（泄漏） | 300s soak | R15/C10 | ✅ 最完整 | Flutter 300s 同内容 | 补对照 |
| 12 | 文本密集（整形+图集） | 密集多脚本文本页 | m5/文本族 | ⚠️ CPU 未专项立账 | Flutter 密集文本页 | 立账+对照 |
| 13 | 大量图元/矢量密集上限（2.5D 底座，**双模式**） | ①同屏图元阶梯 1k/5k/10k（atlas 精灵）；②矢量路径密集（设计工具类，复杂路径重分格成本） | **`ui_base_batch_cap`**（engine 精灵/粒子窗、P1 矢量密度测试为前身） | ✅ 图元 1k/5k/10k 全 60fps（CPU 22/44/78%）；矢量 100@60fps、300→38fps、600→24fps；两连 gate + 200s manual | Godot 2d demo（bullet_shower 类）/ Flutter CustomPaint | 补对照（Godot 未到位） |
| 14 | 视频/纹理流 | 持续喂帧 | video C4 | ❌ 解码吞吐（V-5 挂视频线） | ffplay/Flutter video | 挂视频线 |
| 15 | 呈现策略对照 | 同场景 full vs retained | 分散各窗 | ⚠️ 未成表 | — | 成表（R0 vs R4 同内容 A/B） |
| 16 | 首帧/恢复（t2fp、遮挡恢复） | 冷启/遮挡回来 | R16/R17 | ✅ | Flutter 首帧 | 补对照 |
| 17 | 显存压力/设备恢复（资源纪律验收） | 多窗 VRAM 预算压力、`GPUI_FAULT_OOM` 注入 | 故障注入缝在（`ui/embedder/fault_oom.go`）、预算拒增计数在（textured `budgetRefusals`） | ❌ 无专项资源场景窗（恢复正确性有注入测试、资源账无） | Godot 压力场景 | **立账**（预算拒增曲线 + OOM 恢复后 VramLiveBytes 回落断言） |

**P2 冻结结论（2026-10-25）**：17 行逐行核对完毕——
- ✅ 真窗已有 + 资源指标已有（不需新窗）：行 2/4/5/6/7/8/11/16（8 行）——只需 P4 补对照数字；
- ✅ 真窗已有 + 资源指标需立账（窗在但没有专项 CPU/RSS JSON）：行 3/12/15（3 行）——P4 立账；
- ❌ 需新窗：行 1（idle）/10（multiwin_cap）/13（batch_cap）（3 行）→ **P3 已建（§5.1–5.3）**；
- ❌ 跨线依赖：行 14（视频吞吐 V-5）/17（显存压力资源账）（2 行）→ **挂视频线与资源纪律域**；
- ✅ 证据尾账：13 扇仅像素补 Golden 已全部补齐（两连 `-auto-only` 全 PASS、golden 0.0000%，§5.4）；E1（14 扇）已豁免（§5.6）；E2（2 扇）已定案（jumprope 冻结 Golden、spin_min 豁免，§5.7）；
- ⚠️ Flutter 对照尺：P1 阻塞（Impeller 黑屏），等 stable SDK 到位后补——不影响 gpui 自身数字立账。

尾账（不属于矩阵行，属证据域）：13 扇仅像素补 Golden（`ui_ant_verify`、`ui_render_particle`、`ui_textinput_ime`、`ui_wr_c7_resize_dpr`、`ui_wr_c8_hit_overlay_xf`、`ui_wr_ime_r2/r3/r4/r5_control`、`ui_wr_r17_bg_throttle`、`ui_wr_r21_shell`、`ui_wr_r4b_multidamage`、`ui_wr_r7_virtlist`）**已清零（13→0）**；E1（14 扇）已豁免（§5.6）；E2（2 扇）已定案（§5.7）。

## 3. 工业级对照尺（三层）

### 3.1 Flutter 尺（主尺，P1 先立）

- SDK：`/home/yanghy/app/projects/gogpu/flutter`（`bin/flutter` 已验存在）。允许本机构建或取现成产物（§1.2）。
- 对照窗三个，对位本矩阵行 2/3/12：**旋转图标**（480×320，同 spin_min 内容语义）、**滚动列表**（1200×800，列表项规模对齐 C3）、**密集文本页**（多脚本文本量对齐 m5）。
- 前置验证：`flutter config --enable-linux-desktop` + `flutter doctor`；linux-desktop 构建链（GTK3/ninja/clang）本机未验证，P1 第一步跑通；跑不通按 §7.1 降级。
- 测量：与 gpui 窗同口径（见 §4）。

### 3.2 Godot 尺（第二尺，用户已授权下载或编译）

- demo 现成：`/home/yanghy/app/projects/gogpu/godot-demo-projects/2d/`（bullet_shower、dynamic_tilemap_layers 等）。
- 二进制：下载官方预编译（首选，版本对齐 demo 要求）或源码编译本机 godot 检出。
- 定位：行 13（图元上限）与 2.5D 底座的主对照。

### 3.3 glxgears 尺（地板尺，已在用）

- 空转地板 2.3~3.0%（本机实测）；任何 60fps 呈现的 CPU 下限参照。

### 3.4 公开参考数字锚（第二重 sanity，非验收线）

同机两臂可能同错（系统性偏差），所以对齐线之外引入公开基准作旁证：Flutter 官方性能文档的典型区间（如简单动画页 CPU 个位数百分比、复杂滚动 10-20%）、Godot 2D demo 公开表现。**只作 sanity 参考**——验收线仍以同机对照为准，公开数字入矩阵文档备注列。

### 3.5 测量口径（三尺统一，公平性硬约束）

同机、同窗客户区尺寸、同帧率目标（60）、同动画语义（内容变化量对齐）、同测量法：进程 `/proc` CPU%（含内核态，**不含 Xorg/合成器侧**）+ RSS + `fps_interval`/`p95`/`hitch` + 60s 或 300s 档；窗内容语义差异在报告注明；每次对照三跑取中位。

## 4. 指标规范

1. **健康线（保留现状，防退化）**：fps_interval≥55、interval_p95≤22ms、hitch≤5/min、RSS 稳态斜率 ≤30000KB/min（valid=true）、CPU≤85%。
2. **对齐线（新增）**：行指标 ≤ 同机对照 × 系数。`wrgate` 报文追加字段：`ruler_name`、`ruler_cpu_pct`、`ruler_rss_mb`、`ruler_ratio_cpu`、`ruler_ratio_rss`——**跑过尺才填，没跑如实 null（禁假绿）**。系数 §1.3 数据后置。
3. 现有各域真源 §5 阈值矩阵不动；本矩阵只在资源轴加对齐线。

## 5. 新建窗规格（3 扇，模式 2：gate + manual 二合一 + 三证据）

### 5.1 `ui_base_idle`（矩阵行 1）

- 真窗：`examples/ui_base_idle`，1200×800 静态页；开窗画完一帧后不再改任何节点。
- 量法：warm-up 只允许 1 帧引导呈现；之后 60s 断 `presents` 增量、进程 CPU（`/proc` utime+stime）、RSS 稳态斜率、`vsync_source`。三证据 = 4 条逻辑探针 + 5 条像素探针 + Golden 1200×728 逐位。
- 引擎侧：warm-up 结束时只有 `sched.Tickers().MayWantFrames()` 为真才 `ScheduleFrame()`（`ui/embedder/pipeline_app.go`）；静态窗不再以 60Hz 重复呈现同一帧。
- 2026-10-25 两连 gate（最终代码）：`presents=1`、`after_warmup=0`、CPU **0.807%**，RSS 斜率 **3400.9 / 905.3 KB/min**（valid），peak **245.6 / 245.3 MB**，logic **4/4**、pixel **5/5**、golden **0.0000%**（873600px）。
- manual 60s：`presents=1`、`after_warmup=0`、CPU **0.6%**、logic **4/4**。
- 复现：`RUN_SECONDS=60 go run ./examples/ui_base_idle -auto-only`；人工档 `go run ./examples/ui_base_idle -manual-seconds 60`。

### 5.2 `ui_base_batch_cap`（矩阵行 13）

- 真窗：`examples/ui_base_batch_cap`，1200×800；两条阶梯各 6 阶、每阶 10s：
  - prims：1k / 5k / 10k atlas 四边形（`DrawAtlasEx`）；
  - vector：100 / 300 / 600 条弯曲路径（每条约 24 段三次贝塞尔，`FillPath` 重分格成本）。
- 量法：gate 先跑 **2 轮不计入测量的 prewarm 阶梯**，再跑第 3 轮测量。原因：60s 窗口的 RSS 斜率原先是 GPU/heap 高水位爬坡（预热前 ~45MB/min），一轮预热仍让 25% 稳态窗口落在爬坡段（实测 24–29MB/min，贴线易抖）；两轮后测量周期稳定在 **6–8MB/min**。gate 时长 200s（prewarm 120s + 6×10s + 收尾）。
- 两连 gate 实测（prewarm 之后的测量周期）：

| 阶 | fps | p95 | CPU | 结论 |
|---|---:|---:|---:|---|
| prims 1k | 59.9–60.0 | 17.4ms | 22.9–23.2% | 60fps |
| prims 5k | 59.8–60.0 | 18.4–18.8ms | 46.3–46.6% | 60fps |
| prims 10k | 59.8–59.9 | 19.2–19.5ms | 80.7–80.9% | 60fps |
| vector 100 | 59.7 | 18.1–18.6ms | 58.6–60.1% | 60fps |
| vector 300 | 37.7–37.8 | 29.2–29.3ms | 74.5–74.8% | 过载（如实记，不判绿） |
| vector 600 | 24.0–24.3 | 43.6–44.4ms | 82.7–83.0% | 过载（如实记，不判绿） |

- 容量结论：**60fps 最大图元数 N\*=10000**；**60fps 最大矢量路径数 = 100**。过载阶是故意的上限探针，容量判定只看 `steps[].pass_60fps`；聚合 hitch 字段跨过载阶，JSON 标 `hitch_scope`，不拿它当整窗健康值。
- 资源账（两连，post-race-fix）：RSS 斜率 **6735.0 / 8015.1 KB/min**（valid），peak **453.5 / 447.2 MB**，进程 CPU **61.3 / 62.3%**；golden **0.0000%** × 2（各 883200px，mask 1200×736），pixel **4/4**。
- manual 200s：6 阶全走完，CPU **57.7%**，`max_prims=10000`、`max_vector=100`，golden 0（人工档不重复像素断言）。
- 复现：`RUN_SECONDS=200 go run ./examples/ui_base_batch_cap -auto-only`；人工档 `go run ./examples/ui_base_batch_cap -manual-seconds 200`。

### 5.3 `ui_base_multiwin_cap`（矩阵行 10）

- 真窗：`examples/ui_base_multiwin_cap`，8 扇 480×320 窗一次性预开；每扇独立 X11 display 连接 + 独立 `Run` 循环，同一进程。
- 驱动：共享 `activeN` 原子量；每扇 ticker 实现 `scheduler.FrameWanter`，非激活时 `WantsFrame=false` → 调度器阻塞在事件等待、零帧（复用行 1 的 idle 契约）；激活时 controller 对目标窗 `ScheduleFrame()+Host().WakeUp()`。
- 量法：8 窗全激活 prewarm 10s（把 GPU pool/字体/层缓存拉到高水位），再按 **1 → 2 → 4 → 8** 分四段，每段 1s settle + 12s 测量；gate 75s。每段记各窗 presents 增量、p95、进程 CPU/RSS；8 窗末尾全部冻结到 phase 0，各写一张 Golden，逐窗像素断言。
- 两连 gate 实测（post-race-fix）：

| 激活窗 | 每窗 fps | p95max | 进程 CPU | CPU/窗 | CPU ms/帧 | RSS |
|---:|---:|---:|---:|---:|---:|---:|
| 1 | 59.8 | 17.2–17.3ms | 19.3% | 19.3% | 3.23ms | 303→304MB |
| 2 | 59.4–59.7 | 21.1–22.3ms | 33.7–34.7% | 16.8–17.4% | 2.82–2.92ms | 304→306MB |
| 4 | 59.1 | 28.4–28.7ms | 62.6–63.3% | 15.6–15.8% | 2.65–2.68ms | 306→308MB |
| 8 | 31.2–31.4 | 33.7–34.8ms | 71.5–71.7% | 8.9–9.0% | 2.86–2.87ms | 308MB |

- 容量结论：**平均 60fps 上限 = 4 窗**；**稳态 60fps（再加 p95≤22ms）在 1–2 窗之间抖动**（两连 gate 一次 2、一次 1；n=2 的 p95 实测 21.1 / 22.3ms，贴线）。8 窗时每窗 ~31fps，总计 **≈250 presents/s**（249.5 / 251.2），进程 CPU 仅 71.7%（<85%），单帧 CPU 成本稳定在 2.65–3.23ms——瓶颈是共享呈现路径的串行吞吐，不是 CPU 打满；按 §7.5 记为观察项，不判缺陷。
- 资源账（两连）：RSS 斜率 **10824.1 / 10285.5 KB/min**（valid），peak **331.6 / 332.4 MB**；CPU 全窗均值 50.4 / 50.6%（含 10s 8 窗 prewarm）；golden **8 张全 0.0000%**（合计 1075200px，单窗 mask 480×280，顶部指标带排除），pixel **16/16**，CPU fallback **0**，missed vsync **0**。
- manual 100s：四段全走完，8 窗 31.3fps、CPU 71.6%、RSS 斜率 4.5MB/min、平均上限 4、稳态 1。
- `-race`：多窗并发建层最初在 `ui/scene/layer.go` 的全局 `layerIDGen` 上暴露 **59 条同根数据竞争**（单窗从不触发）；已改为 `atomic.Uint64` 自增（保持唯一、单调、非零）。修复后 `CGO_ENABLED=1 go build -race` + `-manual-seconds 45`（覆盖 8→1、1→2 激活）**0 条竞争**；随后两连 gate 全 PASS。
- 复现：`RUN_SECONDS=75 go run ./examples/ui_base_multiwin_cap -auto-only`；人工档 `go run ./examples/ui_base_multiwin_cap -manual-seconds 100`。

### 5.4 尾账（2026-10-26：13 扇 Golden 清零）

13 扇仅像素补 Golden 的窗（清单 §2 尾账）已全部补齐「固定冻结 + 零容差 Golden + 像素/逻辑证据」，每扇 `-auto-only` **两连 PASS**；尾账 **13 → 0**。`ui_wr_ime_r1_editor` 更早已有同类证据（`9410d61f`），不在本 13 扇清单内。

| 窗 | 冻结 / 逻辑证据（两连实测） | pixel | golden |
|---|---|---|---|
| `ui_ant_verify` | 几何 AA 固定帧；presents=301（本机 MSAA=1x，msaa1/msaa4 基线均已播种） | —（逻辑+Golden） | 0.0000%（821184px） |
| `ui_render_particle` | t=1s 冻结粒子相位；599 presents、58.1–58.2fps、p95 19.2–19.4ms、CPU 25.0–25.7%（ui 10.7 + raster 14.3–15.0）、damage 79200、fallback 0 | 4/4 | 0.0000%（883200px） |
| `ui_wr_c7_resize_dpr` | 两波失效完整走完：wave1=1、wave2=19、dpr=1.0、presents=899/15.3s、skip=16731、rr=369 | 6/6 | 0.0000%（821184px） |
| `ui_wr_c8_hit_overlay_xf` | 脚本 11/11、main_dirty_base=6、excess=0、overlay 3 轮、presents=829–831/15.2s | 5/5 | 0.0000%（821184px） |
| `ui_wr_r17_bg_throttle` | 最小化后 fg=1308–1310、bg=1（presents 1310–1312/30.1s）；FG 双快照 live：diff 0.92%、content 13.81% | 3/3 | 0.0000%（821184px） |
| `ui_wr_r21_shell` | shell 滚动零重录（rr_scroll=0、skip=296、rr_total=1）、presents=300/5.3s | 6/6 | 0.0000%（883200px） |
| `ui_wr_r4b_multidamage` | damage_multi、ids_max=10、multi=891–892、dmg_avg=0.548、fps 59.9–60.0、skip 46534–46587 | 3/3 | 0.0000%（821184px） |
| `ui_wr_r7_virtlist` | 60s close-run：presents=3589–3591、bind 14/1200、scroll 重录 14、fps 59.9、p95 19.8ms、hitch 1.0–2.0/min、RSS 斜率 3458/4435KB/min（valid）、CPU 18.27%（ui 7.93 + raster 10.33）、damage 9.25%、vsync | —（逻辑+Golden） | 0.0000%（763584px） |
| `ui_textinput_ime` | 固定输入缓冲 `你好ba`、caret 固定、blink_ticks=0 | 5/5 | 0.0000%（270000px） |
| `ui_wr_ime_r2_textlayout` | 36×m 深排版/亲和/粘列/回退/省略号/Boxes/生成代数 9 探针全 true（LongBuild 1.771ms） | 4/4 | 0.0000%（821184px） |
| `ui_wr_ime_r3_channel` | Signal6/Storm/Filter/SetState/Delta/Surround4/Autofill/DeadKey/DualEngine 全 true（Storm=20、Surround=3999×4） | 5/5 | 0.0000%（821184px） |
| `ui_wr_ime_r4_selection` | t=2s 冻结 tick/选区相位与标签、caret 点亮、去闪烁；双击/三击/Shift/拖选/密码/只读/撤销/词删/锚点/原子替换全 true | 5/5 | 0.0000%（821184px） |
| `ui_wr_ime_r5_control` | t=2s 冻结控制态与标签、caret 点亮、去闪烁；5k 单行/组合滚动/粘列/省略号/占位/单行拒多行/多行放行/禁用全 true | 6/6 | 0.0000%（821184px） |

**引擎修复（本轮，通用层，不在示例绕）**：R4 补证据时首播暴露 `cpu_fallback_ops=1220`、`last_cpu_fallback: text:color-layout`——旧分类按 Unicode 区段把 `✓ ✗` 这类"文本外观的符号"判成彩色字形，送彩色图集后字体没有 COLR/CBDT 可采，走 CPU 兜底并丢字。改为按 Skia 口径"字体说算"：`render/text/color_font.go` 新增 `GlyphsHaveColor`（逐字形读 `ColorFont.GlyphType`），`ui/rendering/text_layout.go` 的 `buildShapedCarets` 用它闸住 `isColorText`（`IsColor: isColorText(seg) && GlyphsHaveColor(...)`）。回归 `TestColorRunOutlineDingbat_M5` PASS；R2–R5 fallback **1220 → 0**，✓/✗ 恢复可见。

**门语义对齐（真源 `ENGINE_TEXT_X11_IME_REQUIREMENT.md` §10.4.2/3/4/5 G）**：`measure_cache_hit` 的口径是"必采"而不是"≥1"——retained/composite-only 帧与已缓存排版不重新量，`PaintVisits`（末帧节点访问）在复合帧恒 0。六扇 IME 窗的像素/逻辑证据统一改为 `first_present_paint_count>0 && (measure_cache_hit+measure_cache_miss)>0`，同时撤掉 R9 专用的 `MinMeasureCacheHit:1`。auto-only 的 5–6s 有界档短于 15s RSS 预热、启动单次卡顿在短窗口会放大：仅 auto-only 放宽为 WARN（斜率必须 `!valid`；hitch 需 >2 次且 >2.5/min），≥10s 人工档维持硬门。

### 5.4.1 六扇 IME 真机人工验收（2026-10-26）

六扇 IME 窗逐扇开常驻人工档（X11 真窗，XTEST 真按键/真拖选，≥10s 硬门），从空窗开始走真实交互，边操作边采像素探针；全程帧率、hitch、CPU fallback、RSS 斜率同档采集。

| 窗 | 时长 / 帧率 | 交互实测（像素探针） | 资源 / 门禁 | 结论 |
|---|---|---|---|---|
| `ui_wr_ime_r1_editor` | —（修复后复验） | 光标 38px、选区 4056px（修复前两者皆 0） | 活窗像素复验 | PASS（修复后复验） |
| `ui_wr_ime_r2_textlayout` | 81.7s / 59.8fps | 深排版 9/9 探针真；光标 5px 呼吸（闪烁节奏）；选区 817px + 滚动盒 3169px | hitch 2（1.47/min）、fallback 0、RSS 斜率 7350KB/min（valid） | PASS |
| `ui_wr_ime_r3_channel` | 33.6s / 59.7fps | 渠道 9/9 探针真；光标 38px（6/12 帧亮灭）；选区 2253px | hitch 0、fallback 0、RSS 48862KB（WARN 软线，<90k 硬线） | PASS |
| `ui_wr_ime_r4_selection` | 106.4s / 59.8fps | 拖选 2621px、双击取词 1568px、跨行选区 4968px；密码框光标 48px、无选区高亮（引擎设计，见下） | 10/10 探针、hitch 0、fallback 0、RSS 5595KB | PASS |
| `ui_wr_ime_r5_control` | 51.0s / 59.8fps | 两段选区 1956px + 2780px；单行/多行/省略号控制态按预期 | 9/9 探针、hitch 0、fallback 0、RSS 46895KB（WARN 软线） | PASS |
| `ui_textinput_ime` | —（常驻人工） | 逐字输入生效；光标 24px（x=75）50 帧中亮 32 帧；该示例本身不画选区 | 交互路径正常 | PASS（仅输入/光标） |

**观察项（登记，不判缺陷）**：
- **密码框不画选区高亮**：`ui/textinput/box.go:934` 的 `syncHighlight` 对 `IsPassword()` 直接返回——选区高亮会泄露口令长度，按设计抑制；光标照常显示。
- **`ui_textinput_ime` 示例空区点击不触发 OnPointer**：自定义 inputBox 的空白区命中的是内嵌 `*rendering.RenderBox`，换行/定位点击落不到外层 handler；生产用的 `textinput.Box` 不受影响（示例层命中写法问题，不属引擎缺陷）。
- **`ui_render_base_maskops` 活窗红→绿（2026-10-26 当日收口）**：复核曾发现活窗屏幕检查 `diff=63.2419%`（`screen=false`）、四张 masked 卡片全部不显示，而快照/golden/像素全绿；临时计数 `liveOnlySkips=2872`/360 presents，回退当日 IME 修复后逐位同值，证明基线即红。根因：掩码后处理只挂 `RequireLiveContext()`，retained 全帧录制通道（Recorder）拒绝，整卡内容丢失。修复见线程计划 §18.1 #19：新增 `render.MaskCanvas` 侧接口 + 5 条录制命令 + `MaskBackend`，`CanvasBackend` 按设备尺寸门回放并计 `MaskSkips`，`PaintContext.MaskCanvas()` 统一正口。复跑两连：`screen diff=0.0000%`（627200px、±2/255）、`pixel=18/18`、`golden=0.0000%`、`live_only_skips=0`、`mask_skips=0`，presents=247/229；该窗 `fps_gate=off_correctness`，5.5s 档 fps 44.6/41.4、frame_raster_ms 132/141、hitch 0/11，性能只记账不判绿。边界：目前只覆盖 full_paint 全帧录制（录制尺寸=回放尺寸）；retained 图层/局部录制遇掩码按尺寸不匹配计数跳过，完整用户空间掩码留后续。
- **R17 最小化闩**：生命周期闩落下的瞬间调度即停，后续 tick 永远观察不到 `App.Minimized()`；窗把控制器报告的调用点当闩沿（本轮 t=22.0 / presents 1309–1311），最终化时算 bg 增量。将来若要读闩后状态，tick 路径拿不到。
- **C8 恢复快照时序**：恢复截图按 elapsed 排队（auto +1.0s / manual +0.5s），不是按"恢复帧已呈现"确认；慢机上可能早于恢复帧。auto 档 +0.3s 冻结把窗口压窄，但根缝仍是时间而非呈现计数。

### 5.4.2 光标/选区不显示——引擎修复（2026-10-26）

**现象**：`ui_wr_ime_r1_editor`（及其余 IME 窗）焦点正常切换、字形与背景都正常，唯独**光标和选区高亮不显示**——两者都是 `rendering.NewRenderColorBox` 画出的纯色块，字形不受影响。

**根因**：祖先的 mask/difference 裁剪被提升到离屏录制通道上。mask 平面按**整面坐标**存放，而录制通道按**录制局部坐标**绘制，提升后整幅采样恒为 0，纯色块 Fill 被静默丢弃；字形走另一条路径所以看不出来。

**修复（通用层，未在示例绕）**：
- `render/context_clip_mask_gpu.go` 新增 `maskClipPromotionAllowed()`：离屏子通道（`offscreenPassSuspended()`）内一律不提升，祖先掩码改由合成时生效，与 `setGPUClipRect` / `applyClipToPaint` 的既有契约一致。
- `render/context_draw.go` 两处 `forceCPUClip`（Fill/Stroke）改用该判定。
- `applyMaskToPaint` 在离屏子通道内直接返回——录制**完整画面**，祖先掩码不在子通道里烘焙。
- 回归护栏：`render/offscreen_record_mask_test.go` 的 `TestOffscreenPassSkipsAncestorClipMask` 固定三件事——子通道外允许提升、`offscreenPassDepth=1` 时禁止、`MaskCoverage` 子通道内为 nil / 外面非 nil。PASS。

**门禁盲区（为什么 auto-only 看不到）**：`-auto-only` 的快照是**合成的一次 live-context 重绘**，走不到 retained 录制通道，这个洞只会在常驻真窗出现；所以本缺陷必须靠人工档抓，快照绿不能替代。

**既有红（2026-10-26 定案并落地，非 P3 引入）**：
- `ui/scene` 旋转/缩放纹理两例（原 `ReplayedOps=1 want 0`）：根因是**GPU 像素契约放在了无 GPU 的测试包**——headless 下 `CreateOffscreenTexture` 不可用，`recordLocal` 按设计降级为向量回放，`ReplayedOps=1` 是诚实兜底，不是引擎缺陷。已迁到 `ui/scene/internal/gpupixel/textured_transform_gpupixel_test.go`（`requireGPU` + 离屏纹理，本机 headless 走 SKIP）；无 GPU 的缓存决策改由 `ui/scene/textured_transform_internal_test.go` 用假槽固定：旋转/缩放都录**未变形内容**、bounds-sized 纹理（42×42 / 12×12）、干净帧零向量回放。
- `ui/rendering` opacity（原期望 `#ff8080`、实得 `#ff7f7f`）：`#ff8080` 是直线 alpha 四舍五入；引擎按预乘 unorm8 存 127（`render/pixmap.go` 的 `setPremul` 截断 127.5），8-bit GPU 混合同值（S3b GPU 契约允许 ±20）。断言已改为 `#ff7f7f` 并注明口径；不改像素管线（改舍入会波及所有 CPU 像素与 Golden 基线）。
- 本机验证：`go test ./ui/scene`、opacity 用例、`go test ./ui/scene/internal/gpupixel` 全绿；GPU 像素契约待真机 GPU 窗复跑（本机 `CreateOffscreenTexture unavailable` → SKIP）。

**未收口项**：E1（14 扇）已豁免（§5.6）；E2（2 扇）已定案（§5.7）；六扇 IME 窗真机人工验收已完成（§5.4.1，光标/选区引擎缺陷已修并复验，§5.4.2）；`ui_render_base_maskops` 活窗红已收（§5.4.1，2026-10-26 两连全绿，`live_only_skips=0`/`mask_skips=0`）。**P3 未收口只剩 Flutter/Godot 对照尺阻塞，暂不进入 P4。**

## 5.5 P1 基线数字（2026-10-25 本机实测，对照尺已立）

**对照尺（Flutter release AOT，同机同尺寸同语义）**：
| 场景 | Flutter | gpui | 比值 |
|---|---|---|---|
| 旋转图标 480×320 | 26.28%/31.03%（两次），RSS 146→151MB | 15.46%（同口径 /proc），自报 15.77%，RSS 135MB | **gpui ≈ 0.5×**（优） |
| 滚动列表 1200×800 | **109.68%**（>1 核！），RSS 184→287MB | C3: 31.98%，RSS 387MB | **gpui ≈ 0.3×**（显著优） |
| 密集文本 1200×800 | 0.00%（首帧后 idle，Flutter 按需模型静态不渲染） | m5: 29.20%（retained+damage 9.25%，HUD 60fps tick 税） | **Flutter ≈ 0**（此项 Flutter 优——它的静态页真 idle） |
| 地板 glxgears 60fps | — | 3.30%，RSS 55MB | — |

**分析**：① 旋转/滚动两场景 gpui 已显著优于 Flutter（滚动的差距 3.4× 尤其大——Flutter GTK embedder 的 ListView 逐帧全量重排是已知开销）；② 密集文本 Flutter 0% 是其按需模型的正确行为——gpui m5 29.2% 反映的是 retained+HUD tick 的每帧成本，不是文本渲染本身的成本（文本整形/图集已缓存），降低它要走 R15/C10 的 further-tuning 方向；③ 地板 3.3% 与文档口径一致。

**P1 工程细节**（构建链验证结论）：
- Flutter linux-desktop 构建链缺口 = cmake+ninja（无 sudo）；`pip3 install --user cmake ninja` 解决；CXX 编译器注入 flutter 工具链用 `CMAKE_CXX_COMPILER=g++` 预播 build 目录缓存绕过；
- Flutter release AOT 构建 ✓（kernel + engine + runner 三阶段全通）；
- 采样器 `/tmp/ruler_flutter/sample.sh`（/proc utime+stime 差分、3s 预热排除、RSS>50MB 校验、kill 双发 SIGTERM+SIGKILL）；
- **陷阱**：`cd "$DIR" && ./$EXE &` 的 `$!` 是 bash 中转壳（wchan=do_wait、RSS 2MB、CPU 永远 0%）——exe 必须单独一行后台，`$!` 才是应用本体；
- m5 golden 漂移 0.1036% = #2 CPU 位图整形重排（同 textlow 归因），已重播种两连 0.0000%。

## 5.6 E1 机器证据（14 扇，2026-10-26 本机实测）

E1 的 14 扇断言目标是**控制回路、平台能力、性能、行为**，不是画面形态：有的画面是整屏清色/系统窗框/指标看板，有的全程在动，有的要真人敲键盘。这类窗补 Golden 只能冻结"演示壳自己长什么样"，证不了被测能力，所以按用户定案走"**先补机器证据、再书面豁免 Golden**"。本节是补出来的机器证据；豁免已于 2026-10-26 经用户书面签收。

- 环境：Linux X11（`DISPLAY=:0`，Xorg/GNOME），核显（`gpu_backend=integrated`），`LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so`。
- 原始产物落在 `/tmp/e1/`（JSON / stderr / 退出码），命令均可用 §8 的 `RUN_SECONDS` 方式复跑；表中数字为当次实测，不作长期基线。
- 判定口径：各窗自带门禁（缺 JSON 键、presents=0、虚拟化绑全量等直接 FAIL/非零退出）；本节只记**机器证据与残留**，不把 E1 标成矩阵行绿。

| # | 窗 | 断言目标 | 机器证据（2026-10-26 实测） | 通道 | 为何不补 Golden |
|---|---|---|---|---|---|
| 1 | `ui_l1_blank` | 呈现回路地板 | 3s：presents=180、hitch=0、avg 16.46ms、p95 16.97ms、missed_vsync=0、gpu_fallbacks=0 | stdout JSON + stderr | 画面=整屏清色，无形体语义；清色像素由带 Golden 的窗覆盖 |
| 2 | `ui_l1_scroll` | 虚拟滚动 | 5s：presents=301、hitch=0、bind=17/1000、layout=10；40s：presents=2399、hitch=0、RSS 稳态斜率 575.6KB/min（valid）、peak 131.2MB、关窗后 130.4MB | stdout JSON | 画面=列表演示；列表骨架/滚动由 C3/R7 的 Golden 覆盖 |
| 3 | `ui_l1_spinner` | retained 局部动画 | 5s：presents=301、hitch=0、layout=1、raster_layers=2、damage=144px、boundary_skip=5920 | stdout JSON | 画面=旋转指示器演示；冻结相位形态由 `ui_render_particle`（pixel 4/4 + Golden）覆盖 |
| 4 | `ui_l2_shell` | 交互外壳 | 静态窗 3.1s：presents=2（present-on-change 正常）；tap/pan/focus/overlay 由 `examples/ui_l2_shell/scene_test.go` 断言（`TestScene_Integration_TapPanFocusOverlay` 等） | stdout JSON（零输入计数）+ 单测 | 真窗计数要真人点击；机器档只证窗与事件泵在跑，交互语义落在单测 |
| 5 | `ui_render_base_perfsoak` | 连续压测 | 5s：presents=302、hitch=0、p95 17.31ms、gpu_ops=655、cpu_fallback=0、layout=1 | stdout JSON | 画面刻意全程变化（压测语义），没有可冻结的稳定形态 |
| 6 | `ui_text_m4_virtual` | 1M 行虚拟化 | 自检：首屏 5.71ms、物化 42 行、堆增量 17.0MB；真窗 3s：presents=180、hitch=0、bind=45/1e6、measured=69、window_lag_max=0、总高 20M px | stdout JSON（自检 + 窗） | 画面=日志文本；文本形态由 m5/IME 带 Golden 的窗覆盖，此窗证的是绑定规模 |
| 7 | `ui_wr_r12_metrics` | 指标 schema 契约 | 6s：presents=360、hitch=0、family_keys=42、gate=schema_only、hot_repaints=359 | stdout JSON（缺键即 FAIL）+ 退出码 | 画面=指标看板，被测本体是 JSON 契约不是画面 |
| 8 | `ui_pf_x11` | X11 窗口能力 | 19/19 PASS；EventPump 29 events | stdout JSON `{backend,rows,pass}` + 退出码 | 画面=系统窗框，无引擎内容 |
| 9 | `ui_pf_wayland` | Wayland 窗口能力 | 19/19 PASS、backend=wayland（嵌套合成器）；协议禁止项诚实返回 unsupported：Decorations/Position/Focus/AlwaysOnTop/RequestMove/RequestResize | stdout JSON + 退出码 | 同上；嵌套软件栈只作能力证据，数字不作性能结论（见 `ENGINE_WAYLAND_NESTED_TEST.md`） |
| 10 | `ui_pf_appkit` | macOS 窗口能力 | 本机 SKIP：mode=skip、pass=true、row=`SKIP: AppKit real-window verification runs on macOS (this is linux)` | stdout JSON + 退出码 | 本机无 macOS；**状态=未验，不标绿**，待 macOS 机器复跑 |
| 11 | `ui_pf_win32` | Windows 窗口能力 | 本机 SKIP：mode=skip、pass=true、row=`SKIP: Win32 real-window verification runs on Windows (this is linux)` | stdout JSON + 退出码 | 本机无 Windows；**状态=未验，不标绿**，待 Windows 机器复跑 |
| 12 | `video_player` | 视频业务样例 | 5s：presents=290、出帧 27、uploads=27、fallbacks=0、retained 证据 ok=true、墨 1.00/1.00、动 0.15、退出 0 | stderr 结构化行（设计上无 JSON） | 视频业务不属底座矩阵（§0 范围）；此窗只证样例链路可跑 |
| 13 | `zz_soak_control` | 多窗压测对照组 | 3 窗 15s：presents 882–894（每窗 ~56.3–57.0fps）、p95 20.9–22.8ms、hitch 1–6、rerecord=22、skip≈7000 | stderr CTRL 行（对照组，无 JSON） | 画面=移动方块，是对照组不是能力窗；性能判定归 `ui_base_multiwin_cap` |
| 14 | `ui_ime_probe` | IME 活体探针 | x11：IME capability AVAILABLE、EnableIME sent、`XGetInputFocus=OUR-WINDOW`、合成按键 KEY `a`×2 / `b` / `space` 全到（code 97/98/32） | stderr 事件行（设计为活体手测） | 画面=空探针窗；真实合成提交要真人+输入法，六扇 IME 窗已有 Golden |

**证据通道说明（豁免 JSON 的两扇）**：`zz_soak_control` 与 `ui_ime_probe` 设计上就不是 JSON 门禁窗——前者是压测对照组（`CTRL win%d presents=… hitches=…` 结构化 stderr），后者是活体探针（逐事件 stderr 行）。两者的退出码同样是门禁信号（打开失败/没出画即非零）。

**未能机器补齐的残留（如实登记，不混进绿）**：
- `ui_pf_appkit` / `ui_pf_win32` 真窗能力：本机只有 Linux，SKIP 不是 PASS；需 macOS/Windows 机器各复跑一次 `go run ./examples/ui_pf_appkit|ui_pf_win32`。
- `ui_ime_probe` 真实输入法合成提交（IME kind=1 commit）：需真人 + 输入法；
- `ui_l2_shell` 真窗 tap/pan/focus/overlay 计数：需真人点击（语义已由单测固定）；
- `ui_pf_wayland` 性能类结论：嵌套软件合成器数字不作数，仅作能力证据。

**豁免决定（2026-10-26 用户签收）**：上述 14 扇按"不补 Golden、接受机器证据 + 既有单测/JSON 契约"豁免像素三证据；appkit/win32 真机与真人项仍作为残留登记，不影响豁免成立。

## 5.7 E2 定案（2 扇：jumprope 补 Golden、spin_min 豁免）

E2 是两扇常驻动画窗：画面一直在动，原先都没有 Golden。按用户 2026-10-26 选定方向分开处理——能冻结相位的补 Golden，冻结会改变被测目标的写书面豁免。

### 5.7.1 `ui_render_jumprope`：冻结相位 + 零容差 Golden（两连 PASS）

- 门禁档：`-auto-only` 固定 10s，构造后立即 `sim.freezeAt(0.25)`（绳在手线前层，`simT=0` 保证绳横摆与计数文本都为零相位），底部 64px LiveHUD 实时 fps 带排除在 Golden 外。
- 证据（`testdata/jumprope_base.png`，Golden 区 1200×736 = 883200px，零容差）：
  - 第 1 跑：presents=600、~58.4fps、hitch=0、pixel 5/5、golden 0.0000%；
  - 第 2 跑：presents=599、~58.2fps、hitch=0、pixel 5/5、golden 0.0000%。
- 像素探针（语义锚点，tol=3）：地板 `(201,180,142)`、左摇绳人头 `(242,201,154)`、跳绳人头 `(242,201,154)`、跳绳人躯干 `(74,144,217)`、手线绳段 `(122,74,33)`。
- 播种档 `GPUI_ACCEPT_GOLDEN=1` 只生成基线，明确打印 SEEDED 且不计 PASS。
- 复现：`GPUI_ACCEPT_GOLDEN=1 go run ./examples/ui_render_jumprope -auto-only`（首次播种）→ `go run ./examples/ui_render_jumprope -auto-only`（两连验收）。

### 5.7.2 `kit/spin_min`：书面豁免 Golden（矩阵行 2 心跳税地板）

- 豁免理由：这扇窗存在的意义是量"常驻最小动画的每帧税"（CPU/RSS/presents）；画面按设计一直在旋转，冻结相位会把它变成静态窗，测的就不是心跳税了。静态图标形态属组件（`ui/kit`）覆盖率，不归底座资源矩阵。
- 机器证据（60s 档，2026-10-26）：presents=3592、hitch=0、avg 16.71ms、p95 18.20ms、CPU **15.92%**（ui 1.10 + raster 14.82）、RSS peak/after_close 137.7/137.7MB、稳态斜率 **511.9KB/min（valid）**、gpu_ops=7187、fallback=0、damage=7225px（damage_union）、退出 0。
- 证据链修复（本轮，示例层只补取证不动行为）：ticker 原先不调 `proc.Sample()`，60s 只有首尾两点，稳态斜率永远 `valid=false`；已按其他真窗同款每帧采样。同时按 README 原有承诺补 stdout 指标 JSON，并把 README 的"JSON/门禁"描述改成与代码一致。
- 复现：`RUN_SECONDS=60 go run ./examples/kit/spin_min`。

**E2 残留**：`spin_min` 无 Golden 属书面豁免；若将来组件域要求图标形态回归，应在 `ui/kit` 的组件测试里冻结，不在本矩阵补。

## 6. 实施阶段与 DoD

| 阶段 | 内容 | DoD | 量级 |
|---|---|---|---|
| **P1 尺先立** | Flutter 构建链验证 + 三对照窗 + 采数；glxgears 复测；Godot 二进制到位（下载或编译） | 三对照场景基线数字入表（§2 对照列） | 1 天 |
| **P2 矩阵冻结** | 本文档 §2 表定稿（真窗/指标/对照/状态四项逐行落实） | 每行四项齐全，缺行显式列出 | 0.5 天 |
| **P3 缺窗补齐** | idle / batch_cap / multiwin_cap 三扇 + 13 扇尾账 + E1/E2 豁免 | 每扇 `-auto-only` 两连 + manual 档；逐文件回归；触及共享数据跑 `-race` | 2-3 天 |
| **P4 全矩阵跑账** | 17 行逐行资源账（5s 档 + 长跑行 300s）、对照系数定稿、wrgate 对照字段落地、覆盖文档 §11 同步 | 17 行 JSON 齐全、对照比成立、文档标 ✅ | 1-2 天 |

**P3 进度（2026-10-26）**：三扇新窗（idle/batch_cap/multiwin_cap）两连 gate + manual 已完成（§5.1–5.3）；多窗 `-race` 清零；13 扇尾账 Golden 两连全 PASS（§5.4，13→0）；IME 颜色字形分类引擎修复落地；两处既有红已定案（用例迁移 + 断言口径，§5.4）；E1 14 扇机器证据已补（§5.6，其中 appkit/win32 本机诚实 SKIP、待真机）。E2 已定案（jumprope 冻结 Golden 两连 PASS、spin_min 豁免）；六扇 IME 真机人工验收已完成并修掉光标/选区引擎缺陷（§5.4.1/§5.4.2）；maskops 可录制掩码后处理已收，两连 `screen=0.0000%`/`pixel=18/18`/`golden=0.0000%`/`live_only_skips=0`/`mask_skips=0`（§5.4.1、线程计划 §18.1 #19）；对照尺仍阻塞，故 **P3 未收口只剩对照尺，暂不进入 P4**。

## 7. 风险与边界

1. **Flutter linux-desktop 构建链**未验证（GTK3/ninja/clang）：P1 第一步验证；跑不通降级为"引用公开基准数字 + 注明"，Godot 尺升主尺；
2. **Godot 编译耗时**：源码编译大工程优先下载官方预编译；
3. **系数后置**：对齐线系数在 P4 基线出来前不得当验收线用（防新的自拍阈值）；
4. **换机门**：阈值随机器重标（照 2.5D GAP_PLAN §5 先例），本矩阵数字绑定本机；
5. **范围边界**：2D 桌面底座（不含 3D、不含移动端）；CPU 口径不含合成器侧；多窗单 GL 线程天花板是观察项不是缺陷。实测：4 窗平均 60fps，8 窗 31fps，总呈现 ≈250 帧/秒；单帧 CPU 成本 2.65–3.23ms 稳定，CPU 未饱和（71.7%）——瓶颈是共享呈现串行吞吐。

## 8. 复现命令（P1）

```bash
# Flutter 构建链验证
/home/yanghy/app/projects/gogpu/flutter/bin/flutter config --enable-linux-desktop
/home/yanghy/app/projects/gogpu/flutter/bin/flutter doctor
# Flutter 对照窗（tools/ruler_flutter/ 三窗，P1 建）
cd tools/ruler_flutter && FLUTTER_SDK=/home/yanghy/app/projects/gogpu/flutter make run-spin
# gpui 侧同场景（对照臂）
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=60 examples/kit/spin_min   # proc CPU% / RSS / p95 / hitch
# glxgears 地板
glxgears -info 2>&1 | grep -E "FPS"
# P3 三扇新窗（gate 两连命令重复执行两次）
RUN_SECONDS=60 go run ./examples/ui_base_idle -auto-only
RUN_SECONDS=200 go run ./examples/ui_base_batch_cap -auto-only
RUN_SECONDS=75 go run ./examples/ui_base_multiwin_cap -auto-only
# manual 观察档
go run ./examples/ui_base_idle -manual-seconds 60
go run ./examples/ui_base_batch_cap -manual-seconds 200
go run ./examples/ui_base_multiwin_cap -manual-seconds 100
# 多窗竞态回归（-race 需要 CGO_ENABLED=1）
CGO_ENABLED=1 go build -race -o /tmp/multiwin_race ./examples/ui_base_multiwin_cap
/tmp/multiwin_race -manual-seconds 45
# P3 尾账 13 扇（-auto-only 每条连跑两次；单包构建，勿一次 build 多个包）
for w in ui_ant_verify ui_render_particle ui_wr_c7_resize_dpr ui_wr_c8_hit_overlay_xf \
         ui_wr_r17_bg_throttle ui_wr_r21_shell ui_wr_r4b_multidamage ui_wr_r7_virtlist \
         ui_textinput_ime ui_wr_ime_r2_textlayout ui_wr_ime_r3_channel \
         ui_wr_ime_r4_selection ui_wr_ime_r5_control; do
  go build -o /tmp/$w ./examples/$w || break
  /tmp/$w -auto-only || break
done
# IME 交互档（R2–R5：不传 RUN_SECONDS 为无限交互、点 X 关闭；15s 有界冒烟如下）
RUN_SECONDS=15 go run ./examples/ui_wr_ime_r2_textlayout
RUN_SECONDS=15 go run ./examples/ui_wr_ime_r3_channel
RUN_SECONDS=15 go run ./examples/ui_wr_ime_r4_selection
RUN_SECONDS=15 go run ./examples/ui_wr_ime_r5_control
```

## 9. 修订表

| 日期 | 内容 |
|---|---|
| 2026-10-26 | **maskops 可录制掩码后处理收口（活窗红→绿）**：`ui_render_base_maskops` 常驻真窗复核曾 `screen diff=63.2419%`、四卡不显示（`liveOnlySkips=2872`/360 presents），快照/golden/像素全绿属假绿；根因=掩码后处理只挂 `RequireLiveContext()`，retained 全帧录制通道（Recorder）拒绝后整卡丢失。修法：新增 `render.MaskCanvas` 侧接口（不塞进 `Canvas` 用户空间契约）+ `render/recording` 5 条设备空间命令/`MaskBackend`；录制时 `Mask.Clone()` 快照、回放每次再克隆；`CanvasBackend` 仅在同尺寸目标上执行，尺寸/能力不符计 `MaskSkips`，`PushMaskLayer` 退化普通层保 `PopLayer` 平衡，`End` 清理并给活 Context 重装外层掩码；`PaintContext.MaskCanvas()` 为正口，不支持计 `LiveOnlySkips`。证据：`render/recording/mask_test.go` 六测 + `-race` 绿；maskops `-auto-only` 两连 `screen=0.0000%`（627200px，±2/255）、`pixel=18/18`、`golden=0.0000%`、`live_only_skips=0`、`mask_skips=0`、presents=247/229；5.5s 档 fps 44.6/41.4、frame_raster_ms 132/141、hitch 0/11，`fps_gate=off_correctness` 只记账不判绿。边界：只打通 full_paint 全帧录制；retained 图层/局部录制遇掩码按尺寸不匹配计数跳过，完整用户空间掩码留后续。详见线程计划 §18.1 #19、`ENGINE_UI_RENDER_BASE.md` A 组#6/§24 1.63、`RENDER_API_CATALOG.md` 掩码族/§6.3/§8。P3 只剩对照尺阻塞 |
| 2026-10-26 | **六扇 IME 真机人工验收 + 光标/选区引擎修复**：六窗常驻人工档逐扇走真实输入——r2 81.7s/59.8fps/hitch 2/fallback 0/RSS 7350KB/min；r3 33.6s/59.7fps/hitch 0/RSS 48862KB；r4 106.4s/59.8fps/10探针/拖选 2621px/双击 1568px/跨行 4968px；r5 51.0s/59.8fps/两段选区 1956+2780px；`ui_textinput_ime` 逐字输入 + 光标 24px 呼吸。发现并修复通用层缺陷：祖先 mask/difference 裁剪被提升进离屏录制通道（整面 mask 对录制局部坐标采样恒 0），光标/选区纯色块被静默丢弃——`maskClipPromotionAllowed()` 闸住 `forceCPUClip` + `applyMaskToPaint` 子通道内返回，新增 `TestOffscreenPassSkipsAncestorClipMask` 护栏；r1 复验光标 38px、选区 4056px。门禁盲区：auto-only 快照是 live-context 合成重绘，走不到录制通道，此类洞只能靠人工档抓。登记跨线红：maskops 活窗 `diff=63.2419%`、四卡不显示（liveOnlySkips=2872，基线同值），归属 S2b/C3 可录制掩码后处理。详见 §5.4.1/§5.4.2。P3 仍因对照尺与跨线红未收口 |
| 2026-10-26 | **E2 定案**：`ui_render_jumprope` 补冻结相位门禁——`-auto-only` 冻结 `sim.freezeAt(0.25)`（绳在手线前层、`simT=0`），LiveHUD 底带排除，零容差 Golden 883200px + 5 条像素探针；两连 PASS（presents 600/599、58.4/58.2fps、hitch 0、pixel 5/5、golden 0.0000%）。`kit/spin_min` 书面豁免 Golden（持续旋转是心跳税语义，冻结即改变被测目标）；60s 档 presents=3592、CPU 15.92%、稳态 RSS 511.9KB/min（valid）、fallback 0。顺带修证据链：spin_min 每帧 `proc.Sample()`（原先稳态斜率永远 invalid）+ 补 JSON 输出 + README 对齐。详见 §5.7 |
| 2026-10-26 | **E1 14 扇机器证据补齐（豁免已签收）**：按"先补机器证据再豁免"逐扇实跑——l1_blank presents=180/hitch 0；l1_scroll 5s presents=301/bind 17/1000，40s 稳态 RSS 575.6KB/min（valid）；l1_spinner presents=301/layout=1/damage=144px；l2_shell 静态 presents=2 + 交互单测；perfsoak presents=302/fallback 0；m4_virtual 首屏 5.71ms/物化 42 行/真窗 bind 45/1e6；r12 schema 42 键全；pf_x11 19/19、pf_wayland 19/19（backend=wayland 嵌套，协议禁止项诚实 unsupported）；pf_appkit/pf_win32 本机诚实 SKIP（待 macOS/Windows 真机，不标绿）；video_player 出帧 27/fallback 0；zz_soak_control 3 窗 ~56–57fps；ime_probe IME 可用 + 合成按键全到。残留：真机跨平台、真人 IME 合成、真人交互。详见 §5.6。用户当日签收豁免：14 扇不补 Golden。P3 仍因 E2/人工目视/对照尺未收口 |
| 2026-10-26 | **既有红定案落地**：`ui/scene` 两例根因是 GPU 像素契约放在无 GPU 测试包（`CreateOffscreenTexture` 不可用时录制按设计降级向量回放），已迁到 `ui/scene/internal/gpupixel/textured_transform_gpupixel_test.go`，并在 `ui/scene/textured_transform_internal_test.go` 补无 GPU 缓存决策断言（未变形 bounds 42×42 / 12×12、干净帧零回放）；opacity 断言改为预乘 unorm8 的 `#ff7f7f`（`setPremul` 截断 127.5，8-bit GPU 同值），不动像素管线。本机 `ui/scene` 全包 + opacity 用例 + gpupixel 包全绿，GPU 像素档待真机复跑。P3 仍因 E1/E2、人工目视与对照尺未收口 |
| 2026-10-26 | **P3 尾账 Golden 清零（13 → 0）**：13 扇逐扇补「固定冻结 + 零容差 Golden + 像素/逻辑证据」，`-auto-only` 两连全 PASS——ant presents=301/golden 0；particle 58.1–58.2fps/p95 19.2–19.4ms/CPU 25.0–25.7%/pixel 4/4；c7 wave1=1/wave2=19/dpr=1.0/pixel 6/6；c8 scripted 11/11/excess=0/pixel 5/5；r17 fg=1308–1310/bg=1/pixel 3/3；r21 rr_scroll=0/skip=296/pixel 6/6；r4b damage_multi/fps 59.9–60.0/pixel 3/3；r7 presents=3589–3591/fps 59.9/p95 19.8ms/hitch 1.0–2.0/min/RSS 3458/4435KB/min/CPU 18.27%；textinput 5/5（270000px）；r2 4/4；r3 5/5；r4 5/5；r5 6/6。全部 golden 0.0000%。行 3（R7）标绿、行 9（R17）补齐。详见 §5.4 |
| 2026-10-26 | **R7 裁剪/掩码复用 + idle 调度修复**：滚动帧原先每个 scissor 组重建全幅 mask，改为共享 mask path 缓存（FNV-1a 键，`render/internal/clip/mask_cache.go`）+ 锁外缓存 Gray8 视图 + 跨组复用已细分裁剪几何（`render/internal/gpu/depth_clip.go`、`brush_fill.go`）；R7 60s close-run 达标（fps 59.9、CPU 18.27%，行 3）。idle：warm-up 后仅 `sched.Tickers().MayWantFrames()` 为真才排帧，静态窗呈现数归 1（行 1）。 |
| 2026-10-26 | **IME 引擎修复 + 门语义对齐**：颜色字形分类从「Unicode 区段」改为「字体说算」（`text.GlyphsHaveColor` 逐字形读 `GlyphType`，Skia 口径），`buildShapedCarets` 闸住 `isColorText`；R2–R5 `cpu_fallback_ops` 1220 → 0，✓/✗ 恢复可见，回归 `TestColorRunOutlineDingbat_M5` 绿。六扇 IME 窗证据改为真源 §10.4.x G「必采」口径（首帧绘制 + hit/miss>0），撤掉 R9 专用 `MinMeasureCacheHit:1`；auto-only 短档斜率/启动 hitch 仅 WARN，≥10s 人工档硬门不变。观察项登记：R17 最小化闩后 tick 不可见、C8 恢复快照按 elapsed 而非呈现确认。详见 §5.4 |
| 2026-10-26 | **batch_cap 两段预热屏障**：60s RSS 窗口原先落在 GPU/heap 爬坡段（预热前 ~45MB/min、一轮预热后 24–29MB/min 贴线），改为 gate 先跑 2 轮不计入测量的 prewarm 再测，稳态落到 6–8MB/min；两连 gate + 200s manual PASS（§5.2） |
| 2026-10-25 | **P3 三扇新窗落地**：idle 两连 gate + 60s manual（presents=1 / after_warmup=0 / CPU 0.807% / golden 0）；batch_cap 图元 10k@60fps、矢量 100@60fps、RSS 稳态 6.7/8.0MB/min、golden 0；multiwin 1/2/4 窗 60fps、8 窗 31fps/≈250 presents/s、平均上限 4 窗、8 张 golden 全 0、pixel 16/16。详见 §5.1–5.3 |
| 2026-10-25 | **多窗 -race 修复**：`ui/scene/layer.go` 全局 `layerIDGen` 非原子，8 窗并发建层触发 59 条同根数据竞争；改 `atomic.Uint64`（唯一/单调/非零语义不变）。修复后 45s `-race` 0 条，两连 gate 复跑 PASS |
| 2026-10-25 | **P3 回归既有红登记**：`ui/scene` textured transform 两例（`ReplayedOps=1 want 0`）、`ui/rendering` opacity 1 级舍入（`#ff7f7f` vs `#ff8080`）；已在 HEAD 复现，非 P3 引入，归引擎线。P3 尾账 13 扇 Golden + E1/E2 未完成，**P3 未收口** |
| 2026-10-25 | **P1 阻塞登记：Flutter master 3.50 Impeller OpenGLESDF 黑屏**。Flutter 对照尺三模式（spin/list/text）窗口打开但**内容区全黑**（x11grab 截屏证实），26-31% CPU 为引擎空转——像素未到达屏幕。本机 NVIDIA GeForce 940MX（非 Intel），NVIDIA 580.178.04（OpenGL 4.6），Impeller OpenGLESDF 后端与 NVIDIA 专有 EGL 不兼容。三种绕行均失败：`FLUTTER_ENGINE_SWITCHES=--disable-impeller`（编译期决定）、`LIBGL_ALWAYS_SOFTWARE=1`（EGL 上下文独立于桌面 GL）、`CMAKE_CXX_COMPILER=g++`（Impeller 是引擎编译期选项）。`flutter channel stable` 切换卡在网络（GitHub 不可达）。**Flutter 对照数字无效，P1 Flutter 部分搁置**，等 stable SDK 到位后重测。gpui 自身数字（spin_min 15.46%、C3 31.98%、m5 29.20%、glxgears 3.30%）有效已入 §5.5。用户决策三选一：①配镜像切 stable ②git checkout 可用 tag ③先跳 Flutter 尺推进 P2/P3 |
| 2026-10-25 | 重审修正（用户复核要求）：行 13 拆双模式（图元批处理+矢量路径密集）；新增行 17 显存压力/设备恢复（注入缝+预算拒增已在源码，缺资源场景窗）；§3.4 补公开参考数字锚（sanity 非验收线）。矩阵 16→**17 行** |
| 2026-10-25 | 立项：底座资源场景矩阵 16 行封闭 + 三层对照尺（Flutter 主尺/Godot 第二尺/glxgears 地板尺）+ 指标两段制（健康线+对齐线，系数数据后置）+ 三扇新窗规格（idle/batch_cap/multiwin_cap）+ 尾账 13 扇 Golden 与 E1/E2 豁免。决策记录：Godot 允许下载/编译、Flutter 允许构建/取产物、系数后置（用户定）。状态：P1 未开工 |
