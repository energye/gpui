# TreeSelect 树选择
> 来源：[Ant Design 6.5.x TreeSelect](https://ant.design/components/tree-select)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：树型选择控件。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

树型选择控件。

**TreeSelect** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 多选 | 多选标签/勾选外观 |
| 从数据直接生成 | 复现「从数据直接生成」视觉与布局 |
| 可勾选 | 复现「可勾选」视觉与布局 |
| 异步加载 | loading 指示与防重复 |
| 线性样式 | 复现「线性样式」视觉与布局 |
| 弹出位置 | placement 方位 |
| 形态变体 | variant 线框/填充差异 |
| 自定义状态 | 自定义渲染/插槽外观 |
| 最大选中数量 | 复现「最大选中数量」视觉与布局 |
| 前后缀 | 复现「前后缀」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `allowClear`

- **说明**：自定义清除按钮
- **类型**：boolean | { clearIcon?: ReactNode }
- **默认值**：false
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

- **说明**：是否禁用
- **类型**：boolean
- **默认值**：false

#### `dropdownStyle`

- **说明**：下拉菜单的样式，使用 `styles.popup.root` 替换
- **类型**：object
- **默认值**：-

#### `labelInValue`

- **说明**：是否把每个选项的 label 包装到 value 中，会把 value 类型从 `string` 变为 {value: string, label: ReactNode, halfChecked: boolean(选项列表是否为半选状态，并且不会展示到值中) } 的格式
- **类型**：boolean
- **默认值**：false

#### `loadData`

- **说明**：异步加载数据。在过滤时不会调用以防止网络堵塞，可参考 FAQ 获得更多内容
- **类型**：function(node)
- **默认值**：-

#### `maxCount`

- **说明**：指定可选中的最多 items 数量，仅在 `multiple=true` 时生效。如果此时 (`showCheckedStrategy = 'SHOW_ALL'` 且未开启 `treeCheckStrictly`)，或使用 `showCheckedStrategy = 'SHOW_PARENT'`，则maxCount无效。
- **类型**：number
- **默认值**：-
- **版本**：5.23.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `treeCheckStrictly` | 官方取值 `treeCheckStrictly` |

#### `placement`

- **说明**：选择框弹出的位置
- **类型**：`bottomLeft` `bottomRight` `topLeft` `topRight`
- **默认值**：bottomLeft
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

- **说明**：选择框大小
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
- **默认值**：``

#### `switcherIcon`

- **说明**：自定义树节点的展开/折叠图标
- **类型**：ReactNode | ((props: AntTreeNodeProps) => ReactNode)
- **默认值**：-
- **版本**：renderProps: 4.20.0

#### `treeCheckStrictly`

- **说明**：`checkable` 状态下节点选择完全受控（父子节点选中状态不再关联），会使得 `labelInValue` 强制为 true
- **类型**：boolean
- **默认值**：false
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `checkable` | 官方取值 `checkable` |
  | `labelInValue` | 官方取值 `labelInValue` |

#### `treeDataSimpleMode`

- **说明**：使用简单格式的 treeData，具体设置参考可设置的类型 (此时 treeData 应变为这样的数据结构: \[{id:1, pId:0, value:'1', title:"test1",...},...]， `pId` 是父节点的 id)
- **类型**：boolean | object<{ id: string, pId: string, rootPId: string }>
- **默认值**：false
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `pId` | 官方取值 `pId` |

#### `treeIcon`

- **说明**：是否展示 TreeNode title 前的图标，没有默认样式，如设置为 true，需要自行定义图标相关样式
- **类型**：boolean
- **默认值**：false

#### `treeLine`

- **说明**：是否展示线条样式，请参考 [Tree - showLine](/components/tree-cn#tree-demo-line)
- **类型**：boolean | object
- **默认值**：false
- **版本**：4.17.0

#### `treeLoadedKeys`

- **说明**：（受控）已经加载的节点，需要配合 `loadData` 使用
- **类型**：string[]
- **默认值**：[]
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `loadData` | 官方取值 `loadData` |

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

#### `title`

- **说明**：树节点显示的内容
- **类型**：ReactNode
- **默认值**：`---`

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

类似 Select 的选择控件，可选择的数据结构是一个树形结构时，可以使用 TreeSelect，例如公司层级、学科系统、分类目录等等。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **多选**（`multiple.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **从数据直接生成**（`treeData.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **可勾选**（`checkable.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **异步加载**（`async.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **线性样式**（`treeLine.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **弹出位置**（`placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **形态变体**（`variant.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **自定义状态**（`status.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **最大选中数量**（`maxCount.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **前后缀**（`suffix.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 指定当前选中的条目 |
| `defaultValue` | 非受控默认值 | 指定默认选中的条目 |
| `onChange` | 值变化 | 选中树节点时调用此函数 |
| `onSelect` | 选中 | 被选中时调用 |
| `open` | 受控显隐 | 是否展开下拉菜单 |
| `onOpenChange` | 显隐变化 | 展开下拉菜单的回调 |
| `disabled` | 禁用 | 是否禁用 |
| `treeData` | 树数据 | treeNodes 数据，如果设置则不需要手动构造 TreeNode 节点（value 在整个树范围内唯一） |
| `showSearch` | 搜索 | 是否支持搜索框 |
| `loadData` | 异步加载 | 异步加载数据。在过滤时不会调用以防止网络堵塞，可参考 FAQ 获得更多内容 |
| `virtual` | 虚拟滚动 | 设置 false 时关闭虚拟滚动 |
| `getPopupContainer` | 浮层容器 | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例](https://codepen.io/afc163/pen/zEjNOy?editors=0010) |
| `onSearch` | 搜索回调 | 文本框值变化时的回调 |
| `onClear` | 清除 | 清除内容时回调 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 多选 | `multiple.tsx` | 否 |
| 从数据直接生成 | `treeData.tsx` | 否 |
| 可勾选 | `checkable.tsx` | 否 |
| 异步加载 | `async.tsx` | 否 |
| 线性样式 | `treeLine.tsx` | 否 |
| 弹出位置 | `placement.tsx` | 否 |
| 形态变体 | `variant.tsx` | 否 |
| 自定义状态 | `status.tsx` | 否 |
| 最大选中数量 | `maxCount.tsx` | 否 |
| 前后缀 | `suffix.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| \_InternalPanelDoNotUseOrYouWillBeFired | `render-panel.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |

### 2.5 实例方法 / Ref

#### 方法

### Tree 方法

| 名称    | 描述     | 版本 |
| ------- | -------- | ---- |
| blur()  | 移除焦点 |      |
| focus() | 获取焦点 |      |

### 2.6 FAQ

## FAQ

### onChange 时如何获得父节点信息？ {#faq-parent-node-info}

从性能角度考虑，我们默认不透出父节点信息。你可以这样获得：

### 自定义 Option 样式导致滚动异常怎么办？ {#faq-custom-option-scroll}

请参考 Select 的 [FAQ](/components/select-cn)。

### 为何在搜索时 `loadData` 不会触发展开？ {#faq-load-data-expand}

在 v4 alpha 版本中，默认在搜索时亦会进行搜索。但是经反馈，在输入时会快速阻塞网络。因而改为搜索不触发 `loadData`。但是你仍然可以通过 `filterTreeNode` 处理异步加载逻辑：

```tsx
 {
    const match = YOUR_LOGIC_HERE;

    if (match && !treeNode.isLeaf && !treeNode.children) {
      // Do some loading logic
    }

    return match;
  }}
/>
```

### 为何弹出框不能横向滚动？ {#faq-popup-not-scroll}

关闭虚拟滚动即可，因为开启虚拟滚动时无法准确的测量完整列表的 `scrollWidth`。

### 2.7 组合关系

- **依赖等级 L3**：浮层件 + 表单件；树行复用 `tree`（行高 24/缩进 24/loading 转圈），弹层复用 Select 定位；先做 `tree`/`select` 再做本件。
- **Form**：`value`（单选 string / 多选 `[]string`，`labelInValue` 走 `TreeSelectValue`）直绑。
- **ConfigProvider**：size/variant/status 全局默认。
- **浮层**：Modal/Drawer 内注意 `getPopupContainer`；`placement` 四角。
- **文件归属**：`ui/kit/tree-select/`（触发器 + 树弹层 + 勾选回填）。
---
## 3. 配置（API）
通用属性参考：[Common props](https://ant.design/docs/react/common-props)。

以下为官方 API 全文，作为 kit 配置面与类型设计的权威清单。

## API

通用属性参考：[通用属性](/docs/react/common-props)

### Tree props

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| allowClear | 自定义清除按钮 | boolean \| { clearIcon?: ReactNode } | false | 5.8.0: 支持对象形式 | × |
| ~~autoClearSearchValue~~ | 当多选模式下值被选择，自动清空搜索框 | boolean | true |  | × |
| ~~bordered~~ | 是否带边框，请使用 `variant` 替代 | boolean | true | - | × |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - |  | 5.25.0 |
| defaultOpen | 是否默认展开下拉菜单 | boolean | - |  | × |
| defaultValue | 指定默认选中的条目 | string \| string\[] | - |  | × |
| disabled | 是否禁用 | boolean | false |  | × |
| ~~dropdownClassName~~ | 下拉菜单的 className 属性，请使用 `classNames.popup.root` 替代 | string | - | - | × |
| ~~dropdownMatchSelectWidth~~ | 下拉菜单和选择器是否同宽，请使用 `popupMatchSelectWidth` 替代 | boolean \| number | true | - | × |
| ~~popupClassName~~ | 下拉菜单的 className 属性，使用 `classNames.popup.root` 替换 | string | - | 4.23.0 | × |
| popupMatchSelectWidth | 下拉菜单和选择器同宽。默认将设置 `min-width`，当值小于选择框宽度时会被忽略。false 时会关闭虚拟滚动 | boolean \| number | true | 5.5.0 | × |
| ~~dropdownRender~~ | 自定义下拉框内容，使用 `popupRender` 替换 | (originNode: ReactNode, props) => ReactNode | - |  | × |
| popupRender | 自定义下拉框内容 | (originNode: ReactNode, props) => ReactNode | - |  | × |
| ~~dropdownStyle~~ | 下拉菜单的样式，使用 `styles.popup.root` 替换 | object | - |  | × |
| fieldNames | 自定义节点 label、value、children 的字段 | object | { label: `label`, value: `value`, children: `children` } | 4.17.0 | × |
| ~~filterTreeNode~~ | 是否根据输入项进行筛选，默认用 treeNodeFilterProp 的值作为要筛选的 TreeNode 的属性值 | boolean \| function(inputValue: string, treeNode: TreeNode) | function |  | × |
| getPopupContainer | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例](https://codepen.io/afc163/pen/zEjNOy?editors=0010) | function(triggerNode) | () => document.body |  | × |
| labelInValue | 是否把每个选项的 label 包装到 value 中，会把 value 类型从 `string` 变为 {value: string, label: ReactNode, halfChecked: boolean} 的格式 | boolean | false |  | × |
| listHeight | 设置弹窗滚动高度 | number | 256 |  | × |
| loadData | 异步加载数据。在过滤时不会调用以防止网络堵塞，可参考 FAQ 获得更多内容 | function(node) | - |  | × |
| maxCount | 指定可选中的最多 items 数量，仅在 `multiple=true` 时生效。如果此时 (`showCheckedStrategy = 'SHOW_ALL'` 且未开启 `treeCheckStrictly`)，或使用 `showCheckedStrategy = 'SHOW_PARENT'`，则maxCount无效。 | number | - | 5.23.0 | × |
| maxTagCount | 最多显示多少个 tag，响应式模式会对性能产生损耗 | number \| `responsive` | - | responsive: 4.10 | × |
| maxTagPlaceholder | 隐藏 tag 时显示的内容 | ReactNode \| function(omittedValues) | - |  | × |
| maxTagTextLength | 最大显示的 tag 文本长度 | number | - |  | × |
| multiple | 支持多选（当设置 treeCheckable 时自动变为 true） | boolean | false |  | × |
| notFoundContent | 当下拉列表为空时显示的内容 | ReactNode | `Not Found` |  | × |
| open | 是否展开下拉菜单 | boolean | - |  | × |
| placeholder | 选择框默认文字 | string | - |  | × |
| placement | 选择框弹出的位置 | `bottomLeft` `bottomRight` `topLeft` `topRight` | bottomLeft |  | × |
| prefix | 自定义前缀 | ReactNode | - | 5.22.0 | × |
| ~~searchValue~~ | 搜索框的值，可以通过 `onSearch` 获取用户输入 | string | - |  | × |
| ~~showArrow~~ | 是否显示箭头图标，请使用 `suffixIcon={null}` 替代 | boolean | true | - | × |
| showCheckedStrategy | 配置 `treeCheckable` 时，定义选中项回填的方式。`TreeSelect.SHOW_ALL`: 显示所有选中节点(包括父节点)。`TreeSelect.SHOW_PARENT`: 只显示父节点(当父节点下所有子节点都选中时)。 默认只显示子节点 | `TreeSelect.SHOW_ALL` \| `TreeSelect.SHOW_PARENT` \| `TreeSelect.SHOW_CHILD` | `TreeSelect.SHOW_CHILD` |  | × |
| showSearch | 是否支持搜索框 | boolean \| [Object](#showsearch) | 单选：false \| 多选：true |  | × |
| size | 选择框大小 | `large` \| `medium` \| `small` | - |  | × |
| status | 设置校验状态 | 'error' \| 'warning' | - | 4.19.0 | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - |  | × |
| suffixIcon | 自定义的选择框后缀图标 | ReactNode | `<DownOutlined />` |  | × |
| switcherIcon | 自定义树节点的展开/折叠图标 | ReactNode \| ((props: AntTreeNodeProps) => ReactNode) | - | renderProps: 4.20.0 | 5.28.0 |
| tagRender | 自定义 tag 内容，多选时生效 | (props) => ReactNode | - |  | × |
| treeCheckable | 显示 Checkbox | boolean | false |  | × |
| treeCheckStrictly | `checkable` 状态下节点选择完全受控（父子节点选中状态不再关联），会使得 `labelInValue` 强制为 true | boolean | false |  | × |
| treeData | treeNodes 数据，如果设置则不需要手动构造 TreeNode 节点（value 在整个树范围内唯一） | array<{value, title, children, [disabled, disableCheckbox, selectable, checkable]}> | \[] |  | × |
| treeDataSimpleMode | 使用简单格式的 treeData，具体设置参考可设置的类型 (此时 treeData 应变为这样的数据结构: [{id:1, pId:0, value:'1', title:"test1",...}]， `pId` 是父节点的 id) | boolean \| object<{ id: string, pId: string, rootPId: string }> | false |  | × |
| treeDefaultExpandAll | 默认展开所有树节点 | boolean | false |  | × |
| treeDefaultExpandedKeys | 默认展开的树节点 | string\[] | - |  | × |
| treeExpandAction | 点击节点 title 时的展开逻辑，可选：false \| `click` \| `doubleClick` | string \| boolean | false | 4.21.0 | × |
| treeExpandedKeys | 设置展开的树节点 | string\[] | - |  | × |
| treeIcon | 是否展示 TreeNode title 前的图标，没有默认样式，如设置为 true，需要自行定义图标相关样式 | boolean | false |  | × |
| treeLine | 是否展示线条样式，请参考 [Tree - showLine](/components/tree-cn#tree-demo-line) | boolean \| object | false | 4.17.0 | × |
| treeLoadedKeys | （受控）已经加载的节点，需要配合 `loadData` 使用 | string[] | [] |  | × |
| ~~treeNodeFilterProp~~ | 输入项过滤对应的 treeNode 属性 | string | `value` |  | × |
| treeNodeLabelProp | 作为显示的 prop 设置 | string | `title` |  | × |
| treeTitleRender | 自定义渲染节点 | (nodeData) => ReactNode | - | 5.12.0 | × |
| value | 指定当前选中的条目 | string \| string\[] | - |  | × |
| variant | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 | 5.19.0 |
| virtual | 设置 false 时关闭虚拟滚动 | boolean | true | 4.1.0 | × |
| onChange | 选中树节点时调用此函数 | function(value, label, extra) | - |  | × |
| onClear | 清除内容时回调 | () => void | - | - | × |
| ~~onDropdownVisibleChange~~ | 展开下拉菜单的回调，使用 `onOpenChange` 替换 | (open: boolean) => void | - |  | × |
| onOpenChange | 展开下拉菜单的回调 | (open: boolean) => void | - |  | × |
| onPopupScroll | 下拉列表滚动时的回调 | (event: UIEvent) => void | - | 5.17.0 | × |
| ~~onSearch~~ | 文本框值变化时的回调 | function(value: string) | - |  | × |
| onSelect | 被选中时调用 | function(value, node, extra) | - |  | × |
| onTreeExpand | 展示节点时调用 | function(expandedKeys) | - |  | × |
### showSearch

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| autoClearSearchValue | 当多选模式下值被选择，自动清空搜索框 | boolean | true |  | × |
| filterTreeNode | 是否根据输入项进行筛选，默认用 treeNodeFilterProp 的值作为要筛选的 TreeNode 的属性值 | boolean \| function(inputValue: string, treeNode: TreeNode) | function |  | × |
| searchValue | 搜索框的值，可以通过 `onSearch` 获取用户输入 | string | - |  | × |
| treeNodeFilterProp | 输入项过滤对应的 treeNode 属性 | string | `value` |  | × |
| onSearch | 文本框值变化时的回调 | function(value: string) | - |  | × |
| 名称    | 描述     | 版本 |
| ------- | -------- | ---- |
| blur()  | 移除焦点 |      |
| focus() | 获取焦点 |      |

### TreeNode props

> 建议使用 treeData 来代替 TreeNode，免去手动构造的麻烦

| 参数            | 说明                                               | 类型      | 默认值 | 版本 |
| --------------- | -------------------------------------------------- | --------- | ------ | ---- |
| checkable       | 当树为 Checkbox 时，设置独立节点是否展示 Checkbox  | boolean   | -      |      |
| disableCheckbox | 禁掉 Checkbox                                      | boolean   | false  |      |
| disabled        | 是否禁用                                           | boolean   | false  |      |
| isLeaf          | 是否是叶子节点                                     | boolean   | false  |      |
| key             | 此项必须设置（其值在整个树范围内唯一）             | string    | -      |      |
| selectable      | 是否可选                                           | boolean   | true   |      |
| title           | 树节点显示的内容                                   | ReactNode | `---`  |      |
| value           | 默认根据此属性值进行筛选（其值在整个树范围内唯一） | string    | -      |      |

### 导入方式

```js
import { TreeSelect } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `allowClear` | 自定义清除按钮 | boolean \| { clearIcon?: ReactNode } | false | 5.8.0: 支持对象形式 |
| `autoClearSearchValue` | 当多选模式下值被选择，自动清空搜索框 | boolean | true | — |
| `bordered` | 是否带边框，请使用 `variant` 替代 | boolean | true | - |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `defaultOpen` | 是否默认展开下拉菜单 | boolean | - | — |
| `defaultValue` | 指定默认选中的条目 | string \| string\[] | - | — |
| `disabled` | 是否禁用 | boolean | false | — |
| `dropdownClassName` | 下拉菜单的 className 属性，请使用 `classNames.popup.root` 替代 | string | - | - |
| `dropdownMatchSelectWidth` | 下拉菜单和选择器是否同宽，请使用 `popupMatchSelectWidth` 替代 | boolean \| number | true | - |
| `popupClassName` | 下拉菜单的 className 属性，使用 `classNames.popup.root` 替换 | string | - | 4.23.0 |
| `popupMatchSelectWidth` | 下拉菜单和选择器同宽。默认将设置 `min-width`，当值小于选择框宽度时会被忽略。false 时会关闭虚拟滚动 | boolean \| number | true | 5.5.0 |
| `dropdownRender` | 自定义下拉框内容，使用 `popupRender` 替换 | (originNode: ReactNode, props) => ReactNode | - | — |
| `popupRender` | 自定义下拉框内容 | (originNode: ReactNode, props) => ReactNode | - | — |
| `dropdownStyle` | 下拉菜单的样式，使用 `styles.popup.root` 替换 | object | - | — |
| `fieldNames` | 自定义节点 label、value、children 的字段 | object | { label: `label`, value: `value`, children: `children` } | 4.17.0 |
| `filterTreeNode` | 是否根据输入项进行筛选，默认用 treeNodeFilterProp 的值作为要筛选的 TreeNode 的属性值 | boolean \| function(inputValue: string, treeNode: TreeNode) (函数需要返回 bool 值) | function | — |
| `getPopupContainer` | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例](https://codepen.io/afc163/pen/zEjNOy?editors=0010) | function(triggerNode) | () => document.body | — |
| `labelInValue` | 是否把每个选项的 label 包装到 value 中，会把 value 类型从 `string` 变为 {value: string, label: ReactNode, halfChecked: boolean(选项列表是否为半选状态，并且不会展示到值中) } 的格式 | boolean | false | — |
| `listHeight` | 设置弹窗滚动高度 | number | 256 | — |
| `loadData` | 异步加载数据。在过滤时不会调用以防止网络堵塞，可参考 FAQ 获得更多内容 | function(node) | - | — |
| `maxCount` | 指定可选中的最多 items 数量，仅在 `multiple=true` 时生效。如果此时 (`showCheckedStrategy = 'SHOW_ALL'` 且未开启 `treeCheckStrictly`)，或使用 `showCheckedStrategy = 'SHOW_PARENT'`，则maxCount无效。 | number | - | 5.23.0 |
| `maxTagCount` | 最多显示多少个 tag，响应式模式会对性能产生损耗 | number \| `responsive` | - | responsive: 4.10 |
| `maxTagPlaceholder` | 隐藏 tag 时显示的内容 | ReactNode \| function(omittedValues) | - | — |
| `maxTagTextLength` | 最大显示的 tag 文本长度 | number | - | — |
| `multiple` | 支持多选（当设置 treeCheckable 时自动变为 true） | boolean | false | — |
| `notFoundContent` | 当下拉列表为空时显示的内容 | ReactNode | `Not Found` | — |
| `open` | 是否展开下拉菜单 | boolean | - | — |
| `placeholder` | 选择框默认文字 | string | - | — |
| `placement` | 选择框弹出的位置 | `bottomLeft` `bottomRight` `topLeft` `topRight` | bottomLeft | — |
| `prefix` | 自定义前缀 | ReactNode | - | 5.22.0 |
| `searchValue` | 搜索框的值，可以通过 `onSearch` 获取用户输入 | string | - | — |
| `showArrow` | 是否显示箭头图标，请使用 `suffixIcon={null}` 替代 | boolean | true | - |
| `showCheckedStrategy` | 配置 `treeCheckable` 时，定义选中项回填的方式。`TreeSelect.SHOW_ALL`: 显示所有选中节点(包括父节点)。`TreeSelect.SHOW_PARENT`: 只显示父节点(当父节点下所有子节点都选中时)。 默认只显示子节点 | `TreeSelect.SHOW_ALL` \| `TreeSelect.SHOW_PARENT` \| `TreeSelect.SHOW_CHILD` | `TreeSelect.SHOW_CHILD` | — |
| `showSearch` | 是否支持搜索框 | boolean \| [Object](#showsearch) | 单选：false \| 多选：true | — |
| `size` | 选择框大小 | `large` \| `medium` \| `small` | - | — |
| `status` | 设置校验状态 | 'error' \| 'warning' | - | 4.19.0 |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `suffixIcon` | 自定义的选择框后缀图标 | ReactNode | `<DownOutlined />` | — |
| `switcherIcon` | 自定义树节点的展开/折叠图标 | ReactNode \| ((props: AntTreeNodeProps) => ReactNode) | - | renderProps: 4.20.0 |
| `tagRender` | 自定义 tag 内容，多选时生效 | (props) => ReactNode | - | — |
| `treeCheckable` | 显示 Checkbox | boolean | false | — |
| `treeCheckStrictly` | `checkable` 状态下节点选择完全受控（父子节点选中状态不再关联），会使得 `labelInValue` 强制为 true | boolean | false | — |
| `treeData` | treeNodes 数据，如果设置则不需要手动构造 TreeNode 节点（value 在整个树范围内唯一） | array<{value, title, children, \[disabled, disableCheckbox, selectable, checkable]}> | \[] | — |
| `treeDataSimpleMode` | 使用简单格式的 treeData，具体设置参考可设置的类型 (此时 treeData 应变为这样的数据结构: \[{id:1, pId:0, value:'1', title:"test1",...},...]， `pId` 是父节点的 id) | boolean \| object<{ id: string, pId: string, rootPId: string }> | false | — |
| `treeDefaultExpandAll` | 默认展开所有树节点 | boolean | false | — |
| `treeDefaultExpandedKeys` | 默认展开的树节点 | string\[] | - | — |
| `treeExpandAction` | 点击节点 title 时的展开逻辑，可选：false \| `click` \| `doubleClick` | string \| boolean | false | 4.21.0 |
| `treeExpandedKeys` | 设置展开的树节点 | string\[] | - | — |
| `treeIcon` | 是否展示 TreeNode title 前的图标，没有默认样式，如设置为 true，需要自行定义图标相关样式 | boolean | false | — |
| `treeLine` | 是否展示线条样式，请参考 [Tree - showLine](/components/tree-cn#tree-demo-line) | boolean \| object | false | 4.17.0 |
| `treeLoadedKeys` | （受控）已经加载的节点，需要配合 `loadData` 使用 | string[] | [] | — |
| `treeNodeFilterProp` | 输入项过滤对应的 treeNode 属性 | string | `value` | — |
| `treeNodeLabelProp` | 作为显示的 prop 设置 | string | `title` | — |
| `treeTitleRender` | 自定义渲染节点 | (nodeData) => ReactNode | - | 5.12.0 |
| `value` | 指定当前选中的条目 | string \| string\[] | - | — |
| `variant` | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 |
| `virtual` | 设置 false 时关闭虚拟滚动 | boolean | true | 4.1.0 |
| `onChange` | 选中树节点时调用此函数 | function(value, label, extra) | - | — |
| `onClear` | 清除内容时回调 | () => void | - | - |
| `onDropdownVisibleChange` | 展开下拉菜单的回调，使用 `onOpenChange` 替换 | (open: boolean) => void | - | — |
| `onOpenChange` | 展开下拉菜单的回调 | (open: boolean) => void | - | — |
| `onPopupScroll` | 下拉列表滚动时的回调 | (event: UIEvent) => void | - | 5.17.0 |
| `onSearch` | 文本框值变化时的回调 | function(value: string) | - | — |
| `onSelect` | 被选中时调用 | function(value, node, extra) | - | — |
| `onTreeExpand` | 展示节点时调用 | function(expandedKeys) | - | — |
| `blur()` | 移除焦点 | — | — | — |
| `focus()` | 获取焦点 | — | — | — |
| `checkable` | 当树为 Checkbox 时，设置独立节点是否展示 Checkbox | boolean | - | — |
| `disableCheckbox` | 禁掉 Checkbox | boolean | false | — |
| `isLeaf` | 是否是叶子节点 | boolean | false | — |
| `key` | 此项必须设置（其值在整个树范围内唯一） | string | - | — |
| `selectable` | 是否可选 | boolean | true | — |
| `title` | 树节点显示的内容 | ReactNode | `---` | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **TreeSelect** 的验收清单：

1. **配置面**：覆盖 §6.8 P0 字段；P1 可分期但命名预留。
2. **视觉态**：default / hover / active / focus / disabled / loading。
3. **尺寸态**：small / medium / large（适用者）。
4. **受控/非受控**：value+onChange 与 defaultValue。
5. **数据驱动**：options / items / columns / treeData / fileList 等。
6. **无障碍**：焦点、角色、键盘、读屏。
7. **RTL**：placement / orientation 镜像。
8. **浮层**：z-index、挂载容器、遮挡、滚动。
9. **性能**：虚拟列表、防抖、减少重绘。
10. **主题**：Token 化；支持 reduced-motion。
11. **示例矩阵**：§6.8 P0 主路径 8 例（基本、多选、从数据直接生成、可勾选、异步加载、线性样式、弹出位置、形态变体）；余下 4 例（自定义状态、最大选中数量、前后缀、语义结构）P1，Notes 显式列出。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/tree-select
- 中文文档：https://ant.design/components/tree-select-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/tree-select
- 驱动 gpui kit：`tree-select`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **TreeSelect** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/tree-select/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（TreeSelect）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 受控输入/选择、弹层、清除、校验 status、尺寸档 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（TreeSelect）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：树型选择控件。

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
| 树行高 `titleHeight` | **24** | Tree `controlHeightSM`（下拉树每行，见 [tree.md](./tree.md#621-几何与组件-token)） |
| switcher 宽高 | **24** | = titleHeight（展开/折叠图标热区） |
| 每层缩进 `indentSize` | **24** | = titleHeight（子层左缩进） |
| 树行内边距 | **4** | `paddingXS/2`（行块内边） |
| 下拉滚动高 `listHeight` | **256** | 弹窗滚动高度，超高列内滚动 |
| 下拉面板内边距 | **4** | Select `paddingXXS` |
| showLine 叶图标直径 | **14** | 组件常量（线框圆，`treeLine` 时） |

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
| `allowClear` | 自定义清除按钮 | boolean \ | { clearIcon?: ReactNode } |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `defaultOpen` | 是否默认展开下拉菜单 | boolean | - |
| `defaultValue` | 指定默认选中的条目 | string \ | string\[] |
| `disabled` | 是否禁用 | boolean | false |
| `popupMatchSelectWidth` | 下拉菜单和选择器同宽。默认将设置 `min-width`，当值小于选择框宽度时会被忽略。false 时会关闭虚拟滚动 | boolean \ | number |
| `popupRender` | 自定义下拉框内容 | (originNode: ReactNode, props) => Rea… | - |
| `fieldNames` | 自定义节点 label、value、children 的字段 | object | { label: `label`, value: `value`, children: `children` } |
| `getPopupContainer` | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例]… | function(triggerNode) | () => document.body |
| `labelInValue` | 是否把每个选项的 label 包装到 value 中，会把 value 类型从 `string` 变为 {valu… | boolean | false |
| `listHeight` | 设置弹窗滚动高度 | number | 256 |
| `loadData` | 异步加载数据。在过滤时不会调用以防止网络堵塞，可参考 FAQ 获得更多内容 | function(node) | - |
| `maxCount` | 指定可选中的最多 items 数量，仅在 `multiple=true` 时生效。如果此时 (`showCheck… | number | - |
| `maxTagCount` | 最多显示多少个 tag，响应式模式会对性能产生损耗 | number \ | `responsive` |
| `maxTagPlaceholder` | 隐藏 tag 时显示的内容 | ReactNode \ | function(omittedValues) |
| `maxTagTextLength` | 最大显示的 tag 文本长度 | number | - |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
closed ──open──► dropdown tree
single: click node ──► commit value + close + onChange
multiple: click ──► toggle + stay open (+ maxCount gate) + onChange
checkable: check ──► cascade (unless strictly) ──► refill by strategy ──► tags + onChange
search ──► filter tree (loadData 不触发) ──► autoExpand hit branch
async: expand unloaded ──► loadData ──► loadedKeys + children
controlled: value/open/expandedKeys 外部优先；内部只回调
disabled ──► 不打开
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| TSE-S1 | 单选点节点 | `value` 单值 + 关下拉 + `onChange/onSelect` |
| TSE-S2 | 多选（`multiple`） | 值数组 toggle；下拉不关；`maxCount` 触顶拒绝并保持 |
| TSE-S3 | 勾选回填 `SHOW_CHILD`（默认） | 仅回填子节点标签 |
| TSE-S4 | 勾选回填 `SHOW_ALL` | 回填全部选中节点（含父） |
| TSE-S5 | 勾选回填 `SHOW_PARENT` | 父全选时仅回填父标签 |
| TSE-S6 | `treeCheckStrictly=true` | 父子不联动，且 `labelInValue` 强制 true（含 halfChecked） |
| TSE-S7 | 搜索（`showSearch/filterTreeNode`） | 按 `value/title` 过滤，命中分支自动展开；搜索不调 `loadData` |
| TSE-S8 | 异步（`loadData/treeLoadedKeys`） | 未加载节点转圈，完成后挂 `children` + `onTreeExpand/onLoad` |
| TSE-S9 | 受控（`value/open/treeExpandedKeys`） | 外部优先，内部仅回调（`onChange/onOpenChange/onTreeExpand`） |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 / 变体 | 规则 |
| --- | --- |
| default | 容器底 + 边框（outlined）或族默认皮；Token 色 |
| hover | 边框/底强调 |
| focus | **可见** focus ring；主色边 |
| disabled | 降对比；不可编辑 |
| status=error/warning | 语义色边框/反馈 |
| 弹层 open | elevation 阴影；与触发器对齐 placement |

**variant 矩阵（`variant` × chrome，L2，触发器壳与 Input 同规则）：**

| variant | 填充 | 边框 | focus | 备注 |
| --- | --- | --- | --- | --- |
| `outlined`（默认） | `colorBgContainer` | 1px `colorBorder` 全边框 | 主色边 + 可见 ring | 默认 |
| `filled` | `colorFillAlter` 浅底 | 无/弱边框 | 主色边 + ring | 浅底形态 |
| `borderless` | 透明 | 无 | 仅 ring 可见 | 无 chrome |
| `underlined` | 透明 | 仅底边 1px `colorBorder` | 底边走主色 | 底边线形态 |


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
| 单选/多选/勾选/搜索/异步主路径（§6.1 L1） | **对等** | P0 L1 |
| 尺寸/色 Token（§6.2：触发器 24/32/40 + 树行 24/缩进 24 + 滚动高 256） | **对等** | P0 L2 |
| 勾选回填三策略（`showCheckedStrategy`，`checkable.tsx`） | **映射写实**：`SHOW_CHILD`（默认，仅回填子标签）/ `SHOW_ALL`（回填全部含父）/ `SHOW_PARENT`（父全选仅回填父）；kit `SetShowCheckedStrategy`，见 TSE-S3~S5 | P0 行为 |
| `treeCheckStrictly`（父子不联动） | **映射写实**：`true` → 联动关闭且 `labelInValue` 强制 true（含 halfChecked），见 TSE-S6 | P0 行为 |
| `maxCount` 与回填策略互斥 | **写实**：`SHOW_ALL` 未开 `treeCheckStrictly`、或 `SHOW_PARENT` 时 `maxCount` 无效（API 原文）；kit `SetMaxCount` 同规则拒绝超选 | P0 行为 |
| 搜索不调 `loadData`（FAQ） | **写实**：过滤走 `filterTreeNode`，命中分支自动展开；异步加载只由展开未加载节点触发 | P0 行为 |
| 简单数据（`treeDataSimpleMode`：`{id,pId}` 转树） | **映射**：kit 侧 `SetFieldNames` + 业务预转，P1 建构造糖 | P1 |
| 虚拟滚动（`popupMatchSelectWidth=false` 关虚拟；`virtual=false`） | **分期**：大数据长树 | P1 |
| 自定义渲染（`treeTitleRender` / `tagRender` / `popupRender` / `switcherIcon`） | **分期** | P1 |
| `getPopupContainer`（Modal 内挂载） | **映射**：kit 弹层一律 Portal 就地锚定触发器 | P1 |
| Semantic classNames/styles | kit 语义钩子 | P1 |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 逐像素官网哈希 | **不做** | — |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `value` | 受控选中值（单选 string / 多选 `[]string`；`labelInValue=true` 时为 `TreeSelectValue`），外部优先，内部只回调 |
| `defaultValue` | 非受控初值（单选 string / 多选 `[]string`），仅首次挂载生效 |
| `onChange` | 选中变化回调 `(value, label, extra)`；单选提交后关层，多选 toggle 后常开 |
| `disabled` | 整控件禁用：不打开弹层，不响应选择/搜索/清除 |
| `size` | small / middle / large → 触发器高 24 / 32 / 40（§6.2） |
| `variant` | outlined / filled / borderless / underlined 触发器 chrome（§6.5 矩阵） |
| `status` | error / warning 语义边框，不阻断选择 |
| `open` | 受控弹层显隐（`SetOpen`）；非受控由点击/搜索驱动 |
| `onOpenChange` | 弹层显隐回调 `(open bool)` |
| `treeData` | 树数据源（`TreeSelectNode{Value,Title,Key,Children,…}`，value 全树唯一）；`fieldNames` 可重映射 label/value/children |
| `title` | 节点显示文本来源（`treeNodeLabelProp` 默认 `title`，缺省占位 `---`） |
| `placement` | 弹层四角 bottomLeft（默认）/ bottomRight / topLeft / topRight |
| `allowClear` | 有值时显清除钮；点清除 → 值空 + `onClear` + `onChange` 空值 |
| `showSearch` | 搜索框显隐（单选默认 false，多选默认 true）；按 value/title 过滤，命中分支自动展开，搜索不调 `loadData`（TSE-S7） |
| `multiple` / `treeCheckable` | 多选值数组 toggle 下拉常开；勾选父子默认联动（TSE-S2/S3，`checkable.tsx`） |
| `showCheckedStrategy` | 勾选回填 `SHOW_CHILD`（默认）/ `SHOW_ALL` / `SHOW_PARENT`（TSE-S3~S5，见 §6.7） |
| `treeCheckStrictly` | 父子不联动且 `labelInValue` 强制 true（含 halfChecked，TSE-S6） |
| `labelInValue` | `{value,label,halfChecked}` 回填形态（`treeCheckStrictly` 下强制） |
| `maxCount` | 多选上限；与 `SHOW_ALL`（未开 strictly）/ `SHOW_PARENT` 互斥时无效（见 §6.7） |
| `loadData` / `treeLoadedKeys` / `treeExpandedKeys` | 异步回填 children + 转圈 + `onTreeExpand`（TSE-S7/S8，`async.tsx`；搜索不触发） |
| `onSelect` | 选中回调 `(value, node, extra)` |
| 官方主路径示例 | 基本、多选、从数据直接生成、可勾选、异步加载、线性样式、弹出位置、形态变体 |
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
| 其余示例 | 自定义状态, 最大选中数量, 前后缀, 自定义语义结构的样式和类 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestTreeSelect_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 TreeSelect 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| TSE-01 | L1 | NewTreeSelect 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| TSE-02 | L1 | 选单叶节点 | `value` 单值回填 title + 关层 + `onChange/onSelect` 各一次 |
| TSE-03 | L1 | `treeCheckable` 勾父节点 | 子全选（默认联动）；按 `SHOW_CHILD` 仅回填子标签，`onChange` 为数组 |
| TSE-04 | L1 | 搜索关键字过滤 | 仅命中分支可见并自动展开；搜索不调 `loadData` |
| TSE-05 | L1 | 有值时点清除 | 值空 + `onClear` + `onChange` 空值，弹层关闭 |
| TSE-06 | L1 | `multiple` 点两节点 | `Values()` 长度 2 且下拉保持打开，tag 逐个出现 |
| TSE-07 | L1 | 展开未加载节点（`loadData`） | 先转圈，异步回填 children 后子可见 + `onTreeExpand` 一次 |
| TSE-08 | L1 | `disabled=true` 下点击/键盘 | 不打开弹层，无 `onChange/onOpenChange` |
| TSE-09 | L1 | `size` 三档实测 | 触发器高 small=24 / middle=32 / large=40（±0.5） |
| TSE-10 | L1 | 复现官方示例「基本」（`basic.tsx`） | 单选点叶节点：输入框回填 title 关层，`onChange` 值为该节点 value |
| TSE-11 | L1 | 复现官方示例「多选」（`multiple.tsx`） | 点两节点 values 长度 2 且层不关；再点已选节点则取消选中回到长度 1 |
| TSE-12 | L1 | 复现官方示例「从数据直接生成」（`treeData.tsx`） | `treeData` 直驱渲染三级树：子层相对父左缩进 24/层，行高 24，点叶回填正确 |
| TSE-13 | L1 | 复现官方示例「可勾选」（`checkable.tsx`） | 勾父全选子（默认联动）；全选父时按 `SHOW_CHILD` 仅回填子标签，无父重复 |
| TSE-14 | L1 | 复现官方示例「异步加载」（`async.tsx`） | 展开空子父节点先转圈，`loadData` 回填后子出现可点选，`treeLoadedKeys` 追加该 key |
| TSE-15 | L1 | 复现官方示例「线性样式」（`treeLine.tsx`） | `treeLine=true` 时连接线逐层可见（含叶 14 直径圆），switcher 图标不自动 rotate |
| TSE-16 | L1 | 复现官方示例「弹出位置」（`placement.tsx`） | 切 bottomLeft/bottomRight/topLeft/topRight：弹层锚在触发器对应角，树行可选链路不变 |
| TSE-17 | L1 | 复现官方示例「形态变体」（`variant.tsx`） | 同值切 outlined/filled/borderless/underlined：§6.5 矩阵 chrome 逐一对上，无残留边框 |
| TSE-18 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| TSE-19 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| TSE-20 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| TSE-21 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| TSE-22 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| TSE-23 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| TSE-24 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
type TreeSelectNode struct {
  Value, Title, Key string
  Children []TreeSelectNode
  Disabled, DisableCheckbox, IsLeaf, Selectable bool
}
type TreeSelectValue struct { Value, Label string; HalfChecked bool } // labelInValue=true
type ShowCheckedStrategy int // SHOW_CHILD | SHOW_ALL | SHOW_PARENT

NewTreeSelect(nodes ...TreeSelectNode) *TreeSelect

// 数据
SetTreeData(...TreeSelectNode) / SetFieldNames(label, value, children string)
// 单选 / 多选 / 勾选
SetMultiple(bool) / SetTreeCheckable(bool)
SetShowCheckedStrategy(ShowCheckedStrategy) // 默认 SHOW_CHILD
SetTreeCheckStrictly(bool)                  // true → labelInValue 强制 true
SetLabelInValue(bool)
SetMaxCount(int)
// 值（受控优先）
SetValue(string) / SetValues([]string) / SetLabelValues([]TreeSelectValue)
SetDefaultValue(string) / SetDefaultValues([]string)
Value() string / Values() []string / LabelValues() []TreeSelectValue
// 下拉与搜索
SetOpen(bool) / SetDefaultOpen(bool) / IsOpen()
SetPlacement(bottomLeft|bottomRight|topLeft|topRight)
SetShowSearch(bool) / SetSearchValue(string) / SetFilterTreeNode(func(q string, n TreeSelectNode) bool)
SetTreeExpandedKeys([]string) / SetTreeDefaultExpandedKeys([]string) / SetTreeDefaultExpandAll(bool)
SetTreeExpandAction(click|doubleClick|false)
// 异步
SetLoadData(func(TreeSelectNode)) / SetTreeLoadedKeys([]string)
// 展示
SetDisabled / SetAllowClear / SetSize / SetVariant / SetStatus / SetPlaceholder
SetListHeight(float64) // 默认 256；popupMatchSelectWidth=false 时关虚拟滚动
// 回调
SetOnChange(func(value any, label any)) / SetOnSelect / SetOnSearch / SetOnClear
SetOnOpenChange(func(bool)) / SetOnTreeExpand(func([]string))
// 主题 / a11y / 树
SetTheme(*Theme) / SetFace / SetStyle / SetAriaLabel
Node() core.Node / Popup() core.Node
Focus() / Blur()
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Disabled / Multiple / TreeCheckable / TreeCheckStrictly / LabelInValue | false |
| ShowCheckedStrategy | `SHOW_CHILD` |
| ShowSearch | 单选 false，多选 true |
| Size | middle（高 32）；Variant outlined；Status none；Placement bottomLeft |
| ListHeight | 256；Virtual true；PopupMatchSelectWidth true |
| 受控值/展开 | 未 Set 时用 default* 或内部状态 |
| 其余 | 对齐 antd 6.5 §3 表 |

### 6.11 结构与绘制分层（实现提示）

```text
Column (Wrap，宽=触发器宽)
  ├─ Selector（Decorated，高 24/32/40，variant 见 §6.5）
  │    └─ Flex(Row) CrossStretch gap≈6
  │         prefix? · display value（单选 title / 多选 tags +N）· search input?(showSearch) · clear? · suffix arrow?
  └─ AnchoredPopup (Portal，placement 四角，默认 bottomLeft，min-width=触发器宽)
       └─ Decorated tree panel（圆角 8，内边距 4，滚动高 listHeight=256，阴影 boxShadowSecondary）
            └─ tree rows × N（行高 titleHeight=24，每层左缩进 indentSize=24，行内边 4）
                 ├─ indent · switcher（24 热区，showLine 时不 rotate）· checkbox?(checkable) · title
                 ├─ hover：行底 controlItemBgHover；selected：底 controlItemBgActive
                 ├─ disabled 行：降对比 + 不可选/勾
                 ├─ loading 行：switcher 位转圈（Ticker，随 Host Tick）
                 └─ showLine 连接线：colorBorder，叶 14 直径圆
```

- 组合 `ui/primitive` + `ui/kit`（Tree + Select + 下拉 Portal + 虚拟列表）+ `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **TreeSelect 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/tree-select.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` TreeSelect 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
