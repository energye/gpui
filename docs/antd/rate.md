# Rate 评分
> 来源：[Ant Design 6.5.x Rate](https://ant.design/components/rate)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：用于对事物进行评分操作。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

用于对事物进行评分操作。

**Rate** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 尺寸 | 不同 size 档位的高宽/字号/内边距 |
| 半星 | 复现「半星」视觉与布局 |
| 文案展现 | 复现「文案展现」视觉与布局 |
| 只读 | 复现「只读」视觉与布局 |
| 清除 | 复现「清除」视觉与布局 |
| 其他字符 | 复现「其他字符」视觉与布局 |
| 自定义字符 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `allowClear`

- **说明**：是否允许再次点击后清除
- **类型**：boolean
- **默认值**：true

#### `count`

- **说明**：star 总数
- **类型**：number
- **默认值**：5

#### `disabled`

- **说明**：只读，无法进行交互
- **类型**：boolean
- **默认值**：false

#### `keyboard`

- **说明**：支持使用键盘操作
- **类型**：boolean
- **默认值**：true
- **版本**：5.18.0

#### `size`

- **说明**：星星尺寸（antd `SizeType`）
- **类型**：'small' | 'middle' | 'large'（文档偶写 medium＝middle）
- **默认值**：'middle'
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `small` | starSize **15**（controlHeightSM×0.625） |
  | `middle` | starSize **20**（默认） |
  | `large` | starSize **25**（controlHeightLG×0.625） |

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

- 至少区分根容器、内容区、装饰/图标区；浮层再分 popup/mask。

- 颜色、圆角、间距、动效走 Design Token；支持亮暗色与品牌色。

- 动效可关（reduced-motion / 全局 motion、wave 配置）。
---
## 2. 功能
### 2.1 使用场景

- 对评价进行展示。
- 对事物进行快速的评级操作。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **尺寸**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **半星**（`half.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **文案展现**（`text.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **只读**（`disabled.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **清除**（`clear.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **其他字符**（`character.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **自定义字符**（`character-function.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 当前数，受控值 |
| `defaultValue` | 非受控默认值 | 默认值 |
| `onChange` | 值变化 | 选择时的回调 |
| `disabled` | 禁用 | 只读，无法进行交互 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 尺寸 | `size.tsx` | 否 |
| 半星 | `half.tsx` | 否 |
| 文案展现 | `text.tsx` | 否 |
| 只读 | `disabled.tsx` | 否 |
| 清除 | `clear.tsx` | 否 |
| 其他字符 | `character.tsx` | 否 |
| 自定义字符 | `character-function.tsx` | 否 |
| 组件 Token | `component-token.tsx` | 是 |

### 2.5 实例方法 / Ref

#### 方法

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

| 属性 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| allowClear | 是否允许再次点击后清除 | boolean | true | allowHalf | 是否允许半选 | boolean | false | character | 自定义字符 | ReactNode \| (RateProps) => ReactNode | &lt;StarFilled /> | function(): 4.4.0 | × |
| count | star 总数 | number | 5 | defaultValue | 默认值 | number | 0 | disabled | 只读，无法进行交互 | boolean | false | keyboard | 支持使用键盘操作 | boolean | true | 5.18.0 | × |
| size | 星星尺寸 | 'small' \| 'medium' \| 'large' | 'medium' | tooltips | 自定义每项的提示信息 | [TooltipProps](/components/tooltip-cn#api)[] \| string\[] | - | value | 当前数，受控值 | number | - | onBlur | 失去焦点时的回调 | function() | - | onChange | 选择时的回调 | function(value: number) | - | onFocus | 获取焦点时的回调 | function() | - | onHoverChange | 鼠标经过时数值变化的回调 | function(value: number) | - | onKeyDown | 按键回调 | function(event) | - 
## 方法

| 名称    | 描述     |
| ------- | -------- |
| blur()  | 移除焦点 |
| focus() | 获取焦点 |

### 导入方式

```js
import { Rate } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `allowClear` | 是否允许再次点击后清除 | boolean | true | — |
| `allowHalf` | 是否允许半选 | boolean | false | — |
| `character` | 自定义字符 | ReactNode \| (RateProps) => ReactNode | <StarFilled /> | function(): 4.4.0 |
| `count` | star 总数 | number | 5 | — |
| `defaultValue` | 默认值 | number | 0 | — |
| `disabled` | 只读，无法进行交互 | boolean | false | — |
| `keyboard` | 支持使用键盘操作 | boolean | true | 5.18.0 |
| `size` | 星星尺寸 | 'small' \| 'middle' \| 'large' | 'middle' | — |
| `tooltips` | 自定义每项的提示信息 | [TooltipProps](/components/tooltip-cn#api)[] \| string\[] | - | — |
| `value` | 当前数，受控值 | number | - | — |
| `onBlur` | 失去焦点时的回调 | function() | - | — |
| `onChange` | 选择时的回调 | function(value: number) | - | — |
| `onFocus` | 获取焦点时的回调 | function() | - | — |
| `onHoverChange` | 鼠标经过时数值变化的回调 | function(value: number) | - | — |
| `onKeyDown` | 按键回调 | function(event) | - | — |
| `blur()` | 移除焦点 | — | — | — |
| `focus()` | 获取焦点 | — | — | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Rate** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **8** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/rate
- 中文文档：https://ant.design/components/rate-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/rate
- 驱动 gpui kit：`rate`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Rate** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/rate/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Rate）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 点击/切换、禁用、键盘激活、受控值正确 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Rate）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：用于对事物进行评分操作。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

数值来自 antd `components/rate/style` `prepareComponentToken`（`starSize = controlHeight * 0.625`，星间距 `marginXS`）。

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 默认 count | **5** | API |
| 星星尺寸 middle | **20** | `starSize` = `controlHeight`×0.625（32×0.625） |
| 星星尺寸 small | **15** | `starSizeSM` = `controlHeightSM`×0.625（24×0.625） |
| 星星尺寸 large | **25** | `starSizeLG` = `controlHeightLG`×0.625（40×0.625） |
| 星间距（marginInlineEnd） | **8** | antd `marginXS`（kit 回落 `DefaultRateStarGap`；勿与本库 `TokenMarginXS=4` 混用） |
| 字号 middle（字符） | **= starSize** | 字符字号跟星尺寸 |
| 圆角 | **6** | `borderRadius`（focus ring 等） |
| 边框线宽 | **1** | `lineWidth` |
| Focus ring outset | ≈ **1.5px** 可见 | 可调，必须可见 |
| 悬停缩放 | scale(1.1) | `starHoverScale`（P0 可瞬时/近似） |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 星星填充色 | `starColor` = antd **yellow6** ≈ `#FADB14` | kit：`DefaultRateStarColor`；`Style.Text` 可覆盖 |
| 星星空底 | `starBg` = `colorFillContent` ≈ `colorFillSecondary` | kit：`TokenColorFillSecondary` |
| 主色 / hover / active | `colorPrimary` + 变体 | 非 Rate 主填充；强调/其它控件 |
| 错误 / 成功 / 警告 | `colorError` / `Success` / `Warning` | status 与反馈 |
| 文本 / 次级文本 | `colorText` / `colorTextSecondary` | 文案展现旁路文字 |
| 边框 / 分割 / 容器底 | `colorBorder` / `colorSplit` / `colorBgContainer` | |
| 禁用 | `colorDisabledBg` / `colorDisabledText` | 降对比；无 hover 高亮 |
| 浮层阴影 / 遮罩 | `boxShadowSecondary` / `colorBgMask` | Tooltip 适用者 |

禁止硬编码品牌色作为唯一默认皮（`#FADB14` 仅作 antd yellow6 回落常量，须可被 Theme/Style 覆盖）。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据录入**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `allowClear` | 是否允许再次点击后清除 | boolean | true |
| `allowHalf` | 是否允许半选 | boolean | false |
| `character` | 自定义字符 | ReactNode \ | (RateProps) => ReactNode |
| `count` | star 总数 | number | 5 |
| `defaultValue` | 默认值 | number | 0 |
| `disabled` | 只读，无法进行交互 | boolean | false |
| `keyboard` | 支持使用键盘操作 | boolean | true |
| `size` | 星星尺寸 | `'small'` \| `'middle'` \| `'large'`（文档偶写 medium＝middle） | middle |
| `tooltips` | 自定义每项的提示信息 | `string[]`（完整 TooltipProps 形态 P1） | - |
| `value` | 当前数，受控值 | number（半星为 x.5） | - |
| `onBlur` | 失去焦点时的回调 | function() | - |
| `onChange` | 选择时的回调 | function(value: number) | - |
| `onFocus` | 获取焦点时的回调 | function() | - |
| `onHoverChange` | 鼠标经过时数值变化的回调 | function(value: number) | - |
| `onKeyDown` | 按键回调 | function(event) | - |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
hover 预览 ──► 临时高亮
click 第 n 星 ──► value=n + onChange
allowHalf ──► 半星区 value=n-0.5
allowClear 再点当前值 ──► 0
disabled ──► 不改
```

\*默认 count=5。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| RAT-S1 | 点第 3 星 | value=3 |
| RAT-S2 | allowHalf 点半区 | x.5 |
| RAT-S3 | allowClear 再点 | 0 |
| RAT-S4 | disabled | 不改 |
| RAT-S5 | count=10 | 10 星 |
| RAT-S6 | 键盘（适用） | 可调 |
| RAT-S7 | tooltips | 悬停文案 |
| RAT-S8 | 受控 value | 外部优先 |
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
| `value` / `defaultValue` / 受控 | float64；半星 x.5；Controlled 时 click 只派发 onChange |
| `onChange` | 选择时回调（含 allowClear→0） |
| `onHoverChange` | 悬停预览数值变化（离开时 0） |
| `disabled` | 只读，不改值、无 hover 预览 |
| `size` | small \| middle \| large → starSize 15/20/25 |
| `count` | star 总数，默认 5 |
| `allowClear` | 默认 true；再点当前值 → 0 |
| `allowHalf` | 半星命中与绘制 |
| `character` / `character(index)` | 自定义字符或按 index 渲染（字符串；图标节点 P1） |
| `tooltips` | 每星悬停文案（string[]；完整 TooltipProps P1） |
| `keyboard` | 默认 true；方向键调值 |
| 官方主路径示例 | 基本、尺寸、半星、文案展现、只读、清除、其他字符、自定义字符 |
| 度量 §6.2 | Token / Default 断言 |
| a11y §6.6 | role + 焦点 ring + 键盘 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 深度 | 分期 |
| `character` 为复杂 ReactNode / 图标节点 | 分期（P0 仅 string / index→string） |
| `tooltips` 完整 TooltipProps（placement 等） | 分期（P0 仅 string[]） |
| 悬停 scale(1.1) 像素级 | 分期（P0 可用瞬时高亮） |
| 动画像素级 / 复杂虚拟列表 | 分期 |
| 浏览器-only API 或桌面无等价项 | 分期 |
| ConfigProvider 全局默认 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestRate_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Rate 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| RAT-01 | L1 | NewRate 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| RAT-02 | L1 | 点第 3 星 | value=3 |
| RAT-03 | L1 | allowHalf 点半区 | x.5 |
| RAT-04 | L1 | allowClear 再点 | 0 |
| RAT-05 | L1 | disabled | 不改 |
| RAT-06 | L1 | count=10 | 10 星 |
| RAT-07 | L1 | 键盘（适用） | 可调 |
| RAT-08 | L1 | tooltips | 悬停文案 |
| RAT-09 | L1 | 受控 value | 外部优先 |
| RAT-10 | L1 | 复现官方示例「基本」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RAT-11 | L1 | 复现官方示例「尺寸」（`size.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RAT-12 | L1 | 复现官方示例「半星」（`half.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RAT-13 | L1 | 复现官方示例「文案展现」（`text.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RAT-14 | L1 | 复现官方示例「只读」（`disabled.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RAT-15 | L1 | 复现官方示例「清除」（`clear.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RAT-16 | L1 | 复现官方示例「其他字符」（`character.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RAT-17 | L1 | 复现官方示例「自定义字符」（`character-function.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RAT-18 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| RAT-19 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| RAT-20 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| RAT-21 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| RAT-22 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| RAT-23 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| RAT-24 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewRate() *Rate

// 值（float64，半星 x.5）
SetValue(v) / Value()          // 写入当前值；不派发 OnChange
SetDefaultValue(v)             // 非受控初始
SetControlled(bool)
// 配置（§6.3 / §3 P0）
SetCount(n)                    // 默认 5
SetAllowClear(bool)            // 默认 true
SetAllowHalf(bool)             // 默认 false
SetSize(RateSize)              // RateSmall|RateMiddle|RateLarge
SetCharacter(string)           // 默认 "★"
SetCharacterAt(func(index int) string)  // 0-based；优先于 Character
SetTooltips([]string)
SetKeyboard(bool)              // 默认 true
SetDisabled(bool)
// 回调
SetOnChange(func(float64))
SetOnHoverChange(func(float64))
SetOnFocus / SetOnBlur / SetOnKeyDown
// 主题 / a11y / 挂树
SetTheme(*Theme)；Style 可选覆盖（Style.Text → starColor）
SetAriaLabel / SetFace
Node() core.Node               // Root 身份稳定
HandleKey(*KeyEvent) bool      // 方向键（keyboard=true）
StarNodes() []core.Node        // 测试/几何
StarSize() float64             // 当前档 starSize
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Value | 0 |
| Count | 5 |
| AllowClear | true |
| AllowHalf | false |
| Disabled | false |
| Keyboard | true |
| Size | middle（starSize **20**） |
| Character | `"★"` |
| 受控值 | 未 SetControlled 时本地提交；Controlled 时 click 只 OnChange |
| 其余 | 对齐 antd 6.5 §3 表 |

### 6.11 结构与绘制分层（实现提示）

```text
Flex Row (Root, role=radiogroup, gap=starGap)
  └─ rateStar × count   (role=radio, focusable)
       └─ paint ★ / character（full | half-clip | empty）
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 半星：同盒 left 50% 命中 = n-0.5，right 50% = n；绘制 left clip 填色。  
- `rebuild()` 只在 count/size/character/tooltips 结构变化时重建星节点；`SetValue`/hover 只 `applyChrome`。  
- 命中区域与布局盒一致（`hit == layout == paint`）；Root 身份跨 `SetValue` 稳定。  
- Rate 无 loading API；无需 Ticker（P0）。  
- 动画（hover scale）P1；P0 瞬时高亮即可。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Rate 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/rate.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Rate 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
