# Masonry 瀑布流
> 来源：[Ant Design 6.5.x Masonry](https://ant.design/components/masonry)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：布局（Layout）  
> 说明：  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

**Masonry** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基础用法 | 复现「基础用法」视觉与布局 |
| 响应式 | 断点响应式 |
| 图片 | 复现「图片」视觉与布局 |
| 动态更新 | 复现「动态更新」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `fresh`

- **说明**：是否持续监听子项尺寸变化
- **类型**：`boolean`
- **默认值**：`false`

#### `gutter`

- **说明**：间距，可以是固定值、响应式配置或水平垂直间距配置
- **类型**：[Gap](#gap) | \[[Gap](#gap), [Gap](#gap)\]
- **默认值**：`0`

#### `styles`

- **说明**：语义化结构 style，支持对象和函数形式
- **类型**：Record | ((info: { props }) => Record)
- **默认值**：-
- **版本**：6.0.0

#### `height`

- **说明**：高度
- **类型**：`number`
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

- 展示不规则高度的图片或卡片时
- 需要按照列数均匀分布内容时
- 需要响应式调整列数时

### 2.2 核心功能（按官方示例拆解）

1. **基础用法**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **响应式**（`responsive.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **图片**（`image.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **动态更新**（`dynamic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `items` | 数据化 items | 瀑布流项 |
| `columns` | 列配置 | 列数，可以是固定值或响应式配置 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基础用法 | `basic.tsx` | 否 |
| 响应式 | `responsive.tsx` | 否 |
| 图片 | `image.tsx` | 否 |
| 动态更新 | `dynamic.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 持续更新 | `fresh.tsx` | 是 |

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

### Masonry

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | 6.0.0 | 6.0.0 |
| columns | 列数，可以是固定值或响应式配置 | `number \| { xs?: number; sm?: number; md?: number }` | `3` | fresh | 是否持续监听子项尺寸变化 | `boolean` | `false` | gutter | 间距，可以是固定值、响应式配置或水平垂直间距配置 | [Gap](#gap) \| \[[Gap](#gap), [Gap](#gap)\] | `0` | items | 瀑布流项 | [MasonryItem](#masonryitem)[] | - | itemRender | 自定义项渲染 | `(item: MasonryItem) => React.ReactNode` | - | styles | 语义化结构 style，支持对象和函数形式 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| ((info: { props }) => Record<[SemanticDOM](#semantic-dom), CSSProperties>) | - | 6.0.0 | 6.0.0 |
| onLayoutChange | 列排序回调 | `({ key: React.Key; column: number }[]) => void` | - 
### MasonryItem

| 参数     | 说明                                             | 类型                 | 默认值 |
| -------- | ------------------------------------------------ | -------------------- | ------ |
| children | 自定义展示内容，相对 `itemRender` 具有更高优先级 | `React.ReactNode`    | -      |
| column   | 自定义所在列                                     | `number`             | -      |
| data     | 自定义存储数据                                   | `T`                  | -      |
| height   | 高度                                             | `number`             | -      |
| key      | 唯一标识                                         | `string` \| `number` | -      |

### Gap

Gap 是项之间的间距，可以是固定值，也可以是响应式配置。

```ts
type Gap = undefined | number | Partial<Record<'xs' | 'sm' | 'md' | 'lg' | 'xl' | 'xxl', number>>;
```

### 导入方式

```js
import { Masonry } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `columns` | 列数，可以是固定值或响应式配置 | `number \| { xs?: number; sm?: number; md?: number }` | `3` | — |
| `fresh` | 是否持续监听子项尺寸变化 | `boolean` | `false` | — |
| `gutter` | 间距，可以是固定值、响应式配置或水平垂直间距配置 | [Gap](#gap) \| \[[Gap](#gap), [Gap](#gap)\] | `0` | — |
| `items` | 瀑布流项 | [MasonryItem](#masonryitem)[] | - | — |
| `itemRender` | 自定义项渲染 | `(item: MasonryItem) => React.ReactNode` | - | — |
| `styles` | 语义化结构 style，支持对象和函数形式 | Record \| ((info: { props }) => Record) | - | 6.0.0 |
| `onLayoutChange` | 列排序回调 | `({ key: React.Key; column: number }[]) => void` | - | — |
| `children` | 自定义展示内容，相对 `itemRender` 具有更高优先级 | `React.ReactNode` | - | — |
| `column` | 自定义所在列 | `number` | - | — |
| `data` | 自定义存储数据 | `T` | - | — |
| `height` | 高度 | `number` | - | — |
| `key` | 唯一标识 | `string` \| `number` | - | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Masonry** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **5** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/masonry
- 中文文档：https://ant.design/components/masonry-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/masonry
- 驱动 gpui kit：`masonry`
