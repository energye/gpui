# P3.8 批 4f：showcase 窗口锯齿调查——窗口展示链无罪，SDF 覆盖公式根修

日期：2026-11-09。前序：批 4e 回滚后转本会话，开工任务=「窗口展示环节锯齿」
调查（用户：图形一样、边缘明显有锯齿不顺滑，两侧明显不一样）。

## 1. 调查三谜全解

### 1.1 (324,156) 之谜：位置分毫不差，无漂移

4e 记录「金图里花冠在 (324,156)，代码声明 (40,96)」。本批破解：
**ShellChrome 的 Body 面板有偏移 (284,60)**（legendW=260 + gap×2=284；
topH=48+gap=60）——ours 面板全局 = (284+12, 60+70) = (296,130)，花冠
面板内局部 (28,26) → 全局 **(324,156) 分毫不差**。4e 漏算 Body offset，
「布局漂移」推断作废。

### 1.2 GPU 展示链无罪：bilinear 降采样逐位忠实

同跑闭环（同一次窗口运行里 dump 烘焙图 + 快照）：金图 ours 面板 vs
CPU bilinear(同跑烘焙) **avg=0.0388 / max=5 / over8=0**；ref 面板 vs
bilinear(ref1024) avg=0.015。DrawImageBuf→GPU quad→swapchain 读回
全程忠实，无额外重采样层。**注意跨跑对比会得到 avg≈1.2 的假差异**
（虚线渲染跨进程漂移，见 §5）。

### 1.3 真凶：窗口进程的 SVG 烘焙走了 SDF 形状加速，质量与验证环境脱钩

决定性二分（import render/gpu 的探针进程里逐个注销）：

| 配置 | bake vs 纯 CPU 探针基线 |
|---|---|
| render/gpu 原样（RegisterAccelerator + RegisterCoverageFiller） | **32.324%** |
| 注销 CoverageFiller（nofiller） | 32.324%（不变） |
| **注销 Accelerator（noaccel）** | **0.468%（残差=虚线）** |

- **真凶 = render/gpu init 注册的 SDFAccelerator**（CoverageFiller 路由
  无关——nofiller 不变）。
- 机制链：窗口进程 Fill → tryGPUFillWithMode → SDFAccelerator 接单 →
  有 device 走 GPU SDF shader（sdf_render.wgsl 单点采样 +
  0.75px smoothstep）、无 device 走 `shared.cpuFallback` =
  **render.SDFAccelerator（CPU SDF，render/sdf.go 单点采样 +
  smoothstepCoverage(0.75)）**——filled circle/ellipse/rrect 全部。
- 差异图内容吻合：圆形元素（badge circle、flowerClip circle、mask 圆、
  虚线圆、SVG 徽章）+ 圆角卡片框全部亮边。
- **4e 的全部验证（conformance 94.1%、resvg 对比、真值探针）都在无
  gpu import 的进程跑（AnalyticFiller 解析覆盖）——门禁测的是 A 实现
  （解析），生产窗口跑的是 B 实现（SDF）**。「门禁绿 ≠ 底层洞已修」
  的结构性案例。

## 2. 修复（红测先行，用户拍板「根修 SDF 覆盖公式」）

### 2.1 红测（先红）

`render/sdf_coverage_quality_test.go`：三类形状（circle/ellipse/rrect）
× 多组分数圆心/半径，对 128-256²/像素 数值积分真值统计边缘覆盖误差
（0-255 尺度），锁 avg≤2/max≤8。修复前 circle avg=8.0/max=19（红）。

### 2.2 CPU 解析行积分（render/sdf.go）

- `exactCircleRectArea`：矩形与圆的行积分闭式解——行截面
  w(t)=√(r²−t²)，R+=clamp(w−u,0,rectW)、R−=clamp(−u−w,0,rectW)，分段点
  w=k ⇒ t=±√(r²−k²)，原函数 W(t)=(t·w+r²·asin(t/r))/2 逐段精确积分。
  快速路径：四角全内（凸性）→1、最近点在外→0。
- `exactEllipsePixelArea`：仿射到单位圆空间（矩形同步仿射）×rx·ry。
- `exactRRectPixelArea`：直边区常数截面 + 角区圆弧原函数；R− 形态边界
  W=cx−rx0/cx−rx1 必须入分段（漏切即错，开发中实踩）。
- `SDFFilledCircleCoverage/SDFFilledEllipseCoverage/
  SDFFilledRRectCoverage` 改调解析版；`fillCircleSDF/fillEllipseSDF/
  fillRRectSDF`（CPU SDF 腿）随之升级。stroke 腿（smoothstep 环带）
  本批不动（随虚线/描边批）。
- **修复后**：circle avg=0.26/max=1.2、ellipse avg=0.02/max=0.3、
  rrect avg=0.01/max=0.1——**30-800 倍改善**，与 AnalyticFiller
  （avg 0.0-0.3）同级。

### 2.3 GPU shader 尝试与回滚（驱动黑盒，登记教训）

