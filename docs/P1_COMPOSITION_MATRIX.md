# 阶段 A — 任意组合维度矩阵（Composition Matrix）

> 版本：1.1 | 日期：2026-07-15  
> 主线：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) v1.35+  
> 能力表：[`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md)  
> 形态密度（旁证）：[`P1_COMPLEX_UI_MATRIX.md`](./P1_COMPLEX_UI_MATRIX.md)  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`

## 定位

| 是 | 不是 |
|----|------|
| 验证 **任意组合** 图元/状态交叉后像素与 GPU 链路仍正确 | Ant Design / 控件层实现 |
| 用维度轴覆盖场景空间（可扩展、可组合） | 固定产品 UI 清单打勾即完工 |
| S4 之前的正确性/覆盖度门禁 | 性能数字门槛 |

复杂 UI Tier A–U 是 **形态密度采样**；本矩阵是 **组合完备性** 主轴。目标：render 层 primitive + convenience API 足以支撑任意场景拼装，而不是只覆盖 antd 命名场景。

## 硬规则

1. `WGPU_NATIVE_PATH` 真库；`GPUOps > 0`  
2. 无 silent CPU-only 冒充 GPU 完成  
3. 关键结构像素 / 内外区域检查  
4. 性能数字 **不** 作为 A 关闭条件（留给 S4.0）  
5. 发现 ABI/facade 缺口：回 S1/S2 补，再继续 A  

## 维度轴（可增不可假关闭）

| 轴 | 代表 API / 语义 |
|----|-----------------|
| **clip** | `ClipRect` / `ClipRoundRect` / path `Clip` 嵌套 |
| **layer** | `PushLayer` / `PopLayer` 透明度与嵌套 |
| **blend** | `SetBlendMode` / layer blend（SrcOver/Plus/Multiply…） |
| **text** | `DrawString` / 图集路径 |
| **image** | `DrawImage` / `DrawImageQuad` / 可选 `DrawGPUTexture` |
| **transform** | `Translate` / `Scale` / `Rotate` + CTM 下 clip/fill |
| **HiDPI** | `WithDeviceScale` + hairline / 文本 |
| **backdrop/damage** | `PushBackdropLayer`；多区域重绘（脏区语义预热，S4.4 再优化） |

组合写法：`Dxx = 轴1 × 轴2 × …`，每条探针至少 **3 轴交叉**。

## 探针状态

| ID | 交叉 | 场景意图 | 门禁 | 状态 |
|----|------|----------|------|------|
| D01 | clip × layer × text | 嵌套矩形 clip + 半透明层 + 标签 | `TestP1_Comp_D01_ClipLayerText` | ✅ |
| D02 | clip × image × blend | 圆角 clip 内图像 + Plus 叠色 | `TestP1_Comp_D02_ClipImageBlend` | ✅ |
| D03 | clipPath × layer × fill | 多边形 path clip + 层内填充 | `TestP1_Comp_D03_ClipPathLayerFill` | ✅ |
| D04 | HiDPI × hairline × text | DPR=2 下 hairline 与文字共存 | `TestP1_Comp_D04_HiDPIHairlineText` | ✅ |
| D05 | layer × blend × clip | 外 clip + Multiply 层叠 | `TestP1_Comp_D05_LayerBlendClip` | ✅ |
| D06 | image × text × clip × backdrop | 内容区 + 文字 + backdrop dim | `TestP1_Comp_D06_ImageTextClipBackdrop` | ✅ |
| D07 | transform × clip × fill | Translate/Scale 下 clip 与填充 | `TestP1_Comp_D07_TransformClipFill` | ✅ |
| D08 | multi-region redraw | 全量底图后多脏区局部重绘正确性 | `TestP1_Comp_D08_MultiRegionRedraw` | ✅ |

## A 关闭清单

- [x] D01–D08 真 GPU 绿（本批）  
- [ ] 主交叉轴无明显空洞；按需增 D09+（仍维度 ID，不绑控件名）  
- [ ] `go test ./render -run 'TestP1_Comp_|TestP1_|TestS3a_|TestS3b_|TestS3c_|TestP12GPUFixedPixel'` 绿  
- [ ] 主线焦点切到 **S4.0 基线**  

## 验证命令

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=/home/yanghy/app/projects/gogpu/gpui/lib:$LD_LIBRARY_PATH

go test -count=1 ./render -run 'TestP1_Comp_' -timeout 120s
```

## 与后续 S4 的关系

- A 场景将作为 S4.0 **测量输入**（尤其 D05/D06/D08 与既有 P1 压力场景）  
- S4 优化不得删减本矩阵断言；只能在基线文档中记录加速比  

## 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | 建立阶段 A；首批 D01–D08 |
| 2026-07-15 | 1.1 | D01–D08 真 GPU 门禁全绿（gpu_ops>0, cpu_fallback=0） |
