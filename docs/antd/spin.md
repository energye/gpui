# Spin 加载中
> 来源：[Ant Design 6.5.x Spin](https://ant.design/components/spin)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：反馈（Feedback）  
> 说明：用于页面和区块的加载中状态。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

用于页面和区块的加载中状态。

**Spin** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本用法 | 复现「基本用法」视觉与布局 |
| 各种大小 | 不同 size 档位 |
| 卡片加载中 | loading 指示与防重复 |
| 自定义描述文案 | 自定义渲染/插槽外观 |
| 延迟 | 复现「延迟」视觉与布局 |
| 自定义指示符 | 自定义渲染/插槽外观 |
| 进度 | 进度条/圈 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |
| 全屏 | 复现「全屏」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `delay`

- **说明**：延迟显示加载效果的时间（防止闪烁）
- **类型**：number (毫秒)
- **默认值**：-

#### `description`

- **说明**：可以自定义描述文案
- **类型**：ReactNode
- **默认值**：-
- **版本**：6.3.0

#### `fullscreen`

- **说明**：显示带有 `Spin` 组件的背景
- **类型**：boolean
- **默认值**：false
- **版本**：5.11.0

#### `indicator`

- **说明**：加载指示符
- **类型**：ReactNode
- **默认值**：-

#### `percent`

- **说明**：展示进度，当设置 `percent="auto"` 时会预估一个永远不会停止的进度
- **类型**：number | 'auto'
- **默认值**：-
- **版本**：5.18.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `auto` | 官方取值 `auto` |

#### `size`

- **说明**：组件大小，可选值为 `small` `medium` `large`
- **类型**：string
- **默认值**：`medium`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `small` | 小尺寸（更紧凑） |
  | `medium` | 中尺寸（默认节奏） |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |

#### `spinning`

- **说明**：是否为加载中状态
- **类型**：boolean
- **默认值**：true

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `tip`

- **说明**：当作为包裹元素时，可以自定义描述文案。已废弃，请使用 `description`
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

页面局部处于等待异步数据或正在渲染过程时，合适的加载动效会有效缓解用户的焦虑。

### 2.2 核心功能（按官方示例拆解）

1. **基本用法**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **各种大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **卡片加载中**（`nested.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **自定义描述文案**（`tip.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **延迟**（`delayAndDebounce.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **自定义指示符**（`custom-indicator.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **进度**（`percent.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **全屏**（`fullscreen.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `percent` | 进度值 | 展示进度，当设置 `percent="auto"` 时会预估一个永远不会停止的进度 |
| `spinning` | 是否旋转 | 是否为加载中状态 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本用法 | `basic.tsx` | 否 |
| 各种大小 | `size.tsx` | 否 |
| 卡片加载中 | `nested.tsx` | 否 |
| 自定义描述文案 | `tip.tsx` | 否 |
| 延迟 | `delayAndDebounce.tsx` | 否 |
| 自定义指示符 | `custom-indicator.tsx` | 否 |
| 进度 | `percent.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 全屏 | `fullscreen.tsx` | 否 |

### 2.5 实例方法 / Ref

#### 方法

### 静态方法

- `Spin.setDefaultIndicator(indicator: ReactNode)`

  你可以自定义全局默认 Spin 的元素。

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
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> | - | delay | 延迟显示加载效果的时间（防止闪烁） | number (毫秒) | - | description | 可以自定义描述文案 | ReactNode | - | 6.3.0 | × |
| fullscreen | 显示带有 `Spin` 组件的背景 | boolean | false | 5.11.0 | × |
| indicator | 加载指示符 | ReactNode | - | percent | 展示进度，当设置 `percent="auto"` 时会预估一个永远不会停止的进度 | number \| 'auto' | - | 5.18.0 | × |
| size | 组件大小，可选值为 `small` `medium` `large` | string | `medium` | spinning | 是否为加载中状态 | boolean | true | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | ~~tip~~ | 当作为包裹元素时，可以自定义描述文案。已废弃，请使用 `description` | ReactNode | - | ~~wrapperClassName~~ | 包装器的类属性。已废弃，请使用 `classNames.root` | string | - 
### 静态方法

- `Spin.setDefaultIndicator(indicator: ReactNode)`

  你可以自定义全局默认 Spin 的元素。

### 导入方式

```js
import { Spin } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `delay` | 延迟显示加载效果的时间（防止闪烁） | number (毫秒) | - | — |
| `description` | 可以自定义描述文案 | ReactNode | - | 6.3.0 |
| `fullscreen` | 显示带有 `Spin` 组件的背景 | boolean | false | 5.11.0 |
| `indicator` | 加载指示符 | ReactNode | - | — |
| `percent` | 展示进度，当设置 `percent="auto"` 时会预估一个永远不会停止的进度 | number \| 'auto' | - | 5.18.0 |
| `size` | 组件大小，可选值为 `small` `medium` `large` | string | `medium` | — |
| `spinning` | 是否为加载中状态 | boolean | true | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `tip` | 当作为包裹元素时，可以自定义描述文案。已废弃，请使用 `description` | ReactNode | - | — |
| `wrapperClassName` | 包装器的类属性。已废弃，请使用 `classNames.root` | string | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Spin** 的验收清单：

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
- 官方文档：https://ant.design/components/spin
- 中文文档：https://ant.design/components/spin-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/spin
- 驱动 gpui kit：`spin`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Spin** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/spin/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Spin）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 展示/自动关闭/堆叠/类型语义 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Spin）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：用于页面和区块的加载中状态。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`controlHeightLG=40`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。  
源码：`components/spin/style/index.ts` → `prepareComponentToken` + `genIndicatorStyle`。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| **dotSize**（medium） | **20** | `controlHeightLG/2`；本库 `TokenSpinSize` |
| **dotSizeSM**（small） | **14** | `controlHeightLG * 0.35` |
| **dotSizeLG**（large） | **32** | `controlHeight` |
| **contentHeight** | **400** | 组件 token（嵌套空态最小高参考） |
| section 指示↔文案 gap | **12** | `paddingSM` / `TokenPaddingSM` |
| description 字号 | **14** | `fontSize` / `TokenFontSize` |
| 4-dot 单项边长 | `(dotSize − marginXXS/2) / 2` | antd CSS var `dot-item-size` |
| 进度环 viewBox | 100；stroke = 20；r = 40 | `Indicator/Progress.tsx` |
| 旋转周期 | **1.2s** / 转（≈0.833 rps） | `antRotate` 1.2s linear |
| percent=auto 步进 | **200ms** 桶 [30→5%, 70→3%, 96→1%] | `usePercent.ts` |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 指示符 / description | `colorPrimary` | section 色；禁止硬编码品牌蓝 |
| 进度环底轨 | `colorFillSecondary` | percent 模式 |
| 嵌套 children 遮罩 | `colorBgContainer` @ ≈0.4 | spinning 时盖在 container 上 |
| 嵌套 children 视觉降对比 | container opacity ≈ **0.5** | P0 可用遮罩近似 |
| fullscreen 遮罩（P1） | `colorBgMask` | 视口 fixed |
| fullscreen 文案（P1） | `colorTextLightSolid` / 白 | |

禁止硬编码品牌色作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**反馈**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `spinning` | 是否为加载中状态（props）；`delay>0` 时显示态可滞后 | bool | **true** |
| `size` | `small` / `medium`（`default` 兼容映射到 medium）/ `large` | string | **medium** |
| `percent` | 进度环；`auto` 为永不停止的模拟进度；未设则 4-dot | number \| `auto` | 未设 |
| `delay` | 延迟显示加载效果（防闪烁），毫秒 | number (ms) | **0** |
| `description` | 描述文案（与指示同显） | string / Node | 空 |
| `tip` | **已废弃**，等价 `description` | string | 空 |
| `indicator` | 自定义指示符；优先于全局默认与内置 4-dot | Node | 空 |
| `fullscreen` | 视口级遮罩（**P1**） | bool | false |
| `classNames` / `styles` | 语义钩子 root/section/indicator/description/container | 浅层 Record | 空 |
| `children` / content | 嵌套内容；有 content 时为 nested 模式 | Node | 空 |

**配置优先级：** 实例 `indicator` > `SetDefaultIndicator` 全局 > 内置 Looper（4-dot / percent 环）。  
`description` > 废弃 `tip`。`size="default"` → `medium`。

### 6.4 交互状态机（L1）

```text
props.spinning=false ──► display=false；仅 children（可点）；无指示
props.spinning=true && delay==0 ──► display=true 立即
props.spinning=true && delay>0 ──► display 保持 false，经 delay ms 后 true（Ticker）
display=true 无 children ──► 仅 section（indicator [+ description]）
display=true 有 children ──► container(children) + 居中 section + 遮罩挡点击
percent 未设 ──► 4-dot 旋转（或自定义 indicator）
percent 数值|>0 ──► 进度环（4-dot 隐藏）；percent=auto ──► 模拟进度爬升
reduced-motion ──► 停转 / 停 auto 步进，指示仍可见
fullscreen（P1）──► 视口 mask + section
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| SPN-S1 | spinning=true（delay=0） | 可见指示（4-dot / percent / indicator） |
| SPN-S2 | spinning=false | 无指示；children 可点 |
| SPN-S3 | description（或 tip 别名）非空 | 与指示同显 |
| SPN-S4 | fullscreen | **P1** 全屏遮罩 |
| SPN-S5 | delay=500 + spinning↑ | 500ms 内 display 仍 false（不闪） |
| SPN-S6 | 嵌套 children | children 仍在树中（spinning 时亦然） |
| SPN-S7 | reduced-motion | Tick 返回 false 或相位不再推进 |
| SPN-S8 | percent 数值 / auto | 进度环；auto 随 Tick 爬升且 <100 |
| SPN-S9 | 自定义 indicator | 使用实例/全局 indicator，非内置 4-dot |
| SPN-S10 | size small/medium/large | DotSize 14 / 20 / 32（±0.5） |

### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default / spinning | 指示色 `colorPrimary`；几何 §6.2 |
| nested spinning | children 降对比 + 遮罩挡点；section 居中 |
| percent | 环 fill=`colorPrimary`，轨=`colorFillSecondary` |
| reduced-motion | 静止指示，无旋转相位 |
| 主题切换 | 色与尺寸随 Theme / Token 更新 |

**动效：** 4-dot 旋转与 percent=auto 由 **Tree Ticker** 驱动；须尊重 reduced-motion。P0 不要求与 CSS keyframes 逐帧一致。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 / 实时区 | root `role=status`，`aria-live=polite`，`aria-busy=displaySpinning` |
| 可访问名 | `description` / `AriaLabel` / 默认 `"Loading"` |
| 不抢焦点 | Spin 本身不抢焦点 |
| 进度 | percent 模式暴露 progressbar 语义（valuemin/max/now） |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 主路径行为（§6.1 L1） | **对等** | P0 L1 |
| 尺寸/色 Token（§6.2） | **对等** | P0 L2 |
| 4-dot / percent 动画 | **近似**（Ticker 相位） | P0 行为 / P1 像素 |
| fullscreen 视口 fixed | Portal / 视口 mask | **P1** |
| Semantic classNames/styles 深度（函数式） | 浅层字段 P0；函数/深度 P1 | P0 浅 / P1 深 |
| ConfigProvider `spin` 全局 | 随 ConfigProvider | P1 |
| 逐像素官网哈希 | **不做** | — |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `spinning` + 显示态 | delay=0 立即；含 `IsDisplaySpinning` |
| `size` small/medium/large | DotSize 14/20/32 |
| `percent` number \| auto | 进度环 + auto 爬升 |
| `delay` | 毫秒防闪烁（Ticker） |
| `description` + 废弃 `tip` 别名 | 与指示同显 |
| `indicator` + `SetDefaultIndicator` | 自定义 / 全局默认 |
| nested `content` | children 在树；spinning 时挡点 |
| 浅层 `classNames` / `styles` | 够 style-class 示例 |
| 官方主路径示例 | basic / size / nested / tip / delay / custom-indicator / percent / style-class |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | status + busy + label |
| §6.9 中 **无 P1/N/A/L3/L4 标记** 的 L1/L2 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| `fullscreen` | 视口级 mask + 示例 |
| semantic classNames/styles 函数式/深度 | 分期 |
| 动画像素级（4-dot keyframes / 405°） | 分期 |
| ConfigProvider 全局 spin | 分期 |
| debug / `_semantic.tsx` / 官网逐像素 | 分期 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestSpin_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 / N/A / L3 / L4 标记）全部通过** 才可宣称 Spin 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| SPN-01 | L1 | NewSpin 默认创建 | spinning=true、size=medium、delay=0；Node 非空；RepaintBoundary |
| SPN-02 | L1 | spinning=true | IsDisplaySpinning；有指示节点；Tick 推进 |
| SPN-03 | L1 | spinning=false + children | 无指示；children 可命中 |
| SPN-04 | L1 | SetDescription / SetTip | 文案节点与指示同显 |
| SPN-05 | L1 **P1** | fullscreen | 全屏遮罩（本阶段不测） |
| SPN-06 | L1 | delay=500 后 SetSpinning(true) | 累时 <500ms 时 display=false；≥500ms 后 true |
| SPN-07 | L1 | 嵌套 children | spinning 时 children 仍在树中 |
| SPN-08 | L1 | reduced-motion | Tick 不再推进旋转 / 返回 false |
| SPN-09 | L1 | 官方 basic | 独立 Spin 可布局 |
| SPN-10 | L1 | 官方 size | small/medium/large DotSize 14/20/32 |
| SPN-11 | L1 | 官方 nested | 切换 spinning 指示显隐 |
| SPN-12 | L1 | 官方 tip/description | 三档 size + description |
| SPN-13 | L1 | 官方 delay | 同 SPN-06 路径可复现 |
| SPN-14 | L1 | 官方 custom-indicator | Indicator 非空且优先于 4-dot |
| SPN-15 | L1 | 官方 percent | SetPercent / SetPercentAuto；EffectivePercent 合理 |
| SPN-16 | L1 | 官方 style-class | 浅层 ClassNames/Styles 可设且不崩 |
| SPN-17 | L2 | §6.2 尺寸 | DotSize medium=20、gap=12、font=14（±0.5） |
| SPN-18 | L2 | 默认皮颜色 | 指示色走 `colorPrimary` Token |
| SPN-19 | L2 **N/A** | disabled | Spin 无 disabled API |
| SPN-20 | L1 **N/A** | 键盘/焦点 | Spin 非焦点控件 |
| SPN-21 | L3 | golden | visualtest 基线 |
| SPN-22 | L4 | 人眼 | 并排签字 |
| SPN-23 | **P1** | fullscreen 等 | Notes 标明 |

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API（旧 `Size float64` 像素字段删除）。语义对齐 antd 6.5。

```text
NewSpin(content core.Node) *Spin   // content 可 nil；默认 spinning=true, size=medium
Node() core.Node
AttachTicker(*core.Tree) / Tick(dt) bool   // delay + 旋转 + percent=auto

SetContent(core.Node)
SetSpinning(bool)                 // props；触发 delay 状态机
SetDelay(ms float64)              // 毫秒；0=立即
SetSize(SpinSize)                 // small|medium|large（"default"→medium）
SetDescription(string)            // 主文案
SetTip(string)                    // 废弃别名 → Description
SetPercent(float64)               // 数值进度；清除 auto
SetPercentAuto()                  // percent="auto"
ClearPercent()                    // 回到 4-dot
SetIndicator(core.Node)           // 实例自定义指示符
SetDefaultIndicator(core.Node)    // 包级全局默认（antd Spin.setDefaultIndicator）
SetTheme(*Theme) / SetStyle(Style)
SetClassNames(SpinClassNames) / SetStyles(SpinStyles)  // 浅层
SetAriaLabel(string)

IsDisplaySpinning() bool          // 经过 delay 后的显示态
EffectivePercent() float64        // auto 模拟值或数值；未设 → 0 且 HasPercent()=false
HasPercent() bool
DotSize() float64                 // 当前 size 对应几何
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Spinning | **true** |
| Size | **medium**（DotSize=20） |
| Delay | **0** |
| Description / Tip | 空 |
| Percent | 未设（4-dot） |
| Fullscreen | false（P1） |
| Indicator | 空 → 全局默认 → 内置 |

### 6.11 结构与绘制分层（实现提示）

```text
spinHost (RepaintBoundary, role=status, aria-live=polite, aria-busy)
  ├─ simple（无 content）
  │    └─ section (column: indicator + description?)   // display 时
  └─ nested（有 content）
       stack
         ├─ container (children)                         // 始终在树
         ├─ mask (HitBlock, colorBgContainer@0.4)        // 仅 display
         └─ section Positioned(center)                   // 仅 display
              column(indicator, description?)
```

- 指示：自定义 Node **或** Canvas 绘 4-dot / percent 环；相位仅 `MarkNeedsPaint`，不整树 rebuild。  
- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读 Default / 字段 / Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）；nested spinning 时 mask `HitBlock` 挡 children。  
- 动画跟随 Host **Ticker**；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Spin 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/spin.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Spin 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
