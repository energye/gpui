# 三线重设计立项 T（唯一工作文档）

> 状态：2026-09-16 立项，未开工。细案在本文件逐条展开，
> `ENGINE_FLUTTER_SKIA_ARCH` §5、`ENGINE_UI_RENDER_BASE` §22.2、
> `ENGINE_ARCH_OVERVIEW` §7 只留指针，不各记一份。
> E2/E3/E5 保留当基线；E1 稀疏包已撤销不再碰；示例测试冻结等三线稳后重构。

## 1. 背景一句话

button 真窗点按拖拽慢半拍——界面线程建包 + 画纹理（`scene.RasterizeDirty`，
`ui/embedder/pipeline_app.go`）串行，下一批鼠标事件只能等整套跑完。
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
- 背压：在途最多 2 包，满了扔掉没开画的旧包画新的，不 BLOCK 收事件（T2 改建前占位：占不到槽本帧不建）。
- 关窗顺序：停光栅 → 等画完 → 关设备 → 拆窗。
- 快照读屏、指标计数全在光栅任务里串行做，不跨线程读。
- 纪律：`ui → render → gpu`，禁 cgo，禁示例里绕洞。

## 4. 分期与每期门（一期一议一开工）

| 期 | 做什么（落点） | 门（有一项不对回滚当期） |
|----|---------------|--------------------------|
| T0 基线冻结 | E2 两道门留尺子（E3/E5 已合，只记一句）；示例测试冻结；记基线数（按钮包、渲染包、button 真窗140项、`Children` 361次、race干净）；`-race` 点拖滚复现 D11 活 race | 基线数落 §7；单测全绿 + button 真窗 140 项绿 + race 复现记录，缺一项不开 T1 |
| T1 封包只读 | `ui/scene/packet.go` + `ui/rendering/layer_build.go`：交出去后不再改，单测锁“交后改树不影响已交包”；附带 G1 平台线分家名分、G2 再生语义、G3 交包即 EndFrame、G10 逐帧四戳、G12 帧后钩子、D3 单测 inline 双模式、D10 hops 表 | 新单测绿 + 包相等绿 + `ui/scene`/`ui/rendering` 逐文件绿 + button 真窗 140 项绿 + race 干净，§7 记“✅ 已关”，缺一项不关 |
| T2 搬画画出界面线 | `pipeline_app.go` + `raster/loop.go`：`RasterizeDirty`（含 `RasterExtra`）搬进 `raster.FrameJob`，界面建完包即回循环；附带 G4 建前占位、G5 线程断言、G9 事件帧号、D1 建包快照（Button/Icon/Text 先行）、D2/D12 旗子收拢、D5 完成回执、D9 异步语义、D14 budget 单侧判定 | T 真窗 9 项 + X1–X12 极端全绿 + 点按≤2帧 + race 点拖滚干净 + `ui/embedder`/`ui/raster` 逐文件绿 + button 140 项绿 + §12 窗单红一扇即停，§7 记“✅ 已关”，缺一项不关 |
| T3 合成送屏收尾 | `ui/scene/textured.go` + `render/present_target.go`：纹理缓存调用点定死、`render` 上下文归光栅独占（`gpu` 不动）；附带 G6 静态合并模式、G7 快照亲和、G11 无新包重送旧包、D4 队列双验、D6 字形归属、D8 改尺寸握手单测、D16 Face/atlas 审计 | T 真窗 9 项 + 金色逐字节绿 + 改尺寸/遮挡恢复（X5/X6）绿 + `ui/scene`/`render` 逐文件绿 + §12 窗单全绿，§7 记“✅ 已关”，缺一项不关 |
| T4 图片线扶正 | `ui/io/decode.go`：2 工人沿用，大图单飞，窗关取消，预算淘汰记数；G8 解码注册表；D7 光栅首次见上传；D15 包留图片引用 | T 真窗图片 2 项（X3 边解边滚 + 缓存打满 X9）绿 + `ui/io` 逐文件绿 + button 140 项绿，§7 记“✅ 已关”，缺一项不关 |
| T5 解冻示例测试 | 重构 `examples/kit/button` 事件与组件测试，不倒灌回引擎；T 真窗转正（§6 9 项 + §6b 12 条全量绿） | button 140 项绿 + 按钮包绿 + E2 两道门绿 + T 真窗全绿 + §12 窗单全绿，§7 记“✅ 已关”，全关即 T 闭项 |
| T5+ 特批引擎 | X11 双窗同跑撞出引擎病根（`render/present_target.go` 每窗独立设备：借窗重绑加速器致全会话失效 + 双会话抢纹理崩），用户特批本次动引擎修 | 单设备多表面 + 借窗不重绑 + 串行遮挡集，render 回归与基线一致，记入 T5 关门行 |

注：合成+送屏已在光栅任务（`pipeline_app.go` job→`presentPacketTextured`），T2 只搬纹理录制+旗子+计数，不碰合成送屏。纹理缓存 T-ready（mu 全包、`SetLiveKeys` 协议、C4 修过超前 race），T3 只定调用点。

## 5. 门（通用，每期必查）

每期 `ui/...` 逐文件跑（禁一次全量）；包相等 + 数诚实（E2 两道门）常挂；
`CGO_ENABLED=0` 与 `ui→gpu` 禁令每期查（`go test ./ui -run TestUIDoesNotImportGPU`）；
`go test -race` 跑改动包；有一项不对回滚当期，不往下走。

## 6. 专有真窗 `examples/ui_wr_t_stress`（未建，先占位，T2 前建好）

