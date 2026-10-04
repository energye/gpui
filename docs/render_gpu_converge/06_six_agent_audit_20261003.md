# 06 六路源码级盘点原样汇总（2026-10-03）

> 地位：只装结果，不定方案，不改代码。6 路任务书相同标准：从源码级每行代码检查到，源码功能与上下文读到分析到。
> 方法：6 路并行只读（主包/GPU 核/text/子包/CPU 侧/文档交叉），口径见各节头；grep 只能定位嫌疑，不能定逻辑；无 GPU 环境跑不了的写 SKIP，禁止静默假绿。
> 范围：`render/`、`gpu/`、`render/text`、`render/internal` CPU 侧、文档交叉。`ui/`、`examples/`、`video/`、`engine/` 只作调用证据，不展开。
> 与 00–05 的关系：本文件是 2026-10-03 六路盘点的原样汇总；01–05 是 2026-09-17 收敛扫描（全量统计 + 嫌疑抽查，不是终审）；实施进度四批见 `实施进度.md`。

## 0. 一句话

主路能走，闲的没删，碎的没并，账没对齐，文档红屏。

## 1. 规模实测（`ls` + `go doc` + `grep`，文档已过期）

| 地方 | 文档说 | 磁盘实测 |
|---|---|---|
| `render/*.go` | 204 顶层（01 §1） | 224 个，含测试，非测试 75 个 |
| `render/internal/gpu` | 213 | 222 个，非测试 90 个，加 `res` 9 个、`tilecompute` 22 个，共 253 个，加 `wgsl` 32 个 |
| `render/text` | 241 导出 | 220 个文件，非测试 108 个，加子包 57 个，共 277 个 |
| `render/scene` | 顶层 143 | 实测函数 67 加类型 64，共 132 |
| `render/recording` | 顶层 85 | `go doc` 59，文件 18 个 |
| `render/surface` | 顶层 53 | `go doc` 33 |
| `render/gpu` | 顶层 12（§6.8 表头写 18） | `go doc` 8 |
| `render` 主包导出 | 类型 104、函数 131、方法 185、常量 133、变量 16 | 实测类型 118、函数 148、方法 196（§3.1 写 106，§0 写 185，前后矛盾） |

机器校验 `go run ./scripts/apidoc` 红屏：缺 37 个符号没进目录（Atlas/Depth/Vert/Frame 计时/Shared/Vram/Note/RunErr 等）。漏的正好是主路：多窗共享、显存账本、绘制新口、帧计时那批。按纪律非零退出就不能合入。

## 2. 第 1 路：render 主包（75 非测试，12 个逐行，其余头段 + 符号 + 调用 grep）

生产主链路完整：建窗（`present_target.go:339` PresentTarget + `backend.go:88` 二选一）→绘制（Context 族）→提交（`present.go:97` PresentFrame 系）→降级（`oom_purge.go:39` + `oom_exit.go:30` + `shared_recovery.go:48`）→视频直传（`video_direct.go:87` 桥 + 双池）全有生产调用；目录状态与实测基本相符，未发现主包级断链。但“健康”仅限静态接线，像素级三证据未在本路验证。

后端耦合（AGENTS 合入前必查口径，测试/`present_swapchain`/`CreateInstance`/blank-import 除外）：原始 18 行。允许例外 5 处（`backend.go:23`、`present_target.go:28` 注册行；`backend.go:102`、`present_swapchain.go:20`、`present_swapchain_gl.go:18,:29` 建链句）。真耦合 9 处，全在记账线：`adapter_policy.go:263,274`、`shared_budget.go:56,61,66,71,108`、`vram_ledger_forward.go:18,22`，全是直接调 `webgpu.Vram*`。通用插口 `hal` 里账本已经有了（`gpu/hal/vram_ledger.go` 在，`gpu/webgpu/vram.go:20-27` 已转调），但 `render/` 读数口没切过去。账搬了，读数没搬，GL 窗和 WebGPU 窗分叉。关窗后账本从 53 卡在 34 下不去的原因之一。

