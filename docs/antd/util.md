# Util 工具类
> 来源：[Ant Design 6.5.x Util](https://ant.design/components/util)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：其他（Other）  
> 说明：辅助开发，提供一些常用的工具方法。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 说明（无外观）

Util 无 UI：`_util/` 工具箱 + `GetRef/GetProps/GetProp` 类型工具。不应进 kit，无视觉验收。
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
## 4. gpui 落地说明

> Util 不应进 `ui/kit`：类型工具按 §6.3 Go 对照，运行时纯函数按需入 `internal/`；无 UI，豁免 gallery。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/util
- 中文文档：https://ant.design/components/util-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/_util
- 驱动 gpui kit：`util`

---

## 6. Go 对照规格（非控件验收）

> Util 为纯工具，无 UI：类型工具给 Go 对照，运行时工具按取舍表取用。不应进 `ui/kit`，豁免 gallery。
> 真源：`/home/yanghy/app/projects/ant-design/components/_util/`（类型见 §3，工具箱见下表）。

### 6.1 对照定义

| antd | Go 对照 | 说明 |
| --- | --- | --- |
| `GetProps<typeof X>` | 泛型约束/具名参数 struct | 取组件 props 类型；Go 用泛型类型参或显式 `XProps` |
| `GetRef<typeof X>` | 句柄类型（如 `*Select`） | 取 ref 句柄；Go 直接用组件指针/接口 |
| `GetProp<P,K>` | 字段类型/函数返回值 | 取单字段；`Return` 变体取函数返回类型 |

### 6.2 Token/度量

无视觉 Token。纯函数不读 Theme；仅业务色判断（如 `isPresetColor`）复用 Theme 常量。

### 6.3 类型对照细则

| antd | Go 写法 |
| --- | --- |
| `GetProps<typeof Checkbox.Group>` | `CheckboxGroupProps` struct + 泛型约束 |
| `GetProps<typeof Context>` | context value struct 直用 |
| `GetRef<typeof Select>` | `*Select` 句柄 |
| `GetProp<P,'options'>[number]` | `Option` 元素类型 |
| `GetProp<P,'func','Return'>` | 函数返回类型具名 |

### 6.4 运行时工具取舍表（`_util/`）

| 移植（纯逻辑） | 不移植（浏览器/React） |
| --- | --- |
| `colors`（`isPresetColor/isPresetStatusColor`）、`is` 断言、`toList`、`transKeys`、`capitalize`、`easings` 数学、`getRenderPropValue` 语义 | `responsiveObserver`、`scrollTo/getScroll`、`wave`、`placements/dom-align`、`styleChecker`、`zindexContext/hooks` React 态、`throttleByAnimationFrame` |

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| UTL-S1 | `GetProps`→泛型约束对照存在 | 文档可查 |
| UTL-S2 | `GetRef`→句柄类型对照存在 | 文档可查 |
| UTL-S3 | 取舍表覆盖移植/不移植两列 | 有对照指引 |

### 6.5 视觉 chrome

不适用（无 UI）。

### 6.6 无障碍

不适用（无 UI）。

### 6.7 平台边界

| 能力 | 策略 |
| --- | --- |
| 类型工具（§6.3） | Go 对照，P0 |
| 纯函数（§6.4 左列） | 按需移植，P0 |
| 浏览器-only（§6.4 右列） | 不做 |

### 6.8 能力裁剪（P0 / P1）

#### P0

| 能力 | 说明 |
| --- | --- |
| 类型对照（§6.3） | 必须 |
| 纯函数移植（§6.4 左列） | 按需 |
| §6.9 UTL-01–UTL-03 | 通过 |

#### P1

| 能力 | 说明 |
| --- | --- |
| 浏览器-only（§6.4 右列） | 不做 |

### 6.9 验收用例表（可测）

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| UTL-01 | L1 | `GetProps` 对照可查 | 有泛型约束写法 |
| UTL-02 | L1 | `GetRef` 对照可查 | 有句柄类型写法 |
| UTL-03 | L1 | 取舍表可查 | 移植/不移植两列齐 |
| UTL-04 | P1 | 浏览器-only | 不做，Notes 标明 |

### 6.10 Go 对照（类型侧，无 kit 控件）

```text
// GetProps<typeof X> → 泛型约束或具名 Props
type CheckboxGroupProps[T any] struct { Options []T; Value []T }
// GetRef<typeof Select> → 句柄
type SelectHandle = *Select
// GetProp<P,K> → 字段类型
type SelectOption = GetPropEquivalent[SelectProps, Option]
```

### 6.11 落地分层（实现提示）

```text
internal/util/  纯函数（colors/is/toList/transKeys/…）
类型对照        各 kit 控件 Props/Handle 定义处注释引用 §6.3
```

- 不建 `ui/kit/util` 控件包；不进 gallery（无 UI，见 §6.12）。

### 6.12 完成定义（DoD）

同时满足即可宣布 **Util 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. **示例程序**：Util 无运行时 UI，**豁免** gallery（见 [README · ui_polish_gallery](./README.md#示例程序examplesui_polish_gallery强制)）；在 `coverage.go` Notes 标明「无 UI」。
6. `coverage.go` Notes：P0 已对齐 `docs/antd/util.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Util 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
