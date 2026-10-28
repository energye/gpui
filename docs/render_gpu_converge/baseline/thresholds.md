# 收敛验收阈值与环境清单（P0.1）

> 用途：P1 基线采集（P1.1–P1.8）与 P2 行为零变化重放（P2.6）的**唯一数值口径**。凡判红判绿，以本文件的数字为准；本文件没写的数字，不得临时自定。
>
> 规则：
> 1. 每个数字必须带来源。来源列写「文档 §章节」或「实测窗名 + 日期」，两类都不是的不许写进来。
> 2. 标 **[治理值]** 的条目不是上游真源的移植，是本收敛计划自己定的约定，写出推导过程，允许用户直接改数字。
> 3. 不留「待定」。本轮确认不了的，写成明确的「只观测、不设门禁」并写明理由。
> 4. 改本文件任何数字，必须在 §9 修订表记一笔，并同批重跑负向对照（§8）。

---

## 1 测量前置条件

### 1.1 基线机（唯一采集机）

| 项 | 本次值 | 采集方式 |
|---|---|---|
| 主机 / 内核 | Linux yanghy-pc 6.8.0-138-generic x86_64 | `uname -srmo` |
| 发行版 | Ubuntu 22.04.5 LTS | `/etc/os-release` |
| CPU | Intel i5-6200U，4 线程 | `nproc` |
| 内存 | 7 GB | `free -g` |
| GPU | NVIDIA GeForce 940MX，1024 MiB | `nvidia-smi --query-gpu=name,memory.total` |
| 驱动 | 580.178.04 | 同上 |
| Go | go1.25.5 linux/amd64 | `go version` |
| CGO | 0（纯 Go + purego） | `go env CGO_ENABLED` |
| 会话 / 显示 | Wayland；`DISPLAY=:1`（XWayland）；`WAYLAND_DISPLAY=wayland-0` | 环境变量 |
| 仓库起点 | `876b7835`（P0.1 开工时的 HEAD） | `git rev-parse --short HEAD` |
| 已有 tag | 无（P0.3 建演练 tag，正式基线 tag 在 P1.8 通过后打） | `git tag --list` |

**基线绑机规则**：性能类数字（§3、§4 时序、§5）只在本机可比。换机重采必须新建一份 baseline 目录并在文件名注明机型，不得混用。

### 1.2 运行纪律（每次采集都要遵守）

| 项 | 要求 | 来源 |
|---|---|---|
| 构建档位 | 只认正式 release 包；`-race`、插桩、debug 数字只作正确性参考，**不得**当性能基线 | `docs/RENDER_2_5D_GAP_PLAN.md` §5（「只认正式 release 包」） |
| 预热 | 预热 5 秒丢弃，正式段才开始计数 | 同上 |
| 重复次数 | 跑 3 遍，去掉最好和最差，取最差那遍判（只贴最好那遍 = 假绿） | 同上 |
| 逐文件/逐窗 | 每个 `*_test.go` 单独跑、一个跑完再跑下一个；每个真窗单独跑，禁止一次全量 | `AGENTS.md`「测试数据与阶段回归」；本计划 TASKS §3 P1.3 |
| 窗口尺寸 | 客户区 1200×800 逻辑像素 | `docs/ENGINE_UI_WIDGET_RENDER.md` U15（:6、:142） |
| 最短时长 | `RUN_SECONDS ≥ 5`，实现侧 `<5` 直接 FAIL；关闭用时长按该窗档位 | 同上 U16（:36、:143、:214） |
| 参考时长档 | 动画 30s、虚拟列表 60s、soak 300s | 同上 §2.5（:236–:239） |
| vsync 来源 | 必须输出 `vsync_source`；`fallback` 禁止宣称锁 60Hz | 同上 U13（:33）、§2.2.2（:134 后文） |
| 批量/长跑安全 | 批量开窗、长循环压测必须在隔离显示环境；单窗验收可按 `AGENTS.md` 当桌面规则执行 | `AGENTS.md`「GPU 真窗与压测安全」 |

