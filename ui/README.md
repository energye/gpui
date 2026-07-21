# ui — 控件框架（primitive 组合底座）

> 规格：[`docs/UI_FRAMEWORK_MAP.md`](../docs/UI_FRAMEWORK_MAP.md)

| 包 | 状态 |
|----|------|
| `ui/core` … `ui/kit` | **M0–M5 ✅** |

```bash
go test ./ui/...
export DISPLAY=:1 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_core_smoke      # M0
go run ./examples/ui_kit_smoke       # M1
go run ./examples/ui_kit_b1_smoke    # M2
go run ./examples/ui_kit_b2_smoke    # M3
go run ./examples/ui_kit_b3_smoke    # M4
go run ./examples/ui_kit_m5_smoke    # M5
```
