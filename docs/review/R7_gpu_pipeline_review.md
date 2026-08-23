# R7 · render/ GPU 提交管线审查报告

> 审查范围：`render/internal/gpu/render_session.go`、`gpu_render_context.go`、`render/pipeline_mode.go`、`render/sample_count.go`、`render/options.go`、damage 链路（context.go / present_target.go / ggcanvas）、`gpu/webgpu/`、`gpu/rwgpu/`、`gpu/shader/`。
> 对照标准：`docs/ENGINE_FRAME_PRESENT_STANDARD.md`、`docs/ENGINE_GPU_RESOURCE_LIFECYCLE.md`、Flutter Impeller / Skia Ganesh。
> 方法：前两批调查记录（`.material_gpu_pipeline.txt`）全量采信，其中 5 条 P0/P1 线索由本轮逐行核实源码确认。

---

## 一句直话结论

**这条 GPU 主管线「默认路径能用、结构不差」，但离工业级还有明显距离：OS 级增量呈现整条链是死的（接口没人实现 + 后端直接丢弃），4x MSAA 一开中帧提交就读未定义内存，「Auto 选管线」是个算了等于没算的摆设，仓库里还躺着编译不过的临时目录。** 默认 1x + 全量重绘的主窗口路径是正确的，但文档里吹的几个高级能力（per-rect OS blit、Auto→Compute）经不起代码对照。

---

## 二、正确性 / 隐藏 Bug

### 2.1 帧提交时序

**【P1】BeginFrame 释放命令缓冲依赖「present 即屏障」的假设，Wayland FifoRelaxed 下不成立**
- 位置：`gpu/webgpu/swapchain.go`（present 路径）、`render/internal/gpu/render_session.go:905-943`（BeginFrame 只在 `lastSubmitUsedSurface=false` 时 drainQueue，surface 提交后靠 present 当屏障直接 FreeCommandBuffer）。
- 问题：块3 改造后 Wayland 恒 FifoRelaxed（present 不阻塞客户端），present 返回 ≠ GPU 执行完。下一帧 BeginFrame 直接释放上一帧命令缓冲时，GPU 可能还在跑。项目自己有过 `vkResetCommandPool` 在飞 UB 的前科（注释里写着），这个假设变弱了却没有补验证。
- 缓解：wgpu-native Submit 内部持有引用，实际是否 UB 取决于 FFI 层 FreeCommandBuffer 的实现——**需要真机压测确认，当前属于「未证实的定时炸弹」**。

**【P2】blit-only 快路径对刚 acquire 的 swapchain 图像做 LoadOpLoad**
- 位置：`render/internal/gpu/render_session.go:5256-5290`：`if s.frameRendered || hasDamage { loadOp = LoadOpLoad }`，而窗口路径每帧 `frameRendered` 都被重置为 false（见 2.3），所以 hasDamage 帧会对内容未定义的图像 Load。被 `fullSurface := !s.frameRendered` 强制全幅合成掩盖住了，结果正确，但这是「靠全量重绘兜底」而不是设计正确。

### 2.2 damage 与残影

**【P1】OS 级增量呈现整条链路是死的——两处断链**
- 断点一：`gpu/webgpu/surface.go:310-314` 的 `PresentWithDamage` 直接 `return s.Present(st)`，damage rects 参数名都改成 `_` 了；`surface_browser.go:177-181` 同样。而上游 `swapchain.go` 还在认真把 rects 传下来。
- 断点二：`render/integration/ggcanvas/canvas.go:631` 定义的 `DamageRectSetter` 接口，全仓库只有测试 mock 实现（`canvas_test.go:1085`）；`canvas.go:361` 的类型断言在生产环境永远失败，rects 被静默丢弃。接口注释声称 "gogpu.ContextRenderTarget implements this via Context.SetDamageRects()"——`render.Context` 根本没有这个方法（grep 全仓库证实），注释是假的。
- 结论：注释和 ADR-021/028 声称的「ggcanvas → SetDamageRects → PresentWithDamage per-rect OS blit」三环链，**两环是空的**。damage 目前的全部实际价值只剩 GPU 内组级 scissor 省一点 overdraw。

