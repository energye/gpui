# Switch 开关
> 来源：[Ant Design 6.5.x Switch](https://ant.design/components/switch)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：使用开关切换两种状态之间。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

使用开关切换两种状态之间。

**Switch** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 不可用 | disabled 态 |
| 文字和图标 | icon 与文本混排 |
| 两种大小 | 不同 size 档位 |
| 加载中 | loading 指示与防重复 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `checked`

- **说明**：指定当前是否选中
- **类型**：boolean
- **默认值**：false

#### `checkedChildren`

- **说明**：选中时的内容
- **类型**：ReactNode
- **默认值**：-

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `disabled`

- **说明**：是否禁用
- **类型**：boolean
- **默认值**：false

#### `loading`

- **说明**：加载中的开关
- **类型**：boolean
- **默认值**：false

#### `size`

- **说明**：开关大小，可选值：`medium` `small`
- **类型**：`'medium'` | `'small'`
- **默认值**：`medium`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `unCheckedChildren`

- **说明**：非选中时的内容
- **类型**：ReactNode
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

- 需要表示开关状态/两种状态之间的切换时；
- 和 `checkbox` 的区别是，切换 `switch` 会直接触发状态改变，而 `checkbox` 一般用于状态标记，需要和提交操作配合。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **不可用**（`disabled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **文字和图标**（`text.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **两种大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **加载中**（`loading.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | `checked` 的别名 |
| `defaultValue` | 非受控默认值 | `defaultChecked` 的别名 |
| `onChange` | 值变化 | 变化时的回调函数 |
| `onClick` | 点击 | 点击时的回调函数 |
| `disabled` | 禁用 | 是否禁用 |
| `loading` | 加载中 | 加载中的开关 |
| `checked` | 选中布尔 | 指定当前是否选中 |
| `defaultChecked` | 默认选中 | 初始是否选中 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 不可用 | `disabled.tsx` | 否 |
| 文字和图标 | `text.tsx` | 否 |
| 两种大小 | `size.tsx` | 否 |
| 加载中 | `loading.tsx` | 否 |
| 自定义组件 Token | `component-token.tsx` | 是 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |

### 2.5 实例方法 / Ref

#### 方法

| 名称    | 描述     |
| ------- | -------- |
| blur()  | 移除焦点 |
| focus() | 获取焦点 |

### 2.6 FAQ

## FAQ

### 为什么在 Form.Item 下不能绑定数据？ {#faq-binding-data}

Form.Item 默认绑定值属性到 `value` 上，而 Switch 的值属性为 `checked`。你可以通过 `valuePropName` 来修改绑定的值属性。

```tsx | pure

  

```

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
| checked | 指定当前是否选中 | boolean | false | checkedChildren | 选中时的内容 | ReactNode | - | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | defaultChecked | 初始是否选中 | boolean | false | defaultValue | `defaultChecked` 的别名 | boolean | - | 5.12.0 | × |
| disabled | 是否禁用 | boolean | false | loading | 加载中的开关 | boolean | false | size | 开关大小，可选值：`medium` `small` | `'medium'` \| `'small'` | `medium` | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | unCheckedChildren | 非选中时的内容 | ReactNode | - | value | `checked` 的别名 | boolean | - | 5.12.0 | × |
| onChange | 变化时的回调函数 | function(checked: boolean, event: Event) | - | onClick | 点击时的回调函数 | function(checked: boolean, event: Event) | - 
## 方法

| 名称    | 描述     |
| ------- | -------- |
| blur()  | 移除焦点 |
| focus() | 获取焦点 |

### 导入方式

```js
import { Switch } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `checked` | 指定当前是否选中 | boolean | false | — |
| `checkedChildren` | 选中时的内容 | ReactNode | - | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `defaultChecked` | 初始是否选中 | boolean | false | — |
| `defaultValue` | `defaultChecked` 的别名 | boolean | - | 5.12.0 |
| `disabled` | 是否禁用 | boolean | false | — |
| `loading` | 加载中的开关 | boolean | false | — |
| `size` | 开关大小，可选值：`medium` `small` | `'medium'` \| `'small'` | `medium` | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `unCheckedChildren` | 非选中时的内容 | ReactNode | - | — |
| `value` | `checked` 的别名 | boolean | - | 5.12.0 |
| `onChange` | 变化时的回调函数 | function(checked: boolean, event: Event) | - | — |
| `onClick` | 点击时的回调函数 | function(checked: boolean, event: Event) | - | — |
| `blur()` | 移除焦点 | — | — | — |
| `focus()` | 获取焦点 | — | — | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Switch** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **6** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/switch
- 中文文档：https://ant.design/components/switch-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/switch
- 驱动 gpui kit：`switch`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Switch** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/switch/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Switch）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 点击/切换、禁用、键盘激活、受控值正确 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Switch）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：使用开关切换两种状态之间。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| Switch 轨高 trackHeight | **≈22** | fontSize×lineHeight |
| Switch 轨最小宽 | **≈44** | handle×2+padding×4 |
| Switch small 轨高 | **16** | controlHeight/2 |
| 默认轨 | **≈44×22** | prepareComponentToken 公式 |
| small 轨 | **≈28×16** | SM 公式 |
| 轨内边距 trackPadding | **2** | prepareComponentToken 固定值 |
| handle 直径 default | **≈18** | trackHeight − 2×padding |
| handle 直径 small | **≈12** | trackHeightSM − 2×padding |
| 字号（内文） | **12**（小）/ 跟随内容 | 内文非按钮 controlHeight |
| 圆角（轨） | **轨高/2**（胶囊） | 非 `borderRadius` 矩形 |
| Focus ring outset | ≈ **1.5px** 可见 | 可调，必须可见；形状跟胶囊 |

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
| `checkedChildren` | 选中时的内容 | ReactNode | - |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `defaultChecked` | 初始是否选中 | boolean | false |
| `defaultValue` | `defaultChecked` 的别名 | boolean | - |
| `disabled` | 是否禁用 | boolean | false |
| `loading` | 加载中的开关 | boolean | false |
| `size` | 开关大小，可选值：`medium` `small` | `'medium'` \ | `'small'` |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |
| `unCheckedChildren` | 非选中时的内容 | ReactNode | - |
| `value` | `checked` 的别名 | boolean | - |
| `onChange` | 变化时的回调函数 | function(checked: boolean, event: Event) | - |
| `onClick` | 点击时的回调函数 | function(checked: boolean, event: Event) | - |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
mount ──► unchecked | checked（受控/非受控）
             │
             ├── click / Space（聚焦）──► 若 !disabled && !loading
             │                              └── value' = !value ──► onChange(value', event)
             ├── loading=true ──► 不切换；handle 内 spinner；可保持 checked 外观
             ├── disabled=true ──► 不切换；禁用皮
             ├── 受控 checked ──► 仅外部改值才变；内部 click 只抛 onChange
             └── size=small ──► 轨几何 SM（约 28×16）