### 1.3 环境核对清单（开跑前逐项打勾）

- [ ] 显示环境已确认（单窗当前桌面 / 批量隔离环境），并记录 `DISPLAY`、`WAYLAND_DISPLAY`、`XDG_SESSION_TYPE`
- [ ] `vsync_source` 能取到（X11 走 DRM vblank，Wayland 走 frame callback 或 DRM 打点）
- [ ] GPU 驱动版本与 §1.1 一致；不一致则本批数字标「非基线机」
- [ ] `GPUI_VRAM_BUDGET_MB` 未设置（走默认 768），或用例显式声明了覆盖值
- [ ] 被测窗金图路径存在，容差已在窗 README 声明（未声明按零容差，见 §6）
- [ ] 输出目录可写，JSON + PNG 落盘成对
- [ ] 上一轮进程已正常退出（`WM_DELETE` → `Quit`），无残留 GPU 进程

---

## 2 首帧时间（族 H）

| 场景 | 预算 | 判定 | 来源 |
|---|---|---|---|
| 普通 UI 真窗冷启（同机） | `< 2000 ms` | `time_to_first_present_ms` 超预算 FAIL；同时要求首帧有内容（`first_present_paint_count > 0`） | `docs/ENGINE_UI_WIDGET_RENDER.md` §2.2.1 H 族（:126）、R16 行（:357） |
| 文本输入类真窗（窗口首帧有字） | `< 1000 ms` | 超预算 FAIL | `docs/ENGINE_TEXT_WAYLAND_IME_REQUIREMENT.md`（:337、:368、:396、:424、:451） |
| 输入法候选/选区窗（首焦即弹） | `< 800 ms` | 超预算 FAIL | 同上（:368、:451、:475） |

**实测参考值（非门禁，用于识别回归）**

| 窗 | 实测 | 来源 |
|---|---|---|
| `ui_wr_r16_warmup` | 89.6 ms | `docs/ENGINE_UI_WIDGET_RENDER.md` :670（2026-08 实测） |
| `ui_wr_c0_smoke` | 110 ms | 同上 :673 波收口记录 |
| `ui_wr_r4_composite` | 83.3–95.5 ms | 同上 :572（2026-10-23 两连） |
| `ui_wr_r9_text_cache` | 86.4–88.8 ms | 同上 :576 |

> 首帧也属于「长跑门禁之外」的短指标，`warmup=true` 时首帧同步 Present；首帧黑屏一律 FAIL，不看后续是否恢复。

---

## 3 滚动 / 动画帧时（族 A）

适用：R6、R7、R7b、C3、C5、C10 等动画/滚动/持续 tick 窗。

| 指标 | 硬门禁 | 说明 | 来源 |
|---|---|---|---|
| 有效 FPS | `fps_interval ≥ 55`（优先 `fps_interval`，无则 `fps_wall`） | `fps_interval = 1000 / interval_avg_ms` | `docs/ENGINE_UI_WIDGET_RENDER.md` §2.2.2（:134、:139）；`ui/kit/internal/behavior/performance.go` `DefaultBudgets()` |
| p95 帧间隔 | `interval_p95_ms ≤ 22` | 硬 FAIL | 同上 |
| p99 帧间隔 | **只观测，不设绝对值门禁** | 理由见下 | 见「p99 说明」 |
| p99 回归带 | 相对基线增量 `≤ 8 ms` | **[治理值]** 沿用 `ui/scheduler/baseline.go` 的 `P95IntervalMs: 8` 容差档，把同档套到 p99 | `ui/scheduler/baseline.go` `DefaultBaselineTolerance()`；[治理值] |
| 卡顿率 | ≥60s soak 窗 `hitch_rate_per_min ≤ 5`（窗 README 可收紧） | 短窗只记 `hitch_count`，不折算每分钟 | `docs/ENGINE_UI_WIDGET_RENDER.md` §2.2.2（:135） |
| 持续 tick 正确性窗（R0 等） | 运行 ≥5s 后有效 FPS ≥55 | 短于 5s 不得关 | 同上（:136） |

