# O1 内存平台化（RSS 平台化）复测与定性（2026-11-12）

> 范围：`ui_l1_spinner`、`ui_text_m4_virtual`；native 与 `GPUI_BACKEND=go` 各一轮，
> 每轮 `RUN_SECONDS=55`（单窗自关，wall ≤56.4s，满足 ≤1 分钟约束）。
> 标准口径：`baseline/thresholds.md` §5（`rss_slope_steady_kb_per_min > 30000` → FAIL，
> `rss_slope_valid` 需 ≥30s、推荐 60s；2h RSS 涨 ≤5%）；平台化参照
> `examples/ui_wr_c10_soak/README.md`（300s：74→305MB 后 slope 1164 KB/min）。
> 测量工具：`tools/probe.py`（0.25s `/proc/<pid>/status` + `smaps` + gctrace）、
> `tools/analyze.py`。原始数据在四个 run 目录。

## 0 直话结论

- **没有发现活对象泄漏**：Go 活堆在 55s 内基本稳定（l1 42→52MB，m4 152→158MB），
  `[anon: Go: heap]` 的增长是 Go 堆向 GC goal（l1 ~104MB / m4 ~305MB）爬坡，
  驱动文件映射（~97MB）在 10s 后不再增长，GPU 池稳定（m4 59/60 条、17.8MB）。
- **RSS 在 60s 内还没完全平台化**：l1 在 48.6–55.4s 已走平（+1.1MB/6.8s ≈ 10MB/min）；
  m4 末段仍有 ~28MB/min 的堆 arena 触碰增长。按 P0.1 的 55s steady 斜率（15s 预热）
  四轮为 30.4–49.3MB/min，会判 FAIL，但这是**启动堆爬坡**，不是持续泄漏。
- **抓到两处测量口径缺陷**（都能复现，见 §3）：
  1. `Run()` 内 `defer a.Close()` 使窗在 `app.Run()` 返回前完成 GPU 拆机；
     窗随后 `proc.Stop()` 采到的是**拆机后** RSS。l1 拆机额外 +7MB（30s 轮）到 +57MB（55s 轮），
     直接污染 `rss_end/peak`（55s 轮 peak 298.9MB，运行末真实 241.7MB）。
  2. steady 斜率的预热是 `max(elapsed/4, 15s)`；55–60s 轮只预热 15s，
     而本机 Go 堆高水位到 ~50s 才到，判据把爬坡算成泄漏（C5 已登记 30s/60s 口径不一致）。

## 1 四轮实测（55s，单窗，串行）

| run | 后端 | RSS 起→末（运行段） | 15s 预热 steady | 末 15s 斜率 | 活堆 | 堆大小 | 归因（8→45s） |
|---|---|---:|---:|---:|---:|---:|---|
| l1_spinner_native_smaps | native | 0.4→241.7MB | 43.4 MB/min（官方 43.7） | 37.4 MB/min | 52MB 稳 | ~104MB | Go heap +21.8MB，其余 0 |
| l1_spinner_go | go | 0.4→229.6MB | 36.1 MB/min（官方 36.2） | 18.7 MB/min | 110MB* | ~145MB | Go heap +20.4MB，其余 0 |
| m4_virtual_go | go | 0.2→438.5MB | 37.2 MB/min（官方 37.1） | 28.5 MB/min | 155MB 稳 | ~305MB | Go heap +28.7MB，其余 0 |
| m4_virtual_native | native | 0.4→441.4MB | 30.3 MB/min（官方 30.4） | 11.6 MB/min | 156MB 稳 | ~305MB | Go heap +34.3MB，其余 0 |

\* l1 go 的活堆末值来自最后一个 GC（145MB goal/110MB live），前段 GC live 42→52MB。

- 官方字段（`rss_slope_steady_kb_per_min` / `rss_end_kb` / `rss_peak_kb`）见各 run
  `stdout.txt`；`rss_slope_valid` 四轮全 true。
- 关键曲线：l1 native 48.65→55.42s 的 RSS 从 240.4→241.7MB（≈10MB/min）；
  55.67s（`Run()` 返回后）跳到 298.8MB，随后进程退出。
- 30s 对照（`l1_spinner_native_30s/`）：跳变发生在 30.56s（`Run()` 返回点），幅度 +7MB；
  55s 轮同位置 +57MB → 跳变跟拆机绑定，不是固定时刻的泄漏。

## 2 标准对齐判读

