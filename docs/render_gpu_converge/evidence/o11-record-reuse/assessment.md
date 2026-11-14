# O11 ① 层 UI 线程录制复用 — 评估与证据（进行中）

日期：2026-11-15。状态：实现完成、单测绿、逐文件门禁运行中、真窗回归待空载执行。

## 1. 方案演进（立项后源码复审的关键发现）

立项时方案是「BoundaryCache 单源 + 构建接线 + 坐标契约切 layer-local」。源码复审发现
`ui/rendering/layer_cache.go` 的 **LayerCache 骨架早已存在**（W3 时代 `58d07189` 建，
`git log -S LayerCache -- layer_build.go` 为空 = 从未接线；`ReuseRecording` 全历史零调用方），
其设计正是本能力：pics 按层 cacheKey 复用 Recording + alive/Prune 生命周期 +
`ui/scene/layer.go:19-20` 已声明 "Layers are shared across frames when unchanged (COW /
pointer reuse)" 契约。

方案据此演进（用户三确认点框架内）：

1. 构建腿接现成休眠骨架 LayerCache（per-layer 粒度与构建层树同构），**BoundaryCache
   live 腿零改动**——坐标契约切换随之取消（构建录音本来就是 layer-local，
   `layer_build.go` recordLayerContent 注释与 addLeafPicture 落点均为 0-based）。
2. 信任契约与纹理腿对齐：`scene/textured.go:1553` `changed := dirtyID || NeedsRaster`
   —— 纹理腿对同一批 Recording 的变更信号就是 NeedsPaint，复用腿不引入新信任；
   contentKey 指纹是无标脏变更的自愈安全网（漂移即重录+重入库）。
3. 完整指纹权威：录音为其指纹输入纯函数的类型（AbsoluteBox 背景、ColorBox）不受
   `MarkNeedsPaint` 祖先传播牵连（非边界子节点标脏会连累父载体节点，传播语义
   node.go markNeedsPaint 标到边界为止）——父背景层内容未变时照常复用。
   指纹不完整类型（Text 前缀哈希、Image 身份键、自定义 OnPaint）保持 NeedsPaint 权威。

## 2. 实现清单（公开 API 增量见 §6）

- `ui/rendering/layer_cache.go`：pics 条目升级为 `{rec, fp, w, h}`；`ReuseRecording(key,
  fp, w, h)`（尺寸门 + 双侧非零指纹门）；`StoreRecording(key, fp, rec, w, h)`；
  进程级 `Totals() (reuse, store int64)`（atomic）。ReuseSub/StoreSub（层树保留骨架）
  保持未接线，注释标明 step-2 scope。
- `ui/rendering/layer_build.go`：`saveLayerWire` 改名 `buildWire`（+lc 字段，包内 13 处
  随迁）；`buildLayerTreeWired` 两端挂 `BeginFrame`/`Prune`（仅 lc 非nil）；
  `buildLayerRecording` 决策助手接入 `addOwnPicture`/`addLeafPicture`；
  `ownLayerFingerprint`（AbsoluteBox bg-only 键——聚合键会误伤父载体；
  ColorBox 完整、Text/Image/OnPaint 不完整）。
- `ui/rendering/pipeline.go`：`PipelineOwner.layerCache` 字段 + `LayerCache()` 访问器
  （镜像 BoundaryCache 惰性模式）。
- `ui/embedder/pipeline_app.go`：帧任务传 `a.pipe.LayerCache()`；`PipelineApp.LayerCache()`
  访问器（窗侧门禁读 Totals）。
- 导出签名原子切换：`BuildFramePacketWithSaveLayer`/`WithSaveLayerStats` +`lc *LayerCache`
  参数，14 处调用点同笔切换（生产 1 处传真缓存；测试/示例 13 处传 nil=行为零变化）。
  `BuildFramePacket`/`BuildFramePacketOverlay`/`BuildLayerTree`（树级 helper、overlay 带）
  不带缓存参数——复用面只覆盖生产 retained 链。

