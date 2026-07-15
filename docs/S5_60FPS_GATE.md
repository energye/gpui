# S5.3 — 主路径 60fps 门禁

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S5.3 关闭**  
> 依赖：S5.1 present-only + S5.2 帧模型

---

## 1. 门禁定义

| 项 | 值 |
|----|-----|
| 预算默认 | **16.7ms**（60Hz） |
| 指标 | present-only **p50**（非 avg，抑冷启动毛刺） |
| 覆盖 | `S5_FPS_BUDGET_MS` |
| 软过载 | `S5_ALLOW_SLOW=1`（机器过载时） |
| 路径 | 无 ReadPixels |

---

## 2. 场景

| ID | 模型 | 硬门禁 |
|----|------|--------|
| **U01** StaticShell | retained + damage（状态 chip） | ✅ ≤16.7ms p50 |
| **U02** ListScroll | retained + 3 行脏带 | ✅ |
| **U03** FormField | retained + 字段脏区 | ✅ |
| **U04** ModalStatic | retained 轻量全量 | ✅ |
| **U05** KitchenSink | 应力 | ❌ 只记录 |

测试：`TestS53_MainPath60FPS_Gate`。

---

## 3. 本机结果（与 S5.1 同机）

见 `tmp/s5_present_baseline.json`：U01–U04 的 p50 均 **低于 16.7ms**（U05 除外）。

---

## 4. 退出条件

| 条件 | 状态 |
|------|------|
| U01–U04 门禁绿 | ✅ |
| U05 不硬门 | ✅ |
| 书面可降级路径 | ✅ `S5_ALLOW_SLOW` / 预算 env |

**S5.3 关闭。** 下一：**S5.4 能力补丁**（S5.0 队列为空则文档关闭）。
