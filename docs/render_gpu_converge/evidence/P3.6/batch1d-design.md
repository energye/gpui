# P3.6 批 1d 方案：动图生产接线（2026-11-02 拍板后落盘）

拍板（用户 2026-11-02，四条全按推荐）：**D17** 把 `Blinkable` 泛化成通用「按时间轴推进显示态」注册位，光标与动图共用同一条驱动路（不造第二套泵）；**D17b** 新组件按 `ui/kit/doc.go:11-21` 的**扁平分面**政策落地（`ui/kit/anim_image_*.go`），`AGENTS.md:21` 的 `ui/kit/<name>/` 措辞需同步纠正（该文件由用户持有，我只登记不擅改）；**D18** 内容世代号并进 `imageContentKey`，让"忘标脏"从画错降级成只是重录；**D19** 帧推进在 **Paint 期内**做（沿用 video/特效 RT 既有形状，不新增跨线程写 hazard）。

D17 执行中用户追加两条拍板（同日）：改名换业界（Flutter）命名；发现 `ui/scheduler.Ticker` 与树级接口**同形状不同语义**（`Tick(dt) bool` 的返回值一边是「是否翻转」一边是「是否继续驱动」）后，拍板**本批就把契约合一**——`ui/rendering` 不再自定义参与者接口，直接采用 `scheduler.Ticker` 单一契约，见 §1/§1.1。

## 1 节奏：一条驱动路，不是两套

现状（实测）：`ui/rendering/pipeline.go` 有 `Blinkable.BlinkTick(dt) bool`（`:47-52`）、可选 `BlinkDeadliner.NextBlinkIn()`（`:54-59`）、`Note/Drop/BlinkActive/NextBlinkWake`（`:61-135`）、embedder 回调 `blinkCtl`；泵在 `ui/embedder/pipeline_app.go:726-765`（`Tick :732`、`NextWake :756`），唯一消费者是光标（`ui/textinput/box.go:611,621,1781-1794`）。既有硬约定保留：**帧由脏驱动，不由 ticker 活着驱动**（`pipeline.go:42-45` 注释）。

契约合一（最终形态；接口全量切换制，删旧不留垫片）：

```go
// ui/rendering/pipeline.go —— 参与者契约与实现都直接用 ui/scheduler 那一份
type Ticker interface{ Tick(dt float64) bool } // ui/scheduler 已有：返回 false = 注册表自动摘除
PipelineOwner.AddTicker / RemoveTicker / HasTickers / NextTickerWake / TickAllTickers / SetTickerCtl
// 树级注册表字段从自建 map+mu 换成 *scheduler.TickerRegistry（TickAll 的摘除/边沿/汇总语义单一）
```
- 中间态已作废：先按 Paced 系改过一轮名（`Blinkable→Paced`、`BlinkTick→PaceTick`、`Note/DropBlinkable→Note/DropPaced` 等 140 处/9 文件，见 §1.1），契约合一拍板后该组符号随旧契约一并删除，不再有任何 Paced/Blink 参与者符号。
- 语义对齐：`BlinkTick` 的返回值曾是「是否翻转」，`scheduler.Ticker.Tick` 的返回值是「是否继续驱动」——翻转/可见态变化由参与者自己 `MarkNeedsPaint` 承载（帧跟脏态来，`pipeline.go:42-45` 注释不变）；「继续驱动」负责注册表存续（返回 false 自动摘除）。两者不是一回事，这正是当初同形状不同语义的坑。
- 睡眠提示：`ui/scheduler` 公开面 +1 方法 `TickerRegistry.PollNextWake()`（当场问一遍在册 `DeadlineWanter` 取最早截止）。存在理由是一次真实缺陷：`NextWake()` 只读上一次 `TickAll` 的快照，新注册者在被第一次 tick 之前报不出截止点，事件循环正因空闲睡住时「注册即生效」路径（光标聚焦、动图开播）会睡到下一次外部输入。`PipelineOwner.NextTickerWake` 直接转调它。
- `ui/textinput/box.go`：`PaceTick→Tick`（返回 `focused` = 还要继续驱动）、`NextPaceIn→NextWake`（实现 `scheduler.DeadlineWanter`）、`syncPaceRegistration→syncTickerRegistration`（**行为零改动**，光标半周期/相位/聚焦注册一律不动，R1/R5 既有钉必须继续绿；光标自有 blink 语义名 `blinkHalfPeriod/stepBlink/blinkElapsed/caretOn` 刻意保留——闪烁就是它的业界词）。
- `ui/embedder/pipeline_app.go`：泵 `pacePump→tickPump`（树与帧调度器之间的桥）；Tick 用真实墙上间隔而非钳过的 dt，返回 `pipe.HasTickers()` 让自己空了自动摘，`WantsFrame` 恒 false；`:716` 的 ctl 安装点同步。
- 全仓旧名必须 `go/ast`+grep 双查无残留（含注释里指向旧名的话），`ui/kit` 门面 `behavior.go`/`gate.go` 若有导出也同批切。

