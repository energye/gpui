# Steps 步骤条
> 来源：[Ant Design 6.5.x Steps](https://ant.design/components/steps)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：导航（Navigation）  
> 说明：引导用户按照流程完成任务的导航条。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
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

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

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

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Steps** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/steps/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Steps）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 选中/展开/分页或步骤切换与键盘 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Steps）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：引导用户按照流程完成任务的导航条。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 控件高度 middle | **32** | `controlHeight` |
| 控件高度 small | **24** | `controlHeightSM` |
| 控件高度 large | **40** | `controlHeightLG` |
| 字号 middle | **14** | `fontSize` |
| 圆角 | **6** | `borderRadius` |
| 边框线宽 | **1** | `lineWidth` |
| Focus ring outset | ≈ **1.5px** 可见 | 可调，必须可见 |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 主色 / hover / active | `colorPrimary` + 变体 | 强调、选中、开态 |
| 错误 / 成功 / 警告 | `colorError` / `Success` / `Warning` | status 与反馈 |
| 文本 / 次级文本 | `colorText` / `colorTextSecondary` | |
| 边框 / 分割 / 容器底 | `colorBorder` / `colorSplit` / `colorBgContainer` | |
| 禁用 | `colorDisabledBg` / `colorDisabledText` | 无 hover 高亮 |
| 浮层阴影 / 遮罩 | `boxShadowSecondary` / `colorBgMask` | 适用者 |

禁止硬编码品牌色作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**导航**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `current` | 指定当前步骤，从 0 开始记数。在子 Step 元素中，可以通过 `status` 属性覆盖状态 | number | 0 |
| `iconRender` | 自定义渲染图标，请优先使用 `items.icon` | (oriNode, info: { index, active, item… | - |
| `initial` | 起始序号，从 0 开始记数 | number | 0 |
| `maxCount` | 最大可见步骤项数量（`>= 3`）。超出数量的步骤区间会聚合成禁用的省略号步骤。 | number | - |
| `orientation` | 指定步骤条方向。目前支持水平（`horizontal`）和竖直（`vertical`）两种方向 | string | `horizontal` |
| `percent` | 当前 `process` 步骤显示的进度条进度（只对基本类型的 Steps 生效） | number | - |
| `responsive` | 当屏幕宽度小于 `532px` 时自动变为垂直模式 | boolean | true |
| `size` | 指定大小，目前支持普通（`medium`）和迷你（`small`） | string | `medium` |
| `status` | 指定当前步骤的状态，可选 `wait` `process` `finish` `error` | string | `process` |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |
| `titlePlacement` | 指定标签放置位置，默认水平放图标右侧，可选 `vertical` 放图标下方 | string | `horizontal` |
| `type` | 步骤条类型，可选 `default` `dot` `inline` `navigation` `panel` | string | `default` |
| `variant` | 设置样式变体 | `filled` \ | `outlined` |
| `onChange` | 点击切换步骤时触发 | (current) => void | - |
| `items` | 配置选项卡内容 | [StepItem](#stepitem) | [] |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
current=i
  items[0..i-1] finish；i process；>i wait（可被 status 覆盖）
  onChange 可点 ──► current'
  status=error ──► 当前错误皮
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| STP-S1 | current=1（0-based 实现需锁定） | 对应步为 process |
| STP-S2 | 点可点步 | onChange |
| STP-S3 | status=error | 错误样式 |
| STP-S4 | vertical | 纵向 |
| STP-S5 | size=small | 更小 |
| STP-S6 | disabled 步 | 不可点 |
| STP-S7 | 自定义 icon | 可见 |
| STP-S8 | description | 可见 |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 符合 §6.2 Token |
| hover/active/focus | 可交互者具备反馈与 focus ring |
| disabled / loading / empty | 按本控件语义 |
| 主题切换 | 色与间距随 Theme 更新 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | navigation / menu / tablist 等 |
| 当前 | aria-current / selected |
| 键盘 | 方向键与激活 |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 主路径行为（§6.1 L1） | **对等** | P0 L1 |
| 尺寸/色 Token（§6.2） | **对等** | P0 L2 |
| 动画/波纹/CSS 特效 | **近似**或瞬时 | P1 |
| IME/剪贴板/滚动宿主（适用者） | **宿主** | P0 宿主 |
| 浏览器-only API | **映射**或 P1 不做 | P1 |
| Semantic classNames/styles | kit 语义钩子 | P1 |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 逐像素官网哈希 | **不做** | — |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `onChange` | 必须 |
| `disabled` | 必须 |
| `size` | 必须 |
| `type` | 必须 |
| `variant` | 必须 |
| `status` | 必须 |
| `items` | 必须 |
| `title` | 必须 |
| `content` | 必须 |
| `orientation` | 必须 |
| `icon` | 必须 |
| `percent` | 必须 |
| 官方主路径示例 | 基本用法、步骤运行错误、竖直方向的步骤条、可点击、面板式步骤、带图标的步骤条、标签放置位置与进度、限量展示 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | 最低要求 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 深度 | 分期 |
| 动画像素级 / 复杂虚拟列表 | 分期 |
| 浏览器-only API 或桌面无等价项 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |
| 其余示例 | 点状步骤条, 导航步骤, 内联步骤, 内联样式组合 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestSteps_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Steps 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| STP-01 | L1 | NewSteps 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| STP-02 | L1 | current=1（0-based 实现需锁定） | 对应步为 process |
| STP-03 | L1 | 点可点步 | onChange |
| STP-04 | L1 | status=error | 错误样式 |
| STP-05 | L1 | vertical | 纵向 |
| STP-06 | L1 | size=small | 更小 |
| STP-07 | L1 | disabled 步 | 不可点 |
| STP-08 | L1 | 自定义 icon | 可见 |
| STP-09 | L1 | description | 可见 |
| STP-10 | L1 | 复现官方示例「基本用法」（`simple.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| STP-11 | L1 | 复现官方示例「步骤运行错误」（`error.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| STP-12 | L1 | 复现官方示例「竖直方向的步骤条」（`vertical.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| STP-13 | L1 | 复现官方示例「可点击」（`clickable.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| STP-14 | L1 | 复现官方示例「面板式步骤」（`panel.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| STP-15 | L1 | 复现官方示例「带图标的步骤条」（`icon.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| STP-16 | L1 | 复现官方示例「标签放置位置与进度」（`title-placement.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| STP-17 | L1 | 复现官方示例「限量展示」（`max-count.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| STP-18 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| STP-19 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| STP-20 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| STP-21 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| STP-22 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| STP-23 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| STP-24 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewSteps(...) *Steps

// 配置：对 §6.3 / §3 中 P0 字段提供 SetXxx
// 回调：OnChange / OnClick / OnOpenChange / OnConfirm … 按 API
// 状态：SetDisabled / SetLoading（适用者）
// 主题：SetTheme(*Theme)；Style 可选覆盖
// a11y：SetAriaLabel / 焦点与键盘
// 挂树：Node() core.Node
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Disabled | false |
| Size（适用者） | middle / 控件默认 |
| 受控值 | 未 Set 时用 default* 或零值 |
| 其余 | 对齐 antd 6.5 §3 表 |

### 6.11 结构与绘制分层（实现提示）

```text
Nav root
  └─ items / panels / connectors
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Steps 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. gallery 展示主路径（对照官方非 debug 示例与 P0）。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/steps.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Steps 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