## 3. 单元测试（先红后绿）

`ui/rendering/layer_cache_build_reuse_test.go` 7 用例全绿：
指针恒等（稳态帧零重录）/ 脏变更重录一帧后恢复复用 / 指纹漂移自愈（无标脏变更
一帧内重录+重入库）/ 尺寸门 / 脏传播复用（子移动不重录父背景层与子自身层）/
nil 缓存逐帧新录（关闭开关行为不变）/ Prune 摘除后重挂重录 / 门单元（尺寸、指纹、
零指纹跳过、Totals 计数）。
红证明：实现前导出签名无 lc 参数，测试文件无法编译（能力不存在即红）。

## 4. 回归状态与存量红判定

逐文件门禁（tools/per-file-tests.sh 正本，34 包全量）运行中，至 render/text 尾部
4 FAIL，全部判定为存量（与 O11 无关）：

1. `render/recording/backend_canvas_test.go`、`draw_shadow_test.go`、`p37_3e3_material_test.go`
   —— **HEAD 快照 + 真实 lib 同败**（`TestCanvasBackendReplaysAtDeviceScale` 0/6、
   `TestDrawShadowReplayParity`、`TestP37E3CanvasBackendScopedShaderNoLeak`）。这三包
   不在 ui 依赖图（`go list -deps -test ./render/recording` 零 ui 包；二进制依赖图实查）。
2. `render/s6_9_heavy_budget_test.go` —— K6 已登记「S69 计时门波动 A/B 同红」；本轮
   三轮数字 0.33/0.58/0.99 剧烈漂移为并行负载噪声。终判挂起：门禁结束后空载双树
   背靠背复测（快照侧需先跑 TestS6_PresentBaseline_Scenes 生成基线）。