## 1.1 步骤 1 执行结果（2026-11-02，未提交）

（中间态，已被契约合一取代）泵家族先按 Paced 系全量改过一轮名：`Blinkable→Paced`、`BlinkTick→PaceTick`、
`BlinkDeadliner→PacedDeadliner`、`NextBlinkIn→NextPaceIn`、`Note/DropBlinkable→Note/DropPaced`、
`BlinkActive→PacedActive`、`NextBlinkWake→NextPaceWake`、`TickBlink→TickPace`、`SetBlinkTickerCtl→SetPaceTickerCtl`、
`blinkers→pacedSet`、`blinkCtl→paceCtl`、`blinkPump→pacePump`、`fakeBlinker→fakePacer`、
`syncBlinkRegistration→syncPaceRegistration`；共 **140 处 / 9 文件**。改完后用户发现 `ui/scheduler.Ticker`
与树级接口同形状不同语义，拍板本批契约合一 → 上述 Paced 系符号随旧契约**全部删除**，不再存在。

（最终形态）契约合一已落地并复证：

- `ui/rendering/pipeline.go`：删自定义参与者接口（`Blinkable/BlinkDeadliner` 与 Paced 系），注册表字段
  `pacedSet map+blinkCtl` → `tickers *scheduler.TickerRegistry + tickerCtl`；公开 API
  `AddTicker/RemoveTicker/HasTickers/NextTickerWake/TickAllTickers/SetTickerCtl`，旧组（`Note/DropBlinkable`、
  `BlinkActive`、`NextBlinkWake`、`TickBlink`、`SetBlinkTickerCtl` 及其 Paced 系中间名）全删不留垫片。
- `ui/scheduler/ticker.go`：公开面 +1 方法 `PollNextWake()`（当场问在册 `DeadlineWanter` 取最早截止，
  不依赖上次 TickAll 快照；修复「新注册者睡过注册即生效」的真实缺陷）。
- `ui/textinput/box.go`：`PaceTick→Tick(dt) bool`（推进闪烁相位、返回 `b.focused` = 还要继续驱动；翻转仍由
  `TickCaret` 内部 `MarkNeedsPaint`，帧跟脏态来）、`NextPaceIn→NextWake`（实现 `scheduler.DeadlineWanter`）、
  `syncPaceRegistration→syncTickerRegistration`、私有 `nextPaceIn→blinkNextWake`；光标自有 blink 语义名保留。
- `ui/embedder/pipeline_app.go`：`pacePump→tickPump`（Tick 喂真实墙上间隔、返回 `HasTickers()` 空转自摘、
  `WantsFrame` 恒 false）。
- 四个 `examples/ui_wr_ime_r{2,3,4,5}`：`o.DropBlinkable(b)→o.RemoveTicker(b)`。
- 测试：`ui/rendering/pipeline_test.go` 两钉重写为 `TestOwner_TickerRegistry`（返回 false 自动摘除、摘掉的
  不再被 tick、空/非空边沿只翻一次）与 `TestOwner_NextTickerWake`（注册完立刻报得出截止、且注册不 tick），
  `fakePacer→fakeTicker`；`ui/textinput/blink_owner_test.go` 迁新契约（原「拿 Tick 返回值当是否翻转」的
  断言改为按契约断言注册存续/摘除，闪烁奇偶真值由 `IsCaretOn` 断言保留不丢）。