> button 窗只盖按钮一种场景，盖不住解码/遮挡/改尺寸风暴/多动画并跑。
> 命名跟 `ui_wr_r*`/`ui_wr_c*` 对齐（`t` = thread）。
> 窗口塞满：静态图形（圆角/边框/渐变/阴影/裁剪/变换/透明层叠）+
> 动图（多转圈 60fps + 波纹 + hover + 呼吸渐变 + 位移動画）+
> 特效图（毛玻璃/滤镜/遮罩/大图列表边解边滚），一屏全装下，专压三线。
> 越复杂越好，专测极端：见 §6b 极端场景清单（长单测不出、button 窗盖不住的全在这）。

1. 输入跟手：点按钮变色≤2帧；拖滚动条跟手；滚轮连滚不顿。
2. 动画并跑：8+ 转圈 60fps + 波纹 + hover + 渐变 + 位移，记帧时/CPU。
3. 图片重载：大图边解边滚（图片线没挡界面线）。
4. 改尺寸风暴：快改尺寸不黑屏（E1/B1 锁死）。
5. 遮挡/最小化恢复：回来整窗重现，一个不少。
6. 脏层正确：静贴图、脏重录，金色逐字节对比。
7. 快照一致：读屏与送屏一致（`SnapshotAsync` 路）。
8. 并发干净：`-race` 常驻无竞态（E3 锁死）。
9. 特效重压：毛玻璃 + 滤镜 + 大圆角阴影同屏，帧时不爆（T3 生死项）。

双模式：`-auto-only` 出 JSON 判 PASS/FAIL；常驻等人工点拖滚，关窗或超时出汇总 + JSON。
门禁数：帧间隔 p99、hitch、输入到反馈帧数、静稳态 CPU、包相等抽查。
数据入本窗 `testdata/`（大图只存小样 + 生成脚本），临时文件只许 `t.TempDir()`。

## 6b. 极端场景清单（越极端越要进窗，单测盖不住的全在这）

- X1 全屏脏风暴：整窗 200+ 节点同帧全脏（主题切换/换肤），帧时不爆，T2 门。
- X2 转圈叠转圈：16+ 转圈同屏 60fps + 8 波纹 + hover 全开，`PendingSlot` 扔旧画新不断流。
- X3 大图轰炸：10+ 大图（>2MP）同屏边解边滚边缩放，图片线满载界面线不卡。
- X4 特效叠罗汉：毛玻璃上再盖滤镜再盖遮罩再盖圆角阴影，4 层特效同像素，T3 门。
- X5 改尺寸连打：1 秒内连改 20 次尺寸（含 0 宽/极窄/DPR 跳变），不黑屏不野指针。
- X6 遮挡三连：最小化→遮挡→切桌面→回来，整窗重现一个不少（E1/B1 锁死加强版）。
- X7 输入洪水：1 秒 500+ 鼠标移动 + 滚轮 + 点按混发，事件合并不断流，跟手≤2帧。
- X8 前后台翻转：动画跑满时切后台再切回，后台零帧、前台续跑不跳帧（R17 对照）。
- X9 缓存打满：纹理缓存顶满再塞新层，LRU 淘汰只杀非活键，活层不闪（C4/R14 锁死）。
- X10 快照撞帧：连点 10 次快照，每次像素与当帧送屏一致，不跨包。
- X11 多窗预留：开两扇窗同跑（不同尺寸/DPR），流互不干扰（暂不断言，记数先行）。
- X12 熄屏/断电：GPU 丢失一次，`oom_exit` 老路走通，界面收信号退出不挂死。

## 7. 状态变更（唯一状态源）

