# Timeline 时间轴
> 来源：[Ant Design 6.5.x Timeline](https://ant.design/components/timeline)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：垂直展示的时间流信息。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

垂直展示的时间流信息。

**Timeline** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本用法 | 复现「基本用法」视觉与布局 |
| 变体样式 | variant 线框/填充差异 |
| 等待及排序 | 复现「等待及排序」视觉与布局 |
| 交替展现 | 复现「交替展现」视觉与布局 |
| 水平布局 | 横向布局 |
| 自定义时间轴点 | 自定义渲染/插槽外观 |
| 另一侧时间轴点 | 复现「另一侧时间轴点」视觉与布局 |
| 标题 | 复现「标题」视觉与布局 |
| 标题占比 | 复现「标题占比」视觉与布局 |
| 语义化自定义 | 自定义渲染/插槽外观 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `mode`

- **说明**：通过设置 `mode` 可以改变时间轴和内容的相对位置
- **类型**：`start` | `alternate` | `end`
- **默认值**：`start`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `alternate` | 官方取值 `alternate` |
  | `end` | 逻辑结束侧 |

#### `orientation`

- **说明**：设置时间轴的方向
- **类型**：`vertical` | `horizontal`
- **默认值**：`vertical`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `vertical` | 垂直排布 |
  | `horizontal` | 水平排布 |

#### `pending`

- **说明**：指定最后一个幽灵节点是否存在或内容，请使用 `item.loading` 代替
- **类型**：ReactNode
- **默认值**：false

#### `pendingDot`

- **说明**：当最后一个幽灵节点存在時，指定其时间图点，请使用 `item.icon` 代替
- **类型**：ReactNode
- **默认值**：<LoadingOutlined />

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `titleSpan`

- **说明**：设置标题占比空间，为到 dot 中心点距离
- **类型**：number | string
- **默认值**：12

#### `variant`

- **说明**：设置样式变体
- **类型**：`filled` | `outlined`
- **默认值**：`outlined`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `filled` | 浅底填充 |
  | `outlined` | 描边空心 |

#### `color`

- **说明**：指定圆圈颜色 `blue`、`red`、`green`、`gray`，或自定义的色值
- **类型**：string
- **默认值**：`blue`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `blue` | 官方取值 `blue` |
  | `red` | 官方取值 `red` |
  | `green` | 官方取值 `green` |
  | `gray` | 官方取值 `gray` |

#### `content`

- **说明**：设置内容
- **类型**：ReactNode
- **默认值**：-

#### `dot`

- **说明**：自定义时间轴点，请使用 `icon` 替换
- **类型**：ReactNode
- **默认值**：-

#### `icon`

- **说明**：自定义节点图标
- **类型**：ReactNode
- **默认值**：-

#### `loading`

- **说明**：设置加载状态
- **类型**：boolean
- **默认值**：false

#### `placement`

- **说明**：自定义节点位置
- **类型**：`start` | `end`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |

#### `position`

- **说明**：自定义节点位置，请使用 `placement` 替换
- **类型**：`start` | `end`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |

#### `title`

- **说明**：设置标题
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

- 当有一系列信息需按时间排列时，可正序和倒序。
- 需要有一条时间轴进行视觉上的串联时。

### 2.2 核心功能（按官方示例拆解）

1. **基本用法**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **变体样式**（`variant.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **等待及排序**（`pending.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **交替展现**（`alternate.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **水平布局**（`horizontal.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **自定义时间轴点**（`custom.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **另一侧时间轴点**（`end.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **标题**（`title.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **标题占比**（`title-span.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **语义化自定义**（`semantic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `loading` | 加载中 | 设置加载状态 |
| `items` | 数据化 items | 选项配置 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本用法 | `basic.tsx` | 否 |
| 变体样式 | `variant.tsx` | 否 |
| 等待及排序 | `pending.tsx` | 否 |
| 最后一个及排序 | `pending-legacy.tsx` | 是 |
| 交替展现 | `alternate.tsx` | 否 |
| 水平布局 | `horizontal.tsx` | 否 |
| 水平布局 | `horizontal-debug.tsx` | 是 |
| 自定义时间轴点 | `custom.tsx` | 否 |
| 另一侧时间轴点 | `end.tsx` | 否 |
| 标题 | `title.tsx` | 否 |
| 标题占比 | `title-span.tsx` | 否 |
| 语义化自定义 | `semantic.tsx` | 否 |
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

### Timeline

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | items | 选项配置 | [Items](#items)[] | - | mode | 通过设置 `mode` 可以改变时间轴和内容的相对位置 | `start` \| `alternate` \| `end` | `start` | orientation | 设置时间轴的方向 | `vertical` \| `horizontal` | `vertical` | ~~pending~~ | 指定最后一个幽灵节点是否存在或内容，请使用 `item.loading` 代替 | ReactNode | false | ~~pendingDot~~ | 当最后一个幽灵节点存在時，指定其时间图点，请使用 `item.icon` 代替 | ReactNode | &lt;LoadingOutlined /&gt; | reverse | 节点排序 | boolean | false | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | titleSpan | 设置标题占比空间，为到 dot 中心点距离 <InlinePopover previewURL="https://mdn.alipayobjects.com/huamei_7uahnr/afts/img/A*1NJISa7bpqgAAAAAR5AAAAgAerJ8AQ/original"></InlinePopover> | number \| string | 12 | variant | 设置样式变体 | `filled` \| `outlined` | `outlined` 
### Items

时间轴的每一个节点。

| 参数 | 说明 | 类型 | 默认值 |
| --- | --- | --- | --- |
| color | 指定圆圈颜色 `blue`、`red`、`green`、`gray`，或自定义的色值 | string | `blue` |
| content | 设置内容 | ReactNode | - |
| ~~children~~ | 设置内容，请使用 `content` 替换 | ReactNode | - |
| ~~dot~~ | 自定义时间轴点，请使用 `icon` 替换 | ReactNode | - |
| icon | 自定义节点图标 | ReactNode | - |
| ~~label~~ | 设置标签，请使用 `title` 替换 | ReactNode | - |
| loading | 设置加载状态 | boolean | false |
| placement | 自定义节点位置 | `start` \| `end` | - |
| ~~position~~ | 自定义节点位置，请使用 `placement` 替换 | `start` \| `end` | - |
| title | 设置标题 | ReactNode | - |

### 导入方式

```js
import { Timeline } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `items` | 选项配置 | [Items](#items)[] | - | — |
| `mode` | 通过设置 `mode` 可以改变时间轴和内容的相对位置 | `start` \| `alternate` \| `end` | `start` | — |
| `orientation` | 设置时间轴的方向 | `vertical` \| `horizontal` | `vertical` | — |
| `pending` | 指定最后一个幽灵节点是否存在或内容，请使用 `item.loading` 代替 | ReactNode | false | — |
| `pendingDot` | 当最后一个幽灵节点存在時，指定其时间图点，请使用 `item.icon` 代替 | ReactNode | <LoadingOutlined /> | — |
| `reverse` | 节点排序 | boolean | false | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `titleSpan` | 设置标题占比空间，为到 dot 中心点距离 | number \| string | 12 | — |
| `variant` | 设置样式变体 | `filled` \| `outlined` | `outlined` | — |
| `color` | 指定圆圈颜色 `blue`、`red`、`green`、`gray`，或自定义的色值 | string | `blue` | — |
| `content` | 设置内容 | ReactNode | - | — |
| `children` | 设置内容，请使用 `content` 替换 | ReactNode | - | — |
| `dot` | 自定义时间轴点，请使用 `icon` 替换 | ReactNode | - | — |
| `icon` | 自定义节点图标 | ReactNode | - | — |
| `label` | 设置标签，请使用 `title` 替换 | ReactNode | - | — |
| `loading` | 设置加载状态 | boolean | false | — |
| `placement` | 自定义节点位置 | `start` \| `end` | - | — |
| `position` | 自定义节点位置，请使用 `placement` 替换 | `start` \| `end` | - | — |
| `title` | 设置标题 | ReactNode | - | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Timeline** 的验收清单：

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
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/timeline
- 中文文档：https://ant.design/components/timeline-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/timeline
- 驱动 gpui kit：`timeline`
