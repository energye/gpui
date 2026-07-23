# Util 工具类
> 来源：[Ant Design 6.5.x Util](https://ant.design/components/util)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：其他（Other）  
> 说明：辅助开发，提供一些常用的工具方法。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。
---
## 1. 控件外观
### 1.1 基础形态

辅助开发，提供一些常用的工具方法。

**Util** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

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

实现与 antd **Util** 对等的业务能力。

### 2.2 核心功能（按官方示例拆解）

1. 基础渲染与配置能力。

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| — | — | — |

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

自 `5.13.0` 版本开始提供这些方法。

## GetRef

获取组件的 `ref` 属性定义，这对于未直接暴露或者子组件的 `ref` 属性定义非常有用。

```tsx
import { Select } from 'antd';
import type { GetRef } from 'antd';

type SelectRefType = GetRef<typeof Select>; // BaseSelectRef
```

## GetProps

获取组件的 `props` 属性定义：

```tsx
import { Checkbox } from 'antd';
import type { GetProps } from 'antd';

type CheckboxGroupType = GetProps<typeof Checkbox.Group>;
```

同时也支持获取 Context 的属性定义：

```tsx
import type { GetProps } from 'antd';

interface InternalContextProps {
  name: string;
}

const Context = React.createContext<InternalContextProps>({ name: 'Ant Design' });

type ContextType = GetProps<typeof Context>; // InternalContextProps
```

### 与 `React.ComponentProps` 的区别 {#react-componentprops-diff}

`React.ComponentProps` 是 React 官方提供的通用工具类型，用于获取原生标签或 React 组件接受的 props，例如 `React.ComponentProps<'button'>` 或 `React.ComponentProps<typeof Button>`，而 `GetProps` 则是 Ant Design 提供的补充类型：它不支持原生标签名，但除了 React 组件外，还可以直接获取 `React.Context` 的 value 类型，或者透传已经拿到的 props 类型对象。

## GetProp

获取组件的单个 `props` 或者 `context` 属性定义。它已经将 `NonNullable` 进行了封装，所以不用再考虑为空的情况：

```tsx
import { Select } from 'antd';
import type { GetProp, SelectProps } from 'antd';

// 以下两种都可以生效
type SelectOptionType1 = GetProp<SelectProps, 'options'>[number];
type SelectOptionType2 = GetProp<typeof Select, 'options'>[number];
type ContextOptionType = GetProp<typeof Context, 'name'>;
```

同时，支持通过第三个参数 `Return` 获取函数属性的返回值类型：

```tsx
import type { GetProp } from 'antd';

interface Props {
  func?: (value: number) => string;
  configOrFunc?: { configA?: string } | (() => { anotherB?: string });
}

type OnChangeReturn = GetProp<Props, 'func', 'Return'>; // string
type ClassNamesReturn = GetProp<Props, 'configOrFunc', 'Return'>; // { anotherB?: string }
```

### 导入方式

```ts
import type { GetRef, GetProps, GetProp } from 'antd';
```

---
## 4. gpui kit 实现要点
实现 gpui kit 版 **Util** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **0** 个，均需可复现。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/util
- 中文文档：https://ant.design/components/util-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/_util
- 驱动 gpui kit：`util`
