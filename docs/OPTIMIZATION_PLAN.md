# gg 渲染库底层优化开发计划

> 版本：2.0 | 更新日期：2026-07-13 | 状态：进行中

---

## 📋 项目概述

### 项目名称
gg 渲染库底层性能优化 - 对标 Skia

### 项目目标
将 gg 渲染库优化到能够支撑复杂 UI 控件库和高性能 2D 渲染的水平，性能对标 Skia。

### 当前状态
- ✅ 基础架构完善（批处理、实例化渲染、MSDF 文本）
- ✅ ShaderModule 内存泄漏已修复
- ⚠️ 路径渲染性能不足
- ⚠️ 渐变渲染使用 CPU
- ⚠️ 内存管理可优化

### 目标性能
| 场景 | 当前 | 目标 | 差距 |
|------|------|------|------|
| 1000 个圆角矩形动画 | 38 FPS | 60 FPS | 1.6x |
| 1000 个路径动画 | ~25 FPS | 60 FPS | 2.4x |
| 复杂文本渲染 | 45 FPS | 60 FPS | 1.3x |
| 渐变填充 | ~30 FPS | 60 FPS | 2x |

---

## 🎯 里程碑规划

### 里程碑 1：核心渲染路径优化（第 1-3 周）
**目标**：解决最核心的性能瓶颈

### 里程碑 2：内存和资源优化（第 4-5 周）
**目标**：减少内存占用，提升资源复用

### 里程碑 3：高级渲染特性（第 6-9 周）
**目标**：实现 Skia 级别的渲染能力

### 里程碑 4：验证和调优（第 10-11 周）
**目标**：性能对标，质量验证

---

## 📝 详细任务清单

---

### 【任务 1.1】路径缓存系统

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
- 在 `GPURenderSession` 中管理缓存生命周期

**验收标准**：
- [ ] 相同路径第二次渲染时，FPS 提升 50% 以上
- [ ] 路径动画场景 FPS 从 25 提升到 40+
- [ ] 内存占用合理（不超过 256MB）
- [ ] 缓存命中率 > 80%（动画场景）
- [ ] 无内存泄漏

**测试用例**：
```go
// 测试 1：静态路径缓存
func TestPathCacheStatic(t *testing.T) {
    ctx := gg.NewContext(800, 600)
    path := createComplexPath()
    
    // 第一次渲染
    start := time.Now()
    ctx.DrawPath(path)
    ctx.Fill()
    firstTime := time.Since(start)
    
    // 第二次渲染（应该命中缓存）
    start = time.Now()
    ctx.DrawPath(path)
    ctx.Fill()
    secondTime := time.Since(start)
    
    // 验证第二次更快
    assert.Less(t, secondTime, firstTime/2)
}

// 测试 2：动画路径缓存
func TestPathCacheAnimation(t *testing.T) {
    ctx := gg.NewContext(800, 600)
    
    fps := measureFPS(func(frame int) {
        path := createPathWithAnimation(float64(frame))
        ctx.DrawPath(path)
        ctx.Fill()
    }, 100)
    
    assert.Greater(t, fps, 40.0) // 目标 40+ FPS
}
```

**依赖项**：无

**预计工时**：5 天

**负责人**：___________

**状态**：⬜ 未开始 / 🔄 进行中 / ✅ 已完成

**问题记录**：
| 日期 | 问题描述 | 解决方案 | 状态 |
|------|----------|----------|------|
|      |          |          |      |

---

### 【任务 1.2】GPU 渐变支持

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
type GPUGradient struct {
    Texture  *Texture
    Type     GradientType  // Linear, Radial, Conic
    Stops    []ColorStop
    Matrix   Matrix
    Key      GradientKey
}

