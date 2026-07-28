# docs/

> 2026-07-28 · **L1 收口 · 自定义控件渲染基座施工中**

## 读哪里

| 优先级 | 文档 | 说明 |
|--------|------|------|
| **1** | [`ENGINE_L1_CLOSEOUT.md`](./ENGINE_L1_CLOSEOUT.md) | L1 状态 · 验收命令 · 限制 |
| **1b** | [`ENGINE_CODING_RULES.md`](./ENGINE_CODING_RULES.md) | 禁止 CGO · purego · 依赖 · 架构/示例边界 |
| **1c** | [`ENGINE_UI_RENDER_BASE.md`](./ENGINE_UI_RENDER_BASE.md) | **画什么**（母表 · §22 收口 · §25） |
| **1d** | [`ENGINE_UI_WIDGET_RENDER.md`](./ENGINE_UI_WIDGET_RENDER.md) | **自定义控件渲染基座统一真源**（§R · 域地图 · L0 预留 · W0–W6 排期） |
| 2 | [`ENGINE_ARCH_OVERVIEW.md`](./ENGINE_ARCH_OVERVIEW.md) | 架构图 |
| 3 | [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md) | 架构条款全文 |
| 4 | [`ENGINE_PHASE_P0_P3.md`](./ENGINE_PHASE_P0_P3.md) | P0–P3 考古 ✅ |
| 5 | [`ENGINE_PHASE_P4.md`](./ENGINE_PHASE_P4.md) | P4 ✅ |
| 6 | [`ENGINE_PHASE_P5.md`](./ENGINE_PHASE_P5.md) | P5 L2 骨架 |
| 7 | [`ENGINE_PHASE_P4_P7_OUTLINE.md`](./ENGINE_PHASE_P4_P7_OUTLINE.md) | P4–P7 大纲 |
| — | [`antd/`](./antd/) | L3 控件需求（**后置**，基座 W6 后） |

## 状态

```text
L1 P0–P3 / P4 / P5     ✅
渲染基座 §22 主路径     ✅  ENGINE_UI_RENDER_BASE §25
自定义控件渲染基座      🔄  W0✅ W1✅ W2✅；W3–W6 ⬜（ENGINE_UI_WIDGET_RENDER v3.1）
L3 Kit / 桌面深做 / IME ⏸
```

## 验收

```bash
go test ./ui/... -count=1
go test ./examples/wrgate -count=1
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
# 真窗：1200×800 · RUN_SECONDS≥5 · 时长见 §2.5
RUN_SECONDS=15 go run ./examples/ui_wr_r4_composite
RUN_SECONDS=15 go run ./examples/ui_wr_r11_dpr
RUN_SECONDS=5  go run ./examples/ui_wr_r13_hit
RUN_SECONDS=10 go run ./examples/ui_wr_r18_savelayer
RUN_SECONDS=15 go run ./examples/ui_wr_c7_resize_dpr
```

## 约定

- 依赖：`ui → render → gpu`（禁止 ui→gpu）  
- **禁止 CGO**；FFI 仅 purego — [`ENGINE_CODING_RULES.md`](./ENGINE_CODING_RULES.md)  
- demo 仅 `examples/`，不进 `ui/*`  
- 坐标：逻辑 px、Y-down；物理 = 逻辑 × dpr  
