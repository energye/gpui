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

- **依赖等级**：L3（导航：依赖 Affix 钉住 + 滚动宿主定位）。
- **等谁**：等 `kit.Affix`（`affix=true` 默认钉住的承载）与滚动视口（`getContainer` 桌面映射 `ScrollTarget`）就绪；`affix=false` 静态模式可先行。
- **文件归属**：`ui/kit/anchor/`。
- **组合**：默认包一层 Affix（`offsetTop`/`bounds` 透传）；`getContainer` 指向的滚动宿主由上层注入，见 §6.7。
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
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `getContainer` | 指定滚动的容器 | () => HTMLElement | () => window | — |
| `getCurrentAnchor` | 自定义高亮的锚点 | (activeLink: string) => string | - | — |
| `offsetTop` | 距离窗口顶部达到指定偏移量后触发 | number | 0 | — |
| `showInkInFixed` | `affix={false}` 时是否显示小方块 | boolean | false | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `targetOffset` | 锚点滚动偏移量，默认与 offsetTop 相同，[例子](#anchor-demo-targetoffset) | number | - | — |
| `onChange` | 监听锚点链接改变 | (currentActiveLink: string) => void | - | — |
| `onClick` | `click` 事件的 handler | (e: MouseEvent, link: object) => void | - | — |
| `items` | 数据化配置选项内容，支持通过 children 嵌套 | { key, href, title, target, children }\[] [具体见](#anchoritem) | - | 5.1.0 |
| `direction` | 设置导航方向 | `vertical` \| `horizontal` | `vertical` | 5.2.0 |
| `replace` | 替换浏览器历史记录中项目的 href 而不是推送它 | boolean | false | 5.7.0 |
| `replace`（item 级） | 单链替换历史而非 push，覆盖全局 `replace`（源码 `Anchor.tsx createNestedLink` 中 `{...item}` 覆盖） | boolean | false | 5.7.0 |
| `targetOffset`（单链） | 单链滚动偏移，覆盖全局 `targetOffset`（`linkTargetOffsetRef`，`targetOffsetParams ?? targetOffset ?? offsetTop`，P1） | number | - | 6.4.0 |
| `key` | 唯一标志 | string \| number | - | — |
| `href` | 锚点链接 | string | - | — |
| `target`（item 级） | 该属性指定在何处显示链接的资源 | string | - | — |
| `title` | 文字内容 | ReactNode | - | — |
| `children` | 嵌套的 Anchor Link，`注意：水平方向该属性不支持` | [AnchorItem](#anchoritem)\[] | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Anchor** 的验收清单：

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
11. **示例矩阵**：P0 按 §6.8 逐例对照表（8 例主路径），余下 P1 一次做完（单链 `targetOffset`（debug 仅参考不验收）、`legacy-anchor`、`component-token`、`style-class` 语义深度）。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/anchor
- 中文文档：https://ant.design/components/anchor-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/anchor
- 驱动 gpui kit：`anchor`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Anchor** 补成 **可开发、可测试、可验收** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5.1** 官网截图 99.99% 相似（见 ACCEPTANCE 对齐定义）。只允字体光栅亚像素差；颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即不算对齐。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/anchor/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Anchor）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 点击跳转锚点、滚动跟随高亮（bounds 交叠阈值）、ink 指示、affix 钉住与键盘激活 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | showcase 大图 + 官网并排 | showcase大图进testdata按§6.8摆全多种式样（CPU容差内比对）+ 与官网同示例截图并排验收（颜色/尺寸/圆角/间距/边框/阴影/布局任一项肉眼可见差异即挂，只允字体光栅亚像素差） | showcase/官网并排 |
| **L4** | 官网并排必验（非可选） | 建/大改基线时与官网截图并排验收并留验收记录（见L3），CI比showcase基线，人眼签官网并排 | 建/大改基线必验 |

**明确不做（Anchor）：**

- 与官网截图逐字节哈希一致（只要求 99.99% 相似，允字体光栅亚像素差）。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，单测Skip写清平台原因）。  
- 官方 **debug** 示例不验收（仅参考）。  

> 控件说明：用于跳转到页面指定位置。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design Anchor `style/index.ts` + 本库 Theme 默认** 为准（`scale=1`，种子：`fontSize=14`、`padding=16`、`paddingXXS=4`、`lineWidth=1`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。源码：`components/anchor/style/index.ts` `prepareComponentToken` + `mergeToken`（`holderOffsetBlock=paddingXXS=4`；次级/标题/圆点见 `AnchorToken`）。

> Anchor **无** `size` / `controlHeight` 档位（与 Button 不同）；通用 controlHeight 表不适用。

> Anchor **无** `size` / `controlHeight` 档位（与 Button 不同）；通用 controlHeight 表不适用。

#### 6.2.1 几何与组件 Token（antd `prepareComponentToken` / `mergeToken`）

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 字号 | **14** | `fontSize` |
| 链接纵向内间距 `linkPaddingBlock` | **4** | `paddingXXS` → kit `TokenPaddingXS` |
| 链接横向起边距 `linkPaddingInlineStart` | **16** | `padding` → kit `TokenPadding`（±0.5px） |
| 容器块偏移 `holderOffsetBlock` | **4** | `paddingXXS`（`marginBlockStart=-4` / `paddingBlockStart=4`，±0.5px） |
| 次级链接纵向间距 `anchorPaddingBlockSecondary` | **2** | `paddingXXS / 2`（`mergeToken` 计算值，±0.5px） |
| 标题块间距 `anchorTitleBlock` | **3** | `fontSize/14*3`（仅有嵌套子链时；±0.5px） |
| 指示条粗细（ink / 左轨） | **2** | antd `lineWidthBold`（≈ `lineWidth*2`；kit 回落 2，±0.5px） |
| 分割轨色 | `colorSplit` | 垂直左轨 / 水平底轨 |
| 圆角（容器） | **6** | `borderRadius`（链接本身无圆角强制；±0.5px） |
| 边框线宽（轨） | **1** | `lineWidth`（水平底部分割线） |
| Focus ring outset | ≈ **1.5px** 可见 | 可调，必须可见 |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 链接默认字色 | `colorText` | title 默认 |
| 激活 / ink | `colorPrimary` | active title + ink 条 |
| hover 字色 | `colorPrimary` 或 `colorPrimaryHover` | 可交互反馈 |
| 分割轨 | `colorSplit` | 垂直 `borderInlineStart` / 水平 `borderBottom` |
| 禁用（适用者） | `colorDisabledText` | Anchor 本体无 disabled API；预留 |
| 容器底 | `colorBgContainer` | 透明可 |

禁止硬编码品牌色作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**导航**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `affix` | 固定模式；P0 仅 bool，对象形态（`Omit<AffixProps,…>`）透传为 P1 | boolean \ | Omit<AffixProps, 'offsetTop' \ |
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
| `replace`（item 级） | 单链替换历史而非 push，覆盖全局 `replace`（源码 `createNestedLink` 中 `{...item}` 覆盖） | boolean | false（5.7.0） |
| `targetOffset`（单链） | 单链滚动偏移，覆盖全局 `targetOffset`（`targetOffsetParams ?? targetOffset ?? offsetTop`） | number | -（6.4.0） |
| `key` | 唯一标志 | string \ | number |
| `href` | 锚点链接 | string | - |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
click link ──► 滚动到 href 目标（应用 targetOffset 停位偏移）+ onClick({title,href,key})
scroll（滚动宿主 ScrollTarget 上报 Y）──► 按 bounds 交叠阈值计算 active link + ink 同步 + onChange(activeHref)
affix=true ──► Affix 钉住（offsetTop 阈值，见 Affix AFX-S1）
```

**触发条件 + 容差（可断言）：**

- 交叠阈值：`bounds` 默认 **5**（px）；滚动位置落在 `[sectionTop - targetOffset - bounds, sectionTop - targetOffset + bounds]` 即视为进入该锚点区，active 切换。断言：构造两 section 间距 20，`bounds=5` 时分界点前后 ±0.5px 内切换正确。
- ink 偏移：ink 条顶部 = active title 盒顶部 ±0.5px；`showInkInFixed=false + affix=false` 时 ink 隐藏（`InkVisible()==false`）。
- `targetOffset` 停位：`ScrollTo(href)` 后视口 Y = `sectionY - (targetOffset ?? offsetTop)`，±0.5px。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| ANC-S1 | 点击项 | 滚到锚点（停位 = sectionY - targetOffset/offsetTop，±0.5px） |
| ANC-S2 | 滚动经过 section（含 bounds=5 交叠带） | active 切换；onChange(activeHref) |
| ANC-S3 | affix=true 默认 | 钉住（阈值走 Affix offsetTop 规则） |
| ANC-S4 | ink | 指示条顶部与 active title 对齐 ±0.5px |
| ANC-S5 | 嵌套 items（vertical） | 二级可见；horizontal 忽略 children |
| ANC-S6 | targetOffset | 停位偏移 = sectionY - targetOffset（±0.5px） |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | title 字色 `colorText`（14px，`linkPaddingBlock=4` / `linkPaddingInlineStart=16`）；左轨 `colorSplit` 宽 2（`lineWidthBold`） |
| hover | title 字色切 `colorPrimaryHover`（或 `colorPrimary`），无底色块 |
| active | title 字色 `colorPrimary`；ink 条（宽 2，主色）在 active title 左侧，顶部对齐 ±0.5px |
| focus | title 可聚焦时 focus ring 可见（outset ≈1.5px） |
| horizontal | 底轨分割线（`lineWidth=1` + `colorSplit`）+ 底 ink 条；children 不渲染 |
| 主题切换 | 色与间距随 Theme 更新 |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | 根 `role=navigation`；链接列为 link 列表 |
| 当前 | active 项 `aria-current=true`（或 `aria-current="location"`），读屏报“当前位置” |
| 键盘 | Tab 逐项聚焦，focus ring 可见；Enter/Space 跳转并触发 OnClick；方向键上下在项间移动焦点 |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 真实映射（gpui 侧落点） | 级别 |
| --- | --- | --- |
| 主路径行为（点击跳转/滚动高亮/ink） | **对等**：`ScrollTo` + `SyncFromScroll` + ink 同步 | P0 L1 |
| 链接/ink 度量（§6.2 pad 4/16/轨 2） | **对等** | P0 L2 |
| `affix` 钉住与 `offsetTop`/`bounds` | **对等**：包 `kit.Affix`，`offsetTop` 透传 Affix 阈值（桌面 Sticky 映射） | P0 L1 |
| `getContainer` 滚动容器 | **映射**：桌面 `ScrollTarget *ScrollViewport` + `SectionOffsets map[href]Y` + `SyncFromScroll`（对等 DOM id 查询 + scroll 监听） | P0 宿主 |
| `replace` 历史替换 | **映射**：桌面 History 栈改末项（`History []string + CurrentHref`，`Replace=true` 时不 push） | P0 L1 |
| `getCurrentAnchor(activeHref)` 自定义高亮 | **对等**：`SetGetCurrentAnchor(func)`，`ActiveLink` 存解析后 href | P0 L1 |
| per-link `targetOffset`（6.4.0） | P1 必做（`linkTargetOffsetRef` 单链覆盖，见 targetOffset-per-link debug） | P1 |
| ink 滑动动画 | P0 瞬时同步 active title 盒，像素级滑动 P1 | P0 L1/P1 |
| Semantic classNames/styles | kit 语义钩子 | P1 |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 官网截图相似（99.99%） | **不做** | — |

### 6.8 能力范围（P0 / P1 全做）

#### P0（本阶段必须 1:1，与P1一次做完）

| 配置 / 能力 | 说明 |
| --- | --- |
| `items` + `title` + `href` + `key` | 数据化配置；title/href 为链接展示与跳转主键 |
| `children` 嵌套 | 垂直方向二级可见；水平方向忽略 children（与 antd 一致） |
| `direction` | `vertical`（默认）/ `horizontal` |
| `affix` | 默认 **true**；`false` 为静态位置（static 示例） |
| `showInkInFixed` | `affix=false` 时是否仍显示 ink；默认 false |
| `bounds` | 滚动高亮边界，默认 **5** |
| `offsetTop` / `targetOffset` | affix 顶偏 / 滚动停位偏移（targetOffset 默认等同 offsetTop） |
| `getCurrentAnchor` | 自定义高亮 href（customizeHighlight） |
| `onChange` / `onClick` | 必须；onClick 载荷 `{title, href}`（+ key） |
| `replace` | 导航时替换历史项而非 push（桌面映射见 §6.7） |
| ink 指示条 | active 时可见（vertical 左轨 / horizontal 底轨）；`affix=false && !showInkInFixed` 隐藏 |
| 滚动宿主映射 | 桌面：`ScrollTarget *ScrollViewport` + `SectionOffsets map[href]Y` + `SyncFromScroll`（对等 `getContainer` + DOM id 查询） |
| 官方主路径示例（P0，8 例） | 基本（`basic.tsx`）、横向 Anchor（`horizontal.tsx`）、静态位置（`static.tsx`，`affix=false`）、自定义 onClick（`onClick.tsx`）、自定义锚点高亮（`customizeHighlight.tsx`，`getCurrentAnchor`）、设置锚点滚动偏移量（`targetOffset.tsx`）、监听锚点链接改变（`onChange.tsx`）、替换历史中的 href（`replace.tsx`） |
| 度量 §6.2 | Token 断言（pad 4/16、轨 2，±0.5px） |
| a11y §6.6 | role=navigation；当前 `aria-current`；键盘方向键 + 激活 |
| §6.9 中 L1/L2 用例 | 测试通过 |

**逐例 P0/P1 对照表**（§2.4 全量；P0+P1=§6.8 非debug 8+4 例一次做完）：

| 示例 | 范围 | 原因 |
| --- | --- | --- |
| 基本 / 横向 / 静态位置 / 自定义 onClick / 自定义锚点高亮 / targetOffset / onChange / replace | P0 | 点击跳转 + 滚动高亮 + ink + affix 主路径 |
| 每个链接单独的滚动偏移量（`targetOffset-per-link.tsx`，debug，6.4.0） | P1 | 单链 `targetOffset` 覆盖 P1必做 |
| 废弃的 JSX 示例（`legacy-anchor.tsx`，debug） | P1 | 旧 JSX 形态，不进主路径 |
| 自定义语义结构的样式和类（`style-class.tsx`）/ `_semantic.tsx` | P1 | semantic 深度 |
| 组件 Token（`component-token.tsx`，debug） | P1 | 调试页，不验收 |

#### P1（本阶段必须 1:1，与 P0 同标准；真做不了的单测 Skip 写清平台原因）

| 配置 / 能力 | 说明 |
| --- | --- |
| per-link `targetOffset`（6.4.0 / targetOffset-per-link debug） | 不验收（仅参考） |
| semantic classNames/styles 深度 | P1 必做（style-class / _semantic） |
| ink 滑动动画像素级 | P1 必做；P0+P1 瞬时切换 |
| 真浏览器 history / `window` 容器 | 桌面用 History 栈 + ScrollTarget 映射 |
| ConfigProvider 全局默认 | P1 必做 |
| debug 示例 | 不验收（仅参考） |

### 6.9 验收用例表（可测）

> 测试名建议：`TestAnchor_PRD_<ID>` 或 gallery 场景 ID。  
> **P0+P1 相关用例全部通过** 才可宣称 Anchor 完成 1:1。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| ANC-01 | L1 | NewAnchor 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| ANC-02 | L1 | 点击项 | 滚到锚点（停位=sectionY-targetOffset/offsetTop，±0.5px）；OnClick 载荷含 title/href |
| ANC-03 | L1 | 滚动经过 section（含 bounds=5 交叠带） | active 切换；onChange(activeHref) |
| ANC-04 | L1 | affix=true 默认 | 钉住（阈值走 Affix offsetTop） |
| ANC-05 | L1 | ink | 指示条顶部与 active title 对齐 ±0.5px；affix=false 且 !showInkInFixed 时隐藏 |
| ANC-06 | L1 | 嵌套 items（vertical）/ horizontal 忽略 children | 二级可见 / 水平不渲染子链 |
| ANC-07 | L1 | targetOffset=100 后 ScrollTo | 停位=sectionY-100（±0.5px） |
| ANC-08 | L1 | 复现官方示例「基本」（`basic.tsx`） | 三项纵排可点；点击切 active 并 ScrollTo 对应 section（±0.5px） |
| ANC-09 | L1 | 复现官方示例「横向 Anchor」（`horizontal.tsx`） | 横排底轨 + 底 ink；点击切 active；children 不渲染 |
| ANC-10 | L1 | 复现官方示例「静态位置」（`static.tsx`） | `affix=false` 不钉住；`showInkInFixed=false` 时 ink 隐藏 |
| ANC-11 | L1 | 复现官方示例「自定义 onClick 事件」（`onClick.tsx`） | 点击触发 OnClick 且载荷含 title/href；仍滚动到锚点 |
| ANC-12 | L1 | 复现官方示例「自定义锚点高亮」（`customizeHighlight.tsx`） | `getCurrentAnchor` 返回值即 ActiveLink；ink 跟随解析后 href |
| ANC-13 | L1 | 复现官方示例「设置锚点滚动偏移量」（`targetOffset.tsx`） | 停位=sectionY-targetOffset（±0.5px）；滚动高亮分界同偏移 |
| ANC-14 | L1 | 复现官方示例「监听锚点链接改变」（`onChange.tsx`） | 滚动切 active 时 OnChange 恰调一次且参数为新 href |
| ANC-15 | L1 | 复现官方示例「替换历史中的 href」（`replace.tsx`） | `replace=true` 跳转改 History 末项不 push；长度不变 |
| ANC-16 | L2 | 读取 §6.2 关键尺寸/间距 | pad 纵 4/横 16、轨 2（±0.5px） |
| ANC-17 | L2 | 默认皮颜色 | title 默认 `colorText`、active/ink `colorPrimary`、轨 `colorSplit`；无硬编码品牌色 |
| ANC-18 | L2 | Anchor 无 disabled API | 本用例记 N/A（`colorDisabledText` 仅预留不断言） |
| ANC-19 | L1 | 键盘/焦点主路径 | Tab 逐项聚焦 ring 可见；Enter/Space 跳转触发 OnClick；上下键移动焦点 |
| ANC-20 | L3 | 关键态 golden 截图 | 与showcase基线一致（容差内）+官网并排验收通过 |
| ANC-21 | L4 | 与 ant.design 并排 | 官网并排验收记录（必验） |
| ANC-22 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 **breaking** 旧 `NewAnchor(...string)` / `Active string` API；以下为产品契约。

```text
type AnchorItem struct {
  Key, Href, Title, Target string
  Children []AnchorItem
  Replace  bool // item-level replace（可选；默认跟 Anchor.Replace）
}

type AnchorLinkInfo struct { Title, Href, Key string }

type AnchorDirection int // AnchorVertical=0 | AnchorHorizontal

NewAnchor(items ...AnchorItem) *Anchor

// items / 方向 / affix
SetItems([]AnchorItem)
SetDirection(AnchorDirection)   // or SetHorizontal(bool)
SetAffix(bool)                  // default true
SetShowInkInFixed(bool)         // default false
SetBounds(float64)              // default 5
SetOffsetTop(float64)
SetTargetOffset(float64)        // 未 Set 时回落 OffsetTop
SetReplace(bool)
SetGetCurrentAnchor(func(active string) string)

// 滚动宿主（桌面映射 getContainer + section tops）
SetScrollTarget(*primitive.ScrollViewport)
SetSectionOffsets(map[string]float64) // href → content Y
SyncFromScroll()                      // scroll-spy → ActiveLink + OnChange + ink
ScrollTo(href string)                 // 应用 targetOffset 后 SetScroll

// 回调 / 状态
OnChange func(currentActiveLink string)
OnClick  func(link AnchorLinkInfo)
ActiveLink string                     // 当前高亮 href（getCurrentAnchor 解析后）
// 历史映射（replace）：History []string + CurrentHref；Replace=true 时改末项

// 主题 / a11y / 挂树
SetTheme(*Theme)  SetFace(text.Face)  SetAriaLabel(string)
Node() core.Node  ChromeNode() core.Node
InkVisible() bool                     // 测试用：ink 是否绘制
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Direction | vertical |
| Affix | **true** |
| ShowInkInFixed | false |
| Bounds | **5** |
| OffsetTop | 0 |
| TargetOffset | 等同 OffsetTop |
| Replace | false |
| ActiveLink | ""（无高亮 / ink 隐藏） |
| Items | 构造参数或空 |

### 6.11 结构与绘制分层（实现提示）

```text
[Affix/Sticky?]                              // affix=true 时钉住
  └─ wrapper (role=navigation)
       └─ Stack  (relative)
            ├─ rail   垂直: 左 colorSplit 线；水平: 底部分割线
            ├─ ink    PositionedAt primary 条（active 时）
            └─ links  Flex column|row
                 └─ link Pressable(title) + nested children (vertical only)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读字段 / Token；ink 位置在 Layout 后根据 active title 盒同步（可瞬时，动画 P1）。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Anchor 1:1 完成**：

1. §6.8 **P0+P1** 全部实现（官方非debug一个不少，真做不了的单测Skip写清平台原因）。  
2. §6.9 中 **P0+P1** 用例全部通过。
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 showcase大图+官网并排验收通过（控件可见时必需，见§6.1）。
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0+P1** 全部（官方非 debug 一个不少；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 必须进 gallery，真做不了的单测 Skip 写清平台原因。
6. `coverage.go` Notes：P0+P1 已对齐 `docs/antd/anchor.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Anchor 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围定义（无裁剪）。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