复证：`go build ./ui/...` 干净；四示例编译过；全仓 grep 旧名（Paced 系 + Blink 系参与者符号，含注释）
零残留；`ui/scheduler` 全包逐文件 12/12 绿、`ui/embedder` 全包逐文件 14/14 绿、`ui/textinput` 全包逐文件
19/19 绿（含 R1/R2/R4/R5 已关账面钉）、`ui/rendering` 逐文件 84 绿 + 1 基线 flaky。
一处需要如实说明的观察：整包一把跑 `ui/rendering` 时 `TestBoxesForRange_FullLinear_M1` 会红，
但**单跑三次全绿**、且批 2 逐文件门禁里 `ui/rendering/m1_noquadratic_test.go` 记 **PASS**
⇒ 判为整包并行/时序耦合产物，非本步回归；按台账纪律逐文件模式才是门禁口径（同 `TestKeystrokeRatio_M5`
属基线同名存量红——本步逐文件跑它也曾单跑 PASS/连跑 FAIL 交替，确认是时序敏感的比值测试，按基线 FAIL 名单记）。

## 2 缓冲承接面：`ui/rendering.RenderImage` 开动图入口

- 新入口 `SetAnimatedImage(a *render.AnimatedImage) error`（装时接管所有权：组件持有解码器、`RenderImage` 以 `SetImageShared(a.Image())` 显示当前帧；换/清时不 Dispose 别人的缓冲——沿用 `:105-133` 的 owns/demote 语义与 `releaseOwnedExcept :165-176`）。
- 控制动词（都是"动作"，不是行为实现）：`AdvanceFrame() bool`（把到期的帧推进交给自己 Paint 期调用）、`SetAnimatedPaused(bool)`、`SeekFrame(i int) error`、`AnimatedFrameIndex() int`、`AnimatedInfo() (frames int, loop int)`。
- **推进点固定在 Paint 内**（D19）：`Paint :191-214` 进入时若有待推进帧，就 `anim.DecodeFrame(i)`（内部 `NotifyPixelsChanged` 换代际）→ 走既有 `drawImageBuf`（`paint_context.go:406-409 → image_draw.go:25-37`）。这样写像素与光栅线程读像素天然错开（先例 `video_bridge.go:173-179`、`anim.go:365-374`、`video_direct_node.go:15-22`）。
- 时间轴只存在于 kit 组件（见 §3），`ui/rendering` 不持 timer、不知 fps、不做循环策略——引擎不持节奏的纪律继续成立。

## 3 组件：`ui/kit/anim_image_*.go`（扁平分面，D17b）

文件按既有前缀惯例：`anim_image_props.go`（属性/选项/哨兵错）、`anim_image_state.go`（时间轴状态机：当前帧、累计时间、循环计数、速度、暂停）、`anim_image_render.go`（构建/绘制接线）、`anim_image_widget.go`（`BuildAnimatedImage` 与 widget 侧装配）、`anim_image_theme.go`（只有确有需要才建，不硬编码文案/尺寸——引擎层硬编码纪律）。

状态机职责（**全部是"节奏与显示态"，不碰解码与绘制实现**）：
- `Tick(dt)`：实现 `scheduler.Ticker`——累加时间，跨过当前帧 `FrameInfo.Duration` 就把 `RenderImage` 的待推进标记置位并 `MarkNeedsPaint()`；返回「是否继续驱动」（播放中 true；暂停/播完不循环/组件销毁 false，注册表自动摘除）——这正是契约合一后组件层白拿的收益，播完即不再占泵。
- `NextWake()`：实现 `scheduler.DeadlineWanter`——到下一帧还剩多久（`ok=false`=暂停/单帧图/已停在末帧不循环）→ 事件循环按它睡，不空转。
- 循环语义按容器真值：`RepetitionCount()` 负=无限、0/-1/N 的映射沿用批 1b 已钉口径；`Duration==0` 按容器原值交给策略层，**不在组件里偷偷改写**（批 1b 已登记"节奏归消费者"）。
- 聚焦/驻留控制：不可见（`Layout` 零尺寸或节点被裁剪掉）时应能暂停，避免看不见的动图持续烧解码（钉一条）。
- 注册/注销时机照光标先例（`box.go` `syncTickerRegistration` 同一形状，`owner.AddTicker/RemoveTicker`）。

## 4 D18：指纹带图号 —— 实测推翻前提，收回原判断（待确认）

