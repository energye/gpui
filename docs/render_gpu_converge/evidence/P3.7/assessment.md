# P3.7 2.5D — 影响面评估（开工实测，2026-11-03）

- 分支 `feat/p2-收敛整合`，HEAD `1d70d150`（工作区仅本任务证据目录新增，零代码变更）。
- 环境三件套照门禁口径：`CGO_ENABLED=1 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so`。
- 方法：全部结论来自本轮实测（读码到行号 + 探针跑数），规划文档只作对照；凡与登记不符，单列「与登记不符」。
- 探针脚本为一次性临时文件，跑完即删，归档在 `probes/`（原始输出）。

---

## 1 透视链现状：类型与数学齐了，**绘制侧只有单张 quad，且不能染色不能合批**

### 1.1 已交付的一截（P3.5 批 3）逐点复核

| 环节 | 位置 | 实测状态 |
|---|---|---|
| 4×4 类型与数学 | `render/matrix4.go:22,63,78,95` | Mul/TransformVec4/Invert/At/IsIdentity 齐 |
| 投影构造 | `render/matrix4.go:151 ProjectorMatrix4(focal,center,camera)` | 行主序装 8 个非零元，非有限/焦点≤0 拒绝 |
| 投影口 | `render/matrix4.go:180 ProjectPoint` / `:198 ProjectQuad` | 返回屏幕点 + 齐次 w；behind-eye（w≤0）与非有限拒绝；整 quad 任一角失败即全失败 |
| 仿射回退判定 | `render/matrix4.go:218 Affine` | 非纯提升仿射返回 ok=false，不静默压扁 |
| 绘制口 | `render/m4_quadclass.go:42,52`（`QuadDrawOptions.Perspective`+`Weights[4]`） | 权重非法 `ErrQuadBadWeights`；透视+bicubic `ErrQuadPerspectiveBicubic` |
| CPU 腿 | `render/m4_trisampler.go:32 newTriSamplerPerspective`、`render/m4_quadimage.go:194-196` | 1/w 有理插值 |
| GPU 腿 | `render/internal/gpu/gpu_render_context.go:1445 QueueImageDrawQuadW` → `render/internal/gpu/render_session.go:4291 packPerspectiveQuads` → `:4344` 发射 | 36B 顶点、clip-space 真 w、独立 `textured_quad_persp.wgsl` 管线（`:195-213` perspStencil/perspDepthClip 一对） |

**ProjectorMatrix4 与引擎 Projector 的真类型对拍**（探针 `probes/projector-align.txt`；引擎侧 `engine/camera/project.go:76`，批 3 只在 `render/matrix4_test.go:126` 对了闭式公式，没对真类型）：

- 294 格（3 组相机 × 14 档深度 × 7 个世界点，含焦点面、眼后、Inf/NaN、1e307）：
  - **数值漂移最大相对差 1.819e-09**（绝对 ≈4.5e-7 像素），出现在 `d=-99.999999`（scale 1e8）的近焦点面极端档——两边算法求序不同（`center+(world-camera)*scale` vs `(f·x + cx·d + f(cx−camx))/(d+f)`）。**判定：无实质漂移，不需回改。**
  - **ok 判定分叉 17 格，全是同一形态**：`world = 1e307` 时引擎接受、Matrix4 拒绝。根因实测清楚——Matrix4 先乘 `focal*x`（1e307×100 → Inf）再除 w，引擎先算 scale 再乘。**方向是 Matrix4 更早 fail-closed（宁可整张不画，不画错像素）**，不是画错。实际 2.5D 世界坐标量级（≤1e6）永不触界；登记不修，写进 P3.7 文档口径即可。
  - 未发现 Matrix4 接受而引擎拒绝的反向分叉。

### 1.2 真缺口一：**透视 quad 一张一 draw call，uniform slab 每帧 250KB**

合批谓词 `render/internal/gpu/image_pipeline.go:1172 canMergeImageDraw` 第 `:1178` 行：

```go
if a.Perspective || b.Perspective {
    return false   // 透视 quad 永不与任何东西合批，包括另一张透视 quad
}
```

