# 11 · 坐标 · DPR · HitTest

> **状态（组级）：** A · **Wave：** — · 修订：2026-07-28  
> 拆自 `ENGINE_UI_RENDER_BASE`；状态码见 [00_meta](./00_meta.md)  
> **施工真源分册** — 改状态先改本文件，再同步 [README](./README.md)

## 1. 目标 / 非目标

**目标：** 逻辑像素、Y-down、DPR、HitTest 契约稳定（F07）。  
**非目标：** Transform 下精确逆 CTM 命中（见 [05](./05_transform.md)）。  
**已落地：** 全程逻辑 px；Host ScaleFactor；各 RO HitTest；Overlay 优先。

## 2. 依赖（交叉关联）

| 方向 | 分册 / 说明 |
|------|-------------|
| **前置** | [00_meta](./00_meta.md) |
| **同级可并行** | 见 [91_wave_order](./91_wave_order.md) |
| **解锁** | 全部指针与布局 |

## 3. 能力表（母表全文）

## §16 坐标 · DPR · HitTest 母表

| ID | Flutter 能力 | Flutter 参考 | UI 场景用途 | gpui.ui | gpui.render | scene/RO | 状态 | 证据 | 缺口/备注 | Wave |
|----|--------------|--------------|------------|---------|-------------|----------|------|------|-----------|------|
| FCoord-LOGICAL | 逻辑像素 | logical pixels / dp | 布局点击 | 全程逻辑 px | DeviceScale | Host.Size | A | 架构文 §4 | F07 | — |
| FCoord-PHYSICAL | 物理像素 | device pixels | 纹理 | — | PixelWidth/Height | Swapchain | A | render | — | — |
| FCoord-DPR | devicePixelRatio | MediaQuery.devicePixelRatio | HiDPI | Context.Scale | WithDeviceScale | Host.ScaleFactor | A | — | — | — |
| FCoord-YDOWN | Y 向下 | Flutter 布局 | 一致 | Y-down | 2D UI 惯例 | — | A | — | — | — |
| FCoord-HIT | HitTest | HitTestResult | 指针 | HitTest | — | 各 RO + Overlay 先 | A | embedder | — | — |

---



## 4. 代码落点

| 层 | 路径 |
|----|------|
| Size/Scale | platform Host · embedder |
| HitTest | 各 RO · overlay |
| DeviceScale | render |

## 5. 指标挂钩

与 M-INPUT-LAG（P1）、正确性门禁相邻。

完整定义 → [90_metrics](./90_metrics.md)。**无 baseline 的优化无效**（[00_meta](./00_meta.md)）。

## 6. 验收 DoD

- [x] 逻辑/物理/Y-down 架构  
- [x] Overlay 命中优先  
- [ ] Transform 逆变换 hit  
- [ ] 多窗坐标（P6 工具向）

## 7. 风险与非宣称

Transform 命中当前为 AABB 近似。

## 8. 相关分册

- [总集 README](./README.md) · [实施顺序 91](./91_wave_order.md) · [指标 90](./90_metrics.md) · [F 交叉 92](./92_f_crosswalk.md)
