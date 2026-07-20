# device_lost_redraw

零配置 device-lost 示例：`RequestRedraw` 驱动 60fps + 独立渲染 goroutine。

## 运行（无需参数 / 环境变量）

在仓库根目录：

```bash
go run ./examples/device_lost_redraw
```

或：

```bash
go build -o /tmp/device_lost_redraw ./examples/device_lost_redraw
/tmp/device_lost_redraw
```

默认会：

- 自动找 `lib/libwgpu_native.so`（当前目录 / 可执行文件旁）
- 自动尝试 X11（`DISPLAY` 未设时试 `:0` / `:1`）
- **窗口可自由缩放 / 最大化 / 平铺**（无锁尺寸）
- 关窗口或 Ctrl+C 退出

## 行为摘要

| 项目 | 默认 |
|------|------|
| FPS | 60（Fifo Prefer + 帧预算 sleep；HUD 用 1s 滑动窗口） |
| 缩放 | **零闪**：拖动中不 `Surface.Configure`（合成器拉伸旧 buffer + 持续 present）；静默 ~32ms 再一次 Configure + 全量 Present。X11 `background=None` + bit gravity。 |
| 动画 | 类 S12 复合（卡片/光球/网格） |
| 完全遮挡 / 最小化 | 暂停 acquire，不 GCT |
| 失焦仍可见 | 继续画 |
| Device lost | AutoRecover，进程不退出 |

盖住窗口 soak 后露出，应继续动画且无 SIGABRT。

## 与 soft native / Skia 对齐

- 需要 **soft 补丁** `libwgpu_native.so`（`gogpu/rwgpu` 构建产物放到 `lib/`）。
- Device lost：`sc.BeginFrame` 返回 `ErrDeviceLost` → skip + `EnableAutoRecover`（无强制 `MarkLost`）。
- 不可 present：`!mapped` / minimized / **VisibilityFullyObscured** 时不 acquire。
- 注意：部分 WM（如 GNOME）盖窗时**不发** FullyObscured；此时仍可能 present（依赖 soft 不崩 + AutoRecover）。更严遮挡检测见 `mem_anim_window` 几何 stacking。
