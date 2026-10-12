# 60帧变换复用立项（spin_min 线）

> 状态：第一档落地已验证，CPU 目标未达（2026-10-12）。只认代码和实测：三招里第一招已实锤，后两招已实锤缺口，真方案对齐 Flutter/GLFW 损伤加年龄做法。
> 约束：60fps 不动；优化通用能力（任意静内容加旋转变换），不动示例行为；render/gpu 高风险区动手前另行确认。

## 0. 结论

- 浏览器那 1% 来自三招：印章只盖一次、损伤剪刀真生效、损伤按缓冲年龄合并。spin_min 现在只做实了第一招。
- 基线（2026-10-12，补底后，GPUI_BACKEND=go，480x320，10 秒）：600 presents，CPU 16.99%（进程口径，约整机 4%），Build 0.02ms，Raster 0.68ms，Flush 0.44ms，损伤 8281px（整面 5%），PresentMode damage_union，CacheEntries 2，BoundaryRerecord 2 对 BoundarySkip 1188。
- P1（跳重建）只能省 0.02ms 级别，砍了也看不见数。真动手点在提交路径，不在 Go 侧结构。

## 1. 三招核实（证据链）

### 第一招：印章只盖一次（已实锤）

- spin 图标 66x66 纹理只录一次：BoundaryRerecord=2（预热加首次），BoundarySkip=1188（10 秒 600 帧）。
- 拼包 Phase1 约 0.004ms 加 Phase2 约 0.009ms（实测 TexturedPhaseMs1/2，已撤出的临时观测值），不是瓶颈。
- 证据：ui/scene/textured.go pushRecordCTM 去旋转录制、合成走 live CTM；spin_min 日志 frame6 record local w=66 h=66 后逐帧 blit。

### 第二招：损伤剪刀没生效（已实锤缺口）

- GLES 交换链每次 BeginFrame 都 `CreateTextureView` 一个新 Go 对象（gpu/gwgpu/gles/swapchain.go:279），会话层 `view != s.lastView`（render/internal/gpu/render_session.go:5619/6100/6234）恒成立，frameRendered 恒 false，fullSurface 恒 true，损伤剪刀分支走不到。
- webgpu 同理：GetCurrentTexture 逐帧新包装（gpu/webgpu/surface.go），NativeHandle 回 0（gpu/webgpu/texture.go），没有可比的稳定身份。
- 后果：指标写 damage_union，实际每帧整面合成加整面提交。8281px 损伤算出来了但提交没用上。

### 第三招：损伤要并历史（已实锤缺口，Flutter 有标准做法）

- 交换链图片轮转，当前这张上次展示是几帧前，只画当前帧损伤会漏旧位置（白圈就是这么来的）。
- Flutter/GLFW 标准做法：查 EGL_BUFFER_AGE，无扩展按 4 处理，并最近 age-1 帧损伤（engine/src/flutter/examples/glfw_drm/FlutterEmbedderGLFW.cc:222-247）；embedder 侧 populate_existing_damage，部分提交关不掉就退整面（embedder.cc:390-406）。
- 本仓现状：render 层无损伤历史，无 buffer-age 查询，无 per-image 帧状态。grep damageHistory/damageForPlan/ClearDamageHistory 全空。

## 2. 真方案（对齐项）

1. 视图稳定身份：比父纹理或原生句柄，不比 Go 包装指针。GLES 用 GL 纹理 id（resource.go NativeHandle），webgpu 找稳定句柄，找不到就保持整面（诚实回退）。
2. 损伤并历史：记最近 4 帧损伤（轮转一般 3 到 4 张，取 4 兜底），提交用并集；重配/尺寸变清历史加整帧。
3. 上屏透脏区：render 调 EndFrameWithDamage，GLES 接已有 SwapBuffersWithDamage（queue_linux.go 已写好，一直传 nil），webgpu 保持整面回退。

## 3. 预期和风险

- 预期：17% 到 11% 上下（估计值，实测为准），到不了 1%。60 次提交的固定开销还在，60fps 不动是底线。
- 风险：render/gpu 高风险区；全回归（逐文件单测）加像素对比（快照加真窗抓图）；resize/多窗/双后端都要过。
- 不动：示例行为、公开 API（只加内部计时则另议）、别线文件。

