# 三线重设计立项 T（唯一工作文档）

> 状态：2026-09-16 立项，未开工。细案在本文件里逐条展开，
> `ENGINE_FLUTTER_SKIA_ARCH` §5、`ENGINE_UI_RENDER_BASE` §22.2、
> `ENGINE_ARCH_OVERVIEW` §7 只留一句话指过来，不各记一份。
> E2/E3/E5 保留当基线；E1 稀疏包已撤销不再碰；示例测试冻结等三线稳后重构。

## 1. 背景一句话

button 真窗点按拖拽慢半拍——界面线程建包 + 画纹理（`scene.RasterizeDirty`，
`ui/embedder/pipeline_app.go` 约1176行）串行，下一批鼠标事件只能等整套跑完。
标准要三线各干各的，现状光栅线只管送（`ui/raster/Loop`）、不管画。
本库开发阶段无用户，组件示例可重写，先把基座三线写对，再修测试示例。

## 2. 三线分工（对齐 Flutter：平台 / 界面 / 光栅 / 图片各一条）

| 线 | 在哪跑 | 干什么 | 铁律 |
|----|--------|--------|------|
| 界面线 | `ui/embedder` `Run` 循环 + `ui/rendering` + `ui/scheduler` + `ui/input`/`gestures`/`focus`/`textinput` | 收原生事件、排版、录图、建 `FramePacket`，交完马上回事件循环 | 禁止 gpu Submit / 直接操作 Device；脏旗只在这条读写 |
| 光栅线 | `ui/raster/Loop`（锁系统线程）+ `ui/scene` 画与合成 + `render` 上下文 + `gpu` | 接包、画脏层纹理、合成、送屏，一次一包、新包顶旧包 | 禁止读可变 RO 树，只读封好包；`ui` 不直调 `gpu` |
| 图片线 | `ui/io` 解码池 | 解图片，不挡前两条，解完跳回界面线挂树 | 排版画画时不许同步解 |

## 3. 交接规矩

- `FramePacket`（`ui/scene/packet.go`）交出去即只读：根层指针共享 + 脏号小拷，不整树深拷（只许 `CloneShallow`）。
- 画画步骤里的大块头（路径深拷、字形、图片缓冲）建包时定死，光栅照着画，不回头问树。
- 脏旗只在界面线清；纹理缓存（`PictureTextureCache`）只在光栅线读写；显卡提交只在光栅。
- 背压：在途最多 2 包，满了扔掉没开画的旧包画新的，不 BLOCK 收事件。
- 关窗顺序：停光栅 → 等画完 → 关设备 → 拆窗。
- 快照读屏、指标计数全在光栅任务里串行做，不跨线程读。
- 纪律：`ui → render → gpu`，禁 cgo，禁示例里绕洞。

## 4. 分期与每期门（细案在本文件逐条议，一期一议一开工）

| 期 | 做什么（落点） | 门（有一项不对回滚当期） |
|----|---------------|--------------------------|
| T0 基线冻结 | E2/E3/E5 留基线不动；示例测试冻结；记基线数（按钮包、渲染包、真窗140项、`Children` 361次、race干净） | 基线数落 §8 状态表 |
| T1 封包只读 | `ui/scene/packet.go` + `ui/rendering/layer_build.go` 建包十几行：交出去后不再改，单测锁“交后改树不影响已交包” | 新单测绿；包相等门绿；`ui/scene` + `ui/rendering` 逐文件绿 |
| T2 搬画画出界面线 | `pipeline_app.go` 约40行 + `raster/loop.go` 十几行：`RasterizeDirty` 搬进 `raster.FrameJob`，界面建完包即回循环（延迟消失在这步） | 点按反馈≤2帧（T真窗实测）；`ui/embedder` + `ui/raster` 逐文件绿；真窗140项绿 |
| T3 合成送屏收尾 | `ui/scene/textured.go` + `render/present_target.go` 调用点：纹理缓存、损伤区、快照、指标串进光栅任务，`render` 上下文归光栅独占（`gpu` 不动） | 金色像素对比绿；改尺寸/遮挡恢复不黑屏；`ui/scene` + `render` 逐文件绿 |
| T4 图片线扶正 | `ui/io/decode.go`：工人数量与回跳界面线规矩写死，排版画画里同步解码报错 | 大图解码时滚列表不卡（T真窗实测）；`ui/io` 绿 |
| T5 解冻示例测试 | 重构 `examples/kit/button` 事件与组件测试，不倒灌回引擎 | 真窗140项绿；按钮包绿；E2 两道门常挂 |

