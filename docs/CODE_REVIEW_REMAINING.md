# GPUI 代码审查 — 剩余问题清单

> 来源：2026-07-25 审查总结（分支 `v1/13-控件-kit`）  
> 更新日期：2026-07-26  
> 说明：本文只跟踪**尚未关闭**的项；已修项见文末「已关闭」。

---

## 建议推进顺序

```text
0. #9+#6 控件 Skin 推广：已全部完成 ✅
1. #12 PaintContext 分配 — 有 profile 再做（P2）
2. #13 多窗口 — 独立里程碑（P1 功能）
3. #4  巨型文件拆分 — 穿插重构（P3）
4. #3  三套 GPU 管线收敛 — 长期（P3）
```

---

## #6 Skin 全量完成（2026-07-26）

清单内全部 kit 控件已接入：

1. `ui/kit/doc.go` — 统一 `TypeXxx` 常量  
2. `ui/skin/default/skin.go` — 默认 Painter 注册（Decorated / Flex / host 遍历）  
3. 各控件 `ensureBuilt` / `structureChange` / `chromeChange`（或等价）  
4. 主 chrome `SkinType` 或 host `TypeID` + `Paint` 查 Skin  
5. 回归：`TestAllKit_SkinPaintersRegistered` + `TestWave*` / 抽样 `Node()`  

