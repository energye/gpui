# Carousel 走马灯
> 来源：[Ant Design 6.5.x Carousel](https://ant.design/components/carousel)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据展示（Data Display）  
> 说明：一组轮播的区域。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
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
