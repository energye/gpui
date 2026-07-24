# Radio 单选框
> 来源：[Ant Design 6.5.x Radio](https://ant.design/components/radio)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：用于在多个备选项中选中单个状态。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
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

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

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

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Radio** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/radio/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Radio）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 点击/切换、禁用、键盘激活、受控值正确 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Radio）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：用于在多个备选项中选中单个状态。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 圆点 | **16** | interactive |
| Button 高 | **32/24/40** | controlHeight* |
| 控件高度 middle | **32** | `controlHeight` |
| 控件高度 small | **24** | `controlHeightSM` |
| 控件高度 large | **40** | `controlHeightLG` |
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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据录入**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `checked` | 指定当前是否选中 | boolean | false |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `defaultChecked` | 初始是否选中 | boolean | false |
| `disabled` | 禁用 Radio | boolean | false |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |
| `value` | 根据 value 进行比较，判断是否选中 | any | - |
| `block` | 将 RadioGroup 宽度调整为其父宽度的选项 | boolean | false |
| `buttonStyle` | RadioButton 的风格样式，目前有描边和填色两种风格 | `outline` \ | `solid` |
| `defaultValue` | 默认选中的值 | any | - |
| `name` | RadioGroup 下所有 `input[type="radio"]` 的 `name` 属性。若未设置，则将回… | string | - |
| `options` | 以配置形式设置子元素 | string\[] \ | number\[] \ |
| `optionType` | 用于设置 Radio `options` 类型 | `default` \ | `button` |
| `orientation` | 排列方向 | `horizontal` \ | `vertical` |
| `size` | 大小，只对按钮样式生效 | `large` \ | `medium` \ |
| `vertical` | 值为 true，Radio Group 为垂直方向。与 `orientation` 同时存在，以 `orienta… | boolean | false |
| `onChange` | 选项变化时的回调函数 | function(e:Event) | - |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
Group value=v
  点 option A ──► value=A + onChange（互斥，仅 A）
  点已选 A ──► 保持 A（不取消，除非实现允许）
  optionType=button ──► 按钮皮
  buttonStyle=solid|outline ──► 填充/描边
  disabled option ──► 不可选
  键盘方向 ──► 组内移动选中
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| RDO-S1 | 两点不同 option | 仅后者选中 |
| RDO-S2 | `optionType=button` | 按钮组外观 |
| RDO-S3 | `buttonStyle=solid` | 实心选中态 |
| RDO-S4 | disabled 项 | 不可选 |
| RDO-S5 | 受控 value | 外部优先 |
| RDO-S6 | 圆点尺寸 | 16 |
| RDO-S7 | Radio.Button 高度 middle | 32 |
| RDO-S8 | 键盘方向 | 移动选中 |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | Token 默认皮 |
| hover / active | 可交互反馈 |
| focus | 可见 focus ring |
| checked/selected/active（适用者） | 主色强调 |
| disabled | 降对比；无 hover |
| loading | 指示器；防重复 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | button / checkbox / switch / radio 等与语义一致 |
| 名称 | 可交互必有名；仅图标必须 AriaLabel |
| 焦点 | Tab 可达；ring 可见 |
| 键盘 | Space/Enter 或方向键按角色 |
| 禁用 | 不可激活；读屏可感知（平台支持时） |

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
| `value` | 必须 |
| `defaultValue` | 必须 |
| `checked` | 必须 |
| `onChange` | 必须 |
| `disabled` | 必须 |
| `size` | 必须 |
| `options` | 必须 |
| `title` | 必须 |
| `orientation` | 必须 |
| 官方主路径示例 | 基本、不可用、单选组合、Radio.Group 垂直、Block 单选组合、Radio.Group 组合 - 配置方式、按钮样式、单选组合 - 配合 name 使用 |
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
| 其余示例 | 大小, 填底的按钮样式, 自定义语义结构的样式和类, _semantic.tsx |

### 6.9 验收用例表（可测）

> 测试名建议：`TestRadio_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Radio 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| RDO-01 | L1 | NewRadio 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| RDO-02 | L1 | 两点不同 option | 仅后者选中 |
| RDO-03 | L1 | `optionType=button` | 按钮组外观 |
| RDO-04 | L1 | `buttonStyle=solid` | 实心选中态 |
| RDO-05 | L1 | disabled 项 | 不可选 |
| RDO-06 | L1 | 受控 value | 外部优先 |
| RDO-07 | L1 | 圆点尺寸 | 16 |
| RDO-08 | L1 | Radio.Button 高度 middle | 32 |
| RDO-09 | L1 | 键盘方向 | 移动选中 |
| RDO-10 | L1 | 复现官方示例「基本」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RDO-11 | L1 | 复现官方示例「不可用」（`disabled.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RDO-12 | L1 | 复现官方示例「单选组合」（`radiogroup.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RDO-13 | L1 | 复现官方示例「Radio.Group 垂直」（`radiogroup-more.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RDO-14 | L1 | 复现官方示例「Block 单选组合」（`radiogroup-block.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RDO-15 | L1 | 复现官方示例「Radio.Group 组合 - 配置方式」（`radiogroup-options.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RDO-16 | L1 | 复现官方示例「按钮样式」（`radiobutton.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RDO-17 | L1 | 复现官方示例「单选组合 - 配合 name 使用」（`radiogroup-with-name.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RDO-18 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| RDO-19 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| RDO-20 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| RDO-21 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| RDO-22 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| RDO-23 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| RDO-24 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewRadio(...) *Radio

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
Pressable
  └─ Decorated chrome
       └─ content (icon/label/indicator)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Radio 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/radio.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Radio 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
