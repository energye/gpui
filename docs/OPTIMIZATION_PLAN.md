# GPUI 渲染库底层优化与 WebGPU 架构迁移开发计划

> 版本：4.0 | 更新日期：2026-07-15 | 状态：**主线已切换** — 见 `docs/MAINLINE_PLAN.md` + `docs/SKIA_2D_CAPABILITY_MATRIX.md`
> 项目：github.com/energye/gpui
> 文档位置：/home/yanghy/app/projects/gogpu/gpui/docs/OPTIMIZATION_PLAN.md
>
> **重要（2026-07-15）**：执行优先级以精简主线为准：
> 1. `docs/SKIA_2D_CAPABILITY_MATRIX.md` — Skia 2D 全面能力表  
> 2. `docs/MAINLINE_PLAN.md` — S0→S1 rwgpu ABI → S2 webgpu → S3 render → S4 性能  
> 本文档其余章节保留为历史与细节档案；与主线冲突时以 MAINLINE 为准。杂项任务（控件层、过早性能大任务等）主线排除。

## 📋 项目概述

### 项目名称
GPUI 渲染库底层 WebGPU 迁移与性能优化 - 对标 Skia / Ant Design 级 UI 渲染

### 项目背景
GPUI 是从 gogpu 生态迁移而来的独立渲染库。当前架构调整的核心目标是：

```text
render/internal/gpu
  -> gpu/webgpu object facade
  -> gpu/rwgpu
  -> libwgpu_native.so / wgpu_native.dll / libwgpu_native.dylib
```

本轮迁移不是继续保留原 go-wgpu 作为 GPU 实现，也不是让 `render` 直接调用 `rwgpu`。目标是用 `gpu/webgpu` 作为面向渲染层的对象 API，并由 `gpu/webgpu` 内部绑定 Rust wgpu-native 能力。

已完成：
- ✅ 阶段一：代码迁移与基础清理
- ✅ 阶段二：goffi 替换为 purego（FFI 中间层）
- ✅ 阶段三：基础 ABI 与 native device / clear pass / readback 验证
- ✅ 阶段四 A：`render` 依赖方向收敛到 `gpu/webgpu`
- ✅ 阶段四 B：shader module、texture upload/download、clear pass、pipeline cache 进入真实 native 调用
- ✅ P0.A 首轮：`rwgpu` render pipeline primitive enum 转换修复并加 ABI 测试
- ✅ P0.B 首轮：`gpu/webgpu` descriptor 转换与 pointer lifetime 修复并加测试
- ✅ P0.C 首轮：公共 imagediff 工具与 `text_transform` CPU/GPU 程序化视觉诊断，不上传 PNG
- ✅ P0.D 首轮：glyph mask scale 与 GPU stroke hairline 语义修复
- ✅ P0.E 首轮：第一批必测 examples 的 CPU/GPU 程序化视觉基线已接入（含 images/text/cjk_text/scene/scene_gpu/gpu）
- ✅ P0.F 局部：GPU DrawImage 支持 CTM 旋转/缩放四角点；非 solid paint GPU fill 回退 CPU（images pattern）
- ✅ P0.F 局部：GPU glyph mask / MSDF 跳过 GID 0 (.notdef)；isCJK 扫描整串，修复 cjk_text body/mixed 覆盖膨胀
- ✅ P0.2：native texture format/readback 校准（RGBA8/BGRA8/R8、非 256 对齐 row pitch、clear-via-upload）
- ✅ P0.3：UI 关键 blend 固定像素测试（Normal/SourceOver/Copy/Plus/Multiply）+ premul/straight 边界文档
- ✅ P0.F：第一批必测 examples STRICT 视觉门禁全部 PASS；残留 AA/ fringe 差异已登记为可接受阈值

已完成（P1 首轮）：
- ✅ P1.0：`RenderPathStats`（gpu_ops/cpu_fallback_ops）接入 Context + visualcmd 日志 + STRICT 解析
- ✅ P1.2 首轮：GPU 固定像素 SourceOver/opaque/clip/hairline（真链 `render`→GPU→readback）

进行中：
- ⏳ **P1 渲染能力门禁**：复杂 UI 场景矩阵 + 分层硬证据（ABI → webgpu → render 真链路）
- ⏳ 阶段四 C：`rwgpu` ABI / enum / descriptor 映射全量审计（render 已用 + UI 控件会用到的子集）
- ⏳ 阶段四 D：语义精修升级为门禁级（premul/blend GPU 固定像素、clip/AA、fallback 可观测）
- ⏳ 阶段四 E：清理旧 stub / legacy helper，避免它们进入生产渲染链路
- ⏳ 阶段五：LCL surface handle / swapchain / 窗口渲染集成
- ⏳ 阶段六：性能优化、批处理、资源缓存、图集化（**仅 P1 门禁通过后启动**）

### 项目目标
将 GPUI 渲染库优化到能够支撑复杂 UI 控件库和高性能 2D 渲染的水平，能力对标 Ant Design 类控件库的渲染需求，性能方向对标 Skia。

必须同时满足：
- 渲染正确性：路径、文本、图片、渐变、clip、blend、alpha、transform、MSAA/resolve 与源实现一致或差异可解释。
- 架构稳定性：`render` 不直接依赖 `gpu/rwgpu`；Rust wgpu-native 绑定只通过 `gpu/webgpu` object facade 暴露。
- 动态库调用真实有效：测试必须真实加载 `libwgpu_native.so`，不能只通过 stub 或空跑编译。
- UI 控件库可用性：支持大量小图元、文本、图标、圆角、阴影、裁剪、透明叠加、滚动区域和重复绘制。
- 后续可维护性：ABI 绑定最终需要由工具从 wgpu-native header 自动生成，避免手写 ABI 长期漂移。

**范围与主线（2026-07-15 澄清）**：
- 本计划**只做渲染栈**：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`。
- **不包含** Ant Design 控件层/组件库实现；“对标 Ant Design / Skia”指 **render 层渲染能力与正确性标准**，不是开工写控件。
- 正确推进顺序：
  1. `rwgpu` ABI 与 native 绑定正确、可测
  2. `gpu/webgpu` facade 能力补全并对齐
  3. `render` 层语义/视觉达到 Ant Design 级 UI 所需的绘制能力 + Skia 方向质量
- P0 第一批 examples STRICT 只是“迁移可跑”门禁，**不足以**声明 render 已达 Ant Design/Skia 级能力。
- P1 是 **render 能力门禁**（复杂场景 + 分层硬证据），关闭后表示底层渲染可用于后续独立的控件库项目，**不是本仓库的控件层交付**。

### 库结构
```
gpui/
├── render/              # 2D 渲染库（原 gg）
│   ├── context.go       # 核心 Context
│   ├── path.go          # 路径系统
│   ├── gradient.go      # 渐变
│   ├── text.go          # 文本渲染
│   ├── software.go      # CPU 光栅化
│   ├── gpu/             # GPU 加速
│   ├── internal/        # 内部实现
│   ├── raster/          # CPU 光栅化
│   ├── scene/           # 场景图
│   └── examples/        # 示例程序
├── gpu/                 # GPU HAL 层（原 wgpu）
│   ├── webgpu/          # 面向 render 的 WebGPU 对象 facade，内部调用 rwgpu
│   ├── rwgpu/           # Rust wgpu-native 的 Go 绑定层
│   ├── shader/          # 着色器
│   └── types/           # 类型定义
├── ffi/                 # FFI 中间层（purego）
│   ├── ffi.go           # FFI 实现
│   └── types/           # 类型定义
└── docs/                # 文档
```

### 已有任务计划
- `TASK_PLAN.md` - 迁移和 FFI 替换任务（已完成）

### 当前状态
| 组件 | 状态 | 说明 |
|------|------|------|
| render（原 gg） | ⚠️ 可运行，首个视觉问题已收敛 | 2D 渲染核心；`text_transform` 的 scale glyph mask 与 0.5px crosshair 已有程序化诊断和首轮修复，仍需扩大 examples 覆盖 |
| gpu/webgpu | ⚠️ 已接通但需继续校验 | 作为 render 层对象 API；已修复 StencilOperation 和 vertex attribute lifetime，其他 descriptor 仍需系统审计 |
| gpu/rwgpu | ⚠️ 可调用但未完整 | Rust wgpu-native 绑定；primitive topology/frontFace/cullMode 已修复，其他 ABI/enum/descriptor 仍需继续审计 |
| ffi | ✅ 完成 | purego FFI 中间层 |
| text | ✅ 可用 | 文本渲染 |
| scene | ✅ 可用 | 场景图 |

---

## 🧭 当前固定架构与边界

### 目标调用链

```text
render.Context / render examples
  -> render/internal/gpu.GPURenderContext / GPUSceneRenderer / GPURenderSession
  -> gpu/webgpu.Device / Queue / Texture / Buffer / RenderPipeline / CommandEncoder
  -> gpu/rwgpu
  -> wgpu-native dynamic library
```

### 强制边界

1. `render` 层不得直接 import `github.com/energye/gpui/gpu/rwgpu`。
2. `render` 层不得重新接回旧 go-wgpu HAL/core 路径。
3. `gpu/webgpu` 是 render 层唯一稳定 GPU 对象入口。
4. `gpu/rwgpu` 负责 Rust wgpu-native ABI 绑定与低层对象生命周期。
5. legacy stub helper 只能用于旧单元测试，不得作为生产渲染路径。

### 当前已验证事实（2026-07-14）

本地动态库：

```bash
WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
GOCACHE=/tmp/gpui-go-cache
```

已通过验证：

```bash
env WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so \
  GOCACHE=/tmp/gpui-go-cache \
  timeout 180s go test -count=1 ./render/internal/gpu

env WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so \
  GOCACHE=/tmp/gpui-go-cache \
  timeout 120s go test -run '^$' ./gpu/webgpu/... ./gpu/rwgpu/... ./render/internal/gpu ./render
