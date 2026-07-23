# Cascader 级联选择
> 来源：[Ant Design 6.5.x Cascader](https://ant.design/components/cascader)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：级联选择框。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

级联选择框。

**Cascader** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 默认值 | 复现「默认值」视觉与布局 |
| 可以自定义显示 | 自定义渲染/插槽外观 |
| 移入展开 | 展开/折叠指示 |
| 禁用选项 | disabled 灰态与不可点 |
| 选择即改变 | 复现「选择即改变」视觉与布局 |
| 多选 | 多选标签/勾选外观 |
| 自定义回填方式 | 自定义渲染/插槽外观 |
| 大小 | 不同 size 档位 |
| 自定义已选项 | 自定义渲染/插槽外观 |
| 搜索 | 带搜索框外观 |
| 动态加载选项 | loading 指示与防重复 |
| 自定义字段名 | 自定义渲染/插槽外观 |
| 前后缀 | 复现「前后缀」视觉与布局 |
| 扩展菜单 | 复现「扩展菜单」视觉与布局 |
| 弹出位置 | placement 方位 |
| 形态变体 | variant 线框/填充差异 |
| 自定义状态 | 自定义渲染/插槽外观 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |
| = 5.10.0">面板使用 | 复现「= 5.10.0">面板使用」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `allowClear`

- **说明**：支持清除
- **类型**：boolean | { clearIcon?: ReactNode }
- **默认值**：true
- **版本**：5.8.0: 支持对象形式

#### `bordered`

- **说明**：是否带边框，请使用 `variant` 替代
- **类型**：boolean
- **默认值**：true
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `variant` | 官方取值 `variant` |

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `disabled`

- **说明**：禁用
- **类型**：boolean
- **默认值**：false

#### `expandIcon`

- **说明**：自定义次级菜单展开图标
- **类型**：ReactNode
- **默认值**：-
- **版本**：4.4.0

#### `loadData`

- **说明**：用于动态加载选项，无法与 `showSearch` 一起使用
- **类型**：(selectedOptions) => void
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `showSearch` | 官方取值 `showSearch` |

#### `loadingIcon`

- **说明**：自定义的加载图标
- **类型**：ReactNode
- **默认值**：-

#### `placement`

- **说明**：浮层预设位置
- **类型**：`bottomLeft` `bottomRight` `topLeft` `topRight`
- **默认值**：`bottomLeft`
- **版本**：4.17.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `bottomLeft` | 下左 |
  | `bottomRight` | 下右 |
  | `topLeft` | 上左 |
  | `topRight` | 上右 |

#### `prefix`

- **说明**：自定义前缀
- **类型**：ReactNode
- **默认值**：-
- **版本**：5.22.0

#### `showArrow`

- **说明**：是否显示箭头图标，请使用 `suffixIcon={null}` 替代
- **类型**：boolean
- **默认值**：true

#### `size`

- **说明**：输入框大小
- **类型**：`large` | `medium` | `small`
- **默认值**：`medium`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

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

#### `suffixIcon`

- **说明**：自定义的选择框后缀图标
- **类型**：ReactNode
- **默认值**：-

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

#### `removeIcon`

- **说明**：自定义的多选框清除图标
- **类型**：ReactNode
- **默认值**：-

#### `dropdownMenuColumnStyle`

- **说明**：下拉菜单列的样式，请使用 `styles.popup.listItem` 替换
- **类型**：CSSProperties
- **默认值**：-

#### `popupMenuColumnStyle`

- **说明**：下拉菜单列的样式，请使用 `styles.popup.listItem` 替换
- **类型**：CSSProperties
- **默认值**：-

#### `searchIcon`

- **说明**：自定义的搜索图标
- **类型**：ReactNode
- **默认值**：-
- **版本**：6.3.0

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

- 需要从一组相关联的数据集合进行选择，例如省市区，公司层级，事物分类等。
- 从一个较大的数据集合中进行选择时，用多级分类进行分隔，方便选择。
- 比起 Select 组件，可以在同一个浮层中完成选择，有较好的体验。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **默认值**（`default-value.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **可以自定义显示**（`custom-trigger.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **移入展开**（`hover.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **禁用选项**（`disabled-option.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **选择即改变**（`change-on-select.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **多选**（`multiple.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **自定义回填方式**（`showCheckedStrategy.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **自定义已选项**（`custom-render.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **搜索**（`search.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **动态加载选项**（`lazy.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **自定义字段名**（`fields-name.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
14. **前后缀**（`suffix.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
15. **扩展菜单**（`custom-dropdown.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
16. **弹出位置**（`placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
17. **形态变体**（`variant.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
18. **自定义状态**（`status.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
19. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
20. **= 5.10.0">面板使用**（`panel.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 指定选中项 |
| `defaultValue` | 非受控默认值 | 默认的选中项 |
| `onChange` | 值变化 | 选择完成后的回调 |
| `open` | 受控显隐 | 控制浮层显隐 |
| `onOpenChange` | 显隐变化 | 显示/隐藏浮层的回调 |
| `disabled` | 禁用 | 禁用 |
| `options` | 数据化 options | 可选项数据源 |
| `showSearch` | 搜索 | 在选择框中显示搜索框 |
| `loadData` | 异步加载 | 用于动态加载选项，无法与 `showSearch` 一起使用 |
| `getPopupContainer` | 浮层容器 | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例](https://codepen.io/afc163/pen/zEjNOy?editors=0010) |
| `onSearch` | 搜索回调 | 监听搜索，返回输入的值 |
| `onClear` | 清除 | 清除内容时回调 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 默认值 | `default-value.tsx` | 否 |
| 可以自定义显示 | `custom-trigger.tsx` | 否 |
| 移入展开 | `hover.tsx` | 否 |
| 禁用选项 | `disabled-option.tsx` | 否 |
| 选择即改变 | `change-on-select.tsx` | 否 |
| 多选 | `multiple.tsx` | 否 |
| 自定义回填方式 | `showCheckedStrategy.tsx` | 否 |
| 大小 | `size.tsx` | 否 |
| 自定义已选项 | `custom-render.tsx` | 否 |
| 搜索 | `search.tsx` | 否 |
| 动态加载选项 | `lazy.tsx` | 否 |
| 自定义字段名 | `fields-name.tsx` | 否 |
| 前后缀 | `suffix.tsx` | 否 |
| 扩展菜单 | `custom-dropdown.tsx` | 否 |
| 弹出位置 | `placement.tsx` | 否 |
| 形态变体 | `variant.tsx` | 否 |
| 自定义状态 | `status.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| = 5.10.0">面板使用 | `panel.tsx` | 否 |
| 菜单项省略样式调试 | `ellipsis-debug.tsx` | 是 |
| _InternalPanelDoNotUseOrYouWillBeFired | `render-panel.tsx` | 是 |
| Component Token | `component-token.tsx` | 是 |

### 2.5 实例方法 / Ref

#### 方法 {#methods}

| 名称    | 描述     | 版本 |
| ------- | -------- | ---- |
| blur()  | 移除焦点 |      |
| focus() | 获取焦点 |      |

> 注意，如果需要获得中国省市区数据，可以参考 [china-division](https://gist.github.com/afc163/7582f35654fd03d5be7009444345ea17)。

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

```jsx
<Cascader options={options} onChange={onChange} />
```

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| allowClear | 支持清除 | boolean \| { clearIcon?: ReactNode } | true | 5.8.0: 支持对象形式 | `clearIcon`: 6.4.0 |
| ~~autoClearSearchValue~~ | 是否在选中项后清空搜索框，只在 `multiple` 为 `true` 时有效 | boolean | true | 5.9.0 | × |
| ~~bordered~~ | 是否带边框，请使用 `variant` 替代 | boolean | true | - | × |
| changeOnSelect | 单选时生效（multiple 下始终都可以选择），点选每级菜单选项值都会发生变化。 | boolean | false | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | defaultOpen | 是否默认展示浮层 | boolean | - | defaultValue | 默认的选中项 | string\[] \| number\[] | \[] | disabled | 禁用 | boolean | false | displayRender | 选择后展示的渲染函数 | (label, selectedOptions) => ReactNode | label => label.join(`/`) | `multiple`: 4.18.0 | × |
| tagRender | 自定义 tag 内容 render，仅在多选时生效 | ({ label: string, onClose: function, value: string }) => ReactNode | - | ~~popupClassName~~ | 自定义浮层类名，使用 `classNames.popup.root` 替换 | string | - | 4.23.0 | × |
| ~~dropdownClassName~~ | 自定义浮层类名，请使用 `classNames.popup.root` 替代 | string | - | - | × |
| ~~dropdownRender~~ | 自定义下拉框内容，请使用 `popupRender` 替换 | (menus: ReactNode) => ReactNode | - | 4.4.0 | × |
| popupRender | 自定义下拉框内容 | (menus: ReactNode) => ReactNode | - | ~~dropdownStyle~~ | 下拉菜单的 style 属性，使用 `styles.popup.root` 替换 | CSSProperties | - | expandIcon | 自定义次级菜单展开图标 | ReactNode | - | 4.4.0 | 6.3.0 |
| expandTrigger | 次级菜单的展开方式，可选 'click' 和 'hover' | string | `click` | fieldNames | 自定义 options 中 label value children 的字段 | object | { label: `label`, value: `value`, children: `children` } | getPopupContainer | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例](https://codepen.io/afc163/pen/zEjNOy?editors=0010) | function(triggerNode) | () => document.body | loadData | 用于动态加载选项，无法与 `showSearch` 一起使用 | (selectedOptions) => void | - | loadingIcon | 自定义的加载图标 | ReactNode | - | maxTagCount | 最多显示多少个 tag，响应式模式会对性能产生损耗 | number \| `responsive` | - | 4.17.0 | × |
| maxTagPlaceholder | 隐藏 tag 时显示的内容 | ReactNode \| function(omittedValues) | - | 4.17.0 | × |
| maxTagTextLength | 最大显示的 tag 文本长度 | number | - | 4.17.0 | × |
| notFoundContent | 当下拉列表为空时显示的内容 | ReactNode | `Not Found` | open | 控制浮层显隐 | boolean | - | 4.17.0 | × |
| options | 可选项数据源 | [Option](#option)\[] | - | placeholder | 输入框占位文本 | string | - | placement | 浮层预设位置 | `bottomLeft` `bottomRight` `topLeft` `topRight` | `bottomLeft` | 4.17.0 | × |
| prefix | 自定义前缀 | ReactNode | - | 5.22.0 | × |
| ~~showArrow~~ | 是否显示箭头图标，请使用 `suffixIcon={null}` 替代 | boolean | true | - | × |
| showSearch | 在选择框中显示搜索框 | boolean \| [Object](#showsearch) | false | size | 输入框大小 | `large` \| `medium` \| `small` | `medium` | status | 设置校验状态 | 'error' \| 'warning' | - | 4.19.0 | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | suffixIcon | 自定义的选择框后缀图标 | ReactNode | - | value | 指定选中项 | string\[] \| number\[] | - | variant | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 | 5.19.0 |
| onChange | 选择完成后的回调 | (value, selectedOptions) => void | - | onClear | 清除内容时回调 | () => void | - | - | × |
| ~~onDropdownVisibleChange~~ | 显示/隐藏浮层的回调，请使用 `onOpenChange` 替换 | (value) => void | - | 4.17.0 | × |
| onOpenChange | 显示/隐藏浮层的回调 | (value) => void | - | ~~onPopupVisibleChange~~ | 显示或隐藏浮层的回调，请使用 `onOpenChange` 替代 | (value) => void | - | - | × |
| multiple | 支持多选节点 | boolean | - | 4.17.0 | × |
| removeIcon | 自定义的多选框清除图标 | ReactNode | - | showCheckedStrategy | 定义选中项回填的方式（仅在 `multiple` 为 `true` 时生效）。`Cascader.SHOW_CHILD`: 只显示选中的子节点。`Cascader.SHOW_PARENT`: 只显示父节点（当父节点下所有子节点都选中时）。 | `Cascader.SHOW_PARENT` \| `Cascader.SHOW_CHILD` | `Cascader.SHOW_PARENT` | 4.20.0 | × |
| ~~searchValue~~ | 设置搜索的值，需要与 `showSearch` 配合使用 | string | - | 4.17.0 | × |
| ~~onSearch~~ | 监听搜索，返回输入的值 | (search: string) => void | - | 4.17.0 | × |
| ~~dropdownMenuColumnStyle~~ | 下拉菜单列的样式，请使用 `styles.popup.listItem` 替换 | CSSProperties | - | ~~popupMenuColumnStyle~~ | 下拉菜单列的样式，请使用 `styles.popup.listItem` 替换 | CSSProperties | - | optionRender | 自定义渲染下拉选项 | (option: Option) => React.ReactNode | - | 5.16.0 | × |

### showSearch

`showSearch` 为对象时，其中的字段：

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| autoClearSearchValue | 是否在选中项后清空搜索框，只在 `multiple` 为 `true` 时有效 | boolean | true | 5.9.0 |
| filter | 接收 `inputValue` `path` 两个参数，当 `path` 符合筛选条件时，应返回 true，反之则返回 false | function(inputValue, path): boolean | - | matchInputWidth | 搜索结果列表是否与输入框同宽（[效果](https://github.com/ant-design/ant-design/issues/25779)） | boolean | true | sort | 用于排序 filter 后的选项 | function(a, b, inputValue) | - | onSearch | 监听搜索，返回输入的值 | (search: string) => void | - | 4.17.0 |
| searchIcon | 自定义的搜索图标 | ReactNode | - | 6.3.0 |

### Option

```typescript
interface Option {
  value: string | number;
  label?: React.ReactNode;
  disabled?: boolean;
  children?: Option[];
  // 标记是否为叶子节点，设置了 `loadData` 时有效
  // 设为 `false` 时会强制标记为父节点，即使当前节点没有 children，也会显示展开图标
  isLeaf?: boolean;
}
```

## 方法 {#methods}

| 名称    | 描述     | 版本 |
| ------- | -------- | ---- |
| blur()  | 移除焦点 |      |
| focus() | 获取焦点 |      |

> 注意，如果需要获得中国省市区数据，可以参考 [china-division](https://gist.github.com/afc163/7582f35654fd03d5be7009444345ea17)。

### 导入方式

```js
import { Cascader } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `allowClear` | 支持清除 | boolean \| { clearIcon?: ReactNode } | true | 5.8.0: 支持对象形式 |
| `autoClearSearchValue` | 是否在选中项后清空搜索框，只在 `multiple` 为 `true` 时有效 | boolean | true | 5.9.0 |
| `bordered` | 是否带边框，请使用 `variant` 替代 | boolean | true | - |
| `changeOnSelect` | 单选时生效（multiple 下始终都可以选择），点选每级菜单选项值都会发生变化。 | boolean | false | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `defaultOpen` | 是否默认展示浮层 | boolean | - | — |
| `defaultValue` | 默认的选中项 | string\[] \| number\[] | \[] | — |
| `disabled` | 禁用 | boolean | false | — |
| `displayRender` | 选择后展示的渲染函数 | (label, selectedOptions) => ReactNode | label => label.join(`/`) | `multiple`: 4.18.0 |
| `tagRender` | 自定义 tag 内容 render，仅在多选时生效 | ({ label: string, onClose: function, value: string }) => ReactNode | - | — |
| `popupClassName` | 自定义浮层类名，使用 `classNames.popup.root` 替换 | string | - | 4.23.0 |
| `dropdownClassName` | 自定义浮层类名，请使用 `classNames.popup.root` 替代 | string | - | - |
| `dropdownRender` | 自定义下拉框内容，请使用 `popupRender` 替换 | (menus: ReactNode) => ReactNode | - | 4.4.0 |
| `popupRender` | 自定义下拉框内容 | (menus: ReactNode) => ReactNode | - | — |
| `dropdownStyle` | 下拉菜单的 style 属性，使用 `styles.popup.root` 替换 | CSSProperties | - | — |
| `expandIcon` | 自定义次级菜单展开图标 | ReactNode | - | 4.4.0 |
| `expandTrigger` | 次级菜单的展开方式，可选 'click' 和 'hover' | string | `click` | — |
| `fieldNames` | 自定义 options 中 label value children 的字段 | object | { label: `label`, value: `value`, children: `children` } | — |
| `getPopupContainer` | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例](https://codepen.io/afc163/pen/zEjNOy?editors=0010) | function(triggerNode) | () => document.body | — |
| `loadData` | 用于动态加载选项，无法与 `showSearch` 一起使用 | (selectedOptions) => void | - | — |
| `loadingIcon` | 自定义的加载图标 | ReactNode | - | — |
| `maxTagCount` | 最多显示多少个 tag，响应式模式会对性能产生损耗 | number \| `responsive` | - | 4.17.0 |
| `maxTagPlaceholder` | 隐藏 tag 时显示的内容 | ReactNode \| function(omittedValues) | - | 4.17.0 |
| `maxTagTextLength` | 最大显示的 tag 文本长度 | number | - | 4.17.0 |
| `notFoundContent` | 当下拉列表为空时显示的内容 | ReactNode | `Not Found` | — |
| `open` | 控制浮层显隐 | boolean | - | 4.17.0 |
| `options` | 可选项数据源 | [Option](#option)\[] | - | — |
| `placeholder` | 输入框占位文本 | string | - | — |
| `placement` | 浮层预设位置 | `bottomLeft` `bottomRight` `topLeft` `topRight` | `bottomLeft` | 4.17.0 |
| `prefix` | 自定义前缀 | ReactNode | - | 5.22.0 |
| `showArrow` | 是否显示箭头图标，请使用 `suffixIcon={null}` 替代 | boolean | true | - |
| `showSearch` | 在选择框中显示搜索框 | boolean \| [Object](#showsearch) | false | — |
| `size` | 输入框大小 | `large` \| `medium` \| `small` | `medium` | — |
| `status` | 设置校验状态 | 'error' \| 'warning' | - | 4.19.0 |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `suffixIcon` | 自定义的选择框后缀图标 | ReactNode | - | — |
| `value` | 指定选中项 | string\[] \| number\[] | - | — |
| `variant` | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 |
| `onChange` | 选择完成后的回调 | (value, selectedOptions) => void | - | — |
| `onClear` | 清除内容时回调 | () => void | - | - |
| `onDropdownVisibleChange` | 显示/隐藏浮层的回调，请使用 `onOpenChange` 替换 | (value) => void | - | 4.17.0 |
| `onOpenChange` | 显示/隐藏浮层的回调 | (value) => void | - | — |
| `onPopupVisibleChange` | 显示或隐藏浮层的回调，请使用 `onOpenChange` 替代 | (value) => void | - | - |
| `multiple` | 支持多选节点 | boolean | - | 4.17.0 |
| `removeIcon` | 自定义的多选框清除图标 | ReactNode | - | — |
| `showCheckedStrategy` | 定义选中项回填的方式（仅在 `multiple` 为 `true` 时生效）。`Cascader.SHOW_CHILD`: 只显示选中的子节点。`Cascader.SHOW_PARENT`: 只显示父节点（当父节点下所有子节点都选中时）。 | `Cascader.SHOW_PARENT` \| `Cascader.SHOW_CHILD` | `Cascader.SHOW_PARENT` | 4.20.0 |
| `searchValue` | 设置搜索的值，需要与 `showSearch` 配合使用 | string | - | 4.17.0 |
| `onSearch` | 监听搜索，返回输入的值 | (search: string) => void | - | 4.17.0 |
| `dropdownMenuColumnStyle` | 下拉菜单列的样式，请使用 `styles.popup.listItem` 替换 | CSSProperties | - | — |
| `popupMenuColumnStyle` | 下拉菜单列的样式，请使用 `styles.popup.listItem` 替换 | CSSProperties | - | — |
| `optionRender` | 自定义渲染下拉选项 | (option: Option) => React.ReactNode | - | 5.16.0 |
| `filter` | 接收 `inputValue` `path` 两个参数，当 `path` 符合筛选条件时，应返回 true，反之则返回 false | function(inputValue, path): boolean | - | — |
| `limit` | 搜索结果展示数量 | number \| false | 50 | — |
| `matchInputWidth` | 搜索结果列表是否与输入框同宽（[效果](https://github.com/ant-design/ant-design/issues/25779)） | boolean | true | — |
| `render` | 用于渲染 filter 后的选项 | function(inputValue, path): ReactNode | - | — |
| `sort` | 用于排序 filter 后的选项 | function(a, b, inputValue) | - | — |
| `searchIcon` | 自定义的搜索图标 | ReactNode | - | 6.3.0 |
| `blur()` | 移除焦点 | — | — | — |
| `focus()` | 获取焦点 | — | — | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Cascader** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **20** 个，均需可复现。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/cascader
- 中文文档：https://ant.design/components/cascader-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/cascader
- 驱动 gpui kit：`cascader`
