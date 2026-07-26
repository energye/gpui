# GPUI 代码审查 — 剩余问题清单

> 来源：2026-07-25 审查总结（分支 `v1/13-控件-kit`）  
> 更新日期：2026-07-26  
> 说明：本文只跟踪**尚未关闭**的项；已修项见文末「已关闭」。

---

## 建议推进顺序

```text
0. #9+#6 按下方「控件推广顺序」手开（当前：Breadcrumb）
1. #12 PaintContext 分配 — 有 profile 再做（P2）
2. #13 多窗口 — 独立里程碑（P1 功能）
3. #4  巨型文件拆分 — 穿插重构（P3）
4. #3  三套 GPU 管线收敛 — 长期（P3）
```

---

## #12 PaintContext 值拷贝 → GC 压力（P2）

### 现象

`WithOrigin` / `WithClip` / `WithForceFullPaint` 值拷贝并逃逸到堆；`DefaultPaintChildren` 对子节点频繁分配。

### 后果

深树 × 高帧率时分配多（审查 5000×60 为上限估算）。模式真实；量级需 **profile** 后再投入。

### 建议解法

1. pprof / alloc 基准确认热点  
2. `sync.Pool` + Release（注意 clip 栈隔离，#7 已 clone）  

### 关键文件

- `ui/core/paint.go`、`ui/core/node.go`

### 工作量

约 1 天；无 profile 不建议抢先。

---

## #13 Application 仅单窗口（P1 功能）

### 现象

`ui/app/app.go`：`session *Session` 仅一个（Phase 1）。

### 建议解法

`sessions map` + 遍历 Pulse；每窗口独立 surface。Linux 真测优先。独立里程碑约 2 周+。

---

## #4 巨型文件（P3）

`render_session.go` ~5k、`gpu_render_context.go` ~3k、`context.go` ~2.6k 等。按关注点拆分，纯搬家 + 测绿。

---

## #3 三套 GPU 管线（P3 长期）

传统 RenderPass / Vello tilecompute（stroke 未齐）/ scene graph。先文档路由表，再收敛。

---

## 后续推广（#6 / #9 模式 → 全 kit）

以 **Button** 为样板（已完成）：

| 模式 | 约定 |
|------|------|
| #9 生命周期 | `ensureBuilt` / `structureChange` / `chromeChange`；setter 分类；禁止散落 `Root==nil` 空跑 |
| #6 Skin | `Decorated.SkinType = kit.TypeXxx`（或等价）；`skin/default` 注册默认 Painter；可用 `core.Override` 验证 |

### 推广原则

1. **先样板再批量**：每波 1～3 个，PR 可测、gallery 可看  
2. **先高频/简单，后复合**：chrome 为主的先做；Table/Form/Tree 靠后  
3. **同波 #9+#6 一起收**：生命周期定型后再打 `SkinType`，避免边 rebuild 边抽皮  
4. **gallery**：该控件页增加 Lifecycle / Skin 小节小节（对齐 Button）  
5. **清单打勾**：完成一项在本表状态列改 `done` 并注明日期  

### 控件推广顺序（推荐）

> 状态：`done` / `next` / `todo`  
> Ant 名与 `ui/kit/coverage.go` 对齐。

#### Wave 0 — 样板（已完成）

| # | 控件 | kit | #9 | #6 | 状态 | 备注 |
|---|------|-----|----|----|------|------|
| 0 | Button | `button.go` | ✓ | ✓ | **done** 2026-07-26 | 参考实现 + gallery |

#### Wave 1 — next：导航 + 基础录入（当前手开）

高频、结构清晰，和近期 kit 提交同向。

| # | 控件 | kit | 状态 | 备注 |
|---|------|-----|------|------|
| 1 | **Breadcrumb** | `breadcrumb.go` | **done** 2026-07-26 | ensureBuilt/structureChange/chromeChange + SkinType + gallery |
| 2 | **Input** | `input.go` | **done** 2026-07-26 | ensureBuilt/structureChange/chromeChange + SkinType + gallery |
| 3 | **Switch** | `switch.go` | **next** | 结构简单，适合巩固样板 |
| 4 | Checkbox | `checkbox.go` | todo | 与 Radio 同型 |
| 5 | Radio | `radio.go` | todo | 含 Radio.Button |
| 6 | Tag | `tag.go` | todo | 偏展示，Skin 收益直观 |

#### Wave 2 — 展示 / 轻反馈

| # | 控件 | kit | 状态 | 备注 |
|---|------|-----|------|------|
| 7 | FloatButton | `float_button.go` | todo | 贴近 Button |
| 8 | Badge | `badge.go` | todo | 已有 countSet 等 flag 经验 |
| 9 | Alert | `alert.go` | todo | |
| 10 | Progress | `progress.go` | todo | |
| 11 | Spin | `spin.go` | todo | Ticker |
| 12 | Skeleton | `skeleton.go` | todo | Ticker；rebuild 重 |
| 13 | Empty | `empty.go` | todo | |
| 14 | Result | `result.go` | todo | |
| 15 | Statistic | `statistic.go` | todo | |
| 16 | Avatar | `avatar.go` | todo | |
| 17 | Divider | `divider.go` | todo | 很轻 |
| 18 | Icon | `icon.go` | todo | |
| 19 | Typography / Text | `typography.go` | todo | |

#### Wave 3 — 导航 / 布局壳

