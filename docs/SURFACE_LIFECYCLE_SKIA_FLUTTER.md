# 表面生命周期（Skia / Flutter 对齐 · 跨硬件自适应）

> 版本：1.1 | 日期：2026-07-21 | **活文档**  
> 目标：所有机器统一策略；**不是**绑死某一台 1GB 卡。  
> 实现：`render/gpu/lifecycle_policy.go` + `examples/exboot/lifecycle.go`（`SurfaceHost`）  
> 关联：[`GPU_修复_device_lost.md`](./GPU_修复_device_lost.md) · [`VRAM_BUDGET.md`](./VRAM_BUDGET.md) · [`ENGINE_GAPS.md`](./ENGINE_GAPS.md) G3

## 1. 对标映射

| Skia / Flutter | 本仓库 |
| --- | --- |
| 不可 present 不 acquire | host `hidden` → 不 `BeginFrame` |
| Surface destroy → 放 swapchain | `Surface.Unconfigure` + `MarkNeedsReconfigure` |
| `GrDirectContext::freeGpuResources` | tier≥**Purge**：`DropGPURenderContext`（session/offscreen pool） |
| `abandonContext` + recreate | tier≥**Recreate** 或 **device lost** / **OOM 自适应**：`AbandonDevice` + `ForceRecoverHealthy` |
| Rasterizer rebind on surface recreate | `WireAutoRecover` + `SetDeviceProvider` |
| 失焦仍绘制 | **不**把 FocusOut 当 hidden |

## 2. 三档策略（`GPUI_LIFECYCLE`）

源码真源：`render/gpu/lifecycle_policy.go`（`ResolveSurfaceLifecycle`）+ `examples/exboot/lifecycle.go`（`SurfaceHost`）。

| Tier | 何时（Resolve） | hide（SurfaceHost.OnUnpresentable） | show（OnPresentable） |
| --- | --- | --- | --- |
| **Normal** | **仅** `GPUI_LIFECYCLE=normal\|flutter\|light` | **先** `PurgeSurfaceResources()`；**不** Unconfigure、**不** DropGPU（host 只停 present） | `ClearRecoverCooldown` + 保持已配置 surface |
| **Purge** | `auto`/`""` 且无 OOM；或显式 `purge` | PurgeSurfaceResources → **Unconfigure** + MarkNeedsReconfigure + **DropGPU** | 同 device **reconfigure**（无 abandon 则不 ForceRecoverHealthy） |
| **Recreate** | 显式 `recreate` **或** 进程内 `TextureOOMCount()>0` | Purge 路径 + **AbandonDevice** | **ForceRecoverHealthy** + BindProvider |

`auto` / 默认（`GPUI_LIFECYCLE` 未设）：

1. 有 OOM 记录 → **Recreate**  
2. **否则一律 Purge**（含离散 GPU；**不是**「离散默认 Normal」）  
3. 显式 `normal` 才是 Flutter-light（不 Unconfigure）

OOM：`createTextureRetryOOM` → `noteTextureOOM` → `NoteTextureOOM()`；任意 GPU 首次 OOM 后升 Recreate。  
引擎侧：`Surface.Unconfigure` 成功后仍回调 `AfterSurfaceUnconfigure` → 再 `PurgeSurfaceResources`（与 host 显式 Purge 可叠加，幂等）。

## 3. 引擎层（不靠示例特例）

| 点 | 行为 |
| --- | --- |
| `Surface.Unconfigure` | 回调 `webgpu.AfterSurfaceUnconfigure` → **`PurgeSurfaceResources`**（全 live session depth/MSAA/offscreen pool） |
| `ForceRecoverHealthy` / AutoRecover | `BeforeDeviceRecover` → Purge；host `OnDeviceAbandon` → DropGPU+Abandon；**force Release**；**RequestDevice+1×1 probe 重试** |
| `GPURenderSession.PurgeSurfaceTextures` | 只卸表面附件，保留 pipeline/device |
| `NoteTextureOOM` | CreateTexture OOM 时升级 lifecycle → Recreate |

## 4. 运行时自适应环

```
frame → CreateTexture OOM? → NoteTextureOOM
     → ResolveSurfaceLifecycle 升为 Recreate
     → RecoverIfOOMPressure（每 OOM 计数一次 ForceRecoverHealthy）
```

这样：

- **默认可移植**：Purge（卸表面附件 + 可选 DropGPU，不 thrash Device）  
- **OOM 后**：自动升 Recreate（任意厂商/容量）  
- **高显存想更轻**：`GPUI_LIFECYCLE=normal`  

## 5. Host 用法（示例统一）

```go
host := &exboot.SurfaceHost{
    SC: sc, Adapter: adapter, Device: &device,
    DropGPU: dropAll, Format: sc.Format,
}
// hide:
host.OnUnpresentable()
// show:
host.OnPresentable()
// after frame that may allocate:
host.RecoverIfOOMPressure()
```

强制档位（测试 / 嵌入式）：

```bash
GPUI_LIFECYCLE=normal    # 仅 Flutter 默认
GPUI_LIFECYCLE=purge     # 后台卸 session
GPUI_LIFECYCLE=recreate   # 后台 abandon + 前台 recreate
GPUI_LIFECYCLE=auto       # 默认
```

## 6. 验收

```bash
# 全 API + 自适应
GPUI_COVERAGE_STRICT=1 GPUI_LIFECYCLE=auto GPUI_FORCE_LOST_AFTER=5 \
  ./tmp/bins/api_coverage_app

# 全 API + minimize/restore（应 oom=0 且 180/180）
GPUI_COVERAGE_STRICT=1 GPUI_LIFECYCLE=auto GPUI_SELFTEST_LIFECYCLE=1 \
  GPUI_SELFTEST_MIN_AT=4 GPUI_SELFTEST_MAP_AT=10 GPUI_SELFTEST_LOST_AT=16 GPUI_SELFTEST_DONE_AT=24 \
  ./tmp/bins/api_coverage_app

# 高显存机模拟「只 Unconfigure」
GPUI_LIFECYCLE=normal GPUI_ANIM_SECONDS=8 ./tmp/bins/api_coverage_app  # 限时；省略 ANIM_SECONDS 则不限时
```

## 7. 明确不做

- 不为某个 NVIDIA 型号写死「永远 recreate」  
- 不把 FocusOut 当 pause  
- 不用 sticky `MarkLost` 做日常注入（用 `ForceRecoverHealthy`）  

## 8. 与引擎缺口

本策略 **已实现**；持续压力与 soak 仍属 **G3**（见 [`ENGINE_GAPS.md`](./ENGINE_GAPS.md)）：多 RT + 滤镜 + force-lost 矩阵需保持绿，不在本文重复列方案。