## 5. 门（通用，每期必查）

每期 `ui/...` 逐文件跑（禁一次全量）；包相等 + 数诚实（E2 两道门）常挂；
`CGO_ENABLED=0` 与 `ui→gpu` 禁令每期查（`go test ./ui -run TestUIDoesNotImportGPU`）；
`go test -race` 跑改动包；有一项不对回滚当期，不往下走。

## 6. 专有真窗 `examples/ui_wr_t_stress`（未建，先占位，T2 前建好）

> 为什么要专有：三线是基座，button 示例只盖住按钮一种场景，
> 盖不住图片解码、遮挡恢复、改尺寸风暴、多动画并跑。
> 按规矩每个主能力要有独立真窗，不拿 button 窗代替。
> 命名跟 `ui_wr_r*`/`ui_wr_c*` 家族对齐（`ui_wr_t` = 线程 thread）。

覆盖场景（越复杂越好，一个窗全装下）：

1. 输入跟手：点按钮→变色目标≤2帧；拖滚动条跟手；滚轮连滚不顿。
2. 动画并跑：4+ 转圈 60fps + 波纹 + hover + 渐变，记帧时/CPU。
3. 图片重载：大图边解边滚列表，滚得动=图片线没挡界面线。
4. 改尺寸风暴：拖边快改尺寸，内容跟边框走，不黑屏（E1/B1 教训锁死）。
5. 遮挡/最小化恢复：全停帧，回来整窗重现，一个按钮不少。
6. 脏层正确：静按钮贴图、脏小层重录，金色像素逐字节对比。
7. 快照一致：读屏像素与送屏像素一致（`SnapshotAsync` 路）。
8. 并发干净：`-race` 跑常驻操作无竞态（E3 教训锁死）。

双模式（跟其他真窗同规矩）：`-auto-only` 跑门禁出 JSON 直接判 PASS/FAIL；
常驻等人工点拖滚，收真事件打日志改标题，关窗或超时后出汇总 + JSON。
门禁数：帧间隔 p99、hitch 数、输入到反馈帧数、静稳态 CPU、包相等抽查。

测试数据一律入本窗 `testdata/`（大图只存小样 + 生成脚本，不提交二进制），
进程临时文件只许 `t.TempDir()`。

## 7. 状态变更（本文件是唯一状态源，状态只认这张表）

| 日期 | 期 | 状态 | 证据 |
|------|----|------|------|
| 2026-09-16 | T 立项 | ⬜ 未开工 | 本文件创建，三处旧址收敛为指针（本地提交 `d868b18`） |
| 2026-09-16 | T 需求扩展 | ⬜ 未开工 | 补 UI/光栅/图片细则（§8–§10）、归属表（§11）、风险（§12）、指标字典（§13）、回归窗单（§14）、回滚规则（§15） |

## 8. UI 线细则（只减负，不加活）

- 留下：收原生事件、`Tick` 心跳、排版（脏才排）、录图、建包、交包。`focus`/`gestures`/
  `textinput`/IME 会话/剪贴板回调全留界面线——它们改树，只能在这条。
- 事件合并：鼠标移动多格合一格再交（X11 事件密，合并不丢终点）；滚轮按帧合并，
  一帧最多消费一次滚动增量，剩下的下帧再算——滚得快不断流，滚得慢不空转。
- 帧预算：建包（含 E2 顺手计数）目标 < 4ms/帧@120 节点树；超了先报指标，不私自拆帧。
- 禁止新增：界面线禁 gpu Submit、禁直接操作 Device、禁同步解图片、禁同步读屏，
  违者单测拦（`TestUIDoesNotImportGPU` 常挂 + 新增 `TestUIThreadNoBlockingIO` 意向）。