```

\*antd：`value` 为 `checked` 别名；loading 期间不得完成切换。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| SW-S1 | 未选中时点击 | `onChange(true)` 一次；外观变为开 |
| SW-S2 | 选中时点击 | `onChange(false)` 一次 |
| SW-S3 | `disabled=true` 点击/Space | 不触发 `onChange` |
| SW-S4 | `loading=true` 点击 | 不触发 `onChange`；有 spinner |
| SW-S5 | 受控 `checked=false` 时点击 | 仍显示关，直到父级改 props；回调仍抛出 |
| SW-S6 | 聚焦 + Space / Enter | 切换（同 click） |
| SW-S7 | `size=small` | 轨高≈16、最小宽≈28 |
| SW-S8 | 默认尺寸 | 轨高≈22、最小宽≈44（公式见 §6.2） |
| SW-S9 | `checkedChildren`/`unCheckedChildren` | 开/关文案随状态显示 |
| SW-S10 | 主题主色 | 开态轨道 = `colorPrimary` |
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
| `checked` / `value`（别名） | 受控当前值；`SetChecked` / `SetValue` |
| `defaultChecked` / `defaultValue`（别名） | 非受控初值 |
| 受控模式 | `SetControlled(true)`：点击只抛回调，外观等父级改值 |
| `onChange` | 必须 |
| `onClick` | 必须（与 onChange 同新值；禁用/加载时不触发） |
| `disabled` | 必须 |
| `loading` | 必须（handle 内 spinner；期间不切换） |
| `size` | `medium`（默认）/ `small` |
| `checkedChildren` / `unCheckedChildren` | 字符串内文（图标名可走文案占位） |
| 官方主路径示例 | 基本、不可用、文字和图标、两种大小、加载中 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | 最低要求（role=switch；焦点环；Space/Enter） |
| §6.9 中 L1/L2 且非 P1 的用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 深度 | 分期（含 style-class / _semantic 示例） |
| 复杂 ReactNode 内文（多节点 Flex 图标） | 分期；P0 仅字符串 |
| 动画像素级 handle 拉伸 / Wave | 分期（P0 可用瞬时或 FloatAnim 滑块） |
| 浏览器-only API 或桌面无等价项 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestSwitch_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Switch 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| SW-01 | L1 | NewSwitch 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| SW-02 | L1 | 未选中时点击 | `onChange(true)` 一次；外观变为开 |
| SW-03 | L1 | 选中时点击 | `onChange(false)` 一次 |
| SW-04 | L1 | `disabled=true` 点击/Space | 不触发 `onChange` |
| SW-05 | L1 | `loading=true` 点击 | 不触发 `onChange`；有 spinner |
| SW-06 | L1 | 受控 `checked=false` 时点击 | 仍显示关，直到父级改 props；回调仍抛出 |
| SW-07 | L1 | 聚焦 + Space | 切换（同 click） |
| SW-08 | L1 | `size=small` | 轨高≈16、最小宽≈28 |
| SW-09 | L1 | 默认尺寸 | 轨高≈22、最小宽≈44（公式见 §6.2） |
| SW-10 | L1 | `checkedChildren`/`unCheckedChildren` | 开/关文案随状态显示 |
| SW-11 | L1 | 主题主色 | 开态轨道 = `colorPrimary` |
| SW-12 | L1 | 复现官方示例「基本」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SW-13 | L1 | 复现官方示例「不可用」（`disabled.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SW-14 | L1 | 复现官方示例「文字和图标」（`text.tsx`，字符串内文） | 交互与主视觉符合文档；无控制台级错误 |
| SW-15 | L1 | 复现官方示例「两种大小」（`size.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SW-16 | L1 | 复现官方示例「加载中」（`loading.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SW-17 | P1 | 复现官方示例「自定义语义结构的样式和类」（`style-class.tsx`） | semantic 深度分期 |
| SW-18 | P1 | 复现官方示例「_semantic.tsx」 | semantic 深度分期 |
| SW-19 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| SW-20 | L2 | 默认皮颜色 | 开态 = `colorPrimary`；关态中性填充（非品牌硬编码） |
| SW-21 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| SW-22 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；Space/Enter 激活 |
| SW-23 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| SW-24 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| SW-25 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewSwitch() *Switch