原方案：`ui/rendering/boundary_cache.go:649-660 imageContentKey` 只看「缓冲指针 + `State` + 尺寸」，
把 `im.Img.GenerationID()` 并进去，让「忘标脏」从画错降级成只是重录。

实测（`batch1d/d18-probe-results.md`，2026-11-02）结论相反：

- 前提不成立。录制里的图像命令存的是**缓冲指针**（`recordAbsoluteOwnContent`→
  `drawImageOn`→`Recorder.DrawImageEx`，回放时 `CanvasBackend` 再调一次活上下文），所以原地换帧后
  即便走重放，画的也是最新一帧。第一版钉（断言「必须重录」）只红在计数器
  （`frame2 rerecord=0`），像素那条是绿的——画面跟手，与指纹无关。
- 并进去只有代价：烘焙了动图的边界每帧作废重录（rerecord/frame 0.00→1.00）。实测
  700×700、边界内 200 子节点时 12.84ms/帧 → 13.66ms/帧（+6.4%）；400×400、24 色块时
  4.45–4.58 → 4.66–4.87ms/帧。文字版那台仪器被光栅化主导（两种指纹都 24.8ms 左右），
  差异落在噪声里，如实登记为无效对照，不拿来当收益说事。
- 盲区不可达：图像缓冲只能经 `SetImage`/`SetImageShared` 装上，两者都按 R10 契约标脏；
  `RenderImage.Img` 在 `ui/ engine/ examples/` 无引擎层直接赋值点。

落地：行为不改（`boundary_cache.go` 的 `git diff` 为空），只加防护——
`ui/rendering/boundary_cache_contentgen_test.go`
`TestRenderImage_InPlaceFrameAdvanceReplaysFreshFrame` 同时钉「画面为最新帧」与「这一帧仍走
重放（rerecord=0 / skip≥1）」，第三条钉「换缓冲必须重录并显示新图」。反向对照：把图号并进
指纹 → 计数器那条红；把 `recordAbsoluteOwnContent` 的图像分支改成 `t.Img.Clone()`（录制即像
素快照）→ 像素那条红（`RGBA{0xdc,0x1e,0x1e}`）。即哪天录制语义改成快照，钉会先叫。

动图的失效通路按 D19 走：推进帧时由组件标脏，不靠指纹。

## 5 验证（三证据 + 双后端 + 负向）

- 逻辑钉（`ui/rendering` / `ui/kit`）：动图入口的所有权语义（共享不 Dispose、换装降级、Clear 不误放）；`Tick` 的到期判定与 `NextWake` 的睡眠提示（含 `Duration=0`、单帧图、暂停、末帧不循环）；循环计数；不可见即停。
- 像素钉：同一动图逐帧画进离屏 target 并按真值断言着墨变化（复用批 1b 的 `testdata/imgfmt` 与 oracle 参照，不新造数据）。
- 真窗：`examples/kit/anim-image/`（组件支柱窗，一目录一组件），**只用控制动词**（Play/Pause/Seek/Speed/循环档切换 + 演示数据切换），`-auto-only` 自检出 JSON + 金图比对（照 `examples/kit/color-model/main.go:66,386-396,515-540` 的形状），常驻等人手工二合一。
- D18 专项：已做完，改为「原地换帧重放仍新鲜 + 仍走重放」的回归钉（见 §4），不改行为。
- 负向至少三路：摘 `Tick` 的标脏 → 组件不动；把推进点从 Paint 挪到 tick → 线程钉（`-race`）能判（这条如果实测判不出就不写"能判"，如实说）；D18 钉的两路反向对照已实测（并进图号 → 计数器红；录制改像素快照 → 像素红，见 `d18-probe-results.md` §4）。
- 门禁全套：apidoc rc=0、vet 触及包、定向、`-race -count=2`（新增动图状态跨线程 + 泵）、conformance、五目标×{默认,nogpu}、逐文件对基线（**本批要格外小心 R1/R5 已关账面**：`ui/rendering`、`ui/textinput`、`ui/embedder` 逐文件全跑）、金图零变化或逐窗 A/B 归因。
- 跨平台标记：本批纯 UI/rendering 层，无后端差异；`-race` 与双后端像素按既有口径写。

## 6 切分（每步可编译可测）