- 解析行积分 wgsl（原函数 asin/atan2、分段排序数组）：**naga 编译
  通过（CreateShaderModule 探针 OK）但 NVIDIA 驱动侧整条 shader 被拒
  ——画面全空**（形状静默消失，无任何错误输出；常数覆盖实验同样白板
  排除了公式错误）。32×32 子采样版（纯 for/if/sqrt）同样全空。
- **全部回滚**，sdf_render.wgsl 恢复原版。教训：本机栈
  （940MX + NVIDIA 580 + wgpu-native）对复杂 wgsl 构造（嵌套循环
  + 数组排序 + 大函数）静默拒绝，naga 层不可见——GPU shader 改动
  必须配真窗像素验收，CreateShaderModule 通过≠驱动可用。

### 2.4 WithCPUOnly：SVG 烘焙钉死 CPU 光栅化（引擎层语义修复）

- `Context.cpuOnly`（WithCPUOnly，注释明言「offscreen rasters」）此前
  只挡 GPU context 创建，**doFill/tryGPUFillWithMode 不查它**——
  cpuOnly Context 的 Fill 仍被全局 accelerator 接单。
- 修复 1（render/context_draw.go）：`CPUOnlyMode() || c.cpuOnly` 同走
  CPU 光栅化分支（设计意图对齐实现）。
- 修复 2（render/svg/svg.go）：`doc.Render` 的 Context 加
  `WithCPUOnly()`——SVG 烘焙是离屏 CPU 场景，结果不再随进程的 GPU
  会话漂移（窗口进程与纯 CPU 进程烘焙逐位一致，验证结论对生产生效）。

## 3. 验证

- 红测转绿：`TestSDFFilled{Circle,Ellipse,RRect}CoverageVsTruth` 全绿。
- 三真窗重种后全绿：showcase（logic 4/4、pixel 10/10、golden 0.0000%）、
  svg_text（6/6、11/11、0.0000%）、svg_structure（6/6、20/20、0.0000%）。
  新旧金图差异图=纯 SDF 边缘带（SVG 徽章/圆环/圆角框），无位移无缺失；
  HUD 条微差为已知登记项。
- 闭环：新金图 ours 面板 vs bilinear(CPU 烘焙) avg=0.39（残差=虚线跨
  进程漂移洞，见 §5）。
- 用户视角：ours vs ref 花冠区 3x 放大对比，边缘平滑度肉眼同级；
  ours vs ref 全面板 over8 从 8449(4.16%)→7190(3.54%)（底盘是既有的
  花冠位移语义差异，见 §5）。
- 回归：svg 包全绿（conformance 94.1% 保持——conformance 恒 CPU 路径，
  数字不变）；internal/gpu、internal/raster 全绿；render 包失败集与
  4e 基线逐名一致（19 项先在 + 抖动，单跑全 PASS；
  TestMiscDrawGPUPath stash 对照确认先在）。spy 诊断测试
  （rasterizer_routing_test.go）修复了自身两处写错（阈值低估、
  同进程 accelerator 测试残留 mock）。

## 4. 改动清单

- render/sdf.go：exactCircleRectArea/exactCirclePixelArea/
  exactEllipsePixelArea/exactRRectPixelArea + 三个公开覆盖函数改调。
- render/sdf_accelerator.go：fillEllipseSDF 改调解析覆盖（circle/rrect
  的 fill 腿经公开函数自动生效）。
- render/context_draw.go：doFillUnprojected 尊重 c.cpuOnly。
- render/svg/svg.go：doc.Render 加 WithCPUOnly()。
- render/sdf_coverage_quality_test.go（新）、render/
  rasterizer_routing_test.go（新，路由诊断钉）。
- 三窗金图重种（showcase/text/structure 的 png+base）。

## 5. 遗留登记

1. **GPU SDF shader 的 smoothstep 覆盖洞**：UI 实时场景的 SDF 形状
   （窗口直接绘制的圆/圆角矩形）仍走单点采样+0.75px smoothstep，
   与解析覆盖差 avg≈8/255。修复需 GPU 侧解析/子采样实现——本机
   NVIDIA 驱动对复杂 wgsl 静默拒绝（§2.3），需要先解决「驱动侧编译
   失败的可见性」（wgpu 编译错误上报）再动 shader（raster 线）。
2. **虚线渲染跨进程漂移**（0.468% 残差）：同代码同输入，虚线（⑭
   蓝虚线×2、⑱虚线圆）在窗口进程与纯 CPU 进程的烘焙间有 AA 级差异
   （over8≈1966、max=55）；同进程内确定。dash 分段/走查的进程状态
   依赖待查（svg 层或 raster 层，归 SVG 线）。
3. **花冠位移语义差异**（既有）：ours 与 resvg 对花冠区域的位置/尺寸
   解析不同（本批前后都在），属 conformance 99 partial 语义洞范畴
   （4d 遗留，SVG 语义线）。
4. **CoverageFiller（SparseStrips）路由**：与 AnalyticFiller 质量接近
   （五类合成边 avg 差≤0.05），但本批未对复杂真实路径做专项对比——
   sparse strips 的复杂路径质量钉测登记 raster 线（低优先）。