| 判据 | 结果 | 说明 |
|---|---|---|
| P0.1 RSS steady >30000 KB/min | 四轮 55s 均过线 | 但 15–55s 含 Go 堆爬坡；末 15s 已降到 11.6–37.4MB/min（l1 native 因拆机前的最后一帧仍偏高） |
| P0.1 2h RSS 涨 ≤5% | 本会话不跑满 2h（用户约束 ≤1min） | 用活堆/堆大小/驱动映射三条替换证据定性：无持续活对象增长 |
| c10 平台化参照 | 同族 300s：74→305MB 后 1164 KB/min | l1 在本机 ~50s 已到 235MB 平台；m4 活堆稳定、RSS 在堆高水位附近 |
| Go 活堆（leak 语义） | l1 52MB、m4 155–158MB，末段 GC 稳定 | 无单调活对象增长 |
| GPU 账本/池 | m4 pool 59/60 条 17.8MB、budget 134MB；`[anon: Go: heap]` 外无 >0.5MB 增长 | 资源侧无泄漏 |

## 3 两处口径缺陷（file:line）

1. **拆机污染采样**：`ui/embedder/pipeline_app.go:1235` `Run()` 里 `defer a.Close()`；
   拆机路径 `pipeline_app.go:1162-1199`（WaitIdle → pictureTex.Close → target.Close → loop.Stop）。
   窗在 `Run()` 返回后才 `proc.Stop()`：`examples/ui_l1_spinner/main.go:80-81`、
   `examples/ui_text_m4_virtual/main.go:318-319`。`ProcessTracker.Stop` 的注释
   （`ui/scheduler/proc_tracker.go:117`）要求「Close GPU 前采」，但窗拿不到拆机前的时点。
2. **预热短于堆爬坡**：`ui/scheduler/proc_tracker.go:175-193`
   `warm := max(elapsed/4, 15s)`；55s 轮预热 15s，而本机堆高水位约 50s 到。
   C5 证据 `evidence/P2.6/c5-rss-slope-gate-assessment.md` §5 已登记同一口径风险。

## 4 待拍板（停下报告）

- **A. 测量正确性修复（推荐，先做）**：把 teardown 从 `Run()` 挪到调用方
  （或加 `BeforeClose`/`RunEnd` 回调），让 `proc.Stop()` 在拆机前采样；
  这会让 `rss_end/peak/after_close` 回到运行段语义，并补一条顺序单测（拆机前采样 < 拆机后）。
  影响面：196 个 `.Run()` 调用点里 195 个已显式 `Close()`，仅 `examples/wrkit/screencheck.go` 需补。
- **B. 判据标准对齐（需改 P0.1 口径，单独拍板）**：steady 判据改为「末段窗口 + 活堆」
  或自适应平台检测（例如末 15–30s 斜率 + 活堆趋势），使 60s 测试可下结论；
  否则 l1/m4 在 60s 窗会持续被启动爬坡判红。
- **C. 只定性不修**：按现有证据把 O1 收口为「一次性预热爬坡，非泄漏」，
  维持 2h soak 余项；不改代码。这与用户「测试和修复」的预期不符，列为备选。

推荐顺序：A（无口径变化、可先红后绿）→ 复跑四轮确认 end/peak/slope 语义 → 再拍板 B。

## 5 证据清单

- `summary.tsv`：四轮汇总（本文件 §1 的机器版）。
- `l1_spinner_native_smaps/`、`l1_spinner_go/`、`m4_virtual_go/`、`m4_virtual_native/`：
  `rss.tsv`（0.25s 序列）、`gctrace.tsv`（GC 活堆/堆大小）、`smaps_{8,25,45}s.tsv`
  （VMA 归因）、`stdout.txt`（官方 JSON）、`stderr.txt`。
- `l1_spinner_native_30s/`：30s 对照（证明拆机跳变随时点移动）。
- `tools/probe.py`、`tools/analyze.py`：可复跑采集/分析。

## 6 300s soak + Go pprof（2026-11-12，用户 5 分钟观察）

命令：`RUN_SECONDS=300`、`GPUI_HEAP_PROF_DIR` 在 60/180/280s 落 heap/goroutine pprof、
0.25s RSS/smaps 采样、`GODEBUG=gctrace=1`；单窗自关 wall=301.0s，rc=0。

| 指标 | 10s | 60s | 120s | 180s | 240s | 300s |
|---|---:|---:|---:|---:|---:|---:|
| RSS (MB) | 211.6 | 241.2 | 270.2 | 343.6 | 320.6 | 327.9 |
| Go 活堆 (MB) | 46 | 52 | 61 | 72 | 89 | 101 |
| Go 堆大小 (MB) | 89 | 102 | 121 | 142 | 175 | 198 |

- RSS 15–300s 斜率 27MB/min、60–300s 25MB/min、240–300s 17MB/min；
  **5 分钟仍未平台化**；官方 `rss_slope_steady_kb_per_min=24907`（300s 预热 75s）反而低于 30000 线，
  但活堆从 46MB 单调涨到 101MB（~12MB/min），不是稳态。
