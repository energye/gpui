P3.7 批 6 · 窗 ① 门禁账（2026-11-05）
======================================

本轮改动面（`git status` 实数）：修改 2 文件、新增 6 路径。

- 渲染层新增：`render/context_image_batch.go`（公开 `ImageBatchStats` + `Context.ImageBatchStats/
  ResetImageBatchStats`，可选接口探针，CPU-only Context 回零值）、
  `render/internal/gpu/image_batch_census.go`（跨帧累计账 + session 三方法）、
  `render/internal/gpu/image_batch_census_test.go`（2 枚）、`render/p37_6_image_batch_test.go`（2 枚端到端）。
- 修改：`render/internal/gpu/render_session.go`（字段 + `buildImageResources` 一处记账，共 4 行）、
  `render/internal/gpu/gpu_render_context.go`（两枚转发，共 16 行）。
- 示例：`examples/engine/p37_persp_batch/`（新窗 + `testdata/goldens/*_{native,go}.png` 两份金图）。
- 文档：`docs/RENDER_API_CATALOG.md`（§3.17 新节 + §0/§3 计数）、`batch6-window-plan.md` §5、
  本目录 `window-1-persp-batch.md`/`window-1-runs.md`/本文件。

跑过的门禁（逐条实测，不凭印象）
--------------------------------
| 门 | 结果 |
|---|---|
| `go build ./...` | rc=0（**取状态位口径修正后复测**，见下方「脚本口径」） |
| `go vet ./render ./render/internal/gpu ./render/internal/filter ./render/hdr ./engine/fx ./examples/engine/p37_persp_batch` | rc=0（同上复测） |
| `gofmt -l`（本轮触及路径：`examples/engine/p37_persp_batch`、`render/bloom.go`、`render/context_image_batch.go`、`render/internal/gpu/image_batch_census.go`、`render/internal/filter`、`engine/fx`） | 净。**另测全包 `gofmt -l render` 命中 3 枚存量未格式化文件**（`render/context_layer.go`、`render/internal/clip/stack.go`、`render/p37_3d_test.go`）——`git status` 对这三个文件为空，HEAD 即如此（`internal/clip/stack.go` 是 `dx*cos+dy*sin` 这类空格），非本批引入，不折进本批提交，登记待另理 |
| 逐文件回归 `tools/per-file-tests.sh`（触及 5 包：`render`、`render/internal/gpu`、`render/internal/filter`、`render/hdr`、`engine/fx`） | **408 文件 = 387 PASS / 6 FAIL / 15 SKIP_TAGGED**，`FAIL_TOTAL=6`；6 条逐条对在册红名单 `baseline/tests/failures.tsv`（`context_misc_draw_gpu`/`filter_present_coherence`/`rasterizer_tile_value`/`s6_9_heavy_budget`/`text_transform_gpu`/`video_direct_upload`）**全在册，零新增红**；15 条 SKIP 全是 bench/无 Test 函数/tag 门（含 `render/internal/gpu/stencil_metal_repro_test.go`，metal 本机不可验） |
| `go run ./scripts/apidoc` | **rc=0**（新公开口进 §3.17，§0 与 §3 计数同步；计数按编译视图重测：`go doc -all render` 类型 **157**、`func (c *Context)` 方法 **228**，与目录写的 156→157 / 226→228 一致——源码文本口径 `^func [A-Z]` 是 2021，因为 tag 变体重复计数，不作口径） |
| `-race ./render/internal/gpu`（CGO_ENABLED=1） | ok 29.188s |
| `-race ./render`（CGO_ENABLED=1） | **跑不到底：不是竞态（`DATA RACE` 命中 0 次），是整包中止**——见下节 H19，与本轮改动无关（HEAD 同点复现） |
| 真窗 ①（双后端各一轮，一轮一窗、自退正常关闭） | native 与 go 门禁相各 `probes=14/14 golden=0.0000% (921600 px)`，`merged=10/7`；跑前跑后 Xwayland 存活。见 `window-1-runs.md` |

脚本口径（本轮自己踩的，写成规矩）
----------------------------------
`cmd 2>&1 | tail -5; echo RC=$?` 取到的是 **`tail`** 的状态位，不是 `cmd` 的——本轮门禁脚本就是这样写的，
所以那一轮打印的 `RACE_RC=0` 是假信号（真实是 rc=1 中止）。批 5 步骤 2 的「整包 `-race` rc=0」
同一条路走死（见下节）。**规矩**：门禁命令一律「先落文件再取 `$?`」或 `set -o pipefail`，
本文件里的 build/vet/apidoc 三项已按此复测，rc 全 0（数值与首轮一致，但这次是真取到的）。