```

当前已真实调用 native 的能力：
- 最小 device 创建
- clear render pass + submit
- texture upload / download readback
- shader module 创建
- blit / blend / strip / composite pipeline 创建
- `render` 包不直接 import `gpu/rwgpu`

### 当前完成度判断（2026-07-15）

总体完成度约 70%-74%（相对迁移与 P0）；相对 **Ant Design/Skia 底层门禁** 仍明显不足。

已经完成的是“架构方向”“部分真实 native 对象能力”“P0.A–F / P0.2 / P0.3 正确性门禁”。
第一批必测 examples STRICT 视觉诊断、format/readback、软件侧 UI blend 固定像素测试均已通过。
尚未完成的是：
- P1 渲染能力门禁（复杂 UI 绘制场景 + GPU 固定像素 + fallback 可观测）
- “ABI 完整性证明”“`gpu/webgpu` facade 到 `gpu/rwgpu` 的全量转换正确性”
- `render` 对标 Ant Design/Skia 所需绘制语义的补全与回归
- 跨平台动态库绑定生成

当前阶段位置：

```text
阶段 1：迁移结构搭起来       已完成
阶段 2：去掉 go-wgpu 主路径   基本完成
阶段 3：rwgpu native 可调用   部分完成
阶段 4：ABI/descriptor 完整   部分完成，继续审计
阶段 5：render 语义对齐       P0 完成；P1 控件原子/GPU 固定像素进行中
阶段 6：视觉回归稳定         P0 examples 绿；P1 复杂矩阵未完成
阶段 7：跨平台完整化         未开始
阶段 8：（本计划外）上层控件库   不在本计划交付范围
```

在 **P1 渲染能力门禁** 与阶段 4/5 完成前，不允许声称 render 已达到 Ant Design/Skia 级渲染能力，也不应该用性能优化掩盖正确性缺口。

### 当前冻结规则

从 2026-07-15 起，P0 正确性门禁已通过；后续仍保持以下约束：

1. 不继续扩大 GPU feature surface，除非是为了修复当前 render 已调用 API 或接入 Stage 5 surface。
2. 大规模批处理、atlas、cache、排序等性能优化仍冻结到 **P1 底层门禁通过** 且 Stage 4 ABI 审计、Stage 5 窗口路径更稳之后；小范围非行为变更清理允许。
7. “对标 Ant Design/Skia”仅作 **render 能力验收标准**；本计划不交付控件层。
3. 不回退目标架构；仍保持 `render -> gpu/webgpu -> gpu/rwgpu -> libwgpu_native`。
4. 不让 `render` 直接 import `gpu/rwgpu`。
5. 所有修复必须说明属于哪一层：`rwgpu ABI`、`gpu/webgpu descriptor 转换`、`render 渲染语义`、`测试/回归工具`。
6. P0 STRICT 视觉门禁与 format/blend 固定像素测试回归失败时，优先修正确性，不开启新性能任务。

### 已确认问题（2026-07-15）

#### 1. `rwgpu` enum / descriptor 转换存在实错（primitive 首轮已修）

`gpu/rwgpu/convert.go` 中把 `PrimitiveTopology`、`FrontFace`、`CullMode` 归类为“与 wgpu-native v29 完全一致”，但 `lib/webgpu.h` 显示并不一致。

示例：

```text
gpu/types.PrimitiveTopologyTriangleList = 0
lib/webgpu.h WGPUPrimitiveTopology_TriangleList = 4
```

已修复：

```go
toWGPUPrimitiveTopology
toWGPUFrontFace
toWGPUCullMode
```

并在 `gpu/rwgpu/abi_test.go`、`gpu/rwgpu/fuzz_test.go` 中加入转换和 primitive wire struct 测试。

剩余风险：当前只完成 render pipeline primitive 相关 enum 的首轮修复，不能据此认为 rust native 绑定层 ABI 已完整正确。必须继续对所有从 `gpu/types` 进入 native wire struct 的 enum 做逐项审计。

#### 2. `text_transform` 中 Scale 文字虚影来自 glyph mask 策略不等价（已首轮修）

`render/text.go` 的 `shouldUseGlyphMask()` 只判断无旋转/斜切，并用 Y scale 估算大小，因此 `Scale(2,2)`、`Scale(3,1)` 等场景会进入 GPU glyph mask。

原问题：`render/internal/gpu/glyph_mask_engine.go` 中 rasterize size 使用：

```go
fontSize := face.Size() * deviceScale
```

没有吸收用户 CTM scale；同时 batch 又保留完整 `Transform: matrix`，最终 shader 再放大 quad。实际效果是低分辨率 glyph mask 被 GPU 放大，导致 scale 文本发虚、有重影感。

已修复：
- 通过 `glyphMaskRasterScale(matrix, deviceScale)` 从 total matrix 中提取用户 Y scale。
- atlas raster font size 使用 `face.Size() * deviceScale * rasterScale`。
- glyph quad 尺寸使用 `bucketScale / (deviceScale * rasterScale)` 抵消重复 scale，最终屏幕尺寸仍由 CTM 决定。

#### 3. 细线 / crosshair / border 与 CPU 不一致来自 GPU stroke 语义缺口（已首轮修）

CPU stroke 在 `render/software.go` 中会：

```go
effectiveWidth := width * transformScale
if effectiveWidth < 1.0 {
    effectiveWidth = 1.0
}
```

原问题：GPU `StrokePath` 直接使用：

```go
Width: paint.EffectiveLineWidth()
```

没有乘 `TransformScale`，也没有 hairline 最小 1px 保护。`text_transform` 中的 `SetLineWidth(0.5)` crosshair 在 GPU 路径下会变成真正 0.5px 几何，容易不可见；缩放下 stroke 线宽也会与 CPU 不一致。

已修复：
- 新增 `render/internal/gpu/effectiveStrokeWidth`，对齐 CPU 的 `width * TransformScale` 与 `>= 1.0` hairline clamp。
- `StrokePath`、`StrokeShape` thin-stroke 判断、SDF stroked shape `HalfStroke` 共用该语义。

#### 4. 源 gg 示例不能直接作为 go-wgpu golden

`/home/yanghy/app/projects/gogpu/gg/examples/text_transform/main.go` 当前导入的是：

```go
_ "github.com/energye/gpui/render/gpu"
```

不是：

```go
_ "github.com/gogpu/gg/gpu"
```

因此该示例当前不能证明“源 go-wgpu GPU 路径正确”。后续如果需要 go-wgpu golden，必须单独使用真正的 `github.com/gogpu/gg/gpu` 路径生成。

---

## 🚦 新人 / AI 开工指南

本节是执行入口。新人或 AI 开发代理必须先读本节，再领取后续优化任务。

### 开工前必读代码

| 主题 | 必读文件 | 目的 |
|------|----------|------|
| Context 绘制入口 | `render/context.go` | 理解 `Fill()` / `Stroke()`、brush、path、GPU fallback |
| GPU 加速接口 | `render/accelerator.go` | 理解 `Accelerator`、`GPURenderContextProvider`、`Flush` 合约 |
| GPU 渲染上下文 | `render/internal/gpu/gpu_render_context.go` | 理解 GPU op 收集、flush、clip、pipeline 执行 |
| 软件光栅化 | `render/software.go`、`render/internal/raster/` | 理解 CPU fallback、AA、edge builder、filler |
| 渐变 API | `render/gradient_*.go`、`render/brush.go` | 理解当前实际 API：`NewLinearGradientBrush`、`SetFillBrush` |
| 场景图 | `render/scene/` | 理解批量绘制和并行遍历的现有入口 |

### 实际调用链

```text
render.Context
  ├─ Fill() / Stroke()
  │   ├─ tryGPUFillWithMode() / tryGPUStrokeWithMode()
  │   │   └─ Accelerator / GPURenderContextProvider
  │   │       └─ render/internal/gpu.GPURenderContext
  │   │           └─ Flush(target)
  │   └─ SoftwareRenderer fallback
  └─ Brush / Pattern / Path / Transform state
```

### 本地验证命令

```bash
# 快速单元测试
go test ./render/... -short

# 渲染核心测试
go test ./render/... -run 'Test.*(Context|Gradient|Raster|Accelerator)'

# 性能基准（任务 0 完成后必须稳定可用）
go test ./render/... -bench=BenchmarkSceneFPS -benchmem -count=3
```

如果 GPU 环境不可用，任务实现必须保留 CPU fallback，并在 PR/提交说明中写明哪些测试因本机 GPU 环境未运行。

### Rust WebGPU native 路径验证命令

迁移期间必须优先跑这些命令，确认当前路径不是 stub：

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache

# 编译和包边界检查
go test -run '^$' ./gpu/webgpu/... ./gpu/rwgpu/... ./render/internal/gpu ./render

# 真实 native 调用：device、shader、pipeline、texture readback
timeout 180s go test -count=1 ./render/internal/gpu

# 关键 native 子集，适合快速回归
timeout 120s go test -count=1 -run \
  'TestCompileShadersNative|TestPipelineCacheNativePipelines|TestTextureUploadDownloadNative' \
  ./render/internal/gpu
```

通过编译不等于渲染正确。examples 视觉输出必须单独验证。

---

## P0：ABI 与渲染效果一致性修复计划

### 背景

本轮架构是从源 go-wgpu 路径切到 Rust wgpu-native 路径。当前 `render/examples` 中部分示例在新路径下渲染效果与 CPU / 源 go-wgpu 不一致，总体观感不好。这个问题优先级高于性能优化。

当前问题已经确认不是单一 shader 或单一示例问题，而是至少包含：

```text
rwgpu ABI / enum / descriptor 转换风险
+ gpu/webgpu facade 转换完整性风险
+ render 文本 / stroke 语义不一致
```

在 ABI 与视觉正确性未收敛前，不要继续做批处理排序、atlas、缓存等优化，因为这些优化会扩大排查面。

### 目标

建立可重复的 ABI 与视觉回归流程，并把当前 native 路径输出校准到以下基准之一：

1. 源 go-wgpu 实现输出。
2. 当前 CPU/software renderer 输出。
3. 明确写入文档的预期差异，例如采样精度或平台字体差异。

### P0 执行顺序

必须按以下顺序推进，不允许跳过 ABI 直接修示例：

```text
P0.A rwgpu ABI / enum / descriptor 审计
  -> P0.B gpu/webgpu facade 转换审计
  -> P0.C 无图片上传的程序化视觉回归
  -> P0.D text_transform 定点修复
  -> P0.E 第一批 examples 端到端收敛
```

原因：如果 `rwgpu` wire 层仍有 enum/descriptor 错误，render 层修复可能只是绕过当前驱动表现，不能作为架构验收。

### 必测 examples

第一批必须覆盖：

| 示例 | 关注点 |
|------|--------|
| `render/examples/text_transform` | 当前核心回归；glyph mask scale、stroke hairline、clip、transform |
| `render/examples/basic` | 基础形状、颜色、线宽 |
| `render/examples/shapes` | path、fill rule、AA 边缘 |
| `render/examples/clipping` | clip stack、裁剪边界 |
| `render/examples/images` | texture upload、采样、premultiply alpha |
| `render/examples/text` | glyph mask、subpixel、baseline |
| `render/examples/cjk_text` | 字体 fallback、CJK glyph |
| `render/examples/scene` | scene encoding、批量绘制顺序 |
| `render/examples/scene_gpu` | GPU scene path |
| `render/examples/gpu` | GPU backend 端到端 |

### 视觉回归工具要求

新增或整理一个统一工具，建议位置：

```text
render/test_output/
render/internal/gpu/visual_test.go
render/internal/testutil/imagediff/
```

工具必须支持：
- 固定尺寸输出 PNG。
- 记录 backend：`software`、`source-go-wgpu`、`rwgpu-native`。
- 输出 per-pixel diff、max diff、mean diff、不同像素数量、结构化 JSON / text 报告。
- 支持阈值：文本和 GPU AA 可有小阈值，但纯色矩形、图片、clear、blend 不允许大面积差异。
- 失败时保留 actual / expected / diff 三张图到本地文件，但默认不要把 PNG 图片上传或粘贴到对话中。
- 对关键区域做语义检测，例如 red crosshair 像素数量、边框像素数量、文本 bounding box、非背景像素覆盖率。

### 无图片上传测试规则

排查时默认不把 PNG 图片发到对话中，避免浪费上下文 token。测试输出应尽量是文本：

```text
example=text_transform backend=rwgpu-native
scale_2_text_bbox=(637,132)-(813,178)
scale_2_text_rmse=...
crosshair_red_pixels=0 want>=40
border_gray_pixels=... want>=...
max_diff=...
mean_diff=...
```

允许保留图片文件路径，例如：

```text
/tmp/gpui-visual/text_transform/cpu.png
/tmp/gpui-visual/text_transform/gpu.png
/tmp/gpui-visual/text_transform/diff.png
```

只有当程序化指标不足以定位问题时，才人工查看图片。

### 排查顺序

必须按层排查，不要一次性改大段 render 代码。

1. Clear / render target format
   - 验证 RGBA8 / BGRA8 是否和目标一致。
   - 验证 clear color、load/store op、alpha 初值。
   - 验证 readback 是否有 BGRA/RGBA 通道交换。

2. Texture upload / sampling
   - 验证 `BytesPerRow`、row padding、premultiplied alpha。
   - 验证 sampler：nearest / linear、clamp、mipmap 默认值。
   - 用 2x2、3x1、非 256 对齐宽度图片做 readback。

3. Blend / alpha
   - 单独验证 SourceOver、Premultiplied、Copy、Plus。
   - 用红/绿半透明叠加测试，比较 CPU 与 native 输出。
   - 确认 shader blend 与 hardware blend 没有重复 premultiply。

4. Transform / coordinate system
   - 验证 y 轴方向、pixel center、viewport、scissor。
   - 用 1px 线、半像素位移、整数矩形测试。

5. Clip / stencil / depth clip
   - 单独验证矩形 clip、圆角 clip、嵌套 clip。
   - 排查 `SetBindGroup(1)`、depth/stencil attachment、pipeline layout 是否匹配。

6. Path AA / fill rule
   - 对比 software raster 与 GPU path。
   - 先修 fill rule、边界和 winding，再谈性能。

7. Text / glyph
   - 验证 glyph atlas、mask format、LCD/subpixel、baseline。
   - 字体差异要单独记录，不能混入 GPU 渲染差异。

