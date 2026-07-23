# Space 间距
> 来源：[Ant Design 6.5.x Space](https://ant.design/components/space)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：布局（Layout）  
> 说明：设置组件之间的间距。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

设置组件之间的间距。

**Space** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本用法 | 复现「基本用法」视觉与布局 |
| 垂直间距 | 纵向布局 |
| 间距大小 | 不同 size 档位 |
| 对齐 | 复现「对齐」视觉与布局 |
| 自动换行 | 复现「自动换行」视觉与布局 |
| 分隔符 | 复现「分隔符」视觉与布局 |
| 紧凑布局组合 | 复现「紧凑布局组合」视觉与布局 |
| Button 紧凑布局 | 复现「Button 紧凑布局」视觉与布局 |
| 垂直方向紧凑布局 | 纵向布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `align`

- **说明**：对齐方式
- **类型**：`start` | `end` |`center` |`baseline`
- **默认值**：-
- **版本**：4.2.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |
  | `center` | 居中 |
  | `baseline` | 官方取值 `baseline` |

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props: SpaceProps })=> Record
- **默认值**：-

#### `direction`

- **说明**：间距方向
- **类型**：`vertical` | `horizontal`
- **默认值**：`horizontal`
- **版本**：4.1.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `vertical` | 垂直排布 |
  | `horizontal` | 水平排布 |

#### `orientation`

- **说明**：间距方向
- **类型**：`vertical` | `horizontal`
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `vertical` | 垂直排布 |
  | `horizontal` | 水平排布 |

#### `size`

- **说明**：间距大小
- **类型**：[Size](#size) | [Size\[\]](#size)
- **默认值**：`small`
- **版本**：4.1.0 | Array: 4.9.0

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props: SpaceProps })=> Record
- **默认值**：-

#### `vertical`

- **说明**：是否垂直，和 `orientation` 同时配置以 `orientation` 优先
- **类型**：boolean
- **默认值**：false
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `orientation` | 官方取值 `orientation` |

#### `wrap`

- **说明**：是否自动换行，仅在 `horizontal` 时有效
- **类型**：boolean
- **默认值**：false
- **版本**：4.9.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |

#### `block`

- **说明**：将宽度调整为父元素宽度的选项
- **类型**：boolean
- **默认值**：false
- **版本**：4.24.0

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

避免组件紧贴在一起，拉开统一的空间。

- 适合行内元素的水平间距。
- 可以设置各种水平对齐方式。
- 需要表单组件之间紧凑连接且合并边框时，使用 Space.Compact（自 `antd@4.24.0` 版本开始提供该组件）。

### 与 Flex 组件的区别 {#difference-with-flex-component}

- Space 为内联元素提供间距，其本身会为每一个子元素添加包裹元素用于内联对齐。适用于行、列中多个子元素的等距排列。
- Flex 为块级元素提供间距，其本身不会添加包裹元素。适用于垂直或水平方向上的子元素布局，并提供了更多的灵活性和控制能力。

### 2.2 核心功能（按官方示例拆解）

1. **基本用法**（`base.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **垂直间距**（`vertical.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **间距大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **对齐**（`align.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **自动换行**（`wrap.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **分隔符**（`separator.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **紧凑布局组合**（`compact.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **Button 紧凑布局**（`compact-buttons.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **垂直方向紧凑布局**（`compact-button-vertical.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本用法 | `base.tsx` | 否 |
| 垂直间距 | `vertical.tsx` | 否 |
| 间距大小 | `size.tsx` | 否 |
| 对齐 | `align.tsx` | 否 |
| 自动换行 | `wrap.tsx` | 否 |
| 分隔符 | `separator.tsx` | 否 |
| 紧凑布局组合 | `compact.tsx` | 否 |
| Button 紧凑布局 | `compact-buttons.tsx` | 否 |
| 垂直方向紧凑布局 | `compact-button-vertical.tsx` | 否 |
| 调试 Input 前置/后置标签 | `compact-debug.tsx` | 是 |
| 紧凑布局嵌套 | `compact-nested.tsx` | 是 |
| 多样的 Child | `debug.tsx` | 是 |
| Flex gap 样式 | `gap-in-line.tsx` | 是 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 自定义主题 | `component-token.tsx` | 是 |

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

### Space

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| align | 对齐方式 | `start` \| `end` \|`center` \|`baseline` | - | 4.2.0 | × |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props: SpaceProps })=> Record<[SemanticDOM](#semantic-dom), string> | - | ~~direction~~ | 间距方向 | `vertical` \| `horizontal` | `horizontal` | 4.1.0 | × |
| orientation | 间距方向 | `vertical` \| `horizontal` | `horizontal` | size | 间距大小 | [Size](#size) \| [Size\[\]](#size) | `small` | 4.1.0 \| Array: 4.9.0 | 5.6.0 |
| ~~split~~ | 设置分隔符, 请使用 `separator` 替换 | ReactNode | - | 4.7.0 | × |
| separator | 设置分隔符 | ReactNode | - | - | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props: SpaceProps })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | vertical | 是否垂直，和 `orientation` 同时配置以 `orientation` 优先 | boolean | false | - | × |
| wrap | 是否自动换行，仅在 `horizontal` 时有效 | boolean | false | 4.9.0 | × |

### Size

`'small' | 'medium' | 'large' | number`

### Space.Compact

需要表单组件之间紧凑连接且合并边框时，使用 Space.Compact，支持的组件有：

- Button
- AutoComplete
- Cascader
- DatePicker
- Input/Input.Search
- InputNumber
- Select
- TimePicker
- TreeSelect

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| block | 将宽度调整为父元素宽度的选项 | boolean | false | 4.24.0 |
| ~~direction~~ | 指定排列方向 | `vertical` \| `horizontal` | `horizontal` | 4.24.0 |
| orientation | 指定排列方向 | `vertical` \| `horizontal` | `horizontal` | vertical | 是否垂直，和 `orientation` 同时配置以 `orientation` 优先 | boolean | false | - |

### Space.Addon

> 自 antd@5.29.0 版本开始提供该组件。

用于在紧凑布局中创建自定义单元格。

| 参数     | 说明       | 类型      | 默认值 | 版本   |
| -------- | ---------- | --------- | ------ | ------ |
| children | 自定义内容 | ReactNode | -      | 5.29.0 |

### 导入方式

```js
import { Space } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `align` | 对齐方式 | `start` \| `end` \|`center` \|`baseline` | - | 4.2.0 |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props: SpaceProps })=> Record | - | — |
| `direction` | 间距方向 | `vertical` \| `horizontal` | `horizontal` | 4.1.0 |
| `orientation` | 间距方向 | `vertical` \| `horizontal` | `horizontal` | — |
| `size` | 间距大小 | [Size](#size) \| [Size\[\]](#size) | `small` | 4.1.0 \| Array: 4.9.0 |
| `split` | 设置分隔符, 请使用 `separator` 替换 | ReactNode | - | 4.7.0 |
| `separator` | 设置分隔符 | ReactNode | - | - |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props: SpaceProps })=> Record | - | — |
| `vertical` | 是否垂直，和 `orientation` 同时配置以 `orientation` 优先 | boolean | false | - |
| `wrap` | 是否自动换行，仅在 `horizontal` 时有效 | boolean | false | 4.9.0 |
| `block` | 将宽度调整为父元素宽度的选项 | boolean | false | 4.24.0 |
| `children` | 自定义内容 | ReactNode | - | 5.29.0 |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Space** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **10** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/space
- 中文文档：https://ant.design/components/space-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/space
- 驱动 gpui kit：`space`
