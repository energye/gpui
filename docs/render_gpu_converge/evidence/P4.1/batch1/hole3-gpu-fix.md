# P4.1 批 1 引擎洞 #3：Porter-Duff 层 GPU 复合路径 ✅（2026-11-10）

> 上游评估 `hole3-gpu-assessment.md`。B 卡（mesh × BlendPlus）双后端 FAIL 收口，
> P4.1 四子项全部 ✅。

## 0 现象与根因

- 修前：`pixel B_plus_centroid at (964,251) got=(0,0,255) want=(85,85,255)`（native 与 go 同值）。
- 根因两层：
  1. `render/context_layer_stack.go` 的 `canGPUPorterDuff` 分支走 **CPU 回读**：
     `materializeLayerGPUToPixmap` 读回层 RT → `compositeLayer` 写父 pixmap。
     帧中间写回 pixmap 后，窗口 present 只重放排队尾段（同文件 mask 分支注释记录的
     反模式），层内容进不了呈现帧。
  2. 回读只读 `layer.gpuView`，而层内 mesh 命令还在队列里（无 flush），读回内容为空。
- 对齐标准：Skia `SkCanvas::restore()` 把层图像当纹理、用 paint 的 blend mode 画回父面，
  不读回 CPU；本仓既有同形态模范 = mask 层 `queueMaskedLayerGPU`。

## 1 修法（引擎层，通用）

`PopLayer` 的 Porter-Duff 非 Normal 分支改为**排队 GPU 复合**：

- `drawGPUTextureWithOpacityBlend(layer.gpuView, ..., layer.blendMode)`
  → `QueueGPUTextureDrawBlend`（复用既有 `ensureBlitBlendPipeline` 固定函数族），
  层 RT 的 release 进 `layerGPUReleases`（等 GPU 无在飞工作才放，C5 契约）。
- 复合区按层内容界（`layer.damage` +2px，`fullComposite` 时整层）用 **clip** 约束；
  不用 dst 子矩形——blend 队列无 UV 变体，子矩形会把整张层纹理压进小矩形（本轮实踩，
  就是第一次尝试「看着生效但探针仍纯蓝」的原因）。
- 边界语义与 CPU 的 content-bounded 复合对齐：SrcIn/Clear 等破坏性算子不会擦掉层外的父内容。

顺带的行为面（不动引擎）：`render/vertices_paint_blend_test.go` 的背景从
`ClearWithColor` 改为非 AA 的全幅 Fill——`ClearWithColor` 只写 CPU pixmap
（`context_path.go:33-43`），读 dst 的合成在无窗口 headless 下看不到它；窗口 present
另有 base seeding，所以生产窗不受影响。该 seam 已登记（P4.2 族）。

## 2 验证

| 项 | 结果 |
|---|---|
| 单测 `TestP41LayerPlusGPUComposites` | 绿（层 + Plus 复合 = 底+源） |
| 单测 `TestP41LayerPlusMeshVertices` | 绿（公共 mesh 路径同形） |
| 单测 `TestP41LayerPorterDuffFamily` | 绿（Plus/Xor/SrcIn/DstIn/SrcOut/DstOut/Clear/SrcAtop/DstAtop/DstOver 10 族 vs CPU 参考 ≤1） |
| 真窗 native | pixel 8/8，golden **0.0000%**（356 presents，重种后复验） |
| 真窗 go/GL | pixel 8/8，golden **0.0000%**（348 presents，重种后复验） |
| B 卡实测 | 探针 (964,251) = **(85,85,255)**，与 Skia 契约一致 |
| 负向 `GPUI_P41_BREAK=normal-blend` | B 卡红（got 85,85,85）→ 还原复绿 |
| 邻窗回归 | evenodd_msaa 0.0000%、p34_filters 0.0000%、drawshadow 0.0000%；filterops 0.4216% 与 P4.3 期实测**逐位同**（既有红，未新增） |
| 门禁 | `go build ./...`、`-tags nogpu`、`go vet ./render/` rc=0；`TestVerticesPaint*` 全绿 |
| 金图 | 旧基线归档 `golden-reseed/`（含 sha256），新基线 native+go 各自 0.0000% |

## 3 边界

- gwgpu-metal 不可验（无 macOS 真机）。
- 复合按层内容界，非整幅（Skia 的 saveLayer bounds 语义按本仓 CPU 既有契约取内容界）；
  `fullComposite` 层（backdrop/mask）仍整层。
- blend 队列无 UV 变体：暂不支持「只复合 damage 子矩形且不缩采样」的进一步裁剪，
  性能优化留 P4.3 线（当前一次整层 quad + scissor，成本与 Normal 层同量级）。