## 9. 光栅线细则（一包一任务，画完即送）

- 一个 `raster.FrameJob` = 画脏层纹理 + 合成 + 送屏 + 串行快照/指标，画不完新包到，
  则画完手头这包就画新的（`PendingSlot` 最新 wins 保留）。
- `render.Context` 今天非线程安全（pixmap、裁剪栈、GPU 会话全是裸字段，无锁）：
  光栅线独占一个 Context，界面线不再拿它画屏；界面线要量尺寸只走逻辑尺寸。
- 纹理缓存（`PictureTextureCache`）只活在光栅线：建、查、淘汰、释放全串行，
  界面线连读都不读——释放时正在画的包已画完（同一串行队列），无释放后使用。
- 改尺寸：`PresentTarget.Resize` 保持“记录”语义，真正的 swapchain 重配在光栅任务里
  串行做；重配完成前旧缓冲继续送，不黑屏（沿用 resize 风暴期的 Mailbox 策略）。
- 送屏模式：Fifo/Mailbox 切换只在光栅线 रिकॉर्ड（record-only），下一 present 边界生效。
- OOM/驱动报错：沿 `oom_exit` 老路在光栅任务里记，界面线只收退出信号，不在光栅里弹业务。
- 快照读屏：只在 present 完成后同任务读，读到的是本包像素，不跨包、不跨线程。

## 10. 图片线细则（解完跳回，不挡路）

- `ui/io` 工人池固定 2 工人（`Default = NewPool(2)` 沿用），大图（>2MP）单飞一个任务，
  不堵小图；解码中窗关了，任务取消（查 quit 标记即扔，不回调）。
- 内存顶：解完图片进缓存前查预算，超了先淘汰最久不用，记 `cache_evictions`，
  不静默吃内存。
- 回跳规矩：回调一律跳回界面线再挂树（现 `DecodeFile` 的 done 即如此，写死不断），
  光栅线永远不直接消费解码结果。
- 排版/画画里同步解码 = 报错（debug 断言 + 指标记一次 `sync_decode_violation` 意向），
  不只靠口头禁止。

## 11. 共享对象归属表（谁建、谁读写、谁释放）

| 对象 | 主人 | 规矩 |
|------|------|------|
| RO 树（`RenderObject`） | 界面线 | 光栅碰一下就算错；单测可用 race 跑并发建包抓 |
| `FramePacket` | 界面建、光栅只读 | 交出即冻结，只许 `CloneShallow`；脏号小拷 |
| `Picture`/`PictureOps` | 界面录、光栅放 | 路径深拷在建包时定死；放完不写回 |
| `ImageBuf` | 图片线解、界面挂、光栅画 | 释放只在光栅画完后（引用随包走，包扔即放） |
| `PictureTextureCache` | 光栅独占 | 界面不读不写 |
| `render.Context`（画屏那台） | 光栅独占 | 界面另用纯逻辑量尺，不碰这台 |
| 脏旗 `needsPaint` | 界面独占 | 光栅不动（E1 教训） |
| `MetricsStore` | 两线只写原子项 | JSON 拼装只在界面线读快照拼，不跨线程拼半截 |
| 快照队列 | 界面写、光栅读 | E3 那把锁保留，协议不变 |
| X11/Wayland 句柄 | 界面线（事件泵线程） | Xlib 非线程安全，`NotifyFrameDrawn` 等跨线程调用走事件泵线程 deferred（沿用现有注释做法） |

## 12. 风险与对策（我想到的坑）

1. `render.Context` 无锁——对策：第 9 条，光栅独占一台，不加锁硬顶。
2. X11/Wayland 线程亲和——对策：事件泵和窗口句柄不出界面线，光栅只碰 GPU 面。
3. Go GC 抖动：每帧 new 包——对策：包结构体小（指针 + 小拷），大块头（路径/字形/图）随脏才建；
   T2 后看 p99，涨了先查分配再动刀。
