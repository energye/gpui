# Tooltip 文字提示
> 来源：[Ant Design 6.5.x Tooltip](https://ant.design/components/tooltip)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：简单的文字提示气泡框。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

简单的文字提示气泡框。

**Tooltip** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 平滑过渡 | 复现「平滑过渡」视觉与布局 |
| 位置 | placement 方位 |
| 箭头展示 | arrow 指示 |
| 贴边偏移 | 复现「贴边偏移」视觉与布局 |
| 多彩文字提示 | 复现「多彩文字提示」视觉与布局 |
| 禁用 | disabled 灰态与不可点 |
| 自定义子组件 | 自定义渲染/插槽外观 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `title`

- **说明**：提示文字
- **类型**：ReactNode | () => ReactNode
- **默认值**：-

#### `color`

- **说明**：设置背景颜色，使用该属性后内部文字颜色将自适应
- **类型**：string
- **默认值**：-
- **版本**：5.27.0

#### `classNames`

- **说明**：语义化结构 class
- **类型**：Record | (info: { props }) => Record
- **默认值**：-
- **版本**：5.23.0

#### `styles`

- **说明**：语义化结构 style
- **类型**：Record | (info: { props }) => Record
- **默认值**：-
- **版本**：5.23.0

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

#### `zIndex`

- **说明**：设置 Tooltip 的 `z-index`
- **类型**：number
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `z-index` | 官方取值 `z-index` |

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

鼠标移入则显示提示，移出消失，气泡浮层不承载复杂文本和操作。

可用来代替系统默认的 `title` 提示，提供一个 `按钮/文字/操作` 的文案解释。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **平滑过渡**（`smooth-transition.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **位置**（`placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **箭头展示**（`arrow.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **贴边偏移**（`shift.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **多彩文字提示**（`colorful.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **禁用**（`disabled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **自定义子组件**（`wrap-custom-component.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `open` | 受控显隐 | 用于手动控制浮层显隐，小于 4.23.0 使用 `visible`（[为什么?](/docs/react/faq#弹层类组件为什么要统一至-open-属性)） |
| `onOpenChange` | 显隐变化 | 显示隐藏的回调 |
| `getPopupContainer` | 浮层容器 | 浮层渲染父节点，默认渲染到 body 上 |
| `destroyOnHidden` | 隐藏销毁 | 关闭后是否销毁 dom |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 平滑过渡 | `smooth-transition.tsx` | 否 |
| 位置 | `placement.tsx` | 否 |
| 箭头展示 | `arrow.tsx` | 否 |
| 贴边偏移 | `shift.tsx` | 否 |
| 自动调整位置 | `auto-adjust-overflow.tsx` | 是 |
| 隐藏后销毁 | `destroy-on-close.tsx` | 是 |
| 多彩文字提示 | `colorful.tsx` | 否 |
| _InternalPanelDoNotUseOrYouWillBeFired | `render-panel.tsx` | 是 |
| Debug | `debug.tsx` | 是 |
| 禁用 | `disabled.tsx` | 否 |
| 禁用子元素 | `disabled-children.tsx` | 是 |
| 自定义子组件 | `wrap-custom-component.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |

### 2.6 FAQ

## FAQ

### 为何有时候 HOC 组件无法生效？ {#faq-hoc-component}

请确保 `Tooltip` 的子元素能接受 `onMouseEnter`、`onMouseLeave`、`onPointerEnter`、`onPointerLeave`、`onFocus`、`onClick` 事件。

请查看 https://github.com/ant-design/ant-design/issues/15909

### 为何 Tooltip 的内容在关闭时不会更新？ {#faq-content-not-update}

Tooltip 默认在关闭时会缓存内容，以防止内容更新时出现闪烁：

```jsx
// `title` 不会因为 `user` 置空而闪烁置空

```

如果需要在关闭时也更新内容，可以设置 `fresh` 属性（例如 [#44830](https://github.com/ant-design/ant-design/issues/44830) 中的场景）：

```jsx

```

---

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

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| title | 提示文字 | ReactNode \| () => ReactNode | - | - | × |
| color | 设置背景颜色，使用该属性后内部文字颜色将自适应 | string | - | 5.27.0 | × |
| classNames | 语义化结构 class | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> | - | 5.23.0 | 5.23.0 |
| styles | 语义化结构 style | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 5.23.0 | 5.23.0 |

### 共同的 API

#### 继承 Tooltip 共同 API

<Antd component="Alert" title="以下 API 为 Tooltip、Popconfirm、Popover 共享的 API。" type="info" banner="true"></Antd>

<!-- prettier-ignore -->
| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| align | 请参考 [dom-align](https://github.com/yiminghe/dom-align) 进行配置 | object | - |  | × |
| arrow | 修改箭头的显示状态以及修改箭头是否指向目标元素中心 | boolean \| { pointAtCenter: boolean } | true | 5.2.0 | Tooltip: 6.0.0，Popover: 6.0.0，Popconfirm: 6.0.0 |
| autoAdjustOverflow | 气泡被遮挡时自动调整位置 | boolean | true |  | × |
| color | 背景颜色 | string | - | 4.3.0 | × |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> | - | 5.23.0 | Tooltip: 5.23.0，Popover: 5.23.0，Popconfirm: 5.23.0 |
| defaultOpen | 默认是否显隐 | boolean | false | 4.23.0 | × |
| ~~destroyTooltipOnHide~~ | 关闭后是否销毁 dom | boolean | false |  | × |
| destroyOnHidden | 关闭后是否销毁 dom | boolean | false | 5.25.0 | × |
| fresh | 默认情况下，Tooltip 在关闭时会缓存内容。设置该属性后会始终保持更新 | boolean | false | 5.10.0 | × |
| getPopupContainer | 浮层渲染父节点，默认渲染到 body 上 | (triggerNode: HTMLElement) => HTMLElement | () => document.body |  | × |
| mouseEnterDelay | 鼠标移入后延时多少才显示 Tooltip，单位：秒 | number | 0.1 |  | × |
| mouseLeaveDelay | 鼠标移出后延时多少才隐藏 Tooltip，单位：秒 | number | 0.1 |  | × |
| ~~overlayClassName~~ | 卡片类名, 请使用 `classNames.root` 替换 | string | - |  | × |
| ~~overlayStyle~~ | 卡片样式, 请使用 `styles.root` 替换 | React.CSSProperties | - |  | × |
| ~~overlayInnerStyle~~ | 卡片内容区域的样式对象, 请使用 `styles.container` 替换 | React.CSSProperties | - |  | × |
| placement | 气泡框位置，可选 `top` `left` `right` `bottom` `topLeft` `topRight` `bottomLeft` `bottomRight` `leftTop` `leftBottom` `rightTop` `rightBottom` | string | `top` |  | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 5.23.0 | Tooltip: 5.23.0，Popover: 5.23.0，Popconfirm: 5.23.0 |
| trigger | 触发行为，可选 `hover` \| `focus` \| `click` \| `contextMenu`，可使用数组设置多个触发行为 | string \| string\[] | `hover` |  | × |
| open | 用于手动控制浮层显隐，小于 4.23.0 使用 `visible`（[为什么?](/docs/react/faq#弹层类组件为什么要统一至-open-属性)） | boolean | false | 4.23.0 | × |
| zIndex | 设置 Tooltip 的 `z-index` | number | - |  | × |
| onOpenChange | 显示隐藏的回调 | (open: boolean) => void | - | 4.23.0 | × |

</embed>

### ConfigProvider - tooltip.unique {#config-provider-tooltip-unique}

可以通过 ConfigProvider 全局配置 Tooltip 的唯一性显示。当 `unique` 设置为 `true` 时，同一时间 ConfigProvider 下的 Tooltip 只会显示一个，提供更好的用户体验和平滑的过渡效果。

注意：配置后 `getContainer`、`arrow` 等属性将会失效。

```tsx
import { Button, ConfigProvider, Space, Tooltip } from 'antd';

export default () => (
  <ConfigProvider
    tooltip={{
      unique: true,
    }}
  >
    <Space>
      <Tooltip title="第一个提示">
        <Button>按钮 1</Button>
      </Tooltip>
      <Tooltip title="第二个提示">
        <Button>按钮 2</Button>
      </Tooltip>
    </Space>
  </ConfigProvider>
);
```

### 导入方式

```js
import { Tooltip } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `title` | 提示文字 | ReactNode \| () => ReactNode | - | - |
| `color` | 设置背景颜色，使用该属性后内部文字颜色将自适应 | string | - | 5.27.0 |
| `classNames` | 语义化结构 class | Record \| (info: { props }) => Record | - | 5.23.0 |
| `styles` | 语义化结构 style | Record \| (info: { props }) => Record | - | 5.23.0 |
| `align` | 请参考 [dom-align](https://github.com/yiminghe/dom-align) 进行配置 | object | - | — |
| `arrow` | 修改箭头的显示状态以及修改箭头是否指向目标元素中心 | boolean \| { pointAtCenter: boolean } | true | 5.2.0 |
| `autoAdjustOverflow` | 气泡被遮挡时自动调整位置 | boolean | true | — |
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
| `trigger` | 触发行为，可选 `hover` \| `focus` \| `click` \| `contextMenu`，可使用数组设置多个触发行为 | string \| string\[] | `hover` | — |
| `open` | 用于手动控制浮层显隐，小于 4.23.0 使用 `visible`（[为什么?](/docs/react/faq#弹层类组件为什么要统一至-open-属性)） | boolean | false | 4.23.0 |
| `zIndex` | 设置 Tooltip 的 `z-index` | number | - | — |
| `onOpenChange` | 显示隐藏的回调 | (open: boolean) => void | - | 4.23.0 |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Tooltip** 的验收清单：

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
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/tooltip
- 中文文档：https://ant.design/components/tooltip-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/tooltip
- 驱动 gpui kit：`tooltip`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Tooltip** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/tooltip/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Tooltip）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | hover/focus/click 开合、空 title 不显示、placement 12 向、arrow、color、delay、受控 open | Headless / behavior 测试 |
| **L2** | Token / 几何 | padding / 圆角 / 字号 / maxWidth / 默认皮色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Tooltip）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：简单的文字提示气泡框（悬停/聚焦/点击触发的黑底反白 tip）。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`、`borderRadius=6`）。  
源码：`components/tooltip/style`（`prepareComponentToken` + `genTooltipStyle`）。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 字号 | **14** | `fontSize` |
| 圆角 | **6** | `borderRadius`（`tooltipBorderRadius`） |
| 水平 padding | **8** | antd `paddingXS`；kit 常量 `DefaultTooltipPaddingX` |
| 垂直 padding | **6** | antd `paddingSM / 2`；kit 常量 `DefaultTooltipPaddingY` |
| 内容 minHeight | **32** | `controlHeight` |
| 最大宽度 | **250** | 组件 Token `maxWidth` |
| 箭头边长 | **8** | `sizePopupArrow / 2` 近似；kit `DefaultTooltipArrowSize` |
| 锚点间距 Gap | **8** | 与箭头半高同量级；`AnchoredPopup.Gap` |
| z-index | **1070** 档 | antd `zIndexPopupBase + 70`；kit `DefaultTooltipPortalZ` |
| Focus ring outset | ≈ **1.5px** 可见 | 触发器可聚焦时必须可见 |
| mouseEnterDelay | **0.1 s** | antd 默认；Ticker 累计 |
| mouseLeaveDelay | **0.1 s** | antd 默认；Ticker 累计 |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token / 回落 | 备注 |
| --- | --- | --- |
| 默认气泡底 | `colorBgSpotlight`（未进 Theme 时回落 `rgba(0,0,0,0.85)`） | antd light seed = textBase @ 0.85 |
| 默认气泡字 | `colorTextInverse`（= `colorTextLightSolid`） | 反白 |
| `color` 预设 | 预设 solid/dark 色（与 Tag 预设同源） | pink/red/…/lime；字色自适应反白 |
| `color` 自定义 | `#hex` | 底色 = 该色；字色按亮度自适应 |
| 禁用触发 | `colorDisabledBg` / `colorDisabledText` | 触发器禁用皮；tip 不打开 |

禁止硬编码品牌主色作为唯一默认皮（默认黑底走 spotlight / 常量回落，非 primary）。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据展示**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `title` | 提示文字；**空 / nil 时不显示**（antd 官方禁用 demo 即用此语义） | string \| Node | — |
| `color` | 背景色（预设名或 `#hex`）；设后字色自适应 | string | —（spotlight 默认皮） |
| `placement` | 12 向：`top` / `topLeft` / … / `rightBottom` | enum | `top` |
| `arrow` | 显示箭头；`pointAtCenter` 时边角贴中 | bool \| { pointAtCenter } | true |
| `trigger` | `hover` \| `focus` \| `click` \| `contextMenu`（可多选） | enum[] | `[hover]` |
| `open` / `defaultOpen` | 受控 / 非受控显隐 | bool | false |
| `onOpenChange` | 显隐回调 | `(open bool)` | — |
| `mouseEnterDelay` / `mouseLeaveDelay` | 开/关延迟（秒） | float64 | 0.1 / 0.1 |
| `autoAdjustOverflow` | 贴边 flip/shift | bool | true |
| `zIndex` | 浮层 z-order | int | 组件默认档 |
| `classNames` / `styles` | 语义化钩子 | Record | —（**P1**） |

**配置优先级：** 受控 `open` > 显式 `defaultOpen` > 组件默认 > ConfigProvider 全局（P1）。

**12 向 placement 映射**（主方位 + 边角对齐；`pointAtCenter=true` 时边角方位贴主轴中点）：

| placement | 面板在锚点 | 对齐 |
| --- | --- | --- |
| `top` / `topLeft` / `topRight` | 上 | 中 / 左 / 右 |
| `bottom` / `bottomLeft` / `bottomRight` | 下 | 中 / 左 / 右 |
| `left` / `leftTop` / `leftBottom` | 左 | 中 / 上 / 下 |
| `right` / `rightTop` / `rightBottom` | 右 | 中 / 上 / 下 |

### 6.4 交互状态机（L1）

```text
                     ┌─ title 空 / Disabled ──► 永远 closed（TIP-S5）
                     │
  mount ──► closed ──┤
                     │  hover（delay enter）/ focus / click / contextMenu
                     ▼
                   open tip ── leave（delay leave）/ blur / 再 click / Esc* ──► closed
                     ▲
                     └── 受控 SetOpen：外部决定；触发只发 OnOpenChange（TIP-S4）
```

\*Esc：click/focus 触发时关闭（hover-only 可不抢焦点；P0 至少 click 路径可关）。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| TIP-S1 | hover 打开（过 enterDelay） | title 可见；`IsOpen()==true` |
| TIP-S2 | 离开关闭（过 leaveDelay） | 不可见 |
| TIP-S3 | `placement=bottom`（及 12 向） | popup Placement 映射正确；面板在锚点对应侧 |
| TIP-S4 | 受控 `open` | 触发只回调，不擅自改 `Open`；`SetOpen` 生效 |
| TIP-S5 | 空 title / nil TitleNode | 任何触发都不打开 |
| TIP-S6 | `arrow` true/false / pointAtCenter | 箭头节点有无；center 时边角映射到主轴中点 placement |
| TIP-S7 | `color` 预设 / 自定义 | 面板底色变化；字色反白或自适应 |
| TIP-S8 | delay | enterDelay>0 时 Tick 累计后才开；0 则立即 |

### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| panel | 默认 spotlight 底 + 反白字 + 圆角 6 + pad 6×8；`color` 覆盖底色 |
| arrow | 与面板同色三角指示；`arrow=false` 时不绘制 |
| open/close | P0 瞬时切换；zoom-big-fast 像素级 **P1** |
| disabled 触发 | 触发器禁用皮；不打开 tip |
| 空 title | 无 tip 展示（等价 antd `title={null}`） |

**动效：** 入场 zoom 可关 / reduced-motion；P0 允许瞬时。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | 气泡 `role=tooltip`；触发器可聚焦时 `role=button`（或保留子节点角色） |
| 可访问名 | 触发器 Label = `AriaLabel` 或 `TriggerLabel` 或 title 摘要 |
| 焦点 | focus 触发：聚焦打开、失焦关闭；hover 不强制抢焦点 |
| Esc | click/focus 打开时可关（若实现 key 路由） |
| 遮罩 | Tooltip **无** mask（与 Modal/Drawer 不同） |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 主路径行为（§6.1 L1） | **对等** | P0 L1 |
| 尺寸/色 Token（§6.2） | **对等** | P0 L2 |
| zoom-big-fast 入场动画 | **瞬时**或近似 | P1 |
| `getPopupContainer` / DOM 容器 | Portal 统一挂载；容器选择 **P1** | P1 |
| Semantic classNames/styles | kit 语义钩子 | P1 |
| ConfigProvider `tooltip.unique` | 全局唯一显示 | P1（smooth-transition 主路径用多 tip 独立展示） |
| `align` / `fresh` / `destroyOnHidden` | 分期 | P1 |
| 逐像素官网哈希 | **不做** | — |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `title` / TitleNode | 必须；空则不显示 |
| `placement` 12 向 | 默认 top |
| `arrow` + `pointAtCenter` | 默认 true |
| `trigger` hover（默认）/ focus / click | contextMenu 可做；至少 hover+click |
| `open` / `defaultOpen` / `onOpenChange` | 受控 + 非受控 |
| `color` 预设 + `#hex` | colorful 示例 |
| `mouseEnterDelay` / `mouseLeaveDelay` | 默认 0.1；Ticker |
| `autoAdjustOverflow` | flip/shift（贴边偏移） |
| 官方主路径示例 | basic / smooth-transition（无 unique）/ placement / arrow / shift / colorful / disabled / wrap-custom-component |
| 度量 §6.2 | Token / 常量断言 |
| a11y §6.6 | role=tooltip + 触发名 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles / style-class 示例 | 分期 |
| ConfigProvider `tooltip.unique` 平滑唯一浮层 | smooth-transition 完整 unique 行为 |
| zoom-big-fast 动画像素级 | 分期 |
| `getPopupContainer` / `align` / `fresh` / `destroyOnHidden` | 分期 |
| debug / render-panel / 官网逐像素 | 不做 / 分期 |
| `_semantic.tsx` 自定义语义结构 | 分期 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestTooltip_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Tooltip 完成 1:1 主路径。  
> L3/L4（TIP-22/23）与 P1（TIP-24）本阶段不强制。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| TIP-01 | L1 | NewTooltip 默认创建 | 不崩溃；placement=top、arrow=true、trigger=hover、delay=0.1、closed |
| TIP-02 | L1 | hover 打开 | title 可见；IsOpen |
| TIP-03 | L1 | 离开关闭 | 不可见 |
| TIP-04 | L1 | placement=bottom（及 12 向可设） | popup Placement 映射正确 |
| TIP-05 | L1 | 受控 open | 触发只 OnOpenChange；SetOpen 生效 |
| TIP-06 | L1 | 空 title | 不显示（antd 行为） |
| TIP-07 | L1 | arrow true/false / center | 箭头节点有无；center 映射 |
| TIP-08 | L1 | color 预设 / 自定义 | 底色变 |
| TIP-09 | L1 | mouseEnterDelay | delay>0 需 Tick 后开；0 立即 |
| TIP-10 | L1 | 复现官方示例「基本」（`basic.tsx`） | 交互与主视觉符合文档 |
| TIP-11 | L1 | 复现「平滑过渡（降级版，不带 unique）」（`smooth-transition.tsx` 去 unique） | 多 tip 可独立开合；无崩溃 |
| TIP-12 | L1 | 复现「位置」（`placement.tsx`） | 12 向可构造 |
| TIP-13 | L1 | 复现「箭头展示」（`arrow.tsx`） | Show/Hide/Center |
| TIP-14 | L1 | 复现「贴边偏移」（`shift.tsx`） | autoAdjustOverflow + Viewport 不崩溃；可开 |
| TIP-15 | L1 | 复现「多彩文字提示」（`colorful.tsx`） | 预设 + 自定义色 |
| TIP-16 | L1 | 复现「禁用」（`disabled.tsx`） | title 空/切换后可开 |
| TIP-17 | L1 | 复现「自定义子组件」（`wrap-custom-component.tsx`） | SetTriggerNode 自定义触发 |
| TIP-18 | L2 | 读取 §6.2 关键尺寸/间距 | pad 6×8、radius 6、fontSize 14、maxWidth 250（±0.5） |
| TIP-19 | L2 | 默认皮颜色 | 底非 primary 硬编码；字走 inverse Token |
| TIP-20 | L2 | disabled 触发（适用者） | 不打开 |
| TIP-21 | L1 | 键盘/焦点主路径 | focus 触发可开；Focus ring 可见（可聚焦触发） |
| TIP-22 | L3 | 关键态 golden 截图 | 与仓库基线一致（本阶段可选） |
| TIP-23 | L4 | 与 ant.design 并排 | 人眼签字（本阶段可选） |
| TIP-24 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |

### 6.10 产品 API 契约（Go kit 侧）

> **Breaking OK。** 旧 `NewTooltip(trigger, title)` 删除；以下为产品契约。

```text
NewTooltip(title string) *Tooltip

// 内容
SetTitle(string) / SetTitleNode(core.Node)   // 空 title 且无 TitleNode → 永不打开
// 触发器
SetTriggerLabel(string) / SetTriggerNode(core.Node)
SetTrigger(TooltipTrigger) / SetTriggerModes(...TooltipTrigger)  // 空 → hover
// 几何 / 皮
SetPlacement(TooltipPlacement)               // 默认 Top
SetArrow(bool) / SetArrowConfig(show, pointAtCenter bool)
SetColor(string)                             // 预设名或 #hex；"" 恢复默认皮
SetAutoAdjustOverflow(bool)                  // 默认 true
SetZIndex(int)
// 开合
SetOpen(bool)                                // 受控
SetDefaultOpen(bool)                         // 非受控初值
SetOnOpenChange(func(bool))
SetMouseEnterDelay(sec float64)              // 默认 0.1
SetMouseLeaveDelay(sec float64)              // 默认 0.1
SetDisabled(bool)                            // 额外硬关（官方 demo 多用空 title）
// 主题 / a11y / 树
SetTheme(*Theme) / SetFace(text.Face)
SetAriaLabel(string)
Node() core.Node
IsOpen() bool
Popup() *primitive.AnchoredPopup
Panel() *primitive.Decorated
TriggerShell() *primitive.Pressable
// Ticker（delay）
AttachTicker(*core.Tree) / Tick(dt float64) bool
Sync() // deprecated 几何刷新
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Title | 构造参数 |
| Placement | Top |
| Arrow | true |
| ArrowPointAtCenter | false |
| Triggers | 空 → hover |
| AutoAdjustOverflow | true |
| MouseEnterDelay / Leave | 0.1 / 0.1 |
| Open | false（非受控） |
| Disabled | false |
| Color | ""（spotlight 默认皮） |
| zIndex | DefaultTooltipPortalZ |

### 6.11 结构与绘制分层（实现提示）

```text
Wrap (Column)
  ├─ shell Pressable (trigger)
  └─ AnchoredPopup
       └─ panel Decorated (+ optional arrow Canvas)
            └─ title Text | TitleNode
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- **delay** 用 `core.Ticker`（`Tick`）累计，不另起 goroutine。  
- 空 title：`requestOpen(true)` 直接 no-op。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Tooltip 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过（TIP-01…TIP-21；L3/L4/P1 除外）。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 可选（本阶段不阻塞）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：Tooltip 页覆盖 **§6.8 P0** 主路径官方示例；P1 可不进 gallery。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/tooltip.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Tooltip 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。

**§6 修订说明（相对模板稿）：**  
- **§6.1** 行为描述改为 tip 开合/placement/color/delay。  
- **§6.2** 补 antd 源码级 padding 6×8、maxWidth 250、spotlight 底、delay 0.1、zIndex 档。  
- **§6.3 / §6.10** 写成可实现的 P0 字段与 Go API（breaking `NewTooltip(title)`）。  
- **§6.4** 状态机补空 title、受控、delay 规则 ID。  
- **§6.6** 改为 `role=tooltip`、无 mask。  
- **§6.7 / §6.8** 明确 unique / semantic / zoom 为 P1；smooth-transition P0 用不带 unique 的多 tip。  
- **§6.9** 标明 L3/L4/P1 不阻塞；TIP-11 无 unique。
