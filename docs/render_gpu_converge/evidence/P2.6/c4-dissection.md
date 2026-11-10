# C4 解剖：四窗「rc=1 而 JSON 自报通过、金图 diff=0」

> 台账来源：§2.2 C4（P1.4 重审 T2，归属 P2.6 前诊断）。诊断日期 2026-11-10。
> 四窗：`ui_wr_r1_layout` / `ui_wr_r20_filter` / `ui_wr_r22_selection_stub` / `ui_wr_c6_savelayer_group`。
> P1.4 原始数据：`baseline/windows/summary.md` FAIL 表 111-118 行；逐窗归档 `baseline/windows/<窗名>/`。

## 共同机制：「JSON 自报通过、rc=1」的真正成因（已源码钉死）

所有 wr 窗的报告顺序是**先打印 JSON、后跑门评估**（以 r20 为例
`examples/ui_wr_r20_filter/main.go:518-519` 先 `json.Marshal` + `Println`，
`:525` 才 `wrgate.EvaluateGates`，失败 `:543-544` `FAIL:` + `os.Exit(1)`）。

⇒ JSON 的能力字段（scripted/golden/pool/probes）与 rc 携带的信息面不同：
**rc 才是最终门判定**；P1.4 归档的 `json_ok=true` 含义是「JSON 可解析 + 能力字段
自报」，不是最终判定（`record.json` 的 `json_ok: true` 与 `rc: 1` 并存即此理）。
四窗金图全部 0.0000%（421120/306280/318160/529400 px），**不存在渲染假绿**。
外部消费者若只读 JSON 不看 rc 才会得出「自报通过」的假象——这是 C4 表象的答案。

## 逐窗病灶（P1.4 vs 现行树，全部 30s 自检轮实测）

| 窗 | P1.4 rc=1 原因 | 现行树（2026-11-10 native 30s 实测） |
|---|---|---|
| r1_layout | hitch 11.99/min > 5：**窗自己的 U21 快照回读**（steady+recover raster-thread double-shot）每次把帧间隔顶过 33.4ms；窗 JSON 自注 `hitch_note: "2 hitches = the two U21 snapshot readbacks; no other hitches"` | **PASS（rc=0）**：hitch 2 次 = 4.00/min ≤ 5；快照回读已快于 33.4ms 阈值 |
| c6_savelayer_group | hitch 12.01/min > 5：同族（快照回读 6 次/30s） | **PASS（rc=0）**：hitch 1 次 = 2.00/min；frame_raster_ms=133 含收尾快照重绘+readback |
| r22_selection_stub | hitch 48.18/min > 5（24 次/30s） | **仍 FAIL（rc=1）**：hitch 3 次 = 6.01/min > 5，压线不过（P1.4 的 24 次 → 3 次，快照回读大幅变快但预算卡 5；2 次确认是双相位快照回读、第 3 次未单独归因） |
| r20_filter | scripted 4/5（GRAY 卡 I3 家族，与 I3 同一现场）；hitch 17.98/min 过关（r20 预算 30） | scripted **5/5（I3 窗内现象已消）**、金图 0.0000%，但 rc=1 换病灶：**池域名门** `RequirePoolDomains` 里的 `layer-offscreen` 未出现（现行池仅 `device-surface`+`filter-graph`） |

## r20 现行病灶细节：layer-offscreen 池域名期望与引擎路由演进脱节

- 门来源：`examples/ui_wr_r20_filter/main.go:534-541`（P2.4d 复用探针注释）要求
  `filter-graph`+`device-surface`+`layer-offscreen` 三域在探针中出现。
- 引擎现状：`layer-offscreen` 池（`render/internal/gpu/gpu_render_context.go:4138-4158`，
  `BudgetLayerOffscreen`）是懒建池，现行可达入口只剩：
  ① `ensureFilterSrcRT`（`filter_ops_gpu.go:952`，**仅中帧 pending 滤镜路** `:185` 需
  `pending>0`）；② `acquireEffectPublishView`（`context_layer_pass.go:19-33`，F14 效果
  发布）；③ 公开 `Context.CreateOffscreenTexture`。
- r20 场景（retained + scene 滤镜层）滤镜 RT 走 `filter-graph` 池、不走中帧 pending
  路 ⇒ layer-offscreen 在该场景 legitimately 不出现。**是窗的 P2.4d 期期望过时**，
  不是引擎丢池（该池在其他场景仍有生产可达性）。
