# Divider 分割线
> 来源：[Ant Design 6.5.x Divider](https://ant.design/components/divider)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：布局（Layout）  
> 说明：区隔内容的分割线。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

区隔内容的分割线。

**Divider** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 水平分割线 | 横向布局 |
| 带文字的分割线 | 复现「带文字的分割线」视觉与布局 |
| 设置分割线的间距大小 | 不同 size 档位 |
| 分割文字使用正文样式 | 复现「分割文字使用正文样式」视觉与布局 |
| 垂直分割线 | 纵向布局 |
| 变体 | variant 线框/填充差异 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `children`

- **说明**：嵌套的标题
- **类型**：ReactNode
- **默认值**：-

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `dashed`

- **说明**：是否虚线
- **类型**：boolean
- **默认值**：false

#### `orientation`

- **说明**：水平或垂直类型
- **类型**：`horizontal` | `vertical`
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |
  | `vertical` | 垂直排布 |

#### `orientationMargin`

- **说明**：标题和最近 left/right 边框之间的距离，去除了分割线，同时 `titlePlacement` 不能为 `center`。如果传入 `string` 类型的数字且不带单位，默认单位是 px
- **类型**：string | number
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `titlePlacement` | 官方取值 `titlePlacement` |
  | `center` | 居中 |

#### `plain`

- **说明**：文字是否显示为普通正文样式
- **类型**：boolean
- **默认值**：false
- **版本**：4.2.0

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `size`

- **说明**：间距大小，仅对水平布局有效
- **类型**：`small` | `medium` | `large`
- **默认值**：-
- **版本**：5.25.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `small` | 小尺寸（更紧凑） |
  | `medium` | 中尺寸（默认节奏） |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |

#### `titlePlacement`

- **说明**：分割线标题的位置
- **类型**：`start` | `end` | `center`
- **默认值**：`center`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |
  | `center` | 居中 |

#### `type`

- **说明**：水平还是垂直类型
- **类型**：`horizontal` | `vertical`
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |
  | `vertical` | 垂直排布 |

#### `variant`

- **说明**：分割线是虚线、点线还是实线
- **类型**：`dashed` | `dotted` | `solid`
- **默认值**：solid
- **版本**：5.20.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `dashed` | 虚线边框 |
  | `dotted` | 官方取值 `dotted` |
  | `solid` | 实心填充 |

#### `vertical`

- **说明**：是否垂直，和 orientation 同时配置以 orientation 优先
- **类型**：boolean
- **默认值**：false

### 1.4 交互视觉状态（实现检查表）

| 状态 | 要求 |
| --- | --- |
| default | 默认色、边框、阴影符合 token |
| hover | 可交互控件需有悬停反馈 |
| active/pressed | 按下态对比或反馈（若适用） |
| focus | 可见 focus ring，键盘可达 |
| disabled | 降对比 + 禁止交互，布局稳定 |
| loading | 指示器 + 通常阻止重复触发 |
| error/warning | 与 status/Form 语义色一致 |

### 1.5 语义化 DOM 与主题

- 支持 `classNames` / `styles`；kit 应对齐语义节点钩子。

- 颜色、圆角、间距、动效走 Design Token；支持亮暗色与品牌色。

- 动效可关（reduced-motion / 全局 motion、wave 配置）。
---
## 2. 功能
### 2.1 使用场景

- 对不同章节的文本段落进行分割。
- 对行内文字/链接进行分割，例如表格的操作列。

### 2.2 核心功能（按官方示例拆解）

1. **水平分割线**（`horizontal.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **带文字的分割线**（`with-text.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **设置分割线的间距大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **分割文字使用正文样式**（`plain.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **垂直分割线**（`vertical.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **变体**（`variant.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 水平分割线 | `horizontal.tsx` | 否 |
| 带文字的分割线 | `with-text.tsx` | 否 |
| 设置分割线的间距大小 | `size.tsx` | 否 |
| 分割文字使用正文样式 | `plain.tsx` | 否 |
| 垂直分割线 | `vertical.tsx` | 否 |
| 样式自定义 | `customize-style.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |
| 变体 | `variant.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |

### 2.7 组合关系

- **Form**：录入类注意 `value`/`checked` 与 `valuePropName`。
- **ConfigProvider**：尺寸、主题、locale、空状态、默认 props。
- **App**：message / modal / notification 上下文。
- **浮层**：Modal/Drawer 内注意 `getPopupContainer`。
- **Space / Flex / Grid / Layout**：布局与间距。
---
## 3. 配置（API）
通用属性参考：[Common props](https://ant.design/docs/react/common-props)。

以下为官方 API 全文，作为 kit 配置面与类型设计的权威清单。

## API

通用属性参考：[通用属性](/docs/react/common-props)

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| children | 嵌套的标题 | ReactNode | - | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | dashed | 是否虚线 | boolean | false | orientation | 水平或垂直类型 | `horizontal` \| `vertical` | `horizontal` | - | × |
| ~~orientationMargin~~ | 标题和最近 left/right 边框之间的距离，去除了分割线，同时 `titlePlacement` 不能为 `center`。如果传入 `string` 类型的数字且不带单位，默认单位是 px | string \| number | - | plain | 文字是否显示为普通正文样式 | boolean | false | 4.2.0 | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | size | 间距大小，仅对水平布局有效 | `small` \| `medium` \| `large` | - | 5.25.0 | × |
| titlePlacement | 分割线标题的位置 | `start` \| `end` \| `center` | `center` | - | × |
| ~~type~~ | 水平还是垂直类型 | `horizontal` \| `vertical` | `horizontal` | - | × |
| variant | 分割线是虚线、点线还是实线 | `dashed` \| `dotted` \| `solid` | solid | 5.20.0 | × |
| vertical | 是否垂直，和 orientation 同时配置以 orientation 优先 | boolean | false | - | × |

### 导入方式

```js
import { Divider } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `children` | 嵌套的标题 | ReactNode | - | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `dashed` | 是否虚线 | boolean | false | — |
| `orientation` | 水平或垂直类型 | `horizontal` \| `vertical` | `horizontal` | - |
| `orientationMargin` | 标题和最近 left/right 边框之间的距离，去除了分割线，同时 `titlePlacement` 不能为 `center`。如果传入 `string` 类型的数字且不带单位，默认单位是 px | string \| number | - | — |
| `plain` | 文字是否显示为普通正文样式 | boolean | false | 4.2.0 |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `size` | 间距大小，仅对水平布局有效 | `small` \| `medium` \| `large` | - | 5.25.0 |
| `titlePlacement` | 分割线标题的位置 | `start` \| `end` \| `center` | `center` | - |
| `type` | 水平还是垂直类型 | `horizontal` \| `vertical` | `horizontal` | - |
| `variant` | 分割线是虚线、点线还是实线 | `dashed` \| `dotted` \| `solid` | solid | 5.20.0 |
| `vertical` | 是否垂直，和 orientation 同时配置以 orientation 优先 | boolean | false | - |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Divider** 的验收清单：

1. **配置面**：覆盖 API 表常用字段；冷门字段可分期但命名兼容。
2. **视觉态**：default / hover / active / focus / disabled / loading。
3. **尺寸态**：small / medium / large（适用者）。
4. **受控/非受控**：value+onChange 与 defaultValue。
5. **数据驱动**：options / items / columns / treeData / fileList 等。
6. **无障碍**：焦点、角色、键盘、读屏。
7. **RTL**：placement / orientation 镜像。
8. **浮层**：z-index、挂载容器、遮挡、滚动。
9. **性能**：虚拟列表、防抖、减少重绘。
10. **主题**：Token 化；支持 reduced-motion。
11. **示例矩阵**：官方非 debug 示例约 **7** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/divider
- 中文文档：https://ant.design/components/divider-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/divider
- 驱动 gpui kit：`divider`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Divider** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/divider/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Divider）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 布局参数驱动子项几何正确 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Divider）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：区隔内容的分割线。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`fontSize=14`、`fontSizeLG=16`、`lineWidth=1`）。实现必须通过 Token / `DefaultDivider*` 读取；下表为 Token 未覆盖时的回落。  
源码：`components/divider/style/index.ts`（`prepareComponentToken` + `genSharedDividerStyle` + `genSizeDividerStyle`）。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 水平无 size / size=large `marginBlock` | **24** | antd `marginLG`（无 `-sm`/`-md` class 时的默认） |
| size=medium（兼容 middle）`marginBlock` | **16** | antd `margin`（`-md` class） |
| size=small `marginBlock` | **8** | antd `marginXS`（`-sm` class） |
| 带文字水平 `marginBlock` 基线 | **16** | `dividerHorizontalWithTextGutterMargin` = `margin`；size class 仍可覆盖 |
| 线宽 | **1** | `lineWidth` |
| 标题默认字号（非 plain） | **16** | `fontSizeLG`；字重视觉≈500（kit 可用常规面 + 字号） |
| 标题 plain 字号 | **14** | `fontSize`；字重 normal；色 `colorText` |
| 标题 `paddingInline` | **1em** | 组件 Token `textPaddingInline` |
| `titlePlacement` start/end 默认轨宽比 | **0.05** | 组件 Token `orientationMargin`（近侧轨 `0.05*100%`，远侧 `0.95*100%`） |
| 垂直线高度 | **0.9em** | 相对当前正文字号（≈`0.9 * fontSize`） |
| 垂直线 `marginInline` | **8** | 组件 Token `verticalMarginInline` = antd `marginXS` |
| 垂直线垂直偏移 | **-0.06em** | 光学对齐（P0 可近似 0，L3 再抠） |

> **size 未设置**：antd 不挂 size class → 水平 `marginBlock=24`。  
> **size 仅水平有效**；垂直忽略 size。  
> **children + vertical**：antd 忽略文字（warning）；kit 同样不画标题。

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 分割线色 | `colorSplit` | 默认皮；未注册时回落 `colorBorderSecondary` / fill 浅色 |
| 标题默认色 | `colorTextHeading` 或 `colorText` | 非 plain |
| 标题 plain 色 | `colorText` | |
| 次级文本 | `colorTextSecondary` | 适用者 |
| 容器底 | `colorBgContainer` | 页面背景，非线色 |

禁止硬编码品牌色（如 `#1677FF`）作为唯一默认线色。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**布局**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `children` / Title | 嵌套标题（水平 with-text） | string / Node | 空 = 纯线 |
| `orientation` | 水平或垂直 | `horizontal` \| `vertical` | `horizontal` |
| `vertical` | 是否垂直；与 `orientation` 同时配置时 **orientation 优先** | boolean | false |
| `size` | 水平 `marginBlock` 档位 | `small` \| `medium` \| `large` | 未设 → 24（等同 large 节奏） |
| `variant` | 实线 / 虚线 / 点线 | `solid` \| `dashed` \| `dotted` | `solid` |
| `dashed` | 虚线糖；`variant=solid` 时仍可出虚线 | boolean | false |
| `plain` | 标题用正文字号/字重 | boolean | false |
| `titlePlacement` | 标题位置 | `start` \| `end` \| `center`（兼容 left/right） | `center` |
| `orientationMargin` | start/end 近侧比例或绝对边距（legacy） | number | 组件 Token 0.05 |
| `classNames` / `styles` | 语义钩子 root/rail/content | — | **P1** |

**有效线型：** `variant=dotted` > `variant=dashed` \|\| `dashed` > `solid`。  
**配置优先级（通用）：** 显式 props > 组件默认 > ConfigProvider（P1）。

### 6.4 交互状态机（L1）

```text
mount ──► orientation=horizontal|vertical
             ├── variant solid|dashed|dotted（dashed 糖）
             ├── children? + titlePlacement ──► rail-start · content · rail-end
             ├── plain ──► 标题 fontSize=14 / colorText
             ├── size ──► 水平 marginBlock（8/16/24）
             └── 无 hover / press / focus / loading 态
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| DIV-S1 | 默认 | 水平实线；`marginBlock≈24`；线宽 1；色 colorSplit |
| DIV-S2 | vertical / orientation=vertical | 行内竖线高≈0.9em；`marginInline≈8` |
| DIV-S3 | `dashed` 或 `variant=dashed` | 虚线轨 |
| DIV-S4 | `variant=dotted` | 点线轨 |
| DIV-S5 | 标题 + center（默认） | 双轨 flex 等分；标题居中 |
| DIV-S6 | `titlePlacement=start` | 近侧轨短（默认比 0.05）、远侧长 |
| DIV-S6b | `titlePlacement=end` | 近侧（右）轨短、远侧长 |
| DIV-S7 | plain | 标题字号≈14、字重 normal |
| DIV-S7b | 非 plain 标题 | 字号≈16（fontSizeLG） |
| DIV-S8 | size=small | `marginBlock≈8` |
| DIV-S8b | size=medium | `marginBlock≈16` |
| DIV-S8c | size=large / 未设 | `marginBlock≈24` |
| DIV-S9 | 线宽 | 1（Token lineWidth） |
| DIV-S10 | 线色 | colorSplit（Theme） |
| DIV-S11 | role | `separator`（装饰分隔） |

### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 符合 §6.2 Token；轨 + 可选标题 |
| hover/active/focus | **不适用**（非交互控件，无 ring / 无 press chrome） |
| disabled / loading | **不适用** |
| 主题切换 | 线色 / 字色 / 间距随 Theme 更新 |

**动效：** 无入场动画要求；P0 瞬时。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| role | 根节点 `role=separator` |
| 装饰分隔 | 默认无强制可聚焦；可选 `AriaLabel`；纯装饰不抢焦点 |
| 键盘 | 不适用（无激活键） |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 主路径行为（§6.1 L1） | **对等** | P0 L1 |
| 尺寸/色 Token（§6.2） | **对等** | P0 L2 |
| dashed/dotted 像素纹样 | **近似** stroke dash | P0 可见区分；像素级 P1 |
| 垂直 `-0.06em` 光学偏移 | **近似** | P0 可 0 |
| Semantic classNames/styles | kit 语义钩子 / Style 覆盖 | **P1**（gallery 可示意结构） |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 逐像素官网哈希 | **不做** | — |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `orientation` / `vertical` | 水平 + 垂直；orientation 优先 |
| `size` | small / medium / large / 未设 |
| `variant` + `dashed` | solid / dashed / dotted |
| `children` / Title | 带文字分割线 |
| `titlePlacement` | start / end / center |
| `plain` | 正文字号标题 |
| 官方主路径示例 | horizontal、with-text、size、plain、vertical、variant；style-class / _semantic **结构可挂载**（深度 P1） |
| 度量 §6.2 | Token / Default 断言 |
| a11y §6.6 | role=separator |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 深度（root/rail/content 函数式） | 分期；P0 可用 `Style` 做线色等一锤子覆盖 |
| `orientationMargin` 绝对 px + styles.content.margin 全矩阵 | 分期；P0 默认比例 0.05 |
| 垂直光学 top:-0.06em 像素级 | 分期 |
| dashed/dotted 与浏览器纹样逐像素 | 分期 |
| ConfigProvider 全局 Divider 默认 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestDivider_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Divider 完成 1:1 主路径。  
> L3/L4 与 P1 不阻塞本阶段 DoD。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| DIV-01 | L1 | `NewDivider()` | 不崩溃；默认 orientation=horizontal、variant=solid、size 未设、plain=false、titlePlacement=center |
| DIV-02 | L1 | 默认布局 | 水平实线；`marginBlock≈24`；线宽 1 |
| DIV-03 | L1 | `SetVertical(true)` 或 `SetOrientation(vertical)` | 竖线；高≈0.9×fontSize；`marginInline≈8` |
| DIV-04 | L1 | `SetDashed(true)` 或 `SetVariant(dashed)` | 有效线型 dashed |
| DIV-05 | L1 | `SetVariant(dotted)` | 有效线型 dotted |
| DIV-06 | L1 | `SetTitle("Text")` 默认 placement | 线-文-线；双轨 grow 等分 |
| DIV-07 | L1 | `titlePlacement=start` | 近侧轨 grow 比 0.05 / 远侧 0.95 |
| DIV-08 | L1 | `titlePlacement=end` | 近侧（右）轨短 |
| DIV-09 | L1 | plain + Title | 标题字号≈14 |
| DIV-10 | L1 | 非 plain + Title | 标题字号≈16 |
| DIV-11 | L1 | size=small / medium / large | marginBlock≈8 / 16 / 24 |
| DIV-12 | L1 | 线宽 | Thickness / lineWidth = 1 |
| DIV-13 | L1 | 线色 | 走 Theme colorSplit（或回落），非硬编码品牌主色 |
| DIV-14 | L1 | a11y | 根 `Role=separator` |
| DIV-15 | L1 | 官方 horizontal 组合（实线+dashed） | 可挂树布局；无 panic |
| DIV-16 | L1 | 官方 with-text 组合 | center/start/end 可挂树 |
| DIV-17 | L1 | 官方 size 组合 | 三档 margin 可断言 |
| DIV-18 | L1 | 官方 plain 组合 | 字号 14 |
| DIV-19 | L1 | 官方 vertical 组合 | 行内竖线可挂树 |
| DIV-20 | L1 | 官方 variant 组合 | solid/dotted/dashed 可区分 |
| DIV-21 | L1 | style-class / _semantic | 结构可挂载（深度 P1，不测 class 字符串） |
| DIV-22 | L2 | §6.2 关键 margin / 字号 / 线宽 | ±0.5px |
| DIV-23 | L2 | 默认皮颜色 | Theme Token；无硬编码 `#1677FF` 线色 |
| DIV-24 | L3 | 关键态 golden | 与仓库基线一致（AA）；本阶段可选 |
| DIV-25 | L4 | 与 ant.design 并排 | 人眼签字 |
| DIV-26 | P1 | §6.8 P1 任一能力 | 单独用例；Notes 标明 |

### 6.10 产品 API 契约（Go kit 侧）

> 允许 **breaking** 旧 API（`SetText` / 仅 `Root *primitive.Divider` 等）；以下为产品契约。

```text
type DividerOrientation  // Horizontal | Vertical
type DividerSize         // SizeUnset(0) | Small | Medium | Large  （Medium 兼容 middle）
type DividerVariant      // Solid | Dashed | Dotted
type DividerTitlePlacement // Center | Start | End  （Left→Start, Right→End）

NewDivider() *Divider
NewDividerWithTitle(title string) *Divider   // 可选糖

// 配置（P0）
SetOrientation(DividerOrientation)
SetVertical(bool)                 // orientation 已设时以 Orientation 为准
SetSize(DividerSize)
SetVariant(DividerVariant)
SetDashed(bool)                   // 糖：true 时若 variant=solid 则按 dashed 画
SetPlain(bool)
SetTitle(string)                  // children；空=纯线
SetTitleNode(core.Node)           // 可选；优先于 Title 字符串
SetTitlePlacement(DividerTitlePlacement)
SetOrientationMargin(ratio float64) // 0→默认 0.05；仅 start/end

// 主题 / 覆盖
SetTheme(*core.Theme)
SetStyle(Style)                   // Border→线色；Text→标题色；FontSize→标题字号
SetFace(text.Face)
SetAriaLabel(string)

// 查询（测试 / 组合）
EffectiveOrientation() DividerOrientation
EffectiveVariant() DividerVariant   // dotted > dashed|Dashed > solid
IsVertical() bool
MarginBlock() float64               // 水平上下外边距
LineWidth() float64
LineColor() render.RGBA             // 解析后 Token 色

// 挂树
Node() core.Node                    // 始终同一根，rebuild 替换子树
ChromeNode() core.Node              // 轨或 with-text 根（测试）
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Orientation | Horizontal |
| Vertical | false |
| Size | 未设（marginBlock=24） |
| Variant | Solid |
| Dashed | false |
| Plain | false |
| Title | "" |
| TitlePlacement | Center |
| OrientationMargin | 0 → 0.05 |
| 其余 | 对齐 antd 6.5 §3 |

### 6.11 结构与绘制分层（实现提示）

```text
// 水平无标题
Box/Flex root (padding = marginBlock 上下)
  └─ primitive.Divider  (rail, ColorToken=colorSplit, Dash?)

// 水平 with-text
Flex row root (padding = marginBlock 上下; CrossCenter; Gap≈0)
  ├─ Flexible(growStart) → primitive.Divider rail-start
  ├─ Text / TitleNode      (paddingInline ≈ 1em)
  └─ Flexible(growEnd)   → primitive.Divider rail-end

// 垂直
Box root (marginInline; Height≈0.9em)
  └─ primitive.Divider Vertical
```

- 组合 `ui/primitive` + `ui/core`；禁止第二套 Hit / 帧循环。  
- `rebuild()` 只读 Default / 字段 / Token；线型进 `primitive.Divider` Dash。  
- `hit == layout == paint`；根 `HitTransparent` 或 `HitDefer`（装饰）。  
- 无 Ticker（无 loading）。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Divider 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例（DIV-01…DIV-23）测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 本阶段可选（控件可见但非交互）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：Divider 页覆盖 horizontal / with-text / size / plain / vertical / variant；style-class 仅示意。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/divider.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Divider 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。

**本轮 §6 修订说明（相对批量模板）：**  
- **§6.2**：按 antd `style/index.ts` 重写 margin 档（未设/large=24、medium=16、small=8）、标题字号 LG/plain、orientationMargin=0.05、vertical 0.9em；去掉无关 controlHeight/focus ring/圆角。  
- **§6.3 / §6.4**：补 effective variant 规则、DIV-S* 细则、role=separator。  
- **§6.5 / §6.6**：明确非交互、无 focus ring / disabled。  
- **§6.7 / §6.8**：semantic / orientationMargin 深度 / 光学偏移 标 P1。  
- **§6.9**：用例改为可断言的 L1/L2；style-class 深度 P1。  
- **§6.10 / §6.11**：具体 Go API 与 rail/content 分层。
