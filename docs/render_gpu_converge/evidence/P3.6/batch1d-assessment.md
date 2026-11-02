# P3.6 批 1d 影响面评估：动图生产接线（2026-11-02 开工登记）

直话：动图解码（批 1b）与显存消费链（批 1c/1b）都齐了，但**从 `ui/` 往上没有一行代码碰过 `AnimatedImage`**。批 1d 要接的就是这一层：一个能被真实使用方驱动的动图入口 + 节奏归消费方的正口（引擎不持 timer，与 Flutter 把播放放 framework 一致，design.md §5 已登记）。

一手实测（子代理广度扫描 + 本机逐条复核行号）：

## 1 现有承接面

- `ui/rendering/image.go`：`ImageState{Idle,Loading,Ready,Error}`（`:18-25`）、`RenderImage`（`:35-44`）、`SetLoading :58`、`SetImage :71`（拿所有权，已 Dispose 的拒收 `:75-83`，同指针重装保命 `:85-90`）、`SetImageShared :105`（不拿所有权，只降级 `:117-122`）、`Clear :135`、`SetError :145`、`OwnsImage :155`、`releaseOwnedExcept :165-176`、`Paint :191-214`、`HitTest :217`；绘制经 `paint_context.go:406-409` → `image_draw.go:25-37`。**没有动图入口，也没有驱动这套状态机的生产代码**（`SetLoading` 只有示例与测试在调）。
- kit 侧只有 `Picture` 门面（`ui/kit/prim.go:269,274,279` → `ui/kit/internal/prim/content.go:315-336`），**没有任何 kit 组件渲染图像**（Image/Avatar/Skeleton/Carousel 均不存在）；`examples/kit/qrcode/main.go:172-173` 因为 kit 没有图像组件，只能在示例里手搭 `RenderImage`。
- `ui/kit` 是**扁平分面**且有明文政策（`ui/kit/doc.go:11-21`「无按组件子包」），而 AGENTS.md:21 写的是 `ui/kit/<name>/`——两份真源打架（见 §4 待拍 D17b）。新组件按既有前缀惯例是 `anim_image_{props,state,render,theme,widget}.go` + `BuildAnimatedImage` + `prim.go` 附近门面。
- `ui/io` 异步解码池**没有多帧路**：`Result{Img *render.ImageBuf}`（`ui/io/decode.go:47-55`）、`RegisterDecoder :72`、`DecodeFile :402`、`init` 注册四格式 `:663-678`；GIF 走这道门只拿得到**第一帧**（`render/internal/image/io.go:159-168`）。

## 2 节奏设施（都在，缺的是一等公民正口）

- 调度：`ui/scheduler/ticker.go` `Ticker :26`、`TickerRegistry.Add :63`/`Remove :78`/`TickAll :107-168`/`NextWake :205`；`ui/scheduler/scheduler.go` `Tickers() :491`、`Tick :697 → TickAll :720`、`WaitTimeout :604`（按 `NextWake` 睡）；生产驱动点 `ui/embedder/pipeline_app.go:1457`（每到期帧一次 `sched.Tick()`）、`:1251` `MayWantFrames` 闸门。
- 仓内**唯一的「N 毫秒后叫我」既有模式是 `Blinkable`**（`ui/rendering/pipeline.go:49-160` + 泵 `pipeline_app.go:726-765`），当前消费者只有光标：`ui/textinput/box.go:611` `BlinkTick`、`:621` `NextBlinkIn`、`:1781-1794` 聚焦期注册。
- 组件自持 dt 的先例：`ui/kit/icon_state.go:146-166` `IconInstance.Tick(dt)`（空闲返回 false 让宿主注销）+ 宿主在示例里 `TickAll`（`examples/kit/spin_min/main.go:30-43,112`）；以及 `ui/kit/internal/behavior/motion.go:91-145` 的 `Spinner` 包 `ui/animation/controller.go`（`Start :159`/`reg.Add :172`/`Stop :215-227`）。
- 结论：**没有通用的「宿主驱动组件推进」正口**——要么每组件自建 ticker 宿主（示例代劳、生产零消费），要么把 `Blinkable` 抽象成第二个 `*Wanter` 注册位（调度设施已在，属扩展非新造）。→ 待拍 D17。

## 3 两条会画错图的坑（引擎层，必须在批 1d 关）

