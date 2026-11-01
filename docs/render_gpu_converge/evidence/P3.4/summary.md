# P3.4 滤镜家族 — 收口报告（2026-11-01，状态 🟡：五批+真窗主体完成，C 卡 GPU 路径专项遗留）

> 依据：`assessment.md`（D1 全做四批/D2 merge N 层/D3 连带 Paint 字段/D4 mask 链 A8 预处理/D5 GPU 优先单输入族）。实施按五批独立提交。

## 1. 五批交付终态

| 批 | 交付 | 提交 | 门禁 |
|---|---|---|---|
| 1 | erode/dilate/offset/alpha-threshold：CPU（premul 域形态学）+GPU mode 7-10+Kind 四处+Context 四口 | `d3be922f`+`10f57f50` | 值级钉 3 组（对偶性/位移/二值）+既有六族回归绿 |
| 2 | lighting（point diffuse，alpha 高度图中心差分法向）+tile（周期平铺）；GPU Params 128→192B（rect0-3 四 vec4 槽）+mode 11/12 | `4f3971ce`+修正 | 值级钉 2 组（光照几何含负 z 物理/周期性）+回归绿 |
| 3 | merge（Context 编排层 N 层 over，pristine 层派生）+displacement CPU | `3ae47859` | 编排钉（双层落地）+回归绿 |
| 4 | ColorFilter 接口（render straight 域）+Blend/Lighting/Table/Compose+ApplyColorFilter | `3403f712`+修正×2 | 值级钉 4 组（乘加/LUT/复合两向/blend） |
| 5 | Paint.ImageFilter/MaskFilterRadius 字段+fillPaintFilteredAsImage 路由+RegisteredGPUFilterGraphHooks 暴露+SetPaintMaskFilter/SetMaskFilter（mask 链 A8 预处理） | `3ba0a8d3` | 值级钉 2 组（边缘羽化 CPU 模式/merge 双层） |

## 2. 真窗 `ui_wr_p34_filters`（隔离 :3，native）

- **A 卡形态学对偶 2/2 ✅**、**B 卡 merge 双层 2/2 ✅**、**C 卡 Paint.MaskFilter 1/2**（硬边钉绿、羽化钉未达）——**4/6 探针**，既有族回归绿，apidoc rc=0（catalog §13 P3.4 转正，16 符号补齐）。

## 3. 排障实录（全有 dump/实证）

1. `mod` 不是 WGSL 内建（GLSL 惯性）→ shader 编译失败、GPU 图整体回退 CPU（3 例红实证）→ `%` 修正。
2. SDF 捷径吃 filtered paint（P3.2 同坑再现）→ tryGPUOpRC 绕行条件扩 ImageFilter/MaskFilter。
3. 不透明 base 的全图滤镜层互相覆盖（Merge 数学事实）→ 卡改透明底演示。
4. 层 2 的 base 拷贝被层 1 over 污染 → **pristine 层派生**修复。
5. GPU FromView 种子=当前 GPU 内容（含前层）→ **ApplyMerge 层跑改 CPU-only**，GPU per-layer seeding 登记后续。
6. C 卡羽化钉：doFill(4)→tryGPUFillWithMode(4) ✓→tryGPUOpRC **零进入**——bake dc 的 gpuCtx 建立问题，六轮诊断现场存档（assessment §7）；CPU 模式钉绿证明羽化数学与管线本身正确。

## 4. 遗留与后续（全部登记）

- displacement GPU aux 上传（批 3 CPU 面完整）、ApplyMask/PushMaskLayer 的 blur 预处理采用、merge GPU per-layer seeding、erode/dilate O(n) 分离优化、真窗金图「先 diff 后写」、ui paintRuns 迁移。
- Stroke 级 paint 滤镜（StrokeShape/StrokePath 无 filtered 分支，stroke 带滤镜静默直绘）——C 卡收口时登记的能力缺口，独立小步。
- 高级混合模式 × paint 级滤镜的组合（filtered 贴回两档均按 SourceOver premul 语义）——两档一致，登记后续。
- 负向对照：morphology 对偶破坏钉在批 1 值级钉内建（min/max 交换断言）；**真窗级负向已补**（见 §6）。

## 5. 证据

`evidence/P3.4/`（assessment §1-§8 / summary / apidoc-p34 / build-five-target / window 日志经 /tmp 归档摘要入 assessment §7-§8）；代码提交 `d3be922f`/`10f57f50`/`4f3971ce`/`3ae47859`/`3403f712`/`3ba0a8d3`/`baf4b802`；真窗 `examples/ui_wr_p34_filters/`（金图已播种，羽化 C 卡）。

## 6. C 卡专项收口（2026-11-01 第二会话，状态 ✅）

**上一会话的「bake dc gpuCtx 建立链」怀疑方向不成立**。诊断结论（三根因，全有单测/探针实证，详 assessment §8）：

