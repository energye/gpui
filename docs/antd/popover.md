# Popover 气泡卡片
> 来源：[Ant Design 6.5.x Popover](https://ant.design/components/popover)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：点击/鼠标移入元素，弹出气泡式的卡片浮层。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

点击/鼠标移入元素，弹出气泡式的卡片浮层。

**Popover** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 三种触发方式 | 复现「三种触发方式」视觉与布局 |
| 位置 | placement 方位 |
| 箭头展示 | arrow 指示 |
| 贴边偏移 | 复现「贴边偏移」视觉与布局 |
| 从浮层内关闭 | 复现「从浮层内关闭」视觉与布局 |
| 悬停点击弹出窗口 | 复现「悬停点击弹出窗口」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `content`

- **说明**：卡片内容
- **类型**：ReactNode | () => ReactNode
- **默认值**：-

#### `title`

- **说明**：卡片标题
- **类型**：ReactNode | () => ReactNode
- **默认值**：-

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

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

当目标元素有进一步的描述和相关操作时，可以收纳到卡片中，根据用户的操作行为进行展现。

和 `Tooltip` 的区别是，用户可以对浮层上的元素进行操作，因此它可以承载更复杂的内容，比如链接或按钮等。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **三种触发方式**（`triggerType.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **位置**（`placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **箭头展示**（`arrow.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **贴边偏移**（`shift.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **从浮层内关闭**（`control.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **悬停点击弹出窗口**（`hover-with-click.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

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
| 三种触发方式 | `triggerType.tsx` | 否 |
| 位置 | `placement.tsx` | 否 |
| 箭头展示 | `arrow.tsx` | 否 |
| Arrow.pointAtCenter | `arrow-point-at-center.tsx` | 是 |
| 贴边偏移 | `shift.tsx` | 否 |
| 从浮层内关闭 | `control.tsx` | 否 |
| 悬停点击弹出窗口 | `hover-with-click.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| _InternalPanelDoNotUseOrYouWillBeFired | `render-panel.tsx` | 是 |
| 线框风格 | `wireframe.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |

### 2.6 FAQ

## FAQ

更多问题，请参考 [Tooltip FAQ](/components/tooltip-cn#faq)。

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
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | content | 卡片内容 | ReactNode \| () => ReactNode | - | title | 卡片标题 | ReactNode \| () => ReactNode | - | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - 
<!-- 共同的 API -->

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

### 导入方式

```js
import { Popover } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `content` | 卡片内容 | ReactNode \| () => ReactNode | - | — |
| `title` | 卡片标题 | ReactNode \| () => ReactNode | - | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `align` | 请参考 [dom-align](https://github.com/yiminghe/dom-align) 进行配置 | object | - | — |
| `arrow` | 修改箭头的显示状态以及修改箭头是否指向目标元素中心 | boolean \| { pointAtCenter: boolean } | true | 5.2.0 |
| `autoAdjustOverflow` | 气泡被遮挡时自动调整位置 | boolean | true | — |
| `color` | 背景颜色 | string | - | 4.3.0 |
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

实现 gpui kit 版 **Popover** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **8** 个，均需可复现。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/popover
- 中文文档：https://ant.design/components/popover-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/popover
- 驱动 gpui kit：`popover`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Popover** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/popover/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Popover）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 开合、Esc/外点、trigger（hover/click/focus）、placement、受控 open、浮层内可交互 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 内边距、圆角 LG、标题最小宽、颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Popover）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：点击/鼠标移入元素，弹出气泡式的卡片浮层（title + content）。与 Tooltip 的区别：浮层内容可交互（链接/按钮等）。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。  
源码：`components/popover/style/index.ts` → `prepareComponentToken` / `genBaseStyle`。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 | kit 常量建议 |
| --- | --- | --- | --- |
| 字号 | **14** | `fontSize` | `DefaultPopoverFontSize` |
| 面板圆角 | **8** | `borderRadiusLG`（非 `borderRadius`） | |
| 边框线宽 | **1** | `lineWidth` | |
| 面板内边距（非 wireframe） | **12** | 组件 token `innerPadding` | `DefaultPopoverInnerPadding` |
| 标题最小宽度 | **177** | 组件 token `titleMinWidth` | `DefaultPopoverTitleMinWidth` |
| 标题下间距 | **8** | `marginXS`（kit 映射） | `DefaultPopoverTitleMarginBottom` |
| 触发器↔面板间距 | **8** | 约 `sizePopupArrow` 量级 | `DefaultPopoverGap` |
| 箭头边长（示意） | **8** | `sizePopupArrow` 近似 | `DefaultPopoverArrowSize` |
| Focus ring outset | ≈ **1.5px** 可见 | 可调，必须可见 | |
| z-index 阶梯 | `zIndexPopupBase+30` | antd 组件 token；kit 映射 `Portal.ZOrder`（可 `SetZIndex`） | |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 面板底 | `colorBgContainer`（≈ antd `colorBgElevated` / `popoverBg`） | 禁止硬编码白 |
| 正文色 | `colorText`（`popoverColor`） | |
| 标题色 | `colorText` / heading 语义 | 加粗可选（P1 字重） |
| 边框 | `colorBorder` | 面板描边 |
| 禁用触发 | `colorDisabledBg` / `colorDisabledText` | 无 hover 高亮 |
| 浮层阴影 | 宿主/Decorated 阴影（P1 像素级） | P0 可用边框+底近似 |
| `color` 预设底色 | PresetColors → 面板/箭头底 | **P1** |

禁止硬编码品牌色作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据展示**。  
Popover **继承 Tooltip 共享 API**（placement / trigger / open / arrow / autoAdjustOverflow / …），并增加 `title` / `content`。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `title` | 卡片标题 | string / Node | — |
| `content` | 卡片内容（可交互） | string / Node | — |
| `trigger` | 触发行为；可多选 | `hover` \| `focus` \| `click` \| `contextMenu` | `hover` |
| `placement` | 12 向 | `top` / `topLeft` / … / `rightBottom` | `top` |
| `arrow` | 是否显示；`pointAtCenter` | bool \| `{ pointAtCenter }` | `true` |
| `open` / `defaultOpen` | 受控 / 非受控初值 | bool | `false` |
| `onOpenChange` | 显隐回调 | `(open bool)` | — |
| `autoAdjustOverflow` | 贴边 flip/shift | bool | `true` |
| `disabled` | 触发器禁用，不打开 | bool | `false` |
| `zIndex` | 浮层 ZOrder | number | 默认 portal 阶梯 |
| `mouseEnterDelay` / `mouseLeaveDelay` | 悬停延时（秒） | number | `0.1`（**P1**；P0 可瞬时+离开关闭宽限） |
| `destroyOnHidden` / `fresh` | 关闭销毁 / 始终刷新 | bool | false（**P1**） |
| `color` | 面板背景色 | string / color | —（**P1** 预设色板） |
| `classNames` / `styles` | 语义节点钩子 | Record | —（**P1**） |
| `getPopupContainer` / `align` | 挂载容器 / dom-align | — | 桌面映射 **P1** |

**配置优先级（通用）：** 受控 props（`open`）> 显式非受控 `defaultOpen` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
  mount ──► closed
               │
   trigger=hover ── enter trigger ──► open ── leave (宽限 Tick) ──► closed
   trigger=click ── click trigger ──► toggle open/closed
   trigger=focus ── focus trigger  ──► open ── blur ──► closed
   trigger=contextMenu ── right-down ──► open
               │
   open ── outside pointer (DismissOnOutside) ──► closed（非受控）
   open ── Esc (FocusScope) ──► closed
   open ── content 内按钮/链接可点（不自动关，除非业务 SetOpen(false)）
               │
   disabled ── 吞所有打开意图
   controlled open ── 仅 OnOpenChange 通知；显隐等 SetOpen
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| POP-S1 | 打开 | 面板可见；`title`/`content` 按设置渲染 |
| POP-S2 | 关闭 | 面板不可见；`IsOpen()==false` |
| POP-S3 | `placement` 12 向 | 映射 `AnchoredPopup.Placement`；打开后几何有效 |
| POP-S4 | 受控 `open` | `SetOpen` 决定显隐；用户意图只触发 `OnOpenChange` |
| POP-S5 | `trigger=click` | 点击触发器切换；外点关闭（非受控） |
| POP-S6 | 复杂 `content` | 浮层内按钮/链接可点（不吞事件） |
| POP-S7 | `trigger=hover` | 悬停开；离开触发器且未进入面板 → 关（Tick 宽限） |
| POP-S8 | `trigger=focus` | 聚焦开、失焦关 |
| POP-S9 | Esc | 打开时 Esc 关闭（非受控；受控走 `OnOpenChange`） |
| POP-S10 | `disabled` | 不打开；触发器禁用皮 |
| POP-S11 | `arrow` | `true` 时绘制示意箭头；`pointAtCenter` 时主轴居中映射 |
| POP-S12 | `autoAdjustOverflow` | `true` 时传入 Viewport 做 flip/shift；`false` 清 Viewport |

### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| panel/popup | `colorBgContainer` 底 + 边框 + **圆角 LG(8)** + 内边距 **12**；无全屏 mask（Popover ≠ Modal） |
| title | 可选；最小宽 **177**；与 content 间距 **titleMarginBottom**；标题色/字重强调 |
| content | 正文字色 `colorText`；可承载任意 Node |
| arrow | `arrow=true` 时在 placement 主轴侧绘制示意箭头；`pointAtCenter` 指向触发器中心 |
| open/close | 动画可关 / reduced-motion；**P0 瞬时切换** |
| disabled 触发 | 触发器禁用皮，不打开 |

**动效：** zoom-big 属 **P1**；P0 瞬时切换即可。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | 触发器可激活（`button`）；浮层容器 `dialog`（或等价）；有 `title` 时作为可访问名 |
| 名称 | 触发器可访问名 = 标签 / `AriaLabel`；面板 `Label` 优先 `title` |
| 焦点 | 触发器可聚焦并显示 focus ring；打开后 Esc 可关 |
| Esc | 打开时 Esc 关闭（POP-S9） |
| 键盘 | click 触发：Enter/Space 切换（与 Button 一致，经 Pressable） |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 主路径行为（§6.1 L1） | **对等** | P0 L1 |
| 尺寸/色 Token（§6.2） | **对等** | P0 L2 |
| 动画 zoom-big / 阴影像素 | **近似**或瞬时 | P1 |
| `mouseEnterDelay` / `mouseLeaveDelay` | **近似**（P0 瞬时+宽限；精确秒级 P1） | P1 |
| `getPopupContainer` / `align` / `destroyOnHidden` / `fresh` | **映射**或分期 | P1 |
| Semantic classNames/styles | kit 语义钩子 | P1 |
| `color` 预设色板 | 分期；可用 `SetPanelBackground` 近似 | P1 |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 逐像素官网哈希 | **不做** | — |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `title` / `content` | 字符串或 Node；面板内渲染 |
| `trigger` | `hover`（默认）/ `click` / `focus` / `contextMenu`；可多选 |
| `placement` | 12 向；默认 `top` |
| `arrow` / `pointAtCenter` | 默认显示箭头 |
| `open` / `defaultOpen` / `onOpenChange` | 受控 + 非受控 |
| `autoAdjustOverflow` | 默认 true |
| `disabled` | 不打开 |
| `zIndex` | 可选覆盖 Portal.ZOrder |
| 官方主路径示例 | 基本、三种触发方式、位置、箭头展示、贴边偏移、从浮层内关闭、悬停点击弹出窗口 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | 最低要求 |
| §6.9 中 L1/L2 用例（无 P1 标记） | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 深度；`style-class.tsx` | 分期 |
| `mouseEnterDelay` / `mouseLeaveDelay` 精确秒级 | 分期 |
| `color` 预设色 / `destroyOnHidden` / `fresh` / `getPopupContainer` / `align` | 分期 |
| 动画像素级 zoom-big / 复杂阴影 | 分期 |
| ConfigProvider 全局 popover 默认 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |
| 其余示例 | `_semantic.tsx` / wireframe / component-token |

### 6.9 验收用例表（可测）

> 测试名建议：`TestPopover_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Popover 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| POP-01 | L1 | `NewPopover` 默认创建 | 不崩溃；placement=top、arrow=true、trigger=hover、closed、AutoAdjustOverflow=true |
| POP-02 | L1 | 打开（`SetOpen(true)` 或 click） | title/content 可见；`IsOpen()` |
| POP-03 | L1 | 关闭 | 不可见 |
| POP-04 | L1 | `placement` 12 向 | 映射成功；打开不崩 |
| POP-05 | L1 | 受控 `open=false` 时点击 | 仍关；触发 `OnOpenChange`；`SetOpen(true)` 后开 |
| POP-06 | L1 | `trigger=click` | 点击切换；外点关闭（非受控） |
| POP-07 | L1 | 复杂 content 内按钮 | 可点并回调 |
| POP-08 | L1 | 复现官方示例「基本」（`basic.tsx`） | title+content；默认 hover 可构树 |
| POP-09 | L1 | 复现官方示例「三种触发方式」（`triggerType.tsx`） | hover/focus/click 均可构造并主路径可用 |
| POP-10 | L1 | 复现官方示例「位置」（`placement.tsx`） | 12 向可构造 |
| POP-11 | L1 | 复现官方示例「箭头展示」（`arrow.tsx`） | arrow true/false/pointAtCenter |
| POP-12 | L1 | 复现官方示例「贴边偏移」（`shift.tsx`） | `autoAdjustOverflow` + Viewport |
| POP-13 | L1 | 复现官方示例「从浮层内关闭」（`control.tsx`） | 受控 open；content 内关闭 |
| POP-14 | L1 | 复现官方示例「悬停点击弹出窗口」（`hover-with-click.tsx`） | 嵌套 hover+click 可构造 |
| POP-15 | P1 | 复现官方示例「自定义语义结构的样式和类」（`style-class.tsx`） | 单独用例；Notes 标明 |
| POP-16 | L2 | 读取 §6.2 关键尺寸/间距 | 14/8/12/177/1 等 ±0.5 |
| POP-17 | L2 | 默认皮颜色 | 面板底/字/边框走 Theme Token |
| POP-18 | L2 | disabled | 不打开；触发器禁用 |
| POP-19 | L1 | 键盘/焦点主路径 | Focus ring 可见；Esc 关；focus 触发可用 |
| POP-20 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| POP-21 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| POP-22 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewPopover(triggerLabel string) *Popover

// 内容
SetTitle(string) / SetTitleNode(core.Node)
SetContent(string) / SetContentNode(core.Node)

// 触发器
SetTriggerLabel(string)
SetTriggerNode(core.Node)                 // 自定义触发节点
SetTriggerModes(...PopoverTrigger)        // hover|click|focus|contextMenu；可多选
SetTrigger(PopoverTrigger)                // 单模式便利

// 浮层
SetPlacement(PopoverPlacement)            // 默认 Top；12 向
SetArrow(bool) / SetArrowConfig(show, pointAtCenter bool)  // 默认 arrow=true
SetAutoAdjustOverflow(bool)               // 默认 true
SetOpen(bool)                             // 受控 open
SetDefaultOpen(bool)                      // 非受控初值
SetOnOpenChange(func(open bool))
SetZIndex(int)                            // Portal.ZOrder；0=默认

// 状态
SetDisabled(bool)

// 主题 / a11y / 挂树
SetTheme(*Theme) / Theme 字段 / SetFace
SetAriaLabel(string)
Node() core.Node
Popup() *primitive.AnchoredPopup
Panel() *primitive.Decorated
TriggerShell() *primitive.Pressable
IsOpen() bool
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Placement | `top`（antd） |
| Trigger | `[hover]`（空切片表示默认 hover） |
| Arrow | `true` |
| AutoAdjustOverflow | `true` |
| Open | `false`；未 `SetOpen` 前为非受控 |
| Disabled | `false` |
| 其余 | 对齐 antd 6.5 §3 表 |

### 6.11 结构与绘制分层（实现提示）

```text
Column (Wrap)
  ├─ pointer host? (contextMenu) / Pressable shell (trigger)
  └─ AnchoredPopup (Portal)
       └─ FocusScope (Esc)
            └─ panel Decorated (+ optional arrow)
                 └─ Column
                      ├─ title?
                      └─ content
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- hover 离开宽限用 `Ticker`（跟随 Host Tick）；无控件级 loading 帧循环。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Popover 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过（POP-01–POP-14、POP-16–POP-19）。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见；可后补）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/popover.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Popover 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
