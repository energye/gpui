# GPUI 渲染栈主线计划（精简）

> 版本：1.0 | 日期：2026-07-15  
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

- [ ] 能力表 §2.1–2.4 所列 API 均有绑定或书面豁免（豁免不得挡 M2）  
- [ ] enum 不再依赖“碰巧与 header 相等”  
- [ ] `go test ./gpu/rwgpu` 全绿  
- [ ] 文档：`docs/RWGPU_SKIA_SUBSET_CHECKLIST.md`（API ↔ 文件 ↔ 测试）

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

- [ ] 子集 API 均有 facade 对象方法  
- [ ] conversion 测试覆盖高风险 enum/stencil/blend/topology  
- [ ] `go test ./gpu/webgpu` 全绿  

---

### S3 — render 对标 Skia 2D（切片）

按能力表优先级推进；每切片：实现 → GPU 真链路测试 → 更新能力表状态列。

| 切片 | 能力焦点 | 退出条件（摘要） |
|------|----------|------------------|
| **S3a M0–M1** | 清屏、path fill/stroke、AA、CTM、solid、clip rect、hairline | CPU/GPU 基础图元门禁绿 |
| **S3b M2** | blend/premul、image、text、rrect、layer opacity、dash、gradient、MSAA | UI 级 2D 门禁绿 |
| **S3c M3** | 高级 clip/filter/shadow、surface present、color space… | 完整 2D + 窗口路径 |

**硬规则**：

- 声称 GPU：必须 `WGPU_NATIVE_PATH` 真库 + 可观测 `gpu_ops`（已有 P1.0 可保留）  
- 未解释 fallback 不得关闭切片  
- 性能数字不作为 S3 退出条件  

**完成标准（S3b 作为“可宣称 Skia 级 UI 2D 能力”门槛）**：

- [ ] 能力表 M0–M2 必选行使 rwgpu/webgpu/render 均非 ⬜  
- [ ] 关键固定像素 + 场景回归绿  
- [ ] 已知差异全部写回能力表备注  

---

### S4 — 性能（后置）

仅当对应能力在 S3 正确后：批处理、图集、缓存、并行等。  
仍须回归 S3 门禁。

---

## 4. 当前执行焦点（2026-07-15）

| 顺序 | 动作 | 状态 |
|------|------|------|
| 1 | S0 能力表 + 主线计划 | ✅ 本轮 |
| 2 | S1 编写 `RWGPU_SKIA_SUBSET_CHECKLIST` 并开始 ABI 审计 | ⬜ **下一步** |
| 3 | S1 `go test ./gpu/rwgpu` 缺口清零 | ⬜ |
| 4 | S2 facade 对齐 | ⬜ |
| 5 | S3a → S3b render 对标 | ⬜ |

已完成可复用资产（并入主线，不另开叙事）：

- P0 视觉 STRICT / format readback / path stats / SourceOver GPU 固定像素 → 作为 S3 回归工具  

---

## 5. 目录与文档

| 文件 | 角色 |
|------|------|
| `docs/SKIA_2D_CAPABILITY_MATRIX.md` | **能力真相来源** |
| `docs/MAINLINE_PLAN.md` | **执行主线**（本文） |
| `docs/RWGPU_SKIA_SUBSET_CHECKLIST.md` | S1 产出（待建） |
| `docs/OPTIMIZATION_PLAN.md` | 历史大计划；服从主线 |

---

## 6. 修订记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | 确立 S0–S4；排除控件层与杂项目录；能力表驱动 ABI→facade→render |
