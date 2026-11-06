批 6 收官后补 · H21 修复账：场景回放进显示表（2026-11-06，人工肉眼暴露）
====================================================================================

一、现象与复现（全部实测）
--------------------------
1. 用户裸跑 `go run ./examples/ui_wr_p37_scene`：窗口只有文字、六面板全空；
   `ui_render_base_scene` **A 卡（GPU scene）无图形、B/C/D 三卡有图形**——
   与「A 卡走 live-only 回放、B/C/D 走离屏位图+贴图」的分岔逐卡吻合。
2. 本机计数复现：门禁相 `presents=240 skips=239`、常驻相 `presents=358
   skips=357`——每帧一笔「scene 回放被录制腿跳过」；跳过的正是呈现帧的合成路。
3. 诊断弯路如实记：先按「切 FullPaint 每帧 live 走查」试过（照
   ui_render_base_scene:148 口径）——`skips=480≈2×presents`，FullPaint 的每帧
   也是 `recordFullFrame`（Recorder 录制→raster 重放），live 分支
   `presentTreeOpts` 在两种政策下都轮不到 ⇒ FullPaint 救不了，回退该实验行。
4. 呈现链取证（`ui/embedder/pipeline_app.go:1549/1626-1710`）：
   `packetPath = useRetained && !inFullRecovery`；retained 走
   `presentPacketTextured`、FullPaint 走 `recordFullFrame +
   presentRecordedFrame`——两条路的画面都来自**显示表重放**，显示表里没有
   场景回放这一项（recording 契约 39 方法零个 scene 动词）。

二、标准依据
------------
- **Skia**：`SkCanvas::drawPicture(SkPicture*)`——一份录制作为另一份录制的
  可重放项；
- **Flutter**：`DisplayListBuilder::drawDisplayList(sk_sp<DisplayList>)`
  （`dl_builder.h:108`）同形；
- 仓内先例两条：批 4 PathEffect「录制期取值、回放只读」；批 3e-3「Backend
  契约尾参全量切换 + CanvasBackend 可选接口断言」。本修不新造第三套形态。

三、改动清单（引擎层，接口全量切换制同笔）
------------------------------------------
- `render/recording/scene_replay.go`（新）：`ScenePayload` 协议
  （`RenderSceneInto(*render.Context) error`）、`SceneReplayCommand`、
  `Recorder.RenderScene`（void，与画布动词全家族一致）、
  `CanvasBackend.RenderScene`（live 目标 Push/Pop 包裹执行；`*Recorder` 目标
  折叠嵌套；其余目标 `ErrUnsupportedSceneTarget` 具名错，绝不静默）；
- `command.go`：+`CmdSceneReplay`（enum+名字表）；`backend.go`：`Backend`
  +`RenderScene` 方法（契约 39→**40**）；`recorder.go`：Playback 分发 case
  （错误即返，不吞）；
- 实现者同笔：`CanvasBackend`（执行/折叠，45→**46**）、raster 经内嵌自动继承、
  `backend_stub_test.go`、`p37_3e3_material_test.go` 桩（嵌入继承）、
  `examples/ui_render_base_recording` 探针后端（计数留观察面）；
- `render/scene/scene_record.go`（新）：`(*Scene).RenderSceneInto`（就是既有
  `GPUSceneRenderer.RenderScene` 的薄载体，零第二实现）、`RenderSceneTo(dc
  render.Canvas, s *Scene)`（`*render.Context` 直画 / `*recording.Recorder`
  录制 / 其余 `ErrSceneReplayUnsupported` 具名拒）、`var _
  recording.ScenePayload = (*Scene)(nil)`；依赖方向 scene→recording，实测
  成环零（recording 不引 scene）；
- 计数（go/ast 实测）：recording types 86（+2：ScenePayload/SceneReplayCommand）
  consts 65（+1）vars 2（+1）`Recorder` 139（+1）`Backend` 40（+1）
  `CanvasBackend` 46（+1）；scene 顶层函数 +1（`RenderSceneTo`）、var +1、
  `Scene` 方法 41→42。目录 §6.2/§6.3 随迁，apidoc rc=0。