探针实测（`probes/persp-batch-cost.txt`，同一张图 N=1000 张 quad，同 opacity/采样/视口/图源）：

| 形态 | draw call（=slot 数） | 合批进多 quad 槽的 quad | uniform slab |
|---|---|---|---|
| 仿射 | **1** | 1000 | 0.2KB |
| 仿射 + 逐顶点染色 | **1** | 1000 | 0.2KB |
| 透视（权重逐张不同） | **1000** | 0 | **250.0KB** |

顶点侧不是障碍：`render_session.go:4552 packPerspectiveQuads` 已把**所有**透视 quad 连续打进同一块 36B 缓冲（`pIdx` 按命令序递增），发射处 `:4345-4364` 却写死 `vertCnt: 6`。也就是说**多 quad 透视合批需要的管线、缓冲、槽位机制全在，只差谓词放行 + firstVertex/vertCnt 改按 pIdx 跨度**。参照物：`stage_chase` 窗 1000 prop 远先近后（`examples/engine/stage_chase/main.go:241`）就是这类量级。

### 1.3 真缺口二：**透视腿不带染色、不带混合、不带 bicubic**

- `QueueImageDrawQuadW`（`gpu_render_context.go:1445`）签名里没有 tint/blend 参数，注释自陈 `this entry never carries tint, blend, or bicubic`；`ImageDrawCommand.Tint*`（`image_pipeline.go:104-110`）在透视路径恒零值。
- `DrawImageQuad` 的 `BlendMode` 非 Normal 直接 `ErrQuadUnsupportedBlend`（`render/m4_quadclass.go:41,58`）。
- 对照：仿射批绘腿**有** tint——`render/vertices_atlas.go:282 QueueImageDrawTint`。
- 后果：2.5D 的「精灵按深度投影 + 被灯光染色」这两个需求现在**互斥**，只能选一个。灯光侧不是假想需求：`engine/light/light.go:358 FactorAt`/`:402 LitCPU` 已在算逐点光强，`engine/sprite/atlas.go` 的 `AtlasSprite.Tint` 是它的出口。

### 1.4 真缺口三：**批绘入口完全没有透视腿**

- `AtlasSprite`（`render/vertices.go:77-95`）字段：SrcX/Y/W/H、DstX/Y/W/H、Opacity、Rot/Flip/Pivot、Tint、Filter、Mipmap——**四角 + 权重无处放**。
- `AtlasDrawOptions` 是空结构体（`render/vertices.go:110`）；`DepthDrawOptions` 只有 `DepthTest bool`（`render/depth_r5.go:48-51`）。
- `DepthSprite`（`render/depth_r5.go:43`）= `AtlasSprite + Depth + Name`，`DrawDepthSprites` 排序后直接调 `DrawAtlasEx`（`:132` 起）——**Depth 只用来排序，从不参与投影**，深度值不改变精灵屏幕尺寸。
- 现网实践佐证：`examples/engine/camera/main.go:924,940` 调 `proj.Project(sh, 0)` 后 `_, ` **丢弃返回的 scale**，矩形尺寸写死 80×60 / 60×28；全仓非测试调用点无一处用 scale 改尺寸（`DepthToScale` 生产消费者 = 0，仅 `project.go:80,117` 内部与 `engine/camera/parallax.go:129`）。**即「按深度缩放精灵」这条 2.5D 基本形态目前不存在。**

### 1.5 与登记不符

- **PLAN §5.3「render 层缺 4×4 透视」**：类型/数学/投影口/单张 quad 透视已交付（P3.5 批 3），真实缺的是**批绘腿、染色腿、合批**三项 + Canvas 级 4×4 栈（批 3 明文挂本项，`台账 §2.1` P3.5 行 D4）。
- **catalog §13 P3.7 行「相机/投影参数进绘制选项」**：`QuadDrawOptions` 现在吃的是**已投影的屏幕角 + 权重**，投影在调用方（engine 侧 `ProjectorMatrix4`→`ProjectQuad`）做完。「绘制选项里放 Matrix4/相机」目前**不存在也没有必要**——但「批绘选项里放投影器」是缺的，见 §1.4。

