# Typography 排版
> 来源：[Ant Design 6.5.x Typography](https://ant.design/components/typography)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：通用（General）  
> 说明：文本的基本格式。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
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
