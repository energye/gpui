# L1 深审报告：P3.1 色彩管理地基（2026-11-03，只读深审代理产出）

状态：**待逐条复现核实**（本报告为审查代理结论，未修码；纪律=每条先复现红再修）。

## 结论
4 条 P1 + 3 条 P2，集中在「linear 生效域」的路由/遥测与「降级=双端行为一致」契约；无显存/线程类暗病。

## P1
- P1-1 layer/DrawPaint 路由 advanced+Linear 静默回 sRGB 域（context_misc_draw.go:81-88 改走
  PushLayer+BlendNormal，layer 合成链无 space 维度：context_layer_blend.go:108-119、
  advanced_layer_cpu.go:80、dual_tex_blend.go:1358-1363/1570/1731 硬写 false）。复现：
  BlendDifference+Linear+DrawPaint → sRGB 76 而非线性 168。
- P1-2 固定功能族 Linear 在 CPU 被路由到另一套 PD 实现（software_blend.go:39-42 linear 分支
  排在 :46-165 固定功能 switch 之前，Clear/Copy/Plus/Modulate/... 全落
  compositeAdvancedLinear→GetBlendFunc 退化；blendClear 恒 0 忽略 coverage vs
  software_blend.go:48-52 按覆盖半擦）→ 双端不一致 + AA 边缘全擦/半擦分叉。
- P1-3 DrawImageEx 泄漏环境 BlendMode/BlendSpace（context_image_draw.go:195-214 注释称保存
  paint，实际 Push/Pop 不碰 c.paint：context_layer_pass.go:37-57/63-106；advanced 模式
  context_image_gpu.go:107-110 必回落此路；ApplyMask CPU 档同路径 context_mask.go:128-136）。
  对照正确写法 recording/backend_canvas.go:229-245 withBlend。
- P1-4 降级契约三处断言无判别力（不透明源两域 Normal 恒等：scene.go:43-44/118-125、
  p31_blend_space_test.go:99-104/:144-152）——同文件 :18-21 已写明判别对却没用。假绿形态。

## P2
- P2-5 降级计数器漏主干场景（FillPath solid+fixed 分支与 shader/filter/mask 三支不计数；
  gpu_shared.go:91-96 注释与实态不符）。
- P2-6 非分离模式亮度系数双端不同（WGSL Rec.709 0.2126/0.7152/0.0722 vs CPU Rec.601 三套；
  W3C/Skia 标准=709，CPU 偏离且 advanced 四模式双端出不同像素）。
- P2-7 公共文档与实现相反（blendmode.go:45-46、context_layer_blend.go:46-47「CPU 全模式
  支持 Linear」与 internal/blend/linear.go:59、internal/image/draw.go:359/424-437 门控矛盾）。

## 疑点（需复现验证）
- bicubic CPU 回落的 linear 包装按 straight 解码而 dst 疑为 premul
  （internal/image/draw.go:362-366 vs context_image_draw.go:137-157、
  context_layer_blend.go:93-101）→ 需 bicubic+Multiply+Linear premul 源用例定量级。
- 单/多 op 共用 cache.uniform 与延迟 submit 交叉覆写（dualTexAdvancedBlendNoReadback
  内部 submitGated 疑已安全，stash 路径未逐一验证）。

## 已核对无问题面（覆盖证明）
管线键/批谓词无跨 space 合批；WGSL Params 布局对齐；alpha 不 gamma；GPU/CPU
unpremul→decode→premul 顺序正确；sRGB 传递函数双端共用；录制四命令 5/5 携带 space；
Backend 契约委托无漂移；Paint.Clone；零值 SRGB；P3.1b 无半截代码；render 无后端类型泄漏。