### 1.6 补测（写方案时二次核实，两处对方案有实质影响）

- **透视腿的 tint 槽已经在布局和 shader 里，只是 Go 侧写死成白**：`perspVertexLayout`（`render/internal/gpu/image_pipeline.go:1248-1258`）第 3 个属性就是 `@location(2) tint vec4<f32>`（offset 16，36B = pos8+uv8+tint16+w4），`shaders/textured_quad_persp.wgsl:27` 亦声明 `tint` 并随 `VertexOutput` 传到片元。真正写死的是 `buildPerspVerticesInto:1264-1272` 里的 `const tr,tg,tb,ta = 1,1,1,1`（注释自陈「透视 quad 只走 DrawImageQuad 队列口，那里没有 tint 通道」）。⇒ **§1.3 的「透视腿不能染色」是 Go 侧管道缺失，不是布局/shader 限制**：补 `QueueImageDrawQuadW` 形参 + `ImageDrawCommand.Tint*`（字段本就在 `image_pipeline.go:104-110`，premul 存储约定与仿射同）+ 打包函数取值即可，**WGSL 与顶点布局零改动**。
- **`AtlasSprite` 的 Mipmap 字段在引擎出口被整段丢掉**：`engine/sprite/atlas.go:217 ToRender()` 逐字段转 `render.AtlasSprite`，只填 `Filter`（来源是 sprite 自家的四档 `AtlasFilter`，`:23-34`），**从不填 `Mipmap`**（P3.6 批 2 加的字段，`render/vertices.go:92-94`）；且 `engine/tex.Filter`（带 `Mip`）与 `sprite.AtlasFilter` 之间**没有任何映射**——`tex.Filter.SamplerParams` 零消费者只是表象，真断点是两处：① `tex.Filter` 无从进入 sprite 的过滤选择；② sprite→render 转换不传 mip 档。⇒ 批 4 的接线面比 §5 原记的「补一个 renderconv 函数」大一点，要同时闭这两处。


---

## 2 z 序与绘制序：**排序三线并存、render 层零 z-test、深度附件已被裁剪占用**

### 2.1 引擎侧三套同名能力（实测）

| 能力 | 位置 | 口径 | 生产消费者 |
|---|---|---|---|
| `camera.YSort25D` | `engine/camera/ysort25d.go:60`，键 `YSortKey(p)=Y+0.001*(X+Z)`（`:43`，**是函数不是常量**），`sort.SliceStable`（`:67`）+ 平票回落 `Order`（`:49`） | 冻结 2026-10-04（`engine/camera/doc.go:94`） | **engine 侧 0**；仅 `examples/engine/25d_basis/main.go:392,528` |
| `sprite.Item + FeetY` 分带 | `engine/sprite/ysort.go:37,40`（`Item{Name,Layer,FeetY}`）、`Sort`（`:68`）、`Layer`（`:22`） | 脚底 y 分带 | `examples/engine/stage_chase/main.go:227` |
| `sprite.DeepItem/DepthSort` | `engine/sprite/depth.go:23,73`，注释自陈「follows the R5/engine/camera convention」（`:19-21`） | Depth 越大越远 | `engine/particle/*`（trail/emitter/gpu）+ `examples/engine/stage_chase/main.go:241,910` |
| `render.DepthSprite/SortDepthSprites` | `render/depth_r5.go:43,99,132` | 同口径的另一份实现（在 render 层） | 仅 `examples/engine/sprite/main.go:411-484`、`examples/ui_render_base_recording/scene.go:553`（spy） |

`engine/sprite/depth.go` 与 `render/depth_r5.go` 是**同一能力的两份实现**（`SetDepth` 两份：`render/depth_r5.go:86` 与 `engine/sprite/depth.go:44`）。按「收敛只动实现、能力面随迁」与「分层归位（排序是 2.5D 语义不是通用图形算法）」，本项要拍板归位方向，不是简单加东西。

