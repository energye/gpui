# Pagination 分页
> 来源：[Ant Design 6.5.x Pagination](https://ant.design/components/pagination)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：导航（Navigation）  
> 说明：分页器用于分隔长列表，每次只加载一个页面。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

分页器用于分隔长列表，每次只加载一个页面。

**Pagination** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 方向 | 复现「方向」视觉与布局 |
| 更多 | 复现「更多」视觉与布局 |
| 改变 | 复现「改变」视觉与布局 |
| 跳转 | 复现「跳转」视觉与布局 |
| 尺寸 | 不同 size 档位的高宽/字号/内边距 |
| 简洁 | 复现「简洁」视觉与布局 |
| 受控 | 复现「受控」视觉与布局 |
| 总数 | 复现「总数」视觉与布局 |
| 全部展示 | 复现「全部展示」视觉与布局 |
| 上一步和下一步 | 复现「上一步和下一步」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `align`

- **说明**：对齐方式
- **类型**：start | center | end
- **默认值**：-
- **版本**：5.19.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `center` | 居中 |
  | `end` | 逻辑结束侧 |

#### `classNames`

- **说明**：自定义组件内部各语义化结构的类名。支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `disabled`

- **说明**：禁用分页
- **类型**：boolean
- **默认值**：-

#### `responsive`

- **说明**：当 size 未指定时，根据屏幕宽度自动调整尺寸
- **类型**：boolean
- **默认值**：-

#### `simple`

- **说明**：当添加该属性时，显示为简单分页
- **类型**：boolean | { readOnly?: boolean }
- **默认值**：-

#### `size`

- **说明**：组件尺寸
- **类型**：`large` | `medium` | `small`
- **默认值**：`medium`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `styles`

- **说明**：自定义组件内部各语义化结构的内联样式。支持对象或函数
- **类型**：Record | (info: { props }) => Record
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

- 当加载/渲染所有数据将花费很多时间时；
- 可切换页码浏览数据。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **方向**（`align.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **更多**（`more.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **改变**（`changer.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **跳转**（`jump.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **尺寸**（`mini.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **简洁**（`simple.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **受控**（`controlled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **总数**（`total.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **全部展示**（`all.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **上一步和下一步**（`itemRender.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onChange` | 值变化 | 页码或 `pageSize` 改变的回调，参数是改变后的页码及每页条数 |
| `disabled` | 禁用 | 禁用分页 |
| `current` | 当前步骤/页 | 当前页数 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 方向 | `align.tsx` | 否 |
| 更多 | `more.tsx` | 否 |
| 改变 | `changer.tsx` | 否 |
| 跳转 | `jump.tsx` | 否 |
| 尺寸 | `mini.tsx` | 否 |
| 简洁 | `simple.tsx` | 否 |
| 受控 | `controlled.tsx` | 否 |
| 总数 | `total.tsx` | 否 |
| 全部展示 | `all.tsx` | 否 |
| 上一步和下一步 | `itemRender.tsx` | 否 |
| 线框风格 | `wireframe.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |
| 变体 Debug | `variant-debug.tsx` | 是 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |

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
<Pagination onChange={onChange} total={50} />
```

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| align | 对齐方式 | start \| center \| end | - | 5.19.0 | × |
| classNames | 自定义组件内部各语义化结构的类名。支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> | - | current | 当前页数 | number | - | defaultCurrent | 默认的当前页数 | number | 1 | defaultPageSize | 默认的每页条数 | number | 10 | disabled | 禁用分页 | boolean | - | hideOnSinglePage | 只有一页时是否隐藏分页器 | boolean | false | itemRender | 用于自定义页码的结构，可用于优化 SEO | (page, type: 'page' \| 'prev' \| 'next', originalElement) => React.ReactNode | - | pageSize | 每页条数 | number | - | pageSizeOptions | 指定每页可以显示多少条 | number\[] | \[`10`, `20`, `50`, `100`] | responsive | 当 size 未指定时，根据屏幕宽度自动调整尺寸 | boolean | - | showLessItems | 是否显示较少页面内容 | boolean | false | showQuickJumper | 是否可以快速跳转至某页 | boolean \| { goButton: ReactNode } | false | showSizeChanger | 是否展示 `pageSize` 切换器 | boolean \| [SelectProps](/components/select-cn#api) | - | SelectProps: 5.21.0 | 4.21.0，SelectProps: 5.21.0 |
| showTitle | 是否显示原生 tooltip 页码提示 | boolean | true | showTotal | 用于显示数据总量和当前数据顺序 | function(total, range) | - | simple | 当添加该属性时，显示为简单分页 | boolean \| { readOnly?: boolean } | - | size | 组件尺寸 | `large` \| `medium` \| `small` | `medium` | styles | 自定义组件内部各语义化结构的内联样式。支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | total | 数据总数 | number | 0 | totalBoundaryShowSizeChanger | 当 `total` 大于该值时，`showSizeChanger` 默认为 true | number | 50 | onChange | 页码或 `pageSize` 改变的回调，参数是改变后的页码及每页条数 | function(page, pageSize) | - | onShowSizeChange | pageSize 变化的回调 | function(current, size) | - 
### 导入方式

```js
import { Pagination } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `align` | 对齐方式 | start \| center \| end | - | 5.19.0 |
| `classNames` | 自定义组件内部各语义化结构的类名。支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `current` | 当前页数 | number | - | — |
| `defaultCurrent` | 默认的当前页数 | number | 1 | — |
| `defaultPageSize` | 默认的每页条数 | number | 10 | — |
| `disabled` | 禁用分页 | boolean | - | — |
| `hideOnSinglePage` | 只有一页时是否隐藏分页器 | boolean | false | — |
| `itemRender` | 用于自定义页码的结构，可用于优化 SEO | (page, type: 'page' \| 'prev' \| 'next', originalElement) => React.ReactNode | - | — |
| `pageSize` | 每页条数 | number | - | — |
| `pageSizeOptions` | 指定每页可以显示多少条 | number\[] | \[`10`, `20`, `50`, `100`] | — |
| `responsive` | 当 size 未指定时，根据屏幕宽度自动调整尺寸 | boolean | - | — |
| `showLessItems` | 是否显示较少页面内容 | boolean | false | — |
| `showQuickJumper` | 是否可以快速跳转至某页 | boolean \| { goButton: ReactNode } | false | — |
| `showSizeChanger` | 是否展示 `pageSize` 切换器 | boolean \| [SelectProps](/components/select-cn#api) | - | SelectProps: 5.21.0 |
| `showTitle` | 是否显示原生 tooltip 页码提示 | boolean | true | — |
| `showTotal` | 用于显示数据总量和当前数据顺序 | function(total, range) | - | — |
| `simple` | 当添加该属性时，显示为简单分页 | boolean \| { readOnly?: boolean } | - | — |
| `size` | 组件尺寸 | `large` \| `medium` \| `small` | `medium` | — |
| `styles` | 自定义组件内部各语义化结构的内联样式。支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `total` | 数据总数 | number | 0 | — |
| `totalBoundaryShowSizeChanger` | 当 `total` 大于该值时，`showSizeChanger` 默认为 true | number | 50 | — |
| `onChange` | 页码或 `pageSize` 改变的回调，参数是改变后的页码及每页条数 | function(page, pageSize) | - | — |
| `onShowSizeChange` | pageSize 变化的回调 | function(current, size) | - | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Pagination** 的验收清单：

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

---
## 5. 参考链接
- 官方文档：https://ant.design/components/pagination
- 中文文档：https://ant.design/components/pagination-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/pagination
- 驱动 gpui kit：`pagination`