4. 纹理释放后使用——对策：第 9 条串行队列天然 cover，另加 debug 模式毒化释放句柄。
5. IME/剪贴板系统回调半路杀到——对策：一律先收进界面线队列，帧边界再处理，不在回调里改树。
6. 多窗预留：现在一窗一 `Loop` 一包流；第二扇窗另起同构流，不共享 Context/缓存，
   `ENGINE_GPU_MULTIWINDOW_PLAN` 到时再对接，本期只保证不写死单窗假设（Loop/包/缓存全挂 `PipelineApp` 实例上，不落全局）。
7. 指标“假进步”：今天 `build_ms` 里含画纹理，搬走后 build 跌 raster 涨——对策见 §14，
   跌涨必须成对出现，单边好看即假。
8. 旧窗回归面大：`pipeline_app.go` 是所有真窗总口——对策：T2 单做，§15 窗单全跑，红一扇即回滚。

## 13. 指标字典（JSON 里看什么，怎么防假）

- `frame_build_ms`（界面建包）搬后必跌；`frame_raster_ms`（光栅画+送）必涨；
  两者之和对比搬前总和，不涨即赢——单看 build 跌不算赢。
- 新增 `input_latency_frames`（点按/滚动到反馈上屏的帧数，T 真窗实测）：T2 的生死线，
  目标点按 ≤2 帧、滚动跟手不断流。
- 常挂：`hitch_count`、p95/p99、`vsync_source`、`boundary_count`/`boundary_max_depth`
  （E2 顺手数，搬后不得变）、`damage_area_px`（静帧趋零）、`cache_evictions`、
  `pending_coalesce_drop`（扔旧包计数，新增意向：只增不减，证明背压在干活）。
- 防假：搬迁期凡 build 跌而 raster 不涨、总数不变而帧数涨、静帧 damage 不零，
  一律按“数不诚实”回滚（沿用 E2 数诚实规矩）。

## 14. 回归窗清单（T2/T3 必跑，红一扇即停）

动画与脏层：`ui_wr_c5_anim_over_static`、`ui_wr_r3_boundary`、`ui_wr_r3b_compbits`、
`ui_wr_r4_composite`、`ui_wr_r4b_multidamage`、`ui_wr_c1_boundary_nest`、
`ui_wr_c2_retained_scene`、`ui_wr_r6_layer_anim`；
滚动：`ui_wr_c3_list_scroll`、`ui_wr_r7_virtlist`、`ui_wr_r7b_scroll_reuse`、
`ui_l1_scroll`；改尺寸：`ui_wr_c7_resize_dpr`；图片：`ui_wr_r10_async_image`；
后台：`ui_wr_r17_bg_throttle`； soak：`ui_wr_r15_soak`、`ui_wr_c10_soak`；
像素：`ui_wr_r19_snap`、`ui_wr_c4_shell_overlay`（金色对比）；
叠加层：`ui_wr_r8_overlay`、`ui_wr_c4_shell_overlay`；文本：`ui_wr_r9_text_cache`；
策略切换：`ui_wr_c11_policy_switch`；button 真窗 140 项。
跑法：每窗 `-auto-only` 先过，再抽 3 扇常驻人工点拖（滚动+改尺寸+动画必抽）。

## 15. 回滚规则

- 一期一滚：只回滚当期，不连带；T2 回滚即回 `pipeline_app.go` + `raster/loop.go`
  当期 diff，E2/E3/E5 基线不动。
- 本库无用户，不留双路开关（沿 F1/F2 规矩：直接重写，绿了就合，红了就 revert）。
- 回滚后 §7 状态表记一行“回滚+原因”，不抹账。

## 16. Flutter 逐项对齐表（极致版，逐条抠自 `engine/src/flutter` 真代码）