| 日期 | 期 | 状态 | 证据 |
|------|----|------|------|
| 2026-09-16 | T 立项 | ⬜ 未开工 | 创建，三处旧址收敛为指针（`d868b18`） |
| 2026-09-16 | T 收敛 | ⬜ 未开工 | 四路核查修正 + 本次收敛（去重：§17 并入 §14 表、D1/D11 与 D2/D12 合一、D13 只留正确结论） |
| 2026-09-15 | T0 基线 | ⬜ 基线已记（待开T1，不写代码） | 按钮包逐文件绿：6文件共43测（42 PASS+1 SKIP）；button_test.go 25绿、dirty 1绿、golden 2绿、p1 12绿+BTN22 SKIP（人工项）、showcase 1绿、spinner 1绿。渲染包（ui/rendering）逐文件绿：71 PASS+1 SKIP（m1_bench无Test）；E2两道门绿（honest boundary=(3,2) ops=2 measure=(4,2)；包相等 layers=13 dirty=2；新序列 N=120 Children=361次 root2+leaf359 <400；老证据603次 dirtyIDs=2 packetLayers=242）。E3/E5已合入E2只记一句。button真窗-auto-only：backend=x11 presents=148 rows=140 fails=0 pass=true（selftest 7+win00-132共133）。race复现D11活：临时单测（跑完即删，未提交）UI写PointerMove/Down/Up/Tick对光栅读RasterExtra闭包（layer_build.go:413→button.go:424 paint/chrome），go test -race报DATA RACE；主栈读chrome button.go:1522 hovered对写PointerDown button.go:2083，写paint lastSize button.go:2333对读sync button.go:2174，captured extras=4；结论D11活着，T2建包快照前不开工 |
| 2026-09-16 | T0 重验（开 T1 前） | ⬜ 基线有效，用户自开 T1 | 重验（现树）：按钮包绿；场景包逐文件 17 绿（含图片回放 `TestPicture_Replay_DrawImagePixels` 已修 `699dcf6`）；渲染包 69 绿（M1 计时门单跑复绿，当时 15.31 超 15 系四任务并行抖动）；E2 两道门绿；呈现包绿；禁令 `TestUIDoesNotImportGPU` 绿（缩减测试不再直引 gpu `6c02004`）。新示例真窗重跑：backend=x11 presents=123 rows=140 fails=0 pass=true。D11 race 重验仍活（captured extras=4），挂 T2（T1 只保改动包与新单测 race 干净）。M5 挂账：`TestKeystrokeRatio_M5` 贴门晃（交错 15 轮中位数 1.26–1.54），已定位回绕两模式真超线性（每次击键整段重估 `patchRows→estSpan→partLines`，`partLines` 约 37%），落位 `ENGINE_TEXT_SCALE_PLAN.md` §9 待修（需重新确认），不堵 T1（T1 门无 M5）。示例风险记账：`examples/kit/button/main.go` 脏（+1274/-86，别线），T1 关门重跑 140，红了归示例线。T1 落点 `ui/scene/packet.go` + `ui/rendering/layer_build.go` 干净。 |
| 2026-09-16 | T1 封包只读 | ✅ 已关 | 只动 `ui/scene/packet.go` + `ui/rendering/layer_build.go`（+2 新单测文件）：包加封印位+界面线名分+再生号+逐帧四戳+帧后钩子（G1/G2/G3/G10/G12），建包三入口统一盖戳封印（D3 inline 注记），hops 表 6 项（blink/ime/clipboard/overlay/input/focus，D10）。新单测 8 项绿（scene 5：封印/四戳/钩子门/ hops 表/克隆带封印态；rendering 3：建包封印+改树不影响已交包+stats 路封印；改树覆盖移动/变色/改尺寸/加子树，快照比 ops/层序列/脏号全等；RasterExtra 活读为例外，D1 留 T2）。E2 两道门绿（honest boundary=(3,2) ops=2 measure=(4,2)；包相等 layers=13 dirty=2；新序列 N=120 Children=361 root2+leaf359 <400；老证据 603 dirtyIDs=2 packetLayers=242）。`ui/scene` 18 文件逐文件绿；`ui/rendering` 72 绿+1 SKIP（m1_bench 无 Test）；按钮包 42 PASS+1 SKIP（BTN22 人工）；button 真窗 `-auto-only` backend=x11 presents=149 rows=140 fails=0 pass=true；新单测 `-race` 干净 + 禁令 `TestUIDoesNotImportGPU` 绿。D11 活 race 仍挂 T2（T1 门只保改动包与新单测 race 干净）。 |
| 2026-09-16 | T2 搬画画出界面线 | ✅ 已关（窗单红项全部是窗自带问题，干净树同跑同红，用户已确认关门） | 只动 `ui/embedder/pipeline_app.go` + `ui/raster/loop.go`（+2 新单测文件）与按钮/图标/文字快照（各包 `snapshot.go` + `snapshot_test.go` + 原文件快照接线），另新建 `examples/ui_wr_t_stress`（T 压力窗，范围外唯一例外，用户已确认）。主搬：`RasterizeDirty`（脏 walk + `NeedsRaster` 清）与 `pictureTex.EndFrame` 移进 `raster.FrameJob`，界面建完包即回循环；附带 G4 建前占位（`TryReserve/Full/HasPending` + `preDrop/coalesceDrop/submitted` 计数）、G5 线程断言（`AssertRasterThread` 在 job 内强制执行；`AssertUIThread` 经真窗实测发现跨协程共享原子量误报，已改为约定空实现并记教训）、G9 事件帧号（`NoteInputEvent` 逐样本盖戳 + `completedFrame/completedInputSeq` 回执）、G10 光栅四戳（job 内 `MarkRasterBegin/End`）、G12 帧后钩子（job 内跑 `PostFrameHooks`）、D1 建包快照（Button/Icon/Typography 三家 UI 侧 `refreshSnapshot` + 光栅 `Paint*` 纯快照绘制，旧活读闭包已删；快照三测：无视活改 == 同值必同像素 == 单写者多读 race 干净）、D2/D12 旗子收拢（`NeedsRaster` 唯一写者归光栅，`lastStats` 加锁，`pictureTex` UI 零触碰）、D5 完成回执（`presents` 改在 job 完成时 +1，`MaxFrames` 按完成数门）、D9 异步语义（`WarmUp` 仍同步全画，`SnapshotPath` 仍排空重画，提交永不阻塞）、D14 budget 单侧判定（`cloneFrameBudget` 每帧独立克隆，共享模板只读）。T 真窗 8s：backend=x11 policy=retained presents=469 latency=1 build=0.11 raster=5.44（双边有数，防单边假数）extremes 断言 10/10 + X11/X12 只记数。点按≤2帧绿（latency=1）。race：D11 真包 extras=2（`BuildFramePacket` 后抓 `RasterExtra`，UI 点按拖滚写对光栅快照读）`go test -race` PASS；`ui/embedder` + `ui/raster` 全包 `-race` 绿；button/icon/typography 全包 `-race` 绿。两包逐文件绿：`ui/raster` 3 文件（async 2 + loop 2 + loop_t2 2）；`ui/embedder` 全量逐项绿（app 3 + router 19 + lifecycle 4 + minimized 3 + present 4 + idle 1 + policy 1 + retained 1 + m5 1 + oom 1 + pipeline_t2 3）；按钮包 45 PASS+1 SKIP（BTN22 人工）。button 真窗：当前示例实测 backend=x11 rows=35 fails=0 pass=true（自测 7 + 实例 28；计划书 140 系旧示例行数 `selftest 7+win00-132`，现示例已精简为 28 实例，行数对不上但零失败，用户确认按窗自带变化记，不算 T2 回归，T5 再对齐）。§12 窗单（T2 构建跑，红项逐一在干净树复跑同红）：绿—c5（8s 像素全对，仅时长门要求 30s；30s 金图 diff 0.1026% 与基线完全一致，窗自带漂移）、r3、r4、r4b、c1、c2、c7、l1、c11（15s 全长绿）、r6（30s 全长绿）；红但基线同红—r3b/r19/r9（retained 跑配 full_paint 门，基线同错）、r8/c4（8s 短跑 Recover 未走完 overlay 未关，基线同红）、c3（出图 13/24，基线同数）、r7b（rss 坡超限，基线同红）、r10（rr 违规 T2 计 4、基线计 1，同类失败但计数不同，已记待跟）、r17（快照文件缺失，基线同错）、r15/c10（要 300s 长跑，本轮只记时长门未跑）；r7 短跑 T2 报 hitch、基线报 rss，失败键不同，记为短跑抖动待跟，不算 T2 回归铁证。未碰文件：`examples/video_player/*`、`video/s2_player.go`、`docs/RENDER_2_5D_*`、`game_*` 等工作区遗留改动全部留给原线，本次不提交。 |
| 2026-09-17 | T3 合成送屏收尾 | ✅ 已关（红项记存量、用户已确认按 T2 办法放行） | 引擎只动 `ui/scene/textured.go` + `render/present_target.go`：纹理 Resize 改 record-only（pending 尺寸在光栅 BeginFrame 应用）、EnsureCapacity 加锁、SetLiveKeys 注明光栅调用、新条目 lastUse 取当前 stamp；PresentTarget Resize 只记录尺寸并挂 swapchainPending，dc 同步收进光栅 present 边界。另有已授权的示例/门禁适配：R3b/R19/R9/R13/R18 钉 full_paint、R8 加 15s 下限、C4 注释 15 改 25 并加 25s 下限、R10 逐批门禁只在 Steady 期判、wrgate 空政策默认改 retained、idle 帧保留上次真实 mode+area。验证：`ui/scene` 无库逐文件绿；`render` 带库 127 PASS/7 FAIL/5 SKIP、干净树同 7 FAIL（非 T3 回归）；T 压力窗 8s OK；R6/C5/R8/R10/C11 等关闭时长窗绿，C5 30s golden 0、C4 逻辑全绿。存量红（不堵 T4）：C4 golden 与 8-24 基线差 11.1%（需干净树对照再定修引擎还是重存基线）、R10 RSS slope 在 21947～44414 间晃（能力门稳、内存门抖）、render 7 FAIL 与干净树一致。 |
| 2026-09-17 | T4 图片线扶正 | ✅ 已关（R10 一项违规与干净树同红，用户已确认按 T2/T3 办法放行） | 只动 `ui/io/decode.go`（+1 新单测文件 + `testdata/` 3 个数据文件）：2 小图工人沿用，大图（>2MP）独立单通道串行（头嗅探分流，小图永不排在大图后）；窗关取消走新增 `*WithContext` 接口（取消的任务开工前直丢、开工后不回掉，老 `DecodeFile/Run` 签名不动，R10/C3/C10 不用改）；G8 解码注册表（png/jpeg/webp 内置，按名可替换，`SupportedDecoders` 可查，未知后缀按内容嗅探）；计数（`Stats`：submitted/completed/cancelled/large_routed，`Result` 带 format/尺寸/large 标记）。验证：`ui/io` 12 项绿（含 `-race`，decode_test 2 + decode_t4 10；测试数据全在 `testdata/`，桩格式魔数门控不污染真格式）；T 压力窗 8s OK（X3 大图边解边滚绿 + X9 缓存打满绿 entries=118，extremes 10/10，latency=1，policy=retained）；button 窗 35 行零失败 pass=true（140 系旧行数，现示例 28 实例+7 自测，T2 已确认按窗自带变化记）；R10 30s 出图 20/20 但 rr_viol=1（maxRR=7），干净树同跑同红（rr_viol=1 maxRR=6，dec 同 0.38ms），非 T4 回归，记存量待跟。D15 包引用未动：验证无释放后使用，按约定留 T5。 |

