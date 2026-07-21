# ui — 控件框架（primitive 组合底座）

> 规格：[`docs/UI_FRAMEWORK_MAP.md`](../docs/UI_FRAMEWORK_MAP.md)  
> 入口条件：[`docs/S5_WIDGET_ENTRY.md`](../docs/S5_WIDGET_ENTRY.md) ✅

## 分层

| 包 | 职责 | 状态 |
|----|------|------|
| `ui/core` | 管线：树·布局·Hit·Event·Paint·Frame·Plugin·Theme/Token·Focus | M0–M1 ✅ |
| `ui/primitive` | 积木 P0+P1：Box/Flex/Stack/…/Decorated/Icon/Slot/… | M0–M1 ✅ |
| `ui/platform` | SPI + Headless + Linux 薄适配 | M0 ✅ |
| `ui/skin/default` | Ant 向 Token + 默认 Skin | M1 ✅ |
| `ui/kit` | 产品控件 B0：Button / Text / Icon | M1 ✅ |

依赖只向下：`app → kit → primitive → core → render`；`platform` 不进 core。

## 测试

```bash
go test ./ui/core ./ui/platform ./ui/primitive ./ui/kit
```

## Smoke

```bash
export DISPLAY=:1 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# M0：primitive Pressable+Text
GPUI_ANIM_SECONDS=12 GPUI_SMOKE_AUTOCLICK=30 go run ./examples/ui_core_smoke

# M1：kit.Button 状态矩阵 + Icon
GPUI_ANIM_SECONDS=12 GPUI_SMOKE_AUTOCLICK=40 go run ./examples/ui_kit_smoke
```
