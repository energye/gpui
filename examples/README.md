# examples/

L1 验收示例（与旧示例无关）。

| 目录 | 阶段 | 说明 |
|------|------|------|
| [`ui_l1_blank`](./ui_l1_blank) | P0 | 清色真窗 + JSON 指标 |
| [`ui_l1_spinner`](./ui_l1_spinner) | P3 | Spinner + 异步 present + JSON |
| [`ui_l1_scroll`](./ui_l1_scroll) | P4 | VirtualList 1k 行 + 自动滚动；窗口可缩放 |
| [`ui_l2_shell`](./ui_l2_shell) | P5 | **L2 机制烟囱**（示例场景，非 ui 包）：Tap/Pan · Focus · Overlay |
| [`exhost`](./exhost) | — | 示例共用窗口宿主（**自动 X11 / Wayland**） |

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
# 禁止 CGO：Wayland/X11 均 purego
export CGO_ENABLED=0

# 默认各跑 60 秒；看卡帧可加长：
RUN_SECONDS=180 go run ./examples/ui_l1_blank
RUN_SECONDS=180 go run ./examples/ui_l1_spinner
RUN_SECONDS=180 go run ./examples/ui_l1_scroll
RUN_SECONDS=120 go run ./examples/ui_l2_shell

# 强制后端：
GPUI_DISPLAY=wayland go run ./examples/ui_l1_spinner
GPUI_DISPLAY=x11     go run ./examples/ui_l1_spinner
```

## 显示后端（自动适配）

| 优先级 | 条件 | 行为 |
|--------|------|------|
| 1 | `GPUI_DISPLAY=x11\|wayland` | 强制该后端 |
| 2 | Auto + 有 `DISPLAY` | **X11**（含 XWayland）— **有系统标题栏** |
| 3 | Auto + 仅 `WAYLAND_DISPLAY` | 原生 **Wayland** |
| 4 | Wayland 失败且有 `DISPLAY` | 回退 X11 |

### 为什么有时没有标题栏？

- **原生 Wayland**（`xdg_toplevel`）本身 **不画** 标题栏；要靠  
  - 合成器 **SSD**（`zxdg_decoration_manager_v1`，KDE/Sway 等），或  
  - 应用自己画 **CSD**（示例尚未做）  
- **GNOME Wayland** 不提供 SSD → 纯 Wayland 窗口会像「无边框色块」。  
- **默认 Auto 优先 X11/XWayland**，由桌面 WM 画标题栏/关闭按钮。  
- 强制原生 Wayland：`GPUI_DISPLAY=wayland`（有 SSD 的合成器会请求服务端装饰）。

引擎侧：`PresentNativeSurface.Platform` 决定 `CreateSurface` 走 Xlib 还是 Wayland，**句柄类型与 surface 后端始终一致**。

## 纪律

- **禁止 CGO**（`import "C"`）；原生库只用 **purego** — [`docs/ENGINE_CODING_RULES.md`](../docs/ENGINE_CODING_RULES.md)
- **示例 ≠ 库：** 场景图、布局常量、演示文案只在本目录；**不要** 往 `ui/` 塞 demo 包 — 同上 §5（架构 vs 示例）
- 默认时长 **60s**；**`RUN_SECONDS`** 覆盖
- **关窗：** 先停 GPU Present 再毁窗
- 需要可用显示。详见 `docs/ENGINE_L1_CLOSEOUT.md`