// 值
SetChecked(bool) / Checked          // 当前值（受控时父级写入）
SetValue(bool)                      // checked 别名；标记受控
SetDefaultChecked(bool)             // 非受控初值（defaultChecked）
SetDefaultValue(bool)               // defaultChecked 别名
SetControlled(bool)                 // true：点击只抛 OnChange/OnClick，不翻转本地 Checked
// 配置
SetSize(SwitchSize)                 // medium | small
SetDisabled(bool) / SetLoading(bool)
SetCheckedChildren(string) / SetUnCheckedChildren(string)
// 回调
SetOnChange(func(bool)) / SetOnClick(func(bool))
// 主题 / 样式
Theme *Theme；SetStyle(Style)；SetBackground(关态) / SetActiveColor(开态)
// a11y
SetAriaLabel(string)；role=switch；Focusable；Space/Enter
// 挂树 / 动画
Node() core.Node；ChromeNode() / IndicatorNode()
AttachTicker(*Tree)；Tick(dt)        // thumb 滑动 + loading spinner
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Checked | false |
| Controlled | false（非受控，点击翻转） |
| Disabled / Loading | false |
| Size | medium（轨 ≈44×22） |
| CheckedChildren / UnCheckedChildren | 空 |
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

同时满足即可宣布 **Switch 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/switch.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Switch 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
