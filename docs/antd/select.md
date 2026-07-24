# Select 选择器
> 来源：[Ant Design 6.5.x Select](https://ant.design/components/select)  
> 版本依据：Ant Design **v6.5.1**  
> 分类：数据录入（Data Entry）  
> 说明：下拉选择器。  
> 用途：**gpui kit** 控件开发规格（外观 / 功能 / 配置对齐 antd 6.5）。

**1:1 产品验收（度量 / 状态机 / P0·P1 / 用例 / Go API）→ [§6](#6-11-产品需求增量gpui-验收规格)**。手写对齐 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
---
## 1. 控件外观
### 1.1 基础形态

下拉选择器。

**Select** 的视觉由结构层（根容器 / 内容 / 装饰 / 浮层）与状态层（default / hover / active / focus / disabled / loading 等）组成。gpui kit 实现时需与 antd **6.5** 的尺寸节奏、圆角、颜色语义对齐。

### 1.2 文档示例对应的外观形态

| 示例名 | 形态/状态要点（kit 验收） |
| --- | --- |
| 基本使用 | 复现「基本使用」视觉与布局 |
| 带搜索框 | 带搜索框外观 |
| 自定义搜索 | 带搜索框外观 |
| 多字段搜索 | 带搜索框外观 |
| 多选 | 多选标签/勾选外观 |
| 三种大小 | 不同 size 档位 |
| 自定义下拉选项 | 自定义渲染/插槽外观 |
| 带排序的搜索 | 带搜索框外观 |
| 标签 | 复现「标签」视觉与布局 |
| 分组 | Group 组合外观 |
| 联动 | 复现「联动」视觉与布局 |
| 获得选项的文本 | 复现「获得选项的文本」视觉与布局 |
| 自动分词 | 复现「自动分词」视觉与布局 |
| 自定义分词 | 自定义渲染/插槽外观 |
| 搜索用户 | 带搜索框外观 |
| 前后缀 | 复现「前后缀」视觉与布局 |
| 扩展菜单 | 复现「扩展菜单」视觉与布局 |
| 隐藏已选择选项 | 复现「隐藏已选择选项」视觉与布局 |
| 形态变体 | variant 线框/填充差异 |
| 自定义选择标签 | 自定义渲染/插槽外观 |
| 自定义选中 label | 自定义渲染/插槽外观 |
| 响应式 maxTagCount | 断点响应式 |
| 大数据 | 复现「大数据」视觉与布局 |
| 自定义状态 | 自定义渲染/插槽外观 |
| 弹出位置 | placement 方位 |
| 最大选中数量 | 复现「最大选中数量」视觉与布局 |
| 自定义语义结构的样式和类 | 自定义渲染/插槽外观 |

### 1.3 外观相关配置逐项说明

下列配置会改变绘制结果，kit 应建立样式枚举或 token 映射：

#### `allowClear`

- **说明**：自定义清除按钮
- **类型**：boolean | { clearIcon?: ReactNode }
- **默认值**：false
- **版本**：5.8.0: 支持对象类型

#### `bordered`

- **说明**：是否带边框，请使用 `variant` 替代
- **类型**：boolean
- **默认值**：true
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `variant` | 官方取值 `variant` |

#### `classNames`

- **说明**：用于自定义 Select 组件内部各语义化结构的 class，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `disabled`

- **说明**：是否禁用
- **类型**：boolean
- **默认值**：false

#### `labelInValue`

- **说明**：是否把每个选项的 label 包装到 value 中，会把 Select 的 value 类型从 `string` 变为 { value: string, label: ReactNode } 的格式
- **类型**：boolean
- **默认值**：false

#### `loading`

- **说明**：加载中状态
- **类型**：boolean
- **默认值**：false

#### `loadingIcon`

- **说明**：自定义的加载图标
- **类型**：ReactNode
- **默认值**：``
- **版本**：6.4.0

#### `maxCount`

- **说明**：指定可选中的最多 items 数量，仅在 `mode` 为 `multiple` 或 `tags` 时生效
- **类型**：number
- **默认值**：-
- **版本**：5.13.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `mode` | 官方取值 `mode` |
  | `multiple` | 多选 |
  | `tags` | 标签模式 |

#### `menuItemSelectedIcon`

- **说明**：自定义多选时当前选中的条目图标
- **类型**：ReactNode
- **默认值**：``

#### `mode`

- **说明**：设置 Select 的模式为多选或标签
- **类型**：`multiple` | `tags`
- **默认值**：-
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `multiple` | 多选 |
  | `tags` | 标签模式 |

#### `placement`

- **说明**：选择框弹出的位置
- **类型**：`bottomLeft` `bottomRight` `topLeft` `topRight`
- **默认值**：bottomLeft
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `bottomLeft` | 下左 |
  | `bottomRight` | 下右 |
  | `topLeft` | 上左 |
  | `topRight` | 上右 |

#### `prefix`

- **说明**：自定义前缀
- **类型**：ReactNode
- **默认值**：-
- **版本**：5.22.0

#### `removeIcon`

- **说明**：自定义的多选框清除图标
- **类型**：ReactNode
- **默认值**：``

#### `showArrow`

- **说明**：是否显示箭头图标，请使用 `suffixIcon={null}` 替代
- **类型**：boolean
- **默认值**：true

#### `size`

- **说明**：选择框大小
- **类型**：`large` | `medium` | `small`
- **默认值**：`medium`
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `large` | 大尺寸（更高/更大字号/更宽内边距） |
  | `medium` | 中尺寸（默认节奏） |
  | `small` | 小尺寸（更紧凑） |

#### `status`

- **说明**：设置校验状态
- **类型**：'error' | 'warning'
- **默认值**：-
- **版本**：4.19.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `error` | 错误红语义 |
  | `warning` | 警告橙语义 |

#### `styles`

- **说明**：用于自定义 Select 组件内部各语义化结构的行内 style，支持对象或函数
- **类型**：Record | (info: { props }) => Record
- **默认值**：-

#### `suffixIcon`

- **说明**：自定义的选择框后缀图标。以防止图标被用于其他交互，替换的图标默认不会响应展开、收缩事件，可以通过添加 `pointer-events: none` 样式透传。
- **类型**：ReactNode
- **默认值**：``

#### `variant`

- **说明**：形态变体
- **类型**：`outlined` | `borderless` | `filled` | `underlined`
- **默认值**：`outlined`
- **版本**：5.13.0 | `underlined`: 5.24.0
- **可选值与外观含义**：

  | 值 | 外观/语义 |
  | --- | --- |
  | `outlined` | 描边空心 |
  | `borderless` | 无边框 |
  | `filled` | 浅底填充 |
  | `underlined` | 底边线形态 |

#### `searchIcon`

- **说明**：自定义的搜索图标
- **类型**：ReactNode
- **默认值**：``
- **版本**：6.4.0

#### `title`

- **说明**：选项上的原生 title 提示
- **类型**：string
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

- 弹出一个下拉菜单给用户选择操作，用于代替原生的选择器，或者需要一个更优雅的多选器时。
- 当选项少时（少于 5 项），建议直接将选项平铺，使用 [Radio](/components/radio-cn/) 是更好的选择。
- 如果你在寻找一个可输可选的输入框，那你可能需要 [AutoComplete](/components/auto-complete-cn/)。

### 2.2 核心功能（按官方示例拆解）

1. **基本使用**（`basic.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
2. **带搜索框**（`search.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
3. **自定义搜索**（`search-filter-option.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
4. **多字段搜索**（`search-multi-field.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
5. **多选**（`multiple.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
6. **三种大小**（`size.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
7. **自定义下拉选项**（`option-render.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
8. **带排序的搜索**（`search-sort.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
9. **标签**（`tags.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
10. **分组**（`optgroup.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
11. **联动**（`coordinate.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
12. **获得选项的文本**（`label-in-value.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
13. **自动分词**（`automatic-tokenization.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
14. **自定义分词**（`custom-tokenization.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
15. **搜索用户**（`select-users.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
16. **前后缀**（`suffix.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
17. **扩展菜单**（`custom-dropdown-menu.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
18. **隐藏已选择选项**（`hide-selected.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
19. **形态变体**（`variant.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
20. **自定义选择标签**（`custom-tag-render.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
21. **自定义选中 label**（`custom-label-render.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
22. **响应式 maxTagCount**（`responsive.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
23. **大数据**（`big-data.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
24. **自定义状态**（`status.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
25. **弹出位置**（`placement.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
26. **最大选中数量**（`maxCount.tsx`）— kit 需用对等 API 复现该示例的交互与展示。
27. **自定义语义结构的样式和类**（`style-class.tsx`）— kit 需用对等 API 复现该示例的交互与展示。

### 2.3 行为 API 能力

| API | 能力 | 说明 |
| --- | --- | --- |
| `value` | 受控值 | 指定当前选中的条目，多选时为一个数组。（value 数组引用未变化时，Select 不会更新） |
| `defaultValue` | 非受控默认值 | 指定默认选中的条目 |
| `onChange` | 值变化 | 选中 option，或 input 的 value 变化时，调用此函数 |
| `onSelect` | 选中 | 被选中时调用，参数为选中项的 value (或 key) 值 |
| `onDeselect` | 取消选中 | 取消选中时调用，参数为选中项的 value (或 key) 值，仅在 `multiple` 或 `tags` 模式下生效 |
| `open` | 受控显隐 | 是否展开下拉菜单 |
| `onOpenChange` | 显隐变化 | 展开下拉菜单的回调 |
| `disabled` | 禁用 | 是否禁用 |
| `loading` | 加载中 | 加载中状态 |
| `options` | 数据化 options | 数据化配置选项内容，相比 jsx 定义会获得更好的渲染性能 |
| `showSearch` | 搜索 | 配置是否可搜索 |
| `filterOption` | 过滤 | 是否根据输入项进行筛选。当其为一个函数时，会接收 `inputValue` `option` 两个参数，当 `option` 符合筛选条件时，应返回 true，反之则返回 false。[示例](#select-demo-search) |
| `virtual` | 虚拟滚动 | 设置 false 时关闭虚拟滚动 |
| `getPopupContainer` | 浮层容器 | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例](https://codesandbox.io/s/4j168r7jw0) |
| `onSearch` | 搜索回调 | 文本框值变化时回调 |
| `onClear` | 清除 | 清除内容时回调 |

### 2.4 示例全表

| 示例 | 源文件 | debug |
| --- | --- | --- |
| 基本使用 | `basic.tsx` | 否 |
| 带搜索框 | `search.tsx` | 否 |
| 自定义搜索 | `search-filter-option.tsx` | 否 |
| 多字段搜索 | `search-multi-field.tsx` | 否 |
| 多选 | `multiple.tsx` | 否 |
| 三种大小 | `size.tsx` | 否 |
| 自定义下拉选项 | `option-render.tsx` | 否 |
| 带排序的搜索 | `search-sort.tsx` | 否 |
| 标签 | `tags.tsx` | 否 |
| 分组 | `optgroup.tsx` | 否 |
| 联动 | `coordinate.tsx` | 否 |
| 获得选项的文本 | `label-in-value.tsx` | 否 |
| 自动分词 | `automatic-tokenization.tsx` | 否 |
| 自定义分词 | `custom-tokenization.tsx` | 否 |
| 搜索用户 | `select-users.tsx` | 否 |
| 前后缀 | `suffix.tsx` | 否 |
| 扩展菜单 | `custom-dropdown-menu.tsx` | 否 |
| 隐藏已选择选项 | `hide-selected.tsx` | 否 |
| 形态变体 | `variant.tsx` | 否 |
| Filled debug | `filled-debug.tsx` | 是 |
| 自定义选择标签 | `custom-tag-render.tsx` | 否 |
| 自定义选中 label | `custom-label-render.tsx` | 否 |
| 响应式 maxTagCount | `responsive.tsx` | 否 |
| 大数据 | `big-data.tsx` | 否 |
| 自定义状态 | `status.tsx` | 否 |
| 弹出位置 | `placement.tsx` | 否 |
| 动态高度 | `placement-debug.tsx` | 是 |
| Debug 专用 | `debug.tsx` | 是 |
| \_InternalPanelDoNotUseOrYouWillBeFired | `render-panel.tsx` | 是 |
| 选项文本居中 | `option-label-center.tsx` | 是 |
| 翻转+偏移 | `debug-flip-shift.tsx` | 是 |
| 组件 Token | `component-token.tsx` | 是 |
| 最大选中数量 | `maxCount.tsx` | 否 |
| 自定义语义结构的样式和类 | `style-class.tsx` | 否 |

### 2.6 FAQ

## FAQ

### `mode="tags"` 模式下为何搜索有时会出现两个相同选项？ {#faq-tags-mode-duplicate}

这一般是 `options` 中的 `label` 和 `value` 不同导致的，你可以通过 `optionFilterProp="label"` 将过滤设置为展示值以避免这种情况。

### 点击 `popupRender` 里的元素，下拉菜单不会自动消失？ {#faq-popup-not-close}

你可以使用受控模式，手动设置 `open` 属性：[codesandbox](https://codesandbox.io/s/ji-ben-shi-yong-antd-4-21-7-forked-gnp4cy?file=/demo.js)。

### 反过来希望点击 `popupRender` 里元素不消失该怎么办？ {#faq-popup-keep-open}

Select 当失去焦点时会关闭下拉框，你可以通过阻止默认行为来避免丢失焦点导致的关闭：

```tsx
 (
     {
        e.preventDefault();
        e.stopPropagation();
      }}
    >
      Some Content
    
  )}
