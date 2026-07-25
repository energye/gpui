# Transfer 穿梭框
> 来源：[Ant Design 6.5.x Transfer](https://ant.design/components/transfer)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：双栏穿梭选择框。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
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

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

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

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Transfer** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/transfer/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Transfer）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 受控输入/选择、弹层、清除、校验 status、尺寸档 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Transfer）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：双栏穿梭选择框。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，种子：`controlHeight=32`、`controlHeightLG=40`、`fontSize=14`、`lineHeight≈1.571`）。实现必须通过 Theme Token 读取；组件级默认见 `prepareComponentToken`（antd `components/transfer/style`）。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 列表宽 `listWidth` | **180** | 组件 Token `listWidth` |
| 列表高 `listHeight` | **200** | 组件 Token `listHeight` |
| 分页列表宽 `listWidthLG` | **250** | 组件 Token `listWidthLG`（`pagination` 开启时） |
| 顶栏高 `headerHeight` | **40** | `controlHeightLG` |
| 列表项高 `itemHeight` | **32** | `controlHeight` |
| 列表项纵向 pad | ≈ **(itemHeight − fontHeight) / 2** | `itemPaddingBlock` |
| 操作按钮 | size=**small**（高 **24**），type=primary | Button Token |
| 两栏间距 / 操作列 margin | **8**（`marginXS`） | Theme |
| 操作按钮 gap | **4**（`marginXXS`） | Theme |
| 列表圆角 | **8** | `borderRadiusLG` |
| 边框线宽 | **1** | `lineWidth` |
| 字号 | **14** | `fontSize` |
| 分页默认 `pageSize` | **10**；`simple=true` | antd ListBody |
| Focus ring outset | ≈ **1.5px** 可见 | 可调，必须可见 |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 主色 / 操作按钮 | `colorPrimary` + 变体 | 穿梭 Button primary |
| 列表边框 / 分割 | `colorBorder` / `colorSplit` | section 边框、header 底边 |
| 容器底 | `colorBgContainer` | 列表底 |
| 项 hover / 选中底 | `controlItemBgHover` / `controlItemBgActive` | 列表项 |
| 错误 / 警告 | `colorError` / `colorWarning` | `status` 时 section 边框 |
| 文本 / 次级 / 禁用文 | `colorText` / `colorTextSecondary` / `colorTextDisabled` | 标题、计数、空态 |
| 禁用容器 | `colorBgContainerDisabled` | 整控件 disabled 时 section 底 |

禁止硬编码品牌色作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据录入**。  
官方说明：**Transfer 只支持受控 `targetKeys`**（父级在 `onChange` 后回写）；kit 为易用可提供本地提交，但 **Controlled 模式外部优先**。

| 配置 | 说明 | 类型（kit） | 默认 |
| --- | --- | --- | --- |
| `dataSource` | 全量数据；`targetKeys` 中的项渲染到右栏，其余左栏 | `[]TransferItem` | `[]` |
| `targetKeys` | 右栏 key 集合（受控主值） | `[]string` | `[]` |
| `selectedKeys` | 当前勾选项（左右合计或分侧） | `[]string` | `[]` |
| `onChange` | 穿梭完成：`(targetKeys, direction, moveKeys)` | callback | — |
| `onSelectChange` | 勾选变化：`(sourceSelected, targetSelected)` | callback | — |
| `onSearch` | 搜索框变化：`(direction, value)` | callback | — |
| `disabled` | 整控件禁用 | bool | false |
| `oneWay` | 单向：仅右移操作钮；右栏项显示移除 | bool | false |
| `showSearch` | 两侧搜索框 | bool | false |
| `showSelectAll` | 顶栏全选勾选框 | bool | true |
| `filterOption` | 搜索过滤；默认按 render 文案包含 | `func(input, item, dir) bool` | 默认 includes |
| `actions` | 操作文案 `[toRight, toLeft]` | `[]string` | `["", ""]`（图标箭头） |
| `titles` | 左右标题 | `[2]string` | `["", ""]` |
| `status` | 校验态 | `error` \| `warning` \| 空 | 空 |
| `pagination` | 列表分页；自定义 ListBody 时无效 | bool \| `{pageSize, simple}` | false；pageSize=10 simple |
| `render` | 行文案（P0 字符串） | `func(item) string` | `item.Title` 或 key |
| `footer` | 底栏渲染 | `func(dir) Node` | — |
| `children` / ListBody | 自定义列表体（表格穿梭） | `func(ListBodyProps) Node` | 默认 checkbox 列表 |
| `locale` | 文案：项单位、搜索占位、空态 | struct | 中文默认「项」「请输入搜索内容」 |

**配置优先级：** 受控 `targetKeys` / `selectedKeys` > 本地非受控状态 > 组件默认。ConfigProvider 全局默认 P1。

### 6.4 交互状态机（L1）

```text
[左栏勾选] ── actions[0] / > ──► 右栏 targetKeys += moveKeys；onChange(right)
[右栏勾选] ── actions[1] / < ──► 右栏 targetKeys -= moveKeys；onChange(left)
[oneWay 右栏移除] ─────────────► 等同左移单项
[search] 过滤当前栏 filteredItems（不改 targetKeys）
[全选] 勾选当前栏可见（过滤后、非 disabled）项
[disabled] 禁止勾选 / 穿梭 / 搜索输入
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| TF-S1 | 右移一项 | `targetKeys` 含该 key；`moveKeys` 正确；方向 `right` |
| TF-S2 | 左移 | 从 `targetKeys` 移除 |
| TF-S3 | search 左 | 左栏可见项按 filter 过滤；`onSearch` 触发 |
| TF-S4 | 全选右移 | 批量加入 `targetKeys` |
| TF-S5 | disabled | 不可移、不可改选 |
| TF-S6 | oneWay | 无左移操作钮；右栏项可单条移除 |
| TF-S7 | 受控 targetKeys | 本地不改值；仅 `onChange`；父 `SetTargetKeys` 后 UI 更新 |
| TF-S8 | 项 disabled | 不可勾选；不计入全选可选项 |
| TF-S9 | pagination | 仅当前页可勾选展示；pageSize 默认 10 |

### 6.5 视觉 chrome 规则（L2 摘要）

| 态 / 变体 | 规则 |
| --- | --- |
| default | section：`colorBgContainer` + `colorBorder` 边 + `borderRadiusLG` |
| item hover | `controlItemBgHover` |
| item checked | `controlItemBgActive` |
| focus | 可聚焦控件 **可见** focus ring |
| disabled | section 底 `colorBgContainerDisabled`；无 hover 高亮；操作钮 disabled |
| status=error/warning | section 边框语义色（搜索框保持普通边） |
| 操作钮 | primary small；无选中可移项时 disabled |
| 空列表 | 居中 `notFoundContent`（默认「暂无数据」/locale） |

**动效：** P0 瞬时切换；操作钮 loading 用 Button Ticker。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | 根 `group`（或无障碍组名）；列表区 listbox/list；项 option/可勾选；搜索 textbox |
| 标签 | `SetAriaLabel` 或 titles 作为侧栏名；操作钮有可访问名（文案或「to right/left」） |
| 全选 | checkbox + 与计数文案关联 |
| 错误 | `status=error` 时暴露 invalid（平台支持时） |
| 键盘 | 操作钮 Space/Enter；搜索可输入；焦点环可见 |

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
| `dataSource` / `targetKeys` / 受控 | 双栏数据；Controlled 时仅 onChange |
| `selectedKeys` / `onSelectChange` | 勾选（左右分侧） |
| `onChange(targetKeys, direction, moveKeys)` | 穿梭必须 |
| `disabled` | 整控件禁用 |
| `status` | error \| warning 边框 |
| `oneWay` | 仅右移 + 右栏移除 |
| `showSearch` + `filterOption` + `onSearch` | 两侧搜索 |
| `showSelectAll` | 默认 true；顶栏全选 / 半选 |
| `actions` | 操作文案；默认箭头 |
| `titles` | 左右标题 |
| `render` | 行字符串文案 |
| `footer` | 底栏工厂 |
| `pagination` | bool / pageSize；默认 pageSize=10 simple |
| `ListBody` children | 自定义列表体（表格穿梭） |
| 操作钮 loading | Button Ticker（自定义 actions 演示） |
| 官方主路径示例 | 基本用法、单向样式、带搜索框、高级用法、自定义渲染行数据、自定义操作按钮、分页、表格穿梭框 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | 最低要求 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 深度 | 分期 |
| `selectionsIcon` / 顶栏下拉「全选/反选/清空」菜单 | 分期（P0 仅 checkbox 全选） |
| `selectAllLabels` 自定义 | 分期 |
| 树穿梭框（tree-transfer） | 分期 |
| 自定义状态完整 demo 页以外深度 / style-class | 分期（status 边框 P0 已覆盖） |
| 动画像素级 / 超大虚拟列表 | 分期（分页 P0 可承载 large-data 主路径） |
| 浏览器-only API 或桌面无等价项 | 分期 |
| ConfigProvider 全局默认 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |
| 其余示例 | 树穿梭框, 自定义语义结构的样式和类, _semantic.tsx, component-token |

### 6.9 验收用例表（可测）

> 测试名建议：`TestTransfer_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Transfer 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| TF-01 | L1 | NewTransfer 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| TF-02 | L1 | 右移一项 | targetKeys 含 key |
| TF-03 | L1 | 左移 | 移除 |
| TF-04 | L1 | search 左 | 过滤 |
| TF-05 | L1 | 全选右移 | 批量 |
| TF-06 | L1 | disabled | 不可移 |
| TF-07 | L1 | oneWay | 无回移 |
| TF-08 | L1 | 受控 targetKeys | 外部优先 |
| TF-09 | L1 | 复现官方示例「基本用法」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TF-10 | L1 | 复现官方示例「单向样式」（`oneWay.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TF-11 | L1 | 复现官方示例「带搜索框」（`search.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TF-12 | L1 | 复现官方示例「高级用法」（`advanced.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TF-13 | L1 | 复现官方示例「自定义渲染行数据」（`custom-item.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TF-14 | L1 | 复现官方示例「自定义操作按钮」（`actions.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TF-15 | L1 | 复现官方示例「分页」（`large-data.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TF-16 | L1 | 复现官方示例「表格穿梭框」（`table-transfer.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| TF-17 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| TF-18 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| TF-19 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| TF-20 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| TF-21 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| TF-22 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| TF-23 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API（删除 `NewTransfer([]string)` / `Source`/`Target` List 直出 / `MoveAllToTarget` 等简化接口）；以下为 **产品需求层** 契约。

```text
type TransferItem struct {
  Key, Title, Description string
  Disabled bool
}
type TransferDirection string // "left" | "right"
type TransferStatus int      // None | Error | Warning

type TransferListBodyProps struct {
  Direction TransferDirection
  Disabled bool
  FilteredItems []TransferItem
  SelectedKeys []string
  OnItemSelect    func(key string, selected bool)
  OnItemSelectAll func(keys []string, selected bool) // selected=true 全选传入 keys；false 清空
}

NewTransfer() *Transfer
TransferItemsFromTitles(titles ...string) []TransferItem  // 测试/迁移糖

// 数据
SetDataSource([]TransferItem)
SetTargetKeys([]string) / TargetKeys() []string
SetControlled(bool)                 // true：穿梭只 OnChange，不改本地 targetKeys
SetSelectedKeys([]string)           // 合并左右勾选（或按侧过滤）
SourceSelectedKeys() / TargetSelectedKeys()
// 配置 P0
SetDisabled / SetOneWay / SetShowSearch / SetShowSelectAll
SetStatus(TransferStatus)
SetTitles(left, right string)
SetActions(toRight, toLeft string)  // 空串 → 默认 ">" / "<" 文案
SetActionLoading(toRight, toLeft bool) // 操作钮 Button.Loading + Ticker
SetFilterOption(func(input string, item TransferItem, dir TransferDirection) bool)
SetRender(func(item TransferItem) string)
SetFooter(func(dir TransferDirection) core.Node)
SetPagination(enabled bool) / SetPaginationConfig(pageSize int, simple bool)
SetListWidth / SetListHeight        // 覆盖默认 180×200；分页默认宽 250
SetListBody(func(TransferListBodyProps) core.Node) // 表格穿梭等
SetLocale(TransferLocale)
// 回调
SetOnChange(func(targetKeys []string, direction TransferDirection, moveKeys []string))
SetOnSelectChange(func(sourceSelected, targetSelected []string))
SetOnSearch(func(direction TransferDirection, value string))
// 命令（测试 / 程序化）
SelectItem(dir, key string, selected bool)
SelectAllVisible(dir TransferDirection)
Move(dir TransferDirection)         // right=左→右；left=右→左
SetSearch(dir TransferDirection, query string)
RemoveTargetItem(key string)        // oneWay 右栏移除
// 主题 / a11y / 挂树
SetTheme / SetFace / SetStyle / SetAriaLabel
AttachTicker(*core.Tree)            // 转发操作钮 loading Ticker
Node() core.Node                    // Root 身份稳定
ListWidth() / ListHeight() / HeaderHeight() / ItemHeight()  // 度量测试
SectionNodes() (left, right core.Node)
ActionButtons() (toRight, toLeft *Button)
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| dataSource / targetKeys / selected | 空 |
| Disabled / OneWay / ShowSearch | false |
| ShowSelectAll | true |
| Status | None |
| Actions | `">"`, `"<"` |
| Titles | 空 |
| Pagination | off；开启后 pageSize=**10** simple |
| listWidth × listHeight | **180 × 200** |
| Controlled | false（本地提交；设 true 或文档示例用受控） |
| locale.itemUnit / searchPlaceholder | `"项"` / `"请输入搜索内容"` |
| 其余 | 对齐 antd 6.5 §3 |

### 6.11 结构与绘制分层（实现提示）

```text
Flex Row (Root, role=group, gap/margin 操作列)
  ├─ Section left  (Decorated column, width=listWidth, height=listHeight)
  │    ├─ Header: Checkbox? + title + "n/m 项"
  │    ├─ Search Input? (showSearch)
  │    ├─ Body: default item list | ListBody(props) | 空态
  │    ├─ Footer?
  │    └─ Pagination? (simple)
  ├─ Actions column (primary small Buttons)
  │    ├─ toRight  (>)
  │    └─ toLeft   (<)  // !oneWay
  └─ Section right (同 left；oneWay 时项带移除)
```

- 组合 `ui/primitive` + `ui/kit`（Checkbox / Input / Button / Pagination）+ `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 在数据/配置变化时重建；Root 身份稳定。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 操作钮 loading 走 Button Ticker；`AttachTicker` 转发。  
- 自定义 ListBody 时内建分页/默认项列表不渲染（对齐 antd「自定义渲染列表下 pagination 无效」）。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Transfer 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/transfer.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Transfer 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
