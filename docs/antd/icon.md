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

> 本章把 antd **Icon** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/icon/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Icon）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 展示形态与可选交互（复制/预览/关闭） | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Icon）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：语义化的矢量图形。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 默认 | **1em / 14–16px** | 随字号或显式 size |
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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**通用**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `className` | 设置图标的样式名 | string | - |
| `rotate` | 图标旋转角度（IE9 无效） | number | - |
| `spin` | 是否有旋转动画 | boolean | false |
| `style` | 设置图标的样式，例如 `fontSize` 和 `color` | CSSProperties | - |
| `twoToneColor` | 仅适用双色图标。设置双色图标的主要颜色，或主要颜色和次要颜色 | string \ | \[string, string] |
| `component` | 控制如何渲染图标，通常是一个渲染根标签为 `<svg>` 的 React 组件 | ComponentType&lt;CustomIconComponentP… | - |
| `extraCommonProps` | 给所有的 `svg` 图标 `<Icon />` 组件设置额外的属性 | { \[key: string]: any } | {} |
| `scriptUrl` | [iconfont.cn](https://iconfont.cn/) 项目在线生成的 js 地址，`@ant-d… | string \ | string\[] |
| `fill` | `svg` 元素填充的颜色 | string | `currentColor` |
| `height` | `svg` 元素高度 | string \ | number |
| `width` | `svg` 元素宽度 | string \ | number |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
resolve(name) ──► paint
spin ──► Tick 旋转（reduced-motion 停）
rotate ──► 静态角
```

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| ICO-S1 | 已知名 | 绘出 |
| ICO-S2 | 未知名 | 不 panic |
| ICO-S3 | SetSize(24) | 约 24×24 |
| ICO-S4 | spin | 角度随时间变 |
| ICO-S5 | reduced-motion+spin | 不转 |
| ICO-S6 | rotate=180 | 倒置 |
| ICO-S7 | SetColor | 着色 |
| ICO-S8 | 装饰默认 | 不进 Tab |
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
| 装饰图 | alt 或 aria-hidden |
| 有意义操作 | 复制/关闭/展开有名 |

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
| `className` | 必须 |
| `rotate` | 必须 |
| `spin` | 必须 |
| `style` | 必须 |
| `twoToneColor` | 必须 |
| `component` | 必须 |
| `extraCommonProps` | 必须 |
| `scriptUrl` | 必须 |
| 官方主路径示例 | 基本用法、多色图标、自定义图标、使用 iconfont.cn、使用 iconfont.cn 的多个资源 |
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

### 6.9 验收用例表（可测）

> 测试名建议：`TestIcon_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Icon 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| ICO-01 | L1 | NewIcon 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| ICO-02 | L1 | 已知名 | 绘出 |
| ICO-03 | L1 | 未知名 | 不 panic |
| ICO-04 | L1 | SetSize(24) | 约 24×24 |
| ICO-05 | L1 | spin | 角度随时间变 |
| ICO-06 | L1 | reduced-motion+spin | 不转 |
| ICO-07 | L1 | rotate=180 | 倒置 |
| ICO-08 | L1 | SetColor | 着色 |
| ICO-09 | L1 | 装饰默认 | 不进 Tab |
| ICO-10 | L1 | 复现官方示例「基本用法」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| ICO-11 | L1 | 复现官方示例「多色图标」（`two-tone.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| ICO-12 | L1 | 复现官方示例「自定义图标」（`custom.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| ICO-13 | L1 | 复现官方示例「使用 iconfont.cn」（`iconfont.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| ICO-14 | L1 | 复现官方示例「使用 iconfont.cn 的多个资源」（`scriptUrl.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| ICO-15 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| ICO-16 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| ICO-17 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| ICO-18 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| ICO-19 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| ICO-20 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| ICO-21 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewIcon(...) *Icon

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
Display root
  └─ content (+ actions?)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Icon 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序** [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)：在对应控件页**增加或更新**示例，覆盖 **§6.8 P0** 主路径（官方非 debug 优先；细则见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；P1 可不进 gallery。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/icon.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Icon 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
