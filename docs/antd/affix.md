# Affix 固钉
> 来源：[Ant Design 6.5.x Affix](https://ant.design/components/affix)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：其他（Other）  
> 说明：将页面元素钉在可视范围。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

将页面元素钉在可视范围。

**Affix** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本 | 复现「基本」视觉与布局 |
| 固定状态改变的回调 | 固定头/列/侧栏 |
| 滚动容器 | 复现「滚动容器」视觉与布局 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `onChange`

- **说明**：固定状态改变时触发的回调函数
- **类型**：(affixed?: boolean) => void
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

当内容区域比较长，需要滚动页面时，这部分内容对应的操作或者导航需要在滚动范围内始终展现。常用于侧边菜单和按钮组合。

页面可视范围过小时，慎用此功能以免出现遮挡页面内容的情况。

> 开发者注意事项：
>
> 自 `5.10.0` 起，由于 Affix 组件由 class 重构为 FC，之前获取 `ref` 并调用内部实例方法的写法都会失效。

### 2.2 核心功能（按官方示例拆解）

1. **基本**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **固定状态改变的回调**（`on-change.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **滚动容器**（`target.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `onChange` | 值变化 | 固定状态改变时触发的回调函数 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本 | `basic.tsx` | 否 |
| 固定状态改变的回调 | `on-change.tsx` | 否 |
| 滚动容器 | `target.tsx` | 否 |
| 调整浏览器大小，观察 Affix 容器是否发生变化。跟随变化为正常。#17678 | `debug.tsx` | 是 |

### 2.6 FAQ

## FAQ

### Affix 使用 `target` 绑定容器时，元素会跑到容器外。 {#faq-target-container}

从性能角度考虑，我们只监听容器滚动事件。如果希望任意滚动，你可以在窗体添加滚动监听：

相关 issue：[#3938](https://github.com/ant-design/ant-design/issues/3938) [#5642](https://github.com/ant-design/ant-design/issues/5642) [#16120](https://github.com/ant-design/ant-design/issues/16120)

### Affix 在水平滚动容器中使用时， 元素 `left` 位置不正确。 {#faq-horizontal-scroll}

Affix 一般只适用于单向滚动的区域，只支持在垂直滚动容器中使用。如果希望在水平容器中使用，你可以考虑使用 原生 `position: sticky` 实现。

相关 issue: [#29108](https://github.com/ant-design/ant-design/issues/29108)

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
| offsetBottom | 距离窗口底部达到指定偏移量后触发 | number | - | offsetTop | 距离窗口顶部达到指定偏移量后触发 | number | 0 | target | 设置 `Affix` 需要监听其滚动事件的元素，值为一个返回对应 DOM 元素的函数 | () => Window \| HTMLElement \| null | () => window | onChange | 固定状态改变时触发的回调函数 | (affixed?: boolean) => void | - 
**注意：**`Affix` 内的元素不要使用绝对定位，如需要绝对定位的效果，可以直接设置 `Affix` 为绝对定位：

```jsx
<Affix style={{ position: 'absolute', top: y, left: x }}>...</Affix>
```

### 导入方式

```js
import { Affix } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `offsetBottom` | 距离窗口底部达到指定偏移量后触发 | number | - | — |
| `offsetTop` | 距离窗口顶部达到指定偏移量后触发 | number | 0 | — |
| `target` | 设置 `Affix` 需要监听其滚动事件的元素，值为一个返回对应 DOM 元素的函数 | () => Window \| HTMLElement \| null | () => window | — |
| `onChange` | 固定状态改变时触发的回调函数 | (affixed?: boolean) => void | - | — |

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Affix** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **3** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/affix
- 中文文档：https://ant.design/components/affix-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/affix
- 驱动 gpui kit：`affix`
