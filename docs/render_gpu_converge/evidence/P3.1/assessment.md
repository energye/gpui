# P3.1 影响面评估（2026-11-01）

> 任务：P3.1 色彩管理地基（含线性混合接线，前移项）。PLAN §7.4 / §6「线性空间混合」行 / catalog §13 P3.1 规划行。
> 依据：P3.0 依赖图（`evidence/P3.0/dependency-graph.md`，P3.1=第 1 位）；P3.0 assessment §1.1 现状；本评估两路深查（GPU 色彩格式链路 / 混合路径全图），HEAD `ff2e9fa5`。
> 性质：**新能力（P3）**——按台账 §4.1「先写会红的能力测试，再实现；真窗必须新增或扩展并出新金图」。方向待拍板后才动代码。

## 0. 能力定义（草案，拍板后定稿）

**线性空间混合**：绘制时把源与目标颜色从 sRGB 编码域解码到线性域、混合、再编码回 sRGB。消除 sRGB 域直接混合的中间调偏暗/过饱和（Skia「sRGB-aware blending」、Flutter linear working space 同语义）。

- **opt-in，默认 SRGB 域**：默认路径行为零变化（P2.6 的 374 轮基线与金图不破）；开启 linear 的绘制才走新路径。
- 数学已存在于 `render/internal/blend/linear.go:37-121`（unpremul → sRGB→linear → premul → blend → unpremul → linear→sRGB → premul；alpha 不做 gamma），全仓 0 生产调用——本步把它接进主路。

## 1. 关键现状（两路深查新事实，补 P3.0 评估）

### 1.1 GPU 混合是两条执行路（不是一条）

| 路径 | 覆盖模式 | 执行方式 | linear 接入点 |
|---|---|---|---|
| 固定功能 | 13 模式：Normal/Copy/Clear/Plus/Modulate + 8 个 Porter-Duff | 硬件 blend state（`blend_gpu.go:57-184` 手工 factor 表），混在 **sRGB 编码域** | 固定功能无法插转换——正解是 **sRGB 目标格式**（硬件 blend 自动 linear 化），shader 伪造是绕路 |
| advanced（dual-tex） | 15 模式：Multiply/Screen/Overlay/HSL 族/Darken…Exclusion | **shader 管线**采样 dst+src 合成（`dual_tex_blend.go`，模式码 1..15） | shader 内插转换即可——uniform 标志或 shader 变体，**不动管线布局** |

路由：`FillPath` advanced → `fillAdvancedBlendAsImage`（`gpu_render_context.go:1983-1996`）；全不支持才 `ErrFallbackToCPU`。

### 1.2 全链路「字节=sRGB 编码值」单一约定

- 约 **50 处** render pipeline `ColorTargetState` 硬编码 `BGRA8Unorm`（convex/stencil/depth_clip/image/glyph_mask/dual_tex 各文件）；swapchain 格式白名单只认非 Srgb（`swapchain_configure.go:154-158`、gles 同 `gles/swapchain.go:159-165`）；离屏/层 RT/图片纹理全部 Unorm；shader 输出无 gamma 代码；读回无 gamma 处理。
- 图片上传：解码器 sRGB 字节原样 `WriteTexture`（`image_cache.go:355-414`）——若 RT 切 Srgb，此路径天然兼容（Unorm 采样不解码 + 输出时一次编码）。
- hal/types 的 Srgb 枚举与双后端映射**已就绪**（`types/texture.go:73-84`；gles `device_linux.go:986-991`→SRGB8_ALPHA8、metal `conv.go:56-67`→MTLPixelFormat*SRGB）。

### 1.3 sRGB 目标格式路线（全局切线性工作空间）的改动面与代价

- 改动面：swapchain 白名单 ×2 后端 + **~50 处管线 target 格式参数化**（wgpu 要求管线 target==attachment 格式）+ 离屏 resolve RT 联动 + GLES blit 跨 sRGB 合法性与 glReadPixels 读回语义待实测。
- 代价：**全局行为变化 → P2.6 冻结的全部像素金图与 parity 断言失效，需要重采/重冻结基线**。
- 定性：这是色彩管理/HDR 的**最终工作空间形态**，是整链专项（宜独立立项），不是「线性混合能力」本身。

### 1.4 CPU 混合三套并存（linear 接线要动生产在用的两套）

| 实现 | 消费方 | 状态 |
|---|---|---|
| `internal/image/draw.go blend()`（draw.go:322-329） | `Context.compositeLayer` 层合成（`context_layer_blend.go:87-104`）+ scene CPU 合成（`scene/renderer.go:971-1000`） | **生产在用** |
| `software_blend.go compositeAdvanced`（:32-217） | 软件填充 advanced 模式（`software_fill.go:745,813`）；**独立内联公式**（"keep in sync" 注释 :297），GPU 不支持时回退链也落这里 | **生产在用** |
| `internal/blend GetBlendFunc` | `blend.NewLayerStack`/`BlendBatch`（生产 0 调用）+ scene 便捷 API | 半备而未用 |

