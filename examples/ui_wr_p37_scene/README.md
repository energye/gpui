# p37_scene — 批 6 ⑥ scene 流收口窗

P3.7 批 6 第 ⑥ 扇真窗（收台账 745/746/751：批 3e-1 scene 投影口、3e-2
quad+权重通道、3e-3 item 级材质编码）。放 `examples/` 顶层 `ui_wr_` 族、
**不进 engine/**——「engine/、ui/ 对 render/scene 零引用」是批 3e-2 钉下的
结构约束，本窗就是 scene 侧的像素收口载体。满窗即内容（六面板），指标行浮
左上（金图 mask `y<32`）。

链路全程：**Scene 录制 → Encoding（tag 流+载荷流）→ Decoder 走查 →
GPUSceneRenderer.RenderScene 回放到 Context**，JSON 带 `enc_bytes/enc_tags`
与逐场景 tag 计数——不是直绘冒充。

面板：

- **A1 投影层**：`SetProjection(Matrix4 真 homography)` 后录的红块被投影
  变形（全 dyadic 元素，float32 编码逐位精确往返）。链序口径：投影在设备域
  生效（1d 钉的既有口径），语义锚落点因此**从与窗体同形的离屏回放输出反推**
  （红 mask bbox 中心），不从公式反推结论。
- **A2 恒等对照**：同景不装投影（鉴别臂）。
- **A3 Push/Pop 栈**：平→`PushProjection` 内块→`Pop`→平尾块，两枚 tag。
- **B quad+权重腿**：`Scene.DrawImageQuad`——`ProjectorMatrix4.ProjectQuad`
  出口「算好再喂」的梯形+齐次权重（与 scene 投影层**不叠用**，scene.go 明文）。
- **C 材质粘滞+清除**：`SetMaterial(渐变)` 粘滞到后续 fill 与 image（item 级
  语义：图像臂吃 opts.Material），`SetMaterial(nil)` 发清除载荷，之后 fill
  回刷色。
- **D 拒绝态**：透视 quad 挂材质 ⇒ 校验期具名拒、不入队、不计数、该笔不画
  （③ 窗口径的 scene 流版）。

三证据合证：15 枚探针（载荷回环逐值：投影 mode+16 元素、quad float32 折算
角点+权重、材质 [索引,0xFFFFFFFF 清除]；回放 ≡ 手写**逐位 diff=0** 三路各自
+ 反向鉴别 + 非空转；CPU 脏区腿两态：材质拒画计数 MaterialSkips、清除后投影
下 image 拒画计数 ProjectionDegrades——批 3 收尾登记的「脏区腿材质表态」
在此核掉）+ 4 枚语义像素（材质中点按 premul 直出公式、余为不透明精确色）+
双后端金图逐位 0（两基线相同）。零发射纪律：不调三动词/装恒等投影 ⇒ tag 流
与 Hash 逐位不变；Hash 吞投影/材质进场景身份。

H15 定帧 240 六轮恒 0（presents 240/241；静态内容窗常驻相金图同 0 如实记）。
H16 两变异（窗体场景各删一行发射，跑完还原核对净）：① 摘 SetProjection ⇒
五枚探针红（counts/payload/回放≡手写 diff=14351/鉴别归零/Hash）+ 金图
**2.0477%**（首跑探针 2 got 串无守卫索引 panic——窗自身缺陷，修成报红不崩后
重跑取数，如实记）；② 摘 SetMaterial ⇒ 四枚探针红 + 语义像素红（渐变块回
实心）+ 金图 **3.3924%**。

live-only 口径：scene GPU 回放经 `RequireLiveContext`——生产帧每帧是 Recorder
走查，回放合法跳过并计数（`live_only_skips≈presents`，只报数不判绿，
`ui_render_base_scene` 同款既有口径）；呈现画面以 warm-up live 全量绘制为准，
**scene 内容确在呈现帧由语义像素锚扛**（恒等/投影/材质/清除四锚全在 scene 画面里）。

## 跑法

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_wr_p37_scene -auto-only
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=8 go run ./examples/ui_wr_p37_scene -auto-only  # 播种，仍不算过
go run ./examples/ui_wr_p37_scene -manual-seconds 6                              # 常驻人工
GPUI_BACKEND=go RUN_SECONDS=8 go run ./examples/ui_wr_p37_scene -auto-only       # 第二后端一轮
```

零新增公开 API。
