# Form 表单
> 来源：[Ant Design 6.5.x Form](https://ant.design/components/form)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：高性能表单控件，自带数据域管理。包含数据录入、校验以及对应样式。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

高性能表单控件，自带数据域管理。包含数据录入、校验以及对应样式。

**Form** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本使用 | 复现「基本使用」视觉与布局 |
| 表单方法调用 | 复现「表单方法调用」视觉与布局 |
| 表单布局 | 复现「表单布局」视觉与布局 |
| 表单混合布局 | 复现「表单混合布局」视觉与布局 |
| 表单禁用 | disabled 灰态与不可点 |
| 表单变体 | variant 线框/填充差异 |
| 必选样式 | 复现「必选样式」视觉与布局 |
| 表单尺寸 | 不同 size 档位的高宽/字号/内边距 |
| 表单标签可换行 | 复现「表单标签可换行」视觉与布局 |
| 非阻塞校验 | 复现「非阻塞校验」视觉与布局 |
| 字段监听 Hooks | 复现「字段监听 Hooks」视觉与布局 |
| 校验时机 | 复现「校验时机」视觉与布局 |
| 仅校验 | 复现「仅校验」视觉与布局 |
| 字段路径前缀 | 复现「字段路径前缀」视觉与布局 |
| 动态增减表单项 | 复现「动态增减表单项」视觉与布局 |
| 动态增减嵌套字段 | 复现「动态增减嵌套字段」视觉与布局 |
| 拖拽排序 | 复现「拖拽排序」视觉与布局 |
| 复杂的动态增减表单项 | 复现「复杂的动态增减表单项」视觉与布局 |
| 嵌套结构与校验信息 | 复现「嵌套结构与校验信息」视觉与布局 |
| 复杂一点的控件 | 复现「复杂一点的控件」视觉与布局 |
| 自定义表单控件 | 自定义渲染/插槽外观 |
| 表单数据存储于上层组件 | 复现「表单数据存储于上层组件」视觉与布局 |
| 多表单联动 | 复现「多表单联动」视觉与布局 |
| 内联登录栏 | 复现「内联登录栏」视觉与布局 |
| 登录框 | 复现「登录框」视觉与布局 |
| 注册新用户 | 复现「注册新用户」视觉与布局 |
| 高级搜索 | 带搜索框外观 |
| 弹出层中的新建表单 | 复现「弹出层中的新建表单」视觉与布局 |
| 时间类控件 | 复现「时间类控件」视觉与布局 |
| 自行处理表单数据 | 复现「自行处理表单数据」视觉与布局 |
| 自定义校验 | 自定义渲染/插槽外观 |
| 动态校验规则 | 复现「动态校验规则」视觉与布局 |
| 校验与更新依赖 | 复现「校验与更新依赖」视觉与布局 |
| 滑动到错误字段 | 复现「滑动到错误字段」视觉与布局 |
| 校验其他组件 | 复现「校验其他组件」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |
| getValueProps + normalize | 复现「getValueProps + normalize」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `disabled`

- **说明**：设置表单组件禁用，仅对 antd 组件有效
- **类型**：boolean
- **默认值**：false
- **版本**：4.21.0

#### `fields`

- **说明**：通过状态管理（如 redux）控制表单字段，如非强需求不推荐使用。查看[示例](#form-demo-global-state)
- **类型**：[FieldData](#fielddata)\[]
- **默认值**：-

#### `feedbackIcons`

- **说明**：当 `Form.Item` 有 `hasFeedback` 属性时可以自定义图标
- **类型**：[FeedbackIcons](#feedbackicons)
- **默认值**：-
- **版本**：5.9.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `Form.Item` | 官方取值 `Form.Item` |
  | `hasFeedback` | 官方取值 `hasFeedback` |

#### `layout`

- **说明**：表单布局
- **类型**：`horizontal` | `vertical` | `inline`
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |
  | `vertical` | 垂直排布 |
  | `inline` | 内联紧凑 |

#### `name`

- **说明**：表单名称，会作为表单字段 `id` 前缀使用
- **类型**：string
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `id` | 官方取值 `id` |

#### `requiredMark`

- **说明**：必选样式，可以切换为必选或者可选展示样式。此为 Form 配置，Form.Item 无法单独配置
- **类型**：boolean | `optional` | ((label: ReactNode, info: { required: boolean }) => ReactNode)
- **默认值**：true
- **版本**：`renderProps`: 5.9.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `optional` | 官方取值 `optional` |

#### `size`

- **说明**：设置字段组件的尺寸（仅限 antd 组件）
- **类型**：`small` | `medium` | `large`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `small` | 小尺寸（更紧凑） |
  | `medium` | 中尺寸（默认节奏） |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `variant`

- **说明**：表单内控件变体
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

#### `wrapperCol`

- **说明**：需要为输入控件设置布局样式时，使用该属性，用法同 labelCol
- **类型**：[object](/components/grid-cn#col)
- **默认值**：-

#### `extra`

- **说明**：额外的提示信息，和 `help` 类似，当需要错误信息和提示文案同时出现时，可以使用这个。
- **类型**：ReactNode
- **默认值**：-

#### `hasFeedback`

- **说明**：配合 `validateStatus` 属性使用，展示校验状态图标，建议只配合 Input 组件使用 此外，它还可以通过 Icons 属性获取反馈图标。
- **类型**：boolean | { icons: [FeedbackIcons](#feedbackicons) }
- **默认值**：false
- **版本**：icons: 5.9.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `validateStatus` | 官方取值 `validateStatus` |

#### `noStyle`

- **说明**：为 `true` 时不带样式，作为纯字段控件使用。当自身没有 `validateStatus` 而父元素存在有 `validateStatus` 的 Form.Item 会继承父元素的 `validateStatus`
- **类型**：boolean
- **默认值**：false
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `validateStatus` | 官方取值 `validateStatus` |

#### `required`

- **说明**：必填样式设置。如不设置，则会根据校验规则自动生成
- **类型**：boolean
- **默认值**：false

#### `validateStatus`

- **说明**：校验状态，如不设置，则会根据校验规则自动生成，可选：'success' 'warning' 'error' 'validating'
- **类型**：string
- **默认值**：-

#### `scrollToField`

- **说明**：滚动到对应字段位置
- **类型**：(name: [NamePath](#namepath), options: [ScrollOptions](https://github.com/stipsan/scroll-into-view-if-needed/tree/ece40bd9143f48caf4b99503425ecb16b0ad8249#options) | { focus: boolean }) => void
- **默认值**：—
- **版本**：focus: 5.24.0

#### `setFields`

- **说明**：设置一组字段状态
- **类型**：(fields: [FieldData](#fielddata)\[]) => void
- **默认值**：—

#### `defaultField`

- **说明**：仅在 `type` 为 `array` 类型时有效，用于指定数组元素的校验规则
- **类型**：[rule](#rule)
- **默认值**：—
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `type` | 官方取值 `type` |

#### `len`

- **说明**：string 类型时为字符串长度；number 类型时为确定数字； array 类型时为数组长度
- **类型**：number
- **默认值**：—

#### `max`

- **说明**：必须设置 `type`：string 类型为字符串最大长度；number 类型时为最大值；array 类型时为数组最大长度
- **类型**：number
- **默认值**：—
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `type` | 官方取值 `type` |

#### `min`

- **说明**：必须设置 `type`：string 类型为字符串最小长度；number 类型时为最小值；array 类型时为数组最小长度
- **类型**：number
- **默认值**：—
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `type` | 官方取值 `type` |

#### `type`

- **说明**：类型，常见有 `string` | `number` | `boolean` | `url` | `email` | `tel`。更多请参考[此处](https://github.com/react-component/async-validator#type)
- **类型**：string
- **默认值**：—
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `url` | 官方取值 `url` |
  | `email` | 官方取值 `email` |
  | `tel` | 官方取值 `tel` |

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

- 用于创建一个实体或收集信息。
- 需要对输入的数据类型进行校验时。

### 2.2 核心功能（按官方示例拆解）

1. **基本使用**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **表单方法调用**（`control-hooks.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **表单布局**（`layout.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **表单混合布局**（`layout-multiple.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **表单禁用**（`disabled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **表单变体**（`variant.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **必选样式**（`required-mark.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **表单尺寸**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **表单标签可换行**（`layout-can-wrap.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **非阻塞校验**（`warning-only.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **字段监听 Hooks**（`useWatch.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **校验时机**（`validate-trigger.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **仅校验**（`validate-only.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
14. **字段路径前缀**（`form-item-path.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
15. **动态增减表单项**（`dynamic-form-item.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
16. **动态增减嵌套字段**（`dynamic-form-items.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
17. **拖拽排序**（`dynamic-form-items-drag-sorting.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
18. **复杂的动态增减表单项**（`dynamic-form-items-complex.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
19. **嵌套结构与校验信息**（`nest-messages.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
20. **复杂一点的控件**（`complex-form-control.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
21. **自定义表单控件**（`customized-form-controls.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
22. **表单数据存储于上层组件**（`global-state.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
23. **多表单联动**（`form-context.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
24. **内联登录栏**（`inline-login.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
25. **登录框**（`login.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
26. **注册新用户**（`register.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
27. **高级搜索**（`advanced-search.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
28. **弹出层中的新建表单**（`form-in-modal.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
29. **时间类控件**（`time-related-controls.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
30. **自行处理表单数据**（`without-form-create.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
31. **自定义校验**（`validate-static.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
32. **动态校验规则**（`dynamic-rule.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
33. **校验与更新依赖**（`form-dependencies.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
34. **滑动到错误字段**（`validate-scroll-to-field.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
35. **校验其他组件**（`validate-other.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
36. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
37. **getValueProps + normalize**（`getValueProps-normalize.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 字段对应值 |
| `disabled` | 禁用 | 设置表单组件禁用，仅对 antd 组件有效 |
| `rules` | 校验规则 | 校验规则，设置字段的校验逻辑。点击[此处](#form-demo-basic)查看示例 |
| `onFinish` | 提交成功 | 提交表单且数据验证成功后回调事件 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本使用 | `basic.tsx` | 否 |
| 表单方法调用 | `control-hooks.tsx` | 否 |
| 表单布局 | `layout.tsx` | 否 |
| 表单混合布局 | `layout-multiple.tsx` | 否 |
| 表单禁用 | `disabled.tsx` | 否 |
| 表单变体 | `variant.tsx` | 否 |
| 必选样式 | `required-mark.tsx` | 否 |
| 表单尺寸 | `size.tsx` | 否 |
| 表单标签可换行 | `layout-can-wrap.tsx` | 否 |
| 非阻塞校验 | `warning-only.tsx` | 否 |
| 字段监听 Hooks | `useWatch.tsx` | 否 |
| 校验时机 | `validate-trigger.tsx` | 否 |
| 仅校验 | `validate-only.tsx` | 否 |
| 字段路径前缀 | `form-item-path.tsx` | 否 |
| 动态增减表单项 | `dynamic-form-item.tsx` | 否 |
| 动态增减嵌套字段 | `dynamic-form-items.tsx` | 否 |
| 拖拽排序 | `dynamic-form-items-drag-sorting.tsx` | 否 |
| 动态增减嵌套纯字段 | `dynamic-form-items-no-style.tsx` | 是 |
| 复杂的动态增减表单项 | `dynamic-form-items-complex.tsx` | 否 |
| 嵌套结构与校验信息 | `nest-messages.tsx` | 否 |
| 复杂一点的控件 | `complex-form-control.tsx` | 否 |
| 自定义表单控件 | `customized-form-controls.tsx` | 否 |
| 表单数据存储于上层组件 | `global-state.tsx` | 否 |
| 多表单联动 | `form-context.tsx` | 否 |
| 内联登录栏 | `inline-login.tsx` | 否 |
| 登录框 | `login.tsx` | 否 |
| 注册新用户 | `register.tsx` | 否 |
| 高级搜索 | `advanced-search.tsx` | 否 |
| 弹出层中的新建表单 | `form-in-modal.tsx` | 否 |
| 时间类控件 | `time-related-controls.tsx` | 否 |
| 自行处理表单数据 | `without-form-create.tsx` | 否 |
| 自定义校验 | `validate-static.tsx` | 否 |
| 动态校验规则 | `dynamic-rule.tsx` | 否 |
| 校验与更新依赖 | `form-dependencies.tsx` | 否 |
| 滑动到错误字段 | `validate-scroll-to-field.tsx` | 否 |
| 校验其他组件 | `validate-other.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| getValueProps + normalize | `getValueProps-normalize.tsx` | 否 |
| Disabled Input Debug | `disabled-input-debug.tsx` | 是 |
| 测试 label 省略 | `label-debug.tsx` | 是 |
| 测试特殊 col 24 用法 | `col-24-debug.tsx` | 是 |
| 引用字段 | `ref-item.tsx` | 是 |
| Custom feedback icons | `custom-feedback-icons.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |

### 2.5 实例方法 / Ref

#### 方法

### 为什么 `normalize` 不能是异步方法？ {#faq-normalize-async}

React 中异步更新会导致受控组件交互行为异常。当用户交互触发 `onChange` 后，通过异步改变值会导致组件 `value` 不会立刻更新，使得组件呈现假死状态。如果你需要异步触发变更，请通过自定义组件实现内部异步状态。

### 2.6 FAQ

## FAQ

### Segmented 为什么不能被 Form `disabled` 禁用? {#faq-segmented-cannot-disabled}

Segmented 设计上为数据展示类组件，而非表单控件组件。虽然它可以作为类似 Radio 的表单控件使用，但并非为此设计。因而行为上更类似于 Tabs 组件，不会被 Form 的 `disabled` 所禁用。相关讨论参考 [#54749](https://github.com/ant-design/ant-design/pull/54749#issuecomment-3797737096)。

### Switch、Checkbox 为什么不能绑定数据？ {#faq-switch-checkbox-binding}

Form.Item 默认绑定值属性到 `value` 上，而 Switch、Checkbox 等组件的值属性为 `checked`。你可以通过 `valuePropName` 来修改绑定的值属性。

```tsx | pure

  

```

### name 为数组时的转换规则？ {#faq-name-array-rule}

当 `name` 为数组时，会按照顺序填充路径。当存在数字且 form store 中没有该字段时会自动转变成数组。因而如果需要数组为 key 时请使用 string 如：`['1', 'name']`。

### 为何在 Modal 中调用 form 控制台会报错？ {#faq-form-modal-error}

> Warning: Instance created by `useForm` is not connect to any Form element. Forget to pass `form` prop?

这是因为你在调用 form 方法时，Modal 还未初始化导致 form 没有关联任何 Form 组件。你可以通过给 Modal 设置 `forceRender` 将其预渲染。示例点击[此处](https://codesandbox.io/s/antd-reproduction-template-ibu5c)。

### 为什么 Form.Item 下的子组件 `defaultValue` 不生效？ {#faq-item-default-value}

当你为 Form.Item 设置 `name` 属性后，子组件会转为受控模式。因而 `defaultValue` 不会生效。你需要在 Form 上通过 `initialValues` 设置默认值。

### 为什么第一次调用 `ref` 的 Form 为空？ {#faq-ref-first-call}

`ref` 仅在节点被加载时才会被赋值，请参考 React 官方文档：

### 为什么 `resetFields` 会重新 mount 组件？ {#faq-reset-fields-mount}

`resetFields` 会重置整个 Field，因而其子组件也会重新 mount 从而消除自定义组件可能存在的副作用（例如异步数据、状态等等）。

### Form 的 initialValues 与 Item 的 initialValue 区别？ {#faq-initial-values-diff}

在大部分场景下，我们总是推荐优先使用 Form 的 `initialValues`。只有存在动态字段时你才应该使用 Item 的 `initialValue`。默认值遵循以下规则：

1. Form 的 `initialValues` 拥有最高优先级
2. Field 的 `initialValue` 次之 \*. 多个同 `name` Item 都设置 `initialValue` 时，则 Item 的 `initialValue` 不生效

### 为什么 `getFieldsValue` 在初次渲染的时候拿不到值？ {#faq-get-fields-value}

`getFieldsValue` 默认返回收集的字段数据，而在初次渲染时 Form.Item 节点尚未渲染，因而无法收集到数据。你可以通过 `getFieldsValue(true)` 来获取所有字段数据。

### 为什么 `setFieldsValue` 设置字段为 `undefined` 时，有的组件不会重置为空？ {#faq-set-fields-undefined}

在 React 中，`value` 从确定值改为 `undefined` 表示从受控变为非受控，因而不会重置展示值（但是 Form 中的值确实已经改变）。你可以通过 HOC 改变这一逻辑：

```jsx
const MyInput = ({
  // 强制保持受控逻辑
  value = '',
  ...rest
}) => ;

  
;
```

### 为什么字段设置 `rules` 后更改值 `onFieldsChange` 会触发三次？ {#faq-rules-trigger-three-times}

字段除了本身的值变化外，校验也是其状态之一。因而在触发字段变化会经历以下几个阶段：

1. Trigger value change
2. Rule validating
3. Rule validated

在触发过程中，调用 `isFieldValidating` 会经历 `false` > `true` > `false` 的变化过程。

### 为什么 Form.List 不支持 `label` 还需要使用 ErrorList 展示错误？ {#faq-form-list-no-label}

Form.List 本身是 renderProps，内部样式非常自由。因而默认配置 `label` 和 `error` 节点很难与之配合。如果你需要 antd 样式的 `label`，可以通过外部包裹 Form.Item 来实现。

### 为什么 Form.Item 的 `dependencies` 对 Form.List 下的字段没有效果？ {#faq-dependencies-form-list}

Form.List 下的字段需要包裹 Form.List 本身的 `name`，比如：

```tsx

  {(fields) =>
    fields.map((field) => (
      
        
        
      
    ))
  }

```

依赖则是：`['users', 0, 'name']`

### 为什么 `normalize` 不能是异步方法？ {#faq-normalize-async}

React 中异步更新会导致受控组件交互行为异常。当用户交互触发 `onChange` 后，通过异步改变值会导致组件 `value` 不会立刻更新，使得组件呈现假死状态。如果你需要异步触发变更，请通过自定义组件实现内部异步状态。

### `scrollToFirstError` 和 `scrollToField` 失效？ {#faq-scroll-not-working}

1. 使用了自定义表单控件

类似问题：[#28370](https://github.com/ant-design/ant-design/issues/28370) [#27994](https://github.com/ant-design/ant-design/issues/27994)

从 `5.17.0` 版本开始，滑动操作将优先使用表单控件元素所转发的 ref 元素。因此，在考虑自定义组件支持校验滚动时，请优先考虑将其转发给表单控件元素。

滚动依赖于表单控件元素上绑定的 `id` 字段，如果自定义控件没有将 `id` 赋到正确的元素上，这个功能将失效。你可以参考这个 [codesandbox](https://codesandbox.io/s/antd-reproduction-template-forked-25nul?file=/index.js)。

2. 页面内有多个表单

页面内如果有多个表单，且存在表单项 `name` 重复，表单滚动定位可能会查找到另一个表单的同名表单项上。需要给表单 `Form` 组件设置不同的 `name` 以区分。

### 继上，为何不通过 `ref` 绑定元素？ {#faq-ref-binding}

当自定义组件不支持 `ref` 时，Form 无法获取子元素真实 DOM 节点，而通过包裹 Class Component 调用 `findDOMNode` 会在 React Strict Mode 下触发警告。因而我们使用 id 来进行元素定位。

### `setFieldsValue` 不会触发 `onFieldsChange` 和 `onValuesChange`？ {#faq-set-fields-no-trigger}

是的，change 事件仅当用户交互才会触发。该设计是为了防止在 change 事件中调用 `setFieldsValue` 导致的循环问题。如果仅仅需要组件内消费，可以通过 `useWatch` 或者 `Field.renderProps` 来实现。

### 为什么 `dependencies` 不能响应 `setFieldsValue` 触发的更新？ {#faq-dependencies-set-fields}

`dependencies` 主要用于字段间的校验联动，依赖字段由用户交互触发更新时，会重新触发当前字段的更新与校验。如果需要根据 `setFieldsValue` 后的值变化来渲染额外内容或切换字段选项，请使用 `shouldUpdate` 或 `useWatch`。

### 为什么 Form.Item 嵌套子组件后，不更新表单值？ {#faq-item-nested-update}

Form.Item 在渲染时会注入 `value` 与 `onChange` 事件给子元素，当你的字段组件被包裹时属性将无法传递。所以以下代码是不会生效的：

```jsx

  
    I am a wrapped Input
    
  

```

你可以通过 HOC 自定义组件形式来解决这个问题：

```jsx
const MyInput = (props) => (
  
    I am a wrapped Input
    
  
);

  
;
```

### 为什么表单点击 label 会更改组件状态？ {#faq-label-click-change}

> 相关 issue：[#47031](https://github.com/ant-design/ant-design/issues/47031),[#43175](https://github.com/ant-design/ant-design/issues/43175), [#52152](https://github.com/ant-design/ant-design/issues/52152)

> FAQ 较长，完整内容见官方文档。

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

### Form

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | colon | 配置 Form.Item 的 `colon` 的默认值。表示是否显示 label 后面的冒号 (只有在属性 layout 为 horizontal 时有效) | boolean | true | disabled | 设置表单组件禁用，仅对 antd 组件有效 | boolean | false | 4.21.0 | × |
| component | 设置 Form 渲染元素，为 `false` 则不创建 DOM 节点 | ComponentType \| false | form | fields | 通过状态管理（如 redux）控制表单字段，如非强需求不推荐使用。查看[示例](#form-demo-global-state) | [FieldData](#fielddata)\[] | - | form | 经 `Form.useForm()` 创建的 form 控制实例，不提供时会自动创建 | [FormInstance](#forminstance) | - | feedbackIcons | 当 `Form.Item` 有 `hasFeedback` 属性时可以自定义图标 | [FeedbackIcons](#feedbackicons) | - | 5.9.0 | × |
| initialValues | 表单默认值，只有初始化以及重置时生效 | object | - | labelAlign | label 标签的文本对齐方式 | `left` \| `right` | `right` | labelWrap | label 标签的文本换行方式 | boolean | false | 4.18.0 | × |
| labelCol | label 标签布局，同 `<Col>` 组件，设置 `span` `offset` 值，如 `{span: 3, offset: 12}` 或 `sm: {span: 3, offset: 12}` | [object](/components/grid-cn#col) | - | layout | 表单布局 | `horizontal` \| `vertical` \| `inline` | `horizontal` | name | 表单名称，会作为表单字段 `id` 前缀使用 | string | - | preserve | 当字段被删除时保留字段值。你可以通过 `getFieldsValue(true)` 来获取保留字段值 | boolean | true | 4.4.0 | × |
| requiredMark | 必选样式，可以切换为必选或者可选展示样式。此为 Form 配置，Form.Item 无法单独配置 | boolean \| `optional` \| ((label: ReactNode, info: { required: boolean }) => ReactNode) | true | `renderProps`: 5.9.0 | 4.8.0 |
| scrollToFirstError | 提交失败自动滚动到第一个错误字段 | boolean \| [Options](https://github.com/stipsan/scroll-into-view-if-needed/tree/ece40bd9143f48caf4b99503425ecb16b0ad8249#options) \| { focus: boolean } | false | focus: 5.24.0 | 5.2.0 |
| size | 设置字段组件的尺寸（仅限 antd 组件） | `small` \| `medium` \| `large` | - | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | tooltip | 配置提示属性 | [TooltipProps](/components/tooltip-cn#api) & { icon?: ReactNode } | - | 6.3.0 | 6.3.0 |
| validateMessages | 验证提示模板，说明[见下](#validatemessages) | [ValidateMessages](https://github.com/ant-design/ant-design/blob/6234509d18bac1ac60fbb3f92a5b2c6a6361295a/components/locale/en_US.ts#L88-L134) | - | validateTrigger | 统一设置字段触发验证的时机 | string \| string\[] | `onChange` | 4.3.0 | × |
| variant | 表单内控件变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 | 5.19.0 |
| wrapperCol | 需要为输入控件设置布局样式时，使用该属性，用法同 labelCol | [object](/components/grid-cn#col) | - | onFieldsChange | 字段更新时触发回调事件 | function(changedFields, allFields) | - | onFinish | 提交表单且数据验证成功后回调事件 | function(values) | - | onFinishFailed | 提交表单且数据验证失败后回调事件 | function({ values, errorFields, outOfDate }) | - | onValuesChange | 字段值更新时触发回调事件 | function(changedValues, allValues) | - | clearOnDestroy | 当表单被卸载时清空表单值 | boolean | false | 5.18.0 | × |

> 支持原生 form 除 `onSubmit` 外的所有属性。

### validateMessages

Form 为验证提供了[默认的错误提示信息](https://github.com/ant-design/ant-design/blob/6234509d18bac1ac60fbb3f92a5b2c6a6361295a/components/locale/en_US.ts#L88-L134)，你可以通过配置 `validateMessages` 属性，修改对应的提示模板。一种常见的使用方式，是配置国际化提示信息：

```jsx
const validateMessages = {
  required: "'${name}' 是必选字段",
  // ...
};

<Form validateMessages={validateMessages} />;
```

此外，[ConfigProvider](/components/config-provider-cn) 也提供了全局化配置方案，允许统一配置错误提示模板：

```jsx
const validateMessages = {
  required: "'${name}' 是必选字段",
  // ...
};

<ConfigProvider form={{ validateMessages }}>
  <Form />
</ConfigProvider>;
```

## Form.Item

表单字段组件，用于数据双向绑定、校验、布局等。

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| colon | 配合 `label` 属性使用，表示是否显示 `label` 后面的冒号 | boolean | true | extra | 额外的提示信息，和 `help` 类似，当需要错误信息和提示文案同时出现时，可以使用这个。 | ReactNode | - | getValueProps | 为子元素添加额外的属性 (不建议通过 `getValueProps` 生成动态函数 prop，请直接将其传递给子组件) | (value: any) => Record<string, any> | - | 4.2.0 |
| hasFeedback | 配合 `validateStatus` 属性使用，展示校验状态图标，建议只配合 Input 组件使用 此外，它还可以通过 Icons 属性获取反馈图标。 | boolean \| { icons: [FeedbackIcons](#feedbackicons) } | false | icons: 5.9.0 |
| help | 提示信息，如不设置，则会根据校验规则自动生成 | ReactNode | - | htmlFor | 设置子元素 label `htmlFor` 属性 | string | - | label | `label` 标签的文本，当不需要 label 又需要与冒号对齐，可以设为 null | ReactNode | - | null: 5.22.0 |
| labelAlign | 标签文本对齐方式 | `left` \| `right` | `right` | messageVariables | 默认验证字段的信息，查看[详情](#messagevariables) | Record&lt;string, string> | - | 4.7.0 |
| name | 字段名，支持数组 | [NamePath](#namepath) | - | noStyle | 为 `true` 时不带样式，作为纯字段控件使用。当自身没有 `validateStatus` 而父元素存在有 `validateStatus` 的 Form.Item 会继承父元素的 `validateStatus` | boolean | false | required | 必填样式设置。如不设置，则会根据校验规则自动生成 | boolean | false | shouldUpdate | 自定义字段更新逻辑，说明[见下](#shouldupdate) | boolean \| (prevValue, curValue) => boolean | false | trigger | 设置收集字段值变更的时机。点击[此处](#form-demo-customized-form-controls)查看示例 | string | `onChange` | validateDebounce | 设置防抖，延迟毫秒数后进行校验 | number | - | 5.9.0 |
| validateStatus | 校验状态，如不设置，则会根据校验规则自动生成，可选：'success' 'warning' 'error' 'validating' | string | - | valuePropName | 子节点的值的属性。注意：Switch、Checkbox 的 valuePropName 应该是 `checked`，否则无法获取这个两个组件的值。该属性为 `getValueProps` 的封装，自定义 `getValueProps` 后会失效 | string | `value` | layout | 表单项布局 | `horizontal` \| `vertical` | - | 5.18.0 |

被设置了 `name` 属性的 `Form.Item` 包装的控件，表单控件会自动添加 `value`（或 `valuePropName` 指定的其他属性） `onChange`（或 `trigger` 指定的其他属性），数据同步将被 Form 接管，这会导致以下结果：

1. 你**不再需要也不应该**用 `onChange` 来做数据收集同步（你可以使用 Form 的 `onValuesChange`），但还是可以继续监听 `onChange` 事件。
2. 你不能用控件的 `value` 或 `defaultValue` 等属性来设置表单域的值，默认值可以用 Form 里的 `initialValues` 来设置。注意 `initialValues` 不能被 `setState` 动态更新，你需要用 `setFieldsValue` 来更新。
3. 你不应该用 `setState`，可以使用 `form.setFieldsValue` 来动态改变表单值。

### dependencies

当字段间存在依赖关系时使用。如果一个字段设置了 `dependencies` 属性。那么它所依赖的字段更新时，该字段将自动触发更新与校验。一种常见的场景，就是注册用户表单的“密码”与“确认密码”字段。“确认密码”校验依赖于“密码”字段，设置 `dependencies` 后，“密码”字段更新会重新触发“校验密码”的校验逻辑。你可以参考[具体例子](#form-demo-dependencies)。

`dependencies` 不应和 `shouldUpdate` 一起使用，因为这可能带来更新逻辑的混乱。

### FeedbackIcons

`({ status: ValidateStatus, errors: ReactNode, warnings: ReactNode }) => Record<ValidateStatus, ReactNode>`

### shouldUpdate

Form 通过增量更新方式，只更新被修改的字段相关组件以达到性能优化目的。大部分场景下，你只需要编写代码或者与 [`dependencies`](#dependencies) 属性配合校验即可。而在某些特定场景，例如修改某个字段值后出现新的字段选项、或者纯粹希望表单任意变化都对某一个区域进行渲染。你可以通过 `shouldUpdate` 修改 Form.Item 的更新逻辑。

当 `shouldUpdate` 为 `true` 时，Form 的任意变化都会使该 Form.Item 重新渲染。这对于自定义渲染一些区域十分有帮助，要注意 Form.Item 里包裹的子组件必须由函数返回，否则 `shouldUpdate` 不会起作用：

相关issue：[#34500](https://github.com/ant-design/ant-design/issues/34500)

```jsx
<Form.Item shouldUpdate>
  {() => {
    return <pre>{JSON.stringify(form.getFieldsValue(), null, 2)}</pre>;
  }}
</Form.Item>
```

你可以参考[示例](#form-demo-inline-login)查看具体使用场景。

当 `shouldUpdate` 为方法时，表单的每次数值更新都会调用该方法，提供原先的值与当前的值以供你比较是否需要更新。这对于是否根据值来渲染额外字段十分有帮助：

```jsx
<Form.Item shouldUpdate={(prevValues, curValues) => prevValues.additional !== curValues.additional}>
  {() => {
    return (
      <Form.Item name="other">
        <Input />
      </Form.Item>
    );
  }}
</Form.Item>
```

你可以参考[示例](#form-demo-control-hooks)查看具体使用场景。

### messageVariables

你可以通过 `messageVariables` 修改 Form.Item 的默认验证信息。

```jsx
<Form>
  <Form.Item
    messageVariables={{ another: 'good' }}
    label="user"
    rules={[{ required: true, message: '${another} is required' }]}
  >
    <Input />
  </Form.Item>
  <Form.Item
    messageVariables={{ label: 'good' }}
    label={<span>user</span>}
    rules={[{ required: true, message: '${label} is required' }]}
  >
    <Input />
  </Form.Item>
</Form>
```

自 `5.20.2` 起，当你希望不要转译 `${}` 时，你可以通过 `\\${}` 来跳过：

```jsx
{ required: true, message: '${label} is convert, \\${label} is not convert' }

// good is convert, ${label} is not convert
```

## Form.List

为字段提供数组化管理。

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| children | 渲染函数 | (fields: Field\[], operation: { add, remove, move }, meta: { errors }) => React.ReactNode | - | name | 字段名，支持数组。List 本身也是字段，因而 `getFieldsValue()` 默认会返回 List 下所有值，你可以通过[参数](#getfieldsvalue)改变这一行为 | [NamePath](#namepath) | - 
```tsx
<Form.List>
  {(fields) =>
    fields.map((field) => (
      <Form.Item {...field}>
        <Input />
      </Form.Item>
    ))
  }
</Form.List>
```

注意：Form.List 下的字段不应该配置 `initialValue`，你始终应该通过 Form.List 的 `initialValue` 或者 Form 的 `initialValues` 来配置。

## operation

Form.List 渲染表单相关操作函数。

| 参数   | 说明       | 类型                                               | 默认值      | 版本  |
| ------ | ---------- | -------------------------------------------------- | ----------- | ----- |
| add    | 新增表单项 | (defaultValue?: any, insertIndex?: number) => void | insertIndex | 4.6.0 |
| move   | 移动表单项 | (from: number, to: number) => void                 | -           |       |
| remove | 删除表单项 | (index: number \| number\[]) => void               | number\[]   | 4.5.0 |

## Form.ErrorList

4.7.0 新增。错误展示组件，仅限配合 Form.List 的 rules 一同使用。参考[示例](#form-demo-dynamic-form-item)。

| 参数   | 说明     | 类型         | 默认值 |
| ------ | -------- | ------------ | ------ |
| errors | 错误列表 | ReactNode\[] | -      |

## Form.Provider

提供表单间联动功能，其下设置 `name` 的 Form 更新时，会自动触发对应事件。查看[示例](#form-demo-form-context)。

| 参数 | 说明 | 类型 | 默认值 |
| --- | --- | --- | --- |
| onFormChange | 子表单字段更新时触发 | function(formName: string, info: { changedFields, forms }) | - |
| onFormFinish | 子表单提交时触发 | function(formName: string, info: { values, forms }) | - |

```jsx
<Form.Provider
  onFormFinish={(name) => {
    if (name === 'form1') {
      // Do something...
    }
  }}
>
  <Form name="form1">...</Form>
  <Form name="form2">...</Form>
</Form.Provider>
```

### FormInstance

| 名称 | 说明 | 类型 | 版本 |
| --- | --- | --- | --- |
| getFieldError | 获取对应字段名的错误信息 | (name: [NamePath](#namepath)) => string\[] | getFieldsError | 获取一组字段名对应的错误信息，返回为数组形式 | (nameList?: [NamePath](#namepath)\[]) => FieldError\[] | getFieldValue | 获取对应字段名的值 | (name: [NamePath](#namepath)) => any | isFieldTouched | 检查对应字段是否被用户操作过 | (name: [NamePath](#namepath)) => boolean | resetFields | 重置一组字段到 `initialValues` | (fields?: [NamePath](#namepath)\[]) => void | setFields | 设置一组字段状态 | (fields: [FieldData](#fielddata)\[]) => void | setFieldsValue | 设置表单的值（该值将直接传入 form store 中并且**重置错误信息**。如果你不希望传入对象被修改，请克隆后传入）。如果你只想修改 Form.List 中单项值，请通过 `setFieldValue` 进行指定 | (values) => void | validateFields | 触发表单验证，设置 `recursive` 时会递归校验所有包含的路径 | (nameList?: [NamePath](#namepath)\[], config?: [ValidateConfig](#validatefields)) => Promise 
```tsx
export interface ValidateConfig {
  // 5.5.0 新增。仅校验内容而不会将错误信息展示到 UI 上。
  validateOnly?: boolean;
  // 5.9.0 新增。对提供的 `nameList` 与其子路径进行递归校验。
  recursive?: boolean;
  // 5.11.0 新增。校验 dirty 的字段（touched + validated）。
  // 使用 `dirty` 可以很方便的仅校验用户操作过和被校验过的字段。
  dirty?: boolean;
}
```

返回示例：

```jsx
validateFields()
  .then((values) => {
    /*
  values:
    {
      username: 'username',
      password: 'password',
    }
  */
  })
  .catch((errorInfo) => {
    /*
    errorInfo:
      {
        values: {
          username: 'username',
          password: 'password',
        },
        errorFields: [
          { name: ['password'], errors: ['Please input your Password!'] },
        ],
        outOfDate: false,
      }
    */
  });
```

## Hooks

### Form.useForm

`type Form.useForm = (): [FormInstance]`

创建 Form 实例，用于管理所有数据状态。

### Form.useFormInstance

`type Form.useFormInstance = (): FormInstance`

`4.20.0` 新增，获取当前上下文正在使用的 Form 实例，常见于封装子组件消费无需透传 Form 实例：

```tsx
const Sub = () => {
  const form = Form.useFormInstance();

  return <Button onClick={() => form.setFieldsValue({})} />;
};

export default () => {
  const [form] = Form.useForm();

  return (
    <Form form={form}>
      <Sub />
    </Form>
  );
};
```

### Form.useWatch

`type Form.useWatch = (namePath: NamePath | (selector: (values: Store) => any), formInstance?: FormInstance | WatchOptions): Value`

`5.12.0` 新增 `selector`

用于直接获取 form 中字段对应的值。通过该 Hooks 可以与诸如 `useSWR` 进行联动从而降低维护成本：

```tsx
const Demo = () => {
  const [form] = Form.useForm();
  const userName = Form.useWatch('username', form);

  const { data: options } = useSWR(`/api/user/${userName}`, fetcher);

  return (
    <Form form={form}>
      <Form.Item name="username">
        <AutoComplete options={options} />
      </Form.Item>
    </Form>
  );
};
```

如果你的组件被包裹在 `Form.Item` 内部，你可以省略第二个参数，`Form.useWatch` 会自动找到上层最近的 `FormInstance`。

`useWatch` 默认只监听在 Form 中注册的字段，如果需要监听非注册字段，可以通过配置 `preserve` 进行监听：

```tsx
const Demo = () => {
  const [form] = Form.useForm();

  const age = Form.useWatch('age', { form, preserve: true });
  console.log(age);

  return (
    <div>
      <Button onClick={() => form.setFieldValue('age', 2)}>Update</Button>
      <Form form={form}>
        <Form.Item name="name">
          <Input />
        </Form.Item>
      </Form>
    </div>
  );
};
```

### Form.Item.useStatus

`type Form.Item.useStatus = (): { status: ValidateStatus | undefined, errors: ReactNode[], warnings: ReactNode[] }`

`4.22.0` 新增，可用于获取当前 Form.Item 的校验状态，如果上层没有 Form.Item，`status` 将会返回 `undefined`。`5.4.0` 新增 `errors` 和 `warnings`，可用于获取当前 Form.Item 的错误信息和警告信息：

```tsx
const CustomInput = ({ value, onChange }) => {
  const { status, errors } = Form.Item.useStatus();
  return (
    <input
      value={value}
      onChange={onChange}
      className={`custom-input-${status}`}
      placeholder={(errors.length && errors[0]) || ''}
    />
  );
};

export default () => (
  <Form>
    <Form.Item name="username">
      <CustomInput />
    </Form.Item>
  </Form>
);
```

#### 与其他获取数据的方式的区别

Form 仅会对变更的 Field 进行刷新，从而避免完整的组件刷新可能引发的性能问题。因而你无法在 render 阶段通过 `form.getFieldsValue` 来实时获取字段值，而 `useWatch` 提供了一种特定字段访问的方式，从而使得在当前组件中可以直接消费字段的值。同时，如果为了更好的渲染性能，你可以通过 Field 的 renderProps 仅更新需要更新的部分。而当当前组件更新或者 effect 都不需要消费字段值时，则可以通过 `onValuesChange` 将数据抛出，从而避免组件更新。

## Interface

### NamePath

`string | number | (string | number)[]`

### GetFieldsValue

`getFieldsValue` 提供了多种重载方法：

#### getFieldsValue(nameList?: true | [NamePath](#namepath)\[], filterFunc?: FilterFunc)

当不提供 `nameList` 时，返回所有注册字段，这也包含 List 下所有的值（即便 List 下没有绑定 Item）。

当 `nameList` 为 `true` 时，返回 store 中所有的值，包含未注册字段。例如通过 `setFieldsValue` 设置了不存在的 Item 的值，也可以通过 `true` 全部获取。

当 `nameList` 为数组时，返回规定路径的值。需要注意的是，`nameList` 为嵌套数组。例如你需要某路径值应该如下：

```tsx
// 单个路径
form.getFieldsValue([['user', 'age']]);

// 多个路径
form.getFieldsValue([
  ['user', 'age'],
  ['preset', 'account'],
]);
```

#### getFieldsValue({ filter?: FilterFunc })

### FilterFunc

用于过滤一些字段值，`meta` 会返回字段相关信息。例如可以用来获取仅被用户修改过的值等等。

```tsx
type FilterFunc = (meta: { touched: boolean; validating: boolean }) => boolean;
```

### FieldData

| 名称       | 说明             | 类型                     |
| ---------- | ---------------- | ------------------------ |
| errors     | 错误信息         | string\[]                |
| warnings   | 警告信息         | string\[]                |
| name       | 字段名称         | [NamePath](#namepath)\[] |
| touched    | 是否被用户操作过 | boolean                  |
| validating | 是否正在校验     | boolean                  |
| value      | 字段对应值       | any                      |

### Rule

Rule 支持接收 object 进行配置，也支持 function 来动态获取 form 的数据：

```tsx
type Rule = RuleConfig | ((form: FormInstance) => RuleConfig);
```

| 名称 | 说明 | 类型 | 版本 |
| --- | --- | --- | --- |
| defaultField | 仅在 `type` 为 `array` 类型时有效，用于指定数组元素的校验规则 | [rule](#rule) | fields | 仅在 `type` 为 `array` 或 `object` 类型时有效，用于指定子元素的校验规则 | Record&lt;string, [rule](#rule)> | max | 必须设置 `type`：string 类型为字符串最大长度；number 类型时为最大值；array 类型时为数组最大长度 | number | min | 必须设置 `type`：string 类型为字符串最小长度；number 类型时为最小值；array 类型时为数组最小长度 | number | required | 是否为必选字段 | boolean | type | 类型，常见有 `string` \| `number` \| `boolean` \| `url` \| `email` \| `tel`。更多请参考[此处](https://github.com/react-component/async-validator#type) | string | validator | 自定义校验，接收 Promise 作为返回值。[示例](#form-demo-register)参考 | ([rule](#rule), value) => Promise | whitespace | 如果字段仅包含空格则校验不通过，只在 `type: 'string'` 时生效 | boolean 
| 名称     | 说明                                  | 类型         | 默认值                 | 版本  |
| -------- | ------------------------------------- | ------------ | ---------------------- | ----- |
| form     | 指定 Form 实例                        | FormInstance | 当前 context 中的 Form | 5.4.0 |
| preserve | 是否监视没有对应的 `Form.Item` 的字段 | boolean      | false                  | 5.4.0 |

### 导入方式

```js
import { Form } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `colon` | 配置 Form.Item 的 `colon` 的默认值。表示是否显示 label 后面的冒号 (只有在属性 layout 为 horizontal 时有效) | boolean | true | — |
| `disabled` | 设置表单组件禁用，仅对 antd 组件有效 | boolean | false | 4.21.0 |
| `component` | 设置 Form 渲染元素，为 `false` 则不创建 DOM 节点 | ComponentType \| false | form | — |
| `fields` | 通过状态管理（如 redux）控制表单字段，如非强需求不推荐使用。查看[示例](#form-demo-global-state) | [FieldData](#fielddata)\[] | - | — |
| `form` | 经 `Form.useForm()` 创建的 form 控制实例，不提供时会自动创建 | [FormInstance](#forminstance) | - | — |
| `feedbackIcons` | 当 `Form.Item` 有 `hasFeedback` 属性时可以自定义图标 | [FeedbackIcons](#feedbackicons) | - | 5.9.0 |
| `initialValues` | 表单默认值，只有初始化以及重置时生效 | object | - | — |
| `labelAlign` | label 标签的文本对齐方式 | `left` \| `right` | `right` | — |
| `labelWrap` | label 标签的文本换行方式 | boolean | false | 4.18.0 |
| `labelCol` | label 标签布局，同 `` 组件，设置 `span` `offset` 值，如 `{span: 3, offset: 12}` 或 `sm: {span: 3, offset: 12}` | [object](/components/grid-cn#col) | - | — |
| `layout` | 表单布局 | `horizontal` \| `vertical` \| `inline` | `horizontal` | — |
| `name` | 表单名称，会作为表单字段 `id` 前缀使用 | string | - | — |
| `preserve` | 当字段被删除时保留字段值。你可以通过 `getFieldsValue(true)` 来获取保留字段值 | boolean | true | 4.4.0 |
| `requiredMark` | 必选样式，可以切换为必选或者可选展示样式。此为 Form 配置，Form.Item 无法单独配置 | boolean \| `optional` \| ((label: ReactNode, info: { required: boolean }) => ReactNode) | true | `renderProps`: 5.9.0 |
| `scrollToFirstError` | 提交失败自动滚动到第一个错误字段 | boolean \| [Options](https://github.com/stipsan/scroll-into-view-if-needed/tree/ece40bd9143f48caf4b99503425ecb16b0ad8249#options) \| { focus: boolean } | false | focus: 5.24.0 |
| `size` | 设置字段组件的尺寸（仅限 antd 组件） | `small` \| `medium` \| `large` | - | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `tooltip` | 配置提示属性 | [TooltipProps](/components/tooltip-cn#api) & { icon?: ReactNode } | - | 6.3.0 |
| `validateMessages` | 验证提示模板，说明[见下](#validatemessages) | [ValidateMessages](https://github.com/ant-design/ant-design/blob/6234509d18bac1ac60fbb3f92a5b2c6a6361295a/components/locale/en_US.ts#L88-L134) | - | — |
| `validateTrigger` | 统一设置字段触发验证的时机 | string \| string\[] | `onChange` | 4.3.0 |
| `variant` | 表单内控件变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 |
| `wrapperCol` | 需要为输入控件设置布局样式时，使用该属性，用法同 labelCol | [object](/components/grid-cn#col) | - | — |
| `onFieldsChange` | 字段更新时触发回调事件 | function(changedFields, allFields) | - | — |
| `onFinish` | 提交表单且数据验证成功后回调事件 | function(values) | - | — |
| `onFinishFailed` | 提交表单且数据验证失败后回调事件 | function({ values, errorFields, outOfDate }) | - | — |
| `onValuesChange` | 字段值更新时触发回调事件 | function(changedValues, allValues) | - | — |
| `clearOnDestroy` | 当表单被卸载时清空表单值 | boolean | false | 5.18.0 |
| `dependencies` | 设置依赖字段，说明[见下](#dependencies) | [NamePath](#namepath)\[] | - | — |
| `extra` | 额外的提示信息，和 `help` 类似，当需要错误信息和提示文案同时出现时，可以使用这个。 | ReactNode | - | — |
| `getValueFromEvent` | 设置如何将 event 的值转换成字段值 | (..args: any\[]) => any | - | — |
| `getValueProps` | 为子元素添加额外的属性 (不建议通过 `getValueProps` 生成动态函数 prop，请直接将其传递给子组件) | (value: any) => Record | - | 4.2.0 |
| `hasFeedback` | 配合 `validateStatus` 属性使用，展示校验状态图标，建议只配合 Input 组件使用 此外，它还可以通过 Icons 属性获取反馈图标。 | boolean \| { icons: [FeedbackIcons](#feedbackicons) } | false | icons: 5.9.0 |
| `help` | 提示信息，如不设置，则会根据校验规则自动生成 | ReactNode | - | — |
| `hidden` | 是否隐藏字段（依然会收集和校验字段） | boolean | false | 4.4.0 |
| `htmlFor` | 设置子元素 label `htmlFor` 属性 | string | - | — |
| `initialValue` | 设置子元素默认值，如果与 Form 的 `initialValues` 冲突则以 Form 为准 | string | - | 4.2.0 |
| `label` | `label` 标签的文本，当不需要 label 又需要与冒号对齐，可以设为 null | ReactNode | - | null: 5.22.0 |
| `messageVariables` | 默认验证字段的信息，查看[详情](#messagevariables) | Record<string, string> | - | 4.7.0 |
| `normalize` | 组件获取值后进行转换，再放入 Form 中。不支持异步 | (value, prevValue, prevValues) => any | - | — |
| `noStyle` | 为 `true` 时不带样式，作为纯字段控件使用。当自身没有 `validateStatus` 而父元素存在有 `validateStatus` 的 Form.Item 会继承父元素的 `validateStatus` | boolean | false | — |
| `required` | 必填样式设置。如不设置，则会根据校验规则自动生成 | boolean | false | — |
| `rules` | 校验规则，设置字段的校验逻辑。点击[此处](#form-demo-basic)查看示例 | [Rule](#rule)\[] | - | — |
| `shouldUpdate` | 自定义字段更新逻辑，说明[见下](#shouldupdate) | boolean \| (prevValue, curValue) => boolean | false | — |
| `trigger` | 设置收集字段值变更的时机。点击[此处](#form-demo-customized-form-controls)查看示例 | string | `onChange` | — |
| `validateFirst` | 当某一规则校验不通过时，是否停止剩下的规则的校验。设置 `parallel` 时会并行校验 | boolean \| `parallel` | false | `parallel`: 4.5.0 |
| `validateDebounce` | 设置防抖，延迟毫秒数后进行校验 | number | - | 5.9.0 |
| `validateStatus` | 校验状态，如不设置，则会根据校验规则自动生成，可选：'success' 'warning' 'error' 'validating' | string | - | — |
| `valuePropName` | 子节点的值的属性。注意：Switch、Checkbox 的 valuePropName 应该是 `checked`，否则无法获取这个两个组件的值。该属性为 `getValueProps` 的封装，自定义 `getValueProps` 后会失效 | string | `value` | — |
| `children` | 渲染函数 | (fields: Field\[], operation: { add, remove, move }, meta: { errors }) => React.ReactNode | - | — |
| `add` | 新增表单项 | (defaultValue?: any, insertIndex?: number) => void | insertIndex | 4.6.0 |
| `move` | 移动表单项 | (from: number, to: number) => void | - | — |
| `remove` | 删除表单项 | (index: number \| number\[]) => void | number\[] | 4.5.0 |
| `errors` | 错误列表 | ReactNode\[] | - | — |
| `onFormChange` | 子表单字段更新时触发 | function(formName: string, info: { changedFields, forms }) | - | — |
| `onFormFinish` | 子表单提交时触发 | function(formName: string, info: { values, forms }) | - | — |
| `getFieldError` | 获取对应字段名的错误信息 | (name: [NamePath](#namepath)) => string\[] | — | — |
| `getFieldInstance` | 获取对应字段实例 | (name: [NamePath](#namepath)) => any | — | 4.4.0 |
| `getFieldsError` | 获取一组字段名对应的错误信息，返回为数组形式 | (nameList?: [NamePath](#namepath)\[]) => FieldError\[] | — | — |
| `getFieldsValue` | 获取一组字段名对应的值，会按照对应结构返回。默认返回现存字段值，当调用 `getFieldsValue(true)` 时返回所有值 | [GetFieldsValue](#getfieldsvalue) | — | — |
| `getFieldValue` | 获取对应字段名的值 | (name: [NamePath](#namepath)) => any | — | — |
| `isFieldsTouched` | 检查一组字段是否被用户操作过，`allTouched` 为 `true` 时检查是否所有字段都被操作过 | (nameList?: [NamePath](#namepath)\[], allTouched?: boolean) => boolean | — | — |
| `isFieldTouched` | 检查对应字段是否被用户操作过 | (name: [NamePath](#namepath)) => boolean | — | — |
| `isFieldValidating` | 检查对应字段是否正在校验 | (name: [NamePath](#namepath)) => boolean | — | — |
| `resetFields` | 重置一组字段到 `initialValues` | (fields?: [NamePath](#namepath)\[]) => void | — | — |
| `scrollToField` | 滚动到对应字段位置 | (name: [NamePath](#namepath), options: [ScrollOptions](https://github.com/stipsan/scroll-into-view-if-needed/tree/ece40bd9143f48caf4b99503425ecb16b0ad8249#options) \| { focus: boolean }) => void | — | focus: 5.24.0 |
| `setFields` | 设置一组字段状态 | (fields: [FieldData](#fielddata)\[]) => void | — | — |
| `setFieldValue` | 设置表单的值（该值将直接传入 form store 中并且**重置错误信息**。如果你不希望传入对象被修改，请克隆后传入） | (name: [NamePath](#namepath), value: any) => void | — | 4.22.0 |
| `setFieldsValue` | 设置表单的值（该值将直接传入 form store 中并且**重置错误信息**。如果你不希望传入对象被修改，请克隆后传入）。如果你只想修改 Form.List 中单项值，请通过 `setFieldValue` 进行指定 | (values) => void | — | — |
| `submit` | 提交表单，与点击 `submit` 按钮效果相同 | () => void | — | — |
| `validateFields` | 触发表单验证，设置 `recursive` 时会递归校验所有包含的路径 | (nameList?: [NamePath](#namepath)\[], config?: [ValidateConfig](#validatefields)) => Promise | — | — |
| `warnings` | 警告信息 | string\[] | — | — |
| `touched` | 是否被用户操作过 | boolean | — | — |
| `validating` | 是否正在校验 | boolean | — | — |
| `value` | 字段对应值 | any | — | — |
| `defaultField` | 仅在 `type` 为 `array` 类型时有效，用于指定数组元素的校验规则 | [rule](#rule) | — | — |
| `enum` | 是否匹配枚举中的值（需要将 `type` 设置为 `enum`） | any\[] | — | — |
| `len` | string 类型时为字符串长度；number 类型时为确定数字； array 类型时为数组长度 | number | — | — |
| `max` | 必须设置 `type`：string 类型为字符串最大长度；number 类型时为最大值；array 类型时为数组最大长度 | number | — | — |
| `message` | 错误信息，不设置时会通过[模板](#validatemessages)自动生成 | string \| ReactElement | — | — |
| `min` | 必须设置 `type`：string 类型为字符串最小长度；number 类型时为最小值；array 类型时为数组最小长度 | number | — | — |
| `pattern` | 正则表达式匹配 | RegExp | — | — |
| `transform` | 将字段值转换成目标值后进行校验 | (value) => any | — | — |
| `type` | 类型，常见有 `string` \| `number` \| `boolean` \| `url` \| `email` \| `tel`。更多请参考[此处](https://github.com/react-component/async-validator#type) | string | — | — |
| `validator` | 自定义校验，接收 Promise 作为返回值。[示例](#form-demo-register)参考 | ([rule](#rule), value) => Promise | — | — |
| `warningOnly` | 仅警告，不阻塞表单提交 | boolean | — | 4.17.0 |
| `whitespace` | 如果字段仅包含空格则校验不通过，只在 `type: 'string'` 时生效 | boolean | — | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Form** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **37** 个，均需可复现。
12. **表单专项**：rules、dependencies、scrollToFirstError。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/form
- 中文文档：https://ant.design/components/form-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/form
- 驱动 gpui kit：`form`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Form** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/form/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Form）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 受控输入/选择、弹层、清除、校验 status、尺寸档 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Form）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：高性能表单控件，自带数据域管理。包含数据录入、校验以及对应样式。

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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据录入**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `colon` | 配置 Form.Item 的 `colon` 的默认值。表示是否显示 label 后面的冒号 (只有在属性 lay… | boolean | true |
| `disabled` | 设置表单组件禁用，仅对 antd 组件有效 | boolean | false |
| `component` | 设置 Form 渲染元素，为 `false` 则不创建 DOM 节点 | ComponentType \ | false |
| `fields` | 通过状态管理（如 redux）控制表单字段，如非强需求不推荐使用。查看[示例](#form-demo-global… | [FieldData](#fielddata)\[] | - |
| `form` | 经 `Form.useForm()` 创建的 form 控制实例，不提供时会自动创建 | [FormInstance](#forminstance) | - |
| `feedbackIcons` | 当 `Form.Item` 有 `hasFeedback` 属性时可以自定义图标 | [FeedbackIcons](#feedbackicons) | - |
| `initialValues` | 表单默认值，只有初始化以及重置时生效 | object | - |
| `labelAlign` | label 标签的文本对齐方式 | `left` \ | `right` |
| `labelWrap` | label 标签的文本换行方式 | boolean | false |
| `labelCol` | label 标签布局，同 `<Col>` 组件，设置 `span` `offset` 值，如 `{span: 3,… | [object](/components/grid-cn#col) | - |
| `layout` | 表单布局 | `horizontal` \ | `vertical` \ |
| `name` | 表单名称，会作为表单字段 `id` 前缀使用 | string | - |
| `preserve` | 当字段被删除时保留字段值。你可以通过 `getFieldsValue(true)` 来获取保留字段值 | boolean | true |
| `requiredMark` | 必选样式，可以切换为必选或者可选展示样式。此为 Form 配置，Form.Item 无法单独配置 | boolean \ | `optional` \ |
| `scrollToFirstError` | 提交失败自动滚动到第一个错误字段 | boolean \ | [Options](https://github.com/stipsan/scroll-into-view-if-needed/tree/ece40bd9143f48caf4b99503425ecb16b0ad8249#options) \ |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
mount ──► initialValues 写入字段
             │
             ├── 字段编辑 ──► 内部 store + 子控件显示
             ├── submit ──► validateFields
             │                 ├── 失败 ──► 字段 error + onFinishFailed
             │                 └── 成功 ──► onFinish(values)
             ├── setFieldsValue ──► 更新显示
             ├── resetFields ──► 回 initial + 清 error
             ├── dependencies 变更 ──► 依赖字段再校验
             └── Form.List add/remove ──► 数组字段增删
```

\*与子控件桥接：Input 用 value/onChange；Switch/Checkbox 用 valuePropName=checked。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| FRM-S1 | required 字段为空提交 | `onFinishFailed`；字段展示错误 |
| FRM-S2 | 字段合法后提交 | `onFinish` 收到完整 values 一次 |
| FRM-S3 | `setFieldsValue` | 对应控件显示更新 |
| FRM-S4 | `resetFields` | 回到 initialValues；错误消失 |
| FRM-S5 | `disabled` Form | 子控件不可编辑 |
| FRM-S6 | dependencies 触发 | 关联字段变更后规则重跑 |
| FRM-S7 | List add | 多一项；name 路径可提交 |
| FRM-S8 | List remove | 项减少 |
| FRM-S9 | `layout=horizontal` | label 与控件水平排布 |
| FRM-S10 | 自定义 validator 失败 | 展示 message |
| FRM-S11 | validateTrigger=onBlur（若设） | 失焦才校验 |
| FRM-S12 | 嵌套 name=['a','b'] | values 嵌套结构正确 |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 / 变体 | 规则 |
| --- | --- |
| default | 容器底 + 边框（outlined）或族默认皮；Token 色 |
| hover | 边框/底强调 |
| focus | **可见** focus ring；主色边 |
| disabled | 降对比；不可编辑 |
| status=error/warning | 语义色边框/反馈 |
| 弹层 open | elevation 阴影；与触发器对齐 placement |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | textbox / combobox / spinbutton / listbox 等 |
| 标签 | 与 Form.Item label 或 aria-labelledby 关联 |
| 清除/下拉 | 控件有可访问名称 |
| 错误 | status=error 时暴露 invalid |
| 键盘 | 主路径可选/提交/关闭 |

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
| `disabled` | 必须 |
| `size` | 必须 |
| `type` | 必须 |
| `variant` | 必须 |
| `children` | 必须 |
| `trigger` | 必须 |
| 官方主路径示例 | 基本使用、表单方法调用、表单布局、表单混合布局、表单禁用、表单变体、必选样式、表单尺寸 |
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
| 其余示例 | 表单标签可换行, 非阻塞校验, 字段监听 Hooks, 校验时机 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestForm_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Form 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| FRM-01 | L1 | NewForm 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| FRM-02 | L1 | required 字段为空提交 | `onFinishFailed`；字段展示错误 |
| FRM-03 | L1 | 字段合法后提交 | `onFinish` 收到完整 values 一次 |
| FRM-04 | L1 | `setFieldsValue` | 对应控件显示更新 |
| FRM-05 | L1 | `resetFields` | 回到 initialValues；错误消失 |
| FRM-06 | L1 | `disabled` Form | 子控件不可编辑 |
| FRM-07 | L1 | dependencies 触发 | 关联字段变更后规则重跑 |
| FRM-08 | L1 | List add | 多一项；name 路径可提交 |
| FRM-09 | L1 | List remove | 项减少 |
| FRM-10 | L1 | `layout=horizontal` | label 与控件水平排布 |
| FRM-11 | L1 | 自定义 validator 失败 | 展示 message |
| FRM-12 | L1 | validateTrigger=onBlur（若设） | 失焦才校验 |
| FRM-13 | L1 | 嵌套 name=['a','b'] | values 嵌套结构正确 |
| FRM-14 | L1 | 复现官方示例「基本使用」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| FRM-15 | L1 | 复现官方示例「表单方法调用」（`control-hooks.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| FRM-16 | L1 | 复现官方示例「表单布局」（`layout.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| FRM-17 | L1 | 复现官方示例「表单混合布局」（`layout-multiple.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| FRM-18 | L1 | 复现官方示例「表单禁用」（`disabled.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| FRM-19 | L1 | 复现官方示例「表单变体」（`variant.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| FRM-20 | L1 | 复现官方示例「必选样式」（`required-mark.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| FRM-21 | L1 | 复现官方示例「表单尺寸」（`size.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| FRM-22 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| FRM-23 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| FRM-24 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| FRM-25 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| FRM-26 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| FRM-27 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| FRM-28 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewForm(...) *Form

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
Field / Selector
  ├─ prefix?
  ├─ editable / display value
  ├─ clear? / suffix?
  └─ Portal popup? (list/panel)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Form 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/form.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Form 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
