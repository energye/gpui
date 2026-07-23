# Card 卡片
> 来源：[Ant Design 6.5.x Card](https://ant.design/components/card)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：通用卡片容器。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

通用卡片容器。

**Card** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 典型卡片 | card 风格容器 |
| 无边框 | bordered 网格线 |
| 简洁卡片 | card 风格容器 |
| 更灵活的内容展示 | 复现「更灵活的内容展示」视觉与布局 |
| 栅格卡片 | card 风格容器 |
| 预加载的卡片 | loading 指示与防重复 |
| 网格型内嵌卡片 | card 风格容器 |
| 内部卡片 | card 风格容器 |
| 带页签的卡片 | card 风格容器 |
| 支持更多内容配置 | 复现「支持更多内容配置」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `actions`

- **说明**：卡片操作组，位置在卡片底部
- **类型**：Array<ReactNode>
- **默认值**：-

#### `bordered`

- **说明**：是否有边框, 请使用 `variant` 替换
- **类型**：boolean
- **默认值**：true
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `variant` | 官方取值 `variant` |

#### `bodyStyle`

- **说明**：卡片内容区域样式，请使用 `styles.body` 替代
- **类型**：CSSProperties
- **默认值**：-

#### `variant`

- **说明**：形态变体
- **类型**：`outlined` | `borderless`
- **默认值**：`outlined`
- **版本**：5.24.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `outlined` | 描边空心 |
  | `borderless` | 无边框 |

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `cover`

- **说明**：卡片封面
- **类型**：ReactNode
- **默认值**：-

#### `extra`

- **说明**：卡片右上角的操作区域
- **类型**：ReactNode
- **默认值**：-

#### `headStyle`

- **说明**：卡片头部样式，请使用 `styles.header` 替代
- **类型**：CSSProperties
- **默认值**：-

#### `loading`

- **说明**：当卡片内容还在加载中时，可以用 loading 展示一个占位
- **类型**：boolean
- **默认值**：false

#### `size`

- **说明**：card 的尺寸
- **类型**：`medium` | `small`
- **默认值**：`medium`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `tabList`

- **说明**：页签标题列表
- **类型**：[TabItemType](/components/tabs-cn#tabitemtype)[]
- **默认值**：-

#### `title`

- **说明**：卡片标题
- **类型**：ReactNode
- **默认值**：-

#### `type`

- **说明**：卡片类型，可设置为 `inner` 或 不设置
- **类型**：string
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `inner` | 官方取值 `inner` |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `avatar`

- **说明**：头像/图标
- **类型**：ReactNode
- **默认值**：-

#### `description`

- **说明**：描述内容
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

最基础的卡片容器，可承载文字、列表、图片、段落，常用于后台概览页面。

### 2.2 核心功能（按官方示例拆解）

1. **典型卡片**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **无边框**（`border-less.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **简洁卡片**（`simple.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **更灵活的内容展示**（`flexible-content.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **栅格卡片**（`in-column.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **预加载的卡片**（`loading.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **网格型内嵌卡片**（`grid-card.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **内部卡片**（`inner.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **带页签的卡片**（`tabs.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **支持更多内容配置**（`meta.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `loading` | 加载中 | 当卡片内容还在加载中时，可以用 loading 展示一个占位 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 典型卡片 | `basic.tsx` | 否 |
| 无边框 | `border-less.tsx` | 否 |
| 简洁卡片 | `simple.tsx` | 否 |
| 更灵活的内容展示 | `flexible-content.tsx` | 否 |
| 栅格卡片 | `in-column.tsx` | 否 |
| 预加载的卡片 | `loading.tsx` | 否 |
| 网格型内嵌卡片 | `grid-card.tsx` | 否 |
| 内部卡片 | `inner.tsx` | 否 |
| 带页签的卡片 | `tabs.tsx` | 否 |
| 支持更多内容配置 | `meta.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 封面和操作区不渲染 body | `no-body-debug.tsx` | 是 |
| 按钮对齐 | `button-alignment-debug.tsx` | 是 |
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

```jsx
<Card title="卡片标题">卡片内容</Card>
```

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| actions | 卡片操作组，位置在卡片底部 | Array&lt;ReactNode> | - | activeTabKey | 当前激活页签的 key | string | - | ~~bordered~~ | 是否有边框, 请使用 `variant` 替换 | boolean | true | ~~bodyStyle~~ | 卡片内容区域样式，请使用 `styles.body` 替代 | CSSProperties | - | - | × |
| variant | 形态变体 | `outlined` \| `borderless` | `outlined` | 5.24.0 | 5.24.0 |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | cover | 卡片封面 | ReactNode | - | defaultActiveTabKey | 初始化选中页签的 key，如果没有设置 activeTabKey | string | `第一个页签的 key` | extra | 卡片右上角的操作区域 | ReactNode | - | hoverable | 鼠标移过时可浮起 | boolean | false | ~~headStyle~~ | 卡片头部样式，请使用 `styles.header` 替代 | CSSProperties | - | - | × |
| loading | 当卡片内容还在加载中时，可以用 loading 展示一个占位 | boolean | false | size | card 的尺寸 | `medium` \| `small` | `medium` | tabBarExtraContent | tab bar 上额外的元素 | ReactNode | - | tabList | 页签标题列表 | [TabItemType](/components/tabs-cn#tabitemtype)[] | - | tabProps | [Tabs](/components/tabs-cn#tabs) | - | - | title | 卡片标题 | ReactNode | - | type | 卡片类型，可设置为 `inner` 或 不设置 | string | - | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | onTabChange | 页签切换的回调 | (key) => void | - 
### Card.Grid

| 参数      | 说明             | 类型    | 默认值 | 版本 |
| --------- | ---------------- | ------- | ------ | ---- |
| hoverable | 鼠标移过时可浮起 | boolean | true   |      |

### Card.Meta

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| avatar | 头像/图标 | ReactNode | - | description | 描述内容 | ReactNode | - | title | 标题内容 | ReactNode | - 
### 导入方式

```js
import { Card } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `actions` | 卡片操作组，位置在卡片底部 | Array<ReactNode> | - | — |
| `activeTabKey` | 当前激活页签的 key | string | - | — |
| `bordered` | 是否有边框, 请使用 `variant` 替换 | boolean | true | — |
| `bodyStyle` | 卡片内容区域样式，请使用 `styles.body` 替代 | CSSProperties | - | - |
| `variant` | 形态变体 | `outlined` \| `borderless` | `outlined` | 5.24.0 |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `cover` | 卡片封面 | ReactNode | - | — |
| `defaultActiveTabKey` | 初始化选中页签的 key，如果没有设置 activeTabKey | string | `第一个页签的 key` | — |
| `extra` | 卡片右上角的操作区域 | ReactNode | - | — |
| `hoverable` | 鼠标移过时可浮起 | boolean | false | — |
| `headStyle` | 卡片头部样式，请使用 `styles.header` 替代 | CSSProperties | - | - |
| `loading` | 当卡片内容还在加载中时，可以用 loading 展示一个占位 | boolean | false | — |
| `size` | card 的尺寸 | `medium` \| `small` | `medium` | — |
| `tabBarExtraContent` | tab bar 上额外的元素 | ReactNode | - | — |
| `tabList` | 页签标题列表 | [TabItemType](/components/tabs-cn#tabitemtype)[] | - | — |
| `tabProps` | [Tabs](/components/tabs-cn#tabs) | - | - | — |
| `title` | 卡片标题 | ReactNode | - | — |
| `type` | 卡片类型，可设置为 `inner` 或 不设置 | string | - | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `onTabChange` | 页签切换的回调 | (key) => void | - | — |
| `avatar` | 头像/图标 | ReactNode | - | — |
| `description` | 描述内容 | ReactNode | - | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Card** 的验收清单：

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
- 官方文档：https://ant.design/components/card
- 中文文档：https://ant.design/components/card-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/card
- 驱动 gpui kit：`card`
