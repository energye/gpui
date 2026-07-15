# P1 — 复杂 UI 场景矩阵门禁

> 版本：1.11 | 日期：2026-07-15  
> 主线：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) / 能力表 [`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md)  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> **非控件层**：场景只模拟 Ant Design 级 UI 的绘制形态。

## 硬规则

1. `WGPU_NATIVE_PATH` 真库  
2. 声称 GPU：`GPUOps > 0` 且无 silent CPU-only  
3. 关键结构像素/区域检查  
4. 性能数字不作为关闭条件  

## Tier A（控件原子形态）

| ID | 场景 | 门禁 | 状态 |
|----|------|------|------|
| A1 | Button states | `TestP1_A1_UIButtonStates` | ✅ |
| A2 | Input field | `TestP1_A2_UIInputField` | ✅ |
| A3 | Menu overlay | `TestP1_A3_UIMenuOverlay` | ✅ |
| A4 | Modal mask | `TestP1_A4_UIModalMask` | ✅ |
| A5 | Table cells | `TestP1_A5_UITableCells` | ✅ |
| A6 | Tabs / badge / tag | `TestP1_A6_UITabsBadge` | ✅ |
| A7 | Icon + mixed text | `TestP1_A7_UIIconTextMix` | ✅ |
| A8 | Scroll nested clip | `TestP1_A8_UIScrollClip` | ✅ |

## Tier B（压力正确性）

| ID | 场景 | 门禁 | 状态 |
|----|------|------|------|
| B1 | Many rrects | `TestP1_B1_ManyRRectsCorrectness` | ✅ |
| B2 | Text atlas stress | `TestP1_B2_StressTextAtlas` | ✅ |
| B3 | Image gallery density | `TestP1_B3_StressImageGallery` | ✅ |
| B4 | Blend stack (Copy/Plus) | `TestP1_B4_StressBlendStack` | ✅ |
| B5 | Path AA stress | `TestP1_B5_StressPathAA` | ✅ |
| B6 | HiDPI stress | `TestP1_B6_StressHiDPI` | ✅ |

## Tier C（更深嵌套密度）

| ID | 场景 | 门禁 | 状态 |
|----|------|------|------|
| C1 | Nested modal + form + menu | `TestP1_C1_NestedModalFormMenu` | ✅ |
| C2 | Table scroll + FAB overlay | `TestP1_C2_TableScrollOverlayDensity` | ✅ |

## Tier D（Ant 布局密度）

| ID | 场景 | 门禁 | 状态 |
|----|------|------|------|
| D1 | Drawer + tree + select | `TestP1_D1_DrawerTreeSelectDensity` | ✅ |
| D2 | Tabs + badge + popconfirm | `TestP1_D2_TabsBadgePopconfirmDensity` | ✅ |

## Tier E（Ant 面板密度）

| ID | 场景 | 门禁 | 状态 |
|----|------|------|------|
| E1 | DatePicker 面板 | `TestP1_E1_DatePickerPanelDensity` | ✅ |
| E2 | Transfer 双列表 | `TestP1_E2_TransferListDensity` | ✅ |

## Tier F（Cascader / 虚拟列表）

| ID | 场景 | 门禁 | 状态 |
|----|------|------|------|
| F1 | Cascader 多列 | `TestP1_F1_CascaderPanelDensity` | ✅ |
| F2 | Virtual list + sticky | `TestP1_F2_VirtualListDensity` | ✅ |

## Tier G（TreeSelect / Carousel 密度）

| ID | 场景 | 门禁 | 状态 |
|----|------|------|------|
| G1 | TreeSelect 面板 | `TestP1_G1_TreeSelectPanelDensity` | ✅ |
| G2 | Carousel 舞台 | `TestP1_G2_CarouselStageDensity` | ✅ |

## Tier H（大虚拟窗口 / Transfer 重密度）

| ID | 场景 | 门禁 | 状态 |
|----|------|------|------|
| H1 | Large virtual list window | `TestP1_H1_LargeVirtualListWindow` | ✅ |
| H2 | Transfer dual-list heavy | `TestP1_H2_TransferDualListHeavy` | ✅ |

## Tier I（Dashboard / Modal 栈密度）

| ID | 场景 | 门禁 | 状态 |
|----|------|------|------|
| I1 | Dashboard shell | `TestP1_I1_DashboardShellDensity` | ✅ |
| I2 | Modal stack + popconfirm | `TestP1_I2_ModalStackDensity` | ✅ |

## 能力表同步收口

| ID | 能力 | 门禁 |
|----|------|------|
| S.03 | 窗口 present | `window_present` / X11 tags |
| S.05 | Resize + GPU 重绘 | `TestP1_Capability_S05_ResizeGPU` |
| S.08 | HiDPI hairline | `TestP1_Capability_S08_HiDPIHairline` |
| B.02 | PD fixed GPU (full set incl. DstOver/SrcIn/…) | `TestP12GPUFixedPixel_Blend*` |
| B.03 | Multiply/Screen GPU path | `TestP1_Capability_B03_*` |
| B.06 | Paint alpha | `TestP1_Capability_B06_PaintAlpha` |
| B.07 | Plus GPU | `TestP12GPUFixedPixel_BlendPlus` |
| D.03 | Sweep gradient GPU | `TestP1_Capability_D03_SweepGradientGPU` |
| D.04 | Multi-stop + Repeat/Reflect | `TestP1_Capability_D04_*` |
| D.05 | ImagePattern GPU | `TestP1_Capability_D05_*` |
| D.06 | Pattern local matrix | `TestP1_Capability_D06_*` |
| G.06 | RRect XY radii | `TestP1_Capability_G06_RRectXYRadiiGPU` |
| L.06 | R8 mask GPU modulate | `TestP1_Capability_L06_*` |
| P.05/P.06 | Stroke cap/join pixels | `TestP1_Capability_P05_*` / `P06_*` |
| B.05 | Premul pipeline | `TestP1_Capability_B05_PremulPipelineGPU` |
| X.05 | LCD two-pass | `TestP1_Capability_X05_*` |
| B.03 | dual-tex Multiply/Screen/Overlay | `TestP1_Capability_B03_*` |
| B.05 | premul solid/image/layer/text | `TestP1_Capability_B05_*` |
| Q.04 | premul AA edges | `TestP1_Capability_Q04_*` |
| H.03 | EvenOdd vs NonZero | `TestP1_Capability_H03_*` |
| P.04 | Hairline | `TestP1_Capability_P04_*` |
| F.03 | Image filter graph ping-pong | `TestP1_Capability_F03_*` |
| L.06 | MaskAware native R8 upload | `TestP1_Capability_L06_MaskAware*` |
| L.06 | Convex cover-inline R8 | `TestP1_Capability_L06_CoverInlineR8GPU` |
| L.06 | SDF cover-inline R8 | `TestP1_Capability_L06_SDFCoverInlineR8GPU` |

## 命令

```bash
export WGPU_NATIVE_PATH=/path/to/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache

go test -count=1 ./render -run 'TestP1_'
go test -count=1 ./render -run 'TestS3c_|TestS3b_|TestS3a_|TestP12GPUFixedPixel|TestP1_'
```

## 仍 open（下一切片）

- F.03 GPU multi-RT ping-pong（pixmap multi-pass on GPU pixels ✅；true multi-RT 可再加深）  
- 更极端 Ant 数据集 / 滚动 damage 加压  
- M4 后置质量项  
- L.06 stencil-then-cover 路径 mask 内联（convex/SDF 已 ✅）  
