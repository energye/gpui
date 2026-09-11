# BorderBeam 边框流光
> 来源：[Ant Design 6.5.x BorderBeam](https://ant.design/components/border-beam)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：其他（Other）  
> 说明：为容器边框提供持续流动的装饰性高亮效果。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

为容器边框提供持续流动的装饰性高亮效果。

**BorderBeam** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基础用法 | 复现「基础用法」视觉与布局 |
| 鼠标悬浮时显示 | 复现「鼠标悬浮时显示」视觉与布局 |
| 自定义容器 | 自定义渲染/插槽外观 |
| 渐变色 | 渐变填充 |
| 动画时长 | 复现「动画时长」视觉与布局 |
| 尺寸 | 不同 size 档位的高宽/字号/内边距 |
| 线宽 | 复现「线宽」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `color`

- **说明**：流光颜色配置，支持单色字符串或渐变停靠点数组。`percent` 使用 `0 ~ 100` 的输入区间，组件会在内部为尾部透明过渡预留空间
- **类型**：`string | { color: string; percent: number }[]`
- **默认值**：-
- **版本**：6.4.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `percent` | 官方取值 `percent` |

#### `duration`

- **说明**：流光完成一圈动画的时间，单位秒
- **类型**：number
- **默认值**：6
- **版本**：6.5.0

#### `lineWidth`

- **说明**：流光线宽，数字类型按像素处理
- **类型**：`number | string`
- **默认值**：`1px`
- **版本**：6.5.0

#### `outset`

- **说明**：流光层相对容器边缘的外扩距离，遇到裁剪容器时可设为 `0`
- **类型**：`number | string`
- **默认值**：-
- **版本**：6.4.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `0` | 官方取值 `0` |

#### `size`

- **说明**：流光可见段的尺寸，数字类型按像素处理
- **类型**：`number | string`
- **默认值**：100
- **版本**：6.5.0

### 1.4 交互视觉状态（实现检查表）

BorderBeam 为装饰层（`pointer-events: none`、`aria-hidden`，源码 `BorderBeamEffect`），不抢焦点不参与命中：hover / active / focus ring / disabled / loading / error-warning 皮均标 **N/A**，焦点与键盘由被包裹的 children 自理。只验 beam 显示/隐藏与 reduced-motion 隐藏（见 §6.4）。

### 1.5 语义化 DOM 与主题

- 至少区分根容器、内容区、装饰/图标区；浮层再分 popup/mask。

- 颜色、圆角、间距、动效走 Design Token；支持亮暗色与品牌色。

- 动效可关（reduced-motion / 全局 motion、wave 配置）。
---
## 2. 功能
### 2.1 使用场景

- 需要强化某个容器的视觉关注度，但又不希望引入业务状态语义时。
- 适合登录面板、推荐卡片、AI 模块、重点 CTA 区域等场景。
- 它是装饰性效果，不应替代焦点态、校验态或业务状态边框。

### 2.2 核心功能（按官方示例拆解）

1. **基础用法**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **鼠标悬浮时显示**（`hover.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **自定义容器**（`custom-container.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **渐变色**（`customized-color.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **动画时长**（`duration.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **尺寸**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **线宽**（`line-width.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基础用法 | `basic.tsx` | 否 |
| 鼠标悬浮时显示 | `hover.tsx` | 否 |
| 自定义容器 | `custom-container.tsx` | 否 |
| 渐变色 | `customized-color.tsx` | 否 |
| 动画时长 | `duration.tsx` | 否 |
| 尺寸 | `size.tsx` | 否 |
| 线宽 | `line-width.tsx` | 否 |
| 不规则圆角 | `non-uniform-radius.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |

### 2.6 FAQ

## FAQ

### 开启减少动态效果后会怎样？ {#faq-reduced-motion}

`BorderBeam` 会将流光视为装饰效果。当命中 `prefers-reduced-motion: reduce` 时，组件会隐藏 beam 效果。

### `color` 中的 `percent` 表示什么？ {#faq-color-percent}

`percent` 表示渐变停靠点的输入位置，取值范围为 `0 ~ 100`。组件会将这些停靠点映射到可见 beam 段内，并为尾部透明过渡保留空间，以保持流光尾迹连续可见。

### 为什么 `BorderBeam` 没有效果？ {#faq-not-working}

`BorderBeam` 需要通过 `children` 获取实际 DOM 节点，并将流光层插入到该节点中。请确保被包裹的内容是原生 DOM 元素，或是正确透传 `ref` 到 DOM 的 React 组件，否则组件无法定位真实容器，也就无法渲染流光效果。

流光层使用 `position: absolute` 定位，因此被索引到的 DOM 节点还需要提供定位上下文，通常可以为它设置 `position: relative`。`BorderBeam` 不会主动检测或修正子节点的定位样式。

为保证性能，`children` 是否可以插入以及其定位信息会在初始化时判断，后续不会持续监听子节点结构或定位样式变化。

### 如何让流光边框跟随容器圆角？ {#faq-radius}

`BorderBeam` 会在初始化时读取实际容器的计算后 `border-radius`。这个能力更适合 `Card` 这类单容器子节点场景；若子节点结构较复杂，建议直接把圆角写在实际容器根节点上，以获得更稳定的结果。

为保证性能，圆角计算完成后不会持续重新测量。后续由尺寸、祖先样式或子节点内部状态引起的圆角变化，不保证自动重新同步。动画轨迹在运行时可能会做内部平滑处理。

例如：

```tsx
const radius = 24;

  
;
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

### BorderBeam

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| children | 装饰内容 | `ReactNode` | - | 6.4.0 | × |
| color | 流光颜色配置，支持单色字符串或渐变停靠点数组。`percent` 使用 `0 ~ 100` 的输入区间，组件会在内部为尾部透明过渡预留空间 | `string \| { color: string; percent: number }[]` | - | 6.4.0 | × |
| duration | 流光完成一圈动画的时间，单位秒 | number | 6 | 6.5.0 | × |
| lineWidth | 流光线宽，数字类型按像素处理 | `number \| string` | `1px` | 6.5.0 | × |
| outset | 流光层相对容器边缘的外扩距离，遇到裁剪容器时可设为 `0` | `number \| string` | - | 6.4.0 | × |
| size | 流光可见段的尺寸，数字类型按像素处理 | `number \| string` | 100 | 6.5.0 | × |

### 导入方式

```js
import { BorderBeam } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `children` | 装饰内容 | `ReactNode` | - | 6.4.0 |
| `color` | 流光颜色配置，支持单色字符串或渐变停靠点数组。`percent` 使用 `0 ~ 100` 的输入区间，组件会在内部为尾部透明过渡预留空间 | `string \| { color: string; percent: number }[]` | - | 6.4.0 |
| `duration` | 流光完成一圈动画的时间，单位秒 | number | 6 | 6.5.0 |
| `lineWidth` | 流光线宽，数字类型按像素处理 | `number \| string` | `1px` | 6.5.0 |
| `outset` | 流光层相对容器边缘的外扩距离，遇到裁剪容器时可设为 `0` | `number \| string` | - | 6.4.0 |
| `size` | 流光可见段的尺寸，数字类型按像素处理 | `number \| string` | 100 | 6.5.0 |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **BorderBeam** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **7** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/border-beam
- 中文文档：https://ant.design/components/border-beam-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/border-beam
- 驱动 gpui kit：`border-beam`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **BorderBeam** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/border-beam/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（BorderBeam）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 展示形态与可选交互（复制/预览/关闭） | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（BorderBeam）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：为容器边框提供持续流动的装饰性高亮效果。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。  
源码：`components/border-beam/style/index.ts`、`util.ts`（`DEFAULT_BORDER_BEAM_DURATION=6`、`MAX_BEAM_COLOR_STOP_PERCENT=70`）。

> **注意：** antd `size` 是 **流光可见段长度（px）**，不是 Button 的 small/middle/large 档。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 流光段长 `size` | **100** px | props；CSS var `--ant-border-beam-size` |
| 线宽 `lineWidth` | **1** px | `lineWidth` / props |
| 一圈时长 `duration` | **6** s | props；CSS var duration |
| 渐变色有效映射上限 | **70%** | `MAX_BEAM_COLOR_STOP_PERCENT`：用户 0–100 映射到可见段前 70%，尾部 30% 留给透明淡出 |
| 容器圆角（跟随 children） | 读容器 / 回落 **6** | `borderRadius`；Card 示例常用 **8**（`borderRadiusLG`） |
| 字号（内容区） | **14** | `fontSize`（仅 children 文本，非 beam 本体） |
| Focus ring outset | **不适用** | BorderBeam 装饰层 `pointer-events: none`，不抢焦点 |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 默认流光渐变头 | `colorPrimary` | 未设 `color` 时 |
| 默认流光渐变中 | `colorPrimaryHover` | 映射到约 70% 后接透明 |
| 容器底 / 边 | `colorBgContainer` / `colorBorder` | 仅 children 容器皮，非 beam |
| 文本 | `colorText` / `colorTextSecondary` | children 文案 |

禁止硬编码品牌色作为唯一默认皮（须经 Theme Token；测试可断言读到 primary）。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**其他（Other）**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `children` | 被装饰的容器内容；kit 侧为 `core.Node` | Node | - |
| `color` | 流光色：单色或 `{color, percent}[]`（percent 输入 0–100） | 色 / 停靠点数组 | Theme primary 渐变 |
| `duration` | 流光完成一圈的时间（秒） | number | **6** |
| `lineWidth` | 流光线宽（px） | number | **1** |
| `outset` | 流光层相对容器边缘的外扩（px）；裁剪容器可设 **0** | number | -（未设；kit 未设时回落 **0** 贴边，属 kit 扩展） |
| `size` | **流光可见段长度**（px），非控件 size 档 | number | **100** |

**桌面映射（非 antd props，但官方示例需要）：**

| 能力 | 说明 |
| --- | --- |
| `showOnHover` | 映射 `hover.tsx`：默认隐藏 beam，指针进入容器后显示并运行 |
| `borderRadius` | 映射 FAQ：跟随容器圆角；可显式 Set |

**配置优先级：** 显式 SetXxx > 组件默认 > Theme Token 回落。无受控 value。

### 6.4 交互状态机（L1）

```text
mount ──► running（Tick 推进 phase 0→1 循环）
             │
             ├── SetDuration / SetSize / SetColor / SetLineWidth / SetOutset ──► 下帧生效
             ├── showOnHover=true 且未 hover ──► beam 隐藏（phase 可冻结）
             ├── showOnHover=true 且 hover ──► beam 显示 + running
             └── ReduceMotion=true ──► beam 隐藏（antd：::before display:none）
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| BB-S1 | 默认（非 reduced-motion） | beam 可见；Ticker 推进 phase |
| BB-S2 | `Clock.ReduceMotion=true` | **隐藏** beam（非仅静止可见）；Ticker 可停 |
| BB-S3 | `SetDuration(d)` d>0 | 角速度 = 1/d 圈/秒；d 变小则变快 |
| BB-S4 | `SetColor` / 渐变 stops | 解析色变；默认走 primary Token |
| BB-S5 | children 非空 | 内容节点在树中且可布局/可见 |
| BB-S6 | `SetShowOnHover(true)` | 未 hover 时 beam 不可见；hover 后可见 |
| BB-S7 | `SetSize` / `SetLineWidth` | 段长 / 线宽立即参与绘制 |
| BB-S8 | beam 层 | `HitTransparent`；不抢 children 点击 |

### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 圆角矩形路径上流光；默认 primary→primaryHover→透明 |
| reduced-motion | **不绘制** beam |
| showOnHover 未悬停 | **不绘制** beam |
| showOnHover 悬停 | 绘制 + 动画 |
| 主题切换 | 默认色随 Theme 更新 |
| children 容器 | 业务自备边框/底；BorderBeam 不替代 focus/校验边框 |

**动效：** 线性循环（antd `animation-timing-function: linear`）；P0 用沿周长的短段描边近似 CSS `offset-path`，不要求 mask-composite 像素级。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | 装饰层 `presentation` / `aria-hidden` 等价（antd beam `aria-hidden="true"`） |
| 交互 | beam **不**参与命中；焦点与键盘留给 children |
| 名称 | 无强制业务名；可选 `SetAriaLabel` 挂在根（少用） |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| children 包裹 + 配置字段 | **对等** | P0 L1 |
| duration / size / lineWidth / color / outset | **对等** | P0 L1+L2 |
| 沿边循环动画 | **近似**（Tick + 路径采样，非 CSS offset-path） | P0 行为 / P1 像素级 |
| reduced-motion 隐藏 | **对等** | P0 L1 |
| 插入真实 DOM / portal 进 children | **映射**：kit 叠 beam 层于 host 内 | P0 |
| 读 computed border-radius 持续监听 | **映射**：显式 `SetBorderRadius` + 默认 Token | P0 近似 |
| `mask-composite` + `offset-path` 环形轨迹 | 源码真实存在（`style/index.ts` `@supports`）：P0 用沿周长短段描边近似，像素级对齐为 P1 | P0 近似 / P1 像素级 |
| 官网逐像素哈希 | **不做** | — |
| semantic classNames/styles | kit Style 钩子 | P1 |
| ConfigProvider `borderBeam` 全局 | 随 ConfigProvider | P1 |
| debug 示例（non-uniform-radius / component-token） | 分期 | P1 |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `children` | 包裹内容节点 |
| `color` | 单色 + 渐变 stops（percent 0–100 → 映射到 70% 可见段） |
| `duration` | 默认 6s |
| `lineWidth` | 默认 1px |
| `outset` | 可选；未设回落 0（布局盒内贴边） |
| `size` | 流光段长，默认 100px |
| `showOnHover` | 复现 hover.tsx |
| `borderRadius` | 跟随 / 显式 |
| reduced-motion | 隐藏 beam |
| Ticker | 仅 beam 可见且需动画时挂载 |
| 官方主路径示例 | basic / hover / custom-container / customized-color / duration / size / line-width |
| 度量 §6.2 | Token / 默认数字断言 |
| a11y §6.6 | 装饰层不抢 hit |
| §6.9 中 L1/L2 非 P1 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| semantic classNames/styles 深度 | 分期 |
| CSS offset-path / mask-composite 像素级 | 分期 |
| 自动测量 children 运行时 border-radius 变化 | 分期 |
| ConfigProvider 全局 `borderBeam` | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |
| non-uniform 四角圆角 | 分期 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestBorderBeam_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 / L3 / L4 标记，且适用者）全部通过** 才可宣称 BorderBeam 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| BB-01 | L1 | `NewBorderBeam(nil)` 默认 | 不崩溃；duration=6、size=100、lineWidth=1；Node 可 Layout |
| BB-02 | L1 | 默认 running | `IsBeamVisible()`；`Tick` 推进 `Phase()` |
| BB-03 | L1 | Tree `ReduceMotion=true` | beam **隐藏**；Tick 不推进 / 返回 idle |
| BB-04 | L1 | `SetDuration(3)` vs `12` | 相同 dt 下 phase 增量反比于 duration |
| BB-05 | L1 | `SetColor` / `SetColorStops` | 解析色变；stops 生效 |
| BB-06 | L1 | children 文本/节点 | 内容在树中可找到 |
| BB-07 | L1 | 复现 `basic.tsx` | Card/容器 + 默认 BorderBeam；有 beam 层 |
| BB-08 | L1 | 复现 `hover.tsx` | `ShowOnHover`：未 hover 不可见，hover 后可见 |
| BB-09 | L1 | 复现 `custom-container.tsx` | 自定义容器 children；radius≈8 可设 |
| BB-10 | L1 | 复现 `customized-color.tsx` | 多 stops 渐变可切换 |
| BB-11 | L1 | 复现 `duration.tsx` | 3 / 6 / 12 三档 duration |
| BB-12 | L1 | 复现 `size.tsx` | 默认 100 / 56 / 160 |
| BB-13 | L1 | 复现 `line-width.tsx` | lineWidth=2 |
| BB-14 | L2 | 读 §6.2 默认数字 | duration/size/lineWidth/fontSize/radius 容差 ±0.5 |
| BB-15 | L2 | 默认皮颜色 | 默认 stops 来自 Theme primary（非写死唯一皮） |
| BB-16 | L2 | disabled（不适用） | N/A — 跳过 |
| BB-17 | L1 | 键盘/焦点（不适用） | N/A — beam 不聚焦；children 自理 |
| BB-18 | L3 | 关键态 golden | 可选；有则与基线 AA 容差 |
| BB-19 | L4 | 与 ant.design 并排 | 人眼签字 |
| BB-20 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 契约。

```text
NewBorderBeam(child core.Node) *BorderBeam

// 内容
SetChild(core.Node)

// 流光配置（antd props）
SetColor(render.RGBA)                         // 单色
SetColorStops(...BorderBeamColorStop)         // {Color, Percent 0..100}
ClearColor()                                  // 回落 Theme 默认渐变
SetDuration(seconds float64)                  // ≤0 → 默认 6
SetLineWidth(px float64)                      // ≤0 → 默认 1
SetOutset(px float64)                         // 显式含 0
ClearOutset()                                 // 未设
SetSize(px float64)                           // 流光段长；≤0 → 默认 100
SetBorderRadius(px float64)                   // 路径圆角

// 桌面映射
SetShowOnHover(bool)                          // hover.tsx
SetHovered(bool)                              // 测试 / 宿主注入；或 host 实现 hoverable

// 主题 / 覆盖
SetTheme(*core.Theme)
SetStyle(Style)
SetAriaLabel(string)

// 查询（测试 / 宿主）
Node() core.Node
Child() core.Node
Phase() float64                               // 0..1
IsBeamVisible() bool
ResolvedDuration() / ResolvedSize() / ResolvedLineWidth() / ResolvedOutset() / ResolvedBorderRadius()
ResolvedColorStops() []BorderBeamColorStop

// 动画
AttachTicker(*core.Tree)
Tick(dt float64) bool
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Duration | **6** |
| Size（段长） | **100** |
| LineWidth | **1** |
| Outset | -（未设；kit 回落 **0** 贴容器边，属 kit 扩展；antd 未设时跟子容器边框宽） |
| Color | Theme `colorPrimary` → `colorPrimaryHover` → 透明 |
| BorderRadius | Token `borderRadius`（**6**） |
| ShowOnHover | false（始终尝试显示 beam） |
| Disabled / Loading | 不适用 |

### 6.11 结构与绘制分层（实现提示）

```text
borderBeamHost（HitDefer · layout = children 盒）
  ├─ child（业务容器；可点）
  └─ beamLayer（HitTransparent · aria-hidden/presentation）
       └─ 沿圆角矩形周长采样短段，按 phase 与 size 着色描边
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读 Default / 字段 / Token。  
- `hit == layout == paint`：host 盒 = children 布局盒；beam 不扩大命中。  
- 动画跟随 Host Tick；`ReduceMotion` 时不挂有效动画且不绘制 beam。  
- outset>0 时允许绘制略超出 host 盒（父级裁剪由业务决定）。

### 6.12 完成定义（DoD）

同时满足即可宣布 **BorderBeam 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2 适用用例（BB-01–15）** 测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 可选（装饰控件；有则覆盖 1 关键可见态）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：BorderBeam 页覆盖 **§6.8 P0** 官方非 debug 七例。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/border-beam.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` BorderBeam 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。

**§6 修订说明（相对模板薄稿）：** 6.2 改为流光专用度量（去掉错误的 focus-ring/禁用色表）；6.3–6.4 补 `showOnHover` 与 reduced-motion **隐藏**；6.8 P0 列全 color/duration/lineWidth/outset/size；6.9 明确 BB-16/17 N/A；6.10 写成具体 Go API；6.11 改为 host+beamLayer。
