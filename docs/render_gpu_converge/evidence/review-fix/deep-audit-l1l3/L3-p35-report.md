# L3 深审报告：P3.5 路径与几何五项（2026-11-03，只读深审代理产出）

状态：**待逐条复现核实**（代理结论当线索，纪律=复现红才动手；A3 代理自带探针数字）。

## 结论
交付面总体扎实，抓到 6 条真实暗病：1×P0（公开 API 组合 panic）、2×P1（透视降级画错、
ClipRRect 双路不等价且从未对拍）、2×P2、1×P3；另有 5 条疑点。

## 确认问题
- **A1【P0】** GPU 描边腿缺 conic 降换：PathEffect 原型带 ConicTo 即 panic
  （context_path.go:274-282 只降设备入口；gpu_render_context.go:2305-2326 效果后不降 →
  path_convert.go:154-159 盲转 → stroke/expander.go:310-312 panic；CPU 两腿有防线唯 GPU 漏；
  全仓 ConicTo∩PathEffect 测试=0）。复现=公开构造器 NewPath1D/2DPathEffect 原型用 conic。
  修法=效果输出后统一 ExpandConicsToQuads（同 CPU 腿）；vello_accelerator.go:261-269 同形潜在。
- **A2【P1】** 透视管线降级把顶点写在上传之后：render_session.go:4263-4271 先指纹+上传，
  :4282→packPerspectiveQuads:4527-4537 管线建不成才写仿射顶点不再上传 → 批合并当仿射并入
  （firstVertex=i*6）→ 画上一帧残留顶点。与「降级仿射不丢绘制」文档不符。
  复现=ensurePerspectivePipelines 注入 error + matrix4_quad_persp_gpu_test 探针。
  修法=透视打包/降级移到指纹+上传之前。
- **A3【P1，代理实测】** ClipRRect 两档画面不等价且从未对拍：context_clip.go:155-167 分流
  SDF 快路（aa_hw=0.75 斜坡）vs 掩码栅格（4× 过采样+16 段 cubic）；同几何仅
  Scale(1,1+1e-9) 翻守卫实测 r=2/8/12/30 两比例：差异像素 380–476、最大覆盖差 0.071–0.094
  （18–24/255），直边像素也差。批 2 只有分流钉无两路对拍。后果=旋转/非等比缩放跨守卫瞬间
  裁剪边发虚跳变，真窗可撞。
- **A4【P2】** ConicTo 放行 NaN/+Inf 权重（path_build.go:62-66 只挡 w<=0；实测 NaN/+Inf 入库、
  展开出非有限坐标）；Skia AdjustConic 把非有限当直线。
- **A5【P2】** 绕过 Iterate 哨兵的裸 verb 消费点漏 conic：internal/clip/mask.go:471-550
  （ci 不进阶梯→后续坐标整体错位）、tessellate.go:180-232/690-735、stroke_hairline.go:22-103
  （丢段=弦代弧）、transform_uniform.go:113-186（unbakedHash 步长 0 配对错位、unbakePath 丢段）。
  现靠设备入口兜底，裸 Verbs()/Coords() 环无防护。
- **A6【P3】** Path.Append 不维护增量 bounds（path_build.go:106-114）：实测 Path2D 效果输出
  Bounds()=(0,0)-(0,0) vs BoundingBox()=(96,96)-(180,180)；现无消费，接上 Bounds().Empty()
  类判空（text_gpu.go:260、brush_fill.go）即整块不画。

## 疑点（需复现）
- path_effect.go:56-67 自定义效果按实例地址入键且缓存不持引用（2000 轮 GC 探针未复现复用，
  机制风险）；Skia sk_sp 强引用。
- scaleEffect/gpu 守卫挡不住 NaN TransformScale。
- path_boolean.go:125-134 网格钳 ±1e6 vs 回贴未钳精确子段超界不一致。
- matrix4.go:112 绝对 |det|<1e-12 判奇异拒极小比例真逆（Skia 相对判据）。
- refit 重合/相切曲线同网格点仅 ±1 邻接消歧，无退化钉。

## 已核对无问题面（覆盖证明）
conic 数值扫 w∈[1e-12,1e6] 全链有限、w=1 恰 1 quad、ToQuads 增长温和；度量/面积/bbox/
winding/Trim/Length/dash 原生 conic、convex 拒 conic、AA-off 保权；批合并 seal/谓词等价关系/
透视 cmdCount=1/预留槽不变式自洽；透视 CPU 1/w 与 WGSL clip-w 等价、twin 切分双端一致、
ProjectQuad 整 quad 拒绝；PathEffect 哈希全参数、Discrete 无 PRNG 确定性成立；RRect 归一化
对齐 SkRRect。
