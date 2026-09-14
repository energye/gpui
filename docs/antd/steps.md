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
| 基本用法 | `default`：圆图标 + 右侧标题描述 + 连接线 |
| 步骤运行错误 | `default` + 当前步 error 红（图标字轨同红） |
| 竖直方向的步骤条 | `default` 纵排，连接线转竖 |
| 可点击 | `default` 可点步（hover 底 + focus ring） |
| 面板式步骤 | `panel`：卡片 panel 块，当前块主色边 |
| 带图标的步骤条 | `default` 图标位换自定义 icon |
| 标签放置位置与进度 | `default` + `titlePlacement=vertical`（标题压图标下）+ process 步进度环 |
| 限量展示 | `default` + `maxCount` 折叠，省略步禁用态 |
| 点状步骤条 | `dot`（P1）：圆点代图标，标签强制图标下方 |
| 导航步骤 | `navigation`（P1）：箭头导航块，当前块主色字 |
| 内联步骤 | `inline`（P1）：单行紧凑无描述 |
| 内联样式组合 | `inline` + `variant`（P1）：filled 浅底 / outlined 描边 |
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
  | `wait` | 等待（灰底图标 + 次级字） |
  | `process` | 进行中：主色 `colorPrimary` 实心图标 + 反白字 |
  | `finish` | 完成：主色描边/浅底图标 + 主色勾，连接线主色 |
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
  | `default` | 默认中性外观（P0） |
  | `panel` | 面板式（P0） |
  | `dot` | 点状（P1，枚举预留） |
  | `inline` | 内联紧凑（P1，枚举预留） |
  | `navigation` | 导航式（P1，枚举预留） |

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

- **依赖等级**：L2（导航：纯展示 + 可点切换，无浮层定位）。
- **等谁**：无上游等待；`percent` 进度环复用 Progress 环语义，图标复用 `kit.Icon`；被业务向导页组合。
- **文件归属**：`ui/kit/steps/`。
- **组合**：单步 `icon` 经 Icon 注册表解析；`maxCount` 省略步为禁用态不可点，见 §6.4。
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

1. **配置面**：覆盖 API 表全部字段；冷门字段必须实现且命名兼容。
2. **视觉态**：default / hover / active / focus / disabled / loading。
3. **尺寸态**：small / medium / large（适用者）。
4. **受控/非受控**：value+onChange 与 defaultValue。
5. **数据驱动**：options / items / columns / treeData / fileList 等。
6. **无障碍**：焦点、角色、键盘、读屏。
7. **RTL**：placement / orientation 镜像。
8. **浮层**：z-index、挂载容器、遮挡、滚动。
9. **性能**：虚拟列表、防抖、减少重绘。
10. **主题**：Token 化；支持 reduced-motion。
11. **示例矩阵**：P0 按 §6.8 逐例对照表（8 例主路径），余下 P1 一次做完（`progress-dot`/`nav`/`inline`/`inline-variant`/`style-class`；6 个 debug 仅参考不验收）。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/steps
- 中文文档：https://ant.design/components/steps-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/steps
- 驱动 gpui kit：`steps`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Steps** 补成 **可开发、可测试、可验收** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5.1** 官网截图 99.99% 相似（见 ACCEPTANCE 对齐定义）。只允字体光栅亚像素差；颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即不算对齐。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/steps/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Steps）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | current/index 状态推导、可点切换、error 态、orientation 布局与 maxCount 折叠 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | showcase 大图 + 官网并排 | showcase大图进testdata按§6.8摆全多种式样（CPU容差内比对）+ 与官网同示例截图并排验收（颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即挂，只允字体光栅亚像素差） | showcase/官网并排 |
| **L4** | 官网并排必验（非可选） | 建/大改基线时与官网截图并排验收并留验收记录（见L3），CI比showcase基线，人眼签官网并排 | 建/大改基线必验 |

**明确不做（Steps）：**

- 与官网截图逐字节哈希一致（只要求 99.99% 相似，允字体光栅亚像素差）。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，单测Skip写清平台原因）。  
- 官方 **debug** 示例不验收（仅参考）。  