### 当前已确认差异来源

根据 2026-07-15 的代码级排查，当前已确认：

- `rwgpu` primitive/front-face/cull-mode enum 转换说明与 `lib/webgpu.h` 不一致。
- glyph mask rasterize size 没有吸收 CTM scale，但 shader 继续应用 CTM，导致 scale text 被低分辨率放大。
- GPU stroke 没有对齐 CPU 的 `transformScale` 与 hairline 最小 1px 行为，导致 0.5px crosshair 和 1px 边框表现不一致。
- 源 gg `text_transform` 示例当前导入 `github.com/energye/gpui/render/gpu`，不能直接作为 go-wgpu golden。

仍需继续验证：

- `TextureFormatRGBA8Unorm` 与 surface / compositor 期望 `BGRA8Unorm` 是否存在隐藏差异。
- premultiplied alpha 在 CPU、texture upload、shader blend 中是否存在重复或遗漏。
- sampler 默认值是否导致图片或 glyph 边缘差异。
- stencil / depth clip pipeline 在 rwgpu 路径下的 depth/stencil 状态是否完全正确。
- legacy stub 路径是否仍被某些示例间接调用。

### P0 任务卡

#### Task P0.A rwgpu ABI / enum / descriptor 审计

目标：
- 以 `lib/webgpu.h` 为准，校准当前 render 已调用的全部 `rwgpu` native ABI。

先读：
- `lib/webgpu.h`
- `gpu/rwgpu/convert.go`
- `gpu/rwgpu/render_pipeline.go`
- `gpu/rwgpu/texture.go`
- `gpu/rwgpu/sampler.go`
- `gpu/rwgpu/render.go`
- `gpu/rwgpu/bindgroup.go`
- `gpu/rwgpu/command.go`

修改范围：
- `gpu/rwgpu/`
- `gpu/rwgpu/*_test.go`

实现要点：
- 修正 `PrimitiveTopology`、`FrontFace`、`CullMode` native enum 转换。
- 扫描所有直接把 `types.*` 写入 native wire struct 的地方。
- 对当前 render 已使用 API 建立 enum mapping 单元测试。
- 对关键 wire struct 建立 size / offset 测试。
- 真实 native 调用仍必须通过 `WGPU_NATIVE_PATH` 运行。

验证：
```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache

go test -count=1 ./gpu/rwgpu
go test -count=1 ./gpu/webgpu
go test -count=1 ./render/internal/gpu -run 'Test.*(Native|Pipeline|Clear|Texture|Stencil)'
```

完成标准：
- `convert.go` 文档与 `lib/webgpu.h` 一致。
- render 当前用到的 enum 不再依赖“碰巧相等”。
- 新增测试能在未来 header 更新时暴露 enum / layout 漂移。

#### Task P0.B gpu/webgpu facade 转换审计

目标：
- 确保 `gpu/webgpu` 到 `gpu/rwgpu` 的 descriptor 转换完整、显式、可测试。

先读：
- `gpu/webgpu/device.go`
- `gpu/webgpu/queue.go`
- `gpu/webgpu/encoder.go`
- `gpu/webgpu/renderpass.go`
- `gpu/webgpu/texture.go`
- `gpu/webgpu/bind.go`

修改范围：
- `gpu/webgpu/`
- `gpu/webgpu/*_test.go`

实现要点：
- 不允许 facade 层无证明地直传 enum。
- 对 render pipeline、bind group layout、texture、sampler、render pass descriptor 建立转换测试。
- 检查 pointer lifetime：slice / StringView / nested descriptor 在 native 调用期间必须存活。

验证：
```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
go test -count=1 ./gpu/webgpu/... ./gpu/rwgpu/...
```

完成标准：
- `gpu/webgpu` 的每类 descriptor 都有转换测试。
- render 通过 `gpu/webgpu` 创建的 pipeline / texture / bind group 都是真实 native 对象。

#### Task P0.C text_transform 程序化视觉回归（首轮完成）

目标：
- 为 `render/examples/text_transform` 建立不上传图片的 CPU vs rwgpu-native 程序化回归。

先读：
- `render/examples/text_transform/main.go`
- `render/text.go`
- `render/internal/gpu/glyph_mask_engine.go`
- `render/internal/gpu/gpu_render_context.go`

修改范围：
- `render/internal/testutil/imagediff/`
- `render/internal/visualcmd/text_transform/`
- `render/text_transform_gpu_visual_test.go`

实现要点：
- 使用同一份内部 helper 绘制 `text_transform`，默认 CPU，`-tags gpui_visual_gpu` 时 blank import `render/gpu`。
- 测试默认 skip；设置 `GPUI_TEXT_TRANSFORM_VISUAL=1` 后生成 CPU baseline 与 GPU actual。
- 默认只输出 text 指标。
- 指标至少覆盖：
  - Scale(2,2) 文本 bbox / RMSE / 非背景像素数量。
  - Scale(3,1) non-uniform 文本 bbox / RMSE。
  - red crosshair 像素数量。
- 失败时保留 PNG 文件路径，但不上传图片。

验证：
```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
GPUI_TEXT_TRANSFORM_VISUAL=1 \
GPUI_TEXT_TRANSFORM_VISUAL_STRICT=1 \
go test -count=1 ./render -run TestTextTransformCPUvsGPUVisualDiagnostic -v
```

完成标准：
- 当前诊断可稳定输出 CPU/GPU diff、每个 cell 的 dark pixel、red pixel 和 bbox。
- 后续修复能用同一测试证明结果改善。

最新指标（2026-07-15，P0.D 修复后）：
```text
diff changed=11270/630000 mean_abs=1.253 rmse=12.572 max_delta=212
scale2x    cpu_dark=2135 gpu_dark=2133 ratio=0.999 cpu_bbox=203x46 gpu_bbox=207x46
scale_down cpu_dark=175  gpu_dark=178  ratio=1.017 cpu_bbox=67x15  gpu_bbox=72x15
crosshair red pixels 已恢复：identity/translate/scale2x/scale_down 均 cpu_red=4 gpu_red=4
```

#### Task P0.D text scale 与 stroke hairline 修复（首轮完成）

目标：
- 修复 `text_transform` 中 Scale 文本虚影、crosshair 和 thin border 消失/变弱问题。

先读：
- `render/text.go`
- `render/internal/gpu/glyph_mask_engine.go`
- `render/internal/gpu/glyph_mask_pipeline.go`
- `render/internal/gpu/gpu_render_context.go`
- `render/software.go`

修改范围：
- `render/internal/gpu/glyph_mask_engine.go`
- `render/internal/gpu/gpu_render_context.go`
- `render/internal/gpu/sdf_render.go`
- 必要测试文件

实现要点：
- glyph mask 不得在低分辨率 rasterize 后再被 CTM 放大。
- 对 horizontal scale，按目标 device size rasterize，然后在 quad 尺寸中抵消重复 scale。
- 对 non-uniform scale、rotation、shear，优先走 vector/MSDF 或明确写入策略。
- GPU stroke 必须对齐 CPU 的 `transformScale` 和 hairline 最小 1px 语义。

验证：
```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
go test -count=1 ./render/internal/gpu -run 'TestGlyphMask|TestEffectiveStrokeWidth|TestDetectedShapeToRenderShapeStroked'
GPUI_TEXT_TRANSFORM_VISUAL=1 GPUI_TEXT_TRANSFORM_VISUAL_STRICT=1 \
go test -count=1 ./render -run TestTextTransformCPUvsGPUVisualDiagnostic -v
```

完成标准：
- `text_transform` 程序化指标通过。
- CPU 与 GPU 对 scale text、0.5px crosshair 的差异在阈值内。
- 没有通过强制 CPU fallback 掩盖 GPU 路径问题。

本轮实际改动：
- `render/internal/gpu/stroke_width.go`：新增 `effectiveStrokeWidth`。
- `render/internal/gpu/gpu_render_context.go`：`StrokePath` 和 `StrokeShape` 使用 GPU/CPU 一致的 stroke width 语义。
- `render/internal/gpu/sdf_render.go`：stroked SDF shape `HalfStroke` 使用同一 helper。
- `render/internal/gpu/glyph_mask_engine.go`：glyph mask raster size 吸收用户 Y scale，quad 尺寸抵消重复 scale。
- `render/internal/gpu/glyph_mask_spacing_test.go`：HiDPI 测试输入改为 total matrix，符合生产调用契约。

#### Task P0.E examples 视觉基线采集（首轮完成）

目标：
- 对必测 examples 生成 software 与 rwgpu-native 输出，保存 PNG 和 diff 报告。

先读：
- `render/examples/*/main.go`
- `render/context.go`
- `render/internal/gpu/gpu_render_context.go`
- `render/internal/gpu/render_session.go`

修改范围：
- `render/internal/testutil/imagediff/`
- `render/internal/visualcmd/<example>/`
- `render/*_gpu_visual_test.go`
- 可新增 examples runner，但不要修改示例绘制语义。

验证：
```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so

GPUI_TEXT_TRANSFORM_VISUAL=1 GPUI_TEXT_TRANSFORM_VISUAL_STRICT=1 \
go test -count=1 ./render -run TestTextTransformCPUvsGPUVisualDiagnostic -v

GPUI_BASIC_VISUAL=1 GPUI_BASIC_VISUAL_STRICT=1 \
go test -count=1 ./render -run TestBasicCPUvsGPUVisualDiagnostic -v

GPUI_SHAPES_VISUAL=1 GPUI_SHAPES_VISUAL_STRICT=1 \
go test -count=1 ./render -run TestShapesCPUvsGPUVisualDiagnostic -v

GPUI_CLIPPING_VISUAL=1 GPUI_CLIPPING_VISUAL_STRICT=1 \
go test -count=1 ./render -run TestClippingCPUvsGPUVisualDiagnostic -v

GPUI_IMAGES_VISUAL=1 GPUI_IMAGES_VISUAL_STRICT=1 \
go test -count=1 ./render -run TestImagesCPUvsGPUVisualDiagnostic -v

GPUI_TEXT_VISUAL=1 GPUI_TEXT_VISUAL_STRICT=1 \
go test -count=1 ./render -run TestTextCPUvsGPUVisualDiagnostic -v

GPUI_CJK_TEXT_VISUAL=1 GPUI_CJK_TEXT_VISUAL_STRICT=1 \
go test -count=1 ./render -run TestCJKTextCPUvsGPUVisualDiagnostic -v

GPUI_SCENE_VISUAL=1 GPUI_SCENE_VISUAL_STRICT=1 \
go test -count=1 ./render -run TestSceneCPUvsGPUVisualDiagnostic -v

GPUI_SCENE_GPU_VISUAL=1 GPUI_SCENE_GPU_VISUAL_STRICT=1 \
go test -count=1 ./render -run TestSceneGPUCPUvsGPUVisualDiagnostic -v

GPUI_GPU_EXAMPLE_VISUAL=1 GPUI_GPU_EXAMPLE_VISUAL_STRICT=1 \
go test -count=1 ./render -run TestGPUExampleCPUvsGPUVisualDiagnostic -v
```

完成标准：
- 至少覆盖 basic、shapes、clipping、images、text、scene_gpu。
- 每个失败样例都有 actual / expected / diff。
- 报告能指出是通道、alpha、位置、clip、AA 还是字体类差异。

已接入：
- `render/internal/testutil/imagediff`：PNG decode、per-pixel diff、region metric、bbox 格式化。
- `render/internal/visualcmd/text_transform` + `TestTextTransformCPUvsGPUVisualDiagnostic`。
- `render/internal/visualcmd/basic` + `TestBasicCPUvsGPUVisualDiagnostic`。
- `render/internal/visualcmd/shapes` + `TestShapesCPUvsGPUVisualDiagnostic`。
- `render/internal/visualcmd/clipping` + `TestClippingCPUvsGPUVisualDiagnostic`。
- `render/internal/visualcmd/images` + `TestImagesCPUvsGPUVisualDiagnostic`。
- `render/internal/visualcmd/text` + `TestTextCPUvsGPUVisualDiagnostic`。
- `render/internal/visualcmd/cjk_text` + `TestCJKTextCPUvsGPUVisualDiagnostic`。
- `render/internal/visualcmd/scene` + `TestSceneCPUvsGPUVisualDiagnostic`。
- `render/internal/visualcmd/scene_gpu` + `TestSceneGPUCPUvsGPUVisualDiagnostic`。
- `render/internal/visualcmd/gpu` + `TestGPUExampleCPUvsGPUVisualDiagnostic`。

