# docs/

> 2026-07-27 · **L1 P0–P3 已收口**

## 读哪里

| 优先级 | 文档 | 说明 |
|--------|------|------|
| **1** | [`ENGINE_L1_CLOSEOUT.md`](./ENGINE_L1_CLOSEOUT.md) | **L1 状态 · 验收命令 · 限制 · 下一步** |
| 2 | [`ENGINE_ARCH_OVERVIEW.md`](./ENGINE_ARCH_OVERVIEW.md) | 架构图（四层 / 依赖 / 坐标） |
| 3 | [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md) | 架构条款全文 |
| 4 | [`ENGINE_PHASE_P0_P3.md`](./ENGINE_PHASE_P0_P3.md) | P0–P3 任务考古（已完成） |
| 5 | [`ENGINE_PHASE_P4.md`](./ENGINE_PHASE_P4.md) | **P4 详细任务卡（滚动/虚拟列表/IO）** |
| 6 | [`ENGINE_PHASE_P4_P7_OUTLINE.md`](./ENGINE_PHASE_P4_P7_OUTLINE.md) | P4–P7 大纲 |
| — | [`antd/`](./antd/) | L3 控件需求（后置） |

## 状态

```text
L1 体验雏形（P0–P3）  ✅  已收口
P4 滚动/文本/IO       ✅  细卡已实现
L2 / L3               ⬜
```

## 验收

```bash
go test ./ui/... -count=1
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_l1_blank
go run ./examples/ui_l1_spinner
```

## 约定

- 依赖：`ui → render → gpu`（ui 禁止 import gpu）  
- 窗口：宿主注入 `NativeSurface`  
- 坐标：逻辑 px、Y-down；物理 = 逻辑 × dpr  
