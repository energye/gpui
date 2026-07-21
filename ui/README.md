# ui — 控件框架（primitive 组合底座）

> 规格：[`docs/UI_FRAMEWORK_MAP.md`](../docs/UI_FRAMEWORK_MAP.md)  
> 入口条件：[`docs/S5_WIDGET_ENTRY.md`](../docs/S5_WIDGET_ENTRY.md) ✅

## 分层

| 包 | 职责 | 状态 |
|----|------|------|
| `ui/core` | 树·布局·Hit·Event·Paint·Frame·Theme·Focus·**OverlayHost** | M0–M2 ✅ |
| `ui/primitive` | P0/P1 + **Editable/Scroll/Overlay/Mask/Anchored/Trigger** | M0–M2 ✅ |
| `ui/platform` | SPI + Headless + Linux（Scroll/Text/IME 事件） | M0–M2 ✅ |
| `ui/skin/default` | Ant 向 Token + Skin | M1 ✅ |
| `ui/kit` | B0 Button/Text/Icon + **B1 Input/Checkbox/Radio/Switch/Tooltip/Popover** | M1–M2 ✅ |

## 测试

```bash
go test ./ui/core ./ui/platform ./ui/primitive ./ui/kit
```

## Smoke

```bash
export DISPLAY=:1 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_core_smoke      # M0
go run ./examples/ui_kit_smoke       # M1
go run ./examples/ui_kit_b1_smoke    # M2
```
