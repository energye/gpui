# S5.0 — Skia 控件向能力对账

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S5.0 关闭**  
> 依据：`docs/SKIA_2D_CAPABILITY_MATRIX.md` + `docs/MAINLINE_PLAN.md` S5  
> 目标：按 **未来 UI 控件依赖度** 重排缺口；**不实现控件层**。

---

## 1. 方法

1. 以能力表为唯一真相来源，逐项标 **P0 / P1 / P2 / 旁路**。  
2. 每项状态：`已有测试` | `需补（阻塞 UI）` | `书面后置`。  
3. **需补** 仅当：控件主路径绕不开，且当前无真 GPU 语义/像素门禁。  
4. R.02 PDF/SVG、真 multiplanar YUV 按主线 **旁路**，不阻塞 S5 / 控件入口。

---

## 2. 优先级定义

| 级 | 含义 | 控件/场景 |
|----|------|-----------|
| **P0** | 几乎所有控件依赖 | text/clip/layer/image/rrect/blend/present/damage/HiDPI |
| **P1** | 常见浮层与装饰 | backdrop/shadow/blur/dash/gradient/nine/filter |
| **P2** | 少见或可绕开 | mesh 边角、F16 全链路、冷门 blend、录制 |
| **旁路** | 不阻塞 UI 引擎入口 | PDF/SVG、真 YUV multiplanar、Skia 绝对 FPS 报表 |

---

## 3. P0 对账结果

| ID | 能力 | 状态 | 证据 / 结论 |
|----|------|------|-------------|
| X.* | 文本 shape/LCD/CJK/atlas/modes | **已有测试** | `TestP1_Capability_X03/X04/X05/X06/X11` 等 |
| C.* | Clip rect/rrect/path/AA | **已有测试** | `TestP1_Capability_C05*`、S3b/S3c clip 门禁 |
| L.01–L.04 | Layer opacity / save-restore | **已有测试** | Layer GPU 门禁 + 组合 D 系列 |
| L.06 | Mask layer | **已有测试** | `TestP1_Capability_L06_*` |
| I.01–I.03 | Image 采样 | **已有测试** | Image GPU 门禁 / `I03` |
| P.01–P.06 | Paint stroke/fill/rrect/caps | **已有测试** | S3a/S3b + `P04/P05/P06` |
| B.01–B.06 | Blend / premul / alpha | **已有测试** | SourceOver 固定像素 + blend 门禁；alpha 可精修不阻塞 |
| H.* | Path fill 规则 | **已有测试** | `H03 EvenOdd` 等 |
| S.03 | PresentFrame | **已有测试** | `TestS3c_M3_PresentFrame_*` / X11 可选 |
| Damage | FrameDamage / PresentDamage | **已有测试** | D63/D152 + S4.4；S5.2 固化惯例 |
| T.01–T.03 | CTM / scale / HiDPI | **已有测试** | damage_scale + transform 组合 |
| Q.01/Q.03 | MSAA / pixel snap | **已有测试** | S3b MSAA / Q03 |

**P0 阻塞项：无。**  
不进入 S5.4「必补实现」队列。

---

## 4. P1 对账结果

| ID | 能力 | 状态 | 结论 |
|----|------|------|------|
| L.05 | Backdrop layer | 已有测试 | `L05_BackdropLayerGPU` |
| F.*/Shadow | Filter / blur / shadow | 已有测试 | S3c ApplyBlur/Shadow 门禁 |
| P.dash / E.* | Dash / path effect / trim | 已有测试 | GPU dash + E03 |
| D.* | Gradient / pattern | 已有测试 | 多 stop / image pattern；sweep 等已知差异书面后置 |
| I.07 | Nine-patch | 已有测试 | DrawImageNine |
| I.04 | Bicubic / mipmap | 已有测试 | S3c |

**P1 阻塞项：无。**  
精修（质量/性能）归 S5.3/S4 后续，不挡控件入口。

---

## 5. P2 / 旁路

| ID | 能力 | 优先级 | 状态 |
|----|------|--------|------|
| B.07 Modulate 等 | 冷门 blend | P2 | **书面后置**（Plus 已 ✅） |
| B.06 预乘精修 | 边角质量 | P2 | 可精修，不阻塞 |
| CS.02 | F16 Context 全链路 | P2 | RT ✅；render Context 仍 8-bit — **书面后置** |
| K.02 高层 multi-draw | scene 间接绘制 | P2 | ABI/低层 ✅；高层后置 |
| V.03 Mesh | 自定义 mesh | P2 | 已有 indexed 门禁 |
| R.02 | PDF/SVG | **旁路** | ⬜ 不阻塞 S5 |
| I.08 真 multiplanar YUV | 媒体 | **旁路** | external ✅；真 YUV 后置 |
| Skia 绝对 FPS 报表 | 性能对表 | **旁路** | S5 用本机 present 预算 |

---

## 6. S5.4 输入（能力补丁队列）

| 队列 | 项 |
|------|----|
| **必补（阻塞）** | *空* |
| **建议观察（不阻塞关闭）** | B.06/B.07 边角、CS.02 Context 宽色域、窗口 X11 multi-rect 长期 e2e |
| **旁路** | R.02、真 YUV、Skia FPS 报表 |

→ **S5.4 退出条件**：确认 P0/P1 无阻塞缺口 + 回归绿（无需新 API 实现）。

---

## 7. 与后续子阶段关系

| 子阶段 | 依赖本对账 |
|--------|------------|
| S5.1 Present 基线 | P0 Present 已具备 → 可直接测 present-only |
| S5.2 帧模型 | Damage API 已具备 → 固化惯例 |
| S5.3 60fps | 主路径场景建立在 P0 能力上 |
| S5.4 补丁 | 队列为空则文档关闭 |
| S5.5 控件入口 | 引用本节 P0 清零结论 |

---

## 8. 退出条件

| 条件 | 状态 |
|------|------|
| P0/P1/P2/旁路清单完整 | ✅ |
| 每项已有测试 / 需补 / 后置 | ✅ |
| 阻塞 UI 的需补项列出（可为空） | ✅ **空** |
| 不实现控件层 | ✅ |

**S5.0 关闭。** 下一焦点：**S5.1 Present-only 基线**。

## 9. 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | 首版对账；P0/P1 无阻塞缺口 |
