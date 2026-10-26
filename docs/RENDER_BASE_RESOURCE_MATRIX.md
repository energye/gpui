# 底座资源场景矩阵（RENDER_BASE_RESOURCE_MATRIX）

日期：2026-10-26 ｜ 仓库：`/home/yanghy/app/projects/gogpu/gpui` ｜ 分支：`feat/框架代码整理`
状态：**实施中——P2 已冻结；P3 尾账 Golden 已清零（13→0），但 P3 未收口**（E1/E2 豁免未定案、两处既有红归引擎线、Flutter 尺阻塞）；P4 未开工。范围：支撑三栋楼（任意图形渲染 / UI 组件库 / 2.5D 引擎）的**共用底座能力**的资源场景真窗覆盖，指标对齐**工业级产品同机实测**（Flutter/Godot 对照尺）。上层业务场景（组件交互全集、引擎玩法全集、视频业务）归各自域真源，本矩阵只管底座。

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
- ✅ 证据尾账：13 扇仅像素补 Golden 已全部补齐（两连 `-auto-only` 全 PASS、golden 0.0000%，§5.4）；E1（14 扇）/E2（2 扇）豁免仍待书面定案；
- ⚠️ Flutter 对照尺：P1 阻塞（Impeller 黑屏），等 stable SDK 到位后补——不影响 gpui 自身数字立账。

尾账（不属于矩阵行，属证据域）：13 扇仅像素补 Golden（`ui_ant_verify`、`ui_render_particle`、`ui_textinput_ime`、`ui_wr_c7_resize_dpr`、`ui_wr_c8_hit_overlay_xf`、`ui_wr_ime_r2/r3/r4/r5_control`、`ui_wr_r17_bg_throttle`、`ui_wr_r21_shell`、`ui_wr_r4b_multidamage`、`ui_wr_r7_virtlist`）**已清零（13→0）** + E1（14 扇）/E2（2 扇）豁免书面定案。

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

**观察项（登记，不判缺陷）**：
- **R17 最小化闩**：生命周期闩落下的瞬间调度即停，后续 tick 永远观察不到 `App.Minimized()`；窗把控制器报告的调用点当闩沿（本轮 t=22.0 / presents 1309–1311），最终化时算 bg 增量。将来若要读闩后状态，tick 路径拿不到。
- **C8 恢复快照时序**：恢复截图按 elapsed 排队（auto +1.0s / manual +0.5s），不是按"恢复帧已呈现"确认；慢机上可能早于恢复帧。auto 档 +0.3s 冻结把窗口压窄，但根缝仍是时间而非呈现计数。

**既有红（复现于 HEAD，归引擎线，未计入 P3 绿）**：
- `ui/scene`：`TestCompositeFramePacketTextured_TransformNoTextureCache`、`TestCompositeFramePacketTextured_ScaleNoTextureCache`，均 `ReplayedOps=1 want 0`（`textured_transform_test.go:43/89`）；
- `ui/rendering`：`TestRenderOpacity_PaintCompositesGroup`，期望 `#ff8080`、实得 `#ff7f7f`（1/255 舍入差，`opacity_test.go:34`）。
  两条已把 `layer.go` 临时还原到 HEAD 复现，非 P3 引入；修好或书面豁免前，P3 不能宣称全绿。

**未收口项**：E1（14 扇）/E2（2 扇）豁免仍待书面定案；六扇 IME 窗人工档待真机目视（冻结门仅 `-auto-only` 生效，交互路径不变）；Flutter/Godot 对照尺仍阻塞。**P3 未收口，暂不进入 P4。**

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

## 6. 实施阶段与 DoD

| 阶段 | 内容 | DoD | 量级 |
|---|---|---|---|
| **P1 尺先立** | Flutter 构建链验证 + 三对照窗 + 采数；glxgears 复测；Godot 二进制到位（下载或编译） | 三对照场景基线数字入表（§2 对照列） | 1 天 |
| **P2 矩阵冻结** | 本文档 §2 表定稿（真窗/指标/对照/状态四项逐行落实） | 每行四项齐全，缺行显式列出 | 0.5 天 |
| **P3 缺窗补齐** | idle / batch_cap / multiwin_cap 三扇 + 13 扇尾账 + E1/E2 豁免 | 每扇 `-auto-only` 两连 + manual 档；逐文件回归；触及共享数据跑 `-race` | 2-3 天 |
| **P4 全矩阵跑账** | 17 行逐行资源账（5s 档 + 长跑行 300s）、对照系数定稿、wrgate 对照字段落地、覆盖文档 §11 同步 | 17 行 JSON 齐全、对照比成立、文档标 ✅ | 1-2 天 |

**P3 进度（2026-10-26）**：三扇新窗（idle/batch_cap/multiwin_cap）两连 gate + manual 已完成（§5.1–5.3）；多窗 `-race` 清零；13 扇尾账 Golden 两连全 PASS（§5.4，13→0）；IME 颜色字形分类引擎修复落地。E1/E2 豁免、两处既有红、人工目视与对照尺仍欠，故 **P3 未收口，暂不进入 P4**。

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
