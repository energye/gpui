# M5 P1：管线颜色目标格式参数化（2026-11-11）

> 上游：evidence/M5/assessment-design.md（D2 拍板=全格式参数化，8bit 默认行为零变化）。
> 本批只做「格式成为管线参数」，不接 float 离屏 RT、不改呈现、不改公开 API。

## 0 结论

12 个会话颜色目标管线文件、43 处 ColorTargetState 的硬编码 BGRA8Unorm 改为
colorFormatOr(<owner>.colorFormat)。owner 的 colorFormat 零值（TextureFormatUndefined）
解析为历史 BGRA8Unorm，所以 8bit 路径逐字节零变化；float 目标只需在创建管线前把该字段
设成 RGBA16Float。新增 color_format.go 作为唯一口径，新增单测证明 SDF/图像/文字三类管线
能按 RGBA16Float 建出来、零值仍按 BGRA8 建出来。

## 1 机制（与 Skia / Impeller 对齐）

- Skia：program/pipeline 键含目标色彩类型与格式（include/core/SkColorType.h:98
  kRGBA_F16_SkColorType 为一等色彩类型）。
- Impeller：pipeline descriptor 含 color attachment format；PixelFormat::kR16G16B16A16Float
  （impeller/core/formats.h:109,312,665）。
- 本仓：render/internal/gpu/color_format.go
  - defaultColorFormat = types.TextureFormatBGRA8Unorm
  - colorFormatOr(f)：f==Undefined ⇒ 历史 BGRA8，否则原样。
- 每个管线 owner（ConvexRenderer / DepthClipPipeline / GlyphMaskPipeline /
  TexturedQuadPipeline / SDFRenderPipeline / StencilRenderer / MSDFTextPipeline /
  GPURenderSession / GPURenderContext）新增 colorFormat 字段，创建 ColorTargetState 时
  统一走 colorFormatOr。

## 2 改动清单（43 处描述符）

| 文件 | 目标管线数 | 说明 |
|---|---|---|
| image_pipeline.go | 9 | textured quad 全家族（stencil/depth-clip/bicubic/persp/blend/blit） |
| stencil_pipeline.go | 7 | stencil fill/cover/AA band/depth-clip |
| convex_renderer.go | 6 | fill/stroke/mesh/blend 变体 |
| glyph_mask_pipeline.go | 4 | AA/彩色/LCD/depth-clip |
| sdf_render.go | 3 | 默认/stencil/depth-clip |
| depth_clip.go | 3 | cover/fill（WriteMaskNone 也必须匹配附件格式） |
| text_pipeline.go | 3 | MSDF 默认/stencil/depth-clip |
| video_yuv_pipeline.go | 3 | YUV stencil/depth-clip/blit |
| session_textured_cover.go | 2 | 纹理/图案 cover |
| stencil_renderer.go | 1 | cover blend |
| material_pipeline.go | 1 | 会话材质管线 |
| user_shader.go | 1 | 用户着色器管线 |
| **合计** | **43** | 12 文件 |

字段 owner：9 个结构体（见 §1）。

## 3 有意不动的面（附理由）

- dual_tex_blend.go：RGBA8/BGRA8 两条固定管线按目标纹理格式二选一，且调用点无条件用
  pipelineBGRA；要做 float 需改成 per-format map，随 P2/P4 的 float 图层路径一起做。
- textured_stencil_linear.go / textured_stencil_pattern.go：管线写的是自建
  tex_stencil_*_result 中间纹理（固定 BGRA8），不写会话目标；float 渐变图层要连中间纹理
  一起参数化，归 P2。
- filter_gpu_graph / linear_ramp_mask / mask_r8_modulate / pattern_mask_sample：内部固定
  RGBA8/R8 中间面，不是会话颜色目标，保持原样（float 滤镜链后续另立）。

## 4 验证

### 4.1 P1 参数化钉（render/internal/gpu/color_format_test.go）

- colorFormatOr(Undefined) == BGRA8、colorFormatOr(RGBA16Float) == RGBA16Float。
- SDF 管线（createPipeline）、图像管线（ensurePipelineWithStencil）、MSDF 文字管线
  （ensurePipelineWithStencil）在 colorFormat=RGBA16Float 下全部建成、句柄非空。
- 控制腿：零值 SDF 管线仍按 BGRA8 建成。
- 结果：PASS（evidence/M5/p1-unit-tests.txt）。
- 边界：16F 的绘制/混合语义由 P0 底座探针证明（hdr_16f_probe_test.go，half 位型精确）；
  生产管线端到端 16F 绘制在 P4 真窗。GLES 腿在 P4（无 surfaceless 独立设备）。

### 4.2 8bit 行为零变化

- go build ./...、go vet ./render/...、go build/vet -tags nogpu ./render/... 全 rc=0。
- P0.4 一致性台：TestConformanceDualBackend PASS（1.58s）。
- SDF 家族 + P0 + P1 单测：ok（2.5s）。
- 真窗金图（双后端）：p35_rrect4 native 0.0000%/go 0.0000%（logic 6/6）、
  p34_filters 0.0000%（pixel 29/29）、base_pathshape 0.0000%（logic 14/14、pixel 11/11）、
  ui_ant_verify msaa1 0.0000%；每轮单窗自退出 + 显示健康检查（evidence/M5/p1-windows/）。

## 5 风险与下一步

- 字段目前没有 setter：P2 接 float 离屏 RT 时，由会话在创建管线前设置 owner 的
  colorFormat，并在格式变化时销毁重建（pipeline 键含格式）。本批不提前加 setter，
  避免无消费者的抽象。
- P2 还要做：池键含 Format、float 离屏 RT 创建/账本/能力回退、中间纹理（dual-tex /
  textured-stencil result）格式参数化。
- P3：tonemap pass 与 hdr.Encode 同式；P4：端到端真窗。
