# 08 · scene.Layer 全量 · Picture

> **状态（组级）：** A/C/D 混 · **Wave：** — / P1 / **P6** · 修订：2026-07-28  
> 拆自 `ENGINE_UI_RENDER_BASE`；状态码见 [00_meta](./00_meta.md)  
> **施工真源分册** — 改状态先改本文件，再同步 [README](./README.md)

## 1. 目标 / 非目标

**目标：** 层类型盘点完整；Boundary/Offset/Opacity/ClipRect/Transform 可用；Picture 诚实为旗标直至 P6。  
**非目标：** 现在宣称 dirty-rect Present 或 GPU 层 RT。  
**已落地：** Container/Offset/Opacity/ClipRect/**ClipRRect**/Picture/Boundary/**Transform**；Overlay band；COW FramePacket；RasterizeDirty **统计**；Transform 接入 layer_build。

## 2. 依赖（交叉关联）

| 方向 | 分册 / 说明 |
|------|-------------|
| **前置** | [01_pipeline_dirty](./01_pipeline_dirty.md) |
| **同级可并行** | 见 [91_wave_order](./91_wave_order.md) |
| **解锁** | [05_transform](./05_transform.md) · P6 显示列表 · [12](./12_filter_shadow.md) 滤镜层 |

## 3. 能力表（母表全文）

## §12 Layer 类型全量母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FL-CONTAINER | 容器层 | ContainerLayer | 树节点 | — | — | ContainerLayer | A | scene/layer.go | — | — |
| FL-OFFSET | 位移层 | OffsetLayer | 滚动 | — | — | OffsetLayer | A | layer.go | — | — |
| FL-TRANSFORM | 变换层 | TransformLayer | 旋转缩放合成 | RenderTransform | CTM | **TransformLayer** | A/C | scene/layer.go · transform.go | 命中 AABB；Present 全画 | P1 |
| FL-OPACITY | 透明层 | OpacityLayer | 淡入 | MutSetOpacity | — | OpacityLayer | A/C | compositing.go | 真合成未接到 Present | P1 |
| FL-CLIP-RECT | 裁剪层 | ClipRectLayer | 溢出 | — | — | ClipRectLayer | A | layer.go | Build 使用有限 | — |
| FL-CLIP-RRECT | 圆角裁剪层 | ClipRRectLayer | 卡片 | — | — | **ClipRRectLayer** | A | layer.go · build.go | PushClipRRect；Present 仍全画 | — |
| FL-CLIP-PATH | 路径裁剪层 | ClipPathLayer | 异形 | — | — | **无** | D | — | — | P2 |
| FL-PICTURE | 图层层 | PictureLayer | 录制内容 | — | — | PictureLayer+NeedsRaster | C | picture.go | 无 op 缓冲 | P6 |
| FL-TEXTURE | 纹理层 | TextureLayer | 视频 | — | DrawGPUTexture | **无** | D | — | — | P2 |
| FL-PLATFORM-VIEW | 平台视图 | PlatformViewLayer | WebView/地图 | — | — | **无** | D | — | — | 远期 |
| FL-SHADER-MASK | 着色遮罩 | ShaderMaskLayer | 渐变淡出列表 | — | Mask 近似 | **无** | D | — | — | P2 |
| FL-BACKDROP | 背景滤镜层 | BackdropFilterLayer | 毛玻璃 | — | PushBackdropLayer | **无** | D | — | — | P2 |
| FL-COLOR-FILTER | 颜色滤镜层 | ColorFilterLayer | 子树置灰 | — | ApplyColorMatrix | **无** | D | — | — | P2 |
| FL-IMAGE-FILTER | 图像滤镜层 | ImageFilterLayer | 模糊子树 | — | ApplyBlur | **无** | D | — | — | P2 |
| FL-LEADER | LeaderLayer | LeaderLayer | 跟随定位锚点 | — | — | **无** | D | — | Overlay 高级 | P2 |
| FL-FOLLOWER | FollowerLayer | FollowerLayer | Tooltip 跟随 | — | — | **无** | D | — | — | P2 |
| FL-ANNOTATED | 注解层 | AnnotatedRegionLayer | 语义/系统 UI | semantics 薄 | — | — | C | ui/semantics | 非 Layer 实现 | P2 |
| FL-PERF | 性能叠加层 | PerformanceOverlayLayer | HUD | — | — | **无** | D | — | P6 HUD | P6 |
| FL-BOUNDARY | 重绘边界 | RepaintBoundary→层 | 脏区隔离 | SetRepaintBoundary | — | BoundaryLayer | A | node.go · layer.go | — | — |
| FL-OVERLAY-BAND | 覆盖带 | Overlay | 弹层 | ui/overlay | — | FramePacket.Overlay | A | overlay · scene | F13 | — |

---
## §9 Picture / 录制回放母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FPic-RECORDER | Picture 录制 | PictureRecorder+Canvas | 层缓存 | — | 无完整 recorder 对 UI | Picture Valid 旗 | C/D | ui/scene/picture.go | 显示列表 P6 | P6 |
| FPic-PLAYBACK | 回放 | drawPicture | 静态层复用 | RasterizeDirty 统计复用 | — | NeedsRaster 旗 | C | scene/rasterize.go | 非 GPU 纹理复用证明 | P6 |
| FPic-TO-IMAGE | 栅格化 | Picture.toImage | 截图 | — | Export/Image | — | B | context | — | P2 |
| FPic-DISPOSE | 释放 | dispose | 防泄漏 | — | — | — | C | — | 规范待补 | P1 |

---



## 4. 代码落点

| 层 | 路径 |
|----|------|
| scene | `layer.go` · `build.go` · `picture.go` · `rasterize.go` · `packet.go` · `compositing.go` |
| build from RO | `rendering/layer_build.go` |
| overlay | `ui/overlay` · `FramePacket.Overlay` |
| 现状 | 母表 §18.2 已并入上方 §12 更新 |

## 5. 指标挂钩

| 指标 | 说明 |
|------|------|
| M-RASTER-LAYER / SKIP | RasterizeDirty |
| M-COMPOSITE-LAYER | opacity/transform mut（占位加深） |
| M-DAMAGE-AREA / UPLOAD | **仅 P6** |

完整定义 → [90_metrics](./90_metrics.md)。**无 baseline 的优化无效**（[00_meta](./00_meta.md)）。

## 6. 验收 DoD

- [x] TransformLayer 类型+测+build  
- [x] ClassifyDirty 含 MutSetTransform  
- [x] COW ShareRoot  
- [ ] PictureRecorder 显示列表（P6）  
- [ ] dirty-rect Present（P6）  
- [ ] ClipRRect/Path/Filter/Texture **Layer 类型**（D）

## 7. 风险与非宣称

Picture 现为 Valid/NeedsRaster **旗**，不是 op 缓冲。**禁止**用当前 Present 路径宣称层纹理复用。

## 8. 相关分册

- [总集 README](./README.md) · [实施顺序 91](./91_wave_order.md) · [指标 90](./90_metrics.md) · [F 交叉 92](./92_f_crosswalk.md)
