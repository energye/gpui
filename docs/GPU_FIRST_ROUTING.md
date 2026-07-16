# GPU 优先路由原则与「有 GPU 仍 CPU」清单

> 版本：1.0 | 日期：2026-07-16  
> 状态：**主线硬原则（执行中）**  
> 权威：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) §1b  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`

---

## 1. 硬原则（不可破）

### R0 — GPU 优先

**有可用 GPU / 加速器时：绘制与合成主路径必须走 GPU。**  
**仅当平台没有可用 GPU，或设备/后端创建失败时，才允许退化到 CPU。**

| 情况 | 要求行为 |
|------|----------|
| `WGPU_NATIVE_PATH` 有效 + 加速器已注册 | 主路径 GPU；`GPUOps>0` |
| 某能力 **尚未** GPU 实现 | **记缺口 + 排期 GPU 化**；不得把「长期 CPU」当完成 |
| 无 GPU / `ensureGPU` 失败 / software adapter 不可用 | **显式** CPU fallback；`cpu_fallback_ops` + `LastCPUFallbackReason` |
| 测试 / soak（有 GPU） | **`cpu_fallback_ops=0`**（书面登记的临时例外除外） |
| 禁止 | silent CPU、假 `GPUOps`、关 AA / 降语义刷 PASS |

### R1 — Fallback 必须可观测

- 计数：`Context.RenderPathStats()` → `GPUOps` / `CPUFallbackOps`  
- 原因：`LastCPUFallbackReason`（短标签，如 `clip-mask`、`no-accel`、`brush-nonsolid`）  
- 日志：首次全局 fallback 可 warn，禁止刷屏掩盖根因  

### R2 — 「有 GPU 仍 CPU」的合法类别（仅三类）

| 类 | 含义 | 是否算完成 |
|----|------|------------|
| **A. 平台不可用** | 无设备 / 创建失败 / 明确 software adapter 策略 | ✅ 允许（唯一「正常」CPU 主路径） |
| **B. 能力缺口（有 GPU）** | GPU 在，但该 API/组合还没 GPU 实现或未默认走 GPU | ❌ **未完成**；必须进下方清单并清零或改默认 |
| **C. 正确性临时强制** | 例如某 clip 仅 CPU 正确，GPU 会 silent wrong | ⚠ 允许短期，**必须有 reason + 计划 GPU 修正确** |

**产品 60fps / Skia 对标的完成定义**：B 类主路径项清零或仅剩已签字后置；C 类有期限。

### R3 — 与性能门禁的关系

- UI 主路径（有 GPU）：稳定 ~60fps **且** `cpu_fb=0`  
- 重特效叠压：可分级预算（见 S6.9），**仍禁止** silent CPU 冒充 GPU 完成  
- mem_anim：`cpu_fb=0` 硬门禁；见 [`MEM_ANIM_LONGSOAK_PLAN.md`](./MEM_ANIM_LONGSOAK_PLAN.md) §0c  

---

## 2. 路由状态符号

| 符号 | 含义 |
|------|------|
| **GPU** | 有加速器时默认 GPU |
| **GPU\*** | 有 GPU 路径，但实现为「CPU 栅格/采样 + GPU blit」过渡（仍计 GPUOps，**未达原生 GPU**） |
| **CPU†** | **有 GPU 时仍强制/默认 CPU**（B/C 类缺口） |
| **CPU‡** | 仅无 GPU 时使用（A 类，合规） |
| **混合** | 部分模式 GPU、部分 CPU† |

---

## 3. 清单：有 GPU 仍走 CPU / 非原生 GPU（B/C 类）

> 用途：清缺口排期。优先修 **P0**。  
> 证据列指向代码或已知 soak 行为。状态随实现回写，**只增解释、不删未完成项**。

### 3.1 填充 / 笔刷 / 混合（P0）

| ID | 能力 | 状态 | 现状 | 证据 | 目标 |
|----|------|------|------|------|------|
| G.01 | Solid fill/stroke（简单几何） | **GPU** | SDF/path 主路径 | `tryGPUFill` / convex / path | 保持 |
| G.02 | Linear / Radial / Sweep **gradient fill** | **GPU\*** / 大面积易掉帧 | 非 solid 常 `fillBrushAsImage`：CPU `ColorAt` 栅格再上传；或掉回 CPU | `brush_fill.go` `fillBrushAsImage`；`isGPUSolidPaint` 拒非 solid | **原生 GPU brush/shader**，默认不逐像素 ColorAt |
| G.03 | Image **pattern** fill | **GPU\*** | 同 G.02 过渡路径 | 同上 | GPU texture pattern sampler |
| G.04 | CustomBrush / 任意 ColorAt brush | **CPU†** | 无 GPU 等价 | `Brush.ColorAt` 软件路径 | GPU 不支持则显式 reason；勿 silent |
| G.05 | Blend **SourceOver / Plus** 等 fixed-function | **GPU** | WebGPU blend state | `blend_gpu.go` `gpuBlendStateForPaint` | 保持 |
| G.06 | Blend **Multiply/Screen/Overlay** 等 advanced | **混合** | dual-tex GPU 路径存在；非 solid / 超大 bounds / 部分组合仍 `ErrFallbackToCPU` | `brush_fill.go`；`dual_tex_blend.go`；`paint.go` 注释 | **默认 dual-tex/GPU**；CPU 仅无 GPU |
| G.07 | 全屏/大区域 advanced blend on present | **CPU† 风险** | 注释：非 normal 曾走 CPU 合成 | `paint.go` BlendMode 注释；layer Pop 历史 ~50ms | 禁止大屏 CPU 公式；有界 RT + GPU |

### 3.2 Layer / Backdrop / Filter（P0）

| ID | 能力 | 状态 | 现状 | 证据 | 目标 |
|----|------|------|------|------|------|
| L.01 | `PushLayer` 内 **Fill/Stroke** | **CPU†** | `forceCPULayer`：层内故意 CPU 画进 pixmap，避免每帧 GPU→CPU readback | `context.go` `doFill`/`doStroke` `forceCPULayer` | **GPU layer RT**（纹理目标），Pop 时 GPU composite |
| L.02 | `PopLayer` 合成 | **混合** | damage 有界 CPU blend 优化过；全屏仍贵 | `context_layer.go` damage 注释 | GPU blend/composite pass |
| L.03 | `PushBackdropLayer` 快照 | **混合** | 有 GPU 门禁与池化；路径仍重 | S6.4 / L05 测试；mem_anim S07 | GPU snapshot + composite 默认 |
| L.04 | `ApplyBlur` / DropShadow / ColorMatrix 等 | **混合 / CPU† 热点** | 大表面 CPU 不可接受；mem_anim 用小 RT | `m4_extensions` fallback 计数；effects.go | GPU image filter pass；禁止 present 全屏 CPU blur |

### 3.3 Clip / Mask（P1）

| ID | 能力 | 状态 | 现状 | 证据 | 目标 |
|----|------|------|------|------|------|
| C.01 | Rect / 多数 path clip | **GPU** | scissor / stencil / depth-clip | GPU clip 路径 | 保持 |
| C.02 | **Mask clip** 且无 `gpuClipPath` | **CPU† (C 类)** | `forceCPUClip` + reason `clip-mask`，防 silent wrong | `context.go` `forceCPUClip` | 补 GPU depth/mask clip 后取消强制 |
| C.03 | ClipOpDifference 等边角 | **CPU† 风险** | 与 mask 同类正确性强制 | 同上 | GPU 语义对齐后默认 GPU |

### 3.4 文本（P0/P1）

| ID | 能力 | 状态 | 现状 | 证据 | 目标 |
|----|------|------|------|------|------|
| X.01 | GlyphMask / LCD（有加速器） | **GPU** | Tier6 + LCD 双 pass；混批 per-drawCall isLCD | `glyph_mask_pipeline.go` RecordDraws | 保持；禁止再混 BGL |
| X.02 | 热路径 **CJK reshape** 每帧 | **CPU 热成本** | shape 在 CPU；结果应缓存 | S6.5；mem_anim 闪屏史 | 强缓存；滚动只更新可见 run |
| X.03 | TextModeBitmap / 导出 | **CPU‡ 可接受** | 设计如此 | TextMode 文档 | 仅 export/无 GPU |
| X.04 | Vector/MSDF 不可用时 | **混合** | 可回退 | accelerator 路由 | fallback 必须 reason |

### 3.5 几何 / 路径（P1）

| ID | 能力 | 状态 | 现状 | 证据 | 目标 |
|----|------|------|------|------|------|
| P.01 | Convex / SDF 常用形状 | **GPU** | 主路径 | convex / SDF | 保持 |
| P.02 | 复杂 path / dash stroke | **GPU**（缓存加深中） | S4.3/S6.6 | path geometry docs | 保持缓存命中 |
| P.03 | 极冷门 path effect | **CPU† 可能** | 视实现 | 能力表 E.* | 逐项 GPU 或书面后置 |

### 3.6 图像 / 像素（P1）

| ID | 能力 | 状态 | 现状 | 证据 | 目标 |
|----|------|------|------|------|------|
| I.01 | DrawImage / Nine / Rounded 等 | **GPU** | textured quad | image pipeline | 保持 |
| I.02 | WritePixels | **GPU** 优先 + CPU mirror | `tryGPUWritePixels` | `context.go` | 保持双写语义正确 |
| I.03 | 读回 Image/SavePNG | **需同步** | Flush + readback（不是「绘制走 CPU」） | FlushGPU | 仅读回时同步；绘制仍 GPU |

### 3.7 帧 / Present（P0 策略）

| ID | 能力 | 状态 | 现状 | 证据 | 目标 |
|----|------|------|------|------|------|
| F.01 | `PresentFrameAuto` damage/idle | **GPU** | S6.1 | frame.go | 应用默认 |
| F.02 | 动画示例 `PresentFrameFull` 每帧 | **策略偏全量** | mem_anim 连续动画 | mem_anim main | UI 稳态 damage；全屏特效才 Full |
| F.03 | mid-frame FlushGPU | **混合** | 多 pass / LoadOpLoad | render_session | 单帧尽量单 submit |

### 3.8 平台 / 适配器（A 类，合规）

| ID | 能力 | 状态 | 现状 | 目标 |
|----|------|------|------|------|
| A.01 | 无 `WGPU_NATIVE_PATH` / 创建失败 | **CPU‡** | SoftwareRenderer | 保持显式 |
| A.02 | Software adapter（llvmpipe 等） | **策略可选 CPU‡** | SDF 在 software adapter 上曾 hang（BUG-SW-002） | 文档化策略；非 silent |

---

## 4. 清缺口优先级（执行序）

与「能 GPU 就 GPU」一致，**按 ROI**：

| 优先级 | 清单 ID | 一句话 |
|--------|---------|--------|
| **P0-1** | L.01 / L.02 | Layer 内绘制与 Pop → GPU RT + GPU composite（去掉 forceCPULayer 作为默认） |
| **P0-2** | G.02 / G.03 | 梯度 / pattern → 原生 GPU brush（消灭大面积 ColorAt） |
| **P0-3** | G.06 / G.07 | Advanced blend → 默认 dual-tex/GPU |
| **P0-4** | L.04 | Blur/shadow/filter → GPU pass；禁止 present 全屏 CPU |
| **P1-1** | X.02 | 文本 shape/atlas 强缓存 |
| **P1-2** | C.02 / C.03 | Mask/Difference clip → GPU 正确后取消 forceCPUClip |
| **P1-3** | F.02 / F.03 | 默认 damage + 少 mid-frame flush |
| **P2** | G.04 / 冷门 path effect | 显式后置或 reason |

关闭条件（本清单）：

- [ ] P0 项状态无 **CPU†**（可 **GPU** 或过渡期 **GPU\*** 但须有「升原生」子任务）  
- [ ] 有 GPU 的 mem_anim / S5/S6 门禁：`cpu_fb=0`  
- [ ] `forceCPULayer` / `forceCPUClip` 仅剩「无 GPU 等价且 reason 登记」或已删除  

---

## 5. 验收命令（有 GPU 环境）

```bash
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/lib:$LD_LIBRARY_PATH
export GOCACHE=/tmp/gpui-go-cache

# 路由计数：用例内 GPUOps>0 且 cpu_fallback_ops=0（除书面例外）
go test -count=1 ./render -run 'TestS5_|TestS6_|TestP1_Capability_' -timeout 300s

# 真窗口 soak（单场景）
GPUI_SCENARIO=S12 GPUI_ANIM_SECONDS=30 /tmp/mem_anim_window
# 结果行须 cpu_fb=0
```

---

## 6. 回写约定

1. 某 ID 改为默认 GPU 后：本表状态 → **GPU**，并链到测试名。  
2. 仅实现「CPU 栅格 + blit」：标 **GPU\***，不得标完成原生。  
3. 新增「有 GPU 仍 CPU」路径：**必须先加清单行**，禁止 silent。  
4. MAINLINE / mem_anim 排障每次对照 **§1 R0**。

---

## 7. 修订

| 版本 | 说明 |
|------|------|
| 1.0 | 初版：硬原则 + B/C 类清单 + 清缺口序；对齐用户目标「能 GPU 就 GPU，平台不能才 CPU」 |
