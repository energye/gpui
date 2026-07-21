# 表面生命周期（Skia / Flutter 对齐 · 跨硬件自适应）

> 目标：所有机器统一策略；**不是**绑死某一台 1GB 卡。  
> 实现：`render/gpu/lifecycle_policy.go` + `examples/exboot/lifecycle.go`（`SurfaceHost`）

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

| Tier | 何时 | hide | show |
| --- | --- | --- | --- |
| **Normal** | 默认离散 GPU、无 OOM 史 | Unconfigure | reconfigure |
| **Purge** | `GPUI_LOW_VRAM=1` / 核显/CPU / `GPUI_LIFECYCLE=purge` | Unconfigure + DropGPU | reconfigure（session 懒建） |
| **Recreate** | `GPUI_LIFECYCLE=recreate` **或进程内出现过 CreateTexture OOM** | Unconfigure + DropGPU + AbandonDevice | **ForceRecoverHealthy** |

`auto`（默认）：

1. 有 OOM 记录 → **Recreate**  
2. 否则 → **Purge**（含离散 GPU：Unconfigure + purge session 纹理 + DropGPU；**不**每次 recreate Device）  
3. 显式 `GPUI_LIFECYCLE=normal` → 仅 pause present（桌面 Flutter 轻量）  

OOM 由引擎 `createTextureRetryOOM` → `NoteTextureOOM()` 记入，**任意 GPU** 上首次 OOM 后自动升级，无需为某型号写死。

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
GPUI_LIFECYCLE=normal GPUI_ANIM_SECONDS=8 ./tmp/bins/api_coverage_app
```

## 7. 明确不做

- 不为某个 NVIDIA 型号写死「永远 recreate」  
- 不把 FocusOut 当 pause  
- 不用 sticky `MarkLost` 做日常注入（用 `ForceRecoverHealthy`）  
