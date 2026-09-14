# Checkbox 多选框
> 来源：[Ant Design 6.5.x Checkbox](https://ant.design/components/checkbox)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：收集用户的多项选择。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

收集用户的多项选择。

**Checkbox** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本用法 | 复现「基本用法」视觉与布局 |
| 不可用 | disabled 态 |
| 受控的 Checkbox | 复现「受控的 Checkbox」视觉与布局 |
| Checkbox 组 | 复现「Checkbox 组」视觉与布局 |
| 全选 | 复现「全选」视觉与布局 |
| 布局 | 复现「布局」视觉与布局 |
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

#### `disabled`

- **说明**：失效状态
- **类型**：boolean
- **默认值**：false

#### `indeterminate`

- **说明**：设置 indeterminate 状态，只负责样式控制
- **类型**：boolean
- **默认值**：false

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `title`

- **说明**：选项的 title
- **类型**：`string`
- **默认值**：-

#### `style`

- **说明**：选项的样式
- **类型**：`React.CSSProperties`
- **默认值**：-

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

- 在一组可选项中进行多项选择时；
- 单独使用可以表示两种状态之间的切换，和 `switch` 类似。区别在于切换 `switch` 会直接触发状态改变，而 `checkbox` 一般用于状态标记，需要和提交操作配合。

### 2.2 核心功能（按官方示例拆解）

1. **基本用法**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **不可用**（`disabled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **受控的 Checkbox**（`controller.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **Checkbox 组**（`group.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **全选**（`check-all.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **布局**（`layout.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 指定选中的选项 |
| `defaultValue` | 非受控默认值 | 默认选中的选项 |
| `onChange` | 值变化 | 变化时的回调函数 |
| `disabled` | 禁用 | 失效状态 |
| `options` | 数据化 options | 指定可选项 |
| `checked` | 选中布尔 | 指定当前是否选中 |
| `indeterminate` | 半选 | 设置 indeterminate 状态，只负责样式控制 |
| `defaultChecked` | 默认选中 | 初始是否选中 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本用法 | `basic.tsx` | 否 |
| 不可用 | `disabled.tsx` | 否 |
| 受控的 Checkbox | `controller.tsx` | 否 |
| Checkbox 组 | `group.tsx` | 否 |
| 全选 | `check-all.tsx` | 否 |
| 布局 | `layout.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 同行布局 | `debug-line.tsx` | 是 |
| 禁用下的 Tooltip | `debug-disable-popover.tsx` | 是 |
| Group 内勾选框等宽 | `debug-group-width.tsx` | 是 |
| 自定义 lineWidth | `custom-line-width.tsx` | 是 |

### 2.5 实例方法 / Ref

#### 方法

### 方法 {#methods}

#### Checkbox

| 名称          | 描述                      | 版本   |
| ------------- | ------------------------- | ------ |
| blur()        | 移除焦点                  |        |
| focus()       | 获取焦点                  |        |
| nativeElement | 返回 Checkbox 的 DOM 节点 | 5.17.3 |

### 2.6 FAQ

## FAQ

### 为什么在 Form.Item 下不能绑定数据？ {#faq-form-item-limitations}

Form.Item 默认绑定值属性到 `value` 上，而 Checkbox 的值属性为 `checked`。你可以通过 `valuePropName` 来修改绑定的值属性。

```tsx | pure

  

```

### 2.7 组合关系

- **依赖等级 L3**：表单件（无浮层）；单框 + Group，多选值走 `[]string`；先做 `form` 值桥（`valuePropName=checked`）再做本件。
- **Form**：`checked`（单框）/ `value []string`（Group），`title` 可作可访问名回落。
- **ConfigProvider**：禁用/尺寸下发（行高对齐 controlHeight）。
- **文件归属**：`ui/kit/checkbox/`（单框指示器 + Group + 全选半选）。
---
## 3. 配置（API）
通用属性参考：[Common props](https://ant.design/docs/react/common-props)。

以下为官方 API 全文，作为 kit 配置面与类型设计的权威清单。

## API

通用属性参考：[通用属性](/docs/react/common-props)

#### Checkbox

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| checked | 指定当前是否选中 | boolean | false | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | defaultChecked | 初始是否选中 | boolean | false | disabled | 失效状态 | boolean | false | indeterminate | 设置 indeterminate 状态，只负责样式控制 | boolean | false | onChange | 变化时的回调函数 | (e: CheckboxChangeEvent) => void | - | onBlur | 失去焦点时的回调 | function() | - | onFocus | 获得焦点时的回调 | function() | - | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - 
#### Checkbox.Group

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| defaultValue | 默认选中的选项 | (string \| number)\[] | \[] | name | CheckboxGroup 下所有 `input[type="checkbox"]` 的 `name` 属性 | string | - | value | 指定选中的选项 | (string \| number \| boolean)\[] | \[] | className | 选项的类名 | `string` | - | 5.25.0 |
| style | 选项的样式 | `React.CSSProperties` | - 
##### Option

```typescript
interface Option {
  label: string;
  value: string;
  disabled?: boolean;
}
```

### 方法 {#methods}

#### Checkbox

| 名称          | 描述                      | 版本   |
| ------------- | ------------------------- | ------ |
| blur()        | 移除焦点                  |        |
| focus()       | 获取焦点                  |        |
| nativeElement | 返回 Checkbox 的 DOM 节点 | 5.17.3 |

### 导入方式

```js
import { Checkbox } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `checked` | 指定当前是否选中 | boolean | false | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `defaultChecked` | 初始是否选中 | boolean | false | — |
| `disabled` | 失效状态 | boolean | false | — |
| `indeterminate` | 设置 indeterminate 状态，只负责样式控制 | boolean | false | — |
| `onChange` | 变化时的回调函数 | (e: CheckboxChangeEvent) => void | - | — |
| `onBlur` | 失去焦点时的回调 | function() | - | — |
| `onFocus` | 获得焦点时的回调 | function() | - | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `defaultValue` | 默认选中的选项 | (string \| number)\[] | \[] | — |
| `name` | CheckboxGroup 下所有 `input[type="checkbox"]` 的 `name` 属性 | string | - | — |
| `options` | 指定可选项 | string\[] \| number\[] \| Option\[] | \[] | — |
| `value` | 指定选中的选项 | (string \| number \| boolean)\[] | \[] | — |
| `title` | 选项的 title | `string` | - | — |
| `className` | 选项的类名 | `string` | - | 5.25.0 |
| `style` | 选项的样式 | `React.CSSProperties` | - | — |
| `blur()` | 移除焦点 | — | — | — |
| `focus()` | 获取焦点 | — | — | — |
| `nativeElement` | 返回 Checkbox 的 DOM 节点 | — | — | 5.17.3 |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Checkbox** 的验收清单：

1. **配置面**：覆盖 API 表全部字段；冷门字段必须实现且命名兼容。
2. **视觉态**：default / hover / active / focus / disabled / loading。
3. **尺寸态**：small / medium / large（适用者）。
4. **受控/非受控**：value+onChange 与 defaultValue。
5. **数据驱动**：options / items / columns / treeData / fileList 等。
6. **无障碍**：焦点、角色、键盘、读屏。
7. **RTL**：placement / orientation 镜像。
8. **浮层**：z-index、挂载容器、遮挡、滚动。
9. **性能**：虚拟列表、防抖、减少重绘。
10. **主题**：Token 化；支持 reduced-motion。
11. **示例矩阵**：官方非 debug **7** 个全部 P0（§6.8）；`_semantic` 仅主题钩子，debug 4 个不验收。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/checkbox
- 中文文档：https://ant.design/components/checkbox-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/checkbox
- 驱动 gpui kit：`checkbox`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Checkbox** 补成 **可开发、可测试、可验收** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5.1** 官网截图 99.99% 相似（见 ACCEPTANCE 对齐定义）。只允字体光栅亚像素差；颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即不算对齐。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/checkbox/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Checkbox）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 点击/切换、禁用、键盘激活、受控值正确 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | showcase 大图 + 官网并排 | showcase大图进testdata按§6.8摆全多种式样（CPU容差内比对）+ 与官网同示例截图并排验收（颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即挂，只允字体光栅亚像素差） | showcase/官网并排 |
| **L4** | 官网并排必验（非可选） | 建/大改基线时与官网截图并排验收并留验收记录（见L3），CI比showcase基线，人眼签官网并排 | 建/大改基线必验 |

**明确不做（Checkbox）：**

- 与官网截图逐字节哈希一致（只要求 99.99% 相似，允字体光栅亚像素差）。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，单测Skip写清平台原因）。  
- 官方 **debug** 示例不验收（仅参考）。  

> 控件说明：收集用户的多项选择。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| Checkbox 指示器 | **16×16** | `controlInteractiveSize` / kit `TokenSizeIndicator` |
| 指示器圆角 | **4** | `borderRadiusSM`（antd `style/index.ts` 用 SM，非 `borderRadius=6`） |
| 指示器↔标签间距 | **8** | `marginXS` / kit `TokenMarginSM` |
| Group 项间距 | **8** | `marginXS`（`columnGap`） |
| 控件行高对齐 middle | **32** | `controlHeight`（表单行；指示器仍 16） |
| 控件行高对齐 small | **24** | `controlHeightSM` |
| 控件行高对齐 large | **40** | `controlHeightLG` |
| 字号 middle | **14** | `fontSize` |
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
| `disabled` | 失效状态 | boolean | false |
| `indeterminate` | 设置 indeterminate 状态，只负责样式控制 | boolean | false |
| `onChange` | 变化时的回调函数 | (e: CheckboxChangeEvent) => void | - |
| `onBlur` | 失去焦点时的回调 | function() | - |
| `onFocus` | 获得焦点时的回调 | function() | - |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |
| `defaultValue` | 默认选中的选项 | (string \ | number)\[] |
| `name` | CheckboxGroup 下所有 `input[type="checkbox"]` 的 `name` 属性 | string | - |
| `options` | 指定可选项 | string\[] \ | number\[] \ |
| `value` | 指定选中的选项 | (string \ | number \ |
| `title` | 选项的 title | `string` | - |
| `className` | 选项的类名 | `string` | - |
| `style` | 选项的样式 | `React.CSSProperties` | - |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
unchecked ── click/Space ──► checked ──► onChange(e.checked=true)
checked ── click ──► unchecked
indeterminate=true ──► 半选皮；点击后通常 checked=true 并清半选
Group：各 box 独立；value 为数组
disabled ──► 不切换
```

\*指示器 16×16（controlInteractiveSize）。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| CB-S1 | 点未选 | checked=true；onChange |
| CB-S2 | 点已选 | checked=false |
| CB-S3 | indeterminate 显示 | 半选视觉 |
| CB-S4 | indeterminate 时点击 | 进入 checked 并取消半选（对齐 antd） |
| CB-S5 | Group 选两项 | value 长度 2 |
| CB-S6 | disabled | 不切换 |
| CB-S7 | Space 聚焦 | 切换 |
| CB-S8 | 指示器尺寸 | 16×16 |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| unchecked | 白底空框 16×16 + 1px `colorBorder` + 圆角 4 |
| checked | 主色底 + 白勾（线宽 `lineWidthBold`，45° 对勾）；边框同主色 |
| indeterminate（半选） | 主色底 + 白横杠；仅样式，点击后进 checked 并清半选（CB-S4） |
| hover | 边框走 `colorPrimaryHover`；禁用无 hover |
| focus | 输入聚焦时外框可见 focus ring（`genFocusOutline`） |
| disabled（unchecked/checked/半选） | 底 `colorBgContainerDisabled` + 字 `colorTextDisabled`；勾/杠降对比；不可点 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | 单框 `checkbox`（`aria-checked` true/false/mixed）；Group 容器 `group` + 可访问名 |
| 名称 | 默认 label 文本；无 label 时必须 `AriaLabel`/`title` 回落，否则测试失败 |
| 焦点 | Tab 到隐藏 input；Focus ring 落在 16×16 框上 |
| 键盘 | Space 切换（Enter 同效）；Group 内方向键可选（P1） |
| 禁用 | `disabled` 不切换；读屏可感知禁用 |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 勾选/半选/Group/禁用主路径 | **对等** | P0 L1 |
| 尺寸/色 Token（§6.2：框 16×16/圆角 4/间距 8） | **对等** | P0 L2 |
| 全选联动（`check-all`：全选↔半选↔空） | **对等** | P0 L1 |
| Form 值桥（`valuePropName=checked`，Group `value []string`） | **对等** | P0 L1 |
| `onFocus`/`onBlur`、semantic 函数形态、`nativeElement` | P1 必做 | P1 |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 官网截图相似（99.99%） | **不做** | — |

### 6.8 能力范围（P0 / P1 全做）

#### P0（本阶段必须 1:1，与P1一次做完）

| 配置 / 能力 | 说明 |
| --- | --- |
| `checked` / `defaultChecked` | 单框受控 / 非受控 |
| `indeterminate` | 半选皮；点击后进 checked 并清半选（CB-S3/S4） |
| `onChange` | 单框 `(checked bool)`；Group `([]string)` |
| `disabled` | 单框与 Group |
| `title` | 选项 title（可作 a11y 名回落） |
| `Checkbox.Group`：`value` / `defaultValue` / `options` / `name` | 多选组；options 支持 string 或 `{label,value,disabled,title}` |
| 官方主路径示例 | 基本用法、不可用、受控的 Checkbox、Checkbox 组、全选、布局、自定义语义结构的样式和类 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | role=checkbox、名称、焦点 ring、Space/Enter |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（本阶段必须 1:1，与 P0 同标准；真做不了的单测 Skip 写清平台原因）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic `classNames`/`styles` 函数形态深度 | P1 必做；P0+P1 仅暴露 root/icon/label 节点钩子 + Style 覆盖 |
| `onFocus` / `onBlur` | P1 必做 |
| 动画像素级 / Wave | P1 必做 |
| 浏览器-only API 或桌面无等价项 | 单测 Skip 写清平台原因 |
| debug 示例 | 不验收（仅参考） |
| ConfigProvider 全局 checkbox 默认 | P1 必做 |
| `_semantic` 主题钩子（CB-17） | P1 必做 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestCheckbox_PRD_<ID>` 或 gallery 场景 ID。  
> **P0+P1 相关用例全部通过** 才可宣称 Checkbox 完成 1:1。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| CB-01 | L1 | NewCheckbox 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| CB-02 | L1 | 点未选 | checked=true；onChange |
| CB-03 | L1 | 点已选 | checked=false |
| CB-04 | L1 | indeterminate 显示 | 半选视觉 |
| CB-05 | L1 | indeterminate 时点击 | 进入 checked 并取消半选（对齐 antd） |
| CB-06 | L1 | Group 选两项 | value 长度 2 |
| CB-07 | L1 | disabled | 不切换 |
| CB-08 | L1 | Space 聚焦 | 切换 |
| CB-09 | L1 | 指示器尺寸 | 16×16 |
| CB-10 | L1 | 复现官方示例「基本用法」（`basic.tsx`） | 点空框进 checked 再点回空：`OnChange` true/false 各一次，框 16×16 主色切换 |
| CB-11 | L1 | 复现官方示例「不可用」（`disabled.tsx`） | `disabled` 单框与 Group 项均不可点；禁用 checked 仍显示主色降对比勾 |
| CB-12 | L1 | 复现官方示例「受控的 Checkbox」（`controller.tsx`） | `SetControlled(true)`：点击只发 `OnChange` 不改本地，外部 `SetChecked` 写回才变 |
| CB-13 | L1 | 复现官方示例「Checkbox 组」（`group.tsx`） | `SetOptions` 三项勾两项：`Value()` 长 2，`OnChange` 带该数组 |
| CB-14 | L1 | 复现官方示例「全选」（`check-all.tsx`） | 全选框：空→半选（mixed）→全选三态随子项走；点全选一次全勾 |
| CB-15 | L1 | 复现官方示例「布局」（`layout.tsx`） | `Add` 子框 + `SetBody(Row/Col)`：换行/栅格后勾选值链路不变 |
| CB-16 | L1 | 复现官方示例「自定义语义结构的样式和类」（`style-class.tsx`） | 语义节点钩子 + `Style` 覆盖：仅换皮，勾选/`indeterminate` 行为不变 |
| CB-17 | P1 | `_semantic.tsx` 主题钩子 | 单独用例；Notes 标明 |
| CB-18 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| CB-19 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| CB-20 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| CB-21 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| CB-22 | L3 | 关键态 golden 截图 | 与showcase基线一致（容差内）+官网并排验收通过 |
| CB-23 | L4 | 与 ant.design 并排 | 官网并排验收记录（必验） |
| CB-24 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
// ── 单框 ──────────────────────────────────────────────
NewCheckbox(label string) *Checkbox

SetChecked(bool) / SetDefaultChecked(bool)
SetControlled(bool)                 // true：点击只发 onChange，不改本地 Checked
SetIndeterminate(bool)              // 半选皮；SetChecked 会清半选
SetDisabled(bool)
SetLabel(string) / SetTitle(string) // title=选项 title
SetValue(string)                    // Group 内 option value（非 checked）
SetOnChange(func(checked bool))
SetAriaLabel(string)
SetFace / SetStyle / SetTheme
Node() / ChromeNode() / IndicatorNode() / LabelNode()

// ── 组 ────────────────────────────────────────────────
NewCheckboxGroup() *CheckboxGroup
SetOptions(...CheckboxOption)       // label/value/disabled/title
SetStringOptions(...string)         // plainOptions 糖
SetValue([]string) / SetDefaultValue([]string) / Value() []string
SetDisabled(bool) / SetName(string)
SetOnChange(func(values []string))
Add(...*Checkbox)                   // children 布局（layout 示例）
SetBody(core.Node)                  // 自定义根内容（Row/Col 布局）
Node()
```

**值类型转换规则（对齐 antd `string | number` options）：** kit 主值类型只收 `string`；`number` 型 option 由调用方转十进制字符串传入（`strconv.Itoa`，浮点用 `strconv.FormatFloat(v, 'f', -1, 64)`），`Group.Value()`/`onChange` 回传的即该字符串，调用方按需转回；`CheckboxOption{Value string}` 的 `Value` 为空时回落用 `Label`。单框自身的选中态仍走 `SetChecked(bool)`，与 `SetValue`（option 身份）互不混淆。

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Checked / Indeterminate / Disabled / Controlled | false |
| Group.Value | `[]` |
| Group.options 为空且无 children | 空组 |
| 指示器 | 16×16，圆角 4，线宽 1 |
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

同时满足即可宣布 **Checkbox 1:1 完成**：

1. §6.8 **P0+P1** 全部实现（官方非debug一个不少，真做不了的单测Skip写清平台原因）。  
2. §6.9 中 **P0+P1** 用例全部通过。
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 showcase大图+官网并排验收通过（控件可见时必需，见§6.1）。
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0+P1** 全部（官方非 debug 一个不少；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 必须进 gallery，真做不了的单测 Skip 写清平台原因。
6. `coverage.go` Notes：P0+P1 已对齐 `docs/antd/checkbox.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Checkbox 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围定义（无裁剪）。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
