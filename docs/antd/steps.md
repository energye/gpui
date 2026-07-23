# Steps 步骤条
> 来源：[Ant Design 6.5.x Steps](https://ant.design/components/steps)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：导航（Navigation）  
> 说明：引导用户按照流程完成任务的导航条。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

引导用户按照流程完成任务的导航条。

**Steps** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本用法 | 复现「基本用法」视觉与布局 |
| 步骤运行错误 | 复现「步骤运行错误」视觉与布局 |
| 竖直方向的步骤条 | 复现「竖直方向的步骤条」视觉与布局 |
| 可点击 | 复现「可点击」视觉与布局 |
| 面板式步骤 | 复现「面板式步骤」视觉与布局 |
| 带图标的步骤条 | icon 与文本混排 |
| 标签放置位置与进度 | placement 方位 |
| 限量展示 | 复现「限量展示」视觉与布局 |
| 点状步骤条 | 复现「点状步骤条」视觉与布局 |
| 导航步骤 | 复现「导航步骤」视觉与布局 |
| 内联步骤 | 复现「内联步骤」视觉与布局 |
| 内联样式组合 | 复现「内联样式组合」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `current`

- **说明**：指定当前步骤，从 0 开始记数。在子 Step 元素中，可以通过 `status` 属性覆盖状态
- **类型**：number
- **默认值**：0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `status` | 官方取值 `status` |

#### `direction`

- **说明**：指定步骤条方向。目前支持水平（`horizontal`）和竖直（`vertical`）两种方向
- **类型**：string
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |
  | `vertical` | 垂直排布 |

#### `iconRender`

- **说明**：自定义渲染图标，请优先使用 `items.icon`
- **类型**：(oriNode, info: { index, active, item }) => ReactNode
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `items.icon` | 官方取值 `items.icon` |

#### `labelPlacement`

- **说明**：指定标签放置位置，默认水平放图标右侧，可选 `vertical` 放图标下方
- **类型**：string
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `vertical` | 垂直排布 |

#### `maxCount`

- **说明**：最大可见步骤项数量（`>= 3`）。超出数量的步骤区间会聚合成禁用的省略号步骤。
- **类型**：number
- **默认值**：-

#### `orientation`

- **说明**：指定步骤条方向。目前支持水平（`horizontal`）和竖直（`vertical`）两种方向
- **类型**：string
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |
  | `vertical` | 垂直排布 |

#### `percent`

- **说明**：当前 `process` 步骤显示的进度条进度（只对基本类型的 Steps 生效）
- **类型**：number
- **默认值**：-
- **版本**：4.5.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `process` | 进行中 |

#### `progressDot`

- **说明**：点状步骤条，可以设置为一个 function，请使用 `type="dot"` 替代。`titlePlacement` 将强制为 `vertical`
- **类型**：boolean | (iconDot, { index, status, title, content }) => ReactNode
- **默认值**：false
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `titlePlacement` | 官方取值 `titlePlacement` |
  | `vertical` | 垂直排布 |

#### `responsive`

- **说明**：当屏幕宽度小于 `532px` 时自动变为垂直模式
- **类型**：boolean
- **默认值**：true
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `532px` | 官方取值 `532px` |

#### `size`

- **说明**：指定大小，目前支持普通（`medium`）和迷你（`small`）
- **类型**：string
- **默认值**：`medium`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `status`

- **说明**：指定当前步骤的状态，可选 `wait` `process` `finish` `error`
- **类型**：string
- **默认值**：`process`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `wait` | 等待 |
  | `process` | 进行中 |
  | `finish` | 完成 |
  | `error` | 错误红语义 |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `titlePlacement`

- **说明**：指定标签放置位置，默认水平放图标右侧，可选 `vertical` 放图标下方
- **类型**：string
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `vertical` | 垂直排布 |

#### `type`

- **说明**：步骤条类型，可选 `default` `dot` `inline` `navigation` `panel`
- **类型**：string
- **默认值**：`default`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `default` | 默认中性外观 |
  | `dot` | 点状 |
  | `inline` | 内联紧凑 |
  | `navigation` | 导航式 |
  | `panel` | 面板式 |

#### `variant`

- **说明**：设置样式变体
- **类型**：`filled` | `outlined`
- **默认值**：`filled`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `filled` | 浅底填充 |
  | `outlined` | 描边空心 |

#### `content`

- **说明**：步骤的详情描述，可选
- **类型**：ReactNode
- **默认值**：-

#### `description`

- **说明**：步骤的详情描述，可选
- **类型**：ReactNode
- **默认值**：-

#### `disabled`

- **说明**：禁用点击
- **类型**：boolean
- **默认值**：false

#### `icon`

- **说明**：步骤图标的类型，可选
- **类型**：ReactNode
- **默认值**：-

#### `subTitle`

- **说明**：子标题
- **类型**：ReactNode
- **默认值**：-

#### `title`

- **说明**：标题
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

当任务复杂或者存在先后关系时，将其分解成一系列步骤，从而简化任务。

### 2.2 核心功能（按官方示例拆解）

1. **基本用法**（`simple.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **步骤运行错误**（`error.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **竖直方向的步骤条**（`vertical.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **可点击**（`clickable.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **面板式步骤**（`panel.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **带图标的步骤条**（`icon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **标签放置位置与进度**（`title-placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **限量展示**（`max-count.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **点状步骤条**（`progress-dot.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **导航步骤**（`nav.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **内联步骤**（`inline.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **内联样式组合**（`inline-variant.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onChange` | 值变化 | 点击切换步骤时触发 |
| `disabled` | 禁用 | 禁用点击 |
| `items` | 数据化 items | 配置选项卡内容 |
| `current` | 当前步骤/页 | 指定当前步骤，从 0 开始记数。在子 Step 元素中，可以通过 `status` 属性覆盖状态 |
| `percent` | 进度值 | 当前 `process` 步骤显示的进度条进度（只对基本类型的 Steps 生效） |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本用法 | `simple.tsx` | 否 |
| 步骤运行错误 | `error.tsx` | 否 |
| 竖直方向的步骤条 | `vertical.tsx` | 否 |
| 可点击 | `clickable.tsx` | 否 |
| 面板式步骤 | `panel.tsx` | 否 |
| 带图标的步骤条 | `icon.tsx` | 否 |
| 步骤切换 | `step-next.tsx` | 是 |
| 标签放置位置与进度 | `title-placement.tsx` | 否 |
| 限量展示 | `max-count.tsx` | 否 |
| 点状步骤条 | `progress-dot.tsx` | 否 |
| 自定义点状步骤条 | `customized-progress-dot.tsx` | 是 |
| 导航步骤 | `nav.tsx` | 否 |
| 带有进度的步骤 | `progress.tsx` | 是 |
| Progress Debug | `progress-debug.tsx` | 是 |
| Steps 嵌套 Steps | `steps-in-steps.tsx` | 是 |
| 内联步骤 | `inline.tsx` | 否 |
| 内联样式组合 | `inline-variant.tsx` | 否 |
| 变体 Debug | `variant-debug.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |
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

### Steps

整体步骤条。

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | current | 指定当前步骤，从 0 开始记数。在子 Step 元素中，可以通过 `status` 属性覆盖状态 | number | 0 | ~~direction~~ | 指定步骤条方向。目前支持水平（`horizontal`）和竖直（`vertical`）两种方向 | string | `horizontal` | iconRender | 自定义渲染图标，请优先使用 `items.icon` | (oriNode, info: { index, active, item }) => ReactNode | - | initial | 起始序号，从 0 开始记数 | number | 0 | ~~labelPlacement~~ | 指定标签放置位置，默认水平放图标右侧，可选 `vertical` 放图标下方 | string | `horizontal` | maxCount | 最大可见步骤项数量（`>= 3`）。超出数量的步骤区间会聚合成禁用的省略号步骤。 | number | - | orientation | 指定步骤条方向。目前支持水平（`horizontal`）和竖直（`vertical`）两种方向 | string | `horizontal` | percent | 当前 `process` 步骤显示的进度条进度（只对基本类型的 Steps 生效） | number | - | 4.5.0 | × |
| ~~progressDot~~ | 点状步骤条，可以设置为一个 function，请使用 `type="dot"` 替代。`titlePlacement` 将强制为 `vertical` | boolean \| (iconDot, { index, status, title, content }) => ReactNode | false | responsive | 当屏幕宽度小于 `532px` 时自动变为垂直模式 | boolean | true | size | 指定大小，目前支持普通（`medium`）和迷你（`small`） | string | `medium` | status | 指定当前步骤的状态，可选 `wait` `process` `finish` `error` | string | `process` | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | titlePlacement | 指定标签放置位置，默认水平放图标右侧，可选 `vertical` 放图标下方 | string | `horizontal` | type | 步骤条类型，可选 `default` `dot` `inline` `navigation` `panel` | string | `default` | variant | 设置样式变体 | `filled` \| `outlined` | `filled` | onChange | 点击切换步骤时触发 | (current) => void | - | items | 配置选项卡内容 | [StepItem](#stepitem) | [] | 4.24.0 | × |

### StepItem

步骤条内的每一个步骤。

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| content | 步骤的详情描述，可选 | ReactNode | - | disabled | 禁用点击 | boolean | false | status | 指定状态。当不配置该属性时，会使用 Steps 的 `current` 来自动指定状态。可选：`wait` `process` `finish` `error` | string | `wait` | title | 标题 | ReactNode | - 
```js
import { Steps } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `current` | 指定当前步骤，从 0 开始记数。在子 Step 元素中，可以通过 `status` 属性覆盖状态 | number | 0 | — |
| `direction` | 指定步骤条方向。目前支持水平（`horizontal`）和竖直（`vertical`）两种方向 | string | `horizontal` | — |
| `iconRender` | 自定义渲染图标，请优先使用 `items.icon` | (oriNode, info: { index, active, item }) => ReactNode | - | — |
| `initial` | 起始序号，从 0 开始记数 | number | 0 | — |
| `labelPlacement` | 指定标签放置位置，默认水平放图标右侧，可选 `vertical` 放图标下方 | string | `horizontal` | — |
| `maxCount` | 最大可见步骤项数量（`>= 3`）。超出数量的步骤区间会聚合成禁用的省略号步骤。 | number | - | — |
| `orientation` | 指定步骤条方向。目前支持水平（`horizontal`）和竖直（`vertical`）两种方向 | string | `horizontal` | — |
| `percent` | 当前 `process` 步骤显示的进度条进度（只对基本类型的 Steps 生效） | number | - | 4.5.0 |
| `progressDot` | 点状步骤条，可以设置为一个 function，请使用 `type="dot"` 替代。`titlePlacement` 将强制为 `vertical` | boolean \| (iconDot, { index, status, title, content }) => ReactNode | false | — |
| `responsive` | 当屏幕宽度小于 `532px` 时自动变为垂直模式 | boolean | true | — |
| `size` | 指定大小，目前支持普通（`medium`）和迷你（`small`） | string | `medium` | — |
| `status` | 指定当前步骤的状态，可选 `wait` `process` `finish` `error` | string | `process` | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `titlePlacement` | 指定标签放置位置，默认水平放图标右侧，可选 `vertical` 放图标下方 | string | `horizontal` | — |
| `type` | 步骤条类型，可选 `default` `dot` `inline` `navigation` `panel` | string | `default` | — |
| `variant` | 设置样式变体 | `filled` \| `outlined` | `filled` | — |
| `onChange` | 点击切换步骤时触发 | (current) => void | - | — |
| `items` | 配置选项卡内容 | [StepItem](#stepitem) | [] | 4.24.0 |
| `content` | 步骤的详情描述，可选 | ReactNode | - | — |
| `description` | 步骤的详情描述，可选 | ReactNode | - | — |
| `disabled` | 禁用点击 | boolean | false | — |
| `icon` | 步骤图标的类型，可选 | ReactNode | - | — |
| `subTitle` | 子标题 | ReactNode | - | — |
| `title` | 标题 | ReactNode | - | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Steps** 的验收清单：

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
- 官方文档：https://ant.design/components/steps
- 中文文档：https://ant.design/components/steps-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/steps
- 驱动 gpui kit：`steps`