`basic` 最新指标（2026-07-15）：
```text
diff changed=2254/262144 mean_abs=0.074 rmse=1.083 max_delta=63
red_circle   cpu_pixels=29029 gpu_pixels=29040 ratio=1.000 bbox=200x200@156,156
blue_rect    cpu_pixels=15000 gpu_pixels=15000 ratio=1.000 bbox=150x100@100,100
green_stroke cpu_pixels=1464  gpu_pixels=1444  ratio=0.986 bbox=104x104@348,98
```

`shapes` 最新指标（2026-07-15）：
```text
diff changed=4861/480000 mean_abs=0.160 rmse=2.721 max_delta=204
rect_red       ratio=1.000 bbox=150x100@50,50
rrect_green    ratio=1.000 bbox=150x100@250,50
circle_blue    ratio=1.000 bbox=120x120@440,40
ellipse_yellow ratio=0.997 bbox=160x100@570,50
pentagon       ratio=1.025 bbox_cpu=89x94@51,253 bbox_gpu=91x94@50,253
hexagon        ratio=1.012 bbox_cpu=98x86@201,257 bbox_gpu=100x86@200,257
octagon        ratio=1.000 bbox=92x92@354,254
black_line     ratio=1.019 bbox=700x2@50,449
arc_magenta    ratio=0.969 bbox=124x124@588,238
rotated_teal   ratio=0.992 bbox=112x112@344,444
```

`clipping` 最新指标（2026-07-15）：
```text
diff changed=10244/720000 mean_abs=0.248 rmse=3.240 max_delta=128
circular_clip      ratio=0.997 bbox=160x160@70,70
rect_clip          ratio=1.000 bbox=149x149@301,51
rect_clip_leak     cpu_pixels=0 gpu_pixels=0
clip_preserve_star ratio=0.989 bbox=134x127@83,380
nested_clips       ratio=0.999 bbox=160x160@300,350
complex_path       ratio=0.982 bbox=154x101@598,66
round_rect         ratio=0.998 bbox=200x140@50,600
reset_clip_fill    ratio=1.000 bbox=158x158@301,601
reset_clip_leak    cpu_pixels=0 gpu_pixels=0
```

`images` 最新指标（2026-07-15，P0.F 修复后）：
```text
diff changed=445/480000 mean_abs=0.114 rmse=1.159 max_delta=188
basic/scaled/opacity/nearest/src_rect/multiply/pattern ratio≈1.000
transform ratio=1.004  # 已修复：CTM 旋转四角点 textured quad
pattern_center delta=0  # 已修复：非 solid paint GPU fill 回退 CPU
STRICT: PASS
```

P0.F 已修问题：
- `tryGPUDrawImage` 原先在非轴对齐 CTM 时直接失败，回退到 ImagePattern Fill；GPU solid path 无法纹理采样，导致 transform 区域几乎空白。
- 现改为始终将用户空间四角 TL/TR/BR/BL 经 `totalMatrix()` 变换后提交 textured quad 顶点。
- `FillPath`/`FillShape` 对非 solid paint（ImagePattern/Gradient 等）返回 `ErrFallbackToCPU`，避免 `ColorAt(0,0)` 被当成整块纯色。

`text` 最新指标（2026-07-15）：
```text
diff changed=1737/320000 mean_abs=0.253 rmse=5.145 max_delta=204
title/subtitle/left/center/right/measured/fontinfo ratio≈0.997-1.003
```

`cjk_text` 最新指标（2026-07-15，P0.F 修复后）：
```text
diff changed=6642/480000 mean_abs=0.730 rmse=7.561 max_delta=172
body ratio=1.010  # 已修复：跳过 .notdef 豆腐块
display ratio=1.000
mixed ratio=1.001
titles ratio=1.000
```

P0.F cjk_text 根因：
- DroidSansFallback 等 CJK 字体将 Latin 映射到 GID 0 (.notdef)。
- CPU `text.Draw` 跳过 GID 0 只推进笔位；GPU glyph mask 却栅格化 .notdef 矩形，导致 "12px:" 等前缀画出一串豆腐块，coverage 膨胀 2–3x。
- 同步：`isCJKText` / glyph-mask hinting 改为扫描整串 CJK，避免 Latin 前缀误走 Full hinting。

`scene` 最新指标（2026-07-15）：
```text
diff changed=7999/262144 mean_abs=1.020 rmse=10.575 max_delta=192
rect_* / center_circle / blend_zone ratio≈1.000-1.001
gpu path tiles_rendered=0（走 GPUSceneRenderer）
```

`scene_gpu` 最新指标（2026-07-15）：
```text
diff changed=4611/262144 mean_abs=0.145 rmse=1.466 max_delta=49
corner_*/center/orbit ratio≈1.000-1.001
```

`gpu` 最新指标（2026-07-15）：
```text
diff changed=17969/480000 mean_abs=0.761 rmse=4.959 max_delta=113
circle/rrect/stroke/triangle/pentagon/hexagon/star/curve ratio≈0.956-1.024
```

P0.E/F 结论（2026-07-15 关闭）：
- 第一批 examples 视觉基线可重复采集，且 STRICT 门禁全部 PASS。
- `images.transform` / `images.pattern` 与 `cjk_text` body/mixed 已在 P0.F 修复。
- P0.2 format/readback 与 P0.3 blend/alpha 校准已完成。
- 残留差异（scene rmse≈10.6、text/text_transform AA fringe、gpu polygon coverage ±2–4%）记为可接受阈值，不阻断 P0。

#### Task P0.2 render target format 与 readback 校准 ✅

目标：
- 确认 RGBA/BGRA、premultiply、row padding 在 native 路径下完全可控。

实现要点：
- `render/internal/gpu/gpu_texture.go`：`packTextureUpload` / `alignTextureBytesPerRow`（256-byte pitch）
- R8 接受 packed plane 或 RGBA pixmap（取 alpha 作 mask）
- Download 统一按对齐 pitch unpack，BGRA→RGBA，R8→white+alpha

验证：
```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
go test -count=1 ./render/internal/gpu -run 'TestTextureUploadDownloadNative|TestTextureClearNative|TestPackTextureUpload'
```

完成标准：
- ✅ RGBA8、BGRA8、R8 readback 明确（`p0_format_readback_test.go`）
- ✅ 非 256 对齐宽度（3×5 RGBA、7×3 R8、4×2 BGRA）roundtrip 正确
- ✅ clear-via-upload 后像素/alpha 与预期一致

#### Task P0.3 blend / alpha 一致性 ✅

目标：
- 让常用 UI blend 输出与 CPU/source-go-wgpu 一致。

实现要点：
- 固定像素测试：`render/internal/blend/p0_blend_alpha_test.go`
- scene 映射测试：`render/scene/p0_blend_alpha_test.go`
- 覆盖 Normal / SourceOver / Copy / Plus / Multiply

premultiplied vs straight alpha 边界：
- `render/internal/blend` 全部 Porter-Duff / advanced 算子输入输出均为 **premultiplied alpha**（0–255）。
- 若源颜色是 straight alpha，必须先转换：`premul.rgb = straight.rgb * a / 255`，`premul.a = a`。
- 直接把 straight 颜色喂给 SourceOver 会抬高 RGB（P0.3 测试已固定记录该 bug class）。
- `scene.BlendNormal` ≡ `scene.BlendSourceOver` ≡ internal `blend.BlendSourceOver`；`scene.BlendCopy` ≡ internal `blend.BlendSource`。
- GPU texture upload/readback（RGBA8）当前按字节布局传输，不隐式 premultiply；混合前由渲染路径负责 premul 语义。

验证：
```bash
export GOCACHE=/tmp/gpui-go-cache
go test -count=1 ./render/internal/blend ./render/scene -run 'TestP03'
go test -count=1 ./render/internal/gpu -run 'TestBlendMode'
```

完成标准：
- ✅ Normal、SourceOver、Copy、Plus、Multiply 有固定像素测试
- ✅ premultiplied 与 straight alpha 的边界写入本文档

#### Task P0.F examples 端到端修复 ✅

目标：
- 修复第一批必测 examples 的 native 输出差异。

验证（2026-07-15 STRICT 全绿）：
```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
# 对 BASIC/SHAPES/CLIPPING/IMAGES/TEXT/CJK_TEXT/SCENE/SCENE_GPU/GPU_EXAMPLE/TEXT_TRANSFORM
# 设置 GPUI_<NAME>_VISUAL=1 GPUI_<NAME>_VISUAL_STRICT=1 后跑对应 Test*CPUvsGPUVisualDiagnostic
```

STRICT 结果摘要：
```text
basic          changed=2254/262144  rmse=1.083   PASS
shapes         changed=4861/480000  rmse=2.721   PASS
clipping       changed=10244/720000 rmse=3.240   PASS
images         changed=445/480000   rmse=1.159   PASS  transform ratio=1.004
text           changed=1737/320000  rmse=5.145   PASS
cjk_text       changed=6642/480000  rmse=7.561   PASS  body=1.010 mixed=1.001
scene          changed=7999/262144  rmse=10.575  PASS  形状 ratio≈1.0
scene_gpu      changed=4611/262144  rmse=1.466   PASS
gpu_example    changed=17969/480000 rmse=4.959   PASS
text_transform changed=11270/630000 rmse=12.572  PASS
```

可接受残留差异（不阻断 P0，后续可精修）：
- `scene` 较高 rmse（≈10.6）：主要来自 AA/coverage fringe 与 blend 区边缘，bbox/coverage ratio≈1.0。
- `text` / `text_transform` / `cjk_text` fringe：glyph mask 与 CPU FreeType AA 不完全同构，主体 coverage ratio≈1.0。
- `gpu_example` 多边形 coverage ±2–4%：analytic GPU path 与 CPU edge builder 边缘采样差异。
- `images.text_marks` ratio≈0.886：装饰文字小区域，非主图元路径。

完成标准：
- ✅ 第一批 examples STRICT 全部 PASS
- ✅ 残留差异均有原因登记

---

## P1：Ant Design / Skia 级 **render 能力门禁**（Foundation Gate）

> **定位**：P0 证明“native 路径能画、主 examples 不炸”。  
> P1 证明 `render`（经 `webgpu`/`rwgpu`）达到 Ant Design 级 UI **所需的绘制能力** 与 Skia 方向的 2D 正确性。  
> **本计划不实现 Ant Design 控件层**；复杂场景只是 render 回归用的“控件形态压测”，用来逼出 ABI/webgpu/render 缺口。

### 主线顺序（必须遵守）

```text
rwgpu ABI 绑定正确
  -> gpu/webgpu 能力补全（真实 native，非 stub）
  -> render 语义/视觉对标（Ant Design 所需绘制能力 + Skia 质量方向）
  -> （计划外）上层控件库可另开项目，依赖本库渲染能力
```

### 为什么第一批 examples 不够

当前 P0 必测集（basic/shapes/clipping/images/text/cjk/scene/gpu/text_transform）覆盖的是：
- 基础图元、简单 clip、单层 text/image、轻量 scene

要达到 Ant Design/Skia **渲染能力**，底层必须稳定覆盖更复杂的绘制组合，例如：
- 大量小圆角矩形 + 1px 边框 + 半透明遮罩
- 多层叠加（模拟 dropdown/modal/drawer/tooltip 的绘制结构）
- 重复 cell / 网格线 / 文本省略（模拟 table/list 压力）
- 图标字体 / SVG path / 彩色 emoji
- caret、选区、混排文本
- 渐变、阴影、focus ring
- 高 DPI / devicePixelRatio
- damage/redraw 正确性（先于性能）

因此 P1 用 **分层硬证据 + 复杂场景矩阵** 验收 render 栈，而不是再加几个简单 demo。

