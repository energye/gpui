# M5 P4：真窗 HDR 接线（16F 离屏面 → tonemap → 8bit 呈现）

> 上游：evidence/M5/assessment-design.md §5 P4、p4-connection-plan.md（用户拍板方案 B：
> 格式跟随目标视图，不加全局 HDR 开关）。P0/P1/P2/P3 已绿。

## 0 结论

一条真实链路打通：render.Context 把线性 HDR 色（2.0 / 1.6 / 4.0）画进 RGBA16Float
离屏面 → 会话按目标格式 1x 建管线直写该视图 → TonemapView 按「曝光→Reinhard→sRGB」
转 8bit → DrawTextureRef 贴回窗口。新窗 ui_wr_m5_hdr（三档曝光，左 GPU 右 CPU 参考）
双后端 9/9 像素探针 maxdiff=0、金图 0.0000%、offscreen_fmt_fallbacks=0。

## 1 公开 API（方案 B，3 个 Context 方法）

| API | 作用 |
|---|---|
| Context.CreateOffscreenTextureFormat(w,h,format) | 建 BGRA8/RGBA8/RGBA16Float 离屏面；不支持格式具名拒绝、零分配 |
| Context.TonemapView(src,w,h,exposure) | RGBA16Float 视图 → 8bit 池化视图；曲线与 hdr.Encode 同式 |
| Context.OffscreenFormatFallbacks() | float 请求回退 8bit 的计数（诊断） |

不加 SetHDR 开关：标准形态是目标面自己带格式（Skia surface color type / Impeller
PixelFormat），Flush 读目标视图纹理格式后按格式建管线；开关会与真实目标成为两个真源。

## 2 内部接线

- GPURenderContext.Flush：从 target.View 的纹理读格式；16F 时强制 1x（D5）与
  session-owned 管线（不借用共享 8bit 管线）；格式/采样数变化时销毁重建会话。
- GPURenderSession.colorFormat：owned SDF/convex/stencil 与每会话 image/text/glyph
  管线创建时写入（P1 的字段），applyColorFormat 统一盖章。
- surface 模式 sc=1 时颜色附件就是目标视图（gpu_textures.go ensureSurfaceTextures 不带
  resolve），16F 值不经 8bit 中转。
- TonemapView：P2 池化 8bit 目标 + P3 tonemap pass；源格式非 16F 具名拒绝。

## 3 真窗 ui_wr_m5_hdr

- 布局：3 行（曝光 0.5 / 1 / 4），左 GPU、右 CPU 参考；每卡 340×150，三个像素对齐矩形：
  (2.0,0.5,0.25) / (0.25,1.6,3.0) / (0.1,0.5,4.0)。
- GPU 卡：子 Context 画 → CreateOffscreenTextureFormat(16F) → FlushGPUWithView →
  TonemapView(exposure) → RegisterTextureRef → DrawTextureRef。
- CPU 卡：同一场景写入 hdr.FloatTarget → Encode(exposure) → DrawImage。
- 探针：每卡 3 个矩形内部点，GPU vs CPU 逐通道 |d|≤2（实测 maxdiff=0）；曝光 1/4 的
  红通道 >200（证明 2.0 没被 8bit 钳到 1.0 的 ~188）。
- 门禁：header+legend+6 卡逐位金图；hdr_gpu_ops=12、hdr_cpu_fallback_ops=0、
  offscreen_fmt_fallbacks=0。

结果（evidence/M5/p4-windows/verify_native.txt、verify_go.txt）：

| 后端 | 像素探针 | 金图 | presents | 备注 |
|---|---|---|---|---|
| webgpu(native) | 9/9 maxdiff=0 | 0.0000% | 357 | 独立金图 m5_hdr_final.png |
| gwgpu-gles(GPUI_BACKEND=go) | 9/9 maxdiff=0 | 0.0000% | 357 | 独立金图 m5_hdr_final_go.png |

## 4 链路单测（render/internal/gpu/tonemap_chain_test.go）

TestP4HDRDrawTonemapChain：真实 QueueShape 画 (2.0,0.5,0.25) 圆进 16F 面 → Flush →
TonemapView → 读回 vs hdr.Encode：max_diff=0，红通道 213（1.0 钳位约 188）。

## 5 8bit 回归

- build / vet / -tags nogpu build+vet 全 rc=0。
- conformance、S61/S63/S67/P43 上传板、M5 P0–P4 单测全绿（evidence/M5/p4-unit-tests.txt）。
- 真窗：p35_rrect4 native/go、p34_filters、p41_drawshadow native/go、
  ui_render_base_pathshape 金图全 0.0000%。

## 6 边界与后续（P5 收口时登记）

- float 面按 D5 走 1x（MSAA+float 未开）；HDR 直出（10-bit/float swapchain + 色彩空间）
  不在本项。
- float 面进 layer/filter 链、GPU 端合成（不贴回 8bit）尚未接线；窗口用 DrawTextureRef
  贴回现有 8bit 呈现，属设计 D3 的「tonemap→8bit」口径。
- 每卡烘焙一次（gate 静态内容）；连续动画 HDR 的每帧成本未测。
