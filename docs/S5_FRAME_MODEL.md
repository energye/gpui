# S5.2 — Retained + Damage 帧模型

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S5.2 关闭**  
> 依赖：S5.1

---

## 1. 默认 UI 帧模型

```text
Bootstrap (cold):
  NewContext → 全量绘制 → PresentFrame(view)     // 可 LoadOpClear

Steady frame:
  复用同一 Context（retained）
  ResetFrameDamage()
  只绘制脏区域（clip + draw）
  PresentFrameDamageRects(view, FrameDamage())
```

---

## 2. API 惯例

| API | 用途 |
|-----|------|
| `ResetFrameDamage` | 稳态帧开始前清空 |
| `FrameDamage` / `FrameDamageUnion` | 收集脏区 |
| `PresentFrame` | 全量 present（bootstrap / 全屏切换） |
| `PresentFrameDamage` | 单 rect |
| `PresentFrameDamageRects` | 多 rect（ADR-028） |

### 硬规则

1. **禁止**用读回 wall-time 宣称 60fps。  
2. MSAA：**scissor ∩ damage**；不在 MSAA 上依赖 LoadOpLoad（ADR-021）。  
3. Blit-only：`LoadOpLoad` + multi scissor 真保留。  
4. HiDPI：damage 走物理像素（既有 `TestTrackDamage_HiDPI_*`）。  

---

## 3. 测试

| 测试 | 作用 |
|------|------|
| `TestS52_FrameModel_RetainedDamage` | bootstrap + multi-rect damage present |
| D63 / D152 | 既有组合门禁 |
| S5 U01–U03 | 稳态脏区场景 |

---

## 4. 退出条件

| 条件 | 状态 |
|------|------|
| 惯例文档化 | ✅ |
| retained + multi-damage e2e | ✅ |
| 无控件层 | ✅ |

**S5.2 关闭。** 下一：**S5.3 60fps 门禁**。
