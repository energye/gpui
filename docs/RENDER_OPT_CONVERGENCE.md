# Render 层代码优化收敛计划（功能不变）

> 版本：1.8 | 日期：2026-07-18  
> 状态：**R7 收口**；**R8 执行中** → [`PERF_CPU_FORWARD_OPT_PLAN.md`](./PERF_CPU_FORWARD_OPT_PLAN.md)  
> 范围：`render` 主路径 + 为 render 服务的 `gpu/webgpu` / `gpu/rwgpu` 热路径  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> 上位主线：[`MAINLINE_PLAN.md`](./MAINLINE_PLAN.md) · 路由铁律：[`GPU_FIRST_ROUTING.md`](./GPU_FIRST_ROUTING.md)

---

## 0. 一句话目标

在 **S4–S6 已关闭** 的基础上，对渲染层做 **可收敛的等价性能优化**：  
**像素语义、GPU 优先路由、公共 API/ABI、fallback 可观测性、资源生命周期均不得回退**；  
每一刀必须可证明「更快或更省，且功能不变」。

---

## 1. 背景与边界

### 1.1 已完成底座（禁止重做/禁止降级）

| 阶段 | 内容 | 文档 |
|------|------|------|
| S4.0–S4.4 | 基线、batch、glyph atlas、path/image cache、damage | `docs/S4_*.md` |
| S5.0–S5.5 | present / frame / 60fps 门禁 / 控件入口冻结 | `docs/S5_*.md` |
| S6.0–S6.9 | submit、batch deepen、layer/filter、text atlas、path geometry、resources、window、heavy budget | `docs/S6_*.md` |
| GPU_FIRST | 主路径 GPU 默认；silent CPU 禁止 | `docs/GPU_FIRST_ROUTING.md` v3.9.1 **已关闭** |

### 1.2 本轮范围

**做：**

1. `render` / `render/internal/gpu` 热路径：分配、拷贝、冗余同步、batch/cache 命中、submit 录制成本  
2. `gpu/webgpu`：descriptor 转换、Submit/Write*/BeginRenderPass 等 **render 每帧必经** 的 facade 成本  
3. `gpu/rwgpu`：与上列 facade 对应的 FFI 装箱/临时切片（仅热路径等价变换）  
4. 诊断与门禁：stats、baseline 对比、回归脚本

**不做（本轮明确排除）：**

- 控件层实现  
- 为刷分关 AA / 降默认像素语义 / 删组合断言  
- silent CPU 或把已 GPU 路径改回 CPU  
- N3 CustomBrush 任意 Func fragment / N4 bicubic / N5 极冷门 path effect（书面后置，见 GPU_FIRST）  
- 无界 ABI「绑满整个 WebGPU 规范」  
- 大改 swapchain 协议、破坏 present 语义

### 1.3 与历史文档关系

- [`OPTIMIZATION_PLAN.md`](./OPTIMIZATION_PLAN.md) = 历史档案  
- 任务优先级：**本文 + MAINLINE + GPU_FIRST**  
- S4/S5/S6 切片文档 = 已关闭基线与回归锚点；本轮编号 **R7.x** 表示 post-S6 收敛优化

---

## 2. 硬原则（任何切片违反即不得合入）

### P1 — 功能不变（不变量）

| 不变量 | 含义 | 验收 |
|--------|------|------|
| 像素/语义 | fill/stroke/text/image/clip/blend/mask/layer/AA 行为与优化前一致，或仅落在已登记可接受 fringe 内 | `TestP1_Comp_*` 抽样→全量；相关视觉 STRICT |
| GPU 优先 | 有加速器时主路径 `GPUOps>0` 且 `cpu_fallback_ops=0`（书面例外除外） | `RenderPathStats` + 既有 GPU 门禁 |
| Fallback 可观测 | 不得 silent CPU；reason 标签不得消失 | `LastCPUFallbackReason` |
| API 面 | 公共 `render` API 签名/语义不变 | 编译 + 调用方测试 |
| ABI/生命周期 | webgpu/rwgpu 指针、enum、Release 时机不漂 | ABI/facade 测试 + 真 native 加载 |
| 资源安全 | 无新增泄漏；ephemeral 仍帧末释放；iGPU OOM 不恶化 | mem leak / full unit 策略 |
| Present 不回退 | U01–U04 present-only p50 不因本刀恶化 >10% 且仍 ≤16.7ms | `TestS6_*` / S5/S6 baseline |

### P2 — 等价优化优先

变更必须可归入一类：

