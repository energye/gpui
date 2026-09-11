# Skeleton 骨架屏
> 来源：[Ant Design 6.5.x Skeleton](https://ant.design/components/skeleton)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：反馈（Feedback）  
> 说明：在需要等待加载内容的位置提供一个占位图形组合。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-1-1-产品需求增量gpui-验收规格)**。

---
## 1. 控件外观
### 1.1 基础形态

在需要等待加载内容的位置提供占位图形组合：标题条 + 段落行 + 可选头像，按 `loading` 切换骨架 / 真实子组件。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本（`basic.tsx`） | 标题条 + 3 行段落，末行 61% 宽 |
| 复杂的组合（`complex.tsx`） | 头像 + 标题 + 2 行段落左右结构 |
| 动画效果（`active.tsx`） | 同基本结构 + 1.4s 扫光 |
| 按钮/头像/输入框/图像/自定义节点（`element.tsx`） | 各子组件占位尺寸见 §1.3 |
| 包含子组件（`children.tsx`） | `loading=false` 时只显示 children |
| 列表（`list.tsx`） | 多行“小头像 + 标题段落”重复 |
| 自定义语义结构的样式和类（`style-class.tsx`） | 浅层 classNames/styles 钩子生效 |

### 1.3 外观相关配置逐项说明

#### `avatar`

- **说明**：是否显示头像占位图
- **类型**：`boolean | { shape?: 'circle' | 'square'; size?: number | 'large' | 'medium' | 'small' }`
- **默认值**：`false`（独立 `Skeleton.Avatar` 默认 `shape=circle, size=medium`）

#### `title`

- **说明**：是否显示标题占位图
- **类型**：`boolean | { width?: number | string }`
- **默认值**：`true`（宽度规则：无 avatar 有段落 50%，无段落 100%，§6.2）

#### `paragraph`

- **说明**：是否显示段落占位图
- **类型**：`boolean | { rows?: number; width?: number | string | Array<number | string> }`
- **默认值**：`true`（行数默认无 avatar 3 / 有 avatar 2；`width` 为数组时逐行生效，否则只压末行）

#### `active` / `round` / `loading`

- `active: boolean=false`，扫光动画开关；`round: boolean=false`，标题段落取胶囊圆角；`loading: boolean`，`true` 显示骨架。

#### 子组件占位（`Skeleton.Avatar/Button/Input/Image/Node`）

- `Avatar: shape circle|square，size large 40 / medium 32 / small 24（亦可直接传数字 px）`；`Button: 宽=2×高`；`Input: 宽=5×高`；`Image/Node: 默认 96×96`。

#### 骨架灰 → 本库 Surface 系映射

| antd 侧 | 含义 | 本库映射 |
| --- | --- | --- |
| `gradientFromColor`（`colorFillContent`） | 骨架底色 | `Theme.Surface` 上的次级填充（回落 `colorFillSecondary`） |
| `gradientToColor`（`colorFill`） | 扫光终点 | 同底色系更浅一档（回落 `colorBgContainer` 低透明高光） |
| Image svg `#bfbfbf` | 图形占位线条 | `OnSurface` 中透明近似，不单独加 token |

### 1.4 交互视觉状态（实现检查表）

| 状态 | 要求 |
| --- | --- |
| default | 灰块尺寸圆角符合 §6.2 |
| active | 1.4s 扫光，不改变布局 |
| loading=false | 只显示 children |
| round | 标题段落取胶囊圆角 |

Skeleton 为装饰性占位，不可聚焦，无 disabled/loading 焦点态。

### 1.5 语义化 DOM 与主题

- 语义节点：`root/header/section/avatar/title/paragraph`，对齐 `classNames` / `styles` 浅钩子。
- 颜色尺寸走 Token（§6.2）；`active` 动画尊重 reduced-motion。

---
## 2. 功能
### 2.1 使用场景

