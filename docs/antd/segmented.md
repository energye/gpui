# Segmented 分段控制器
> 来源：[Ant Design 6.5.x Segmented](https://ant.design/components/segmented)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：用于展示多个选项并允许用户选择其中单个选项。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

用于展示多个选项并允许用户选择其中单个选项。

**Segmented** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 垂直方向 | 纵向布局 |
| Block 分段选择器 | 宽度撑满父级 |
| 胶囊形状 | 复现「胶囊形状」视觉与布局 |
| 不可用 | disabled 态 |
| 受控模式 | 复现「受控模式」视觉与布局 |
| 自定义渲染 | 自定义渲染/插槽外观 |
| 动态数据 | 复现「动态数据」视觉与布局 |
| 三种大小 | 不同 size 档位 |
| 设置图标 | icon 与文本混排 |
| 只设置图标 | icon 与文本混排 |
| 配合 name 使用 | 复现「配合 name 使用」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `block`

- **说明**：将宽度调整为父元素宽度的选项
- **类型**：boolean
- **默认值**：false

#### `classNames`

- **说明**：用于自定义 Segmented 组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `disabled`

- **说明**：是否禁用
- **类型**：boolean
- **默认值**：false

#### `orientation`

- **说明**：排列方向
- **类型**：`horizontal` | `vertical`
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |
  | `vertical` | 垂直排布 |

#### `size`

- **说明**：控件尺寸
- **类型**：`large` | `medium` | `small`
- **默认值**：`medium`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `styles`

- **说明**：用于自定义 Segmented 组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `vertical`

- **说明**：排列方向，与 `orientation` 同时存在，以 `orientation` 优先
- **类型**：boolean
- **默认值**：`false`
- **版本**：5.21.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `orientation` | 官方取值 `orientation` |

#### `shape`

- **说明**：形状
- **类型**：`default` | `round`
- **默认值**：`default`
- **版本**：5.24.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `default` | 默认中性外观 |
  | `round` | 大圆角/胶囊 |

#### `icon`

- **说明**：分段项的显示图标
- **类型**：ReactNode
- **默认值**：-

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

- 用于展示多个选项并允许用户选择其中单个选项；
- 当切换选中选项时，关联区域的内容会发生变化。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **垂直方向**（`vertical.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **Block 分段选择器**（`block.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **胶囊形状**（`shape.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **不可用**（`disabled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **受控模式**（`controlled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **自定义渲染**（`custom.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **动态数据**（`dynamic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **三种大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **设置图标**（`with-icon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **只设置图标**（`icon-only.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **配合 name 使用**（`with-name.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 当前选中的值 |
| `defaultValue` | 非受控默认值 | 默认选中的值 |
| `onChange` | 值变化 | 选项变化时的回调函数 |
| `disabled` | 禁用 | 是否禁用 |
| `options` | 数据化 options | 数据化配置选项内容 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 垂直方向 | `vertical.tsx` | 否 |
| Block 分段选择器 | `block.tsx` | 否 |
| 胶囊形状 | `shape.tsx` | 否 |
| 不可用 | `disabled.tsx` | 否 |
| 受控模式 | `controlled.tsx` | 否 |
| 自定义渲染 | `custom.tsx` | 否 |
| 动态数据 | `dynamic.tsx` | 否 |
| 三种大小 | `size.tsx` | 否 |
| 设置图标 | `with-icon.tsx` | 否 |
| 只设置图标 | `icon-only.tsx` | 否 |
| 配合 name 使用 | `with-name.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 受控同步模式 | `controlled-two.tsx` | 是 |
| 统一高度 | `size-consistent.tsx` | 是 |
| 自定义组件 Token | `componentToken.tsx` | 是 |

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

> 自 `antd@4.20.0` 版本开始提供该组件。

### Segmented

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| block | 将宽度调整为父元素宽度的选项 | boolean | false | classNames | 用于自定义 Segmented 组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> | - | defaultValue | 默认选中的值 | string \| number | `options` 首项的值 | disabled | 是否禁用 | boolean | false | onChange | 选项变化时的回调函数 | function(value: string \| number) | options | 数据化配置选项内容 | string\[] \| number\[] \| SegmentedItemType\[] | [] | orientation | 排列方向 | `horizontal` \| `vertical` | `horizontal` | size | 控件尺寸 | `large` \| `medium` \| `small` | `medium` | styles | 用于自定义 Segmented 组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom) , CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom) , CSSProperties> | - | vertical | 排列方向，与 `orientation` 同时存在，以 `orientation` 优先 | boolean | `false` | 5.21.0 | × |
| value | 当前选中的值 | string \| number | shape | 形状 | `default` \| `round` | `default` | 5.24.0 | × |
| name | Segmented 下所有 `input[type="radio"]` 的 `name` 属性。若未设置，则将回退到随机生成的名称 | string 
### SegmentedItemType

| 属性 | 描述 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| className | 自定义类名 | string | - | icon | 分段项的显示图标 | ReactNode | - | tooltip | 分段项的工具提示 | string \| [TooltipProps](../tooltip/index.zh-CN.md#api) | - 
### 导入方式

```js
import { Segmented } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `block` | 将宽度调整为父元素宽度的选项 | boolean | false | — |
| `classNames` | 用于自定义 Segmented 组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `defaultValue` | 默认选中的值 | string \| number | `options` 首项的值 | — |
| `disabled` | 是否禁用 | boolean | false | — |
| `onChange` | 选项变化时的回调函数 | function(value: string \| number) | — | — |
| `options` | 数据化配置选项内容 | string\[] \| number\[] \| SegmentedItemType\[] | [] | — |
| `orientation` | 排列方向 | `horizontal` \| `vertical` | `horizontal` | — |
| `size` | 控件尺寸 | `large` \| `medium` \| `small` | `medium` | — |
| `styles` | 用于自定义 Segmented 组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `vertical` | 排列方向，与 `orientation` 同时存在，以 `orientation` 优先 | boolean | `false` | 5.21.0 |
| `value` | 当前选中的值 | string \| number | — | — |
| `shape` | 形状 | `default` \| `round` | `default` | 5.24.0 |
| `name` | Segmented 下所有 `input[type="radio"]` 的 `name` 属性。若未设置，则将回退到随机生成的名称 | string | — | 5.23.0 |
| `className` | 自定义类名 | string | - | — |
| `icon` | 分段项的显示图标 | ReactNode | - | — |
| `label` | 分段项的显示文本 | ReactNode | - | — |
| `tooltip` | 分段项的工具提示 | string \| [TooltipProps](../tooltip/index.zh-CN.md#api) | - | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Segmented** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **13** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/segmented
- 中文文档：https://ant.design/components/segmented-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/segmented
- 驱动 gpui kit：`segmented`