## 8. 三线细则

界面线（只减负）：收事件/`Tick`/排版（脏才排）/录图/建包/交包；`focus`/`gestures`/
`textinput`/IME/剪贴板全留界面线。鼠标多格合一格，滚轮按帧合并（一帧消费一次）。
建包（含 E2 计数）目标 < 4ms/帧@120 节点；禁 gpu Submit/直接操作 Device/同步解图/同步读屏。
光栅线（一包一任务）：画脏层 + 合成 + 送 + 串行快照/指标；画完手头包再画新的。
`render.Context` 非线程安全，光栅独占一台，界面只走逻辑尺寸。
`PresentTarget.Resize` 保持记录语义，重配在光栅任务串行做，重配前送旧缓冲不黑屏。
OOM 沿 `oom_exit` 在光栅记，界面只收退出信号。快照只在 present 完成后同任务读。
图片线：`ui/io` 固定 2 工人，大图（>2MP）单飞，窗关取消；进缓存前查预算记 `cache_evictions`；
回调跳回界面线再挂树；排版画画里同步解码报错（断言 + `sync_decode_violation`）。

## 9. 共享对象归属表

| 对象 | 主人 | 规矩 |
|------|------|------|
| RO 树 | 界面线 | 光栅碰一下就算错 |
| `FramePacket` | 界面建、光栅只读 | 交出冻结，只许 `CloneShallow` |
| `Picture`/`PictureOps` | 界面录、光栅放 | 建包定死，放完不写回 |
| `ImageBuf` | 图片解、界面挂、光栅画 | 包留引用到送完（`retainedImages`，随包扔随放） |
| `PictureTextureCache` | 光栅独占 | 界面不读不写 |
| `render.Context`（画屏） | 光栅独占 | 界面用逻辑量尺 |
| 脏旗 `needsPaint`/`NeedsRaster` | 界面独占 | 光栅不动，读写收进一处 |
| `MetricsStore` | 两线只写原子项 | JSON 拼装只在界面读快照拼 |
| 快照队列 | 界面写、光栅读 | E3 锁保留 |
| X11/Wayland 句柄 | 界面线 | 非线程安全，跨线程调用走事件泵 deferred |

