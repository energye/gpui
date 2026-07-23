# Layout 布局
> 来源：[Ant Design 6.5.x Layout](https://ant.design/components/layout)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：布局（Layout）  
> 说明：协助进行页面级整体布局。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

协助进行页面级整体布局。

**Layout** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本结构 | 复现「基本结构」视觉与布局 |
| 上中下布局 | 复现「上中下布局」视觉与布局 |
| 顶部-侧边布局 | 复现「顶部-侧边布局」视觉与布局 |
| 顶部-侧边布局-通栏 | 复现「顶部-侧边布局-通栏」视觉与布局 |
| 侧边布局 | 复现「侧边布局」视觉与布局 |
| 自定义触发器 | 自定义渲染/插槽外观 |
| 折叠覆盖布局 | 复现「折叠覆盖布局」视觉与布局 |
| 响应式布局 | 断点响应式 |
| 固定头部 | 固定头/列/侧栏 |
| 固定侧边栏 | 固定头/列/侧栏 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `hasSider`

- **说明**：表示子元素里有 Sider，一般不用指定。可用于服务端渲染时避免样式闪动
- **类型**：boolean
- **默认值**：-

#### `classNames`

- **说明**：用于自定义 Sider 组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `collapsed`

- **说明**：当前收起状态
- **类型**：boolean
- **默认值**：-

#### `reverseArrow`

- **说明**：翻转折叠提示箭头的方向，当 Sider 在右边时可以使用
- **类型**：boolean
- **默认值**：false

#### `styles`

- **说明**：用于自定义 Sider 组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `theme`

- **说明**：主题颜色
- **类型**：`light` | `dark`
- **默认值**：`dark`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `light` | 浅色主题 |
  | `dark` | 深色主题 |

#### `width`

- **说明**：宽度
- **类型**：number | string
- **默认值**：200

#### `zeroWidthTriggerStyle`

- **说明**：指定当 `collapsedWidth` 为 0 时出现的特殊 trigger 的样式
- **类型**：object
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

实现与 antd **Layout** 对等的业务能力。

### 2.2 核心功能（按官方示例拆解）

1. **基本结构**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **上中下布局**（`top.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **顶部-侧边布局**（`top-side.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **顶部-侧边布局-通栏**（`top-side-2.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **侧边布局**（`side.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **自定义触发器**（`custom-trigger.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **折叠覆盖布局**（`collapsible-overlay.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **响应式布局**（`responsive.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **固定头部**（`fixed.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **固定侧边栏**（`fixed-sider.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本结构 | `basic.tsx` | 否 |
| 上中下布局 | `top.tsx` | 否 |
| 顶部-侧边布局 | `top-side.tsx` | 否 |
| 顶部-侧边布局-通栏 | `top-side-2.tsx` | 否 |
| 侧边布局 | `side.tsx` | 否 |
| 自定义触发器 | `custom-trigger.tsx` | 否 |
| 折叠覆盖布局 | `collapsible-overlay.tsx` | 否 |
| 响应式布局 | `responsive.tsx` | 否 |
| 固定头部 | `fixed.tsx` | 否 |
| 固定侧边栏 | `fixed-sider.tsx` | 否 |
| 自定义触发器 Debug | `custom-trigger-debug.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |

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

```jsx
<Layout>
  <Header>header</Header>
  <Layout>
    <Sider>left sidebar</Sider>
    <Content>main content</Content>
    <Sider>right sidebar</Sider>
  </Layout>
  <Footer>footer</Footer>
</Layout>
```

### Layout

通用属性参考：[通用属性](/docs/react/common-props)

布局容器。

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| hasSider | 表示子元素里有 Sider，一般不用指定。可用于服务端渲染时避免样式闪动 | boolean | - 
### Layout.Sider

侧边栏。

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| breakpoint | 触发响应式布局的[断点](/components/grid-cn#col) | `xs` \| `sm` \| `md` \| `lg` \| `xl` \| `xxl` \| `xxxl` | - | xxxl: 6.3.0 |
| classNames | 用于自定义 Sider 组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | collapsedWidth | 收缩宽度，设置为 0 会出现特殊 trigger | number | 80 | defaultCollapsed | 是否默认收起 | boolean | false | styles | 用于自定义 Sider 组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | trigger | 自定义 trigger，设置为 null 时隐藏 trigger | ReactNode | - | zeroWidthTriggerStyle | 指定当 `collapsedWidth` 为 0 时出现的特殊 trigger 的样式 | object | - | onCollapse | 展开-收起时的回调函数，有点击 trigger 以及响应式反馈两种方式可以触发 | (collapsed, type) => {} | - 
```js
import { Layout } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `hasSider` | 表示子元素里有 Sider，一般不用指定。可用于服务端渲染时避免样式闪动 | boolean | - | — |
| `breakpoint` | 触发响应式布局的[断点](/components/grid-cn#col) | `xs` \| `sm` \| `md` \| `lg` \| `xl` \| `xxl` \| `xxxl` | - | xxxl: 6.3.0 |
| `classNames` | 用于自定义 Sider 组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `collapsed` | 当前收起状态 | boolean | - | — |
| `collapsedWidth` | 收缩宽度，设置为 0 会出现特殊 trigger | number | 80 | — |
| `collapsible` | 是否可收起 | boolean | false | — |
| `defaultCollapsed` | 是否默认收起 | boolean | false | — |
| `reverseArrow` | 翻转折叠提示箭头的方向，当 Sider 在右边时可以使用 | boolean | false | — |
| `styles` | 用于自定义 Sider 组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `theme` | 主题颜色 | `light` \| `dark` | `dark` | — |
| `trigger` | 自定义 trigger，设置为 null 时隐藏 trigger | ReactNode | - | — |
| `width` | 宽度 | number \| string | 200 | — |
| `zeroWidthTriggerStyle` | 指定当 `collapsedWidth` 为 0 时出现的特殊 trigger 的样式 | object | - | — |
| `onBreakpoint` | 触发响应式布局[断点](/components/grid-cn#api)时的回调 | (broken) => {} | - | — |
| `onCollapse` | 展开-收起时的回调函数，有点击 trigger 以及响应式反馈两种方式可以触发 | (collapsed, type) => {} | - | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Layout** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **10** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/layout
- 中文文档：https://ant.design/components/layout-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/layout
- 驱动 gpui kit：`layout`