重复入口（本路所见）：D1 纯色（`SetRGB` 族 ↔ `Solid*+Set*Brush`，已收敛为兼容别名）；D2 渐变（`CustomBrush.Linear/RadialGradient` ↔ 数据式画刷，前者已标 Deprecated）；D3 裁剪三套（`ClipRect` ↔ `ClipRectOp(Intersect)` 已委托；`Context.Clip` ↔ ui `PushClipPath` 封装；`scene.ClipState/Stack` 第三套）；C1 四体系（`Context` ↔ `scene.Scene` ↔ `recording.Recorder` ↔ `surface`，仅 Context 进 ui 产线）；D4 形状三处（`Path` ↔ `PathBuilder` 后三者已委托，RoundRect 例外 ↔ `Context.Draw*`）；W 呈现三层（`PresentTarget.PresentWith(Auto)` ↔ `Context.PresentFrame(Damage)` ↔ `frame.PlanFramePresent`，Auto 决策语义重叠）。新增：建设备/选卡主包 ↔ internal 镜像（`adapter_policy.go:102` ↔ `internal/gpu/device.go:16,93`；`present_target.go:22` 第三份重试循环）；视频池独立于通用图缓存（设计有意，但与 `PictureCacheFairMax` 分账并存，跨池总账无统一视图）。

前 3 风险：① `webgpu.Vram*` 9 处直调，按口径应打回，P3 统一到 `hal` 前多窗预算与 GL 窗记账分叉；② `RecoverSharedDevice` 无生产直调、仅错误驱动，`PurgeEvictables` 注册仅 image-cache + picture-texture 两家，字形图集页有意不进 purge，多窗高压下口径碎且不完全；③ `path_boolean.go:28` 像素采样近似 + >2048px 截断 + 差集挖洞已知失效仍为仅测试，`pattern.go/Painter/DrawMesh/StrokeString/Dither/Trim/WithCorners/Discrete/ClipOp` 差集长期堆积并存，新 API 易再造入口。

未读缺口：75 个中 63 个仅头段 + 符号 grep + 调用 grep；`context.go`（2841 行）仅读头 80 行，`text.go`（1341 行）、`software.go`（1523 行）、`present_target.go`（1274 行）、`video_direct.go`（1131 行）、`context_image.go`（1108 行）、`vertices.go`（1074 行）、`filter_ops.go`（915 行）均只读头 50–80 行；GPU 单测/真窗零运行（无 `WGPU_NATIVE_PATH`/`DISPLAY`）；任务书“204 非测试”与磁盘“75 非测试”差值未对账；`Context` 185 方法未逐个点名调用方，以族为单位给证据。

## 3. 第 2 路：render/internal/gpu（253 go + 32 wgsl，11 个逐行，约 30 个关键段精读）

大体健康，P1–P6 代码层面基本落地（与实施登记一致），但 3 处登记与代码不符、4 处残留发黄。最大风险是 Submission 的 `Track` 生产零调用（in-flight 计数从没真正转起来）加 `OnSubmittedWorkDone` 建而不用。`go build ./render/internal/gpu/...` 通过；`res` 包单测全绿；GPU 单测缺 `libwgpu_native.so` 带原因 SKIP。

D1–D7：D1 绿（入队 6 函数全经 `viewToResView`，`gpu_render_context.go:1400,1472,1490,1512,1535,1557`；消费 `ResolveCommandView`，`render_session.go:4657,4671`）；D2 绿带黄（session 集走 retireFn，`gpu_textures.go:600` + `render_session.go:738`；`StencilRenderer/textured_stencil` 自有 textureSet 无 retireFn，靠提交后同步 Map，`stencil_renderer.go:1049-1055`）；D3 绿（Ref 保活 + prevCmdBufs + 四处 Drain）；D4 绿带黄（evict 进 pending，`image_cache.go:440`；`ReleaseEphemeral` 在 runClean 与 readback；`BeginFrame` 无 `ReleaseEphemeral`，无提交帧会累积）；D5 绿（11 处 `RetireBuffer`，Drain 齐；单测 3 项 PASS 实测）；D6 绿（`bindgroup.go:200-216` + `:334`）；D7 绿带黄（主漏斗 + readback 接 scope，`:4238,:5382`；全部直连点零 scope）。

