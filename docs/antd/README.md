# Ant Design 6.5.x → gpui kit 控件规格文档

> 依据 [Ant Design 6.5.x](https://ant.design/components/overview) 官方文档整理。  
> **每个控件一份 Markdown**，细度对齐 `button.md`（外观 / 功能 / 完整 API / kit 实现要点）。  
> 用于开发 **gpui kit** 控件时的功能规格与验收清单。

## 文档结构（与 Button 同级）

每个控件文档统一包含：

1. **控件外观**
   - 基础形态
   - 官方示例对应的外观形态表
   - 外观相关配置逐项说明（含枚举值与视觉语义）
   - 交互视觉状态检查表（default/hover/focus/disabled/loading…）
   - 语义化 DOM 与 Design Token
2. **功能**
   - 使用场景（When To Use）
   - 按官方示例拆解的核心功能清单
   - 行为 API 能力表
   - 示例全表（含 debug 标记）
   - 实例方法 / Ref、FAQ（若有）
   - 与其它控件组合关系
3. **配置（API）**
   - 官方 API 全文表格
   - 导入方式
   - 配置项速查表（从 API 解析）
4. **gpui kit 实现要点**（验收清单）
5. **参考链接**

## 版本

- 目标版本：**antd v6.5.1**
- 源文档：`components/*/index.zh-CN.md`（官方仓库）

## 控件索引

### 通用 General

| 控件 | 文档 |
| --- | --- |
| Button | [button.md](./button.md) |
| FloatButton | [float-button.md](./float-button.md) |
| Icon | [icon.md](./icon.md) |
| Typography | [typography.md](./typography.md) |

### 布局 Layout

| 控件 | 文档 |
| --- | --- |
| Divider | [divider.md](./divider.md) |
| Flex | [flex.md](./flex.md) |
| Grid | [grid.md](./grid.md) |
| Layout | [layout.md](./layout.md) |
| Masonry | [masonry.md](./masonry.md) |
| Space | [space.md](./space.md) |
| Splitter | [splitter.md](./splitter.md) |

### 导航 Navigation

| 控件 | 文档 |
| --- | --- |
| Anchor | [anchor.md](./anchor.md) |
| Breadcrumb | [breadcrumb.md](./breadcrumb.md) |
| Dropdown | [dropdown.md](./dropdown.md) |
| Menu | [menu.md](./menu.md) |
| Pagination | [pagination.md](./pagination.md) |
| Steps | [steps.md](./steps.md) |
| Tabs | [tabs.md](./tabs.md) |

### 数据录入 Data Entry

| 控件 | 文档 |
| --- | --- |
| AutoComplete | [auto-complete.md](./auto-complete.md) |
| Cascader | [cascader.md](./cascader.md) |
| Checkbox | [checkbox.md](./checkbox.md) |
| ColorPicker | [color-picker.md](./color-picker.md) |
| DatePicker | [date-picker.md](./date-picker.md) |
| Form | [form.md](./form.md) |
| Input | [input.md](./input.md) |
| InputNumber | [input-number.md](./input-number.md) |
| Mentions | [mentions.md](./mentions.md) |
| Radio | [radio.md](./radio.md) |
| Rate | [rate.md](./rate.md) |
| Select | [select.md](./select.md) |
| Slider | [slider.md](./slider.md) |
| Switch | [switch.md](./switch.md) |
| TimePicker | [time-picker.md](./time-picker.md) |
| Transfer | [transfer.md](./transfer.md) |
| TreeSelect | [tree-select.md](./tree-select.md) |
| Upload | [upload.md](./upload.md) |

### 数据展示 Data Display

| 控件 | 文档 |
| --- | --- |
| Avatar | [avatar.md](./avatar.md) |
| Badge | [badge.md](./badge.md) |
| Calendar | [calendar.md](./calendar.md) |
| Card | [card.md](./card.md) |
| Carousel | [carousel.md](./carousel.md) |
| Collapse | [collapse.md](./collapse.md) |
| Descriptions | [descriptions.md](./descriptions.md) |
| Empty | [empty.md](./empty.md) |
| Image | [image.md](./image.md) |
| List | [list.md](./list.md) |
| Popover | [popover.md](./popover.md) |
| QRCode | [qr-code.md](./qr-code.md) |
| Segmented | [segmented.md](./segmented.md) |
| Statistic | [statistic.md](./statistic.md) |
| Table | [table.md](./table.md) |
| Tag | [tag.md](./tag.md) |
| Timeline | [timeline.md](./timeline.md) |
| Tooltip | [tooltip.md](./tooltip.md) |
| Tour | [tour.md](./tour.md) |
| Tree | [tree.md](./tree.md) |

### 反馈 Feedback

| 控件 | 文档 |
| --- | --- |
| Alert | [alert.md](./alert.md) |
| Drawer | [drawer.md](./drawer.md) |
| Message | [message.md](./message.md) |
| Modal | [modal.md](./modal.md) |
| Notification | [notification.md](./notification.md) |
| Popconfirm | [popconfirm.md](./popconfirm.md) |
| Progress | [progress.md](./progress.md) |
| Result | [result.md](./result.md) |
| Skeleton | [skeleton.md](./skeleton.md) |
| Spin | [spin.md](./spin.md) |
| Watermark | [watermark.md](./watermark.md) |

### 其他 Other

| 控件 | 文档 |
| --- | --- |
| Affix | [affix.md](./affix.md) |
| App | [app.md](./app.md) |
| BorderBeam | [border-beam.md](./border-beam.md) |
| ConfigProvider | [config-provider.md](./config-provider.md) |
| Util | [util.md](./util.md) |

## 再生生成

源文档缓存与生成脚本：

- 源：`/tmp/antd-docs-src/*.md`（官方 zh-CN）
- 脚本：[`_generate_deep_docs.py`](./_generate_deep_docs.py)（历史脚本；当前以会话内增强生成器为准）

更新 antd 版本后，可重新拉取官方 `index.zh-CN.md` 再生成。

## 使用建议（开发 kit）

1. 实现某控件前先读对应 `docs/antd/<name>.md` 第 1–4 节。  
2. 以 **§1.2 示例表 + §2.2 功能清单** 建可视化/交互用例。  
3. 以 **§3 API + 配置项速查** 定公共 props / 枚举。  
4. 以 **§4 kit 实现要点** 做 PR 验收勾选。  

## 说明

- **List** 官方倾向废弃，仍保留完整规格便于兼容。  
- **Icon** 依赖 `@ant-design/icons@6.x`。  
- Table / Form / DatePicker 等 API 极长，文档含官方全文表 + 解析速查。  
