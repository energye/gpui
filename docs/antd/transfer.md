# Transfer 穿梭框
> 来源：[Ant Design 6.5.x Transfer](https://ant.design/components/transfer)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：双栏穿梭选择框。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

双栏穿梭选择框。

**Transfer** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本用法 | 复现「基本用法」视觉与布局 |
| 单向样式 | 复现「单向样式」视觉与布局 |
| 带搜索框 | 带搜索框外观 |
| 高级用法 | 复现「高级用法」视觉与布局 |
| 自定义渲染行数据 | 自定义渲染/插槽外观 |
| 自定义操作按钮 | 自定义渲染/插槽外观 |
| 分页 | 分页器外观 |
| 表格穿梭框 | 复现「表格穿梭框」视觉与布局 |
| 树穿梭框 | 复现「树穿梭框」视觉与布局 |
| 自定义状态 | 自定义渲染/插槽外观 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `actions`

- **说明**：操作文案集合，顺序从上至下。当为字符串数组时使用默认的按钮，当为 ReactNode 数组时直接使用自定义元素
- **类型**：ReactNode\[]
- **默认值**：\[`>`, `<`]
- **版本**：6.0.0

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `disabled`

- **说明**：是否禁用
- **类型**：boolean
- **默认值**：false

#### `selectionsIcon`

- **说明**：自定义下拉菜单图标
- **类型**：React.ReactNode
- **默认值**：—
- **版本**：5.8.0

#### `footer`

- **说明**：底部渲染函数
- **类型**：(props, { direction }) => ReactNode
- **默认值**：-
- **版本**：direction: 4.17.0

#### `listStyle`

- **说明**：两个穿梭框的自定义样式，使用 `styles.section` 代替
- **类型**：object|({direction: `left` | `right`}) => object
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `left` | 左侧 |
  | `right` | 右侧 |

#### `oneWay`

- **说明**：展示为单向样式
- **类型**：boolean
- **默认值**：false
- **版本**：4.3.0

#### `operationStyle`

- **说明**：操作栏的自定义样式，使用 `styles.actions` 代替
- **类型**：CSSProperties
- **默认值**：-

#### `pagination`

- **说明**：使用分页样式，自定义渲染列表下无效
- **类型**：boolean | { pageSize: number, simple: boolean, showSizeChanger?: boolean, showLessItems?: boolean }
- **默认值**：false
- **版本**：4.3.0

#### `selectAllLabels`

- **说明**：自定义顶部多选框标题的集合
- **类型**：(ReactNode | (info: { selectedCount: number, totalCount: number }) => ReactNode)\[]
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
- **版本**：6.0.0

#### `titles`

- **说明**：标题集合，顺序从左至右
- **类型**：ReactNode\[]
- **默认值**：-

#### `direction`

- **说明**：渲染列表的方向
- **类型**：`left` | `right`
- **默认值**：—
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `left` | 左侧 |
  | `right` | 右侧 |

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

- 需要在多个可选项中进行多选时。
- 比起 Select 和 TreeSelect，穿梭框占据更大的空间，可以展示可选项的更多信息。

穿梭选择框用直观的方式在两栏中移动元素，完成选择行为。

选择一个或以上的选项后，点击对应的方向键，可以把选中的选项移动到另一栏。其中，左边一栏为 `source`，右边一栏为 `target`，API 的设计也反映了这两个概念。

> 注意：穿梭框组件只支持受控使用，不支持非受控模式。

### 2.2 核心功能（按官方示例拆解）

1. **基本用法**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **单向样式**（`oneWay.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **带搜索框**（`search.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **高级用法**（`advanced.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **自定义渲染行数据**（`custom-item.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **自定义操作按钮**（`actions.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **分页**（`large-data.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **表格穿梭框**（`table-transfer.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **树穿梭框**（`tree-transfer.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **自定义状态**（`status.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onChange` | 值变化 | 选项在两栏之间转移时的回调函数 |
| `disabled` | 禁用 | 是否禁用 |
| `dataSource` | 数据源 | 数据源，其中的数据将会被渲染到左边一栏中，`targetKeys` 中指定的除外 |
| `showSearch` | 搜索 | 是否显示搜索框，或可对两侧搜索框进行配置 |
| `filterOption` | 过滤 | 根据搜索内容进行筛选，接收 `inputValue` `option` `direction` 三个参数，(`direction` 自5.9.0+支持)，当 `option` 符合筛选条件时，应返回 true，反之则返回 false |
| `pagination` | 分页 | 使用分页样式，自定义渲染列表下无效 |
| `onSearch` | 搜索回调 | 搜索框内容时改变时的回调函数 |
| `targetKeys` | 穿梭目标 | 显示在右侧框数据的 key 集合 |
| `selectedKeys` | 选中 keys | 设置哪些项应该被选中 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本用法 | `basic.tsx` | 否 |
| 单向样式 | `oneWay.tsx` | 否 |
| 带搜索框 | `search.tsx` | 否 |
| 高级用法 | `advanced.tsx` | 否 |
| 自定义渲染行数据 | `custom-item.tsx` | 否 |
| 自定义操作按钮 | `actions.tsx` | 否 |
| 分页 | `large-data.tsx` | 否 |
| 表格穿梭框 | `table-transfer.tsx` | 否 |
| 树穿梭框 | `tree-transfer.tsx` | 否 |
| 自定义状态 | `status.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 自定义全选文字 | `custom-select-all-labels.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |

### 2.6 FAQ

## FAQ

### 怎样让 Transfer 穿梭框列表支持异步数据加载 {#faq-async-data-loading}

为了保持页码同步，在勾选时可以不移除选项而以禁用代替：

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

### Transfer

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| actions | 操作文案集合，顺序从上至下。当为字符串数组时使用默认的按钮，当为 ReactNode 数组时直接使用自定义元素 | ReactNode\[] | \[`>`, `<`] | 6.0.0 | × |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | 6.0.0 | 6.0.0 |
| dataSource | 数据源，其中的数据将会被渲染到左边一栏中，`targetKeys` 中指定的除外 | [RecordType extends TransferItem = TransferItem](https://github.com/ant-design/ant-design/blob/1bf0bab2a7bc0a774119f501806e3e0e3a6ba283/components/transfer/index.tsx#L12)\[] | \[] | disabled | 是否禁用 | boolean | false | selectionsIcon | 自定义下拉菜单图标 | React.ReactNode | filterOption | 根据搜索内容进行筛选，接收 `inputValue` `option` `direction` 三个参数，(`direction` 自5.9.0+支持)，当 `option` 符合筛选条件时，应返回 true，反之则返回 false | (inputValue, option, direction: `left` \| `right`): boolean | - | footer | 底部渲染函数 | (props, { direction }) => ReactNode | - | direction: 4.17.0 | × |
| ~~listStyle~~ | 两个穿梭框的自定义样式，使用 `styles.section` 代替 | object\|({direction: `left` \| `right`}) => object | - | locale | 各种语言 | { itemUnit: string; itemsUnit: string; searchPlaceholder: string; notFoundContent: ReactNode \| ReactNode[]; } | { itemUnit: `项`, itemsUnit: `项`, searchPlaceholder: `请输入搜索内容` } | oneWay | 展示为单向样式 | boolean | false | 4.3.0 | × |
| ~~operations~~ | 操作文案集合，顺序从上至下。使用 `actions` 代替 | string\[] | \[`>`, `<`] | ~~operationStyle~~ | 操作栏的自定义样式，使用 `styles.actions` 代替 | CSSProperties | - | pagination | 使用分页样式，自定义渲染列表下无效 | boolean \| { pageSize: number, simple: boolean, showSizeChanger?: boolean, showLessItems?: boolean } | false | 4.3.0 | × |
| render | 每行数据渲染函数，该函数的入参为 `dataSource` 中的项，返回值为 ReactElement。或者返回一个普通对象，其中 `label` 字段为 ReactElement，`value` 字段为 title | (record) => ReactNode | - | selectAllLabels | 自定义顶部多选框标题的集合 | (ReactNode \| (info: { selectedCount: number, totalCount: number }) => ReactNode)\[] | - | selectedKeys | 设置哪些项应该被选中 | string\[] \| number\[] | \[] | showSearch | 是否显示搜索框，或可对两侧搜索框进行配置 | boolean \| { placeholder:string,defaultValue:string } | false | showSelectAll | 是否展示全选勾选框 | boolean | true | status | 设置校验状态 | 'error' \| 'warning' | - | 4.19.0 | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 6.0.0 | 6.0.0 |
| targetKeys | 显示在右侧框数据的 key 集合 | string\[] \| number\[] | \[] | titles | 标题集合，顺序从左至右 | ReactNode\[] | - | onChange | 选项在两栏之间转移时的回调函数 | (targetKeys, direction, moveKeys): void | - | onScroll | 选项列表滚动时的回调函数 | (direction, event): void | - | onSearch | 搜索框内容时改变时的回调函数 | (direction: `left` \| `right`, value: string): void | - | onSelectChange | 选中项发生改变时的回调函数 | (sourceSelectedKeys, targetSelectedKeys): void | - 
### Render Props

Transfer 支持接收 `children` 自定义渲染列表，并返回以下参数：

| 参数            | 说明           | 类型                                              | 版本 |
| --------------- | -------------- | ------------------------------------------------- | ---- |
| direction       | 渲染列表的方向 | `left` \| `right`                                 |      |
| disabled        | 是否禁用列表   | boolean                                           |      |
| filteredItems   | 过滤后的数据   | RecordType\[]                                     |      |
| selectedKeys    | 选中的条目     | string\[] \| number\[]                            |      |
| onItemSelect    | 勾选条目       | (key: string \| number, selected: boolean)        |      |
| onItemSelectAll | 勾选一组条目   | (keys: string\[] \| number\[], selected: boolean) |      |

#### 参考示例

```jsx
<Transfer {...props}>{(listProps) => <YourComponent {...listProps} />}</Transfer>
```

### 导入方式

```js
import { Transfer } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `actions` | 操作文案集合，顺序从上至下。当为字符串数组时使用默认的按钮，当为 ReactNode 数组时直接使用自定义元素 | ReactNode\[] | \[`>`, `<`] | 6.0.0 |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `dataSource` | 数据源，其中的数据将会被渲染到左边一栏中，`targetKeys` 中指定的除外 | [RecordType extends TransferItem = TransferItem](https://github.com/ant-design/ant-design/blob/1bf0bab2a7bc0a774119f501806e3e0e3a6ba283/components/transfer/index.tsx#L12)\[] | \[] | — |
| `disabled` | 是否禁用 | boolean | false | — |
| `selectionsIcon` | 自定义下拉菜单图标 | React.ReactNode | — | 5.8.0 |
| `filterOption` | 根据搜索内容进行筛选，接收 `inputValue` `option` `direction` 三个参数，(`direction` 自5.9.0+支持)，当 `option` 符合筛选条件时，应返回 true，反之则返回 false | (inputValue, option, direction: `left` \| `right`): boolean | - | — |
| `footer` | 底部渲染函数 | (props, { direction }) => ReactNode | - | direction: 4.17.0 |
| `listStyle` | 两个穿梭框的自定义样式，使用 `styles.section` 代替 | object\|({direction: `left` \| `right`}) => object | - | — |
| `locale` | 各种语言 | { itemUnit: string; itemsUnit: string; searchPlaceholder: string; notFoundContent: ReactNode \| ReactNode[]; } | { itemUnit: `项`, itemsUnit: `项`, searchPlaceholder: `请输入搜索内容` } | — |
| `oneWay` | 展示为单向样式 | boolean | false | 4.3.0 |
| `operations` | 操作文案集合，顺序从上至下。使用 `actions` 代替 | string\[] | \[`>`, `<`] | — |
| `operationStyle` | 操作栏的自定义样式，使用 `styles.actions` 代替 | CSSProperties | - | — |
| `pagination` | 使用分页样式，自定义渲染列表下无效 | boolean \| { pageSize: number, simple: boolean, showSizeChanger?: boolean, showLessItems?: boolean } | false | 4.3.0 |
| `render` | 每行数据渲染函数，该函数的入参为 `dataSource` 中的项，返回值为 ReactElement。或者返回一个普通对象，其中 `label` 字段为 ReactElement，`value` 字段为 title | (record) => ReactNode | - | — |
| `selectAllLabels` | 自定义顶部多选框标题的集合 | (ReactNode \| (info: { selectedCount: number, totalCount: number }) => ReactNode)\[] | - | — |
| `selectedKeys` | 设置哪些项应该被选中 | string\[] \| number\[] | \[] | — |
| `showSearch` | 是否显示搜索框，或可对两侧搜索框进行配置 | boolean \| { placeholder:string,defaultValue:string } | false | — |
| `showSelectAll` | 是否展示全选勾选框 | boolean | true | — |
| `status` | 设置校验状态 | 'error' \| 'warning' | - | 4.19.0 |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `targetKeys` | 显示在右侧框数据的 key 集合 | string\[] \| number\[] | \[] | — |
| `titles` | 标题集合，顺序从左至右 | ReactNode\[] | - | — |
| `onChange` | 选项在两栏之间转移时的回调函数 | (targetKeys, direction, moveKeys): void | - | — |
| `onScroll` | 选项列表滚动时的回调函数 | (direction, event): void | - | — |
| `onSearch` | 搜索框内容时改变时的回调函数 | (direction: `left` \| `right`, value: string): void | - | — |
| `onSelectChange` | 选中项发生改变时的回调函数 | (sourceSelectedKeys, targetSelectedKeys): void | - | — |
| `direction` | 渲染列表的方向 | `left` \| `right` | — | — |
| `filteredItems` | 过滤后的数据 | RecordType\[] | — | — |
| `onItemSelect` | 勾选条目 | (key: string \| number, selected: boolean) | — | — |
| `onItemSelectAll` | 勾选一组条目 | (keys: string\[] \| number\[], selected: boolean) | — | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Transfer** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **11** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/transfer
- 中文文档：https://ant.design/components/transfer-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/transfer
- 驱动 gpui kit：`transfer`
