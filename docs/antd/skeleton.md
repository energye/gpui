# Skeleton 骨架屏
> 来源：[Ant Design 6.5.x Skeleton](https://ant.design/components/skeleton)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：反馈（Feedback）  
> 说明：在需要等待加载内容的位置提供一个占位图形组合。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

在需要等待加载内容的位置提供一个占位图形组合。

**Skeleton** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 复杂的组合 | 复现「复杂的组合」视觉与布局 |
| 动画效果 | 复现「动画效果」视觉与布局 |
| 按钮/头像/输入框/图像/自定义节点 | 自定义渲染/插槽外观 |
| 包含子组件 | 复现「包含子组件」视觉与布局 |
| 列表 | 复现「列表」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `align`

- **说明**：请参考 [dom-align](https://github.com/yiminghe/dom-align) 进行配置
- **类型**：object
- **默认值**：-

#### `arrow`

- **说明**：修改箭头的显示状态以及修改箭头是否指向目标元素中心
- **类型**：boolean | { pointAtCenter: boolean }
- **默认值**：true
- **版本**：5.2.0

#### `autoAdjustOverflow`

- **说明**：气泡被遮挡时自动调整位置
- **类型**：boolean
- **默认值**：true

#### `color`

- **说明**：背景颜色
- **类型**：string
- **默认值**：-
- **版本**：4.3.0

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-
- **版本**：5.23.0

#### `overlayStyle`

- **说明**：卡片样式, 请使用 `styles.root` 替换
- **类型**：React.CSSProperties
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `styles.root` | 官方取值 `styles.root` |

#### `overlayInnerStyle`

- **说明**：卡片内容区域的样式对象, 请使用 `styles.container` 替换
- **类型**：React.CSSProperties
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `styles.container` | 官方取值 `styles.container` |

#### `placement`

- **说明**：气泡框位置，可选 `top` `left` `right` `bottom` `topLeft` `topRight` `bottomLeft` `bottomRight` `leftTop` `leftBottom` `rightTop` `rightBottom`
- **类型**：string
- **默认值**：`top`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `top` | 上方 |
  | `left` | 左侧 |
  | `right` | 右侧 |
  | `bottom` | 下方 |
  | `topLeft` | 上左 |
  | `topRight` | 上右 |
  | `bottomLeft` | 下左 |
  | `bottomRight` | 下右 |
  | `leftTop` | 左上 |
  | `leftBottom` | 左下 |
  | `rightTop` | 右上 |
  | `rightBottom` | 右下 |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-
- **版本**：5.23.0

#### `zIndex`

- **说明**：设置 Tooltip 的 `z-index`
- **类型**：number
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `z-index` | 官方取值 `z-index` |

#### `avatar`

- **说明**：是否显示头像占位图
- **类型**：boolean | [SkeletonAvatar](#skeletonavatar)
- **默认值**：false

#### `loading`

- **说明**：为 true 时，显示占位图。反之则直接展示子组件
- **类型**：boolean
- **默认值**：-

#### `round`

- **说明**：为 true 时，段落和标题显示圆角
- **类型**：boolean
- **默认值**：false

#### `title`

- **说明**：是否显示标题占位图
- **类型**：boolean | [SkeletonTitleProps](#skeletontitleprops)
- **默认值**：true

#### `width`

- **说明**：设置标题占位图的宽度
- **类型**：number | string
- **默认值**：-

#### `shape`

- **说明**：指定头像的形状
- **类型**：`circle` | `square`
- **默认值**：`circle`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `circle` | 圆形 |
  | `square` | 方形 |

#### `size`

- **说明**：设置头像占位图的大小
- **类型**：number | `large` | `medium` | `small`
- **默认值**：`medium`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `block`

- **说明**：将按钮宽度调整为其父宽度的选项
- **类型**：boolean
- **默认值**：false
- **版本**：4.17.0

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

- 网络较慢，需要长时间等待加载处理的情况下。
- 图文信息内容较多的列表/卡片中。
- 只在第一次加载数据的时候使用。
- 可以被 Spin 完全代替，但是在可用的场景下可以比 Spin 提供更好的视觉效果和用户体验。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **复杂的组合**（`complex.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **动画效果**（`active.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **按钮/头像/输入框/图像/自定义节点**（`element.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **包含子组件**（`children.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **列表**（`list.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `open` | 受控显隐 | 用于手动控制浮层显隐，小于 4.23.0 使用 `visible`（[为什么?](/docs/react/faq#弹层类组件为什么要统一至-open-属性)） |
| `onOpenChange` | 显隐变化 | 显示隐藏的回调 |
| `loading` | 加载中 | 为 true 时，显示占位图。反之则直接展示子组件 |
| `getPopupContainer` | 浮层容器 | 浮层渲染父节点，默认渲染到 body 上 |
| `destroyOnHidden` | 隐藏销毁 | 关闭后是否销毁 dom |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 复杂的组合 | `complex.tsx` | 否 |
| 动画效果 | `active.tsx` | 否 |
| 按钮/头像/输入框/图像/自定义节点 | `element.tsx` | 否 |
| 包含子组件 | `children.tsx` | 否 |
| 列表 | `list.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 自定义组件 Token | `componentToken.tsx` | 是 |

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

### 共同的 API

#### 继承 Tooltip 共同 API

<Antd component="Alert" title="以下 API 为 Tooltip、Popconfirm、Popover 共享的 API。" type="info" banner="true"></Antd>

<!-- prettier-ignore -->
| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| align | 请参考 [dom-align](https://github.com/yiminghe/dom-align) 进行配置 | object | - | arrow | 修改箭头的显示状态以及修改箭头是否指向目标元素中心 | boolean \| { pointAtCenter: boolean } | true | 5.2.0 | Tooltip: 6.0.0，Popover: 6.0.0，Popconfirm: 6.0.0 |
| autoAdjustOverflow | 气泡被遮挡时自动调整位置 | boolean | true | color | 背景颜色 | string | - | 4.3.0 | × |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> | - | 5.23.0 | Tooltip: 5.23.0，Popover: 5.23.0，Popconfirm: 5.23.0 |
| defaultOpen | 默认是否显隐 | boolean | false | 4.23.0 | × |
| ~~destroyTooltipOnHide~~ | 关闭后是否销毁 dom | boolean | false | destroyOnHidden | 关闭后是否销毁 dom | boolean | false | 5.25.0 | × |
| fresh | 默认情况下，Tooltip 在关闭时会缓存内容。设置该属性后会始终保持更新 | boolean | false | 5.10.0 | × |
| getPopupContainer | 浮层渲染父节点，默认渲染到 body 上 | (triggerNode: HTMLElement) => HTMLElement | () => document.body | mouseEnterDelay | 鼠标移入后延时多少才显示 Tooltip，单位：秒 | number | 0.1 | mouseLeaveDelay | 鼠标移出后延时多少才隐藏 Tooltip，单位：秒 | number | 0.1 | ~~overlayClassName~~ | 卡片类名, 请使用 `classNames.root` 替换 | string | - | ~~overlayStyle~~ | 卡片样式, 请使用 `styles.root` 替换| React.CSSProperties | - | ~~overlayInnerStyle~~ | 卡片内容区域的样式对象, 请使用 `styles.container` 替换 | React.CSSProperties | - | placement | 气泡框位置，可选 `top` `left` `right` `bottom` `topLeft` `topRight` `bottomLeft` `bottomRight` `leftTop` `leftBottom` `rightTop` `rightBottom` | string | `top` | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 5.23.0 | Tooltip: 5.23.0，Popover: 5.23.0，Popconfirm: 5.23.0 |
| trigger | 触发行为，可选 `hover` \| `focus` \| `click` \| `contextMenu`，可使用数组设置多个触发行为 | string \| string\[] | `hover` | open | 用于手动控制浮层显隐，小于 4.23.0 使用 `visible`（[为什么?](/docs/react/faq#弹层类组件为什么要统一至-open-属性)） | boolean | false | 4.23.0 | × |
| zIndex | 设置 Tooltip 的 `z-index` | number | - | onOpenChange | 显示隐藏的回调 | (open: boolean) => void | - | 4.23.0 | × |

</embed>

### Skeleton

| 属性 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| active | 是否展示动画效果 | boolean | false | avatar | 是否显示头像占位图 | boolean \| [SkeletonAvatar](#skeletonavatar) | false | loading | 为 true 时，显示占位图。反之则直接展示子组件 | boolean | - | paragraph | 是否显示段落占位图 | boolean \| [SkeletonParagraphProps](#skeletonparagraphprops) | true | round | 为 true 时，段落和标题显示圆角 | boolean | false | title | 是否显示标题占位图 | boolean \| [SkeletonTitleProps](#skeletontitleprops) | true 
#### SkeletonTitleProps

| 属性  | 说明                 | 类型             | 默认值 |
| ----- | -------------------- | ---------------- | ------ |
| width | 设置标题占位图的宽度 | number \| string | -      |

#### SkeletonParagraphProps

| 属性 | 说明 | 类型 | 默认值 |
| --- | --- | --- | --- |
| rows | 设置段落占位图的行数 | number | - |
| width | 设置段落占位图的宽度，若为数组时则为对应的每行宽度，反之则是最后一行的宽度 | number \| string \| Array&lt;number \| string> | - |

### Skeleton.Avatar

| 属性 | 说明 | 类型 | 默认值 |
| --- | --- | --- | --- |
| active | 是否展示动画效果，只在独立使用头像时有效 | boolean | false |
| shape | 指定头像的形状 | `circle` \| `square` | `circle` |
| size | 设置头像占位图的大小 | number \| `large` \| `medium` \| `small` | `medium` |

### Skeleton.Button

| 属性 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| active | 是否展示动画效果 | boolean | false | shape | 指定按钮的形状 | `circle` \| `round` \| `square` \| `default` | - 
### Skeleton.Input

| 属性   | 说明             | 类型                           | 默认值   |
| ------ | ---------------- | ------------------------------ | -------- |
| active | 是否展示动画效果 | boolean                        | false    |
| size   | 设置输入框的大小 | `large` \| `medium` \| `small` | `medium` |

### 导入方式

```js
import { Skeleton } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `align` | 请参考 [dom-align](https://github.com/yiminghe/dom-align) 进行配置 | object | - | — |
| `arrow` | 修改箭头的显示状态以及修改箭头是否指向目标元素中心 | boolean \| { pointAtCenter: boolean } | true | 5.2.0 |
| `autoAdjustOverflow` | 气泡被遮挡时自动调整位置 | boolean | true | — |
| `color` | 背景颜色 | string | - | 4.3.0 |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props }) => Record | - | 5.23.0 |
| `defaultOpen` | 默认是否显隐 | boolean | false | 4.23.0 |
| `destroyTooltipOnHide` | 关闭后是否销毁 dom | boolean | false | — |
| `destroyOnHidden` | 关闭后是否销毁 dom | boolean | false | 5.25.0 |
| `fresh` | 默认情况下，Tooltip 在关闭时会缓存内容。设置该属性后会始终保持更新 | boolean | false | 5.10.0 |
| `getPopupContainer` | 浮层渲染父节点，默认渲染到 body 上 | (triggerNode: HTMLElement) => HTMLElement | () => document.body | — |
| `mouseEnterDelay` | 鼠标移入后延时多少才显示 Tooltip，单位：秒 | number | 0.1 | — |
| `mouseLeaveDelay` | 鼠标移出后延时多少才隐藏 Tooltip，单位：秒 | number | 0.1 | — |
| `overlayClassName` | 卡片类名, 请使用 `classNames.root` 替换 | string | - | — |
| `overlayStyle` | 卡片样式, 请使用 `styles.root` 替换 | React.CSSProperties | - | — |
| `overlayInnerStyle` | 卡片内容区域的样式对象, 请使用 `styles.container` 替换 | React.CSSProperties | - | — |
| `placement` | 气泡框位置，可选 `top` `left` `right` `bottom` `topLeft` `topRight` `bottomLeft` `bottomRight` `leftTop` `leftBottom` `rightTop` `rightBottom` | string | `top` | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props }) => Record | - | 5.23.0 |
| `trigger` | 触发行为，可选 `hover` \| `focus` \| `click` \| `contextMenu`，可使用数组设置多个触发行为 | string \| string\[] | `hover` | — |
| `open` | 用于手动控制浮层显隐，小于 4.23.0 使用 `visible`（[为什么?](/docs/react/faq#弹层类组件为什么要统一至-open-属性)） | boolean | false | 4.23.0 |
| `zIndex` | 设置 Tooltip 的 `z-index` | number | - | — |
| `onOpenChange` | 显示隐藏的回调 | (open: boolean) => void | - | 4.23.0 |
| `active` | 是否展示动画效果 | boolean | false | — |
| `avatar` | 是否显示头像占位图 | boolean \| [SkeletonAvatar](#skeletonavatar) | false | — |
| `loading` | 为 true 时，显示占位图。反之则直接展示子组件 | boolean | - | — |
| `paragraph` | 是否显示段落占位图 | boolean \| [SkeletonParagraphProps](#skeletonparagraphprops) | true | — |
| `round` | 为 true 时，段落和标题显示圆角 | boolean | false | — |
| `title` | 是否显示标题占位图 | boolean \| [SkeletonTitleProps](#skeletontitleprops) | true | — |
| `width` | 设置标题占位图的宽度 | number \| string | - | — |
| `rows` | 设置段落占位图的行数 | number | - | — |
| `shape` | 指定头像的形状 | `circle` \| `square` | `circle` | — |
| `size` | 设置头像占位图的大小 | number \| `large` \| `medium` \| `small` | `medium` | — |
| `block` | 将按钮宽度调整为其父宽度的选项 | boolean | false | 4.17.0 |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Skeleton** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **7** 个，均需可复现。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/skeleton
- 中文文档：https://ant.design/components/skeleton-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/skeleton
- 驱动 gpui kit：`skeleton`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Skeleton** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/skeleton/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Skeleton）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 展示/自动关闭/堆叠/类型语义 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Skeleton）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：在需要等待加载内容的位置提供一个占位图形组合。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**反馈**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `active` | 是否展示动画效果 | boolean | false |
| `avatar` | 是否显示头像占位图 | boolean \ | [SkeletonAvatar](#skeletonavatar) |
| `loading` | 为 true 时，显示占位图。反之则直接展示子组件 | boolean | - |
| `paragraph` | 是否显示段落占位图 | boolean \ | [SkeletonParagraphProps](#skeletonparagraphprops) |
| `round` | 为 true 时，段落和标题显示圆角 | boolean | false |
| `title` | 是否显示标题占位图 | boolean \ | [SkeletonTitleProps](#skeletontitleprops) |
| `width` | 设置标题占位图的宽度 | number \ | string |
| `rows` | 设置段落占位图的行数 | number | - |
| `shape` | 指定头像的形状 | `circle` \ | `square` |
| `size` | 设置头像占位图的大小 | number \ | `large` \ |
| `block` | 将按钮宽度调整为其父宽度的选项 | boolean | false |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
loading=true ──► 骨架
loading=false ──► children
active ──► 闪烁动画
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| SKL-S1 | loading 骨架 | 灰块可见 |
| SKL-S2 | loading false | children |
| SKL-S3 | active | 动画 |
| SKL-S4 | avatar+paragraph | 结构 |
| SKL-S5 | paragraph rows=4 | 4 行 |
| SKL-S6 | reduced-motion | 可停动画 |
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
| 实时区域 | message/notification 用 status 语义等价 |
| 关闭 | 可关控件可操作 |
| 不抢焦点 | 轻提示默认不抢（Modal 例外） |

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
| `loading` | 必须 |
| `size` | 必须 |
| `title` | 必须 |
| `shape` | 必须 |
| 官方主路径示例 | 基本、复杂的组合、动画效果、按钮/头像/输入框/图像/自定义节点、包含子组件、列表、自定义语义结构的样式和类、_semantic.tsx |
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
| 其余示例 | _semantic_element.tsx |

### 6.9 验收用例表（可测）

> 测试名建议：`TestSkeleton_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Skeleton 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| SKL-01 | L1 | NewSkeleton 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| SKL-02 | L1 | loading 骨架 | 灰块可见 |
| SKL-03 | L1 | loading false | children |
| SKL-04 | L1 | active | 动画 |
| SKL-05 | L1 | avatar+paragraph | 结构 |
| SKL-06 | L1 | paragraph rows=4 | 4 行 |
| SKL-07 | L1 | reduced-motion | 可停动画 |
| SKL-08 | L1 | 复现官方示例「基本」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SKL-09 | L1 | 复现官方示例「复杂的组合」（`complex.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SKL-10 | L1 | 复现官方示例「动画效果」（`active.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SKL-11 | L1 | 复现官方示例「按钮/头像/输入框/图像/自定义节点」（`element.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SKL-12 | L1 | 复现官方示例「包含子组件」（`children.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SKL-13 | L1 | 复现官方示例「列表」（`list.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SKL-14 | L1 | 复现官方示例「自定义语义结构的样式和类」（`style-class.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SKL-15 | L1 | 复现官方示例「_semantic.tsx」（`_semantic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SKL-16 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| SKL-17 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| SKL-18 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| SKL-19 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| SKL-20 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| SKL-21 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| SKL-22 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewSkeleton(...) *Skeleton

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
Host holder or inline
  └─ item (icon + content + close?)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Skeleton 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. gallery 展示主路径（对照官方非 debug 示例与 P0）。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/skeleton.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Skeleton 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