- P1.4 时代该门尚不存在（归档 record.json 无 pool 数组），门是 P2.4d 后加的，
  所以 P1.4 的 r20 失败只有 scripted、现行才暴露池门。

## hitch 门与验收活动的口径问题（r22/r1/c6 三窗共性）

- hitch 定义：帧间隔 > 33.4ms（`ui/scheduler/metrics.go:19-21`，soak/jank 观测口径）。
- U21 快照回读（重绘 + GPU readback 存 PNG）是**验收活动**，会把 1-2 个帧间隔顶过
  阈值；30s 正确性窗里 2 次快照 = 2-3 次 hitch，5/min 预算只剩 1-2 次余量。
- r1 的窗注记自认「no other hitches」；这是**门语义把验收活动计入 soak 预算**的
  口径问题，不是动画卡顿（interval_p99 全部 <21ms，动画帧本身远低于 33.4ms）。

## 结论与待拍板

1. C4 的「病因」= 窗报告顺序（JSON 先行、rc 带判定）+ 各窗 rc=1 的具体门各不相同。
2. 现行树：r1/c6 已 PASS；r22 差 1 次 hitch 压线；r20 换成池域名期望脱节。
3. 待拍板（修法方向）：r20 池域名门更新 / r22 hitch 口径（验收活动是否计入）/ C4 行销账方式。

## 处置（用户拍板 2026-11-10：r20 更新窗门 / r22 引擎 hitch 暂停口 / C4 销账+子项落位）

1. **引擎口：快照回读不计 hitch**（`ui/scheduler/metrics.go` + `ui/embedder/pipeline_app.go`）：
   - `MetricsStore.PauseHitchCount/ResumeHitchCount`（可嵌套；暂停期间 interval 照记
     ring/avg/max、只 HitchCount 免计；最外层 Resume 后首笔记账恰好横跨暂停活动——
     drain 与 present 完成记点在同一栅格作业相邻——跳过一次）。
   - `PipelineApp.drainSnapshots` 在批量执行快照闭包处包裹（SnapshotAsync 与
     RetainedSnapshot 同走 queueSnapshot，一处覆盖全部快照路径）。
   - 先红后绿：`ui/scheduler/metrics_hitch_pause_test.go` 两枚（暂停免计 + max/avg
     照记的诚实性；首版测试 helper 把参数当绝对时间戳写错已修正）。
2. **r20 窗门更新到现行拓扑**：`RequirePoolDomains` 去掉 `layer-offscreen` 强制
   （保留 filter-graph+device-surface 与命中要求），注释引用本文。
3. **r1 过时注记更新**：头注释与 `hitch_note` 改述新语义（快照回读已由引擎排除）。

## 验证（2026-11-10/11，30s 自检轮，单窗串行自退出）

| 窗 | native | go |
|---|---|---|
| r1_layout | OK hitch 0.00/min scripted 10/10 金图 0.0000% | OK hitch 2.00/min 同左 |
| r22_selection_stub | OK hitch 2.00/min scripted 6/6 金图 0.0000% | OK hitch 0.00/min 同左 |
| r20_filter | OK hitch 2.00/min scripted 5/5 金图 0.0000%（池门过） | OK hitch 4.01/min 同左 |
| c6_savelayer_group | OK hitch 2.00/min scripted 6/6 金图 0.0000% | OK hitch 2.00/min 同左（P1.4 的 go NO_JSON 为 C7，已由 P2.4e 修复） |

四窗双后端 8 轮全 PASS（rc=0）。剩余 1-2 次 hitch（2-4/min ≤5 富余通过）未逐笔归因，
如实登记（快照暂停只覆盖 drain 包裹段；窗关闭/收尾等其余 >33.4ms 间隔照计）。

门禁：`ui/scheduler`+`ui/embedder` 逐目录测试绿；`-race`（CGO_ENABLED=1）×5 全绿
（首跑遇磁盘满全量重建高负载期一枚未留名 FAIL，未复现、未见 DATA RACE，如实登记）；
默认+nogpu 构建 rc=0；全仓 `go build ./...` rc=0；apidoc rc=0（本改动为 ui 层 API，
不在 render catalog 口径，ENGINE_UI_WIDGET_RENDER.md 修订表同步）。