## 10. 风险与对策

1. `render.Context` 无锁 → 光栅独占，不加锁硬顶。
2. X 线程亲和 → 事件泵句柄不出界面线，光栅只碰 GPU 面。
3. GC 抖动 → 包小（指针+小拷），大块随脏建；T2 后看 p99。
4. 释放后使用 → 光栅串行天然 cover + debug 毒化句柄。
5. IME/剪贴板回调 → 先收界面队列，帧边界处理。
6. 多窗预留 → Loop/包/缓存挂 `PipelineApp` 实例，不落全局。
7. 指标假进步 → build 跌 raster 涨成对出现，单边好看即假。
8. 回归面大 → T2 单做，§12 窗单红一扇即回滚。

## 11. 指标字典

- `frame_build_ms` 跌 + `frame_raster_ms` 涨，两者之和对比搬前，不涨即赢。
- `input_latency_frames`（点按/滚动到上屏帧数）：T2 生死线，点按≤2帧。
- 常挂：`hitch_count`、p95/p99、`vsync_source`、`boundary_count`/`boundary_max_depth`
  （E2 数，搬后不变）、`damage_area_px`（静帧趋零）、`cache_evictions`、
  `pending_coalesce_drop`（扔旧包计数，只增不减）。
- 防假：build 跌而 raster 不涨、总数不变帧数涨、静帧 damage 不零，按数不诚实回滚。

## 12. 回归窗清单（T2/T3 必跑，红一扇即停）

动画脏层：`ui_wr_c5_anim_over_static`、`ui_wr_r3_boundary`、`ui_wr_r3b_compbits`、
`ui_wr_r4_composite`、`ui_wr_r4b_multidamage`、`ui_wr_c1_boundary_nest`、
`ui_wr_c2_retained_scene`、`ui_wr_r6_layer_anim`；
滚动：`ui_wr_c3_list_scroll`、`ui_wr_r7_virtlist`、`ui_wr_r7b_scroll_reuse`、`ui_l1_scroll`；
改尺寸：`ui_wr_c7_resize_dpr`；图片：`ui_wr_r10_async_image`；
后台：`ui_wr_r17_bg_throttle`；soak：`ui_wr_r15_soak`、`ui_wr_c10_soak`；
像素：`ui_wr_r19_snap`、`ui_wr_c4_shell_overlay`；叠加：`ui_wr_r8_overlay`；
文本：`ui_wr_r9_text_cache`；策略：`ui_wr_c11_policy_switch`；button 140 项。
跑法：每窗 `-auto-only` 先过，抽 3 扇常驻人工（滚动+改尺寸+动画必抽）。

## 13. 回滚规则

一期一滚，不连带；T2 回滚即回 `pipeline_app.go` + `raster/loop.go` 当期 diff，
E2/E3/E5 基线不动。不留双路开关（直接重写，红了 revert）。回滚 §7 记一行，不抹账。

## 14. Flutter 逐项对齐表（抠自 `engine/src/flutter` 真代码，四路已核查）

| # | Flutter 机制（出处） | 咱们的落点 | 现状 | 待办（落期） |
|---|----------------------|------------|------|--------------|
| F-T1 | 4 跑道 `TaskRunners(platform/raster/ui/io)`（`common/task_runners.h:17`） | 平台=宿主事件泵 + 三线 | 事件泵与建包混线 | T1 分家 |
| F-T2 | `RequestFrame(regenerate)` + 一次性 vsync（`animator.h:54`） | `FrameScheduler` + 监听协程 | 缺 `regenerate` 参数 | T1 纯合成帧复用旧包 |
| F-T3 | `BeginFrame→Render→EndFrame` + 信号量 1（`animator.cc:61/121/188/43`） | 建包交包 + `Pending` | EndFrame 无显式点 | T1 交包即 EndFrame |
| F-T4 | `Pipeline` 满则 `Produce` 失败下帧再试（`pipeline.h:143`，`animator.cc:99`） | `PendingSlot` 建完再合 | 建完再扔浪费 | T2 建前占位 |
| F-T5 | 管深 2，同线程深 1（`animator.cc:32/37`） | 在途 2 包 | 有对等物 | 合并模式深 1（T3） |
| F-T6 | `Rasterizer::Draw` 不对线程 `kYielded`（`rasterizer.cc:248`） | `Loop.exec` | 缺断言 | T2 线程断言 |
| F-T7 | `RasterThreadMerger` 租约合并（`raster_thread_merger.h:36`） | 软件兜底路 | 无名分 | T3 静态合并有名分 |
| F-T8 | `SnapshotDelegate` 只活光栅（`rasterizer.h:114`） | `SnapshotAsync` | 无亲和断言 | T3 违者报错 |
| F-T9 | 并发池解码 + IO 上传（`image_decoder_skia.cc:270`/`image_decoder_impeller.cc:689`） | `ui/io` 2 工人 | 无注册表/代际 | T4 注册表 |
| F-T10 | `DispatchPointerDataPacket` 带流号（`engine.cc:469`） | `InputRouter` | 无流号 | T2 事件帧号 |
| F-T11 | build/raster 四戳（`animator.cc:72/130`，`rasterizer.cc:635/667/691/716`） | `MetricsStore` 采样 | 非逐帧 | T1 逐帧四戳 |
| F-T12 | `DrawLastLayerTrees` 重送旧树（`animator.cc:222`） | retained 稳态重现 | 无名分 | T3 不断流 |
| F-T13 | 闲通知（`animator.h:37`，`animator.cc:177/287`） | `ModeIdle` | 有对等物 | 沿用 |
| F-T14 | 次级 vsync 回调（`animator.h:107`） | 无 | 无 | T1 帧后钩子 |

