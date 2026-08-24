# P1 结论复核报告（批 C / D / E）

复核方法：逐条回当前工作区源码定位，关键项（E1）实际跑 `go build ./...` 实测。行号均为本次复核实测。

## 总表

| 编号 | 原结论 | 判定 | 证据（文件:行号） | 备注 |
|---|---|---|---|---|
| C1 | OS 增量呈现丢参数；DamageRectSetter 无生产实现者 | **成立** | `gpu/webgpu/surface.go:312-314`（`PresentWithDamage(st, _ []image.Rectangle)` 直接 `return s.Present(st)`）；`gpu/webgpu/surface_browser.go:179` 同样丢参数；上游 `swapchain.go:1061-1063` 认真传 rects 但下游扔掉；`render/integration/ggcanvas/canvas.go:630` 定义 `DamageRectSetter`，全仓唯一实现是测试 mock（`canvas_test.go:1069,1090`）；生产断言点 `canvas.go:349` 永不命中 | 原结论写「render surface.go」路径不准，实际在 `gpu/webgpu/surface.go`；注释已自认 wgpu-native 不支持 |
| C2 | MSAA 中帧 LoadOpLoad 读已 discard 内存 | **成立** | `render/internal/gpu/render_session.go:5368-5372`（`frameRendered==true` 时 color/stencil 均 LoadOpLoad）；`:1002-1020`（MSAA 路径 View=msaaView、ResolveTarget=target、**StoreOpDiscard**）；`:5429` 每次提交后置 frameRendered=true | 同帧第二次 pass 对已 discard 的 MSAA 内存做 Load，End 时整幅 resolve 会把垃圾像素盖到上一遍的好内容上。触发条件：MSAA(sampleCount>1) 开启且同帧多次 flush（CPU fallback 中途插画等），非每帧必现 |
| C3 | Auto 选管线死路由：ClipDepth/OverlapFactor 判据存在但生产不填充 | **成立** | `render/pipeline_mode.go:60,65` 用 `ClipDepth`/`OverlapFactor` 判档；`render/internal/gpu/gpu_render_context.go:1457-1458,1571-1572` 只累加 PathCount/ShapeCount，全 internal/gpu 目录 grep `sceneStats.ClipDepth|OverlapFactor` 零命中；绘制路由只认显式 `rc.pipelineMode == PipelineModeCompute`（`:1461,:1578`）；Auto 下 `effectivePipelineMode()`（`:2860-2870`）算出的结果只在 `flushVello`（`:2776-2785`）用于冲刷一个永远空的 pending 队列 | 启发式的答案不改变任何渲染行为；且 Auto 每帧触发 `CanCompute()` 惰性初始化 Vello 管线，纯浪费 |
| C4 | 默认 full_paint | **成立**（属实，但属设计现状） | `ui/embedder/pipeline_app.go:365-371`（`NewPipelineApp` 里 `SetPresentPolicy(PresentPolicyFullPaint)`）；`:120-122` `useRetained` 默认 false；`:176-186` 仅显式 `SetPresentPolicy(retained/hybrid)` 才开 retained；`:888-898` 稳态帧 compositeOnly 由 useRetained 决定 | 注释自述「W0 默认 full_paint、W6 才全局默认 retained」，是有意的分期决策，不是漏接；算不算问题看验收口径 |
| C5 | compositor-only 动画断头 | **成立**（路径有笔误） | 实际文件在 `ui/scene/`：`ui/scene/compositing.go:11` `ClassifyDirty` 全仓仅测试调用（`layer_test.go:57`、`status_test.go:96`）；`ui/scene/packet.go:56-67` `Mutations`/`CompositorDirtyIDs` 无生产者填充（全仓赋值只有 CloneShallow 拷贝 `packet.go:81,89`）；`ui/animation/implicit.go:44-56` AnimatedOpacity 每 tick 产 Mutation 但 `OnMutation` 回调无任何生产接线——animation 包唯一使用者 `examples/ui_l1_spinner/main.go:61-65` 走的是 `SetPhase→MarkNeedsPaint` 重录老路 | 原结论写 `ui/rendering/compositing.go:11; packet.go:57`，目录错了（应为 ui/scene），行号与内容对得上 |
| C6 | viewport 滚动保留路径丢平移 | **成立** | `ui/rendering/layer_build.go:124-139` RenderViewport 分支只 PushBoundary(off)+PushClipRect(0,0,w,h)，子树按各自 `n.Offset()` 入树，全程无 `-scrollX/-scrollY`（该文件 grep "scroll" 仅 ：122 一句注释）；对照直绘路径 `ui/rendering/viewport.go:296-299` 有 `ox/oy = Origin - scroll` 再 `WithOrigin` | 滚动时 `SetScrollOffset→MarkNeedsPaint`（viewport.go:118-127）让 boundary 每帧重录，但重录出的层树照样没有平移 → retained 合成的内容停在旧位置/错位 |
| C7 | Spinner retained 消失：recordLeafContent 只认三叶 | **成立** | `ui/rendering/layer_build.go:263-297` recordLeafContent 仅 switch `RenderColorBox/RenderText/RenderImage`；Spinner 自绘在 `ui/rendering/spinner.go:60-94`（直接 fillRect，不走 RenderBox.OnPaint）；addLeafPicture 的 RasterExtra 兜底只救 `*RenderBox && box.OnPaint != nil`（layer_build.go:248） | Spinner 是 RepaintBoundary（spinner.go:29），retained 树里它是空 Picture 且无兜底 → 合成后不可见。任何非三叶、非 RenderBox.OnPaint 的自绘 RO 在 retained 下都有同样的洞，Spinner 只是代表 |
| C8 | WGSL uniform 数组步长不合 16 字节规范 | **成立** | `gpu/shader/wgsl/internal/lower/lower.go:818-831` 数组 stride = round_up(elemSize, 自然对齐 elemAlign)，f32 数组 stride=4；注释自己承认「uniform buffer 16-byte array stride requirement is enforced by the WGSL spec … but the general type layout uses natural alignment」 | 结构体成员布局（lower.go:706-735）同样只按自然对齐算 offset，uniform 地址空间无特判 |
| C9 | WGSL MatrixStride 自然对齐而非 uniform 要求的 16；uniformStructTypes 声明后从未写入 | **成立** | `gpu/shader/spirv/internal/codegen/backend.go:1610-1636` MatrixStride = 行数系数×标量宽（vec2×f32=8），不看地址空间；`uniformStructTypes` 三处出现：`:155-156` 声明、`:196` make、`:236` clear，grep `uniformStructTypes[` 写入零命中 | 原行号 1622-1635 落在实际区间内；该 map 注释自称「用于套 std140 MatrixStride 规则」却没接线，正是 C8 该用的钩子 |
| C10 | Transform 子 PaintContext 丢字段 | **成立** | `ui/rendering/transform.go:113-121` 手搓 `&PaintContext{DC, OriginX:0, OriginY:0, Scale, PaintVisits}`，丢掉 CompositeOnly/LayerBudget/LayerStats/saveLayerDepth/BoundaryCache/UseBoundaryCache/DebugRepaint/DebugRepaintDraws；后续 `childPC.WithOrigin(...)` 从这个残缺副本拷贝，字段回不来 | 对照标准做法 `paint_context.go:49-69` WithOrigin 全量继承；后果：transform 子树内 SaveLayer 预算失效、retained 跳过失效（多花）、调试统计断流 |
| D1 | GPU Buffer.Map 忙等 | **成立** | `gpu/rwgpu/map_pending.go:190-204` 兜底 goroutine 是 `for { select { case <-req.done: return; default: dev.Poll(false); runtime.Gosched() } }` 纯自旋；等待方本体 `:206-220` 在 select 上阻塞，并不需要这个 goroutine 转 | map 完成前白烧一个核；应改为带 sleep/条件等待或事件驱动 |
| D2 | 渐变逐像素 pow | **成立** | `render/gradient.go:98-133` interpolateColorLinear 每次插值走 SRGBToLinearColor/LinearToSRGBColor；`render/internal/color/convert.go:8-23` 两个转换各含 `math.Pow`；逐采样入口 `gradient_radial.go:92`、`gradient_sweep.go:94`（colorAtOffset→interpolateColorLinear）→ 每像素最多 6 次 math.Pow | `render/internal/color/benchmark_lut_test.go` 显示查表方案已写好（约快 200 倍）但生产 convert.go 没接，属「优化做了没上线」 |
| D3 | 备选字体反复整读文件无上限 | **成立** | `render/text/fontscan_fallback.go:100-127` resolveFace 按 `(rune,size)` 缓存 miss 即 `NewFontSourceFromFile(path)` 整读文件——同一字体文件被不同 rune/size 反复重读；`:70-71` faceCache/pathCache make 后只增不减、无淘汰上限；`:108-110` 的 `fallbackKey{r, size:-1}`「尺寸无关脸」免疫分支全仓无人写入，恒不命中（死检查） | 缓解因素：pathCache 按 rune 挡掉了 FontMap 解析，但文件 IO 层的重复读与无界增长属实 |
| D5 | 门禁公式估算：sumRatio 硬编码几何常数 | **成立** | `examples/ui_wr_c2_retained_scene/main.go:57-66` sumRatio = 固定常数(100²+3·60²+2·190²+1200×72)/(winW·winH)，注释自认 "estimates"；`:377` 稳态门禁、`:474` 收口门禁均消费该值并写入 JSON（`:433` damage_ratio_sum） | 门禁其余分量（dirty ids≥2、BoundarySkip≥3、present_mode≠full）是实测，只有 sum 这一项吃理论值；若某帧实际脏区偏离假设布局，该项失真 |
| E1 | tmp_vlprobe、render/tmp_ref_stroke 使 go build ./... 失败 | **成立（实测）** | 本轮实跑 `go build ./...`：`tmp_vlprobe/main.go:76` `owner.LayerCache undefined`、`:102` `undefined: rendering.BuildFramePacketCached`；`render/tmp_ref_stroke` 包内 main 重复声明（main.go:14 vs contours.go:12），其 wind 子包三个文件互报 main 重复 | 两个目录都是临时探针遗留；`go build ./...` 当前必红 |
| E2 | shaper.go 注释矛盾 | **成立** | `render/text/shaper.go:7-8` 注释「OwnShaper: Pure Go shaper … (default, ADR-048)」；`:20` 实际 `var defaultShaper = NewHbShaper()`；`:29` SetShaper(nil) 文档又写「reset to the default OwnShaper」，实际恢复成 HbShaper | 两处注释都过时（M1 切 HarfBuzz 时没改），误导 API 使用者 |