`GetBlendFuncLinear` 内部有两次 u8 量化往返（linear.go:86-87、:96-101）——接主路时应消掉（float 域直通）或声明精度口径。

### 1.5 录制/回放与 scene 线格式

- blend 是**逐命令携带**：`FillPathCommand/StrokePathCommand/FillRectCommand/StrokeRectCommand.Blend`（`recording/command.go:307-351`）+ `DrawImageCommand`（携 DrawImageOptions）+ `PushLayerCommand{Blend,Opacity}`；回放 `CanvasBackend.withBlend` save/restore（`backend_canvas.go:229-238`）。Paint 加字段则这些结构体同步加字段、回放加口——机械改动，面清晰。
- **scene 线格式 blend 只在 Layer 级**（`encoding.go:660-662` 2-word 布局，解码步进假设 :1091）——层级 linear 动线格式会破坏步进兼容。第一版**不动 scene 线格式**。

### 1.6 batching 正确性约束（不只是性能）

三处管线缓存按 `render.BlendMode` 键控：convex（`convex_renderer.go:150-151`）、stencil cover（`stencil_renderer.go:125`）、textured quad（`image_pipeline.go:408-460`）。batch 合并谓词同样只看 BlendMode：convex range 分段（:1192）、`canMergeImageDraw`（:1007-1010 "never share a batch"）。**linear 开关必须进管线缓存键与合并谓词，否则相邻异空间绘制错用管线——是正确性 bug 不是性能问题**。

### 1.7 ui 层

ui 全目录只写 `BlendNormal`（`ui/scene/composite.go:207`、`textured.go:1685`）；Paint/Options 加字段 ui 层零改动。

## 2. 技术路线（拍板项 D1）

**路线 B（推荐）：混合级 opt-in，第一版覆盖 advanced+CPU**
- advanced 15 模式：dual-tex shader 加 linear 变体（uniform 标志优先，两后端一份 WGSL）；管线缓存键与 batch 谓词加 linear 维度。
- CPU 全模式：`GetBlendFuncLinear`（float 域直通改造）接 `image/draw.go blend()` 与 `compositeAdvanced` 两处生产路径。
- 固定功能 13 模式：**显式降级**——linear 请求落固定功能路径时按 SRGB 域执行并在探针/文档登记（双端一致），不伪造 shader 合成（性能不可接受：Normal 是大头，全走读 dst 合成会拖垮主路）。
- 固定功能 linear 的正解=sRGB 目标格式切换 → **立项 P3.1b（本步收口后紧跟）**：全局切 Srgb 变体（swapchain+~50 管线 target+RT 联动），覆盖固定功能 13 模式，作为 HDR/广色域前置整链专项。届时按行为变化流程重采基线。

**路线 A（不推荐本步做）：本步一口气全局切 Srgb 目标格式**——能力一步到位但 P2.6 基线全废重采，工程量与本步目的（线性混合能力+API 地基）不匹配。

**路线 C（否决）：固定功能 linear 走 shader 读 dst 合成**——Normal 全变慢路径，性能红线。

## 3. 公开 API 草案（拍板项 D2）

| 口 | 形态 A（枚举，推荐） | 形态 B（布尔） |
|---|---|---|
| Paint | `Paint.BlendSpace BlendSpace`（`BlendSpaceSRGB`=0 默认 / `BlendSpaceLinear`） | `Paint.BlendLinear bool` |
| 图片绘制 | `DrawImageOptions.BlendSpace` | `DrawImageOptions.BlendLinear` |
| Context 状态 | `SetBlendSpace/BlendSpace()`（对齐 `SetBlendMode` 惯例，canvas.go 接口 + 录制面） | 同左 |
| Clone | Paint.Clone 复制（:139-161 补一行） | 同左 |
| 内部先验 | `internal/blend.BlendModeLinear{Mode,Linear}` 已是 {mode,linear} 二元组，枚举形态对齐更顺 | — |

推荐枚举：色彩管理后续（广色域/display-p3/HDR 工作空间）沿 `BlendSpace` 扩展，bool 会二次改名（接口全量切换制下无过渡期，现在就锁对的名字）。命名候选 `BlendSpace`（Skia 无同名冲突；Flutter 无对应词）。

## 4. 验证方案（拍板项 D4）

