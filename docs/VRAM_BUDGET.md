# GPU 显存预算与适配（对齐 Skia / Flutter）

> 版本：1.3 | 日期：2026-07-22 | **活文档**  
> 范围：`gpu/rwgpu` InstanceExtras · `render/gpu` adapter 策略 · 示例 `vram_stages` / `device_lost_redraw`  
> 关联：[`SURFACE_LIFECYCLE_SKIA_FLUTTER.md`](./SURFACE_LIFECYCLE_SKIA_FLUTTER.md) · [`ENGINE_GAPS.md`](./ENGINE_GAPS.md) G3.d

## 问题

在 **NVIDIA 940MX (1GB) + wgpu-native Vulkan** 上，即使只 clear 一帧：

| 阶段 | nvidia-smi 进程占用 |
|------|---------------------|
| `RequestDevice` | ~194 MiB |
| 首帧 Present | **~322 MiB** |
| 应用侧 CreateTexture/Buffer | **~5 MiB** |

对比 **v2rayN（Skia 常规列表 UI）~33 MiB**。  
差的不是「画太多 UI」，而是 **Vulkan 独显设备基线 + 驱动预留**。

用户可能：

- 没有独显（核显 / 软渲染）
- 只有几十 MiB 可用显存
- 混合本：独显被 UI 占 300M+ 离谱
- **多开**：每个进程都抢独显 → OOM / 连锁崩溃

## 策略（已收敛）

### 1. Adapter：仅 `GPUI_POWER`

| 环境变量 | 行为 |
|----------|------|
| （默认） | **Default**：混合本有核显则用核显；单卡用那块卡 |
| `GPUI_POWER=high` | 独显优先 |
| `GPUI_POWER=low` | 核显优先 |

实现：`render/gpu.ResolveAdapterPolicy()` + `RequestAdapterWithPolicy`。  
Default 路径：先 `PowerPreferenceNone`，若落在独显且存在核显 → 改用核显（修正 Optimus 上 bare None 仍回 dGPU 的问题）。

**不再使用** `GPUI_LOW_VRAM` / `GPUI_AUTO_VRAM`（limits 按 adapter 类型自动收紧）。

### 2. Device limits：跟 adapter 类型

`DeviceDescriptorForAdapter`：

- Integrated / CPU → `DeviceDescriptorLowVRAM`（更紧 limits）
- Discrete → 默认 UI limits

### 3. Instance：`GPUI_BACKEND`（+ 专家项预算）

- `GPUI_BACKEND=gl|vulkan|primary|all|gl+vulkan`
- `GPUI_VRAM_BUDGET_PCT`（谨慎：过低会 device lost，**不要默认开**）

### 4. Surface lifecycle：仅 `GPUI_LIFECYCLE`

- 默认 auto → **Purge**
- 见过 CreateTexture OOM → **Recreate**
- 显式 `normal|purge|recreate`

### 5. 管线按需（F17）

`ensurePipelines` 只建 SDF+convex；stencil/image/depth-clip 按帧内容 lazy。

## 测量工具

窗口示例默认 **不限时**（`GPUI_ANIM_SECONDS` / `GPUI_VRAM_SECONDS` 未设或 0）；CI 显式设秒数。

UI 窗口 present（`exboot.RunUIDemand`）：

| 环境变量 | 行为 |
|----------|------|
| （默认） / `GPUI_COMPOSITOR=1` | **推荐**：整树 → base 离屏 RT → 上屏只 blit（G2.b） |
| `GPUI_COMPOSITOR=0` | 表面直接矢量 Clear+全画（G2.a 对照） |

本机对照（4 核、kit 类窗口、有限动画控件，2026-07）：

| 路径 | 整机 CPU 约 |
|------|-------------|
| `GPUI_COMPOSITOR=1`（默认） | **~3.05%** |
| `GPUI_COMPOSITOR=0` | ~3.5%（约高 **0.5pp**） |

说明：差值不大但方向稳定；compositor 把矢量留在稳定 offscreen，上屏一轮 blit，表面路径驱动/同步更重。小 Spin 再叠 per-widget 离屏层未必更省，分层优化需按场景再测。

```bash
# 默认（混合本 → 核显；不限时 present）
go run ./examples/vram_stages

# 限时 hold/present
GPUI_VRAM_SECONDS=4 go run ./examples/vram_stages

# 强制独显
GPUI_POWER=high go run ./examples/vram_stages

# 强制核显
GPUI_POWER=low go run ./examples/vram_stages

go run ./examples/device_lost_redraw
```

## 预期占用

| 路径 | 预期占用 | 适用 |
|------|----------|------|
| **默认 Default** | 混合本 → 核显（独显 0）；仅有独显 → 独显 | 桌面 UI |
| `GPUI_POWER=high` | 有独显 → ~200–320 MiB（本机 940MX Vulkan） | 明确要独显性能 |
| `GPUI_POWER=low` | 强制核显 | 强制省独显 |
| 纯软渲染 | 无 GPU 进程显存 | 策略末级 ForceFallback |

## API

```go
policy := gpu.ResolveAdapterPolicy()
adpt, forceFallback, err := gpu.RequestAdapterWithPolicy(inst, surf, policy)
_ = forceFallback
dev, err := adpt.RequestDevice(gpu.DeviceDescriptorForAdapter("app", adpt))
```

## 与 surface lifecycle

- 不可 present（**auto/Purge 默认**）：`Unconfigure` + `PurgeSurfaceResources` / DropGPU  
- OOM：`NoteTextureOOM` → lifecycle 升 **Recreate** → `ForceRecoverHealthy`  
- 双 Device 峰值：recover 必须完整 Release 旧堆；见 device_lost 文档 4c
