# P1 — 复杂 UI 场景矩阵门禁

> 版本：1.12 | 日期：2026-07-15  
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
| L.06 | Stencil cover-inline R8 | `TestP1_Capability_L06_StencilCoverInlineR8GPU` |
| F.03 | GPU multi-RT filter graph | `TestP1_Capability_F03_GPUMultiRTFilterGraph` |
| F.03 | GPU ColorMatrix+DropShadow | `TestP1_Capability_F03_GPUColorMatrixDropShadow` |
| B.04 | HSL Hue/Color GPU dual-tex | `TestP1_Capability_B04_*` |
| S.07 | WritePixels GPU upload | `TestP1_Capability_S07_WritePixelsGPU` |
| B.03 | Darken/Difference/Lighten GPU | `TestP1_Capability_B03_DarkenGPU` / `Difference` / `Lighten` |
| B.03 | SoftLight/HardLight/ColorDodge GPU | `TestP1_Capability_B03_SoftLightGPU` / `HardLight` / `ColorDodge` |

## 命令

```bash
export WGPU_NATIVE_PATH=/path/to/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache

go test -count=1 ./render -run 'TestP1_'
go test -count=1 ./render -run 'TestS3c_|TestS3b_|TestS3a_|TestP12GPUFixedPixel|TestP1_'
```

## Tier J（通知栈 / 双抽屉叠层）

| ID | 场景 | 测试 | 状态 |
|----|------|------|------|
| J1 | Notification stack | `TestP1_J1_NotificationStackDensity` | ✅ |
| J2 | Dual drawer + FAB | `TestP1_J2_DualDrawerOverlayDensity` | ✅ |

## Tier K（damage / HiDPI 密度）

| ID | 场景 | 测试 | 状态 |
|----|------|------|------|
| K1 | Multi-region damage UI | `TestP1_K1_DamageMultiRegionUI` | ✅ |
| K2 | HiDPI toolbar+table+overlay | `TestP1_K2_HiDPIToolbarTableOverlay` | ✅ |

## Tier L（表单校验 / 表格选择+Toast）

| ID | 场景 | 测试 | 状态 |
|----|------|------|------|
| L1 | Form validation dense | `TestP1_L1_FormValidationDense` | ✅ |
| L2 | Table selection + toasts | `TestP1_L2_TableSelectionToasts` | ✅ |

## Tier M（图表/热力 advanced blend）

| ID | 场景 | 测试 | 状态 |
|----|------|------|------|
| M1 | Chart dashboard + Darken/Difference | `TestP1_M1_ChartDashboardBlend` | ✅ |
| M2 | Heatmap SoftLight density | `TestP1_M2_HeatmapSoftLightDensity` | ✅ |

## Tier N（retained 多面板 / IDE 密度）

| ID | 场景 | 测试 | 状态 |
|----|------|------|------|
| N1 | Retained multi-panel + WritePixels | `TestP1_N1_RetainedMultiPanelDamage` | ✅ |
| N2 | IDE layout density | `TestP1_N2_IDELayoutDensity` | ✅ |

## Tier O（日历时间线 / 甘特依赖）

| ID | 场景 | 测试 | 状态 |
|----|------|------|------|
| O1 | Calendar + timeline density | `TestP1_O1_CalendarTimelineDensity` | ✅ |
| O2 | Gantt dependency density | `TestP1_O2_GanttDependencyDensity` | ✅ |

## Tier P（高级混合 / compute 形态）

| ID | 场景 | 测试 | 状态 |
|----|------|------|------|
| P1 | Advanced blend composite density | `TestP1_P1_AdvancedBlendCompositeDensity` | ✅ |
| P2 | Compute path + UI chrome density | `TestP1_P2_ComputePathUIChromeDensity` | ✅ |

## 能力门禁补充（本轮）

| ID | 能力 | 门禁 |
|----|------|------|
| K.01 | Vello compute path | `TestP1_Capability_K01_VelloComputePathGPU` |
| Q.02 | Coverage AA GPU | `TestP1_Capability_Q02_CoverageAAGPU` |
| B.03 | ColorBurn/Exclusion GPU | `TestP1_Capability_B03_ColorBurnExclusionGPU` |
| V.03 | DrawMesh indexed GPU | `TestP1_Capability_V03_DrawMeshIndexedGPU` |
| K.02 | DrawIndirect GPU | `TestP1_Capability_K02_DrawIndirectGPU` |
| CS.02 | RGBA16Float surface | `TestP1_Capability_CS02_RGBA16FloatSurfaceGPU` |

