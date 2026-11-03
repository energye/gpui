# 批 1a · 透视多 quad 合批放行（2026-11-03，交付）

## 结论

同图 1000 张透视 quad 的提交成本：**1000 draw call / 250.0KB uniform slab → 1 draw call / 0.2KB**，与仿射批绘同量级。改动四处、零公开 API 变化、零 WGSL/顶点布局变化。

## 一、改动

| # | 位置 | 改了什么 |
|---|---|---|
| 1 | `render/internal/gpu/image_pipeline.go:1173-1181` | `canMergeImageDraw` 的透视判据：`a.Perspective \|\| b.Perspective → false` 改为 `a.Perspective != b.Perspective → false`。四角与权重是逐顶点数据（与 tint 同性质，`:1166-1170` 早已写明 tint 不进合批键的理由），透视↔透视按其余判据（图号/采样键/透明度/视口/图源尺寸/混合模式）照常合 |
| 2 | `render/internal/gpu/render_session.go:4350-4352` | 透视槽发射前把合并跨度收进连续段：`j = perspRunEnd(cmds, perspIdx, i, j)` |
| 3 | `render/internal/gpu/render_session.go:4361-4364` | `vertCnt` 由写死 `6` 改为 `(j-i)*6`（`firstVertex` 仍是打包后的 `pIdx*6`） |
| 4 | `render/internal/gpu/render_session.go:4523-4537` | 新函数 `perspRunEnd`：段内遇降级命令（`pIdx=-1`）或跳号即在空洞前断开——跨空洞一次 Draw 会读到邻张顶点（P3.5 收口第④洞「命令区间 vs 顶点下标」的同族防护） |
| 5 | `render/internal/gpu/perspective_quad_test.go:99-131` | 旧契约钉按新契约改写（见第四节） |

**没动的东西（有意）**：WGSL、36B/32B 顶点布局、绑定组布局、管线缓存键、`sliceImageResources` 的 `cmdStart/cmdCount` 分组（P3.5 修复保持）、`degradePerspectiveCmds` 的降级时机（D11 钉的「先降级→再打包→再上传」顺序未动）、仿射侧 `firstVertex = i*6` 不变式。

## 二、实测数字

探针与评估 §1.2 同一口径（同图 N=1000、同 opacity/采样/视口/图源），脚本跑完即删：

| 形态 | 轮次 | draw call | 单槽最大 quad | uniform slab/帧 |
|---|---|---|---|---|
| 仿射 | 修复前 | 1 | 1000 | 0.2KB |
| 透视 | **修复前** | **1000** | 1 | **250.0KB** |
| 仿射 | 修复后 | 1 | 1000 | 0.2KB |
| 透视 | **修复后** | **1** | **1000** | **0.2KB** |

日志：修复后 `batch1/merge-measure.txt`、修复前基线 `probes/persp-batch-cost.txt`。

## 三、能力单测（`render/internal/gpu/p37_persp_merge_test.go`，四枚，先红后绿）

| 测试 | 判据 | 真设备 |
|---|---|---|
| `TestP37PerspectiveMergePredicate` | 透视↔透视合；透视↔仿射双向不合；**仅权重不同不得分家**；透明度不同必须分家；跨内容版本必须分家 | 否 |
| `TestP37PerspectiveBatchBuildsOneDraw` | 4 张透视 quad → **1 次 Draw**、`firstVertex=0`、`vertexCount=24`、`cmdStart=0/cmdCount=4`、`vertBuf` 必为 36B 透视缓冲、`LastImageBatchStats()=(1,4)` | 是 |
| `TestP37PerspectiveRunSplitsAtSealAndAffine` | 裁剪封段处切 2 次 Draw、跨度 6/12、第二段 `firstVertex=6`；「透视/透视/仿射/透视」切 3 次 Draw、透视侧 draws=2/quads=3 | 是 |
| `TestP37PerspRunEndGuardsPackingGap` | `perspRunEnd` 纯逻辑：连续整并、跳号断开、`pIdx=-1` 断开、夹非透视成员断开 | 否 |

## 四、门禁抓到的一条反向钉（契约变更，显式登记）

`-race` 整包跑 `render/internal/gpu` 时红在 `perspective_quad_test.go:108`，原断言 `perspective quads must never merge (one draw each)` —— 那是 **P3.5 批 3 的旧契约**，被本批有意替换。处理是**按新契约改写并补足判别**（互斥双向 + 透视↔透视必合 + 权重不进键 + 透明度分家），不删钉。先例同 P3.6 批 2 把 `r1_residual_gpu_test.go` 的假语义钉改正向钉。

## 五、负向对照（逐条实测，红了再还原，日志 `batch1/negative.txt`）

