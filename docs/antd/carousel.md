# Carousel 走马灯
> 来源：[Ant Design 6.5.x Carousel](https://ant.design/components/carousel)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：一组轮播的区域。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

一组轮播的区域。

**Carousel** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 位置 | placement 方位 |
| 自动切换 | 复现「自动切换」视觉与布局 |
| 渐显 | 复现「渐显」视觉与布局 |
| 切换箭头 | arrow 指示 |
| 进度条 | 进度条/圈 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `dotPlacement`

- **说明**：面板指示点位置，可选 `top` `bottom` `start` `end`
- **类型**：string
- **默认值**：`bottom`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `top` | 上方 |
  | `bottom` | 下方 |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |

#### `dotPosition`

- **说明**：面板指示点位置，可选 `top` `bottom` `left` `right` `start` `end`，请使用 `dotPlacement` 替换
- **类型**：string
- **默认值**：`bottom`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `top` | 上方 |
  | `bottom` | 下方 |
  | `left` | 左侧 |
  | `right` | 右侧 |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |
  | `dotPlacement` | 官方取值 `dotPlacement` |

#### `draggable`

- **说明**：是否启用拖拽切换
- **类型**：boolean
- **默认值**：false

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

- 当有一组平级的内容。
- 当内容空间不足时，可以用走马灯的形式进行收纳，进行轮播展现。
- 常用于一组图片或卡片轮播。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **位置**（`placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **自动切换**（`autoplay.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **渐显**（`fade.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **切换箭头**（`arrows.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **进度条**（`dot-duration.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 位置 | `placement.tsx` | 否 |
| 自动切换 | `autoplay.tsx` | 否 |
| 渐显 | `fade.tsx` | 否 |
| 切换箭头 | `arrows.tsx` | 否 |
| 进度条 | `dot-duration.tsx` | 否 |
| 组件 Token | `component-token.tsx` | 是 |

### 2.5 实例方法 / Ref

#### 方法 {#methods}

| 名称                           | 描述                                              |
| ------------------------------ | ------------------------------------------------- |
| goTo(slideNumber, dontAnimate) | 切换到指定面板, dontAnimate = true 时，不使用动画 |
| next()                         | 切换到下一面板                                    |
| prev()                         | 切换到上一面板                                    |

### 2.6 FAQ

## FAQ

### 如何自定义箭头？ {#faq-add-custom-arrows}

可参考 [#12479](https://github.com/ant-design/ant-design/issues/12479)。

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
| arrows | 是否显示箭头 | boolean | false | 5.17.0 | × |
| autoplay | 是否自动切换，如果为 object 可以指定 `dotDuration` 来展示指示点进度条 | boolean \| { dotDuration?: boolean } | false | dotDuration: 5.24.0 | × |
| autoplaySpeed | 自动切换的间隔（毫秒） | number | 3000 | adaptiveHeight | 高度自适应 | boolean | false | dotPlacement | 面板指示点位置，可选 `top` `bottom` `start` `end` | string | `bottom` | ~~dotPosition~~ | 面板指示点位置，可选 `top` `bottom` `left` `right` `start` `end`，请使用 `dotPlacement` 替换 | string | `bottom` | dots | 是否显示面板指示点，如果为 `object` 则可以指定 `dotsClass` | boolean \| { className?: string } | true | draggable | 是否启用拖拽切换 | boolean | false | fade | 使用渐变切换动效 | boolean | false | infinite | 是否无限循环切换（实现方式是复制两份 children 元素，如果子元素有副作用则可能会引发 bug） | boolean | true | speed | 切换动效的时间（毫秒） | number | 500 | easing | 动画效果 | string | `linear` | effect | 动画效果函数 | `scrollx` \| `fade` | `scrollx` | afterChange | 切换面板的回调 | (current: number) => void | - | beforeChange | 切换面板的回调 | (current: number, next: number) => void | - | waitForAnimate | 是否等待切换动画 | boolean | false 
更多 API 可参考：<https://react-slick.neostack.com/docs/api>

## 方法 {#methods}

| 名称                           | 描述                                              |
| ------------------------------ | ------------------------------------------------- |
| goTo(slideNumber, dontAnimate) | 切换到指定面板, dontAnimate = true 时，不使用动画 |
| next()                         | 切换到下一面板                                    |
| prev()                         | 切换到上一面板                                    |

### 导入方式

```js
import { Carousel } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `arrows` | 是否显示箭头 | boolean | false | 5.17.0 |
| `autoplay` | 是否自动切换，如果为 object 可以指定 `dotDuration` 来展示指示点进度条 | boolean \| { dotDuration?: boolean } | false | dotDuration: 5.24.0 |
| `autoplaySpeed` | 自动切换的间隔（毫秒） | number | 3000 | — |
| `adaptiveHeight` | 高度自适应 | boolean | false | — |
| `dotPlacement` | 面板指示点位置，可选 `top` `bottom` `start` `end` | string | `bottom` | — |
| `dotPosition` | 面板指示点位置，可选 `top` `bottom` `left` `right` `start` `end`，请使用 `dotPlacement` 替换 | string | `bottom` | — |
| `dots` | 是否显示面板指示点，如果为 `object` 则可以指定 `dotsClass` | boolean \| { className?: string } | true | — |
| `draggable` | 是否启用拖拽切换 | boolean | false | — |
| `fade` | 使用渐变切换动效 | boolean | false | — |
| `infinite` | 是否无限循环切换（实现方式是复制两份 children 元素，如果子元素有副作用则可能会引发 bug） | boolean | true | — |
| `speed` | 切换动效的时间（毫秒） | number | 500 | — |
| `easing` | 动画效果 | string | `linear` | — |
| `effect` | 动画效果函数 | `scrollx` \| `fade` | `scrollx` | — |
| `afterChange` | 切换面板的回调 | (current: number) => void | - | — |
| `beforeChange` | 切换面板的回调 | (current: number, next: number) => void | - | — |
| `waitForAnimate` | 是否等待切换动画 | boolean | false | — |
| `goTo(slideNumber, dontAnimate)` | 切换到指定面板, dontAnimate = true 时，不使用动画 | — | — | — |
| `next()` | 切换到下一面板 | — | — | — |
| `prev()` | 切换到上一面板 | — | — | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Carousel** 的验收清单：

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
- 官方文档：https://ant.design/components/carousel
- 中文文档：https://ant.design/components/carousel-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/carousel
- 驱动 gpui kit：`carousel`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Carousel** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/carousel/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Carousel）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 数据渲染与选择/展开/分页/加载主路径 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Carousel）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：一组轮播的区域。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。  
源码：`components/carousel/style/index.ts` → `prepareComponentToken`。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 字号 | **14** | `fontSize` |
| 圆角 | **6** | `borderRadius` |
| 边框线宽 | **1** | `lineWidth` |
| Focus ring outset | ≈ **1.5px** 可见 | 可调，必须可见 |
| 指示点宽 `dotWidth` | **16** | 组件 Token |
| 指示点高 `dotHeight` | **3** | 组件 Token |
| 指示点间距 `dotGap` | **4** | `marginXXS`（本库回落 4） |
| 指示点边距 `dotOffset` | **12** | 组件 Token |
| 激活指示点宽 `dotActiveWidth` | **24** | 组件 Token |
| 箭头尺寸 `arrowSize` | **16** | 组件 Token |
| 箭头边距 `arrowOffset` | **8** | `marginXS`（antd 语义；本库回落 8） |
| 舞台默认高（demo 节奏） | **160** | 官方 basic 示例 `height: 160px`；非 antd Token |
| `autoplaySpeed` 默认 | **3000** ms | API 默认 |
| `speed` 默认 | **500** ms | API 默认；P0 可瞬时切换 |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 指示点底色 | `colorBgContainer` | antd dots `button` 底；默认 opacity≈0.2，active 拉高 |
| 箭头色 | inverse / `#fff` | slick 默认白箭头 + opacity 0.4→1 hover |
| 主色 / hover / active | `colorPrimary` + 变体 | 强调（非 Carousel 默认皮主路径） |
| 文本 / 次级文本 | `colorText` / `colorTextSecondary` | 舞台内容由业务 slide 自带 |
| 边框 / 分割 / 容器底 | `colorBorder` / `colorSplit` / `colorBgContainer` | |
| 禁用 | 降 opacity / 禁交互 | 无 hover 高亮；禁拖/禁点/禁 autoplay |
| Focus ring | `colorPrimary` | 键盘可见 |

禁止硬编码品牌色（如 `#1677ff`）作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据展示**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `arrows` | 是否显示箭头 | boolean | false |
| `autoplay` | 是否自动切换；object 可指定 `dotDuration` 指示点进度 | boolean \| { dotDuration?: boolean } | false |
| `autoplaySpeed` | 自动切换间隔（毫秒） | number | 3000 |
| `adaptiveHeight` | 高度随当前 slide 自适应 | boolean | false |
| `dotPlacement` | 指示点位置 `top`/`bottom`/`start`/`end` | string | `bottom` |
| `dots` | 是否显示指示点 | boolean | true |
| `draggable` | 是否启用拖拽切换 | boolean | false |
| `fade` / `effect=fade` | 渐显切换（P0 可瞬时；flag 必须可配） | boolean / effect | false / `scrollx` |
| `infinite` | 是否无限循环 | boolean | **true** |
| `speed` | 切换动效时长（毫秒） | number | 500 |
| `afterChange` | 切换后回调 `(current)` | fn | - |
| `beforeChange` | 切换前回调 `(current, next)` | fn | - |
| `initialSlide` / index | 初始索引 | number | 0 |

**配置优先级（通用）：** 受控 index（`GoTo`/`SetIndex`）> 显式非受控 default > 组件默认 > ConfigProvider 全局默认。

**衍生：** `dotPlacement` 为 `start`/`end` 时舞台为**纵向**（antd `vertical`）；`top`/`bottom` 为横向。

### 6.4 交互状态机（L1）

```text
index=i  (0..n-1)
  Next / Prev / dots[n] / GoTo(i) / autoplay Tick / drag 阈值
       ──► beforeChange(i, next) → index' → afterChange(index')
  infinite=true  边界：末→0、0→末
  infinite=false 边界：夹紧；末张 Next / 首张 Prev 无效；箭头 disabled 隐
  Disabled       ──► 禁 Next/Prev/dots/drag/autoplay
  autoplay+DotDuration ──► 活跃指示点进度 0→1 随 autoplaySpeed
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| CRS-S1 | next | index+1；`afterChange` |
| CRS-S2 | dots 点第 n | 到 n |
| CRS-S3 | autoplay | Tick 累计 ≥ autoplaySpeed → 前进 |
| CRS-S4 | 末张再 next 且 infinite | 回 0 |
| CRS-S5 | arrows=false | 无箭头节点 |
| CRS-S6 | GoTo(i) | 跳转；越界夹紧或按 infinite 取模 |
| CRS-S7 | drag 水平/垂直阈值 | 达阈值 Next/Prev |
| CRS-S8 | infinite=false 末张 Next | 仍停留末张 |
| CRS-S9 | Disabled | 交互无效 |

**P0 瞬时切换时序：** P0 允许省去 `speed`/`easing` 动画帧直接换页，但回调时序与 antd 一致——先同步触发 `beforeChange(current, next)`，再更新 `Index`，最后同步触发 `afterChange(next)`；`GoTo(i, dontAnimate=true)` 与动画关闭同此顺序。像素级过渡动画（`speed`/`easing`/`waitForAnimate`）与 L3 golden 均推迟到 P1，本阶段 L3 不强制（见 §6.9 CRS-18/§6.12）。

### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 符合 §6.2 Token；dots 默认 bottom |
| hover/active/focus | 箭头 opacity 提升；dots/箭头可聚焦 focus ring |
| disabled | 降对比；禁交互 |
| empty（0 slides） | 不崩溃；无 dots/箭头操作 |
| 主题切换 | 色与间距随 Theme 更新 |

**动效：** scrollx/fade 像素级动画为 P1；**P0 允许瞬时切换**。`dotDuration` 进度条 P0 用 Ticker 填充活跃点宽（近似 antd keyframes）。尊重 `ReduceMotion`（autoplay 仍可步进，进度可瞬时满）。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 根 | `role=region`（或 group）；`AriaLabel` 可设 |
| 指示点 | 每个 dot 可激活；有名（如 “Go to slide n”） |
| 箭头 | prev/next 有名；`infinite=false` 边界禁用 |
| 键盘 | 聚焦舞台后 `ArrowLeft`/`ArrowRight`（纵向 `ArrowUp`/`ArrowDown`）切换 |
| Focus ring | 可聚焦控件可见 ring（§6.2 outset） |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 主路径行为（§6.1 L1） | **对等** | P0 L1 |
| 尺寸/色 Token（§6.2） | **对等** | P0 L2 |
| scrollx/fade 像素级动画 | **瞬时**或近似 | P1 |
| react-slick 全量 Settings | **子集**映射 | P1 |
| Semantic classNames/styles | kit 语义钩子 | P1 |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 逐像素官网哈希 | **不做** | — |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `arrows` | 显隐 + 点击 Next/Prev |
| `autoplay` + `autoplaySpeed` | Ticker 自动前进 |
| `autoplay.dotDuration` | 活跃点进度条 |
| `adaptiveHeight` | 固定舞台高 vs 随 slide |
| `dotPlacement` | top/bottom/start/end（含纵向） |
| `dots` | 显隐 + 点击跳转 |
| `draggable` | 拖拽阈值切换 |
| `fade` / `effect` | 可配；P0 切换可瞬时 |
| `infinite` | 默认 true；边界循环 |
| `GoTo` / `Next` / `Prev` | 实例方法 |
| `afterChange` / `beforeChange` | 回调 |
| 官方主路径示例 | 基本、位置、自动切换、渐显、切换箭头、进度条 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | 最低要求 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| scrollx/fade 像素级过渡 / `speed`/`easing`/`waitForAnimate` 动画语义 | 分期 |
| 自定义 `prevArrow`/`nextArrow` 节点 | 分期 |
| semantic classNames/styles 深度 | 分期 |
| react-slick 其余 Settings（variableWidth、centerMode、…） | 分期 |
| ConfigProvider 全局 Carousel 默认 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestCarousel_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Carousel 完成 1:1 主路径。  
> L3/L4（CRS-18/19）与 P1（CRS-20）本阶段不强制。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| CRS-01 | L1 | NewCarousel 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| CRS-02 | L1 | next | index+1；afterChange |
| CRS-03 | L1 | dots 点第 n | 到 n |
| CRS-04 | L1 | autoplay | Tick 后自动前进 |
| CRS-05 | L1 | 到末张再 next infinite | 回首 |
| CRS-06 | L1 | arrows=false | 无箭头 |
| CRS-07 | L1 | GoTo(i) | 跳转 |
| CRS-08 | L1 | 复现官方示例「基本」（`basic.tsx`） | 4 slide；afterChange 可挂；布局不崩 |
| CRS-09 | L1 | 复现官方示例「位置」（`placement.tsx`） | 四向 DotPlacement 可设 |
| CRS-10 | L1 | 复现官方示例「自动切换」（`autoplay.tsx`） | Autoplay=true 可前进 |
| CRS-11 | L1 | 复现官方示例「渐显」（`fade.tsx`） | Effect=fade 或 Fade=true |
| CRS-12 | L1 | 复现官方示例「切换箭头」（`arrows.tsx`） | Arrows + infinite=false 边界 |
| CRS-13 | L1 | 复现官方示例「进度条」（`dot-duration.tsx`） | DotDuration + autoplaySpeed |
| CRS-14 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px） |
| CRS-15 | L2 | 默认皮颜色 | 无硬编码品牌主色；走 Theme Token |
| CRS-16 | L2 | disabled | 禁交互；无推进 |
| CRS-17 | L1 | 键盘主路径 | Arrow 键切换（适用） |
| CRS-18 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差）— **本阶段不强制** |
| CRS-19 | L4 | 与 ant.design 并排 | 人眼签字 — **本阶段不强制** |
| CRS-20 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 契约，实现可微调命名但语义不可丢。

```text
NewCarousel(slides ...core.Node) *Carousel

// 数据
SetSlides(...core.Node)
// 索引 / 方法（antd ref）
Index int                    // 当前
SetIndex(i int)              // 等价 GoTo(i) 无动画
GoTo(i int)                  // 切换到 i（P0 瞬时）
Next() / Prev()
// P0 配置
SetArrows(bool)
SetAutoplay(bool)
SetDotDuration(bool)         // autoplay: { dotDuration: true }
SetAutoplaySpeed(ms int)     // 0 → Default 3000
SetAdaptiveHeight(bool)
SetDotPlacement(CarouselDotPlacement)  // Bottom|Top|Start|End
SetDots(bool)                // 默认 true
SetDraggable(bool)
SetFade(bool) / SetEffect(CarouselEffect)  // ScrollX|Fade
SetInfinite(bool)            // 默认 true
SetSpeed(ms int)             // 记录；P0 切换可瞬时
SetWidth / SetHeight         // 舞台优先尺寸；Height 0 + !adaptive → DefaultStageHeight
// 回调
SetAfterChange(func(current int))
SetBeforeChange(func(current, next int))
// 状态 / 主题 / a11y
SetDisabled(bool)
SetTheme(*Theme) / SetFace / SetStyle
SetAriaLabel(string)
AttachTicker(*Tree) / Tick(dt) bool   // autoplay + dotDuration
// 挂树
Node() core.Node             // 稳定 Root 身份（Next/GoTo 不换根）
ChromeNode() core.Node
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Arrows | false |
| Autoplay / DotDuration | false |
| AutoplaySpeed | 3000 |
| AdaptiveHeight | false |
| DotPlacement | bottom |
| Dots | true |
| Draggable | false |
| Fade / Effect | false / scrollx |
| Infinite | **true** |
| Speed | 500 |
| Index | 0 |
| Disabled | false |
| 度量 | §6.2 表 |

### 6.11 结构与绘制分层（实现提示）

```text
Root (Stack, hit==layout==paint)
  ├─ stage (Clip + 可选 drag PointerHandler)
  │     └─ active slide (Slot；P0 单 slide 挂载，瞬时切换)
  ├─ dots (Positioned top|bottom|start|end；Flex row/col)
  │     └─ li button…  active 更宽 + optional duration fill
  └─ arrows? (Positioned；prev / next)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读 Default/字段/Token；**Root 身份稳定**。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- autoplay / dotDuration 走 `Tree.AddTicker`；静止且无 autoplay 不挂无 Ticker。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Carousel 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例（CRS-01…17）测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 本阶段不强制（CRS-18 deferred）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：Carousel 页覆盖 **§6.8 P0** 官方非 debug 六例。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/carousel.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Carousel 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。

> **§6 修订说明（相对模板稿）**：补强 **§6.2** 组件 Token（dots/arrows 源码表）；**§6.3** 补 infinite 默认与纵向衍生；**§6.4** 增 drag/disabled/边界规则；**§6.6** 具体 a11y；**§6.8** 明确 autoplay.dotDuration 与 infinite；**§6.10** 落地 Go 契约；**§6.11** 改为 Carousel 分层；**§6.9/6.12** 标明 L3/L4 本阶段不强制。