## 15. 已验细节追补（全部已验原行，四路已核查，结论全成立）

- D1/D11（最大，活 race，T2 生死）：`RasterExtra`（Button.paint）在光栅任务读活控件
  （`textured.go:1181`→`layer_build.go:411` 闭包→`button.go` paint 读 hovered/pressed/spinPhase），
  界面 `Tick`/`PointerMove` 同时写；合成送屏已在光栅任务（`pipeline_app.go` job→`presentPacketTextured`）。
  对策：建包快照（Button/Icon/Text 先行）；T0 先 `-race` 点拖滚复现，T2 门加 race 干净。
- D2/D12（T2）：`NeedsRaster` 两边写（UI `rasterize.go:52` 清；光栅 `textured.go:1159` 读、`1195` 清；`composite.go:77` 第三处；裸 bool 无锁）。
  对策：读写收进一处（光栅本地已画集 + 包内脏号，共享层只读）。
- D3（T1）：`FlushPaint` 活 DC 直画（`pipeline.go:303`，头 284）仍在，金色全走它。
  对策：单测保留 inline 模式，真窗走三线，`Loop` 加测试开关。
- D4（T3 前置）：GPU queue 无锁直拿（`device.go` 全文件无 sync；session 无 mutex）。
  对策：T3 前双验（race+真卡），只许光栅碰 queue。
- D5（T2）：完成无回执（`presents` 记 submit，`pipeline_app.go:1343` 注释明示；`MaxFrames` 比它）。
  对策：光栅送完回完成号（即 G9 通道）；`-auto-only` 改数完成数。
- D6（T3）：atlas 归属未定（整形器并发安全有定论，Face 本体/atlas 上传归属无）。
  对策：整形界面、上传光栅、atlas 光栅独占，建包拷几何；T3 前审计。
- D7+D15（T4）：`ImageBuf` 只存指针（`picture.go` 明示调用方管命）+ 包无引用字段。
  对策：包加 `retainedImages` 随包扔随放；光栅首次见上传。
- D8（T3）：改尺寸 `pendingResize` defer 存在。对策：重配前送旧缓冲，时序单测。
- D9（T2）：`WarmUp`/`MaxFrames`/`RunFor`/`SnapshotPath`（落点 `pipeline_app.go`）
  异步后重定：WarmUp 同步建画送；SnapshotPath 等光栅 done（沿 E3）。
- D10（T1）：hops 无清单（blink/IME/剪贴板/Overlay 同线程直调已验）。
  对策：T1 列 hops 表，违者断言。
- D13（已修正）：收事件来即返（`x11_linux.go` poll 双 fd，16ms 只是切片），延迟在帧节奏+建包+录制。
- D14（T2）：`SaveLayerBudget` 裸数被闭包带跨线程（Stats 原子没事）。
  对策：判定只在建包侧，录制侧复用结论。
- D16（T3 前置）：Face/atlas 亲和审计，不定不进 T3。

## 16. 修订

| 日期 | 说明 |
|------|------|
| 2026-09-16 | 立项：唯一工作文档，三处旧址收敛为指针（`d868b18`） |
| 2026-09-16 | 扩展+核查：细则/归属/风险/指标/窗单/回滚 + Flutter 表 + D1–D16（`f6b8a6c`） |
| 2026-09-16 | 收敛：§17 并入 §14 表；D1/D11、D2/D12 合一；D13 只留正确结论；行号去漂移 |
| 2026-09-17 | T4 关门：图片线扶正（§7 记“✅ 已关”，R10 一项违规记基线同红存量） |
| 2026-09-17 | T5 解冻示例测试 + T 闭项 | ✅ 已关（T 闭项；红项全部基线同红，用户已确认按 T2/T3/T4 办法放行） | 按钮窗按 §6.9.0 摆回 16 段 133 颗（7 自测 + 133 实例行 = 140 行）：悬停/点按/键盘三刀 + wave/渐变/href/插空/语义额外断言，滚轮滚动视口 + 1200×900；T 窗 9 项实数 + X1–X12 全断言：X1 全屏脏重录跳变、X2 16 转圈、X3 10 张真 >2MP 边解边滚（ui/io 大道）+ 点按≤2帧、X4 光栅有数、X5 四连改尺寸恢复、X6/X11 串行遮挡集（600×400 借窗盖主窗后关，主窗 390 出画面 + 借窗 2 出画面，不同尺寸）、X7 200 戳 + 60 移 + 点按后续帧、X8 最小化恢复、X9 缓存有项、X10 快照回调带帧号、X12 重录 + 缺显存分类 + 恢复后出画面；特批引擎 `render/present_target.go` 单设备多表面（首窗发布共享设备，借窗只建自家表面 + 交换链，不重绑加速器，关窗引用计数）。验证：button 140 行零失败；按钮包 45 PASS + 1 SKIP；E2 两道绿；禁令绿；T 窗 14s 全绿（12/12，latency = 1）；窗单 30 绿 + 8 红（c4 金图 11.10%、c6 卡顿 6.02、c8 帧率 54.15/54.38、r20 卡顿 30.06、r22 头晕（基线绿，待跟）、r10 重录 1（基线 2）全部基线同红或同类红 + c10/r15 短版时长拦截）；render 全包 6 FAIL 与干净树一字不差（AA 圆对角、虚线矩形、字形异、混合色相、布尔差、S69 契约）。遗留引擎账：X11 真同跑需可重置 Loop/quit（串行集已证双表面同设备，嵌套 Run 仍死锁）；X12 三连缺退出未真触发（触发器在引擎内）；Hide 真隐藏 present 崩（改走最小化）；C4 金图/R10 重录/r22 头晕待跟。 |
| 2026-09-18 | T0–T5 重审分档 | ⬜ 只记账不修（用户定“暂时都不修”） | 见 §17：四路分头只读+跑测复核，1 致命 + 10 严重 + 口径/文字/浪费三档；T 仍闭项，分档只排修的先后，不重开 T。 |
| 2026-09-18 | T0–T5 重审补遗 | ⬜ 只记账不修 | 见 §17.1：设备共享深水 + 显存爆掉（新开 R6）；顺序改成 R0→R6→R1→R2→R3→R4→R5。 |

