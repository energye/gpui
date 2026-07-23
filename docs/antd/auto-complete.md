# AutoComplete 自动完成
> 来源：[Ant Design 6.5.x AutoComplete](https://ant.design/components/auto-complete)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：输入框自动完成功能。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

输入框自动完成功能。

**AutoComplete** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本使用 | 复现「基本使用」视觉与布局 |
| 自定义选项 | 自定义渲染/插槽外观 |
| 自定义输入组件 | 自定义渲染/插槽外观 |
| 不区分大小写 | 不同 size 档位 |
| 查询模式 - 确定类目 | 复现「查询模式 - 确定类目」视觉与布局 |
| 查询模式 - 不确定类目 | 复现「查询模式 - 不确定类目」视觉与布局 |
| 自定义状态 | 自定义渲染/插槽外观 |
| 多种形态 | variant 外观 |
| 自定义清除按钮 | 自定义渲染/插槽外观 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `allowClear`

- **说明**：支持清除
- **类型**：boolean | { clearIcon?: ReactNode }
- **默认值**：false
- **版本**：5.8.0: 支持对象形式

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `disabled`

- **说明**：是否禁用
- **类型**：boolean
- **默认值**：false

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

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `variant`

- **说明**：形态变体
- **类型**：`outlined` | `borderless` | `filled` | `underlined`
- **默认值**：`outlined`
- **版本**：5.13.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `outlined` | 描边空心 |
  | `borderless` | 无边框 |
  | `filled` | 浅底填充 |
  | `underlined` | 底边线形态 |

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

- 需要一个输入框而不是选择器。
- 需要输入建议/辅助提示。

和 Select 的区别是：

- AutoComplete 是一个带提示的文本输入框，用户可以自由输入，关键词是辅助**输入**。
- Select 是在限定的可选项中进行选择，关键词是**选择**。

### 2.2 核心功能（按官方示例拆解）

1. **基本使用**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **自定义选项**（`options.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **自定义输入组件**（`custom.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **不区分大小写**（`non-case-sensitive.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **查询模式 - 确定类目**（`certain-category.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **查询模式 - 不确定类目**（`uncertain-category.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **自定义状态**（`status.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **多种形态**（`variant.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **自定义清除按钮**（`allowClear.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 指定当前选中的条目 |
| `defaultValue` | 非受控默认值 | 指定默认选中的条目 |
| `onChange` | 值变化 | 选中 option，或 input 的 value 变化时，调用此函数 |
| `onSelect` | 选中 | 被选中时调用，参数为选中项的 value 值 |
| `open` | 受控显隐 | 是否展开下拉菜单 |
| `onOpenChange` | 显隐变化 | 展开下拉菜单的回调 |
| `disabled` | 禁用 | 是否禁用 |
| `options` | 数据化 options | 数据化配置选项内容，相比 jsx 定义会获得更好的渲染性能 |
| `dataSource` | 数据源 | 自动完成的数据源，请使用 `options` 替代 |
| `showSearch` | 搜索 | 搜索配置 |
| `filterOption` | 过滤 | 是否根据输入项进行筛选。当其为一个函数时，会接收 `inputValue` `option` 两个参数，当 `option` 符合筛选条件时，应返回 true，反之则返回 false |
| `virtual` | 虚拟滚动 | 设置 false 时关闭虚拟滚动 |
| `getPopupContainer` | 浮层容器 | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例](https://codesandbox.io/s/4j168r7jw0) |
| `onSearch` | 搜索回调 | 搜索补全项的时候调用 |
| `onClear` | 清除 | 清除内容时的回调 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本使用 | `basic.tsx` | 否 |
| 自定义选项 | `options.tsx` | 否 |
| 自定义输入组件 | `custom.tsx` | 否 |
| 不区分大小写 | `non-case-sensitive.tsx` | 否 |
| 查询模式 - 确定类目 | `certain-category.tsx` | 否 |
| 查询模式 - 不确定类目 | `uncertain-category.tsx` | 否 |
| 自定义状态 | `status.tsx` | 否 |
| 多种形态 | `variant.tsx` | 否 |
| 自定义清除按钮 | `allowClear.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 禁用自定义输入 Debug | `disabled-custom-debug.tsx` | 是 |
| 填充形态自定义输入 Debug | `filled-custom-debug.tsx` | 是 |
| 在 Form 中 Debug | `form-debug.tsx` | 是 |
| AutoComplete 和 Select | `AutoComplete-and-Select.tsx` | 是 |
| \_InternalPanelDoNotUseOrYouWillBeFired | `render-panel.tsx` | 是 |

### 2.5 实例方法 / Ref

#### 方法 {#methods}

| 名称    | 描述     | 版本 |
| ------- | -------- | ---- |
| blur()  | 移除焦点 |      |
| focus() | 获取焦点 |      |

### 2.6 FAQ

## FAQ

### 为何受控状态下使用 onSearch 无法输入中文？ {#faq-controlled-onsearch-composition}

请使用 `onChange` 进行受控管理。`onSearch` 触发于搜索输入，与 `onChange` 时机不同。此外，点击选项时也不会触发 `onSearch` 事件。

相关 issue：[#18230](https://github.com/ant-design/ant-design/issues/18230) [#17916](https://github.com/ant-design/ant-design/issues/17916)

### 为何 options 为空时，受控 open 展开不会显示下拉菜单？ {#faq-empty-options-controlled-open}

AutoComplete 组件本质上是 Input 输入框的一种扩展，当 `options` 为空时，显示空文本会让用户误以为该组件不可操作，实际上它仍然可以进行文本输入操作。因此，为了避免给用户带来困惑，当 `options` 为空时，`open` 属性为 `true` 也不会展示下拉菜单，需要与 `options` 属性配合使用。

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

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| allowClear | 支持清除 | boolean \| { clearIcon?: ReactNode } | false | 5.8.0: 支持对象形式 |
| backfill | 使用键盘选择选项的时候把选中项回填到输入框中 | boolean | false | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | defaultActiveFirstOption | 是否默认高亮第一个选项 | boolean | true | defaultValue | 指定默认选中的条目 | string | - | ~~dropdownClassName~~ | 下拉菜单的 className 属性，请使用 `classNames.popup.root` 替代 | string | - | - |
| ~~dropdownMatchSelectWidth~~ | 下拉菜单和输入框是否同宽，请使用 `popupMatchSelectWidth` 替代 | boolean \| number | true | - |
| ~~dropdownRender~~ | 自定义下拉框内容，使用 `popupRender` 替换 | (originNode: ReactNode) => ReactNode | - | 4.24.0 |
| popupRender | 自定义下拉框内容 | (originNode: ReactNode) => ReactNode | - | ~~dropdownStyle~~ | 下拉菜单的 style 属性，使用 `styles.popup.root` 替换 | CSSProperties | - | ~~filterOption~~ | 是否根据输入项进行筛选。当其为一个函数时，会接收 `inputValue` `option` 两个参数，当 `option` 符合筛选条件时，应返回 true，反之则返回 false | boolean \| function(inputValue, option) | true | notFoundContent | 当下拉列表为空时显示的内容 | ReactNode | - | options | 数据化配置选项内容，相比 jsx 定义会获得更好的渲染性能 | { label, value }\[] | - | showSearch | 搜索配置 | true \| [Object](#showsearch) | true | size | 控件大小 | `large` \| `medium` \| `small` | - | value | 指定当前选中的条目 | string | - | virtual | 设置 false 时关闭虚拟滚动 | boolean | true | 4.1.0 |
| onBlur | 失去焦点时的回调 | function() | - | ~~onDropdownVisibleChange~~ | 展开下拉菜单的回调，使用 `onOpenChange` 替换 | (open: boolean) => void | - | onFocus | 获得焦点时的回调 | function() | - | onSelect | 被选中时调用，参数为选中项的 value 值 | function(value, option) | - | onInputKeyDown | 按键按下时回调 | (event: KeyboardEvent) => void | - 
### showSearch

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| filterOption | 是否根据输入项进行筛选。当其为一个函数时，会接收 `inputValue` `option` 两个参数，当 `option` 符合筛选条件时，应返回 true，反之则返回 false | boolean \| function(inputValue, option) | true 
## 方法 {#methods}

| 名称    | 描述     | 版本 |
| ------- | -------- | ---- |
| blur()  | 移除焦点 |      |
| focus() | 获取焦点 |      |

### 导入方式

```js
import { AutoComplete } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `allowClear` | 支持清除 | boolean \| { clearIcon?: ReactNode } | false | 5.8.0: 支持对象形式 |
| `backfill` | 使用键盘选择选项的时候把选中项回填到输入框中 | boolean | false | — |
| `children` | 自定义输入框 | HTMLInputElement \| HTMLTextAreaElement \| React.ReactElement<InputProps> | <Input /> | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `dataSource` | 自动完成的数据源，请使用 `options` 替代 | DataSourceItemType[] | - | - |
| `defaultActiveFirstOption` | 是否默认高亮第一个选项 | boolean | true | — |
| `defaultOpen` | 是否默认展开下拉菜单 | boolean | - | — |
| `defaultValue` | 指定默认选中的条目 | string | - | — |
| `disabled` | 是否禁用 | boolean | false | — |
| `dropdownClassName` | 下拉菜单的 className 属性，请使用 `classNames.popup.root` 替代 | string | - | - |
| `dropdownMatchSelectWidth` | 下拉菜单和输入框是否同宽，请使用 `popupMatchSelectWidth` 替代 | boolean \| number | true | - |
| `dropdownRender` | 自定义下拉框内容，使用 `popupRender` 替换 | (originNode: ReactNode) => ReactNode | - | 4.24.0 |
| `popupRender` | 自定义下拉框内容 | (originNode: ReactNode) => ReactNode | - | — |
| `popupClassName` | 下拉菜单的 className 属性，使用 `classNames.popup.root` 替换 | string | - | 4.23.0 |
| `dropdownStyle` | 下拉菜单的 style 属性，使用 `styles.popup.root` 替换 | CSSProperties | - | — |
| `popupMatchSelectWidth` | 下拉菜单和选择器同宽。默认将设置 `min-width`，当值小于选择框宽度时会被忽略。false 时会关闭虚拟滚动 | boolean \| number | true | — |
| `filterOption` | 是否根据输入项进行筛选。当其为一个函数时，会接收 `inputValue` `option` 两个参数，当 `option` 符合筛选条件时，应返回 true，反之则返回 false | boolean \| function(inputValue, option) | true | — |
| `getPopupContainer` | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例](https://codesandbox.io/s/4j168r7jw0) | function(triggerNode) | () => document.body | — |
| `notFoundContent` | 当下拉列表为空时显示的内容 | ReactNode | - | — |
| `open` | 是否展开下拉菜单 | boolean | - | — |
| `options` | 数据化配置选项内容，相比 jsx 定义会获得更好的渲染性能 | { label, value }\[] | - | — |
| `placeholder` | 输入框提示 | string | - | — |
| `showSearch` | 搜索配置 | true \| [Object](#showsearch) | true | — |
| `status` | 设置校验状态 | 'error' \| 'warning' | - | 4.19.0 |
| `size` | 控件大小 | `large` \| `medium` \| `small` | - | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `value` | 指定当前选中的条目 | string | - | — |
| `variant` | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 |
| `virtual` | 设置 false 时关闭虚拟滚动 | boolean | true | 4.1.0 |
| `onBlur` | 失去焦点时的回调 | function() | - | — |
| `onChange` | 选中 option，或 input 的 value 变化时，调用此函数 | function(value) | - | — |
| `onDropdownVisibleChange` | 展开下拉菜单的回调，使用 `onOpenChange` 替换 | (open: boolean) => void | - | — |
| `onOpenChange` | 展开下拉菜单的回调 | (open: boolean) => void | - | — |
| `onFocus` | 获得焦点时的回调 | function() | - | — |
| `onSearch` | 搜索补全项的时候调用 | function(value) | - | — |
| `onSelect` | 被选中时调用，参数为选中项的 value 值 | function(value, option) | - | — |
| `onClear` | 清除内容时的回调 | function | - | 4.6.0 |
| `onInputKeyDown` | 按键按下时回调 | (event: KeyboardEvent) => void | - | — |
| `onPopupScroll` | 下拉列表滚动时的回调 | (event: UIEvent) => void | - | — |
| `blur()` | 移除焦点 | — | — | — |
| `focus()` | 获取焦点 | — | — | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **AutoComplete** 的验收清单：

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
- 官方文档：https://ant.design/components/auto-complete
- 中文文档：https://ant.design/components/auto-complete-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/auto-complete
- 驱动 gpui kit：`auto-complete`
