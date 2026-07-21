# GPU 显存预算与适配（对齐 Skia / Flutter）

> 日期：2026-07-21  
> 范围：`gpu/rwgpu` InstanceExtras · `render/gpu` adapter 策略 · 示例 `vram_stages` / `device_lost_redraw`

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
- 混合本：独显被 UI 占 300M+ 不合理

## 策略（已实现）

### 1. 默认 Adapter 策略：**独显优先**

`render/gpu.ResolveAdapterPolicy()`：

| 环境变量 | 行为 |
|----------|------|
| （默认） | **HighPerformance**：有独显用独显；**没有独显才**用核显；再不行 software |
| `GPUI_POWER=high` / `auto` | 同上（独显优先） |
| `GPUI_POWER=low` / `GPUI_LOW_VRAM=1` | **核显优先**（混合本省独显 VRAM） |

**默认 = 独显优先。** 弱显存/混合本要省独显时显式设 `GPUI_LOW_VRAM=1` 或 `GPUI_POWER=low`。

### 2. Instance 后端与预算（`gpu/rwgpu.CreateInstance`）

- 修复 **`STypeInstanceExtras` ABI**（曾与 `webgpu.h` 错位，导致 backends 完全失效）
- 支持 `GPUI_BACKEND=gl|vulkan|primary|all|gl+vulkan`
- 支持 `GPUI_VRAM_BUDGET_PCT`（谨慎：过低会 device lost，**不要默认开**）
- X11：`XlibDisplay` 传入 InstanceExtras（GL 表面兼容仍依赖驱动/EGL，本机 NVIDIA 上 GL 仍可能报 not compatible）

### 3. 管线按需（F17）

`ensurePipelines` 只建 SDF+convex；stencil/image/depth-clip 按帧内容创建。  
减少无用变体编译（对 322 基线帮助有限，但对弱设备与启动路径正确）。

### 4. LowVRAM Device limits

`DeviceDescriptorLowVRAM` / `DeviceDescriptorForAdapter`：核显/CPU 使用更紧的 RequiredLimits。

## 测量工具

```bash
# 分阶段归因（默认独显优先）
go run ./examples/vram_stages

# 强制核显（混合本省独显）
GPUI_POWER=low go run ./examples/vram_stages

# 窗口示例
go run ./examples/device_lost_redraw
GPUI_LOW_VRAM=1 go run ./examples/device_lost_redraw
```

## 对标 Skia / Flutter 的诚实边界

| 路径 | 预期占用 | 适用 |
|------|----------|------|
| **默认 High（独显优先）** | 有独显 → ~200–320 MiB（本机 940MX Vulkan） | 正常桌面默认 |
| **无独显 → 核显** | 独显 0；系统内存 | 只有核显的机器 |
| `GPUI_LOW_VRAM=1` / `GPUI_POWER=low` | 强制核显优先 | 混合本要省独显显存 |
| 纯软渲染 | 无 GPU 进程显存 | 策略末级 ForceFallback |

**默认：有独显用独显，没有才用集成显卡。** 要省独显 VRAM 时显式 `GPUI_LOW_VRAM=1`。

## API

```go
policy := gpu.ResolveAdapterPolicy()
adpt, soft, err := gpu.RequestAdapterWithPolicy(inst, surf, policy)
dev, err := adpt.RequestDevice(gpu.DeviceDescriptorForAdapter("app", adpt))
```