P1–P6：P1 绿（res 4 文件 + 单测全过实测 ok）；P2 绿带黄（防御 + scope + 指标齐；`PopErrorScope` 经 Async + `WaitAny` 轮询，`webgpu/device.go:463-469`，登记“无 1ms 轮询”只在成功路径成立；直连点未接）；P3 绿（句柄化 + Raw 回退 + BG 指针 key 回滚有注释）；P4 绿（retireFn + 四 Drain + frameScratch + ImageCache pending；`TestTextureSet` 系列 PASS 实测）；P5 绿带黄（BG 四类 key 对；TexturePool 废弃标注齐；`invalidateTextBindGroups` 直接 `Destroy`，`:3780`；`PipelineCache` 新旧并存，`renderer.go:139` 用旧）；P6 黄（sub/SubmitDone/lastSubmissionIndex/InvalidateForDeviceLoss/grow 队列绿；但 `sub.Track` 生产零调用，全树仅 res 测试调用；在途计数从未启用，延迟释放实际靠 pending 队列；`OnSubmittedWorkDone` 建而不用，生产仍靠 vsync/Map；`offscreenPool` 实际已解禁复用，`gpu_render_context.go:3527-3562` R6-3，与登记“暂缓”矛盾）。

池/重试/预算：尺寸池六处并存（TexturePool 废分配、DestroyAll 仍在 `gpu_shared.go:391` 链上；textureSet；stencilPool cap8；offscreenPool 已复用 bucket4 + 32MB；filter 乒乓；dualTex bucket8 + 32MiB），未收敛进 `res.Cache`（Cache 生产零使用者）。延迟释放实际五队列（pendingTexRetire、pendingBufRetire、ImageCache.pending + ephemeral、pendingBindGroupRelease、pendingViewRetires），另 `res.Cache.pending` + Submission jobs 建而未启用。purge 四条仍在（生产注册仅 image-cache 一处），glyph/filter/dualTex 不在链上。OOM 重试现为四级（flush + WaitIdle → PurgeEvictables → PurgeAcceleratorSurfaceResources → 降 sample，`gpu_textures.go:636-678`），03 文档“三级”已过时。预算至少九口径仍碎。直连 `queue.Submit` 约 20 处，仅 2 处有 error scope。后端耦合（创建入口外）：`logger.go:52`、`gpu_shared.go:306,758,377,403`、`vello_accelerator.go:155,1330`、`backend.go:80`；类型层面仍全 hal。

前 3 风险：① Submission 围栏从未真正启用（Track 零调用 + 回调建而不用），安全全靠“提交后即放 + vsync 屏障”时机假设，提交与 BeginFrame 之间插 resize 仍有窗口期；② `invalidateTextBindGroups` 直接 Destroy 旧 BG（`:3780`），图集换代时在途 CB 可能仍引用旧 BG，全树唯一绕过延迟队列的旧 BG 路径；③ 登记与代码三处脱节（offscreenPool 已解禁但登记写暂缓、OOM 重试已四级但 03 写三级、Pop“无轮询”但实际走 WaitAny）。

未读缺口：50 余非测试文件只头读 + grep；132 测试文件仅关键 12 个读 + 跑；dual-tex 5 个提交点、filter `:1306` 之后、pattern/linear 提交前编码段未逐读；§6 真窗矩阵需 GPU 机；`go test ./render/text/...` 回归未跑（本路范围外）。

## 4. 第 3 路：render/text（277 文件，约 40 个精读，符号级 277/277）

主路通、批量与颜色双通道都有生产调用，`Shape→DrawShapedGlyphs→GPU` 单源已闭合。`Shape` 经 `ui/rendering/text_layout.go:777`，`SegmentText` 经 `:673,685`，`SegmentReuse` 经 `row_reuse.go:65`，批量口 6 处生产点（`ui/rendering/text.go:1031,1118`、`text_picture.go:215,242,254`、`ui/scene/picture.go:234`，另 `render/text.go:167` 与 `render/scene/gpu_renderer.go:257`），颜色口（`ui/rendering/text.go:1105`、`ui/scene/picture.go:231`），`WrapText` 已接三处（`text_layout.go:250`、`paint_context.go:383`、`paragraph.go:414`），`RuneAdvance`（`face.go:81`）经 `text.go:853` 生产调用。

