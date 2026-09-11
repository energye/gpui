# Empty 空状态
> 来源：[Ant Design 6.5.x Empty](https://ant.design/components/empty)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：空状态时的展示占位图。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

空状态时的展示占位图。

**Empty** 的视觉由 image（default/simple/自定义）+ description + footer（children 操作区）纵向居中组成；本体无交互态、无浮层（见 §6.4–§6.5）。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 选择图片 | 复现「选择图片」视觉与布局 |
| 自定义 | 自定义渲染/插槽外观 |
| 全局化配置 | 复现「全局化配置」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |
| 无描述 | 复现「无描述」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `classNames`

- **说明**：用于自定义组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

#### `description`

- **说明**：自定义描述内容
- **类型**：ReactNode
- **默认值**：-

#### `imageStyle`

- **说明**：图片样式，请使用 `styles.image` 替代
- **类型**：CSSProperties
- **默认值**：-

#### `styles`

- **说明**：用于自定义组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props })=> Record
- **默认值**：-

### 1.4 交互视觉状态（实现检查表）

本控件为非交互占位，无 hover / active / focus / disabled / loading / error 态（本体不可点；footer 子 Button 自带交互，见 §6.4–§6.6）。

### 1.5 语义化 DOM 与主题

- 支持 `classNames` / `styles`；kit 应对齐语义节点钩子。

- 颜色、圆角、间距、动效走 Design Token；支持亮暗色与品牌色。

- 动效可关（reduced-motion / 全局 motion、wave 配置）。
---
## 2. 功能
### 2.1 使用场景

- 当目前没有数据时，用于显式的用户提示。
- 初始化场景时的引导创建流程。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **选择图片**（`simple.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **自定义**（`customize.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **全局化配置**（`config-provider.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **无描述**（`description.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 选择图片 | `simple.tsx` | 否 |
| 自定义 | `customize.tsx` | 否 |
| 全局化配置 | `config-provider.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 无描述 | `description.tsx` | 否 |

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

```jsx
<Empty>
  <Button>创建</Button>
</Empty>
```

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| classNames | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), string> | - | description | 自定义描述内容 | ReactNode | - | image | 设置显示图片，为 string 时表示自定义图片地址。 | ReactNode | `Empty.PRESENTED_IMAGE_DEFAULT` | ~~imageStyle~~ | 图片样式，请使用 `styles.image` 替代 | CSSProperties | - | styles | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props })=> Record<[SemanticDOM](#semantic-dom), CSSProperties> | - 
## 内置图片

- Empty.PRESENTED_IMAGE_SIMPLE

  <div class="site-empty-buildIn-img site-empty-buildIn-simple"><div>

- Empty.PRESENTED_IMAGE_DEFAULT

  <div class="site-empty-buildIn-img site-empty-buildIn-default"></div>

<style>
  .site-empty-buildIn-img {
    background-repeat: no-repeat;
    background-size: contain;
  }
  .site-empty-buildIn-simple {
    width: 55px;
    height: 35px;
    background-image: url("https://user-images.githubusercontent.com/507615/54591679-b0ceb580-4a65-11e9-925c-ad15b4eae93d.png");
  }
  .site-empty-buildIn-default {
    width: 121px;
    height: 116px;
    background-image: url("https://user-images.githubusercontent.com/507615/54591670-ac0a0180-4a65-11e9-846c-e55ffce0fe7b.png");
  }
</style>

### 导入方式

```js
import { Empty } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `classNames` | 用于自定义组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props })=> Record | - | — |
| `description` | 自定义描述内容 | ReactNode | - | — |
| `image` | 设置显示图片，为 string 时表示自定义图片地址。 | ReactNode | `Empty.PRESENTED_IMAGE_DEFAULT` | — |
| `imageStyle` | 图片样式，请使用 `styles.image` 替代 | CSSProperties | - | — |
| `styles` | 用于自定义组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props })=> Record | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Empty** 的验收清单：

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
- 官方文档：https://ant.design/components/empty
- 中文文档：https://ant.design/components/empty-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/empty
- 驱动 gpui kit：`empty`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Empty** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/empty/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Empty）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 插画种类（default/simple/自定义）、描述显隐（默认 locale / 自定义 / `false` 隐藏）、footer 操作区有无 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Empty）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：空状态时的展示占位图。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，种子：`controlHeightLG=40`、`fontSize=14`）。实现必须通过 Token / `DefaultEmpty*` 读取；下表对齐 `components/empty/style/index.ts`。