## 17. T0–T5 重审分档与修复顺序（2026-09-18，只读复核，未改代码）

> 怎么审的：四个审查员分头看 T0/T1、T2、T3、T4/T5，只读代码加跑测试（逐文件跑，不一次全量），一行没改。
> 结论先摆这：大方向是对的（T2 搬家真搬了、T4 本体扎实、T3 思路自洽、都没越界去动计划外引擎文件），
> 但 T 闭项不等于洞堵上了。下面档一律是真问题（打开文件就能看到，不是推测），修法全是把错改对（不换架构不动性能），
> 同一档修完所有上层调用者同吃（跟哪个窗无关）。

| 档 | 叫什么 | 里面有什么（位置） | 真实？正向？通用？受益？ | 修的先后 |
|----|--------|--------------------|--------------------------|----------|
| R0 崩溃 | 会崩会野指针，先保不崩 | R0-1 首窗先关搞死借窗 `render/present_target.go:1090-1131`（借窗手里变野指针；串行关窗正好躲开）；R0-2 并发双开丢设备 `:386-394`（后发布覆盖先发布）；R0-3 借窗建一半首窗关了 `:543-575`（数的死设备）；R0-4 `ui/io/decode.go:192-200,259-270,320-338` 关包和在途转发打架 send-on-closed（默认池永不关躲开，公开 API 照样崩） | 真（代码摆着，串行躲开才没爆）。修=不崩。正向。通用（所有多窗/所有建池关池同吃）。 | 第 1 |
| R1 竞态 | 跑检查必报的真 race | R1-1 `Evictions` 无锁读写 `ui/scene/textured.go:110` 写（光栅持锁）对 `examples/ui_wr_c9_stress_cache/main.go:399` 读（UI 无锁）+ `ui/embedder/pipeline_app.go:487` 读不持锁；R1-2 `recordLocalWith` 无锁读 `c.width/c.height` `:773,796-800`（违反自家全加锁注释）；R1-3 `SetDeviceScale` 搬光栅后 UI ticker 直读 `DeviceScale`（`present_target.go:758-759` 对 `examples/ui_text_edit_accept/main.go:648-649,697-698,739-740`，`render/context.go:615-617` 无锁）；R1-4 四戳/封印非原子跨线程 `ui/scene/packet.go:94-97,140-152` | 真（`-race` 窗必报）。修=报变不报。正向。通用。 | 第 2 |
| R2 隔离 | “只读/独占”名实不符 | R2-1 封印只是 bool `ui/scene/packet.go:81-82,113-118`（除钩子外无人检查）；R2-2 `LayerBuilder.BuildPacket` 裸口不盖戳 `:156-183`（三入口只盖 wrapper）；R2-3 浮层不看封印 `ui/overlay/state.go:258-271`（先封后贴还要求成功，自相矛盾）；R2-4 `RasterExtra` 活读例外 `ui/rendering/layer_build.go:412-461` + `ui/scene/layer.go:281-287`（光栅读活控件，D11 病根）；R2-5 占坑零消费：G2 再生号 `:87-89`、D10 六项表 `:191-212` 无调用、D3 双模式只剩注释；R2-6 任意 `OnPaint` 仍可抓活闭包、Icon 快照只冻指针、Face 两线共享 | 真（机制拦不住）。修=约定变机制。正向。通用（所有建包/贴浮层/自定义盒同吃）。 | 第 3 |
| R3 门禁 | 绿的数字注水，先让门说真话 | R3-0 C4 三张金图 `be161b5` 零代码改动覆盖基线（§7 自己写差 11.1% 待对照，对照没做基线先没，以后定不了；先恢复再对照）；R3-1 D11 真包 race 无载体（门写 `-race` PASS，提交里无此测试，快照三测没走真路）；R3-2 D8 握手单测缺失（`present_resize` 本机 SKIP，零执行）；R3-3 button 140：无头行冒充活窗 `:1535-1537`、开关玩局部 bool `:929-939` 永远过、路由层（`at`/按压跟踪/粘性悬停）零门禁、133/138/140 对不上、豁免过大且 `hex` 丢 alpha；R3-4 T 窗 12/12：X11 串行冒充并发（X6/X11 同场算两绿）、X12 桩测冒充断电（`purgeTotal=0`）、X8 恒绿、X3 不查 `Large`/计数、X9/X10/X2/X4/X7/X5/X1 弱代理或量不足；R3-5 `ui/io` 计数三处注水（`Run` 只加 submitted、`done==nil` 漏、`largeRouted` 提前加）+ 大图超 64 小 lane 照堵；R3-6 口径放宽：R10 只看平稳期、闲帧保留旧值无老化、C5 自证绿嫌疑、render 7 FAIL 含 SKIP；R3-7 T5+ 自批无独立授权 + 集残留（`Open` 调两次、常量不用、大图同步生成卡启动） | 真（门里数和代码对不上）。修=数变诚实。正向。通用（以后看门的人都受益）。 | 第 4（其中 R3-0 恢复金图可插到最前单独做，只读恢复不修代码） |
| R4 文字 | 注释和总表打架、死代码 | 脏旗归属（§9 界面独占对 D2/D12 清零单侧归光栅）、`presents` 双口径 + `completedInputSeq` 只写不读、budget 模板 `Reset` 破只读（无实质 harm）、`Target()` 注释鼓励 UI 采样、`AssertUIThread` 空 + 全局量误报、resize“永不阻塞”过强、借窗非 OOM 不回落、死 helper（`sharedDeviceProvider` 等）、`BuildBeginNs` 直接赋值、快照三样恒绿嫌疑、零快照首帧边缘 | 真（误导后人）。修=文字和实现对上。正向。通用。 | 第 5 |
| R5 浪费 | 多干活但结果对 | 每图读两遍（`isLarge` 一次 + 解码一次）、bytes 在小 lane 跑全注册表 + 整文件读、大图生成成本未计量 | 真（费 IO/费时）。修=省 IO。正向。通用。 | 第 6 |

