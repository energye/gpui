# L2 深审报告：P3.2 自定义/运行时着色器（2026-11-03，只读深审代理产出，多数条目带一次性程序实测）

状态：**待逐条复现核实**（代理已用 /tmp 一次性程序实测复现的条目可信度高，仍按纪律逐条自己跑红）。

## 结论
2×P0（对象/显存永久泄漏+Close 空操作、shader 失败退化成环境实心色=契约违背+钉测假绿）、
1×P1（pipeline 缓存键缺绑定态→首绘毒化，实测三条改法全瞎）、4×P1/P2（半透明输出丢色、
入口校验只到语法、uv 裁后拉伸、混合模式丢失）、每绘制 WaitIdle 同步、录制欠账影响面精确化。
多数根因=当时「逐位一致」门禁只覆盖 A=1.0 单路径。

## 确认问题
- **P0-1 泄漏**：user_shader.go:54-103/208-209 缓存以 *ShaderEffect 为键只插不删；
  gpu_shared.go:569-660 abandon 清单独缺 s.userShader → 每 effect +65,792B/+2 handles 线性涨
  （8 轮 +526KB/+16 实测）；shader_effect.go:162 closed 只写不读，Close() 全空；
  device 切换路径对已 Release 的旧 device 调 Destroy()。
- **P0-2 契约违背**：六处 ErrFallbackToCPU（user_shader_fill.go:39/44/60/67/86/123）落到
  context_draw.go:275/326 的 ambient Fill——shader 失败=画环境实心色且计 cpu_fb；
  CPU 模式守卫顺序错误实测 (0,255,0) 绿块；缺兄弟路都有的 ensureGPU() 懒初始化
  （首 GPU op=shader 绘制 2/2 复现 gpu_ops=0）；TestShaderPaintCPUSkips（p32_shader_effect_test.go:78-105）
  黑底黑默认 paint=「漏画」与「跳过」像素等价=假绿样板。
- **P1-3 首绘毒化**：user_shader.go:126-153 BGL MinBindingSize/hasTex 取首绘实时值永久缓存，
  逐帧 bind 用新值；实测晚 SetImage/SetImage(nil)/uniform 32→16B 三种改法全 fb+画错。
- **P1-4 半透明丢色**：shader_effect.go:28-29（straight）与 user_shader.go:26/summary.md
  （premul）文档自相矛盾且两处都不转；实测 A=0.5 红→(126,127,127) 灰。
- **P1-5 入口校验只到语法**：无 fs_main/纯 compute/vec3 返回/512B struct/无 sampler/f64 全收；
  native(ir.Validate CapAll)/gles(glsl.Compile 不 Validate)/metal(LowerWithSource) 三端各异，
  「双端一致」注释不成立。
- **P2-6 uv 裁后拉伸**：user_shader_fill.go:46-58 按裁后 bounds 归一（实测 9/246 vs 整矩形 6/191），
  Skia/Flutter uv 属对象局部坐标不随裁剪变。
- **P2-7 混合丢失**：user_shader_fill.go:131 走 QueueImageDraw 写死 Normal（兄弟路 2136-2140 用
  Blend 版），paint.go:129-133「blending still apply」不成立。
- **P2-8 每绘制全同步**：user_shader_fill.go:215-219 submit+WaitIdle+回读+再上传，RT/纹理每次建拆，
  user_shader.go:124-125「once per distinct image」注释为假；真窗 bake 一次后 DrawImageEx，
  300 presents 只跑 1 次 shader→逐帧成本零覆盖。
- **P3-9 录制欠账精确化**：recording Backend.FillPath/命令 Brush 无 shader 槽（backend.go:123、
  command.go:302-352），ui/rendering 全走录制路→组件用 Paint.Shader 录进画片即静默丢；
  Push/Pop 不存 paint（context_layer_pass.go:34-58 注释与实现不符，同 L1 P1-3）；
  DrawShaderMask 手抄还原漏 Shader（draw_shader_mask.go:30/47-48）且 ambient shader 时
  遮罩矩形被 shader 接管（FillPath:2034 先判 Shader）——P1-6 Pattern 同类。

## 疑点
- 本机偶发整进程 GPU 起不来与 P0-2 LAZY 现象难分，修后复测分离。
- fillShaderAsImage 取 device 即解锁再用（34-38）跨线程风险（单线程渲染下不成立）。
- 池化后池键须含 bw/bh+format。
- 文本/字形路不认 paint.Shader（Skia 生效）——范围缺口还是缺陷待拍板。
- assessment D1 草案含 DrawShaderRect 未交付，API 名以现实现为准。

## 已核对无问题
BGRA8 RT 与 swizzle 自洽；WriteMask/ClearValue/StoreOp 首轮坑已修；vs_main uv 行序一致；
shaderUniformMax=256==槽步长 WriteBuffer 永不越界；逐帧资源 defer 释放无泄漏；
Stroke 展开同走 FillPath 生效；后端零改动成立；语法级拒绝真绿。