| 类 | 定义 | 允许 | 门禁强度 |
|----|------|------|----------|
| **A. 纯等价** | 同输入同像素；仅少 alloc/少拷贝/少 bind/draw | 默认推进 | 窄测 + 抽样组合 + present 烟测 |
| **B. 算法等价** | 不同几何/coverage 实现，目标像素一致 | 需 golden/像素门 | A + 像素 STRICT + 相关矩阵 |
| **C. 语义升级** | 新能力或故意改默认 | **禁止混进 R7 纯优化** | 走能力表主线 |

### P3 — 小步收敛

1. **一刀一路径**（例如：Submit 装箱 / dual-tex uniform scratch / glow 常驻 RT）  
2. 每刀：**改动 → 对口门禁 → 对比基线 → 保留或回滚**  
3. 无收益或正确性风险 → **回滚优先**，不硬扛  
4. 合入前写清：收益、风险面、证据路径

### P4 — 环境可比

默认复现环境（与 `tmp/full_unit/STATUS.md` 对齐）：

```bash
export GOROOT=.../go1.25.5.linux-amd64   # 或本机等价 1.25.x
export PATH=$GOROOT/bin:$PATH
export GOCACHE=$PWD/tmp/go-cache GOWORK=off GOTOOLCHAIN=local
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}
export DISPLAY=:1
export GPUI_SURFACE_SAMPLE_COUNT=1
```

性能对比必须：**同机、同 env、同 warmup/iters、同场景名**；改场景则 bump baseline version。

### P5 — 分层不打穿

- `render` **不**直接依赖 `gpu/rwgpu`  
- facade 优化放在 `gpu/webgpu`；FFI 装箱放在 `gpu/rwgpu`  
- 禁止为省一层而把 native 细节泄漏进 render 公共 API

---

## 3. 收敛流程（每刀 checklist）

```text
[1] 选刀     热路径 + 类 A/B + 禁止改动列表
[2] 冻基线   相关测试绿；记 p50 / gpu_ops / cpu_fb / alloc 若适用
[3] 最小 diff 只碰目标路径；不顺手重构无关模块
[4] 窄测     包内 TestR70_* / 相关 S6x / 单测
[5] 回归锁   L0 present + L1 组合抽样；必要时 L2 全量 / mem
[6] 结论     收益表 + 证据路径；失败回滚
[7] 文档回写 本文 §6 状态表 + 切片小节（可选 docs/R7_*.md）
```

### 3.1 必跑门禁分级

| 级 | 何时 | 命令（示意） |
|----|------|----------------|
| **L0 切片** | 每刀必跑 | 本刀 `TestR7x_*` + 触及包的既有 `TestS6*` / facade smoke |
| **L1 主路径** | 每刀必跑 | `go test ./render -run 'TestS6_L0_|TestS61_|TestS62_|TestS63_Present'` |
| **L1b 组合抽样** | 每刀必跑 | `TestP1_Comp_(D01|D06|D08|D36|D63|D152)_` |
| **L2 组合全量** | 阶段关闭前 / 高风险 B 类 | 全量 `TestP1_Comp_`（可用 full_unit_fast 批隔离） |
| **L3 mem** | 资源/池/生命周期改动 | `scripts/run_mem_leak_tests.sh` |
| **L4 问题墙** | filter/layer/glow 相关 | `scripts/run_problem_suite.sh` 或 PKS 子集 |

**失败策略：**

- 正确性红 → 切片不得关闭，优先回滚  
- present p50 恶化 >10% 且无说明 → 修或书面豁免  
- full_unit 批次 iGPU OOM：solo 全绿可计有效绿（沿用现网策略）

---

## 4. 现状热点与机会（2026-07-17）

### 4.1 已较强的路径

- Flush 无 pending deep-copy；单 scissor group fast path（S6.2）  
- Image/GPUTex multi-quad + text/glyph group coalesce（S4.1/S6.3）  
- shape/layout cache + atlas partial upload（S6.5）  
- path/stroke/dash/convex geometry cache（S6.6）  
- ImageCache / TexturePool / staging pool（S6.7）

### 4.2 仍可收敛（按优先级）

