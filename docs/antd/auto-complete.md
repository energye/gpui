# AutoComplete 自动完成
> 来源：[Ant Design 6.5.x AutoComplete](https://ant.design/components/auto-complete)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：输入框自动完成功能。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
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

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

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

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **AutoComplete** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/auto-complete/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（AutoComplete）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 受控输入/选择、弹层、清除、校验 status、尺寸档 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（AutoComplete）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：输入框自动完成功能。

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
| `allowClear` | 支持清除 | boolean \ | { clearIcon?: ReactNode } |
| `backfill` | 使用键盘选择选项的时候把选中项回填到输入框中 | boolean | false |
| `children` | 自定义输入框 | HTMLInputElement \ | HTMLTextAreaElement \ |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `defaultActiveFirstOption` | 是否默认高亮第一个选项 | boolean | true |
| `defaultOpen` | 是否默认展开下拉菜单 | boolean | - |
| `defaultValue` | 指定默认选中的条目 | string | - |
| `disabled` | 是否禁用 | boolean | false |
| `popupRender` | 自定义下拉框内容 | (originNode: ReactNode) => ReactNode | - |
| `popupMatchSelectWidth` | 下拉菜单和选择器同宽。默认将设置 `min-width`，当值小于选择框宽度时会被忽略。false 时会关闭虚拟滚动 | boolean \ | number |
| `getPopupContainer` | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例]… | function(triggerNode) | () => document.body |
| `notFoundContent` | 当下拉列表为空时显示的内容 | ReactNode | - |
| `open` | 是否展开下拉菜单 | boolean | - |
| `options` | 数据化配置选项内容，相比 jsx 定义会获得更好的渲染性能 | { label, value }\[] | - |
| `placeholder` | 输入框提示 | string | - |
| `showSearch` | 搜索配置 | true \ | [Object](#showsearch) |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
输入 ──► onSearch ──► 过滤 options 开列表
选中 ──► 回填 + onSelect/onChange
allowClear 清空
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| AC-S1 | 输入触发 onSearch | 回调 |
| AC-S2 | 选建议 | 回填 value |
| AC-S3 | clear | 空 |
| AC-S4 | 无匹配 | 空列表/notFound |
| AC-S5 | 键盘选中 | Enter 选 |
| AC-S6 | disabled | 不交互 |
| AC-S7 | 受控 value | 外部 |
| AC-S8 | 高度 | 32 middle |
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
| `value` | 必须 |
| `defaultValue` | 必须 |
| `onChange` | 必须 |
| `disabled` | 必须 |
| `size` | 必须 |
| `variant` | 必须 |
| `status` | 必须 |
| `open` | 必须 |
| `onOpenChange` | 必须 |
| `options` | 必须 |
| `children` | 必须 |
| `allowClear` | 必须 |
| `showSearch` | 必须 |
| 官方主路径示例 | 基本使用、自定义选项、自定义输入组件、不区分大小写、查询模式 - 确定类目、查询模式 - 不确定类目、自定义状态、多种形态 |
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
| 其余示例 | 自定义清除按钮, 自定义语义结构的样式和类, _semantic.tsx |

### 6.9 验收用例表（可测）

> 测试名建议：`TestAutoComplete_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 AutoComplete 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| AC-01 | L1 | NewAutoComplete 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| AC-02 | L1 | 输入触发 onSearch | 回调 |
| AC-03 | L1 | 选建议 | 回填 value |
| AC-04 | L1 | clear | 空 |
| AC-05 | L1 | 无匹配 | 空列表/notFound |
| AC-06 | L1 | 键盘选中 | Enter 选 |
| AC-07 | L1 | disabled | 不交互 |
| AC-08 | L1 | 受控 value | 外部 |
| AC-09 | L1 | 高度 | 32 middle |
| AC-10 | L1 | 复现官方示例「基本使用」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| AC-11 | L1 | 复现官方示例「自定义选项」（`options.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| AC-12 | L1 | 复现官方示例「自定义输入组件」（`custom.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| AC-13 | L1 | 复现官方示例「不区分大小写」（`non-case-sensitive.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| AC-14 | L1 | 复现官方示例「查询模式 - 确定类目」（`certain-category.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| AC-15 | L1 | 复现官方示例「查询模式 - 不确定类目」（`uncertain-category.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| AC-16 | L1 | 复现官方示例「自定义状态」（`status.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| AC-17 | L1 | 复现官方示例「多种形态」（`variant.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| AC-18 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| AC-19 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| AC-20 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| AC-21 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| AC-22 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| AC-23 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| AC-24 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewAutoComplete(...) *AutoComplete

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

同时满足即可宣布 **AutoComplete 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. gallery 展示主路径（对照官方非 debug 示例与 P0）。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/auto-complete.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` AutoComplete 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
