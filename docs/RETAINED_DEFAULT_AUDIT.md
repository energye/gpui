# 默认省帧模式全框架审计记录（2026-09-07）

背景一句话：W6 把窗口默认切成省帧模式（稳态只画脏区、不动复用上一帧）后，只修了启动空白一个洞；
本轮把全框架和省帧沾边的功能块全部翻一遍，只找问题、不修代码。

方法：3 个功能块（A 管线切换 / B 纹理复用与 damage / C 文本与输入框）× 3 条不同路子
（P1 源码精读 / P2 调用链追踪 / P3 测试与历史证据），共 9 个 agent 并行、互不通气；
我对着源码逐条亲验并裁决分歧；真窗一次只开一个、跑完即关。
结论分三档：**100%确认**（我亲验源码）/ **证据级确认**（测试名、提交号可回查）/ **观察项**（单路存疑，不当 bug，只记录）。

## 一、100%确认（我亲验源码，文件行号为准）

### A 管线切换

- A1 overlay 孩子变脏在稳态被丢掉【已修，2026-09-07】：`e.NeedsPaint` 只在 `Insert` 置位
 （`ui/overlay/state.go:63`），`BuildOverlayBand` 被它门控（`:226`），构建完还清零（`:247`）；
  孩子自己的 `MarkNeedsPaint` 只在 overlay 子树内冒泡、进不了主树脏账。
  已挂好的浮层（如开着的下拉框）内容再变，稳态帧不重录它。运行时复现需定制场景
  （只动孩子、不起新 Insert），本次未复现。分歧裁决：B-P1 的“合并动作没错”与本条不矛盾，
  合并的是门控之后吐出来的东西。
  修法：`BuildOverlayBand` 脏收集改 `e.NeedsPaint || len(b.DirtyBoundaryIDs) > 0`，
  新增 `rendering.ClearPaintDirty` 子树清脏 helper 与 `State.ConsumeChildPaint`，
  raster 侧与主树同一序列化点消费（扛 SubmitLatest 合并）；单测
  `TestOverlay_ChildDirtFollowedAfterConsume` 红绿双验（旧门控必红）。
  overlay 全包、embedder 全包绿；R8 15 秒真窗 exit 0、accept 4 秒 exit 0 回归。
  程序化改孩子仍需调用方 ScheduleFrame（与 Generation 注释惯例一致）。
  【已修，2026-09-07】
- A2 SnapshotAsync 在遮挡/最小化时必 500ms 超时且采不到图【已修，2026-09-07】：队列要等 raster 任务尾的
  `drainSnapshots`（`ui/embedder/pipeline_app.go:1146`），而 occluded 分支直接
  `ClearPending + continue`（`:886`），任务投不进去，`done` 没人关（`:204-232`）。
  修法：遮挡时直接打日志返回（不排队不等 500ms）；超时摘除队列项并记 dead，
  drain 跳过已摘除（队列改指针类型，过 vet）；单测锁快返与摘除。
  embedder 全包绿。【已修，2026-09-07】
- A3 超时后过期采样闭包不清【已修，2026-09-07，见 A2 修法注记】：超时直接 return（`:229`），摘除只在 drain（`:240-241`），
  恢复可见后下一帧会执行一次对不上当前帧的采样。
- A4 presents 按提交计数虚高：`SubmitLatest` 后就 `Add(1)`（`:1162`），不管任务是否被
  pending 槽合并替换（`ui/raster/loop.go:142-150`）；`MaxFrames` 窗在高负载下可能提前退出。
- A5 DPR 变化（逻辑尺寸不变）只清 CPU 的 Picture 缓存、不清 GPU 图层纹理：
  事件分支只 `BoundaryCache.Clear()`（`:793-798`），帧边界只调
  `pictureTex.Resize(w,h)`不清条目（`:944-946`）；节点尺寸没变则全干净，
  首个稳态帧 `dirty||NeedsRaster||!Has` 全不满足，直接贴旧 DPR 烤的纹理。
  与 B-P2 第 1 条是两路独立报同一代码事实。像素后果待跨屏真窗。
  修法：事件分支改走 `InvalidateBoundaryCache()`（双清加根标脏加 3 帧全刷），
  与程序化路径对齐；单测 `TestInvalidateBoundaryCache_DPRSymmetric` 锁计数、
  欠帧与清缓存。embedder 全包绿。【已修，2026-09-07】