| # | Flutter 机制（出处） | 咱们的落点 | 现状 | 完美线待办 |
|---|----------------------|------------|------|------------|
| F-T1 | 4 跑道 `TaskRunners(platform/raster/ui/io)`（`common/task_runners.h:17`） | 平台线=宿主事件泵、界面/光栅/图片三线 | 平台线没单独立项，事件泵和建包混在界面线 | T1 立名分家：事件泵归平台线，建包归界面线（§18-G1） |
| F-T2 | `Animator::RequestFrame(regenerate_layer_trees)` + 一次性 `AsyncWaitForVsync`（`shell/common/animator.*`） | `FrameScheduler.ScheduleFrame` + vsync 监听协程 | 有对等物；缺 `regenerate` 参数（脏了才建/纯合成复用上一包） | T1 加 `regenerate` 语义：纯位移/透明帧不重建包（§18-G2） |
| F-T3 | `BeginFrame→OnAnimatorBeginFrame→Render(layer_tree)→EndFrame` + `pending_frame_semaphore_(1)`（`animator.cc:61`） | 收事件→建包→交包 + `Pending` 标记 | 有对等物；`EndFrame` 无显式点（包交出去即 end） | T1 显式化：交包=EndFrame，`Pending` 即信号量（§18-G3） |
| F-T4 | `Pipeline<FrameItem>` + `ProducerContinuation`：满了 `Produce()` 失败，UI 本帧不建、下个 vsync 再试（`pipeline.h:46`，`animator.cc:95`） | `PendingSlot` 最新 wins（建完再合） | 建完再扔，浪费一次建包 | T2 改“建前占位”：占不到槽本帧不建（§18-G4） |
| F-T5 | 管道深 2，平台光栅同线程时深 1（`animator.cc:32`） | 在途 2 包 | 有对等物 | 合并模式深 1（见 F-T7） |
| F-T6 | `Rasterizer::Draw` 跑光栅跑道：取包→画→送；不对线程即 `kYielded` 让路（`rasterizer.cc:248`） | `Loop.exec` | 缺“不对线程让路”断言 | T2 加线程断言：UI 线调画直接报错（§18-G5） |
| F-T7 | `RasterThreadMerger` 租约合线程（平台视图/低端机，`fml/raster_thread_merger.h:43`） | 软件兜底路（GPU 不可用走 CPU） | 合并无名分、无租约 | T3 立合并模式：静态合并（深1）+ 名分，不搞动态租约（§18-G6） |
| F-T8 | `SnapshotDelegate` 只活光栅（`rasterizer.h:114`） | `SnapshotAsync` | 有对等物；无亲和断言 | T3 加亲和断言：界面线读屏报错（§18-G7） |
| F-T9 | 图片解码跑 IO 并发跑道 + 注册表（`lib/ui/painting/image_decoder_skia.cc:270`/`image_decoder_impeller.cc:689` 并发池 + `GetIOTaskRunner` 上传；`ui_dart_state.cc:27` 仅传参） | `ui/io` 2 工人池 | 有对等物；无注册表/代际 | T4 加解码注册表：窗关取消、代际过期图不挂树（§18-G8） |
| F-T10 | `DispatchPointerDataPacket` 平台→界面带 trace 流号（`engine.cc:469`） | `InputRouter` | 无流号，输入延迟只能估 | T2 事件带帧号：点按→上屏帧数实测（§18-G9） |
| F-T11 | `FrameTimingsRecorder` 每帧记 build/raster 起止/present 回框架（`animator.cc:72` 记建起、`cc:130` 记建止；raster 起止在 `rasterizer.cc:635/667/691/716`） | `MetricsStore` 采样 | 采样制，非逐帧；build 含画纹理 | T1 起逐帧四戳（建起/建止/画起/送毕）带帧号上报（§18-G10） |
| F-T12 | `DrawLastLayerTrees` 无新树重送旧树（`animator.cc:228`） | retained 稳态重现 | 有对等物（B1 锁死）；无名分 | T3 立名分：无新包即重送旧包，不断流（§18-G11） |
| F-T13 | `OnAnimatorNotifyIdle` 闲通知（`animator.cc:158`） | `ModeIdle` 阻塞 | 有对等物 | 沿用，不单做 |
| F-T14 | 次级 vsync 回调（`animator.h:93` 一次性、界面线） | 无 | 无对等物 | T1 加：blink/指标等帧后钩子排这里，不插建包队（§18-G12） |

## 17. 完美线缺口清单（本期必补，落期见右列）

