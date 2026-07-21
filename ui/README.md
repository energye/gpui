# ui — 控件框架（primitive 组合底座）

> 规格：[`docs/UI_FRAMEWORK_MAP.md`](../docs/UI_FRAMEWORK_MAP.md)

## 分层

| 包 | 状态 |
|----|------|
| `ui/core` | M0–M3 ✅ 管线 + Theme + Overlay + Form/Selection/KeyboardNav/Notify |
| `ui/primitive` | M0–M3 ✅ 布局/交互/编辑/浮层/Scroll/VirtualList/FocusScope |
| `ui/platform` | Headless + Linux |
| `ui/skin/default` | Ant light tokens |
| `ui/kit` | B0–B3：Button…Form/Select/Menu/Tabs/Modal/Drawer/Message |

## 测试 / Smoke

```bash
go test ./ui/core ./ui/platform ./ui/primitive ./ui/kit

export DISPLAY=:1 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_core_smoke
go run ./examples/ui_kit_smoke
go run ./examples/ui_kit_b1_smoke
go run ./examples/ui_kit_b2_smoke
```