### 2.2 render 层遮挡模型 = 纯画家算法，无 z-test（实测）

- 提交序即盖序：`gpu_render_context.go:4134-4139` 的 `drawTier*`（SDF=1/convex=2/stencil=3/image=4/gpuTex=5/text=6）只用于 `ensureDrawOrder`（`:4169`）在**跨 tier 回退时封批**（`recordScissorSegment`），不是深度。
- 图像管线声明的深度状态全部 `DepthWriteEnabled: false`（`render/internal/gpu/pipeline_helpers.go:44,80`），深度比较 `CompareFunctionGreaterEqual` 是给 **GPU-CLIP-003a 路径裁剪**用的（`depth_clip.go:31-34` 自陈 stencil-then-cover-to-depth）。
- **结论：想给精灵做真 z-test，必须往同一张 Depth24PlusStencil8 附件里写深度，与现有裁剪机制直接抢附件**。这不是「加个字段」，是重写裁剪/深度的职责划分。
- `camera.YSortZ(rank)`（`ysort25d.go:72`，基槽 −4000/步长 2/奇数槽留给影子，`:28-31`）**全仓零消费者**（仅 `engine/camera/basis25d_test.go:140` 自测）；`shadow25d.go:22-23` 那句「窗把影子放在主人 z 槽上方一格」是**未落地口径**——25d_basis 窗里影子画在地面线固定 y（`examples/engine/25d_basis/main.go:556-558`），不占 z 槽。**与登记不符**：冻结清单里的 z 槽从未参与绘制，「排序归引擎、绘制序归 render」的界线至今只剩「引擎排完序按序调用」这一种实现。

### 2.3 世界高度：**引擎实体没有高度字段**

- `engine/world/entity.go:29-32` `Transform{Pos core.Vec2; Rot float64; Scale core.Vec2}` — **无 Z/高度**；全 engine 只有 `camera` 包 3 个文件用 `core.Vec3`（basis25d/ysort25d/shadow25d）。
- `engine/light/{normal,rough}.go` 的 `Height` 是**灯悬挂高度**（`normal.go:255`），不是体表高度。
- 2.5D「站得高被树挡不住 / 影子按身高落地」这条链缺**数据入口**：`shadow25d.go:13-19` 自陈「no 3D bodies, landing math only / the 3D ray hit point feeds in later」——`ShapeCast3D` 那一环不存在。

---

## 3 材质口：P3.2 只覆盖填充路，**精灵（贴图 quad）拿不到 shader**

### 3.1 契约现状（D1/D8/D9/D10 审查修复后，实测）

| 契约 | 位置 | 实测形态 |
|---|---|---|
| WGSL 入口 | `render/shader_effect.go:120`、`render/internal/gpu/user_shader.go:266`、`pipeline_helpers.go:22` | `fs_main` **硬编码三处**，无 entry 参数 |
| 返回类型 | `render/shader_effect.go:177-186` | 必须 `vec4<f32>` |
| 槽位表 | `render/shader_effect.go:109-113,141-168` | binding0 uniform struct（`ir.TypeSize ≤ 256`）、binding1 float texture、binding2 sampler；**binding(3) 直接拒** |
| premul 输出 | 契约文本 `render/shader_effect.go:32-35`；转换 `user_shader_fill.go:86,160-170` | **纯约定无校验**；GPU 侧 pass1 读回后一次性去预乘 |
| 绑定派生 | `render/shader_effect.go:244-257` → `:69 ShaderBinding()` → `user_shader.go:215` | 管线缓存布局唯一真源，`SetImage`/改 uniform 长度不再让缓存过期（钉 `render/p32_pipeline_cache_test.go:83,103,118,134`） |
| 生命周期 | `render/shader_effect.go:365,372,405`、`gpu_shared.go:249,676` | 广播驱逐钩子 + `Closed()` 三处拒用 |
| 非法 WGSL 拒绝 | `render/shader_effect.go:263`（Parse→Lower→Validate） | **只拒自研前端那一层**；gles 的 `CreateShaderModule` 仅存字符串（`gpu/gwgpu/gles/device.go:451-461`），真翻译在 `CreateRenderPipeline`（`gles/device.go:1045`）→ 后端级失败**落在首绘**，`user_shader_fill.go:177-180` 打 stderr 后 `ErrFallbackToCPU` |
| 错误分派 | — | **零导出哨兵**：全仓无 `ErrShader*`/`ErrUniform*`，失败全是现场 `fmt.Errorf` |
| CPU 回退 | `render/context_gpu.go:634`、`render/context_draw.go:324` | **无 CPU 求值器**；无 GPU 上下文静默跳过且不计 fallback |

