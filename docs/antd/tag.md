# Tag 标签
> 来源：[Ant Design 6.5.x Tag](https://ant.design/components/tag)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：进行标记和分类的小标签。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

进行标记和分类的小标签。

**Tag** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 多彩标签 | 复现「多彩标签」视觉与布局 |
| 动态添加和删除 | 复现「动态添加和删除」视觉与布局 |
| 可选择标签 | 复现「可选择标签」视觉与布局 |
| 添加动画 | 复现「添加动画」视觉与布局 |
| 图标按钮 | icon 与文本混排 |
| 预设状态的标签 | 复现「预设状态的标签」视觉与布局 |
| 可拖拽标签 | 复现「可拖拽标签」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `color`

- **说明**：标签色
- **类型**：string
- **默认值**：`variant="solid"` 时为 `default`
- **版本**：`solid` 默认颜色: 6.4.0

#### `disabled`

- **说明**：是否禁用标签
- **类型**：boolean
- **默认值**：false
- **版本**：6.0.0

#### `icon`

- **说明**：设置图标
- **类型**：ReactNode
- **默认值**：-

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `variant`

- **说明**：标签变体
- **类型**：`'filled' | 'solid' | 'outlined'`
- **默认值**：`'filled'`
- **版本**：6.0.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `filled` | 浅底填充 |
  | `solid` | 实心填充 |
  | `outlined` | 描边空心 |

#### `bordered`

- **说明**：是否带边框，请使用 `variant="filled"` 替代
- **类型**：boolean
- **默认值**：true

#### `checked`

- **说明**：设置标签的选中状态
- **类型**：boolean
- **默认值**：false

#### `options`

- **说明**：选项列表。对象类型的选项支持为每一项单独设置 `className` 和 `style`
- **类型**：`Array`
- **默认值**：-
- **版本**：`className` 和 `style`: 6.4.0

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

- 用于标记事物的属性和维度。
- 进行分类。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **多彩标签**（`colorful.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **动态添加和删除**（`control.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **可选择标签**（`checkable.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **添加动画**（`animation.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **图标按钮**（`icon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **预设状态的标签**（`status.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **可拖拽标签**（`draggable.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 选中值 |
| `defaultValue` | 非受控默认值 | 初始选中值 |
| `onChange` | 值变化 | 点击标签时触发的回调 |
| `disabled` | 禁用 | 是否禁用标签 |
| `options` | 数据化 options | 选项列表。对象类型的选项支持为每一项单独设置 `className` 和 `style` |
| `checked` | 选中布尔 | 设置标签的选中状态 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 多彩标签 | `colorful.tsx` | 否 |
| 动态添加和删除 | `control.tsx` | 否 |
| 可选择标签 | `checkable.tsx` | 否 |
| 添加动画 | `animation.tsx` | 否 |
| 图标按钮 | `icon.tsx` | 否 |
| 预设状态的标签 | `status.tsx` | 否 |
| 自定义关闭按钮 | `customize.tsx` | 是 |
| 可拖拽标签 | `draggable.tsx` | 否 |
| 组件 Token | `component-token.tsx` | 是 |
| 禁用标签 | `disabled.tsx` | 是 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |

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

### Tag

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | closeIcon | 自定义关闭按钮。5.7.0：设置为 `null` 或 `false` 时隐藏关闭按钮 | ReactNode | false | 4.4.0 | 5.14.0 |
| color | 标签色 | string | `variant="solid"` 时为 `default` | `solid` 默认颜色: 6.4.0 | × |
| disabled | 是否禁用标签 | boolean | false | 6.0.0 | × |
| href | 点击跳转的地址，指定此属性`tag`组件会渲染成 `<a>` 标签 | string | - | 6.0.0 | × |
| icon | 设置图标 | ReactNode | - | onClose | 关闭时的回调（可通过 `e.preventDefault()` 来阻止默认行为） | (e: React.MouseEvent<HTMLElement, MouseEvent>) => void | - | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | target | 相当于 a 标签的 target 属性，href 存在时生效 | string | - | 6.0.0 | × |
| variant | 标签变体 | `'filled' \| 'solid' \| 'outlined'` | `'filled'` | 6.0.0 | 6.0.0 |
| ~~bordered~~ | 是否带边框，请使用 `variant="filled"` 替代 | boolean | true | - | × |

### Tag.CheckableTag

| 参数     | 说明                 | 类型              | 默认值 | 版本   |
| -------- | -------------------- | ----------------- | ------ | ------ |
| checked  | 设置标签的选中状态   | boolean           | false  |        |
| icon     | 设置图标             | ReactNode         | -      | 5.27.0 |
| onChange | 点击标签时触发的回调 | (checked) => void | -      |        |

### Tag.CheckableTagGroup

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-group), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-group), string> | - | disabled | 禁用选中 | `boolean` | - | options | 选项列表。对象类型的选项支持为每一项单独设置 `className` 和 `style` | `Array<{ className?: string; label: ReactNode; style?: CSSProperties; value: string \| number } \| string \| number>` | - | `className` 和 `style`: 6.4.0 |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-group), CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-group), CSSProperties> | - | onChange | 点击标签时触发的回调 | `(value: string \| number \| Array<string \| number> \| null) => void` | - 
```js
import { Tag } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `closeIcon` | 自定义关闭按钮。5.7.0：设置为 `null` 或 `false` 时隐藏关闭按钮 | ReactNode | false | 4.4.0 |
| `color` | 标签色 | string | `variant="solid"` 时为 `default` | `solid` 默认颜色: 6.4.0 |
| `disabled` | 是否禁用标签 | boolean | false | 6.0.0 |
| `href` | 点击跳转的地址，指定此属性`tag`组件会渲染成 `` 标签 | string | - | 6.0.0 |
| `icon` | 设置图标 | ReactNode | - | — |
| `onClose` | 关闭时的回调（可通过 `e.preventDefault()` 来阻止默认行为） | (e: React.MouseEvent) => void | - | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `target` | 相当于 a 标签的 target 属性，href 存在时生效 | string | - | 6.0.0 |
| `variant` | 标签变体 | `'filled' \| 'solid' \| 'outlined'` | `'filled'` | 6.0.0 |
| `bordered` | 是否带边框，请使用 `variant="filled"` 替代 | boolean | true | - |
| `checked` | 设置标签的选中状态 | boolean | false | — |
| `onChange` | 点击标签时触发的回调 | (checked) => void | - | — |
| `defaultValue` | 初始选中值 | `string \| number \| Array \| null` | - | — |
| `multiple` | 多选模式 | `boolean` | - | — |
| `options` | 选项列表。对象类型的选项支持为每一项单独设置 `className` 和 `style` | `Array` | - | `className` 和 `style`: 6.4.0 |
| `value` | 选中值 | `string \| number \| Array \| null` | - | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Tag** 的验收清单：

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

---
## 5. 参考链接
- 官方文档：https://ant.design/components/tag
- 中文文档：https://ant.design/components/tag-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/tag
- 驱动 gpui kit：`tag`
