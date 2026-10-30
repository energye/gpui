# P2.4b 影响面评估（开工前，2026-10-30）

> 性质：只评估，不动工作区（本文落盘 + 台账登记除外）。
> 依据：台账 §2.1 P2.4b（预算统一：六口径 → 一套 Budget，按域分账；淘汰语义有单测）、PLAN §2.2-B6、PLAN §7.3；盘点来源：两个只读探查（预算点全扫 + descriptor/res 面扫）+ `03_overlap_pools_retry.md` 审计对照。
> 结论一句话：**审计六口径里 MemoryManager 已随 P2.3 删净（零残留），剩余口径 + 深扫发现的 GPU 侧预算点共 10 处声明点，全部收敛到 `render` 包一套 `Budget` 声明表（数字逐个等于现状、淘汰算法不动=零行为）；唯一语义变化是 P2.4a D3 推迟的 descriptor 口径统一；公平份额两套算术合一；hal 全局账本与双端记账不动。**

## 0 预算点清单（10 处声明点，file:line 为盘点实测）

| # | 域 | 现值 | 位置 | 配置入口 | 淘汰语义 | 现有单测 |
|---|---|---|---|---|---|---|
| 1 | image | 128 条 + 64MiB；多窗下限 16 条/8MiB | `render/internal/gpu/image_cache.go:27/30/34-37` | `SetBudgets(entries,bytes)` | LRU 双预算 | TestS67_ImageCache_ByteBudgetEvicts；**多窗公平份额分支无测** |
| 2 | picture-cache | 公平份额：divisor=4、min 64 条 | `render/shared_budget.go:88/92`（消费方 `ui/scene/textured.go:356 SetBudget`） | `SetBudget(n)`（0=自动） | LRU+帧工作集 | TestPictureCacheFairMaxN_*；TestTextureBudget_* |
| 3 | texture-pool | 128MB | `render/internal/gpu/texture_pool.go:21` | `SetBudget(mb)`（`GPUShared.SetTexturePoolBudget`） | EndFrame 逐出（非 LRU） | TestS67_TexturePool_EndFrameBudget |
| 4 | stencil-scratch | 8 尺寸桶 | `render/internal/gpu/gpu_textures.go:126` | 无（硬编码） | LRU(stamp)+retireFn | 仅 prune 3 测；**cap=8 LRU 驱逐无测** |
| 5 | clip-mask | 24 条 + 16MiB + TTL 5s | `render/internal/gpu/depth_clip.go:186/189/190` | 无（硬编码） | LRU+TTL，末条不逐 | **全无**（测试代码零引用 maskCache） |
| 6 | blend-dualtex | 32MiB + 每桶 8 条 | `render/internal/gpu/dual_tex_blend.go:1168/1157` | 无（硬编码） | 超预算丢最大桶尾条 | **无** |
| 7 | layer-offscreen | 每桶 4 + 32MiB | `render/internal/gpu/gpu_render_context.go:169/173` | 无（硬编码） | 双闸入池 | 待核（本轮补） |
| 8 | glyph-color | 4 页 ×1024² | `render/internal/gpu/glyph_color_engine.go:48` | 构造参数 | 页级最冷驱逐 | TestColorGlyphAtlasEvictsCold |
| 9 | glyph-mask | 16384 条 + 4 页 | `render/text/glyph_mask_atlas.go:215-222`（消费 `glyph_mask_engine.go:112`） | Default 构造 | 条级 LRU+32 帧页回收 | TestGlyphMaskAtlas_LRUEviction |
| 10 | video-stage | 8 槽 + 64MiB 单次 staging | `render/video_direct.go:36`、`render/video_texpool.go:53-58/33-34` | 无（硬编码） | 满+idle 逐最旧 | **无 maxSlots 驱逐专测** |
| — | process-global | 768MB（`GPUI_VRAM_BUDGET_MB`，0=禁用） | `gpu/hal/vram_ledger.go:52-59`（唯一全局，注释 "one budget, no second constant"） | env | fail-fast 拒绝（VramCheck:115） | TestVramLedgerBudgetGate 等 4 测 |

配套口径（声明表登记但行为不动）：`DeviceDescriptorForAdapter` 水位联动（`adapter_policy.go:236-275`，fraction 0.8、`GPUI_LOW_VRAM*` env）；`GPUI_VRAM_BUDGET_PCT`（rwgpu 设备创建百分比，`gpu/rwgpu/instance.go:218`，独立口径登记不合并）。CPU 侧池（pixmapPool 96MiB、internal/cache、internal/image Pool、路径几何四缓存、TilePool 4096、brush_field_cache 8）审计判定不与 GPU 预算重叠 → **不入表，归 P2.5**。