> kit `TokenMarginXS=4` 对应 antd `marginXXS`；antd Empty 用的 `marginXS=8` 在 kit 侧以 `TokenMarginSM` 或 `DefaultEmpty*` 回落（与 Rate/Divider 同口径）。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 字号 | **14** | `fontSize` |
| 根 margin-inline | **8** | antd `marginXS`（kit `DefaultEmptyMarginInline` / `TokenMarginSM`） |
| 图 height（default） | **100** | `emptyImgHeight = controlHeightLG × 2.5` |
| 图 height（simple / normal） | **40** | `emptyImgHeightMD = controlHeightLG` |
| 图 height（small 上下文） | **35** | `emptyImgHeightSM = controlHeightLG × 0.875` |
| 图 margin-bottom | **8** | antd `marginXS` |
| 图 opacity | **1** | `opacityImage`（未覆盖时 1） |
| footer margin-top | **16** | antd `margin` |
| simple 根 margin-block | **32** | antd `marginXL`（`empty-normal`） |
| Focus ring outset（footer 可交互子） | ≈ **1.5px** | 子 Button 自带 |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 描述文案 | `colorTextDescription` ≈ `colorTextSecondary` | antd description 色 |
| 内置插画 fill | `colorFill*` / `colorBgContainer` / `colorTextQuaternary` | 随 Theme，禁止硬编码品牌主色当皮 |
| 容器底 / 边框（styles 覆盖） | `colorBgContainer` / `colorBorder` | 仅 style-class 路径 |
| 禁用 | 不适用 | Empty **无** disabled API |
| 插画色回落 | `getAsSolidColor(前景, colorBgContainer)` | 前景半透明时与底实色合成（源码 `empty.tsx/utils.ts`），无底时取 `colorBgContainer` |

#### 6.2.3 内置插画几何色块稿（P0 近似稿，源码 `empty.tsx` / `simple.tsx`）

- **default 稿**（`PRESENTED_IMAGE_DEFAULT`，`viewBox 184×152`）：底部椭圆阴影（`shadowColor=colorFillSecondary`，`cx 67.8 cy 106.9 rx 67.8 ry 12.7`，`fillOpacity .8`）+ 外框（`borderColor=colorTextQuaternary`）+ 面板（`panelBgColor=colorFillTertiary`）+ 细节线（`detailColor=colorFill`）+ 右上气泡（`detailColor` 底 + `iconColor=colorBgContainer` 图形）；kit 用 Canvas 几何色块近似，不逐像素抠 SVG。
- **simple 稿**（`PRESENTED_IMAGE_SIMPLE`，`viewBox 64×41`）：底部椭圆阴影（`shadowColor=colorFillTertiary`）+ 梯形盒描边（`borderColor=colorFill`）+ 内容块（`contentColor=colorFillQuaternary`）；`empty-normal` 上下文 `marginBlock=32`，图高 40。

禁止硬编码品牌色作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据展示**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `description` | 自定义描述；`false`/`null`/空串隐藏（antd `des &&`） | ReactNode \| false | locale `No data` |
| `image` | 内置 default/simple、自定义 Node、或 string 源 | ReactNode \| string | `Empty.PRESENTED_IMAGE_DEFAULT` |
| `children` | 底部操作区（footer） | ReactNode | - |
| `styles` | 语义浅覆盖 root/image/description/footer | Record / Style | - |
| `classNames` | 语义钩子 root/image/description/footer | Record / string tags | - |
| ~~`imageStyle`~~ | 已弃用 → `styles.image`（高度等） | — | — |

**配置优先级（通用）：** 显式 props > 组件默认 > ConfigProvider 全局默认（全局空态见 P1）。

### 6.4 交互状态机（L1）

