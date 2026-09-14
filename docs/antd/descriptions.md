# Descriptions 描述列表
> 来源：[Ant Design 6.5.x Descriptions](https://ant.design/components/descriptions)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：展示多个只读字段的组合。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

展示多个只读字段的组合。

**Descriptions** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 带边框的 | bordered 网格线 |
| 自定义尺寸 | 不同 size 档位的高宽/字号/内边距 |
| 响应式 | 断点响应式 |
| 垂直 | 纵向布局 |
| 垂直带边框的 | 纵向布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |
| 整行 | 复现「整行」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `bordered`

- **说明**：是否展示边框
- **类型**：boolean
- **默认值**：false

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `contentStyle`

- **说明**：自定义内容样式，请使用 `styles.content` 替换
- **类型**：CSSProperties
- **默认值**：-
- **版本**：4.10.0

#### `extra`

- **说明**：描述列表的操作区域，显示在右上方
- **类型**：ReactNode
- **默认值**：-
- **版本**：4.5.0

#### `items`

- **说明**：描述列表项内容
- **类型**：[DescriptionsItem](#descriptionitem)[]
- **默认值**：-
- **版本**：5.8.0

#### `labelStyle`

- **说明**：自定义标签样式，请使用 `styles.label` 替换
- **类型**：CSSProperties
- **默认值**：-
- **版本**：4.10.0

#### `layout`

- **说明**：描述布局
- **类型**：`horizontal` | `vertical`
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |
  | `vertical` | 垂直排布 |

#### `size`

- **说明**：设置列表的大小。可以设置为 `medium` 、`small`, 或不填
- **类型**：`large` | `medium` | `small`
- **默认值**：`large`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `title`

- **说明**：描述列表的标题，显示在最顶部
- **类型**：ReactNode
- **默认值**：-

#### `label`

- **说明**：内容的描述
- **类型**：ReactNode
- **默认值**：-

#### `span`

- **说明**：包含列的数量（`filled` 铺满当前行剩余部分）
- **类型**：number| `filled` | [Screens](/components/grid-cn#col)
- **默认值**：1
- **版本**：`screens: 5.9.0`，`filled: 5.22.0`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `filled` | 浅底填充 |

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

常见于详情页的信息展示。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **带边框的**（`border.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **自定义尺寸**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **响应式**（`responsive.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **垂直**（`vertical.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **垂直带边框的**（`vertical-border.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **整行**（`block.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `items` | 数据化 items | 描述列表项内容 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 带边框的 | `border.tsx` | 否 |
| 复杂文本的情况 | `text.tsx` | 是 |
| 间距 | `padding.tsx` | 是 |
| 自定义尺寸 | `size.tsx` | 否 |
| 响应式 | `responsive.tsx` | 否 |
| 垂直 | `vertical.tsx` | 否 |
| 垂直带边框的 | `vertical-border.tsx` | 否 |
| 自定义 label & wrapper 样式 | `style.tsx` | 是 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| JSX demo | `jsx.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |
| 整行 | `block.tsx` | 否 |

### 2.7 组合关系

- **依赖等级 L2 组合件**：纯只读分组容器，无外部 kit 强依赖；`extra` 可嵌入 `Button` 等可交互控件（各自处理键盘/焦点）。
- **行算法**：对齐 `useRow`（满列换行/末项补齐/`filled` 收行），`column`/`span` 响应式 map + `ViewportWidth` 注入。
- **ConfigProvider**：尺寸、主题、全局 descriptions 默认（P1）。
- **文件归属**：`ui/kit/descriptions/`（`descriptions.go`，行/格 Flex 布局）。
---
## 3. 配置（API）
通用属性参考：[Common props](https://ant.design/docs/react/common-props)。

以下为官方 API 全文，作为 kit 配置面与类型设计的权威清单。

## API

通用属性参考：[通用属性](/docs/react/common-props)

### Descriptions

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| bordered | 是否展示边框 | boolean | false | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | colon | 配置 `Descriptions.Item` 的 `colon` 的默认值。表示是否显示 label 后面的冒号 | boolean | true | column | 一行的 `DescriptionItems` 数量，可以写成像素值或支持响应式的对象写法 `{ xs: 8, sm: 16, md: 24}` | number \| [Record<Breakpoint, number>](https://github.com/ant-design/ant-design/blob/84ca0d23ae52e4f0940f20b0e22eabe743f90dca/components/descriptions/index.tsx#L111C21-L111C56) | 3 | ~~contentStyle~~ | 自定义内容样式，请使用 `styles.content` 替换 | CSSProperties | - | 4.10.0 | × |
| extra | 描述列表的操作区域，显示在右上方 | ReactNode | - | 4.5.0 | × |
| items | 描述列表项内容 | [DescriptionsItem](#descriptionitem)[] | - | 5.8.0 | × |
| ~~labelStyle~~ | 自定义标签样式，请使用 `styles.label` 替换 | CSSProperties | - | 4.10.0 | × |
| layout | 描述布局 | `horizontal` \| `vertical` | `horizontal` | size | 设置列表的大小。可以设置为 `medium` 、`small`, 或不填 | `large` \| `medium` \| `small` | `large` | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | title | 描述列表的标题，显示在最顶部 | ReactNode | - 
### DescriptionItem

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| ~~contentStyle~~ | 自定义内容样式，请使用 `styles.content` 替换 | CSSProperties | - | 4.9.0 |
| label | 内容的描述 | ReactNode | - | span | 包含列的数量（`filled` 铺满当前行剩余部分） | number\| `filled` \| [Screens](/components/grid-cn#col) | 1 | `screens: 5.9.0`，`filled: 5.22.0` |

> span 是 Description.Item 的数量。 span={2} 会占用两个 DescriptionItem 的宽度。当同时配置 `style` 和 `labelStyle`（或 `contentStyle`）时，两者会同时作用。样式冲突时，后者会覆盖前者。

### 导入方式

```js
import { Descriptions } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `bordered` | 是否展示边框 | boolean | false | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `colon` | 配置 `Descriptions.Item` 的 `colon` 的默认值。表示是否显示 label 后面的冒号 | boolean | true | — |
| `column` | 一行的 `DescriptionItems` 数量，可以写成像素值或支持响应式的对象写法 `{ xs: 8, sm: 16, md: 24}` | number \| [Record](https://github.com/ant-design/ant-design/blob/84ca0d23ae52e4f0940f20b0e22eabe743f90dca/components/descriptions/index.tsx#L111C21-L111C56) | 3 | — |
| `contentStyle` | 自定义内容样式，请使用 `styles.content` 替换 | CSSProperties | - | 4.10.0 |
| `extra` | 描述列表的操作区域，显示在右上方 | ReactNode | - | 4.5.0 |
| `items` | 描述列表项内容 | [DescriptionsItem](#descriptionitem)[] | - | 5.8.0 |
| `labelStyle` | 自定义标签样式，请使用 `styles.label` 替换 | CSSProperties | - | 4.10.0 |
| `layout` | 描述布局 | `horizontal` \| `vertical` | `horizontal` | — |
| `size` | 设置列表的大小。可以设置为 `medium` 、`small`, 或不填 | `large` \| `medium` \| `small` | `large` | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `title` | 描述列表的标题，显示在最顶部 | ReactNode | - | — |
| `label` | 内容的描述 | ReactNode | - | — |
| `span` | 包含列的数量（`filled` 铺满当前行剩余部分） | number\| `filled` \| [Screens](/components/grid-cn#col) | 1 | `screens: 5.9.0`，`filled: 5.22.0` |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Descriptions** 的验收清单：

1. **配置面**：覆盖 §6.8 P0+P1 字段（items/title/extra/size/bordered/column/layout/colon/span+filled/响应式map）；semantic 函数形态 P1。
2. **视觉态**：顶栏/非边框/边框表/竖排/column-span/冒号规则（§6.4 DSC-S1~S8，§6.5）。
3. **尺寸态**：large / medium / small（竖档 16/12/8，横 24/24/16，§6.2）。
4. **受控/非受控**：不适用（只读展示；`ViewportWidth` 为显式注入）。
5. **数据驱动**：items[]（label/children/span/filled/spanMap）+ `useRow` 行算法。
6. **无障碍**：只读分组 `role=group`、label+内容配对、本体不抢焦点（§6.6）。
7. **RTL**：行内 label/内容顺序镜像；换行朗读序跟视觉。
8. **浮层**：无自带浮层。
9. **性能**：静态行格，无虚拟列表；断点切换瞬时重排。
10. **主题**：Token 化（§6.2 label Tertiary/内容 Text/底 FillSecondary）；无主路径动画。
11. **示例矩阵**：§6.8 P0 **8** 例（basic/border/size/responsive/vertical/vertical-border/style-class浅/block）；`text/padding/style/jsx` 归 P1。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/descriptions
- 中文文档：https://ant.design/components/descriptions-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/descriptions
- 驱动 gpui kit：`descriptions`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Descriptions** 补成 **可开发、可测试、可验收** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5.1** 官网截图 99.99% 相似（见 ACCEPTANCE 对齐定义）。只允字体光栅亚像素差；颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即不算对齐。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/descriptions/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Descriptions）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 数据渲染与选择/展开/分页/加载主路径 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | showcase 大图 + 官网并排 | showcase大图进testdata按§6.8摆全多种式样（CPU容差内比对）+ 与官网同示例截图并排验收（颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即挂，只允字体光栅亚像素差） | showcase/官网并排 |
| **L4** | 官网并排必验（非可选） | 建/大改基线时与官网截图并排验收并留验收记录（见L3），CI比showcase基线，人眼签官网并排 | 建/大改基线必验 |

**明确不做（Descriptions）：**

- 与官网截图逐字节哈希一致（只要求 99.99% 相似，允字体光栅亚像素差）。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，单测Skip写清平台原因）。  
- 官方 **debug** 示例不验收（仅参考）。  

> 控件说明：展示多个只读字段的组合。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。  
源码：`components/descriptions/style/index.ts` → `prepareComponentToken` + `genBorderedStyle` / `genDescriptionStyles`。

#### 6.2.1 几何与组件 Token

| 项 | 默认值（large） | medium | small | Token / 来源 |
| --- | --- | --- | --- | --- |
| 正文字号 | **14** | 14 | 14 | `fontSize` |
| 标题字号 | **16** | 16 | 16 | `fontSizeLG` |
| 标题下间距 | **20** | 20 | 20 | `titleMarginBottom` = `fontSizeSM × lineHeightSM` ≈ 12×1.666 |
| 非边框 item 下间距 | **16** | **12** | **8** | `itemPaddingBottom`：`padding` / antd `paddingSM` / `paddingXS` |
| 非边框 item 右间距 | **16** | 16 | 16 | `itemPaddingEnd` = `padding` |
| 边框格 padding 竖 | **16** | **12** | **8** | `padding` / antd `paddingSM` / `paddingXS` |
| 边框格 padding 横 | **24** | **24** | **16** | `paddingLG` / `paddingLG` / `padding` |
| 冒号左/右间距 | **2 / 8** | 同 | 同 | `colonMarginLeft`=`marginXXS/2`，`colonMarginRight`=`marginXS` |
| 圆角（view） | **8** | 8 | 8 | `borderRadiusLG` |
| 边框线宽 | **1** | 1 | 1 | `lineWidth` |
| 默认 column | **3** | — | — | 未设时 `DEFAULT_COLUMN_MAP` 在 md+ 为 3 |
| 默认响应式 column | xs=1 sm=2 md…xl=3 xxl=3 xxxl=4 | — | — | `components/descriptions/constant.ts` |
| Focus ring outset | ≈ **1.5px** 可见（可交互子节点） | | | 可调 |

> **注意：** 本库 Theme `TokenPaddingSM=8` 对应 antd `paddingXS`，**不是** antd `paddingSM=12`。Descriptions medium 的 12 用组件常量 `DefaultDescriptionsItemPadBottomMD`，勿直接读 `TokenPaddingSM`。

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 标签字色 | `colorTextTertiary`（组件 `labelColor`） | 非边框 / 边框 label 均用 |
| 内容字色 | `colorText`（`contentColor`） | |
| 标题 / extra 字色 | `colorText` | 标题 `fontWeightStrong` |
| 边框 label 底 | `colorFillSecondary`（≈ antd `colorFillAlter`） | bordered 表头格 |
| 表框 / 分割线 | `colorSplit` | bordered 外框与格线 |
| 容器底 | `colorBgContainer` | |
| 禁用（适用者） | `colorDisabledBg` / `colorDisabledText` | Descriptions 本体无 disabled API |

禁止硬编码品牌色作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据展示**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `bordered` | 是否展示边框 | boolean | **false** |
| `colon` | label 后是否显示冒号（仅非边框；边框态强制无冒号） | boolean | **true** |
| `column` | 一行的 `DescriptionItems` 数量；支持 number 或响应式 map `{ xs, sm, … }` | number \| Record\<Breakpoint, number\> | **3**（或 `DEFAULT_COLUMN_MAP`） |
| `extra` | 右上角操作区 | ReactNode | - |
| `items` | 描述列表项（`label` / `children` / `span`） | DescriptionsItem[] | - |
| `layout` | 描述布局 | `horizontal` \| `vertical` | **horizontal** |
| `size` | 列表尺寸（影响 item / 边框格 padding） | `large` \| `medium` \| `small` | **large** |
| `title` | 顶部标题 | ReactNode | - |
| `span`（Item） | 占用列数；`filled` 铺满当前行剩余 | number \| `filled` \| Screens | **1** |
| `styles` / `classNames` | 语义结构样式钩子 | Record / fn | -（P1 深度） |

**Item 字段：** `label`、`children`（内容）、`span`、可选 `key`。  
**配置优先级（通用）：** 显式 props > 组件默认 > ConfigProvider 全局默认（ConfigProvider 全局为 P1）。

### 6.4 交互状态机（L1）

```text
items 按 column 栅格排布
bordered 表框
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| DSC-S1 | 3 项 column=3 | 一行三格，列宽均分 |
| DSC-S2 | bordered=true | 外框+格线 `colorSplit`，label 格底 `colorFillSecondary`，不画冒号 |
| DSC-S3 | Item span=2（column=3） | 该格占两列宽，行内剩余项补齐后换行 |
| DSC-S4 | size=small/medium/large | 边框格与非边框项 padding 按 §6.2 竖档 8/12/16 切换 |
| DSC-S5 | title+extra | 标题左上、extra 右上同行可见；extra 可点节点独立命名 |
| DSC-S6 | layout=vertical | 每格 label 在上、内容在下竖排 |
| DSC-S7 | colon=false（非边框） | 不画冒号；bordered 时强制无冒号（与 antd 一致） |
| DSC-S8 | span=filled / 响应式 ColumnMap | filled 铺满当前行剩余并收行；ViewportWidth 命中断点列数切换 |
### 6.5 视觉 chrome 规则（L2 摘要）

| 部位 / 态 | 规则（源码 `style/index.ts` + `useRow` 行算法） |
| --- | --- |
| 顶栏 | title 左（16 `fontSizeLG` 加粗 `colorText`）+ extra 右上；标题下间距 **20**（`titleMarginBottom`） |
| 非边框行 | item 下间距 large **16** / medium **12** / small **8**，右间距 16；label `colorTextTertiary` + 冒号（左 2/右 8），内容 `colorText` |
| bordered 表 | 外框 + 格线 `colorSplit`（1px）；label 格底 `colorFillSecondary`；格 pad 竖 16/12/8、横 24/24/16；强制无冒号 |
| layout=vertical | 每格 label 在上、内容在下竖排；bordered 竖表同格线 |
| column/span | 默认 3 列均分；`span=2` 占两列宽后换行；`filled` 铺满剩余并收行；响应式 `ColumnMap` 按视口切列 |
| colon=false | 非边框不画冒号；bordered 恒无冒号 |
| 主题切换 | 色与间距随 Theme 更新（medium 12 走组件常量，非 `TokenPaddingSM`） |

**动效：** 本控件无主路径动画；断点换列 P0 瞬时。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | 根 `role=group`；纯展示容器，不设表格/展开/选中角色 |
| 命名 | 分组名=title 或 `AriaLabel`；每项按 label＋内容配对，`colon` 冒号仅视觉分隔不读出 |
| 键盘 | 本体不抢焦点、无键盘操作；extra 内可交互控件按各自语义处理键盘 |
| 焦点环 | 本体无 ring；extra 或内容区可聚焦子控件聚焦时 ring 可见（outset≈1.5px） |
| 顺序 | 断点换行后朗读序跟视觉行序（左→右、上→下）；`layout=vertical` 先 label 后内容，`span=filled` 整行不断序 |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 主路径行为（§6.1 L1 / §6.4 DSC-S1~S8） | **对等** | P0 L1 |
| 尺寸/色 Token（§6.2 竖档8/12/16 + 横24/16） | **对等** | P0 L2 |
| 行算法 `useRow`（满列换行/末项补齐/filled 收行） | **对等** | P0 L1 |
| 响应式 `column`/`span` map + `ViewportWidth` | **对等**（断点注入） | P0 L1 |
| 浅 `styles`（root/label/content） | kit `Style` 直配 | P0 |
| Semantic 函数形态 / 全节点深度 | P1 必做 | P1 |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 官网截图相似（99.99%） | **不做** | — |

### 6.8 能力范围（P0 / P1 全做）

#### P0（本阶段必须 1:1，与P1一次做完）

| 配置 / 能力 | 说明 |
| --- | --- |
| `items` | `label` / `children` / `span` / `span=filled` / 响应式 span map |
| `title` / `extra` | 顶栏标题 + 右上操作区 |
| `size` | `large`（默认）\| `medium` \| `small` → padding 见 §6.2 |
| `bordered` | 表框 + 格线 + label 底色 |
| `column` | number 默认 3；响应式 `ColumnMap` + `ViewportWidth` |
| `layout` | `horizontal`（默认）\| `vertical`（标签在上） |
| `colon` | 默认 true；bordered 时不画冒号（对齐 antd） |
| 行算法 | 对齐 `useRow`：满列换行、末项补齐、`filled` 收行 |
| 官方主路径示例 | 基本、带边框的、自定义尺寸、响应式、垂直、垂直带边框的、自定义语义结构的样式和类（浅 styles）、整行 |
| 度量 §6.2 | Token / Default 常量断言 |
| a11y §6.6 | 根 `group` + 可选 `AriaLabel`；结构可读 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（本阶段必须 1:1，与 P0 同标准；真做不了的单测 Skip 写清平台原因）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 函数形态 / 全 SemanticDOM 深度 | P1 必做（P0 仅浅 label/content/root Style） |
| 动画像素级 / 复杂虚拟列表 | P1 必做（本控件无主路径动画） |
| 浏览器-only API 或桌面无等价项 | 单测 Skip 写清平台原因 |
| ConfigProvider 全局默认 | P1 必做 |
| debug 示例 | 不验收（仅参考） |
| 其余示例 | text/padding/style/jsx/component-token/_semantic |

### 6.9 验收用例表（可测）

> 测试名建议：`TestDescriptions_PRD_<ID>` 或 gallery 场景 ID。  
> **P0+P1 相关用例全部通过** 才可宣称 Descriptions 完成 1:1。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| DSC-01 | L1 | NewDescriptions 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| DSC-02 | L1 | 3 项 column=3 | 一行三格 |
| DSC-03 | L1 | bordered | 表框 |
| DSC-04 | L1 | span=2 | 占两列 |
| DSC-05 | L1 | size | padding 变 |
| DSC-06 | L1 | title | 标题 |
| DSC-07 | L1 | layout=vertical | 标签在上 |
| DSC-08 | L1 | 复现官方示例「基本」（`basic.tsx`） | title「User Info」+ 5 项非边框三列可布局 |
| DSC-09 | L1 | 复现官方示例「带边框的」（`border.tsx`） | bordered 表框 + `span=2` 占两列可布局；无冒号 |
| DSC-10 | L1 | 复现官方示例「自定义尺寸」（`size.tsx`） | bordered 下 middle/small 格 pad（竖12/8）可断言 |
| DSC-11 | L1 | 复现官方示例「响应式」（`responsive.tsx`） | `span={xl:2}` 响应式 + `ViewportWidth` 切列可布局 |
| DSC-12 | L1 | 复现官方示例「垂直」（`vertical.tsx`） | `layout=vertical` 标签在上 + `span=2` 可布局 |
| DSC-13 | L1 | 复现官方示例「垂直带边框的」（`vertical-border.tsx`） | 竖排 bordered 表可布局 |
| DSC-14 | L1 | 复现官方示例「自定义语义结构的样式和类」（`style-class.tsx`） | 浅 `styles.label` 覆盖可布局（函数形态 P1） |
| DSC-15 | L1 | 复现官方示例「整行」（`block.tsx`） | `span=filled` 铺满剩余并收行可布局 |
| DSC-16 | L2 | 读取 §6.2 关键尺寸/间距 | 非边框竖档16/12/8、边框横24/16、冒号2/8、标题下20（±0.5px） |
| DSC-17 | L2 | 默认皮颜色 | label 走 `colorTextTertiary`、内容 `colorText`、label 底 bordered 时 `colorFillSecondary`；无硬编码品牌色 |
| DSC-18 | L2 | disabled 外观 | **不适用**（Descriptions 无 disabled API；extra 子控件各自处理）— 跳过 |
| DSC-19 | L1 | 键盘/焦点 | 本体不抢焦点；extra 内可交互子控件按各自语义可聚焦（§6.6） |
| DSC-20 | L3 | 关键态 golden 截图 | 与showcase基线一致（容差内）+官网并排验收通过 |
| DSC-21 | L4 | 与 ant.design 并排 | 官网并排验收记录（必验） |
| DSC-22 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
type DescriptionsItem struct {
  Key          string
  Label        string
  LabelNode    core.Node  // 非空时优先于 Label
  Children     string
  ChildrenNode core.Node  // 非空时优先于 Children
  Span         int        // 默认 1；>column 时按 column 钳制
  SpanFilled   bool       // true = filled，铺满当前行剩余
  SpanMap      map[string]int // 响应式 span（如 xs/sm/md），命中时优先于 Span
}

NewDescriptions(items ...DescriptionsItem) *Descriptions

// 数据 / 顶栏
SetItems(items ...DescriptionsItem) *Descriptions
SetTitle(title string) *Descriptions
SetTitleNode(node core.Node) *Descriptions
SetExtra(node core.Node) *Descriptions
// 形态
SetSize(size DescriptionsSize) *Descriptions            // DescriptionsLarge | DescriptionsMiddle | DescriptionsSmall
SetBordered(bordered bool) *Descriptions
SetLayout(layout DescriptionsLayout) *Descriptions      // DescriptionsHorizontal | DescriptionsVertical
SetColon(colon bool) *Descriptions
SetColumn(column int) *Descriptions
SetColumnMap(m map[string]int) *Descriptions            // 响应式 column（如 xs/sm/md/xl/xxl/xxxl）
SetViewportWidth(width float64) *Descriptions           // 响应式断点判定宽
// 浅语义样式（P0 仅三节点；函数形态/全 SemanticDOM 为 P1）
SetLabelStyle(style Style) *Descriptions
SetContentStyle(style Style) *Descriptions
SetStyle(style Style) *Descriptions
// 主题 / a11y / 查询 / 挂树
SetTheme(theme *Theme) *Descriptions
SetFace(face text.Face) *Descriptions
SetAriaLabel(label string) *Descriptions
ResolvedColumn() int
RowCount() int
RowSpans() [][]int
ItemPadBottom() float64
Node() core.Node
ChromeNode() core.Node
// 本控件无 value/onChange、无 disabled/loading 主 API
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Size | **large**（antd Descriptions 默认） |
| Bordered | false |
| Layout | horizontal |
| Colon | true |
| Column | 3（或 `DEFAULT_COLUMN_MAP` + ViewportWidth） |
| Item.Span | 1 |
| 其余 | 对齐 antd 6.5 §3 / §6.2 |

### 6.11 结构与绘制分层（实现提示）

```text
Decorated root (ExpandWidth)
  └─ Column
       ├─ header?  (title · spacer · extra)
       └─ view
            └─ Column of rows
                 └─ Row ExpandMax
                      └─ Flexible(grow=span) × N
                           └─ cell (horizontal: label[:]+content | vertical: label/content 列)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读 Default/字段/Token；行算法对齐 antd `useRow`。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 本控件无主路径 Ticker；子节点若自带 loading 各自 AttachTicker。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Descriptions 1:1 完成**：

1. §6.8 **P0+P1** 全部实现（官方非debug一个不少，真做不了的单测Skip写清平台原因）。  
2. §6.9 中 **P0+P1** 用例全部通过。
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 showcase大图+官网并排验收通过（控件可见时必需，见§6.1）。
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0+P1** 全部（官方非 debug 一个不少；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 必须进 gallery，真做不了的单测 Skip 写清平台原因。
6. `coverage.go` Notes：P0+P1 已对齐 `docs/antd/descriptions.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Descriptions 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围定义（无裁剪）。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
