# UI 绘制质量：对齐 Skia / 浏览器（Chromium）思路

> 版本：1.0 | 日期：2026-07-22  
> 范围：窗口 UI 边缘（圆角、1px 边、圆、图标）vs `ui_ant_compare` CPU 对照图  
> 关联：[`VRAM_BUDGET.md`](./VRAM_BUDGET.md) · compositor · `exboot` · `render/internal/gpu`

## 问题

| 路径 | 观感 |
|------|------|
| `go run ./examples/ui_ant_compare`（CPU 软件 AA） | 接近 Ant Design |
| 窗口 GPU present（曾默认 1× 采样 + 细线 expand） | 硬边、毛刺、不像 Ant |

**不是控件样式两套**，而是 **光栅策略不同**。

## 浏览器 / Skia 怎么做

Chromium + Skia 的默认哲学（简化）：

1. **GPU 照用** — 合成与 2D 主路径在 GPU 上。  
2. **按物理像素画** — `CSS px × devicePixelRatio`；DPR=2 时 1px 边有 2 个物理像素可做覆盖率。  
3. **覆盖率 AA 为主，MSAA 为辅** — 圆/圆角/细线用 SDF 或解析覆盖；MSAA 补几何边，不是唯一 AA。  
4. **形状快路径** — 圆、RRect、1px 描边走专用路径，不整页一律 tessellate。  
5. **显存靠图层/瓦片/回收** — 不靠「默认关掉 UI AA」。

## 我们的对应实现

### 层 1 — 形状快路径 + 覆盖率（最重要）

| 机制 | 位置 | 说明 |
|------|------|------|
| 圆 / 圆角填充 | GPU SDF 片元（`sdf_render.wgsl`） | 每像素 coverage，软边 |
| **1px 实心描边** | `StrokeShape`：线宽 ≥1 走 **SDF 环**，不再强制 expand | 对齐 CPU `sdfMinStrokeWidth` |
| 虚线 / 非 solid | 仍走几何 expand+fill | 与 Skia「复杂 stroke 另一路径」类似 |
| kit 几何 | `ui/core/paint_geom.go` | 统一 round cap、border-box、细环 EvenOdd |

### 层 2 — 物理像素 / 超采样（学 DPR）

| 机制 | 位置 | 说明 |
|------|------|------|
| Host DPR | `Host.ScaleFactor()` · `GPUI_SCALE` / `GDK_SCALE` | 布局逻辑尺寸不变 |
| **UI 超采样** | `exboot` present：默认 `paintScale = max(host, 2)` | 1× 屏上仍 2× 离屏再 blit（接近对照图） |
| 关闭超采样 | `GPUI_UI_SUPERSAMPLE=0` | 省 RT 显存 |

### 层 3 — MSAA（增强，非唯一）

| 机制 | 位置 | 说明 |
|------|------|------|
| 默认 4× | `exboot.InitEnv` + `gpu_shared` 外部 Device | 软边增强 |
| 省显存 | `GPUI_SURFACE_SAMPLE_COUNT=1` | 关 MSAA；**仍可保留超采样 + SDF** |
| 日志 | `exboot: BindProvider ok … SAMPLE_COUNT=…` | 必现 std log |

### 层 4 — 显存（学浏览器「画少、可回收」）

| 机制 | 说明 |
|------|------|
| Adapter 默认核显 | `GPUI_POWER`（混合本不抢 300MB+ 独显基线） |
| Compositor 单 base RT | 全树画进稳定离屏再 blit（G2.b） |
| 可选关超采样 / 关 MSAA | 真紧时再降，而不是默认砍质量 |
| 生命周期 Purge | 不可见时 `Unconfigure` + 丢 surface 附件 |

## 推荐配置

```bash
# 质量优先（默认，接近浏览器）
unset GPUI_SURFACE_SAMPLE_COUNT   # 或 =4
unset GPUI_UI_SUPERSAMPLE         # 默认 ≥2× 离屏
# 可选：export GPUI_SCALE=2

go run ./examples/ui_polish_gallery
# 应看到：
#   exboot: BindProvider ok ... GPUI_SURFACE_SAMPLE_COUNT=4
#   exboot: UI paint scale=2.00 (host=1.00 ...)
```

```bash
# 省显存但仍要软边（Skia 思路：砍 MSAA，不砍覆盖率/超采样）
export GPUI_SURFACE_SAMPLE_COUNT=1
# 不要设 GPUI_UI_SUPERSAMPLE=0

go run ./examples/ui_polish_gallery
#   BindProvider ... SAMPLE_COUNT=1
#   UI paint scale=2.00
```

```bash
# 最低质量（调试/极限 VRAM）
export GPUI_SURFACE_SAMPLE_COUNT=1
export GPUI_UI_SUPERSAMPLE=0
```

## 对照回归

```bash
go run ./examples/ui_ant_compare
# file://$PWD/tmp/ant_compare/index.html
```

窗口观感应接近该 HTML 中的 CPU 图；若窗口仍硬、HTML 仍软 → 查 env 是否 `SAMPLE_COUNT=1` 且 `UI_SUPERSAMPLE=0`。

## 后续（真·Skia 对齐，未全部做完）

1. **图集 / 控件层缓存预算**（Flutter layer budget），按压显存而非关 AA  
2. **虚线按钮** 也尽量 coverage，而不是仅 expand  
3. **从 X11/Wayland 读真实 DPR**，减少对 `GPUI_SCALE` 的依赖  
4. 对照图也可走「GPU + 读回」双路径，防止只测 CPU  

## 一句话

> **Skia/浏览器：GPU + 物理像素 + 覆盖率 AA + 用结构管显存。**  
> **我们：同一路线 —— SDF/细线覆盖、2× UI 超采样、默认 4× MSAA；显存紧时先关 MSAA，最后才关超采样。**
