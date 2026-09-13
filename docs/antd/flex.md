# Flex 弹性布局
> 来源：[Ant Design 6.5.x Flex](https://ant.design/components/flex)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：布局（Layout）  
> 说明：用于对齐的弹性布局容器。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

用于对齐的弹性布局容器。

**Flex** 的视觉即排布本身：主轴方向（`vertical`/`orientation`）+ 间隙（`gap`）+ 主轴/交叉轴对齐（`justify`/`align`）+ 换行（`wrap`）。无颜色、无交互态，只验几何（见 §6.4–§6.5）。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本布局 | 复现「基本布局」视觉与布局 |
| 对齐方式 | 复现「对齐方式」视觉与布局 |
| 设置间隙 | 复现「设置间隙」视觉与布局 |
| 自动换行 | 复现「自动换行」视觉与布局 |
| 组合使用 | 复现「组合使用」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `vertical`

- **说明**：flex 主轴的方向是否垂直，使用 `flex-direction: column`
- **类型**：boolean
- **默认值**：false
- **版本**：5.10.0

#### `wrap`

- **说明**：设置元素单行显示还是多行显示
- **类型**：[flex-wrap](https://developer.mozilla.org/zh-CN/docs/Web/CSS/flex-wrap) | boolean
- **默认值**：nowrap
- **版本**：boolean: 5.17.0

#### `justify`

- **说明**：设置元素在主轴方向上的对齐方式
- **类型**：[justify-content](https://developer.mozilla.org/zh-CN/docs/Web/CSS/justify-content)
- **默认值**：normal

#### `align`

- **说明**：设置元素在交叉轴方向上的对齐方式
- **类型**：[align-items](https://developer.mozilla.org/zh-CN/docs/Web/CSS/align-items)
- **默认值**：normal

#### `gap`

- **说明**：设置网格之间的间隙
- **类型**：`small` | `medium` | `large` | string | number
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `small` | 小尺寸（更紧凑） |
  | `medium` | 中尺寸（默认节奏） |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |

#### `component`

- **说明**：自定义元素类型
- **类型**：React.ComponentType
- **默认值**：`div`

#### `orientation`

- **说明**：主轴的方向类型
- **类型**：`horizontal` | `vertical`
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |
  | `vertical` | 垂直排布 |

### 1.4 交互视觉状态（实现检查表）

不适用——Flex 为纯布局容器，无颜色、无 hover / active / focus / disabled / loading 态，只验子项几何（见 §6.5）。

### 1.5 语义化 DOM 与主题

- 仅根布局容器 + 子项；无装饰区、无浮层（`popup/mask` 不适用）。

- 间隙走 `gap` Token（small 8 / medium 16 / large 24，见 §6.2.1）；无颜色 Token、无动效。
---
## 2. 功能
### 2.1 使用场景

- 适合设置元素之间的间距。
- 适合设置各种水平、垂直对齐方式。

### 与 Space 组件的区别 {#difference-with-space-component}

- Space 为内联元素提供间距，其本身会为每一个子元素添加包裹元素用于内联对齐。适用于行、列中多个子元素的等距排列。
- Flex 为块级元素提供间距，其本身不会添加包裹元素。适用于垂直或水平方向上的子元素布局，并提供了更多的灵活性和控制能力。

### 2.2 核心功能（按官方示例拆解）

1. **基本布局**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **对齐方式**（`align.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **设置间隙**（`gap.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **自动换行**（`wrap.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **组合使用**（`combination.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本布局 | `basic.tsx` | 否 |
| 对齐方式 | `align.tsx` | 否 |
| 设置间隙 | `gap.tsx` | 否 |
| 自动换行 | `wrap.tsx` | 否 |
| 组合使用 | `combination.tsx` | 否 |
| 调试专用 | `debug.tsx` | 是 |

### 2.7 组合关系

- **依赖等级**：**L1 无依赖基础件**；纯弹性布局，不依赖组内其他 10 件，可先行实现。
- **等谁**：无；常作子项容器被引用，不同文件并行安全。
- **文件归属**：`ui/kit/flex/`（只改自己文件，并行安全）。
- **ConfigProvider**：主题（间隙 Token）、默认 props。

> debug 标记依据：`debug.tsx` 文件名含 debug，为内部调试用例，不计 P0。
---
## 3. 配置（API）
通用属性参考：[Common props](https://ant.design/docs/react/common-props)。

以下为官方 API 全文，作为 kit 配置面与类型设计的权威清单。

## API

> 自 `antd@5.10.0` 版本开始提供该组件。Flex 组件默认行为在水平模式下，为向上对齐，在垂直模式下，为拉伸对齐，你可以通过属性进行调整。

通用属性参考：[通用属性](/docs/react/common-props)

| 属性 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| vertical | flex 主轴的方向是否垂直，使用 `flex-direction: column` | boolean | false | 5.10.0 | 5.10.0 |
| wrap | 设置元素单行显示还是多行显示 | [flex-wrap](https://developer.mozilla.org/zh-CN/docs/Web/CSS/flex-wrap) \| boolean | nowrap | boolean: 5.17.0 | × |
| justify | 设置元素在主轴方向上的对齐方式 | [justify-content](https://developer.mozilla.org/zh-CN/docs/Web/CSS/justify-content) | normal | align | 设置元素在交叉轴方向上的对齐方式 | [align-items](https://developer.mozilla.org/zh-CN/docs/Web/CSS/align-items) | normal | flex | flex CSS 简写属性 | [flex](https://developer.mozilla.org/zh-CN/docs/Web/CSS/flex) | normal | gap | 设置网格之间的间隙 | `small` \| `medium` \| `large` \| string \| number | - | component | 自定义元素类型 | React.ComponentType | `div` | orientation | 主轴的方向类型 | `horizontal` \| `vertical` | `horizontal` | - | × |

### 导入方式

```js
import { Flex } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `vertical` | flex 主轴的方向是否垂直，使用 `flex-direction: column` | boolean | false | 5.10.0 |
| `wrap` | 设置元素单行显示还是多行显示 | [flex-wrap](https://developer.mozilla.org/zh-CN/docs/Web/CSS/flex-wrap) \| boolean | nowrap | boolean: 5.17.0 |
| `justify` | 设置元素在主轴方向上的对齐方式 | [justify-content](https://developer.mozilla.org/zh-CN/docs/Web/CSS/justify-content) | normal | — |
| `align` | 设置元素在交叉轴方向上的对齐方式 | [align-items](https://developer.mozilla.org/zh-CN/docs/Web/CSS/align-items) | normal | — |
| `flex` | flex CSS 简写属性 | [flex](https://developer.mozilla.org/zh-CN/docs/Web/CSS/flex) | normal | — |
| `gap` | 设置网格之间的间隙 | `small` \| `medium` \| `large` \| string \| number | - | — |
| `component` | 自定义元素类型 | React.ComponentType | `div` | — |
| `orientation` | 主轴的方向类型 | `horizontal` \| `vertical` | `horizontal` | - |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Flex** 的验收清单：

1. **配置面**：`vertical`/`orientation`、`gap`、`justify`、`align`、`wrap`（最小排布语义，见 §6.3–§6.4）。
2. **几何**：无颜色、无交互态，只验子项位置与间隙（§6.2 gap 8/16/24，§6.9 FLX-02…09）。
3. **示例矩阵**：P0 按 §6.8（5 例）、余下按 P1 分期；与 §4 例数打架以 §6.8 为准。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/flex
- 中文文档：https://ant.design/components/flex-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/flex
- 驱动 gpui kit：`flex`

---

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Flex** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/flex/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Flex）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 布局参数驱动子项几何正确 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Flex）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：用于对齐的弹性布局容器。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

> antd 源：`flexGapSM/MD/LG = paddingXS/padding/paddingLG`，默认算法 `paddingXS=sizeXS=8`、`padding=16`、`paddingLG=24`。  
> kit Theme 里 `TokenPaddingXS` 现为 **4**（偏 xxs），**Flex 不得直用该值当 small gap**；small 走 `DefaultFlexGapSmall=8`（或等价 Size 回落）。

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| Flex gap small | **8** | antd `paddingXS`(=sizeXS)；kit `DefaultFlexGapSmall` |
| Flex gap medium / middle | **16** | `padding` / `TokenPadding` |
| Flex gap large | **24** | `paddingLG` / `TokenPaddingLG` |
| gap 数字 | 透传 px（±0.5px） | 显式 `SetGap` 覆盖预设 |

> Flex 为无皮布局容器，无字号/圆角/边框/Focus ring 自有 chrome（子项自理）。

#### 6.2.2 颜色 Token（语义）

Flex 为纯布局，无自有颜色：子项底色/边框走子控件自身 Token；禁止为 Flex 硬编码品牌色。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**布局**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `vertical` | flex 主轴的方向是否垂直，使用 `flex-direction: column` | boolean | false |
| `wrap` | 设置元素单行显示还是多行显示 | `bool` / flex-wrap | false（nowrap） |
| `justify` | 设置元素在主轴方向上的对齐方式 | justify-content 枚举 | normal → start |
| `align` | 设置元素在交叉轴方向上的对齐方式 | align-items 枚举 | normal（水平 start / 垂直 stretch） |
| `flex` | 容器自身作 flex item 的 CSS 简写（P1） | string / number | normal |
| `gap` | 子项间隙 | `small` \| `medium`/`middle` \| `large` \| number(px) | 未设 = 0 |
| `component` | 自定义元素类型（浏览器-only，P1） | React.ComponentType | `div` |
| `orientation` | 主轴方向；与 `vertical` 二选一，**orientation 优先** | `horizontal` \| `vertical` | `horizontal` |

**配置优先级（通用）：** 显式 props > 组件默认 > ConfigProvider 全局默认（Flex 无受控 `value/open` 语义）。

> **最小排布语义**：`orientation`/`vertical` 定主轴方向 → `justify` 定主轴分布 → `align` 定交叉轴对齐（未设时水平 start / 垂直 stretch）→ `gap` 定子项间隙 → `wrap` 定窄宽是否换行；五者共同决定子项几何，无颜色无交互。

### 6.4 交互状态机（L1）

```text
direction/gap/justify/align/wrap 布局 children
```

\*gap 8/16/24。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| FLX-S1 | 默认两子（gap=0，horizontal） | 两子顶坐标差≤0.5px，第二子左=第一子右±0.5px |
| FLX-S2 | vertical=true | 两子左坐标差≤0.5px，第二子顶=第一子底±0.5px |
| FLX-S3 | gap=middle | 相邻子间距 16±0.5px |
| FLX-S4 | gap=large | 相邻子间距 24±0.5px |
| FLX-S5 | justify=space-between，容器宽 600，两子宽 100 | 首子左=容器左±0.5px，尾子右=容器右±0.5px |
| FLX-S6 | align=center，子高 20/40 | 矮子交叉轴中线与行中线差≤0.5px |
| FLX-S7 | wrap=true，窄容器（子宽和>容器宽） | 第二项顶坐标大于第一项顶坐标，行数≥2 |
| FLX-S8 | gap=8 数字 | 相邻子间距 8±0.5px |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 子项按 §6.4 排布，间隙符合 §6.2（8/16/24 或数字 px） |
| hover/active/focus/disabled/loading | **不适用**（纯布局容器，无颜色无交互态） |
| 主题切换 | 间隙随 Theme 更新（无颜色切换） |


**动效：** 无（纯布局，无入场动画要求）。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 布局容器 | 无强制 role；可选 `AriaLabel` 命名 landmark（无装饰分隔/把手/浮层） |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 主路径行为（§6.1 L1） | **对等** | P0 L1 |
| 间隙度量（§6.2 gap 8/16/24） | **对等** | P0 L2 |
| `justify`/`align`/`wrap` 排布语义 | **对等** | P0 L1 |
| `flex` CSS 简写（容器作 item） | P1 分期 | P1 |
| `component` 自定义元素类型 | 浏览器-only，P1 不做 | P1 |
| Semantic classNames/styles | kit 语义钩子 | P1 |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 逐像素官网哈希 | **不做** | — |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `orientation` / `vertical` | 主轴方向；`orientation` 优先于 `vertical` 糖 |
| `gap` | `small`\|`medium`/`middle`\|`large`\|数字 px；s/m/l = 8/16/24 |
| `justify` | start / center / end / space-between / space-around / space-evenly |
| `align` | start / center / end / stretch；未设时水平 start、垂直 stretch（antd） |
| `wrap` | bool；窄主轴多行 |
| 官方主路径示例 | 基本布局、对齐方式、设置间隙、自动换行、组合使用 |
| 度量 §6.2 | gap / 字号 / 圆角 Token 断言 |
| a11y §6.6 | 布局容器最低：可选 `AriaLabel`；无装饰分隔/把手 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| `flex` CSS 简写（容器作 flex item） | 分期；item 侧用 `primitive.Flexible` |
| `component` 自定义元素类型 | 浏览器-only |
| `wrap-reverse` 等非 bool wrap | 分期 |
| semantic classNames/styles 深度 | 分期 |
| ConfigProvider 全局 `flex` 默认 | 分期 |
| 动画像素级 / 复杂虚拟列表 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |

**6 例→P0/P1 剪裁对应表**（§2.4 全量；P0=§6.8 主路径 5 例全收，1 例 debug 不计）：

| 示例 | 裁剪 | 原因 |
| --- | --- | --- |
| 基本布局/对齐方式/设置间隙/自动换行/组合使用 | P0 | 方向+对齐+间隙+换行主路径，gallery 必备 |
| 调试专用 | 不计 | 文件名含 debug 的内部用例 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestFlex_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Flex 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| FLX-01 | L1 | NewFlex 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| FLX-02 | L1 | 默认两子（gap=0，horizontal） | 两子顶坐标差≤0.5px，第二子左=第一子右±0.5px |
| FLX-03 | L1 | vertical=true | 两子左坐标差≤0.5px，第二子顶=第一子底±0.5px |
| FLX-04 | L1 | gap=middle | 相邻子间距 16±0.5px |
| FLX-05 | L1 | gap=large | 相邻子间距 24±0.5px |
| FLX-06 | L1 | justify=space-between，容器宽 600，两子宽 100 | 首子左=容器左±0.5px，尾子右=容器右±0.5px |
| FLX-07 | L1 | align=center，子高 20/40 | 矮子交叉轴中线与行中线差≤0.5px |
| FLX-08 | L1 | wrap=true，窄容器 | 行数≥2，第二项顶坐标大于第一项顶坐标 |
| FLX-09 | L1 | gap=8 数字 | 相邻子间距 8±0.5px |
| FLX-10 | L1 | 挂 `basic.tsx`（两子，gap=small） | 两子顶差≤0.5px，间距 8±0.5px |
| FLX-11 | L1 | 挂 `align.tsx`（子高 20/40，align=center） | 矮子中线与行中线差≤0.5px |
| FLX-12 | L1 | 挂 `gap.tsx`（small/middle/large 三行） | 三行间距分别为 8/16/24（±0.5px） |
| FLX-13 | L1 | 挂 `wrap.tsx`（wrap=true，窄容器） | 行数≥2，第二项顶>第一项顶 |
| FLX-14 | L1 | 挂 `combination.tsx`（vertical+justify+align 组合） | 主轴垂直，子左差≤0.5px，间距=gaps±0.5px |
| FLX-15 | L2 | 读取 §6.2 gap small/middle/large | 8/16/24（±0.5px） |
| FLX-16 | L2 | 默认皮颜色 | 无自有颜色；无硬编码品牌色 |
| FLX-17 | L2 | disabled 外观 | **不适用**（纯布局无禁用态） |
| FLX-18 | L1 | 键盘/焦点主路径 | **不适用**（容器不聚焦；子项自理） |
| FLX-19 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| FLX-20 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| FLX-21 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API（删除 `NewFlexRow` / `NewFlexColumn` 薄封装亦可，以 `NewFlex` + `SetOrientation`/`SetVertical` 为准）。  
> 实现可微调命名但 **P0 语义不可丢**。

```text
NewFlex(children ...core.Node) *Flex

SetOrientation(FlexOrientation)     // horizontal | vertical；优先于 Vertical
SetVertical(bool)                   // antd vertical 糖
SetWrap(bool)
SetJustify(FlexJustify)             // start|center|end|space-between|around|evenly
SetAlign(FlexAlign)                 // auto|start|center|end|stretch
SetGapSize(FlexGapSize)             // unset|small|medium|large  → 0/8/16/24
SetGap(px float64)                  // 数字间隙；显式覆盖 preset
Add / SetChildren / ClearChildren
SetTheme(*Theme)
SetAriaLabel(string)                // 可选命名
Node() core.Node
// 解析：EffectiveOrientation / ResolvedGap / IsVertical
// 布局容器：无 Disabled/Loading/OnClick（不适用）
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Orientation | horizontal（`vertical=false`） |
| Wrap | false |
| Justify | start（antd `normal`） |
| Align | auto → 水平 CrossStart / 垂直 CrossStretch |
| Gap | unset = 0；small/medium/large = 8/16/24 |
| 其余 | 对齐 antd 6.5 §3 表；`flex`/`component` 见 P1 |

### 6.11 结构与绘制分层（实现提示）

```text
Flex root（Row/Column，gap=ResolvedGap）
  └─ child × N（justify 主轴分布，align 交叉轴对齐）
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Flex 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/flex.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Flex 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