1. 泵契约合一（D17 最终形态：`scheduler.Ticker` 单一契约 + `PollNextWake`）+ 光标行为零改动复证；
2. D18 指纹：实测推翻前提 → 收回，只加回归钉（`boundary_cache_contentgen_test.go`），行为零改动；待用户确认后关洞；
3. `RenderImage` 动图入口（Paint 期推进，D19）+ 所有权与像素钉；
4. `ui/kit/anim_image_*` 组件 + 注册/注销 + 逻辑钉；
5. `examples/kit/anim-image` 真窗（控制动词 + 自检 + 金图）；
6. 文档收口（catalog 若触及 render 公开面、`ENGINE_UI_WIDGET_RENDER.md` §10、kit 侧账、台账状态与修订行）+ 全套门禁 + 提交前确认。

## 6.1 步骤 3–6 执行结果（2026-11-03，未提交）

- **RenderImage 承接面**（`ui/rendering/image_anim.go`）：与 §2 一致，落地差异——`AdvanceFrame() bool` 是请求式（置 pending 并标脏，返回「有下一帧可推」，真正的解码固定在 Paint 期，D19）；`AnimatedFrameIndex()` 语义为「已解码显示帧」（Paint 期推进后才更新）；`SetImage/SetImageShared/Clear/SetError` 四处加动图让位互斥（显式装静态缓冲即退出动图模式并释放解码器）；已释放解码器拒收转 Error（对齐 SetImage 的 disposed 拒收）。`render.AnimatedImage` +1 方法 `Disposed`（消费者拒收判别，对齐 `ImageBuf.Disposed`；apidoc rc=0、catalog 已同步）。
- **参照对齐**：Flutter `multi_frame_codec.cc:59-145 GetNextFrameImage`（required_frame 回填/disposal 擦矩形/lastRequiredFrame_ 存储）——仓内 `anim.go` 的 `stepTo/applyDisposal/needSave` 逐形状对应；Skia `SkPixelRef`（getGenerationID/notifyPixelsChanged/addGenIDChangeListener）——本地参照仓 skia 目录为空壳（只有 BUILD.gn），按上游 include/core/SkPixelRef.h 核对语义，仓内批 1c 实现同构。
- **kit 组件**（`anim_image_{props,state,widget}.go`）：与 §3 一致，契约按步骤 1 的合一结果——`Tick` 返回「是否继续驱动」（§3 原稿写「返回是否改变了可见状态」，契约合一后此句作废）；组件**自持时间轴 idx**：推进是请求式，用节点已显示帧判定时长会在「连续 tick 而 paint 未跟上」的间隙卡死时间轴（第一版真 bug，kit 钉 `TickAdvancesByFrameDuration` 抓住后修正）；播完自停除 Tick 返回 false（registry 自动摘）外再显式注销一次（直接调用 Tick 不经过注册表循环的路径也干净）。
- **真窗** `examples/kit/anim-image`：三卡三循环档（keep 无限/prev restore-previous 2 轮/once 单播），只用控制动词（键盘 空格/s/x + 标题状态）。-auto-only 5s 三跑：逻辑（keep 推进至帧 2、once 播完自停末帧 1）、像素（区域暖色计数 10848≥150）、金图三连 0.0000%（401920px，冻结帧位=全体 Pause+Seek(0)）、fps 57.6/p95 18.6ms/hitch 0/gpu_ops 7179/fallback 0、正常关闭。过程修正两处自检脚本缺陷（每 tick 重调 Play 把播完卡复活；单点探针落在素材帧底色），非组件缺陷、如实登记。
- **登记不做（新增）**：真窗「不可见（被裁剪掉）即停」只钉了未挂树/零尺寸两态，祖先裁剪感知需要树支持，归后续；常驻人工窗的自动门禁不生效（无快照），以人工观察为准。

## 7 登记不做

- mip 档位的生产接线（`engine/tex.Filter.SamplerParams` → 绘制口）属 2.5D 支柱，归 P3.7；
- `ui/io` 异步池多帧路（现在 GIF 走那道门只拿第一帧）：本批若组件需要异步解码再接，先登记；
- `AGENTS.md:21` 与 `ui/kit/doc.go` 的措辞冲突需用户定夺后由用户改（我不擅改项目规则文件）；
- 动图帧矩形差量上传（省每帧重产链的开销）归性能专项。