本轮**未跑**的门禁与理由（挂账不隐）
---------------------------------
- **五目标交叉构建（默认/nogpu 10 段）与 64 包全量 sweep**：按批 3 收尾登记的口径「交叉构建与全量
  sweep 按『批』跑，子步只跑触及包 + 真窗」，窗 ① 属批 6 子步，排在批 6 收尾补跑并回填 §7。
- **conformance 双后端**：本轮零 WGSL/管线改动（只加计数器与读口），随批 6 收尾一起做。

新登记的洞 H19（整包 `./render` 在 `TestP37ProjectionCounters` 处硬中止）
--------------------------------------------------------------------------
现象（三条都实测）：

1. `CGO_ENABLED=1 go test -race -count=1 ./render` 与 **不带 -race** 的 `go test -count=1 ./render`
   都在同一处中止：`--- PASS: TestP37ProjectionAffineSemantic` 之后 `=== RUN TestP37ProjectionCounters`
   即断，wgpu-native（Rust）在 `src/conv.rs:519:22` 报
   `invalid bind group layout for pipeline layout descriptor` → `panic in a function that cannot unwind`
   → `thread caused non-unwinding panic. aborting.`（SIGABRT，整包退出码 1，后续测试不再执行）。
   `-v` 复跑两次 + 不带 -race 一次 + HEAD 一次，共四次同点同因；`DATA RACE` 命中 **0 次**（不是竞态）。
2. 单测隔离跑绿：`go test -race -run '^TestP37ProjectionCounters$' ./render` → ok 1.385s。
   ⇒ 同进程内前序测试把共享句柄状态带坏了，属顺序依赖，不是这一枚测试自己的问题。
3. **与本轮改动无关**：把批 6 的 6 条新增路径移开、`render_session.go`/`gpu_render_context.go`
   回退到 HEAD（`c3db7e57`，纯簿记枚，其 Go 代码面等同批 5 步骤 2 交付枚 `0e5d2549`——
   `git show --name-only c3db7e57` 里 `.go` 命中 0）后，整包跑仍在同一点、同一句 Rust
   panic 中止（`tmp/b61-head-render.log`）。备份与还原逐字节核对，`go build ./...` 复绿。

定位（到行号）：中止栈 `render.(*Context).Close` → `flushGPUAccelerator` →
`gpu.(*GPUShared).ensurePipelines`（`render/internal/gpu/gpu_shared.go:863`）→
`(*SDFRenderPipeline).EnsureSharedBase` → `createPipeline`（`render/internal/gpu/sdf_render.go:326`）→
`device.CreatePipelineLayout`（同文件 :373-379，把 `uniformLayout` + `clipLayout` + `maskBindLayout`
交给 wgpu-native）。整包只有 **1 个 device**（日志里 `J5TRACE Open device=` 唯一值），
所以不是跨设备误用，而是**同一 device 上 shared clip/mask bind layout 的句柄已经失效、
仍被 `GPUShared` 缓存并再次喂给 `CreatePipelineLayout`**——正是 `gpu_shared.go:884-888`
那段注释自己点过的家族（「one window's close free handles another window's frame was creating」）
的残余形态：关/清路径把 pipeline 句柄清了、却没把 `s.clipBindLayout`/`s.maskBindLayout`
一起清（或被双放）。要钉的是释放侧（`convex_renderer.go:709-715` 的 owned/shared 分别是非对称的，
`maskLayoutOwned` 只保护 mask 一路）。

连带纠一处假绿：**批 5 步骤 2 门禁账里 `CGO_ENABLED=1 go test -race ./render`（整包）记的 rc=0 不成立**。
本轮复现证明该中止在批 5 交付的代码面上就在（HEAD 未含批 6 改动也中止），当时那个 rc=0 是
**管道取状态位的假绿**——本轮我自己的门禁脚本也重演了同一写法错误：
`go test -race … 2>&1 | tail -5; echo RACE_RC=$?` 取到的是 `tail` 的 0，不是 `go test` 的 1。
批 5 其余结论（触及包逐文件、金图、负向、双后端）不受此影响：那几项都是逐文件/逐窗跑的，
逐文件进程隔离，正好绕开了这个同进程中止。

口径影响（必须写清，别只挂着）：整包 `-race` 这一道门在本仓**当前不可用**（跑不到底），
而逐文件门禁口径看不见它——这是逐文件口径的盲区，不是它的豁免。所以：
`-race ./render/internal/gpu` + 触及包逐文件 + 真窗，是本轮实际拿到的竞态证据；
`./render` 整包的竞态证据要等 H19 修完才能重新拿到。
