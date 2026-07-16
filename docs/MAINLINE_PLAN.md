# GPUI 渲染栈主线计划（精简）

> 版本：1.68 | 日期：2026-07-16  
> 状态：**唯一执行主线**  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> 能力基准：[`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md)

---

## 1. 目标与非目标

### 目标

1. **`rwgpu`**：按 Skia 级 2D 所需 WebGPU 能力，把 ABI **绑全、绑对、可测**（对照 `lib/webgpu.h`）。
2. **`gpu/webgpu`**：对象 facade 完整承接上述子集（转换、生命周期、真 native）。
3. **`render`**：按同一能力表实现对标 Skia 的 2D 渲染语义与可验证像素结果。
4. **`S5`**：在 Skia 级 2D 能力之上，把引擎做成 **可支撑复杂 2D / 未来 UI 控件** 的产品形态（present/retained/damage/主路径帧预算）。  
5. **`S6`**：在 S4 底座 + S5 帧模型之上做 **生产级深度性能优化**（全量重绘/重特效/真窗口/提交路径），**正确性回归不得降级**；S6 期间默认仍不实现控件层（可与控件层并行，但优化不得为控件特化而破坏通用语义）。

### 非目标（本主线排除）

- Ant Design / 任何 **控件层** 实现  
- 在 S3 正确性之前做大规模性能优化；**S4=底座，S5=UI 帧模型/主路径 60fps，S6=生产级深度优化**  
- 用降像素精度、关 AA、silent CPU、删组合断言等方式“刷”性能数字  
- 旧计划中与主线无关的杂项里程碑（并行光栅化任务卡等）**暂不执行**；Skia 绝对 FPS **报表**仅作 S6 可选附录，不替代本机 present 门禁  
- 无界“WebGPU 规范 100% 每个扩展都绑”

历史文档 [`OPTIMIZATION_PLAN.md`](./OPTIMIZATION_PLAN.md) 保留作档案；**任务优先级以本文 + 能力表为准**。

---

## 1b. 硬原则：GPU 优先（能 GPU 就 GPU）

> 详细清单与清缺口序：[`GPU_FIRST_ROUTING.md`](./GPU_FIRST_ROUTING.md)

### 原则正文

1. **有可用 GPU / 加速器时**：绘制、合成、present 主路径 **必须走 GPU**（`render → webgpu → rwgpu → libwgpu_native`）。  
2. **仅当平台没有可用 GPU**（未加载 native、设备创建失败、或书面登记的 software-adapter 策略）时，才允许 **显式退化 CPU**。  
3. **禁止**：
   - silent CPU（有 GPU 路径却不记 `cpu_fallback_ops` / 无 reason）  
   - 假 `GPUOps`、关 AA / 降语义 / 减场景内容刷性能或刷 PASS  
   - 把「有 GPU 但实现仍是 CPU」标成能力完成（须进 [`GPU_FIRST_ROUTING.md`](./GPU_FIRST_ROUTING.md) 清单）  
4. **Fallback 必须可观测**：`GPUOps` / `CPUFallbackOps` / `LastCPUFallbackReason`。  
5. **有 GPU 的门禁**：宣称 GPU 的路径 **`GPUOps>0` 且 `cpu_fallback_ops=0`**（书面临时例外除外，见清单 C 类）。  
6. **清缺口方向**：清单 B 类（有 GPU 仍 CPU†）按 P0→P1 改为默认 GPU；过渡期允许 **GPU\***（CPU 栅格 + GPU blit），但不得当作原生 GPU 完成。

### 与阶段的关系

| 阶段 | 要求 |
|------|------|
| S1–S3 / 组合 D | 正确性优先；发现 silent CPU → 记 reason 或回补 GPU |
| S4–S6 | 性能优化 **不得** 引入 silent CPU；回归锁 `cpu_fb=0` |
| mem_anim / 真窗口 | 硬门禁 `cpu_fb=0`；排障遵循 §0c + 本原则 |
| 控件层（后置） | 建立在 GPU 优先后端之上，不得另开 CPU 主路径 |

### 「有 GPU 仍 CPU」当前 P0 摘要（完整见清单文档）

| 优先级 | 项 | 状态 |
|--------|-----|------|
| P0-1 | Layer 内 Fill/Stroke + Pop 合成（Normal/Copy GPU RT） | **DONE** GPU RT + texture composite；advanced/mask 仍 CPU |
| P0-2 | 梯度 / pattern 大面积填充 | **DONE** 原生 + session-inline（N1/N2）；Custom 仍 GPU\* `brush:custom` |
| P0-3 | Advanced blend 默认路径 | **DONE** dual-tex tile + layer advanced Pop |
| P0-4 | Blur / filter 大表面 | **DONE** standalone `Apply*` + graph → GPU multi-RT；Gaussian 对齐 CPU；`TestP04_*` `cpu_fb=0` |
| P1 | Mask clip 强制 CPU、文本热 reshape、动画 Full present 策略 | 见清单 |

**GPU_FIRST 主线**：[`GPU_FIRST_ROUTING.md`](./GPU_FIRST_ROUTING.md) **v3.9.1 已关闭**（N1/N2 session-inline 完成；N3 fragment / N4 bicubic / N5 极冷门 **书面后置**）。硬原则仍有效，禁止降级已有 GPU 路径。

**下一执行刀（建议）**：[`CAPABILITY_MATRIX_WINDOW.md`](./CAPABILITY_MATRIX_WINDOW.md) **P3 C26–C29**（L0+C21–C25 已绿）；控件层仅在 `S5_WIDGET_ENTRY` 条件满足后。**不要默认**再开 N3/N5 优化。

**GPU_FIRST 回归**：关闭后必跑/选跑命令与证据见 [`GPU_FIRST_ROUTING.md`](./GPU_FIRST_ROUTING.md) **§10**。

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
  → S6  生产级深度性能优化（提交路径 / 重场景 / 真窗口；正确性回归锁定）
  → （推荐 S6 主路径达标后）控件层 / 类 Ant Design 组件
```

每个 S3 切片若发现 ABI/facade 缺口：**先回 S1/S2 补齐再继续**，禁止用 CPU silent fallback 冒充 GPU 完成（**§1b + [`GPU_FIRST_ROUTING.md`](./GPU_FIRST_ROUTING.md)**）。

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


### S5 — UI 引擎硬化（✅ 已关闭）

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

- [x] S5.0–S5.5 文档与对应测试均已产出  
- [x] Present-only 基线可复跑；主路径 U01–U04 达预算（p50≤16.7ms）  
- [x] P0 控件向 Skia 缺口清零或仅剩已签字后置（S5.0/S5.4 队列空）  
- [x] 回归抽样绿 + `GPUOps>0`  
- [x] **仍无控件层代码**；`S5_WIDGET_ENTRY.md` 入口条件已冻结  

**S5.0–S5.5 已关闭。** 允许开控件层主线（另章）。  

**S5 关闭后**：才允许开 **控件层主线**（类 Ant Design 组件），且控件实现不得绕过 render 能力表另起一套光栅化。

---


### S6 — 生产级深度性能优化（S6.0–S6.9 ✅；S5 之后）

**进入条件**：S4.0–S4.4 关闭 + S5.0–S5.5 关闭 + 主路径 U01–U04 present-only 门禁可复跑。  
**总目标**：把引擎从「主路径能 60fps」推进到 **可支撑真实复杂 2D / 高密度 UI 的生产性能**——全量重绘、layer/blend 叠压、长列表、复杂 path、真窗口 present 均可解释、可回归、可优化。  
**一句话**：S4=底座，S5=能扛 UI 主路径，**S6=生产级深度优化（非玩具）**。

#### 非目标（S6 排除 / 克制）

- 为刷分关闭 AA、降低默认像素语义、删 `TestP1_Comp_*` 断言  
- silent CPU / 假 `GPUOps`  
- 无界扩 D201+ 组合探针（A 仅回归）  
- 完整 PDF/SVG（R.02）、真 multiplanar YUV（旁路，除非成为实测瓶颈再单列）  
- 控件组件实现（可并行另章；S6 优化必须保持 **通用 render 语义**）  
- 把 Skia 绝对 FPS 数字当作唯一关闭条件（本机 present 门禁 + 相对 S6.0 基线改进 为主）

#### 硬规则（正确性优先于速度）

1. **正确性回归锁（每个 S6.x 切片结束强制）**  
   - 真 `WGPU_NATIVE_PATH`；声称 GPU 的路径 **`GPUOps>0` 且 `cpu_fallback_ops=0`**（允许书面解释的 CPU 段除外；对齐 **§1b / GPU_FIRST_ROUTING**）。  
   - 像素/结构门禁不得削弱：至少  
     - `TestS3*`（或 S3a/b/c 门禁集）  
     - `TestP1_Comp_` 抽样 **D01/D06/D08/D36/D63/D152** + 每切片相关 Comp  
     - 切片触及的 `TestP1_Capability_*` / `TestS4*` / `TestS5*`  
   - **全量** `TestP1_Comp_`（D01–D200）在 **S6.0 基线锁** 与 **S6 关闭前** 各跑至少一次；切片中期可用抽样，但关闭 S6 必须全量绿。  
   - 任何语义变化必须更新能力表 + 固定像素/区域断言；禁止“看起来差不多”。  

2. **性能计量锁**  
   - 产品/门禁数字只认 **present-only**（`PresentFrame*`，无 ReadPixels）；与 S4 读回基线 **分列**。  
   - 主指标：`total_ms_p50` / `fps_p50`；avg/p95 仅诊断。  
   - 每个优化切片必须提交：优化前/后 JSON 片段 + 回归命令输出摘要。  
   - 禁止用减少场景内容冒充优化（场景定义冻结在 S6.0）。  

3. **架构锁**  
   - 仍走 `render → webgpu → rwgpu → libwgpu_native`。  
   - MSAA damage 仍遵守 ADR-021（scissor∩damage，非 LoadOpLoad）。  
   - 发现 ABI/facade 缺口：先回 S1/S2。  

#### 回归测试准确性（强制契约）

| 层级 | 套件 | 何时 | 失败含义 |
|------|------|------|----------|
| L0 烟测 | `TestS54_*` / `TestS52_*` / `TestS53_*`（U01–U04） | 每切片 | 帧模型或主路径 60fps 回退 |
| L1 正确性 | `TestS3*` + Comp 抽样 + 相关 Capability | 每切片 | 像素/语义回退 |
| L2 组合全量 | `TestP1_Comp_` D01–D200 | S6.0 锁存 + S6 关闭 | 组合维度回归 |
| L3 性能 | `TestS5_PresentBaseline_Scenes` + S6 扩展场景 | 每切片前后 | 无改进或无故回退 |
| L4 窗口（可选加深） | `gpui_x11_present` / S6.8 套件 | S6.8 及关闭前 | 真 present 回退 |

**准确性要求**：

- 断言基于 **采样点 / 区域结构 / 路径统计**，不只 `err==nil`。  
- 性能门禁用 **p50**，固定 `S5_PERF_WARMUP`/`ITERS`（S6.0 写入文档）；机器差异用文档记录 GPU/驱动，不静默放宽。  
- 允许 `S5_ALLOW_SLOW=1` 仅用于过载机器软过；**不得**作为切片关闭的常规手段。  
- 回归失败 → 切片不得关闭；性能回退超过 S6.0 文档阈值（默认主路径 p50 恶化 >10% 且无说明）→ 必须修或书面豁免。

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=/home/yanghy/app/projects/gogpu/gpui/lib:$LD_LIBRARY_PATH

# L0+L3 性能与帧模型
go test -count=1 ./render -run 'TestS5_|TestS52_|TestS53_|TestS54_|TestS6_' -timeout 300s

# L1 正确性抽样
go test -count=1 ./render -run 'TestS3|TestP1_Comp_(D01|D06|D08|D36|D63|D152)_' -timeout 300s

# L2 组合全量（S6.0 / S6 关闭）
go test -count=1 ./render -run 'TestP1_Comp_' -timeout 600s
```

#### 子阶段（深度、可验收）

| 子阶段 | 优化范围（全面） | 退出条件 |
|--------|------------------|----------|
| **S6.0 深基线与回归锁** | 冻结场景集（S5 U/P + 重场景 present-only）；锁定 warmup/iters；双轨基线；回归契约 | ✅ `docs/S6_PERF_BASELINE.md` + `TestS6_*` + `tmp/s6_present_baseline.json`；L2 Comp 锁存 |
| **S6.1 帧模型强制** | 默认 API/文档/辅助：idle 帧、局部 invalidation、禁止无意义全清屏；damage 合并策略；HiDPI 物理脏区；多 region 与 union 选择策略 | ✅ `docs/S6_1_FRAME_ENFORCE.md` + `TestS61_*` / `TestPlanFramePresent_*`；`BeginFrame`/`PresentFrameAuto`/`PresentFrameFull` |
| **S6.2 录制/提交 CPU 路径** | 命令缓冲录制分配、pass 合并、多余 `Flush`、同步点、`Queue.Write*` 批次数、encoder 生命周期；减少每帧 Go alloc | ✅ `docs/S6_2_SUBMIT_PATH.md` + `TestS62_*`；Flush 去 deep-copy；SingleGroupFast；SubmitPathStats |
| **S6.3 绘制合并加深** | 在 S4.1 之上：SDF/convex/text/glyphMask/image 跨更长 run 合并；禁止错误跨 clip/blend/scissor 合并；instance/vertex 打包 | ✅ `docs/S6_3_BATCH_DEEPEN.md` + `TestS63_*`；GPUTex multi-quad；text/glyph 组内 coalesce；BatchDrawStats |
| **S6.4 Layer / Backdrop / Filter** | PushLayer 成本、backdrop 快照复用、blur/shadow 中间 RT、滤镜图节点合并；避免每帧重建 | ✅ `docs/S6_4_LAYER_FILTER.md` + `TestS64_*`；pixmapPool；filter 池；ColorMatrix coalesce |
| **S6.5 Text / Shaping / Atlas** | shape 结果缓存、字体run合并、LCD/glyph 上传、atlas 增长策略、滚动复用 | ✅ `docs/S6_5_TEXT_ATLAS.md` + `TestS65_*`；LayoutGlyphs/Shape 缓存；scroll 上传→0 |
| **S6.6 Path / Stroke / Geometry** | tess/stroke 缓存命中率、dash 几何、复杂 polygon、stencil vs convex 选择、AA 成本可控 | ✅ `docs/S6_6_PATH_GEOMETRY.md` + `TestS66_*`；zero-copy tess；dash/convex cache |
| **S6.7 上传 / 资源 / 内存** | 纹理/buffer 池、暂存上传、图像 generation 失效、glyph/image 预算、显存峰值 | ✅ `docs/S6_7_RESOURCES.md` + `TestS67_*`；ImageCache 字节预算/ephemeral；TexturePool stats |
| **S6.8 真窗口 Present 管线** | X11/swapchain：Acquire/Present、vsync、damage present、多帧；与离屏基线对照；减少 CPU-GPU 空等 | ✅ `docs/S6_8_WINDOW_PRESENT.md` + `TestS68_*`；Fifo p50≈11ms；suboptimal reconfig |
| **S6.9 重场景预算与总回归** | 为 U05/kitchen-sink/嵌套 clip-layer 设 **分级预算**（非一律 16.7ms）；全量 L2 + L0–L3；回写 S6 总表 | ✅ `docs/S6_9_HEAVY_BUDGET.md` + `TestS69_*` + `tmp/s6_9_heavy_budget.json`；L2 Comp 分片 201 绿 |
| **S6.10 可选附录** | 与 Skia 同场景绝对 FPS **对照报表**（不阻塞关闭） | 可选 `docs/S6_10_SKIA_FPS_APPENDIX.md` |

#### S6 场景与预算策略

| 级别 | 代表 | 预算策略 |
|------|------|----------|
| **P0 主路径** | U01–U04（S5） | 保持 **p50≤16.7ms**；S6 期间 **禁止回退**（>10% 需修） |
| **P1 高密度 UI** | 长列表、多卡片壳、表单多字段局部更新 | S6.0 定标后设目标（建议 ≤16.7ms 或 ≤8.3ms@120 可选） |
| **P2 重特效** | U05、多层 blend、backdrop+blur | **分级预算**（S6.9）；要求相对 S6.0 显著下降并可解释 |
| **P3 应力** | 嵌套 clip×layer×text 极限 | 只防回归性恶化 + 正确性 |

#### S6 关闭条件（总）

- [x] S6.0 基线 + 回归契约文档落地；场景定义冻结  
- [x] S6.1–S6.9 均有退出文档或书面「无工作量」说明（不得空跳）（S6.1–S6.9 ✅）  
- [x] P0 主路径 U01–U04 present p50 **未回退**且仍 ≤16.7ms（S6.9 测 U01–U04 p50≪16.7）  
- [x] P2 重场景相对 S6.0 达文档目标或签字豁免（U05/H02 ~19–21%↓；H03 ~50%↓）  
- [x] L2 全量 `TestP1_Comp_` 绿（分片 201）；L0/L1/L3 绿；`GPUOps>0` / 无 silent CPU  
- [x] 无靠降语义刷分；能力表与固定像素门禁仍成立  

**S6 关闭后**：推荐进入控件层主线；控件绘制必须享受 S6 帧模型与合并路径，不得旁路。

### M — 内存 / VRAM 生命周期（S4–S6 正确性锁并存）

> 详案：`docs/MEM_LEAK_TEST_PLAN.md`。目标：在 **不降 S4–S6 正确性/语义** 的前提下，堵住 render→webgpu→rwgpu 释放链泄漏，并用进程隔离档自动反复验证。

| 档 | 测试 | 说明 |
|----|------|------|
| T0–T2 | `TestMem_T0_` / `T1_` / `T2_` | CreateClose / RetainedResize / ResetAccelerator |
| T3 | `TestMem_T3_ComplexOffscreen_*` | 复杂离屏 + 尺寸抖动 |
| T4 | `TestMem_T4_WindowComplex_*` | X11 窗口 resize + 复杂动态帧 |
| T5 | `GPUI_MEM_STRESS=1` | 可选长压 |

```bash
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export GPUI_SURFACE_SAMPLE_COUNT=1   # 低 VRAM 主机必开
./scripts/run_mem_leak_tests.sh      # 进程隔离；默认 COUNT=3

# S4–S6 正确性抽样（建议与 mem 一样进程隔离重窗口用例）
go test -count=1 ./render -run 'TestS54_|TestS52_|TestS53_' -timeout 120s
go test -count=1 ./render -run 'TestP1_Comp_(D01|D06|D08|D36|D63|D152)_' -timeout 180s
go test -count=1 ./render -run 'TestS68_WindowPresent_MultiFrameDraw$' -timeout 120s
```

**硬规则**：

1. mem 修复若触及 encoder/queue/device/session/Close，必须重跑上表 L0 + Comp 抽样 + 相关 S6x。  
2. 本机 iGPU 共享内存紧时：**单进程串跑全 S6x+窗口可能 OOM abort**；门禁以进程隔离为准，不把「同进程全绿」当唯一标准。  
3. 控件层（W）后置不阻塞 M；M 也不替代 L2 Comp 全量。

- [x] T0–T4 + 脚本 + 释放链修复（2026-07-15）  
- [x] S4–S6 抽样在隔离下可复跑  


### W — 控件层（类设计系统组件；走 render）

> 入口：`docs/S5_WIDGET_ENTRY.md`（✅）。绘制必须走 `render.Context`；享受 S6 帧模型/合并，不得旁路。

| 子阶段 | 范围 | 退出 |
|--------|------|------|
| **W0 脚手架 + 第一批绘制** | `widget` 包；Button/Input/Modal/ListRow/TableCell；Theme；CPU+GPU 门禁 | ✅ `docs/W0_WIDGET_LAYER.md` + `TestW0_*` |
| **W1 状态与命中** | pressed/hover/focus/disabled 矩阵；统一 HitTest；焦点环 | `docs/W1_STATE_HIT.md` + `TestW1_*` |
| **W2 组合壳** | Form / List / Table 场景用控件拼装（非 antd 全量） | `docs/W2_COMPOSITION_SHELL.md` + present 软预算 |
| **W3 交互最小环** | 可选：指针/键盘路由草图（仍可不绑平台窗体） | 后置 |

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=/home/yanghy/app/projects/gogpu/gpui/lib:$LD_LIBRARY_PATH
go test -count=1 ./widget -timeout 120s
```


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
| 11 | **S5 UI 引擎硬化** | ✅ **S5.0–S5.5 全线关闭**（见 `docs/S5_*.md`） |
| 12 | **S6 生产级深度性能** | ✅ **S6.0–S6.9 关闭**（可选 S6.10 Skia FPS 附录不阻塞） |
| 13 | **控件层 W*** | 🔄 **W0 ✅** → 当前 **W1 状态/命中契约**（`docs/W0_WIDGET_LAYER.md`） |

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
**A / S4 / S5 已关闭。**  
**S6.0–S6.9 已关闭**（深基线 + 帧/提交/batch/layer/text/path/资源/窗口 + 重场景分级预算）。  
**S6.0–S6.9 已关闭**。**M 内存生命周期档已落地**（`docs/MEM_LEAK_TEST_PLAN.md`）。用户当前侧重 **内存/VRAM 泄漏验证与释放链**；控件层 W 仍后置。可选 S6.10 Skia FPS 附录仍不阻塞。

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
| `docs/S5_SKIA_UI_GAP.md` | S5.0 产出（✅ 控件向 Skia 对账） |
| `docs/S5_PRESENT_BASELINE.md` | S5.1 产出（✅ present-only 基线） |
| `docs/S5_FRAME_MODEL.md` | S5.2 产出（✅ 帧模型） |
| `docs/S5_60FPS_GATE.md` | S5.3 产出（✅ 60fps 门禁） |
| `docs/S5_WIDGET_ENTRY.md` | S5.5 产出（✅ 控件入口；**S5 关闭**） |
| `docs/S6_PERF_BASELINE.md` | S6.0 产出（✅ 深基线+回归锁+JSON） |
| `docs/S6_1_FRAME_ENFORCE.md` | S6.1 产出（✅ 帧模型强制） |
| `docs/S6_2_SUBMIT_PATH.md` | S6.2 产出（✅ 录制/提交 CPU 路径） |
| `docs/S6_3_BATCH_DEEPEN.md` | S6.3 产出（✅ 绘制合并加深） |
| `docs/S6_4_LAYER_FILTER.md` | S6.4 产出（✅ Layer/Backdrop/Filter） |
| `docs/S6_5_TEXT_ATLAS.md` | S6.5 产出（✅ Text/Shaping/Atlas） |
| `docs/S6_6_PATH_GEOMETRY.md` | S6.6 产出（✅ Path/Stroke/Geometry） |
| `docs/S6_7_RESOURCES.md` | S6.7 产出（✅ 上传/资源/内存） |
| `docs/S6_8_WINDOW_PRESENT.md` | S6.8 产出（✅ 真窗口 Present） |
| `docs/S6_9_HEAVY_BUDGET.md` | S6.9 产出（✅ 重场景分级预算 + TestS69 + JSON） |
| `docs/S5_4_CAPABILITY_PATCH.md` | S5.4 产出（✅ 无阻塞补丁队列） |
| `docs/OPTIMIZATION_PLAN.md` | 历史大计划；服从主线 |

---

## 6. 修订记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-16 | — | GPU-first §4 关闭：非凸 bootstrap reason + S5/S6/mem_anim S12 cpu_fb=0；可选升原生/控件层 |
| 2026-07-16 | — | residual G.02 对角/径向 field + G.04 brush:custom reason；下一步 soak cpu_fb=0 / 非凸 |
| 2026-07-16 | — | P1-3 DONE：layer Pop 少 mid-flush + FrameFlushes + damage 门禁；下一步 residual G.02/G.04 |
| 2026-07-16 | — | P1-1 CJK shape 缓存门禁；G.02 H/V linear ExtendRepeat 1D ramp；下一步 P1-3 / residual |
| 2026-07-16 | — | P0-4 L.04 DONE：standalone filter Apply* GPU multi-RT + Gaussian；下一步 residual CPU† / clip |
| 2026-07-15 | 1.66 | **M 内存泄漏套件**：T0–T4 + `scripts/run_mem_leak_tests.sh`；修 encoder/pass/queue/adapter/cmdbuf 释放与 session pipeline 所有权；lazy Vello；与 S4–S6 正确性门禁进程隔离并存；`docs/MEM_LEAK_TEST_PLAN.md` |
| 2026-07-15 | 1.65 | **W0 关闭**：`widget` 包 Button/Input/Modal/ListRow/TableCell + Theme；CPU/GPU present 门禁；`docs/W0_WIDGET_LAYER.md`；焦点 → W1 |
| 2026-07-15 | 1.64 | **S6.9 关闭**：分级预算 P0/P1/P2/P3；`TestS69_*` + `tmp/s6_9_heavy_budget.json`；P2 相对 S6.0 可解释下降；L2 Comp 分片 201 绿；S6 总关闭条件齐；焦点 → 可选 S6.10 / 控件层入口 |
| 2026-07-15 | 1.63 | **S6.8 关闭**：Swapchain stats/reconfigure/Fifo；X11 multi-frame PresentFrameAuto p50≈11ms；`docs/S6_8_WINDOW_PRESENT.md`；焦点 → S6.9 |
| 2026-07-15 | 1.62 | **S6.7 关闭**：ImageCache 字节预算/ephemeral/staging；TexturePool stats+budget；`docs/S6_7_RESOURCES.md`；焦点 → S6.8 |
| 2026-07-15 | 1.61 | **S6.6 关闭**：tess 零拷贝、stroke 共享 path、dash/convex cache；`docs/S6_6_PATH_GEOMETRY.md`；焦点 → S6.7 |
| 2026-07-15 | 1.60 | **S6.5 关闭**：LayoutGlyphs/Shape 结果缓存、MultiFace Runs 缓存、scroll/LCD 上传收敛；`docs/S6_5_TEXT_ATLAS.md`；焦点 → S6.6 |
| 2026-07-15 | 1.59 | **S6.4 关闭**：layer/filter pixmap 池、backdrop 去 Clear、ColorMatrix 合并；`docs/S6_4_LAYER_FILTER.md`；焦点 → S6.5 |
| 2026-07-15 | 1.58 | **S6.3 关闭**：GPUTex multi-quad+seal、text/glyph 组内 coalesce、BatchDrawStats；`docs/S6_3_BATCH_DEEPEN.md`；焦点 → S6.4 |
| 2026-07-15 | 1.57 | **S6.2 关闭**：Flush 去 deep-copy、SingleGroupFast/scratch、uniform/clip BytesInto、SubmitPathStats；`docs/S6_2_SUBMIT_PATH.md`；焦点 → S6.3 |
| 2026-07-15 | 1.56 | **S6.1 关闭**：`BeginFrame`/`Invalidate`/`PresentFrameAuto`/`PresentFrameFull` + damage multi/union/full 策略；`docs/S6_1_FRAME_ENFORCE.md`；焦点 → S6.2 |
| 2026-07-15 | 1.55 | **S6.0 关闭**：13 场景 present-only 深基线 + 回归契约 + `TestS6_*`；L2 Comp 分片全绿；焦点 → S6.1 |
| 2026-07-15 | 1.54 | **写入 S6 生产级深度性能**：S6.0–S6.10 定义；回归 L0–L4 契约；当前焦点 S6.0；控件层推荐 S6 后 |
| 2026-07-15 | 1.53 | **S5.x 全线关闭**：S5.0–S5.5 文档+TestS5*/S52/S53/S54；U01–U04 present-only p50≤16.7ms；控件入口冻结 |
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

### mem_anim 质量门禁

见 `docs/MEM_ANIM_LONGSOAK_PLAN.md`（**v2.6：S01–S21**，S15–S21 为 Skia 缺口扩展）「质量门禁」与 **§0c / §9 目标审计**：

- **权威结果**：v9 S01–S12 fail=0；v10_regress S11/S12；**v11 S12@180 / S13@90(真密度) / S14@180 fail=0** → `docs/MEM_ANIM_LONGSOAK_PLAN.md` §10
- **60fps+**（ema≥55 / avg≥48，目标 60）
- **任何渲染内容都要丝滑**（Q-SILKY：稳态 work ≤16.7ms）
- **不能出现闪烁**（Q-NOFLICKER：真实效果每帧持续可见，禁止稀疏一帧真 API）
- **cpu_fb=0**，真 present 链 render→webgpu→rwgpu
- **对标 Skia**：重效果用有界 offscreen/saveLayer 等价，禁止假视觉刷分
- 关键：`doStroke` GPU 批处理；滤镜/层/Backdrop/高级混合小离屏真实 API + `ExportImageBuf`/`MarkEphemeral`
