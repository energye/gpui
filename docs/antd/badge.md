# Badge 徽标数
> 来源：[Ant Design 6.5.x Badge](https://ant.design/components/badge)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：图标右上角的圆形徽标数字。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

图标右上角的圆形徽标数字。

**Badge** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 独立使用 | 复现「独立使用」视觉与布局 |
| 封顶数字 | 复现「封顶数字」视觉与布局 |
| 讨嫌的小红点 | 复现「讨嫌的小红点」视觉与布局 |
| 动态 | 复现「动态」视觉与布局 |
| 可点击 | 复现「可点击」视觉与布局 |
| 自定义位置偏移 | placement 方位 |
| 大小 | 不同 size 档位 |
| 状态点 | 复现「状态点」视觉与布局 |
| 多彩徽标 | Badge 叠加 |
| 缎带 | 复现「缎带」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `color`

- **说明**：自定义小圆点的颜色
- **类型**：string
- **默认值**：-

#### `count`

- **说明**：展示的数字，大于 overflowCount 时显示为 `${overflowCount}+`，为 0 时隐藏
- **类型**：ReactNode
- **默认值**：-

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `dot`

- **说明**：不展示数字，只有一个小红点
- **类型**：boolean
- **默认值**：false

#### `offset`

- **说明**：设置状态点的位置偏移
- **类型**：\[number, number]
- **默认值**：-

#### `overflowCount`

- **说明**：展示封顶的数字值
- **类型**：number
- **默认值**：99

#### `showZero`

- **说明**：当数值为 0 时，是否展示 Badge
- **类型**：boolean
- **默认值**：false

#### `size`

- **说明**：在设置了 `count` 的前提下有效，设置小圆点的大小
- **类型**：`medium` | `small`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `status`

- **说明**：设置 Badge 为状态点
- **类型**：`success` | `processing` | `default` | `error` | `warning`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `success` | 成功绿语义 |
  | `processing` | 进行中 |
  | `default` | 默认中性外观 |
  | `error` | 错误红语义 |
  | `warning` | 警告橙语义 |

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `text`

- **说明**：在设置了 `status` 的前提下有效，设置状态点的文本
- **类型**：ReactNode
- **默认值**：-

#### `title`

- **说明**：设置鼠标放在状态点上时显示的文字。设置为 `null` 或 `false` 时移除原生 tooltip
- **类型**：string | null | false
- **默认值**：-
- **版本**：6.5.0

#### `placement`

- **说明**：缎带的位置，`start` 和 `end` 随文字方向（RTL 或 LTR）变动
- **类型**：`start` | `end`
- **默认值**：`end`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `start` | 逻辑起始侧 |
  | `end` | 逻辑结束侧 |

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

一般出现在通知图标或头像的右上角，用于显示需要处理的消息条数，通过醒目视觉形式吸引用户处理。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **独立使用**（`no-wrapper.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **封顶数字**（`overflow.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **讨嫌的小红点**（`dot.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **动态**（`change.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **可点击**（`link.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **自定义位置偏移**（`offset.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **状态点**（`status.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **多彩徽标**（`colorful.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **缎带**（`ribbon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 独立使用 | `no-wrapper.tsx` | 否 |
| 封顶数字 | `overflow.tsx` | 否 |
| 讨嫌的小红点 | `dot.tsx` | 否 |
| 动态 | `change.tsx` | 否 |
| 可点击 | `link.tsx` | 否 |
| 自定义位置偏移 | `offset.tsx` | 否 |
| 大小 | `size.tsx` | 否 |
| 状态点 | `status.tsx` | 否 |
| 多彩徽标 | `colorful.tsx` | 否 |
| 缎带 | `ribbon.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| Ribbon Debug | `ribbon-debug.tsx` | 是 |
| 各种混用的情况 | `mix.tsx` | 是 |
| 自定义标题 | `title.tsx` | 是 |
| 多彩徽标支持 count 显示 Debug | `colorful-with-count-debug.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |

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

### Badge

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| color | 自定义小圆点的颜色 | string | - | count | 展示的数字，大于 overflowCount 时显示为 `${overflowCount}+`，为 0 时隐藏 | ReactNode | - | classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | dot | 不展示数字，只有一个小红点 | boolean | false | offset | 设置状态点的位置偏移 | \[number, number] | - | overflowCount | 展示封顶的数字值 | number | 99 | showZero | 当数值为 0 时，是否展示 Badge | boolean | false | size | 在设置了 `count` 的前提下有效，设置小圆点的大小 | `medium` \| `small` | - | - | × |
| status | 设置 Badge 为状态点 | `success` \| `processing` \| `default` \| `error` \| `warning` | - | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | text | 在设置了 `status` 的前提下有效，设置状态点的文本 | ReactNode | - | title | 设置鼠标放在状态点上时显示的文字。设置为 `null` 或 `false` 时移除原生 tooltip | string \| null \| false | - | 6.5.0 | × |

### Badge.Ribbon

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | color | 自定义缎带的颜色 | string | - | placement | 缎带的位置，`start` 和 `end` 随文字方向（RTL 或 LTR）变动 | `start` \| `end` | `end` | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | text | 缎带中填入的内容 | ReactNode | - 
### 导入方式

```js
import { Badge } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `color` | 自定义小圆点的颜色 | string | - | — |
| `count` | 展示的数字，大于 overflowCount 时显示为 `${overflowCount}+`，为 0 时隐藏 | ReactNode | - | — |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `dot` | 不展示数字，只有一个小红点 | boolean | false | — |
| `offset` | 设置状态点的位置偏移 | \[number, number] | - | — |
| `overflowCount` | 展示封顶的数字值 | number | 99 | — |
| `showZero` | 当数值为 0 时，是否展示 Badge | boolean | false | — |
| `size` | 在设置了 `count` 的前提下有效，设置小圆点的大小 | `medium` \| `small` | - | - |
| `status` | 设置 Badge 为状态点 | `success` \| `processing` \| `default` \| `error` \| `warning` | - | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `text` | 在设置了 `status` 的前提下有效，设置状态点的文本 | ReactNode | - | — |
| `title` | 设置鼠标放在状态点上时显示的文字。设置为 `null` 或 `false` 时移除原生 tooltip | string \| null \| false | - | 6.5.0 |
| `placement` | 缎带的位置，`start` 和 `end` 随文字方向（RTL 或 LTR）变动 | `start` \| `end` | `end` | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Badge** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **12** 个，均需可复现。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/badge
- 中文文档：https://ant.design/components/badge-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/badge
- 驱动 gpui kit：`badge`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Badge** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/badge/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Badge）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 展示形态与可选交互（复制/预览/关闭） | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Badge）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：图标右上角的圆形徽标数字。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，种子：`fontSize=14`、`fontSizeSM=12`、`lineHeight≈1.5714`、`lineWidth=1`、`paddingXS(antd)=8`）。  
组件 Token 来自 `components/badge/style` 的 `prepareComponentToken` / `prepareToken`；实现以常量回落 + Theme 色为准（禁止把品牌红写死为唯一默认皮）。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| `overflowCount` | **99** | API 默认 |
| `indicatorHeight`（count medium） | **20** | `round(fontSize×lineHeight) − 2×lineWidth` |
| `indicatorHeightSM`（count small） | **14** | `fontSize` |
| `dotSize` | **6** | `fontSizeSM / 2` |
| `statusSize` | **6** | `fontSizeSM / 2` |
| `textFontSize` / `textFontSizeSM` | **12** | `fontSizeSM` |
| `paddingInline`（多字符 count 水平内边距） | **8** | antd `paddingXS`（本库常量；非 `TokenPaddingXS=4`） |
| count 描边（叠在 children 上） | **1** | `lineWidth`；色 `colorBgContainer`（badgeShadowColor） |
| count 圆角 | **height/2**（胶囊） | 由高度推导 |
| 徽标默认锚点 | 子角点 + `translate(50%, -50%)` | 半出右上角；`offset=[x,y]` → 再 `+x` / `+y` |
| Focus ring outset（可点击时） | ≈ **1.5px** 可见 | Pressable 默认 |
| Ribbon 默认 `placement` | **end** | API |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token / 来源 | 备注 |
| --- | --- | --- |
| count / 默认红点底 | `colorError` | `badgeColor` |
| count 字色 | `colorTextInverse`（≈ `colorTextLightSolid`） | `badgeTextColor` |
| status success / error / warning | `colorSuccess` / `colorError` / `colorWarning` | |
| status default | `colorTextQuaternary`（中性点） | antd 中性点 |
| status processing | `colorPrimary` + 脉冲环（Ticker） | 非静态 |
| 自定义 `color` | 解析后的 RGBA / 预设映射 | 覆盖 count/status 点色 |
| 禁用（适用者） | `colorDisabledText` 降对比 | 无 hover 高亮 |
| Ribbon 默认底 | `colorPrimary` | 可 `color` 覆盖 |

禁止硬编码 `#FF4D4F` 等品牌色作为**唯一**默认皮（Theme 可改 `colorError` 必须生效）。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据展示**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `count` | 展示数字；`> overflowCount` → `` `${overflowCount}+` ``；`0` 且 `showZero=false` 隐藏 | int / 自定义 Node | — |
| `showZero` | `count=0` 时是否仍展示 | bool | false |
| `overflowCount` | 封顶阈值 | int | 99 |
| `dot` | 不展示数字，只显示小红点 | bool | false |
| `offset` | 徽标相对默认锚点的 `[x, y]` 偏移 | `[number, number]` | — |
| `size` | 仅 count 有效：`medium` \| `small` | enum | medium |
| `status` | 状态点：`success` \| `processing` \| `default` \| `error` \| `warning` | enum | — |
| `text` | 状态点旁文案（`status`/`color` 独立使用时） | string | — |
| `color` | 自定义点/count 底色（hex 或解析色） | color | — |
| `title` | 悬停/可访问名；`null`/`false` 清除（不回落到 count） | string \| none | 回落 count 字符串 |
| `placement` | **Ribbon** 位置：`start` \| `end` | enum | end |
| children | 被包裹节点；无 children 为独立使用（`not-a-wrapper`） | Node | — |
| onClick | 可点击（官方 link 示例：整枚可点） | callback | — |

**配置优先级（通用）：** 显式 Set > 组件默认 > ConfigProvider 全局默认（ConfigProvider 为 P1）。

**显示优先级（antd）：**

1. `dot=true` 且非「零且隐藏」→ 红点（忽略数字文案）。  
2. 否则若有可渲染 `count` / `CountNode` → count 胶囊。  
3. 否则若 `status` 或 `color` 且无有效 count → 状态点（可带 `text`）。  
4. 隐藏：无 count/dot/status/color 可展示，或 `count=0 && !showZero && !dot`。

### 6.4 交互状态机（L1）

```text
children?
  yes → Stack：child + 角标（count|dot|status-dot）
  no  → 独立：count 胶囊 | status 点(+text) | 隐藏

count > overflowCount ──► `${overflowCount}+`
showZero=false ∧ count=0 ∧ !dot ──► 隐藏角标
status=processing ──► 主色点 + Ticker 脉冲环
offset ──► 默认半出右上角后再平移
Ribbon.placement ──► start|end 缎带
OnClick ──► Pressable；Disabled 时不触发
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| BDG-S1 | count=5 | 显示 `5` |
| BDG-S2 | count=100 overflow=99 | `99+` |
| BDG-S3 | dot | 红点可见 |
| BDG-S4 | showZero=false count=0 | 角标隐藏 |
| BDG-S5 | showZero=true count=0 | 显示 `0` |
| BDG-S6 | status=success/error/… | 状态点色走 Token |
| BDG-S7 | offset=[x,y] | 角标相对默认锚点平移 |
| BDG-S8 | Ribbon placement | 缎带在 start/end |
| BDG-S9 | status=processing | Ticker 脉冲进行中 |
| BDG-S10 | SetOnClick + 点击 | 触发回调；Disabled 不触发 |

### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | §6.2 Token；count 胶囊白字 + error 底；描边 `colorBgContainer` |
| hover/active/focus | 仅可点击（OnClick）时 Pressable 反馈 + focus ring |
| disabled | 角标/缎带降对比；点击无效 |
| processing | 脉冲环（Ticker）；P0 固定周期即可 |
| 主题切换 | 色随 Theme 更新 |

**动效：** count 进出场 / ScrollNumber 翻滚为 P1；P0 瞬时切换。processing 脉冲为 P0（可 reduced-motion 关）。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 装饰角标 | 默认可 `aria-hidden` 等价（Decorative）；有意义 count 可用 `title` / `SetAriaLabel` |
| `title` | 作为悬停名与可访问名回落；显式 none 则不回落 count |
| 可点击 | 角色 button 等价；Space/Enter 激活；Focus ring 可见 |
| 状态点 + text | text 即为可读名 |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 主路径行为（§6.1 L1） | **对等** | P0 L1 |
| 尺寸/色 Token（§6.2） | **对等** | P0 L2 |
| count 进出场 / ScrollNumber 翻滚 | **瞬时** | P1 |
| processing 脉冲 | **对等**（Ticker） | P0 |
| 预设色名（pink/red/…）全表 | **近似**：P0 支持 hex/RGBA；具名预设可映射常用项 | P0 hex / P1 全预设 |
| Semantic classNames/styles | kit 语义钩子 | P1 |
| ConfigProvider 全局默认 | 随 ConfigProvider | P1 |
| 原生 HTML title tooltip | **映射**为 Title 字段 + 可选 kit.Tooltip | P0 字段 / 悬停气泡可简化 |
| 逐像素官网哈希 | **不做** | — |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `count` / `showZero` / `overflowCount` | 数字与封顶 |
| `dot` | 小红点 |
| `offset` | 角标偏移 |
| `size` medium \| small | count 高度档 |
| `status` + `text` | 状态点 API 与色（含 processing Ticker） |
| `color`（自定义色） | count/点底色；Style.Background 可覆盖 |
| `title` | 字段 + 回落规则 |
| `CountNode` | 自定义 count 内容（basic 时钟图标） |
| children / 独立使用 | wrapper 与 not-a-wrapper |
| OnClick / Disabled / a11y | 可点击主路径 |
| **Badge.Ribbon** + `placement` + `text` + `color` | 缎带 API（BDG-09） |
| 官方主路径示例（gallery） | 基本、独立使用、封顶数字、讨嫌的小红点、动态、可点击、自定义位置偏移、大小 |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | 最低要求 |
| §6.9 中 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| 官方示例「状态点 / 多彩徽标 / 缎带」完整 gallery 铺陈 | API 已在 P0；页面级示例可 later |
| 预设色名全表与 colorful 深度 | 分期 |
| semantic classNames/styles | 分期 |
| ScrollNumber 翻滚 / zoom 入场动画像素级 | 分期 |
| ConfigProvider 全局 badge 默认 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestBadge_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Badge 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| BDG-01 | L1 | NewBadge 默认创建 | 不崩溃；overflow=99；size=medium；非 dot；非 disabled |
| BDG-02 | L1 | count=5 | DisplayCount=`5`；Visible |
| BDG-03 | L1 | count=100 overflow=99 | DisplayCount=`99+` |
| BDG-04 | L1 | dot | IsDot 可见；非数字胶囊 |
| BDG-05 | L1 | showZero=false count=0 | !Visible |
| BDG-06 | L1 | showZero=true count=0 | DisplayCount=`0`；Visible |
| BDG-07 | L1 | status=success/error/warning/default/processing | 状态点色走对应 Token；processing 可 Tick |
| BDG-08 | L1 | offset=[10,10] | 角标 Offset 相对默认锚点含 +10,+10 |
| BDG-09 | L1 | NewRibbon + placement | 缎带可布局；start/end 可切换 |
| BDG-10 | L1 | 复现「基本」（`basic.tsx`） | count=5 / showZero0 / CountNode 可布局 |
| BDG-11 | L1 | 复现「独立使用」（`no-wrapper.tsx`） | 无 child 的 count + color 可布局 |
| BDG-12 | L1 | 复现「封顶数字」（`overflow.tsx`） | 99 / 99+ / 10+ / 999+ |
| BDG-13 | L1 | 复现「讨嫌的小红点」（`dot.tsx`） | dot 包 icon/text 可布局 |
| BDG-14 | L1 | 复现「动态」（`change.tsx`） | SetCount / SetDot 动态切换 |
| BDG-15 | L1 | 复现「可点击」（`link.tsx`） | OnClick 触发 |
| BDG-16 | L1 | 复现「自定义位置偏移」（`offset.tsx`） | offset 生效 |
| BDG-17 | L1 | 复现「大小」（`size.tsx`） | medium 高 20 / small 高 14（±0.5） |
| BDG-18 | L2 | 读取 §6.2 关键尺寸 | indicator 20/14；dot/status 6；字号 12；paddingInline 8 |
| BDG-19 | L2 | 默认皮颜色 | count 底=TokenColorError；字=TextInverse；改 Theme 生效 |
| BDG-20 | L2 | disabled 外观（适用者） | 降对比；OnClick 不触发 |
| BDG-21 | L1 | 键盘/焦点（可点击时） | Space/Enter 触发；Focus ring 可开 |
| BDG-22 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| BDG-23 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| BDG-24 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |

> PRD 测试覆盖 **BDG-01…BDG-21**（L1/L2 P0）。L3/L4/P1 不进 `TestBadge_PRD_*` 门禁。

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 契约。

```text
// ── Badge ──
NewBadge() *Badge

type BadgeSize int     // BadgeMedium | BadgeSmall
type BadgeStatus int   // BadgeStatusNone | Success | Processing | Default | Error | Warning

// 配置
SetChild(n core.Node)              // children；nil = 独立使用
SetCount(n int)                    // 数字 count
SetCountNode(n core.Node)          // 自定义 count（优先于数字文案）
SetShowZero(v bool)
SetOverflowCount(n int)            // ≤0 → 99
SetDot(v bool)
SetOffset(x, y float64)
SetSize(BadgeSize)                 // medium | small
SetStatus(BadgeStatus)
SetText(s string)                  // status 旁文案
SetColor(hexOrEmpty string)        // 空清除；支持 #RGB/#RRGGBB
SetColorRGBA(c render.RGBA)
SetTitle(s string)                 // 悬停/可访问名
SetTitleNone()                     // title=null|false：不回落 count
SetOnClick(fn func())
SetDisabled(v bool)
SetTheme(*core.Theme)
SetStyle(Style)                    // Background 可覆盖 count 底
SetFace(text.Face)
SetAriaLabel(s string)

// 查询（测试 / 宿主）
DisplayCount() string              // "5" | "99+" | "0" | ""
Visible() bool
IsDot() bool
IndicatorHeight() float64
StatusColor() render.RGBA
ResolvedTitle() string
Node() / ChromeNode() / IndicatorNode() core.Node
AttachTicker(*core.Tree)           // processing 脉冲
Tick(dt float64) bool

// ── Badge.Ribbon ──
NewRibbon(text string) *Ribbon
SetChild(n core.Node)
SetText(s string)
SetColor(hex string) / SetColorRGBA(c)
SetPlacement(RibbonStart | RibbonEnd)
SetTheme / SetFace / SetStyle / Node()
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| OverflowCount | 99 |
| Size | BadgeMedium |
| Dot / ShowZero / Disabled | false |
| Status | BadgeStatusNone |
| Ribbon.Placement | RibbonEnd |
| Title | 回落 count 显示串；`SetTitleNone` 后为空 |
| 其余 | 对齐 antd 6.5 §3 |

### 6.11 结构与绘制分层（实现提示）

```text
Badge root (Stack；可外包 Pressable)
  ├─ child?                         // wrapper
  ├─ indicator host (半出右上 + offset)
  │    ├─ count 胶囊 (Decorated + Text | CountNode)
  │    ├─ dot Box
  │    └─ status-dot (+ processing 环 Canvas)
  └─ status text?                   // 独立 status 行

Ribbon root (Stack)
  ├─ child
  └─ ribbon band (placement start|end) + corner
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读字段/Token；根节点身份尽量稳定。  
- 命中区域与布局盒一致（`hit == layout == paint`）；角标半出部分仍随 Stack 偏移绘制。  
- processing 脉冲跟随 Host Tick；尊重 reduced-motion 时可停。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Badge 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例（BDG-01…21）测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若仓库已有 visualtest 路径则保持可布局；不阻 P0 门禁）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：Badge 页覆盖 §6.8 P0 官方主路径 8 例；P1 示例可不进 gallery。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/badge.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Badge 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。

**§6 修订记录（实现前校正）：**

- **§6.2**：由通用按钮式度量改为 Badge `prepareComponentToken` 真值（20/14/6/12/8 等）。  
- **§6.3 / §6.4**：补显示优先级、processing、OnClick；纠正 `placement` 仅属 Ribbon。  
- **§6.8**：P0 与官方 8 例 + status/Ribbon API 对齐；「状态点/多彩/缎带」完整 gallery 划入 P1（API 仍 P0 可测）。  
- **§6.9 / §6.10 / §6.11**：用例期望可测化；Go API 契约与分层结构落地。