### 3.2 真缺口（逐条实测）

1. **shader 只有一个消费者**：`gpu_render_context.go:2037`（`FillPath` 首查 `paint.Shader`）+ `context_gpu.go:554` 的绕行守卫。全仓 `.Shader` 消费点共 7 处（`context_draw.go:324`、`context_gpu.go:554,634`、`user_shader_fill.go:30,99`、`draw_shader_mask.go:50`、`paint.go:178`）。**`DrawImageOptions`/`QuadDrawOptions`/`AtlasSprite` 三个图像入口都没有 shader 字段** → 2.5D 想要的「材质随精灵走」现在做不到。
2. **描边快路吃不到 shader**：`StrokePath` 末尾 `fastStroke → fillPathUser`（`gpu_render_context.go:2413-2423`）从不查 `paint.Shader`，`isGPUSolidPaint`（`sdf_gpu.go:439-453`）也不查 → `SetRGB+SetShader` 的 AA 无虚线描边**画成环境实心色**，与 `Context.SetShader` 文档「fills **and strokes**」（`shader_effect.go:422-424`）矛盾。全仓无 stroke+shader 测试。→ 这是引擎洞，本项要么修要么显式改文档口径。
3. **录制面无法表达 shader**：`FillPathCommand`（`render/recording/command.go:299-310`）只有 Path/Brush/Rule/Blend/BlendSpace；`render/recording/` 对 `Shader` **零命中**（L2 深审时登记过该欠账，实测仍在）。
4. **逐次绘制一次 GPU 往返**：`user_shader_fill.go:266-267` 建 per-call scratch RT → `device.WaitIdle()` → `readTextureViewRegionRGBA`；scratch 未池化（`:174-175` 注释自认「pooling into res.Cache is a registered follow-up」）；面积上限 `maxBrushFillPixels = 8<<20`（`brush_fill.go:24`）。多材质逐帧 2.5D 场景直接放大此项。
5. **uniform 只有一个 256B 槽**：`user_shader.go:245-246` `layoutSize = max(256, roundup(len(uniforms),256))` 而契约上限也是 256 → 传不进 4×4 矩阵 + 多纹理材质（一个 mat4 = 64B，尚可；两张 LUT 就不行，binding 槽位被封死）。
6. **材质三处并存**（收敛口径要拍）：`render.ShaderEffect`（GPU WGSL）、`render.CustomBrush`+`ColorFunc`（CPU 逐像素回调，`render/brush_custom.go:17,34,62`）、`engine/fx.Custom`（CPU 图后处理 Outline/Dissolve，`engine/fx/custom.go:44,68,76`，**零生产消费者**）。三者不是同一能力的重复副本，但「用户自定义外观」这件事有三个入口，P3.7 要写清各自归属。

---

## 4 HDR / 泛光：**render 层代码量为零；地基只有 BlendSpace；引擎侧有一份 CPU 近似**