- smaps 归因：`[anon: Go: heap]` 99.0MB（30s）→154.8（150s）→235.2（270s）；
  其他映射增长 <1MB。增长在 Go 堆。
- `go tool pprof -inuse_space -base heap_060s heap_280s`：
  **`render/internal/gpu.(*GPURenderContext).QueueGPUTextureDrawQuad` +46.37MB（占 64.64%）**，
  堆栈 `PictureTextureCache.blit → DrawGPUTextureWithOpacity → QueueGPUTextureDrawQuad`，
  来自 `presentPacketTextured → PresentTarget.present`。绝对活内存 60s 12.5MB / 180s 37.4MB /
  280s 58.6MB，线性 ~10.5MB/min。
- `os.readFileContents` 38–39MB 为字体文件一次性加载，60s 后不再增长；
  goroutine 快照 60/180/280s 对象数不变。

## 7 5s/3s 队列诊断（`GPUI_QUEUE_DIAG=1`，临时探针已移除）

3s 运行（182 presents / `gpu_ops=3647` ≈ 20/frame）：

```
QDIAG present n=1 mode=full force=true
QDIAG flushCore n=1 op=FlushGPUWithView ... off=false   # 仅 warm-up 一次 flush
QDIAG plan n=1 mode=damage_union union=(0,0)-(12,12)    # 之后不再进入 PresentFramePlan
QDIAG quad len=1 cap=1 ...
QDIAG quad len=580 cap=646 ...
QDIAG quad len=1180 cap=1509 ...
QDIAG quad len=2980 cap=3772 ...                        # +20/帧，从未清空
QDIAG present n=60/120/180 mode=idle union=(0,0)-(0,0)  # 稳态全 idle
```

- 稳态每帧：`draw(dc)` 先把 20 条纹理合成命令入队，`PlanPresent` 返回 **idle**，
  `present()` 在 idle 分支直接 return，**既不 flush 也不丢弃队列** → 队列无限增长。
- 同一时刻 `boundary_rerecord=20`（全程只重录一次）、`boundary_skip=3540`（≈20/帧），
  spinner 的 RepaintBoundary 首帧后不再重录 → 画面冻结；`present_count` 统计的是
  raster job 完成数（`pipeline_app.go:1797`），不是真实 swapchain present，
  所以 fps/帧率指标仍显示 ~60，掩盖了冻结。
- 结论：**内存增长不是 GC 节奏/驱动造成，而是 present idle 早退遗留待处理命令队列**；
  5 分钟持续增长与该机制线性吻合（20 条/帧 × ~176B × 60fps ≈ 12MB/min）。

## 8 根因链与修复方向（待拍板）

根因链：
1. 重录边界（spinner RepaintBoundary）首帧后不再重录 → 帧 damage 为空；
2. `PlanPresent` 判 idle → `present()` 早退，未 flush/丢弃 `draw` 已入队的命令；
3. `pendingGPUTextureCommands` 每帧 +20 且永不释放 → Go 堆线性增长；
4. `present_count`/damage 指标由 raster job 与 `lastDamageArea` 填充，看起来仍 60fps，掩盖冻结。

修复方向（都要先红后绿，修后 300s 复测 + 55s 双后端）：

- **A. 空闲帧安全不变量（防漏，最小）**：`PlanPresent`/`present()` 在 idle 但存在待处理命令时，
  不得直接返回；按标准（Skia：有 draw 必有 damage）用待绘制命令的 Dst 并集作为 damage 呈现，
  或显式丢弃并入 damage 统计。可独立修掉内存泄漏。
- **B. 边界重录/绘制失效（治冻结，治本）**：让 `MarkNeedsPaint` 的 RepaintBoundary
  在 retained 路径重新进入 `NeedsRaster`，重录后按 prev∪current bounds 产生 damage；
  这是 spinner 动画恢复与 damage 非空的根。
- **C. 指标诚实性**：`present_count`/fps 改用真实 swapchain present 计数
  （或新增 `raster_jobs` 区分），避免再次用 job 数掩盖冻结；`rss_end/peak` 在拆机前采样。

推荐：B（治本）+ A（防漏安全网）+ C（指标诚实），一次提交前先补先红后绿用例；
若用户只想先止血，可先 A。

## 9 修复结果（2026-11-12，A+B+C 全做）

### 改动

- **B 治本（retained 录制补通用叶子）**：`ui/rendering/layer_build.go` `recordLeafContent`
  增加 default 分支——未命中 4 个快路径、且无 `OwnPaint` 的叶子通过自身 `Paint` 录进
  display list；录制期间保存/恢复 `needsPaint`，建包保持只读（`TestE2Merged...` 的
  「同脏态两建包脏集全等」恢复绿）。spinner 因此从「空 recording」回到有 bounds/ops
  的真实录制，churn 拒绝后按 `RefusedBounds` 每帧 vector replay 并产生 damage。