- A6 程序化 `InvalidateBoundaryCache` 是双清加根标脏加 3 帧全刷（`:1472-1487`），
  与上面的事件路径不对称，事件路径少了纹理清理。

### B 纹理复用与 damage

- B1 静态旋转/缩放层、无 CacheKey 层每帧必画必报损伤、稳态进不了 Idle【2026-09-08 撤销，不修】：
  初版修法（干净层跳过重放与损伤）上线后 accept 真窗直播黑屏（xwd 均值 1.26、91% 纯黑，
  快照全量重画路却正常——JSON 全绿、屏全黑）。
  根因（`render/context.go FlushGPUWithViewDamage` 注释原文）：损伤保留只在纯贴图帧生效，
  一帧里只要有矢量图形就走 MSAA 全清重画。本引擎稳态帧必须每帧重现全部可见内容
  （靠便宜贴图），跳过干净层等于每帧清屏。回退 gate 后直播恢复（accept 19.63、R4 43.63，
  与修前基线一致）。教训：① 省帧 damage 在此引擎只是 GPU-work 提示，不是像素持久机制；
  ② JSON 门禁绿不等于屏上对，真窗验证必须抓直播像素（xwd 亮度门），快照路掩盖直播问题；
  ③ 单测 `TestCompositeTextured_SteadyFrameRepresentsAll` 改锁“干净帧仍重现”。
  `Place()` 改动标脏保留（边界缓存路仍需要，与本撤销无关）。
  `CompositeFramePacketTextured` 第二阶段 Picture 分支无脏检查（`ui/scene/textured.go`），
  noTex 分支无条件追加四角损伤。正确性没问题，是多报加多 present。
  初版修法（干净层跳过）已撤销，见本条头部——在此引擎 present 模型下“多报”是正确性的
  一部分，不是可优化项。B1 作为性能项关闭，不再排期。
- B2 纯位移（内容不脏）零损伤【2026-09-08 注记：随 Place 标脏关闭】：第二阶段只给 `RecordedThisFrame` 的层报损伤，
  blit 本身不记损伤。机制确认；现实里滚动动画都顺带标脏所以没爆，
  “只改位移不标脏”的生产者是否存在待确认。
  注记：未标脏的位移唯一已知口是 `Place()`（B7），已修标脏；位移必带脏成立后，
  本条无独立触发面，关闭。另 B1 撤销已证明：稳态重现全部图片是正确性要求，
  idle 只发生在真正零绘制帧。
- B3 `transformBounds` 右下边向零取整、最多差 1px【已修，2026-09-08】：`image.Rect(int(minX),…)`（`:1312`），
  同文件 `measureTextBounds`（`:651-659`）与 `trackDamage` 用的都是 Floor/Ceil 保守取整。
  1px 残留待真窗抓帧。
  修法：改 Floor(min)/Ceil(max)，与同文件两处口径一致；内部单测小数位正负双向断言，
  修前必红。ui/scene 全包绿。
  【修法注记，2026-09-08】
- B4 量不到 bounds 的文本脏层重录了但零上报：`recordWith` 记 `e.bounds = pic.Bounds`，
  文本-only 层该值恒空，上报门 `!b.Empty()`（`:1022-1028`）直接跳过。损伤覆盖率全押在
  `measureTextBounds` 的估计质量上。机制确认；估计少算待验证。
- B5 回退路径丢 `RasterExtra`：叶子条件含 `RasterExtra!=nil`（`:1005`），回退只
  `Picture.Replay`（`:1017-1020`），OnPaint-only 层（Picture 为空、见
  `ui/rendering/layer_build.go:393-399`）缓存未命中时画空白。代码事实确认；
  可达性（base 是否恒成立）待确认。是单帧滞留、非永久黑洞（下帧 `!Has` 重试）。
  修法：回退分支补 `RasterExtra`（Push/Pop 包裹，与 record 顺序一致）；
  单测 `TestCompositeTextured_FallbackRunsRasterExtra` 空 Picture 加回调，回退必执行，修前必红。
  【已修，2026-09-07】