> 控件说明：引导用户按照流程完成任务的导航条。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

> Steps 组件 Token 源：`components/steps/style/index.ts` `prepareComponentToken`：`iconSize=controlHeight`、`iconSizeSM=fontSizeHeading3`（默认算法下 ≈ **24**）、`customIconSize=controlHeight`、`dotSize=controlHeight/4`。

#### 6.2.1 几何与组件 Token（Steps 专用）

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 图标容器 middle（`size=medium`） | **32** | Steps `iconSize` ← `controlHeight`（±0.5px） |
| 图标容器 small | **24** | Steps `iconSizeSM`（≈ `controlHeightSM` / heading3，±0.5px） |
| 自定义图标容器 | 同 size 档 | `customIconSize`（±0.5px） |
| 标题字号 | **16** | `fontSizeLG`（title） |
| 正文字号 / content | **14** | `fontSize` |
| 子标题字号 | **14** | `fontSize`；色 `colorTextSecondary` |
| 图标内数字字号 middle | **14** | `fontSize` |
| 图标内数字字号 small | **14** | `fontSize`（sm 档仅容器缩小，字号不缩） |
| 圆角 | **6** | `borderRadius`（±0.5px） |
| 边框线宽 / rail | **1** | `lineWidth` |
| 步骤间距 gap（水平 rail 区） | **8+** | 实现可读；rail 可 flex 填充（±0.5px 起） |
| Focus ring outset | ≈ **1.5px** 可见 | 可点步可聚焦时必须可见 |
| percent 进度环 stroke | ≈ **2–3** | process 步图标外环（±0.5px） |

> 注：antd Steps **无 large size**（仅 `medium` / `small`）。通用表里的 large 不适用于本控件。

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| process / finish 主色 | `colorPrimary` + hover/active | 当前步、已完成轨/图标 |
| finish 浅底 | `colorPrimaryBg` | filled 完成图标底 |
| wait 图标底 / 字 | `colorFillSecondary` / `colorTextSecondary` | 未达步 |
| error | `colorError` | status=error 当前或单步 |
| 文本 / 次级 / 描述 | `colorText` / `colorTextSecondary` | title / subtitle / content |
| 边框 / 分割 / 容器底 | `colorBorder` / `colorSplit` / `colorBgContainer` | outlined / panel / rail |
| 禁用 | `colorDisabledBg` / `colorDisabledText` | disabled 步；无 hover 高亮 |
| 反白字 | `colorTextInverse` | process solid 数字/勾 |

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

**配置优先级：** 单步显式 `status` > `current`/`initial` 推导（mapped=current-initial）> 整体 `status`（默认 process）> 组件默认；`onChange` 非 nil 且步未 disabled 才可点。

### 6.4 交互状态机（L1）

```text
current=i（0-based；与 antd 一致；Initial 偏移后 mapped = current-initial）
  未显式 item.status 时：
    index < mapped  → finish
    index == mapped → Steps.status（默认 process）
    index > mapped  → wait
  item.status 显式覆盖单步
  OnChange 非 nil 且步未 disabled ──► 可点；点击触发 OnChange(originIndex)
  Steps.status=error ──► 当前 mapped 步错误皮（可被 item.status 覆盖）
  maxCount>=3 且 items 更长 ──► 折叠为可见集 + 禁用省略步；OnChange 仍用原始下标
```

**触发条件 + 容差（可断言）：**