**p99 说明（为什么不设绝对硬线）**：上游 UI 真源只对 p95 给了 22ms 硬线，p99 是「必采、先观测、后升硬」（`docs/ENGINE_UI_WIDGET_RENDER.md` §2.2.1 A 族 :119、§2.2.2 :134）。2.5D 目标线写了 p95≤16ms / p99≤20ms（`docs/RENDER_2_5D_GAP_PLAN.md` §5 :78），但该线按 60Hz 目标设定，且同文档 S52 行已就本机 59.93Hz 同步底 16.69ms 给出 p95 豁免（:1072、:1784），说明这条线在 940MX 上物理不可全达；它是 2.5D 的目标线，不能直接搬来当 UI 硬门禁。因此本收敛把 p99 定义为**回归观测项**：记录绝对值，只判「相对基线涨没涨」，涨超 8ms 记回归。该 8ms 是治理值，不是上游标准，用户可直接改。

**2.5D 目标线（目标，不作 UI 硬门禁）**：p95 进 16ms、p99 进 20ms、超 20ms 记一次卡顿、每分不超 3 次。来源：`docs/RENDER_2_5D_GAP_PLAN.md` §5（:78）。该线用于 2.5D 窗的目标判定与 S52 豁免说明，UI 窗不执行。

**必采字段（缺一即 FAIL）**：`fps_wall`、`interval_avg_ms`、`interval_p50_ms`、`interval_p95_ms`、`interval_p99_ms`、`hitch_count`、`hitch_rate_per_min`、`vsync_source`、`target_hz`。来源：`docs/ENGINE_UI_WIDGET_RENDER.md` §2.2.1 A 族（:117、:119）。

---

## 4 显存上限（族 E 的 GPU 部分）

| 项 | 数值 | 判定 | 来源 |
|---|---|---|---|
| 单进程显存预算 | **768 MiB**（`GPUI_VRAM_BUDGET_MB` 覆盖，`0` = 关闭） | 账本 `live` 超预算 → OOM 拒绝分配并报错 | `gpu/hal/vram_ledger.go:38–41`（默认值依据）、`:50–59`（读取）、`:127`（超预算报错） |
| 窗内账本水位 | **0.9 × 预算**（默认约 691 MiB） | `peak > 0.9×budget` → 窗 FAIL | `examples/ui_base_vram_cap/main.go:71`、`:253` |
| 低显存自动档 | **0.8 × 预算**（默认约 614 MiB） | 超线自动切 LowVRAM 描述符（降档不是失败，但要记录切档点） | `render/adapter_policy.go:257` |
| 单重窗经验值 | 约 100 MiB/窗 | 用于排布多窗压测，不是门禁 | `gpu/hal/vram_ledger.go:39` 注释 |
| 八窗压测实测 | 峰值 362.9–381.4 MiB | 作为基线回归带：同配置不得高于该范围上沿 +10% | `docs/RENDER_BASE_RESOURCE_MATRIX.md`（ui_base_vram_cap 行） |
| 关窗/切场景回落 | 回落到开窗前基线值 | 差异逐条记录；只增不回 = FAIL | `docs/ENGINE_GPU_RESOURCE_LIFECYCLE.md` §6 验收口径 |
| OOM 注入回落 | 注入后账本增量 **0.00 MiB** | 注入被拒后账本必须归零 | 同上；`docs/RENDER_BASE_RESOURCE_MATRIX.md`（OOM delta 0.00MB） |