1. **SDF 形状捷径吃 filtered paint**（真断链）：全仓对 `paint.MaskFilterRadius/ImageFilter` 的守卫只有 `FillPath`（gpu_render_context.go:1981）一处；矩形是 SDF 形状，`tryGPUOpRC→FillShape` 直接 `QueueShape` 画硬边返回 nil（cpu_fb=0、gpu_ops 照计）。旧加速器路径 `SDFAccelerator.FillShape` 委托同一 `rc.FillShape`——两条路由同洞。summary 排障实录 2 的「tryGPUOpRC 绕行条件扩 ImageFilter/MaskFilter」git 历史无落地（意图登记了、代码没写），本会话修正该记载。
2. **CPU 模式钉是空洞断言（假绿）**：`TestPaintMaskFilterSoftensEdges` 在不透明黑底上断言「边缘 alpha≠0」恒真；实测 CPU 模式纯硬边——CPU 侧 paint 滤镜零应用点，「CPU 钉绿证明羽化数学正确」的原结论不成立，已修正。
3. **gles CopyTextureToBuffer 硬编码 `gl.BGRA`**（go 档 C 卡蓝块）：读回字节序不看纹理格式，RGBA8Unorm 的 filter 图 RT 被 GL 驱动翻成 BGRA——纯红进蓝出（hook 微探针实证 `[0,0,255,255]`）；sdf resolve 是 BGRA8 所以存量链全对。修复=按 `srcTexture.format` 选 ReadPixels 格式（wgpu copyTextureToBuffer 契约对齐），mask_r8_modulate 等同类 RGBA8 读回一并纠正。

**修复面（A+B+C+D+GL，用户拍板「A+B 修+CPU 一并补齐」）**：

- A `tryGPUOpRC` 绕行条件扩（P3.2 Shader 同款）：`Shader≠nil ∥ len(ImageFilter)>0 ∥ MaskFilterRadius>0 → pathFn`（render/context_gpu.go）。
- B `GPURenderContext.FillShape` filtered 守卫 → `detectedShapeToPath→FillPath`（render/internal/gpu/gpu_render_context.go）。
- C 假绿钉改真钉：`TestPaintMaskFilterSoftensEdges` 改透明底+四点断言（内部不透明/边界羽化中间值/外向光晕/远场未触）；新增 `TestP34PaintMaskFilterGPURoute`（native 真机路由钉，render/p34_paint_filter_gpu_route_test.go）与 `TestP34GLOffscreenMaskFilterFill`（GL 档通道序+羽化钉，render/p34_gl_maskfilter_fill_test.go，含 hook 字节序微钉）。
- D CPU 版羽化 `fillPaintFilteredCPU`（render/context_fill_filtered.go，GPU `fillPaintFilteredAsImage` 的 CPU 孪生：内容光栅→blurXY CPU 闭包→ImageFilter CPU 节点→mask×clip 调制→premul SO 贴回）+ doFill CPU 回退接线 + `Fill()` 损伤区按 blur pad 外扩（羽化光晕不再被损伤裁剪）；GPU 侧 `fillPaintFilteredAsImage` 同步加 blur pad 外扩（Skia mask-filter 语义）。
- GL `gles/command_execute.go` ReadPixels 格式按纹理格式选择。

**验证（全绿）**：CPU 钉绿（真断言）；GPU 路由钉负向红（HEAD worktree 同钉「no outward halo」红）→修复绿；`-count=2 -race` 绿（顺带修 `TestApplyMergeAndDisplacement` 全局滤镜注册表不恢复的测试卫生洞——桩闭包覆写全局导致跨用例污染，加 save/restore）；真窗隔离 :3 native **6/6 探针+金图重播后逐位 0.0000%**、go 档 **6/6+金图逐位 0.0000%**（GL 修复后双后端像素一致）；真窗级负向：摘 A+B 两层守卫 → c_soft_edge 红（5/6 复现原始 bug）→ 还原绿；conformance 双后端 PASS；逐文件 315 PASS/6 FAIL=基线红名单逐名一致零新增（HEAD A/B 逐名复现定性）；构建全仓绿（`ui_wr_t_stress` C2 既有）；apidoc rc=0。route 对账：A/B 卡计数与 HEAD 逐位一致（4/1/2），仅 C 卡路由改变。

**顺带发现（登记，非本步）**：`render/recording/backends/raster/backend_coverage_test.go:24` vet 编译级错（FillRect 签名未跟 P3.1 变更，HEAD 同错）——recording 域既有缺口入台账待办池；go+GOGPU_RENDER_MODE=cpu 组合下 A 卡 erode 探针红（CPU 滤镜链与 GPU 图的边缘语义差，cpu 组合非真窗验收档，登记观察）。
