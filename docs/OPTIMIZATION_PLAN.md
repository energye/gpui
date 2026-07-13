# GPUI 渲染库底层优化开发计划

> 版本：3.1 | 更新日期：2026-07-13 | 状态：待启动
> 项目：github.com/energye/gpui
> 文档位置：/home/yanghy/app/projects/gogpu/gpui/docs/OPTIMIZATION_PLAN.md

---

## 📋 项目概述

### 项目名称
GPUI 渲染库底层性能优化 - 对标 Skia

### 项目背景
GPUI 是从 gogpu 生态迁移而来的独立渲染库，已完成：
- ✅ 阶段一：代码迁移与基础清理
- ✅ 阶段二：goffi 替换为 purego（FFI 中间层）
- ✅ 阶段三：验证测试
- ⏳ 阶段四：代码清理与优化（待开始）
- ⏳ 阶段五：框架集成（待开始）

### 项目目标
将 GPUI 渲染库优化到能够支撑复杂 UI 控件库和高性能 2D 渲染的水平，性能对标 Skia。

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
│   ├── webgpu/          # WebGPU 后端
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
| render（原 gg） | ✅ 可用 | 2D 渲染核心 |
| gpu（原 wgpu） | ✅ 可用 | GPU HAL 层 |
| ffi | ✅ 完成 | purego FFI 中间层 |
| text | ✅ 可用 | 文本渲染 |
| scene | ✅ 可用 | 场景图 |

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
| 0.1 FPS 测量器 |  | W1D1 | W1D2 |  |  | ⬜ |  |
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
|  |  |  |  |  |  |

### 补充需求

| 日期 | 需求描述 | 来源 | 优先级 | 状态 |
|------|----------|------|--------|------|
|  |  |  |  |  |

### 技术决策记录

| 日期 | 决策 | 原因 | 影响 |
|------|------|------|------|
|  |  |  |  |

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
|  |  |  |  |

---

**文档维护**：
- 每周五更新进度
- 发现问题及时记录
- 补充需求需评审后添加

**联系方式**：
- 技术问题：___________
- 进度问题：___________