**预算口径统一说明**：0.9 是「窗内账本自证的上限」，0.8 是「驱动自动降档的触发线」，768 是「进程分配上限」。三者是不同用途的三条线，不许互相替代；P2.4b 预算统一时以本节三条线为语义基准。

---

## 5 内存与 CPU（族 D / E）

| 指标 | 硬门禁 | 说明 | 来源 |
|---|---|---|---|
| `rss_start_kb` / `rss_end_kb` / `rss_peak_kb` | 必采 | Linux 缺失 → FAIL | `docs/ENGINE_UI_WIDGET_RENDER.md` §2.2.4（:160 附近） |
| `rss_slope_steady_kb_per_min` | `> 30000 KB/min` → FAIL | **只在 `rss_slope_valid=true`（≥30s，推荐 60s）时判**；默认建议值，场景可收紧 | 同上（:160） |
| `rss_slope_kb_per_min` | 短窗必采，只观测 | `rss_slope_valid=false` = 不能下结论，**不是没漏**；禁止拿 5s/15s 斜率当泄漏结论 | 同上（:160） |
| 回归带（相对基线） | RSS 斜率增量 `≤ 2000 KB/min` | **[治理值]** soak 友好带，非产品 SLO | `ui/scheduler/baseline.go` `DefaultBaselineTolerance()` |
| `cpu_pct_avg`（短窗 <30s） | 只必采，尖峰记日志 | 不判红 | 同上 §2.2.3（:150） |
| `cpu_pct_avg`（长窗 ≥60s） | `> 85%` 单核折算且持续 → FAIL | 默认建议，窗 README 可写更严 | 同上（:150） |
| `cpu_ui_pct` / `cpu_raster_pct` | 必采 | 动画窗禁止双 0 却靠降画质过 fps 门禁 | 同上（:150 后文） |
| 长时间稳定性 | 2 小时 RSS 涨 ≤5%（2.5D 大包 8 小时） | 收敛期不强制跑满，P3 前至少跑一次 2h 档 | `docs/RENDER_2_5D_GAP_PLAN.md` §5 |

**基线回归带（I 族）**：`hitch_rate_per_min +5`、`p95 +8ms`、`rss_slope +2000 KB/min`、`cpu_fallback_ops +50`、`hitch_count +10`。来源：`ui/scheduler/baseline.go` `DefaultBaselineTolerance()`（文件自注「loose soak-friendly band, not a product SLO」）。P1.7/P1.8 对照用这组数，不另造标准。

---

## 6 金图与像素容差（族 J 的视觉部分）

### 6.1 总口径

| 项 | 规则 | 来源 |
|---|---|---|
| 同引擎金图对比 | **默认零容差**（任一通道差即 FAIL）；尺寸不一致直接 FAIL | `docs/UI_PIXEL_ASSERTION_STANDARD.md` §0（Flutter `precisionTolerance` 语义）、§1 |
| 非零容差 | 必须在该窗 README **显式声明数值与依据**；未声明 = 零容差 | 同上 §4（:55） |
| 差异率 | `pixel_golden_diff_pct` 超声明 tolerance → FAIL | 同上 §4 |
| 跑法 | 连续两次运行，静态区逐位一致；动态区按形态矩阵判定 | 同上 §3.5 |
| 快照 | `SnapshotPath` 至少 2 张（稳态中段 + Recover 段），随 JSON 留档 | 同上 §3.5 |
| 三证据 | 逻辑探针 + 像素断言 + 金图，缺一不得标绿 | `AGENTS.md`「真窗与状态真源」；同上 §5 |

### 6.2 按形态的容差（像素断言用，不是金图容差）