| ID | 层 | 问题 | 类 | 风险 |
|----|----|------|----|------|
| **R7.0** | webgpu + rwgpu | 每帧 `Submit` / `CreateBindGroup` / `BeginRenderPass` / `WriteTexture` 反复 `make` 转换切片 | A | 低 |
| **R7.1** | render/gpu | dual-tex / advanced blend 热路径 per-op `make([]byte)`、临时 view | A | 低–中 |
| **R7.2** | render/gpu | filter/glow：**ExportImageBuf / 隔帧 readback** 尖刺（PKS P_GRAD_RT / P_BLEND_GLOW） | A/B | 中（碰 present 语义时升 B） |
| **R7.3** | render/gpu | 多 encoder/submit 合并（同帧可合并的独立 pass） | A/B | 中 |
| **R7.4** | render | damage/retained 在复杂 clip×filter 下过宽脏区 | B | 中 |
| **R7.5** | render | 文本/布局缓存命中与滚动场景进一步降 reshape（不改字形像素） | A | 低 |
| **R7.6** | webgpu | BindGroup/Pipeline 描述转换与小对象池（仅 render 已证实热） | A | 低 |

**已知产品压力信号（优化输入，不是刷分借口）：**

- `examples/particle_kitchen_sink/PROBLEMS_LATEST.md`：glow/filter export hitch、combo UI fps、fps jitter  
- 修复方向：**减 readback / 常驻 RT / 合并 submit**，禁止靠降内容过门禁

---

## 5. 切片计划

### R7.0 — Facade/FFI 热路径零临时分配（本轮首刀）

**目标：** 不改变任何 GPU 命令语义，去掉 render 每帧必经的装箱 alloc。

| 位置 | 改动 |
|------|------|
| `gpu/webgpu/queue.go` | `Submit`：1-CB 快路径；≤8 CB 栈上缓冲 |
| `gpu/webgpu/queue.go` | `WriteTexture`：栈上 `ImageCopyTexture`/`Layout`/`Extent`，避免每调 3 次堆描述符 |
| `gpu/webgpu/device.go` | `CreateBindGroup` / `CreateBindGroupLayout` / `CreatePipelineLayout`：≤8 entries 栈上转换 |
| `gpu/webgpu/encoder.go` | `BeginRenderPass`：本地栈转换 color/depth attachment（避免 convert 返回悬挂栈切片） |
| `gpu/rwgpu/bindgroup.go` | `CreateBindGroup` wire entries 小 N 栈上 |
| `gpu/rwgpu/render.go` | `BeginRenderPass` native color attachments 小 N 栈上 |
| `render/internal/gpu/dual_tex_blend.go` | dual-tex into-dest：循环外复用 256B uniform slot scratch |

**禁止：** 改 load/store op、改 blend 公式、改 submit 顺序语义、缓存已 Release 的 native 句柄。

**验收：**

```bash
go test -count=1 ./gpu/webgpu ./gpu/rwgpu -run 'TestR70_|TestS2_|TestABI|TestInit' -timeout 180s
go test -count=1 ./render/internal/gpu -run 'TestR70_|TestS62_|TestS63_|TestS67_|TestP03_|TestP04_' -timeout 300s
go test -count=1 ./render -run 'TestS6_L0_|TestS61_|TestS62_Present|TestS63_Present|TestP1_Comp_(D01|D06|D08|D36|D63|D152)_|TestF1_|TestP03_|TestP04_' -timeout 600s
```

**退出条件：**

- [x] 上述 L0/L1/L1b 绿  
- [x] `cpu_fb=0` 的既有 GPU 断言不回退  
- [x] 无新 leak 信号（资源路径未改生命周期则 L3 可选；本刀未改 Release 时序）  
- [x] 本文 §6 回写  

**R7.0 关闭（2026-07-17）。** 下一：**R7.1 dual-tex / layer scratch 深化**。

### R7.1 — dual-tex / layer 热路径 scratch 深化 ✅

| 位置 | 改动 |
|------|------|
| `dual_tex_blend.go` | `dualTexWriteParams` 48B `sync.Pool`；upload row-pitch 走 `imageStagingPool` |
| `dual_tex_blend.go` | `dualTexAdvancedBlendTexturesRegionSized` / into-dest 走 `viewForBGRATexture` 全视图缓存 |
| `layer_gpu_io.go` | `UploadRGBAToView` BGRA+pad 走 staging 池 |
| `glyph_mask_pipeline.go` + `render_session.go` | LCD uniform `Into` + session scratch 复用 |

**验收（2026-07-17）：** F1/P03/P04 `cpu_fb=0`；S5.3/S6 L0 present 绿；D01/D06/D08/D36/D63/D152 绿；`TestR71_*` 0-alloc 热测绿。证据：`tmp/r7_1_STATUS.md`。

