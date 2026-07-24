# Dropdown 下拉菜单
> 来源：[Ant Design 6.5.x Dropdown](https://ant.design/components/dropdown)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：导航（Navigation）  
> 说明：向下弹出的列表。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

向下弹出的列表。

**Dropdown** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 额外节点 | 复现「额外节点」视觉与布局 |
| 弹出位置 | placement 方位 |
| 箭头 | arrow 指示 |
| 其他元素 | 复现「其他元素」视觉与布局 |
| 箭头指向 | arrow 指示 |
| 触发方式 | 复现「触发方式」视觉与布局 |
| 触发事件 | 复现「触发事件」视觉与布局 |
| 带下拉框的按钮 | 复现「带下拉框的按钮」视觉与布局 |
| 扩展菜单 | 复现「扩展菜单」视觉与布局 |
| 多级菜单 | 复现「多级菜单」视觉与布局 |
| 菜单隐藏方式 | 复现「菜单隐藏方式」视觉与布局 |
| 右键菜单 | 复现「右键菜单」视觉与布局 |
| 加载中状态 | loading 指示与防重复 |
| 菜单可选选择 | 复现「菜单可选选择」视觉与布局 |
| 划词操作 | 复现「划词操作」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `autoAdjustOverflow`

- **说明**：下拉框被遮挡时自动调整位置
- **类型**：boolean
- **默认值**：true
- **版本**：5.2.0

#### `classNames`

- **说明**：用于自定义 Dropdown 组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `disabled`

- **说明**：菜单是否禁用
- **类型**：boolean
- **默认值**：-

#### `overlayStyle`

- **说明**：下拉根元素的样式，请使用 `styles.root`
- **类型**：CSSProperties
- **默认值**：-

#### `placement`

- **说明**：菜单弹出位置：`top` `topLeft` `topRight` `bottom` `bottomLeft` `bottomRight` `left` `leftTop` `leftBottom` `right` `rightTop` `rightBottom`
- **类型**：string
- **默认值**：`bottomLeft`
- **版本**：`left` `leftTop` `leftBottom` `right` `rightTop` `rightBottom`：6.5.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `top` | 上方 |
  | `topLeft` | 上左 |
  | `topRight` | 上右 |
  | `bottom` | 下方 |
  | `bottomLeft` | 下左 |
  | `bottomRight` | 下右 |
  | `left` | 左侧 |
  | `leftTop` | 左上 |
  | `leftBottom` | 左下 |
  | `right` | 右侧 |
  | `rightTop` | 右上 |
  | `rightBottom` | 右下 |

#### `styles`

- **说明**：用于自定义 Dropdown 组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `onOpenChange`

- **说明**：菜单显示状态改变时调用，点击菜单按钮导致的消失不会触发
- **类型**：(open: boolean, info: { source: 'trigger' | 'menu' }) => void
- **默认值**：-
- **版本**：`info.source`: 5.11.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `trigger` | 官方取值 `trigger` |
  | `menu` | 官方取值 `menu` |

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

当页面上的操作命令过多时，用此组件可以收纳操作元素。点击或移入触点，会出现一个下拉菜单。可在列表中进行选择，并执行相应的命令。

- 用于收罗一组命令操作。
- Select 用于选择，而 Dropdown 是命令集合。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **额外节点**（`extra.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **弹出位置**（`placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **箭头**（`arrow.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **其他元素**（`item.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **箭头指向**（`arrow-center.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **触发方式**（`trigger.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **触发事件**（`event.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **带下拉框的按钮**（`dropdown-button.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **扩展菜单**（`custom-dropdown.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **多级菜单**（`sub-menu.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **菜单隐藏方式**（`overlay-open.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **右键菜单**（`context-menu.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
14. **加载中状态**（`loading.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
15. **菜单可选选择**（`selectable.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
16. **划词操作**（`selection.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
17. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `open` | 受控显隐 | 菜单是否显示 |
| `onOpenChange` | 显隐变化 | 菜单显示状态改变时调用，点击菜单按钮导致的消失不会触发 |
| `disabled` | 禁用 | 菜单是否禁用 |
| `getPopupContainer` | 浮层容器 | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例](https://codepen.io/afc163/pen/zEjNOy?editors=0010) |
| `destroyOnHidden` | 隐藏销毁 | 关闭后是否销毁 Dropdown |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 额外节点 | `extra.tsx` | 否 |
| 弹出位置 | `placement.tsx` | 否 |
| 箭头 | `arrow.tsx` | 否 |
| 其他元素 | `item.tsx` | 否 |
| 箭头指向 | `arrow-center.tsx` | 否 |
| 触发方式 | `trigger.tsx` | 否 |
| 触发事件 | `event.tsx` | 否 |
| 带下拉框的按钮 | `dropdown-button.tsx` | 否 |
| 扩展菜单 | `custom-dropdown.tsx` | 否 |
| 多级菜单 | `sub-menu.tsx` | 否 |
| 多级菜单 | `sub-menu-debug.tsx` | 是 |
| 菜单隐藏方式 | `overlay-open.tsx` | 否 |
| 右键菜单 | `context-menu.tsx` | 否 |
| 加载中状态 | `loading.tsx` | 否 |
| 菜单可选选择 | `selectable.tsx` | 否 |
| 划词操作 | `selection.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| Menu 完整样式 | `menu-full.tsx` | 是 |
| \_InternalPanelDoNotUseOrYouWillBeFired | `render-panel.tsx` | 是 |
| Icon debug | `icon-debug.tsx` | 是 |

### 2.6 FAQ

## FAQ

### Dropdown 在水平方向超出屏幕时会被挤压该怎么办？ {#faq-dropdown-squeezed}

你可以通过 `width: max-content` 来解决这个问题，参考 [#43025](https://github.com/ant-design/ant-design/issues/43025#issuecomment-1594394135)。

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

### Dropdown

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| arrow | 下拉框箭头是否显示 | boolean \| { pointAtCenter: boolean } | false | autoAdjustOverflow | 下拉框被遮挡时自动调整位置 | boolean | true | 5.2.0 | × |
| classNames | 用于自定义 Dropdown 组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> | - | disabled | 菜单是否禁用 | boolean | - | ~~destroyPopupOnHide~~ | 关闭后是否销毁 Dropdown，使用 `destroyOnHidden` 替换 | boolean | false | destroyOnHidden | 关闭后是否销毁 Dropdown | boolean | false | 5.25.0 | × |
| ~~dropdownRender~~ | 自定义下拉框内容，使用 `popupRender` 替换 | (menus: ReactNode) => ReactNode | - | 4.24.0 | × |
| popupRender | 自定义弹出框内容 | (menus: ReactNode) => ReactNode | - | 5.25.0 | × |
| getPopupContainer | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例](https://codepen.io/afc163/pen/zEjNOy?editors=0010) | (triggerNode: HTMLElement) => HTMLElement | () => document.body | menu | 菜单配置项 | [MenuProps](/components/menu-cn#api) | - | ~~overlayClassName~~ | 下拉根元素的类名称, 请使用 `classNames.root` 替换 | string | - | ~~overlayStyle~~ | 下拉根元素的样式，请使用 `styles.root` | CSSProperties | - | placement | 菜单弹出位置：`top` `topLeft` `topRight` `bottom` `bottomLeft` `bottomRight` `left` `leftTop` `leftBottom` `right` `rightTop` `rightBottom` | string | `bottomLeft` | `left` `leftTop` `leftBottom` `right` `rightTop` `rightBottom`：6.5.0 | × |
| styles | 用于自定义 Dropdown 组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom) , CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom) , CSSProperties> | - | trigger | 触发下拉的行为，移动端不支持 hover | Array&lt;`click`\|`hover`\|`contextMenu`> | \[`hover`] | open | 菜单是否显示 | boolean | - | onOpenChange | 菜单显示状态改变时调用，点击菜单按钮导致的消失不会触发 | (open: boolean, info: { source: 'trigger' \| 'menu' }) => void | - | `info.source`: 5.11.0 | × |