/>
```

### 自定义 Option 样式导致滚动异常怎么办？ {#faq-custom-option-scroll}

这是由于虚拟滚动默认选项高度为 `24px`，如果你的选项高度小于该值则需要通过 `listItemHeight` 属性调整，而 `listHeight` 用于设置滚动容器高度：

```tsx

```

注意：`listItemHeight` 和 `listHeight` 为内部属性，如无必要，请勿修改该值。

### 为何无障碍测试会报缺失 `aria-` 属性？ {#faq-aria-attribute}

Select 无障碍辅助元素仅在弹窗展开时创建，因而当你在进行无障碍检测时请先打开下拉后再进行测试。对于 `aria-label` 与 `aria-labelledby` 属性缺失警告，请自行为 Select 组件添加相应无障碍属性。

Select 虚拟滚动会模拟无障碍绑定元素。如果需要读屏器完整获取全部列表，你可以设置 `virtual={false}` 关闭虚拟滚动，无障碍选项将会绑定到真实元素上。

### 使用 `tagRender` 生成的自定义标签，点击关闭时会呼出下拉框 {#faq-tagrender-dropdown}

如果你不希望点击某个元素后下拉框自动出现（例如关闭按钮），可以在其上阻止 `MouseDown` 事件的传播。

```tsx
 {
    const { closable, label, onClose } = props;
    return (
      
        {label}
        {closable ? (
           e.stopPropagation()}
            onClick={onClose}
            className="cursor-pointer"
          >
            ❎
          
        ) : null}
      
    );
  }}
