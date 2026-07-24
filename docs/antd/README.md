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

- **antd 源码（权威）**：`/home/yanghy/app/projects/ant-design/components/*`
- §1–§5 历史脚本：[`_generate_deep_docs.py`](./_generate_deep_docs.py)（可作骨架，需校对）
- **§6**：**逐个手写**（Button 模板）；`_deepen_sec6.py` 仅历史批量草稿，**不得**再覆盖已手写章节

手写下一控件流程：读 `index.zh-CN.md` + `style/*.ts` + 组件 TS → 按 button.md §6 结构写满 6.1–6.12 → 只替换该文件 `## 6.` 起至文末。

## 使用建议（开发 kit）

1. 实现前读对应 `docs/antd/<name>.md` 的 **§6**（DoD）；§1–§3 为 antd 能力全集参考。  
2. 以 **§6.8 P0** 定实现范围；**§6.10** 定 Go API；**§6.4 / §6.9** 写自动测试。  
3. 以 **§6.12** 做完成勾选（含 gallery，见下）。  
4. `coverage.go` Notes：P0 已对齐该 md §6；P1 显式列出。

### 完成定义（全库统一）

控件宣布 **1:1 主路径完成** 须同时满足：

| # | 要求 | 文档锚点 |
| --- | --- | --- |
| 1 | §6.8 **P0** 已实现 | 各控件 §6.8 |
| 2 | §6.9 中 **P0 / L1 / L2** 自动测试通过 | 各控件 §6.9 |
| 3 | L2 Token/度量断言（适用者） | §6.2 |
| 4 | L3 golden 至少一个关键态（控件可见时） | §6.9 L3 |
| 5 | **示例程序** 已增加/更新 | 见下节 **ui_polish_gallery** |
| 6 | `ui/kit/coverage.go` Notes 已更新 | P0 对齐路径 + P1 列表 |

### 示例程序：`examples/ui_polish_gallery`（强制）

**权威约定（全局只此一处详述）：**

- **路径：** 仓库根目录 [`examples/ui_polish_gallery`](../../examples/ui_polish_gallery)  
- **何时：** 实现某控件 **§6.8 P0** 后、勾选 §6.12 完成前，必须在 gallery **增加或更新**该控件页示例。  
- **覆盖范围：** 对齐该控件 **§6.8 P0** 与 §6.12 中列出的 gallery 场景；官方 **非 debug** 主路径 demo 优先。  
- **不要**强求覆盖 §1–§3 全部官方 demo（避免范围爆炸）。  
- **P1：** 可不进 gallery，或单独标明 Later；须在 `coverage.go` Notes 写出。  
- **组织：** 一控件一页/tab；能力用 section 分块（参考 Button 页：Type / Size / Icon / …）。  
- **豁免：** 无运行时 UI 的条目（如 **Util** 类型工具）可跳过 gallery，在 Notes 写「无 UI」。  
- **细节实现：** 见 `examples/ui_polish_gallery/catalog.go` 与 [`ui/kit/doc.go`](../../ui/kit/doc.go)（Demos 规则）。

各控件文档 **§6.12** 仅保留短锚点，指向本节，避免 72 份重复长文。

## 1:1 产品规格（§6）

**写法约定：** 全部控件 §6 对齐 [Button §6](./button.md) 模板细度（6.1–6.12：度量 Token、状态机规则 ID、chrome、a11y、平台边界、P0/P1、可测用例、Go API、结构、DoD）。  
**依据源码：** `/home/yanghy/app/projects/ant-design/components/<name>/`。  
**再生（跳过 button 样板）：**

```bash
python3 docs/antd/_write_button_depth_sec6.py --force
python3 docs/antd/_write_button_depth_sec6.py input modal  # 子集
```

| 状态 | 说明 |
| --- | --- |
| **样板** | [button.md §6](./button.md)（手写最细，不自动覆盖） |
| **全库** | **71 控件** §6 已按 Button 结构从本地 antd 源码重写（约 200–260 行/控件；无「见上文」filler） |
| **实现时** | 以该控件 §6 为 DoD；复杂控件可再对照 style/*.ts 补业务专用数字 |

## 说明

- **List** 官方倾向废弃，仍保留完整规格便于兼容。  
- **Icon** 依赖 `@ant-design/icons@6.x`。  
- Table / Form / DatePicker 等 API 极长，文档含官方全文表 + 解析速查。  
- §1–§3 = antd 能力全集；**§6 = gpui 可交付的 1:1 产品需求**（含裁剪与平台边界）。  
