# Dropdown 下拉菜单
> 来源：[Ant Design 6.5.x Dropdown](https://ant.design/components/dropdown)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：导航（Navigation）  
> 说明：向下弹出的列表。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
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
