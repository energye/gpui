# P3.1b 完整线性工作空间 —— 影响面评估与分批计划（2026-11-12）

> 任务：`docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` 主表 P3.1b（2026-11-12 立项）。
> 依据：P3.1 收口遗留（`evidence/P3.1/assessment.md` §1.3/§2/§7 D1、`summary.md` §4）；M5 已完成的管线格式参数化（`evidence/M5/closeout.md` §1/§2）。
> **标准对齐复核（2026-11-12，补充）**：Skia 的默认表面色空间是 sRGB，`SkColorSpace::MakeSRGBLinear()` 是显式 opt-in，`SRGBToLinearGamma`/`LinearToSRGBGamma` 是显式转换滤镜；Flutter/Impeller 用 `DlColorSpace::kSRGB/kExtendedSRGB/kDisplayP3`，并提供 `SrgbToLinearFilter`/`LinearToSrgbFilter`，**没有全局线性工作空间**。因此「全局 A 完整线性工作空间」不是标准默认；标准形态是**按表面/按目标选色空间 + 边界显式转换**。
> **修订建议**：P3.1b 改为 **per-surface/per-target opt-in 线性目标**（默认 sRGB 表面不变，linear 目标才走硬件线性混合；边界用显式转换），既对齐 Skia/Flutter，也避免全局 CPU/GPU 线性化与全量基线重采。下方 A 路线保留为备选，但不再推荐。
> 性质：**新能力 + 行为边界选择**。本文件只做评估与分批计划，不改代码；拍板后再写会红的能力测试与真窗。

## 0 能力定义与目标

**完整线性工作空间**：渲染器内部所有颜色运算（渐变插值、AA/覆盖、图片采样、固定功能/高级混合、滤镜、层合成、CPU 软件光栅）统一在**线性光域**进行；最终写 sRGB 目标时由硬件编码回 sRGB（CPU 路径在输出边界编码）。

- 不是「只切 sRGB 目标格式」：目标格式只是最后一层；输入颜色/图片/渐变、shader 中间数学、CPU 光栅/滤镜都必须线性化，否则会出现「线性目标 + sRGB 中间值」的双重编码/偏色。
- 固定功能 13 模式天然在线性域混合；advanced 15 模式与 CPU 两套路径也要线性。
- 这是 HDR/广色域/ICC 的最终工作空间形态；P3.1b 是这条线的地基。

## 1 现状（2026-11-12 实测）

| 层 | 现状 | 位置 |
|---|---|---|
| 管线 target 格式 | M5 已参数化：owner 带 `colorFormat`，零值=BGRA8；43 处描述符已跟随 | `render/internal/gpu/color_format.go:24-31`、`convex_renderer.go` 等 |
| 渲染目标格式来源 | `Flush` 从目标 view 的 texture format 取 `targetFormat`（M5 P4） | `render/internal/gpu/gpu_render_context.go:2949-2962` |
| 离屏格式白名单 | 只允许 `BGRA8Unorm` / `RGBA8Unorm` / `RGBA16Float`，**不含 sRGB** | `render/internal/gpu/offscreen_format.go:36-41` |
| native swapchain 格式 | 能力列表里优先 `BGRA8Unorm` / `RGBA8Unorm`，**不选 sRGB** | `gpu/webgpu/swapchain_configure.go:163-170` |
| gles swapchain 格式 | 同样优先 Unorm；默认 BGRA8 | `gpu/gwgpu/gles/swapchain.go:76,116-118,154-161` |
| sRGB 枚举/映射 | 双后端已就绪：gles `SRGB8_ALPHA8`、webgpu 枚举、metal 映射 | `gpu/types/texture.go:73-84`、`gpu/gwgpu/gles/device_linux.go:1003-1008`、`gpu/gwgpu/gles/adapter.go:143-144` |
| GPU shader 输出 | 所有 fragment shader 直接返回 sRGB 域颜色（`in.color * coverage`、采样值×opacity 等），无 gamma 代码 | `render/internal/gpu/shaders/*.wgsl` 的 `fs_main` |
| 固定功能 blend | 13 模式用硬件 blend state，混在 sRGB 编码域；linear 请求显式降级 | `render/internal/gpu/blend_gpu.go:57-184` |
| advanced blend | 15 模式走 dual-tex shader；P3.1 已有 linear 分支 | `render/internal/gpu/dual_tex_blend.go`、`shaders/dual_tex_blend.wgsl` |
| CPU 混合 | `internal/image/draw.go blend()` 与 `software_blend.go compositeAdvanced` 都在 sRGB 域；P3.1 的 linear 只在 opt-in 分支 | `render/internal/image/draw.go:322-329`、`render/software_blend.go:32-217` |
| CPU 滤镜 | `render/internal/filter`、`filter_ops_cpu.go` 在 sRGB 字节域 | `render/internal/filter/`、`render/filter_ops_cpu.go` |
| CPU 读回/存储 | pixmap 存 premultiplied RGBA sRGB 字节；`SavePNG` 直接编码 | `render/pixmap.go`、`render/context_output.go:61-66` |
| GLES sRGB FBO 状态 | 仓内**未见** `GL_FRAMEBUFFER_SRGB` 开关；sRGB FBO 的写/混合/blit 语义未验证 | `gpu/gwgpu/gles/*.go` |
| 公开颜色 API | `render.RGBA` 是 sRGB 编码浮点；渐变 stop/图片解码也按 sRGB 约定 | `render/color.go`、`render/paint.go` |

