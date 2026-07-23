# Flex 弹性布局
> 来源：[Ant Design 6.5.x Flex](https://ant.design/components/flex)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：布局（Layout）  
> 说明：用于对齐的弹性布局容器。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

用于对齐的弹性布局容器。

**Flex** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本布局 | 复现「基本布局」视觉与布局 |
| 对齐方式 | 复现「对齐方式」视觉与布局 |
| 设置间隙 | 复现「设置间隙」视觉与布局 |
| 自动换行 | 复现「自动换行」视觉与布局 |
| 组合使用 | 复现「组合使用」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `vertical`

- **说明**：flex 主轴的方向是否垂直，使用 `flex-direction: column`
- **类型**：boolean
- **默认值**：false
- **版本**：5.10.0

#### `wrap`

- **说明**：设置元素单行显示还是多行显示
- **类型**：[flex-wrap](https://developer.mozilla.org/zh-CN/docs/Web/CSS/flex-wrap) | boolean
- **默认值**：nowrap
- **版本**：boolean: 5.17.0

#### `justify`

- **说明**：设置元素在主轴方向上的对齐方式
- **类型**：[justify-content](https://developer.mozilla.org/zh-CN/docs/Web/CSS/justify-content)
- **默认值**：normal

#### `align`

- **说明**：设置元素在交叉轴方向上的对齐方式
- **类型**：[align-items](https://developer.mozilla.org/zh-CN/docs/Web/CSS/align-items)
- **默认值**：normal

#### `gap`

- **说明**：设置网格之间的间隙
- **类型**：`small` | `medium` | `large` | string | number
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `small` | 小尺寸（更紧凑） |
  | `medium` | 中尺寸（默认节奏） |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |

#### `component`

- **说明**：自定义元素类型
- **类型**：React.ComponentType
- **默认值**：`div`

#### `orientation`

- **说明**：主轴的方向类型
- **类型**：`horizontal` | `vertical`
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |
  | `vertical` | 垂直排布 |

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

- 至少区分根容器、内容区、装饰/图标区；浮层再分 popup/mask。

- 颜色、圆角、间距、动效走 Design Token；支持亮暗色与品牌色。

- 动效可关（reduced-motion / 全局 motion、wave 配置）。
---
## 2. 功能
### 2.1 使用场景

- 适合设置元素之间的间距。
- 适合设置各种水平、垂直对齐方式。

### 与 Space 组件的区别 {#difference-with-space-component}

- Space 为内联元素提供间距，其本身会为每一个子元素添加包裹元素用于内联对齐。适用于行、列中多个子元素的等距排列。
- Flex 为块级元素提供间距，其本身不会添加包裹元素。适用于垂直或水平方向上的子元素布局，并提供了更多的灵活性和控制能力。

### 2.2 核心功能（按官方示例拆解）

1. **基本布局**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **对齐方式**（`align.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **设置间隙**（`gap.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **自动换行**（`wrap.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **组合使用**（`combination.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本布局 | `basic.tsx` | 否 |
| 对齐方式 | `align.tsx` | 否 |
| 设置间隙 | `gap.tsx` | 否 |
| 自动换行 | `wrap.tsx` | 否 |
| 组合使用 | `combination.tsx` | 否 |
| 调试专用 | `debug.tsx` | 是 |

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

> 自 `antd@5.10.0` 版本开始提供该组件。Flex 组件默认行为在水平模式下，为向上对齐，在垂直模式下，为拉伸对齐，你可以通过属性进行调整。

通用属性参考：[通用属性](/docs/react/common-props)

| 属性 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| vertical | flex 主轴的方向是否垂直，使用 `flex-direction: column` | boolean | false | 5.10.0 | 5.10.0 |
| wrap | 设置元素单行显示还是多行显示 | [flex-wrap](https://developer.mozilla.org/zh-CN/docs/Web/CSS/flex-wrap) \| boolean | nowrap | boolean: 5.17.0 | × |
| justify | 设置元素在主轴方向上的对齐方式 | [justify-content](https://developer.mozilla.org/zh-CN/docs/Web/CSS/justify-content) | normal | align | 设置元素在交叉轴方向上的对齐方式 | [align-items](https://developer.mozilla.org/zh-CN/docs/Web/CSS/align-items) | normal | flex | flex CSS 简写属性 | [flex](https://developer.mozilla.org/zh-CN/docs/Web/CSS/flex) | normal | gap | 设置网格之间的间隙 | `small` \| `medium` \| `large` \| string \| number | - | component | 自定义元素类型 | React.ComponentType | `div` | orientation | 主轴的方向类型 | `horizontal` \| `vertical` | `horizontal` | - | × |

### 导入方式

```js
import { Flex } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `vertical` | flex 主轴的方向是否垂直，使用 `flex-direction: column` | boolean | false | 5.10.0 |
| `wrap` | 设置元素单行显示还是多行显示 | [flex-wrap](https://developer.mozilla.org/zh-CN/docs/Web/CSS/flex-wrap) \| boolean | nowrap | boolean: 5.17.0 |
| `justify` | 设置元素在主轴方向上的对齐方式 | [justify-content](https://developer.mozilla.org/zh-CN/docs/Web/CSS/justify-content) | normal | — |
| `align` | 设置元素在交叉轴方向上的对齐方式 | [align-items](https://developer.mozilla.org/zh-CN/docs/Web/CSS/align-items) | normal | — |
| `flex` | flex CSS 简写属性 | [flex](https://developer.mozilla.org/zh-CN/docs/Web/CSS/flex) | normal | — |
| `gap` | 设置网格之间的间隙 | `small` \| `medium` \| `large` \| string \| number | - | — |
| `component` | 自定义元素类型 | React.ComponentType | `div` | — |
| `orientation` | 主轴的方向类型 | `horizontal` \| `vertical` | `horizontal` | - |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Flex** 的验收清单：

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
- 官方文档：https://ant.design/components/flex
- 中文文档：https://ant.design/components/flex-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/flex
- 驱动 gpui kit：`flex`
