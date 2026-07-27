# 06 · 图像 / 解码 / Atlas

> **状态（组级）：** A/B/C · **Wave：** — / P1–P2 · 修订：2026-07-28  
> 拆自 `ENGINE_UI_RENDER_BASE`；状态码见 [00_meta](./00_meta.md)  
> **施工真源分册** — 改状态先改本文件，再同步 [README](./README.md)

## 1. 目标 / 非目标

**目标：** 异步解码不堵 UI；RenderImage 状态机；多种 Draw 变体。  
**非目标：** GIF 多帧产品化；TextureLayer 视频；P6。  
**已落地：** `RenderImage`；`ui/io.Pool` DecodeFile；Image 轴：sync/async、Ex/SrcRect/Rounded/Circular/Nine、paint pulse、metrics 门禁。

## 2. 依赖（交叉关联）

| 方向 | 分册 / 说明 |
|------|-------------|
| **前置** | [02_paint_context](./02_paint_context.md) · [01_pipeline_dirty](./01_pipeline_dirty.md) |
| **同级可并行** | 见 [91_wave_order](./91_wave_order.md) |
| **解锁** | 头像/图标/列表图 · [12](./12_filter_shadow.md) 图滤镜 |

## 3. 能力表（母表全文）

## §7 图像 / Atlas / 纹理母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FImg-DRAW | 绘 Image | drawImage | 位图 | DrawImageBuf | DrawImage | RenderImage | A | image.go | — | — |
| FImg-RECT | 源矩形采样 | drawImageRect | 精灵/裁剪 | 部分 via Ex | DrawImageEx | — | A/B | context_image.go | — | P1 |
| FImg-NINE | 九宫格 | drawImageNine | 气泡边框 | — | DrawImageNine | — | B | nine_patch.go | — | P1 |
| FImg-ROUND | 圆角图 | ClipRRect+image | 头像 | — | DrawImageRounded | — | B | context_image.go | — | P0 |
| FImg-CIRCLE | 圆形图 | DecorationImage | 头像 | — | DrawImageCircular | — | B | context_image.go | — | P0 |
| FImg-QUAD | 四角映射 | 自定义 | 透视广告 | — | DrawImageQuad | — | B | m4_extensions.go | — | P2 |
| FImg-CODEC | 解码 | instantiateImageCodec | 加载图 | ui/io.Pool DecodeFile | ImageBuf | RenderImage 状态机 | A | ui/io/decode.go | F12；多帧 GIF 弱 | P1 |
| FImg-DISPOSE | 释放图像 | image.dispose | 防泄漏 | — | ImageBuf 生命周期 | — | C | — | 需规范 Release | P1 |
| FImg-TO-BYTES | 读回像素 | toByteData | 截图/测试 | — | ExportImageBuf/Image/SavePNG | — | B | context_image.go | — | P2 |
| FImg-GPU-TEX | 外部纹理 | Texture | 视频帧 | — | DrawGPUTexture* | 无 TextureLayer | B/D | context_image.go | FL-TEXTURE 关联 | P2 |
| FImg-ATLAS | 图集 | drawAtlas | 合批图标 | — | DrawAtlas | — | B | vertices.go | — | P2 |

---



## 4. 代码落点

| 层 | 路径 |
|----|------|
| RO | `ui/rendering/image.go` |
| IO | `ui/io/decode.go` Pool |
| render | `DrawImage`/`Ex`/`Nine`/`Rounded`/`Circular` · ImageBuf |
| 窗测 | `examples/ui_render_base_image` ✅ |
| 测 | `ui/io/decode_test.go` · render image 测 |

## 5. 指标挂钩

| 指标 | 通过方向 |
|------|----------|
| M-IMG-DECODE-MS | worker，不进 UI 线程 |
| M-LAYOUT-COUNT | SetImage **paint-only** |
| M-RSS-* | 换图/关窗观察 |
| async 回调 | Image 轴 ≥1（RUN≥2s） |

完整定义 → [90_metrics](./90_metrics.md)。**无 baseline 的优化无效**（[00_meta](./00_meta.md)）。

## 6. 验收 DoD

- [x] Image 轴窗测 + layout 门禁 + async 回调  
- [x] FImg-DRAW/CODEC 路径  
- [ ] ImageBuf Dispose 规范（C）  
- [ ] Atlas/GPU tex 层（P2）  
- [ ] 解码耗时进 JSON 字段 |

## 7. 风险与非宣称

Dispose/泄漏以 RSS slope（长 soak）为准，短跑 GPU 冷启动会抬 RSS。

## 8. 相关分册

- [总集 README](./README.md) · [实施顺序 91](./91_wave_order.md) · [指标 90](./90_metrics.md) · [F 交叉 92](./92_f_crosswalk.md)
