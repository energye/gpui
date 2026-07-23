# Anchor 锚点
> 来源：[Ant Design 6.5.x Anchor](https://ant.design/components/anchor)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：导航（Navigation）  
> 说明：用于跳转到页面指定位置。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

用于跳转到页面指定位置。

**Anchor** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 横向 Anchor | 复现「横向 Anchor」视觉与布局 |
| 静态位置 | placement 方位 |
| 自定义 onClick 事件 | 自定义渲染/插槽外观 |
| 自定义锚点高亮 | 自定义渲染/插槽外观 |
| 设置锚点滚动偏移量 | 复现「设置锚点滚动偏移量」视觉与布局 |
| 监听锚点链接改变 | 复现「监听锚点链接改变」视觉与布局 |
| 替换历史中的 href | 复现「替换历史中的 href」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `direction`

- **说明**：设置导航方向
- **类型**：`vertical` | `horizontal`
- **默认值**：`vertical`
- **版本**：5.2.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `vertical` | 垂直排布 |
  | `horizontal` | 水平排布 |

#### `title`

- **说明**：文字内容
- **类型**：ReactNode
- **默认值**：-

#### `children`

- **说明**：嵌套的 Anchor Link，`注意：水平方向该属性不支持`
- **类型**：[AnchorItem](#anchoritem)\[]
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

需要展现当前页面上可供跳转的锚点链接，以及快速在锚点之间跳转。

> 开发者注意事项：
>
> 自 `4.24.0` 起，由于组件从 class 重构成 FC，之前一些获取 `ref` 并调用内部实例方法的写法都会失效

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **横向 Anchor**（`horizontal.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **静态位置**（`static.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **自定义 onClick 事件**（`onClick.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **自定义锚点高亮**（`customizeHighlight.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **设置锚点滚动偏移量**（`targetOffset.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **监听锚点链接改变**（`onChange.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **替换历史中的 href**（`replace.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onChange` | 值变化 | 监听锚点链接改变 |
| `onClick` | 点击 | `click` 事件的 handler |
| `items` | 数据化 items | 数据化配置选项内容，支持通过 children 嵌套 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 横向 Anchor | `horizontal.tsx` | 否 |
| 静态位置 | `static.tsx` | 否 |
| 自定义 onClick 事件 | `onClick.tsx` | 否 |
| 自定义锚点高亮 | `customizeHighlight.tsx` | 否 |
| 设置锚点滚动偏移量 | `targetOffset.tsx` | 否 |
| 每个链接单独的滚动偏移量 | `targetOffset-per-link.tsx` | 是 |
| 监听锚点链接改变 | `onChange.tsx` | 否 |
| 替换历史中的 href | `replace.tsx` | 否 |
| 废弃的 JSX 示例 | `legacy-anchor.tsx` | 是 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 组件 Token | `component-token.tsx` | 是 |

### 2.6 FAQ

## FAQ

### 在 `5.25.0+` 版本中，锚点跳转后，目标元素的 `:target` 伪类未按预期生效 {#faq-target-pseudo-class}

出于页面性能优化考虑，锚点跳转的实现方式从 `window.location.href` 调整为 `window.history.pushState/replaceState`。由于 `pushState/replaceState` 不会触发页面重载，因此浏览器不会自动更新 `:target` 伪类的匹配状态。可以手动构造完整URL：`href = window.location.origin + window.location.pathname + '#xxx'` 来解决这问题。

相关issues：[#53143](https://github.com/ant-design/ant-design/issues/53143) [#54255](https://github.com/ant-design/ant-design/issues/54255)

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

### Anchor Props

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| affix | 固定模式 | boolean \| Omit<AffixProps, 'offsetTop' \| 'target' \| 'children'> | true | object: 5.19.0 | × |
| bounds | 锚点区域边界 | number | 5 | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | getContainer | 指定滚动的容器 | () => HTMLElement | () => window | getCurrentAnchor | 自定义高亮的锚点 | (activeLink: string) => string | - | offsetTop | 距离窗口顶部达到指定偏移量后触发 | number | 0 | showInkInFixed | `affix={false}` 时是否显示小方块 | boolean | false | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | targetOffset | 锚点滚动偏移量，默认与 offsetTop 相同，[例子](#anchor-demo-targetoffset) | number | - | onChange | 监听锚点链接改变 | (currentActiveLink: string) => void | - | onClick | `click` 事件的 handler | (e: MouseEvent, link: object) => void | - | items | 数据化配置选项内容，支持通过 children 嵌套 | { key, href, title, target, children }\[] [具体见](#anchoritem) | - | 5.1.0 | × |
| direction | 设置导航方向 | `vertical` \| `horizontal` | `vertical` | 5.2.0 | × |
| replace | 替换浏览器历史记录中项目的 href 而不是推送它 | boolean | false | 5.7.0 | × |

### AnchorItem

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| key | 唯一标志 | string \| number | - | target | 该属性指定在何处显示链接的资源 | string | - | children | 嵌套的 Anchor Link，`注意：水平方向该属性不支持` | [AnchorItem](#anchoritem)\[] | - | targetOffset | 设置单个锚点的滚动偏移量，会覆盖 Anchor 组件的 targetOffset 属性 | number | - | 6.4.0 |

### Link Props

建议使用 items 形式。

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| href | 锚点链接 | string | - | title | 文字内容 | ReactNode | - 
### 导入方式

```js
import { Anchor } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `affix` | 固定模式 | boolean \| Omit | true | object: 5.19.0 |
| `bounds` | 锚点区域边界 | number | 5 | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `getContainer` | 指定滚动的容器 | () => HTMLElement | () => window | — |
| `getCurrentAnchor` | 自定义高亮的锚点 | (activeLink: string) => string | - | — |
| `offsetTop` | 距离窗口顶部达到指定偏移量后触发 | number | 0 | — |
| `showInkInFixed` | `affix={false}` 时是否显示小方块 | boolean | false | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `targetOffset` | 锚点滚动偏移量，默认与 offsetTop 相同，[例子](#anchor-demo-targetoffset) | number | - | — |
| `onChange` | 监听锚点链接改变 | (currentActiveLink: string) => void | - | — |
| `onClick` | `click` 事件的 handler | (e: MouseEvent, link: object) => void | - | — |
| `items` | 数据化配置选项内容，支持通过 children 嵌套 | { key, href, title, target, children }\[] [具体见](#anchoritem) | - | 5.1.0 |
| `direction` | 设置导航方向 | `vertical` \| `horizontal` | `vertical` | 5.2.0 |
| `replace` | 替换浏览器历史记录中项目的 href 而不是推送它 | boolean | false | 5.7.0 |
| `key` | 唯一标志 | string \| number | - | — |
| `href` | 锚点链接 | string | - | — |
| `target` | 该属性指定在何处显示链接的资源 | string | - | — |
| `title` | 文字内容 | ReactNode | - | — |
| `children` | 嵌套的 Anchor Link，`注意：水平方向该属性不支持` | [AnchorItem](#anchoritem)\[] | - | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Anchor** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **9** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/anchor
- 中文文档：https://ant.design/components/anchor-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/anchor
- 驱动 gpui kit：`anchor`
