# 第 3 轮终审报告（ROUND3）

> 终审方式：对 ROUND2 第六节 12 条抽验清单做**源码级复核**（不跑 GPU 窗口；涉及真机/竞态的条目只做静态推演并标「需真机复测」），并对第三节漏审模块做快速扫描。
> 本轮工具调用约 46 次，全部为读码/检索，未改任何引擎代码。

## 一句话直话总结

**12 条抽验 10 条静态坐实、2 条（MSAA UB、FifoRelaxed 时序）静态推演成立但要真机才能钉死，没有一条被推翻；漏审模块快扫没挖出新 P0/P1，整体维持 ROUND2 结论，只把 X03 的竞争面描述收窄、新增 3 条 P2。**

---

## 第一节：12 条抽验结果表

| # | 抽验内容 | 结论 | 判定 | 关键证据（文件:行号） |
|---|---|---|---|---|
| 1 | X03：光栅线程原地写共享层树 | **部分成立**（竞争面比原表述窄，需真机 `-race`） | P0 族维持，描述收窄 | 光栅线程确存在：`ui/raster/loop.go:54-85`（LockOSThread 消费 FrameJob）；packet 在 UI 线程构建后异步提交：`ui/embedder/pipeline_app.go:843`（BuildFramePacket）→ `:997`（SubmitLatest）；光栅侧写共享字段：`ui/scene/rasterize.go:50-53`（`t.NeedsRaster=false; t.Picture.Valid=true` 无锁）、`ui/embedder/pipeline_app.go:1189`（lastBoundaryFrame 虽是 atomic.Value 但 BoundaryCache.entries 本身无锁，`boundary_cache.go` 仅 atomic 用于 ID）。收窄理由：BuildLayerTree **每帧新建层树**，当帧 PictureLayer 多为新对象；真正跨帧共享的是 BoundaryCache 缓存的 Picture 与脏标志，以及同一 pkt 在 UI 线程 `:865` RasterizeDirty 统计、raster 线程再消费的两段访问。**需真机 -race 复测** |
| 2 | ConsumeNeedsPaint 是否在 raster 线程 | **成立** | P0（X02 后半） | `ui/embedder/pipeline_app.go:1199`：`pipe.ConsumeNeedsPaint()` 在 `presentPacketTextured` 的 draw 闭包内 → 该闭包经 `:997` SubmitLatest 进入 `ui/raster/loop.go` 的 raster OS 线程执行；`ui/rendering/pipeline.go:176-193` 遍历整树 `clearPaintDirty`，bool 无原子保护；同时 UI 线程 ticker 回调（`pipeline_app.go:736` sched.Tick → 用户 MarkNeedsPaint）可并发写同一标志 |
| 3 | X05：GPU 主文字路径绕过 shaping | **成立**（代码级坐实，行为级需实测） | P0 维持 | `render/internal/gpu/gpu_text.go:179`：主绘制直接 `for glyph := range face.Glyphs(s)`；`render/text/face.go:160-220` iterGlyphs 就是逐 rune `GlyphIndex`(cmap)+advance 推进，**全程无 GSUB/GPOS/kerning**；`render/text/shape_result_cache.go:15` 注释自认 shapeModeLayout =「cmap + advance; GPU LayoutText path」。连字/kerning/阿拉伯变形在该路径全丢 |
| 4 | X04：布尔运算逐像素近似 + EvenOdd 配对 + 2048 截断 | **成立** | P0 维持 | `render/path_boolean.go:62-100`：双重循环逐像素 `Winding()!=0` 判 inside（即恒按 NonZero 口径，EvenOdd 填充规则的路径布尔结果必错）；`:63-68` maxDim=2048 **静默截断**无告警；clip 侧同病：`render/internal/clip/mask.go:210-224` 交点排序两两配对、忽略绕向 |
| 5 | R7 MSAA 中帧 LoadOpLoad 读已 discard 内存 | **成立（静态推演，需 4x MSAA 真机复测）** | P1 维持 | `render/internal/gpu/render_session.go:5368-5373`：`frameRendered` 时中帧 pass `colorLoadOp=LoadOpLoad`；而 MSAA 路径 `colorAttachment`（`:1002-1019`）渲入 msaaView 且 `StoreOp=StoreOpDiscard`（注释 ADR-021 自认 resolve 后 msaa 内容不要了）→ discard 后下一 pass Load 读未定义内容。逻辑闭环完整，闪块现象需真机 |
| 6 | Wayland FifoRelaxed 命令缓冲释放时序 | **部分成立（作者自标未证实，需真机）** | 待验维持 | `render/internal/gpu/render_session.go:896-925`：BeginFrame 注释明说 surface 模式「present 即屏障」故不 drain，`prevCmdBufs` 直接 FreeCommandBuffer；该假设依赖 wgpu-native present 内部同步语义，源码层面无法证实。维持「未证实的定时炸弹」，需真机压测 |
| 7 | viewport 滚动平移丢失 + spinner retained 消失 | **成立（静态钉死）** | P1 维持 | viewport：`ui/rendering/layer_build.go:126-139` retained 分支只推 boundary+offset+ClipRect，**没有把 scrollX/Y 推成 Transform/Offset 层**；对照 Paint 路径 `viewport.go:296-298` 明确有 `oy = OriginY - scrollY` 平移——两条路径行为分叉坐实。spinner：`layer_build.go:263-298` recordLeafContent 只认 RenderColorBox/RenderText/RenderImage 三种叶子，RenderSpinner 不在 switch 内也无 OnPaint（走不了 :204 的 RasterExtra 兜底）→ retained 路径记录成空 picture，spinner 消失。真窗可一步复现 |
| 8 | acquire 锁黑洞与死锁恢复 | **部分成立（恢复路径存在但有洞，需故障注入）** | P1 维持 | `gpu/webgpu/swapchain.go:971-1000`：acquire 超时 abandon 阻塞 cgo goroutine（channel 带 1 缓冲，迟回值不泄漏内存，但 native 线程可能永久占用）+ acquireHung 锁存；`:1005-1021` acquireHungLocked 每 500ms 经 reconfigureThrottled 解锁；洞：reconfigureThrottled（`:941-967`）自身受 500ms 限流可能连续失败，hung 状态下帧循环持续拿 ErrAcquireTimeout，无升级/放弃策略。需真机故障注入 |
| 9 | CPU 渐变不受 CTM 影响 | **成立** | P1 维持 | 几何：路径顶点在录入时就经 CTM 变换（`render/context.go:1038-1046` MoveTo/LineTo）；着色：软件光栅以**设备像素坐标**直接采样 `paint.ColorAt(x,y)`（`render/software.go:531、:597、:984`），Paint→Brush 链路（`paint.go:188-196`、`painter.go:53`）**无逆 CTM 变换**。旋转/剪切坐标系下几何对、渐变方向错 |
| 10 | scene CPU tile 丢图层 alpha | **成立** | P1 维持 | `render/scene/renderer.go:716-721`：TagPushLayer `_, alpha := dec.PushLayer(); _ = alpha`，TagPopLayer 空分支，TODO 注释自认「per-layer alpha blending 未实现」→ CPU tile 路径半透明图层变全不透明。像素对照未做（代码即证据） |
| 11 | C2 公式门禁（改参数门禁自动变绿） | **成立** | P1 卫生维持 | `examples/ui_wr_c2_retained_scene/main.go:63-67` sumRatio 硬编码场景几何常数（100²+3·60²+2·190²+1200×72）；`:377` 门禁 `sumRatio() <= 0.35` 直接吃这个设计理论值，**不是实测**；`:58-62` 注释坦白口径。改 hotSize/picBox 等场景参数门禁自动跟着变绿，属「降难度凑门禁」温和形态，修法照旧：从 frame 决策处导出实测 rects 面积和 |
| 12 | focal 渐变 StartRadius 未参与计算 | **成立** | P1 维持 | `render/gradient_radial.go:129-192` computeTFocal：射线-圆求交只用 EndRadius（`:149` c = fx²+fy²−EndRadius²），最终 `return pointDist/intersectDist`（`:190`），**StartRadius 全程未出现**（仅 `:84-87` radiusDiff==0 早退用到一次）；对照简单情形 computeTSimple（`:119-125`）有 `(distance−StartRadius)` 偏移。focal 渐变的起始环半径被静默忽略 |