- 网络慢、首次加载数据时占位；图文列表/卡片可用 Skeleton 代替 Spin。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— 默认标题 + 段落结构。
2. **复杂的组合**（`complex.tsx`）— 头像 + 标题 + 段落组合。
3. **动画效果**（`active.tsx`）— `active` 扫光。
4. **按钮/头像/输入框/图像/自定义节点**（`element.tsx`）— 子组件占位。
5. **包含子组件**（`children.tsx`）— `loading` 切换 children。
6. **列表**（`list.tsx`）— `active + avatar + paragraph rows=4` 列表。
7. **自定义语义结构的样式和类**（`style-class.tsx`）— 浅层钩子。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `loading` | 骨架/内容切换 | `true` 显示骨架，`false` 显示 children |
| `active` | 扫光动画 | 1.4s 循环，可停（reduced-motion） |
| `avatar` / `title` / `paragraph` | 结构开关 | bool 快捷或 §1.3 对象形态 |
| `round` | 圆角化 | 标题段落取胶囊圆角 |
| `children` | 真实内容 | `loading=false` 时展示 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 复杂的组合 | `complex.tsx` | 否 |
| 动画效果 | `active.tsx` | 否 |
| 按钮/头像/输入框/图像/自定义节点 | `element.tsx` | 否 |
| 包含子组件 | `children.tsx` | 否 |
| 列表 | `list.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |
| 自定义组件 Token | `componentToken.tsx` | 是 |

### 2.7 组合关系

- **Card/List**：常作骨架容器；**Spin**：同为加载反馈，按场景二选一。

---
## 3. 配置（API）

### Skeleton

| 参数 | 说明 | 类型 | 默认值 |
| --- | --- | --- | --- |
| active | 是否展示动画效果 | boolean | false |
| avatar | 是否显示头像占位图 | boolean \| SkeletonAvatar | false |
| loading | 为 true 时显示占位图，反之展示子组件 | boolean | - |
| paragraph | 是否显示段落占位图 | boolean \| SkeletonParagraphProps | true |
| round | 段落和标题是否圆角化 | boolean | false |
| title | 是否显示标题占位图 | boolean \| SkeletonTitleProps | true |

#### SkeletonTitleProps

| 参数 | 说明 | 类型 | 默认值 |
| --- | --- | --- | --- |
| width | 标题占位图宽度 | number \| string | - |

#### SkeletonParagraphProps

| 参数 | 说明 | 类型 | 默认值 |
| --- | --- | --- | --- |
| rows | 段落行数 | number | - |
| width | 每行宽度（数组逐行，否则压末行） | number \| string \| Array<number \| string> | - |

### Skeleton.Avatar / Button / Input

| 参数 | 说明 | 类型 | 默认值 |
| --- | --- | --- | --- |
| active | 独立使用时的动画开关 | boolean | false |
| shape | Avatar: `circle` \| `square`；Button 加 `round` \| `default` | string | Avatar `circle` |
| size | `large` \| `medium` \| `small`（Avatar 亦可数字 px） | string \| number | `medium` |
| block | Button/Input 宽度铺满父级 | boolean | false |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

1. **结构**：`SkeletonHost` 下按 `avatar? + title? + paragraph rows?` 组合；`loading=false` 只挂 children。
2. **对象形态**：P0 先接 `SetTitleWidth` / `SetParagraphWidths` / `SetAvatarSize` 主路径（见 §6.8），完整对象形态 P1。
3. **动画**：`active` 走 Host Tick 扫光，尊重 reduced-motion；不动布局盒。
4. **示例矩阵**：§2.4 非 debug 示例均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/skeleton
- 中文文档：https://ant.design/components/skeleton-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/skeleton
- 驱动 gpui kit：`skeleton`

---
## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Skeleton** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design 6.5 桌面主路径在行为与设计体系上对齐；不是浏览器 ant.design 逐像素哈希一致。  
> 源码：`/home/yanghy/app/projects/ant-design/components/skeleton/`。

### 6.1 对齐级别定义（Skeleton）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | loading 切换、active 动画、默认结构、子组件语义 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排同系 | 大改时签字 |

**明确不做（Skeleton）：**

- 浏览器 ant.design 逐像素哈希一致。
- 为抠图破坏 `hit == layout == paint`。
- 浏览器-only 且桌面无等价映射的 API，放到 P1。
- 官方 debug 示例不计入 P0。

### 6.2 度量与 Design Token（L2 基线）

依据 antd `components/skeleton/style`（`controlHeight=32` 基线）：

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 标题条高度 `titleHeight` | 16 | `controlHeight / 2` |
| 段落行高 `paragraphLiHeight` | 16 | `controlHeight / 2` |
| 条块圆角 `blockRadius` | 4 | `borderRadiusSM` |
| `round` 条块圆角 | 胶囊 | 大圆角 / 999 |
| 头像 large / middle / small | 40 / 32 / 24 | `controlHeightLG` / `controlHeight` / `controlHeightSM` |
| Button 高×宽 | h × 2h | 同 controlHeight 档 |
| Input 高×宽 | h × 5h | 同 controlHeight 档 |
| Image / Node 默认 | 96×96 | `imageSizeBase*2`，`imageSizeBase=controlHeight*1.5` |
| 骨架底色 `gradientFromColor` | 次级填充 | `colorFillSecondary`（≈ antd `colorFillContent`） |
| active 扫光 | 轻高光 | `colorBgContainer` 低透明；时长约 1.4s |
| 标题默认宽比 | 38% / 50% / 100% | 无 avatar+有段落 / 有 avatar+段落 / 仅标题 |
| 末行段落宽比 | 61% | antd basic `width:'61%'` / CSS last-child |

禁止硬编码品牌色作为唯一默认皮。Skeleton 为装饰性占位，**不要求 focus ring**。

### 6.3 关键配置与语义

| 配置 | 说明 | 默认 |
| --- | --- | --- |
| `loading` | true 时显示骨架，false 时显示 children | true |
| `active` | 是否展示动画效果 | false |
| `avatar` | 是否显示头像占位图 | false |
| `title` | 是否显示标题占位图 | true |
| `paragraph` | 是否显示段落占位图 | true |
| `round` | 标题和段落是否圆角化 | false |
| `children` | loading=false 时展示的内容 | nil |
| `classNames` | root/header/section/avatar/title/paragraph 语义钩子 | nil |
| `styles` | root/header/section/avatar/title/paragraph 浅样式钩子 | nil |

`avatar` / `title` / `paragraph` 的对象形态见 §1.3；P0 先保证 bool 主路径与默认结构。

### 6.4 交互状态机（L1）

```text
loading=true  -> skeleton
loading=false -> children
active        -> shimmer / pulse
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| SKL-S1 | loading 骨架 | 灰块可见 |
| SKL-S2 | loading false | children 可见 |
| SKL-S3 | active | 动画可见 |
| SKL-S4 | avatar+paragraph | 结构正确 |
| SKL-S5 | paragraph rows=4 | 4 行 |
| SKL-S6 | reduced-motion | 动画可停 |

### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 符合 §6.2 token |
| active | 轻 shimmer / pulse，不抢布局 |
| round | 段落和标题转为更圆润的条块 |
| theme 切换 | 色与间距随 Theme 更新 |

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| loading 骨架 | 装饰性占位，不抢焦点 |
| children | loading=false 时由业务内容自己承担 a11y |
| 语义节点 | classNames/styles 仅作语义钩子，不改变可达性 |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| 主路径行为（§6.4 L1） | 对等 | P0 |
| 尺寸 / 色 Token（§6.2） | 对等 | P0 |
| `active` Ticker 扫光路径（可停 / reduced-motion） | 对等（近似即可） | P0 |
| 动画像素级 / 渐变 keyframes 与官网一致 | 近似 | P1 |
| 浏览器-only API | 映射或不做 | P1 |
| semantic classNames/styles **浅** 钩子（root/header/section/avatar/title/paragraph） | 对等 | P0 |
| semantic classNames/styles **深度函数形态** | 先浅后深 | P1 |
| debug 示例 / 逐像素哈希 | 不做 | - |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `loading` | 必须 |
| `active` | 必须 |
| `avatar` | 必须 |
| `title` | 必须 |
| `paragraph` | 必须 |
| `round` | 必须 |
| `children` | 必须 |
| 官方主路径示例 | 基本、复杂的组合、动画效果、按钮/头像/输入框/图像/自定义节点、包含子组件、列表、自定义语义结构的样式和类、_semantic |
| 度量 §6.2 | Token 断言 |
| a11y §6.6 | 最低要求 |
| §6.9 中 P0 用例 | 全部通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| `title` / `paragraph` / `avatar` 完整对象形态（`width` 数组/百分比字符串、avatar size 数字） | 分期；P0 提供 `SetTitleWidth` / `SetParagraphWidths` / `SetAvatarSize` 主路径 |
| semantic classNames/styles 深度函数形态（`styles(info)=>`） | 分期；P0 浅 struct 钩子 |
| 动画像素级 shimmer / 复杂自定义节点装饰 | 分期；P0 Ticker 扫光可测 |
| 浏览器-only API 或桌面无等价项 | 分期 |
| debug 示例与官网逐像素哈希 | 分期 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestSkeleton_PRD_<ID>`。  
> **P0 相关用例全部通过** 才可宣称 Skeleton 主路径完成。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| SKL-01 | L1 | NewSkeleton 默认创建 | 不崩溃；默认值符合 §6.10 |
| SKL-02 | L1 | loading 骨架 | 灰块可见 |
| SKL-03 | L1 | loading false + children | children 可见 |
| SKL-04 | L1 | active | 动画可见 |
| SKL-05 | L1 | avatar+paragraph | 结构正确 |
| SKL-06 | L1 | paragraph rows=4 | 4 行 |
| SKL-07 | L1 | reduced-motion | 可停动画 |
| SKL-08 | L1 | 基本 (`basic.tsx`) | 主视觉与布局符合文档 |
| SKL-09 | L1 | 复杂的组合 (`complex.tsx`) | 主视觉与布局符合文档 |
| SKL-10 | L1 | 动画效果 (`active.tsx`) | 主视觉与布局符合文档 |
| SKL-11 | L1 | 按钮/头像/输入框/图像/自定义节点 (`element.tsx`) | 主视觉与布局符合文档 |
| SKL-12 | L1 | 包含子组件 (`children.tsx`) | 主视觉与布局符合文档 |
| SKL-13 | L1 | 列表 (`list.tsx`) | 主视觉与布局符合文档 |
| SKL-14 | L1 | 自定义语义结构的样式和类 (`style-class.tsx`) | 主视觉与布局符合文档 |
| SKL-15 | L1 | `_semantic.tsx` | 主视觉与布局符合文档 |
| SKL-16 | L2 | §6.2 关键数字 | 与表一致 |
| SKL-17 | L2 | 默认皮颜色 | 走 Theme Token |
| SKL-18 | L2 | round 结构 | 条块圆润化 |
| SKL-19 | L3 | 关键态 golden | 与仓库基线一致 |
| SKL-20 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| SKL-21 | P1 | 任一 P1 能力 | 单独用例；Notes 标明 |

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下是建议契约，语义优先。

```text
NewSkeleton() *Skeleton
NewSkeletonAvatar() *SkeletonAvatar
NewSkeletonButton() *SkeletonButton
NewSkeletonInput() *SkeletonInput
NewSkeletonImage() *SkeletonImage
NewSkeletonNode(child core.Node) *SkeletonNode

