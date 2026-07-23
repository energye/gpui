# Icon 图标
> 来源：[Ant Design 6.5.x Icon](https://ant.design/components/icon)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：通用（General）  
> 说明：语义化的矢量图形。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
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
