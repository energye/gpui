# Divider 分割线
> 来源：[Ant Design 6.5.x Divider](https://ant.design/components/divider)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：布局（Layout）  
> 说明：区隔内容的分割线。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
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
