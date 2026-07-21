# S5.0 — Skia 控件向能力对账

> 版本：1.1 | 日期：2026-07-21  
> 状态：**S5.0 关闭**（历史对账保留）  
> 依据：`docs/SKIA_2D_CAPABILITY_MATRIX.md`  
> **现网引擎缺口真源：[`ENGINE_GAPS.md`](./ENGINE_GAPS.md)**（2026-07-21 起）

---

## 1. 方法（S5 当时）

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

## 3. P0 / P1 对账结果（S5 关闭时）

**P0 阻塞项：无。** **P1 阻塞项：无。**  
（绘制 API / 像素门禁维度；证据见当时 `TestP1_*` / S3* 门禁。）

细节表见 git 历史；**不再在此维护逐 ID 复制**，以免与能力矩阵分叉。  
权威能力状态：[`SKIA_2D_CAPABILITY_MATRIX.md`](./SKIA_2D_CAPABILITY_MATRIX.md)。

---

## 4. 2026-07-21 引擎侧补充（非 S5「缺 API」）

S5.0 关闭的是 **「控件主路径绕不开的绘制 API」**。  
后续源码与 antd 引擎职责评估表明，**仍属引擎必跟** 的是深度/效率/稳，见：

| 缺口 | 摘要 | 真源 |
|------|------|------|
| **G1 文本** | CFF 轮廓、复杂 shaping、Input/长文深度 | `ENGINE_GAPS` G1 |
| **G2 脏区效率** | 矢量 MSAA 常 LoadOpClear；OS Present damage no-op | G2 |
| **G3 稳定性** | 重层/滤镜/多 RT + lifecycle/VRAM soak | G3 |

这些 **不推翻**「P0 绘制 API 可开工控件层」；它们是 **生产级画布后端** 的后续必补。

---

## 5. 旁路（仍成立）

| ID | 能力 | 状态 |
|----|------|------|
| R.02 | PDF/SVG | ⬜ 不阻塞画布 100% |
| I.08 真 multiplanar YUV | 媒体 | 后置 |
| CS.02 Context 宽色域全链 | F16 RT 有；Context 8-bit | 后置 |

---

## 6. 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | 首版对账；P0/P1 无阻塞缺口 |
| 2026-07-21 | 1.1 | 指向 `ENGINE_GAPS`；压缩重复表；与现网对齐 |
