# P4.1 批 1 引擎洞 #3：Porter-Duff 非 Normal 层 GPU 复合路径（评估，2026-11-10）

> 状态：**评估完成，未动代码**。B 卡（`ui_wr_p41_mesh_paint_blend`）双后端一致
> FAIL，是 P4.1 唯一未收口子项（另四子项均已 ✅）。

## 0 现象（可复现）

- 门禁实测（两后端同值）：
  - `evidence/P4.1/batch1/gate-native.txt:11`、`gate-go.txt:12`
    `pixel B_plus_centroid at (964,251) got=(0,0,255) want=(85,85,255)`
  - `B_outside_blue` 通过 ⇒ 蓝底在、mesh 层内容缺席（plus 的 +85,+85,0 没发生）。
- 复现命令：`RUN_SECONDS=6 go run ./examples/ui_wr_p41_mesh_paint_blend -auto-only`
  （native）/ `GPUI_BACKEND=go ...`（go/GL），两后端同 FAIL。
- 同链无窗口探针是绿的：`TestP12GPUFixedPixel_BlendPlus` PASS（fill 命令 + Plus 层）。
  ⇒ 缺口不在 Porter-Duff 数学，而在 **PopLayer 的层复合调度**。

## 1 现状路径（定位到行号）

- `render/context_layer_stack.go:407-410` 判 `canGPUPorterDuff`
  （非 Normal/Copy/Modulate、非高级 blend、层内容只在 GPU RT）。
- `:486-500` 分支实现 = **CPU 回读**：`materializeLayerGPUToPixmap(layer)` →
  `compositeLayer` → 写父 pixmap。注释自陈「GPU texture blend blit 已搭架子
  但未验，启用时丢层内容，先关」。
- `render/context_layer_blend.go:614` `materializeLayerGPUToPixmap` **不 flush**
  就直接 `ReadbackViewRGBA(layer.gpuView)` ⇒ 层 RT 上排队未提交的 mesh 命令
  读不到（空内容），plus 空 = 不变 ⇒ 纯蓝。
- 更根本：该分支在帧中间写回父 pixmap，正是同文件 `:333-341` mask 分支注释记录的
  反模式——「mid-frame materialize 后，present 只重放尾部，之前画的全部进不了呈现帧」。
  mask 分支的解法是 `queueMaskedLayerGPU`（`context_layer_blend.go:724`）：
  GPU 上做 mask 调制 + **把复合本身排进命令流**，present 整帧重放。

## 2 对齐标准

- Skia `SkCanvas::restore()`（saveLayer 收尾）：把层图像当作 `SkImage` 用 paint
  的 blend mode 画回父 surface；GPU 后端即「纹理 quad + 固定函数 blend state」，
  不读回 CPU。Flutter/Impeller 同形（`EntityPass` 收尾为一个带 blend 的 quad）。
- 本仓已有同形态模范：mask 层 `queueMaskedLayerGPU`；Porter-Duff 侧的架子为
  `render/context_image_gpu.go:558-578` `drawGPUTextureWithOpacityBlend`
  （→ `QueueGPUTextureDrawBlend`）+ `render/internal/gpu/image_pipeline.go:775-825`
  `ensureBlitBlendPipeline`（按 BlendMode 缓存 blit 兼容管线）。**两者当前零调用者**。

## 3 候选修法

- **A（推荐，标准形态）**：PopLayer PorterDuff 分支改为排队 GPU 复合——
  `drawGPUTextureWithOpacityBlend(layer.gpuView, ...)` + release 进
  `layerGPUReleases` + `noteParentCompositeDamage`，与 `canGPUComposite`（:425-486）
  同构；CPU 回读分支删除或降为无 GPU 会话兜底。
- B（不推荐）：保留 CPU 回读，补 `flushLayerViewGPU` 再读——即便读对，mid-frame
  写回仍会被 present 丢（mask 分支已实证），等于绕不过去。

## 4 需要先诊断的未知（A 的前置）

1. 上次「启用即丢内容」的具体失败点：`ensureBlitBlendPipeline` 的 blend state 与
   blit pass 的 target 格式/采样是否匹配（`gpuBlendStateForPaint` 对
   Plus/Xor/SrcIn/DstIn/Clear 的映射是否齐全）。
2. 复合 cub 的记录顺序：层内 mesh 命令必须先于复合命令进同一命令流（present 重放
   顺序），现有 `QueueGPUTextureDrawBlend` 是否落在同一 target/同一 pass。
3. damage 传播：`noteParentCompositeDamage` 对非 Normal 层是否同样成立
   （否则 damage-tight present 会裁掉复合）。
诊断手段：头less 先红测（层 + Plus mesh，CPU 对照）+ 命令流计数，再单窗验收。

## 5 影响面

- 触及：`PopLayer` 全部非 Normal/Copy/Modulate、非高级的 Porter-Duff 层
  （Plus/Xor/SrcIn/DstIn/SrcOut/DstOut/Clear/Src/Dst…）；normal 与高级 blend 分支不动。
- 窗口：`ui_wr_p41_mesh_paint_blend` B 卡转绿是主判据；其金图**含 B 卡已知缺口**，
  修好后必须重种（batch1 summary §47-51 已登记）。
- 邻窗回归：`ui_wr_p41_evenodd_msaa` / `ui_wr_p41_drawshadow` / `ui_render_base_filterops`
  / `ui_wr_p34_filters`（C 卡 mask）双后端金图零新增红。
- 跨端：native + go/GL 均验；metal 不可验（无真机，写明）。
