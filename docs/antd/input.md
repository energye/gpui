# Input 输入框
> 来源：[Ant Design 6.5.x Input](https://ant.design/components/input)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：通过鼠标或键盘输入内容，是最基础的表单域的包装。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

通过鼠标或键盘输入内容，是最基础的表单域的包装。

**Input** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本使用 | 复现「基本使用」视觉与布局 |
| 三种大小 | 不同 size 档位 |
| 形态变体 | variant 线框/填充差异 |
| 紧凑模式 | 复现「紧凑模式」视觉与布局 |
| 搜索框 | 带搜索框外观 |
| 搜索框 loading | 带搜索框外观 |
| 文本域 | 复现「文本域」视觉与布局 |
| 适应文本高度的文本域 | 复现「适应文本高度的文本域」视觉与布局 |
| 一次性密码框 | 复现「一次性密码框」视觉与布局 |
| 输入时格式化展示 | 复现「输入时格式化展示」视觉与布局 |
| 前缀和后缀 | 复现「前缀和后缀」视觉与布局 |
| 密码框 | 复现「密码框」视觉与布局 |
| 带移除图标 | icon 与文本混排 |
| 带字数提示 | 复现「带字数提示」视觉与布局 |
| = 5.10.0">定制计数能力 | 复现「= 5.10.0">定制计数能力」视觉与布局 |
| 自定义状态 | 自定义渲染/插槽外观 |
| 聚焦 | 复现「聚焦」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `allowClear`

- **说明**：可以点击清除图标删除内容
- **类型**：boolean | { clearIcon: ReactNode, disabled?: boolean }
- **默认值**：-
- **版本**：disabled: 6.4.0

#### `bordered`

- **说明**：是否有边框, 请使用 `variant` 替换
- **类型**：boolean
- **默认值**：true
- **版本**：4.5.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `variant` | 官方取值 `variant` |

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：5.4.0

#### `count`

- **说明**：字符计数配置
- **类型**：[CountConfig](#countconfig)
- **默认值**：-
- **版本**：5.10.0

#### `disabled`

- **说明**：是否禁用状态，默认为 false
- **类型**：boolean
- **默认值**：false

#### `prefix`

- **说明**：带有前缀图标的 input
- **类型**：ReactNode
- **默认值**：-

#### `status`

- **说明**：设置校验状态
- **类型**：'error' | 'warning'
- **默认值**：-
- **版本**：4.19.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `error` | 错误红语义 |
  | `warning` | 警告橙语义 |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：5.4.0

#### `size`

- **说明**：控件大小。注：标准表单内的输入框大小限制为 `medium`
- **类型**：`large` | `medium` | `small`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `suffix`

- **说明**：带有后缀图标的 input
- **类型**：ReactNode
- **默认值**：-

#### `type`

- **说明**：声明 input 类型，同原生 input 标签的 type 属性，见：[MDN](https://developer.mozilla.org/zh-CN/docs/Web/HTML/Element/input#属性)(请直接使用 `Input.TextArea` 代替 `type="textarea"`)
- **类型**：string
- **默认值**：`text`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `Input.TextArea` | 官方取值 `Input.TextArea` |

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

#### `loading`

- **说明**：搜索 loading
- **类型**：boolean
- **默认值**：false

#### `onSearch`

- **说明**：点击搜索图标、清除图标，或按下回车键时的回调
- **类型**：function(value, event, { source: "input" | "clear" })
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `input` | 官方取值 `input` |
  | `clear` | 官方取值 `clear` |

#### `searchIcon`

- **说明**：自定义搜索图标
- **类型**：ReactNode
- **默认值**：-
- **版本**：6.4.0

#### `autoComplete`

- **说明**：输入元素的 autocomplete 属性，例如 `one-time-code` 可用于 OTP 自动填充
- **类型**：string
- **默认值**：-
- **版本**：6.3.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `one-time-code` | 官方取值 `one-time-code` |

#### `formatter`

- **说明**：格式化展示，留空字段会被 ` ` 填充
- **类型**：(value: string) => string
- **默认值**：-

#### `mask`

- **说明**：自定义展示，和 `formatter` 的区别是不会修改原始值
- **类型**：boolean | string
- **默认值**：`false`
- **版本**：`5.17.0`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `formatter` | 官方取值 `formatter` |

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

- 需要用户输入表单域内容时。
- 提供组合型输入框，带搜索的输入框，还可以进行大小选择。

### 2.2 核心功能（按官方示例拆解）

1. **基本使用**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **三种大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **形态变体**（`variant.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **紧凑模式**（`compact-style.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **搜索框**（`search-input.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **搜索框 loading**（`search-input-loading.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **文本域**（`textarea.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **适应文本高度的文本域**（`autosize-textarea.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **一次性密码框**（`otp.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **输入时格式化展示**（`tooltip.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **前缀和后缀**（`presuffix.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **密码框**（`password-input.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **带移除图标**（`allowClear.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
14. **带字数提示**（`show-count.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
15. **= 5.10.0">定制计数能力**（`advance-count.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
16. **自定义状态**（`status.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
17. **聚焦**（`focus.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
18. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 输入框内容 |
| `defaultValue` | 非受控默认值 | 输入框默认内容 |
| `onChange` | 值变化 | 输入框内容变化时的回调 |
| `disabled` | 禁用 | 是否禁用状态，默认为 false |
| `loading` | 加载中 | 搜索 loading |
| `onSearch` | 搜索回调 | 点击搜索图标、清除图标，或按下回车键时的回调 |
| `onClear` | 清除 | 按下清除按钮的回调 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本使用 | `basic.tsx` | 否 |
| 三种大小 | `size.tsx` | 否 |
| 形态变体 | `variant.tsx` | 否 |
| 面性变体 Debug | `filled-debug.tsx` | 是 |
| 前置/后置标签 | `addon.tsx` | 是 |
| 紧凑模式 | `compact-style.tsx` | 否 |
| 输入框组合 | `group.tsx` | 是 |
| 搜索框 | `search-input.tsx` | 否 |
| 搜索框 loading | `search-input-loading.tsx` | 否 |
| 文本域 | `textarea.tsx` | 否 |
| 适应文本高度的文本域 | `autosize-textarea.tsx` | 否 |
| 一次性密码框 | `otp.tsx` | 否 |
| 输入时格式化展示 | `tooltip.tsx` | 否 |
| 前缀和后缀 | `presuffix.tsx` | 否 |
| 密码框 | `password-input.tsx` | 否 |
| 带移除图标 | `allowClear.tsx` | 否 |
| 带字数提示 | `show-count.tsx` | 否 |
| = 5.10.0">定制计数能力 | `advance-count.tsx` | 否 |
| 自定义状态 | `status.tsx` | 否 |
| 聚焦 | `focus.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| Style Debug | `borderless-debug.tsx` | 是 |
| 文本对齐 | `align.tsx` | 是 |
| 文本域 | `textarea-resize.tsx` | 是 |
| debug 前置/后置标签 | `debug-addon.tsx` | 是 |
| debug token | `component-token.tsx` | 是 |

### 2.6 FAQ

## FAQ

### 为什么我动态改变 `prefix/suffix/showCount` 时，Input 会失去焦点？ {#faq-lose-focus}

当 Input 动态添加或者删除 `prefix/suffix/showCount` 时，React 会重新创建 DOM 结构而新的 input 是没有焦点的。你可以预设一个空的 `` 来保持 DOM 结构不变：

```jsx
const suffix = condition ?  : ;

;
```

### 为何 TextArea 受控时，`value` 可以超过 `maxLength`？ {#faq-textarea-exceed-max}

受控时，组件应该按照受控内容展示，这是为了防止在表单组件内使用时显示值和提交值不一致的问题。

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

### Input

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| ~~addonAfter~~ | 带标签的 input，设置后置标签，请使用 Space.Compact 替换 | ReactNode | - | ~~addonBefore~~ | 带标签的 input，设置前置标签，请使用 Space.Compact 替换 | ReactNode | - | allowClear | 可以点击清除图标删除内容 | boolean \| { clearIcon: ReactNode, disabled?: boolean } | - | disabled: 6.4.0 | 5.15.0 |
| ~~bordered~~ | 是否有边框, 请使用 `variant` 替换 | boolean | true | 4.5.0 | × |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-input), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-input), string> | - | 5.4.0 | 5.7.0 |
| count | 字符计数配置 | [CountConfig](#countconfig) | - | 5.10.0 | × |
| defaultValue | 输入框默认内容 | string | - | disabled | 是否禁用状态，默认为 false | boolean | false | - | × |
| id | 输入框的 id | string | - | maxLength | 最大长度 | number | - | prefix | 带有前缀图标的 input | ReactNode | - | showCount | 是否展示字数 | boolean \| { formatter: (info: { value: string, count: number, maxLength?: number }) => ReactNode } | false | 4.18.0 info.value: 4.23.0 | × |
| status | 设置校验状态 | 'error' \| 'warning' | - | 4.19.0 | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-input), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-input), CSSProperties> | - | 5.4.0 | 5.7.0 |
| size | 控件大小。注：标准表单内的输入框大小限制为 `medium` | `large` \| `medium` \| `small` | - | suffix | 带有后缀图标的 input | ReactNode | - | type | 声明 input 类型，同原生 input 标签的 type 属性，见：[MDN](https://developer.mozilla.org/zh-CN/docs/Web/HTML/Element/input#属性)(请直接使用 `Input.TextArea` 代替 `type="textarea"`) | string | `text` | value | 输入框内容 | string | - | variant | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 | 5.19.0 |
| onChange | 输入框内容变化时的回调 | function(e) | - | onPressEnter | 按下回车的回调 | function(e) | - | onClear | 按下清除按钮的回调 | () => void | - | 5.20.0 | × |

> 如果 `Input` 在 `Form.Item` 内，并且 `Form.Item` 设置了 `id` 属性，则 `value` `defaultValue` 和 `id` 属性会被自动设置。

Input 的其他属性和 React 自带的 [input](https://zh-hans.react.dev/reference/react-dom/components/input) 一致。

#### CountConfig

```tsx
interface CountConfig {
  // 最大字符数，不同于原生 `maxLength`，超出后标红但不会截断
  max?: number;
  // 自定义字符计数，例如标准 emoji 长度大于 1，可以自定义计数策略将其改为 1
  strategy?: (value: string) => number;
  // 同 `showCount`
  show?: boolean | ((args: { value: string; count: number; maxLength?: number }) => ReactNode);
  // 当字符数超出 `count.max` 时的自定义裁剪逻辑，不配置时不进行裁剪
  exceedFormatter?: (value: string, config: { max: number }) => string;
}
```

### Input.TextArea

同 Input 属性，外加：

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| autoSize | 自适应内容高度，可设置为 true \| false 或对象：{ minRows: 2, maxRows: 6 } | boolean \| object | false | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-textarea), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-textarea), string> | - | 5.4.0 | 5.15.0 |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-textarea) , CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-textarea) , CSSProperties> | - | 5.4.0 | 5.15.0 |

`Input.TextArea` 的其他属性和浏览器自带的 [textarea](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/textarea) 一致。

### Input.Search

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-search), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-search), string> | - | 6.0.0 | 6.0.0 |
| enterButton | 是否有确认按钮，可设为按钮文字。该属性会与 `addonAfter` 冲突。 | ReactNode | false | loading | 搜索 loading | boolean | false | onSearch | 点击搜索图标、清除图标，或按下回车键时的回调 | function(value, event, { source: "input" \| "clear" }) | - | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-search) , CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-search) , CSSProperties> | - | 6.0.0 | 6.0.0 |
| searchIcon | 自定义搜索图标 | ReactNode | - | 6.4.0 | 6.4.0 |

其余属性和 Input 一致。

### Input.Password

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 语义化结构 class | Record<[SemanticDOM](#semantic-password), string> | - | 5.4.0 | 6.4.0 |
| iconRender | 自定义切换按钮 | (visible) => ReactNode | (visible) => (visible ? &lt;EyeOutlined /> : &lt;EyeInvisibleOutlined />) | 4.3.0 | 6.4.0 |
| styles | 语义化结构 style | Record<[SemanticDOM](#semantic-password), CSSProperties> | - | 5.4.0 | 6.4.0 |
| visibilityToggle | 是否显示切换按钮或者控制密码显隐 | boolean \| [VisibilityToggle](#visibilitytoggle) | true 
### Input.OTP

`5.16.0` 新增。

> 开发者注意事项：
>
> 当 `mask` 属性的类型为 string 时，我们强烈推荐接收单个字符或单个 emoji，如果传入多个字符或多个 emoji，则会在控制台抛出警告。

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| autoComplete | 输入元素的 autocomplete 属性，例如 `one-time-code` 可用于 OTP 自动填充 | string | - | 6.3.0 | × |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-otp), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-otp), string> | - | 6.0.0 | 6.0.0 |
| defaultValue | 默认值 | string | - | disabled | 是否禁用 | boolean | false | formatter | 格式化展示，留空字段会被 ` ` 填充 | (value: string) => string | - | separator | 分隔符，在指定索引的输入框后渲染分隔符 | ReactNode \|((i: number) => ReactNode) | - | 5.24.0 | × |
| mask | 自定义展示，和 `formatter` 的区别是不会修改原始值 | boolean \| string | `false` | `5.17.0` | × |
| length | 输入元素数量 | number | 6 | status | 设置校验状态 | 'error' \| 'warning' | - | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-otp) , CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-otp) , CSSProperties> | - | 6.0.0 | 6.0.0 |
| size | 输入框大小 | `small` \| `medium` \| `large` | `medium` | variant | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | `underlined`: 5.24.0 | × |
| value | 输入框内容 | string | - | onChange | 当输入框内容全部填充时触发回调 | (value: string) => void | - | onInput | 输入值变化时触发的回调 | (value: string[]) => void | - | `5.22.0` | × |

#### VisibilityToggle

| 参数            | 说明                      | 类型              | 默认值 | 版本  |
| --------------- | ------------------------- | ----------------- | ------ | ----- |
| tabIndex        | 设置切换按钮的 `tabIndex` | number            | 0      | 6.5.0 |
| visible         | 用于手动控制密码显隐      | boolean           | false  | 4.24  |
| onVisibleChange | 显隐密码的回调            | (visible) => void | -      | 4.24  |

#### Input Methods

| 名称 | 说明 | 参数 | 版本 |
| --- | --- | --- | --- |
| blur | 取消焦点 | - 
### 导入方式

```js
import { Input } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `addonAfter` | 带标签的 input，设置后置标签，请使用 Space.Compact 替换 | ReactNode | - | — |
| `addonBefore` | 带标签的 input，设置前置标签，请使用 Space.Compact 替换 | ReactNode | - | — |
| `allowClear` | 可以点击清除图标删除内容 | boolean \| { clearIcon: ReactNode, disabled?: boolean } | - | disabled: 6.4.0 |
| `bordered` | 是否有边框, 请使用 `variant` 替换 | boolean | true | 4.5.0 |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | 5.4.0 |
| `count` | 字符计数配置 | [CountConfig](#countconfig) | - | 5.10.0 |
| `defaultValue` | 输入框默认内容 | string | - | — |
| `disabled` | 是否禁用状态，默认为 false | boolean | false | - |
| `id` | 输入框的 id | string | - | — |
| `maxLength` | 最大长度 | number | - | — |
| `prefix` | 带有前缀图标的 input | ReactNode | - | — |
| `showCount` | 是否展示字数 | boolean \| { formatter: (info: { value: string, count: number, maxLength?: number }) => ReactNode } | false | 4.18.0 info.value: 4.23.0 |
| `status` | 设置校验状态 | 'error' \| 'warning' | - | 4.19.0 |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | 5.4.0 |
| `size` | 控件大小。注：标准表单内的输入框大小限制为 `medium` | `large` \| `medium` \| `small` | - | — |
| `suffix` | 带有后缀图标的 input | ReactNode | - | — |
| `type` | 声明 input 类型，同原生 input 标签的 type 属性，见：[MDN](https://developer.mozilla.org/zh-CN/docs/Web/HTML/Element/input#属性)(请直接使用 `Input.TextArea` 代替 `type="textarea"`) | string | `text` | — |
| `value` | 输入框内容 | string | - | — |
| `variant` | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 |
| `onChange` | 输入框内容变化时的回调 | function(e) | - | — |
| `onPressEnter` | 按下回车的回调 | function(e) | - | — |
| `onClear` | 按下清除按钮的回调 | () => void | - | 5.20.0 |
| `autoSize` | 自适应内容高度，可设置为 true \| false 或对象：{ minRows: 2, maxRows: 6 } | boolean \| object | false | — |
| `enterButton` | 是否有确认按钮，可设为按钮文字。该属性会与 `addonAfter` 冲突。 | ReactNode | false | — |
| `loading` | 搜索 loading | boolean | false | — |
| `onSearch` | 点击搜索图标、清除图标，或按下回车键时的回调 | function(value, event, { source: "input" \| "clear" }) | - | — |
| `searchIcon` | 自定义搜索图标 | ReactNode | - | 6.4.0 |
| `iconRender` | 自定义切换按钮 | (visible) => ReactNode | (visible) => (visible ? <EyeOutlined /> : <EyeInvisibleOutlined />) | 4.3.0 |
| `visibilityToggle` | 是否显示切换按钮或者控制密码显隐 | boolean \| [VisibilityToggle](#visibilitytoggle) | true | — |
| `autoComplete` | 输入元素的 autocomplete 属性，例如 `one-time-code` 可用于 OTP 自动填充 | string | - | 6.3.0 |
| `formatter` | 格式化展示，留空字段会被 ` ` 填充 | (value: string) => string | - | — |
| `separator` | 分隔符，在指定索引的输入框后渲染分隔符 | ReactNode \|((i: number) => ReactNode) | - | 5.24.0 |
| `mask` | 自定义展示，和 `formatter` 的区别是不会修改原始值 | boolean \| string | `false` | `5.17.0` |
| `length` | 输入元素数量 | number | 6 | — |
| `onInput` | 输入值变化时触发的回调 | (value: string[]) => void | - | `5.22.0` |
| `tabIndex` | 设置切换按钮的 `tabIndex` | number | 0 | 6.5.0 |
| `visible` | 用于手动控制密码显隐 | boolean | false | 4.24 |
| `onVisibleChange` | 显隐密码的回调 | (visible) => void | - | 4.24 |
| `(option?: { preventScroll?: boolean, cursor?: 'start' | 'end' | 'all' })` | 获取焦点 | — | — | option - 4.10.0 |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Input** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **18** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/input
- 中文文档：https://ant.design/components/input-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/input
- 驱动 gpui kit：`input`
