# ui — 控件框架（primitive 组合底座）

> 规格：[`docs/UI_FRAMEWORK_MAP.md`](../docs/UI_FRAMEWORK_MAP.md) · **M0–M6 主路径已落地**

| 包 | 状态 |
|----|------|
| `ui/core` / `primitive` / `kit` / `platform` / `skin/default` | M0–M6 ✅ |

## 跨平台

| OS | Host | GPU Present |
|----|------|-------------|
| Linux | `NewLinuxHost` / `NewHost` | ✅ |
| Windows | `NewWindowsHost`（SPI stub） | ❌ 后置 |
| macOS | `NewDarwinHost`（SPI stub） | ❌ 后置 |
| any | `NewHeadless` | paint 到 CPU Context |

## 测试 / 示例

```bash
go test ./ui/...

export DISPLAY=:1 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_core_smoke
go run ./examples/ui_kit_smoke
go run ./examples/ui_kit_b1_smoke
go run ./examples/ui_kit_b2_smoke
go run ./examples/ui_kit_b3_smoke
go run ./examples/ui_kit_m5_smoke
go run ./examples/ui_kit_shell      # M6 kit 桌面壳
```

覆盖率：`kit.AntCoverage()` / `CoverageSummary`。