- 推导：`mapped=current-initial`（0-based）；`index<mapped→finish`，`=mapped→Steps.status`（默认 process），`>mapped→wait`；单步显式 status 覆盖。
- 可点：`OnChange!=nil` 且步未 disabled 才触发 `OnChange(originIndex)`，未受控时 Current 更新；disabled 步不触发（STP-S6）。
- 折叠：`maxCount>=3` 且 items 更长时折叠，省略步 disabled 不可点，回调仍用原始下标。
- 几何：图标容器 middle 32 / small 24（±0.5px，STP-S5）；error 当前步用 `colorError`。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| STP-S1 | current=1（**0-based**） | 第 2 步为 process（或 Steps.status）；0=finish、2=wait |
| STP-S2 | 点可点步（OnChange 已设） | 触发 OnChange(originIndex)；未受控时 Current 更新 |
| STP-S3 | status=error | 当前步错误样式（图标/字/轨 `colorError`） |
| STP-S4 | orientation=vertical | 根轴纵向（Flex column） |
| STP-S5 | size=small | 图标容器 24（±0.5px，小于 middle 32） |
| STP-S6 | item.disabled | 不可点；不触发 OnChange；禁用色 |
| STP-S7 | 自定义 icon | 图标节点可见（`IconNode` 优先） |
| STP-S8 | content（description 别名） | 详情文案可见（`Description→Content`） |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| wait | 灰底图标（`colorFillSecondary`）+ 次级字；轨 `colorSplit` |
| process | 主色实心图标 + 反白字/勾；标题 `colorText` 强调 |
| finish | 主色描边/浅底（`colorPrimaryBg`）+ 主色勾；轨主色 |
| error | 图标/字/轨 `colorError`（整体 status 或单步覆盖） |
| disabled | `colorDisabledBg/Text`，无 hover，不可点 |
| panel | 卡片块 + 当前块主色边；无 rail |
| 可点 hover/focus | hover 底 + focus ring 可见（outset ≈1.5px） |
| 主题切换 | 色与间距随 Theme 更新 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | 根 `navigation`；步骤列为 list，单步为 listitem（可点步为 button 语义） |
| 当前 | 当前步 `aria-current=step`；error 步附加 error 文案通道 |
| 键盘 | 可点步 Tab 可聚焦 ring 可见；Enter/Space 触发 OnChange |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 真实映射（gpui 侧落点） | 级别 |
| --- | --- | --- |
| 状态推导/切换/折叠（§6.4 STP-S1~S8） | **对等**：`SetCurrent/SetInitial/SetStatus` + `ItemStatus(i)` | P0 L1 |
| 图标/字号度量（§6.2 32/24/16/14） | **对等** | P0 L2 |
| `orientation` 横/纵 + `titlePlacement` | **对等**：Flex 行/列 + 标签右/下布局 | P0 L1 |
| `percent` 进度环（仅 default type） | **对等**：process 图标 Canvas 环（stroke 2–3，±0.5px） | P0 L1 |
| `responsive` 532px 自动纵排 | **映射**：桌面 `SetViewportWidth`，小宽切 vertical | P0 宿主 |
| 滚动宿主 | **映射**：长步骤条随容器滚动，无自有浮层 | P0 宿主 |
| `type=dot/navigation/inline` | P1 必做（枚举实现，不拒收） | P1 |
| `iconRender`/`progressDot` 函数 | P1 必做（优先 `items.icon`） | P1 |
| ink/rail 像素级过渡 | P0 瞬时，像素级 P1 | P0 L1/P1 |
| Semantic classNames/styles | kit 语义钩子 | P1 |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 官网截图相似（99.99%） | **不做** | — |

### 6.8 能力范围（P0 / P1 全做）

#### P0（本阶段必须 1:1，与P1一次做完）

| 配置 / 能力 | 说明 |
| --- | --- |
| `items` / `title` / `content` / `subTitle` / `icon` / `disabled` / 单步 `status` | StepItem 主字段；`description` 作 content 别名 |
| `current` / `initial` / `status` | 当前步（0-based）与整体当前态（默认 process） |
| `onChange` | 可点击切换；未设则不可点（展示型） |
| `orientation` | horizontal（默认）/ vertical（`direction` 废弃别名可读） |
| `size` | medium（默认）/ small（无 large） |
| `type` | **P0 子集** `default` / `panel`（`dot`/`inline`/`navigation` → P1） |
| `variant` | filled（默认）/ outlined |
| `titlePlacement` | horizontal（默认）/ vertical（标签在图标下） |
| `percent` | 当前 process 步进度环（0–100；仅 default type） |
| `maxCount` | ≥3 时折叠；省略步 disabled；OnChange 原始下标 |
| 官方主路径示例（P0，8 例） | 简单用法（`simple.tsx`）、步骤运行错误（`error.tsx`）、竖直方向（`vertical.tsx`）、可点击（`clickable.tsx`）、面板式（`panel.tsx`）、带图标（`icon.tsx`）、标签放置与进度（`title-placement.tsx`，含 percent）、限量展示（`max-count.tsx`） |
| 度量 §6.2 | Token 断言（icon 32/24±0.5 等） |
| a11y §6.6 | role=navigation；可点步可聚焦 + focus ring；当前 `aria-current=step` |
| §6.9 中 L1/L2 用例 | 测试通过 |