小结：12 条 = 8 条完全成立 + 2 条部分成立（#1 收窄、#8 有洞）+ 2 条成立但需真机终判（#5、#6）。**零推翻**。

---

## 第二节：漏审模块快扫结果表

| 模块 | P0/P1 有无 | 一句话结论 |
|---|---|---|
| render/filters | 无 | 纯 init 注册包（register.go 共 44 行，注册 6 个 CPU 滤镜回调），无顶层符号无状态，无可藏雷之处 |
| render/raster | 无 | 纯 init 注册包（raster.go 共 22 行，注册 AdaptiveFiller 一个 filler），同上 |
| ui/focus | 无 | FocusManager 单 scope 单线程模型（manager.go），Register/Unregister 幂等、blur 先于摘除，逻辑干净；规模小风险低 |
| ui/semantics | 无（有 P2） | 纯数据节点+Flatten 导出（node.go），自身无 bug；但它**没接到任何平台可达性 API**，属「声明未接线」族，记 P2 |
| ui/theme | 无 | tokens+provider 纯数据包，目录级扫描未见问题（未深审） |
| ui/input | 无 | 事件归一化/IME/键码纯映射层，fromplatform 有测试覆盖，未见 P0/P1 |
| gpu/context | 无 | WindowProvider/GestureEvent 等接口+NullWindowProvider 实现，契约清晰文档好，纯数据层无雷 |
| ui/platform（窗口层） | 无新 P0/P1（建议专项） | clipboard 大量 `uintptr(unsafe.Pointer)` 回调模式（wayland_clipboard_linux.go:122 等），核查后 state 对象由 `wlWin.dds` Go 指针长期持有（wayland_linux.go:526-528），生命周期有锚点，非裸悬垂；DRM vsync 阻塞等待隔离在 sync.Once。unsafe 密度高，建议后续专项细审（P2 建议） |
| lib/（wgpu-native 分发） | 无 P0/P1（有 P2×2） | 二进制已 gitignore 不入库 ✓；版本靠 `lib/wgpu-native-meta/wgpu-native-git-tag`（v1.0.1）；加载发现链合理（env 覆盖→./lib→系统搜索，gpu/rwgpu/wgpu.go:222-250）。P2-a：**无 ABI/版本校验**——so 与 webgpu.yml 头文件代次不匹配时静默错位；P2-b：`lib/wgpu-linux-x86_64-release (2).zip`（16MB）散落仓库目录带下载残留后缀，卫生差 |
| third_party | 无 | go-text/typesetting v0.3.4 本地 vendored + go.mod replace，git 正常跟踪源码，标准做法 |