// 渐变纹理只生成一次
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
// 渐变 fragment shader
@group(0) @binding(0) var gradient_texture: texture_2d<f32>;
@group(0) @binding(1) var gradient_sampler: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    // 根据渐变类型计算 UV
    let grad_uv = calculate_gradient_uv(uv, gradient_params);
    
    // 从纹理采样
    return textureSample(gradient_texture, gradient_sampler, grad_uv);
}
```

3. **渐变缓存**
```go
var gradientCache = struct {
    sync.RWMutex
    cache map[GradientKey]*GPUGradient
    size  int
}{
    cache: make(map[GradientKey]*GPUGradient),
}
```

**验收标准**：
- [ ] 渐变渲染 FPS 提升 100% 以上（从 30 到 60）
- [ ] 渐变质量与 CPU 渲染一致
- [ ] 渐变纹理缓存命中率 > 90%
- [ ] 支持线性、径向、锥形渐变
- [ ] 内存占用合理

**测试用例**：
```go
func TestGPULinearGradient(t *testing.T) {
    ctx := gg.NewContext(800, 600)
    grad := gg.NewLinearGradient(0, 0, 800, 600)
    grad.AddColorStop(0, gg.Red)
    grad.AddColorStop(1, gg.Blue)
    
    fps := measureFPS(func(frame int) {
        ctx.SetGradient(grad)
        ctx.DrawRectangle(0, 0, 800, 600)
        ctx.Fill()
    }, 100)
    
    assert.Greater(t, fps, 55.0)
}
```

**依赖项**：无

**预计工时**：5 天

**负责人**：___________

**状态**：⬜ 未开始 / 🔄 进行中 / ✅ 已完成

---

### 【任务 1.3】批处理排序优化

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
type DrawOp struct {
    Type       OpType
    Material   MaterialID
    BlendMode  BlendMode
    ClipRect   Rectangle
    Priority   int
    Data       interface{}
}

func sortDrawOps(ops []DrawOp) {
    sort.Slice(ops, func(i, j int) bool {
        // 1. 先按材质排序（减少纹理切换）
        if ops[i].Material != ops[j].Material {
            return ops[i].Material < ops[j].Material
        }
        // 2. 再按混合模式排序
        if ops[i].BlendMode != ops[j].BlendMode {
            return ops[i].BlendMode < ops[j].BlendMode
        }
        // 3. 最后按裁剪区域排序
        return ops[i].ClipRect.Min.X < ops[j].ClipRect.Min.X
    })
}
```

2. **延迟排序**
```go
func (rc *GPURenderContext) Flush(target GPURenderTarget) error {
    // 收集所有操作
    ops := rc.collectAllOps()
    
    // 排序优化
    sortDrawOps(ops)
    
    // 执行优化后的操作
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

### 【任务 2.1】纹理图集优化

**优先级**：🟡 P1 - 高

**任务描述**：
优化纹理图集管理，减少纹理切换。

**技术背景**：
- 每次纹理切换都有 GPU 开销
- 小纹理（字形、图标）应该打包到图集
- Skia 使用 GrTextureAtlas 管理

**实现要求**：

1. **图集管理器**
```go
type TextureAtlas struct {
    texture    *Texture
    packer     *BinPacker
    regions    map[AtlasKey]Rectangle
    maxSize    int
    format     TextureFormat
    dirty      bool
}