- G1 平台线分家 → T1：事件泵（收/合/发包）归平台线文件，建包归界面线，跨线只走包。
- G2 `regenerate` 语义 → T1：纯合成帧（位移/透明）复用旧包，`ScheduleFrame(regenerate=false)`。
- G3 交包即 EndFrame → T1：`Pending` 语义=信号量，文档+单测锁死。
- G4 建前占位 → T2：占不到槽本帧不建，下个 vsync 再试；`PendingSlot` 只保已开画包的最新 wins。
- G5 线程断言 → T2：UI 线调画/光栅读树/界面读屏，debug 直接报错（先断言后优化）。
- G6 合并模式 → T3：GPU 不可用=静态合并（深1、画送同界面线），有名分有单测，不搞动态租约。
- G7 快照亲和 → T3：读屏只在光栅任务，违者报错。
- G8 解码注册表 → T4：代际+取消，过期图不挂树。
- G9 事件帧号 → T2：`input_latency_frames` 实测，点按≤2帧生死线。
- G10 逐帧四戳 → T1：建起/建止/画起/送毕带帧号，build 跌 raster 涨必须成对。
- G11 无新包重送旧包 → T3：断流不断屏（改尺寸/遮挡恢复走这条）。
- G12 帧后钩子 → T1：blink/指标排队，不插建包队。

## 18. 遗漏细节追补（2026-09-16 已验代码，T1–T4 分头认领）

> 怎么验的：`layer_build.go:407`/`textured.go:1181`/`rasterize.go:50`/
> `pipeline.go:284`/`pipeline_app.go:176` 原行，不是推出来的。

- D1（最大，T2 生死）：`RasterExtra` 让控件 paint 跑在录制侧——Button 这类
  `OnPaint` 控件 UI 侧录不出显示列表，挂闭包等录制时拿活 DC 现画，
  读的是活控件字段（pressed/hovered/spinPhase/主题）。
  现在录制还在界面线串行所以没事；T2 一搬，paint 跑光栅、界面同时改字段即 race。
  对策：快照语义——建包时把 paint 要读的拷出来；
  先给 Button/Icon/Text 三家做（主战场），其余先走录制期界面让路（记指标）。
  T2 门加一条：`-race` 跑 button 窗点拖滚干净。
- D2（T2）：`RasterizeDirtyToContext` 写共享层状态（`NeedsRaster=false`、
  `Valid=true`，`rasterize.go:50-53`）。搬后即跨线程写。
  对策：收进包/光栅本地——已画集合只活光栅，不写回共享层；共享层只读。
- D3（T1）：`FlushPaint` 直接拿活 DC 画（`pipeline.go:303` `root.Paint(pc)`，函数头 284）那条路还在，
  单测金色全走它。 Counsel：单测保留同步 inline 模式（= Flutter 单测多跑道并一线程），
  真窗走三线；`Loop` 加仅测试 inline 开关，两条路名分写死。
- D4（T3 前置）：GPU 队列提交线程安全没人验过（`device.go` 直接拿 queue 用，
  render session 无锁）。对策：T3 前双验（两线程同 submit，race+真卡）；
  同时审计所有 queue 触点，保证只有光栅碰 queue（天然串行），违者断言。
- D5（T2）：present 完成无回执（`presents` 记的是 submit）。
  对策：光栅送完回扔完成号；`input_latency_frames` 靠它算；
  `-auto-only` 帧数改数完成数。
- D6（T3）：字形 atlas 归属没定（整形在界面，上传在哪、缓存谁读写）。
  对策：整形界面、上传光栅、atlas 缓存光栅独占，界面建包时拷出几何。
- D7（T4）：`ImageBuf` 上传点没定。对策：光栅首次见即上传进缓存（§10 预算位沿用）。
- D8（T3）：改尺寸三方握手（平台收→界面排→光栅配）只写了一句。
  对策：沿用 `pendingResize` defer，重配完前送旧缓冲，时序写成单测。