- **能力单测（先红后绿）**：①`render/internal/blend/linear_test.go` 扩展 float 直通等价钉；②路由单测（linear 请求 → advanced 走 dual-tex linear 变体、固定功能降级路径探针）；③管线缓存键/batch 谓词单测（同 mode 异 space 不合并、不错用管线）；④CPU 两套路径 linear 等价钉；⑤recording 四命令 linear 携带+回放往返钉。
- **真窗（新增）**：`examples/ui_wr_p31_linear_blend`——双卡对比（同内容 sRGB 域 vs linear 域：半透明渐变叠加、灰阶中间调混合，中间调差异最大处布像素断言点），JSON 探针（linear 生效标记+降级计数），出金图；按 AGENTS 满窗即内容、指标浮内容上；隔离 :3 跑，native 必跑 + go 档（dual-tex 两后端一份 WGSL，go 档应可验）。
- **金图**：新窗新金图，容差按 P0.1（默认 0；若 GPU 双后端线性混合实现差异需声明容差，写明依据）。
- **双后端**：webgpu native ✅ 必跑；gwgpu-gles go 档必跑（撞既有红如实登记）；metal darwin 交叉编译；固定功能 linear 降级在双端一致标注。
- **负向对照（必须红过再还原）**：摘掉 dual-tex shader 的 linear 转换分支 → 新窗 linear 卡像素断言红；或摘掉管线键 linear 维度 → ③单测红。
- **回归**：默认域行为零变化——P2.6 相关基线窗（miscdraw 混合卡等）重放一致 + conformance 双后端 0/0 不破 + 逐测试文件改动面。

## 5. 范围边界（拍板项 D3/D5）

- **D3 默认行为**：默认 `BlendSpaceSRGB`，opt-in——硬要求（P2.6 基线不破），列入能力定义不另行拍板。
- **D5 render/hdr GPU 面**（catalog §13 P3.1 行含「FloatTarget.TextureView() 激活」）：**推荐移出 P3.1 第一版**——它需要离屏 Float RT+读回+主路合成，是 HDR 管线本体，等 P3.7 前的 HDR 步做；本步只交付线性混合地基+色彩空间 API 雏形。拍板后同步改 catalog §13 注记。
- **scene 线格式**：第一版不动（Layer 级 linear 随 P3.1b/线格式演进再议）。
- **广色域/ICC/显示 P3**：不在 P3.1（PLAN §5.1 色彩管理行的这些子项随 HDR 专项）。

## 6. 影响面清单（预估，拍板后细化）

| 层 | 文件 | 改动 |
|---|---|---|
| render 公开 | `paint.go`、`context_image.go`（DrawImageOptions）、`context_layer_blend.go`、`canvas.go`、`blendmode.go`（新类型+常量） | +BlendSpace 类型/常量、3 个口、Clone 复制 |
| render 内部 | `internal/blend/linear.go`（float 直通改造）、`software_blend.go`、`internal/image/draw.go` | linear 接线 ×2 + 数学改造 |
| GPU | `dual_tex_blend.go`（shader 变体/uniform）、`blend_gpu.go`（降级探针）、三处管线缓存键、`canMergeImageDraw`、convex range 分段 | 键扩展+变体+谓词 |
| shader | `shaders/dual_tex_blend.wgsl`（linear 分支） | 一份 WGSL 双后端 |
| 录制 | `recording/command.go` 四命令结构体、`recorder_path_draw.go`、`backend_canvas.go` | 字段+回放口 |
| 新增 | `examples/ui_wr_p31_linear_blend/`、能力单测文件 | 先红后绿 |

## 7. 拍板项汇总

| # | 问题 | 推荐 |
|---|---|---|
| D1 | GPU 技术路线 | B：本步 advanced+CPU opt-in，固定功能 13 模式显式降级；sRGB 目标格式全局切换立项 P3.1b 紧随（HDR/广色域前置） |
| D2 | API 形态 | `BlendSpace` 枚举（SRGB 默认/Linear），Paint+DrawImageOptions+Context 三口 |
| D3 | 默认行为 | 默认 SRGB 域（opt-in，P2.6 基线零变化）——硬要求，确认即可 |
| D4 | 验证载体 | 新真窗 `ui_wr_p31_linear_blend`（双卡对比+探针+金图）+ 五组单测 + 负向对照摘 shader 分支 |
| D5 | hdr GPU 面 | 移出 P3.1 第一版，挂 HDR 专项（P3.7 前）；catalog §13 P3.1 行同步注记 |

## 8. 风险与不做的

- 不做：全局 sRGB 格式切换（P3.1b）、hdr GPU 面、scene 线格式、广色域/ICC。
- 风险：①GPU 双后端 linear 混合的数值一致性（shader 数学一份 WGSL，理论一致；实测差需容差依据）；②GLES dual-tex 路径的既有兼容面（go 档既有红如实登记，不阻塞 native 判定）；③CPU float 直通改造涉及 compositeAdvanced 独立公式与 image/draw.go 两处，等价钉必须先行。
- 尾巴知悉：I1（ladder Wayland 呈现卡死）不影响本步（不开 ladder）；C7/J5 已闭合；misc_draw wgpu 执行态与本步无涉。
