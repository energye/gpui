# P3.8 批 6 交付：嵌套 Picture（DrawPicture）+ K5 穿链

- 日期：2026-11-07（交付即收口：真窗双后端各自基线金图逐位）
- 依据：assessment.md §4/D5/D6 + design.md 批 6 细化（三拍板：D6-1 上 Canvas 接口；D6-2 回放期栈护栏；批 5/6 两枚分批提交）

## 1 交付物

**嵌套 Picture（SkCanvas::drawPicture 同位物，H21 CmdSceneReplay 保持不动）**
- render/canvas.go：+`Picture` 接口（`DrawTo(dc *Context)`；接口倒置——render 不能 import recording，Recorder 侧实现）+ `Canvas.DrawPicture(pic Picture)`（全量切换，两实现者 *Context/*Recorder 同笔）
- render/context_picture.go（新）：`Context.DrawPicture`——Push/Pop 包夹 + **回放期身份栈**（pictureStack）环检测（`ErrPictureCycle`）+ 深度 64（`ErrPictureDepth`），fail-closed 跳过+计数（`PictureRefusals()` 可见口，Skia 不设防会栈溢出，本仓设防）
- recording/draw_picture.go（新）：`CmdDrawPicture` + `DrawPictureCommand{Pic render.Picture}` + `Recorder.DrawPicture`（nil 跳过）+ `Recorder.DrawTo`（Picture 实现：CanvasBackend 回放）+ `CanvasBackend.DrawPicture` 双目标（Recorder=引用命令折叠进外层、Context=Context.DrawPicture 直绘）+ `Backend` 契约 +方法 + `ErrUnsupportedPictureTarget`
- Playback 分发 case；bounds walker 不展开 payload（H21 同款登记）

**K5 穿链（GlyphEntry 10B→14B，进程内契约全量切换）**
- scene/encoding.go：GlyphEntry +`Cluster int32`、glyphEntrySize=14、EncodeText 写入
- scene/decoder.go：Text() 读回
- scene/text.go:505：写入点补 `Cluster: int32(g.Cluster)`
- 两腿重建：gpu_renderer.go resolveText、renderer.go renderTextOnTile 的 ShapedGlyph 重建补 Cluster
- **K3 shape 侧缺口随消费面接线补上**：shape_result_cache.go layoutGlyphsUncached 补 `Cluster: glyph.Index`（K3 三处之一——本批消费面 Scene.DrawText 的直接上游；ui/rendering/text_layout.go 两处兜底不在本链路，K3 其余维持「接线消费面前补全」口径在册）

## 2 语义澄清（测试期间钉死）

- **折叠=引用携带**：外层回放到 Recorder 时记 `DrawPictureCommand` 引用（与 H21 SceneReplayCommand 同款承载），**不展开**——展开发生在最终 Context 回放（行为级钉测：folded 流二次回放出 picture 内容 ✓）；评估 §4「折叠进外层」一词按此落实
- **Cluster 语义=RUNE index**（shaped.go 真源）：探针初版按 UTF-8 字节算期望值红测误报，按真源改 0/1/2 后绿
- CanvasBackend Begin/End 已有状态括弧（Push/Pop+brush 快照），pic 内不平衡 Push 被双包夹兜住（StateDoesNotLeak 钉测 ✓）

## 3 验证账

- 单测（先红后绿）：recording 8 组（折叠引用序列/直绘像素+CTM 组合/三层嵌套/环/深度 65/nil/状态不泄漏）；scene K5 往返钉（Cluster 0/1/2 逐值+尺寸 14B 钉随新契约更新）
- 真窗 `ui_wr_p38_picture` 四卡：卡1 一 picture 三处摆位（含 1.5× CTM 组合）卡2 三层嵌套 卡3 自引用拒绝牌（ErrPictureCycle counted 绿字）卡4 中英混排（wqy-microhei 单 face）；logic 6/6（折叠引用/直绘像素/嵌套/环/深度/K5 场景级 Cluster 往返）pixel 4/4；native + go/GL 各自基线 **0.0000%**（912640px）
- 门禁：build rc=0 / vet rc=0 / apidoc rc=0（catalog +§3.17 七符号）/ conformance PASS / -race scene+recording+svg+render 触及面绿
- 存量：ui_render_base_svg 金图 0.0000%（批 5 已验）；场景渲染像素不受 Cluster 携带影响（cluster 无渲染读者，纯数据面穿链）

## 4 已知边界与登记

- **全量 ./render/ 测试间干扰（存量，非本批引入）**：批 5 提交点 99a74660 worktree 干净树复测同集合红（TestS3c_M3_*/TestVideo*/TestMiscDraw* 等约 20 枚——单跑全绿、全量红，全局状态污染面），本会话历次门禁均用定向+逐文件口径；另线登记
- render/text 全包 race：包体量 race 下 >40m（10m/40m 两次超时非竞争），触及面（color*/COLR/SVGTable/adapter/e2e/shape cache）race 绿
- K3 其余两处（ui/rendering/text_layout.go 无 face/不可整形兜底）维持「接线消费面前补全」在册
- bounds walker 不展开 DrawPicture（折叠后命令逐条走 walker 自洽；直绘 damage 由实际绘制产生）——H21 同款口径
- DrawPictureCommand 的 pic 引用在 recording 序列化面（若未来出进程）需要 registry 化——进程内契约无此需求，登记