- B6 重录失败也清 `NeedsRaster`【已修，2026-09-08】（`:976` 在 ok 块之外），脏标记被提前消费，
  重试全靠 `!Has` 旁路。机制确认；单帧 OOM 不错帧，靠旁路与 Idle 兜底。
  修法：只在成功时消费标记；条目被删（`!Has`）则保留，下帧经 phase 1 重试，
  条目保留（旧纹理自洽）则消费，避开持续 OOM 下每帧失败重录；
  单测 `TestFailedRecordKeepsNeedsRaster` 失败留标记，修前必红。ui/scene 全包绿。
  【修法注记，2026-09-08】
- B7 `Place()` 改孩子位置不标脏（`ui/rendering/absolute.go:28-43`，只 `SetOffset`），
  对照 `MoveTo` 会标脏。标脏缺失确认；亚像素穿透指纹待验证。
  【已修，2026-09-07：`Place` 改动偏移才标脏，随 B1 门控一并合入，防裸位移冻住】

### C 文本与输入框

- C1 【误报纠正，2026-09-07 复核】普通字符键入链是通的，不修：
  原结论追的是通知钩子（盒子 `OnText` 空实现），走错了分支。真实链是
  物理键 `KindKey(Rune)` → `InputRouter.routeKey`（`ui/embedder/input_router.go:580-596`
  可打印键直插）→ `Editor.AddText` → `changed()` → 盒子 `OnChange → sync()`
  （`ui/textinput/input_box.go:288`、`:1576`）；单测
  `TestRouterPlainKeyTypesIntoEditor`（`ui/embedder/input_router_test.go:210`）
  锁死“a/b 键入得 ab”、控制键不插入。空 `OnText` 方法与示例空回调只是通知钩子，无害。
  教训已记：单路追踪结论必须经第二条路子复核才能定锤。
- C2 单行/多行拖拽滚只写偏移不置脏：`SetOffset`/`SetViewportHint` 明确不标脏，
  单行拖拽分支（`input_box.go:789-809`）、多行四处（`:2080-2104`）无兜底脏，
  只靠尾部 `SetSelection` 间接触发，选区不变就全丢。路径存在确认；触发待真窗。
  修法：单行两处、多行四处边缘滚动点补 `txt.MarkNeedsPaint + sched`，
  视口盒拖拽两处与双 auto-scroll 补 `sched`（脏自带）；单测
  `TestSingleLine_EdgeDragRequestsFrameWhenSelectionStatic` 两次同事件点按、
  第二次选区钳制静态不断言帧与脏，修前脏断言必红。textinput 全包绿。
  【已修，2026-09-07】
- C3 视口盒拖拽置了脏但没要帧：`Viewport.SetScrollOffset` 自带脏，
  但 `viewport_input.go` 全文件 `sched()` 只在 sync（`:467-469`）出现一次，
  拖拽分支与 `doViewportAutoScroll`（`:910-921`）都没调。Idle 下画面不更新确认；
  在 accept 窗内被常帧掩盖（见 C6）。
  修法：视口盒拖拽两处与 `doViewportAutoScroll` 补 `sched`。随 C2 同批验证。
  【已修，2026-09-07】
- C4 光标闪烁翻转只脏不要帧：`SetCaretOn`/`TickCaret` 只有 `MarkNeedsPaint`
  （`input_box.go:173-198`、`:1481-1505`），无 `sched`；且示例 ticker 只翻 `boxA`、
  从不调任何盒子的 `TickCaret`（`main.go:390-403`）。确认。
  修法：三类盒子 `SetCaretOn` 与 `TickCaret` 翻转分支补 `sched`；
  示例 ticker 改逐盒 `TickCaret(dt)`（A 盒周期与原来手动 30 tick 相当，B–F 顺带驱动）；
  单测 `TestCaretFlipRequestsFrame` 三盒翻转帧数断言，修前 0 帧必红。
  accept 4 秒真窗 exit 0、快照六框与预填文字如前。textinput 全包绿。
  【已修，2026-09-07】
- C5 多行从不设纵向裁剪 hint【已修，2026-09-08】：`MultiLineInputBox.sync`（`:1684-1809`）只有 `SetOffset`；
  `SetViewportRect` 全仓库零生产调用（只剩单测在调）。E 区 1e5 字纵滚每帧全量提交。
  费帧方向，无旧帧风险。确认。
  修法：sync 在 SetOffset 后补 `SetViewportRect(scrollX, visW, scrollY, visH)`（纯 hint，
  不标脏不碰坐标）；单测 `TestMultiLine_ViewportRectCullsBands` 百行滚到底，可见带远小于全量，
  修前带等于全量必红。textinput 全包绿。
  【修法注记，2026-09-08】