**【P2】Stroke/Fill 的 damage 矩形不含线宽外扩——粗线描边必欠估**
- 位置：`render/context.go:1162`（Fill）、`:1175`（Stroke）：都用 `c.path.Bounds()`，Stroke 不加 `lineWidth/2` 外扩。画一条 10px 宽的线，damage 只有中心线的 bbox，边缘 5px 不会被标脏。当前因为 OS 转发是死链所以没暴露，一旦把增量 present 接真就是残影。文本路径 `text.go` 有 ±1px pad 但同样不含特效余量。
- 附带核实结论（前批怀疑、本轮排除）：曲线控制点 AABB 是过估不是欠估（贝塞尔包含在控制点凸包内），安全；`trackDamage` 里 `if !c.deviceMatrix.IsIdentity()` 条件查 matrix 却拿 `deviceScale` 缩放（`context.go:809-818`），两者恰好恒同步所以目前不算 bug，但属于条件与动作不同源的脆弱耦合（P3）。

### 2.3 MSAA

**【P1】sampleCount>1 时中帧 flush 用 LoadOpLoad 读已 Discard 的 MSAA 内存**
- 位置：`render/internal/gpu/render_session.go:1003-1018`——MSAA 时渲染进 `msaaView` 且 `StoreOp: StoreOpDiscard`；`:5370-5373`——同一帧第二次 flush（`frameRendered=true`）时 `colorLoadOp = LoadOpLoad` 加载的就是被 discard 掉的 MSAA 内容 = 未定义像素（WebGPU/Vulkan 规范明确）。
- 触发条件：默认引擎 1x（`render/sample_count.go:11-14`，直绘 targetView 无此问题），但 `SetMSAASampleCount(MSAASampleCount4)` 一开，任何含 CPU fallback 中途 flush 的多 pass 帧就踩雷。讽刺的是 ADR-021 注释自己承认 "MSAA cannot LoadOpLoad after resolve"，却只挡了 damage 场景没挡 frameRendered 场景。

**【P2】每帧新建 TextureView 使「跨帧局部保留」永远走不到**
- 位置：`gpu/webgpu/swapchain.go:915` 每帧 `st.CreateView(nil)` 新建 view 对象 → `render_session.go:4814-4815 / 5256-5257 / 5362-5363` 的 `view != s.lastView` 恒真 → `frameRendered` 每帧重置 → `fullSurface` 恒 true。
- 结果正确（swapchain 图像内容本来就不保证保留，Clear 才是对的），但意味着 blit-only 的 `hasDamage && !fullSurface` 局部保留分支在窗口路径是死分支，相关带宽节省声称打折。

### 2.4 shader ↔ CPU uniform 一致性（本轮好消息）

逐字节核对全部一致，Y 方向约定全链统一：

| Uniform | Go 侧 | WGSL 侧 | 结论 |
|---|---|---|---|
| SDF 渲染 | `sdf_render.go:884-905` 16B | `sdf_render.wgsl:16-20` vec2+u32+pad=16B | ✓ |
| Image | `image_pipeline.go:763-781` mat4(64)+vec4(16)=80B | 同构 | ✓ |
| GlyphMask | `glyph_mask_pipeline.go:846+` 80B | 同构 | ✓ |
| Text/MSDF | `text_pipeline.go:642+` 64+16+16=96B | `msdf_text.wgsl:15-27` | ✓ |
| ClipParams | `clip_params.go:15` 32B | `cover_aa.wgsl:25-30` 对齐后 32B | ✓ |

Y 方向：SDF shader `ndc_y = 1 - y/h*2`、ortho `-2/h` 平移 `F=1`、glyph 正交 `E=-2/vh`，全部 Y-down→NDC 一致；读回 BGRA→RGBA 交换处理正确。这一块没有雷。

小问题：`textured_quad.wgsl` 的 `rrect_clip_coverage` 恒返回 1.0——圆角裁剪对图片/文字层静默无效，接口注释声称覆盖 "~95% 非矩形裁剪" 名不副实（P2 诚实性）。

---

## 三、批处理质量（对比 Flutter Impeller）

**做得好的**：单 render pass 多 tier（SDF→convex→stencil→image→text→glyph）、组级 scissor、convex 按 blend mode 分 range 合批、indexed mesh、sticky uniform slab、共享 encoder（ADR-017）、tier 变化自动切分 scissor 组保住画家算法 z 序。整体思路接近 Skia Ganesh stencil-cover，结构上不算落后。

