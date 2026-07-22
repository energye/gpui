# Window examples — real upper-layer scenarios

对照 **Skia / Flutter / Ant Design** 桌面与移动宿主用法。所有 X11 窗口示例统一：

- `exboot` 启动 + `WireAutoRecover`（Skia abandon + Flutter Rasterizer rebind）
- 最小化 / FullyObscured → **pause present + Surface.Unconfigure**
- 仅失焦、仍可见 → **继续 present**
- `GPUI_FORCE_LOST_AFTER=N` → `ForceRecoverHealthy`（健康路径，不钉 VRAM）
- 可选 `GPUI_SELFTEST_LIFECYCLE=1` 最小化→还原→recover 自测
- **时长**：`GPUI_ANIM_SECONDS` 默认 **0 = 不限时**（关窗/信号退出）；CI/自动化显式设秒数

## 矩阵脚本

```bash
export DISPLAY=:1 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
ROUNDS=2 bash examples/run_window_lifecycle_matrix.sh
```

## 示例对照表

| 示例 | 对标上层 | 覆盖场景 |
|------|----------|----------|
| **antd_desktop_app** | Ant Design Pro / Electron 壳 | Header+Sider+Content+Footer、Table 虚拟滚动、Modal、Drawer、Toast、主题色 token、最小化/recover |
| **flutter_app_shell** | Flutter Material + Skia | Scaffold/AppBar/FAB/NavigationBar、Navigator push 路由层、setState 每帧重建、CustomPainter 离屏、ListView、AppLifecycle pause/resume、recover |
| **app_lifecycle_shell** | 宿主生命周期矩阵 | S_ANIM/S_FOCUS/S_MIN/S_RESIZE/S_LAYER/S_MULTI/S_RECOVER/S_ALL |
| **mem_anim_window** | Skia 2D API 覆盖 + 长泡 | 全 feature 场景 S03–S23、effect RT、selftest |
| **particle_kitchen_sink** | 隔离探针 / 压测 | blend/glow/mesh、resize、Unconfigure |
| **capability_matrix** | SKIA_2D_CAPABILITY_MATRIX | C0x/C2x 能力 ID 门禁 |
| **device_lost_redraw** | Flutter RequestRedraw | 60fps redraw 环、device lost |
| **window_present** | 最小 present 路径 | Swapchain Fifo、PresentFrameAuto |
| **vram_stages** | VRAM 归因诊断 | device/swapchain/clear/full 分阶 |
| **api_coverage_app** | **全量 render.Context 公开 API** | 180/180 覆盖率门禁、产品场景聚类调用、minimize+recover 复现/修复 VRAM |

## 推荐手测命令

```bash
# 交互默认不限时
go run ./examples/antd_desktop_app
go run ./examples/ui_polish_gallery

# Ant Design 桌面壳 + 限时 + 强制 recover（CI 风格）
GPUI_ANIM_SECONDS=15 GPUI_FORCE_LOST_AFTER=50 ./tmp/bins/antd_desktop_app

# Flutter Material 壳 + 生命周期自测
GPUI_SELFTEST_LIFECYCLE=1 GPUI_SELFTEST_MIN_AT=40 GPUI_SELFTEST_MAP_AT=90 \
  GPUI_SELFTEST_LOST_AT=140 GPUI_SELFTEST_DONE_AT=200 ./tmp/bins/flutter_app_shell

# 完整 API 覆盖 + minimize selftest（不限时；selftest 帧点到齐后退出）
GPUI_SCENARIO=S12 GPUI_SELFTEST_LIFECYCLE=1 ./tmp/bins/mem_anim_window
```

## 宿主契约（Skia / Flutter）

1. **遮挡 ≠ device lost**：不可 present 时不 acquire。  
2. **失焦 ≠ 暂停**：仍露出则继续画。  
3. **lost → abandon 全部 Context GPU**（引擎注册表）再 recreate。  
4. **恢复后画最新状态**，不是暂停前缓存帧。


## 全 API 覆盖（强制）

```bash
# 必须 180/180；失败则 GPUI_COVERAGE_STRICT=1 → exit 2
# 限时可选；不设 GPUI_ANIM_SECONDS 则一直跑到关窗
GPUI_COVERAGE_STRICT=1 GPUI_ANIM_SECONDS=15 GPUI_FORCE_LOST_AFTER=5 \
  ./tmp/bins/api_coverage_app

# 最小化 → 还原（resume 时 ForceRecoverHealthy）→ 再 recover
GPUI_COVERAGE_STRICT=1 GPUI_SELFTEST_LIFECYCLE=1 ./tmp/bins/api_coverage_app
```

`api_coverage_app` 不是 UI 仿样，而是把 **每一条** `render.Context` 公开方法放进真实产品路径（路径/笔刷/clip/layer/filter/image/text/mesh/present/damage/shared encoder…），并用 lifecycle 复现问题。