**MemoryManager 结论**：全仓 grep 零匹配（P2.3 已删 `memory.go`）；审计文档 `03_overlap_pools_retry.md:37` 的 "256MB+80%" 行是过时快照，PLAN §2.2-B6 的「删除 MemoryManager 预算逻辑」已由 P2.3 完成，本步无需再删。

## 1 收敛设计（零行为）

1. **一套 Budget**：新增 `render/budget.go`——`BudgetDomain` 枚举（上表 10 域 + process-global）+ `Budget{MaxEntries int, MaxBytes uint64, TTL time.Duration}` + `DefaultBudget(d)` 唯一声明表（数字=上表逐个相等）。
2. **按域分账（声明级）**：每域在表内登记上限三元组；运行时用量聚合（各缓存 live 读数注册汇总）**推迟到 P2.4c/d**（res/registry 归一时各缓存本来要归一，届时顺手挂读数）——hal 全局账本与 rwgpu/gles 双端记账完全不动。
3. **公平份额算术合一**：`image_cache.go effBudget/effBudgetBytes` 与 `shared_budget.go pictureCacheFairMaxN` 共用一份派生核心（windows 均分 + 下限），输出数字不变（pin 单测锁定）。
4. **descriptor 口径统一（本步唯一语义变化，P2.4a D3 推迟决策）**：`render/device_open.go OpenStandaloneDevice` 从 plain `DeviceDescriptor` 改 `DeviceDescriptorForAdapter(label, adpt)`。本机（独显 940MX、水位未超）实测行为不变；混合显卡机上 standalone 探针/compute 跟 present 一样拿核显/水位 LowVRAM 紧限。测试装箱（native_device_test、conformance）保持 plain 口径（独立语义，不改）。
5. **render/text 循环规避**：GlyphMaskAtlas 默认参数由调用方（`glyph_mask_engine.go`，internal/gpu 可 import render）从表传入；`render/text` 包不新增 import。
6. **TexturePool 只入表**：池命运（删分配路/保留销毁路，PLAN §2.3 单点死代码）与同尺寸纹理六处收敛同属 P2.4d，本步不删。

## 2 等价性证明路径（§4.1：重构前先补单测并跑绿）

- 先写 `render/budget_default_test.go` pin 表（对现状常量跑绿）→ 重构切表 → 同测试锁等价。
- 补缺失淘汰单测 6 组（上表加粗项：clip-mask 三口径、dualTex、stencil LRU、video maxSlots、glyph 上传节流、ImageCache 公平份额分支）——对现状行为先绿，重构后同测试锁。
- 沿用等价面：ImageCache 字节 LRU、pixmapPool、res.Cache、两图集 LRU、水位线、PictureCacheFairMax、账本闸门。

## 3 门禁设计

同 P2.4a 口径（五目标构建、vet 61/58、逐测试文件带 lib、API 快照+总账、残留 grep、负向对照、-race）+ 本步专项：conformance 重跑（K1 上界 ±2/131072B 不回退）、vram ladder native 1 轮（预算专项窗，峰值对账 P1.6 的 372MB 档）、ui_l1_blank 双档 + multiwin native（隔离显示；multiwin go 已知撞 C7，登记不阻塞）。

## 4 预期 API 面

`render` 新增导出：`BudgetDomain`（含 String）、`Budget`、`DefaultBudget`；内部缓存切换不新增 internal 导出。总账同步 + apidoc rc=0。

## 5 风险与对策

| # | 风险 | 对策 |
|---|---|---|
| 1 | 声明表切换时数字漂移 | pin 测试先于重构跑绿，重构后同测试锁；代码审查逐点对照评估表 |
| 2 | render/text ↔ render 循环 | 图集参数由 internal/gpu 调用方传入 |
| 3 | 淘汰行为意外变化 | 只动常量来源与派生函数，eviction 算法逻辑零改动 |
| 4 | descriptor 统一混合显卡机不可本机验 | 跨平台格如实登记（语义收敛、混合机行为变化待验） |
| 5 | K1 回退 | conformance 资源断言逐位对照 |
| 6 | vet 基线扰动 | 不触 `ui/platform` 与 `gpu_render_context.go:4296/4344` unreachable 两处 |

## 6 决策记录（2026-10-30 计划批准，默认按推荐执行）

| # | 决策 | 采纳 |
|---|---|---|
| D1 | 预算表覆盖面 | 全部 GPU 侧 10 点（CPU 池不入表归 P2.5） |
| D2 | descriptor 口径 | 统一到 ForAdapter（测试装箱保持 plain） |
| D3 | 按域分账形态 | 声明级分域（运行时用量聚合延 P2.4c/d） |
| D4 | TexturePool | 本步只入表，删池归 P2.4d |
