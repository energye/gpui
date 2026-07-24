# Typography 排版
> 来源：[Ant Design 6.5.x Typography](https://ant.design/components/typography)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：通用（General）  
> 说明：文本的基本格式。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

文本的基本格式。

**Typography** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 标题组件 | 复现「标题组件」视觉与布局 |
| 文本与超链接组件 | 复现「文本与超链接组件」视觉与布局 |
| 可编辑 | 复现「可编辑」视觉与布局 |
| 可复制 | 复现「可复制」视觉与布局 |
| 省略号 | 复现「省略号」视觉与布局 |
| 受控省略展开/收起 | 展开/折叠指示 |
| 省略中间 | 复现「省略中间」视觉与布局 |
| 后缀 | 复现「后缀」视觉与布局 |
| 表格 | 复现「表格」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `actions`

- **说明**：配置操作栏
- **类型**：[actions](#actions)
- **默认值**：-
- **版本**：6.4.0

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.4.0

#### `code`

- **说明**：添加代码样式
- **类型**：boolean
- **默认值**：false

#### `delete`

- **说明**：添加删除线样式
- **类型**：boolean
- **默认值**：false

#### `disabled`

- **说明**：禁用文本
- **类型**：boolean
- **默认值**：false

#### `ellipsis`

- **说明**：自动溢出省略，为对象时不能设置省略行数、是否可展开、onExpand 展开事件。不同于 Typography.Paragraph，Text 组件自身不带 100% 宽度样式，因而默认情况下初次缩略后宽度便不再变化。如果需要自适应宽度，请手动配置宽度样式
- **类型**：boolean | [Omit](#ellipsis)
- **默认值**：false

#### `italic`

- **说明**：是否斜体
- **类型**：boolean
- **默认值**：false
- **版本**：4.16.0

#### `keyboard`

- **说明**：添加键盘样式
- **类型**：boolean
- **默认值**：false
- **版本**：4.3.0

#### `mark`

- **说明**：添加标记样式
- **类型**：boolean
- **默认值**：false

#### `strong`

- **说明**：是否加粗
- **类型**：boolean
- **默认值**：false

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.4.0

#### `type`

- **说明**：文本类型
- **类型**：`secondary` | `success` | `warning` | `danger`
- **默认值**：-
- **版本**：success: 4.6.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `secondary` | 官方取值 `secondary` |
  | `success` | 成功绿语义 |
  | `warning` | 警告橙语义 |
  | `danger` | 危险红语义 |

#### `underline`

- **说明**：添加下划线样式
- **类型**：boolean
- **默认值**：false

#### `level`

- **说明**：重要程度，相当于 `h1`、`h2`、`h3`、`h4`、`h5`
- **类型**：number: 1, 2, 3, 4, 5
- **默认值**：1
- **版本**：5: 4.6.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `h1` | 官方取值 `h1` |
  | `h2` | 官方取值 `h2` |
  | `h3` | 官方取值 `h3` |
  | `h4` | 官方取值 `h4` |
  | `h5` | 官方取值 `h5` |

#### `placement`

- **说明**：设置操作栏相对于文本的位置
- **类型**：`start` | `end`
- **默认值**：`end`
- **版本**：6.4.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |

#### `icon`

- **说明**：自定义拷贝图标：\[默认图标, 拷贝后的图标]
- **类型**：\[ReactNode, ReactNode]
- **默认值**：-
- **版本**：4.6.0

#### `text`

- **说明**：拷贝到剪切板里的文本
- **类型**：string
- **默认值**：-

#### `editing`

- **说明**：控制是否是编辑中状态
- **类型**：boolean
- **默认值**：false

#### `enterIcon`

- **说明**：在编辑段中自定义"enter"图标（传递"null"将删除图标）
- **类型**：ReactNode
- **默认值**：``
- **版本**：4.17.0

#### `triggerType`

- **说明**：编辑模式触发器类型，图标、文本或者两者都设置（不设置图标作为触发器时它会隐藏）
- **类型**：Array<`icon`|`text`>
- **默认值**：\[`icon`]
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `icon` | 官方取值 `icon` |
  | `text` | 文本/弱样式 |

#### `onCancel`

- **说明**：按 ESC 退出编辑状态时触发
- **类型**：function
- **默认值**：-

#### `onEnd`

- **说明**：按 ENTER 结束编辑状态时触发
- **类型**：function
- **默认值**：-
- **版本**：4.14.0

#### `onStart`

- **说明**：进入编辑中状态时触发
- **类型**：function
- **默认值**：-

#### `suffix`

- **说明**：自定义省略内容后缀
- **类型**：string
- **默认值**：-

#### `symbol`

- **说明**：自定义展开描述文案
- **类型**：ReactNode | ((expanded: boolean) => ReactNode)
- **默认值**：`展开` `收起`

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

- 当需要展示标题、段落、列表内容时使用，如文章/博客/日志的文本样式。
- 当需要一列基于文本的基础操作时，如拷贝/省略/可编辑。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **标题组件**（`title.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **文本与超链接组件**（`text.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **可编辑**（`editable.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **可复制**（`copyable.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **省略号**（`ellipsis.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **受控省略展开/收起**（`ellipsis-controlled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **省略中间**（`ellipsis-middle.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **后缀**（`suffix.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **表格**（`table.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onChange` | 值变化 | 文本域编辑时触发 |
| `onClick` | 点击 | 点击 Text 时的回调 |
| `disabled` | 禁用 | 禁用文本 |
| `expandable` | 展开行 | 是否可展开 |
| `onCancel` | 取消 | 按 ESC 退出编辑状态时触发 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 标题组件 | `title.tsx` | 否 |
| 标题与段落 | `paragraph-debug.tsx` | 是 |
| 文本与超链接组件 | `text.tsx` | 否 |
| 可编辑 | `editable.tsx` | 否 |
| 可复制 | `copyable.tsx` | 否 |
| 省略号 | `ellipsis.tsx` | 否 |
| 受控省略展开/收起 | `ellipsis-controlled.tsx` | 否 |
| 省略中间 | `ellipsis-middle.tsx` | 否 |
| 省略号 Debug | `ellipsis-debug.tsx` | 是 |
| 后缀 | `suffix.tsx` | 否 |
| 组件 Token | `componentToken-debug.tsx` | 是 |
| Link danger Debug | `link-danger-debug.tsx` | 是 |
| 表格 | `table.tsx` | 否 |

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

### Typography.Text

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| actions | 配置操作栏 | [actions](#actions) | - | 6.4.0 | × |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | 6.4.0 | 6.4.0 |
| code | 添加代码样式 | boolean | false | copyable | 是否可拷贝，为对象时可进行各种自定义 | boolean \| [copyable](#copyable) | false | delete | 添加删除线样式 | boolean | false | disabled | 禁用文本 | boolean | false | editable | 是否可编辑，为对象时可对编辑进行控制 | boolean \| [editable](#editable) | false | ellipsis | 自动溢出省略，为对象时不能设置省略行数、是否可展开、onExpand 展开事件。不同于 Typography.Paragraph，Text 组件自身不带 100% 宽度样式，因而默认情况下初次缩略后宽度便不再变化。如果需要自适应宽度，请手动配置宽度样式 | boolean \| [Omit<ellipsis, 'expandable' \| 'rows' \| 'onExpand'>](#ellipsis) | false | italic | 是否斜体 | boolean | false | 4.16.0 | × |
| keyboard | 添加键盘样式 | boolean | false | 4.3.0 | × |
| mark | 添加标记样式 | boolean | false | strong | 是否加粗 | boolean | false | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 6.4.0 | 6.4.0 |
| type | 文本类型 | `secondary` \| `success` \| `warning` \| `danger` | - | success: 4.6.0 | × |
| underline | 添加下划线样式 | boolean | false | onClick | 点击 Text 时的回调 | (event) => void | - 
### Typography.Title

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| actions | 配置操作栏 | [actions](#actions) | - | 6.4.0 | × |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | 6.4.0 | 6.4.0 |
| code | 添加代码样式 | boolean | false | copyable | 是否可拷贝，为对象时可进行各种自定义 | boolean \| [copyable](#copyable) | false | delete | 添加删除线样式 | boolean | false | disabled | 禁用文本 | boolean | false | editable | 是否可编辑，为对象时可对编辑进行控制 | boolean \| [editable](#editable) | false | ellipsis | 自动溢出省略，为对象时可设置省略行数、是否可展开、添加后缀等 | boolean \| [ellipsis](#ellipsis) | false | italic | 是否斜体 | boolean | false | 4.16.0 | × |
| level | 重要程度，相当于 `h1`、`h2`、`h3`、`h4`、`h5` | number: 1, 2, 3, 4, 5 | 1 | 5: 4.6.0 | × |
| mark | 添加标记样式 | boolean | false | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 6.4.0 | 6.4.0 |
| type | 文本类型 | `secondary` \| `success` \| `warning` \| `danger` | - | success: 4.6.0 | × |
| underline | 添加下划线样式 | boolean | false | onClick | 点击 Title 时的回调 | (event) => void | - 
### Typography.Paragraph

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| actions | 配置操作栏 | [actions](#actions) | - | 6.4.0 | × |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | 6.4.0 | 6.4.0 |
| code | 添加代码样式 | boolean | false | copyable | 是否可拷贝，为对象时可进行各种自定义 | boolean \| [copyable](#copyable) | false | delete | 添加删除线样式 | boolean | false | disabled | 禁用文本 | boolean | false | editable | 是否可编辑，为对象时可对编辑进行控制 | boolean \| [editable](#editable) | false | ellipsis | 自动溢出省略，为对象时可设置省略行数、是否可展开、添加后缀等 | boolean \| [ellipsis](#ellipsis) | false | italic | 是否斜体 | boolean | false | 4.16.0 | × |
| mark | 添加标记样式 | boolean | false | strong | 是否加粗 | boolean | false | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 6.4.0 | 6.4.0 |
| type | 文本类型 | `secondary` \| `success` \| `warning` \| `danger` | - | success: 4.6.0 | × |
| underline | 添加下划线样式 | boolean | false | onClick | 点击 Paragraph 时的回调 | (event) => void | - 
### actions

    {
      placement: 'start' | 'end',
    }

| 参数      | 说明                       | 类型             | 默认值 | 版本  |
| --------- | -------------------------- | ---------------- | ------ | ----- |
| placement | 设置操作栏相对于文本的位置 | `start` \| `end` | `end`  | 6.4.0 |

### copyable

    {
      text: string | (() => string | Promise<string>),
      onCopy: function(event),
      icon: ReactNode,
      tooltips: false | [ReactNode, ReactNode],
      format: 'text/plain' | 'text/html',
      tabIndex: number,
    }

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| format | 剪切板数据的 Mime Type | 'text/plain' \| 'text/html' | - | 4.21.0 |
| icon | 自定义拷贝图标：\[默认图标, 拷贝后的图标] | \[ReactNode, ReactNode] | - | 4.6.0 |
| tabIndex | 自定义复制按钮的 tabIndex | number | 0 | 5.17.0 |
| text | 拷贝到剪切板里的文本 | string | - | onCopy | 拷贝成功的回调函数 | function | - 
    {
      icon: ReactNode,
      tooltip: ReactNode,
      editing: boolean,
      maxLength: number,
      autoSize: boolean | { minRows: number, maxRows: number },
      text: string,
      onChange: function(string),
      onCancel: function,
      onStart: function,
      onEnd: function,
      triggerType: ('icon' | 'text')[],
      enterIcon: ReactNode,
      tabIndex: number,
    }

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| autoSize | 自动 resize 文本域 | boolean \| { minRows: number, maxRows: number } | - | 4.4.0 |
| editing | 控制是否是编辑中状态 | boolean | false | icon | 自定义编辑图标 | ReactNode | &lt;EditOutlined /> | 4.6.0 |
| maxLength | 编辑中文本域最大长度 | number | - | 4.4.0 |
| tabIndex | 自定义编辑按钮的 tabIndex | number | 0 | 5.17.0 |
| text | 显式地指定编辑文案，为空时将隐式地使用 children | string | - | 4.24.0 |
| tooltip | 自定义提示文本，为 false 时关闭 | ReactNode | `编辑` | 4.6.0 |
| triggerType | 编辑模式触发器类型，图标、文本或者两者都设置（不设置图标作为触发器时它会隐藏） | Array&lt;`icon`\|`text`> | \[`icon`] | onChange | 文本域编辑时触发 | function(value: string) | - | onStart | 进入编辑中状态时触发 | function | - 
```tsx
interface EllipsisConfig {
  rows: number;
  /** `5.16.0` 新增 `collapsible` */
  expandable: boolean | 'collapsible';
  suffix: string;
  /** `5.16.0` 新增渲染函数 */
  symbol: ReactNode | ((expanded: boolean) => ReactNode);
  tooltip: ReactNode | TooltipProps;
  /** `5.16.0` 新增 */
  defaultExpanded: boolean;
  /** `5.16.0` 新增 */
  expanded: boolean;
  /** `5.16.0` 新增 `info` */
  onExpand: (event: MouseEvent, info: { expanded: boolean }) => void;
  onEllipsis: (ellipsis: boolean) => void;
}
```

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| defaultExpanded | 默认展开或收起 | boolean | expandable | 是否可展开 | boolean \| 'collapsible' | - | `collapsible`: 5.16.0 |
| expanded | 展开或收起 | boolean | rows | 最多显示的行数 | number | - | symbol | 自定义展开描述文案 | ReactNode \| ((expanded: boolean) => ReactNode) | `展开` `收起` | onEllipsis | 触发省略时的回调 | function(ellipsis) | - | 4.2.0 |
| onExpand | 点击展开或收起时的回调 | function(event, { expanded: boolean }) | - | `info`: 5.16.0 |

### 导入方式

```js
import { Typography } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `actions` | 配置操作栏 | [actions](#actions) | - | 6.4.0 |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.4.0 |
| `code` | 添加代码样式 | boolean | false | — |
| `copyable` | 是否可拷贝，为对象时可进行各种自定义 | boolean \| [copyable](#copyable) | false | — |
| `delete` | 添加删除线样式 | boolean | false | — |
| `disabled` | 禁用文本 | boolean | false | — |
| `editable` | 是否可编辑，为对象时可对编辑进行控制 | boolean \| [editable](#editable) | false | — |
| `ellipsis` | 自动溢出省略，为对象时不能设置省略行数、是否可展开、onExpand 展开事件。不同于 Typography.Paragraph，Text 组件自身不带 100% 宽度样式，因而默认情况下初次缩略后宽度便不再变化。如果需要自适应宽度，请手动配置宽度样式 | boolean \| [Omit](#ellipsis) | false | — |
| `italic` | 是否斜体 | boolean | false | 4.16.0 |
| `keyboard` | 添加键盘样式 | boolean | false | 4.3.0 |
| `mark` | 添加标记样式 | boolean | false | — |
| `strong` | 是否加粗 | boolean | false | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.4.0 |
| `type` | 文本类型 | `secondary` \| `success` \| `warning` \| `danger` | - | success: 4.6.0 |
| `underline` | 添加下划线样式 | boolean | false | — |
| `onClick` | 点击 Text 时的回调 | (event) => void | - | — |
| `level` | 重要程度，相当于 `h1`、`h2`、`h3`、`h4`、`h5` | number: 1, 2, 3, 4, 5 | 1 | 5: 4.6.0 |
| `placement` | 设置操作栏相对于文本的位置 | `start` \| `end` | `end` | 6.4.0 |
| `format` | 剪切板数据的 Mime Type | 'text/plain' \| 'text/html' | - | 4.21.0 |
| `icon` | 自定义拷贝图标：\[默认图标, 拷贝后的图标] | \[ReactNode, ReactNode] | - | 4.6.0 |
| `tabIndex` | 自定义复制按钮的 tabIndex | number | 0 | 5.17.0 |
| `text` | 拷贝到剪切板里的文本 | string | - | — |
| `tooltips` | 自定义提示文案，为 false 时隐藏文案 | \[ReactNode, ReactNode] | \[`复制`, `复制成功`] | 4.4.0 |
| `onCopy` | 拷贝成功的回调函数 | function | - | — |
| `autoSize` | 自动 resize 文本域 | boolean \| { minRows: number, maxRows: number } | - | 4.4.0 |
| `editing` | 控制是否是编辑中状态 | boolean | false | — |
| `enterIcon` | 在编辑段中自定义"enter"图标（传递"null"将删除图标） | ReactNode | `` | 4.17.0 |
| `maxLength` | 编辑中文本域最大长度 | number | - | 4.4.0 |
| `tooltip` | 自定义提示文本，为 false 时关闭 | ReactNode | `编辑` | 4.6.0 |
| `triggerType` | 编辑模式触发器类型，图标、文本或者两者都设置（不设置图标作为触发器时它会隐藏） | Array<`icon`\|`text`> | \[`icon`] | — |
| `onCancel` | 按 ESC 退出编辑状态时触发 | function | - | — |
| `onChange` | 文本域编辑时触发 | function(value: string) | - | — |
| `onEnd` | 按 ENTER 结束编辑状态时触发 | function | - | 4.14.0 |
| `onStart` | 进入编辑中状态时触发 | function | - | — |
| `defaultExpanded` | 默认展开或收起 | boolean | — | 5.16.0 |
| `expandable` | 是否可展开 | boolean \| 'collapsible' | - | `collapsible`: 5.16.0 |
| `expanded` | 展开或收起 | boolean | — | 5.16.0 |
| `rows` | 最多显示的行数 | number | - | — |
| `suffix` | 自定义省略内容后缀 | string | - | — |
| `symbol` | 自定义展开描述文案 | ReactNode \| ((expanded: boolean) => ReactNode) | `展开` `收起` | — |
| `onEllipsis` | 触发省略时的回调 | function(ellipsis) | - | 4.2.0 |
| `onExpand` | 点击展开或收起时的回调 | function(event, { expanded: boolean }) | - | `info`: 5.16.0 |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Typography** 的验收清单：

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
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/typography
- 中文文档：https://ant.design/components/typography-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/typography
- 驱动 gpui kit：`typography`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Typography** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/typography/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Typography）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 展示形态与可选交互（复制/预览/关闭） | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Typography）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：文本的基本格式。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| Title h1..h5 | **38/30/24/20/16** | `fontSizeHeading1..5`（无 Token 时回落常量） |
| 正文 | **14** | `fontSize` |
| 操作图标 | **14** | `fontSize` |
| 操作间距 | **4** | `marginXXS` / `TokenMarginXS` |
| code/kbd 圆角 | **3** | 组件常量（antd style 硬编码） |
| mark 底色 | gold[2] ≈ `#ffe58f` | 组件常量（antd v4 兼容） |
| 圆角（容器） | **6** | `borderRadius` |
| 边框线宽 | **1** | `lineWidth` |
| Focus ring outset | ≈ **1.5px** 可见 | 可调，必须可见 |
| 复制成功反馈时长 | **≈3s** | Ticker 短时态（非 loading） |

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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**通用**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `actions` | 配置操作栏 | [actions](#actions) | - |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `code` | 添加代码样式 | boolean | false |
| `copyable` | 是否可拷贝，为对象时可进行各种自定义 | boolean \ | [copyable](#copyable) |
| `delete` | 添加删除线样式 | boolean | false |
| `disabled` | 禁用文本 | boolean | false |
| `editable` | 是否可编辑，为对象时可对编辑进行控制 | boolean \ | [editable](#editable) |
| `ellipsis` | 自动溢出省略，为对象时不能设置省略行数、是否可展开、onExpand 展开事件。不同于 Typography.Pa… | boolean \ | [Omit<ellipsis, 'expandable' \ |
| `italic` | 是否斜体 | boolean | false |
| `keyboard` | 添加键盘样式 | boolean | false |
| `mark` | 添加标记样式 | boolean | false |
| `strong` | 是否加粗 | boolean | false |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |
| `type` | 文本类型 | `secondary` \ | `success` \ |
| `underline` | 添加下划线样式 | boolean | false |
| `onClick` | 点击 Text 时的回调 | (event) => void | - |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
Text/Title/Paragraph/Link 渲染
copyable ──► 剪贴板 + onCopy
ellipsis ──► 省略；expandable 展开
editable ──► 编辑态 Enter 提交 Esc 取消
```

\*Title 字号 38/30/24/20/16。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| TYP-S1 | Title level 1..5 | 字号阶梯 |
| TYP-S2 | type=danger 等 | 语义色 |
| TYP-S3 | copyable 点击 | 剪贴板正确；onCopy |
| TYP-S4 | ellipsis 超长 | 省略号 |
| TYP-S5 | expandable 展开 | 全文 |
| TYP-S6 | editable Enter | 提交新文案 |
| TYP-S7 | editable Esc | 取消 |
| TYP-S8 | disabled | 不可点复制/编辑 |
| TYP-S9 | strong/code/mark/delete | 样式可区分 |
| TYP-S10 | Link | 链接色可聚焦 |
| TYP-S11 | 受控 expanded | 外部优先 |
| TYP-S12 | 正文 14 | 字号 |
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
| 装饰图 | alt 或 aria-hidden |
| 有意义操作 | 复制/关闭/展开有名 |

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
| 形态 `Text` / `Title` / `Paragraph` / `Link` | 四种构造；Title 带 `level` 1..5 |
| `type` | `secondary` / `success` / `warning` / `danger` 语义色（§6.2 Token） |
| `disabled` | 禁用文本；不可触发复制 / 编辑 / onClick |
| `copyable` + `onCopy` + `icon` | 复制到 Tree 剪贴板；可自定义拷贝文案 / 图标名 |
| `editable` + `onChange` / `onStart` / `onEnd` / `onCancel` | Enter 提交、Esc 取消；支持受控 `editing` |
| `ellipsis` | 超长省略；`rows` / `expandable`/`collapsible` / 受控 `expanded` / `suffix` / **中间省略** |
| `actions.placement` | 操作栏 `start` / `end`（默认 end） |
| `strong` / `code` / `mark` / `delete` / `underline` / `italic` / `keyboard` | 修饰可区分（code/mark/kbd 有 chrome；delete/underline 装饰线） |
| `onClick` | Text / Title / Paragraph / Link 点击 |
| 正文 / Title 度量 | 正文 **14**；Title **38/30/24/20/16**（§6.2） |
| 官方主路径示例 | 基本、标题组件、文本与超链接组件、可编辑、可复制、省略号、受控省略展开/收起、省略中间 |
| a11y §6.6 | 复制 / 编辑 / 展开操作有可访问名 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 深度 | 分期 |
| copyable.format / tooltips 深度 / tabIndex | 分期 |
| editable.autoSize / maxLength / enterIcon / triggerType 全矩阵 | 分期（P0 支持 icon 触发 + Enter/Esc） |
| ellipsis.tooltip / onEllipsis 像素级 | 分期（P0 有回调钩子即可） |
| 动画像素级 / 复制成功动画像素级 | 分期（P0 可用瞬时或短 Ticker 态） |
| 浏览器-only API 或桌面无等价项 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |
| 其余示例 | 后缀, 表格, _semantic.tsx |
| ConfigProvider 全局 Typography 默认 | 分期 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestTypography_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Typography 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| TYP-01 | L1 | NewTypography 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| TYP-02 | L1 | Title level 1..5 | 字号阶梯 |
| TYP-03 | L1 | type=danger 等 | 语义色 |
| TYP-04 | L1 | copyable 点击 | 剪贴板正确；onCopy |
| TYP-05 | L1 | ellipsis 超长 | 省略号 |
| TYP-06 | L1 | expandable 展开 | 全文 |
| TYP-07 | L1 | editable Enter | 提交新文案 |
| TYP-08 | L1 | editable Esc | 取消 |
| TYP-09 | L1 | disabled | 不可点复制/编辑 |
| TYP-10 | L1 | strong/code/mark/delete | 样式可区分 |
| TYP-11 | L1 | Link | 链接色可聚焦 |
| TYP-12 | L1 | 受控 expanded | 外部优先 |
| TYP-13 | L1 | 正文 14 | 字号 |
| TYP-14 | L1 | 复现官方示例「基本」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TYP-15 | L1 | 复现官方示例「标题组件」（`title.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TYP-16 | L1 | 复现官方示例「文本与超链接组件」（`text.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TYP-17 | L1 | 复现官方示例「可编辑」（`editable.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TYP-18 | L1 | 复现官方示例「可复制」（`copyable.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TYP-19 | L1 | 复现官方示例「省略号」（`ellipsis.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TYP-20 | L1 | 复现官方示例「受控省略展开/收起」（`ellipsis-controlled.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TYP-21 | L1 | 复现官方示例「省略中间」（`ellipsis-middle.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TYP-22 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| TYP-23 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| TYP-24 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| TYP-25 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| TYP-26 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| TYP-27 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| TYP-28 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。  
> 四种形态共享 `*Typography`；`NewText` / `NewTitle` / `NewParagraph` / `NewLink` 为语法糖。

```text
NewTypography(value string) *Typography          // kind=Text
NewText(value string) *Typography                // = NewTypography
NewTitle(value string, level int) *Typography    // level 1..5，默认 1
NewParagraph(value string) *Typography
NewLink(value string) *Typography

// 内容 / 形态
SetValue(string) / Value() string
SetKind(Text|Title|Paragraph|Link)
SetLevel(1..5)                    // Title
SetType(Default|Secondary|Success|Warning|Danger)
SetDisabled(bool)
SetStrong / SetCode / SetMark / SetDelete / SetUnderline / SetItalic / SetKeyboard(bool)

// 复制
SetCopyable(bool)
SetCopyText(string)               // 空 = 用 Value
SetCopyIcon(name string)          // 空 = 默认图标
OnCopy func(text string)

// 编辑
SetEditable(bool)
SetEditing(bool)                  // 受控；未 Set 时内部状态
OnChange func(value string)       // 提交后的新文案
OnStart / OnEnd / OnCancel func()

// 省略
SetEllipsis(bool)
SetEllipsisRows(n int)            // 0/1 = 单行
SetExpandable(bool)               // true → 可展开；+collapsible 可收起
SetCollapsible(bool)
SetExpanded(bool)                 // 受控 expanded
SetDefaultExpanded(bool)
SetEllipsisMiddle(bool)
SetSuffix(string)
SetExpandSymbol(expand, collapse string)  // 默认 「展开」「收起」
OnExpand func(expanded bool)
OnEllipsis func(ellipsis bool)

// 操作栏
SetActionsPlacement(start|end)    // 默认 end

// 布局 / 主题
SetMaxWidth(float64)
SetFontSize(float64)              // 0 → 形态默认（正文 14 / Title 阶梯）
SetFace(text.Face)
SetTheme(*Theme) / Theme 字段
Style 可选覆盖

// 交互 / a11y
OnClick func()
SetAriaLabel(string)
// 操作按钮自带可访问名：复制 / 编辑 / 展开|收起

// 挂树
Node() core.Node
ContentNode() core.Node           // 文本内容节点（测省略 / 字号）
ChromeNode() core.Node            // code/mark/kbd 外壳（适用者）
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Kind | Text（`NewText` / `NewTypography`） |
| Level（Title） | 1（字号 38） |
| Type | default（`colorText`） |
| Disabled / Copyable / Editable / Ellipsis | false |
| ActionsPlacement | end |
| EllipsisRows | 1（Text 单行）；Paragraph 可多行由调用方 Set |
| ExpandSymbol | `展开` / `收起` |
| 正文 FontSize | 14（Token `fontSize`） |
| Title FontSize | 38 / 30 / 24 / 20 / 16 |
| 受控 editing / expanded | 未 `SetEditing` / `SetExpanded` 时用内部非受控状态 |

### 6.11 结构与绘制分层（实现提示）

```text
host (Flex row；无操作时可为内容本身)
  ├─ actions? (placement=start)
  ├─ content chrome? (code / mark / keyboard → Decorated)
  │    └─ Text | EditableText（editing）
  └─ actions? (placement=end)：expand · edit · copy
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读 Default/字段/Token；值变更优先 patch，避免无谓整树重建。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 复制成功反馈跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Typography 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/typography.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Typography 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
