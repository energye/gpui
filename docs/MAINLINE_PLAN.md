# GPUI 渲染栈主线计划（精简）

> 版本：1.52 | 日期：2026-07-15  
> 状态：**唯一执行主线**  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> 能力基准：[`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md)

---

## 1. 目标与非目标

### 目标

1. **`rwgpu`**：按 Skia 级 2D 所需 WebGPU 能力，把 ABI **绑全、绑对、可测**（对照 `lib/webgpu.h`）。
2. **`gpu/webgpu`**：对象 facade 完整承接上述子集（转换、生命周期、真 native）。
3. **`render`**：按同一能力表实现对标 Skia 的 2D 渲染语义与可验证像素结果。
4. **`S5 起`**：在 Skia 级 2D 能力之上，把引擎做成 **可支撑复杂 2D / 未来 UI 控件** 的产品形态（present/retained/damage/主路径帧预算）；**仍不实现控件层**。

### 非目标（本主线排除）

- Ant Design / 任何 **控件层** 实现  
- 在 S3 正确性之前做大规模性能优化；**S4 已完成 batch/atlas/cache/damage 底座**；S5 只做 **present 真帧率 + UI 主路径预算**，不做无边界“刷分”  
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
  → A   任意组合维度门禁（D01–D200）
  → S4  性能底座（batch / atlas / cache / damage）
  → S5  UI 引擎硬化（present 真帧率 + Skia 控件向缺口 + 主路径 60fps）
  → （之后才允许）控件层 / 类 Ant Design 组件
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

### S4 — 性能（已入主线；A 收口后启动）

**进入条件**：S3 对应能力正确 + **阶段 A（组合维度）门禁绿**。  
**硬规则**：任何优化切片结束后必须回归 `TestS3*` / `TestP1_*` / `TestP1_Comp_*` 像素与 `GPUOps>0`；禁止 silent CPU 冒充 GPU 加速。

| 子阶段 | 目标 | 退出条件 |
|--------|------|----------|
| **S4.0 基线** | 只测量、不改算法：在 P1/A 场景上记录 FPS/frame time、`gpu_ops`、`cpu_fallback_ops`、上传/draw 计数（可得则记） | ✅ 产出 `docs/S4_PERF_BASELINE.md` + `TestS4_PerfBaseline_Scenes` + `tmp/s4_baseline.json` |
| **S4.1 batch** | 同类 draw 合并 / instance 或顶点批 | ✅ Image multi-quad（`docs/S4_1_BATCH.md`）；SDF/text 既有 batch 保留 |
| **S4.2 glyph/atlas** | 字形图集命中与上传收敛 | ✅ partial dirty upload + AdvanceFrame（`docs/S4_2_GLYPH_ATLAS.md`） |
| **S4.3 path/texture cache** | path 几何/纹理复用 | ✅ `docs/S4_3_PATH_TEXTURE_CACHE.md` + `TestS43_*` |
| **S4.4 damage/retained** | 脏区/保留层减少重绘 | ✅ `docs/S4_4_DAMAGE_RETAINED.md` + B14/B15 + `TestS44_*` |

**明确后置（S4 已关闭，仍不阻塞主线）**：完整 multiplanar YUV、完整 PDF/SVG 引擎（R.02）、与 Skia 绝对 FPS 对标报表。

---


### S5 — UI 引擎硬化（当前焦点；S4 之后）

**进入条件**：S3 能力门禁绿 + 阶段 A 关闭 + **S4.0–S4.4 关闭**。  
**总目标**：Skia **能画的主路径**在本引擎上不仅“测得过”，还要 **能当复杂 2D / 未来 UI 控件的后端**——窗口 present 稳定、retained/damage 可用、主路径帧预算可验收。  
**一句话**：S3=画对，S4=底座优化，**S5=引擎可扛 UI**。

#### 非目标（S5 仍排除）

- Ant Design / 任何 **控件层** 组件实现（Button/Table/Form 等）  
- 完整 PDF/SVG 引擎（**R.02**，M4 旁路，不阻塞 S5）  
- 完整 multiplanar YUV（媒体旁路，不阻塞 S5）  
- 与 Skia **绝对 FPS 对标报表**（可选附录；S5 门禁用本机 present 预算，不绑定 Skia 分数）  
- 无界扩组合探针（D201+）；A 的 D01–D200 仅作回归  

#### 硬规则

1. 声称 GPU：必须 `WGPU_NATIVE_PATH` 真库 + 可观测 `GPUOps>0`；禁止 silent CPU。  
2. **禁止**用 `FlushGPU` 读回 wall-time 宣称 60fps；60fps 只认 **present 无读回**（或明确标记的 present-only 场景）。  
3. 任何 S5 切片结束：回归 `TestS3*` / `TestP1_*` / `TestP1_Comp_*` 像素与 GPU 路径。  
4. 发现 ABI/facade 缺口：先回 S1/S2 补齐再继续。  
5. “对标 Skia”优先 **语义与可组合能力**；性能门禁服务 **UI 主路径可交互**，不是全面刷分。

#### 子阶段

| 子阶段 | 要做什么 | 主要产出 / 退出条件 |
|--------|----------|---------------------|
| **S5.0 Skia 控件向对账** | 以能力表为源，按 **未来 UI 控件依赖度** 重排缺口：text/clip/layer/image/blend/scroll/modal 相关为 P0；R.02/YUV 等旁路 | `docs/S5_SKIA_UI_GAP.md`：P0/P1/P2 清单 + 每项“已有测试 / 需补 / 书面后置” |
| **S5.1 Present-only 基线** | 窗口（Linux X11）多帧 `PresentFrame` / `PresentFrameDamage*`；**无 CPU 读回**；记录 `present_ms` / FPS；与 S4 读回基线分列 | `docs/S5_PRESENT_BASELINE.md` + `TestS5_Present_*` + `tmp/s5_present_baseline.json` |
| **S5.2 Retained + Damage 帧模型** | 默认 UI 帧 = 复用 Context + 脏区 present；固定 API/惯例（何时 `ResetFrameDamage`、多 region、HiDPI）；MSAA 仍 ADR-021（scissor∩damage，非 LoadOpLoad） | `docs/S5_FRAME_MODEL.md` + 真窗口 multi-region damage e2e 绿 |
| **S5.3 主路径 60fps 门禁** | 在 present-only + retained 上设 **UI 主路径** 预算（默认 16.7ms@60Hz，可按场景分级）；场景至少：静态壳、列表滚动、表单输入脏区、弹层/抽屉 | `docs/S5_60FPS_GATE.md`；主路径场景门禁绿或书面降级理由 |
| **S5.4 控件向能力补丁** | 只补 S5.0 标为 **P0/P1 且阻塞 UI** 的 Skia 缺口（语义+像素+GPU）；不实现控件 | 能力表状态回写；对应 `TestP1_Capability_*` / 新增 S5 门禁 |
| **S5.5 控件层开工冻结** | 写清 **允许开始控件层** 的入口条件（能力+帧模型+主路径预算）；S5 关闭 | `docs/S5_WIDGET_ENTRY.md`；清单全勾后 **S5 关闭** |

#### S5.0 对账维度（写进 gap 文档时必填）

按 UI 依赖，不按“刷完成度”：

| 优先级 | 维度 | 典型控件/场景用途 |
|--------|------|-------------------|
| **P0** | Text（shape/LCD/CJK/atlas）、Clip（rect/rrect/path）、Layer opacity/mask、Image、RRect、Blend/premul | 几乎所有控件 |
| **P0** | Present / FrameDamage / HiDPI / Transform | 窗口与滚动 |
| **P1** | Backdrop、Shadow/Blur、Dash、Gradient/Pattern、Nine-patch、Filter 常用子集 | 浮层/卡片/输入装饰 |
| **P2** | Mesh/Vertices 边角、宽色域 F16 全链路、冷门 blend、录制回放 | 少见或可后置 |
| **旁路** | R.02 PDF/SVG、真 multiplanar YUV | 不阻塞控件层入口 |

#### S5.3 场景与预算（初始；S5.1 基线后可微调数字）

| ID | 场景 | 帧模型 | 初始预算（present-only） |
|----|------|--------|-------------------------|
| U01 | 静态应用壳（顶栏+侧栏+内容底） | retained | ≤16.7ms（60fps） |
| U02 | 虚拟列表滚动（行复用形态，非控件） | retained + damage | ≤16.7ms |
| U03 | 表单局部输入脏区（单字段/校验条） | retained + multi-damage | ≤16.7ms |
| U04 | 模态/抽屉打开后的静止帧 | retained | ≤16.7ms |
| U05 | kitchen-sink 应力（回归用） | 可选 cold/retained | **不设 60fps 硬门**；只记录 + 禁止回归性恶化 |

预算测量：**present 路径 wall-time**（draw+encode+submit+present），不含 `ReadPixels`。  
机器差异：文档记录 GPU/驱动；门禁以 **本仓库 CI/开发机约定环境** 为准，允许 `S5_FPS_BUDGET_MS` 覆盖。

#### S5 验证命令（草案）

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=/home/yanghy/app/projects/gogpu/gpui/lib:$LD_LIBRARY_PATH
export DISPLAY=:1   # 窗口 present 需要

go test -count=1 ./render -run 'TestS5_' -timeout 300s
go test -count=1 ./render -run 'TestS3|TestP1_|TestP1_Comp_' -timeout 300s
```

#### S5 关闭条件（总）

- [ ] S5.0–S5.5 文档与对应测试均已产出  
- [ ] Present-only 基线可复跑；主路径 U01–U04 达预算或书面豁免（豁免须说明为何不阻塞控件）  
- [ ] P0 控件向 Skia 缺口清零或仅剩已签字后置  
- [ ] 回归全绿 + `GPUOps>0`  
- [ ] **仍无控件层代码**；`S5_WIDGET_ENTRY.md` 入口条件已冻结  

**S5 关闭后**：才允许开 **控件层主线**（类 Ant Design 组件），且控件实现不得绕过 render 能力表另起一套光栅化。

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
| 7 | 能力表 🔄 / M4 GPU 相关 | ✅（仅 R.02 PDF/SVG document ⬜ 旁路） |
| 8 | P1 复杂 UI 形态 Tier A–U | ✅（形态密度探针，非控件层） |
| 9 | **阶段 A：任意组合维度 D01–D200** | ✅ **已关闭**（chi 至 D200；停扩组合探针） |
| 10 | **S4 性能** | ✅ **S4.0–S4.4 全线关闭**（见 `docs/S4_*.md`） |
| 11 | **S5 UI 引擎硬化** | 🔄 **当前焦点** → 先 **S5.0 Skia 控件向对账** |

### 阶段 A — 任意组合维度（非 antd 控件清单）

**目标**：用**可组合图元 + 便捷 API** 支撑任意 UI 场景，而不是固定 antd 场景目录。  
**形态**：组合维度矩阵（`clip × layer × blend × text × image × transform × HiDPI × damage/backdrop`），见 [`P1_COMPOSITION_MATRIX.md`](./P1_COMPOSITION_MATRIX.md)。  
**验证**：真 `WGPU_NATIVE_PATH` + `GPUOps>0` + 结构像素/区域检查。

**A 关闭条件**：

- [x] 组合维度文档 + **D01–D200** 探针（omega+sigma+tau+phi+chi）  
- [x] `TestP1_Comp_*` 全绿（真 GPU；200 条）  
- [x] 覆盖 clip(含 Preserve)/layer/blend/text(modes+shape)/image/transform/HiDPI/mask/mesh/atlas/pathEffect/gradient/pattern/backdrop/FrameDamage/PresentFrame/PresentFrameDamageRects/filter/writePixels/external/Resize/multi-context + 更深形态应力  
- [x] **停扩**组合探针（止 D200）  
- [x] 关闭 A 前全量 `TestP1_*`（形态 Tier A–U）回归  
- [x] **阶段 A 关闭** → 启动 **S4.0 基线**  
- [x] **S4.0 基线关闭** → 启动 **S4.1 batch**  
- [x] **S4.1 batch 关闭**（image multi-quad）→ 启动 **S4.2 glyph/atlas**  
- [x] **S4.2 glyph/atlas 关闭**
- [x] **S4.3 path/texture cache 关闭** → `docs/S4_3_PATH_TEXTURE_CACHE.md`
- [x] **S4.4 damage/retained 关闭** → `docs/S4_4_DAMAGE_RETAINED.md`（**S4.x 全线关闭**）  

**A 已关闭**。**S4.0–S4.4 已关闭**。  
**当前焦点：S5 UI 引擎硬化**（先 S5.0 对账 → S5.1 present 基线）。  
R.02 / 真 YUV / Skia 绝对 FPS 报表仍后置，不阻塞 S5。

**非目标（仍排除）**：控件层 / Ant Design 组件实现（直至 **S5.5 入口条件**满足）；R.02 可并行旁路。

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=/home/yanghy/app/projects/gogpu/gpui/lib:$LD_LIBRARY_PATH

# 阶段 A + 回归
go test -count=1 ./render -run 'TestP1_Comp_|TestP1_|TestS3a_|TestS3b_|TestS3c_|TestP12GPUFixedPixel' -timeout 180s
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
| `docs/P1_COMPLEX_UI_MATRIX.md` | P1 复杂 UI **形态**矩阵（Tier A–U，密度探针） |
| `docs/P1_COMPOSITION_MATRIX.md` | **阶段 A** 任意组合维度矩阵（正确性完备） |
| `docs/S4_PERF_BASELINE.md` | S4.0 产出（✅ B01–B15 + retained B14/B15） |
| `docs/S4_1_BATCH.md` | S4.1 产出（✅ image multi-quad） |
| `docs/S4_2_GLYPH_ATLAS.md` | S4.2 产出（✅ dirty partial upload） |
| `docs/S4_3_PATH_TEXTURE_CACHE.md` | S4.3 产出（✅ path/stroke/image cache） |
| `docs/S4_4_DAMAGE_RETAINED.md` | S4.4 产出（✅ damage/retained；**S4.x 关闭**） |
| `docs/S5_SKIA_UI_GAP.md` | S5.0 产出（控件向 Skia 对账；待写） |
| `docs/S5_PRESENT_BASELINE.md` | S5.1 产出（present-only 基线；待写） |
| `docs/S5_FRAME_MODEL.md` | S5.2 产出（retained/damage 帧模型；待写） |
| `docs/S5_60FPS_GATE.md` | S5.3 产出（主路径 60fps 门禁；待写） |
| `docs/S5_WIDGET_ENTRY.md` | S5.5 产出（控件层开工条件；待写） |
| `docs/OPTIMIZATION_PLAN.md` | 历史大计划；服从主线 |

---

## 6. 修订记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.52 | **写入 S5 UI 引擎硬化**：S5.0–S5.5 定义；当前焦点 S5.0；控件层推迟到 S5.5 入口条件 |
| 2026-07-15 | 1.51 | **S4 收口审计**：产出表补齐 S4_1–S4_4；回归套件复验；确认 S4.x 全线关闭 |
| 2026-07-15 | 1.50 | **S4.x 全线关闭**：S4.3 path/texture cache + S4.4 damage/retained；文档 `S4_3_*`/`S4_4_*`；B14/B15 retained 基线 |
| 2026-07-15 | 1.49 | **S4.3 关闭**：Path/Stroke geometry cache + ImageCache stats |
| 2026-07-15 | 1.48 | **S4.2 关闭**：glyph atlas dirty partial upload + AdvanceFrame；焦点 → S4.3 |
| 2026-07-15 | 1.47 | **S4.1 batch 关闭**：image multi-quad coalescing + B13 + TestS41_*；焦点 → S4.2 |
| 2026-07-15 | 1.46 | **S4.0 基线关闭**：`TestS4_PerfBaseline_Scenes` + `docs/S4_PERF_BASELINE.md`；焦点 → S4.1 batch |
| 2026-07-15 | 1.45 | **阶段 A 关闭**：D01–D200 全绿；停扩组合探针；焦点 → S4.0 |
| 2026-07-15 | 1.44 | 阶段 A 扩展至 **D01–D180** phi 组合；继续复杂场景 |
| 2026-07-15 | 1.43 | 阶段 A 扩展至 **D01–D160** tau 组合；继续复杂场景 → 后 S4.0 |
| 2026-07-15 | 1.42 | 阶段 A 扩展至 **D01–D140**（omega+sigma）；**停 D140**；关 A 前全量 P1 → S4.0 |
| 2026-07-15 | 1.41 | 阶段 A 扩展至 **D01–D120** omega 组合；仍焦点 A → 后 S4.0 |
| 2026-07-15 | 1.40 | 阶段 A 扩展至 **D01–D105** hyper 组合；仍焦点 A → 后 S4.0 |
| 2026-07-15 | 1.39 | 阶段 A 扩展至 **D01–D90** ultra 组合；仍焦点 A → 后 S4.0 |
| 2026-07-15 | 1.38 | 阶段 A 扩展至 **D01–D75** mega 组合；仍焦点 A → 后 S4.0 |
| 2026-07-15 | 1.37 | 阶段 A 扩展至 **D01–D58** 极端组合；仍焦点 A → 后 S4.0 |
| 2026-07-15 | 1.36 | 阶段 A 扩展 D09–D36 高密度复杂组合；仍焦点 A（近收口）→ 后 S4.0 |
| 2026-07-15 | 1.35 | **开 S4 入主线**（S4.0–S4.4）；焦点=阶段 A 组合维度 → A 后 S4.0；非控件层 |
| 2026-07-15 | 1.31 | K.01/Q.02 gates + B.03 ColorBurn/Exclusion + Tier O/P + X11 multi-rect PresentDamage |
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