// Skeleton
SetLoading(bool)                 // default true
SetActive(bool)                  // default false; Ticker shimmer
SetAvatar(bool)                  // default false
SetTitle(bool)                   // default true
SetParagraph(bool)               // default true
SetParagraphRows(int)            // <=0 → antd 默认 3(无avatar+title) / 2
SetRound(bool)
SetContent(core.Node)            // loading=false 时 children
SetAvatarShape(SkeletonAvatarShape)  // circle|square（主路径辅助）
SetAvatarSize(SkeletonSize)          // small|middle|large
SetTitleWidth(float64)               // P0 简化；完整对象形态 P1
SetParagraphWidths(...float64)       // P0 简化
SetTheme(*Theme)
SetStyle(Style)
SetClassNames(SkeletonClassNames)    // root/header/section/avatar/title/paragraph
SetStyles(SkeletonStyles)            // 浅样式钩子
Node() core.Node
ChromeNode() core.Node
AttachTicker(*core.Tree)
Tick(dt float64) bool

// Subcomponents (Avatar/Button/Input/Image/Node)
SetSize(SkeletonSize)            // Avatar/Button/Input
SetShape(...)                    // Avatar: circle|square; Button: default|circle|round|square
SetBlock(bool)                   // Button/Input
SetChild(core.Node)              // Node
SetActive / SetTheme / SetStyle / AttachTicker / Tick
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Loading | true |
| Active / Avatar / Round | false |
| Title / Paragraph | true |
| ParagraphRows | 0（推导 3 或 2） |
| AvatarShape / AvatarSize | circle / large |

### 6.11 结构与绘制分层（实现提示）

```text
SkeletonHost (RepaintBoundary)
  └─ shell / content
       └─ avatar? + title? + paragraph rows?
```

- 只组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读默认值、字段和 Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Skeleton 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过。  
4. L3 golden 至少覆盖 1 个关键可见态。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页增加或更新示例，覆盖 **§6.8 P0** 主路径。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/skeleton.md` §6；P1 显式列出。

---

**本章用法**：实现 `ui/kit` Skeleton 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