## 2 已拍板路线 A：完整线性工作空间

### 2.1 输入边界

- **公开 API 保持 sRGB 语义**：`render.RGBA`、`GradientStop`、图片解码结果仍按 sRGB 编码解释；调用方不需要改颜色字面量。
- 进入渲染器时统一转线性：实色/渐变 stop 在 brush/命令构建时转线性；图片保留 sRGB 字节，但上传为 sRGB 纹理（采样自动解码）或采样时手动解码；字形颜色/彩色图集同理。
- 这样既不改公开颜色契约，又保证 shader/光栅内部是线性域。

### 2.2 GPU 路径

- 所有 fragment shader 在**线性域**计算：颜色 uniform 已是线性；图片/图集采样若来自 sRGB 纹理则硬件解码为线性；渐变 stop 线性插值；覆盖/AA 乘 alpha（alpha 不做 gamma）。
- 输出到 `*UnormSrgb` 目标，硬件编码回 sRGB；固定功能 blend 自动在线性域混合。
- advanced dual-tex 采样目标（sRGB）得到线性 dst，按线性域公式混合；P3.1 的 advanced linear 分支转正。
- 所有 render target（swapchain / 离屏 / 层 / mask 合成 RT）统一 sRGB；源纹理（图片/图集）用 sRGB 格式；管线 target 格式由 M5 的 `colorFormat` 跟随。
- 读回：sRGB 目标读回字节是 sRGB 编码值，`SavePNG`/CPU 滤镜按 sRGB 处理；CPU 滤镜若做线性运算则先解码、算完编码回。

### 2.3 CPU 路径（与 GPU 必须逐位/容差对齐）

- `internal/image/draw.go blend()`、`software_blend.go compositeAdvanced`、`internal/filter`、渐变插值、字形覆盖合成统一改线性域；输出到 pixmap 前编码回 sRGB。
- `Pixmap` 的公开字节约定仍是 premultiplied sRGB（`SavePNG`/`ToImage` 不破）；CPU 内部用线性中间缓冲，边界转换。
- P4.2 GPU/CPU 模糊一致、P2.6 金图、conformance 的 CPU 腿都会受影响，必须逐项重对。

### 2.5 标准对齐结论（2026-11-12 复核）

| 框架 | 色空间模型 | 线性混合怎么发生 | 对 P3.1b 的含义 |
|---|---|---|---|
| Skia | 表面/图像自带 `SkColorSpace`；默认 `MakeSRGB()`，`MakeSRGBLinear()` 显式 opt-in | 混合在**目标色空间**发生；线性目标才线性混合；`SRGBToLinearGamma`/`LinearToSRGBGamma` 是显式转换滤镜 | 不全局切线性；按表面/目标选色空间 |
| Flutter/Impeller | `DlColorSpace::kSRGB` / `kExtendedSRGB` / `kDisplayP3`；无线性工作空间枚举 | `SrgbToLinearFilter` / `LinearToSrgbFilter` 显式转换；默认 sRGB 表面 | 不全局切线性；显式转换 + 按需目标 |

**结论**：全局 A（完整线性工作空间）**不是 Skia/Flutter 的标准默认**，会把默认混合语义、CPU/GPU 存储边界、全量金图一起改掉，范围和风险都过大。标准对齐的落法是：