| 面 | 实测 |
|---|---|
| `render/hdr` FloatTarget | `render/hdr/float_target.go`：CPU `[]float32` 存储，`TextureView()`/`Pixels()` **显式返回 nil**（`:74,77`），色调映射是 CPU 逐像素 Reinhard（`:129-158`），`MaxEncodeExposure=16`（`:150`）。**全仓零 importer**（`grep -rn "gpui/render/hdr" --include=*.go` 非文档命中为空） |
| render 目标格式 | `GPURenderTarget`（`render/accelerator.go:71-99`）**没有 Format 字段**，只有 `Data/Stride/View`；呈现固定 `BGRA8Unorm`（`render/present_construct.go:63`）；`internal/gpu` 全部离屏 RT 只出现 `BGRA8Unorm`/`RGBA8Unorm`（实测 30+ 处：convex_renderer、depth_clip、dual_tex_blend、filter_gpu_graph、user_shader_fill） |
| P3.1 交付的地基 | `render.BlendSpace{SRGB,Linear}`（`render/blendmode.go:47-59`）+ Canvas 口（`canvas.go:105-109`）——**是线性混合，不是 float 面**（台账 §2.1 P3.1 行 D5 明文把 hdr GPU 面移出第一版挂 HDR 专项） |
| HAL/后端能力（地基在） | `hal.Adapter.TextureFormatCapabilities`（`gpu/hal/api.go:104-105`，`descriptor.go:101-131` 含 `RenderAttachment/Blendable/Multisample/Storage`）；gles 扩展门控 `GL_EXT_color_buffer_half_float` 等（`gpu/gwgpu/gles/capabilities.go:728-735`→`:800-802`），内部格式 `gl.RGBA16F`（`gles/device.go:996-997`）；metal `MTLPixelFormatRGBA16Float`（`gwgpu/metal/conv.go:84-85`）；browser `"rgba16float"`（`webgpu/internal/browser/convert_enums.go:70`）；VRAM 8B/texel（`gpu/hal/vram_ledger.go:71`）；`RG11B10Ufloat`/`RGB10A2Unorm` 也在可用表（`gpu/types/texture.go:586-587`） |
| 引擎侧既有近似 | `engine/fx`：`Bloom{Threshold,Radius,Intensity}`（`bloom.go:175-179`）全程 `clamp01`（`:229-232`）＝**LDR 无 >1 亮部**；`Tonemap{None,Reinhard,ACES,Filmic}` + `DefaultTonemap=Reinhard`（`lut.go:155-171`）、`ApplyTonemap(src,t,exposure)`（`:259`）；消费者仅 `examples/engine/fx`、`examples/engine/stage_night/doc.go`，`render/`、`ui/` 零 import |
| 数据类型 | `core.Color` 文档口径「components in [0,1]」（`engine/core/color.go:17`），`RGBA()` 构造不夹（`:29`）、`Clamped()` 显式（`:54`）→ 名义 LDR，>1 无处承载于「HDR 值」语义 |

**判定**：真 HDR（float RT + 曝光 + 色调映射 pass + present 端输出）在 render 层是**从零起**的工程，且要动 `GPURenderTarget` 契约与呈现链；P3.1 只是它的地基不是它的一半。RGBA8 上的泛光近似（提亮→模糊→叠加，即 P3.4 滤镜链 + engine/fx 曲线的 GPU 化）是另一个量级的东西。这一条必须拍范围。

---

## 5 mip 档位接线（P3.6 批 1d 明文归本项）：断点在第一跳

- 定义：`engine/tex.Filter.Mip MipMode`（`engine/tex/mipmap.go:107`）、`SamplerParams()`（`:204`，`:209` 返回 `n.Mip == MipLinear`，登记行号 207-214 **相符**）。
- 别名链已齐（P3.6 批 2 交付）：`tex.MipMode = render.MipmapMode`（`mipmap.go:68`）→ `render/context_image.go:27` → 真身 `render/internal/image/mipmap.go:28`。
- GPU 侧也已就绪：`EffectiveImageSamplerKey(nearest,bicubic,mip,aniso)`（`image_pipeline.go:1090`）、`SetPerImageFilter(mip,maxAniso)`（`:1110`）、`QueueImageDraw*` 均收 `render.MipmapMode`。
- **断点实测清楚：`Filter.SamplerParams` 生产消费者 = 0**（命中仅定义处、`doc.go:53`、`mipmap_test.go` ×6、`examples/engine/tex/main.go:359,378` 自检探针）。CPU 侧 `LevelForScale`（`mipmap.go:230`）同样零调用者。**断点精确化见 §1.6**：另有 `engine/sprite/atlas.go:217 ToRender()` 不填 `Mipmap`、`tex.Filter` 与 `sprite.AtlasFilter` 之间无映射两处。
- **另一处硬缺口**：`engine/renderconv/conv.go` 只有 Vec2/Color/Mat2D 六组转换（`:19-40`），**没有任何 Filter/Sampler 转换函数**——而 `engine/camera/project.go:20-21` 自陈「render types are converted once at the render boundary in engine/renderconv」。**与登记不符**：转换层承诺了但没这块，tex.Filter 到 render 命令这一跳根本没桥。

