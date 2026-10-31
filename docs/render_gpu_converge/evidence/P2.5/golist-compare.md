# P2.5 go list 对比文档 — PLAN §3 目标结构 vs 实际包树（2026-10-31）

> 命令：`go list ./...`（HEAD `9bb2e99a`）。范围：`gpu/`、`render/` 全部包。
> 结论先行：**PLAN §3 目标条目 100% EXISTS、零缺失**；实际树比目标树多的部分分三类（目标语义内组成件 / 目标未列新增结构 / 后端目录里的示例），逐包登记理由如下；按 D1 拍板**零物理搬移**，登记后「`go list` 与目标结构差异为零（登记口径）」成立。三条硬规则核验见 §3。

## 1 gpu/ 全表（35 个进 list 包 + 8 个 windows 标签示例目录）

| 包 | 归类 | 定性与登记理由 |
|---|---|---|
| `gpu/hal` | 目标✅ | 唯一抽象接口层 |
| `gpu/hal/noop` | 新增结构 | 无 GPU 后端：测试缝与降级路径用，属 hal 的合法组成（P2.4 起测试台依赖） |
| `gpu/types` | 目标✅ | 类型与常量 |
| `gpu/context` | 目标✅ | 设备桥（gpucontext） |
| `gpu/webgpu` | 目标✅ | native 后端门面（Windows/Linux/macOS；Web 走 internal/browser） |
| `gpu/webgpu/internal/browser` | 组成件 | Web（syscall/js）后端实现，webgpu 的 internal 组成 |
| `gpu/webgpu/internal/thread` | 组成件 | GPU 操作线程抽象，webgpu 的 internal 组成 |
| `gpu/webgpu/examples/browser-compute` | 示例混居 | js/wasm 示例 main 混在门面目录（硬规则①轻微违；**登记豁免**，迁出为可选后续小步，D1 不绑定本轮） |
| `gpu/webgpu/examples/browser-test` | 示例混居 | 同上 |
| `gpu/rwgpu` | 新增结构 | **包名 `wgpu`**（目录≠包名）：wgpu-native 的 purego FFI 绑定层，`gpu/webgpu` 门面 30 文件引它；Web 路径不经它。PLAN §3 目标树漏列此层，本次补列（修订见 PLAN §3 注） |
| `gpu/rwgpu/examples/*`（15 个 main：7 进默认 list、8 个 windows 标签排除） | 示例混居 | 绑定层自带示例 main 混在 FFI 目录（硬规则①轻微违；**登记豁免**，同上可选迁出） |
| `gpu/gwgpu`（根，仅 doc.go） | 组成件 | 后端家族根说明文档 |
| `gpu/gwgpu/gles`（+`egl/`、`gl/`、`wgl/`） | 目标✅ | 目标注释「GL/EGL/WGL」即含三个组成子包 |
| `gpu/gwgpu/metal` | 目标✅ | Metal 后端 |
| `gpu/gwgpu/metal/mslmap` | 组成件 | MSL 入口点映射，metal 组成 |
| `gpu/gwgpu/indirect` | 新增结构 | 间接绘制跨度溢出安全算术，仅 metal 消费；通用算术但当前唯一消费者在 metal，登记留置 |
| `gpu/gwgpu/ffishim` | 新增结构（名字差） | 目标树写 `ffi/...`，实际名 `ffishim`：purego C-ABI 垫片，gles+metal 共用；PLAN §3 已按实际名修订 |
| `gpu/shader`（+`ir/`、`wgsl/`、`glsl/`、`msl/`） | 目标✅ | P2.1 迁入的唯一 shader 翻译器 |
| `gpu/shader/wgsl/internal/{parser,lower}`、`glsl/internal/codegen`、`msl/internal/codegen`、`internal/{registry,textutil}` | 组成件 | 翻译器 internal 实现细节 |

## 2 render/ 全表（33 个进 list 包 + 1 个 stress 标签排除）

