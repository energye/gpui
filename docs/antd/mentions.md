# Mentions 提及
> 来源：[Ant Design 6.5.x Mentions](https://ant.design/components/mentions)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：用于在输入中提及某人或某事。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

用于在输入中提及某人或某事。

**Mentions** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本使用 | 复现「基本使用」视觉与布局 |
| 尺寸 | 不同 size 档位的高宽/字号/内边距 |
| 形态变体 | variant 线框/填充差异 |
| 异步加载 | loading 指示与防重复 |
| 配合 Form 使用 | 复现「配合 Form 使用」视觉与布局 |
| 自定义触发字符 | 自定义渲染/插槽外观 |
| 无效或只读 | 复现「无效或只读」视觉与布局 |
| 向上展开 | 展开/折叠指示 |
| 带移除图标 | icon 与文本混排 |
| 自动大小 | 不同 size 档位 |
| 自定义状态 | 自定义渲染/插槽外观 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `allowClear`

- **说明**：可以点击清除图标删除内容
- **类型**：boolean | { clearIcon?: ReactNode, disabled?: boolean }
- **默认值**：false
- **版本**：5.13.0, disabled: 6.4.0

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `placement`

- **说明**：弹出层展示位置
- **类型**：`top` | `bottom`
- **默认值**：`bottom`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `top` | 上方 |
  | `bottom` | 下方 |

#### `prefix`

- **说明**：设置触发关键字
- **类型**：string | string\[]
- **默认值**：`@`

#### `size`

- **说明**：控件大小
- **类型**：`large` | `medium` | `small`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `status`

- **说明**：设置校验状态
- **类型**：'error' | 'warning' | 'success' | 'validating'
- **默认值**：-
- **版本**：4.19.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `error` | 错误红语义 |
  | `warning` | 警告橙语义 |
  | `success` | 成功绿语义 |
  | `validating` | 官方取值 `validating` |

#### `variant`

- **说明**：形态变体
- **类型**：`outlined` | `borderless` | `filled` | `underlined`
- **默认值**：`outlined`
- **版本**：5.13.0 | `underlined`: 5.24.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `outlined` | 描边空心 |
  | `borderless` | 无边框 |
  | `filled` | 浅底填充 |
  | `underlined` | 底边线形态 |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `label`

- **说明**：选项的标题
- **类型**：React.ReactNode
- **默认值**：-

#### `disabled`

- **说明**：是否可选
- **类型**：boolean
- **默认值**：-

#### `style`

- **说明**：选项样式
- **类型**：React.CSSProperties
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

用于在输入中提及某人或某事，常用于发布、聊天或评论功能。

### 2.2 核心功能（按官方示例拆解）

1. **基本使用**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **尺寸**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **形态变体**（`variant.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **异步加载**（`async.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **配合 Form 使用**（`form.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **自定义触发字符**（`prefix.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **无效或只读**（`readonly.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **向上展开**（`placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **带移除图标**（`allowClear.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **自动大小**（`autoSize.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **自定义状态**（`status.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 设置值 |
| `defaultValue` | 非受控默认值 | 默认值 |
| `onChange` | 值变化 | 值改变时触发 |
| `onSelect` | 选中 | 选择选项时触发 |
| `disabled` | 禁用 | 是否可选 |
| `options` | 数据化 options | 选项配置 |
| `filterOption` | 过滤 | 自定义过滤逻辑 |
| `getPopupContainer` | 浮层容器 | 指定建议框挂载的 HTML 节点 |
| `onSearch` | 搜索回调 | 搜索时触发 |
| `onClear` | 清除 | 按下清除按钮的回调 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本使用 | `basic.tsx` | 否 |
| 尺寸 | `size.tsx` | 否 |
| 形态变体 | `variant.tsx` | 否 |
| 异步加载 | `async.tsx` | 否 |
| 配合 Form 使用 | `form.tsx` | 否 |
| 自定义触发字符 | `prefix.tsx` | 否 |
| 无效或只读 | `readonly.tsx` | 否 |
| 向上展开 | `placement.tsx` | 否 |
| 带移除图标 | `allowClear.tsx` | 否 |
| 自动大小 | `autoSize.tsx` | 否 |
| debug 自动大小 | `autosize-textarea-debug.tsx` | 是 |
| 自定义状态 | `status.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| _InternalPanelDoNotUseOrYouWillBeFired | `render-panel.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |

### 2.5 实例方法 / Ref

#### 方法

### Mentions 方法

| 名称    | 描述     |
| ------- | -------- |
| blur()  | 移除焦点 |
| focus() | 获取焦点 |

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

### Mentions

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| allowClear | 可以点击清除图标删除内容 | boolean \| { clearIcon?: ReactNode, disabled?: boolean } | false | 5.13.0, disabled: 6.4.0 | 6.4.0 |
| autoSize | 自适应内容高度，可设置为 true \| false 或对象：{ minRows: 2, maxRows: 6 } | boolean \| object | false | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | defaultValue | 默认值 | string | - | filterOption | 自定义过滤逻辑 | false \| (input: string, option: OptionProps) => boolean | - | getPopupContainer | 指定建议框挂载的 HTML 节点 | () => HTMLElement | - | notFoundContent | 当下拉列表为空时显示的内容 | ReactNode | `Not Found` | placement | 弹出层展示位置 | `top` \| `bottom` | `bottom` | prefix | 设置触发关键字 | string \| string\[] | `@` | split | 设置选中项前后分隔符 | string | ` ` | size | 控件大小 | `large` \| `medium` \| `small` | - | status | 设置校验状态 | 'error' \| 'warning' \| 'success' \| 'validating' | - | 4.19.0 | × |
| validateSearch | 自定义触发验证逻辑 | (text: string, props: MentionsProps) => void | - | value | 设置值 | string | - | variant | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 | 5.19.0 |
| onBlur | 失去焦点时触发 | () => void | - | onChange | 值改变时触发 | (text: string) => void | - | onClear | 按下清除按钮的回调 | () => void | - | 5.20.0 | × |
| onFocus | 获得焦点时触发 | () => void | - | onResize | resize 回调 | function({ width, height }) | - | onSearch | 搜索时触发 | (text: string, prefix: string) => void | - | onSelect | 选择选项时触发 | (option: OptionProps, prefix: string) => void | - | onPopupScroll | 滚动时触发 | (event: Event) => void | - | 5.23.0 | × |
| options | 选项配置 | [Options](#option) | [] | 5.1.0 | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - 
### Mentions 方法

| 名称    | 描述     |
| ------- | -------- |
| blur()  | 移除焦点 |
| focus() | 获取焦点 |

### Option

| 参数      | 说明           | 类型                | 默认值 |
| --------- | -------------- | ------------------- | ------ |
| value     | 选择时填充的值 | string              | -      |
| label     | 选项的标题     | React.ReactNode     | -      |
| key       | 选项的 key 值  | string              | -      |
| disabled  | 是否可选       | boolean             | -      |
| className | css 类名       | string              | -      |
| style     | 选项样式       | React.CSSProperties | -      |

### 导入方式

```js
import { Mentions } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `allowClear` | 可以点击清除图标删除内容 | boolean \| { clearIcon?: ReactNode, disabled?: boolean } | false | 5.13.0, disabled: 6.4.0 |
| `autoSize` | 自适应内容高度，可设置为 true \| false 或对象：{ minRows: 2, maxRows: 6 } | boolean \| object | false | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `defaultValue` | 默认值 | string | - | — |
| `filterOption` | 自定义过滤逻辑 | false \| (input: string, option: OptionProps) => boolean | - | — |
| `getPopupContainer` | 指定建议框挂载的 HTML 节点 | () => HTMLElement | - | — |
| `notFoundContent` | 当下拉列表为空时显示的内容 | ReactNode | `Not Found` | — |
| `placement` | 弹出层展示位置 | `top` \| `bottom` | `bottom` | — |
| `prefix` | 设置触发关键字 | string \| string\[] | `@` | — |
| `split` | 设置选中项前后分隔符 | string | ` ` | — |
| `size` | 控件大小 | `large` \| `medium` \| `small` | - | — |
| `status` | 设置校验状态 | 'error' \| 'warning' \| 'success' \| 'validating' | - | 4.19.0 |
| `validateSearch` | 自定义触发验证逻辑 | (text: string, props: MentionsProps) => void | - | — |
| `value` | 设置值 | string | - | — |
| `variant` | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 |
| `onBlur` | 失去焦点时触发 | () => void | - | — |
| `onChange` | 值改变时触发 | (text: string) => void | - | — |
| `onClear` | 按下清除按钮的回调 | () => void | - | 5.20.0 |
| `onFocus` | 获得焦点时触发 | () => void | - | — |
| `onResize` | resize 回调 | function({ width, height }) | - | — |
| `onSearch` | 搜索时触发 | (text: string, prefix: string) => void | - | — |
| `onSelect` | 选择选项时触发 | (option: OptionProps, prefix: string) => void | - | — |
| `onPopupScroll` | 滚动时触发 | (event: Event) => void | - | 5.23.0 |
| `options` | 选项配置 | [Options](#option) | [] | 5.1.0 |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `blur()` | 移除焦点 | — | — | — |
| `focus()` | 获取焦点 | — | — | — |
| `label` | 选项的标题 | React.ReactNode | - | — |
| `key` | 选项的 key 值 | string | - | — |
| `disabled` | 是否可选 | boolean | - | — |
| `className` | css 类名 | string | - | — |
| `style` | 选项样式 | React.CSSProperties | - | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Mentions** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **12** 个，均需可复现。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/mentions
- 中文文档：https://ant.design/components/mentions-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/mentions
- 驱动 gpui kit：`mentions`