### P1 分层硬证据（必须全部真实调用链）

所有 P1 测试默认：

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
```

| 层 | 必须证明 | 最低测试形态 | 失败即否决 |
|----|----------|--------------|------------|
| L1 ABI | `rwgpu` enum/struct 与 `lib/webgpu.h` 一致；render/UI 将用 API 无错误 wire | `go test ./gpu/rwgpu -run 'TestABI|TestRenderPass|Test.*Native'` | ABI 错位、struct size/offset 漂移 |
| L2 WebGPU facade | `gpu/webgpu` → `rwgpu` 创建/上传/提交/readback 真实成功 | device/texture/pipeline/queue 原生测试 | facade 空实现、静默 no-op |
| L3 Render 语义 | `render.Context` / scene 路径最终像素正确 | visualcmd + imagediff STRICT；关键路径禁止 silent CPU fallback | 关键 region GPU miss 或 diff 超阈值 |
| L4 GPU 固定像素 | blend/alpha/clip/transform 在 RT 上 readback 可复算 | 小 RT clear→draw→download 与公式/CPU 参考比对 | GPU blend/premul 与文档契约不符 |
| L5 复杂场景 | 控件形态压测场景矩阵（render 能力） | 新增 complex visual suite | 任一 P1 场景 STRICT 失败 |

**强制可观测性**：
- 每个 GPU 视觉用例输出：`accelerator`、`direct`、`gpu_ops`、`cpu_fallback_ops`（新增计数器）。
- 关键 region 要求 `gpu_ops > 0` 且无“本应 GPU 却 fallback”的未解释项。
- 允许的 fallback 必须写进场景说明；否则测试失败。

### P1 场景矩阵（比 examples 更接近控件）

#### Tier A — 控件原子（必须新增 visualcmd + STRICT）

| ID | 场景 | 模拟的 UI 绘制形态（非控件实现） | 底层关注点 |
|----|------|----------------------|------------|
| A1 | `ui_button_states` | Button default/hover/active/disabled | rrect、1px border、text baseline、focus ring |
| A2 | `ui_input_field` | Input / TextArea | 边框、placeholder、caret、selection rect、clip |
| A3 | `ui_menu_overlay` | Dropdown / Menu | 多层半透明、阴影、z-order、popup clip |
| A4 | `ui_modal_mask` | Modal / Drawer | 全屏 mask alpha、居中面板、滚动裁剪 |
| A5 | `ui_table_cells` | Table / List 局部 | 大量重复 cell、网格线、ellipsis 文本、row hover |
| A6 | `ui_tabs_badge` | Tabs / Badge / Tag | 小圆点、描边字、紧凑布局 AA |
| A7 | `ui_icon_text_mix` | Icon + Typography | 图标字体/SVG path + 中西文混排 |
| A8 | `ui_scroll_clip` | Scroll 容器 | nested clip、滚动偏移、damage 边界 |

#### Tier B — 渲染压力（Skia 方向正确性，不只 FPS）

| ID | 场景 | 关注点 |
|----|------|--------|
| B1 | `stress_many_rrects` | 1k~10k 圆角矩形 batch 正确性（先正确后性能） |
| B2 | `stress_text_atlas` | 多字体尺寸 atlas 回收/重绘无花屏 |
| B3 | `stress_image_gallery` | 多图缩放/旋转/opacity |
| B4 | `stress_blend_stack` | Normal/SourceOver/Multiply/Plus 多层堆叠 |
| B5 | `stress_path_aa` | 复杂 path fill/stroke/even-odd |
| B6 | `stress_hidpi` | DPR=1/1.5/2 下 1px hairline 与文本 |

#### Tier C — 既有 examples 升级为门禁（不够则改场景，不降低标准）

现有 examples 全部纳入回归，但 **P1 关闭不以它们为充分条件**：
- 已有：basic/shapes/clipping/images/text/cjk_text/scene/scene_gpu/gpu/text_transform
- 补接入 visual 门禁：`bezier_test`、`pixel_perfect`、`color_emoji`、`text_fallback`、`variable_font`、`sdf`、`recording`、`compute_clip`（若进生产路径）

### P1 任务卡

#### Task P1.0 门禁基础设施 ✅（首轮）

目标：
- 统一 visual harness：env 门禁、STRICT、区域 metric、**fallback 计数**、报告输出。

实现：
- `render.Context`：`RenderPathStats` / `gpu_ops` / `cpu_fallback_ops`（fill/stroke/image/text 路径计数）
- visualcmd 输出 `gpu_ops=N cpu_fallback_ops=M`
- `ParseRenderPathStatsLog` / `RequireGPUPathStats`；basic/shapes/… STRICT 强制 `gpu_ops>0`

验证：
```bash
GPUI_BASIC_VISUAL=1 GPUI_BASIC_VISUAL_STRICT=1 go test -count=1 -v ./render -run TestBasicCPUvsGPUVisualDiagnostic
# gpu_log 含 gpu_ops=3 cpu_fallback_ops=0
```

完成标准：
- ✅ `gpu_ops` / `cpu_fallback_ops` 可在 visual log 中读到
- ✅ STRICT 下 GPU 路径 `gpu_ops==0` 失败（Context 类 examples）
- ⏳ scene/scene_gpu 仍为 scene renderer 路径（暂记 note=scene_renderer）

#### Task P1.1 ABI/WebGPU 全量（UI 子集）审计

目标：
- 对 render/UI 绘制路径会触达的 descriptor/enum 完成 `lib/webgpu.h` 对齐。

完成标准：
- `go test ./gpu/rwgpu ./gpu/webgpu` 全绿（含 ABI size/offset/enum）。
- 文档列出“UI 子集 API 清单”与测试映射。

#### Task P1.2 GPU 固定像素（blend/clip/transform） 🔄

目标：
- 小 RT 真链路上验证 UI 高频语义。

首轮实现（2026-07-15）：
- `render/p1_gpu_fixed_pixel_test.go`（`import _ render/gpu`，真实 accelerator）
- SourceOver premul：中心像素与 `blend.SourceOver` 参考 **完全一致** `(128,0,127,255)`
- Opaque replace、clip outside 保持白、1px hairline 可见
- 强制 `gpu_ops>0`

验证：
```bash
export WGPU_NATIVE_PATH=.../lib/libwgpu_native.so
go test -count=1 -v ./render -run 'TestP12GPUFixedPixel'
```

完成标准：
- 🔄 SourceOver GPU readback 固定像素通过（首轮）
- ⬜ Copy/Plus/Multiply GPU 固定像素
- ✅ clip + hairline 首轮通过
- ⬜ nested clip / rrect / transform 扩展
- ✅ premul SourceOver 与 CPU blend 公式对齐（本机验证）

#### Task P1.3 Tier A 控件原子场景

目标：
- A1–A8 全部有 visualcmd + `*_gpu_visual_test.go` + STRICT。

完成标准：
- 全部 STRICT PASS；每场景登记可接受差异（若有）与原因。

#### Task P1.4 Tier B 压力正确性

目标：
- B1–B6 正确性门禁（性能数字可并行采集，但正确性优先）。

完成标准：
- 无花屏/错层/漏 clip；抽样 region 与 CPU 参考在阈值内。

#### Task P1.5 关闭 render 能力门禁

目标：
- 汇总 L1–L5 证据，宣布“render 栈达到 Ant Design/Skia 级渲染能力门槛”。

完成标准（**全部满足才算 P1 完成**）：
- [ ] L1 ABI 套件绿
- [ ] L2 webgpu native 套件绿
- [ ] L3 现有 + Tier A/B 视觉 STRICT 绿
- [ ] L4 GPU 固定像素绿
- [ ] fallback 可观测且无未解释 fallback
- [ ] 已知差异全部文档化
- [ ] **明确书面结论：render（经 webgpu/rwgpu）达到 Ant Design/Skia 级渲染能力门槛**
- [ ] 不包含、不要求本仓库交付任何控件层代码

### P1 与后续阶段关系

```text
P0（已完成）迁移可跑 / 主 examples 门禁
  -> P1 render 能力门禁（ABI→webgpu→render 对标）
      -> 阶段四 C/D/E 继续补全与精修
      -> 阶段五 窗口/swapchain 真显示路径
      -> 阶段六 性能（batch/atlas/cache）——建议 P1 后
      -> （计划外）上层控件库项目可依赖本渲染能力
```

### AI 开发代理执行规则

1. 不要直接照抄本文伪代码；先用 `rg` 对照实际类型、函数名和调用链。
2. 每个任务必须先提交基线数据，再提交优化结果；没有基线时不得声称性能提升。
3. 优化必须保持渲染语义：alpha 混合、clip、transform、fill rule、绘制顺序不得被无证明地改变。
4. 缓存类任务必须定义 key、生命周期、内存预算、失效条件、并发策略和统计指标。
5. 新增 public API 前必须说明必要性；优先使用现有 `Context`、`Brush`、`Accelerator`、`GPURenderContext` 模型。
6. 每个任务至少包含：单元测试、视觉/像素一致性测试或性能 benchmark 中的一类；高风险任务必须同时包含正确性和性能测试。
7. 不要修改与任务无关的格式、命名、目录结构或历史未跟踪文件。

### 并行开发边界

| 任务 | 是否适合新人直接做 | 并行建议 | 注意事项 |
|------|--------------------|----------|----------|
| 任务 0 性能基准 | ✅ 适合 | 第一个启动，其他任务依赖它的报告格式 | 不改渲染行为，只加测试和报告工具 |
| 任务 1 路径缓存 | ⚠️ 需熟悉 GPU 调用链 | 等任务 0 有基线后启动 | 不能和任务 5 各自实现重复 LRU |
| 任务 2 GPU 渐变 | ⚠️ 需熟悉 brush + shader pipeline | 可和任务 1 并行，但共享资源预算接口 | 示例必须使用实际 `Brush` API |
| 任务 3 批处理排序 | ❌ 不建议新人独立做 | 需先做渲染语义审计 | 透明混合、clip、depth/order 会影响正确性 |
| 任务 4 纹理图集 | ⚠️ 需熟悉纹理生命周期 | 可在任务 5 cache 接口确定后做 | glyph/icon/gradient atlas 不要重复造轮子 |
| 任务 5 资源缓存 LRU | ✅ 可拆给有 Go 经验新人 | 应先定义统一接口，供任务 1/2/4 使用 | 重点是测试淘汰、预算、并发 |
| 任务 6 并行光栅化 | ⚠️ 需熟悉 `internal/parallel` | 可独立实验，但先保持 CPU 输出一致 | 必须跑 race/一致性测试 |
| 任务 7 亚像素精度 | ❌ 暂不建议直接做 | 先分析 overflow 和质量收益 | 当前实现不是 `const aaShift`，而是 `NewEdgeBuilder(2)` |

### 可直接派发任务卡模板

每个具体任务必须补成以下格式后再交给新人或 AI：

```md
#### Task X.Y 标题

目标：
- 一句话说明要交付的可运行结果。

先读：
- 相关源码文件列表。

修改范围：
- 允许新增/修改的文件。

禁止修改：
- 与任务无关的模块或 public API。

实现要点：
- 关键数据结构、调用点、错误处理、fallback、并发/缓存策略。

验证：
- 必跑命令。
- 需要保存或输出的报告。