1. **边界缓存的内容指纹看不见换帧**：`ui/rendering/boundary_cache.go:649` `imageContentKey` = 图像指针 + `State` + 尺寸；``:303` 确实在 `NeedsPaint()` 时重录，`:334-335` 在指纹变化时重录。动图刻意**复用同一 `*ImageBuf`**（批 1b D5），于是只要有一处忘了/来不及每帧标脏，`Ready` 边界就会**静默重放上一帧的录制像素**（不是变慢，是画错）。修法是把 `img.GenerationID()` 并进指纹（引擎侧一处，消费方忘标脏也不会画错）→ 待拍 D18。
2. **跨线程写读**：`render/internal/image/buf.go:44-46,51-58` 约定写者 UI 局限、只有 `Dispose` 跨线程（读侧 `RLock`）；但 `Data()` 返回底层片，**写内容不受锁保护**，而光栅线程在 flush 时才从 `cmd.PixelData` 拷贝（`image_cache.go:556-563`，键为内容代际 `:66-84`）。先例都靠"改写在 Paint 期内"或 GPU slot 回避（`video_bridge.go:173-179,286-292,315-321` 自有 `b.mu :138`；`ui/rendering/video_direct_node.go:15-22` 只在 Paint 里画；`anim.go:365-374`）。动图若从事件/定时器里直接 `DecodeFrame`，就是**新 Hazard**：光栅线程可能正读同一块内存。可取形状是 video 路（解码在 tick、只搬显示态、Paint 期内改画布），代价是 UI 线程同步解码（15 个样本实测单帧解码 0.1–0.6ms）。→ 待拍 D19。
- 附带：显示列表按指针持有缓冲并**回放期重取代际**（`backend_canvas.go:292-301`、`display_list.go:55-65`），所以批 1c 的换号语义在保留/录制两侧本来就自洽。

## 4 其它必须记的事实

- 分层禁令有测试守着：`ui/depcheck_test.go:12-50`（`ui/` 不得直引 `gpu/`），批 1d 全程经 `render`。
- `AGENTS.md:21` 的 `ui/kit/<name>/` 与 `ui/kit/doc.go:11-21` 的「无按组件子包」互相矛盾：新组件按哪个放？→ 待拍 D17b（这不是小事，落错地方整条线要返工）。
- mip 档位的接线**不在批 1d**：`engine/tex` 的 `Filter.SamplerParams`（`engine/tex/mipmap.go:207-214`）是 2.5D 线的出口，其消费属 P3.7；批 1d 只动 UI 组件支柱。
- 真窗：AGENTS「每主能力独立真窗」+ 批 2 期间无窗可验（生产消费者为零），批 1d 交付后动图才第一次有真生产消费者；`examples/kit/<name>/` 一目录一组件（现状 9 个），`-auto-only` 自检 + `evaluateGolden` 金图（例：`examples/kit/color-model/main.go:66,386-396,515-540`）。D9 已拍的 `examples/ui_wr_p36_animplay`（render 侧播放链）与 kit 组件窗是两件事：前者归批 3 引擎窗，后者是本批组件的 `examples/kit/anim-image`（或按 D17b 结论定名）。

## 5 待拍板（D17–D19）

- **D17** 节奏落点：(A) 把 `Blinkable` 泛化成通用 `PaintWanter`（`NextRepaintIn` + `OnRepaintTick`）注册进 `ui/rendering/pipeline.go` 既有泵，光标与动图共用一条驱动路，改到已关账 R1/R5 面；(B) 只在 kit 层自建 `TickerRegistry` 宿主，不碰 rendering 面，代价是每组件各搭一套、与光标那套并存两份驱动。
- **D17b** 组件落点：按 `ui/kit/doc.go` 扁平分面（现状政策）还是按 AGENTS.md:21 的 `ui/kit/animimage/` 子包；两份真源必须择一并改另一份。
- **D18** 是否把生成号并进 `imageContentKey`（推荐：引擎侧一处，把"忘标脏就画错"变成"不会画错"）。
- **D19** 帧推进时机：(A) Paint 期内推进（video 路先例，UI 线程同步解码，线程模型不新增 hazard，代价大图解码可能掉帧）；(B) 解码在 tick / 显示在 Paint（零掉帧，需新锁或 slot，与 `buf.go` 现行约定冲突）。

本轮零代码变更（评估 + 登记）。
