# QRCode 二维码
> 来源：[Ant Design 6.5.x QRCode](https://ant.design/components/qr-code)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：能够将文本转换生成二维码的组件，支持自定义配色和 Logo 配置。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

能够将文本转换生成二维码的组件，支持自定义配色和 Logo 配置。

**QRCode** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本使用 | 复现「基本使用」视觉与布局 |
| 带 Icon 的例子 | 复现「带 Icon 的例子」视觉与布局 |
| 不同的状态 | 复现「不同的状态」视觉与布局 |
| 自定义状态渲染器 | 自定义渲染/插槽外观 |
| 自定义渲染类型 | 自定义渲染/插槽外观 |
| 自定义尺寸 | 不同 size 档位的高宽/字号/内边距 |
| 自定义颜色 | 语义色/预设色 |
| 下载二维码 | 复现「下载二维码」视觉与布局 |
| 纠错比例 | 复现「纠错比例」视觉与布局 |
| 高级用法 | 复现「高级用法」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `type`

- **说明**：渲染类型
- **类型**：`canvas | svg`
- **默认值**：`canvas`
- **版本**：5.6.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `canvas` | 官方取值 `canvas` |
  | `svg` | 官方取值 `svg` |

#### `icon`

- **说明**：二维码中图片的地址（目前只支持图片地址）
- **类型**：string
- **默认值**：-

#### `size`

- **说明**：二维码大小
- **类型**：number
- **默认值**：160

#### `iconSize`

- **说明**：二维码中图片的大小
- **类型**：number | { width: number; height: number }
- **默认值**：40
- **版本**：5.19.0

#### `color`

- **说明**：二维码颜色
- **类型**：string
- **默认值**：`#000`

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

#### `bgColor`

- **说明**：二维码背景颜色
- **类型**：string
- **默认值**：`transparent`
- **版本**：5.5.0

#### `marginSize`

- **说明**：留白（安静区）大小（单位为模块数），`0` 表示无留白
- **类型**：number
- **默认值**：`0`
- **版本**：6.2.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `0` | 官方取值 `0` |

#### `bordered`

- **说明**：是否有边框
- **类型**：boolean
- **默认值**：true

#### `status`

- **说明**：二维码状态
- **类型**：`active | expired | loading | scanned`
- **默认值**：`active`
- **版本**：scanned: 5.13.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `active` | 官方取值 `active` |
  | `expired` | 官方取值 `expired` |
  | `loading` | 官方取值 `loading` |
  | `scanned` | 官方取值 `scanned` |

#### `statusRender`

- **说明**：自定义状态渲染器
- **类型**：(info: [StatusRenderInfo](/components/qr-code-cn#statusrenderinfo)) => React.ReactNode
- **默认值**：-
- **版本**：5.20.0

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-
- **版本**：6.0.0

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

当需要将文本转换成为二维码时使用。

### 2.2 核心功能（按官方示例拆解）

1. **基本使用**（`base.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **带 Icon 的例子**（`icon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **不同的状态**（`status.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **自定义状态渲染器**（`customStatusRender.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **自定义渲染类型**（`type.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **自定义尺寸**（`customSize.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **自定义颜色**（`customColor.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **下载二维码**（`download.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **纠错比例**（`errorlevel.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **高级用法**（`Popover.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 扫描后的文本 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本使用 | `base.tsx` | 否 |
| 带 Icon 的例子 | `icon.tsx` | 否 |
| 不同的状态 | `status.tsx` | 否 |
| 自定义状态渲染器 | `customStatusRender.tsx` | 否 |
| 自定义渲染类型 | `type.tsx` | 否 |
| 自定义尺寸 | `customSize.tsx` | 否 |
| 自定义颜色 | `customColor.tsx` | 否 |
| 下载二维码 | `download.tsx` | 否 |
| 纠错比例 | `errorlevel.tsx` | 否 |
| 高级用法 | `Popover.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |

### 2.6 FAQ

## FAQ

### 关于二维码纠错等级 {#faq-error-correction-level}

纠错等级也叫纠错率，就是指二维码可以被遮挡后还能正常扫描，而这个能被遮挡的最大面积就是纠错率。

通常情况下二维码分为 4 个纠错级别：`L级` 可纠正约 `7%` 错误、`M级` 可纠正约 `15%` 错误、`Q级` 可纠正约 `25%` 错误、`H级` 可纠正约`30%` 错误。并不是所有位置都可以缺损，像最明显的三个角上的方框，直接影响初始定位。中间零散的部分是内容编码，可以容忍缺损。当二维码的内容编码携带信息比较少的时候，也就是链接比较短的时候，设置不同的纠错等级，生成的图片不会发生变化。

> 有关更多信息，可参阅相关资料：[https://www.qrcode.com/zh/about/error_correction](https://www.qrcode.com/zh/about/error_correction.html)

### ⚠️⚠️⚠️ 二维码无法扫描？ {#faq-cannot-scan}

若二维码无法扫码识别，可能是因为链接地址过长导致像素过于密集，可以通过 size 配置二维码更大，或者通过短链接服务等方式将链接变短。

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

> 自 `antd@5.1.0` 版本开始提供该组件。

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| :-- | :-- | :-- | :-- | :-- | --- |
| value | 扫描后的文本 | `string \| string[]` | - | `string[]`: 5.28.0 | × |
| type | 渲染类型 | `canvas \| svg` | `canvas` | 5.6.0 | × |
| icon | 二维码中图片的地址（目前只支持图片地址） | string | - | - | × |
| size | 二维码大小 | number | 160 | - | × |
| iconSize | 二维码中图片的大小 | number \| { width: number; height: number } | 40 | 5.19.0 | × |
| color | 二维码颜色 | string | `#000` | - | × |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | 6.0.0 | 6.0.0 |
| bgColor | 二维码背景颜色 | string | `transparent` | 5.5.0 | × |
| marginSize | 留白（安静区）大小（单位为模块数），`0` 表示无留白 | number | `0` | 6.2.0 | × |
| bordered | 是否有边框 | boolean | true | - | × |
| errorLevel | 二维码纠错等级 | `'L' \| 'M' \| 'Q' \| 'H'` | `M` | - | × |
| boostLevel | 如果启用，自动提升纠错等级，结果的纠错级别可能会高于指定的纠错级别 | `boolean` | true | 5.28.0 | × |
| status | 二维码状态 | `active \| expired \| loading \| scanned` | `active` | scanned: 5.13.0 | × |
| statusRender | 自定义状态渲染器 | (info: [StatusRenderInfo](/components/qr-code-cn#statusrenderinfo)) => React.ReactNode | - | 5.20.0 | × |
| styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 6.0.0 | 6.0.0 |

### StatusRenderInfo

```typescript
type StatusRenderInfo = {
  status: QRStatus;
  locale: Locale['QRCode'];
  onRefresh?: () => void;
};
```

### 导入方式

```js
import { QRCode } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `value` | 扫描后的文本 | `string \| string[]` | - | `string[]`: 5.28.0 |
| `type` | 渲染类型 | `canvas \| svg` | `canvas` | 5.6.0 |
| `icon` | 二维码中图片的地址（目前只支持图片地址） | string | - | - |
| `size` | 二维码大小 | number | 160 | - |
| `iconSize` | 二维码中图片的大小 | number \| { width: number; height: number } | 40 | 5.19.0 |
| `color` | 二维码颜色 | string | `#000` | - |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |
| `bgColor` | 二维码背景颜色 | string | `transparent` | 5.5.0 |
| `marginSize` | 留白（安静区）大小（单位为模块数），`0` 表示无留白 | number | `0` | 6.2.0 |
| `bordered` | 是否有边框 | boolean | true | - |
| `errorLevel` | 二维码纠错等级 | `'L' \| 'M' \| 'Q' \| 'H'` | `M` | - |
| `boostLevel` | 如果启用，自动提升纠错等级，结果的纠错级别可能会高于指定的纠错级别 | `boolean` | true | 5.28.0 |
| `status` | 二维码状态 | `active \| expired \| loading \| scanned` | `active` | scanned: 5.13.0 |
| `statusRender` | 自定义状态渲染器 | (info: [StatusRenderInfo](/components/qr-code-cn#statusrenderinfo)) => React.ReactNode | - | 5.20.0 |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | 6.0.0 |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **QRCode** 的验收清单：

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
- 官方文档：https://ant.design/components/qr-code
- 中文文档：https://ant.design/components/qr-code-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/qr-code
- 驱动 gpui kit：`qr-code`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **QRCode** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/qrcode/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（QRCode）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 展示形态与可选交互（复制/预览/关闭） | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（QRCode）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  
### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。  
源码：`components/qr-code/style/index.ts`（`borderRadiusLG` / `paddingSM` / `colorSplit` / cover 半透明）。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 默认边长 `size` | **160** | API 默认（外框宽高） |
| 内边距（有边框） | **12** | `paddingSM` |
| 内边距（borderless） | **0** | 无边框时清零 |
| 圆角 | **8** | `borderRadiusLG`（非 `borderRadius=6`） |
| 边框线宽 | **1** | `lineWidth` |
| 字号（status 文案） | **14** | `fontSize` |
| 默认 icon 边长 | **40** | API `iconSize` |
| 安静区 `marginSize` | **0**（模块数） | API；`0` = 无留白 |
| Focus ring outset | ≈ **1.5px** 可见 | 刷新按钮等可聚焦者 |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token / 默认 | 备注 |
| --- | --- | --- |
| 模块前景 `color` | **`colorText`**（antd 运行时默认；文档 `#000` 为回落） | 禁止硬编码品牌主色 |
| 模块背景 `bgColor` | **transparent**（API）；有边框时根底 **`#fff` / colorWhite** | 根 `backgroundColor` 合并 `bgColor` |
| 边框 | **`colorSplit`** | 有边框时 |
| 遮罩底 | **`colorBgContainer` @ α≈0.96** | `QRCodeCoverBackgroundColor` |
| 遮罩文案 | **`colorText`** | expired / scanned |
| 刷新 link | primary / link 按钮色 | OnRefresh 按钮 |

禁止硬编码品牌色作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据展示**。  
**P0 必实现**见 §6.8。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `value` | 扫描内容（空值 antd 不渲染） | string \| string[] | —（kit 以 string 主路径） |
| `type` | 渲染后端标签 | `canvas` \| `svg` | `canvas`（桌面均画模块矩阵，语义标签） |
| `size` | 外框边长 | number | **160** |
| `icon` / `iconSize` | 中心图；尺寸 number 或 {w,h} | string / Node · number | icon 空；size **40** |
| `color` / `bgColor` | 模块前景 / 背景 | color | colorText / transparent |
| `bordered` | 是否边框+内边距 | bool | **true** |
| `errorLevel` | 纠错等级 L/M/Q/H | enum | **M** |
| `boostLevel` | 自动抬升纠错 | bool | true（P1：库侧近似） |
| `marginSize` | 安静区模块数 | number | **0** |
| `status` | active / expired / loading / scanned | enum | **active** |
| `onRefresh` | expired 刷新 | func() | — |
| `statusRender` | 自定义状态内容 | func(info) → Node | 默认文案/Spin/刷新 |
| `classNames` / `styles` | 语义 root/cover | Record | 浅 P0；函数形态 P1 |

**配置优先级（通用）：** 显式 Set\* > 组件默认 > ConfigProvider 全局默认（后者 P1）。

**Locale（en_US 默认）：** `expired="QR code expired"` · `refresh="Refresh"` · `scanned="Scanned"`。

### 6.4 交互状态机（L1）

```text
[mount]
  value 非空 ──► encode 矩阵 ──► paint modules（+ 可选 icon 居中 excavate）
  value 空    ──► 不崩；无矩阵（antd 返回 null）

status:
  active  ──► 仅矩阵（+icon）
  loading ──► cover 遮罩 + Spin（Ticker 旋转）
  expired ──► cover + "QR code expired" + [Refresh]（若 OnRefresh）
  scanned ──► cover + "Scanned"

expired · Refresh 点击 ──► OnRefresh()
statusRender 非空 ──► cover 内容替换为自定义 Node（仍占满 cover）
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| QR-S1 | value 非空 | 矩阵 Modules≥21；布局不崩 |
| QR-S2 | size=200 | 外框宽高 **200** |
| QR-S3 | status=expired | 显示 cover 遮罩 |
| QR-S4 | onRefresh + expired | 点击 Refresh 触发一次回调 |
| QR-S5 | icon | 中心 icon 区可见（Node 或占位） |
| QR-S6 | errorLevel L/M/Q/H | 均可 encode 成功（矩阵非空） |
| QR-S7 | 默认 size | **160** |
| QR-S8 | bordered=false | 无边框色；padding=0 |
| QR-S9 | status=loading | cover + spinner；Ticker 驱动 |
| QR-S10 | status=scanned | cover + scanned 文案 |
| QR-S11 | statusRender | 覆盖默认 status 内容 |
| QR-S12 | color / bgColor | 模块色可覆盖；默认走 Token |

### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default + bordered | 外框 size×size；pad=paddingSM；radius=borderRadiusLG；边框 colorSplit；根底 white/bgColor |
| borderless | 边框透明；pad=0；radius=0 |
| modules | 前景 color（默认 colorText）；背景 bgColor（默认 transparent，绘制时对透明回落白/根底） |
| icon | 居中；默认 40；下方模块 excavate（挖空再贴图） |
| cover | 绝对铺满根；半透明 colorBgContainer；居中 status 内容 |
| loading | Spin 环（Ticker）；无文案或 locale 可选 |
| expired | 文案 + link 刷新按钮（可聚焦） |
| scanned | 仅文案 |
| 主题切换 | 默认色/边框/遮罩随 Theme 更新 |

**动效：** loading 旋转跟随 Host Tick；reduced-motion 下可静止（P1 细控）；P0 允许持续转。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 根角色 | `img` 或 `group`；有可访问名（AriaLabel 或 value 摘要） |
| 装饰/矩阵 | 不单独抢焦点 |
| 刷新按钮 | 可聚焦；有名（locale.refresh）；Enter/Space 触发 OnRefresh |
| cover loading | `status` live polite 可选 |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| value→矩阵 / size / status / icon / color / bordered / errorLevel | **对等** | P0 L1+L2 |
| loading Spin + Ticker | **对等** | P0 |
| type=canvas\|svg | **语义标签**（桌面均模块绘制，无 DOM canvas/svg） | P0 标签 / P1 导出差异 |
| 下载二维码（toDataURL / SVG 序列化） | **宿主导出**矩阵/位图 API | P1 |
| boostLevel 精确抬升 | **近似**或分期 | P1 |
| string[] value | **分期**（P0 单 string） | P1 |
| 真 HTTP 解码 icon URL | **宿主**；kit 接受 IconNode / 占位 | P0 Node / P1 HTTP |
| semantic classNames/styles 函数形态 | 浅 hooks P0；深度 P1 | P0/P1 |
| ConfigProvider 全局 qrcode 默认 | 随 ConfigProvider | P1 |
| 逐像素官网哈希 | **不做** | — |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `value`（string） | 编码生成矩阵 |
| `size` 默认 160 | 外框边长 |
| `color` / `bgColor` | 模块色；默认 Token |
| `bordered` | 默认 true；false→borderless |
| `errorLevel` L/M/Q/H 默认 M | 可生成 |
| `marginSize` 默认 0 | 安静区模块数 |
| `icon` + `iconSize` | 中心图标（Node 或 src 占位） |
| `status` active\|expired\|loading\|scanned | 默认 active |
| `onRefresh` | expired 刷新 |
| `statusRender` | 自定义 cover 内容 |
| `type` canvas\|svg 标签 | 存储并影响语义；绘制同路径 |
| Theme Token / §6.2 度量 | paddingSM12 / radiusLG8 / lineW1 / size160 |
| a11y §6.6 | 根名 + 刷新可聚焦 |
| 官方主路径示例 | base / icon / status / customStatusRender / type / customSize / customColor / errorlevel / Popover(borderless) |
| §6.9 L1/L2 P0 用例 | 测试通过 |
| hit == layout == paint | 根盒一致 |
| loading Ticker | status=loading 旋转 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| 下载二维码（download 示例 / canvas.toDataURL / svg 导出） | 宿主 API |
| `boostLevel` 精确语义 | 分期 |
| `value` string[] | 分期 |
| 真 HTTP 解码 icon URL | 分期 |
| semantic classNames/styles 函数形态深度 | 分期 |
| style-class 完整 / ConfigProvider 全局 | 分期 |
| type 导出差异（真 SVG 序列化） | 分期 |
| 动画像素级 / reduced-motion 细控 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestQRCode_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记、非 L3/L4）全部通过** 才可宣称 QRCode 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| QR-01 | L1 | NewQRCode 默认创建 | 不崩溃；size=160；status=active；bordered=true；errorLevel=M |
| QR-02 | L1 | value 非空 | Modules≥21；Layout 成功 |
| QR-03 | L1 | size=200 | 外框宽高 200（±0.5） |
| QR-04 | L1 | status=expired | HasCover；遮罩节点存在 |
| QR-05 | L1 | onRefresh + 触发刷新 | 回调 1 次 |
| QR-06 | L1 | icon | 中心 icon 宿主非空 |
| QR-07 | L1 | errorLevel L/M/Q/H | 均可 Modules>0 |
| QR-08 | L1 | 默认 size | 160 |
| QR-09 | L1 | 复现 base.tsx | value 可变；矩阵随 value 更新 |
| QR-10 | L1 | 复现 status.tsx | loading/expired/scanned 三态 cover |
| QR-11 | L1 | 复现 icon.tsx | errorLevel=H + icon |
| QR-12 | L1 | 复现 customColor.tsx | color/bgColor 覆盖生效 |
| QR-13 | L1 | 复现 errorlevel.tsx | 切换 L→H 可生成 |
| QR-14 | L1 | 复现 type.tsx | type canvas/svg 可设；均出矩阵 |
| QR-15 | L1 | 复现 customSize.tsx | size 48–300；iconSize=size/4 |
| QR-16 | L1 | 复现 customStatusRender | StatusRender 覆盖默认 |
| QR-17 | L1 | bordered=false（Popover 示例） | 无边框；pad=0 |
| QR-18 | L2 | §6.2 度量 | size160 / pad12 / radius8 / lineW1（±0.5） |
| QR-19 | L2 | 默认皮颜色 | 前景≠硬编码品牌 primary；走 colorText / colorSplit |
| QR-20 | L2 | N/A disabled | QRCode 无 disabled；跳过或 Notes |
| QR-21 | L1 | 刷新按钮焦点/键盘 | expired+OnRefresh 可点（按钮路径） |
| QR-22 | L3 | 关键态 golden | 后置可 |
| QR-23 | L4 | 与 ant.design 并排 | 人眼后置 |
| QR-24 | P1 | 下载 / boostLevel / string[] / HTTP icon | Notes；本阶段不做 |

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 契约，实现可微调命名但语义不可丢。

```text
NewQRCode(value string) *QRCode

// 数据 / 形态
SetValue(string) / Value() string
SetType(QRCodeType)                 // canvas | svg（默认 canvas）
SetSize(float64)                    // 默认 160；外框边长
SetIcon(src string) / SetIconNode(core.Node)
SetIconSize(px float64) / SetIconSizeWH(w, h float64)  // 默认 40
SetColor(render.RGBA) / SetBgColor(render.RGBA)
SetBordered(bool)                   // 默认 true
SetErrorLevel(QRErrorLevel)         // L|M|Q|H 默认 M
SetMarginSize(int)                  // 默认 0
SetStatus(QRStatus)                 // active|expired|loading|scanned
SetStatusRender(func(QRStatusRenderInfo) core.Node)
SetOnRefresh(func())

// 主题 / 覆盖 / a11y
SetTheme(*core.Theme) / SetFace(text.Face) / SetStyle(Style)
SetCoverStyle(Style) / SetClassNames(QRCodeClassNames)
SetAriaLabel(string)

// 查询
Size() float64 / Status() QRStatus / Bordered() bool
ErrorLevel() QRErrorLevel / Modules() int
HasCover() bool / HasIcon() bool
ResolvedColor() / ResolvedBgColor() / Padding() / Radius() / LineWidth()
ContentSize() float64               // 内区边长（size - 2*pad）

// 挂树 / 动画
Node() core.Node                    // 稳定 Decorated 根
ChromeNode() core.Node
AttachTicker(*core.Tree)            // loading 时；OnMount 亦可自绑
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Value | 构造参数 |
| Type | canvas |
| Size | **160** |
| IconSize | **40** |
| Color | Theme `colorText` |
| BgColor | transparent |
| Bordered | **true** |
| ErrorLevel | **M** |
| MarginSize | **0** |
| Status | **active** |
| BoostLevel | true（P1 行为） |

### 6.11 结构与绘制分层（实现提示）

```text
Decorated root                    // size×size；pad / border / radius / bg
  └─ Stack (Stretch)
       ├─ PainterNode modules     // 矩阵 + marginSize 安静区
       ├─ iconHost (center)       // 可选
       └─ cover (full)            // status≠active
            └─ status body | StatusRender
                 Spin (Ticker) | expired+Refresh | scanned
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读 Default/字段/Token；根指针尽量稳定（ClearChildren）。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- loading 动画跟随 Host Tick（`tickerLifecycle`）；不自建帧循环。  
- 真实 QR 编码可用纯 Go 库；icon URL 解码归宿主。

### 6.12 完成定义（DoD）

同时满足即可宣布 **QRCode 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例（QR-01–QR-21，除标注 N/A）测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见；可后置）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/qr-code.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` QRCode 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。

**§6 相对模板修订摘要（本轮手写）：**  
- **§6.2**：圆角改为 `borderRadiusLG=8`；补 paddingSM、iconSize、marginSize、cover 色。  
- **§6.3–6.5**：展开真实配置表、状态机规则 QR-S1–S12、chrome。  
- **§6.7–6.8**：P0 字段清单（value/size/status/icon/…）；P1 含下载/boost/HTTP icon。  
- **§6.9**：用例扩到 QR-01–QR-24（L1 官方示例 + L2）。  
- **§6.10–6.11**：Go API 与 Stack 分层具体化。
