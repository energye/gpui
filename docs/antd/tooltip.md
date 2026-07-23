# Tooltip 文字提示
> 来源：[Ant Design 6.5.x Tooltip](https://ant.design/components/tooltip)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：简单的文字提示气泡框。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
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
