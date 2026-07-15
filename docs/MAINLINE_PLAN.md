# GPUI 渲染栈主线计划（精简）

> 版本：1.30 | 日期：2026-07-15  
> 状态：**唯一执行主线**  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> 能力基准：[`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md)

---

## 1. 目标与非目标

### 目标

1. **`rwgpu`**：按 Skia 级 2D 所需 WebGPU 能力，把 ABI **绑全、绑对、可测**（对照 `lib/webgpu.h`）。
2. **`gpu/webgpu`**：对象 facade 完整承接上述子集（转换、生命周期、真 native）。
3. **`render`**：按同一能力表实现对标 Skia 的 2D 渲染语义与可验证像素结果。

### 非目标（本主线排除）

- Ant Design / 任何 **控件层** 实现  
- 过早的大规模性能优化（batch/atlas/cache 大工程）  
- 旧计划中与主线无关的杂项里程碑（Skia FPS 对比报表、并行光栅化任务卡等）**暂不执行**  
- 无界“WebGPU 规范 100% 每个扩展都绑”

历史文档 [`OPTIMIZATION_PLAN.md`](./OPTIMIZATION_PLAN.md) 保留作档案；**任务优先级以本文 + 能力表为准**。

---

## 2. 主线顺序（禁止颠倒）

```text
S0  冻结 Skia 2D 能力表（全面，只增不删必选项）
  → S1  rwgpu ABI：按能力表反推的 WebGPU 子集全面对齐 header + 测试
  → S2  gpu/webgpu：子集 facade 完整、真调用、可测
  → S3  render：按 M0→M1→M2→M3 切片实现对标 + 像素/语义门禁
  → S4  性能（仅 S3 对应能力正确后）
```

每个 S3 切片若发现 ABI/facade 缺口：**先回 S1/S2 补齐再继续**，禁止用 CPU silent fallback 冒充 GPU 完成。

---

## 3. 阶段定义

### S0 — 能力表与基线 ✅（本轮）

| 项 | 状态 |
|----|------|
| 全面 Skia 2D 能力表 | ✅ `docs/SKIA_2D_CAPABILITY_MATRIX.md` |
| WebGPU/rwgpu 反推子集 | ✅ 见表 §2 |
| 现状粗粒度差距 | ✅ 见表 §4 |
| 主线计划替换杂项目录 | ✅ 本文 |

**完成标准**：能力表覆盖 Surface/Transform/Paint/Blend/Path/Clip/Layer/Gradient/Image/Text/Effect/Filter/MSAA/ColorSpace/Recording 等；后续只增行。

---

### S1 — rwgpu ABI 全面（Skia 2D 子集）

**目标**：`lib/webgpu.h` 为准，子集内 enum/struct/函数绑定正确。

**工作项**：

1. 从能力表 §2 列出 **必绑 API 清单**（可机器生成 checklist）。  
2. 审计 `gpu/rwgpu/convert.go` 与 wire struct：凡 `types.*` 写入 native 必须显式映射。  
3. 扩展 `abi_test.go`：size/offset/enum 转换；关键路径 `WGPU_NATIVE_PATH` 烟测。  
4. 缺口：补绑定或标记“非子集延后”，但 M0–M2 依赖项不得延后。

**验证**：

```bash
export WGPU_NATIVE_PATH=/path/to/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
go test -count=1 ./gpu/rwgpu
```

**完成标准**：

- [x] 能力表 §2.1–2.4 所列 API 均有绑定或书面豁免（豁免不得挡 M2）— 函数级齐全；A–E 烟测 ✅；F/MSAA resolve/Indirect 书面后置  
- [x] enum 不再依赖“碰巧与 header 相等” — `TestS1*` + `convert.go`  
- [x] `go test ./gpu/rwgpu` 全绿  
- [x] 文档：`docs/RWGPU_SKIA_SUBSET_CHECKLIST.md`（API ↔ 文件 ↔ 测试）  
- [x] A–E descriptor/lifetime/native 烟测深度清零 — `TestS1AE*`  
- [x] 能力表 §2 状态列回写  

---

### S2 — gpu/webgpu facade 补全

**目标**：render 只依赖 webgpu；子集内无 stub 冒充生产路径。

**工作项**：

1. 对照 S1 清单，审计 `gpu/webgpu` → `rwgpu` 转换。  
2. descriptor 字段完整、pointer lifetime 安全（已有测试模式延续）。  
3. 禁止 render 直 import `rwgpu`（保持架构）。

**验证**：

```bash
go test -count=1 ./gpu/webgpu
go test -count=1 ./render/internal/gpu -run 'Test.*(Native|Pipeline|Texture|Clear)'
```

**完成标准**：

- [x] 子集 API 均有 facade 对象方法 — 见 `docs/WEBGPU_FACADE_S2_CHECKLIST.md`  
- [x] conversion 测试覆盖高风险 enum/stencil/blend/topology — `TestS2Convert*`  
- [x] `go test ./gpu/webgpu` 全绿 — 含 `TestS2AE*` 真链路  

---

### S3 — render 对标 Skia 2D（切片）

按能力表优先级推进；每切片：实现 → GPU 真链路测试 → 更新能力表状态列。

| 切片 | 能力焦点 | 退出条件（摘要） |
|------|----------|------------------|
| **S3a M0–M1** | 清屏、path fill/stroke、AA、CTM、solid、clip rect、hairline | ✅ `docs/S3A_M0M1_RENDER_GATE.md` |
| **S3b M2** | blend/premul、image、text、rrect、layer opacity、dash、gradient、MSAA | UI 级 2D 门禁绿 |
| **S3c M3** | 高级 clip/filter/shadow、vertices/atlas、surface present、color space… | ✅ `docs/S3C_M3_RENDER_GATE.md`（窗口 Present 后置） |

**硬规则**：

- 声称 GPU：必须 `WGPU_NATIVE_PATH` 真库 + 可观测 `gpu_ops`（已有 P1.0 可保留）  
- 未解释 fallback 不得关闭切片  
- 性能数字不作为 S3 退出条件  

**完成标准（S3b 作为“可宣称 Skia 级 UI 2D 能力”门槛）**：

- [x] M2 核心能力 GPU 门禁（含 MSAA/clip path/miter/gradient）— `docs/S3B_M2_RENDER_GATE.md`  
- [x] 固定像素 + STRICT 场景（basic/shapes/images/text/clipping）  
- [x] 已知差异书面后置（完整 PD GPU、sweep gradient 等）  

---

### S4 — 性能（后置）

仅当对应能力在 S3 正确后：批处理、图集、缓存、并行等。  
仍须回归 S3 门禁。

---

## 4. 当前执行焦点（2026-07-15）

| 顺序 | 动作 | 状态 |
|------|------|------|
| 1 | S0 能力表 + 主线计划 | ✅ |
| 2 | S1 enum + A–E 烟测 | ✅ |
| 3 | S2 webgpu facade | ✅ |
| 4 | S3a render M0–M1 GPU 门禁 | ✅ **S3a 关闭** |
| 5 | S3b M2 UI 级 2D | ✅ **S3b 关闭** |
| 6 | S3c M3 + S.03 Swapchain/PresentFrame | ✅ **S3c 关闭**（窗口 e2e 需 DISPLAY） |

S0–S3c 已关闭。S.03 真窗口 draw+present 已通。

**本轮（P1 推进）**：已收矩阵 **rwgpu/webgpu 🔄→✅ 对齐**（render 已证 M0–M3 行）、B.03 Soft/Hard/ColorDodge 门禁补全、Tier G–**N**（retained multi-panel damage + IDE layout）。仍 open：M4 可选项、K.01 vello 深化。

下一步：M4 后置；K.01 或更多 retained e2e（真 view damage present）可选。

```bash
go test ./render -run 'TestS3c_|TestS3b_|TestS3a_|TestP12GPUFixedPixel'
```


门禁：
```bash
go test ./render -run 'TestS3a_|TestP12GPUFixedPixel|TestS3b_'
```


已完成可复用资产（并入主线，不另开叙事）：

- P0 视觉 STRICT / format readback / path stats / SourceOver GPU 固定像素 → 作为 S3 回归工具  
- S1 enum header-lock：身份枚举 + 转换枚举 + silent-identity 回归  
- S1 A–E smoke：`s1_ae_smoke_test.go`（buffer/texture/draw 读回）  
- S2 facade：`s2_facade_conversion_test.go` + `s2_ae_smoke_test.go`  
- S3a gate：`s3a_m0m1_gpu_gate_test.go` + P1.2 fixed pixels  

---

## 5. 目录与文档

| 文件 | 角色 |
|------|------|
| `docs/SKIA_2D_CAPABILITY_MATRIX.md` | **能力真相来源** |
| `docs/MAINLINE_PLAN.md` | **执行主线**（本文） |
| `docs/RWGPU_SKIA_SUBSET_CHECKLIST.md` | S1 产出（已关闭） |
| `docs/WEBGPU_FACADE_S2_CHECKLIST.md` | S2 产出（facade 已关闭） |
| `docs/S3A_M0M1_RENDER_GATE.md` | S3a 产出（M0–M1 GPU 门禁） |
| `docs/S3B_M2_RENDER_GATE.md` | S3b 产出（M2 UI 2D） |
| `docs/S3C_M3_RENDER_GATE.md` | S3c 产出（M3 vertices/atlas/filter/present） |
| `docs/P1_COMPLEX_UI_MATRIX.md` | P1 复杂 UI 场景矩阵（A1–A8/B1） |
| `docs/OPTIMIZATION_PLAN.md` | 历史大计划；服从主线 |

---

## 6. 修订记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.30 | matrix lower-layer align + B.03 Soft/Hard/Dodge gates + Tier N |
| 2026-07-15 | 1.29 | B.03 full separable advanced dual-tex + Tier M chart/heatmap |
| 2026-07-15 | 1.28 | S.07 WritePixels GPU + Tier L form/table/toasts |
| 2026-07-15 | 1.27 | B.04 HSL dual-tex GPU + F.03 CM/DropShadow GPU + Tier K |
| 2026-07-15 | 1.26 | L.06 stencil cover-inline R8 + F.03 true GPU multi-RT + Tier J |
| 2026-07-15 | 1.25 | L.06 SDF cover-inline R8 + mask lifetime-to-flush + Tier I |
| 2026-07-15 | 1.24 | L.06 convex cover-inline R8 + Tier H virtual/transfer density |
| 2026-07-15 | 1.23 | F.03 ApplyImageFilterGraph + L.06 MaskAware + Tier G TreeSelect/Carousel |
| 2026-07-15 | 1.22 | H.03/L.06 PushMask/P.04 + Tier F Cascader/VirtualList |
| 2026-07-15 | 1.21 | B.05 layer/text + Q.04 + Overlay + Tier E |
| 2026-07-15 | 1.20 | L.06 R8 modulate + P.05/P.06/B.05 + Tier D |
| 2026-07-15 | 1.19 | X.05 彩底 two-pass LCD + B.03 dual-tex 合成 |
| 2026-07-15 | 1.18 | T.03/X.06/X.11 GPU 门禁 |
| 2026-07-15 | 1.17 | X.03/X.04/Q.03/L.06 GPU 门禁 |
| 2026-07-15 | 1.16 | B.02 全 PD + D.04–D.06 GPU 门禁；P1 Tier C 复杂 UI |
| 2026-07-15 | 1.13 | P1 复杂 UI 矩阵 A1–A8/B1 + S.05/S.08/B.06 能力表收口 |
| 2026-07-15 | 1.12 | S.03 真窗口 draw+present：SetDeviceProvider 同 device + DeviceDescriptor limits；X11 multi-frame e2e |
| 2026-07-15 | 1.11 | S.03 Swapchain + PresentFrame + CreateSurface 空句柄防护；窗口 e2e（X11） |
| 2026-07-15 | 1.10 | S3c M3 residual 清零：B.04/C.06/H.04/I.04/I.07/X.08–X.10 门禁 |
| 2026-07-15 | 1.9 | S3c 关闭：V.01 DrawVertices + V.02 DrawAtlas GPU 门禁；窗口 Present 后置 |
| 2026-07-15 | 1.8 | S3c 启动：ApplyBlur/Shadow/Color + offscreen present 门禁 |
| 2026-07-15 | 1.7 | S3b 关闭：MSAA Q.01 + STRICT 五场景；下一步 S3c |
| 2026-07-15 | 1.6 | S3b：gradient GPU fillBrushAsImage + SetBlendMode；接近关闭 |
| 2026-07-15 | 1.5 | S3b 推进：M2 核心 GPU 门禁 + GPU dash + Image() flush；gradient GPU 仍 open |
| 2026-07-15 | 1.4 | S3a 关闭：M0–M1 GPU 固定像素门禁；下一步 S3b |
| 2026-07-15 | 1.3 | S2 关闭：facade 转换/烟测；下一步 S3a |
| 2026-07-15 | 1.2 | S1 关闭：A–E `TestS1AE*` 烟测；下一步 S2 |
| 2026-07-15 | 1.1 | S1 枚举 header-lock（TestS1*）；焦点转到 A–E 深度审计 |
| 2026-07-15 | 1.0 | 确立 S0–S4；排除控件层与杂项目录；能力表驱动 ABI→facade→render |
