# docs/ — 文档索引

> 日期：2026-07-26  
> **先读架构总览（四层图）：** [`ENGINE_ARCH_OVERVIEW.md`](./ENGINE_ARCH_OVERVIEW.md)  
> **架构文字真源（详细条款）：** [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md)  
> 与代码冲突时：架构以本文档集为准；实现落地后以 `engine/` 代码为准。

---

## 架构怎么读

| 顺序 | 文档 | 内容 |
|------|------|------|
| 1 | [`ENGINE_ARCH_OVERVIEW.md`](./ENGINE_ARCH_OVERVIEW.md) | **四层总图** · 第一目标 · 线程/一帧简图 · 分期 |
| 2 | [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md) | **真源全文**：L0–L4 边界 · 线程/脏区/F01–F18 · 阶段门禁 · 包结构 |

### 四层（一句话）

| 层 | 是什么 | 何时 |
|----|--------|------|
| **L1 UI 引擎** | 对齐 Flutter Engine + rendering | **当前第一目标** |
| **L2 框架壳** | 手势/焦点/Overlay/滚动视口（不是 Ant 控件） | 控件前 |
| **L3 产品控件** | Ant Kit | 后置（P7） |
| **L4 用户业务** | 页面组合控件 | 最后 |

---

## 文档清单

| 文档 | 用途 |
|------|------|
| [`ENGINE_ARCH_OVERVIEW.md`](./ENGINE_ARCH_OVERVIEW.md) | 总览图（一看就懂） |
| [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md) | 详细真源 |
| [`antd/`](./antd/) | Ant 组件**需求**（L3 后置用；不绑旧 ui/kit 实现） |
| [`README.md`](./README.md) | 本索引 |

---

## 维护规则

1. **架构变更**改 `ENGINE_FLUTTER_SKIA_ARCH.md`，图示同步 `ENGINE_ARCH_OVERVIEW.md`。  
2. **组件需求**只写在 `antd/`。  
3. **禁止**再写「在旧 ui 全树 Paint 上打补丁」的性能卡。  
4. 实现目录：`engine/`（L1/L2）、后置 `kit/`（L3）。  
5. **L1 真源 §5.1 未勾完，不得以 L3 控件库为主线。**