- **默认 sRGB 表面/目标不变**（P2.6 基线不破）。
- **新增 opt-in 线性目标/表面**：只有显式请求 linear 的目标才用 sRGB 目标格式 + 硬件线性混合；固定功能 13 模式在 linear 目标上不再降级。
- **边界显式转换**：sRGB→linear / linear→sRGB 用与 Skia/Impeller 同形的转换函数/滤镜，不把全局中间数学改线性。
- **验证**：新增 `ui_wr_p31b_srgb_chain` 的 linear 目标卡；默认窗全部保持旧基线；只对新增 linear 目标出金图。
- **`BlendSpaceSRGB` 默认不变**：linear 只在显式目标/请求上生效；不需要删枚举或双目标全量合成。

这是 **surface-level B2**：比「全局 A」小得多，也比「per-draw 双目标」干净（色空间是目标属性，不是每条绘制属性）。

### 2.6 BlendSpace 语义（修订）

修订路线（per-surface opt-in）下，**默认 `BlendSpaceSRGB` 不变**：

- 默认 sRGB 表面/目标：固定功能 13 模式仍按 sRGB 域混合（P2.6 基线不破）；linear 请求继续按现有降级/计数处理。
- opt-in linear 目标/表面：只有显式创建/选择的 linear 目标才走 sRGB 目标格式 + 硬件线性混合；固定功能 13 模式在该目标上不再降级。
- `BlendSpaceLinear` 的生效域 = linear 目标上的固定功能 + advanced + CPU；`BlendSpaceSRGB` 保持默认语义。
- 不需要删除枚举、不需要 per-draw 双目标全量合成；边界转换用显式 `SRGBToLinear`/`LinearToSRGB`（对齐 Skia/Impeller）。

如果产品后续决定把**默认**也切线性，再按 A 路线单独立项；P3.1b 先交付 opt-in 线性目标，不赌全局默认。

## 3 分批计划（修订：per-surface opt-in）

| 批 | 内容 | 门禁 |
|---|---|---|
| P3.1b-0 | 1×1/小图探针：sRGB FBO 写/混合/blit/readback、`GL_FRAMEBUFFER_SRGB`、webgpu sRGB swapchain/offscreen 格式 | ✅ 2026-11-12 四项探针 PASS |
| P3.1b-1 | 颜色转换地基：显式 `sRGB↔linear` GPU WGSL + CPU Go 函数；`CreateOffscreenTextureFormat` 加 sRGB 白名单；单测先红后绿 | ✅ 2026-11-12 单测 + GPU/CPU 逐值对拍 PASS |
| P3.1b-2 | opt-in linear 目标：sRGB 目标格式 + `BlendSpaceLinear` 固定功能 13 模式不降级；管线/池键/读回跟随 | 待开工 |
| P3.1b-3 | 线性目标呈现/合成：linear RT → 呈现前显式 `LinearToSRGB`（或目标本身是 sRGB swapchain）；层/backdrop 边界转换 | 待开工 |
| P3.1b-4 | 新真窗 + 负向 + 收口；默认窗全部保持旧基线，不重采 | 待开工 |
| P3.1b-5 | 旁路：若后续要全局默认线性，按 A 路线单独立项（不在本项） | — |

## 4 验证方案

### 4.1 能力单测（先红后绿）

- `sRGB↔linear` 函数：GPU WGSL 与 CPU Go 逐值对拍（1 LSB 内）；边界 0/1/中间调。
- 目标格式选择：sRGB 可用 → 选 sRGB；不可用 → Unorm + fallback 计数。
- 管线格式跟随：linear 目标下固定功能 13 模式不再降级；默认 sRGB 目标仍降级并计数。
- 渐变/图片：显式转换后的线性值 vs CPU 参考；默认 sRGB 路径不变。
- advanced dual-tex：linear 目标采样 dst 线性域混合；与 CPU 参考对拍。
- GLES：`SRGB8_ALPHA8`、`GL_FRAMEBUFFER_SRGB`、sRGB↔Unorm blit/readback。
- 读回：sRGB FBO 字节与 `SavePNG`/`LinearToSRGB` 约定一致。

### 4.2 真窗（新增）

`examples/ui_wr_p31b_srgb_chain`：

