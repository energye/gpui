# P4 接线评估与方案（2026-11-11，停在拍板：新增公开 API）

> 上游：evidence/M5/assessment-design.md §5 P4。P0/P1/P2/P3 已绿；本批要把
> 「16F 离屏面 + tonemap + 8bit 呈现」接进真窗。**本文件只做方案，未动代码。**

## 0 现状与缺口

| 段 | 现状 | 缺口 |
|---|---|---|
| 建 16F 面 | 内部 CreateOffscreenTextureFormat（P2） | 公开口只有 Context.CreateOffscreenTexture（固定 8bit，context_image_gpu.go:620） |
| 画进 16F 面 | FlushGPUWithView（context_gpu.go:293）可写任意 view；会话 surface 模式 sc=1 时颜色附件就是目标视图（gpu_textures.go:133 ensureSurfaceTextures 不带 resolve） | 会话管线仍是共享 8bit；Flush 不看目标格式 |
| tonemap | tonemapPipeline（P3，内部） | 没有公开调用口 |
| 呈现 | DrawGPUTexture（context_image_gpu.go:263）可把 8bit 视图贴回窗口 | 无 |

## 1 建议方案 B（显式 offscreen + tonemap，格式跟随目标视图）

公开 API（沿用现有 gpucontext.TextureView / types.TextureFormat，与
render/device_provider.go 的 SurfaceFormat 同口径）：

- Context.CreateOffscreenTextureFormat(w, h int, format types.TextureFormat)
  (gpucontext.TextureView, func(), error)
- Context.TonemapView(src gpucontext.TextureView, w, h int, exposure float64)
  (gpucontext.TextureView, func(), error)

内部接线（不改默认 8bit 路径）：

1. GPURenderSession 增加 colorFormat（零值 BGRA8）。
2. Flush 读目标视图的纹理格式：format != BGRA8 时把上下文切到 HDR 模式
   （colorFormat=目标格式、preferSampleCount1=true，D5 float 1x），格式变化则重建会话。
3. 会话 owned 管线（preferSampleCount1 路径已自有 SDF/convex/stencil）与每会话的
   image/text/glyph 管线在创建时写 colorFormat（P1 字段已就位）。
4. 目标格式 16F + sc=1 ⇒ 无 resolve 纹理，颜色附件即目标视图（现有路径）。
5. Context.TonemapView：用 P3 tonemapPipeline 把 src 渲到内部 8bit 面，返回视图 + release。

窗口流程（新窗 ui_wr_m5_hdr）：
子 Context 画 HDR 内容（>1 颜色）→ FlushGPUWithView(16F) → TonemapView(8bit) →
DrawGPUTexture 贴回窗口 → 右侧画 CPU 参考（hdr.FloatTarget + Encode）→ 逐像素断言
GPU vs CPU max ≤1 + 双后端金图；JSON 带 OffscreenFormatFallbacks。

## 2 备选

- A：Context.SetHDR(exposure) 全局模式，改默认会话/呈现链（Skia F16 surface 形态）。
  更「标准」，但动默认呈现路径，风险与批次都大，建议后续独立立项。
- C：只做内部探针，不出真窗。违反「每个主能力必须有独立真窗」，不推荐。

## 3 待拍板

- API 形状：B（推荐）/ A / C。
- TonemapView 返回：视图（零回读，窗口再 DrawGPUTexture，推荐）/ 读回 image（简单但每帧回读）。
