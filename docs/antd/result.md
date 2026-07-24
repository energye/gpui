# Result 结果
> 来源：[Ant Design 6.5.x Result](https://ant.design/components/result)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：反馈（Feedback）  
> 说明：用于反馈一系列操作任务的处理结果。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

用于反馈一系列操作任务的处理结果。

**Result** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| Success | 复现「Success」视觉与布局 |
| Info | 复现「Info」视觉与布局 |
| Warning | 复现「Warning」视觉与布局 |
| 403 | 复现「403」视觉与布局 |
| 404 | 复现「404」视觉与布局 |
| 500 | 复现「500」视觉与布局 |
| Error | 复现「Error」视觉与布局 |
| 自定义 icon | 自定义渲染/插槽外观 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：自定义组件内部各语义化结构的类名。支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-
- **版本**：6.0.0

#### `extra`

- **说明**：操作区
- **类型**：ReactNode
- **默认值**：-

#### `icon`

- **说明**：自定义 icon
- **类型**：ReactNode
- **默认值**：-

#### `status`

- **说明**：结果的状态，决定图标和颜色
- **类型**：`success` | `error` | `info` | `warning` | `404` | `403` | `500`
- **默认值**：`info`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `success` | 成功绿语义 |
  | `error` | 错误红语义 |
  | `warning` | 警告橙语义 |
  | `404` | 官方取值 `404` |
  | `403` | 官方取值 `403` |
  | `500` | 官方取值 `500` |

#### `styles`

- **说明**：自定义组件内部各语义化结构的内联样式。支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-
- **版本**：6.0.0

#### `title`

- **说明**：title 文字
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

当有重要操作需告知用户处理结果，且反馈内容较为复杂时使用。

### 2.2 核心功能（按官方示例拆解）

1. **Success**（`success.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **Info**（`info.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **Warning**（`warning.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **403**（`403.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **404**（`404.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **500**（`500.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **Error**（`error.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **自定义 icon**（`customIcon.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| Success | `success.tsx` | 否 |
| Info | `info.tsx` | 否 |
| Warning | `warning.tsx` | 否 |
| 403 | `403.tsx` | 否 |
| 404 | `404.tsx` | 否 |
| 500 | `500.tsx` | 否 |
| Error | `error.tsx` | 否 |
| 自定义 icon | `customIcon.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
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

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 自定义组件内部各语义化结构的类名。支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> | - | 6.0.0 | 6.0.0 |
| extra | 操作区 | ReactNode | - | icon | 自定义 icon | ReactNode | - | status | 结果的状态，决定图标和颜色 | `success` \| `error` \| `info` \| `warning` \| `404` \| `403` \| `500` | `info` | styles | 自定义组件内部各语义化结构的内联样式。支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | 6.0.0 | 6.0.0 |
| subTitle | subTitle 文字 | ReactNode | - | title | title 文字 | ReactNode | - 
### 导入方式

```js
import { Result } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 自定义组件内部各语义化结构的类名。支持对象或函数 | Record \| (info: { props }) => Record | - | 6.0.0 |
| `extra` | 操作区 | ReactNode | - | — |
| `icon` | 自定义 icon | ReactNode | - | — |
| `status` | 结果的状态，决定图标和颜色 | `success` \| `error` \| `info` \| `warning` \| `404` \| `403` \| `500` | `info` | — |
| `styles` | 自定义组件内部各语义化结构的内联样式。支持对象或函数 | Record \| (info: { props }) => Record | - | 6.0.0 |
| `subTitle` | subTitle 文字 | ReactNode | - | — |
| `title` | title 文字 | ReactNode | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Result** 的验收清单：

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
- 官方文档：https://ant.design/components/result
- 中文文档：https://ant.design/components/result-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/result
- 驱动 gpui kit：`result`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Result** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/result/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Result）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 展示/自动关闭/堆叠/类型语义 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Result）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：用于反馈一系列操作任务的处理结果。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**反馈**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `classNames` | 自定义组件内部各语义化结构的类名。支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> |
| `extra` | 操作区 | ReactNode | - |
| `icon` | 自定义 icon | ReactNode | - |
| `status` | 结果的状态，决定图标和颜色 | `success` \ | `error` \ |
| `styles` | 自定义组件内部各语义化结构的内联样式。支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props }) => Record<[SemanticDOM](#semantic-dom), CSSProperties> |
| `subTitle` | subTitle 文字 | ReactNode | - |
| `title` | title 文字 | ReactNode | - |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
status 决定图标
title/subTitle/extra 展示
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| RES-S1 | status=success | 成功图标 |
| RES-S2 | 404 | 404 皮 |
| RES-S3 | extra 按钮 | 可点 |
| RES-S4 | 自定义 icon | 替换 |
| RES-S5 | subTitle | 可见 |
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
| 实时区域 | message/notification 用 status 语义等价 |
| 关闭 | 可关控件可操作 |
| 不抢焦点 | 轻提示默认不抢（Modal 例外） |

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
| `status` | 必须 |
| `title` | 必须 |
| `icon` | 必须 |
| 官方主路径示例 | Success、Info、Warning、403、404、500、Error、自定义 icon |
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

> 测试名建议：`TestResult_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Result 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| RES-01 | L1 | NewResult 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| RES-02 | L1 | status=success | 成功图标 |
| RES-03 | L1 | 404 | 404 皮 |
| RES-04 | L1 | extra 按钮 | 可点 |
| RES-05 | L1 | 自定义 icon | 替换 |
| RES-06 | L1 | subTitle | 可见 |
| RES-07 | L1 | 复现官方示例「Success」（`success.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RES-08 | L1 | 复现官方示例「Info」（`info.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RES-09 | L1 | 复现官方示例「Warning」（`warning.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RES-10 | L1 | 复现官方示例「403」（`403.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RES-11 | L1 | 复现官方示例「404」（`404.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RES-12 | L1 | 复现官方示例「500」（`500.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RES-13 | L1 | 复现官方示例「Error」（`error.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RES-14 | L1 | 复现官方示例「自定义 icon」（`customIcon.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| RES-15 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| RES-16 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| RES-17 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| RES-18 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| RES-19 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| RES-20 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| RES-21 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewResult(...) *Result

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
Host holder or inline
  └─ item (icon + content + close?)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Result 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/result.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Result 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
