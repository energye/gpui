# Splitter 分隔面板
> 来源：[Ant Design 6.5.x Splitter](https://ant.design/components/splitter)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：布局（Layout）  
> 说明：自由切分指定区域  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

自由切分指定区域

**Splitter** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本用法 | 复现「基本用法」视觉与布局 |
| 受控模式 | 复现「受控模式」视觉与布局 |
| 垂直方向 | 纵向布局 |
| 可折叠 | 复现「可折叠」视觉与布局 |
| 可折叠图标显示 | icon 与文本混排 |
| 多面板 | 复现「多面板」视觉与布局 |
| 复杂组合 | 复现「复杂组合」视觉与布局 |
| 延迟渲染模式 | 复现「延迟渲染模式」视觉与布局 |
| 自定义样式 | 自定义渲染/插槽外观 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |
| 双击重置 | 复现「双击重置」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `collapsible`

- **说明**：`motion` 是否开启折叠动画，`icon` 自定义折叠图标
- **类型**：`{ motion?: boolean; icon?: { start?: ReactNode; end?: ReactNode } }`
- **默认值**：-
- **版本**：6.4.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `motion` | 官方取值 `motion` |
  | `icon` | 官方取值 `icon` |

#### `collapsibleIcon`

- **说明**：折叠图标
- **类型**：`{start?: ReactNode; end?: ReactNode}`
- **默认值**：-
- **版本**：6.0.0

#### `draggerIcon`

- **说明**：拖拽图标
- **类型**：`ReactNode`
- **默认值**：-
- **版本**：6.0.0

#### `layout`

- **说明**：布局方向
- **类型**：`horizontal` | `vertical`
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |
  | `vertical` | 垂直排布 |

#### `orientation`

- **说明**：布局方向
- **类型**：`horizontal` | `vertical`
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |
  | `vertical` | 垂直排布 |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `vertical`

- **说明**：排列方向，与 `orientation` 同时存在，以 `orientation` 优先
- **类型**：boolean
- **默认值**：`false`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `orientation` | 官方取值 `orientation` |

#### `onResize`

- **说明**：面板大小变化回调
- **类型**：`(sizes: number[]) => void`
- **默认值**：-

#### `defaultSize`

- **说明**：初始面板大小，支持数字 px 或者文字 '百分比%' 类型
- **类型**：`number | string`
- **默认值**：-

#### `max`

- **说明**：最大阈值，支持数字 px 或者文字 '百分比%' 类型
- **类型**：`number | string`
- **默认值**：-

#### `min`

- **说明**：最小阈值，支持数字 px 或者文字 '百分比%' 类型
- **类型**：`number | string`
- **默认值**：-

#### `size`

- **说明**：受控面板大小，支持数字 px 或者文字 '百分比%' 类型
- **类型**：`number | string`
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

- 可以水平或垂直地分隔区域。
- 当需要自由拖拽调整各区域大小。
- 当需要指定区域的最大最小宽高时。

### 2.2 核心功能（按官方示例拆解）

1. **基本用法**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **受控模式**（`control.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **垂直方向**（`vertical.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **可折叠**（`collapsible.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **可折叠图标显示**（`collapsibleIcon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **多面板**（`multiple.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **复杂组合**（`group.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **延迟渲染模式**（`lazy.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **自定义样式**（`customize.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **双击重置**（`reset.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `destroyOnHidden` | 隐藏销毁 | 折叠时（size 为 0）销毁面板内容，应用于所有面板，可在单个面板上覆盖 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本用法 | `size.tsx` | 否 |
| 受控模式 | `control.tsx` | 否 |
| 垂直方向 | `vertical.tsx` | 否 |
| 可折叠 | `collapsible.tsx` | 否 |
| 可折叠图标显示 | `collapsibleIcon.tsx` | 否 |
| 多面板 | `multiple.tsx` | 否 |
| 复杂组合 | `group.tsx` | 否 |
| 延迟渲染模式 | `lazy.tsx` | 否 |
| 自定义样式 | `customize.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 双击重置 | `reset.tsx` | 否 |
| 标签页中嵌套 | `nested-in-tabs.tsx` | 是 |
| 调试 | `debug.tsx` | 是 |
| 尺寸混合 | `size-mix.tsx` | 是 |

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

> Splitter 组件需要通过子元素计算面板大小，因而其子元素仅支持 `Splitter.Panel`。

### Splitter

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | 6.0.0 | 6.0.0 |
| collapsible | `motion` 是否开启折叠动画，`icon` 自定义折叠图标 | `{ motion?: boolean; icon?: { start?: ReactNode; end?: ReactNode } }` | - | 6.4.0 | × |
| ~~collapsibleIcon~~ | 折叠图标 | `{start?: ReactNode; end?: ReactNode}` | - | 6.0.0 | × |
| destroyOnHidden | 折叠时（size 为 0）销毁面板内容，应用于所有面板，可在单个面板上覆盖 | `boolean` | `false` | 6.4.0 | × |
| draggerIcon | 拖拽图标 | `ReactNode` | - | 6.0.0 | × |
| ~~layout~~ | 布局方向 | `horizontal` \| `vertical` | `horizontal` | - | × |
| lazy | 延迟渲染模式 | `boolean` | `false` | 5.23.0 | × |
| onCollapse | 展开-收起时回调 | `(collapsed: boolean[], sizes: number[]) => void` | - | 5.28.0 | × |
| orientation | 布局方向 | `horizontal` \| `vertical` | `horizontal` | - | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 6.0.0 | 6.0.0 |
| vertical | 排列方向，与 `orientation` 同时存在，以 `orientation` 优先 | boolean | `false` | onDraggerDoubleClick | 双击拖拽条回调 | `(index: number) => void` | - | 6.3.0 | × |
| onResize | 面板大小变化回调 | `(sizes: number[]) => void` | - | - | × |
| onResizeEnd | 拖拽结束回调 | `(sizes: number[]) => void` | - | - | × |
| onResizeStart | 开始拖拽之前回调 | `(sizes: number[]) => void` | - | - | × |

### Panel

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| collapsible | 快速折叠 | `boolean \| { start?: boolean; end?: boolean; showCollapsibleIcon?: boolean \| 'auto' }` | `false` | showCollapsibleIcon: 5.27.0 |
| defaultSize | 初始面板大小，支持数字 px 或者文字 '百分比%' 类型 | `number \| string` | - | - |
| destroyOnHidden | 折叠时（size 为 0）销毁面板内容，覆盖 Splitter 的 `destroyOnHidden` | `boolean` | - | 6.4.0 |
| max | 最大阈值，支持数字 px 或者文字 '百分比%' 类型 | `number \| string` | - | - |
| min | 最小阈值，支持数字 px 或者文字 '百分比%' 类型 | `number \| string` | - | - |
| resizable | 是否开启拖拽伸缩 | `boolean` | `true` | - |
| size | 受控面板大小，支持数字 px 或者文字 '百分比%' 类型 | `number \| string` | - | - |

### 导入方式

```js
import { Splitter } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `collapsible` | `motion` 是否开启折叠动画，`icon` 自定义折叠图标 | `{ motion?: boolean; icon?: { start?: ReactNode; end?: ReactNode } }` | - | 6.4.0 |
| `collapsibleIcon` | 折叠图标 | `{start?: ReactNode; end?: ReactNode}` | - | 6.0.0 |
| `destroyOnHidden` | 折叠时（size 为 0）销毁面板内容，应用于所有面板，可在单个面板上覆盖 | `boolean` | `false` | 6.4.0 |
| `draggerIcon` | 拖拽图标 | `ReactNode` | - | 6.0.0 |
| `layout` | 布局方向 | `horizontal` \| `vertical` | `horizontal` | - |
| `lazy` | 延迟渲染模式 | `boolean` | `false` | 5.23.0 |
| `onCollapse` | 展开-收起时回调 | `(collapsed: boolean[], sizes: number[]) => void` | - | 5.28.0 |
| `orientation` | 布局方向 | `horizontal` \| `vertical` | `horizontal` | - |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `vertical` | 排列方向，与 `orientation` 同时存在，以 `orientation` 优先 | boolean | `false` | — |
| `onDraggerDoubleClick` | 双击拖拽条回调 | `(index: number) => void` | - | 6.3.0 |
| `onResize` | 面板大小变化回调 | `(sizes: number[]) => void` | - | - |
| `onResizeEnd` | 拖拽结束回调 | `(sizes: number[]) => void` | - | - |
| `onResizeStart` | 开始拖拽之前回调 | `(sizes: number[]) => void` | - | - |
| `defaultSize` | 初始面板大小，支持数字 px 或者文字 '百分比%' 类型 | `number \| string` | - | - |
| `max` | 最大阈值，支持数字 px 或者文字 '百分比%' 类型 | `number \| string` | - | - |
| `min` | 最小阈值，支持数字 px 或者文字 '百分比%' 类型 | `number \| string` | - | - |
| `resizable` | 是否开启拖拽伸缩 | `boolean` | `true` | - |
| `size` | 受控面板大小，支持数字 px 或者文字 '百分比%' 类型 | `number \| string` | - | - |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Splitter** 的验收清单：

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
- 官方文档：https://ant.design/components/splitter
- 中文文档：https://ant.design/components/splitter-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/splitter
- 驱动 gpui kit：`splitter`