/>
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

通用属性参考：[通用属性](/docs/react/common-props)

### Select props

| 参数 | 说明 | 类型 | 默认值 | 版本 | [全局配置](/components/config-provider-cn#component-config) |
| --- | --- | --- | --- | --- | --- |
| allowClear | 自定义清除按钮 | boolean \| { clearIcon?: ReactNode } | false | 5.8.0: 支持对象类型 | 6.4.0 |
| ~~autoClearSearchValue~~ | 是否在选中项后清空搜索框，只在 `mode` 为 `multiple` 或 `tags` 时有效 | boolean | true | ~~bordered~~ | 是否带边框，请使用 `variant` 替代 | boolean | true | - | × |
| classNames | 用于自定义 Select 组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), string> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> | - | defaultActiveFirstOption | 是否默认高亮第一个选项 | boolean | true | defaultOpen | 是否默认展开下拉菜单 | boolean | - | defaultValue | 指定默认选中的条目 | string \| string\[] \|<br />number \| number\[] \| <br />LabeledValue \| LabeledValue\[] | - | disabled | 是否禁用 | boolean | false | ~~dropdownClassName~~ | 下拉菜单的 className 属性，请使用 `classNames.popup.root` 替代 | string | - | - | × |
| ~~dropdownMatchSelectWidth~~ | 下拉菜单和选择器是否同宽，请使用 `popupMatchSelectWidth` 替代 | boolean \| number | true | - | × |
| ~~popupClassName~~ | 下拉菜单的 className 属性，使用 `classNames.popup.root` 替换 | string | - | 4.23.0 | × |
| popupMatchSelectWidth | 下拉菜单和选择器同宽。默认将设置 `min-width`，当值小于选择框宽度时会被忽略。false 时会关闭虚拟滚动 | boolean \| number | true | 5.5.0 | × |
| ~~dropdownRender~~ | 自定义下拉框内容，使用 `popupRender` 替换 | (originNode: ReactNode) => ReactNode | - | popupRender | 自定义下拉框内容 | (originNode: ReactNode) => ReactNode | - | 5.25.0 | × |
| ~~dropdownStyle~~ | 下拉菜单的 style 属性，使用 `styles.popup.root` 替换 | CSSProperties | - | fieldNames | 自定义节点 label、value、options、groupLabel 的字段 | object | { label: `label`, value: `value`, options: `options`, groupLabel: `label` } | 4.17.0（`groupLabel` 在 5.6.0 新增） | × |
| ~~filterOption~~ | 是否根据输入项进行筛选。当其为一个函数时，会接收 `inputValue` `option` 两个参数，当 `option` 符合筛选条件时，应返回 true，反之则返回 false。[示例](#select-demo-search) | boolean \| function(inputValue, option) | true | ~~filterSort~~ | 搜索时对筛选结果项的排序函数, 类似[Array.sort](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Array/sort)里的 compareFunction | (optionA: Option, optionB: Option, info: { searchValue: string }) => number | - | `searchValue`: 5.19.0 | × |
| getPopupContainer | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例](https://codesandbox.io/s/4j168r7jw0) | function(triggerNode) | () => document.body | labelInValue | 是否把每个选项的 label 包装到 value 中，会把 Select 的 value 类型从 `string` 变为 { value: string, label: ReactNode } 的格式 | boolean | false | listHeight | 设置弹窗滚动高度 | number | 256 | loading | 加载中状态 | boolean | false | loadingIcon | 自定义的加载图标 | ReactNode | `<LoadingOutlined spin />` | 6.4.0 | 6.4.0 |
| maxCount | 指定可选中的最多 items 数量，仅在 `mode` 为 `multiple` 或 `tags` 时生效 | number | - | 5.13.0 | × |
| maxTagCount | 最多显示多少个 tag，响应式模式会对性能产生损耗 | number \| `responsive` | - | responsive: 4.10 | × |
| maxTagPlaceholder | 隐藏 tag 时显示的内容 | ReactNode \| function(omittedValues) | - | maxTagTextLength | 最大显示的 tag 文本长度 | number | - | menuItemSelectedIcon | 自定义多选时当前选中的条目图标 | ReactNode | `<CheckOutlined />` | mode | 设置 Select 的模式为多选或标签 | `multiple` \| `tags` | - | notFoundContent | 当下拉列表为空时显示的内容 | ReactNode | `Not Found` | open | 是否展开下拉菜单 | boolean | - | ~~optionFilterProp~~ | 已废弃，见 `showSearch.optionFilterProp` | optionLabelProp | 回填到选择框的 Option 的属性值，默认是 Option 的子元素。比如在子元素需要高亮效果时，此值可以设为 `value`。[示例](https://codesandbox.io/s/antd-reproduction-template-tk678) | string | `children` | options | 数据化配置选项内容，相比 jsx 定义会获得更好的渲染性能 | { label, value }\[] | - | optionRender | 自定义渲染下拉选项 | (option: FlattenOptionData\<BaseOptionType\> , info: { index: number }) => React.ReactNode | - | 5.11.0 | × |
| placeholder | 选择框默认文本 | ReactNode | - | placement | 选择框弹出的位置 | `bottomLeft` `bottomRight` `topLeft` `topRight` | bottomLeft | prefix | 自定义前缀 | ReactNode | - | 5.22.0 | × |
| removeIcon | 自定义的多选框清除图标 | ReactNode | `<CloseOutlined />` | ~~searchValue~~ | 控制搜索文本 | string | - | ~~showArrow~~ | 是否显示箭头图标，请使用 `suffixIcon={null}` 替代 | boolean | true | - | × |
| showSearch | 配置是否可搜索 | boolean \| [Object](#showsearch) | 单选为 false，多选为 true | Object: 6.0.0 | 6.4.0 |
| size | 选择框大小 | `large` \| `medium` \| `small` | `medium` | status | 设置校验状态 | 'error' \| 'warning' | - | 4.19.0 | × |
| styles | 用于自定义 Select 组件内部各语义化结构的行内 style，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), CSSProperties> \| (info: { props }) => Record<[SemanticDOM](#semantic-dom), CSSProperties> | - | suffixIcon | 自定义的选择框后缀图标。以防止图标被用于其他交互，替换的图标默认不会响应展开、收缩事件，可以通过添加 `pointer-events: none` 样式透传。 | ReactNode | `<DownOutlined />` | tagRender | 自定义 tag 内容 render，仅在 `mode` 为 `multiple` 或 `tags` 时生效 | (props) => ReactNode | - | labelRender | 自定义当前选中的 label 内容 render （LabelInValueType的定义见 [LabelInValueType](https://github.com/react-component/select/blob/b39c28aa2a94e7754ebc570f200ab5fd33bd31e7/src/Select.tsx#L70)） | (props: LabelInValueType) => ReactNode | - | 5.15.0 | × |
| tokenSeparators | 自动分词的分隔符或自定义分词函数，仅在 `mode="tags"` 或 `mode="multiple"` 时生效 | string[] \| ((input: string) => string[]) | - | function: 6.5.0 | × |
| value | 指定当前选中的条目，多选时为一个数组。（value 数组引用未变化时，Select 不会更新） | string \| string\[] \| <br />number \| number\[] \| <br />LabeledValue \| LabeledValue\[] | - | variant | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 | 5.19.0 |
| virtual | 设置 false 时关闭虚拟滚动 | boolean | true | 4.1.0 | × |
| onActive | 键盘和鼠标交互时触发 | function(value: string \| number \| LabeledValue) | - | onBlur | 失去焦点时回调 | function | - | onChange | 选中 option，或 input 的 value 变化时，调用此函数 | function(value, option:Option \| Array&lt;Option>) | - | onClear | 清除内容时回调 | function | - | 4.6.0 | × |
| onDeselect | 取消选中时调用，参数为选中项的 value (或 key) 值，仅在 `multiple` 或 `tags` 模式下生效 | function(value: string \| number \| LabeledValue) | - | ~~onDropdownVisibleChange~~ | 展开下拉菜单的回调，使用 `onOpenChange` 替换 | (open: boolean) => void | - | onOpenChange | 展开下拉菜单的回调 | (open: boolean) => void | - | onFocus | 获得焦点时回调 | (event: FocusEvent) => void | - | onInputKeyDown | 按键按下时回调 | (event: KeyboardEvent) => void | - | onPopupScroll | 下拉列表滚动时的回调 | (event: UIEvent) => void | - | ~~onSearch~~ | 文本框值变化时回调 | function(value: string) | - | onSelect | 被选中时调用，参数为选中项的 value (或 key) 值 | function(value: string \| number \| LabeledValue, option: Option) | - 
> 注意，如果发现下拉菜单跟随页面滚动，或者需要在其他弹层中触发 Select，请尝试使用 `getPopupContainer={triggerNode => triggerNode.parentElement}` 将下拉弹层渲染节点固定在触发器的父元素中。

### showSearch

| 参数 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| autoClearSearchValue | 是否在选中项后清空搜索框，只在 `mode` 为 `multiple` 或 `tags` 时有效 | boolean | true | filterSort | 搜索时对筛选结果项的排序函数, 类似[Array.sort](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Array/sort)里的 compareFunction | (optionA: Option, optionB: Option, info: { searchValue: string }) => number | - | `searchValue`: 5.19.0 |
| optionFilterProp | 搜索时过滤对应的 `option` 属性，如设置为 `children` 表示对内嵌内容进行搜索。<br/> 若通过 `options` 属性配置选项内容，建议设置 `optionFilterProp="label"` 来对内容进行搜索。<br/> 当传入 `string[]` 时多个字段进行 OR 匹配搜索 | string \| string[] | `value` | `string[]`: 6.1.0 |
| searchValue | 控制搜索文本 | string | - | searchIcon | 自定义的搜索图标 | ReactNode | `<SearchOutlined />` | 6.4.0 |

### Select Methods

| 名称    | 说明     | 版本 |
| ------- | -------- | ---- |
| blur()  | 取消焦点 |      |
| focus() | 获取焦点 |      |

### Option props

| 参数      | 说明                     | 类型             | 默认值 | 版本 |
| --------- | ------------------------ | ---------------- | ------ | ---- |
| className | Option 器类名            | string           | -      |      |
| disabled  | 是否禁用                 | boolean          | false  |      |
| title     | 选项上的原生 title 提示  | string           | -      |      |
| value     | 默认根据此属性值进行筛选 | string \| number | -      |      |

### OptGroup props

| 参数      | 说明                    | 类型            | 默认值 | 版本 |
| --------- | ----------------------- | --------------- | ------ | ---- |
| key       | Key                     | string          | -      |      |
| label     | 组名                    | React.ReactNode | -      |      |
| className | Option 器类名           | string          | -      |      |
| title     | 选项上的原生 title 提示 | string          | -      |      |

### 导入方式

```js
import { Select } from 'antd';
```

### 配置项速查（解析自 API 表）

| 配置项 | 说明 | 类型 | 默认值 | 版本 |
| --- | --- | --- | --- | --- |
| `allowClear` | 自定义清除按钮 | boolean \| { clearIcon?: ReactNode } | false | 5.8.0: 支持对象类型 |
| `autoClearSearchValue` | 是否在选中项后清空搜索框，只在 `mode` 为 `multiple` 或 `tags` 时有效 | boolean | true | — |
| `bordered` | 是否带边框，请使用 `variant` 替代 | boolean | true | - |
| `classNames` | 用于自定义 Select 组件内部各语义化结构的 class，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `defaultActiveFirstOption` | 是否默认高亮第一个选项 | boolean | true | — |
| `defaultOpen` | 是否默认展开下拉菜单 | boolean | - | — |
| `defaultValue` | 指定默认选中的条目 | string \| string\[] \|number \| number\[] \| LabeledValue \| LabeledValue\[] | - | — |
| `disabled` | 是否禁用 | boolean | false | — |
| `dropdownClassName` | 下拉菜单的 className 属性，请使用 `classNames.popup.root` 替代 | string | - | - |
| `dropdownMatchSelectWidth` | 下拉菜单和选择器是否同宽，请使用 `popupMatchSelectWidth` 替代 | boolean \| number | true | - |
| `popupClassName` | 下拉菜单的 className 属性，使用 `classNames.popup.root` 替换 | string | - | 4.23.0 |
| `popupMatchSelectWidth` | 下拉菜单和选择器同宽。默认将设置 `min-width`，当值小于选择框宽度时会被忽略。false 时会关闭虚拟滚动 | boolean \| number | true | 5.5.0 |
| `dropdownRender` | 自定义下拉框内容，使用 `popupRender` 替换 | (originNode: ReactNode) => ReactNode | - | — |
| `popupRender` | 自定义下拉框内容 | (originNode: ReactNode) => ReactNode | - | 5.25.0 |
| `dropdownStyle` | 下拉菜单的 style 属性，使用 `styles.popup.root` 替换 | CSSProperties | - | — |
| `fieldNames` | 自定义节点 label、value、options、groupLabel 的字段 | object | { label: `label`, value: `value`, options: `options`, groupLabel: `label` } | 4.17.0（`groupLabel` 在 5.6.0 新增） |
| `filterOption` | 是否根据输入项进行筛选。当其为一个函数时，会接收 `inputValue` `option` 两个参数，当 `option` 符合筛选条件时，应返回 true，反之则返回 false。[示例](#select-demo-search) | boolean \| function(inputValue, option) | true | — |
| `filterSort` | 搜索时对筛选结果项的排序函数, 类似[Array.sort](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Array/sort)里的 compareFunction | (optionA: Option, optionB: Option, info: { searchValue: string }) => number | - | `searchValue`: 5.19.0 |
| `getPopupContainer` | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例](https://codesandbox.io/s/4j168r7jw0) | function(triggerNode) | () => document.body | — |
| `labelInValue` | 是否把每个选项的 label 包装到 value 中，会把 Select 的 value 类型从 `string` 变为 { value: string, label: ReactNode } 的格式 | boolean | false | — |
| `listHeight` | 设置弹窗滚动高度 | number | 256 | — |
| `loading` | 加载中状态 | boolean | false | — |
| `loadingIcon` | 自定义的加载图标 | ReactNode | `` | 6.4.0 |
| `maxCount` | 指定可选中的最多 items 数量，仅在 `mode` 为 `multiple` 或 `tags` 时生效 | number | - | 5.13.0 |
| `maxTagCount` | 最多显示多少个 tag，响应式模式会对性能产生损耗 | number \| `responsive` | - | responsive: 4.10 |
| `maxTagPlaceholder` | 隐藏 tag 时显示的内容 | ReactNode \| function(omittedValues) | - | — |
| `maxTagTextLength` | 最大显示的 tag 文本长度 | number | - | — |
| `menuItemSelectedIcon` | 自定义多选时当前选中的条目图标 | ReactNode | `` | — |
| `mode` | 设置 Select 的模式为多选或标签 | `multiple` \| `tags` | - | — |
| `notFoundContent` | 当下拉列表为空时显示的内容 | ReactNode | `Not Found` | — |
| `open` | 是否展开下拉菜单 | boolean | - | — |
| `optionFilterProp` | 已废弃，见 `showSearch.optionFilterProp` | — | — | — |
| `optionLabelProp` | 回填到选择框的 Option 的属性值，默认是 Option 的子元素。比如在子元素需要高亮效果时，此值可以设为 `value`。[示例](https://codesandbox.io/s/antd-reproduction-template-tk678) | string | `children` | — |
| `options` | 数据化配置选项内容，相比 jsx 定义会获得更好的渲染性能 | { label, value }\[] | - | — |
| `optionRender` | 自定义渲染下拉选项 | (option: FlattenOptionData\ , info: { index: number }) => React.ReactNode | - | 5.11.0 |
| `placeholder` | 选择框默认文本 | ReactNode | - | — |
| `placement` | 选择框弹出的位置 | `bottomLeft` `bottomRight` `topLeft` `topRight` | bottomLeft | — |
| `prefix` | 自定义前缀 | ReactNode | - | 5.22.0 |
| `removeIcon` | 自定义的多选框清除图标 | ReactNode | `` | — |
| `searchValue` | 控制搜索文本 | string | - | — |
| `showArrow` | 是否显示箭头图标，请使用 `suffixIcon={null}` 替代 | boolean | true | - |
| `showSearch` | 配置是否可搜索 | boolean \| [Object](#showsearch) | 单选为 false，多选为 true | Object: 6.0.0 |
| `size` | 选择框大小 | `large` \| `medium` \| `small` | `medium` | — |
| `status` | 设置校验状态 | 'error' \| 'warning' | - | 4.19.0 |
| `styles` | 用于自定义 Select 组件内部各语义化结构的行内 style，支持对象或函数 | Record \| (info: { props }) => Record | - | — |
| `suffixIcon` | 自定义的选择框后缀图标。以防止图标被用于其他交互，替换的图标默认不会响应展开、收缩事件，可以通过添加 `pointer-events: none` 样式透传。 | ReactNode | `` | — |
| `tagRender` | 自定义 tag 内容 render，仅在 `mode` 为 `multiple` 或 `tags` 时生效 | (props) => ReactNode | - | — |
| `labelRender` | 自定义当前选中的 label 内容 render （LabelInValueType的定义见 [LabelInValueType](https://github.com/react-component/select/blob/b39c28aa2a94e7754ebc570f200ab5fd33bd31e7/src/Select.tsx#L70)） | (props: LabelInValueType) => ReactNode | - | 5.15.0 |
| `tokenSeparators` | 自动分词的分隔符或自定义分词函数，仅在 `mode="tags"` 或 `mode="multiple"` 时生效 | string[] \| ((input: string) => string[]) | - | function: 6.5.0 |
| `value` | 指定当前选中的条目，多选时为一个数组。（value 数组引用未变化时，Select 不会更新） | string \| string\[] \| number \| number\[] \| LabeledValue \| LabeledValue\[] | - | — |
| `variant` | 形态变体 | `outlined` \| `borderless` \| `filled` \| `underlined` | `outlined` | 5.13.0 \| `underlined`: 5.24.0 |
| `virtual` | 设置 false 时关闭虚拟滚动 | boolean | true | 4.1.0 |
| `onActive` | 键盘和鼠标交互时触发 | function(value: string \| number \| LabeledValue) | - | — |
| `onBlur` | 失去焦点时回调 | function | - | — |
| `onChange` | 选中 option，或 input 的 value 变化时，调用此函数 | function(value, option:Option \| Array<Option>) | - | — |
| `onClear` | 清除内容时回调 | function | - | 4.6.0 |
| `onDeselect` | 取消选中时调用，参数为选中项的 value (或 key) 值，仅在 `multiple` 或 `tags` 模式下生效 | function(value: string \| number \| LabeledValue) | - | — |
| `onDropdownVisibleChange` | 展开下拉菜单的回调，使用 `onOpenChange` 替换 | (open: boolean) => void | - | — |
| `onOpenChange` | 展开下拉菜单的回调 | (open: boolean) => void | - | — |
| `onFocus` | 获得焦点时回调 | (event: FocusEvent) => void | - | — |
| `onInputKeyDown` | 按键按下时回调 | (event: KeyboardEvent) => void | - | — |
| `onPopupScroll` | 下拉列表滚动时的回调 | (event: UIEvent) => void | - | — |
| `onSearch` | 文本框值变化时回调 | function(value: string) | - | — |
| `onSelect` | 被选中时调用，参数为选中项的 value (或 key) 值 | function(value: string \| number \| LabeledValue, option: Option) | - | — |
| `searchIcon` | 自定义的搜索图标 | ReactNode | `` | 6.4.0 |
| `blur()` | 取消焦点 | — | — | — |
| `focus()` | 获取焦点 | — | — | — |
| `className` | Option 器类名 | string | - | — |
| `title` | 选项上的原生 title 提示 | string | - | — |
| `key` | Key | string | - | — |
| `label` | 组名 | React.ReactNode | - | — |

---
## 4. gpui kit 实现要点

> 1:1 验收以 **§6** 为准；本节为工程纪律补充。

实现 gpui kit 版 **Select** 的验收清单：

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
11. **示例矩阵**：官方非 debug 示例约 **27** 个，均需可复现。
12. **弹层专项**：autoAdjustOverflow、点击外部关闭、destroyOnHidden。

---
## 5. 参考链接
- 官方文档：https://ant.design/components/select
- 中文文档：https://ant.design/components/select-cn
- 源码：https://github.com/ant-design/ant-design/tree/master/components/select
- 驱动 gpui kit：`select`

---

## 6. 1:1 产品需求增量（gpui 验收规格）

> 本章把 antd **Select** 补成 **可开发、可测试、可裁剪** 的产品规格。  
> **1:1 含义**：与 Ant Design **6.5** 桌面主路径在行为与设计体系上对齐；**不是**与浏览器 ant.design 逐像素哈希一致（见 L1–L4）。  
> **手写对齐** [Button §6](./button.md#6-11-产品需求增量gpui-验收规格) 模板细度（度量档、状态机规则 ID、chrome、P0/P1、可测用例、Go API、DoD）。  
> 源码：`/home/yanghy/app/projects/ant-design/components/select/`（`index.zh-CN.md` + `style/` + 组件实现）。

### 6.1 对齐级别定义（Select）

| 级别 | 名称 | 本控件含义 | 验收方式 |
| --- | --- | --- | --- |
| **L1** | 行为 | 受控输入/选择、弹层、清除、校验 status、尺寸档 | Headless / behavior 测试 |
| **L2** | Token / 几何 | 尺寸与颜色走 Theme；符合 §6.2 | Token 断言 / 布局测 |
| **L3** | 本库 golden | 固定字体、`scale=1`、关键态截图与基线一致（AA 容差） | golden / visualtest |
| **L4** | 人眼气质 | 与 ant.design 并排「一眼同系」 | 建/大改基线时人眼签字 |

**明确不做（Select）：**

- 与浏览器渲染 ant.design **逐像素哈希**一致。  
- 为抠图破坏 `hit == layout == paint` 边界。  
- 浏览器-only 且桌面无等价映射的 API（见 §6.7，标 P1/不做）。  
- 官方 **debug** 示例不计入 P0 验收。  

> 控件说明：下拉选择器。

### 6.2 度量与 Design Token（L2 基线）

数值以 **Ant Design 默认算法 + 本库 Theme 默认** 为准（`scale=1`，常用种子：`controlHeight=32`、`fontSize=14`）。实现必须通过 Token 读取；下表为 Token 未覆盖时的回落。

#### 6.2.1 几何与组件 Token

| 项 | 默认值 | Token / 来源 |
| --- | --- | --- |
| 选择器高 | **32/24/40** | controlHeight* |
| listHeight | **256** | 常见默认 |
| 控件高度 middle | **32** | `controlHeight` |
| 控件高度 small | **24** | `controlHeightSM` |
| 控件高度 large | **40** | `controlHeightLG` |
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

下列为 **产品关键配置**（完整以 §3 / 官方 API 为准）。分类：**数据录入**。

| 配置 | 说明 | 类型（摘录） | 默认 |
| --- | --- | --- | --- |
| `allowClear` | 自定义清除按钮 | boolean \ | { clearIcon?: ReactNode } |
| `classNames` | 用于自定义 Select 组件内部各语义化结构的 class，支持对象或函数 | Record<[SemanticDOM](#semantic-dom), … | (info: { props }) => Record<[SemanticDOM](#semantic-dom), string> |
| `defaultActiveFirstOption` | 是否默认高亮第一个选项 | boolean | true |
| `defaultOpen` | 是否默认展开下拉菜单 | boolean | - |
| `defaultValue` | 指定默认选中的条目 | string \ | string\[] \ |
| `disabled` | 是否禁用 | boolean | false |
| `popupMatchSelectWidth` | 下拉菜单和选择器同宽。默认将设置 `min-width`，当值小于选择框宽度时会被忽略。false 时会关闭虚拟滚动 | boolean \ | number |
| `popupRender` | 自定义下拉框内容 | (originNode: ReactNode) => ReactNode | - |
| `fieldNames` | 自定义节点 label、value、options、groupLabel 的字段 | object | { label: `label`, value: `value`, options: `options`, groupLabel: `label` } |
| `getPopupContainer` | 菜单渲染父节点。默认渲染到 body 上，如果你遇到菜单滚动定位问题，试试修改为滚动的区域，并相对其定位。[示例]… | function(triggerNode) | () => document.body |
| `labelInValue` | 是否把每个选项的 label 包装到 value 中，会把 Select 的 value 类型从 `string`… | boolean | false |
| `listHeight` | 设置弹窗滚动高度 | number | 256 |
| `loading` | 加载中状态 | boolean | false |
| `loadingIcon` | 自定义的加载图标 | ReactNode | `<LoadingOutlined spin />` |
| `maxCount` | 指定可选中的最多 items 数量，仅在 `mode` 为 `multiple` 或 `tags` 时生效 | number | - |
| `maxTagCount` | 最多显示多少个 tag，响应式模式会对性能产生损耗 | number \ | `responsive` |

**配置优先级（通用）：** 受控 props（`value`/`open`/`checked`）> 显式非受控 `default*` > 组件默认 > ConfigProvider 全局默认。

### 6.4 交互状态机（L1）

```text
closed ── click/聚焦+开 ──► open（下拉 Portal）
  │                            ├── 键盘 ↑↓ 移动 active option
  │                            ├── Enter ──► 选中
  │                            │     ├── mode=单选 ──► onChange + 关闭
  │                            │     └── multiple/tags ──► 累加 tag，可保持 open
  │                            ├── 再点已选项（multiple）──► 取消选中（可配）
  │                            ├── 输入搜索（showSearch）──► 过滤 options
  │                            ├── tags 模式输入+Enter ──► 创建选项
  │                            ├── Esc / 外点 ──► 关闭
  │                            └── allowClear ──► value 空
  └── disabled ──► 不打开
```

\*listHeight 默认常见 256；虚拟滚动大数据为 P1 时须在 Notes 标明。

| 规则 ID | 规则 | 期望 |
| --- | --- | --- |
| SEL-S1 | 单选选中一项 | `onChange` 一次且弹层关闭 |
| SEL-S2 | `mode=multiple` 选两项 | value 为数组长度 2；显示两个 tag |
| SEL-S3 | `showSearch` 输入过滤 | 仅匹配项可见 |
| SEL-S4 | `allowClear` 清除 | value 空；`onChange` |
| SEL-S5 | `disabled` | 点击不打开 |
| SEL-S6 | 受控 `open=false` | 无法保持展开 |
| SEL-S7 | Esc 打开时 | 关闭弹层 |
| SEL-S8 | 空 options | 显示 `notFoundContent` |
| SEL-S9 | `mode=tags` 输入新项 Enter | 创建并选中 |
| SEL-S10 | `size` 三档 | 高度 24/32/40 |
| SEL-S11 | 键盘 Enter 选中高亮项 | 同点击选中 |
| SEL-S12 | maxTagCount（适用者） | 多余折叠为 +N |
### 6.5 视觉 chrome 规则（L2 摘要）

| 态 / 变体 | 规则 |
| --- | --- |
| default | 容器底 + 边框（outlined）或族默认皮；Token 色 |
| hover | 边框/底强调 |
| focus | **可见** focus ring；主色边 |
| disabled | 降对比；不可编辑 |
| status=error/warning | 语义色边框/反馈 |
| 弹层 open | elevation 阴影；与触发器对齐 placement |


**动效：** 展开/入场须可关或尊重 reduced-motion；P0 可用瞬时切换。

### 6.6 无障碍（a11y）最低要求

| 项 | 要求 |
| --- | --- |
| 角色 | textbox / combobox / spinbutton / listbox 等 |
| 标签 | 与 Form.Item label 或 aria-labelledby 关联 |
| 清除/下拉 | 控件有可访问名称 |
| 错误 | status=error 时暴露 invalid |
| 键盘 | 主路径可选/提交/关闭 |

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
| `value` | 必须 |
| `defaultValue` | 必须 |
| `onChange` | 必须 |
| `disabled` | 必须 |
| `loading` | 必须 |
| `size` | 必须 |
| `variant` | 必须 |
| `status` | 必须 |
| `open` | 必须 |
| `onOpenChange` | 必须 |
| `options` | 必须 |
| `title` | 必须 |
| `placement` | 必须 |
| `allowClear` | 必须 |
| `showSearch` | 必须 |
| `mode` | 必须 |
| 官方主路径示例 | 基本使用、带搜索框、自定义搜索、多字段搜索、多选、三种大小、自定义下拉选项、带排序的搜索 |
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
| 其余示例 | 标签, 分组, 联动, 获得选项的文本 |

### 6.9 验收用例表（可测）

> 测试名建议：`TestSelect_PRD_<ID>` 或 gallery 场景 ID。  
> **P0 相关用例（无 P1 标记）全部通过** 才可宣称 Select 完成 1:1 主路径。

| ID | 级别 | 步骤 | 期望 |
| --- | --- | --- | --- |
| SEL-01 | L1 | NewSelect 默认创建 | 不崩溃；默认值符合 §6.10 / antd |
| SEL-02 | L1 | 单选选中一项 | `onChange` 一次且弹层关闭 |
| SEL-03 | L1 | `mode=multiple` 选两项 | value 为数组长度 2；显示两个 tag |
| SEL-04 | L1 | `showSearch` 输入过滤 | 仅匹配项可见 |
| SEL-05 | L1 | `allowClear` 清除 | value 空；`onChange` |
| SEL-06 | L1 | `disabled` | 点击不打开 |
| SEL-07 | L1 | 受控 `open=false` | 无法保持展开 |
| SEL-08 | L1 | Esc 打开时 | 关闭弹层 |
| SEL-09 | L1 | 空 options | 显示 `notFoundContent` |
| SEL-10 | L1 | `mode=tags` 输入新项 Enter | 创建并选中 |
| SEL-11 | L1 | `size` 三档 | 高度 24/32/40 |
| SEL-12 | L1 | 键盘 Enter 选中高亮项 | 同点击选中 |
| SEL-13 | L1 | maxTagCount（适用者） | 多余折叠为 +N |
| SEL-14 | L1 | 复现官方示例「基本使用」（`basic.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SEL-15 | L1 | 复现官方示例「带搜索框」（`search.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SEL-16 | L1 | 复现官方示例「自定义搜索」（`search-filter-option.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SEL-17 | L1 | 复现官方示例「多字段搜索」（`search-multi-field.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SEL-18 | L1 | 复现官方示例「多选」（`multiple.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SEL-19 | L1 | 复现官方示例「三种大小」（`size.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SEL-20 | L1 | 复现官方示例「自定义下拉选项」（`option-render.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SEL-21 | L1 | 复现官方示例「带排序的搜索」（`search-sort.tsx`） | 交互与主视觉符合文档；无控制台级错误 |
| SEL-22 | L2 | 读取 §6.2 关键尺寸/间距 | 与表内数字一致（±0.5px，或文档写明容差） |
| SEL-23 | L2 | 默认皮颜色 | 无硬编码品牌色；走 Theme Token |
| SEL-24 | L2 | disabled 外观（适用者） | 禁用色；无 hover 高亮 |
| SEL-25 | L1 | 键盘/焦点主路径（适用者） | 可聚焦者 Focus ring 可见；激活键有效 |
| SEL-26 | L3 | 关键态 golden 截图 | 与仓库基线一致（AA 容差） |
| SEL-27 | L4 | 与 ant.design 并排 | 人眼签字记录 |
| SEL-28 | P1 | §6.8 P1 任一能力（若做） | 单独用例；Notes 标明 |
### 6.10 产品 API 契约（Go kit 侧）

> 允许 breaking 旧 API；以下为 **产品需求层** 建议契约，实现可微调命名但语义不可丢。

```text
NewSelect(...) *Select

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
Field / Selector
  ├─ prefix?
  ├─ editable / display value
  ├─ clear? / suffix?
  └─ Portal popup? (list/panel)
```

- 组合 `ui/primitive` + `ui/core`，禁止第二套事件/帧循环。  
- 浮层统一 Portal / z-index；`rebuild()` 只读 Default/字段/Token。  
- 命中区域与布局盒一致（`hit == layout == paint`）。  
- 动画跟随 Host Tick；尊重 reduced-motion。  

### 6.12 完成定义（DoD）

同时满足即可宣布 **Select 主路径 1:1 完成**：

1. §6.8 **P0** 全部实现。  
2. §6.9 中 **P0 / L1 / L2** 用例测试通过。  
3. L2 度量与 Token 断言通过（§6.2 关键数字）。  
4. L3 golden 至少覆盖 1 个关键可见态（若控件可见）。  
5. gallery 展示主路径（对照官方非 debug 示例与 P0）。  
6. `coverage.go` Notes：P0 已对齐 `docs/antd/select.md` §6；P1 显式列出。  

---

**本章用法**：实现 `ui/kit` Select 时以 **§6 为需求与验收**；§1–§3 为 antd 能力全集；§6.8 为范围裁剪。细度样板见 [Button §6](./button.md#6-11-产品需求增量gpui-验收规格)。