没做完的是测宽：`LayoutText`（`layout.go:129,141,229`）在 ui 零调用，仅测试/工具用；`measureLine:484` 与 `measureRunString:252` 仍用 `text.Measure` 估算，未按洞 3 切真 advance；洞 3 只算做完一半（断行落地、测宽/布局仍估算）。四个 Clear 全仅测试调用（`shape_result_cache.go:104`、`multi.go:431`、`autohint.go:813`、`fontscan_fallback.go:177`），目录 §6.1 标生产夸大；purge 链只有图/纹理两注册，字形/整形缓存未进链。`text/cache` 零生产 import，目录漏标。`text/msdf` 无 UI 直调，经 `internal/gpu`（`gpu_text.go:40,80`、`text_pipeline.go:934,968`）间接生产，目录口径含糊。pelican 标题（`examples/ui_render_pelican/scene.go:493` 逐字节点 + `:476` 烘图 + 右移 1.1 仿粗）绕过整形/分段/排版器，复杂文种连字/kerning 在此路径丢失。

目录差异：`DrawShapedGlyphs` 行号全旧（988→1031、1077→1118、205→215、232→242、244→254）；`MeasureString/WordWrap` 行“UI 布局未接”已过时（已用估算口径，但非真 advance）；洞 3 前置行（`measureLine:331/measureRunString:234/drawTextWrapped:363`）断行部分已落地但目录与计划都没更新半成品态。

前 3 风险：① 测宽仍是估算口径，CJK/Latin 混排与变体字重下换行与 caret 差 1px 级，洞 3 “偏差 <1px”没立住；② 目录行号与接线态失真（6 处漂移、`Clear*` 虚标、`text/cache` 漏标）；③ pelican 标题逐字烘图是制度性绕行，标题换复杂文种露馅。

未读缺口：精读到函数体约 40 文件，余下仅符号 + 调用 grep（TT 字节码、`autohint` 全算法、`gpos/gsub` 全表细节未逐行）；第三方排版 277 文件不在本路范围；真窗像素证据（m5 回退 0、B 区真彩）未独立复现。

## 5. 第 4 路：子包群（55 非测试，全部读到）

该留（有生产链路事实）：`render/scene` 16 文件（render 内部 GPU 后端消费，`internal/gpu` 多文件 import + `filters/register.go:21` 用其 `Rect`）；`filters/register.go`（经 `render/gpu` 副作用激活全部真窗）；`render/gpu` 的 init 注册与 OOM/lifecycle 钩子（`gpu.go:43-58`）。

闲的（零生产消费事实）：`render/surface` 7 文件、`render/svg` 7 文件、`render/recording` 12 文件（含 `backends/raster`）、`render/raster` 1 文件、`render/render` 9 文件；`render/gpu` 的 `SetDeviceProvider/AbandonDevice` 生产零调（`ui/embedder` 零命中，仅 3 个 render 测试文件 + 1 个示例 `ResetAccelerator`）。`DrawImageNine` 只有门面（`ui/rendering/image_draw.go:58-63`）加 2 个示例；`DrawMesh`（`vertices.go:989`，`DrawMeshEx` 在 `:470`）生产外部调用零；`DrawShapedGlyphs`（`render/text.go:339`）与颜色口（`:392`）确在用。`TexturePool`（`texture_pool.go:30-39`）头写废弃但 `DestroyAll` 仍在三处 purge 路（`gpu_shared.go:392,501,579`）。`offscreenPool` 已从禁用反转为复用启用（R6-3，`GPUI_NO_OFFSCREEN_POOL` 门控），04 文档仍写禁用。§7.5 十二组重复全部仍成立，收敛 4 项与源码一致。

目录反例/过时：§6.8 “顶层 18”过时（实际 `go doc` 8，以 §0 的 12 为准）；04 文档 `TexturePool` 行号（24-28 → 30-39）、`DestroyAll` 行号（360-363/471/550 → 392/501/579）过时；04 §2 把 `DrawShapedGlyphs` 列为反例是旧文（现目录已标生产）；任务口径“render 同名子包 8”漏计（实际 9，漏 `float_target.go` S66 HDR 路，目录未收录）；§6.3 “PDF/SVG 后端仓外”易误读（raster 后端仓内已注册，未接的是外部后端，无生产 `NewBackend()` 调用）。

前 3 风险（事实）：① 同一功能 4–5 套入口并存（Context/scene/recording/surface + `render/render` stub），12 组重复全在，新 API 易再长入口；② `TexturePool` 废弃与销毁并存、`offscreenPool` 复用反转而文档仍写禁用，配额语义会被误读；③ GPU 设备生命周期生产零调用，换设备与 OOM 恢复路径无生产覆盖。

