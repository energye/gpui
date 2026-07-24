# Anchor 锚点
> 来源：[Ant Design 6.5.x Anchor](https://ant.design/components/anchor)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：导航（Navigation）  
> 说明：用于跳转到页面指定位置。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

用于跳转到页面指定位置。

**Anchor** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 横向 Anchor | 复现「横向 Anchor」视觉与布局 |
| 静态位置 | placement 方位 |
| 自定义 onClick 事件 | 自定义渲染/插槽外观 |
| 自定义锚点高亮 | 自定义渲染/插槽外观 |
| 设置锚点滚动偏移量 | 复现「设置锚点滚动偏移量」视觉与布局 |
| 监听锚点链接改变 | 复现「监听锚点链接改变」视觉与布局 |
| 替换历史中的 href | 复现「替换历史中的 href」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `direction`

- **说明**：设置导航方向
- **类型**：`vertical` | `horizontal`
- **默认值**：`vertical`
- **版本**：5.2.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `vertical` | 垂直排布 |
  | `horizontal` | 水平排布 |

#### `title`

- **说明**：文字内容
- **类型**：ReactNode
- **默认值**：-

#### `children`

- **说明**：嵌套的 Anchor Link，`注意：水平方向该属性不支持`
- **类型**：[AnchorItem](#anchoritem)\[]
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

需要展现当前页面上可供跳转的锚点链接，以及快速在锚点之间跳转。

> 开发者注意事项：
>
> 自 `4.24.0` 起，由于组件从 class 重构成 FC，之前一些获取 `ref` 并调用内部实例方法的写法都会失效

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **横向 Anchor**（`horizontal.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **静态位置**（`static.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **自定义 onClick 事件**（`onClick.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **自定义锚点高亮**（`customizeHighlight.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **设置锚点滚动偏移量**（`targetOffset.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **监听锚点链接改变**（`onChange.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **替换历史中的 href**（`replace.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onChange` | 值变化 | 监听锚点链接改变 |
| `onClick` | 点击 | `click` 事件的 handler |
| `items` | 数据化 items | 数据化配置选项内容，支持通过 children 嵌套 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 横向 Anchor | `horizontal.tsx` | 否 |
| 静态位置 | `static.tsx` | 否 |
| 自定义 onClick 事件 | `onClick.tsx` | 否 |
| 自定义锚点高亮 | `customizeHighlight.tsx` | 否 |
| 设置锚点滚动偏移量 | `targetOffset.tsx` | 否 |
| 每个链接单独的滚动偏移量 | `targetOffset-per-link.tsx` | 是 |
| 监听锚点链接改变 | `onChange.tsx` | 否 |
| 替换历史中的 href | `replace.tsx` | 否 |
| 废弃的 JSX 示例 | `legacy-anchor.tsx` | 是 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 组件 Token | `component-token.tsx` | 是 |

### 2.6 FAQ

## FAQ

### 在 `5.25.0+` 版本中，锚点跳转后，目标元素的 `:target` 伪类未按预期生效 {#faq-target-pseudo-class}

出于页面性能优化考虑，锚点跳转的实现方式从 `window.location.href` 调整为 `window.history.pushState/replaceState`。由于 `pushState/replaceState` 不会触发页面重载，因此浏览器不会自动更新 `:target` 伪类的匹配状态。可以手动构造完整URL：`href = window.location.origin + window.location.pathname + '#xxx'` 来解决这问题。

相关issues：[#53143](https://github.com/ant-design/ant-design/issues/53143) [#54255](https://github.com/ant-design/ant-design/issues/54255)

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

### Anchor Props

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| affix | 固定模式 | boolean \| Omit<AffixProps, 'offsetTop' \| 'target' \| 'children'> | true | object: 5.19.0 | × |
| bounds | 锚点区域边界 | number | 5 | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | getContainer | 指定滚动的容器 | () => HTMLElement | () => window | getCurrentAnchor | 自定义高亮的锚点 | (activeLink: string) => string | - | offsetTop | 距离窗口顶部达到指定偏移量后触发 | number | 0 | showInkInFixed | `affix={false}` 时是否显示小方块 | boolean | false | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | targetOffset | 锚点滚动偏移量，默认与 offsetTop 相同，[例子](#anchor-demo-targetoffset) | number | - | onChange | 监听锚点链接改变 | (currentActiveLink: string) => void | - | onClick | `click` 事件的 handler | (e: MouseEvent, link: object) => void | - | items | 数据化配置选项内容，支持通过 children 嵌套 | { key, href, title, target, children }\[] [具体见](#anchoritem) | - | 5.1.0 | × |
| direction | 设置导航方向 | `vertical` \| `horizontal` | `vertical` | 5.2.0 | × |
| replace | 替换浏览器历史记录中项目的 href 而不是推送它 | boolean | false | 5.7.0 | × |

### AnchorItem

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| key | 唯一标志 | string \| number | - | target | 该属性指定在何处显示链接的资源 | string | - | children | 嵌套的 Anchor Link，`注意：水平方向该属性不支持` | [AnchorItem](#anchoritem)\[] | - | targetOffset | 设置单个锚点的滚动偏移量，会覆盖 Anchor 组件的 targetOffset 属性 | number | - | 6.4.0 |

### Link Props

建议使用 items 形式。

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| href | 锚点链接 | string | - | title | 文字内容 | ReactNode | - 
### 导入方式

```js
import { Anchor } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `affix` | 固定模式 | boolean \| Omit | true | object: 5.19.0 |
| `bounds` | 锚点区域边界 | number | 5 | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `getContainer` | 指定滚动的容器 | () => HTMLElement | () => window | — |
| `getCurrentAnchor` | 自定义高亮的锚点 | (activeLink: string) => string | - | — |
| `offsetTop` | 距离窗口顶部达到指定偏移量后触发 | number | 0 | — |
| `showInkInFixed` | `affix={false}` 时是否显示小方块 | boolean | false | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `targetOffset` | 锚点滚动偏移量，默认与 offsetTop 相同，[例子](#anchor-demo-targetoffset) | number | - | — |
| `onChange` | 监听锚点链接改变 | (currentActiveLink: string) => void | - | — |
| `onClick` | `click` 事件的 handler | (e: MouseEvent, link: object) => void | - | — |
| `items` | 数据化配置选项内容，支持通过 children 嵌套 | { key, href, title, target, children }\[] [具体见](#anchoritem) | - | 5.1.0 |
| `direction` | 设置导航方向 | `vertical` \| `horizontal` | `vertical` | 5.2.0 |
| `replace` | 替换浏览器历史记录中项目的 href 而不是推送它 | boolean | false | 5.7.0 |
| `key` | 唯一标志 | string \| number | - | — |
| `href` | 锚点链接 | string | - | — |
| `target` | 该属性指定在何处显示链接的资源 | string | - | — |
| `title` | 文字内容 | ReactNode | - | — |
| `children` | 嵌套的 Anchor Link，`注意：水平方向该属性不支持` | [AnchorItem](#anchoritem)\[] | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Anchor** 的验收清单：

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
- 官方文档：https://ant.design/components/anchor
- 中文文档：https://ant.design/components/anchor-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/anchor
- 驱动 gpui kit：`anchor`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Anchor** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/anchor/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Anchor）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 选中/展开/分页或步骤切换与键盘 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Anchor）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：用于跳转到页面指定位置。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**导航**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `affix` | 固定模式 | boolean \ | Omit<AffixProps, 'offsetTop' \ |
| `bounds` | 锚点区域边界 | number | 5 |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> |
| `getContainer` | 指定滚动的容器 | () => HTMLElement | () => window |
| `getCurrentAnchor` | 自定义高亮的锚点 | (activeLink: string) => string | - |
| `offsetTop` | 距离窗口顶部达到指定偏移量后触发 | number | 0 |
| `showInkInFixed` | `affix={false}` 时是否显示小方块 | boolean | false |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> |
| `targetOffset` | 锚点滚动偏移量，默认与 offsetTop 相同，[例子](#anchor-demo-targetoffset) | number | - |
| `onChange` | 监听锚点链接改变 | (currentActiveLink: string) => void | - |
| `onClick` | `click` 事件的 handler | (e: MouseEvent, link: object) => void | - |
| `items` | 数据化配置选项内容，支持通过 children 嵌套 | { key, href, title, target, children … | - |
| `direction` | 设置导航方向 | `vertical` \ | `horizontal` |
| `replace` | 替换浏览器历史记录中项目的 href 而不是推送它 | boolean | false |
| `key` | 唯一标志 | string \ | number |
| `href` | 锚点链接 | string | - |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
click link ──► 滚动到 href 目标 + onClick
scroll ──► 计算 active link + ink + onChange
affix ──► 钉住
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| ANC-S1 | 点击项 | 滚到锚点 |
| ANC-S2 | 滚动经过 section | active 切换；onChange |
| ANC-S3 | affix | 钉住 |
| ANC-S4 | ink | 指示条在 active |
| ANC-S5 | 嵌套 items | 二级可见 |
| ANC-S6 | targetOffset | 停位偏移 |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 符合 §6.2 Token |
| hover/active/focus | 可交互者具备反馈与 focus ring |
| disabled / loading / empty | 按本控件语义 |
| 主题切换 | 色与间距随 Theme 更新 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | navigation / menu / tablist 等 |
| 当前 | aria-current / selected |
| 键盘 | 方向键与激活 |

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
| `onChange` | 必须 |
| `onClick` | 必须 |
| `items` | 必须 |
| `children` | 必须 |
| `title` | 必须 |
| 官方主路径示例 | 基本、横向 Anchor、静态位置、自定义 onClick 事件、自定义锚点高亮、设置锚点滚动偏移量、监听锚点链接改变、替换历史中的 href |
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
| 其余示例 | 自定义语义结构的样式和类, _semantic.tsx |

### 6.9 验收用例表（可测）

> 测试名建议：`TestAnchor_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Anchor 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| ANC-01 | L1 | NewAnchor 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| ANC-02 | L1 | 点击项 | 滚到锚点 |
| ANC-03 | L1 | 滚动经过 section | active 切换；onChange |
| ANC-04 | L1 | affix | 钉住 |
| ANC-05 | L1 | ink | 指示条在 active |
| ANC-06 | L1 | 嵌套 items | 二级可见 |
| ANC-07 | L1 | targetOffset | 停位偏移 |
| ANC-08 | L1 | 复现官方示例「基本」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| ANC-09 | L1 | 复现官方示例「横向 Anchor」（`horizontal.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| ANC-10 | L1 | 复现官方示例「静态位置」（`static.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| ANC-11 | L1 | 复现官方示例「自定义 onClick 事件」（`onClick.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| ANC-12 | L1 | 复现官方示例「自定义锚点高亮」（`customizeHighlight.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| ANC-13 | L1 | 复现官方示例「设置锚点滚动偏移量」（`targetOffset.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| ANC-14 | L1 | 复现官方示例「监听锚点链接改变」（`onChange.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| ANC-15 | L1 | 复现官方示例「替换历史中的 href」（`replace.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| ANC-16 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| ANC-17 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| ANC-18 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| ANC-19 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| ANC-20 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| ANC-21 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| ANC-22 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewAnchor(...) *Anchor

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
Nav root
  └─ items / panels / connectors
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Anchor 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/anchor.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Anchor 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
