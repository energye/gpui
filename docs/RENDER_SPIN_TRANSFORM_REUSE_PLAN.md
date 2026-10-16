# 60帧变换复用立项（spin_min 线）

> 状态：第一档落地已验证，CPU 目标未达（2026-10-12）。只认代码和实测：三招里第一招已实锤，后两招已实锤缺口，真方案对齐 Flutter/GLFW 损伤加年龄做法。
> 状态更新（2026-10-16）：CPU 收敛中——16.99%（立项）→ 9.11%（2128225c 调度轮询+损伤复用）→ 约 7.1%（本日等待切片+着色器磁盘缓存，15s 固定命令三轮 7.21/7.10/7.15%，干净树复测 7.21/7.21%）→ 约 6.5%（本日第三刀 GLES 直渲窗口默认帧缓冲，见 §6 首行：15s 三轮 7.02/6.45/6.38%，同小时配对 A/B 直渲 6.41/6.37/7.06% 对离屏 7.32/7.29/7.42%）。进程目标 4%（整机 1%）未达。
> 同日诊断（不改代码）：逐帧 GL 调用清单（稳态 14 次/帧，约 334µs/帧 ≈ 2.0pt）见 §7；残余第二大块 `runtime.sysmon` 空转（约 1.7pt）定位与三路证伪见修订表。
> 同日标准对齐核查（4 笔「依据不足」收口）：剪刀常驻撤回（前提证伪，`gpu/gwgpu/gles/surface.go` 恢复上屏收尾永远 Disable），单上传快路 / 损伤矩形复用 / 命令缓冲复用补到 Impeller / Flutter 依据留下；撤回后 15s 三轮 7.28/7.07/7.31%（同小时 A/B 常驻 7.16/7.21/7.44%，差约 0.05~0.1pt 噪声内）。两条 blit else 分支的隐含依赖记为新问题待复现，见修订表。
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
| 2026-10-16 | 第三刀落地（GLES 直渲窗口默认帧缓冲）：面 pass 直接写窗口 FBO 0，上屏只剩 eglSwapBuffers[WithDamage]，撤掉每帧一次 blit + 一次 FBO 绑定（§7 清单里那两笔，构成它们的约 0.45~0.55pt）。做法=目标坐标调整块 `GpuiTargetAdjust`（`vec4(ySign, fragYScale, fragYOffset, 0)`，`gpu/gwgpu/gles/target_adjust.go`）：顶点 epilogue 的 `-gl_Position.y` 与 fragment `@builtin(position)` 的 Y 读都改从块取，渲染目标喂 `(-1,1,0,0)`（逐位等于原字面量），窗口喂 `(1,-1,H,0)`（镜像绘制 + 把片段 Y 读回同一自上而下空间）；块无 `layout(binding)`，链接后 `glGetUniformBlockIndex`+`glUniformBlockBinding` 指名到最高槽（实测 `MAX_UNIFORM_BUFFER_BINDINGS-1`=83，应用绑定从 0 顺发不冲突）。深度/模板用窗口自带的 24/8（`eglChooseConfig` 的 DepthSize/StencilSize 下限；EGL 不保证就查询 `glGetConfigAttrib` 不达标自动落回离屏路，`GPUI_GLES_NO_DIRECT_FBO0=1` 手动回退）；裁剪矩形按 GL 自下而上镜像（`SetScissorRect`），`LoadOp` 语义不变，仅补跨帧界的 depth/stencil（EGL 换屏后 back buffer 的 depth/stencil 未定义，`ClearDefaultDepthStencilCommand` 补清）。依据：Impeller GLES 直绑 FBO 0 不附纹理（`render_pass_gles.cc`，Android 要 24/8）；Flutter Windows/GTK embedder 走「窗口包 FBO+`GL_DEPTH24_STENCIL8` renderbuffer」仍要上屏 blit，故不取。实测：15s 固定命令三轮 7.02/6.45/6.38%（901 presents，MissedVSync 0、Hitch 0、无 CPU 回退、60fps 不动），最终 harness 复测 6.31/6.63/6.57%；同小时配对 A/B（同二进制切 `GPUI_GLES_NO_DIRECT_FBO0`，交替跑）直渲 6.41/6.37/7.06% 对离屏 7.32/7.29/7.42%，逐对差 0.36~0.92pt（中位 0.9pt）；pprof 用户态 340ms/10s 对 450ms（`runtime.cgocall` 130 对 170ms），与「少一次 blit+一次 FBO 绑」一致，剩下的差额在驱动内核侧（ioctl）。路线实证（debug harness）：`gles: surface renders into the window default framebuffer depth=24 stencil=8`；真窗两张 xwd 抓图（间隔 1s，1157 像素变）图标在转、底色 `#1A1F29` 不变。测试：gles 35 个测试文件逐文件绿、codegen 两目录逐文件绿、render 与 render/internal/* 320 绿（17 个红为 native/webgpu 环境预先存在，建 `45867530` worktree 同 17 文件同红）；6s 窗 360 帧 0 报错；三查干净（CGO_ENABLED=0 本线编过；`ui` 直引 `gpu` 0 命中；render 后端解耦 0 真命中） |
| 2026-10-16 | 真窗 oracle 与窗口呈现纪律（本轮 test 侧实测，后续窗口测试照此复用）：① 窗口 FBO 0 的 `glReadPixels` 在 mutter+NVIDIA 下不可信（同进程 clear-only 帧 6/8、1/8 失败，三角帧 20/20 过），窗口内容的可靠 oracle 是 X 服务器副本（`XGetImage`/`xwd`，与正确 GL 读逐字节一致），引擎自有 swapchain FBO 仍用 `glReadPixels`；② 换屏到窗口内容可见有 3~26ms 延迟（下一个 vblank 取上新缓冲），抓图必须轮询；③ `LoadOp=Load` 的帧读到的是「上次送显的那张」（H4G 逐字节断言，含 load-only 空帧不变）；④ 未映射窗口上 `XGetImage` 返回 `BadMatch`，Xlib 默认错误处理会直接 exit 进程（测试二进制整体挂掉），测试须先装 `XSetErrorHandler`；⑤ 窗口还没 viewable 时的 present 会丢：不等 viewable 首帧 4/10 丢（窗上留黑底），先等 IsViewable（实测 4.8~21ms）再 present 则 10/10 落地（3.1~7.4ms），测试开窗后统一 `waitUntilViewable`；⑥ `xwd` 文件头 100 字节 + 窗口名 + ncolors×12 颜色表后才是指针数据（PIL 读不了，仓外小解析器）。新问题归位（本轮记入，未动手）：Windows/WGL 侧直渲格留白——`resource_windows.go` 的 `directDefaultFB` 恒 false（pixel format 的 depth/stencil 位未查询，格子在，写清原因）；既有问题两笔——`gpu/gwgpu/gles/queue.go:76` WGL 编译不过（`have (number, error) want (error)`，15aae17f 起，挡 Windows 交叉编译）、`ClearStencilCommand.stencil` 未用（未绑 `glClearStencil`，当前无人传非 0 值） |
| 2026-10-16 | 标准对齐核查 4 笔收口（① 撤，②③④ 留+补依据）：① 剪刀常驻撤回——`gpu/gwgpu/gles/surface.go` 上屏收尾恢复永远 Disable（加一行约束注释）。依据：原注释「下一帧绘制自设剪刀」不成立，`render/internal/gpu/render_session.go` 两条 blit else 分支（6253-6255 `encodeBlitOnlyPass`、6548-6550 `encodeBlitToEncoder`）整 pass 不设剪刀、靠进 pass 时 GL 剪刀已关；PresentFrameFull/forceFull→FlushGPUWithView（damage 空）→blit-only 生产可达；Impeller 方向相反（`render_pass_gles.cc:435-445` 每 pass ResetGLState 显式关剪刀）。代价实测：撤回 7.28/7.07/7.31% 对常驻 7.16/7.21/7.44%（+0.07pt 中位差，噪声内）。② 单上传快路留：Impeller `device_buffer_gles.cc:91-134` 直 bind+BufferSubData 不批。③ 损伤矩形复用留：Flutter `flow/surface_frame.h:71-83` SubmitInfo 损伤值成员。④ 命令缓冲复用留：Impeller `render_pass.h:248-251`、`blit_pass_gles.h:27` 成员 vector。新问题归位：两条 else 分支「依赖进 pass 剪刀已关」（归属 render/internal/gpu 上屏路径；现象=blit-only 无损伤帧遇残留剪刀会被旧框裁），现状无可复现坏画面，待复现后再补全幅剪刀，本轮不动 render 主路。验证：gles 34 测试文件逐文件绿（0 红）；6s 窗 360 presents 0 报错；三查干净（CGO_ENABLED=0 本线范围编过；`./...` 全量被别线 `examples/ui_polish_gallery` 引不存在的 kit 包挡住，预先存在非本线） |
| 2026-10-16 | 每帧 GL 调用清单实测（光栅线程 FFI 计时+命令转储，同二进制两轮）：稳态固定 14 次 FFI/帧，合计约 334µs/帧 ≈ 2.0pt 单核，明细见 §7。判定：uniform 合并上传对本窗无收益（uniform 全链 4 次调用约 35µs ≈ 0.2pt，就算合并成一次上传/一次绑定上限也只有约 0.1pt），不做；清单里唯一属于我们自己的结构开销是 blit+一次 FBO 绑定+blit 剪刀设置（70~90µs/帧 ≈ 0.45~0.55pt），候选做法是 GLES 交换链直接渲窗口默认帧缓冲、上屏只调 eglSwapBuffersWithDamageKHR（对齐 Flutter GLFW embedder 的 buffer-age 损伤），影响面评估与停下报告后再动手 |
| 2026-10-16 | sysmon 空转定位与三路证伪：干净树 15s 两轮 7.21/7.21%（901 presents），线程账 光栅 2.65% + runtime.sysmon 1.72% + 事件循环 0.80%，其余各 <0.5%。sysmon 栈经 `strace -k` 实锤（`runtime.usleep` ← `runtime.sysmon`），实测约 2,000 次/s 的 20µs nanosleep；按倍增序列（40/80/160µs… 各约 33 次/s）反推约 33 次/s「复位」，每次复位约 1.2ms×20µs 轮询。喂它的原因三路都没坐实：① 关 vblank 阻塞（临时探针 `GPUI_NO_VBLANK_WAIT=1`，A/B 15s：6.83% 对 7.12%，sysmon 线程 1.66% 对 1.73%，探针臂全程无 ≥8ms 系统调用，20µs nanosleep 计数 1,815/s 对 2,036/s）——16.6ms ioctl 不是喂它的，与同日 vblank 撤回记录一致；② 关深睡（`GODEBUG=schedtrace=1000`）反而更差（sysmon 线程 3.05%，总 8.37%）；③ GOMAXPROCS=2 复测 7.26%（sysmon 2.12%），早前 60s 5.91% 不复现。另：最小 Go 探针（60Hz ticker+每拍 3 次 write，12s）同现象但量级小（sysmon 线程 0.37%），量与唤醒密度相关、非本引擎独有。结论：runtime 内部行为，应用层暂无杠杆；待办=若再追先做「帧内等待/跨 goroutine 交接次数减半」的 A/B（预期 0~1pt，不确定） |
| 2026-10-16 | 第三刀落地：着色器翻译磁盘缓存（`gpu/gwgpu/gles/shader_disk_cache.go` 新文件 + `shader.go` 接线）。进程退出即丢的内存 memo 之外加一层落盘：键 = GLSL 版本+入口点+binding 布局+WGSL（沿用 `shaderCacheKey` 规范化），值 = GLSL 文本+TranslationInfo，严格解码拒未知字段（字段改名/删除退化成 miss 而非半解码），临时文件+原子改名（多进程竞争无害），`shaderDiskFormatVersion` 常量管 naga/codegen 变更失效（对齐 Mesa 驱动版本号口径）；默认目录 `~/.cache/gpui/shaders`，env `GPUI_SHADER_CACHE=0/off/false` 关、`GPUI_SHADER_CACHE_DIR` 换目录。实测：启动 naga 全量命中，首秒 CPU 430→250ms、分配 3.36M→35k、首帧 238→77ms、HitchCount 1→0；15s 固定命令同二进制 A/B 配对（关缓存 8.19/8.44/8.10 对开缓存 7.46/7.13/7.09，每对低 0.7~1.3pt），三轮终值 7.21/7.10/7.15%。测试：`shader_disk_cache_test.go` 9 用例（roundtrip/损坏拒绝/env 解析/跨进程复用）；gles 34 测试文件逐文件绿（1 SKIP，Windows-only 与 integration tag 文件未含）；CGO_ENABLED=0 编过、ui 直引 gpu 与 render 后端解耦两查干净 |
| 2026-10-16 | 第二刀落地：主循环等待切片（`ui/embedder/pipeline_app.go` `maxHostWait=8ms`）。帧节流期间的主机等待本身 wake-pipe 驱动（输入/跨线程唤醒/帧 deadline 都会打断），切片只多一次重算；把每次阻塞压在 Go runtime 10ms syscall-retake 阈值之下（runtime/proc.go retake 会把手里的 P 交接，并让 sysmon 退回 20us 空转循环）。实测同日回线：pristine 8.70/8.96 对切片 8.11~8.50（六轮中位数 8.25），约省 0.5~0.8pt；空闲窗（负 timeout）保持无限等待不动 |
| 2026-10-16 | vblank 事件化撤回记录：`ui/platform/vsync_drm_linux.go` 曾改成 DRM_VBLANK_EVENT 排队+poll/read 排空（两种形态：8ms 切片轮询 / 整预算单 poll），内核路径先用仓外探针证实可送（card1 上 type=0x1 seq 连号 tval 差 16.6ms），但同机配对实测两种形态都比原阻塞 ioctl 慢（5 轮配对中位数 A 8.25 对 C 8.54；pprof 里 WaitDRMVBlank 的 poll 栈 5~9ms/s 即代价），且 strace nanosleep 计数显示 sysmon 空转不靠这 16.6ms ioctl 喂（切片版 18501/6s 对 pristine 22120/6s）。结论：不落地，文件还原；vblank 继续用阻塞 ioctl。另记两条既有问题：gles `integration` tag 编译不过（integration_test.go 引 `hal.Instance.Destroy`，2128225c worktree 同错）；`ui/platform/x11_window_linux_test.go` 概率性失败（当日 3/5，pristine 同 3/5） |
| 2026-10-13 | 第二档 wiring 落地（gpuTex 路径）：`canMergeGPUTextureDraw` 加角度三元组键（不同角度/旋转对烘焙不合并，同三元组合并；YUV/轴对齐/剪切/镜像保持烘焙）；`buildGPUTextureResources` 顶点改发吸附后轴对齐基座（1/256px，跨角度位一致，哈希可复用），uniform 改 `writeGpuTexUniform`（旋转合成/正交回退），`imageUniformLastKey` 加 `hasRot/rotCX/rotCY/rotAngle`；`image` 路径未动。6 wiring 单测全绿；9 相关测试文件逐个绿+race 绿；像素等价 2e-5 内；基准检测+合成约203~218ns 对烘焙约89~91ns（CPU 侧仍贵，省的是顶点上传 WriteBuffer）；损伤/剪刀/上传/YUV 行为不变 |
| 2026-10-13 | 第二档探针结论（负收益，热路径不接线）：纯数学探针 `render/internal/gpu/image_rotation_uniform.go`（刚性检测+旋转uniform合成+CPU镜像）+ 7单测全绿；像素等价 5角度 NDC 差 2e-5 内，angle=0 与正交uniform逐位一致；基准检测+合成约218~316ns/op，对烘焙约89~94ns/op（约2.5~3倍，负收益）；合并键风险锁死（不同角度同纹理当前合并为真，进uniform必须把角度进键）；6相关测试文件逐个绿+探针race绿；损伤/剪刀/上传/YUV未动。热路径接线暂缓 |
| 2026-10-12 | 第二档调研：提交路径读完，命令缓冲两家一次性已有结论落文档，未动手 |

| 日期 | 内容 |
|---|---|
| 2026-10-12 | 立项：三招核实结论加真方案，基线 600 presents/CPU 16.99%/损伤 8281px |
| 2026-10-12 | 第一档落地：GLES 交换链缓存 surface 视图（同一张纸同一个身份，Configure/Release 失活）；PresentTarget 损伤历史 depth4（重配清历史，forceFull 清历史传空整面上屏）；present 改调 EndFrameWithDamage；GLES 透脏区给 eglSwapBuffersWithDamage（无扩展回退整面），webgpu 保持整面回退。render/present_damage_history_test.go 5 单测全绿；render 损伤相关单测全绿（1 条红系预先存在，干净树同红）；gles 整包绿；真窗 go/native 双 60 帧、损伤 7~8 千、无圈、底深蓝；快照差异仅图标区旋转相位 283 像素，底一致 |
| 2026-10-12 | 第一档验证结论：接线完成、无回归，但 CPU 目标未达（16.99%→16.49%，基本持平）。损伤剪刀只省填充，提交固定开销还在。真省要靠第二档（提交复用），另行立项 |
| 2026-10-12 | 第一档收敛：forceFull 整帧跳过历史入栈（清历史传空整面上屏）；最终验证 go 599 presents/10.2s、damage_union 7921、亮像素 1023→1028 无圈，native 同模式干净。已合入 5402484d（仅本线 9 文件） |

## 7. 每帧 GL 调用清单（2026-10-16 实测，光栅线程）

同二进制两轮稳态（t=2..15s 稳定）：固定 14 次 FFI/帧（840 次/s），全部落在光栅线程；每帧驱动内时间合计约 334µs ≈ 2.0pt 单核（FFI 计时口径，含驱动的 C 内时间）。命令序（17 条命令，状态缓存并掉约 3 条）：BindVAO → BindSurfaceFramebuffer → SetViewport → DrawBuffers → Scissor(195,115,89×89) → BindGroup → VertexBuffer → UseProgram → PipelineState → 6 顶点 Draw → 6 顶点 Draw → blit → swap。

| 调用 | 次/帧 | 单次（µs） | 备注 |
|---|---|---|---|
| eglSwapBuffersWithDamageKHR | 1 | 105~115 | 上屏，最大单项 |
| glBindFramebuffer | 2 | 34~44 | 渲染通道绑定 + blit 前切缓冲 |
| glBlitFramebuffer | 1 | 30~42 | 损伤 blit 到窗口默认帧缓冲 |
| glDrawArrays | 2 | 19~24 | 两张 6 顶点四边形 |
| glBufferSubData | 2 | 13 | 小上传（非顶点：顶点已静止命中） |
| glBindBufferRange | 2 | 4~5 | uniform 绑 |
| glScissor | 2 | ~4 | 损伤剪刀（常驻已撤回，见修订表） |
| glBindTexture | 2 | 3.5 | 纹理绑定 |

- 稳态无 glClear、无 glGetError、无 fence 查询；glGetError/着色器类调用只在启动 t=1s 出现。
- 剪刀常驻已于 2026-10-16 撤回（依据不足+前提证伪）：稳态恢复每帧 1 次 glEnable + 1 次 glDisable，上表 14 次/帧为常驻下的计数，撤回后约 16 次/帧；固定命令 15s 三轮 7.28/7.07/7.31%，同小时 A/B 常驻 7.16/7.21/7.44%（中位差 +0.07pt，噪声内）。
- uniform 合并上传：对本窗收益 ≈ 0。uniform 全链 glBufferSubData×2 + glBindBufferRange×2 约 35µs/帧 ≈ 0.2pt；就算合并成一次上传/一次绑定，上限也只有约 0.1pt。
- 唯一属于我们的结构开销：blit（30~42µs）+ 一次 FBO 绑定（34~44µs）+ blit 剪刀设置，合计 70~90µs/帧 ≈ 0.45~0.55pt。**已于 2026-10-16 落地**：GLES 面 pass 直渲窗口默认帧缓冲（上屏只调 eglSwapBuffers[WithDamage]），做法与依据见 §6 首行；上表 14 次/帧是离屏路下的计数，直渲路少了那条 `glBlitFramebuffer` 与 blit 前的 `glBindFramebuffer`（稳态降到约 12 次/帧），要精确数字再挂一次 FFI 计时探针重测。
- 其余（swap、draw、bind 的驱动侧时间）是驱动固有成本，§4 结论不变。线程账另见修订表：光栅 2.65% + sysmon 1.72% + 事件循环 0.80%。
