# Collapse 折叠面板
> 来源：[Ant Design 6.5.x Collapse](https://ant.design/components/collapse)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：可以折叠/展开的内容区域。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

可以折叠/展开的内容区域。

**Collapse** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 折叠面板 | 复现「折叠面板」视觉与布局 |
| 面板尺寸 | 不同 size 档位的高宽/字号/内边距 |
| 手风琴 | 复现「手风琴」视觉与布局 |
| 面板嵌套 | 复现「面板嵌套」视觉与布局 |
| 简洁风格 | 复现「简洁风格」视觉与布局 |
| 自定义面板 | 自定义渲染/插槽外观 |
| 隐藏箭头 | arrow 指示 |
| 额外节点 | 复现「额外节点」视觉与布局 |
| 幽灵折叠面板 | 透明/反色底 |
| 可折叠触发区域 | 复现「可折叠触发区域」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `bordered`

- **说明**：带边框风格的折叠面板
- **类型**：boolean
- **默认值**：true

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `expandIcon`

- **说明**：自定义切换图标
- **类型**：(panelProps) => ReactNode
- **默认值**：-

#### `expandIconPlacement`

- **说明**：设置图标位置
- **类型**：`start` | `end`
- **默认值**：`start`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |

#### `expandIconPosition`

- **说明**：设置图标位置，请使用 `expandIconPlacement` 替换
- **类型**：`start` | `end`
- **默认值**：-
- **版本**：4.21.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |

#### `ghost`

- **说明**：使折叠面板透明且无边框
- **类型**：boolean
- **默认值**：false
- **版本**：4.4.0

#### `size`

- **说明**：设置折叠面板大小
- **类型**：`large` | `medium` | `small`
- **默认值**：`medium`
- **版本**：5.2.0
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

#### `extra`

- **说明**：自定义渲染每个面板右上角的内容
- **类型**：ReactNode
- **默认值**：-

#### `label`

- **说明**：面板标题
- **类型**：ReactNode
- **默认值**：-

#### `header`

- **说明**：面板标题
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

- 对复杂区域进行分组和隐藏，保持页面的整洁。
- `手风琴` 是一种特殊的折叠面板，只允许单个内容区域展开。

### 2.2 核心功能（按官方示例拆解）

1. **折叠面板**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **面板尺寸**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **手风琴**（`accordion.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **面板嵌套**（`mix.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **简洁风格**（`borderless.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **自定义面板**（`custom.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **隐藏箭头**（`noarrow.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **额外节点**（`extra.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **幽灵折叠面板**（`ghost.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **可折叠触发区域**（`collapsible.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onChange` | 值变化 | 切换面板的回调 |
| `items` | 数据化 items | 折叠项目内容 |
| `destroyOnHidden` | 隐藏销毁 | 销毁折叠隐藏的面板 |
| `activeKey` | 激活面板 | 当前激活 tab 面板的 key |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 折叠面板 | `basic.tsx` | 否 |
| 面板尺寸 | `size.tsx` | 否 |
| 手风琴 | `accordion.tsx` | 否 |
| 面板嵌套 | `mix.tsx` | 否 |
| 简洁风格 | `borderless.tsx` | 否 |
| 自定义面板 | `custom.tsx` | 否 |
| 隐藏箭头 | `noarrow.tsx` | 否 |
| 额外节点 | `extra.tsx` | 否 |
| 幽灵折叠面板 | `ghost.tsx` | 否 |
| 可折叠触发区域 | `collapsible.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 组件 Token | `component-token.tsx` | 是 |

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

### Collapse

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| accordion | 手风琴模式 | boolean | false | activeKey | 当前激活 tab 面板的 key | string\[] \| string <br/> number\[] \| number | [手风琴模式](#collapse-demo-accordion)下默认第一个元素 | bordered | 带边框风格的折叠面板 | boolean | true | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | collapsible | 所有子面板是否可折叠或指定可折叠触发区域 | `header` \| `icon` \| `disabled` | - | 4.9.0 | × |
| defaultActiveKey | 初始化选中面板的 key | string\[] \| string<br/> number\[] \| number | - | ~~destroyInactivePanel~~ | 销毁折叠隐藏的面板 | boolean | false | destroyOnHidden | 销毁折叠隐藏的面板 | boolean | false | 5.25.0 | × |
| expandIcon | 自定义切换图标 | (panelProps) => ReactNode | - | expandIconPlacement | 设置图标位置 | `start` \| `end` | `start` | - | × |
| ~~expandIconPosition~~ | 设置图标位置，请使用 `expandIconPlacement` 替换 | `start` \| `end` | - | 4.21.0 | × |
| ghost | 使折叠面板透明且无边框 | boolean | false | 4.4.0 | × |
| size | 设置折叠面板大小 | `large` \| `medium` \| `small` | `medium` | 5.2.0 | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | onChange | 切换面板的回调 | function | - | items | 折叠项目内容 | [ItemType](#itemtype) | - | 5.6.0 | × |

### ItemType

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| classNames | 语义化结构 className | [`Record<header \| body, string>`](#semantic-dom) | - | 5.21.0 |
| collapsible | 是否可折叠或指定可折叠触发区域 | `header` \| `icon` \| `disabled` | - | extra | 自定义渲染每个面板右上角的内容 | ReactNode | - | key | 对应 activeKey | string \| number | - | showArrow | 是否展示当前面板上的箭头（为 false 时，collapsible 不能设为 icon） | boolean | true 
### Collapse.Panel

:::warning{title=已废弃}
版本 >= 5.6.0 时请使用 items 方式配置面板。
:::

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| collapsible | 是否可折叠或指定可折叠触发区域 | `header` \| `icon` \| `disabled` | - | 4.9.0 (icon: 4.24.0) |
| extra | 自定义渲染每个面板右上角的内容 | ReactNode | - | header | 面板标题 | ReactNode | - | showArrow | 是否展示当前面板上的箭头（为 false 时，collapsible 不能设为 icon） | boolean | true 
```js
import { Collapse } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `accordion` | 手风琴模式 | boolean | false | — |
| `activeKey` | 当前激活 tab 面板的 key | string\[] \| string  number\[] \| number | [手风琴模式](#collapse-demo-accordion)下默认第一个元素 | — |
| `bordered` | 带边框风格的折叠面板 | boolean | true | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `collapsible` | 所有子面板是否可折叠或指定可折叠触发区域 | `header` \| `icon` \| `disabled` | - | 4.9.0 |
| `defaultActiveKey` | 初始化选中面板的 key | string\[] \| string number\[] \| number | - | — |
| `destroyInactivePanel` | 销毁折叠隐藏的面板 | boolean | false | — |
| `destroyOnHidden` | 销毁折叠隐藏的面板 | boolean | false | 5.25.0 |
| `expandIcon` | 自定义切换图标 | (panelProps) => ReactNode | - | — |
| `expandIconPlacement` | 设置图标位置 | `start` \| `end` | `start` | - |
| `expandIconPosition` | 设置图标位置，请使用 `expandIconPlacement` 替换 | `start` \| `end` | - | 4.21.0 |
| `ghost` | 使折叠面板透明且无边框 | boolean | false | 4.4.0 |
| `size` | 设置折叠面板大小 | `large` \| `medium` \| `small` | `medium` | 5.2.0 |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `onChange` | 切换面板的回调 | function | - | — |
| `items` | 折叠项目内容 | [ItemType](#itemtype) | - | 5.6.0 |
| `children` | body 区域内容 | ReactNode | - | — |
| `extra` | 自定义渲染每个面板右上角的内容 | ReactNode | - | — |
| `forceRender` | 被隐藏时是否渲染 body 区域 DOM 结构 | boolean | false | — |
| `key` | 对应 activeKey | string \| number | - | — |
| `label` | 面板标题 | ReactNode | - | - |
| `showArrow` | 是否展示当前面板上的箭头（为 false 时，collapsible 不能设为 icon） | boolean | true | — |
| `header` | 面板标题 | ReactNode | - | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Collapse** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **11** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/collapse
- 中文文档：https://ant.design/components/collapse-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/collapse
- 驱动 gpui kit：`collapse`
