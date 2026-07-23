# Radio 单选框
> 来源：[Ant Design 6.5.x Radio](https://ant.design/components/radio)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：用于在多个备选项中选中单个状态。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

用于在多个备选项中选中单个状态。

**Radio** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 不可用 | disabled 态 |
| 单选组合 | 复现「单选组合」视觉与布局 |
| Radio.Group 垂直 | 纵向布局 |
| Block 单选组合 | 宽度撑满父级 |
| Radio.Group 组合 - 配置方式 | 复现「Radio.Group 组合 - 配置方式」视觉与布局 |
| 按钮样式 | 复现「按钮样式」视觉与布局 |
| 单选组合 - 配合 name 使用 | 复现「单选组合 - 配合 name 使用」视觉与布局 |
| 大小 | 不同 size 档位 |
| 填底的按钮样式 | 复现「填底的按钮样式」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `checked`

- **说明**：指定当前是否选中
- **类型**：boolean
- **默认值**：false

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `disabled`

- **说明**：禁用 Radio
- **类型**：boolean
- **默认值**：false

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `block`

- **说明**：将 RadioGroup 宽度调整为其父宽度的选项
- **类型**：boolean
- **默认值**：false
- **版本**：5.21.0

#### `buttonStyle`

- **说明**：RadioButton 的风格样式，目前有描边和填色两种风格
- **类型**：`outline` | `solid`
- **默认值**：`outline`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `outline` | 描边按钮风 |
  | `solid` | 实心填充 |

#### `optionType`

- **说明**：用于设置 Radio `options` 类型
- **类型**：`default` | `button`
- **默认值**：`default`
- **版本**：4.4.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `default` | 默认中性外观 |
  | `button` | 按钮样式选项 |

#### `orientation`

- **说明**：排列方向
- **类型**：`horizontal` | `vertical`
- **默认值**：`horizontal`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `horizontal` | 水平排布 |
  | `vertical` | 垂直排布 |

#### `size`

- **说明**：大小，只对按钮样式生效
- **类型**：`large` | `medium` | `small`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `vertical`

- **说明**：值为 true，Radio Group 为垂直方向。与 `orientation` 同时存在，以 `orientation` 优先
- **类型**：boolean
- **默认值**：false
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `orientation` | 官方取值 `orientation` |

#### `title`

- **说明**：添加 Title 属性值
- **类型**：`string`
- **默认值**：-
- **版本**：4.4.0

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

- 用于在多个备选项中选中单个状态。
- 和 Select 的区别是，Radio 所有选项默认可见，方便用户在比较中选择，因此选项不宜过多。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **不可用**（`disabled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **单选组合**（`radiogroup.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **Radio.Group 垂直**（`radiogroup-more.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **Block 单选组合**（`radiogroup-block.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **Radio.Group 组合 - 配置方式**（`radiogroup-options.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **按钮样式**（`radiobutton.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **单选组合 - 配合 name 使用**（`radiogroup-with-name.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **填底的按钮样式**（`radiobutton-solid.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 根据 value 进行比较，判断是否选中 |
| `defaultValue` | 非受控默认值 | 默认选中的值 |
| `onChange` | 值变化 | 选项变化时的回调函数 |
| `disabled` | 禁用 | 禁用 Radio |
| `options` | 数据化 options | 以配置形式设置子元素 |
| `checked` | 选中布尔 | 指定当前是否选中 |
| `defaultChecked` | 默认选中 | 初始是否选中 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 不可用 | `disabled.tsx` | 否 |
| 单选组合 | `radiogroup.tsx` | 否 |
| Radio.Group 垂直 | `radiogroup-more.tsx` | 否 |
| Block 单选组合 | `radiogroup-block.tsx` | 否 |
| Radio.Group 组合 - 配置方式 | `radiogroup-options.tsx` | 否 |
| 按钮样式 | `radiobutton.tsx` | 否 |
| 单选组合 - 配合 name 使用 | `radiogroup-with-name.tsx` | 否 |
| 大小 | `size.tsx` | 否 |
| 填底的按钮样式 | `radiobutton-solid.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 测试 Badge 的样式 | `badge.tsx` | 是 |
| Group 内图标等宽 | `debug-group-width.tsx` | 是 |
| 线框风格 | `wireframe.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |
| Upload Debug | `debug-upload.tsx` | 是 |

### 2.5 实例方法 / Ref

#### 方法

### Radio

| 名称    | 描述     |
| ------- | -------- |
| blur()  | 移除焦点 |
| focus() | 获取焦点 |

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

### Radio/Radio.Button

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| checked | 指定当前是否选中 | boolean | false | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | 6.0.0 | 6.0.0 |
| defaultChecked | 初始是否选中 | boolean | false | disabled | 禁用 Radio | boolean | false | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 6.0.0 | 6.0.0 |
| value | 根据 value 进行比较，判断是否选中 | any | - 
### Radio.Group

单选框组合，用于包裹一组 `Radio`。

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| block | 将 RadioGroup 宽度调整为其父宽度的选项 | boolean | false | 5.21.0 |
| buttonStyle | RadioButton 的风格样式，目前有描边和填色两种风格 | `outline` \| `solid` | `outline` | defaultValue | 默认选中的值 | any | - | name | RadioGroup 下所有 `input[type="radio"]` 的 `name` 属性。若未设置，则将回退到随机生成的名称 | string | - | optionType | 用于设置 Radio `options` 类型 | `default` \| `button` | `default` | 4.4.0 |
| orientation | 排列方向 | `horizontal` \| `vertical` | `horizontal` | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 6.0.0 |
| value | 用于设置当前选中的值 | any | - | onChange | 选项变化时的回调函数 | function(e:Event) | - 
| 属性 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| label | 用于作为 Radio 选项展示的文本 | `string` | - | 4.4.0 |
| value | 关联 Radio 选项的值 | `string` \| `number` \| `boolean` | - | 4.4.0 |
| style | 应用到 Radio 选项的 style | `React.CSSProperties` | - | 4.4.0 |
| className | Radio 选项的类名 | `string` | - | 5.25.0 |
| disabled | 指定 Radio 选项是否要禁用 | `boolean` | `false` | 4.4.0 |
| title | 添加 Title 属性值 | `string` | - | 4.4.0 |
| id | 添加 Radio Id 属性值 | `string` | - | 4.4.0 |
| onChange | 当 Radio Group 的值发送改变时触发 | `(e: CheckboxChangeEvent) => void;` | - | 4.4.0 |
| required | 指定 Radio 选项是否必填 | `boolean` | `false` | 4.4.0 |

## 方法

### Radio

| 名称    | 描述     |
| ------- | -------- |
| blur()  | 移除焦点 |
| focus() | 获取焦点 |

### 导入方式

```js
import { Radio } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `checked` | 指定当前是否选中 | boolean | false | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `defaultChecked` | 初始是否选中 | boolean | false | — |
| `disabled` | 禁用 Radio | boolean | false | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `value` | 根据 value 进行比较，判断是否选中 | any | - | — |
| `block` | 将 RadioGroup 宽度调整为其父宽度的选项 | boolean | false | 5.21.0 |
| `buttonStyle` | RadioButton 的风格样式，目前有描边和填色两种风格 | `outline` \| `solid` | `outline` | — |
| `defaultValue` | 默认选中的值 | any | - | — |
| `name` | RadioGroup 下所有 `input[type="radio"]` 的 `name` 属性。若未设置，则将回退到随机生成的名称 | string | - | — |
| `options` | 以配置形式设置子元素 | string\[] \| number\[] \| Array<[CheckboxOptionType](#checkboxoptiontype)> | - | — |
| `optionType` | 用于设置 Radio `options` 类型 | `default` \| `button` | `default` | 4.4.0 |
| `orientation` | 排列方向 | `horizontal` \| `vertical` | `horizontal` | — |
| `size` | 大小，只对按钮样式生效 | `large` \| `medium` \| `small` | - | — |
| `vertical` | 值为 true，Radio Group 为垂直方向。与 `orientation` 同时存在，以 `orientation` 优先 | boolean | false | — |
| `onChange` | 选项变化时的回调函数 | function(e:Event) | - | — |
| `label` | 用于作为 Radio 选项展示的文本 | `string` | - | 4.4.0 |
| `style` | 应用到 Radio 选项的 style | `React.CSSProperties` | - | 4.4.0 |
| `className` | Radio 选项的类名 | `string` | - | 5.25.0 |
| `title` | 添加 Title 属性值 | `string` | - | 4.4.0 |
| `id` | 添加 Radio Id 属性值 | `string` | - | 4.4.0 |
| `required` | 指定 Radio 选项是否必填 | `boolean` | `false` | 4.4.0 |
| `blur()` | 移除焦点 | — | — | — |
| `focus()` | 获取焦点 | — | — | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Radio** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **11** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/radio
- 中文文档：https://ant.design/components/radio-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/radio
- 驱动 gpui kit：`radio`
