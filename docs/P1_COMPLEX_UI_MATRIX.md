# P1 — 复杂 UI 场景矩阵门禁

> 版本：1.0 | 日期：2026-07-15  
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

## Tier B（压力正确性，切片）

| ID | 场景 | 门禁 | 状态 |
|----|------|------|------|
| B1 | Many rrects | `TestP1_B1_ManyRRectsCorrectness` | ✅ 首轮 |
| B2–B6 | atlas / gallery / blend stack / path AA / hidpi | — | 🔄 后续 |

## 能力表同步收口（本轮）

| ID | 能力 | 门禁 |
|----|------|------|
| S.03 | 窗口 present | `window_present` / X11 tags |
| S.05 | Resize + GPU 重绘 | `TestP1_Capability_S05_ResizeGPU` |
| S.08 | HiDPI hairline | `TestP1_Capability_S08_HiDPIHairline` |
| B.06 | Paint alpha | `TestP1_Capability_B06_PaintAlpha` |

## 命令

```bash
export WGPU_NATIVE_PATH=/path/to/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache

go test -count=1 ./render -run 'TestP1_'
go test -count=1 ./render -run 'TestS3c_|TestS3b_|TestS3a_|TestP12GPUFixedPixel|TestP1_'
```

## 仍 open（不挡 Tier A 关闭）

- B.02 完整 Porter-Duff **GPU** 固定函数  
- D.03 Sweep/conic gradient  
- X.05 LCD / subpixel text  
- C.05 Clip AA 深度  
- Tier B B2–B6 压力场景  