完成标准：
- 可客观检查的正确性、性能、内存、兼容性指标。
```

### 第一批推荐派发任务

#### Task 0.1 FPS 测量器

目标：
- 新增可复用 FPS / frame time 测量工具，输出 average/min/max/p95/p99。

先读：
- `render/context.go`
- `render/software.go`
- `render/examples/`

修改范围：
- `render/benchmark_fps_test.go`
- `render/benchmark_scenes_test.go`
- 如需共享给非测试代码，先放在 `render/internal/benchutil/`，不要直接增加 public API。

验证：
```bash
go test ./render/... -run TestFPSMeasureSmoke
go test ./render/... -bench=BenchmarkSceneFPS -benchmem -count=3
```

完成标准：
- 固定场景能输出 frame time 分布。
- 报告包含分辨率、对象数量、backend、CPU/GPU 基本信息。
- 同一机器连续 3 次波动可解释，报告格式稳定。

#### Task 5.1 通用 LRU 缓存

目标：
- 提供可被 path、gradient、texture 共用的预算型 LRU cache。

先读：
- `render/internal/`
- `render/text/glyph_mask_atlas.go`
- `render/internal/gpu/gpu_render_context.go`

修改范围：
- 优先新增 `render/internal/cache/`，除非调用点证明必须放到 public `render` 包。

验证：
```bash
go test ./render/internal/... -run Test.*LRU
go test ./render/... -short
```

完成标准：
- 支持条目数预算和字节预算。
- 支持命中、未命中、淘汰统计。
- 并发安全策略明确，有测试覆盖。

#### Task 1.1 路径缓存设计草案

目标：
- 在不改变渲染行为的前提下，提交路径缓存 key、生命周期和集成点设计，并用测试验证 key 稳定性。

先读：
- `render/path.go`
- `render/context.go`
- `render/internal/gpu/gpu_render_context.go`
- `render/internal/gpu/render_session.go`

修改范围：
- `render/internal/cache/` 或 `render/internal/gpu/` 内部实验文件。
- 不直接改 public `Path` API，除非先写设计说明。

验证：
```bash
go test ./render/... -run Test.*Path.*Cache
go test ./render/... -short
```

完成标准：
- 相同 path + transform 产生稳定 key。
- path 内容变化会失效。
- 明确 CPU tessellation cache 和 GPU buffer cache 是否分层。

---

## 🎯 性能目标

### 当前性能（待测试）
| 场景 | 当前 FPS | 目标 FPS | 差距 |
|------|----------|----------|------|
| 1000 个圆形动画 | ? | 60 | - |
| 1000 个路径动画 | ? | 60 | - |
| 渐变填充 | ? | 60 | - |
| 文本渲染 | ? | 60 | - |
| 混合场景 | ? | 60 | - |

### 对标 Skia
| 场景 | Skia FPS | GPUI 目标 | 差距 |
|------|----------|-----------|------|
| 1000 圆形 | 60 | 60 | 0% |
| 1000 路径 | 60 | 60 | 0% |
| 渐变填充 | 60 | 60 | 0% |
| 文本渲染 | 60 | 60 | 0% |

---

## 📝 优化任务清单

---

### 【任务 0】性能基准测试（前置任务）

**优先级**：🔴 P0 - 最高

**任务描述**：
建立性能基准，量化当前性能，为后续优化提供对比依据。

**实现要求**：

1. **FPS 测量器**
```go
// render/benchmark_fps.go

type FPSResult struct {
    AverageFPS   float64
    MinFPS       float64
    MaxFPS       float64
    P95FPS       float64
    P99FPS       float64
    TotalFrames  int
    TotalTime    time.Duration
    FrameTimes   []time.Duration
}

func MeasureFPS(duration time.Duration, renderFunc func(frame int)) FPSResult {
    // 实现 FPS 测量
}
```

2. **测试场景**
```go
// render/benchmark_scenes_test.go

func BenchmarkSceneFPS(b *testing.B) {
    scenes := []Scene{
        SceneStaticCircles(100),
        SceneStaticCircles(500),
        SceneStaticCircles(1000),
        SceneAnimatedCircles(100),
        SceneAnimatedCircles(500),
        SceneAnimatedCircles(1000),
        SceneGradientFill(),
        SceneTextRendering(50),
        SceneTextRendering(200),
        SceneComplexPath(10),
        SceneComplexPath(50),
        SceneMixed(),
    }
    
    for _, scene := range scenes {
        b.Run(scene.Name, func(b *testing.B) {
            // 测量并报告 FPS
        })
    }
}
```

3. **Skia 对比测试**
```go
// render/benchmark_skia_compare_test.go

func TestCompareWithSkia(t *testing.T) {
    // 运行 Skia Python 脚本
    // 运行 GPUI 测试
    // 生成对比报告
}
```

**验收标准**：
- [ ] FPS 测量器准确（误差 < 5%）
- [ ] 覆盖所有主要渲染路径
- [ ] 自动生成性能报告
- [ ] 与 Skia 对比报告

**测试用例**：
```bash
# 运行所有基准测试
go test ./render/... -bench=BenchmarkSceneFPS -benchmem -count=3

# 生成性能报告
go test ./render/... -v -run=TestGenerateReport

# 运行 Skia 对比
go test ./render/... -v -run=TestCompareWithSkia
```

**依赖项**：无

**预计工时**：3 天

**负责人**：___________

**状态**：⬜ 未开始 / 🔄 进行中 / ✅ 已完成

---

### 【任务 1】路径缓存系统

**优先级**：🔴 P0 - 最高

**任务描述**：
实现路径 tessellation 结果缓存，避免相同路径重复 tessellate。

**技术背景**：
- 当前每次 `Fill()` / `Stroke()` 都会重新 tessellate 路径
- 动画场景中，相同路径每帧都重复计算
- Skia 的 `GrPathRenderer` 会缓存路径的 GPU 数据

**实现要求**：

1. **缓存键设计**
```go
// render/path_cache.go

type PathCacheKey struct {
    VerbHash   uint64   // 路径命令哈希
    CoordHash  uint64   // 坐标哈希
    TransformHash uint64 // 变换矩阵哈希（可选）
}
```

2. **缓存数据结构**
```go
type CachedPath struct {
    Tessellated *TessellatedMesh
    GPUBuffer   *GPUBuffer
    Key         PathCacheKey
    LastUsed    int64
    FrameCount  int
    Bounds      Rectangle
}

type PathCache struct {
    Cache   map[PathCacheKey]*CachedPath
    MaxSize int         // 最大缓存条目数（默认 10000）
    GPUSize int64       // GPU 缓冲区总大小限制（默认 256MB）
}
```

3. **缓存策略**
- LRU 淘汰：超过 MaxSize 时淘汰最久未使用的
- GPU 内存限制：超过 GPUSize 时淘汰
- 脏检测：路径变化时自动失效

4. **集成点**
- 在 `tryGPUFillWithMode()` 中调用缓存
- 在 `tryGPUStrokeWithMode()` 中调用缓存
- 在 GPU 会话中管理缓存生命周期

**验收标准**：
- [ ] 相同路径第二次渲染时，FPS 提升 50% 以上
- [ ] 路径动画场景 FPS 从 ? 提升到 40+
- [ ] 内存占用合理（不超过 256MB）
- [ ] 缓存命中率 > 80%（动画场景）
- [ ] 无内存泄漏

**测试用例**：
```go
func TestPathCacheStatic(t *testing.T) {
    ctx := render.NewContext(800, 600)
    path := createComplexPath()
    
    // 第一次渲染
    start := time.Now()
    ctx.DrawPath(path)
    require.NoError(t, ctx.Fill())
    firstTime := time.Since(start)
    
    // 第二次渲染（应该命中缓存）
    start = time.Now()
    ctx.DrawPath(path)
    require.NoError(t, ctx.Fill())
    secondTime := time.Since(start)
    
    // 验证第二次更快
    assert.Less(t, secondTime, firstTime/2)
}
```

**依赖项**：任务 0

**预计工时**：5 天

**负责人**：___________

**状态**：⬜ 未开始 / 🔄 进行中 / ✅ 已完成

---

### 【任务 2】GPU 渐变支持

**优先级**：🔴 P0 - 最高

**任务描述**：
将渐变渲染从 CPU 迁移到 GPU，使用渐变纹理实现。

**技术背景**：
- 当前渐变在 CPU 端计算每个像素的颜色
- Skia 使用预计算的渐变纹理，GPU 采样
- 渐变纹理可以缓存复用

**实现要求**：

1. **渐变纹理生成**
```go
// render/gpu_gradient.go

type GPUGradient struct {
    Texture  *Texture
    Type     GradientType  // Linear, Radial, Conic
    Stops    []ColorStop
    Matrix   Matrix
    Key      GradientKey
}

func NewGPUGradient(grad Gradient) *GPUGradient {
    key := computeGradientKey(grad)
    
    // 检查缓存
    if cached, ok := gradientCache[key]; ok {
        return cached
    }
    
    // 生成渐变纹理（256x1 或 256x256）
    texture := generateGradientTexture(grad)
    
    gpuGrad := &GPUGradient{
        Texture: texture,
        Type:    grad.Type,
        Stops:   grad.Stops,
        Matrix:  grad.Matrix,
        Key:     key,
    }
    
    gradientCache[key] = gpuGrad
    return gpuGrad
}
```

2. **渐变着色器**
```glsl
// render/gpu/shaders/gradient.wgsl

@group(0) @binding(0) var gradient_texture: texture_2d<f32>;
@group(0) @binding(1) var gradient_sampler: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let grad_uv = calculate_gradient_uv(uv, gradient_params);
    return textureSample(gradient_texture, gradient_sampler, grad_uv);
}
```

3. **渐变缓存**
```go
var gradientCache = struct {
    sync.RWMutex
    cache map[GradientKey]*GPUGradient
}{
    cache: make(map[GradientKey]*GPUGradient),
}
```

**验收标准**：
- [ ] 渐变渲染 FPS 提升 100% 以上（从 ? 到 60）
- [ ] 渐变质量与 CPU 渲染一致
- [ ] 渐变纹理缓存命中率 > 90%
- [ ] 支持线性、径向、锥形渐变
- [ ] 内存占用合理

**测试用例**：
```go
func TestGPULinearGradient(t *testing.T) {
    ctx := render.NewContext(800, 600)
    grad := render.NewLinearGradientBrush(0, 0, 800, 600).
        AddColorStop(0, render.Red).
        AddColorStop(1, render.Blue)
    
    fps := measureFPS(func(frame int) {
        ctx.SetFillBrush(grad)
        ctx.DrawRectangle(0, 0, 800, 600)
        _ = ctx.Fill()
    }, 100)
    
    assert.Greater(t, fps, 55.0)
}
```

**依赖项**：任务 0

**预计工时**：5 天

**负责人**：___________

**状态**：⬜ 未开始 / 🔄 进行中 / ✅ 已完成

---

### 【任务 3】批处理排序优化

**优先级**：🟡 P1 - 高

**任务描述**：
优化绘制操作的排序，减少 GPU 状态切换。

**技术背景**：
- 当前绘制操作按提交顺序执行
- 频繁的状态切换（颜色、纹理、裁剪）会降低性能
- Skia 会按材质、混合模式排序

**实现要求**：

1. **操作排序**
```go
// render/batch_sort.go

type DrawOp struct {
    Type       OpType
    Material   MaterialID
    BlendMode  BlendMode
    ClipRect   Rectangle
    Priority   int
}

func sortDrawOps(ops []DrawOp) {
    sort.Slice(ops, func(i, j int) bool {
        if ops[i].Material != ops[j].Material {
            return ops[i].Material < ops[j].Material
        }
        if ops[i].BlendMode != ops[j].BlendMode {
            return ops[i].BlendMode < ops[j].BlendMode
        }
        return ops[i].ClipRect.Min.X < ops[j].ClipRect.Min.X
    })
}
```

2. **延迟排序**
```go
func (rc *GPURenderContext) Flush(target GPURenderTarget) error {
    ops := rc.collectAllOps()
    sortDrawOps(ops)
    for _, op := range ops {
        rc.executeOp(op)
    }
}
```

3. **批量合并**
```go
func mergeAdjacentOps(ops []DrawOp) []DrawOp {
    merged := make([]DrawOp, 0, len(ops))
    for _, op := range ops {
        if len(merged) > 0 && canMerge(merged[len(merged)-1], op) {
            merged[len(merged)-1] = mergeOps(merged[len(merged)-1], op)
        } else {
            merged = append(merged, op)
        }
    }
    return merged
}
```

**验收标准**：
- [ ] 状态切换次数减少 50% 以上
- [ ] 混合场景 FPS 提升 20%+
- [ ] 排序开销 < 1ms（1000 个操作）
- [ ] 渲染结果与排序前一致

**依赖项**：无

**预计工时**：3 天

---

### 【任务 4】纹理图集优化

**优先级**：🟡 P1 - 高

**任务描述**：
优化纹理图集管理，减少纹理切换。

**实现要求**：

1. **图集管理器**
```go
// render/texture_atlas.go