- C6 示例强制常帧、省帧结论在该窗内验不出来：`main.go:402` 写死
  `ModePersistent`，ticker 每 tick 无条件 `ScheduleFrame()`。C2/C3/C4 类缺帧问题
  在此窗内被掩盖。方法论结论，确认。
- C7 F 区运行中切 wrap/改宽且文本不变时版式不更新【已修，2026-09-07】：sync 直接写 `txt.MaxWidth`
  （`input_box.go:1692/1697`），绕过 `SetMaxWidth`（清缓存加双脏，`text.go:270-278`）；
  文本没变时 `SetText` 早返不标脏（`text.go:111-114`），旧版式继续用。路径确认。
  修法：sync 改走 `txt.SetMaxWidth`（同值 no-op），切 wrap 经 `SetWrap→sync` 即清缓存标脏；
  单测 `TestMultiLine_WrapToggleRelayoutsSameText` 开关回切行数对照，修前 1v1 必红。
  随本批 textinput 全包绿。现象级真窗（F 区有字开关回绕）待补。
  【修法注记，2026-09-07】
- C8 留存快照绕过视口裁剪：`recordRenderText` 逐行全量 `DrawString`
  （`ui/rendering/text_picture.go:11-42`），无任何 hint/cull 引用；
  活路 `Paint` 却走横裁加纵裁。B/C 长行每次重录按全量提交。像素应对得上
  （同源），是花冤枉钱。确认。
- C9 R4b/C2 真窗零像素断言：两窗 main 无任何 pixel/Golden 引用（grep 为零），
  中央静区与直绘回放一致只靠计数器推论。证据缺口，确认。

## 二、证据级确认（测试名、提交号可回查，非源码行为断言）

- B10 活集保护（`SetLiveKeys`）无单测直压；容量三力叠加（自动扩容×显式预算×LRU）未合测；
  Resize 重建、DPR 真链路、滤镜与复用交互、旋转脏尺寸、UI 层多脏拼 multi 均无 CPU 集成测试。
  详见 B 历史路 G1–G7。
- B11 C9 只断言 3 个终帧像素点；R4b/C2 见 C9。详见 B 历史路 G10–G12。
- C10 50000 字 DisplayLines 一致、真脸加省略号一致、横裁剪 5000/50000 量级、回绕段 damage、
  真脸行带提交量、C 框击键硬门禁、F 段首插入、删除回移、边缘点击、preedit 矩形、
  paint/scroll 探针、像素墨迹全框对照、E 框预填稳态、稳态多帧 Golden、ime 窗 Golden 基线，
  全部缺失或只记账。详见 C 历史路 G1–G7。
- C11 `maxScroll` 算法在 `input_box.go` 与 `viewport_input.go` 各写一遍（`ee60d73`），
  残留重复实现。确认。
- A7 无 warm-up 冷启动、遮挡显隐、vsync 切换过程、resize 风暴 embedder 层、idle 交错序列、
  反向与高频策略切换、C11 头注释 3/3 与实际 2/2 口径不一致、C4/C5 新默认下基线过期，
  全部无测试或证据矛盾。详见 A 历史路 G1–G8、H6。
- A8 历史洞 H1–H5（idle 毒化、首帧黑窗、纹理池冲突、SaveLayer 预算失效、纹理四连洞）
  修法与残留对照见 A 历史路；同类写法复发风险未见全仓审计。

## 三、观察项（单路存疑，不当 bug，只记录）

- O1 X11 上 swapchainPending 清除与真 Configure 脱节、dc 与交换链错配一帧（A-P1 R6）。
- O2 运行时 `SetOverlay` 不排帧，静态窗上浮层迟到（A-P1 R7）。
  【已修，2026-09-08：`SetOverlay` 补 `ScheduleFrame`，静态窗运行时挂浮层下一帧即上屏】