**R7.1 关闭。** 下一：**R7.2 filter/glow 去 readback 尖刺**。

### R7.2 — filter/glow 去 readback 尖刺 ✅

| 位置 | 改动 |
|------|------|
| `filter_ops.go` `tryApplyFilterGraphGPU` | FromView 成功 flush 后禁止用 stale pixmap 做 path2 种子；失败时从 filterSrcRT 正确 readback 恢复 |
| `filter_ops.go` | `materializeFilterGPUTo`：一次 Map 写 primary(+optional secondary) |
| `context_image.go` `ExportImageBuf` | 无 pending 不强制 FlushGPU；filter stale 时一次 materialize→ImageBuf+pixmap |
| `r7_2_filter_export_test.go` | GPUFilterTexture + Export 行为门禁 |

**验收（2026-07-17）：** TestR72_*、P04、S3c ApplyBlur、F1、S5.3/S6 L0、D01/D06/D08/D36 绿；`cpu_fb=0`。证据：`tmp/r7_2_STATUS.md`。

**R7.2 关闭。**

### R7.3 — 同帧 submit 合并（dual-tex multi + blit）✅

| 位置 | 改动 |
|------|------|
| `dual_tex_blend.go` | `dualTexAdvancedBlendViewsMultiBundle`：可延后 Submit；静态 encoder desc |
| `render_session.go` | `EnqueueLeadingSubmit` / `submitWithLeading`；`SubmitPathStats.CoalescedCBs` |
| `gpu_render_context.go` | advanced resolve 延后 dual-tex multi，与后续 blit Flush 合并 Submit |
| `r7_3_submit_coalesce_test.go` | deferred + 2-CB coalesce 门禁 |

**验收：** TestR73_*；F1/P03 `cpu_fb=0`；S5.3/S6 L0 present；D01/D06/D08/D36 绿。证据：`tmp/r7_3_STATUS.md`。

**R7.3 关闭。** 下一可选：R7.4 damage 过宽 / PKS glow 实测。

### R7.4 — damage 过宽（group-relevant multi-rect scissor）✅

| 位置 | 改动 |
|------|------|
| `damage_scissor.go` | `damageRectsRelevantToGroup`：只 union 与 group 相交的 damage |
| `render_session.go` | `applyGroupScissorWithDamageRects`；overlay/group 4 路径改 per-group relevant |
| `context.go` | trackDamage 溢出改 `CoalesceDamageRects`（touch-merge 优先） |
| tests | `TestDamageRectsRelevantToGroup` / `TestR74_*` |

**语义：** 无 damage → 全 group；有 damage 无交 → skip；有交 → scissor ⊆ 旧 global AABB∩group。Base multi-rect blit 不变。

**验收（2026-07-17）：** TestR74_* / damage scissor unit；S5.3/S6 L0 present；S61；D01/D06/D08/D36/D63/D152；F1/P03/P04 绿。证据：`tmp/r7_4_STATUS.md`。

**R7.4 关闭。** 下一可选：R7.5 文本 reshape / PKS glow 实测。

### R7.5 — 文本 layout template + scroll rebase ✅

| 位置 | 改动 |
|------|------|
| `glyph_mask_engine.go` | 原点无关 layout template；color 出 key |
| 同上 | 安全 quad rebase（整设备像素 / full-hint snap） |
| 同上 | `LayoutTextAliased` 接入 template |
| `r7_5_layout_template_test.go` | scroll/color/equality/aliased 门禁 |

**语义：** 不改字形像素；不安全位移 miss 回全量 layout。Shape cache（S6.5）保留。

**验收（2026-07-17）：** TestR75_*、S65、S5.3/S6 L0、S61、D01/D06/D08/D36/D63/D76/D152、F1/P03/P04 绿。证据：`tmp/r7_5_STATUS.md`。

**R7.5 关闭。** 下一可选：R7.6 webgpu 小对象池 / PKS glow 实测。

### R7.6 — webgpu/rwgpu 描述转换池与小 N 栈 ✅

| 位置 | 改动 |
|------|------|
| `gpu/webgpu/device.go` | pipeline convert `sync.Pool` scratch；CreateRenderPipeline 走 Into |
| `gpu/webgpu/encoder.go` | CopyTexture* ≤4 region 栈 |
| `gpu/rwgpu/*` | BindGroupLayout / PipelineLayout / RenderPipeline wire 小 N 栈 |
| `r7_6_pipeline_convert_test.go` | convert 门禁 |