---

## 6 分层纪律判定（本项按 AGENTS「render 只做通用难事」划线）

| 能力 | 归位 | 依据 |
|---|---|---|
| 透视校正插值、4×4 投影数学、多 quad 透视合批、权重随顶点走 | **render** | 任意图形可复用（Skia 透视贴图同款） |
| 相机/焦点/世界中心、按深度缩放精灵、y+x+z 排序键、落地影、世界高度语义、精灵 z 序策略 | **engine（2.5D 模块库）** | 只服务 2.5D，与组件库平级；`engine/doc.go:15` 2.5D-IF-1.0 已冻能力清单 |
| 材质（用户 shader 随精灵变换 + 透视走） | **render 提供入口，engine 决定语义** | render 侧缺的是「图像/批绘入口能挂 shader」；「哪个精灵用哪个材质」归 engine |
| HDR/泛光 | render（float 面/tonemap 是通用图形）+ engine（曲线选择与曝光语义） | PLAN §5.3 归属列 |
| 示例层 | 只做验证，禁止绕引擎洞 | AGENTS 硬规则；本轮实测 `examples/engine/camera` 丢弃 scale、`examples/engine/tex` 只拿 SamplerParams 当探针，都是**待搬走**的绕路形态 |

---

## 7 待拍板项（逐条用 question 停）

- **D1 批次切法与范围**：本项四项能力（透视相机口 / 世界高度与 z 序 / 自定义材质 / HDR 泛光）是否一次全做、分几批、每批独立提交。
- **D2 透视绘制口形态**：批绘腿要不要吃投影（`AtlasSprite`/`DepthSprite` 加四角+权重或加 `Matrix4`），以及透视合批是否本轮放行（实测代价：1000 张不合规 = 1000 draw call + 250KB slab/帧）。
- **D3 世界高度与遮挡方式**：画家算法（引擎排序进绘制序）vs z-test（实测与 GPU-CLIP-003a 抢同一张 Depth24+Stencil8 附件）；以及 `render/depth_r5.go` 与 `engine/sprite/depth.go` 两份重复实现怎么归位（能力面随迁、不裁 API）。
- **D4 材质口第一版范围**：是否给图像/批绘入口挂 shader（`Paint.Shader` 现只走 `FillPath`）、256B 单 uniform 槽与 binding 0/1/2 上限是否本轮扩、描边快路洞（§3.2 第 2 条）修还是改文档。
- **D5 HDR/泛光做多大**：真 float RT 全链（从零）vs RGBA8 泛光近似（复用 P3.4 滤镜链 + engine/fx 曲线）vs 推 P4/另立专项。
- **D6 mip 档位接线落点**：`engine/renderconv` 补 Filter/Sampler 转换 + `engine/tex` 出口把档位传进 render 命令，是否作为独立小批并入本项。
- **D7 真窗拆几扇**：按「每个主能力独立真窗、禁止组合窗代替」+「满窗即内容」+「内容往厚里做」，透视/z 序/材质/HDR 各一扇还是并入既有 `examples/engine/{camera,sprite,stage_chase,25d_basis}`。

### 本轮**不做**（登记不顺手）

M2（rrect4 gles 量化分叉）、M3（WGSL 非法 intrinsic，与 §3.1「后端级拒绝落首绘」同族）、M4（render 整包 SIGABRT，触整包跑测按 per-file 口径判）、K2–K5、D5 富文本迁移、动图差量上传。