- O3 UI 超前 1 帧时 `SetLiveKeys` 误逐正在画的帧（B-P1 T8）。
- O4 同一 CacheKey 主带浮层带互盖（B-P1 T9），键唯一靠约定，建议加断言。
- O5 CPU 回退 op 在离屏重录里错位裁剪（B-P1 T10），需 clip 加 CPU 内容才成立。
- O6 跨屏 DPR 无 Resize 事件则失效全不触发（B-P2 第 2 条），取决于平台行为。
- O7 颜色/文本指纹精度（B-P2 第 4/5 条），正常 setter 链路安全，只影响直接写字段窄路。
- O8 `RenderText`/`RenderImage` 独立边界无缓存接线、省帧没省（B-P2 第 6 条）；ColorBox 重存计数噪声（第 7 条）。
- O9 字形图集未进 OOM 熔断链（B-P2 第 8 条）。
- O10 clip 加旋转缩放双重组合回退未钉测试（B 历史 G9）。
- O11 省略号批量丢“…”、兜底叠行首（C-P1 F4），开关级风险，accept 默认走不到。
- O12 B/C 不用击键区间通道、每击键全串 diff（C-P1 F5），费时方向。
- O13 sync 无条件要帧、OnPaint 自脏的持续出帧嫌疑（C-P2 第 5/6 条），费帧方向，需静置真窗。
- O14 compositor-only 脏（Mutations）有生产无消费（A-P2 第 4 条），预留还是断链待定；
  UI 侧 `RasterizeDirty` 统计调用提前改共享层状态（A-P2 第 3 条），现状无错用。
- O15 需要写帧竞态命中才现形的两条（A-P2 第 2 条吞脏窗口、A-P1 R3/R4 的压出条件），
  归压测，不归日常真窗。

## 四、已排除（看过源码，确认没问题）

- 启动双预算是对的（新建欠 3、warm-up 后对齐到 2，idle 在预算期禁用）。
- PresentWithAuto/Full 选择、force 消费、双 idle 配对、Resize 合并跳过、遮挡期 pending 保留、
  FrameSync 与 idle 错峰、双向策略切换自愈，均无误（A-P1 E1–E11）。
- warm-up 吞脏无害（内容已落屏加首 retained 帧全量重录）、ticker 不稀释脏、
  SubmitLatest 合并是重复非丢失（A-P2 已排除三段）。
- Overlay 合并动作、滤镜损伤、clip 下 GPU 双裁剪、Resize 旧纹理、stamp 时钟、同帧双写槽、
  EndFrame 竞态，均不成立或已修（B-P1 已排除八段）。
- 视口滚动强制重画、干净文本跳过、行索引校验、快照共享、指纹第二道门、富文本分支、
  分区裁剪恒等、MaxLines 下标，对得上（C-P1 E1–E8）；去重早返、视口配对更新、
  换行删除密码占位获焦链正常（C-P2 已排除八段）。

## 五、真窗实测记录（一次一窗，跑完即关）

- accept 窗 4 秒：exit 0，`paint_count=4`，`cpu_fallback_ops=0`；
  快照 A–F 六框与 B/C/F 预填文字齐全，与全画时期一致；首 3 帧全画、第 4 帧起稳态。
- R4（8 秒，RUN_SECONDS=8）：exit 0，retained，`present_mode=damage_union`，
  `damage_multi_frames=79`，fps 58.3。稳态复用正常。
- R8（8 秒，RUN_SECONDS=8）：exit 1，`FAIL: overlay still open at exit`。
  这是 8 秒截断的人工假象（该窗长跑相位里 10 秒处才进恢复段），不是产品 bug；
  overlay 脏专项需定制场景另测（只动孩子、不起 Insert）。
- 键入链（C1）运行时无条件：本机无键注入工具，只做源码级确认。

## 六、下一步（修的顺序建议，等确认再动手）

1. A1 浮层脏丢失（丢画面）、A5 DPR 纹理不清（糊屏），两条先修。（C1 经复核为误报，不修。）
   【两条均已修完并验证，2026-09-07，见各条目修法注记】
2. C7 回绕切换、C2/C3 拖滚脏帧、C4 闪烁帧，输入框第二批。
   【已修完并验证，2026-09-07：三单测修前皆红修后绿，textinput/overlay/embedder 全包绿，
   accept 4 秒真窗 exit 0、快照六框与预填文字如前】