**逐例 P0/P1 对照表**（§2.4 全量；P0+P1=§6.8 非debug 8+12 例一次做完）：

| 示例 | 范围 | 原因 |
| --- | --- | --- |
| 基本用法 / 步骤运行错误 / 竖直方向 / 可点击 / 面板式 / 带图标 / 标签放置与进度 / 限量展示 | P0 | 状态推导+切换+panel+percent+折叠主路径 |
| 点状步骤条（`progress-dot.tsx`）/ 导航步骤（`nav.tsx`）/ 内联步骤（`inline.tsx`）/ 内联样式组合（`inline-variant.tsx`） | P1 | `type` 子集一次做完 |
| 自定义语义结构的样式和类（`style-class.tsx`）/ `_semantic*.tsx` | P1 | semantic 深度 |
| 步骤切换（`step-next.tsx`，debug）/ 自定义点状（`customized-progress-dot.tsx`，debug）/ 带有进度的步骤（`progress.tsx`，debug）/ Progress Debug（`progress-debug.tsx`）/ 嵌套（`steps-in-steps.tsx`）/ 变体 Debug（`variant-debug.tsx`）/ 组件 Token（`component-token.tsx`） | P1 | 调试页，不验收 |

#### P1（本阶段必须 1:1，与 P0 同标准；真做不了的单测 Skip 写清平台原因）

| 配置 / 能力 | 说明 |
| --- | --- |
| `type=dot` / `navigation` / `inline` | 点状 / 导航 / 内联 |
| `responsive` 断点自动 vertical | 桌面宿主映射 |
| `iconRender` 深度 / progressDot function | 自定义渲染钩子 |
| semantic classNames/styles 深度 | P1 必做（`style-class.tsx`，见逐例表） |
| 动画像素级 / rail 过渡 | P1 必做；P0+P1 瞬时 |
| 浏览器-only API 或桌面无等价项 | 单测 Skip 写清平台原因 |
| debug 示例 | 不验收（仅参考；7 个 debug，见逐例表） |

### 6.9 验收用例表（可测）

> 测试名建议：`TestSteps_PRD_<ID>` 或 gallery 场景 ID。  
> **P0+P1 相关用例全部通过** 才可宣称 Steps 完成 1:1。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| STP-01 | L1 | NewSteps 默认创建 | 不崩溃；Current=0、Size=middle、Orientation=horizontal、Type=default、Variant=filled、Status=process |
| STP-02 | L1 | current=1（0-based） | ItemStatus(1)=process；0=finish；2=wait |
| STP-03 | L1 | 点可点步 | OnChange(index)；Current 更新（非受控） |
| STP-04 | L1 | status=error | 当前步 ItemStatus=error；图标/字用 error 色 |
| STP-05 | L1 | orientation=vertical | 根 Flex 纵向 |
| STP-06 | L1 | size=small | 图标容器高 ≈24（小于 middle 32） |
| STP-07 | L1 | disabled 步 | 点击不触发 OnChange |
| STP-08 | L1 | 自定义 icon | Icon 节点存在 / 非空 |
| STP-09 | L1 | content（description 别名） | 详情文案进入树 |
| STP-10 | L1 | 官方 simple | current=1 + items 三步 + variant/size 矩阵可建 |
| STP-11 | L1 | 官方 error | current=1 status=error |
| STP-12 | L1 | 官方 vertical | orientation=vertical |
| STP-13 | L1 | 官方 clickable | OnChange 水平/垂直可点 |
| STP-14 | L1 | 官方 panel | type=panel 可建；单步 status=error 可见 |
| STP-15 | L1 | 官方 icon | 自定义 icon + 单步 status |
| STP-16 | L1 | 官方 title-placement | titlePlacement=vertical + percent |
| STP-17 | L1 | 官方 max-count | maxCount=5 时展示折叠 + 省略步 |
| STP-18 | L2 | §6.2 关键尺寸 | icon middle=32 / small=24（±0.5） |
| STP-19 | L2 | 默认皮颜色 | process 用 Theme colorPrimary，非硬编码 |
| STP-20 | L2 | disabled 外观 | 禁用色 Token；Disabled 态 |
| STP-21 | L1 | 键盘/焦点 | 可点步 Focusable；Enter/Space 触发 OnChange |
| STP-22 | L3 | 关键态 golden | 与showcase基线一致（容差内）+官网并排验收通过— 非本阶段强制 PRD 单测 |
| STP-23 | L4 | 与 ant.design 并排 | 官网并排验收记录（必验） |
| STP-24 | P1 | §6.8 P1 任一能力 | 单独用例；Notes 标明 |

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 契约。

