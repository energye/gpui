# Table 表格
> 来源：[Ant Design 6.5.x Table](https://ant.design/components/table)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：展示行列数据。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

展示行列数据。

**Table** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本用法 | 复现「基本用法」视觉与布局 |
| JSX 风格的 API | 复现「JSX 风格的 API」视觉与布局 |
| 可选择 | 复现「可选择」视觉与布局 |
| 选择和操作 | 复现「选择和操作」视觉与布局 |
| 自定义选择项 | 自定义渲染/插槽外观 |
| 筛选和排序 | 复现「筛选和排序」视觉与布局 |
| 树型筛选菜单 | 复现「树型筛选菜单」视觉与布局 |
| 自定义筛选的搜索 | 带搜索框外观 |
| 多列排序 | 复现「多列排序」视觉与布局 |
| 可控的筛选和排序 | 复现「可控的筛选和排序」视觉与布局 |
| 自定义筛选菜单 | 自定义渲染/插槽外观 |
| 远程加载数据 | loading 指示与防重复 |
| 紧凑型 | 复现「紧凑型」视觉与布局 |
| 带边框 | bordered 网格线 |
| 可展开 | 展开/折叠指示 |
| 特殊列排序 | 复现「特殊列排序」视觉与布局 |
| 表格行/列合并 | 复现「表格行/列合并」视觉与布局 |
| 树形数据展示 | 复现「树形数据展示」视觉与布局 |
| 固定表头 | 固定头/列/侧栏 |
| 自动高度 | 复现「自动高度」视觉与布局 |
| 固定列 | 固定头/列/侧栏 |
| 堆叠固定列 | 固定头/列/侧栏 |
| 固定头和列 | 固定头/列/侧栏 |
| 隐藏列 | 复现「隐藏列」视觉与布局 |
| 表头分组 | Group 组合外观 |
| 可编辑单元格 | 复现「可编辑单元格」视觉与布局 |
| 可编辑行 | 复现「可编辑行」视觉与布局 |
| 嵌套子表格 | 复现「嵌套子表格」视觉与布局 |
| 拖拽排序 | 复现「拖拽排序」视觉与布局 |
| 列拖拽排序 | 复现「列拖拽排序」视觉与布局 |
| 拖拽手柄列 | 复现「拖拽手柄列」视觉与布局 |
| 单元格自动省略 | 复现「单元格自动省略」视觉与布局 |
| 统一列配置 | 复现「统一列配置」视觉与布局 |
| 自定义单元格省略提示 | 自定义渲染/插槽外观 |
| 自定义空状态 | 空状态插画/文案 |
| 总结栏 | 复现「总结栏」视觉与布局 |
| 虚拟列表 | 复现「虚拟列表」视觉与布局 |
| 响应式 | 断点响应式 |
| 分页设置 | 分页器外观 |
| 随页面滚动的固定表头和滚动条 | 固定头/列/侧栏 |
| 动态控制表格属性 | 复现「动态控制表格属性」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `bordered`

- **说明**：是否展示外边框和列边框
- **类型**：boolean
- **默认值**：false

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `columns`

- **说明**：表格列的配置描述，具体项见下表
- **类型**：[ColumnsType](#column)\[]
- **默认值**：-

#### `footer`

- **说明**：表格尾部
- **类型**：function(currentPageData)
- **默认值**：-

#### `loading`

- **说明**：页面是否加载中
- **类型**：boolean | [Spin Props](/components/spin-cn#api)
- **默认值**：false

#### `showSorterTooltip`

- **说明**：表头是否显示下一次排序的 tooltip 提示。当参数类型为对象时，将被设置为 Tooltip 的属性
- **类型**：boolean | [Tooltip props](/components/tooltip-cn) & `{target?: 'full-header' | 'sorter-icon' }`
- **默认值**：{ target: 'full-header' }
- **版本**：5.16.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `{target?: 'full-header` | 官方取值 `{target?: 'full-header` |
  | `sorter-icon' }` | 官方取值 `sorter-icon' }` |

#### `size`

- **说明**：表格大小
- **类型**：`large` | `medium` | `small`
- **默认值**：`large`
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

#### `title`

- **说明**：表格标题
- **类型**：function(currentPageData)
- **默认值**：-

#### `scrollTo`

- **说明**：滚动到目标位置（设置 `key` 时为 Record 对应的 `rowKey`）。当指定 `offset` 时，表格会滚动至目标行顶部对齐并应用指定的偏移量。`offset` 对 `top` 无效。可选 `align` 参数控制对齐方式：`start` 顶部对齐、`center` 中间对齐、`end` 底部对齐、`nearest` 智能对齐（默认）。虚拟滚动模式下不支持 `center` 对齐
- **类型**：(config: { index?: number, key?: React.Key, top?: number, offset?: number, align?: 'start' | 'center' | 'end' | 'nearest' }) => void
- **默认值**：—
- **版本**：5.11.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `center` | 居中 |
  | `end` | 逻辑结束侧 |
  | `nearest` | 官方取值 `nearest` |

#### `align`

- **说明**：设置列的对齐方式
- **类型**：`left` | `right` | `center`
- **默认值**：`left`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `left` | 左侧 |
  | `right` | 右侧 |
  | `center` | 居中 |

#### `className`

- **说明**：列样式类名
- **类型**：string
- **默认值**：-

#### `ellipsis`

- **说明**：超过宽度将自动省略，暂不支持和排序筛选一起使用。设置为 `true` 或 `{ showTitle?: boolean }` 时，表格布局将变成 `tableLayout="fixed"`。
- **类型**：boolean | { showTitle?: boolean }
- **默认值**：false
- **版本**：showTitle: 4.3.0

#### `filtered`

- **说明**：标识数据是否经过过滤，筛选图标会高亮
- **类型**：boolean
- **默认值**：false

#### `filteredValue`

- **说明**：筛选的受控属性，外界可用此控制列的筛选状态，值为已筛选的 value 数组
- **类型**：string\[]
- **默认值**：-

#### `filterIcon`

- **说明**：自定义 filter 图标。
- **类型**：ReactNode | (filtered: boolean) => ReactNode
- **默认值**：false

#### `responsive`

- **说明**：响应式 breakpoint 配置列表。未设置则始终可见。
- **类型**：[Breakpoint](https://github.com/ant-design/ant-design/blob/015109b42b85c63146371b4e32b883cf97b088e8/components/_util/responsiveObserve.ts#L1)\[]
- **默认值**：-
- **版本**：4.2.0

#### `sortIcon`

- **说明**：自定义 sort 图标
- **类型**：(props: { sortOrder }) => ReactNode
- **默认值**：-
- **版本**：5.6.0

#### `width`

- **说明**：列宽度（[指定了也不生效？](https://github.com/ant-design/ant-design/issues/13825#issuecomment-449889241)）
- **类型**：string | number
- **默认值**：-

#### `placement`

- **说明**：指定分页显示的位置， 取值为`topStart` | `topCenter` | `topEnd` |`bottomStart` | `bottomCenter` | `bottomEnd`| `none`
- **类型**：Array
- **默认值**：\[`bottomEnd`]

#### `position`

- **说明**：指定分页显示的位置， 取值为`topLeft` | `topCenter` | `topRight` |`bottomLeft` | `bottomCenter` | `bottomRight` | `none`，请使用 `placement` 替换
- **类型**：Array
- **默认值**：\[`bottomRight`]

#### `expandIcon`

- **说明**：自定义展开图标，参考[示例](https://codesandbox.io/s/fervent-bird-nuzpr)
- **类型**：function(props): ReactNode
- **默认值**：-

#### `showExpandColumn`

- **说明**：是否显示展开图标列
- **类型**：boolean
- **默认值**：true
- **版本**：4.18.0

#### `onExpand`

- **说明**：点击展开图标时触发
- **类型**：function(expanded, record)
- **默认值**：-

#### `expandedRowOffset`

- **说明**：废弃：展开行的偏移列数，设置后会强制将其前面的列设为固定列。请改用 `Table.EXPAND_COLUMN` 并通过列顺序控制位置
- **类型**：number
- **默认值**：-
- **版本**：5.26.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `Table.EXPAND_COLUMN` | 官方取值 `Table.EXPAND_COLUMN` |

#### `checkStrictly`

- **说明**：checkable 状态下节点选择完全受控（父子数据选中状态不再关联）
- **类型**：boolean
- **默认值**：true
- **版本**：4.4.0

#### `getTitleCheckboxProps`

- **说明**：标题选择框的默认属性配置
- **类型**：function()
- **默认值**：-

#### `type`

- **说明**：多选/单选
- **类型**：`checkbox` | `radio`
- **默认值**：`checkbox`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `checkbox` | 官方取值 `checkbox` |
  | `radio` | 官方取值 `radio` |

#### `text`

- **说明**：选择项显示的文字
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

- 当有大量结构化的数据需要展现时；
- 当需要对数据进行排序、搜索、分页、自定义操作等复杂行为时。

### 2.2 核心功能（按官方示例拆解）

1. **基本用法**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **JSX 风格的 API**（`jsx.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **可选择**（`row-selection.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **选择和操作**（`row-selection-and-operation.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **自定义选择项**（`row-selection-custom.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **筛选和排序**（`head.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **树型筛选菜单**（`filter-in-tree.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **自定义筛选的搜索**（`filter-search.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **多列排序**（`multiple-sorter.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **可控的筛选和排序**（`reset-filter.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **自定义筛选菜单**（`custom-filter-panel.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **远程加载数据**（`ajax.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **紧凑型**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
14. **带边框**（`bordered.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
15. **可展开**（`expand.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
16. **特殊列排序**（`order-column.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
17. **表格行/列合并**（`colspan-rowspan.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
18. **树形数据展示**（`tree-data.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
19. **固定表头**（`fixed-header.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
20. **自动高度**（`auto-height.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
21. **固定列**（`fixed-columns.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
22. **堆叠固定列**（`fixed-gapped-columns.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
23. **固定头和列**（`fixed-columns-header.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
24. **隐藏列**（`hidden-columns.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
25. **表头分组**（`grouping-columns.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
26. **可编辑单元格**（`edit-cell.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
27. **可编辑行**（`edit-row.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
28. **嵌套子表格**（`nested-table.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
29. **拖拽排序**（`drag-sorting.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
30. **列拖拽排序**（`drag-column-sorting.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
31. **拖拽手柄列**（`drag-sorting-handler.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
32. **单元格自动省略**（`ellipsis.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
33. **统一列配置**（`column-defaults.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
34. **自定义单元格省略提示**（`ellipsis-custom-tooltip.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
35. **自定义空状态**（`custom-empty.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
36. **总结栏**（`summary.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
37. **虚拟列表**（`virtual-list.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
38. **响应式**（`responsive.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
39. **分页设置**（`pagination.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
40. **随页面滚动的固定表头和滚动条**（`sticky.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
41. **动态控制表格属性**（`dynamic-settings.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
42. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onChange` | 值变化 | 分页、排序、筛选变化时触发 |
| `onSelect` | 选中 | 用户手动选择/取消选择某行的回调 |
| `loading` | 加载中 | 页面是否加载中 |
| `dataSource` | 数据源 | 数据数组 |
| `columns` | 列配置 | 表格列的配置描述，具体项见下表 |
| `pagination` | 分页 | 分页器，参考[配置项](#pagination)或 [pagination](/components/pagination-cn) 文档，设为 false 时不展示和进行分页 |
| `rowSelection` | 行选择 | 表格行是否可选择，[配置项](#rowselection) |
| `expandable` | 展开行 | 配置展开属性 |
| `virtual` | 虚拟滚动 | 支持虚拟列表 |
| `getPopupContainer` | 浮层容器 | 设置表格内各类浮层的渲染节点，如筛选菜单 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本用法 | `basic.tsx` | 否 |
| JSX 风格的 API | `jsx.tsx` | 否 |
| 可选择 | `row-selection.tsx` | 否 |
| 选择和操作 | `row-selection-and-operation.tsx` | 否 |
| 自定义选择项 | `row-selection-custom.tsx` | 否 |
| 选择性能 | `row-selection-debug.tsx` | 是 |
| 筛选和排序 | `head.tsx` | 否 |
| 树型筛选菜单 | `filter-in-tree.tsx` | 否 |
| 自定义筛选的搜索 | `filter-search.tsx` | 否 |
| 多列排序 | `multiple-sorter.tsx` | 否 |
| 可控的筛选和排序 | `reset-filter.tsx` | 否 |
| 自定义筛选菜单 | `custom-filter-panel.tsx` | 否 |
| 远程加载数据 | `ajax.tsx` | 否 |
| 紧凑型 | `size.tsx` | 否 |
| 紧凑型 | `narrow.tsx` | 是 |
| 带边框 | `bordered.tsx` | 否 |
| 可展开 | `expand.tsx` | 否 |
| 可自定义展开位置 | `expand-sticky.tsx` | 是 |
| 特殊列排序 | `order-column.tsx` | 否 |
| 表格行/列合并 | `colspan-rowspan.tsx` | 否 |
| 树形数据展示 | `tree-data.tsx` | 否 |
| 树形数据省略情况测试 | `tree-table-ellipsis.tsx` | 是 |
| 树形数据保留key测试 | `tree-table-preserveSelectedRowKeys.tsx` | 是 |
| 固定表头 | `fixed-header.tsx` | 否 |
| 自动高度 | `auto-height.tsx` | 否 |
| 固定列 | `fixed-columns.tsx` | 否 |
| 堆叠固定列 | `fixed-gapped-columns.tsx` | 否 |
| 固定头和列 | `fixed-columns-header.tsx` | 否 |
| 隐藏列 | `hidden-columns.tsx` | 否 |
| 表头分组 | `grouping-columns.tsx` | 否 |
| 可编辑单元格 | `edit-cell.tsx` | 否 |
| 可编辑行 | `edit-row.tsx` | 否 |
| 嵌套子表格 | `nested-table.tsx` | 否 |
| 拖拽排序 | `drag-sorting.tsx` | 否 |
| 列拖拽排序 | `drag-column-sorting.tsx` | 否 |
| 拖拽手柄列 | `drag-sorting-handler.tsx` | 否 |
| 单元格自动省略 | `ellipsis.tsx` | 否 |
| 统一列配置 | `column-defaults.tsx` | 否 |
| 自定义单元格省略提示 | `ellipsis-custom-tooltip.tsx` | 否 |
| 自定义空状态 | `custom-empty.tsx` | 否 |
| 总结栏 | `summary.tsx` | 否 |
| 虚拟列表 | `virtual-list.tsx` | 否 |
| 响应式 | `responsive.tsx` | 否 |
| 嵌套带边框的表格 Debug | `nest-table-border-debug.tsx` | 是 |
| Tabs 中的嵌套表格 Debug | `nested-table-in-tabs-debug.tsx` | 是 |
| 分页设置 | `pagination.tsx` | 否 |
| 自定义选择项组 | `row-selection-custom-debug.tsx` | 是 |
| 随页面滚动的固定表头和滚动条 | `sticky.tsx` | 否 |
| 动态控制表格属性 | `dynamic-settings.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 带下拉箭头的表头 | `selections-debug.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |
| measureRowRender | `measure-row-render.tsx` | 是 |

### 2.6 FAQ

## FAQ

### 如何在没有数据或只有一页数据时隐藏分页栏 {#faq-hide-pagination}

你可以设置 `pagination` 的 `hideOnSinglePage` 属性为 `true`。

### 表格过滤时会回到第一页？ {#faq-filter-to-first-page}

前端过滤时通常条目总数会减少，从而导致总页数小于筛选前的当前页数，为了防止当前页面没有数据，我们默认会返回第一页。

如果你在使用远程分页，很可能需要保持当前页面，你可以参照这个 [受控例子](https://codesandbox.io/s/yuanchengjiazaishuju-ant-design-demo-7y2uf) 控制当前页面不变。

### 表格分页为何会出现 size 切换器？ {#faq-size-changer}

自 `4.1.0` 起，Pagination 在 `total` 大于 50 条时会默认显示 size 切换器以提升用户交互体验。如果你不需要该功能，可以通过设置 `showSizeChanger` 为 `false` 来关闭。

### 为什么 更新 state 会导致全表渲染？ {#faq-state-update-rerender}

由于 `columns` 支持 `render` 方法，因而 Table 无法知道哪些单元会受到影响。你可以通过 `column.shouldCellUpdate` 来控制单元格的渲染。

### 如何排查 Table 性能问题？ {#faq-table-performance}

React DevTools 在分析复杂表格时可能带来额外开销，尤其是行列数量较多的场景。若你遇到明显卡顿，建议先关闭 React DevTools，或在干净的浏览器环境中重新测试。如果在正常运行环境下仍能稳定复现性能问题，欢迎提供最小复现以便我们继续排查。

### 固定列穿透到最上层该怎么办？ {#faq-fixed-column-zindex}

固定列通过 `z-index` 属性将其悬浮于非固定列之上，这使得有时候你会发现在 Table 上放置遮罩层时固定列会被透过的情况。为遮罩层设置更高的 `z-index` 覆盖住固定列即可。

### 如何自定义渲染可选列的勾选框（比如增加 Tooltip）？ {#faq-custom-checkbox-render}

自 `4.1.0` 起，可以通过 [rowSelection](https://ant.design/components/table-cn/#rowselection) 的 `renderCell` 属性控制，可以参考此处 [Demo](https://codesandbox.io/s/table-row-tooltip-v79j2v) 实现展示 Tooltip 需求或其他自定义的需求。

### 为什么 components.body.wrapper 或 components.body.row 在 virtual 开启时会报错？ {#faq-virtual-wrapper-ref}

因为虚拟表格需要获取其 ref 做一些计算，所以你需要使用 `React.forwardRef` 包裹并传递 ref 到 dom。如以下代码：

```tsx
const EditableRow = React.forwardRef(
  ({ index, ...props }, ref) => {
    const [form] = Form.useForm();
    return (
      
        
          
        
      
    );
  },
);
```

对于固定行高纵向滚动的场景，可以使用以下方法：

```tsx

```

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

### Table

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| bordered | 是否展示外边框和列边框 | boolean | false | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | column | 统一列配置，仅在单列未声明同名属性时生效 | Partial<[ColumnType](#column)> | - | 6.4.0 | × |
| columns | 表格列的配置描述，具体项见下表 | [ColumnsType](#column)\[] | - | components | 覆盖默认的 table 元素 | [TableComponents](https://github.com/react-component/table/blob/75ee0064e54a4b3215694505870c9d6c817e9e4a/src/interface.ts#L129) | - | dataSource | 数据数组 | object\[] | - | expandable | 配置展开属性 | [expandable](#expandable) | - | footer | 表格尾部 | function(currentPageData) | - | getPopupContainer | 设置表格内各类浮层的渲染节点，如筛选菜单 | (triggerNode) => HTMLElement | () => TableHtmlElement | loading | 页面是否加载中 | boolean \| [Spin Props](/components/spin-cn#api) | false | locale | 默认文案设置，目前包括排序、过滤、空数据文案 | object | [默认值](https://github.com/ant-design/ant-design/blob/6dae4a7e18ad1ba193aedd5ab6867e1d823e2aa4/components/locale/zh_CN.tsx#L20-L37) | pagination | 分页器，参考[配置项](#pagination)或 [pagination](/components/pagination-cn) 文档，设为 false 时不展示和进行分页 | object \| `false` | - | rowClassName | 表格行的类名 | function(record, index): string | - | rowKey | 表格行 key 的取值，可以是字符串或一个函数 | string \| function(record): string | `key` | rowSelection | 表格行是否可选择，[配置项](#rowselection) | object | - | rowHoverable | 表格行是否开启 hover 交互 | boolean | true | 5.16.0 | × |
| scroll | 表格是否可滚动，也可以指定滚动区域的宽、高，[配置项](#scroll) | object | - | showHeader | 是否显示表头 | boolean | true | showSorterTooltip | 表头是否显示下一次排序的 tooltip 提示。当参数类型为对象时，将被设置为 Tooltip 的属性 | boolean \| [Tooltip props](/components/tooltip-cn) & `{target?: 'full-header' \| 'sorter-icon' }` | { target: 'full-header' } | 5.16.0 | × |
| size | 表格大小 | `large` \| `medium` \| `small` | `large` | sortDirections | 支持的排序方式，取值为 `ascend` `descend` | Array | \[`ascend`, `descend`] | sticky | 设置粘性头部和滚动条 | boolean \| `{offsetHeader?: number, offsetScroll?: number, getContainer?: () => HTMLElement}` | - | 4.6.0 (getContainer: 4.7.0) | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | summary | 总结栏 | (currentData) => ReactNode | - | tableLayout | 表格元素的 [table-layout](https://developer.mozilla.org/zh-CN/docs/Web/CSS/table-layout) 属性，设为 `fixed` 表示内容不会影响列的布局 | - \| `auto` \| `fixed` | 无<hr />固定表头/列或使用了 `column.ellipsis` 时，默认值为 `fixed` | title | 表格标题 | function(currentPageData) | - | virtual | 支持虚拟列表 | boolean | - | 5.9.0 | × |
| onChange | 分页、排序、筛选变化时触发 | function(pagination, filters, sorter, extra: { currentDataSource: \[], action: `paginate` \| `sort` \| `filter` }) | - | onHeaderRow | 设置头部行属性 | function(columns, index) | - | onRow | 设置行属性 | function(record, index) | - | onScroll | 表单内容滚动时触发（虚拟滚动下只有垂直滚动会触发事件） | function(event) | - | 5.16.0 | × |

### Table ref

| 参数 | 说明 | 类型 | 版本 |
| --- | --- | --- | --- |
| nativeElement | 最外层 div 元素 | HTMLDivElement | 5.11.0 |
| scrollTo | 滚动到目标位置（设置 `key` 时为 Record 对应的 `rowKey`）。当指定 `offset` 时，表格会滚动至目标行顶部对齐并应用指定的偏移量。`offset` 对 `top` 无效。可选 `align` 参数控制对齐方式：`start` 顶部对齐、`center` 中间对齐、`end` 底部对齐、`nearest` 智能对齐（默认）。虚拟滚动模式下不支持 `center` 对齐 | (config: { index?: number, key?: React.Key, top?: number, offset?: number, align?: 'start' \| 'center' \| 'end' \| 'nearest' }) => void | 5.11.0 |

#### onRow 用法

适用于 `onRow` `onHeaderRow` `onCell` `onHeaderCell`。

```jsx
<Table
  onRow={(record) => {
    return {
      onClick: (event) => {}, // 点击行
      onDoubleClick: (event) => {},
      onContextMenu: (event) => {},
      onMouseEnter: (event) => {}, // 鼠标移入行
      onMouseLeave: (event) => {},
    };
  }}
  onHeaderRow={(columns, index) => {
    return {
      onClick: () => {}, // 点击表头行
    };
  }}
/>
```

### Column

列描述数据对象，是 columns 中的一项，Column 使用相同的 API。

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| align | 设置列的对齐方式 | `left` \| `right` \| `center` | `left` | colSpan | 表头列合并，设置为 0 时，不渲染 | number | - | defaultFilteredValue | 默认筛选值 | string\[] | - | defaultSortOrder | 默认排序顺序 | `ascend` \| `descend` | - | filterDropdown | 可以自定义筛选菜单，此函数只负责渲染图层，需要自行编写各种交互 | ReactNode \| (props: [FilterDropdownProps](https://github.com/ant-design/ant-design/blob/ecc54dda839619e921c0ace530408871f0281c2a/components/table/interface.tsx#L79)) => ReactNode | - | filteredValue | 筛选的受控属性，外界可用此控制列的筛选状态，值为已筛选的 value 数组 | string\[] | - | filterOnClose | 是否在筛选菜单关闭时触发筛选 | boolean | true | 5.15.0 |
| filterMultiple | 是否多选 | boolean | true | filterSearch | 筛选菜单项是否可搜索 | boolean \| function(input, record):boolean | false | boolean:4.17.0 function:4.19.0 |
| filters | 表头的筛选菜单项 | object\[] | - | fixed | （IE 下无效）列是否固定，可选 `true` (等效于 `'start'`) `'start'` `'end'` | boolean \| string | false | render | 生成复杂数据的渲染函数，参数分别为当前单元格的值，当前行数据，行索引 | (value: V, record: T, index: number): ReactNode | - | rowScope | 设置列范围 | `row` \| `rowgroup` | - | 5.1.0 |
| shouldCellUpdate | 自定义单元格渲染时机 | (record, prevRecord) => boolean | - | 4.3.0 |
| showSorterTooltip | 表头显示下一次排序的 tooltip 提示, 覆盖 table 中 `showSorterTooltip` | boolean \| [Tooltip props](/components/tooltip-cn/#api) & `{target?: 'full-header' \| 'sorter-icon' }` | { target: 'full-header' } | 5.16.0 |
| sortDirections | 支持的排序方式，覆盖 `Table` 中 `sortDirections`， 取值为 `ascend` `descend` | Array | \[`ascend`, `descend`] | sortOrder | 排序的受控属性，外界可用此控制列的排序，可设置为 `ascend` `descend` `null` | `ascend` \| `descend` \| null | - | title | 列头显示文字（函数用法 `3.10.0` 后支持） | ReactNode \| ({ sortColumns, filters }) => ReactNode | - | minWidth | 最小列宽度，只在 `tableLayout="auto"` 时有效 | number | - | 5.21.0 |
| hidden | 隐藏列 | boolean | false | 5.13.0 |
| onCell | 设置单元格属性 | function(record, rowIndex) | - | onHeaderCell | 设置头部单元格属性 | function(column) | - 
| 参数  | 说明         | 类型      | 默认值 |
| ----- | ------------ | --------- | ------ |
| title | 列头显示文字 | ReactNode | -      |

### pagination

分页的配置项。

| 参数 | 说明 | 类型 | 默认值 |
| --- | --- | --- | --- |
| placement | 指定分页显示的位置， 取值为`topStart` \| `topCenter` \| `topEnd` \|`bottomStart` \| `bottomCenter` \| `bottomEnd`\| `none` | Array | \[`bottomEnd`] |
| ~~position~~ | 指定分页显示的位置， 取值为`topLeft` \| `topCenter` \| `topRight` \|`bottomLeft` \| `bottomCenter` \| `bottomRight` \| `none`，请使用 `placement` 替换 | Array | \[`bottomRight`] |

更多配置项，请查看 [`Pagination`](/components/pagination-cn)。

### expandable

展开功能的配置。

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| childrenColumnName | 指定树形结构的列名 | string | children | columnWidth | 自定义展开列宽度 | string \| number | - | defaultExpandedRowKeys | 默认展开的行 | string\[] | - | expandedRowKeys | 展开的行，控制属性 | string\[] | - | expandIcon | 自定义展开图标，参考[示例](https://codesandbox.io/s/fervent-bird-nuzpr) | function(props): ReactNode | - | fixed | 控制展开图标是否固定，可选 `true` `'left'` `'right'` | boolean \| string | false | 4.16.0 |
| indentSize | 展示树形数据时，每层缩进的宽度，以 px 为单位 | number | 15 | showExpandColumn | 是否显示展开图标列 | boolean | true | 4.18.0 |
| onExpand | 点击展开图标时触发 | function(expanded, record) | - | ~~expandedRowOffset~~ | 废弃：展开行的偏移列数，设置后会强制将其前面的列设为固定列。请改用 `Table.EXPAND_COLUMN` 并通过列顺序控制位置 | number | - | 5.26.0 |

### rowSelection

选择功能的配置。

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| align | 设置选择列的对齐方式 | `left` \| `right` \| `center` | `left` | 5.25.0 |
| checkStrictly | checkable 状态下节点选择完全受控（父子数据选中状态不再关联） | boolean | true | 4.4.0 |
| columnTitle | 自定义列表选择框标题 | ReactNode \| (originalNode: ReactNode) => ReactNode | - | fixed | 把选择框列固定在左边 | boolean | - | getTitleCheckboxProps | 标题选择框的默认属性配置 | function() | - | preserveSelectedRowKeys | 当数据被删除时仍然保留选项的 `key` | boolean | - | 4.4.0 |
| renderCell | 渲染勾选框，用法与 Column 的 `render` 相同 | (checked: boolean, record: T, index: number, originNode: ReactNode): ReactNode | - | 4.1.0 |
| selectedRowKeys | 指定选中项的 key 数组，需要和 onChange 进行配合 | string\[] \| number\[] | \[] | selections | 自定义选择项 [配置项](#selection), 设为 `true` 时使用默认选择项 | object\[] \| boolean | true | onCell | 设置单元格属性，用法与 Column 的 `onCell` 相同 | function(record, rowIndex) | - | 5.5.0 |
| onChange | 选中项发生变化时的回调 | function(selectedRowKeys, selectedRows, info: { type }) | - | `info.type`: 4.21.0 |
| onSelect | 用户手动选择/取消选择某行的回调 | function(record, selected, selectedRows, nativeEvent) | - 
| 参数 | 说明 | 类型 | 默认值 |
| --- | --- | --- | --- |
| scrollToFirstRowOnChange | 当分页、排序、筛选变化后是否滚动到表格顶部 | boolean | - |
| x | 设置横向滚动，也可用于指定滚动区域的宽，可以设置为像素值，百分比，`true` 和 ['max-content'](https://developer.mozilla.org/zh-CN/docs/Web/CSS/width#max-content) | string \| number \| true | - |
| y | 设置纵向滚动，也可用于指定滚动区域的高，可以设置为像素值 | string \| number | - |

### selection

| 参数     | 说明                       | 类型                        | 默认值 |
| -------- | -------------------------- | --------------------------- | ------ |
| key      | React 需要的 key，建议设置 | string                      | -      |
| text     | 选择项显示的文字           | ReactNode                   | -      |
| onSelect | 选择项点击回调             | function(changeableRowKeys) | -      |

## 在 TypeScript 中使用 {#using-in-typescript}

```tsx
import React from 'react';
import { Table } from 'antd';
import type { TableColumnsType } from 'antd';

interface User {
  key: number;
  name: string;
}

const columns: TableColumnsType<User> = [
  {
    key: 'name',
    title: 'Name',
    dataIndex: 'name',
  },
];

const data: User[] = [
  {
    key: 0,
    name: 'Jack',
  },
];

const Demo: React.FC = () => (
  <>
    <Table<User> columns={columns} dataSource={data} />
    {/* 使用 JSX 风格的 API */}
    <Table<User> dataSource={data}>
      <Table.Column<User> key="name" title="Name" dataIndex="name" />
    </Table>
  </>
);

export default Demo;
```

TypeScript 里使用 Table 的 [CodeSandbox 实例](https://codesandbox.io/s/serene-platform-0jo5t)。

### 导入方式

```js
import { Table } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `bordered` | 是否展示外边框和列边框 | boolean | false | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `column` | 统一列配置，仅在单列未声明同名属性时生效 | Partial | - | 6.4.0 |
| `columns` | 表格列的配置描述，具体项见下表 | [ColumnsType](#column)\[] | - | — |
| `components` | 覆盖默认的 table 元素 | [TableComponents](https://github.com/react-component/table/blob/75ee0064e54a4b3215694505870c9d6c817e9e4a/src/interface.ts#L129) | - | — |
| `dataSource` | 数据数组 | object\[] | - | — |
| `expandable` | 配置展开属性 | [expandable](#expandable) | - | — |
| `footer` | 表格尾部 | function(currentPageData) | - | — |
| `getPopupContainer` | 设置表格内各类浮层的渲染节点，如筛选菜单 | (triggerNode) => HTMLElement | () => TableHtmlElement | — |
| `loading` | 页面是否加载中 | boolean \| [Spin Props](/components/spin-cn#api) | false | — |
| `locale` | 默认文案设置，目前包括排序、过滤、空数据文案 | object | [默认值](https://github.com/ant-design/ant-design/blob/6dae4a7e18ad1ba193aedd5ab6867e1d823e2aa4/components/locale/zh_CN.tsx#L20-L37) | — |
| `pagination` | 分页器，参考[配置项](#pagination)或 [pagination](/components/pagination-cn) 文档，设为 false 时不展示和进行分页 | object \| `false` | - | — |
| `rowClassName` | 表格行的类名 | function(record, index): string | - | — |
| `rowKey` | 表格行 key 的取值，可以是字符串或一个函数 | string \| function(record): string | `key` | — |
| `rowSelection` | 表格行是否可选择，[配置项](#rowselection) | object | - | — |
| `rowHoverable` | 表格行是否开启 hover 交互 | boolean | true | 5.16.0 |
| `scroll` | 表格是否可滚动，也可以指定滚动区域的宽、高，[配置项](#scroll) | object | - | — |
| `showHeader` | 是否显示表头 | boolean | true | — |
| `showSorterTooltip` | 表头是否显示下一次排序的 tooltip 提示。当参数类型为对象时，将被设置为 Tooltip 的属性 | boolean \| [Tooltip props](/components/tooltip-cn) & `{target?: 'full-header' \| 'sorter-icon' }` | { target: 'full-header' } | 5.16.0 |
| `size` | 表格大小 | `large` \| `medium` \| `small` | `large` | — |
| `sortDirections` | 支持的排序方式，取值为 `ascend` `descend` | Array | \[`ascend`, `descend`] | — |
| `sticky` | 设置粘性头部和滚动条 | boolean \| `{offsetHeader?: number, offsetScroll?: number, getContainer?: () => HTMLElement}` | - | 4.6.0 (getContainer: 4.7.0) |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `summary` | 总结栏 | (currentData) => ReactNode | - | — |
| `tableLayout` | 表格元素的 [table-layout](https://developer.mozilla.org/zh-CN/docs/Web/CSS/table-layout) 属性，设为 `fixed` 表示内容不会影响列的布局 | - \| `auto` \| `fixed` | 无固定表头/列或使用了 `column.ellipsis` 时，默认值为 `fixed` | — |
| `title` | 表格标题 | function(currentPageData) | - | — |
| `virtual` | 支持虚拟列表 | boolean | - | 5.9.0 |
| `onChange` | 分页、排序、筛选变化时触发 | function(pagination, filters, sorter, extra: { currentDataSource: \[], action: `paginate` \| `sort` \| `filter` }) | - | — |
| `onHeaderRow` | 设置头部行属性 | function(columns, index) | - | — |
| `onRow` | 设置行属性 | function(record, index) | - | — |
| `onScroll` | 表单内容滚动时触发（虚拟滚动下只有垂直滚动会触发事件） | function(event) | - | 5.16.0 |
| `nativeElement` | 最外层 div 元素 | HTMLDivElement | — | 5.11.0 |
| `scrollTo` | 滚动到目标位置（设置 `key` 时为 Record 对应的 `rowKey`）。当指定 `offset` 时，表格会滚动至目标行顶部对齐并应用指定的偏移量。`offset` 对 `top` 无效。可选 `align` 参数控制对齐方式：`start` 顶部对齐、`center` 中间对齐、`end` 底部对齐、`nearest` 智能对齐（默认）。虚拟滚动模式下不支持 `center` 对齐 | (config: { index?: number, key?: React.Key, top?: number, offset?: number, align?: 'start' \| 'center' \| 'end' \| 'nearest' }) => void | — | 5.11.0 |
| `align` | 设置列的对齐方式 | `left` \| `right` \| `center` | `left` | — |
| `className` | 列样式类名 | string | - | — |
| `colSpan` | 表头列合并，设置为 0 时，不渲染 | number | - | — |
| `dataIndex` | 列数据在数据项中对应的路径，支持通过数组查询嵌套路径 | string \| string\[] | - | — |
| `defaultFilteredValue` | 默认筛选值 | string\[] | - | — |
| `filterResetToDefaultFilteredValue` | 点击重置按钮的时候，是否恢复默认筛选值 | boolean | false | — |
| `defaultSortOrder` | 默认排序顺序 | `ascend` \| `descend` | - | — |
| `ellipsis` | 超过宽度将自动省略，暂不支持和排序筛选一起使用。设置为 `true` 或 `{ showTitle?: boolean }` 时，表格布局将变成 `tableLayout="fixed"`。 | boolean \| { showTitle?: boolean } | false | showTitle: 4.3.0 |
| `filterDropdown` | 可以自定义筛选菜单，此函数只负责渲染图层，需要自行编写各种交互 | ReactNode \| (props: [FilterDropdownProps](https://github.com/ant-design/ant-design/blob/ecc54dda839619e921c0ace530408871f0281c2a/components/table/interface.tsx#L79)) => ReactNode | - | — |
| `filtered` | 标识数据是否经过过滤，筛选图标会高亮 | boolean | false | — |
| `filteredValue` | 筛选的受控属性，外界可用此控制列的筛选状态，值为已筛选的 value 数组 | string\[] | - | — |
| `filterIcon` | 自定义 filter 图标。 | ReactNode \| (filtered: boolean) => ReactNode | false | — |
| `filterOnClose` | 是否在筛选菜单关闭时触发筛选 | boolean | true | 5.15.0 |
| `filterMultiple` | 是否多选 | boolean | true | — |
| `filterMode` | 指定筛选菜单的用户界面 | 'menu' \| 'tree' | 'menu' | 4.17.0 |
| `filterSearch` | 筛选菜单项是否可搜索 | boolean \| function(input, record):boolean | false | boolean:4.17.0 function:4.19.0 |
| `filters` | 表头的筛选菜单项 | object\[] | - | — |
| `filterDropdownProps` | 自定义下拉属性，在 `<5.22.0` 之前可用 `filterDropdownOpen` 和 `onFilterDropdownOpenChange` | [DropdownProps](/components/dropdown#api) | - | 5.22.0 |
| `fixed` | （IE 下无效）列是否固定，可选 `true` (等效于 `'start'`) `'start'` `'end'` | boolean \| string | false | — |
| `key` | React 需要的 key，如果已经设置了唯一的 `dataIndex`，可以忽略这个属性 | string | - | — |
| `render` | 生成复杂数据的渲染函数，参数分别为当前单元格的值，当前行数据，行索引 | (value: V, record: T, index: number): ReactNode | - | — |
| `responsive` | 响应式 breakpoint 配置列表。未设置则始终可见。 | [Breakpoint](https://github.com/ant-design/ant-design/blob/015109b42b85c63146371b4e32b883cf97b088e8/components/_util/responsiveObserve.ts#L1)\[] | - | 4.2.0 |
| `rowScope` | 设置列范围 | `row` \| `rowgroup` | - | 5.1.0 |
| `shouldCellUpdate` | 自定义单元格渲染时机 | (record, prevRecord) => boolean | - | 4.3.0 |
| `sorter` | 排序函数，本地排序使用一个函数(参考 [Array.sort](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Array/sort) 的 compareFunction)。需要服务端排序可设为 `true`（单列排序） 或 `{ multiple: number }`（多列排序） | function \| boolean \| { compare: function, multiple: number } | - | — |
| `sortOrder` | 排序的受控属性，外界可用此控制列的排序，可设置为 `ascend` `descend` `null` | `ascend` \| `descend` \| null | - | — |
| `sortIcon` | 自定义 sort 图标 | (props: { sortOrder }) => ReactNode | - | 5.6.0 |
| `width` | 列宽度（[指定了也不生效？](https://github.com/ant-design/ant-design/issues/13825#issuecomment-449889241)） | string \| number | - | — |
| `minWidth` | 最小列宽度，只在 `tableLayout="auto"` 时有效 | number | - | 5.21.0 |
| `hidden` | 隐藏列 | boolean | false | 5.13.0 |
| `onCell` | 设置单元格属性 | function(record, rowIndex) | - | — |
| `onFilter` | 本地模式下，确定筛选的运行函数 | function | - | — |
| `onHeaderCell` | 设置头部单元格属性 | function(column) | - | — |
| `placement` | 指定分页显示的位置， 取值为`topStart` \| `topCenter` \| `topEnd` \|`bottomStart` \| `bottomCenter` \| `bottomEnd`\| `none` | Array | \[`bottomEnd`] | — |
| `position` | 指定分页显示的位置， 取值为`topLeft` \| `topCenter` \| `topRight` \|`bottomLeft` \| `bottomCenter` \| `bottomRight` \| `none`，请使用 `placement` 替换 | Array | \[`bottomRight`] | — |
| `childrenColumnName` | 指定树形结构的列名 | string | children | — |
| `columnTitle` | 自定义展开列表头 | ReactNode | - | 4.23.0 |
| `columnWidth` | 自定义展开列宽度 | string \| number | - | — |
| `defaultExpandAllRows` | 初始时，是否展开所有行 | boolean | false | — |
| `defaultExpandedRowKeys` | 默认展开的行 | string\[] | - | — |
| `expandedRowClassName` | 展开行的 className | string \| (record, index, indent) => string | - | string: 5.22.0 |
| `expandedRowKeys` | 展开的行，控制属性 | string\[] | - | — |
| `expandedRowRender` | 额外的展开行 | function(record, index, indent, expanded): ReactNode | - | — |
| `expandIcon` | 自定义展开图标，参考[示例](https://codesandbox.io/s/fervent-bird-nuzpr) | function(props): ReactNode | - | — |
| `expandRowByClick` | 通过点击行来展开子行 | boolean | false | — |
| `indentSize` | 展示树形数据时，每层缩进的宽度，以 px 为单位 | number | 15 | — |
| `rowExpandable` | 设置是否允许行展开（`dataSource` 若存在 `children` 字段将不生效） | (record) => boolean | - | — |
| `showExpandColumn` | 是否显示展开图标列 | boolean | true | 4.18.0 |
| `onExpand` | 点击展开图标时触发 | function(expanded, record) | - | — |
| `onExpandedRowsChange` | 展开的行变化时触发 | function(expandedRows) | - | — |
| `expandedRowOffset` | 废弃：展开行的偏移列数，设置后会强制将其前面的列设为固定列。请改用 `Table.EXPAND_COLUMN` 并通过列顺序控制位置 | number | - | 5.26.0 |
| `checkStrictly` | checkable 状态下节点选择完全受控（父子数据选中状态不再关联） | boolean | true | 4.4.0 |
| `getCheckboxProps` | 选择框的默认属性配置 | function(record) | - | — |
| `getTitleCheckboxProps` | 标题选择框的默认属性配置 | function() | - | — |
| `hideSelectAll` | 隐藏全选勾选框与自定义选择项 | boolean | false | 4.3.0 |
| `preserveSelectedRowKeys` | 当数据被删除时仍然保留选项的 `key` | boolean | - | 4.4.0 |
| `renderCell` | 渲染勾选框，用法与 Column 的 `render` 相同 | (checked: boolean, record: T, index: number, originNode: ReactNode): ReactNode | - | 4.1.0 |
| `selectedRowKeys` | 指定选中项的 key 数组，需要和 onChange 进行配合 | string\[] \| number\[] | \[] | — |
| `defaultSelectedRowKeys` | 默认选中项的 key 数组 | string\[] \| number\[] | \[] | — |
| `selections` | 自定义选择项 [配置项](#selection), 设为 `true` 时使用默认选择项 | object\[] \| boolean | true | — |
| `type` | 多选/单选 | `checkbox` \| `radio` | `checkbox` | — |
| `onSelect` | 用户手动选择/取消选择某行的回调 | function(record, selected, selectedRows, nativeEvent) | - | — |
| `scrollToFirstRowOnChange` | 当分页、排序、筛选变化后是否滚动到表格顶部 | boolean | - | — |
| `x` | 设置横向滚动，也可用于指定滚动区域的宽，可以设置为像素值，百分比，`true` 和 ['max-content'](https://developer.mozilla.org/zh-CN/docs/Web/CSS/width#max-content) | string \| number \| true | - | — |
| `y` | 设置纵向滚动，也可用于指定滚动区域的高，可以设置为像素值 | string \| number | - | — |
| `text` | 选择项显示的文字 | ReactNode | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Table** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **42** 个，均需可复现。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/table
- 中文文档：https://ant.design/components/table-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/table
- 驱动 gpui kit：`table`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Table** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/table/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Table）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 数据渲染与选择/展开/分页/加载主路径 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Table）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：展示行列数据。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 行高 middle 约 | **55** | 实现按 size/token |
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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据展示**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `bordered` | 是否展示外边框和列边框 | boolean | false |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `column` | 统一列配置，仅在单列未声明同名属性时生效 | Partial<[ColumnType](#column)> | - |
| `columns` | 表格列的配置描述，具体项见下表 | [ColumnsType](#column)\[] | - |
| `components` | 覆盖默认的 table 元素 | [TableComponents](https://github.com/… | - |
| `dataSource` | 数据数组 | object\[] | - |
| `expandable` | 配置展开属性 | [expandable](#expandable) | - |
| `footer` | 表格尾部 | function(currentPageData) | - |
| `getPopupContainer` | 设置表格内各类浮层的渲染节点，如筛选菜单 | (triggerNode) => HTMLElement | () => TableHtmlElement |
| `loading` | 页面是否加载中 | boolean \ | [Spin Props](/components/spin-cn#api) |
| `locale` | 默认文案设置，目前包括排序、过滤、空数据文案 | object | [默认值](https://github.com/ant-design/ant-design/blob/6dae4a7e18ad1ba193aedd5ab6867e1d823e2aa4/components/locale/zh_CN.tsx#L20-L37) |
| `pagination` | 分页器，参考[配置项](#pagination)或 [pagination](/components/pagina… | object \ | `false` |
| `rowClassName` | 表格行的类名 | function(record, index): string | - |
| `rowKey` | 表格行 key 的取值，可以是字符串或一个函数 | string \ | function(record): string |
| `rowSelection` | 表格行是否可选择，[配置项](#rowselection) | object | - |
| `rowHoverable` | 表格行是否开启 hover 交互 | boolean | true |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
mount ──► 渲染 columns × dataSource
             ├── pagination 翻页 ──► onChange(pagination, filters, sorter)
             ├── 列头排序 ──► sorter 切换 ascend/descend/none ──► onChange
             ├── 列筛选 ──► filters ──► onChange
             ├── rowSelection 点行/全选 ──► selectedRowKeys + onChange
             ├── expandable 展开 ──► 展开行渲染
             ├── loading=true ──► 表体遮罩，列头可保留
             ├── dataSource=[] ──► Empty
             └── scroll.y / fixed ──► 表体滚动、列钉住
```

\*服务端排序/筛选时：受控 pagination/filters/sorter 由外部驱动。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| TBL-S1 | 渲染 3 列×2 行 | 可见 2 行数据单元格 |
| TBL-S2 | 翻到第 2 页 | `onChange` 带 pagination.current=2 |
| TBL-S3 | 点可排序列头 | 排序态变化并 `onChange` |
| TBL-S4 | 行选中 | `selectedRowKeys` 含该 rowKey |
| TBL-S5 | 全选 | 当前页（或跨页策略按实现）keys 全选 |
| TBL-S6 | `loading=true` | 表体 loading；不丢列头 |
| TBL-S7 | `dataSource=[]` | 显示 Empty |
| TBL-S8 | 展开行 | expanded 内容可见 |
| TBL-S9 | `scroll.y` 超高 | 表体滚动、表头可固定 |
| TBL-S10 | fixed 左列 + 横滚 | 左列钉住 |
| TBL-S11 | 筛选确认 | `onChange` filters 有值 |
| TBL-S12 | rowKey 缺失 | 开发警告或不稳定 key——测试强制提供 rowKey |
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
| 表格/树/列表 | 结构角色与展开/选中态可读 |
| 排序/筛选 | 控件有名 |

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
| `onChange` | 必须 |
| `loading` | 必须 |
| `size` | 必须 |
| `type` | 必须 |
| `dataSource` | 必须 |
| `columns` | 必须 |
| `title` | 必须 |
| `placement` | 必须 |
| 官方主路径示例 | 基本用法、JSX 风格的 API、可选择、选择和操作、自定义选择项、筛选和排序、树型筛选菜单、自定义筛选的搜索 |
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
| 其余示例 | 多列排序, 可控的筛选和排序, 自定义筛选菜单, 远程加载数据 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestTable_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Table 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| TBL-01 | L1 | NewTable 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| TBL-02 | L1 | 渲染 3 列×2 行 | 可见 2 行数据单元格 |
| TBL-03 | L1 | 翻到第 2 页 | `onChange` 带 pagination.current=2 |
| TBL-04 | L1 | 点可排序列头 | 排序态变化并 `onChange` |
| TBL-05 | L1 | 行选中 | `selectedRowKeys` 含该 rowKey |
| TBL-06 | L1 | 全选 | 当前页（或跨页策略按实现）keys 全选 |
| TBL-07 | L1 | `loading=true` | 表体 loading；不丢列头 |
| TBL-08 | L1 | `dataSource=[]` | 显示 Empty |
| TBL-09 | L1 | 展开行 | expanded 内容可见 |
| TBL-10 | L1 | `scroll.y` 超高 | 表体滚动、表头可固定 |
| TBL-11 | L1 | fixed 左列 + 横滚 | 左列钉住 |
| TBL-12 | L1 | 筛选确认 | `onChange` filters 有值 |
| TBL-13 | L1 | rowKey 缺失 | 开发警告或不稳定 key——测试强制提供 rowKey |
| TBL-14 | L1 | 复现官方示例「基本用法」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TBL-15 | L1 | 复现官方示例「JSX 风格的 API」（`jsx.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TBL-16 | L1 | 复现官方示例「可选择」（`row-selection.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TBL-17 | L1 | 复现官方示例「选择和操作」（`row-selection-and-operation.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TBL-18 | L1 | 复现官方示例「自定义选择项」（`row-selection-custom.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TBL-19 | L1 | 复现官方示例「筛选和排序」（`head.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TBL-20 | L1 | 复现官方示例「树型筛选菜单」（`filter-in-tree.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TBL-21 | L1 | 复现官方示例「自定义筛选的搜索」（`filter-search.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TBL-22 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| TBL-23 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| TBL-24 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| TBL-25 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| TBL-26 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| TBL-27 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| TBL-28 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewTable(...) *Table

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
Data view
  ├─ header?
  ├─ body rows/nodes
  └─ pagination/footer?
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Table 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/table.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Table 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
