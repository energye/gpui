# GPUI 代码审查 — 剩余问题清单

> 来源：2026-07-25 审查总结（分支 `v1/13-控件-kit`）  
> 更新日期：2026-07-26  
> 说明：本文只跟踪**尚未关闭**的项；已修项见文末「已关闭」。

---

## 建议推进顺序

```text
1. #12 PaintContext 分配 — 有 profile 再做（P2）
2. #13 多窗口 — 独立里程碑（P1 功能）
3. #4  巨型文件拆分 — 穿插重构（P3）
4. #3  三套 GPU 管线收敛 — 长期（P3）
5. #6  Skin 推广到其余 kit 控件（Button 试点已完成）
6. #9  其它 kit 控件对齐 Button 生命周期样板
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

## 后续推广（#6 / #9 延续）

- **Skin**：Button 已试点；其它控件逐步 `SkinType` + 默认 Painter  
- **生命周期**：Button 已 `ensureBuilt` / `structureChange` / `chromeChange`；推广到 Breadcrumb / Input…

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
| 10 | TightWidthHeight | TightHeight；旧名删除 | 2026-07-26 |
| 11 | OverlayHost Entries | 写时有序；Entries 只 copy | 2026-07-26 |

### #9 / #6 验证入口

- 单测：`ui/kit/button_lifecycle_skin_test.go`  
- 示例：`examples/ui_polish_gallery` → Button 页 → **Lifecycle (#9)** / **Skin painter (#6)**  
