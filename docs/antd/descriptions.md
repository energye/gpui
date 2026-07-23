# Descriptions 描述列表
> 来源：[Ant Design 6.5.x Descriptions](https://ant.design/components/descriptions)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：展示多个只读字段的组合。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
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
实现 gpui kit 版 **Descriptions** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **8** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/descriptions
- 中文文档：https://ant.design/components/descriptions-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/descriptions
- 驱动 gpui kit：`descriptions`
