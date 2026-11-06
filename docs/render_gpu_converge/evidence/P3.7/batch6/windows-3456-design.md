批 6 ③④⑤⑥ 四扇设计（2026-11-06，开工前停报告版；臂清单钉版见 `batch6-window-plan.md` §1/§5/§6，硬要求同 §3 六条）
====================================================================================

一、命名与归位
--------------
③ `examples/engine/p37_material`（材质）／④ `examples/engine/p37_mip_exit`（mip 出口）／
⑤ `examples/engine/p37_bloom`（泛光）——三扇都是 2.5D 引擎窗，归 `examples/engine/`；
⑥ `examples/ui_wr_p37_scene`（scene 流）——scene 流窗不归 engine/（批 3e-2 结构约束：
engine/、ui/ 对 render/scene 零引用），归 examples/ 顶层 ui_wr_ 族惯例。
全部：满窗即内容、指标行浮左上（金图 mask `y<32`）、金图 `{name}_final_{native,go}.png`
fail-closed、`-auto-only` + `-manual-seconds` 二合一、H15 定帧 + H16 变异各带实测。

二、③ 材质窗（收 747/748/749/750 四行）
---------------------------------------
臂 a（3b 三入口）：三入口各挂同一枚用户 shader——`DrawImageOptions.Material`（图像口）、
`QuadDrawOptions.Material`（仿射 quad 口）、`AtlasSprite.Material`（批绘口）——挂材质与
不挂并排，tint 类像素差可见；探针带 `RenderPathStats` 的 GPU op 对账（**无读回断言**：
op 数不随材质张数线性炸）。
臂 b（3a premul）：**半透明材质填充 alpha<255**——`SetRGB+SetShader` 半透明走填充腿
（premul 修复后的画面，对照 p32 d9 期望 (255,128,128)），期望色由同一份常数按 premul
公式折算。
臂 c（3c 多纹理）：`ShaderEffect.SetImageAt(slot1, 第二张)` → shader 采样第二纹理出图，
像素含第二纹理色；摘第二张 → 具名拒绝/回退（实现时按 3c 契约钉死方向）。
臂 d（3d 描边吃 shader）：`SetRGB+SetShader` 的 AA 描边（轮廓展开 → fillShaderAsImage，
AA/coverage 全继承）——描边像素是 shader 输出而非实心色（上下沿 Δ 断言）。
臂 e（3d 拒绝态 + gles 真触发）：底带七哨兵六枚前端具名可见（BadWGSL/NoEntry/
BadReturn/SlotConflict/UniformTooLarge/Closed）+ **透视+bicubic+材质 →
`ErrQuadMaterialUnsupported` 拒绝 + `MaterialSkips++`**（画面拒绝态可见）；
`ErrShaderBackendCompile` 的 **gles 腿真触发**：不可编译的 WGSL 构造在
`GPUI_BACKEND=go` 轮里 `ShaderBackendFails++` 且 fallback 画面正确（本扇的
gles 承载承诺兑现）。
H16 变异：① 摘材质挂载（画面回无材质态）；② 三入口材质全部摘除 → MaterialSkips
清零报警；③ 透视+bicubic+材质臂换合法组合 → 拒绝计数清零报警。

三、④ mip 出口窗（收 756 行，批 4 主跳）
---------------------------------------
臂 a：三档 `sprite.AtlasSprite.Mipmap`（None/Nearest/Linear）× 重缩小并排——引擎出口
`ToRender()` 逐字段带到渲染命令，屏幕差可见（None 走样 vs Linear 平滑），像素断言
三档互异。
臂 b：Filter（Nearest/Bilinear/Bicubic）× Mipmap 合法组合出图 + 零值老路逐位保住
（FilterUnset+MipmapNone ≡ 批 4 前画面）。
臂 c：**摘传递必回退**（负向变异钉）：`ToRender()` 里摘 Mipmap 传递 → 屏幕回 None 态
→ 像素断言报警（H16 承重变异）。
登记口径如实：`tex.Filter↔AtlasFilter→DrawImageOptions.Mipmap` 的 renderconv 映射
（批 4 明文未做，待「档位挂引擎哪个对象」拍板）**不在本窗**，探针不冒充。
H16 变异：① 摘 Mipmap 传递（回退 None）；② 摘 Filter（回 bilinear）。

四、⑤ 泛光窗（收 759/760 两行）
-------------------------------
臂 a：亮部场景（暗底 + 高亮精灵/光斑）× `Context.ApplyBloom(BloomParams)` 扫参——
阈值/半径/强度三组并排，泛光晕像素断言（暗区被点亮、亮区过曝扩散）。
臂 b：恒等参数（Intensity=0）≡ 无泛光**逐位**（diff=0，金图同段零差）。
臂 c：**LDR 三处声明探针**（显式进 JSON + 底带可见）：① 本窗 GPU 四 pass 是 RGBA8
LDR 近似；② `render/hdr` GPU 侧未接（`FloatTarget.TextureView()` 恒 nil）；③
`engine/fx` CPU 泛光 clamp01=LDR。
fps/p95 **只报数不判绿**（§3 硬要求第 5 条，重定标未做）。
H16 变异：① 强度 0.6→0（泛光消失，回恒等帧）；② 阈值 0.6→0.05（全画面泛光）。

五、⑥ scene 流收口窗（收 745/746/751 三行，第 ⑥ 扇）
-----------------------------------------------------
臂 a：`Scene.SetProjection/PushProjection/PopProjection` 经 scene 流（SceneBuilder
录制 → 编码 → 解码 → `GPUSceneRenderer.RenderScene` 回放到 Context）出图——投影四
动词的画面效应可见。
臂 b：`Scene.DrawImageQuad`（梯形+权重）出图。
臂 c：`Scene.SetMaterial` item 级粘滞出图（材质随 item 走、粘滞到后续 item）+
`MaterialSkips` 计数可见。
臂 d：三条既有 3e 钉的窗内像素版（3e-1 投影口、3e-2 quad 通道、3e-3 材质编码）。
链路真实性探针：JSON 带编码字节数/tag 计数（「编码字节→解码→回放」链路真实发生，
不是直绘冒充）。
H16 变异：① 摘 SetProjection（投影回仿射 → 四角错位报警）；② 摘 SetMaterial（材质臂
回无材质态）。

六、批级门禁收尾（四窗两枚提交之后统一补，口径按批 3 收尾登记）
---------------------------------------------------------------
五目标交叉构建 {默认,nogpu} 10 段；64 包全量 sweep 对在册只减不增；conformance
双后端一轮；**整包 `-race ./render` 回填 §7**（H20 修后已恢复，跑一轮记数字）；
四扇跑完逐行把 §7 剩余 ☑（744–752 过程行除外）改回 ✅。

七、待拍板（question 停报告）
------------------------------
四扇命名/归位与臂清单照此开工；⑤ 的 fps/p95 只报数不判绿与 LDR 三处声明口径确认；
⑥ 放 examples/ 顶层 ui_wr_p37_scene（不进 engine/）确认。