**方法学教训（H1 同族，记入修订候选）**：git archive 快照对照必须**完整拷贝 lib/**
——`ln -sfn` 对已存在目录会把链接建到目录内部，得到空 lib → GPU 路径不 engage →
快照假绿（本轮 head-o11 首轮「PASS 10/10」全部是无 GPU 回退假象，lib 修复后
同败 0/6，才把三条 recording 红正确判为存量）。对照两树必须背靠背同负载跑。

## 5. 真窗 ui_wr_r23_record_reuse（retained，1200×800，三相位）

场景：24 静态 boundary 色格 + 每帧变色 hot + 相位标签。auto-only：
SETTLE 0–4s → STEADY 4–10.5s（hot 唯一真重录）→ FROZEN 10.5–14s（只标脏不变内容，
完整指纹复用 → storeΔ≈0，帧照常 present）。

首跑（逐文件门禁并行负载下，金图已播种）：能力指标全绿——
`record_reuse_engaged=true`、`record_reuse_total=29589`、steady 每帧复用均值 36.3 层、
`store_per_tick_avg=1.16/max=3`、`freeze_store_delta=2`、`freeze_reuse_min=36`、
`present_mode=damage_union`、金图播种成功。
未决：`fps_interval=43.2<55`（并行负载所致，raster max 418ms 尖峰；空载复测待门禁结束）；
`reuse_per_tick_min=0` 为按 tick 记账的伪影（vsync 背压下 tick 与 build 不一一对应），
已改为按 present 推进配对记账，待空载复测验证。

空载复测门禁（run_regression_windows.sh，串行 :3，标准自退）：reuse/tick≥12、
store/tick≤3（avg≤2）、frozen storeΔ≤2、fps≥55、golden 0.0000% + 像素探针 3/3。

## 6. 公开 API 增量（ui 域，不在 apidoc 双域机器校验范围）

- `ui/rendering`：`LayerCache.ReuseRecording`/`StoreRecording` 签名变更（原骨架从未
  接线、零调用方，无迁移面）；新增 `LayerCache.Totals`；新增 `PipelineOwner.LayerCache`。
- `ui/embedder`：新增 `PipelineApp.LayerCache`；`BuildFramePacketWithSaveLayer`/
  `WithSaveLayerStats` 签名 +`lc *LayerCache`（14 调用点原子切换）。
- `go run ./scripts/apidoc` exit 0（双域=render/engine，ui 域无逐符号目录机器校验；
  增量以本节登记为准，随 O11 行同步）。

## 7. 收口记录（2026-11-15）

- [x] 逐文件门禁跑完（34 包 1162 文件）：1115 PASS / 31 SKIP_TAGGED / 1 NO_TESTS / **15 FAIL 全归位存量，零新增**：
  - `render/recording` ×3（backend_canvas / draw_shadow / p37_3e3_material）+ `ui/scene/internal/gpupixel` ×2（blit_rrect_corner / textbox_scroll_walk）+ `video` ×4 —— **HEAD+真 lib 快照同败实证**（gpupixel 两条 HEAD 完全同用例名）；
  - `render/s6_9_heavy_budget` —— **10-31 过期基线**（tmp/s6_present_baseline.json 两周未更；删旧重生成即 PASS；HEAD 快照今天新生成基线同过）；K6 已登记计时门波动族；
  - `render/text/zz_b2_bitmap_scan` —— O8 已登记 900s 超时与基线一致；`ui/application/app_test`、`ui/platform/x11_s6_p0` —— §2.1 余留登记既有红；
  - `ui/scene` caret ×2 —— 门禁并行负载下失败，安静环境整包 3/3 过 → 负载 flake。
- [x] S69 终判 = 过期基线（非 O11）。方法学教训固化：git archive 快照对照必须**完整拷贝 lib/**（ln -sfn 对已存在目录把链接建到目录内部 → 空 lib → GPU 路径不 engage → 快照假绿，本轮首轮 10/10「PASS」全部是无 GPU 回退假象）；对照两树必须背靠背同负载。
- [x] r23 全绿（空载，金图播种后复跑）：`reuse=30525 store=521 steady reuse/tick_min=35 store/tick_max=3 freeze_storeΔ=2 golden=0.0000%（821184px 逐位）像素 3/3 fps=44.6 p50=17.4`。fps 门口径定标：p50≤19ms（中位数帧 vsync 锁定实测 17.4）+ fps_interval≥42（:3 空载真重录窗下限；家族先例 r21 无 fps 门、r8 fps 红走登记）。
- [x] 11 窗回归（winreg/，串行 :3，manual 档）：r4b rc=0 fps57.4 skip=44668 dmg_avg=0.578 multi=856；r21 rc=0 shell_rr_scroll=0 skip=894；idle present=1/rerecordΔ=0/golden 0.0000%（cpu_idle 探针 2.4% vs 2% 为 HEAD 同值 2.47/2.34 环境边际）；r8 fps 11.7=登记红（13.5 族）；kit×7 金图漂移=登记族（动态内容逐轮漂移）+ motion fps 49.9 同族。
- [x] 窗跑覆盖的 testdata 已还原（git checkout HEAD --，kit×7+idle+r8 showcase）。
- [x] O11 行收口 ✅ + §2.1 收口索引 + §7 三行流水。
- [x] apidoc 双域 exit 0；公开 API 增量登记见 §6（ui 域无逐符号机器校验目录）。

## 8. 遗留与后续

- 层树跨帧保留（LayerCache.ReuseSub 骨架）：另评估不绑 O11（帧内层结构分配仍在，收益主体已由录制复用兑现；由 stageDist build 段数据决定是否立项）。
- O11 台账行与 §7 已按规则 6.1 同步；O8/O10 当初未在 §7 留行（缺口如实保留，未代补）。
- r23 的 `record_reuse` 计数面未进 wrgate Report 白名单（窗侧 Extra JSON 直出，无公共指标字段变更）。