| 形态 | 容差 | 来源 |
|---|---|---|
| F0 静态点采样 | 通道差 `≤ 8`，≥3 个采样点 | `docs/UI_PIXEL_ASSERTION_STANDARD.md` §2（:28）、§3（:45） |
| F5 半透明/混合 | 通道差 `≤ 12`，须附混合公式 | 同上（:33、:45） |
| GPU/CPU 模糊对拍 | 最大 `6/255`，均值 `≤ 1.5/255` | `docs/RENDER_FEATURE_COVERAGE.md`（:179） |
| 裁剪/旋转/缩放 | 给区间不给等值；占比骤降为 0 = 最高优先 FAIL | `docs/UI_PIXEL_ASSERTION_STANDARD.md` §2 F2/F3/F4 |

### 6.3 单元测试图像对比口径（P2 重构等价性用）

| 项 | 容差 | 来源 |
|---|---|---|
| 图像 diff | 差值 `d > 2` 才计差异，且只比 RGB | `docs/render_gpu_converge/06_six_agent_audit_20261003.md` |
| 混合结果 | 容差 `2` | 同上 |
| 图片绘制 | 容差 `2` | 同上 |
| 色彩 LUT | `0.01/byte` | 同上 |
| 光栅扁平化 | `0.1 px` | 同上 |

> 这两套口径不能混用：§6.2 是「理论色/视觉断言」，§6.3 是「单测数值等价」。P2 重构判定用 §6.3，真窗三证据判定用 §6.1/§6.2。

### 6.4 逐窗容差登记方式（交给 P0.2）

P0.2 的窗表每行必填 `金图路径 | 容差 | 依据`。本文件只给总口径，不预先替任何窗放宽容差。P1.4 若发现某窗必须非零容差，先在该窗 README 写依据，再回填 P0.2 表，不许只在代码里改常量。

---

## 7 GPU 资源生命周期验收（族 E/F 的资源部分）

| 判据 | 数值 | 来源 |
|---|---|---|
| `Resolve` 未命中 | `0` | `docs/ENGINE_GPU_RESOURCE_LIFECYCLE.md`（:7、:125） |
| error scope 计数 | `0` | 同上 |
| usage 冲突 | `0` | 同上 |
| 显存 | `≤ 预算`（§4） | 同上 |
| 设备丢失 | 可恢复（`Invalidate*` 不调 native、in-flight 强制解除） | 同上（:128） |
| 关 GPU 资源顺序 | 停帧 → `WaitIdle` → 释放 surface/swapchain → 关窗 | `AGENTS.md`「GPU 真窗与压测安全」 |

---

## 8 负向对照（P1.8 必须做）

### 8.1 三种人为破坏（每项必红）

| 破坏 | 应当触发 | 来源 |
|---|---|---|
| 画面偏移 1px | 像素断言 / 金图 diff 红 | 本文件 TASKS §3 P1.8 |
| 跳过一次绘制 | 逻辑探针（draw/paint 计数）红 | 同上 |
| 少释放一次 | 显存账本 / 生命周期红 | 同上 |

### 8.2 本文件自身的负向对照（P0.1 自校验）

把一项阈值故意写错（例如把 p95 硬线临时改成 `30ms`、或把容差临时改成 `20/通道`），用同一份 P1.7 数据与 P1.8 破坏跑一遍，**必须显示偏差**；证明「阈值确实在起作用，不是摆设」。对照组跑完立即还原，并把前后输出记入 `baseline/negative/threshold-mutation.txt`。

> 自校验的通过条件：改错 → 判绿（漏报）；还原 → 判红（正确报警）。只做一次不算，必须成对。

---

## 9 修订记录

| 日期 | 修订 | 依据 |
|---|---|---|
| 2026-10-28 | 首版：环境清单 + 首帧/滚动/p95/p99/显存/内存/CPU/金图/生命周期阈值；p99 定为观测 + 8ms 回归带；显存三条线（768 / 0.9 / 0.8）分列 | P0.1 任务定义；来源见各行 |

**待用户确认点（不阻塞执行）**：p99 回归带 8ms 与「RSS 回归带 2000 KB/min」均为治理值，如认为应更严，改 §3/§5 数值并记入本表即可，不影响其他条目。