### 导入方式

```js
import { Dropdown } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `arrow` | 下拉框箭头是否显示 | boolean \| { pointAtCenter: boolean } | false | — |
| `autoAdjustOverflow` | 下拉框被遮挡时自动调整位置 | boolean | true | 5.2.0 |
| `classNames` | 用于自定义 Dropdown 组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `disabled` | 菜单是否禁用 | boolean | - | — |
| `destroyPopupOnHide` | 关闭后是否销毁 Dropdown，使用 `destroyOnHidden` 替换 | boolean | false | — |
| `destroyOnHidden` | 关闭后是否销毁 Dropdown | boolean | false | 5.25.0 |
| `dropdownRender` | 自定义下拉框内容，使用 `popupRender` 替换 | (menus: ReactNode) => ReactNode | - | 4.24.0 |
| `popupRender` | 自定义弹出框内容 | (menus: ReactNode) => ReactNode | - | 5.25.0 |
| `getPopupContainer` | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例](https://codepen.io/afc163/pen/zEjNOy?editors=0010) | (triggerNode: HTMLElement) => HTMLElement | () => document.body | — |
| `menu` | 菜单配置项 | [MenuProps](/components/menu-cn#api) | - | — |
| `overlayClassName` | 下拉根元素的类名称, 请使用 `classNames.root` 替换 | string | - | — |
| `overlayStyle` | 下拉根元素的样式，请使用 `styles.root` | CSSProperties | - | — |
| `placement` | 菜单弹出位置：`top` `topLeft` `topRight` `bottom` `bottomLeft` `bottomRight` `left` `leftTop` `leftBottom` `right` `rightTop` `rightBottom` | string | `bottomLeft` | `left` `leftTop` `leftBottom` `right` `rightTop` `rightBottom`：6.5.0 |
| `styles` | 用于自定义 Dropdown 组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `trigger` | 触发下拉的行为，移动端不支持 hover | Array<`click`\|`hover`\|`contextMenu`> | \[`hover`] | — |
| `open` | 菜单是否显示 | boolean | - | — |
| `onOpenChange` | 菜单显示状态改变时调用，点击菜单按钮导致的消失不会触发 | (open: boolean, info: { source: 'trigger' \| 'menu' }) => void | - | `info.source`: 5.11.0 |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Dropdown** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **17** 个，均需可复现。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/dropdown
- 中文文档：https://ant.design/components/dropdown-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/dropdown
- 驱动 gpui kit：`dropdown`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Dropdown** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/dropdown/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Dropdown）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 开合、遮罩/Esc、placement、确认/取消主路径 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Dropdown）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：向下弹出的列表。

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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**导航**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `arrow` | 下拉框箭头是否显示 | boolean \ | { pointAtCenter: boolean } |
| `autoAdjustOverflow` | 下拉框被遮挡时自动调整位置 | boolean | true |
| `classNames` | 用于自定义 Dropdown 组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> |
| `disabled` | 菜单是否禁用 | boolean | - |
| `destroyOnHidden` | 关闭后是否销毁 Dropdown | boolean | false |
| `popupRender` | 自定义弹出框内容 | (menus: ReactNode) => ReactNode | - |
| `getPopupContainer` | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例]… | (triggerNode: HTMLElement) => HTMLEle… | () => document.body |
| `menu` | 菜单配置项 | [MenuProps](/components/menu-cn#api) | - |
| `placement` | 菜单弹出位置：`top` `topLeft` `topRight` `bottom` `bottomLeft` `… | string | `bottomLeft` |
| `styles` | 用于自定义 Dropdown 组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom) ,… | (info: { props }) => Record<[SemanticDOM](#semantic-dom) , CSSProperties> |
| `trigger` | 触发下拉的行为，移动端不支持 hover | Array&lt;`click`\ | `hover`\ |
| `open` | 菜单是否显示 | boolean | - |
| `onOpenChange` | 菜单显示状态改变时调用，点击菜单按钮导致的消失不会触发 | (open: boolean, info: { source: 'trig… | 'menu' }) => void |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
closed ── trigger(hover/click/contextMenu) ──► open menu
  选 item ──► onClick + 常关闭
  外点/Esc ──► 关闭
  受控 open
  disabled ──► 不打开
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| DD-S1 | click 触发打开 | 菜单可见 |
| DD-S2 | 选一项 | onClick；关闭 |
| DD-S3 | 外点 | 关闭 |
| DD-S4 | Esc | 关闭 |
| DD-S5 | disabled | 不打开 |
| DD-S6 | 受控 open=false | 关 |
| DD-S7 | placement | 位置正确 |
| DD-S8 | hover 触发 | 悬停开，离开关 |
| DD-S9 | 子菜单（适用） | 可展开 |
| DD-S10 | 危险项 | 红色样式 |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| mask | `colorBgMask` 半透明（适用者） |
| panel/popup | 容器底 + 阴影 + 圆角 LG |
| open/close | 动画可关 / reduced-motion |
| disabled 触发 | 触发器禁用皮，不打开 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | dialog / menu / tooltip 等 |
| 焦点 | 打开进入浮层；关闭回触发器（可配） |
| Esc | 关闭（若允许） |
| 标题 | Dialog 必须有可访问名 |
| 遮罩 | 点击策略明确 |

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
| `disabled` | 必须 |
| `open` | 必须 |
| `onOpenChange` | 必须 |
| `placement` | 必须 |
| `trigger` | 必须 |
| 官方主路径示例 | 基本、额外节点、弹出位置、箭头、其他元素、箭头指向、触发方式、触发事件 |
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
| 其余示例 | 带下拉框的按钮, 扩展菜单, 多级菜单, 菜单隐藏方式 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestDropdown_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Dropdown 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| DD-01 | L1 | NewDropdown 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| DD-02 | L1 | click 触发打开 | 菜单可见 |
| DD-03 | L1 | 选一项 | onClick；关闭 |
| DD-04 | L1 | 外点 | 关闭 |
| DD-05 | L1 | Esc | 关闭 |
| DD-06 | L1 | disabled | 不打开 |
| DD-07 | L1 | 受控 open=false | 关 |
| DD-08 | L1 | placement | 位置正确 |
| DD-09 | L1 | hover 触发 | 悬停开，离开关 |
| DD-10 | L1 | 子菜单（适用） | 可展开 |
| DD-11 | L1 | 危险项 | 红色样式 |
| DD-12 | L1 | 复现官方示例「基本」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| DD-13 | L1 | 复现官方示例「额外节点」（`extra.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| DD-14 | L1 | 复现官方示例「弹出位置」（`placement.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| DD-15 | L1 | 复现官方示例「箭头」（`arrow.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| DD-16 | L1 | 复现官方示例「其他元素」（`item.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| DD-17 | L1 | 复现官方示例「箭头指向」（`arrow-center.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| DD-18 | L1 | 复现官方示例「触发方式」（`trigger.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| DD-19 | L1 | 复现官方示例「触发事件」（`event.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| DD-20 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| DD-21 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| DD-22 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| DD-23 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| DD-24 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| DD-25 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| DD-26 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewDropdown(...) *Dropdown

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
Trigger?
  └─ Portal
       ├─ mask?
       └─ panel / popup (+ arrow?)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Dropdown 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/dropdown.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Dropdown 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