| # | 控件 | kit | 状态 | 备注 |
|---|------|-----|------|------|
| 20 | Tabs | `tabs.go` | todo | |
| 21 | Menu | `menu.go` | todo | |
| 22 | Dropdown | `dropdown.go` | todo | Overlay |
| 23 | Pagination | `pagination.go` | todo | |
| 24 | Steps | `steps.go` | todo | rebuild 多 |
| 25 | Anchor | `anchor.go` | todo | |
| 26 | Space | `space.go` | todo | |
| 27 | Flex | `flex.go` | todo | 布局原语向 |
| 28 | Grid | `grid.go` | todo | |
| 29 | Layout | `layout.go` | todo | |
| 30 | Splitter | `splitter.go` | todo | |
| 31 | Scroll | `scroll.go` | todo | 原语向 |

#### Wave 4 — 反馈浮层

| # | 控件 | kit | 状态 | 备注 |
|---|------|-----|------|------|
| 32 | Modal | `modal.go` | todo | 已有 widthSet 等；补生命周期+Skin |
| 33 | Drawer | `drawer.go` | todo | |
| 34 | Message | `message.go` | todo | Host 型 |
| 35 | Notification | `notification.go` | todo | Host 型 |
| 36 | Popover | `popover.go` | todo | |
| 37 | Tooltip | `tooltip.go` | todo | |
| 38 | Popconfirm | `popconfirm.go` | todo | |
| 39 | Tour | `tour.go` | todo | |

#### Wave 5 — 录入复合

| # | 控件 | kit | 状态 | 备注 |
|---|------|-----|------|------|
| 40 | Select | `select.go` | todo | Overlay + 列表 |
| 41 | InputNumber | `input_number.go` | todo | |
| 42 | Mentions | `mentions.go` | todo | |
| 43 | AutoComplete | `auto_complete.go` | todo | |
| 44 | Cascader | `cascader.go` | todo | |
| 45 | TreeSelect | `tree_select.go` | todo | |
| 46 | DatePicker | `date_picker.go` | todo | |
| 47 | TimePicker | `time_picker.go` | todo | |
| 48 | Upload | `upload.go` | todo | |
| 49 | Form | `form.go` | todo | 依赖 Item/录入稳定 |
| 50 | Rate | `rate.go` | todo | |
| 51 | Slider | `slider.go` | todo | |
| 52 | ColorPicker | `color_picker.go` | todo | |
| 53 | Segmented | `segmented.go` | todo | |
| 54 | Transfer | `transfer.go` | todo | |

#### Wave 6 — 数据展示重型

| # | 控件 | kit | 状态 | 备注 |
|---|------|-----|------|------|
| 55 | Table | `table.go` | todo | 最重；靠后 |
| 56 | Tree | `tree.go` | todo | 近期完善中；生命周期后置以免搅局 |
| 57 | List | `list.go` | todo | |
| 58 | Descriptions | `descriptions.go` | todo | |
| 59 | Card | `card.go` | todo | |
| 60 | Calendar | `calendar.go` | todo | |
| 61 | Carousel | `carousel.go` | todo | |
| 62 | Collapse | `collapse.go` | todo | |
| 63 | Image | `image.go` | todo | Preview overlay |
| 64 | Timeline | `timeline.go` | todo | |
| 65 | QRCode | `qrcode.go` | todo | |

#### Wave 7 — 其它 / 特效 / 壳

| # | 控件 | kit | 状态 | 备注 |
|---|------|-----|------|------|
| 66 | Affix | `affix.go` | todo | |
| 67 | Watermark | `watermark.go` | todo | |
| 68 | BorderBeam | `border_beam.go` | todo | |
| 69 | ConfigProvider | `config_provider.go` | todo | 主题入口；Skin 挂载点 |
| 70 | App | （theme density） | todo | 偏壳 |

### 每手开一个控件的检查单

```text
[ ] #9 ensureBuilt / structureChange / chromeChange（或等价命名）
[ ] setter 分类：结构 vs chrome；去掉无意义 Root==nil 空跑
[ ] 单测：setter 顺序不丢样式；Node() 可 ensureBuilt
[ ] #6 SkinType（或 TypeID）+ skin/default 注册
[ ] 单测：Override painter 可命中
[ ] gallery：Lifecycle + Skin 小节小节（若该控件有页）
[ ] 本表状态 → done + 日期
```

### 当前手开指针

```text
→ Wave 1 #3 Switch
```

---

## 已关闭（备查）

| # | 问题 | 处理摘要 | 日期 |
|---|------|----------|------|
| 1 | 包名 gg vs render | 文档/注释统一 render. | 2026-07-26 |
| 2 | BlendMode 碎片 | ToPaintBlendMode；修裸 cast | 2026-07-26 |
| 5 | stroke bootstrap | takeBrushBootstrapIfAny | 2026-07-26 |
| 7 | Clip 深度 6 | 可增长栈 + clone | 2026-07-26 |
| 8 | Modal Width flag | widthSet 等 | 2026-07-26 |
| 9 | Button lifecycle | ensureBuilt/structureChange/chromeChange；测试+gallery | 2026-07-26 |
| 6 | Skin 空壳（Button 试点） | SkinType；default 注册 kit.Button；Override+gallery | 2026-07-26 |
| 9+6 | Breadcrumb 推广 | 同 Button 模式；Flex.SkinType；测试+gallery | 2026-07-26 |
| 9+6 | Input 推广 | Decorated.SkinType；structure/chrome 分类；测试+gallery | 2026-07-26 |
| 10 | TightWidthHeight | TightHeight；旧名删除 | 2026-07-26 |
| 11 | OverlayHost Entries | 写时有序；Entries 只 copy | 2026-07-26 |

### #9 / #6 验证入口

- 单测：`ui/kit/button_lifecycle_skin_test.go`  
- 示例：`examples/ui_polish_gallery` → Button 页 → **Lifecycle (#9)** / **Skin painter (#6)**  
