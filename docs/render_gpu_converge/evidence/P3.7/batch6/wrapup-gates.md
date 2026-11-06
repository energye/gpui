批 6 收官 · 批级门禁账（2026-11-06；口径按批 3 收尾登记：逐文件 sweep 对基线只减不增、五目标交叉构建按「批」为单位跑，整包 `-race` 随 H20 修后回填）
====================================================================================

一、五目标交叉构建 ×{默认,nogpu}
---------------------------------
- 首轮抓到真断口：`render/internal/gpu/image_batch_census.go`（批 6 ① `6f6d3d92` 引入的跨帧合批账）
  **没贴 `//go:build !nogpu`**，而它引用的 `imageDrawCall`/`GPURenderSession` 所在文件全部带该
  tag ⇒ `-tags nogpu` 下 `undefined` 编译失败（exit 123）。
  **归因如实**：洞自 ① 交付那刻就在（① 窗门禁按「五目标留批收尾」的登记口径没跑当轮），
  不是收官新引入的回归——这正是按批门禁存在的意义。
- 修复：补 `//go:build !nogpu` 一行（与同包 `render_session.go` 等逐字符同位形）；默认构建
  零变化（`go build ./render/internal/gpu/` + vet rc=0），nogpu 段 `go build -tags nogpu` rc=0。
- 修复后整轮重跑：**五目标 ×{默认,nogpu} 全绿**（每目标 raw+per-pkg+nogpu 三段 rc=0，
  「3/2 green」×5；日志 `build/*.txt`）。包数对批 3 基线每目标 +6，逐一对上批 6 新建的
  六扇窗包（`p37_persp_batch`/`p37_height_zsort`/`p37_material`/`p37_mip_exit`/
  `p37_bloom`/`ui_wr_p37_scene`）——无未知增减。
  linux 341/337、windows 347/343、darwin 339/335。

二、64 包全量逐文件 sweep
-------------------------
`tools/per-file-tests.sh tools/per-file-tests.pkgs tmp/per-file-b6wrapup`（带 WGPU_NATIVE_PATH）：
**940 行 = 903 PASS / 12 FAIL / 24 SKIP_TAGGED / 1 NO_TESTS**（`sweep-summary.tsv` 在档）。
12 枚 FAIL 逐名对照批 5 步骤 2 全量基线（同 12 枚）：
- 转绿一枚 `render/s6_9_heavy_budget_test.go`——在册墙钟 p50 贴线族，不计收益；
- 转红一枚 `ui/rendering/m5_keystroke_g1_test.go`——同族在册：本轮 FAIL 行为 G1 击键比率
  2.02（门禁 ≤1.5）、隔离三连 **1 绿 2 红**，先例=批 2 同机 A/B「纯净 HEAD 0.89–1.93 同带亦红」
  （`batch2/gates/new-fails-ab.txt`），批 6 变更面（examples+docs+一行 build tag）不触排版链
  ⇒ 登记为在册漂移族；
⇒ **对在册只减不增、零新增红**。

三、conformance 双后端
----------------------
`go test ./render -run TestConformanceDualBackend -count=1`：native rc=0、
`GPUI_BACKEND=go` rc=0（日志 `conformance-{native,go}.log`）。

四、整包 `-race ./render`（H20 修后竞态门回填）
------------------------------------------------
`CGO_ENABLED=1 go test -race ./render -count=1`（`race-render-full.log`）：**跑到底、
DATA RACE 命中 0**。FAIL 19 枚逐名对 H20 轮账（18=9 在册 + `TestDrawShadow…` 存量顺序耦合 +
S3c×7+R72 八枚隔离绿）：
- 同名 18 枚逐名一致（在册/已定性顺序耦合族照旧）；
- 净 +1 枚 `TestPresentResize_FullRecoveryWritesEveryBuffer`——**隔离 `-race -run` 单跑复绿**
  （rc=0 ok 1.448s）⇒ race  instrumentation + 前段全量 sweep 负载的尾效应，与 H20 轮已登记的
  「storm 窗口 race 慢速抖动隔离绿」同族，如实记不当回归也不当收益。
⇒ 整包竞态门恢复可用，本批唯一新增证据：**DATA RACE 0**。

五、链式跑数与留痕
------------------
首轮全链 `wrapup-chain-run1.log`（build-gate→sweep→conformance→race 串行、机器空载）；
census 修复后 build 段重跑（`build/*.txt` 为终态）。

六、遗留（不阻塞收官，账面在案）
--------------------------------
- 档位归属（`renderconv.FilterToRender`/`tex.Filter` 挂哪个引擎对象）待拍板（批 4 登记，④ 窗不冒充）；
- 性能门重定标（⑤ fps/p95 只报数所依）另线待拍；
- Canvas/Recorder 录制面未接 `ApplyBloom`（批 5 余账「要接另拍」，⑤ 窗形态已注明）；
- 整包 `-race` 的 19 枚红里 8 枚 S3c+R72 属「隔离绿顺序耦合」，待整包基线建账（h20-fix.md §六）；
- 存量 gofmt 三枚（render/context_layer.go 等）HEAD 外在脏，未碰。