3. B1/B5 纹理损伤类、A2/A3 快照超时类，第三批。
   【2026-09-07 修完，2026-09-08 订正：B1 撤销（见条目，present 模型不允许跳过），
   B5/A2/A3 保留；四单测修后绿，scene/rendering/overlay/embedder 全包绿，
   R6/C5 动画窗像素与修前逐位相同，accept/R4 直播抓屏恢复内容】
4. 其余费帧费内存方向（C5/C8/B7 等）与观察项，按性能专项排。
   【2026-09-08 第四批收工：B3 取整、B6 语义、O2 排帧、C5 纵裁、C11 注释口径、双脏集成单测
   已修已验（单测修前皆红，scene/rendering/embedder 全包绿，race 干净，R4/accept 真窗双绿）。
   主动 deferred（要动大设计、硬上等于猜）：C8 留存快照全量（裁剪归属要设计）、B4 估计质量
   （度量衡要先行）、B7 上限回落（阈值要实测）、O14 Mutations 死链（合成器直通要设计）、
   O13 持续出帧（要静态窗实测）、像素断言与 Golden 基线缺口（R4b/C2/C4/C5，要逐窗立项）。】

## 七、本轮新增：黑屏回归与 present 模型订正（2026-09-08）

- B1 初版（干净层跳过重放）导致 accept 真窗直播黑屏，已全数回退，见 B1 条目。
- `TestKeystrokeRatio_M5` 在本机（GoLand 36% CPU、load 4.6）抖动红，干净树同红，
  与本次改动无关，记为环境抖动，不挡合入（安静机器单跑绿）。

## 八、accept 窗三问题根治（2026-09-08，闪烁/框缺失/内容缺失）

- 现象（直播抓屏实证，快照全画正常是分水岭）：正常帧缺全部六框边框、缺 B/C
  文字；点击后偶发整片黑闪（标题/侧栏/底/A 行/F 标签/状态条同帧消失）。
- H-A 框边框进包（`ui/rendering/layer_build.go` + `box.go`）：包构造器只认具体
  `*RenderBox`，输入框包壳类型（含 `OnPaint` 边框）被漏掉，保留包里就没有边框层，
  稳态合成无可画/无可贴 → 边框只在全画路径（快照）出现。修法：按接口认边框自绘
  （`ownPaintFn`，包壳经内嵌提升天然满足）+ 取不到 `Base` 时退用提升来的
  `EnsureCacheID` 当跨帧纹理键。新单测两个修前红修后绿。
- H-B 横滚视口按可见窗录纹理（`ui/scene/textured.go` + `layer.go` + 构造器视口分支）：
  5000/50000 字单行停在尾部，纹理却从内容头录（裁剪到 1200 宽），合成时视口裁掉
  → 框内恒空。修法：视口滚动平移打标记，录制窗起点跟滚动走（`scrollRecordWindow`，
  无滚动时与旧行为逐位一致，过滚回退头窗口）；`e.bounds` 改记实际录制窗，损伤
  口径同步缩小。新单测修前红修后绿。
- H-C 巨行图片录制裁剪（`ui/rendering/text_picture.go`）：C（50000 字）纹理录出但
  全空（纹理内容 dump 全黑 vs B 有字），整行 `DrawString` 回放提交数万字形，GPU
  画不出。绘制侧早有同门裁剪（`hCullWindow`），录制侧照镜子：巨单行只录可见子串
  （笔位取缝表精确位置，小文本/无 hint 原样整行）。新单测修前红修后绿。
- 真窗终验（串行一次一窗）：静态三连抓稳定（22.3→22.5）；点击 F 中央获焦蓝框、
  键入 Hi 落字并触发重排、光标跟字走；点击后八连抓无黑帧；fps 58、damage_union。
  示例用法无误（布局/路由/预填均正确），示例层零改动。
- 全局回归：scene/rendering/textinput/embedder/overlay 全包绿 + race 干净；
  C11 策略窗切换 2 次、损伤双模式、截图结构完整，Golden diff 与干净 HEAD
  逐位相同（6.92%，既有漂移）；`apidoc` 绿；硬编码回归空。
- 观察项（均与本轮无关，不挡合入）：`TestKeystrokeRatio_M5` 低负载仍红但干净树
  同红（见 §七）；`TestTextureBudget_EvictsUnderPressure` 偶发红，系 map 迭代随机
  挑受害者（lastUse 全 0 打平），提交文件、改动未碰逐出逻辑，单跑连绿。