四、钉与负向（先红后绿；红形态=目标态测试编译断裂「RenderScene undefined」族，
批 4 先例同款）
----------------------------------------------------------------
新钉 6 枚：`render/recording/p37_6_scene_replay_test.go`（录制携命令 /
Playback 分发顺序 2 枚）+ `render/scene/p37_6_scene_record_test.go`（显示表路
≡ live 路逐位+防空转 / 外层矩阵作用到回放 / 回放后状态零泄漏 / 不支持 Canvas
具名拒 4 枚）——**native 与 `GPUI_BACKEND=go` 两后端全绿**。
负向三条（改码→跑→还原→复绿，脚本化留痕）：
- N1 摘 Playback 分发 case ⇒ 分发钉红（`场景回放分发=[]`）+ scene 逐位钉红；
- N2 摘 RenderSceneTo 的 Recorder 分支 ⇒ 具名拒错误打到三枚 scene 钉（红形态正确）；
- N3 摘 CanvasBackend 的 Push/Pop 包裹 ⇒ **不红，如实记**：GPUSceneRenderer
  自持 baseMatrix/baseProjection/currentMaterial 显式复位（3e-3 纪律），
  Push/Pop 是第二层保险——按 N6/N12b 先例记「不断言的防护」，不冒充已咬。

五、窗侧随迁与复跑
------------------
- `examples/ui_wr_p37_scene`：RequireLiveContext 分支删除，六面板一律
  `scene.RenderSceneTo(pc.DC, s)`；诊断期加的 `SetPresentPolicy(FullPaint)`
  行随修复回退（录制路已能携带场景，retained 照常）；skips 门收回 `==0` 判红。
  复跑：native `probes=15/15 golden=0.0000% skips=0`、go 同数全绿、常驻
  `355 presents skips=0 golden=0.0000%`（金图对既有基线逐位 0——快照回读重画
  的画面与显示表路逐位同，由钉①背书）。
- `examples/ui_render_base_scene` A 卡同款换臂。native 复跑 0.0000%（21/21
  logic + 17/17 point + ink 1/1）；**go 轮对既有基线 0.0032%（≈20px）——
  干净 HEAD worktree A/B 同命令**同值**复现（未修态也一样），定性为存量
  「双后端共用一枚 native 播种基线」工程债、非本修引入；该窗 go 端基线需
  单锁一枚 `_go` 或裁掉差区，登记为该窗欠账（下一步单独处置）。

六、方法论补账（这是关键的一条）
--------------------------------
⑥ 的快照金图门当时全绿而肉眼空白：`SnapshotPath` 走「回读重画」live 路，
天生带场景内容——**它验的是现场，不是呈现帧**。教训落两条硬则：
- 凡含 live-only/设备直连绘制的窗，**人工肉眼过呈现帧是收口的必过门**，
  快照与探针不能代替（AGENTS「真窗自测+人工二合一」的这半第一次真抓到了东西）；
- `live_only_skips≈presents` 这种「每帧都跳过关键绘制」的计数形态本身就是
  告警信号，不得记成「合法计数、只报数」。

七、门禁数字（2026-11-06 实跑）
--------------------------------
- `CGO_ENABLED=1 -race`：`render/recording` ok 2.104s、`render/scene` ok 3.858s、
  `render/recording/backends/raster` ok 1.536s；
- 逐文件：recording 19 文件 0 红、scene 31 文件 0 红（tools/per-file-tests.sh，
  输出 tmp/per-file-h21-rec|sc，summary 关键数誊此）；
- 整包 `CGO_ENABLED=1 go test -race ./render`：**DATA RACE=0、FAILS=18**——
  逐名与 H20 轮 18 枚账全等（9 在册 + drawShadow 存量顺序耦合 + S3c×7 + R72），
  收官轮那次 +1 的 `TestPresentResize_FullRecoveryWritesEveryBuffer` 本轮回绿
  （负载浮动族，不计收益）⇒ 本修对整包面零新增红；
- 全仓 `go build ./...` rc=0、gofmt 触及净、apidoc rc=0（目录 §6.2/§6.3 随迁）；
- 窗复跑：⑥ native/go/常驻全绿 skips=0、base_scene native 21/21+17/17+0、
  go 端 0.0032% 与 HEAD worktree A/B 同值（存量登记见五）。

八、H21 台账登记
----------------
§7 追加修复行；§2.2 待办池不新建行（本修即销案）；`window-6-scene.md` 与本件
互指——⑥ 的像素收口**维持 ✅ 不变**（六臂判据、逐位对拍、H15/H16 全成立），
本修改变的是「呈现帧到达路径」，判据数字未变（两基线复核仍 0.0000%）。