## 窗口 Present multi-rect damage

| ID | 场景 | 测试 | 状态 |
|----|------|------|------|
| S.03 multi-rect | X11 PresentFrame + PresentFrameDamageRects | `TestS3c_M3_WindowPresentFrame_X11Draw` (`-tags gpui_x11_present`) | ✅ |

## Tier Q（multi-viewport / retained damage）

| ID | 场景 | 测试 | 状态 |
|----|------|------|------|
| Q1 | Multi-viewport retained density | `TestP1_Q1_MultiViewportRetainedDensity` | ✅ |
| Q2 | Triple-pane damage density | `TestP1_Q2_TriplePaneDamageDensity` | ✅ |

## Tier R（spreadsheet / pivot heatmap）

| ID | 场景 | 测试 | 状态 |
|----|------|------|------|
| R1 | Spreadsheet grid density | `TestP1_R1_SpreadsheetGridDensity` | ✅ |
| R2 | Pivot heatmap density | `TestP1_R2_PivotHeatmapDensity` | ✅ |

## M4 能力门禁（本轮）

| ID | 能力 | 门禁 |
|----|------|------|
| V.03 | DrawMesh indexed | `TestP1_Capability_V03_DrawMeshIndexedGPU` |
| K.02 | DrawIndirect | `TestP1_Capability_K02_DrawIndirectGPU` |
| CS.02 | RGBA16Float RT | `TestP1_Capability_CS02_RGBA16FloatSurfaceGPU` |
| CS.03 | Linear blend mid | `TestP1_Capability_CS03_LinearBlendMidGPU` |

## Tier S（Kanban / Storyboard）

| ID | 场景 | 测试 | 状态 |
|----|------|------|------|
| S1 | Kanban board density | `TestP1_S1_KanbanBoardDensity` | ✅ |
| S2 | Storyboard filmstrip + ImageQuad | `TestP1_S2_StoryboardFilmstripDensity` | ✅ |

## Tier T（Mail / Settings modal）

| ID | 场景 | 测试 | 状态 |
|----|------|------|------|
| T1 | Mail client density | `TestP1_T1_MailClientDensity` | ✅ |
| T2 | Settings modal + backdrop | `TestP1_T2_SettingsModalBackdropDensity` | ✅ |

## M4 门禁补充（本轮）

| ID | 能力 | 门禁 |
|----|------|------|
| E.03 | Path.Trim | `TestP1_Capability_E03_TrimPathGPU` |
| P.09 | SetDither | `TestP1_Capability_P09_DitherGPU` |
| T.04 | DrawImageQuad | `TestP1_Capability_T04_ImageQuadGPU` |
| L.05 | PushBackdropLayer | `TestP1_Capability_L05_BackdropLayerGPU` |

## Tier U（Vector design / Media gallery）

| ID | 场景 | 测试 | 状态 |
|----|------|------|------|
| U1 | Vector design tool density | `TestP1_U1_VectorDesignToolDensity` | ✅ |
| U2 | Media gallery + external tiles | `TestP1_U2_MediaGalleryExternalTiles` | ✅ |

## M4 门禁补充（本轮）

| ID | 能力 | 门禁 |
|----|------|------|
| E.02 | Corner/Discrete path effects | `TestP1_Capability_E02_PathEffectsGPU` |
| I.08 | External GPU texture composite | `TestP1_Capability_I08_ExternalTextureGPU` |

## 仍 open / 后续

- **阶段 A（当前主线焦点）**：任意组合维度 → [`P1_COMPOSITION_MATRIX.md`](./P1_COMPOSITION_MATRIX.md)  
- A 收口后：**S4.0 性能基线**（见主线 S4）  
- 旁路：R.02 PDF/SVG document；真 multiplanar YUV 全量  
- 本文件 Tier A–U 保留为 **形态密度回归**；新增场景优先写组合维度 ID，不绑控件产品名  