| 编号 | 破坏方式 | 结果 |
|---|---|---|
| N1 | 谓词还原成「透视一律不合」 | **三枚同红**：谓词钉 + `合批后 draw call=4, want 1` + 分段钉 |
| N2 | 放行合批但 `vertCnt` 仍写死 6 | **两枚红**：`合并跨度 firstVertex=0 vertCnt=6, want 0/24` + `封段后跨度=6/6, want 6/12` ⇒ 证明「合了批却只画第一张」会被抓住 |
| N3 | 摘掉调用点的 `perspRunEnd` 守卫 | **绿——如实登记不可观测**：谓词已保证段内全为透视、打包按命令序连续，生产路径造不出空洞，摘调用点判不出红；该守卫属防御性，其本体由 N3b 判红。**没为凑绿把 N3 写成红** |
| N3b | `perspRunEnd` 函数体掏空 | `pIdx 跳号应在空洞前断开, got 3` 红 ⇒ 守卫本体有判别力 |

## 六、门禁清单（日志 `batch1/gates/`）

| 门禁 | 结果 |
|---|---|
| `go run ./scripts/apidoc` | **rc=0**（本批零公开 API，catalog 无需改行） |
| `go build ./...` / `go vet ./render/internal/gpu` | rc=0 / rc=0 |
| 五目标 cross build（linux/amd64、linux/arm64、windows/amd64、darwin/amd64、darwin/arm64）×{默认, nogpu} | **5/5 全绿**（每目标 `# exit: 0`；raw `go build ./...` 亦 rc=0） |
| conformance 双后端 | `TestConformanceDualBackend` **PASS**（`TestConformanceInner` SKIP=内层由双后端驱动，既有口径） |
| `-race ./render/internal/gpu` | **整包绿**（29.1s，含四枚新钉与改写后的旧钉） |
| `go test ./render/internal/gpu` 整包 | **绿**（20.6s） |
| `tools/per-file-tests.sh` 主 34 包逐文件 | **752 = 722 PASS / 21 SKIP_TAGGED / 8 FAIL / 1 NO_TESTS**；对基线 750=719/21/9/1：**零新增红**（`comm` 差集为空），基线九枚里 `ui_rendering__m1_noquadratic_test` **转绿**（墙钟比率门漂移，批 3 同族转绿在先），总数 +2=本批新增两枚测试文件所在包计数变化 |
| `go test ./render` 单进程整包 | 崩在 `conv.rs:519 invalid bind group layout`，命中测试与在册 **M4** 逐字相同（`TestContextFrameDamage_MultipleFillsSeparateRects`；`render/path_bounds_test.go` 单独跑 ok 0.309s），非本批引入，按逐文件口径不受阻 |
| **真窗复验（本批就做了，不留到批 6）** | `examples/ui_wr_p35_persp`（一帧多张透视 quad 的既有窗，正是本批改动的直接消费者形态）双后端各一轮、一轮一窗、WM_DELETE 正常关闭、每轮查显示服务（Xwayland 全程存活，本会话是 Wayland 会话 + XWayland :0，无 Xorg）：**native（webgpu）logic 9/9、pixel 17/17、金图 0.0000% / 763584 px、seeded=false（比对既有金图非重采）**；**go（gwgpu/gles）logic 9/9、pixel 17/17、金图 0.0000% / 763584 px、seeded=false**；两后端 `gpu_ops=51340` 相同、`cpu_fallback_ops=0`、`hitch_count=0`、fps_wall 57.1/56.9、p95 18.77/19.04ms、首帧 118.3/125.9ms。**⇒ 存量金图逐位零变化实测成立**（本批只改「几次 Draw」不改「画什么」被像素证据钉住）。注：合批收益不体现在 `gpu_ops`（它数的是入队 op 数不是 draw call），draw call 从 1000→1 的证据是 §二 的计数探针与 §三 的 `LastImageBatchStats()=(1,4)`。日志 `batch1/windows/` |

## 七、口径与登记

- **透视合批当前生产消费者 = 0**：能合批的能力已就绪，但没有生产方批量提交透视 quad（`DrawImageQuad` 是单张口；批绘腿要到批 1c）。按 AGENTS「生产与原子验证口径」，本批状态=**已验证待接线**，不得当接线标绿。
- 登记不做：透视腿 tint（批 1b）、批绘腿吃四角+权重（批 1c）、Canvas 级 4×4 栈（批 1d，形态依 `p2-ctm-callsites.md` 并存式结论）、透视下非 Normal 混合（`ErrQuadUnsupportedBlend` 保留，放开是独立决定）。
- 存量欠账复照：批 1a 不改 `SetEffectSurface`/`ExportImageBuf` 链路，M6 与本批无关、仍在池。
- 批 1 剩余：1b（透视腿补 tint，`buildPerspVerticesInto` 写死白，布局与 WGSL 已备好该通道）、1c（`AtlasSprite`/`DepthSprite` 加四角+权重 + engine 侧投影出口）、1d（Canvas 4×4 栈并存式）。