## 4. 第二档调研结论（只读代码，未动手）

- 每帧固定动作：编码器新建（blit 走 sessionBlitEncoderDesc）、渲染通道开关、提交一次、换图一次。60 帧下这四件套省不掉。
- 命令缓冲两家都是一次性的：webgpu 提交后标记 submitted 再交就报错，GLES 提交时把命令列表清空回收了。所以“编一次投多次”两边都不支持，要做就得另起机制，不是顺手改。
- 已经省掉的部分：uniform slab 只有宽高和透明度变了才上传（spin 这扇窗基本不传）；绑定组按槽位缓存，纹理和采样器不变就不建；顶点上传带哈希比对，不变不传。这三块前人已经做过。
- 每帧还在变的东西只有一件：旋转把四个角坐标烘进顶点里了，所以顶点每帧重算加上传（两张四边形，一共 12 个点，量很小）。
- PresentCount 是帧数的两倍只是计数口径（循环里记一次、结果里再记一次），真实提交一帧一次（FrameFlushes 恒 1），不用管。
- 结论：第二档真要动手，方向是把旋转从顶点里拿出来改成参量传（着色器里转），顶点一次上传后面只比哈希；再往下就是提交本身的固定开销，那是驱动的事，省不了。预期收益小（个位数），风险在 render 加 gpu，要全回归。

## 5. 第二档设计（已核实，未动手）

- 放哪：旋转只进 uniform 矩阵，顶点只发轴对齐四边形。着色器里 `transform` 矩阵槽是现成的，现在只写了正交投影，旋转直接乘进去就行，不改顶点格式，不管线布局。
- 比对键：uniform 跳过上传只比宽高透明度（两条路共用 `imageUniformLastKey`），角度必须进键，不然角度冻住。顶点哈希比对不动：轴对齐顶点静止后自动命中，上传跳过。
- 合并：`canMerge*` 都不看角落，旋转走 uniform 不影响合批。
- 命令缓冲两家都是一次性，复投等于重录，不做。
- 损伤、剪刀、上屏逻辑都不动：损伤还是按包围盒在 CPU 算，PresentCount 双计数只是 metrics 口径问题。
- 预期：个位数收益（省 12 个顶点重算加一次小上传）；提交固定开销（编码器、提交、换图）还在，60fps 底线不动。风险：render 加 gpu 双后端一次改到位，全回归加像素对比。

## 6. 修订

| 日期 | 内容 |
|---|---|
| 2026-10-12 | 第二档调研：提交路径读完，命令缓冲两家一次性已有结论落文档，未动手 |

| 日期 | 内容 |
|---|---|
| 2026-10-12 | 立项：三招核实结论加真方案，基线 600 presents/CPU 16.99%/损伤 8281px |
| 2026-10-12 | 第一档落地：GLES 交换链缓存 surface 视图（同一张纸同一个身份，Configure/Release 失活）；PresentTarget 损伤历史 depth4（重配清历史，forceFull 清历史传空整面上屏）；present 改调 EndFrameWithDamage；GLES 透脏区给 eglSwapBuffersWithDamage（无扩展回退整面），webgpu 保持整面回退。render/present_damage_history_test.go 5 单测全绿；render 损伤相关单测全绿（1 条红系预先存在，干净树同红）；gles 整包绿；真窗 go/native 双 60 帧、损伤 7~8 千、无圈、底深蓝；快照差异仅图标区旋转相位 283 像素，底一致 |
| 2026-10-12 | 第一档验证结论：接线完成、无回归，但 CPU 目标未达（16.99%→16.49%，基本持平）。损伤剪刀只省填充，提交固定开销还在。真省要靠第二档（提交复用），另行立项 |
| 2026-10-12 | 第一档收敛：forceFull 整帧跳过历史入栈（清历史传空整面上屏）；最终验证 go 599 presents/10.2s、damage_union 7921、亮像素 1023→1028 无圈，native 同模式干净。已合入 5402484d（仅本线 9 文件） |