```text
mount ──► 显示 image + description? + children(footer)?
             ├── image=PRESENTED_IMAGE_SIMPLE ──► 简图（empty-normal 度量）
             ├── image=string|Node ──► 自定义图
             ├── description 未设 ──► locale「No data」
             ├── description=false|"" ──► 隐藏文案
             └── children ──► footer 可点（子控件自身交互）
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| EMP-S1 | 默认 Empty | 默认插画 + locale「No data」 |
| EMP-S2 | simple 图 | 简图高度 40 + normal 外边距 |
| EMP-S3 | 自定义 description | 文案替换 |
| EMP-S4 | children 按钮 | footer 存在且可点 |
| EMP-S5 | 自定义 image | 显示指定 Node/src |
| EMP-S6 | 主题切换 | 描述色/插画色随 Theme |
| EMP-S7 | description=false | 无描述节点 |

### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 符合 §6.2 Token；居中；image→description→footer 纵向 |
| hover/active/focus | Empty 本体无交互态；footer 子控件自带 |
| disabled / loading | **不适用**（Empty 无此 API） |
| 主题切换 | 色与间距随 Theme 更新 |

**动效：** 无入场强制动效；P0 瞬时。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 装饰内置图 | 无 Role/Label（装饰）；有 `AriaLabel` 时可标名 |
| string 图 | Role=`img`，alt=描述文案或 `AriaLabel` |
| footer 操作 | 子 Button 自带可访问名 |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 主路径行为（§6.1 L1） | **对等** | P0 L1 |
| 尺寸/色 Token（§6.2） | **对等** | P0 L2 |
| 内置 SVG 插画 | **近似** Canvas/几何色块（随 Token） | P0 近似 |
| 真 URL 图片解码 | **映射**为 src 标签/占位 | P1 |
| ConfigProvider `renderEmpty` / 全局 empty.image | 随 ConfigProvider | P1 |
| semantic classNames/styles **函数形态**深度 | 分期 | P1 |
| styles/classNames **浅覆盖** | kit Style / ClassNames 钩子 | P0 |
| debug `_semantic.tsx` / 官网逐像素 | **不做** / P1 | P1 |
| 动画/波纹 | 瞬时 | P1 |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `description` | 默认 locale；自定义；`false`/空隐藏 |
| `image` | DEFAULT / SIMPLE / 自定义 Node / string src |
| `children` | footer 操作区 |
| `styles` 浅覆盖 | root / image / description / footer（Style） |
| `classNames` 浅钩子 | root / image / description / footer 字符串标签 |
| `styles.image.height` | 覆盖图高度（含旧 `imageStyle` 语义） |
| 官方主路径示例 | **基本**、**选择图片**、**自定义**、**无描述**、**style-class**（浅） |
| 度量 §6.2 | Token / DefaultEmpty* 断言 |
| a11y §6.6 | 装饰图 + 有意义操作 |
| §6.9 中 L1/L2 **无 P1 标记** 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| ConfigProvider `renderEmpty` / 全局 empty 默认图 | `config-provider.tsx` |
| semantic classNames/styles 函数形态与深度合并 | 分期 |
| `_semantic.tsx` debug 语义预览 | 文档工具 |
| 真 HTTP/SVG 原样解码、动画像素级 | 分期 |
| 浏览器-only API / 官网逐像素哈希 | 不做 / 分期 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestEmpty_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1/L3/L4 标记）全部通过** 才可宣称 Empty 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| EMP-01 | L1 | NewEmpty 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| EMP-02 | L1 | 默认 Empty | 默认插画 kind=default + 文案「No data」 |
| EMP-03 | L1 | simple 图 | kind=simple；图高 ≈40 |
| EMP-04 | L1 | 自定义 description | 文案替换 |
| EMP-05 | L1 | children 按钮 | footer 存在；子可点 |
| EMP-06 | L1 | 自定义 image Node/src | 显示指定图 |
| EMP-07 | L1 | 主题切换 | 描述色随 Theme Token |
| EMP-08 | L1 | 复现官方「基本」（`basic.tsx`） | 默认可布局 |
| EMP-09 | L1 | 复现官方「选择图片」（`simple.tsx`） | simple 度量 |
| EMP-10 | L1 | 复现官方「自定义」（`customize.tsx`） | 自定义图高/描述/footer 按钮 |
| EMP-11 | P1 | 复现「全局化配置」（`config-provider.tsx`） | ConfigProvider.renderEmpty |
| EMP-12 | L1 | 复现「style-class」（`style-class.tsx`） | 浅 Root/Image/Description/Footer Style |
| EMP-13 | L1 | 复现「无描述」（`description.tsx`） | 无描述节点 |
| EMP-14 | P1 | `_semantic.tsx` | debug 语义预览 |
| EMP-15 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px） |
| EMP-16 | L2 | 默认皮颜色 | 描述色走 Theme；无硬编码品牌主色 |
| EMP-17 | L2 | disabled 外观 | **不适用**（Empty 无 disabled）— 跳过 |
| EMP-18 | L1 | 键盘/焦点 | footer 子 Button 可聚焦（本体无焦点） |
| EMP-19 | L3 | 关键态 golden | 本库 golden（可后补） |
| EMP-20 | L4 | 与 ant.design 并排 | 人眼签字 |
| EMP-21 | P1 | §6.8 其余 P1 | Notes 标明 |

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 契约。

```text
NewEmpty() *Empty

// image
EmptyImageDefault / EmptyImageSimple          // ≈ PRESENTED_IMAGE_*
SetImage(kind EmptyImageKind)
SetImageNode(n core.Node)                     // 自定义 Node
SetImageSrc(src string)                       // string 图
SetImageHeight(h float64)                     // styles.image.height；0=按 kind

// description
SetDescription(s string)                      // "" → 隐藏（antd 假值）
SetDescriptionNode(n core.Node)
HideDescription()                             // description=false
ResolvedDescription() string
HasDescription() bool

// footer
SetChildren(kids ...core.Node)
HasFooter() bool

// styles / classNames（浅）
SetStyle(Style)                               // root
SetImageStyle / SetDescriptionStyle / SetFooterStyle(Style)
SetClassNames(EmptyClassNames)

// theme / a11y / mount
SetTheme(*Theme)  SetFace(Face)  SetAriaLabel(string)
Node() / ChromeNode() core.Node

// L2 只读
ImageHeight() / ImageMarginBottom() / FooterMarginTop() / MarginInline()
DescriptionColor() / FontSize()
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Image | `EmptyImageDefault`（高 100） |
| Description | locale **`No data`**（`DefaultEmptyDescription`） |
| Children | 无 footer |
| styles/classNames | 空（走 Token） |

### 6.11 结构与绘制分层（实现提示）

```text
Decorated root          // styles.root / classNames.root
  └─ Column (CrossCenter)
       ├─ image box     // Canvas 内置 or ImageNode；styles.image
       ├─ description?  // Text / Node；styles.description
       └─ footer?       // children；styles.footer；marginTop=16
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读 Default/字段/Token；Root 指针尽量稳定（ClearChildren）。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 无 Ticker 需求（Empty 本体无 loading）；footer 子 Button 自管。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Empty 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/empty.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Empty 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