**验收（2026-07-17）：** TestR76_*、R70、rwgpu S1、render L0/F1/P03/D01 等绿。证据：`tmp/r7_6_STATUS.md`。

**R7.6 关闭。** **§4.2 R7.0–R7.6 全部关闭。**

### R7 之后 — 新热点再开 R8 / PKS 实测

**执行案（2026-07-18）：** 见 [`PERF_CPU_FORWARD_OPT_PLAN.md`](./PERF_CPU_FORWARD_OPT_PLAN.md)（稳 60 + CPU 正向；R8.0 baseline → pprof 选刀）。

---

## 6. 执行状态

| 切片 | 状态 | 证据 | 备注 |
|------|------|------|------|
| R7.0 Facade/FFI + dual-tex slot | **✅ 关闭** | `tmp/r7_0_STATUS.md` + 下方验收日志 | 纯 A 类；功能门禁绿 |
| R7.1 dual-tex/layer scratch | **✅ 关闭** | `tmp/r7_1_STATUS.md` | 纯 A 类 |
| R7.2 filter/glow 去 readback | **✅ 关闭** | `tmp/r7_2_STATUS.md` | A/B：正确性修复 + export 路径 |
| R7.3 同帧 submit 合并 | **✅ 关闭** | `tmp/r7_3_STATUS.md` | dual-tex multi + blit 一次 Queue.Submit |
| R7.4 damage 过宽 | **✅ 关闭** | `tmp/r7_4_STATUS.md` | group-relevant scissor + trackDamage coalesce |
| R7.5 文本 layout template | **✅ 关闭** | `tmp/r7_5_STATUS.md` | scroll rebase + color-independent |
| R7.6 webgpu 描述转换池 | **✅ 关闭** | `tmp/r7_6_STATUS.md` | pipeline pool + 小 N 栈 |
| R8 / PKS | 可选 | 新热点或 glow hitch 实测 | — |

---

## 7. 回滚条件（预设）

出现任一条立即回滚该刀：

1. 像素/组合门禁失败且非环境噪声  
2. 有 GPU 场景新增 `cpu_fallback_ops>0` 或 silent 路径  
3. present 主路径 p50 恶化 >10% 无合理解释  
4. ABI/生命周期 UAF、double-free、新增 mem leak  
5. full_unit 逻辑失败（solo 亦失败）

---

## 8. 文档与代码索引

| 用途 | 路径 |
|------|------|
| 本计划 | `docs/RENDER_OPT_CONVERGENCE.md` |
| 主线 | `docs/MAINLINE_PLAN.md` |
| GPU 优先 | `docs/GPU_FIRST_ROUTING.md` |
| 性能基线 JSON | `tmp/s4_baseline.json` / `tmp/s5_present_baseline.json` / `tmp/s6_present_baseline.json` |
| Full unit 状态 | `tmp/full_unit/STATUS.md` |
| 问题墙 | `examples/particle_kitchen_sink/PROBLEMS_LATEST.md` |
| 全量脚本 | `scripts/run_full_unit_tests.sh` |
| Mem | `scripts/run_mem_leak_tests.sh` |

---

## 9. 变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-07-17 | 首版：post-S6 渲染层优化收敛；启动 R7.0 |
| 1.1 | 2026-07-17 | R7.0 关闭：webgpu/rwgpu 热路径栈上装箱 + dual-tex uniform slot 复用；门禁见 `tmp/r7_0_STATUS.md` |
| 1.2 | 2026-07-17 | R7.1 关闭：dual-tex params/view/upload scratch + LCD uniform Into + layer upload pool；见 `tmp/r7_1_STATUS.md` |
| 1.3 | 2026-07-17 | R7.2 关闭：filter FromView 正确恢复 + Export 单次 materialize；见 `tmp/r7_2_STATUS.md` |
| 1.4 | 2026-07-17 | R7.3 关闭：dual-tex multi + blit 同次 Queue.Submit；见 `tmp/r7_3_STATUS.md` |
| 1.5 | 2026-07-17 | R7.4 关闭：group-relevant multi-rect damage scissor + trackDamage CoalesceDamageRects；见 `tmp/r7_4_STATUS.md` |
| 1.6 | 2026-07-17 | R7.5 关闭：glyph layout template + scroll-safe rebase；见 `tmp/r7_5_STATUS.md` |
| 1.7 | 2026-07-17 | R7.6 关闭：pipeline convert pool + rwgpu/webgpu 小 N 栈；R7 计划表收口；见 `tmp/r7_6_STATUS.md` |