type TextureAtlas struct {
    texture    *Texture
    packer     *BinPacker
    regions    map[AtlasKey]Rectangle
    maxSize    int
    format     TextureFormat
    dirty      bool
}

func (at *TextureAtlas) Add(key AtlasKey, data []byte, size image.Point) (Rectangle, error) {
    region, err := at.packer.Pack(size)
    if err != nil {
        return Rectangle{}, err
    }
    at.texture.Upload(region, data)
    at.regions[key] = region
    at.dirty = true
    return region, nil
}
```

2. **多图集支持**
```go
type AtlasManager struct {
    glyphAtlas    *TextureAtlas
    iconAtlas     *TextureAtlas
    gradientAtlas *TextureAtlas
}
```

**验收标准**：
- [ ] 纹理切换次数减少 70%+
- [ ] 图集利用率 > 80%
- [ ] 支持动态添加/移除
- [ ] 内存占用合理

**依赖项**：无

**预计工时**：4 天

---

### 【任务 5】资源缓存 LRU

**优先级**：🟡 P1 - 高

**任务描述**：
实现统一的资源缓存系统，使用 LRU 淘汰策略。

**实现要求**：

1. **LRU 缓存**
```go
// render/lru_cache.go

type LRUCache struct {
    capacity int
    size     int
    items    map[CacheKey]*CacheItem
    head     *CacheItem
    tail     *CacheItem
    mu       sync.RWMutex
}

type CacheItem struct {
    Key      CacheKey
    Value    interface{}
    Size     int
    LastUsed int64
    prev     *CacheItem
    next     *CacheItem
}
```

2. **缓存预算管理**
```go
type ResourceCache struct {
    pathCache     *LRUCache
    gradientCache *LRUCache
    textureCache  *LRUCache
    totalBudget   int
    currentUsage  int
}
```

**验收标准**：
- [ ] 缓存命中率 > 85%
- [ ] 淘汰策略有效（内存不超限）
- [ ] 并发安全
- [ ] 性能开销 < 1%

**依赖项**：无

**预计工时**：3 天

---

### 【任务 6】并行光栅化

**优先级**：🟡 P1 - 高

**任务描述**：
深度集成并行光栅化，提升 CPU 回退性能。

**技术背景**：
- `internal/parallel` 包已存在但未深度集成
- 多核 CPU 可以并行 tessellate 路径
- Skia 使用线程池并行光栅化

**实现要求**：

1. **并行 tessellate**
```go
// render/parallel_raster.go

func (r *Rasterizer) TessellateParallel(paths []Path) []TessellatedMesh {
    numWorkers := runtime.NumCPU()
    results := make([]TessellatedMesh, len(paths))
    
    var wg sync.WaitGroup
    semaphore := make(chan struct{}, numWorkers)
    
    for i, path := range paths {
        wg.Add(1)
        semaphore <- struct{}{}
        
        go func(idx int, p Path) {
            defer wg.Done()
            defer func() { <-semaphore }()
            results[idx] = r.tessellatePath(p)
        }(i, path)
    }
    
    wg.Wait()
    return results
}
```

2. **并行光栅化**
```go
func (r *Rasterizer) RasterizeParallel(paths []Path, bounds Rectangle) []Mask {
    numWorkers := runtime.NumCPU()
    masks := make([]Mask, len(paths))
    
    chunkSize := len(paths) / numWorkers
    var wg sync.WaitGroup
    
    for i := 0; i < numWorkers; i++ {
        start := i * chunkSize
        end := start + chunkSize
        if i == numWorkers-1 {
            end = len(paths)
        }
        
        wg.Add(1)
        go func(start, end int) {
            defer wg.Done()
            for j := start; j < end; j++ {
                masks[j] = r.rasterizePath(paths[j])
            }
        }(start, end)
    }
    
    wg.Wait()
    return masks
}
```

**验收标准**：
- [ ] CPU 回退场景 FPS 提升 2x+（多核）
- [ ] 并行效率 > 70%
- [ ] 无竞态条件
- [ ] 内存占用合理

**依赖项**：无

**预计工时**：5 天

---

### 【任务 7】亚像素精度提升

**优先级**：🟢 P2 - 中

**任务描述**：
将子像素精度从 4x 提升到 8x，改善小字号渲染质量。

**实现要求**：

1. **修改 EdgeBuilder AA 参数**
```go
// render/software.go

// 当前
eb := raster.NewEdgeBuilder(2) // 4x AA