**说明：** 早期 Wave 0–1 含完整 lifecycle 单测 + gallery 小节；Wave 2 后半至 Wave 7 以批量 Skin 管道 + ensureBuilt + 注册表测试为主。  
**Gallery（2026-07-26）：** `examples/ui_polish_gallery` 全部组件页（70 页，不含 main/helpers/catalog/app 壳）均已增加 **Lifecycle (#9)** 与 **Skin painter (#6)** 小节。

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
| 3 | **Switch** | `switch.go` | **done** 2026-07-26 | ensureBuilt/structureChange/chromeChange + SkinType + gallery |
| 4 | **Checkbox** | `checkbox.go` | **done** 2026-07-26 | ensureBuilt/chromeChange + indicator SkinType + gallery |
| 5 | **Radio** | `radio.go` | **done** 2026-07-26 | ensureBuilt/chromeChange + SkinType (dot/btn) + gallery |
| 6 | **Tag** | `tag.go` | **done** 2026-07-26 | ensureBuilt/structureChange + Root.SkinType + gallery |

#### Wave 2 — 展示 / 轻反馈

| # | 控件 | kit | 状态 | 备注 |
|---|------|-----|------|------|
| 7 | **FloatButton** | `float_button.go` | **done** 2026-07-26 | ensureBuilt/chromeChange + SkinType kit.FloatButton + gallery |
| 8 | **Badge** | `badge.go` | **done** 2026-07-26 | ensureBuilt + host TypeID + SkinType + gallery |
| 9 | **Alert** | `alert.go` | **done** 2026-07-26 | ensureBuilt/structureChange + Root.SkinType + gallery |
| 10 | **Progress** | `progress.go` | **done** 2026-07-26 | ensureBuilt/structureChange/chromeChange + host TypeID + gallery |
| 11 | **Spin** | `spin.go` | **done** 2026-07-26 | ensureBuilt + host TypeID/Paint Skin + batch test |
| 12 | **Skeleton** | `skeleton.go` | **done** 2026-07-26 | ensureBuilt + host TypeID/Paint Skin + batch test |
| 13 | **Empty** | `empty.go` | **done** 2026-07-26 | ensureBuilt + Root.SkinType + batch test |
| 14 | **Result** | `result.go` | **done** 2026-07-26 | ensureBuilt + Flex.SkinType + batch test |
| 15 | **Statistic** | `statistic.go` | **done** 2026-07-26 | ensureBuilt + host TypeID/Paint Skin + batch test |
| 16 | **Avatar** | `avatar.go` | **done** 2026-07-26 | ensureBuilt + Root.SkinType + batch test |
| 17 | **Divider** | `divider.go` | **done** 2026-07-26 | ensureBuilt + Flex.SkinType + batch test |
| 18 | **Icon** | `icon.go` | **done** 2026-07-26 | ensureBuilt + host Paint Skin + batch test |
| 19 | **Typography / Text** | `typography.go` | **done** 2026-07-26 | ensureBuilt + host Paint Skin + batch test |

#### Wave 3 — 导航 / 布局壳

| # | 控件 | kit | 状态 | 备注 |
|---|------|-----|------|------|
| 20 | **Tabs** | `tabs.go` | **done** 2026-07-26 | ensureBuilt + Flex.SkinType + skin |
| 21 | **Menu** | `menu.go` | **done** 2026-07-26 | ensureBuilt + Root.SkinType + skin |
| 22 | **Dropdown** | `dropdown.go` | **done** 2026-07-26 | ensureBuilt + skin |
| 23 | **Pagination** | `pagination.go` | **done** 2026-07-26 | ensureBuilt + Flex.SkinType + skin |
| 24 | **Steps** | `steps.go` | **done** 2026-07-26 | ensureBuilt + Flex.SkinType + skin |
| 25 | **Anchor** | `anchor.go` | **done** 2026-07-26 | ensureBuilt + skin |
| 26 | **Space** | `space.go` | **done** 2026-07-26 | ensureBuilt + Flex.SkinType + skin |
| 27 | **Flex** | `flex.go` | **done** 2026-07-26 | ensureBuilt + TypeKitFlex + skin |
| 28 | Grid | `grid.go` | **done** 2026-07-26 | TypeID kit.Row/Col (layout primitive) |
| 29 | **Layout** | `layout.go` | **done** 2026-07-26 | ensureBuilt + TypeLayout + skin |
| 30 | **Splitter** | `splitter.go` | **done** 2026-07-26 | ensureBuilt + TypeSplitter + skin |
| 31 | **Scroll** | `scroll.go` | **done** 2026-07-26 | ensureBuilt + TypeScroll + skin |

#### Wave 4 — 反馈浮层

| # | 控件 | kit | 状态 | 备注 |
|---|------|-----|------|------|
| 32–39 | Modal/Drawer/Message/Notification/Popover/Tooltip/Popconfirm/Tour | 对应文件 | **done** 2026-07-26 | ensureBuilt + panel SkinType + skin 注册 |

#### Wave 5 — 录入复合

| # | 控件 | kit | 状态 | 备注 |
|---|------|-----|------|------|
| 40–54 | Select…Transfer（全表） | 对应文件 | **done** 2026-07-26 | ensureBuilt + Skin 注册 + 抽样 Node 测试 |

#### Wave 6 — 数据展示重型

| # | 控件 | kit | 状态 | 备注 |
|---|------|-----|------|------|
| 55–65 | Table…QRCode（全表） | 对应文件 | **done** 2026-07-26 | ensureBuilt + Skin 注册 + 抽样 Node 测试 |

#### Wave 7 — 其它 / 特效 / 壳

| # | 控件 | kit | 状态 | 备注 |
|---|------|-----|------|------|
| 66–70 | Affix/Watermark/BorderBeam/ConfigProvider/App | 对应文件 | **done** 2026-07-26 | ensureBuilt + Skin 注册（App 为 theme 壳） |


### 每手开一个控件的检查单

```text
[ ] #9 ensureBuilt / structureChange / chromeChange（或等价命名）
[ ] setter 分类：结构 vs chrome；去掉无意义 Root==nil 空跑
[ ] 单测：setter 顺序不丢样式；Node() 可 ensureBuilt
[ ] #6 SkinType（或 TypeID）+ skin/default 注册
[ ] 单测：Override painter 可命中
[x] gallery：Lifecycle + Skin 小节小节（若该控件有页）— 全量 2026-07-26
[ ] 本表状态 → done + 日期
```

### 当前手开指针

```text
→ #9+#6 控件 Skin 推广：全部完成 ✅
→ gallery Lifecycle/Skin 小节：全部完成 ✅（70 组件页）
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
| 9+6 | Switch 推广 | track.SkinType；structure/chrome；测试+gallery | 2026-07-26 |
| 9+6 | Checkbox 推广 | indicator.SkinType；chrome 为主；测试+gallery | 2026-07-26 |
| 9+6 | Radio 推广 | default/button SkinType；structure/chrome；测试+gallery | 2026-07-26 |
| 10 | TightWidthHeight | TightHeight；旧名删除 | 2026-07-26 |
| 11 | OverlayHost Entries | 写时有序；Entries 只 copy | 2026-07-26 |

### #9 / #6 验证入口

- 单测：`ui/kit/*_lifecycle_skin_test.go`、`wave*_skin_lifecycle_test.go`、`TestAllKit_SkinPaintersRegistered`  
- 示例：`examples/ui_polish_gallery` → 各组件页底部 **Lifecycle (#9)** / **Skin painter (#6)**  
- 注意：Icon/Spin/Skeleton/Statistic 等内嵌 `RepaintBoundary` 的 host **不** 用 Skin 拦截 `Paint`（会断 glyph/layer）；gallery 仅展示 TypeID 注册。