```text
NewSteps(items ...StepItem) *Steps
// 便利：也可用 NewSteps() 后 SetItems

type StepItem struct {
  Title, Content, Description, SubTitle string // Description → Content 别名
  Disabled bool
  Icon     string      // 注册表名；"loading" → spinner（Ticker）
  IconNode core.Node   // 优先于 Icon 字符串
  Status   StepsStatus // 空 = 自动
}

// 形态
SetItems([]StepItem)
SetCurrent(int)                 // 受控 current（0-based）
SetDefaultCurrent(int)          // 非受控初值
SetInitial(int)                 // 起始序号偏移，默认 0
SetStatus(StepsStatus)          // 当前步整体态 wait|process|finish|error
SetSize(StepsSize)              // middle|small
SetType(StepsType)              // default|panel；dot|inline|navigation 为 P1 预留枚举（StepsTypeDot/Inline/Navigation），本阶段传参直接拒收
SetVariant(StepsVariant)        // filled|outlined
SetOrientation(StepsOrientation)// horizontal|vertical
SetTitlePlacement(StepsTitlePlacement) // horizontal|vertical
SetPercent(float64)             // 0..100；<0 表示未设
SetMaxCount(int)                // 0=关闭；>=3 生效
SetOnChange(func(current int))

// 主题 / a11y / 挂树
SetFace / SetTheme / SetAriaLabel
Node() / ChromeNode()
// 测试钩子
ItemStatus(i int) StepsStatus
ItemPressable(i int) *Pressable  // 原始下标；折叠省略返回 nil
IconSize() float64
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Current | 0 |
| Initial | 0 |
| Status | process |
| Size | middle（medium） |
| Type | default |
| Variant | filled |
| Orientation | horizontal |
| TitlePlacement | horizontal |
| Percent | 未设（无环） |
| MaxCount | 0（关闭） |
| OnChange | nil（不可点） |

### 6.11 结构与绘制分层（实现提示）

```text
Flex root (role=navigation)  — 稳定 Root，rebuild ClearChildren
  └─ [item Pressable|Box] × N
       └─ wrapper (row|column by titlePlacement / orientation)
            icon Decorated(圆)  ·  title/subTitle/content
       + rail Box（panel 隐藏；finish 主色）
  maxCount：display 列表可含 disabled 省略步；点击回调原始 originIndex
  percent：process 图标 Canvas 环；loading icon：Ticker 旋转
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Steps 1:1 完成**：

1. §6.8 **P0+P1** 全部实现（官方非debug一个不少，真做不了的单测Skip写清平台原因）。  
2. §6.9 中 **P0+P1** 用例全部通过。
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 showcase大图+官网并排验收通过（控件可见时必需，见§6.1）。
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0+P1** 全部（官方非 debug 一个不少；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 必须进 gallery，真做不了的单测 Skip 写清平台原因。  
6. `coverage.go` Notes：P0+P1 已对齐 `docs/antd/steps.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Steps 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围定义（无裁剪）。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