未读缺口：全部 `*_test.go`（约 45 个）未读；`scene/encoding.go` 1000 行后与 `renderer.go` 1000 行后截断阅读；主包争议点仅片段读；`ResolveSurfaceLifecycle/TextureOOMCount` 生产调用链未闭环；目录 §6.1/§7.1/§8/§9/附录计数未用 `go doc` 重跑；`ENGINE_UI_WIDGET_RENDER.md` 未读。

## 6. 第 5 路：internal CPU 侧（61 非测试，全覆盖，1 个部分深读）

零 GPU 分配属实（grep 仅 2 条注释命中，零 GPU 类型、零分配调用）。blend/clip/color/image/parallel/raster/stroke/wide/cache 职责清晰，注释含算法出处与误差声明。`ErrFallbackToCPU`（`gpu/brush_advanced.go` 10 处）+ shared-encoder + `pass_scratch`（`context_pass_scratch.go:69-144`）三条回退与 CPU 语义一致。

池上限碎片：通用 `cache.Cache` 按 softLimit 条数（超限删到 3/4）；`ShardedCache` 每片默认 256；`image.Pool` 每桶默认 8；`parallel.TilePool` 无上限按尺寸建桶、边缘丢弃；`WorkerPool` 队列 `workers*4`；filter `kernelCache` 64 条满清半（非 LRU）；`tempBufferPool` 建 4M float（约 16MB）、回池 `cap≤16M`、Get 全片清零；shadow 无池直接造；blend LayerStack 缺省 8 桶；raster 只有复用缓冲无条数上限。GPU 侧对照：`gpu TilePool` 4096，几何四件套 512/256/256/512。条数 vs 字节 vs 无上限三类口径并存。

测试缺口：总数 blend 12 / clip 3 / color 3 / filter 5 / image 10 / parallel 6 / raster 21 / stroke 2 / wide 5 / cache 2。Skip 全带原因（short 跳 stress、退化输入跳、缺外部 C++ 件跳），无无原因空转。只断没崩：`alpha_runs_extended:202`、`software_coverage:655`、`sdf_accelerator_coverage:403`、stress 全文件只数 scanline/edge。金图容差：全局 `imagediff.go:62` d>2 只比 RGB 不比 A；blend 容差 2；color LUT 0.01/1byte；filter `colorApproxEqual`；image draw 容差 2；raster flatten 0.1px。`examples/compute_*` 手工看图非门禁，stress 需 tag 默认不跑，golden 3 项靠外部件。

前 3 风险：① `parallel.TilePool` 无界 + shadow 无池 + kernel 非 LRU，异常尺寸/半径轰炸可致内存抖动；② golden 外部件 + stress tag + 只比 RGB，门禁只看绿会漏画；③ `blend.Mode` vs `BlendMode` vs `image.BlendMode` 三套混合枚举并存，`wide.SourceOverBatchAA` 与 blend 批式重复（有避环注释）。

未读缺口：`raster/analytic_filler.go` 千行后半只 grep + 抽查；全部 `_test.go`（69 个）未逐行；`tmp/*.png`、`testdata/golden/*.png` 未重比；无编译级 callgraph，“无 GPU 分配”基于 grep 符号判否。

## 7. 第 6 路：文档交叉（只读不改，数字本次实测）

增量主线（Backend/Video/Quad/选卡/降级）确已同步，04 死代码 5 抽查全成立、03 重叠与 §7.5 互证，方向诚实；但 apidoc 红屏 37 漏 + 规模数过期 + 3 处凭印象标注，违反 API 同步纪律。

规模差异：主包类型 104→118、函数 131→148、方法 185→196；text 241 vs `go doc` 157（口径不一，无复算证据）；scene 143 vs 132（多 11 无解释）；recording 85 vs 59；surface 53 vs 33；svg 15 vs 14；`render/gpu` 12 vs 8；`doc_package_map` render 204 vs 224；internal/gpu 213 vs 222；`gpu/types` 16 vs 18；`gpu/context` 23 vs 27；`gpu/rwgpu` 85 vs 84；`gpu/webgpu` 71 vs 77；`gpu/hal` 23 未入尺子表。P2/P3-A 视频两行与 §3.11 重复登记且状态相反（P0 仅测试 vs P2 生产在用，未写清分界）；`RuneAdvance`/`SegmentReuse` 在 §6.1 有行但 §0 未复算；`Backend` 在 §3.7 有行但 §8/§9 缺方法/常量。