// 优化后
eb := raster.NewEdgeBuilder(3) // 8x AA
```

2. **更新相关计算**
```go
// 注意：
// - 当前软件渲染入口在 NewSoftwareRenderer() 中创建 EdgeBuilder。
// - render/internal/raster/ 中也有多个测试显式使用 aaShift=2 或 aaShift=4。
// - 修改前必须确认 FDot6 -> FDot16 overflow 边界和视觉收益。
```

**验收标准**：
- [ ] 小字号（< 12px）渲染质量提升
- [ ] 性能开销 < 10%
- [ ] 无视觉瑕疵

**依赖项**：无

**预计工时**：3 天

---

## 🧪 测试计划

### 单元测试
- **覆盖率目标**：> 80%
- **测试范围**：路径、矩阵、颜色、变换等核心组件
- **运行频率**：每次 PR 必须通过
- **命令**：`go test ./render/... -v -short`

### 视觉回归测试
- **测试用例**：45 个基准测试用例
- **像素差异容忍度**：< 1%
- **运行频率**：每次提交自动运行
- **命令**：`go test ./render/... -v -run TestVisualRegression`

### 性能基准测试
- **运行频率**：每个优化前后都要运行
- **报告格式**：自动生成 FPS 对比报告
- **告警阈值**：FPS 下降 > 10%
- **命令**：`go test ./render/... -bench=. -benchmem -count=3`

### 压力测试
- **对象数量**：10000+ 对象渲染
- **帧数**：10000 帧内存稳定性
- **边界测试**：快速调整大小测试
- **命令**：`go test ./render/... -v -run TestStress -timeout 30m`

### 兼容性测试
- **GPU 测试**：Intel、NVIDIA、AMD
- **后端测试**：Vulkan、GLES、Software
- **分辨率测试**：720p - 4K
- **命令**：`go test ./render/... -v -run TestCompatibility`

---

## 🎯 性能基准目标

### 里程碑 1：基准测试建立（第 1 周）
| 任务 | 目标 | 状态 |
|------|------|------|
| 0.1 FPS 测量器 | 准确测量 FPS | ⬜ |
| 0.2 测试场景 | 覆盖所有渲染路径 | ⬜ |
| 0.3 Skia 对比 | 生成对比报告 | ⬜ |

### 里程碑 2：核心路径优化（第 2-4 周）
| 场景 | 目标 FPS | 当前 FPS | 提升 |
|------|----------|----------|------|
| 1000 圆形动画 | 60 | ? | - |
| 1000 路径动画 | 55 | ? | - |
| 渐变填充 | 60 | ? | - |

### 里程碑 3：内存和资源优化（第 5-6 周）
| 场景 | 目标 FPS | 当前 FPS | 提升 |
|------|----------|----------|------|
| 1000 圆形动画 | 60 | - | - |
| 1000 路径动画 | 60 | - | - |
| 渐变填充 | 60 | - | - |
| 内存占用 | < 200MB | - | - |

### 里程碑 4：高级渲染特性（第 7-10 周）
| 场景 | 目标 FPS | 当前 FPS | 提升 |
|------|----------|----------|------|
| 复杂 UI 场景 | 60 | ? | - |
| 混合渲染 | 60 | ? | - |

### 最终目标（对标 Skia）
| 场景 | Skia FPS | GPUI 目标 | 差距 |
|------|----------|-----------|------|
| 1000 圆形 | 60 | 60 | 0% |
| 1000 路径 | 60 | 60 | 0% |
| 渐变填充 | 60 | 60 | 0% |
| 文本渲染 | 60 | 60 | 0% |

---

## ⚠️ 风险评估

### 技术风险
| 风险 | 影响 | 概率 | 应对策略 |
|------|------|------|----------|
| GPU 兼容性问题 | 高 | 中 | 多 GPU 测试，回退到 CPU |
| 性能优化不及预期 | 中 | 高 | 分阶段验证，及时调整方向 |
| 内存泄漏 | 高 | 中 | 自动化内存测试，监控工具 |
| 视觉渲染错误 | 中 | 低 | 视觉回归测试，人工审核 |

### 进度风险
| 风险 | 影响 | 概率 | 应对策略 |
|------|------|------|----------|
| 优化难度超预期 | 中 | 中 | 预留 20% 缓冲时间 |
| 依赖库问题 | 中 | 低 | 提前调研，准备备选方案 |
| 人员变动 | 高 | 低 | 文档完善，知识共享 |

### 质量风险
| 风险 | 影响 | 概率 | 应对策略 |
|------|------|------|----------|
| 性能优化引入 Bug | 高 | 中 | 充分测试，代码审查 |
| 渲染质量下降 | 中 | 低 | 视觉测试，质量指标 |

---

## 📦 资源需求

### 硬件资源
- **测试 GPU**：
  - Intel HD Graphics（集成显卡）✅ 已有
  - NVIDIA GTX 1060+（独立显卡）
  - AMD Radeon（可选）
- **测试机器**：
  - Linux（主要开发）✅ 已有
  - Windows（兼容性测试）
  - macOS（Metal 后端）

### 软件资源
- **开发工具**：
  - Go 1.25+ ✅ 已有
  - Vulkan SDK ✅ 已有
  - RenderDoc（GPU 调试）
- **测试工具**：
  - pprof（性能分析）✅ 已有
  - valgrind（内存检查）
  - 自动化测试框架

### 人力需求
- **主要开发**：1-2 人
- **测试**：0.5 人
- **代码审查**：0.5 人
- **文档**：0.5 人

### 时间预算
- **总工期**：10 周
- **缓冲时间**：2 周（20%）
- **总预算**：12 周

---

## ✅ 验收标准

### 里程碑 1 验收（基准测试）
- [ ] FPS 测量器准确（误差 < 5%）
- [ ] 覆盖所有主要渲染路径
- [ ] 自动生成性能报告
- [ ] 与 Skia 对比报告

### 里程碑 2 验收（核心优化）
- [ ] 路径缓存命中率 > 80%
- [ ] 1000 圆形 FPS ≥ 60
- [ ] 1000 路径 FPS ≥ 55
- [ ] 渐变填充 FPS ≥ 60
- [ ] 内存占用 < 300MB
- [ ] 无内存泄漏
- [ ] 单元测试覆盖率 > 80%

### 里程碑 3 验收（资源优化）
- [ ] 纹理图集利用率 > 80%
- [ ] 资源缓存命中率 > 85%
- [ ] 内存占用 < 250MB
- [ ] 所有基准测试通过

### 里程碑 4 验收（高级特性）
- [ ] 并行光栅化效率 > 70%
- [ ] 亚像素精度提升可见
- [ ] 复杂场景 FPS ≥ 60

### 最终验收
- [ ] 所有性能目标达成
- [ ] 视觉回归测试全部通过
- [ ] 兼容性测试通过
- [ ] 文档完整
- [ ] 代码审查通过

---

## 📊 持续监控

### 性能监控
- **每日**：自动运行基准测试
- **每周**：生成性能趋势报告
- **每月**：性能对比分析

### 质量监控
- **每次提交**：单元测试 + 视觉测试
- **每日**：集成测试
- **每周**：压力测试

### 告警机制
- FPS 下降 > 10%：立即告警
- 内存泄漏：立即告警
- 测试失败：阻止合并

### 调优策略
- **快速调优**：热点代码优化
- **深度调优**：架构级优化
- **持续调优**：性能预算管理

---

## 📚 文档计划

### 开发文档
- [ ] 架构设计文档
- [ ] API 参考文档
- [ ] 性能优化指南
- [ ] 贡献者指南

### 用户文档
- [ ] 快速开始指南
- [ ] 最佳实践
- [ ] 常见问题
- [ ] 示例代码

### 测试文档
- [ ] 测试策略
- [ ] 测试用例说明
- [ ] 性能报告模板
- [ ] 故障排查指南

### 发布文档
- [ ] 变更日志
- [ ] 版本说明
- [ ] 升级指南
- [ ] 已知问题

---

## 🚀 发布计划

### 版本策略
- **主版本**：重大架构变更（1.0, 2.0）
- **次版本**：新功能 + 性能优化（1.1, 1.2）
- **补丁版本**：Bug 修复（1.1.1, 1.1.2）

### 发布周期
- **Alpha**：内部测试（每月）
- **Beta**：外部测试（每季度）
- **RC**：候选发布（按需）
- **Stable**：正式发布（按需）

### 发布检查清单
- [ ] 所有测试通过
- [ ] 性能基准达标
- [ ] 文档更新
- [ ] 变更日志更新
- [ ] 版本号更新
- [ ] 发布说明编写

### 回滚策略
- 保留最近 3 个版本
- 快速回滚机制
- 紧急修复流程

---

## 👀 代码审查标准

### 审查要点
- **正确性**：逻辑是否正确
- **性能**：是否有性能问题
- **可读性**：代码是否清晰
- **测试**：是否有充分测试
- **文档**：是否有必要注释

### 审查流程
1. 提交 PR
2. 自动测试运行
3. 人工审查（至少 1 人）
4. 修改反馈
5. 合并

### 审查清单
- [ ] 代码风格一致
- [ ] 无明显性能问题
- [ ] 有充分测试覆盖
- [ ] 文档更新（如需要）
- [ ] 无安全问题

---

## 💬 沟通计划

### 会议安排
- **每日站会**：15 分钟，同步进度
- **每周评审**：1 小时，代码审查
- **每月回顾**：2 小时，总结改进

### 沟通工具
- **即时通讯**：Slack/Teams
- **文档协作**：GitHub Wiki
- **代码管理**：GitHub Issues/PR

### 状态同步
- **进度看板**：GitHub Projects
- **性能仪表盘**：自动生成
- **测试报告**：自动化

---

## 📊 进度追踪表

### 里程碑 1：基准测试建立
| 任务 | 负责人 | 计划开始 | 计划结束 | 实际开始 | 实际结束 | 状态 | 备注 |
|------|--------|----------|----------|----------|----------|------|------|
| P0.E examples 视觉基线采集 |  | W0D1 | W0D2 | 2026-07-15 | 2026-07-15 | ✅ | 第一批必测 examples 基线已接入 |
| P0.2 render target/readback 校准 |  | W0D2 | W0D3 | 2026-07-15 | 2026-07-15 | ✅ | RGBA/BGRA/R8 + 256 pitch + clear-via-upload |
| P0.3 blend/alpha 一致性 |  | W0D3 | W0D4 | 2026-07-15 | 2026-07-15 | ✅ | Normal/SourceOver/Copy/Plus/Multiply 固定像素 + premul 边界 |
| P0.F examples 端到端修复 |  | W0D4 | W0D5 | 2026-07-15 | 2026-07-15 | ✅ | STRICT 全绿；残留 AA 记为可接受阈值 |
| P1.0 门禁基础设施（fallback 计数） |  | W1D1 | W1D2 | 2026-07-15 | 2026-07-15 | ✅ | Context path stats + visualcmd + STRICT |
| P1.1 ABI/WebGPU UI 子集审计 |  | W1D2 | W1D4 |  |  | ⬜ | L1/L2 硬证据 |
| P1.2 GPU 固定像素 blend/clip |  | W1D3 | W1D5 | 2026-07-15 |  | 🔄 | SourceOver/opaque/clip/hairline 首轮绿；扩展 Multiply/Plus/Copy |
| P1.3 Tier A 复杂 UI 绘制场景 A1–A8 |  | W1D4 | W2D3 |  |  | ⬜ | render 能力压测（非控件实现） |
| P1.4 Tier B 压力正确性 B1–B6 |  | W2D2 | W2D5 |  |  | ⬜ | Skia 方向正确性 |
| P1.5 render 能力门禁关闭评审 |  | W2D5 | W3D1 |  |  | ⬜ | 通过后可声明对标能力门槛 |
| 0.1 FPS 测量器 |  | W3D1 | W3D2 |  |  | ⬜ | 仅 P1 后启动性能基线 |
| 0.2 测试场景 |  | W1D2 | W1D3 |  |  | ⬜ |  |
| 0.3 Skia 对比 |  | W1D3 | W1D5 |  |  | ⬜ |  |

### 里程碑 2：核心路径优化
| 任务 | 负责人 | 计划开始 | 计划结束 | 实际开始 | 实际结束 | 状态 | 备注 |
|------|--------|----------|----------|----------|----------|------|------|
| 1 路径缓存 |  | W2D1 | W2D5 |  |  | ⬜ |  |
| 2 GPU 渐变 |  | W3D1 | W3D5 |  |  | ⬜ |  |
| 3 批处理排序 |  | W4D1 | W4D3 |  |  | ⬜ |  |

### 里程碑 3：内存和资源优化
| 任务 | 负责人 | 计划开始 | 计划结束 | 实际开始 | 实际结束 | 状态 | 备注 |
|------|--------|----------|----------|----------|----------|------|------|
| 4 纹理图集 |  | W5D1 | W5D4 |  |  | ⬜ |  |
| 5 资源缓存 LRU |  | W5D5 | W6D3 |  |  | ⬜ |  |

### 里程碑 4：高级渲染特性
| 任务 | 负责人 | 计划开始 | 计划结束 | 实际开始 | 实际结束 | 状态 | 备注 |
|------|--------|----------|----------|----------|----------|------|------|
| 6 并行光栅化 |  | W7D1 | W7D5 |  |  | ⬜ |  |
| 7 亚像素精度 |  | W8D1 | W8D3 |  |  | ⬜ |  |

---

## 🚨 问题记录和补充需求

### 发现的问题

| 日期 | 问题描述 | 影响范围 | 优先级 | 解决方案 | 状态 |
|------|----------|----------|--------|----------|------|
| 2026-07-14 | 切到 Rust wgpu-native 路径后，部分 `render/examples` 输出与源 go-wgpu 不一致，整体渲染效果不好 | examples、UI 控件渲染正确性、后续性能优化可信度 | P0 | 先建立 examples 视觉基线和 diff，再按 format/readback、texture、blend、transform、clip、path AA、text 分层修复 | 已缓解（P0 STRICT 门禁通过；残留 AA 后续精修） |
| 2026-07-14 | `commands.go` 仍保留旧 `Stub*ID` API，容易被误认为生产 WebGPU 命令路径 | 新人理解、架构维护 | P1 | 已标记为 legacy/test helper；生产路径集中到 `CoreCommandEncoder` 与 `gpu/webgpu` 对象 | 已缓解 |

### 补充需求

| 日期 | 需求描述 | 来源 | 优先级 | 状态 |
|------|----------|------|--------|------|
| 2026-07-14 | 固定最终架构为 `render -> gpu/webgpu object facade -> gpu/rwgpu -> wgpu-native dynamic library` | 架构调整 | P0 | 进行中 |
| 2026-07-14 | 新增 examples 视觉回归工具，支持 software/source-go-wgpu/rwgpu-native 输出对比和 PNG diff | 渲染差异排查 | P0 | 已实现（程序化 imagediff + env-gated STRICT；不强制上传 PNG） |
| 2026-07-14 | 后续从 wgpu-native header 自动生成 ABI binding，覆盖 Linux/Windows/macOS 动态库 | ABI 维护 | P1 | 待设计 |

### 技术决策记录

| 日期 | 决策 | 原因 | 影响 |
|------|------|------|------|
| 2026-07-14 | 不再把原 go-wgpu 作为 GPUI GPU 实现保留，默认目标改为 Rust wgpu-native | LCL Go 绑定使用 purego ffi；原 go-wgpu/goffi 路径存在冲突，且后续希望统一到 wgpu-native 能力 | 需要补齐 `gpu/webgpu` object facade，修复迁移后渲染差异 |
| 2026-07-14 | `render` 不直接调用 `gpu/rwgpu`，只调用 `gpu/webgpu` 对象层 | 避免 render 层被 ABI 细节污染，保持后续替换和跨平台 surface 接入空间 | `gpu/webgpu` 需要承担完整 object API 和资源生命周期 |
| 2026-07-14 | 在修复 examples 视觉一致性前暂停大规模性能优化 | 当前输出效果与源 go-wgpu 不一致，性能优化会掩盖正确性问题 | P0 阶段先做视觉回归、format/readback、blend/alpha 校准 |
| 2026-07-14 | `commands.go` 保留为 legacy/test helper，生产命令路径使用 `CoreCommandEncoder` | 该文件参数仍是 `Stub*ID`，强行接 native 会形成重复且半真实的 API | 后续可逐步把旧测试迁移到 `CoreCommandEncoder` 后删除 |

---

## 📚 参考资料

### Skia 源码
- `src/gpu/ganesh/GrPathRenderer.cpp` - 路径渲染
- `src/gpu/ganesh/GrOpsTask.cpp` - 操作批处理
- `src/gpu/ganesh/GrTextureAtlas.cpp` - 纹理图集
- `src/core/SkRasterizer.cpp` - 光栅化器

### 相关论文
- "A Resolution Independent Rendering Framework for Vector Graphics" - GPU 路径渲染
- "Fast GPU Path Rendering" - NVIDIA 路径渲染优化

### 内部文档
- `TASK_PLAN.md` - 已有任务计划（迁移和 FFI 替换）
- `render/internal/` - 内部实现

---

## 📝 变更日志

| 日期 | 版本 | 变更内容 | 作者 |
|------|------|----------|------|
| 2026-07-13 | 1.0 | 初始版本 | Claude |
| 2026-07-13 | 2.0 | 补充测试计划、性能目标、风险评估、资源需求、验收标准、监控策略、文档计划、发布计划、代码审查、沟通计划 | Claude |
| 2026-07-13 | 3.0 | 根据 gpui 库实际情况重写，更新项目背景、库结构、依赖关系 | Claude |
| 2026-07-13 | 3.1 | 补充新人/AI 开工指南、并行开发边界、任务卡模板，并修正渐变和 AA 示例 API | Codex |
| 2026-07-15 | 4.0 | 主线切换：Skia 2D 能力表 + MAINLINE S0–S4；本文降为档案 | Codex |
| 2026-07-15 | 3.11 | 澄清主线：仅 render/webgpu/rwgpu 能力补全与对标；移除“控件层开工”误表述 | Codex |
| 2026-07-15 | 3.10 | P1.0 path stats + P1.2 GPU 固定像素首轮（SourceOver/clip/hairline）；STRICT 强制 gpu_ops | Codex |
| 2026-07-15 | 3.9 | 升级 P1 Foundation Gate：复杂 UI 场景矩阵 + 分层硬证据；明确控件层开工阻断条件 | Codex |
| 2026-07-15 | 3.8 | P0 整体完成：P0.2 format/readback、P0.3 blend 固定像素、P0.F STRICT 全绿；更新进度表与可接受 AA 残留 | Codex |
| 2026-07-15 | 3.7 | P0.F：GPU 跳过 .notdef + isCJK 整串检测；cjk_text body/mixed ratio≈1.0 | Codex |
| 2026-07-15 | 3.6 | P0.F 首轮：GPU DrawImage CTM 四角点 + 非 solid fill 回退 CPU；images STRICT 通过 | Codex |
| 2026-07-15 | 3.5 | P0.E 第一批 examples 程序化视觉基线接入完成，记录 images/text/cjk/scene/gpu 指标与 P0.F 优先问题 | Codex |
| 2026-07-14 | 3.2 | 记录 Rust wgpu-native 目标架构、当前 native 验证状态、examples 视觉一致性 P0 计划、架构决策和遗留风险 | Codex |
|  |  |  |  |

---

**文档维护**：
- 每周五更新进度
- 发现问题及时记录
- 补充需求需评审后添加

**联系方式**：
- 技术问题：___________
- 进度问题：___________