- D9（T2）：`WarmUp`/`MaxFrames`/`RunFor`/`SnapshotPath` 异步后语义。
  对策：WarmUp 同步跑完建+画+送；SnapshotPath 等光栅完成（done 通道沿用 E3）。
- D10（T1）：blink/IME/剪贴板/Overlay/HUD 回调 hops 没清单。
  对策：T1 列 hops 表（一行一回调：哪来、跳哪、帧哪处理），违者断言。

## 18b. 第二轮抠（2026-09-16，全部已验原行，有两条修正旧结论）

- D11（最大，活 race，不是未来风险）：`RasterExtra`（Button.paint）在光栅任务里
  读活控件（`textured.go:1181` 经 `recordLocalWith` 执行 `layer_build.go:411` 闭包），
  界面同时 `Tick`/`PointerMove` 写同一字段——合成+送屏今天已在光栅任务里
  （`pipeline_app.go:1238-1259` job.Run→presentPacketTextured），所以 D1 的 race
  今天就存在，不是 T2 才引入。对策不变（建包快照，Button/Icon/Text 先行），
  但 T0 先加 `-race` 常驻点拖滚复现，实锤再开 T2。
- D12（T2）：`NeedsRaster` 两边写——UI 侧 `rasterize.go:52` 清，
  光栅侧 `textured.go:1159` 读、`1195` 清，无锁。对策：读写收进一处
  （光栅本地已画集 + 包内脏号，共享层只读；D2 合并到本条执行）。
- D13（修正旧结论）：收事件不等满 16ms——`x11_linux.go:1116` poll 双 fd，
  来事件即返，16ms 只是轮询切片。之前“等满一片”的说法收回；
  延迟在帧节奏 + 建包 + 录制，不在收事件。平台线事件合并（§8）照做，
  理由改为减调用次数，不减等待。
- D14（T2）：`SaveLayerBudget` 是裸 int（`paint_context.go:416`），`RasterExtra`
  闭包带着它跨线程（`layer_build.go:412/458`）；Stats 是原子的没事。
  对策：budget 判定只在建包侧做，录制侧复用结论，不两边计数。
- D15（T4）：`ImageBuf` 包不留引用（`PictureOp.Image` 只存指针，树换图即野）。
  对策：包留引用到光栅送完（包内 `retainedImages` 小表，随包扔随放）。
- D16（T3 前置）：`Face`/atlas 亲和没定——整形器并发安全（`shaper_own.go:24`），
  但 Face 本体与 atlas 上传谁读写没写。对策：T3 前审计，不定不进 T3。
- 好消息（T2 变小）：合成+送屏已在光栅任务，T2 只搬纹理录制（含 RasterExtra）
  + 旗子 bookkeeping + 计数，不碰合成送屏。
  纹理缓存本身 T-ready（mu 全包、`SetLiveKeys` 协议、C4 修过 UI 超前 race，
  `textured.go:216/376`），T3 只定调用点，不重写缓存。
- 门禁语义（D5/D9 合一）：`MaxFrames` 数的是 submit（`pipeline_app.go:890`），
  T2 起改数光栅完成数，回执通道即 G9 帧号通道，一事两用。

## 19. 修订

| 日期 | 说明 |
|------|------|
| 2026-09-16 | 立项：本文件为唯一工作文档，三处旧址改为指针 |
| 2026-09-16 | 补分期门（§4）、通用门（§5）、专有真窗占位（§6）、状态表（§7） |
| 2026-09-16 | 需求扩展：三线细则/归属表/风险/指标字典/回归窗单/回滚规则（§8–§15） |
| 2026-09-16 | 极致对齐：Flutter 真代码逐项对齐表（§16，F-T1–F-T14）+ 完美线缺口 G1–G12（§17） |
| 2026-09-16 | 遗漏追补：已验代码 D1–D10（§18，RasterExtra/共享层写/直接画/队列安全/完成回执/atlas/上传/握手/异步语义/hops 表） |
| 2026-09-16 | 第二轮抠：D11 活 race 实锤/D12 旗子两边写/D13 收回等满16ms/D14 budget 裸数/D15 图片引用/D16 字体亲和/T2 变小/门禁改数完成数（§18b） |