- **A 防漏安全网（idle 帧不遗留队列）**：`GPURenderContext.DiscardPending()` +
  `Context.PendingGPUCount()/DiscardPendingGPU()`；`PresentTarget.present` 在
  `plan.Mode==Idle` 且存在待处理命令时显式丢弃（下一帧重排队），不再跨帧累积。
- **C 指标诚实**：
  - `present_count` 只计真实到 swapchain 的 present（`out.Idle` 排除），
    `frame_count` 仍计 raster job；`MaxFrames` 改看 `frames`；
  - 新增 `PipelineOptions.OnRunEnd`（Run 内 `defer` 顺序保证在 GPU teardown 前执行），
    l1/m4 用它采 `rss_end/peak`，拆机 +7~57MB 不再计入。
- **验证辅助（env 开关，默认零影响）**：`examples/wrkit.StartHeapProfiles`（heap/goroutine
  pprof 定时落盘）；`ui_l1_spinner` 的 `L1_SNAP_DIR` 双快照像素自检。

### 先红后绿

| 用例 | 修前 | 修后 |
|---|---|---|
| `ui/rendering.TestBuildLayerTree_CustomLeafPaintRecorded` | 红：spinner 录制 0 条命令 | 绿：录制命令 >0 |
| `render/internal/gpu.TestDiscardPendingDropsQueuedDraws` | 新增白盒（方法不存在时不可编译） | 绿：队列/advanced/hold 全清、计数归零 |
| `ui/rendering.TestE2Merged_CountsHonestAndPacketEqual` | 通用 Paint 清脏导致 dirty 2→1 | 绿：两次建包 dirty=2 全等 |
| `ui/rendering.TestKeystrokeRatio_M5` | 红（1.6–2.6） | 红（干净 HEAD `7a7fc15d` 同样 1.8）→ 既有环境计时红，非本批回归 |

### 真窗复测（单窗自关，串行）

| run | 时长 | present/frame | damage | RSS | 稳态斜率 | 备注 |
|---|---|---:|---:|---:|---:|---:|---|
| `fix1_l1_spinner_native_55s` | 55s | 3264/3262 | 2304px | 12.8→214.1MB | **1440 KB/min** | 修前同窗 43700–49337 |
| `fix1_l1_spinner_native_300s` | 300s | 17848/17846 | 2304px | 210.1→211.0MB | **-91 KB/min** | 10s 后走平；活堆 42MB 恒定；pprof 60→280s diff **0** |
| `fix1_l1_spinner_go_55s` | 55s | 3254/3252 | 2304px | 12.6→208.3MB | **533 KB/min** | 双后端一致 |
| `fix1_m4_virtual_go_55s` | 55s | 3150/3149 | 704868px | 52.0→451.1MB | 34139 KB/min | 启动堆爬坡（与修前同量级） |
| `fix1_m4_virtual_go_300s` | 300s | 17277/17276 | 707586px | 409.6→462.7MB | **1581 KB/min** | 120s 后走平；活堆 149–165MB；池 61 条 17.9MB |

- **像素证据**：`pixel_proof/l1_a.png` / `l1_b.png`（相隔 2s 的 `RetainedSnapshot`）
  逐像素对比——`inside_diff=220` 全部落在 48×48 spinner 回放框内、`outside_diff=0`
  （`tools/compare_snapshots.py` PASS）；证明 retained 路径的动画确实在逐帧呈现。
- **指标诚实**：修复后 l1 `present_count≈frame_count`（动画每帧真呈现）；修前同窗
  `present_count` 也计 idle job，面板 60fps 但画面冻结。
- **资源账本**：m4 pool 59→61 条、17.9MB、budget 128MB，300s 稳定。

### 余留（登记，不混入本次收口）

- m4 60s steady 仍会因 Go 堆高水位爬坡 >30000（300s 回落到 1581）——P0.1「推荐 60s」
  与 Go 堆启动节奏不匹配，属判据口径复核（O3）。
- m4 300s pprof 残差：`noteRerecord` 归因 map 随滚动 cache key 增长 ~4MB/220s，
  `buildShapedCarets` ~4MB；300s 总斜率未破线，2h 档需复核（O4）。
- `present_count`/`fps` 口径改为真实 swapchain present 后，静态/idle 窗的计数会低于旧
  「raster job」口径；动画窗 l1/m4 已复验（present≈frame），静态窗门禁集未全量重放，
  登记 O5 抽检（l1_blank/l1_shell/r1_layout 等）。