---

## 第三节：对 ROUND2 P0/P1 总表的最终修正

1. **X03 描述收窄，级别不动**：原表述「光栅线程原地写共享层树字段」过宽——每帧层树多为新建对象，真正跨线程共享的是 BoundaryCache 缓存条目/Picture 及脏标志、以及同一 pkt 在 UI 线程统计段与 raster 线程消费段的两段访问。**P0 族（D3 线程纪律系统性缺失）维持**，修复时按收窄后的点位下手即可，工作量比原文表述小。
2. **X05 证据升级**：从「注释级证据」升级为代码级闭环（gpu_text.go:179 → face.go:160-220 全链路无 GSUB/GPOS），P0 维持无疑义。
3. **其余 X01/X02/X04/X06、P1 批 A–E 全部维持原级别**，本轮抽验无一推翻；R7 的 MSAA UB 与 FifoRelaxed 两条维持 P1/待验，最终定级等真机复测。
4. **无新增 P0/P1**；新增 P2 三条：semantics 未接平台可达性、wgpu-native 无 ABI 校验、lib 目录 zip 残留（详见第二节）。
5. M2 口径裁定（取严原则）复核无误，继续执行。

---

## 第四节：全项目最终问题台账更新说明

- **P0：6 项，全部维持**（X01–X06），其中 X03 修复范围收窄、X05 证据补强。X03/X05 的最终实锤仍建议各补一次 `-race` 真窗与连字行为真窗。
- **P1：批 A–E 全部维持**，本轮第 7、9、10、11、12 条抽验均为其补充了更精确的行号证据；#5（MSAA）与 #6（FifoRelaxed）挂「需真机复测」标签，暂不移出台账。
- **P2：新增 3 条**——
  - P2-new1：`ui/semantics` 未接入任何平台可达性出口（声明未接线族）；
  - P2-new2：wgpu-native 加载无版本/ABI 校验（`gpu/rwgpu/wgpu.go:222-250` 只找文件不验指纹）；
  - P2-new3：`lib/` 内 16MB 下载残留 zip 与 meta 目录混放（工程卫生）。
- **待办标签**：12 条中 #5、#6、#1(-race)、#3(行为真窗)、#8(故障注入) 五项留真机验证尾巴，其余十条可直接进修复排期。
- 台账载体不变：仍以本文档 + ROUND2 总表为准；后续修复按 AGENTS.md 流程走对应 skill，不在示例层绕引擎洞。

*第 3 轮终审完。*
