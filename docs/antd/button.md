# Button 按钮
> 来源：[Ant Design 6.5.x Button](https://ant.design/components/button)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：通用（General）  
> 说明：按钮用于开始一个即时操作。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
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
## 4. gpui kit 实现要点
实现 gpui kit 版 **Button** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **16** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/button
- 中文文档：https://ant.design/components/button-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/button
- 驱动 gpui kit：`button`