## 判定汇总

16 条全部核实为**成立**，无误报、无需降级为「部分成立」的条目。但有以下口径/细节修正需要在后续修复时注意：

### C1/C5：路径与行号修正

- C1 原结论写「render surface.go 约 310-314」，实际文件是 `gpu/webgpu/surface.go`（render 包下没有 surface.go，只有 `render/surface/` 子包，与本案无关）。行为描述完全正确。
- C5 原结论写 `ui/rendering/compositing.go:11; packet.go:57`，实际两个文件都在 `ui/scene/` 下（`ui/rendering/` 目录里不存在 compositing.go 和 packet.go）。行号与内容一致，属目录笔误。

### C4：事实成立，但性质是「分期设计」不是「漏接」

`pipeline_app.go:176` 注释明写「Default remains full_paint until W6 makes retained the global default」，`:888` 也有同义注释。retained 链路本身是通的（`SetPresentPolicy(retained)` → useRetained → compositeOnly 稳态帧），真窗也是显式开的。所以这条的正确表述应是「retained 尚非全局默认（按分期计划属正常）」，而不是「retained 链路死了」。修不修取决于产品节奏，不属于坏链。

### D5：只有 sum 分量是理论值

门禁表达式 `len(ids)>=2 && MaxDirtyLayerIDCount>=2 && present_mode!=full && sumRatio()<=0.35 && BoundarySkip>=3` 里，前两个和最后一个都是运行时实测，唯独 `sumRatio()` 是写死的几何假设（假设脏区永远是那几块固定矩形）。风险在于：一旦示例布局改动而常数没跟着改，门禁要么假绿要么误杀。建议改为从 DamageStats 实测求和或至少把常数与布局常量绑在同一处定义（目前 hotSize/picBox 已复用，但 100²、1200×72 两项仍是裸数字）。

### D2/D9 关联观察：优化件已造好、未上线

D2 的查表转换（benchmark_lut_test.go 所测）和 C9 的 uniformStructTypes 钩子都属于「零件已加工、没装上车」。这两处修复成本低：前者把 convert.go 的 math.Pow 换成已有 LUT，后者要么真正接入 uniform 地址空间判断、要么删掉。