func (at *TextureAtlas) Add(key AtlasKey, data []byte, size image.Point) (Rectangle, error) {
    // 尝试打包到图集
    region, err := at.packer.Pack(size)
    if err != nil {
        return Rectangle{}, err // 图集已满
    }
    
    // 上传数据
    at.texture.Upload(region, data)
    at.regions[key] = region
    at.dirty = true
    
    return region, nil
}
```

2. **多图集支持**
```go
type AtlasManager struct {
    glyphAtlas    *TextureAtlas  // 字形图集
    iconAtlas     *TextureAtlas  // 图标图集
    gradientAtlas *TextureAtlas  // 渐变图集
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

### 【任务 2.2】资源缓存 LRU

**优先级**：🟡 P1 - 高

**任务描述**：
实现统一的资源缓存系统，使用 LRU 淘汰策略。

**实现要求**：

1. **LRU 缓存**
```go
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

### 【任务 3.1】并行光栅化

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
    
    // 分配工作
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

### 【任务 3.2】亚像素精度提升

**优先级**：🟢 P2 - 中

**任务描述**：
将子像素精度从 4x 提升到 8x，改善小字号渲染质量。

**实现要求**：

1. **修改 aaShift 常量**
```go
// 当前
const aaShift = 2  // 4x 子像素

// 优化后
const aaShift = 3  // 8x 子像素
```

2. **更新相关计算**
```go
func (r *Rasterizer) rasterizePath(path Path) Mask {
    // 使用 8x 子像素
    shift := aaShift
    scale := 1 << shift
    
    // 坐标缩放
    scaledPath := path.Transform(Matrix{
        A: float64(scale), D: float64(scale),
    })
    
    // 光栅化...
}
```

**验收标准**：
- [ ] 小字号（< 12px）渲染质量提升
- [ ] 性能开销 < 10%
- [ ] 无视觉瑕疵

**依赖项**：无

**预计工时**：3 天

---

### 【任务 4.1】基准测试套件

**优先级**：🔴 P0 - 最高

**任务描述**：
创建完整的基准测试套件，用于量化优化效果。

**测试场景**：

1. **基础图形测试**
```go
func BenchmarkCircles(b *testing.B) {
    ctx := gg.NewContext(800, 600)
    
    b.Run("100", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            for j := 0; j < 100; j++ {
                ctx.DrawCircle(rand.Float64()*800, rand.Float64()*600, 20)
                ctx.Fill()
            }
        }
    })
}
```

2. **动画测试**
3. **渐变测试**
4. **文本测试**
5. **路径测试**
6. **混合场景测试**

**验收标准**：
- [ ] 覆盖所有主要渲染路径
- [ ] 测试结果可复现
- [ ] 自动生成报告

**依赖项**：无

**预计工时**：3 天

---

## 🧪 测试计划

### 单元测试
- **覆盖率目标**：> 80%
- **测试范围**：路径、矩阵、颜色、变换等核心组件
- **运行频率**：每次 PR 必须通过

### 视觉回归测试
- **测试用例**：45 个基准测试用例
- **像素差异容忍度**：< 1%
- **运行频率**：每次提交自动运行

### 性能基准测试
- **运行频率**：每个优化前后都要运行
- **报告格式**：自动生成 FPS 对比报告
- **告警阈值**：FPS 下降 > 10%

### 压力测试
- **对象数量**：10000+ 对象渲染
- **帧数**：10000 帧内存稳定性
- **边界测试**：快速调整大小测试

### 兼容性测试
- **GPU 测试**：Intel、NVIDIA、AMD
- **后端测试**：Vulkan、Metal、DX12
- **分辨率测试**：720p - 4K

---

## 🎯 性能基准目标

### 里程碑 1 完成后
| 场景 | 目标 FPS | 当前 FPS | 提升 |
|------|----------|----------|------|
| 1000 圆形动画 | 60 | 38 | 58% |
| 1000 路径动画 | 55 | 25 | 120% |
| 渐变填充 | 60 | 30 | 100% |

### 里程碑 2 完成后
| 场景 | 目标 FPS | 当前 FPS | 提升 |
|------|----------|----------|------|
| 1000 圆形动画 | 60 | - | - |
| 1000 路径动画 | 60 | - | - |
| 渐变填充 | 60 | - | - |
| 内存占用 | < 200MB | - | - |

### 里程碑 3 完成后
| 场景 | 目标 FPS | 当前 FPS | 提升 |
|------|----------|----------|------|
| 复杂 UI 场景 | 60 | 20 | 200% |
| 混合渲染 | 60 | 25 | 140% |

### 最终目标（对标 Skia）
| 场景 | Skia FPS | gg 目标 | 差距 |
|------|----------|---------|------|
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
  - Intel HD Graphics（集成显卡）
  - NVIDIA GTX 1060+（独立显卡）
  - AMD Radeon（可选）
- **测试机器**：
  - Linux（主要开发）
  - Windows（兼容性测试）
  - macOS（Metal 后端）

### 软件资源
- **开发工具**：
  - Go 1.21+
  - Vulkan SDK
  - RenderDoc（GPU 调试）
- **测试工具**：
  - pprof（性能分析）
  - valgrind（内存检查）
  - 自动化测试框架

### 人力需求
- **主要开发**：1-2 人
- **测试**：0.5 人
- **代码审查**：0.5 人
- **文档**：0.5 人

### 时间预算
- **总工期**：11 周
- **缓冲时间**：2 周（20%）
- **总预算**：13 周

---

## ✅ 验收标准

### 里程碑 1 验收
- [ ] 路径缓存命中率 > 80%
- [ ] 1000 圆形 FPS ≥ 60
- [ ] 1000 路径 FPS ≥ 55
- [ ] 渐变填充 FPS ≥ 60
- [ ] 内存占用 < 300MB
- [ ] 无内存泄漏
- [ ] 单元测试覆盖率 > 80%

### 里程碑 2 验收
- [ ] 纹理图集利用率 > 80%
- [ ] 资源缓存命中率 > 85%
- [ ] 内存占用 < 250MB
- [ ] 所有基准测试通过

### 里程碑 3 验收
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

### 里程碑 1：核心渲染路径优化
| 任务 | 负责人 | 计划开始 | 计划结束 | 实际开始 | 实际结束 | 状态 | 备注 |
|------|--------|----------|----------|----------|----------|------|------|
| 1.1 路径缓存 |  | W1D1 | W1D5 |  |  | ⬜ |  |
| 1.2 GPU 渐变 |  | W2D1 | W2D5 |  |  | ⬜ |  |
| 1.3 批处理排序 |  | W3D1 | W3D3 |  |  | ⬜ |  |

### 里程碑 2：内存和资源优化
| 任务 | 负责人 | 计划开始 | 计划结束 | 实际开始 | 实际结束 | 状态 | 备注 |
|------|--------|----------|----------|----------|----------|------|------|
| 2.1 纹理图集 |  | W4D1 | W4D4 |  |  | ⬜ |  |
| 2.2 资源缓存 LRU |  | W4D5 | W5D3 |  |  | ⬜ |  |

### 里程碑 3：高级渲染特性
| 任务 | 负责人 | 计划开始 | 计划结束 | 实际开始 | 实际结束 | 状态 | 备注 |
|------|--------|----------|----------|----------|----------|------|------|
| 3.1 并行光栅化 |  | W6D1 | W6D5 |  |  | ⬜ |  |
| 3.2 亚像素精度 |  | W7D1 | W7D3 |  |  | ⬜ |  |

### 里程碑 4：验证和调优
| 任务 | 负责人 | 计划开始 | 计划结束 | 实际开始 | 实际结束 | 状态 | 备注 |
|------|--------|----------|----------|----------|----------|------|------|
| 4.1 基准测试 |  | W10D1 | W10D3 |  |  | ⬜ |  |
| 4.2 性能调优 |  | W10D4 | W11D3 |  |  | ⬜ |  |
| 4.3 文档完善 |  | W11D4 | W11D5 |  |  | ⬜ |  |

---

## 🚨 问题记录和补充需求

### 发现的问题

| 日期 | 问题描述 | 影响范围 | 优先级 | 解决方案 | 状态 |
|------|----------|----------|--------|----------|------|
| 2026-07-13 | ShaderModule.irModule 内存泄漏 | 所有场景 | 🔴 高 | 添加 m.irModule = nil | ✅ 已修复 |
|      |  |  |  |  |  |

### 补充需求

| 日期 | 需求描述 | 来源 | 优先级 | 状态 |
|------|----------|------|--------|------|
|  |  |  |  |  |

### 技术决策记录

| 日期 | 决策 | 原因 | 影响 |
|------|------|------|------|
| 2026-07-13 | 选择路径缓存作为第一个优化 | 通用性最强，收益最大 | 所有场景 |
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
- `internal/gpu/README.md` - GPU 架构说明
- `docs/ARCHITECTURE.md` - 整体架构

---

## 📝 变更日志

| 日期 | 版本 | 变更内容 | 作者 |
|------|------|----------|------|
| 2026-07-13 | 1.0 | 初始版本 | Claude |
| 2026-07-13 | 2.0 | 补充测试计划、性能目标、风险评估、资源需求、验收标准、监控策略、文档计划、发布计划、代码审查、沟通计划 | Claude |
|  |  |  |  |

---

**文档维护**：
- 每周五更新进度
- 发现问题及时记录
- 补充需求需评审后添加

**联系方式**：
- 技术问题：___________
- 进度问题：___________