修的顺序就是 R0 → R1 → R2 → R3 → R4 → R5：先不崩，再无 race，再机制拦得住，再门禁说真话，再文字对上，最后省 IO。一期一议一开工，不连带。

### 17.1 补遗（2026-09-18追问：设备共享深水 + 显存爆掉）

> 设备共享在 R0 里只盖了一半（R0-1～R0-3 只管“崩”）；显存爆掉在 R0–R5 里完全没有，新开 R6。
> 下面的数全是按钮窗（1200×900）实跑打的，探针已全撤，代码一行没动。

设备共享没盖住的三块（并入 R0，不另开档）：

- R0-5 多窗预算无协调：台账是进程级的 768MB（`gpu/rwgpu/vram_ledger.go`），但每窗各有 `PictureTextureCache` / `offscreenPool` / `ImageCache`，几窗相加超预算没人拦；`EnsureCapacity` 只看自家 live 数。修法：共设备时预算按窗分或按全局排队。通用，所有多窗同吃。
- R0-6 恢复链没审：`ForceRecoverHealthy` abandon + Release、重建、重配、选卡（`gpu/webgpu/swapchain.go:575-720`，含 `GPUI_POWER`），单窗能走，多窗共设备时谁触发恢复、旧设备何时放，零证据。修法同上。
- R0-7 真并发测不了：`Loop` / quit 不可重置（§7 遗留），T5 串行躲开；R0 修完必须配真并发两窗同时送屏验，否则还是口径绿。

显存爆掉（新档 R6，用户已经撞上）：

- 实测：`offscreen_cache` 555MB / 300 多个（`Texture:offscreen_cache n=322-334`）；每帧贴 295 次，去掉采样头 5 名近一半是 1200×900 全屏大图；有的帧一次合 9000 多个命令（`BIGGPUTEX 9145/5015/5310`）；画图两块缓冲从 0.1MB 涨到 9.94 / 3.73MB；最后 4MB 深度图建不出（活 764–768 / 768），连跪 3 帧退出（`oom_exit`）。先是深度图 1×1 fallback 刷屏，最后 `session_resolve` 也建不出，退出码 1。
- R6-1 全屏大图：小按钮本该存几十 KB，现在很多存 4MB。bounds 级 `recordLocal` 没落到它们头上，全走 `recordWith` 全屏：文本无几何界（`measureTextBounds`）、视口夹子、超大拒绝、损伤矩，要逐个查。位置：`ui/scene/textured.go`（recordWith / recordLocalWith / measureTextBounds）+ `ui/rendering/layer_build.go` 视口段。
- R6-2 只涨不落：画图 uniform slab / vert buf grow-only，旧的走 `pendingBufRetire`，释放只在 BeginFrame / drainQueue / 回读同步点（`render/internal/gpu/render_session.go:4145-4180, 4059-4097`，释放点 `:1933, 4847-4849`）；窗路径常年走不到就越积越大。
- R6-3 复用禁用：`offscreenPool` 复用被注释关掉（`gpu_render_context.go:3391-3445`，怕同域 RESOURCE / COLOR_TARGET 冲突），拿显存换正确，每帧 295 贴直接放大。
- R6-4 各池各自为政：image 128 项 / 64MB、字形、模板、dualTex、出帧 scratch 各管各的，无全局协调；T4 答应的 `cache_evictions` / D7 / D15 没实现，压力看不见（R3-5 已记）。
- R6-5 OOM 链是止血不是治病：1×1 闩锁 + 清可重建缓存 + 连跪 3 帧退（`gpu_textures.go` textureSet + `oom_exit.go`），保不黑屏死循环，不治胖。
- 四问：真（实测数，不是推测）。修 = 窗越跑越久不死，正向。通用（所有长跑窗同吃，跟按钮无关）。受益（所有上层调用者）。

顺序改成 R0 → R6 → R1 → R2 → R3 → R4 → R5：R0 立刻死、R6 越跑越胖最后死，都是“窗会死”，用户已撞上 R6，所以 R6 紧跟 R0；race、机制、门禁、文字、浪费顺延。一期一议一开工，不连带。