**与 Impeller 的差距**：
1. **【P2】clip 主要靠 stencil-then-cover（每 path 两次 draw）**；Impeller 用 depth buffer 做 clip/fill 免 stencil pass。这里有 GPU-CLIP-003a 深度裁剪管线，但 `needDepth` 只在有 ClipPath 的组才开，没有全面切换。
2. **【P2】高级混合走 dual-texture 中帧读回**（Submit + PollWait + Map 卡在帧中间），Impeller 用 framebuffer fetch 一遍出。每次高级混合都是一次流水线停顿。
3. **【P1】「Auto 选管线」是死逻辑**：`render/pipeline_mode.go:60/65` 的 SelectPipeline 判据用到 `ClipDepth`/`OverlapFactor`，但生产代码**从不填充这两个字段**（grep 全仓库零命中，只有 ShapeCount/PathCount/TextCount 在累加）。更糟的是 Auto 模式下绘制路由只认显式 `rc.pipelineMode == PipelineModeCompute`（`gpu_render_context.go:1461`），`effectivePipelineMode` 的选择结果只在 `flushVello`（`:2776-2777`）里用于冲刷一个在 Auto 下永远是空的 pending 队列——**启发式算出来的答案不会改变任何渲染行为**。且 `effectivePipelineMode` 每次触发 `CanCompute()` 会惰性初始化整套 Vello compute 管线，纯浪费。
4. 无遮挡剔除；跨帧资源复用有 fingerprint 缓解但不彻底。

---

## 四、性能

- **【P2】Map 忙等烧核**：`gpu/rwgpu/map_pending.go:191-202` 起独立 goroutine 死循环 `dev.Poll(false)+runtime.Gosched()` 直到 map 完成——每次读回期间空转一个核。应换成事件/信号量或至少加 sleep 退避。
- **【P2】offscreenPool 复用禁用**：多层/resize 场景每帧新分配纹理，VRAM churn（代码 NOTE 已登记原因 usage conflict，属已知成本）。
- **做得对的**：surface 路径 BeginFrame 不 WaitIdle（靠 present/vsync 屏障），offscreen 才 drain；vertex buffer 有 retire 复用；WriteBuffer 有 scratch slab。基本盘不差，坏在上述同步点上。

---

## 五、文档诚实性

| 声称 | 实际 | 判定 |
|---|---|---|
| ADR-021/028 + `render/context.go:698`、`ggcanvas/canvas.go:316` 注释：「per-rect OS blit（VK_KHR_incremental_present / Wayland damage_buffer）」 | `PresentWithDamage` 丢参数（surface.go:312）+ `SetDamageRects` 生产无实现者（canvas.go:631 仅测试 mock） | **【P1】名不副实** |
| ENGINE_GPU_RESOURCE_LIFECYCLE.md P1-P6 全 ✅ | P4/P6 结构存在但生产未接线：`res.Cache` 无生产构造点、`res.Submission.Track` 零调用——SubmitDone 清的是永远空的队列，真正保命的是 prevCmdBufs + vsync 屏障（文档自己也承认后者） | **【P2】半诚实**：机制存在≠生效 |
| ENGINE_FRAME_PRESENT_STANDARD.md | 块1/2/3 与代码相符（fixedNoVsync 等），X11 XPresent cookie 解析问题在 §8 如实登记待真机 | **✅ 诚实** |
| `FlushGPUWithViewDamage` 文档说 MSAA 忽略 damageRect 并警告 | `RenderFrameGrouped` 只是 log 一条 Debug，damage rects 照样以 scissor 形式参与 MSAA pass | **【P2】言行不一** |
| catalog：图片/文字 rrect 裁剪覆盖 "~95%" | `textured_quad.wgsl`/glyph/text shader 的 rrect_clip_coverage 恒 1.0 | **【P2】虚标** |
| 「Auto selects best pipeline」（pipeline_mode.go 注释） | 见 §三-3，选择结果不影响任何路由 | **【P1】虚设功能** |

---

## 六、可读性与冗余

- **【P2】巨石文件**：`render_session.go` 约 5600 行（`encodeSubmitSurfaceGrouped` 单函数 200+ 行）、`gpu_render_context.go` Flush 单函数约 440 行。submit/readback/grouped/blit 四套编码路径大量复制粘贴式重复，改一处漏三处的温床。
- **【P3】注释错位**：`render/accelerator.go:361-405`——AcceleratorCanRenderDirect 的 doc 注释离函数隔了两层，AbandonAcceleratorDevice 的注释悬空挂在 Purge 函数前面、函数本体反而裸奔。godoc 显示错乱。
- **【P2】卫生**：`render/tmp*`、`render/tmp_ref_stroke` 含重名 main，`go build ./...` / `go vet ./render/...` 直接红。临时目录不该躺在主干。
- **【P3】命名混淆**：`render.SDFAccelerator`（CPU）与 internal gpu 包同名类型并存，新人极易看混。