- 同内容默认 sRGB 目标 vs opt-in linear 目标对照；固定功能 13 模式 + 半透明叠加 + 灰阶中间调；linear 目标呈现前显式转换。
- JSON 探针：实际目标格式、固定功能/advanced 线性分支计数、GPU/CPU 回退、读回编码。
- 金图：新窗新金图，容差 0；默认窗旧金图不动。
- 双后端：native 必跑，go 必跑；metal 交叉编译（本机不可验）。

### 4.3 负向对照

- 强制 Unorm 目标 → 固定功能 linear 降级/像素差红。
- 摘 linear 目标的 sRGB 格式/输出转换 → 颜色偏差异红。
- 摘默认 sRGB 路径 → 旧金图红（证明默认没被改）。

### 4.4 回归与基线

- **默认路径零变化**：P2.6 旧金图全部保持 0.0000%（本项不重采全量基线）。
- 新窗出新金图；linear 目标卡单独断言。
- conformance、`-race`、五目标构建、vet、API 快照同步。
- 性能：linear 目标多一次显式转换；默认路径无新增开销；p95/p99 红线不破。

## 5 拍板项（标准对齐复核后修订）

| # | 问题 | 推荐 |
|---|---|---|
| D1 | 技术路线 | **per-surface/per-target opt-in 线性目标（surface-level B2）**；A 全局线性工作空间非 Skia/Flutter 标准默认，若要做另立专项 |
| D2 | 默认语义 | **默认 `BlendSpaceSRGB` / sRGB 目标不变**；linear 只在显式 linear 目标上生效；不删枚举、不做 per-draw 双目标 |
| D3 | CPU/GPU | 默认 CPU/GPU 都保持 sRGB；linear 目标是 GPU 硬件路径，CPU 侧只提供显式转换参考/对拍，不改全局 CPU 光栅 |
| D4 | 图片/图集 | 默认 Unorm 不变；linear 目标 shader 输出侧转换；图片 sRGB 纹理作为后续优化另议 |
| D5 | 基线 | **默认路径零变化，旧金图不重采**；只新增 linear 目标窗的新金图 |
| D6 | GLES 策略 | **先探针后整链（已拍板）**：sRGB FBO/`GL_FRAMEBUFFER_SRGB`/blit/readback 探针先行 |
| D7 | 公开面 | `CreateOffscreenTextureFormat` 加 sRGB；目标格式/降级计数进 JSON；`BlendSpace` 默认值不变 |

## 6 风险

1. **默认路径必须零变化**：任何全局 shader/CPU 改动都可能破 P2.6 金图；本项把改动限制在 opt-in linear 目标/管线变体。
2. **linear 目标的 shader 变体**：输出转换/advanced 采样线性域需要管线键与分支；默认 sRGB 路径不启用。
3. **GLES sRGB FBO/blit/readback**：`GL_FRAMEBUFFER_SRGB`、sRGB↔Unorm blit、`glReadPixels` 编码语义；先 1×1/小图探针。
4. **线性目标呈现**：linear RT 到 sRGB swapchain 的转换/合成要显式；不能把线性字节直接当 sRGB 显示。
5. **性能**：linear 目标多一次输出转换；默认路径无新增开销；p95/p99 红线不破。
6. **跨平台**：Windows/macOS/Metal 本机不可验；只交叉编译，运行时标「待验证」。

## 7 状态

- P3.1b 🟡 2026-11-12：路线 **opt-in 线性目标**；默认 sRGB 不变、默认金图不重采。
- P3.1b-0 ✅：四项探针 PASS（native sRGB target、GLES sRGB FBO/`GL_FRAMEBUFFER_SRGB`、X11 sRGB swapchain、GLES sRGB↔Unorm blit）。
- P3.1b-1 ✅：`colorSpaceWGSL` + Go `srgbToLinear`/`linearToSRGB` 单一真源；GPU/CPU 256 点逐值对拍 PASS；`CreateOffscreenTextureFormat` sRGB 白名单 + 4 B/px 账本/clear 测试 PASS；dual-tex 改用共享 snippet，既有测试全 PASS。
- 下一步：P3.1b-2（linear 目标固定功能 13 模式不降级 + 输出转换接线），再 P3.1b-3（呈现/合成）。

## 8 P3.1b-0 探针结果（2026-11-12）

### 8.1 native webgpu（`TestP31bSrgbTargetProbe`）

