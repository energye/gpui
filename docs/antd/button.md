# Button 按钮

> 来源：[Ant Design 6.5.x Button](https://ant.design/components/button)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：通用（General）  
> 说明：按钮用于开始一个即时操作。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**（全库样板）。

---

## 1. 控件外观

### 1.1 基础形态


按钮用于开始一个即时操作。

**Button** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 语法糖 | type 语法糖预设 |
| 颜色与变体 | 语义色/预设色 |
| 按钮图标 | icon 与文本混排 |
| 按钮图标位置 | icon 与文本混排 |
| 按钮尺寸 | 不同 size 档位的高宽/字号/内边距 |
| 不可用状态 | disabled 态 |
| 加载中状态 | loading 指示与防重复 |
| 多个按钮组合 | 复现「多个按钮组合」视觉与布局 |
| 幽灵按钮 | 透明/反色底 |
| 危险按钮 | danger 红色语义 |
| Block 按钮 | 宽度撑满父级 |
| 渐变按钮 | 渐变填充 |
| 自定义按钮波纹 | 自定义渲染/插槽外观 |
| 移除两个汉字之间的空格 | 空状态插画/文案 |
| 自定义禁用样式背景 | disabled 灰态与不可点 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `block`

- **说明**：将按钮宽度调整为其父宽度的选项
- **类型**：boolean
- **默认值**：false

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `color`

- **说明**：设置按钮的颜色
- **类型**：`default` | `primary` | `danger` | [PresetColors](#presetcolors)
- **默认值**：`variant="solid"` 时为 `primary`
- **版本**：`default`、`primary` 和 `danger`: 5.21.0, `PresetColors`: 5.23.0, `solid` 默认颜色: 6.4.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `default` | 默认中性外观 |
  | `primary` | 主色强调 |
  | `danger` | 危险红语义 |

#### `danger`

- **说明**：语法糖，设置危险按钮。当设置 `color` 时会以后者为准
- **类型**：boolean
- **默认值**：false

#### `disabled`

- **说明**：设置按钮失效状态
- **类型**：boolean
- **默认值**：false

#### `ghost`

- **说明**：幽灵属性，使按钮背景透明
- **类型**：boolean
- **默认值**：false

#### `icon`

- **说明**：设置按钮的图标组件
- **类型**：ReactNode
- **默认值**：-

#### `iconPlacement`

- **说明**：设置按钮图标组件的位置
- **类型**：`start` | `end`
- **默认值**：`start`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |

#### `iconPosition`

- **说明**：设置按钮图标组件的位置,请使用 `iconPlacement` 替换
- **类型**：`start` | `end`
- **默认值**：`start`
- **版本**：5.17.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |

#### `loading`

- **说明**：设置按钮载入状态
- **类型**：boolean | { delay: number, icon: ReactNode }
- **默认值**：false
- **版本**：icon: 5.23.0

#### `loadingIcon`

- **说明**：（仅支持全局配置）设置按钮的加载图标
- **类型**：ReactNode
- **默认值**：``

#### `shape`

- **说明**：设置按钮形状
- **类型**：`default` | `circle` | `round`
- **默认值**：`default`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `default` | 默认中性外观 |
  | `circle` | 圆形 |
  | `round` | 大圆角/胶囊 |

#### `size`

- **说明**：设置按钮大小
- **类型**：`large` | `medium` | `small`
- **默认值**：`medium`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `type`

- **说明**：语法糖，设置按钮类型。当设置 `variant` 与 `color` 时以后者为准
- **类型**：`primary` | `dashed` | `link` | `text` | `default`
- **默认值**：`default`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `primary` | 主色强调 |
  | `dashed` | 虚线边框 |
  | `link` | 链接样式 |
  | `text` | 文本/弱样式 |
  | `default` | 默认中性外观 |

#### `variant`

- **说明**：设置按钮的变体
- **类型**：`outlined` | `dashed` | `solid` | `filled` | `text` | `link`
- **默认值**：-
- **版本**：5.21.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `outlined` | 描边空心 |
  | `dashed` | 虚线边框 |
  | `solid` | 实心填充 |
  | `filled` | 浅底填充 |
  | `text` | 文本/弱样式 |
  | `link` | 链接样式 |

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

标记了一个（或封装一组）操作命令，响应用户点击行为，触发相应的业务逻辑。

在 Ant Design 中我们提供了五种按钮。

- 🔵 主按钮：用于主行动点，一个操作区域只能有一个主按钮。
- ⚪️ 默认按钮：用于没有主次之分的一组行动点。
- 😶 虚线按钮：常用于添加操作。
- 🔤 文本按钮：用于最次级的行动点。
- 🔗 链接按钮：一般用于链接，即导航至某位置。

以及四种状态属性与上面配合使用。

- ⚠️ 危险：删除/移动/修改权限等危险操作，一般需要二次确认。
- 👻 幽灵：用于背景色比较复杂的地方，常用在首页/产品页等展示场景。
- 🚫 禁用：行动点不可用的时候，一般需要文案解释。
- 🔃 加载中：用于异步操作等待反馈的时候，也可以避免多次提交。

### 2.2 核心功能（按官方示例拆解）

1. **语法糖**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **颜色与变体**（`color-variant.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **按钮图标**（`icon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **按钮图标位置**（`icon-placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **按钮尺寸**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **不可用状态**（`disabled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **加载中状态**（`loading.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **多个按钮组合**（`multiple.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **幽灵按钮**（`ghost.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **危险按钮**（`danger.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **Block 按钮**（`block.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **渐变按钮**（`linear-gradient.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **自定义按钮波纹**（`wave.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
14. **移除两个汉字之间的空格**（`chinese-space.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
15. **自定义禁用样式背景**（`custom-disabled-bg.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
16. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onClick` | 点击 | 点击按钮时的回调 |
| `disabled` | 禁用 | 设置按钮失效状态 |
| `loading` | 加载中 | 设置按钮载入状态 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 语法糖 | `basic.tsx` | 否 |
| 颜色与变体 | `color-variant.tsx` | 否 |
| 调试颜色与变体 | `debug-color-variant` | 是 |
| 按钮图标 | `icon.tsx` | 否 |
| 按钮图标位置 | `icon-placement.tsx` | 否 |
| 调试图标按钮 | `debug-icon.tsx` | 是 |
| 调试按钮block属性 | `debug-block.tsx` | 是 |
| 按钮尺寸 | `size.tsx` | 否 |
| 不可用状态 | `disabled.tsx` | 否 |
| 加载中状态 | `loading.tsx` | 否 |
| 多个按钮组合 | `multiple.tsx` | 否 |
| 幽灵按钮 | `ghost.tsx` | 否 |
| 危险按钮 | `danger.tsx` | 否 |
| Block 按钮 | `block.tsx` | 否 |
| 废弃的 Block 组 | `legacy-group.tsx` | 是 |
| 加载中状态 bug 还原 | `chinese-chars-loading.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |
| 渐变按钮 | `linear-gradient.tsx` | 否 |
| 自定义按钮波纹 | `wave.tsx` | 否 |
| 移除两个汉字之间的空格 | `chinese-space.tsx` | 否 |
| 自定义禁用样式背景 | `custom-disabled-bg.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |

### 2.6 FAQ

## FAQ

### 类型和颜色与变体如何选择？ {#faq-type-color-variant}

类型本质上是颜色与变体的语法糖，内部为其提供了一组颜色与变体的映射关系。如果两者同时存在，优先使用颜色与变体。

```jsx
click
```

等同于

```jsx

  click

```

### 如何关闭点击波纹效果？ {#faq-close-wave-effect}

如果你不需要这个特性，可以设置 [ConfigProvider](/components/config-provider-cn#api) 的 `wave` 的 `disabled` 为 `true`。

```jsx

  click

```

.site-button-ghost-wrapper {
  padding: 16px;
  background: rgb(190, 200, 200);
}

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

通过设置 Button 的属性来产生不同的按钮样式，推荐顺序为：`type` -> `shape` -> `size` -> `loading` -> `disabled`。

按钮的属性说明如下：

| 属性 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| autoInsertSpace | 我们默认提供两个汉字之间的空格，可以设置 `autoInsertSpace` 为 `false` 关闭 | boolean | `true` | 5.17.0 | 5.17.0 |
| block | 将按钮宽度调整为其父宽度的选项 | boolean | false | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | 6.0.0 | 6.0.0 |
| color | 设置按钮的颜色 | `default` \| `primary` \| `danger` \| [PresetColors](#presetcolors) | `variant="solid"` 时为 `primary` | `default`、`primary` 和 `danger`: 5.21.0, `PresetColors`: 5.23.0, `solid` 默认颜色: 6.4.0 | 5.25.0 |
| danger | 语法糖，设置危险按钮。当设置 `color` 时会以后者为准 | boolean | false | disabled | 设置按钮失效状态 | boolean | false | ghost | 幽灵属性，使按钮背景透明 | boolean | false | href | 点击跳转的地址，指定此属性 button 的行为和 a 链接一致 | string | - | htmlType | 设置 `button` 原生的 `type` 值，可选值请参考 [HTML 标准](https://developer.mozilla.org/zh-CN/docs/Web/HTML/Element/button#type) | `submit` \| `reset` \| `button` | `button` | icon | 设置按钮的图标组件 | ReactNode | - | iconPlacement | 设置按钮图标组件的位置 | `start` \| `end` | `start` | - | × |
| ~~iconPosition~~ | 设置按钮图标组件的位置,请使用 `iconPlacement` 替换 | `start` \| `end` | `start` | 5.17.0 | × |
| loading | 设置按钮载入状态 | boolean \| { delay: number, icon: ReactNode } | false | icon: 5.23.0 | × |
| loadingIcon | （仅支持全局配置）设置按钮的加载图标 | ReactNode | `<LoadingOutlined />` | onClick | 点击按钮时的回调 | (event: React.MouseEvent<HTMLElement, MouseEvent>) => void | - | shape | 设置按钮形状 | `default` \| `circle` \| `round` | `default` | size | 设置按钮大小 | `large` \| `medium` \| `small` | `medium` | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 6.0.0 | 6.0.0 |
| target | 相当于 a 链接的 target 属性，href 存在时生效 | string | - | type | 语法糖，设置按钮类型。当设置 `variant` 与 `color` 时以后者为准 | `primary` \| `dashed` \| `link` \| `text` \| `default` | `default` | variant | 设置按钮的变体 | `outlined` \| `dashed` \| `solid` \| `filled` \| `text` \| `link` | - | 5.21.0 | 5.25.0 |

支持原生 button 的其他所有属性。

### PresetColors

> type PresetColors = 'blue' | 'purple' | 'cyan' | 'green' | 'magenta' | 'pink' | 'red' | 'orange' | 'yellow' | 'volcano' | 'geekblue' | 'lime' | 'gold';

### 导入方式

```js
import { Button } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `autoInsertSpace` | 我们默认提供两个汉字之间的空格，可以设置 `autoInsertSpace` 为 `false` 关闭 | boolean | `true` | 5.17.0 |
| `block` | 将按钮宽度调整为其父宽度的选项 | boolean | false | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `color` | 设置按钮的颜色 | `default` \| `primary` \| `danger` \| [PresetColors](#presetcolors) | `variant="solid"` 时为 `primary` | `default`、`primary` 和 `danger`: 5.21.0, `PresetColors`: 5.23.0, `solid` 默认颜色: 6.4.0 |
| `danger` | 语法糖，设置危险按钮。当设置 `color` 时会以后者为准 | boolean | false | — |
| `disabled` | 设置按钮失效状态 | boolean | false | — |
| `ghost` | 幽灵属性，使按钮背景透明 | boolean | false | — |
| `href` | 点击跳转的地址，指定此属性 button 的行为和 a 链接一致 | string | - | — |
| `htmlType` | 设置 `button` 原生的 `type` 值，可选值请参考 [HTML 标准](https://developer.mozilla.org/zh-CN/docs/Web/HTML/Element/button#type) | `submit` \| `reset` \| `button` | `button` | — |
| `icon` | 设置按钮的图标组件 | ReactNode | - | — |
| `iconPlacement` | 设置按钮图标组件的位置 | `start` \| `end` | `start` | - |
| `iconPosition` | 设置按钮图标组件的位置,请使用 `iconPlacement` 替换 | `start` \| `end` | `start` | 5.17.0 |
| `loading` | 设置按钮载入状态 | boolean \| { delay: number, icon: ReactNode } | false | icon: 5.23.0 |
| `loadingIcon` | （仅支持全局配置）设置按钮的加载图标 | ReactNode | `` | — |
| `onClick` | 点击按钮时的回调 | (event: React.MouseEvent) => void | - | — |
| `shape` | 设置按钮形状 | `default` \| `circle` \| `round` | `default` | — |
| `size` | 设置按钮大小 | `large` \| `medium` \| `small` | `medium` | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `target` | 相当于 a 链接的 target 属性，href 存在时生效 | string | - | — |
| `type` | 语法糖，设置按钮类型。当设置 `variant` 与 `color` 时以后者为准 | `primary` \| `dashed` \| `link` \| `text` \| `default` | `default` | — |
| `variant` | 设置按钮的变体 | `outlined` \| `dashed` \| `solid` \| `filled` \| `text` \| `link` | - | 5.21.0 |

---

## 4. gpui kit 实现要点（工程清单）

实现 / 重写 gpui kit **Button** 时，除下列通用项外，**必须以 §6 为 1:1 产品验收依据**。

1. **配置面**：覆盖 §6.8 P0 字段；P1 可分期但命名预留。
2. **视觉态**：default / hover / active / focus / disabled / loading（§6.4）。
3. **尺寸态**：small / medium / large（§6.2）。
4. **主题**：度量与颜色一律走 Theme Token（§6.2、§6.5）；禁止硬编码品牌色当唯一默认。
5. **无障碍**：可聚焦、Space/Enter 激活、读屏名 = label（§6.6）。
6. **RTL**：`iconPlacement` start/end 随写作方向镜像。
7. **动效**：波纹/旋转在 reduced-motion 下可关（§6.7）。
8. **示例矩阵**：§6.9 用例全部可勾选。
9. **纪律**：遵循 [`UI_KIT_DEV_GUIDE.md`](../UI_KIT_DEV_GUIDE.md)（Default + Set + rebuild，禁止魔法 offset）。
10. **对齐级别**：遵循 [`UI_KIT_ANT_V5_SPEC.md`](../UI_KIT_ANT_V5_SPEC.md) L1–L4 定义（§6.1）。

---

## 5. 参考链接

- 官方文档：https://ant.design/components/button
- 中文文档：https://ant.design/components/button-cn
- 设计规范：https://ant.design/docs/spec/buttons
- 源码：https://github.com/ant-design/ant-design/tree/master/components/button
- 本库对齐规格：[`UI_KIT_ANT_V5_SPEC.md`](../UI_KIT_ANT_V5_SPEC.md)
- 本库实现纪律：[`UI_KIT_DEV_GUIDE.md`](../UI_KIT_DEV_GUIDE.md)
- 驱动 gpui kit：`button`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd 文档补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> 本章为 **全库控件 1:1 增量规格的样板**（最细）。其它控件已按同级结构补齐 §6；实现时以各控件自己的 §6 为验收，复杂处可回看本章。

### 6.1 对齐级别定义（Button）

| 级别 | 名称 | Button 上的含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 点击、禁用、loading 吞事件、键盘激活、type/color/variant 语义切换正确 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 高度、字号、圆角、水平 padding、主色/边框/禁用色读 Theme，与下表基线一致 | `ant_style_test` + 读 Token 断言 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态（如 primary middle）截图与仓库基线一致（允许 AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design Button 并排「一眼同系」 | 建/大改基线时人眼签字，非 CI 绑官网 |

**明确不做（Button）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 依赖 CSS 的复杂渐变波纹算法 100% 复刻（可用等价反馈，见 §6.7）。  
- 原生 HTML `submit`/`reset` 表单语义以外的浏览器默认行为。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`）。实现必须通过 Token 读取，下表「默认值」为 Token 未覆盖时的回落。

#### 6.2.1 尺寸档位

| size（antd） | kit 枚举建议 | 高度 h | 字号 | 水平 padding | 圆角 | 图标约 |
| --- | --- | --- | --- | --- | --- | --- |
| `small` | `ButtonSmall` | **24**（`controlHeightSM`） | **12**（`fontSizeSM`） | **7**（`buttonPaddingInlineSM`） | **4**（`borderRadiusSM`） | ≈ 字号 |
| `medium`（默认） | `ButtonMiddle` | **32**（`controlHeight`） | **14**（`fontSize`） | **15**（`buttonPaddingInline`） | **6**（`borderRadius`） | ≈ 字号+2 |
| `large` | `ButtonLarge` | **40**（`controlHeightLG`） | **16**（`fontSizeLG`） | **15**（`buttonPaddingInlineLG`） | **8**（`borderRadiusLG`） | ≈ 字号 |

其它几何：

| 项 | 默认 | Token / 说明 |
| --- | --- | --- |
| 边框线宽 | **1** | `lineWidth`；`solid` 主按钮可无描边（fill 即边界） |
| 图标与文字间距 | **8** 左右 | 约 `marginXS(4)+4`；small 可 4 |
| 仅图标 + `shape=circle` | 宽=高=h | 内容水平垂直居中 |
| `shape=round` | 胶囊 | 圆角 ≈ h/2（或 token 大圆角） |
| `block=true` | 宽=父容器 | 高度仍按 size |
| Focus ring | 可见 | 半径≈控件圆角；outset ≈ 1.5px（实现可调，但必须可见） |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 主色 / hover / active | `colorPrimary` / `PrimaryHover` / `PrimaryActive` | solid primary |
| 危险色及态 | `colorError` + hover/active 变体 | danger / color=danger |
| 成功 / 警告（若支持 preset 子集） | `colorSuccess` / `colorWarning` | P1 |
| 默认字色 / 反白字 | `colorText` / `colorTextInverse` | solid 上用反白 |
| 边框 / 容器底 | `colorBorder` / `colorBgContainer` | default outlined |
| 文本按钮 hover 底 | `colorBgTextHover` / `BgTextActive` | 弱填充 |
| 禁用底 / 禁用字 | `colorDisabledBg` / `colorDisabledText` | 全局禁用态 |

**PresetColors 全色板**（blue/purple/…）属 **P1**：P0 至少 `default | primary | danger`（+ 可选 success/warning 若 Theme 已有）。

### 6.3 `type` ↔ `color` + `variant` 映射

与 antd 一致：`type` 为语法糖；同时存在 `color`/`variant` 时 **以后者为准**。

| type | 等价 color | 等价 variant |
| --- | --- | --- |
| `primary` | `primary` | `solid` |
| `default` | `default` | `outlined` |
| `dashed` | `default` | `dashed` |
| `text` | `default` | `text` |
| `link` | `primary`（链接强调） | `link` |
| `danger` 语法糖 | 视为 `color=danger`（与当前 variant 组合） | 保持 / 按实现解析顺序 |

`variant` 全集：`solid | outlined | dashed | filled | text | link`。

### 6.4 交互状态机（L1）

```text
                     ┌──────────────┐
                     │   disabled   │◄──── SetDisabled(true) 或 Loading 期间视为不可点*
                     └──────▲───────┘
                            │
  mount ──► default ──hover──► hovered ──press──► pressed ──release/in-bounds──► click
               │                  │                    │
               │                  └── leave ───────────┘
               └── focus (键盘) ── Space/Enter ──► click
               └── SetLoading(true) ──► loading（显示 spinner，吞重复 click）
```

\*Loading 时：控件表现为不可重复提交；是否保留焦点由实现定，但 **不得再次触发 OnClick**。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| B-S1 | `disabled=true` | 不触发 OnClick；样式为禁用态 |
| B-S2 | `loading=true` | 显示加载指示；不触发 OnClick；可与 disabled 样式叠加 |
| B-S3 | 指针 press 后在界内 release | 触发 **一次** OnClick |
| B-S4 | press 后拖出界外 release | **不**触发 OnClick |
| B-S5 | 聚焦 + Space 或 Enter | 触发 OnClick（与可访问按钮一致） |
| B-S6 | `loading` 从 false→true | 立即进入 loading 外观；可选 delay（P1：`loading.delay`） |
| B-S7 | 切换 type/variant/color/size/ghost/danger | 下一帧/rebuild 后 chrome 正确，无残留错误边框 |
| B-S8 | `block` 开关 | 宽度约束变化正确，高度不变 |
| B-S9 | `iconPlacement` start/end | 图标与文字顺序交换；间距保持 |
| B-S10 | 仅图标 circle | 命中区域为圆形/方框按 shape；标签可空但 a11y 名不能空（见 §6.6） |

### 6.5 视觉 chrome 规则（L2 摘要）

| variant | 默认填充 | 默认边框 | 默认文字 | hover | 备注 |
| --- | --- | --- | --- | --- | --- |
| `solid` | accent | 无/同填充 | inverse | accentHover | primary/danger 主按钮 |
| `outlined` | 容器底 | border/accent | text/accent | 浅底或边框强调 | default 按钮 |
| `dashed` | 同 outlined | **虚线** border | 同 outlined | 同 outlined | dash 周期约 3–2 |
| `filled` | 浅 accent 底 | 无/弱 | accent | 稍深浅底 | 弱强调 |
| `text` | 透明 | 无 | text/accent | 文本 hover 底 | |
| `link` | 透明 | 无 | primary/accent | 悬停下划线可选 | |

| 修饰 | 规则 |
| --- | --- |
| `ghost` | 填充透明；文字与边框用可反色/浅色，适配深色底 |
| `danger` / `color=danger` | accent 走 error 色系 |
| `disabled` | 禁用底+禁用字；无 hover 变化 |
| `loading` | 前导（或替换 icon）旋转指示；文字可保留 |

**渐变按钮（官方 demo linear-gradient）**：属 **P1**——允许业务 `Style` 覆盖背景；不强制内置渐变 API。

**波纹 wave**：属 **P1**——P0 可用 press 态色反馈代替；若实现 wave，须尊重 reduced-motion。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | 按钮（或等价可激活控件） |
| 名称 | 默认识别名 = `label`；仅图标时 **必须** 提供无障碍名（API：`AriaLabel` / `SetAriaLabel` 或等价） |
| 焦点 | Tab 可聚焦；Focus ring 可见（§6.2） |
| 键盘 | Space / Enter 触发 click（B-S5） |
| 禁用 | disabled/loading 不触发激活；读屏需感知禁用（若平台支持） |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 点击 / 键盘 / 禁用 / loading | **对等** | P0 L1 |
| size / type / variant / color(default,primary,danger) / shape / block / ghost / iconPlacement | **对等** | P0 L1+L2 |
| 中文两字间距 `autoInsertSpace` | **对等或近似**（可按字形宽度插空） | P1 |
| `href` / `target` | **近似**：桌面可映射为打开 URL 回调，非 `<a>` 导航 | P1 |
| `htmlType` submit/reset | **近似**：由上层 Form 解释，Button 抛事件即可 | P1 |
| 点击波纹 wave | **近似**或 P1 不做 | P1 |
| 渐变预设 | **Style 覆盖**，非必做 API | P1 |
| PresetColors 全色板 | **分期** | P1 |
| `loading.delay` / 自定义 loading 图标 | **分期** | P1 |
| Semantic `classNames`/`styles` | 映射为 kit 语义样式钩子 / `Style` | P1 |
| ConfigProvider 全局 button 默认 | 随 ConfigProvider 能力 | P1 |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `type` | primary / default / dashed / text / link |
| `size` | small / medium / large |
| `shape` | default / circle / round |
| `variant` + `color` | variant 全集；color：default / primary / danger |
| `danger` / `ghost` / `block` / `disabled` / `loading`（bool） | 状态与修饰 |
| `icon` + `iconPlacement` | start / end |
| `onClick` | 单击一次 |
| 状态 | default/hover/active/focus/disabled/loading |
| 度量 | §6.2 表 |
| a11y | §6.6 |
| 示例 | 语法糖、尺寸、禁用、loading、图标、图标位置、幽灵、危险、Block、颜色与变体（主路径） |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| PresetColors 全色板 | blue/purple/… |
| `loading` 对象（delay、自定义 icon） | |
| `autoInsertSpace` | 中文两字插空 |
| `href` / `target` / `htmlType` | 桌面映射 |
| wave / 渐变内置 API | |
| `classNames` / `styles` 语义节点 | |
| ConfigProvider 全局 button 默认 | autoInsertSpace、默认 variant 等 |
| success/warning 等扩展 color | 若 Theme 支持可提前 |

### 6.9 验收用例表（可测）

> 每个用例对应测试名建议：`TestButton_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 全部通过** 才可宣称 Button 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| BTN-01 | L1 | NewButton("确定") 默认 | type=default，size=middle，可点 |
| BTN-02 | L1 | SetType(primary) 后点击 | 触发 OnClick 一次 |
| BTN-03 | L1 | SetDisabled(true) 后点击/键盘 | 不触发 OnClick |
| BTN-04 | L1 | SetLoading(true) 后点击 | 不触发 OnClick；有 spinner |
| BTN-05 | L1 | press 后移出 release | 不触发 OnClick |
| BTN-06 | L1 | 聚焦 + Enter/Space | 触发 OnClick |
| BTN-07 | L2 | size=small/middle/large 布局 | 高度 24/32/40（±0.5） |
| BTN-08 | L2 | middle 水平 padding | 内容区与 chrome 符合 paddingInline≈15 |
| BTN-09 | L2 | primary solid 色 | 填充=primary，字=inverse |
| BTN-10 | L2 | default outlined | 有边框，非实心主色底 |
| BTN-11 | L2 | dashed | 边框虚线 |
| BTN-12 | L2 | text/link | 无实心底/弱边框 |
| BTN-13 | L1/L2 | danger 或 color=danger | 使用 error 色系 |
| BTN-14 | L1/L2 | ghost=true（深色底场景） | 透明底，边框/字可见 |
| BTN-15 | L1 | block=true | 宽度随父（布局测） |
| BTN-16 | L1 | shape=circle 仅图标 | 宽高相等≈h |
| BTN-17 | L1 | shape=round | 大圆角/胶囊 |
| BTN-18 | L1 | icon + label，placement end | 图标在文字后 |
| BTN-19 | L1 | type 与 variant 同时设 | variant 优先 |
| BTN-20 | L1 | color+variant 矩阵抽测 | solid/outlined × primary/default/danger |
| BTN-21 | L3 | primary middle 截图 | 与 golden 基线一致（容差内） |
| BTN-22 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| BTN-23 | L1 | 仅图标无 label | 必须设 AriaLabel 否则测试失败 |
| BTN-24 | L2 | disabled 外观 | 禁用色，无 hover 高亮 |
| BTN-25 | P1 | loading delay | 延迟后才显示 spinner |
| BTN-26 | P1 | autoInsertSpace | 「确定」两字视觉间距 |
| BTN-27 | P1 | wave 关闭 | Config/reduced-motion 下无波纹 |

### 6.10 产品 API 契约（Go kit 侧，便于 1:1 重写）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewButton(label string) *Button

// 形态
SetType(ButtonType)           // default|primary|dashed|text|link
SetSize(ButtonSize)           // small|middle|large
SetShape(ButtonShape)         // default|circle|round
SetVariant(ButtonVariant)     // auto|solid|outlined|dashed|filled|text|link
SetColor(ButtonColor)         // default|primary|danger|(success|warning P1)
SetDanger(bool)
SetGhost(bool)
SetBlock(bool)

// 内容
SetLabel(string)
SetIcon(name or node)
SetIconPlacement(start|end)
SetLoading(bool)              // P1: SetLoadingConfig(delay, icon)

// 状态
SetDisabled(bool)
// OnClick func()

// 主题 / 覆盖
SetTheme(*Theme) / Theme 字段
Style 可选覆盖

// a11y
SetAriaLabel(string)          // 仅图标必填
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Type | default |
| Size | middle（medium） |
| Shape | default |
| Variant | auto（由 Type 推导） |
| Color | default |
| Danger/Ghost/Block/Loading/Disabled | false |
| IconPlacement | start |

### 6.11 结构与绘制分层（实现提示）

```text
Pressable（命中、hover/press/focus、键盘）
  └─ Decorated（底、边框、圆角、虚线、高度）
       └─ Flex(Row) gap
            Spinner? · Icon? · Text · Icon?
```

- 命中区域 = 布局盒；`hit == layout == paint` 边界一致。  
- Loading spinner 可用 Canvas 矢量环，随 `Tick` 旋转。  
- 不在 kit 内自建帧循环；跟随 Host/App 的 Tick。

### 6.12 完成定义（DoD）

同时满足即可宣布 **Button 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 相关用例（BTN-01–BTN-24）** 测试通过。  
3. L2 度量与 Token 断言通过（高度档位等）。  
4. L3 golden 至少覆盖 primary middle 与 default middle 之一。  
5. gallery 展示：type 语法糖、尺寸、颜色变体、图标、loading、disabled、ghost、danger、block。  
6. `coverage.go` Notes 更新为：P0 已对齐 docs/antd/button.md §6；P1 项显式列出。

---

**本章用法**：重写 `ui/kit` Button 时，以 §6 为需求与验收；§1–§3 为 antd 能力全集参考；§6.8 为范围裁剪。其它控件复制 §6 结构即可形成同等 1:1 产品规格。
