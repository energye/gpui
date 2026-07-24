# Drawer 抽屉
> 来源：[Ant Design 6.5.x Drawer](https://ant.design/components/drawer)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：反馈（Feedback）  
> 说明：屏幕边缘滑出的浮层面板。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

屏幕边缘滑出的浮层面板。

**Drawer** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基础抽屉 | 复现「基础抽屉」视觉与布局 |
| 自定义位置 | placement 方位 |
| 可调整大小 | 不同 size 档位 |
| 加载中 | loading 指示与防重复 |
| 额外操作 | 复现「额外操作」视觉与布局 |
| 渲染在当前 DOM | 复现「渲染在当前 DOM」视觉与布局 |
| 抽屉表单 | 复现「抽屉表单」视觉与布局 |
| 信息预览抽屉 | 复现「信息预览抽屉」视觉与布局 |
| 多层抽屉 | 复现「多层抽屉」视觉与布局 |
| 预设宽度 | 复现「预设宽度」视觉与布局 |
| 遮罩 | mask 层 |
| 关闭按钮位置 | placement 方位 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `bodyStyle`

- **说明**：抽屉内容区域样式，请使用 `styles.body` 替代
- **类型**：CSSProperties
- **默认值**：-

#### `classNames`

- **说明**：用于自定义 Drawer 组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `closable`

- **说明**：是否显示关闭按钮。可通过 `placement` 配置其位置
- **类型**：boolean | { closeIcon?: React.ReactNode; disabled?: boolean; placement?: 'start' | 'end' }
- **默认值**：true
- **版本**：placement: 5.28.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |

#### `contentWrapperStyle`

- **说明**：抽屉包裹层样式，请使用 `styles.wrapper` 替代
- **类型**：CSSProperties
- **默认值**：-

#### `drawerStyle`

- **说明**：抽屉面板样式，请使用 `styles.section` 替代
- **类型**：CSSProperties
- **默认值**：-

#### `extra`

- **说明**：抽屉右上角的操作区域
- **类型**：ReactNode
- **默认值**：-
- **版本**：4.17.0

#### `footer`

- **说明**：抽屉的页脚
- **类型**：ReactNode
- **默认值**：-

#### `footerStyle`

- **说明**：抽屉底部样式，请使用 `styles.footer` 替代
- **类型**：CSSProperties
- **默认值**：-

#### `getContainer`

- **说明**：指定 Drawer 挂载的节点，**并在容器内展现**，`false` 为挂载在当前位置
- **类型**：HTMLElement | () => HTMLElement | Selectors | false
- **默认值**：body

#### `headerStyle`

- **说明**：抽屉头部的样式，请使用 `styles.header` 替换
- **类型**：CSSProperties
- **默认值**：-

#### `height`

- **说明**：高度，在 `placement` 为 `top` 或 `bottom` 时使用，请使用 `size` 替换
- **类型**：string | number
- **默认值**：378
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `placement` | 官方取值 `placement` |
  | `top` | 上方 |
  | `bottom` | 下方 |
  | `size` | 官方取值 `size` |

#### `keyboard`

- **说明**：是否支持键盘 esc 关闭
- **类型**：boolean
- **默认值**：true

#### `loading`

- **说明**：显示骨架屏
- **类型**：boolean
- **默认值**：false
- **版本**：5.17.0

#### `mask`

- **说明**：遮罩效果
- **类型**：boolean | `{ enabled?: boolean, blur?: boolean, closable?: boolean }`
- **默认值**：true
- **版本**：mask.closable: 6.3.0

#### `maskStyle`

- **说明**：抽屉遮罩样式，请使用 `styles.mask` 替代
- **类型**：CSSProperties
- **默认值**：-

#### `maxSize`

- **说明**：可拖拽的最大尺寸（宽度或高度，取决于 `placement`）
- **类型**：number
- **默认值**：-
- **版本**：6.0.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `placement` | 官方取值 `placement` |

#### `placement`

- **说明**：抽屉的方向
- **类型**：`top` | `right` | `bottom` | `left`
- **默认值**：`right`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `top` | 上方 |
  | `right` | 右侧 |
  | `bottom` | 下方 |
  | `left` | 左侧 |

#### `resizable`

- **说明**：是否启用拖拽改变尺寸
- **类型**：boolean | [ResizableConfig](#resizableconfig)
- **默认值**：-
- **版本**：boolean: 6.1.0

#### `rootStyle`

- **说明**：可用于设置 Drawer 最外层容器的样式，和 `style` 的区别是作用节点包括 `mask`
- **类型**：CSSProperties
- **默认值**：-

#### `size`

- **说明**：预设抽屉宽度（或高度），default `378px` 和 large `736px`，或自定义数字
- **类型**：'default' | 'large' | number | string
- **默认值**：'default'
- **版本**：4.17.0, string: 6.2.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `default` | 默认中性外观 |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |

#### `style`

- **说明**：Drawer 面板的样式，如需仅配置 body 部分，请使用 `styles.body`
- **类型**：CSSProperties
- **默认值**：-

#### `styles`

- **说明**：用于自定义 Drawer 组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `title`

- **说明**：标题
- **类型**：ReactNode
- **默认值**：-

#### `width`

- **说明**：宽度，请使用 `size` 替换
- **类型**：string | number
- **默认值**：378
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `size` | 官方取值 `size` |

#### `zIndex`

- **说明**：设置 Drawer 的 `z-index`
- **类型**：number
- **默认值**：1000
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `z-index` | 官方取值 `z-index` |

#### `onClose`

- **说明**：点击遮罩层或左上角叉或取消按钮的回调
- **类型**：function(e)
- **默认值**：-

#### `onResizeStart`

- **说明**：开始拖拽调整大小时的回调
- **类型**：() => void
- **默认值**：-
- **版本**：6.0.0

#### `onResize`

- **说明**：拖拽调整大小时的回调
- **类型**：(size: number) => void
- **默认值**：-
- **版本**：6.0.0

#### `onResizeEnd`

- **说明**：结束拖拽调整大小时的回调
- **类型**：() => void
- **默认值**：-
- **版本**：6.0.0

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

抽屉从父窗体边缘滑入，覆盖住部分父窗体内容。用户在抽屉内操作时不必离开当前任务，操作完成后，可以平滑地回到原任务。

- 当需要一个附加的面板来控制父窗体内容，这个面板在需要时呼出。比如，控制界面展示样式，往界面中添加内容。
- 当需要在当前任务流中插入临时任务，创建或预览附加内容。比如展示协议条款，创建子对象。

> 开发者注意事项：
>
> 自 `5.17.0` 版本，我们提供了 `loading` 属性，内置 Spin 组件作为加载状态，但是自 `5.18.0` 版本开始，我们修复了设计失误，将内置的 Spin 组件替换成了 Skeleton 组件，同时收窄了 `loading` api 的类型范围，只能接收 boolean 类型。

### 2.2 核心功能（按官方示例拆解）

1. **基础抽屉**（`basic-right.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **自定义位置**（`placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **可调整大小**（`resizable.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **加载中**（`loading.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **额外操作**（`extra.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **渲染在当前 DOM**（`render-in-current.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **抽屉表单**（`form-in-drawer.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **信息预览抽屉**（`user-profile.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **多层抽屉**（`multi-level-drawer.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **预设宽度**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **遮罩**（`mask.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **关闭按钮位置**（`closable-placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `open` | 受控显隐 | Drawer 是否可见 |
| `loading` | 加载中 | 显示骨架屏 |
| `destroyOnHidden` | 隐藏销毁 | 关闭时销毁 Drawer 里的子元素 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基础抽屉 | `basic-right.tsx` | 否 |
| 自定义位置 | `placement.tsx` | 否 |
| 可调整大小 | `resizable.tsx` | 否 |
| 加载中 | `loading.tsx` | 否 |
| 额外操作 | `extra.tsx` | 否 |
| 渲染在当前 DOM | `render-in-current.tsx` | 否 |
| 抽屉表单 | `form-in-drawer.tsx` | 否 |
| 信息预览抽屉 | `user-profile.tsx` | 否 |
| 多层抽屉 | `multi-level-drawer.tsx` | 否 |
| 预设宽度 | `size.tsx` | 否 |
| 遮罩 | `mask.tsx` | 否 |
| 关闭按钮位置 | `closable-placement.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| ConfigProvider | `config-provider.tsx` | 是 |
| 无遮罩 | `no-mask.tsx` | 是 |
| _InternalPanelDoNotUseOrYouWillBeFired | `render-panel.tsx` | 是 |
| 滚动锁定调试 | `scroll-debug.tsx` | 是 |
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

通用属性参考：[通用属性](/docs/react/common-props)

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| afterOpenChange | 切换抽屉时动画结束后的回调 | function(open) | - | ~~bodyStyle~~ | 抽屉内容区域样式，请使用 `styles.body` 替代 | CSSProperties | - | - | × |
| className | Drawer 容器外层 className 设置，如果需要设置最外层，请使用 rootClassName | string | - | classNames | 用于自定义 Drawer 组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | closable | 是否显示关闭按钮。可通过 `placement` 配置其位置 | boolean \| { closeIcon?: React.ReactNode; disabled?: boolean; placement?: 'start' \| 'end' } | true | placement: 5.28.0 | 5.15.0，placement: 6.1.1 |
| ~~contentWrapperStyle~~ | 抽屉包裹层样式，请使用 `styles.wrapper` 替代 | CSSProperties | - | - | × |
| ~~destroyOnClose~~ | 关闭时销毁 Drawer 里的子元素 | boolean | false | ~~destroyInactivePanel~~ | 关闭时销毁 Drawer 里的子元素，请使用 `destroyOnHidden` 替代 | boolean | false | - | × |
| destroyOnHidden | 关闭时销毁 Drawer 里的子元素 | boolean | false | 5.25.0 | × |
| ~~drawerStyle~~ | 抽屉面板样式，请使用 `styles.section` 替代 | CSSProperties | - | - | × |
| extra | 抽屉右上角的操作区域 | ReactNode | - | 4.17.0 | × |
| footer | 抽屉的页脚 | ReactNode | - | ~~footerStyle~~ | 抽屉底部样式，请使用 `styles.footer` 替代 | CSSProperties | - | - | × |
| forceRender | 预渲染 Drawer 内元素 | boolean | false | focusable | 抽屉内焦点管理的配置 | `{ trap?: boolean, focusTriggerAfterClose?: boolean }` | - | 6.2.0 | 6.4.0 |
| getContainer | 指定 Drawer 挂载的节点，**并在容器内展现**，`false` 为挂载在当前位置 | HTMLElement \| () => HTMLElement \| Selectors \| false | body | ~~headerStyle~~ | 抽屉头部的样式，请使用 `styles.header` 替换 | CSSProperties | - | ~~height~~ | 高度，在 `placement` 为 `top` 或 `bottom` 时使用，请使用 `size` 替换 | string \| number | 378 | keyboard | 是否支持键盘 esc 关闭 | boolean | true | loading | 显示骨架屏 | boolean | false | 5.17.0 | × |
| mask | 遮罩效果 | boolean \| `{ enabled?: boolean, blur?: boolean, closable?: boolean }` | true | mask.closable: 6.3.0 | 6.0.0，mask.closable: 6.3.0 |
| ~~maskClosable~~ | 点击蒙层是否允许关闭 | boolean | true | ~~maskStyle~~ | 抽屉遮罩样式，请使用 `styles.mask` 替代 | CSSProperties | - | - | × |
| maxSize | 可拖拽的最大尺寸（宽度或高度，取决于 `placement`） | number | - | 6.0.0 | × |
| open | Drawer 是否可见 | boolean | false | placement | 抽屉的方向 | `top` \| `right` \| `bottom` \| `left` | `right` | push | 用于设置多层 Drawer 的推动行为 | boolean \| { distance: string \| number } | { distance: 180 } | 4.5.0+ | × |
| resizable | 是否启用拖拽改变尺寸 | boolean \| [ResizableConfig](#resizableconfig) | - | boolean: 6.1.0 | × |
| rootStyle | 可用于设置 Drawer 最外层容器的样式，和 `style` 的区别是作用节点包括 `mask` | CSSProperties | - | size | 预设抽屉宽度（或高度），default `378px` 和 large `736px`，或自定义数字 | 'default' \| 'large' \| number \| string | 'default' | 4.17.0, string: 6.2.0 | × |
| style | Drawer 面板的样式，如需仅配置 body 部分，请使用 `styles.body` | CSSProperties | - | styles | 用于自定义 Drawer 组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | title | 标题 | ReactNode | - | ~~width~~ | 宽度，请使用 `size` 替换 | string \| number | 378 | zIndex | 设置 Drawer 的 `z-index` | number | 1000 | onClose | 点击遮罩层或左上角叉或取消按钮的回调 | function(e) | - | drawerRender | 自定义渲染抽屉 | (node: ReactNode) => ReactNode | - | 5.18.0 | × |

### ResizableConfig

| 参数          | 说明                     | 类型                   | 默认值 | 版本  |
| ------------- | ------------------------ | ---------------------- | ------ | ----- |
| onResizeStart | 开始拖拽调整大小时的回调 | () => void             | -      | 6.0.0 |
| onResize      | 拖拽调整大小时的回调     | (size: number) => void | -      | 6.0.0 |
| onResizeEnd   | 结束拖拽调整大小时的回调 | () => void             | -      | 6.0.0 |

### 导入方式

```js
import { Drawer } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `afterOpenChange` | 切换抽屉时动画结束后的回调 | function(open) | - | — |
| `bodyStyle` | 抽屉内容区域样式，请使用 `styles.body` 替代 | CSSProperties | - | - |
| `className` | Drawer 容器外层 className 设置，如果需要设置最外层，请使用 rootClassName | string | - | — |
| `classNames` | 用于自定义 Drawer 组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `closable` | 是否显示关闭按钮。可通过 `placement` 配置其位置 | boolean \| { closeIcon?: React.ReactNode; disabled?: boolean; placement?: 'start' \| 'end' } | true | placement: 5.28.0 |
| `contentWrapperStyle` | 抽屉包裹层样式，请使用 `styles.wrapper` 替代 | CSSProperties | - | - |
| `destroyOnClose` | 关闭时销毁 Drawer 里的子元素 | boolean | false | — |
| `destroyInactivePanel` | 关闭时销毁 Drawer 里的子元素，请使用 `destroyOnHidden` 替代 | boolean | false | - |
| `destroyOnHidden` | 关闭时销毁 Drawer 里的子元素 | boolean | false | 5.25.0 |
| `drawerStyle` | 抽屉面板样式，请使用 `styles.section` 替代 | CSSProperties | - | - |
| `extra` | 抽屉右上角的操作区域 | ReactNode | - | 4.17.0 |
| `footer` | 抽屉的页脚 | ReactNode | - | — |
| `footerStyle` | 抽屉底部样式，请使用 `styles.footer` 替代 | CSSProperties | - | - |
| `forceRender` | 预渲染 Drawer 内元素 | boolean | false | — |
| `focusable` | 抽屉内焦点管理的配置 | `{ trap?: boolean, focusTriggerAfterClose?: boolean }` | - | 6.2.0 |
| `getContainer` | 指定 Drawer 挂载的节点，**并在容器内展现**，`false` 为挂载在当前位置 | HTMLElement \| () => HTMLElement \| Selectors \| false | body | — |
| `headerStyle` | 抽屉头部的样式，请使用 `styles.header` 替换 | CSSProperties | - | — |
| `height` | 高度，在 `placement` 为 `top` 或 `bottom` 时使用，请使用 `size` 替换 | string \| number | 378 | — |
| `keyboard` | 是否支持键盘 esc 关闭 | boolean | true | — |
| `loading` | 显示骨架屏 | boolean | false | 5.17.0 |
| `mask` | 遮罩效果 | boolean \| `{ enabled?: boolean, blur?: boolean, closable?: boolean }` | true | mask.closable: 6.3.0 |
| `maskClosable` | 点击蒙层是否允许关闭 | boolean | true | — |
| `maskStyle` | 抽屉遮罩样式，请使用 `styles.mask` 替代 | CSSProperties | - | - |
| `maxSize` | 可拖拽的最大尺寸（宽度或高度，取决于 `placement`） | number | - | 6.0.0 |
| `open` | Drawer 是否可见 | boolean | false | — |
| `placement` | 抽屉的方向 | `top` \| `right` \| `bottom` \| `left` | `right` | — |
| `push` | 用于设置多层 Drawer 的推动行为 | boolean \| { distance: string \| number } | { distance: 180 } | 4.5.0+ |
| `resizable` | 是否启用拖拽改变尺寸 | boolean \| [ResizableConfig](#resizableconfig) | - | boolean: 6.1.0 |
| `rootStyle` | 可用于设置 Drawer 最外层容器的样式，和 `style` 的区别是作用节点包括 `mask` | CSSProperties | - | — |
| `size` | 预设抽屉宽度（或高度），default `378px` 和 large `736px`，或自定义数字 | 'default' \| 'large' \| number \| string | 'default' | 4.17.0, string: 6.2.0 |
| `style` | Drawer 面板的样式，如需仅配置 body 部分，请使用 `styles.body` | CSSProperties | - | — |
| `styles` | 用于自定义 Drawer 组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `title` | 标题 | ReactNode | - | — |
| `width` | 宽度，请使用 `size` 替换 | string \| number | 378 | — |
| `zIndex` | 设置 Drawer 的 `z-index` | number | 1000 | — |
| `onClose` | 点击遮罩层或左上角叉或取消按钮的回调 | function(e) | - | — |
| `drawerRender` | 自定义渲染抽屉 | (node: ReactNode) => ReactNode | - | 5.18.0 |
| `onResizeStart` | 开始拖拽调整大小时的回调 | () => void | - | 6.0.0 |
| `onResize` | 拖拽调整大小时的回调 | (size: number) => void | - | 6.0.0 |
| `onResizeEnd` | 结束拖拽调整大小时的回调 | () => void | - | 6.0.0 |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Drawer** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **13** 个，均需可复现。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/drawer
- 中文文档：https://ant.design/components/drawer-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/drawer
- 驱动 gpui kit：`drawer`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Drawer** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/drawer/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Drawer）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 开合、遮罩/Esc、placement、确认/取消主路径 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Drawer）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：屏幕边缘滑出的浮层面板。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 默认 width | **378** | API 默认 |
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
| `afterOpenChange` | 切换抽屉时动画结束后的回调 | function(open) | - |
| `className` | Drawer 容器外层 className 设置，如果需要设置最外层，请使用 rootClassName | string | - |
| `classNames` | 用于自定义 Drawer 组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `closable` | 是否显示关闭按钮。可通过 `placement` 配置其位置 | boolean \ | { closeIcon?: React.ReactNode; disabled?: boolean; placement?: 'start' \ |
| `destroyOnHidden` | 关闭时销毁 Drawer 里的子元素 | boolean | false |
| `extra` | 抽屉右上角的操作区域 | ReactNode | - |
| `footer` | 抽屉的页脚 | ReactNode | - |
| `forceRender` | 预渲染 Drawer 内元素 | boolean | false |
| `focusable` | 抽屉内焦点管理的配置 | `{ trap?: boolean, focusTriggerAfterC… | - |
| `getContainer` | 指定 Drawer 挂载的节点，**并在容器内展现**，`false` 为挂载在当前位置 | HTMLElement \ | () => HTMLElement \ |
| `keyboard` | 是否支持键盘 esc 关闭 | boolean | true |
| `loading` | 显示骨架屏 | boolean | false |
| `mask` | 遮罩效果 | boolean \ | `{ enabled?: boolean, blur?: boolean, closable?: boolean }` |
| `maxSize` | 可拖拽的最大尺寸（宽度或高度，取决于 `placement`） | number | - |
| `open` | Drawer 是否可见 | boolean | false |
| `placement` | 抽屉的方向 | `top` \ | `right` \ |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
closed ── open ──► 侧滑 panel + mask
             ├── close / Esc / mask ──► onClose
             ├── placement 四边 ──► 滑出方向
             └── destroyOnHidden ──► 卸载
```

\*默认 width=378。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| DRW-S1 | open=true | 可见 |
| DRW-S2 | onClose 路径 | 关闭 |
| DRW-S3 | placement=left | 从左出 |
| DRW-S4 | maskClosable=false | 点 mask 不关 |
| DRW-S5 | Esc | 关（keyboard） |
| DRW-S6 | 默认宽 | 378 |
| DRW-S7 | footer | 底栏可见 |
| DRW-S8 | destroyOnHidden | 卸载子树 |
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
| `loading` | 必须 |
| `size` | 必须 |
| `open` | 必须 |
| `title` | 必须 |
| `placement` | 必须 |
| 官方主路径示例 | 基础抽屉、自定义位置、可调整大小、加载中、额外操作、渲染在当前 DOM、抽屉表单、信息预览抽屉 |
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
| 其余示例 | 多层抽屉, 预设宽度, 遮罩, 关闭按钮位置 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestDrawer_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Drawer 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| DRW-01 | L1 | NewDrawer 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| DRW-02 | L1 | open=true | 可见 |
| DRW-03 | L1 | onClose 路径 | 关闭 |
| DRW-04 | L1 | placement=left | 从左出 |
| DRW-05 | L1 | maskClosable=false | 点 mask 不关 |
| DRW-06 | L1 | Esc | 关（keyboard） |
| DRW-07 | L1 | 默认宽 | 378 |
| DRW-08 | L1 | footer | 底栏可见 |
| DRW-09 | L1 | destroyOnHidden | 卸载子树 |
| DRW-10 | L1 | 复现官方示例「基础抽屉」（`basic-right.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| DRW-11 | L1 | 复现官方示例「自定义位置」（`placement.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| DRW-12 | L1 | 复现官方示例「可调整大小」（`resizable.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| DRW-13 | L1 | 复现官方示例「加载中」（`loading.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| DRW-14 | L1 | 复现官方示例「额外操作」（`extra.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| DRW-15 | L1 | 复现官方示例「渲染在当前 DOM」（`render-in-current.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| DRW-16 | L1 | 复现官方示例「抽屉表单」（`form-in-drawer.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| DRW-17 | L1 | 复现官方示例「信息预览抽屉」（`user-profile.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| DRW-18 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| DRW-19 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| DRW-20 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| DRW-21 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| DRW-22 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| DRW-23 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| DRW-24 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewDrawer(...) *Drawer

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

同时满足即可宣布 **Drawer 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. gallery 展示主路径（对照官方非 debug 示例与 P0）。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/drawer.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Drawer 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
