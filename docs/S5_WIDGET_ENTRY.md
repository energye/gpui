# S5.5 — 控件层开工入口条件

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S5.5 关闭 / S5 全线关闭**  
> 依赖：S5.0–S5.4

---

## 1. 允许开始「类 Ant Design 控件层」之前必须满足

| # | 条件 | 证据 | 状态 |
|---|------|------|------|
| 1 | Skia 主路径 P0 能力无阻塞缺口 | `docs/S5_SKIA_UI_GAP.md` | ✅ |
| 2 | Present-only 基线可复跑 | `docs/S5_PRESENT_BASELINE.md` + json | ✅ |
| 3 | Retained + damage 帧模型固化 | `docs/S5_FRAME_MODEL.md` + `TestS52_*` | ✅ |
| 4 | 主路径 U01–U04 60fps 门禁绿 | `docs/S5_60FPS_GATE.md` + `TestS53_*` | ✅ |
| 5 | P0/P1 阻塞补丁队列清空 | S5.0/S5.4 | ✅（队列空） |
| 6 | 回归：`TestS3*` / 抽样 `TestP1_*` / Comp 绿 + GPUOps>0 | 测试 | ✅ |
| 7 | **控件实现不得另起光栅化**；必须走 `render` 能力表 | 架构约束 | ✅ 写入本文 |

---

## 2. 控件层启动后仍禁止

- silent CPU 冒充 GPU  
- 绕过 `PresentFrame*` 自管 swapchain 像素语义却宣称引擎能力  
- 把 R.02 PDF/SVG、真 multiplanar YUV 当作控件阻塞依赖  

---

## 3. 建议的控件层第一批（启动后，非 S5 范围）

形态复用已有探针：Button / Input / Modal / List row / Table cell — **仅组件 API**，绘制全走 Context。

---

## 4. 退出条件（S5.5 / S5）

| 条件 | 状态 |
|------|------|
| 入口清单全勾 | ✅ |
| S5.0–S5.5 文档齐全 | ✅ |
| 仍无控件层实现代码（S5 阶段） | ✅ |

**S5.5 关闭。S5.x 全线关闭。**  
**允许**开启后续「控件层主线」（另开计划章节）。