- sRGB 目标 `RGBA8UnormSrgb`：清黑 + 线性 (0.5,0.25,0.1) → 读回 sRGB 编码；50% alpha 线性灰叠加 → **线性混合**。
- 实测：`sRGB blend bytes=[188 165 149 191]`；Unorm 对照 `[127 95 76 191]`。
- 结论：webgpu sRGB target 可建/可写/可混合/可 `CopyTextureToBuffer` 读回；线性混合语义成立。
- 证据：`probes/native-webgpu.txt`。

### 8.2 GLES（`TestP31bSrgbFBOProbe`）

- `SRGB8_ALPHA8` FBO + `GL_FRAMEBUFFER_SRGB` 开启：base `[188 137 89 255]`（sRGB 编码）、blend `[188 165 149 191]`（线性混合）。
- 关闭 `GL_FRAMEBUFFER_SRGB` / Unorm FBO：base `[127 64 25 255]`、blend `[127 95 76 191]`（编码域）。
- 结论：GLES 腿可走 sRGB FBO；`GL_FRAMEBUFFER_SRGB` 必须在 sRGB FBO 绑定期间开启，否则写入/混合按编码域。
- 证据：`probes/gles-srgb-fbo.txt`。

### 8.3 swapchain sRGB 格式探针（webgpu X11）

- X11 surface 广告格式：`[BGRA8UnormSrgb BGRA8Unorm]`（sRGB 优先可用）。
- `BGRA8UnormSrgb` Configure + BeginFrame + clear + EndFrame/Present **PASS**。
- 结论：native X11 swapchain 可直接走 sRGB 格式；Wayland/Windows 仍需各自 surface 探针（本机 Wayland 可后续补）。
- 证据：`probes/webgpu-swapchain.txt`。

### 8.4 sRGB↔Unorm blit 探针（GLES）

| 组合 | `GL_FRAMEBUFFER_SRGB` | 读回 | 语义 |
|---|---|---|---|
| sRGB FBO base | 开 | `[188 137 89 255]` | 线性 (0.5,0.25,0.1) 编码成 sRGB |
| Unorm FBO base | 关 | `[127 64 25 255]` | 原始值 |
| sRGB→Unorm | 开 | `[128 64 26 255]` | 解码成线性 |
| sRGB→Unorm | 关 | `[188 137 89 255]` | 原样拷贝 |
| Unorm→sRGB | 开 | `[187 137 88 255]` | 编码成 sRGB |
| Unorm→sRGB | 关 | `[127 64 25 255]` | 原样拷贝 |

- 结论：GLES blit 合法，无 `GL_INVALID_OPERATION`；`GL_FRAMEBUFFER_SRGB` 决定「解码/编码」还是「原样拷贝」。
- 对 present 路径的含义：若 linear 目标是 sRGB FBO、窗口 FBO 0 不是 sRGB，present blit 应**关闭** `GL_FRAMEBUFFER_SRGB` 原样拷贝 sRGB 字节（否则会被解码成线性、显示变暗）；若 FBO 0 本身是 sRGB，则按实际颜色空间决定。P3.1b-2 实现时必须显式管理该状态。
- 证据：`probes/gles-srgb-blit.txt`。

### 8.5 待补探针

- `CreateOffscreenTextureFormat` 加 sRGB 后的引擎级路径探针（P3.1b-1）。
- 固定功能 13 模式在 sRGB 目标上的逐模式像素探针（P3.1b-2）。
- Wayland/Windows surface sRGB 格式探针（本机仅 X11 已验）。

## 9 P3.1b-1 颜色转换地基（2026-11-12）

- 单一真源：`render/internal/gpu/color_space.go` 的 `colorSpaceWGSL`（WGSL 两份函数）+ Go `srgbToLinear`/`linearToSRGB`；`dual_tex_blend.go` 改用共享 snippet，删掉内联副本。
- 逐值对拍：`TestP31bColorSpaceGPUMatchesCPU` 用 256×1 RGBA8Unorm 目标把 WGSL 输出原样读回，与 CPU 函数逐点比，±1.5/255 内全过。
- 白名单：`offscreenEngineFormats` 加 `RGBA8UnormSrgb`/`BGRA8UnormSrgb`；`CreateOffscreenTextureFormat` 可建/清/记账（4 B/px），sRGB 与 Unorm 池键分开。
- 默认路径：`BlendSpaceSRGB`/Unorm 目标未改；dual-tex/blendspace 既有测试全 PASS。
- 证据：`p31b-1-tests.txt`。