状态行复核：§7.1 Present/Context/Brush/渐变/损伤/编码器成立；`DrawShapedColorGlyphs` 生产成立但 `/tmp/m5_verify/` 按纪律不得作合入证据；`SetVsync` “两平台生效”与 `present_target.go:195-201` + `:800-804` Wayland 恒非阻塞矛盾，属残留；选卡/降级生产成立但 `nvidia-smi` 目检无 JSON 落盘；`DrawShapedGlyphs` 在 §3.8 已改生产但 §7.2 仍列“无消费者”表内自相矛盾，行号全漂移；§7.2 svg/surface/recording/raster/布尔/ClipOp 差集/Trim/SetDither/Pattern/Painter/StrokeString/DrawMesh 成立；`DrawImageNine` 按“门面 + 示例不算 widget 生产”则成立，但目录未写口径；`SetDeviceProvider` “`examples/ggcanvas` 注入”全仓无此目录，属凭印象，打回；视频“17 扇”与实数 21 对不上，`VideoTexturePool Stats` 在 §8 写仅测试与 §3.11 写生产矛盾；§7.3 `Clip()` 修复存在但像素数无落盘基线，不符合三证据；§7.5 编号错乱但 12 项可信。

后端耦合：允许例外 5 处合规。注释误报 3 处（`accelerator.go:124`、`path_boolean.go:26`、`device_provider.go:23`），现行 grep 会命中，建议加词边界。打回 9 处代码（同第 1 路）。桥齐（`hal` 23 + `types` + `gpucontext`，`device_provider.go:14-16` 走桥无断言）。Vram 留 P3 有案但矛盾：账本已搬 `hal`（GL 计划体称 2026-09-30 搬完，GL 三处记账，render 零改动），实际 `gpu/hal/vram_ledger.go` 已存在、`gpu/webgpu/vram.go:20-27` 已转调，但 `render/` 仍直调 `webgpu.Vram*` 未切 `hal.Vram*`，“零改动”不成立，须补“render 切 hal”一项。

前 3 风险：① 37 符号漏同步阻塞合入（`SharedWindowCount/SharedDeviceGeneration/PictureCacheFairMax/RecoverSharedDevice` 多窗主路、`Vram*` 账本主口、`DrawAtlasEx/DrawMeshEx/DrawDepthSprites/DrawVerticesEx` 新口）；② `render` 仍直调 `webgpu.Vram*`，GL 与 WebGPU 账本分叉，`present_target.go:398` Go 后端仅 X11/Wayland 而目录写四平台全支持，跨平台表须一次标齐；③ 证据链弱（`SetVsync` 矛盾、ggcanvas 不存在、`/tmp` 不可复现），须按“消费者 → 文件:行号；实测 → 窗名/日期/JSON”重写 §7，apidoc 归零后再合入。

未覆盖缺口：37 符号中多窗共享与显存口不可留白；`render/render` 同名子包无族表；`render/text` 220 文件未逐函数比行为；`gpu/shader` 子树截断；双实现 double count 与块式漏计已证实；`WIDGET_RENDER` §10 无对应修订行；Wayland/X11 真窗 JSON 无落盘路径。

## 8. 前 5 风险（六路交叉，按严重排）

1. 37 符号漏同步，机器校验红屏，多窗共享和显存主口都在漏单里，先补目录再谈绿。
2. `render` 直调 `webgpu.Vram*` 9 处，账本搬而读数没搬，多窗和 GL 有分叉风险。
3. 提交围栏没真正启用加回调建而不用，安全靠时机假设，提交和帧开始之间插改尺寸仍有窗口期。
4. 旧绑定组直接销毁一处（`render_session.go:3780`），图集换代时在途命令可能仍引用旧资源。
5. 测宽估算加标题绕排版，换复杂文种先出问题，目录写成已完成会误导后人。

## 9. 没做到的（六路诚实汇总，不假绿）

主包 63/75 只读头加 grep；GPU 核 50 余文件只头读加 grep、测试 132 只精读 12、5 个提交点没逐读；文本 240 函数体没逐行、第三方排版没读；子包测试 45 没读、计数没重跑；CPU 侧 analytic 后半只抽查、69 测试没逐行、金图没重比；文档 `render/render` 无族表、shader 截断。真窗像素和单测都没跑，本机没卡没显示服务，损伤呈现和降级链只有静态证据。
