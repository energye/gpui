# Icon 图标
> 来源：[Ant Design 6.5.x Icon](https://ant.design/components/icon)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：通用（General）  
> 说明：语义化的矢量图形。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

语义化的矢量图形。

**Icon** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本用法 | 复现「基本用法」视觉与布局 |
| 多色图标 | icon 与文本混排 |
| 自定义图标 | icon 与文本混排 |
| 使用 iconfont.cn | 复现「使用 iconfont.cn」视觉与布局 |
| 使用 iconfont.cn 的多个资源 | 复现「使用 iconfont.cn 的多个资源」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `className`

- **说明**：设置图标的样式名
- **类型**：string
- **默认值**：-

#### `rotate`

- **说明**：图标旋转角度（IE9 无效）
- **类型**：number
- **默认值**：-

#### `style`

- **说明**：设置图标的样式，例如 `fontSize` 和 `color`
- **类型**：CSSProperties
- **默认值**：-

#### `twoToneColor`

- **说明**：仅适用双色图标。设置双色图标的主要颜色，或主要颜色和次要颜色
- **类型**：string | \[string, string]
- **默认值**：-

#### `component`

- **说明**：控制如何渲染图标，通常是一个渲染根标签为 `` 的 React 组件
- **类型**：ComponentType<CustomIconComponentProps>
- **默认值**：-

#### `extraCommonProps`

- **说明**：给所有的 `svg` 图标 `` 组件设置额外的属性
- **类型**：{ \[key: string]: any }
- **默认值**：{}
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `svg` | 官方取值 `svg` |

#### `scriptUrl`

- **说明**：[iconfont.cn](https://iconfont.cn/) 项目在线生成的 js 地址，`@ant-design/icons@4.1.0` 之后支持 `string[]` 类型
- **类型**：string | string\[]
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

- 至少区分根容器、内容区、装饰/图标区；浮层再分 popup/mask。

- 颜色、圆角、间距、动效走 Design Token；支持亮暗色与品牌色。

- 动效可关（reduced-motion / 全局 motion、wave 配置）。
---
## 2. 功能
### 2.1 使用场景

使用图标组件，你需要安装 [@ant-design/icons](https://github.com/ant-design/ant-design-icons) 图标组件包：

:::info{title=温馨提示}
使用 antd@6.x 版本时, 请确保安装配套的 `@ant-design/icons@6.x` 版本，避免版本不匹配带来的 Context 问题。详见 [#53275](https://github.com/ant-design/ant-design/issues/53275#issuecomment-2747448317)
:::

### 2.2 核心功能（按官方示例拆解）

1. **基本用法**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **多色图标**（`two-tone.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **自定义图标**（`custom.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **使用 iconfont.cn**（`iconfont.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **使用 iconfont.cn 的多个资源**（`scriptUrl.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本用法 | `basic.tsx` | 否 |
| 多色图标 | `two-tone.tsx` | 否 |
| 自定义图标 | `custom.tsx` | 否 |
| 使用 iconfont.cn | `iconfont.tsx` | 否 |
| 使用 iconfont.cn 的多个资源 | `scriptUrl.tsx` | 否 |

### 2.5 实例方法 / Ref

#### 使用方法 {#how-to-use}

使用图标组件，你需要安装 [@ant-design/icons](https://github.com/ant-design/ant-design-icons) 图标组件包：

<InstallDependencies npm='npm install @ant-design/icons@6.x --save' yarn='yarn add @ant-design/icons@6.x' pnpm='pnpm install @ant-design/icons@6.x --save' bun='bun add @ant-design/icons@6.x'></InstallDependencies>

<!-- prettier-ignore -->
:::info{title=温馨提示}
使用 antd@6.x 版本时, 请确保安装配套的 `@ant-design/icons@6.x` 版本，避免版本不匹配带来的 Context 问题。详见 [#53275](https://github.com/ant-design/ant-design/issues/53275#issuecomment-2747448317)
:::

### 2.6 FAQ

## FAQ

### 为什么有时 icon 注入的样式会引起全局样式异常？{#faq-icon-bad-style}

相关 issue：[#54391](https://github.com/ant-design/ant-design/issues/54391)

启用 `layer` 时，icon 的样式可能会使 `@layer antd` 优先级降低，并导致所有组件样式异常。

这个问题可以通过以下两步解决：

1. 使用 `@ant-design/icons@6.x` 配合 `antd@6.x`。
2. 停止使用 `message`, `Modal` 和 `notification` 的静态方法，改为使用 hooks 版本或 App 提供的实例。

如果无法避免使用静态方法，可以在 App 组件下立刻使用任一一个 icon 组件，以规避静态方法对样式的影响。

```diff

  
    
+     {/* any icon */}
+     
      {/* your pages */}
    
  

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

从 4.0 开始，antd 不再内置 Icon 组件，请使用独立的包 `@ant-design/icons`。

### 通用图标 {#common-icon}

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| className | 设置图标的样式名 | string | - | rotate | 图标旋转角度（IE9 无效） | number | - | spin | 是否有旋转动画 | boolean | false | style | 设置图标的样式，例如 `fontSize` 和 `color` | CSSProperties | - | twoToneColor | 仅适用双色图标。设置双色图标的主要颜色，或主要颜色和次要颜色 | string \| \[string, string] | - 
其中我们提供了三种主题的图标，不同主题的 Icon 组件名为图标名加主题做为后缀。

```jsx
import { StarOutlined, StarFilled, StarTwoTone } from '@ant-design/icons';

<StarOutlined />
<StarFilled />
<StarTwoTone twoToneColor="#eb2f96" />
```

### 自定义 Icon {#custom-icon}

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| component | 控制如何渲染图标，通常是一个渲染根标签为 `<svg>` 的 React 组件 | ComponentType&lt;CustomIconComponentProps> | - | rotate | 图标旋转角度（IE9 无效） | number | - | spin | 是否有旋转动画 | boolean | false | style | 设置图标的样式，例如 `fontSize` 和 `color` | CSSProperties | - 
### 关于 SVG 图标 {#about-svg-icons}

在 `3.9.0` 之后，我们使用了 SVG 图标替换了原先的 font 图标，从而带来了以下优势：

- 完全离线化使用，不需要从 CDN 下载字体文件，图标不会因为网络问题呈现方块，也无需字体文件本地部署。
- 在低端设备上 SVG 有更好的清晰度。
- 支持多色图标。
- 对于内建图标的更换可以提供更多 API，而不需要进行样式覆盖。

更多讨论可参考：[#10353](https://github.com/ant-design/ant-design/issues/10353)。

所有的图标都会以 `<svg>` 标签渲染，可以使用 `style` 和 `className` 设置图标的大小和单色图标的颜色。例如：

```jsx
import { MessageOutlined } from '@ant-design/icons';

<MessageOutlined style={{ fontSize: '16px', color: '#08c' }} />;
```

### 双色图标主色 {#set-two-tone-color}

对于双色图标，可以通过使用 `getTwoToneColor()` 和 `setTwoToneColor(colorString)` 来全局设置图标主色。

```jsx
import { getTwoToneColor, setTwoToneColor } from '@ant-design/icons';

setTwoToneColor('#eb2f96');
getTwoToneColor(); // #eb2f96
```

### 自定义 font 图标 {#custom-font-icon}

在 `3.9.0` 之后，我们提供了一个 `createFromIconfontCN` 方法，方便开发者调用在 [iconfont.cn](https://iconfont.cn/) 上自行管理的图标。

```jsx
import React from 'react';
import { createFromIconfontCN } from '@ant-design/icons';
import ReactDOM from 'react-dom/client';

const MyIcon = createFromIconfontCN({
  scriptUrl: '//at.alicdn.com/t/font_8d5l8fzk5b87iudi.js', // 在 iconfont.cn 上生成
});

ReactDOM.createRoot(mountNode).render(<MyIcon type="icon-example" />);
```

其本质上是创建了一个使用 `<use>` 标签来渲染图标的组件。

options 的配置项如下：

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| extraCommonProps | 给所有的 `svg` 图标 `<Icon />` 组件设置额外的属性 | { \[key: string]: any } | {} | scriptUrl | [iconfont.cn](https://iconfont.cn/) 项目在线生成的 js 地址，`@ant-design/icons@4.1.0` 之后支持 `string[]` 类型 | string \| string\[] | - 
在 `scriptUrl` 都设置有效的情况下，组件在渲染前会自动引入 [iconfont.cn](https://iconfont.cn/) 项目中的图标符号集，无需手动引入。

见 [iconfont.cn 使用帮助](https://iconfont.cn/help/detail?spm=a313x.7781069.1998910419.15&helptype=code) 查看如何生成 js 地址。

### 自定义 SVG 图标 {#custom-svg-icon}

如果使用 `webpack`，可以通过配置 [@svgr/webpack](https://www.npmjs.com/package/@svgr/webpack) 来将 `svg` 图标作为 `React` 组件导入。`@svgr/webpack` 的 `options` 选项请参阅 [svgr 文档](https://github.com/smooth-code/svgr#options)。

```js
// webpack.config.js
module.exports = {
  // ... other config
  test: /\.svg(\?v=\d+\.\d+\.\d+)?$/,
  use: [
    {
      loader: 'babel-loader',
    },
    {
      loader: '@svgr/webpack',
      options: {
        babel: false,
        icon: true,
      },
    },
  ],
};
```

如果使用 `vite`，可以通过配置 [vite-plugin-svgr](https://www.npmjs.com/package/vite-plugin-svgr) 来将 `svg` 图标作为 `React` 组件导入。`vite-plugin-svgr` 的 `options` 选项请参阅 [svgr 文档](https://github.com/smooth-code/svgr#options)。

```js
// vite.config.js
export default defineConfig(() => ({
  // ... other config
  plugins: [svgr({ svgrOptions: { icon: true } })],
}));
```

```jsx
import React from 'react';
import Icon from '@ant-design/icons';
import MessageSvg from 'path/to/message.svg'; // 你的 '*.svg' 文件路径

// import MessageSvg from 'path/to/message.svg?react'; // 使用vite 你的 '*.svg?react' 文件路径.
import ReactDOM from 'react-dom/client';

// in create-react-app:
// import { ReactComponent as MessageSvg } from 'path/to/message.svg';

ReactDOM.createRoot(mountNode).render(<Icon component={MessageSvg} />);
```

`Icon` 中的 `component` 组件的接受的属性如下：

| 字段 | 说明 | 类型 | 只读值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| className | 计算后的 `svg` 类名 | string | - | fill | `svg` 元素填充的颜色 | string | `currentColor` | height | `svg` 元素高度 | string \| number | `1em` | style | 计算后的 `svg` 元素样式 | CSSProperties | - | width | `svg` 元素宽度 | string \| number | `1em` 
### 导入方式

```js
import { HomeOutlined } from '@ant-design/icons';
// 需 @ant-design/icons@6.x
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `className` | 设置图标的样式名 | string | - | — |
| `rotate` | 图标旋转角度（IE9 无效） | number | - | — |
| `spin` | 是否有旋转动画 | boolean | false | — |
| `style` | 设置图标的样式，例如 `fontSize` 和 `color` | CSSProperties | - | — |
| `twoToneColor` | 仅适用双色图标。设置双色图标的主要颜色，或主要颜色和次要颜色 | string \| \[string, string] | - | — |
| `component` | 控制如何渲染图标，通常是一个渲染根标签为 `` 的 React 组件 | ComponentType<CustomIconComponentProps> | - | — |
| `extraCommonProps` | 给所有的 `svg` 图标 `` 组件设置额外的属性 | { \[key: string]: any } | {} | — |
| `scriptUrl` | [iconfont.cn](https://iconfont.cn/) 项目在线生成的 js 地址，`@ant-design/icons@4.1.0` 之后支持 `string[]` 类型 | string \| string\[] | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Icon** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **5** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/icon
- 中文文档：https://ant.design/components/icon-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/icon
- 驱动 gpui kit：`icon`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Icon**（`@ant-design/icons` 产品面）补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/icon/`（`index.zh-CN.md` + demos）；图标实现来自 `@ant-design/icons`（antd 4+ 不再内置）。

### 6.1 对齐级别定义（Icon）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 注册名解析、尺寸、旋转、spin、双色、自定义绘制、离线 iconfont 映射 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 默认尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Icon）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7 / §6.8 P1）。  
- 官方 **debug** 示例不计入 P0 验收。  
- 全量 `@ant-design/icons` 上千字形（P0 仅内建注册表 + 可扩展 Register）。  

> 控件说明：语义化的矢量图形；默认**装饰性**（不进 Tab）。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| **默认边长** | **16**（≈1em 产品常用） | `DefaultIconSize`；`Size==0` 时用此值（亦可读 `fontSize` 作参考，产品默认固定 16） |
| 线宽（内建字形） | ≈ `size×0.125`，夹在 1.6–2.5 | 字形几何，非控件边框 |
| 命中盒 | **边长 × 边长** | `hit == layout == paint` |
| 圆角 / Focus ring | 不适用（装饰 Icon 默认无焦点环） | 可交互宿主（Button 等）负责 |

#### 6.2.2 颜色 Token（语义）

| 用途 | Token 建议 | 备注 |
| --- | --- | --- |
| 默认单色 | `colorText` | `Color.A==0` 时回落 Theme |
| 双色主色 | `colorPrimary` 或 `SetTwoToneColor` | antd `twoToneColor` / 全局 `setTwoToneColor` |
| 双色次色 | 主色半透明或显式 secondary | `SetTwoToneColors(primary, secondary)` |
| 禁用（适用者） | `colorDisabledText` | Icon 自身可选 `Disabled` 外观 |
| 强调 / 状态 | `colorError` / `Success` / `Warning` | 业务 `SetColor` 覆盖 |

禁止硬编码品牌色作为唯一默认皮。

### 6.3 关键配置与语义

下列为 **产品关键配置**。浏览器 CSS / React 字段映射到 kit 列于「桌面映射」。

| antd 配置 | 说明 | 桌面映射（kit） | 默认 |
| --- | --- | --- | --- |
| `type` / 图标名 | 注册表中的图标名 | `Name` / `SetName` | 构造参数 |
| `rotate` | 静态旋转角（度） | `SetRotate(deg)` | 0 |
| `spin` | 是否旋转动画 | `SetSpin(bool)` + Ticker | false |
| `style.fontSize` | 边长 | `SetSize` | **16** |
| `style.color` | 单色 | `SetColor` | Theme `colorText` |
| `twoToneColor` | 双色主色或 [主, 次] | `SetTwoToneColor` / `SetTwoToneColors`；全局 `SetTwoToneColorGlobal` | 全局主色 |
| `component` | 自定义 SVG 渲染 | `SetPainter(IconPainter)` | nil |
| `createFromIconfontCN` + `scriptUrl` | 远程 iconfont 脚本 | **离线** `CreateFromIconfont` + `RegisterSource`（不拉 CDN） | — |
| `className` | CSS 类名 | `ClassName` 字符串钩子（无 CSS 引擎） | "" |
| `extraCommonProps` | 透传 DOM | **P1 不做** | — |

**配置优先级：** 显式 `SetXxx` > 全局 two-tone 默认 > 组件默认 > Theme Token。

### 6.4 交互状态机（L1）

```text
resolve(name | painter) ──► paint glyph
rotate(deg) ──────────────► 静态角
spin=true ────────────────► Tick 累加相位（Clock.ReduceMotion → 停）
spin=false ───────────────► 相位冻结；仅 static rotate
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| ICO-S1 | 已知名（注册表命中） | 绘出对应字形；布局 `size×size` |
| ICO-S2 | 未知名 | **不 panic**；绘占位（如菱形）或空绘 |
| ICO-S3 | `SetSize(24)` | 布局约 **24×24**（±0.5） |
| ICO-S4 | `spin=true` 且非 reduced-motion | 有效角度随 Tick 变化 |
| ICO-S5 | `Clock.ReduceMotion` + spin | **不转**（相位不推进） |
| ICO-S6 | `rotate=180` | 有效角含 180°（倒置） |
| ICO-S7 | `SetColor` | 单色字形使用该色（A>0） |
| ICO-S8 | 装饰默认 | **不进 Tab**；`HitDefer`；无强制 Role |
| ICO-S9 | `SetAriaLabel` 非空 | 有意义图：`Role=img`，Label=名 |
| ICO-S10 | `SetPainter` | 优先自定义绘制，忽略 name 字形 |
| ICO-S11 | two-tone | 主色（+ 可选次色）参与绘制 |
| ICO-S12 | Iconfont 多源 | 后注册源覆盖同名（对齐 antd 多 `scriptUrl`） |

### 6.5 视觉 chrome 规则（L2 摘要）

| 态 | 规则 |
| --- | --- |
| default | 边长 §6.2；色 `colorText`（或显式 Color） |
| spin | 绕中心旋转；reduced-motion 静止 |
| rotate | 静态角叠加在 spin 相位上：`effective = rotate + spinPhase×360` |
| two-tone | 主/次色；全局默认可 `SetTwoToneColorGlobal` |
| disabled | 若设 `Disabled`：`colorDisabledText`；无 hover 高亮 |
| 主题切换 | 默认色随 Theme 更新 |

**动效：** spin 用 `Tree.AddTicker` + `MarkNeedsPaint`；**禁止** `ContinuousRender`；尊重 reduced-motion。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 装饰默认 | 等价 `aria-hidden`：无 Label、不进 Tab、HitDefer |
| 有意义图标 | `SetAriaLabel` → `Role=img` + Label |
| 可点击图标 | 应由 **Button / FloatButton** 等宿主承载；纯 Icon 不做 click 状态机 |

### 6.7 平台边界（gpui vs 浏览器 antd）

| 能力 | 策略 | 级别 |
| --- | --- | --- |
| name / size / color / rotate / spin | **对等** | P0 L1 |
| twoToneColor（主 + 可选次） | **对等**（几何字形近似） | P0 L2 |
| component 自定义绘制 | **映射** `IconPainter` | P0 |
| iconfont / 多 scriptUrl | **映射** 离线 Register 多源（不拉 CDN） | P0 离线 |
| 远程 `//at.alicdn.com/...js` 注入 | **不做** | P1 |
| `className` / CSS `style` 全量 | 语义钩子 / size+color | P1 深度 |
| `extraCommonProps` / DOM 透传 | **不做** | P1 |
| 全量 ant icons SVG Path | 注册表扩展；真 SVG Path 引擎 | P1 |
| ConfigProvider 全局默认 | 分期 | P1 |
| 逐像素官网哈希 | **不做** | — |

### 6.8 能力裁剪（P0 / P1）

#### P0（本阶段必须 1:1，否则不算完成）

| 配置 / 能力 | 说明 |
| --- | --- |
| `name` 注册解析 | 内建 + `Register` / 多源 |
| `size`（style.fontSize） | 默认 **16**；`SetSize` |
| `color`（style.color） | Theme Token；`SetColor` |
| `rotate` | 度；`SetRotate` |
| `spin` | Ticker；`AttachTicker` / OnMount；reduced-motion |
| `twoToneColor` | 实例 + 全局 `Set/GetTwoToneColorGlobal` |
| `component` | `SetPainter` 自定义绘制 |
| iconfont / 多源 | `CreateFromIconfont` + `RegisterSource`（离线；**不**拉远程 script） |
| 官方主路径示例 | 基本用法、多色、自定义、iconfont、多源（映射 API） |
| 度量 §6.2 | 默认 16；SetSize 布局 |
| a11y §6.6 | 装饰默认；AriaLabel |
| §6.9 中 **非 P1** 的 L1/L2 用例 | 测试通过 |

#### P1（可 later，须在 coverage Notes 写明）

| 配置 / 能力 | 说明 |
| --- | --- |
| 远程 iconfont.cn `scriptUrl` 网络加载 | 分期 |
| `extraCommonProps` / 真 DOM classNames | 分期 |
| 全量 SVG Path / 上千官方字形 | 分期 |
| spin 动画像素级 / 官网哈希 | 分期 |
| ConfigProvider 全局 Icon 默认 | 分期 |
| 可聚焦交互 Icon（非装饰） | 宿主控件负责；纯 Icon 不做 |

### 6.9 验收用例表（可测）

> 测试名：`TestIcon_PRD_<ID>`。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Icon 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| ICO-01 | L1 | `NewIcon("check")` 默认创建 | 不崩溃；Size=0→默认 16；Spin=false；Rotate=0；装饰 |
| ICO-02 | L1 | 已知名 `check` | `Known()` true；布局 16×16；可 paint |
| ICO-03 | L1 | 未知名 `no-such-icon-xyz` | 不 panic；`Known()` false；布局仍成立 |
| ICO-04 | L1 | `SetSize(24)` | 布局约 24×24（±0.5） |
| ICO-05 | L1 | `spin=true`，Tick 推进 | `Angle()` / 相位变化 |
| ICO-06 | L1 | ReduceMotion + spin | Tick 后相位**不变** |
| ICO-07 | L1 | `rotate=180` | `Rotate==180`；有效角含 180 |
| ICO-08 | L1 | `SetColor` 非零 | 存储色 A>0；glyph 使用该色 |
| ICO-09 | L1 | 装饰默认 | 不进 Tab；无 Focusable 角色要求 |
| ICO-10 | L1 | 官方「基本用法」 | 多 name + spin + rotate 并存不崩 |
| ICO-11 | L1 | 官方「多色图标」 | twoTone 主色/双色可设 |
| ICO-12 | L1 | 官方「自定义图标」 | Painter 绘制；Size 可变 |
| ICO-13 | L1 | 官方「iconfont」映射 | CreateFromIconfont + 注册 type 可 NewIcon |
| ICO-14 | L1 | 官方「多 scriptUrl」映射 | 双源；后源覆盖同名 |
| ICO-15 | L2 | 默认尺寸 | 未 SetSize → 布局 **16**（±0.5） |
| ICO-16 | L2 | 默认皮颜色 | `EffectiveColor` 走 Theme `colorText`（非硬编码品牌） |
| ICO-17 | L2 | `SetDisabled(true)` | 有效色为禁用文本 Token |
| ICO-18 | L1 | 有意义名 | `SetAriaLabel` → Role=img + Label |
| ICO-19 | L3 | 关键态 golden | 基线一致（另测 / 可选） |
| ICO-20 | L4 | 与 ant.design 并排 | 人眼签字 |
| ICO-21 | P1 | 远程 scriptUrl / 全量 SVG 等 | Notes 标明；本阶段不做 |

> **P0 测试范围：** ICO-01…ICO-18（L1/L2）。ICO-19/20 为 L3/L4；ICO-21 为 P1。

### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 契约，实现可微调命名但语义不可丢。

```text
// —— 单图标 ——
NewIcon(name string) *Icon

SetName(string)
SetSize(float64)                 // 0 → DefaultIconSize(16)
SetColor(render.RGBA)            // A==0 → Theme colorText
SetRotate(deg float64)           // 静态角（度）
SetSpin(bool)                    // Ticker 旋转
SetTwoToneColor(primary RGBA)    // 仅主色；次色默认派生
SetTwoToneColors(primary, secondary RGBA)
SetPainter(IconPainter)          // antd component；非 nil 优先
SetTheme(*core.Theme)
SetAriaLabel(string)             // 非空 → 有意义 img
SetDecorative(bool)              // 默认 true
SetDisabled(bool)                // 外观；默认 false
SetClassName(string)             // 语义钩子；无 CSS
SetRegistry(*primitive.IconRegistry) // 可选；默认 GlobalIcons

Known() bool
EffectiveSize() float64
EffectiveColor() render.RGBA
EffectiveAngle() float64         // rotate + spinPhase*360（度）
Node() core.Node
ChromeNode() core.Node           // 根绘制节点（primitive.Icon / host）
AttachTicker(*core.Tree)
Tick(dt float64) bool            // core.Ticker

// —— 全局双色 ——
SetTwoToneColorGlobal(render.RGBA)
GetTwoToneColorGlobal() render.RGBA

// —— iconfont 离线族（antd createFromIconfontCN）——
CreateFromIconfont(IconfontOptions) *IconfontFamily
// IconfontOptions.Sources []string  // 多源 id，对齐 scriptUrl[] 覆盖序
// family.Register(typeName, IconDef | Painter)
// family.NewIcon(typeName) *Icon
RegisterIconSource(sourceID string, icons map[string]primitive.IconDef)
```

**默认值（未 Set 时）：**

| 字段 | 默认 |
| --- | --- |
| Size | **0 → Effective 16**（`DefaultIconSize`） |
| Color | A=0 → Theme `colorText` |
| Rotate | 0 |
| Spin | false |
| Decorative | **true** |
| Disabled | false |
| Painter | nil（走 name 注册表） |
| TwoTone | 全局 GetTwoToneColorGlobal（默认同 colorPrimary 语义） |
| Registry | `primitive.GlobalIcons` |

### 6.11 结构与绘制分层（实现提示）

```text
iconHost (RepaintBoundary 可选；OnMount/OnUnmount 绑 Ticker)
  └─ primitive.Icon  (Size / Color / RotateDeg / SpinPhase / Painter / TwoTone)
       paint: Push → RotateAbout(center) → glyph / painter → Pop
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- `rebuild()` 只读 Default / 字段 / Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 `Clock.ReduceMotion`。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Icon 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例（ICO-01…ICO-18）测试通过。  
3. L2 度量与 Token 断言通过（默认边长 16、默认色走 Theme）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见；ICO-19 可另 PR）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：Icon 页覆盖 **§6.8 P0** 主路径（基本 / spin+rotate / two-tone / custom / iconfont 多源）。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/icon.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Icon 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。

**§6 修订说明（相对模板稿）：**  
- **§6.2**：默认边长明确为 **16**；去掉无关控件圆角/focus ring 硬套。  
- **§6.3 / §6.7 / §6.8**：浏览器 `className/style/component/scriptUrl/extraCommonProps` 改为桌面映射；**远程 scriptUrl / extraCommonProps / 全量 SVG** 降为 **P1**；离线多源 Register 为 P0。  
- **§6.4**：补 ICO-S9…S12（a11y / painter / two-tone / 多源覆盖）。  
- **§6.9**：P0 范围锁定 ICO-01…18；L3/L4/P1 分开。  
- **§6.10**：写出完整 Go API（含 IconfontFamily、全局 two-tone、Ticker）。