---

## 七、「离工业级还差多少」

直话：**主链路（1x MSAA + 全量重绘 + 组级合批）已经达到可发布水准，但「工业级」的三块硬骨头还没啃**：

1. **增量呈现是真需求也是真缺口**——现在 damage 从 OS 转发到 GPU 保留到 present 全链断光，等于只有「全量重绘 + 组内省 overdraw」一种模式。Impeller/Skia 的 partial repaint 是标配。差一个完整子系统的量。
2. **非默认配置不可信**——4x MSAA 中帧 UB、Auto→Compute 死路由、rrect 图片裁剪静默失效：凡是不在主验证路径上的开关，开了就可能有惊喜。工业级的定义恰恰是「所有公开开关都可信」。
3. **工程卫生**——编译不过的 tmp 目录、假注释、死字段、5600 行文件，这些不影响跑通 demo，但决定十个人维护三年后的成本。

量化地说：正确性和批处理架构大约到了 Impeller 早期（2019-2020）水平；damage/present 和管线选择的完成度还在「演示级」。补齐 TOP5 之后才有资格谈对齐。

---

## 八、评分表（各维度满分 10 分）

| 维度 | 得分 | 一句话理由 |
|---|---|---|
| 正确性（默认路径） | **7** | 1x 主路径稳，uniform/Y 方向全对；扣分在中帧时序假设未验证、blit LoadOpLoad 靠兜底 |
| 正确性（全开关） | **4.5** | 4x MSAA 中帧 UB、rrect 图片裁剪失效、damage 欠估，非默认即雷 |
| 批处理质量 | **7** | 单 pass 多 tier + 合批结构对齐 Ganesh/Impeller 思路；stencil 全开、无遮挡剔除 |
| 性能 | **6** | present 屏障免等待、scratch 复用是亮点；Map 忙等、中帧读回、offscreen 池禁用拖后腿 |
| 架构收敛性 | **6** | tier 抽象清晰；四套编码路径重复、Auto 选择虚设、res.Cache/Track 未接线 |
| 文档诚实性 | **5.5** | FRAME_PRESENT_STANDARD 本体诚实；ADR-021/028、catalog 裁剪覆盖、Lifecycle P4/P6 存在明显虚标 |
| 可读性 / 可维护性 | **4.5** | 5600 行巨石 + 440 行单函数 + 注释错位 + 编译不过的 tmp 目录 |

**综合：约 5.8 / 10。**

## 九、TOP5 必修清单（按优先级）

1. **【P1】接通或删掉 OS 增量呈现链**：给 `DamageRectSetter` 一个真实实现或在宿主层落地 `SetDamageRects`；同时修正 `render/context.go:698`、`ggcanvas/canvas.go:316` 的虚假注释。做不通就在文档明写「wgpu-native 不支持，仅 GPU 内 scissor 生效」，不许留假链。
2. **【P1】堵 MSAA 中帧 LoadOpLoad**：`render_session.go:5370-5373`（及 4823-4826 同型逻辑）在 `sampleCount>1` 时禁止 LoadOpLoad——要么 resolve 后 blit 回拷，要么强制该帧全量 Clear+重画。
3. **【P1】验证 Wayland FifoRelaxed 下的命令缓冲释放时序**：真机压测 BeginFrame FreeCommandBuffer 是否早于 GPU 完成；不稳就把 surface 提交也纳入 prevCmdBufs fence 化。
4. **【P2】修 damage 欠估**：`render/context.go:1175` Stroke 补 `lineWidth/2 + join/cap` 外扩；顺带把 `ClipDepth/OverlapFactor` 要么填充要么从 `pipeline_mode.go` 判据里删掉，别留死输入。
5. **【P2】工程卫生**：删除/移出 `render/tmp*` 让 `go build ./...` 变绿；拆分 `render_session.go` 四套编码路径的公共部分；修 accelerator.go 注释错位。

---
*审查方法说明：本文基于前两轮调查记录全量采信 + 本轮对 5 条 P0/P1 线索（PresentWithDamage 空壳/SetDamageRects 无实现者、MSAA StoreOpDiscard×中帧 LoadOpLoad、ClipDepth/OverlapFactor 死字段与 Auto 死路由、每帧新建 view 重置 frameRendered、Stroke damage 无线宽外扩）的逐行源码复核。所有行号均为当前工作区实测。*