| 包 | 归类 | 定性与登记理由 |
|---|---|---|
| `render`（根） | 目标✅ | 门面：Context/Canvas/Path/Paint/Text/Image（329 文件，54 个 `!nogpu` 等标签文件） |
| `render/gpu` | 新增结构 | GPU 注册包（`!nogpu`/`nogpu` 双形态，P1.1b B1 引擎层正解，A4 评估保留）；PLAN §3 未列，本次补列 |
| `render/filters` | 新增结构 | CPU 图像滤镜在 Context 上的注册入口 |
| `render/integration/ggcanvas` | 新增结构 | gg 2D 库 ↔ render 对接桥 |
| `render/examples/{compute_clip,compute_pipeline}` | 示例 | render 自带可运行示例（`!nogpu`） |
| `render/tests/stress` | 测试设施 | `stress` 标签测试专用（不进默认 list） |
| `render/scene` | 目标✅ | 保留场景 |
| `render/recording`（+`backends/raster`） | 目标✅ | 录制回放 |
| `render/text`（+`cache/`、`emoji/`、`hint/`(+`cmd/fdiff`)、`msdf/`、`tools/gen_autohint_scripts`） | 目标✅+组成件 | 排版与字形；四个子域为 text 实现组成（目标树未展开）；`tools/` 为字形工具脚本生成器 |
| `render/svg` | 目标✅ | SVG 图标子集 |
| `render/hdr` | 目标✅ | FloatTarget + 色调映射（P2.2 迁入） |
| `render/internal/gpu` | 目标✅ | 主路径（SDF/AdaptiveFiller/session/texture/text/mask/filter；216 文件） |
| `render/internal/gpu/res` | 新增结构 | GPU 资源生命周期（P2.4c 建的标准实现：registry/cache/gate）；本次补列 |
| `render/internal/gpu/tilecompute` | 新增结构 | Vello CPU 光栅化 1:1 移植（主路径组成；CPU 能力居于 gpu 目录下，因与 GPU tile 管线同构配对，登记留置） |
| `render/internal/image` | 目标✅ | 图像缓冲管理 |
| `render/internal/color` | 目标✅ | 色彩空间 |
| `render/internal/blend` | 目标✅ | 混合（P2.4e e3 值域合一后为边界唯一） |
| `render/internal/filter` | 目标✅ | 图像滤镜 |
| `render/internal/raster` | 目标✅ | CPU 光栅化基元 |
| `render/internal/cache` | 目标✅ | 通用缓存基元 |
| `render/internal/parallel` | 目标✅ | tile 并行渲染设施 |
| `render/internal/clip` | 新增结构 | 几何裁剪（路径/形状裁剪算法） |
| `render/internal/stroke` | 新增结构 | 描边展开→填充轮廓 |
| `render/internal/wide` | 新增结构 | SIMD 友好宽类型（批量像素处理） |
| `render/internal/conformance` | 测试设施 | P0.4 双后端一致性测试台（断言器+场景运行器） |
| `render/internal/testutil/imagediff` | 测试设施 | 图像 diff 测试工具 |

## 3 三条硬规则核验

1. **后端目录只放后端实现**：基本满足；两处轻微违=`gpu/rwgpu/examples`（15 main）与 `gpu/webgpu/examples`（2 main）示例混居——已登记豁免，迁出为可选后续小步。`gpu/webgpu` 内的 `gate.go`/`device_provider.go`/`gpucontext_helpers.go`/`lifecycle_hooks.go`/`vram.go` 属门面职责（建链/注册/桥接），不算违。
2. **通用图形能力在 render**：满足；唯一分层债=`render/internal/gpu/gpu_shared.go` 非测试直引 `gpu/rwgpu`（`WithForceNativeReleaseOnLost`，全仓 render 侧唯一直引）——按 D2 拍板本步收敛到 hal。render 非测试引后端全景白名单 7 文件（`backend.go`、`present_swapchain.go`、`present_swapchain_gl.go`、`register_gles.go`、`gpu/gpu.go`、`internal/gpu/gpu_shared.go`、`internal/gpu/vello_accelerator.go`），守卫测试按此白名单钉死。
3. **不存在同名子包**：满足——无 `render/render` 残留、无任何 X/X 同名嵌套；`gpu/shader/tmp`（dxil 产物）不存在。

## 4 结论

- 目标条目缺失：**0**；同名子包违例：**0**；硬规则①②违例：示例混居 2 处（豁免登记）+ gpu_shared 直引 1 处（D2 本步收敛）。
- 按 D1 拍板零搬移；上述登记 + PLAN §3 修订（补列 `rwgpu` 层、`ffishim` 实名、render 侧 `gpu/`/`filters/`/`integration/`、internal 补列 `clip/stroke/wide/conformance/testutil` 与 `gpu/{res,tilecompute}`）后，完成标准「`go list` 与目标结构差异为零或已登记理由」达成。
