# L1 加深任务计划 — Phase 4（滚动 · 文本 · 图像 IO · 裁剪）

> **版本：1.1** | 日期：2026-07-27  
> **状态：已实现（主路径）· 可验收**  
> **前置：** P0–P3 已收口 — [`ENGINE_L1_CLOSEOUT.md`](./ENGINE_L1_CLOSEOUT.md)  
> **真源：** [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md) §4.5 / F05 / F12 / F16  
> **大纲父页：** [`ENGINE_PHASE_P4_P7_OUTLINE.md`](./ENGINE_PHASE_P4_P7_OUTLINE.md)  

---

## 0. 目标与非目标

### 目标

在 **不破坏 P0–P3 管线契约** 的前提下，让「真页面素材」仍满足 **帧成本 ∝ 脏区**：

| 能力 | 一句话 |
|------|--------|
| **滚动协议** | offset 在 Layer 上；滚动默认不整树 `MarkNeedsLayout` |
| **虚拟列表** | 1k 行只挂载可见（+缓存窗）；每帧 bind 有上界 |
| **拖拽滚动** | 最新点跟手；CompositeOnly / compositor dirty |
| **裁剪** | 视口 clip；离屏行不 paint |
| **文本（引擎级）** | 简单 DrawString / 多行基础；可选缓存键 |
| **图像 IO** | 解码在 **IO 线程**；UI 只收结果、MarkNeedsPaint |
| **saveLayer 纪律** | 预算接口，默认禁止大面积 |

### 非目标（本阶段不做）

| 不做 | 放到 |
|------|------|
| Ant List/Table/Image 产品 API | P7 |
| 完整 IME / 选区编辑器 | 后置 |
| GestureArena 完整竞技 | P5 |
| 嵌套滚动完整竞争（仅最小规则） | 完整 P5 |
| per-layer GPU RT 池打磨 | P6 |
| 改 `ui → render → gpu` 依赖方向 | 永不 |

### 全局约束（继承 P0–P3）

| ID | 约束 |
|----|------|
| G1 | `ui → render → gpu`，**ui 禁止 import gpu** |
| G3 | 逻辑 px、Y-down；物理 = 逻辑 × dpr |
| G4 | 管道 depth、背压、SubmitLatest 不堵 UI |
| G5 | 文本/图能力不够 → 改 **render**（或 gpu），不在 ui 复制 GPU |
| G7 | `go test ./ui/...`；动 render 时测 `./render` |

### 建议 baseline（开工前）

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_l1_spinner 2> /tmp/l1_spinner_baseline.err | tee /tmp/l1_spinner_baseline.json
```

P4 合入后同场景 JSON 不得显著回退（layout 空转、raster_layer 暴涨）。

---

## 1. 交付物总览

```text
ui/rendering/
  viewport.go          // RenderViewport：clip + scroll offset
  sliver_list.go       // 虚拟列表（itemExtent 固定行高 MVP）
  clip.go              // RenderClipRect（可选独立类型）
  text.go              // RenderParagraph / RenderText 最小
  image.go             // RenderImage：占位 + 异步结果
ui/painting/
  context.go           // + ClipRect / DrawText / DrawImage 逻辑坐标封装
ui/io/                 // 新建：解码 worker 池（F12）
  decode.go
  decode_test.go
ui/scene/
  layer.go / build.go  // OffsetLayer 承载 scroll；Clip 层
examples/
  ui_l1_scroll/        // S5/S6 真窗：1k 行虚拟列表 + 拖拽/滚轮
render/                // 仅当 DrawString/Image 门面不够时小改
```

---

# 分轨任务（建议顺序：A → B → C → D → E）

原则：**先滚动门禁（A/B），再素材（C/D），最后纪律与示例（E）。**  
每轨可单独 PR；A 必须先合。

---

## 轨 A — 滚动协议（F05 核心）

**目标：** `scrollOffset` 变化默认只 `MarkNeedsPaint` / compositor dirty，**不** `MarkNeedsLayout` 整链。

### A.1 `RenderViewport`

| 属性 | 说明 |
|------|------|
| `ViewportSize` | 逻辑可见区域（通常 = 自身 Size） |
| `ScrollOffset` | Point，Y-down 正值内容上移（或文档约定：+Y 向下滚） |
| `Child` | 单一内容子（或 content root） |
| `Clip` | paint 时 clip 到本地 bounds |

**约定（写进代码注释 + 单测）：**

```text
ScrollOffset.Y 增大 → 内容相对视口向上移动（常见列表「向下滚」）
HitTest：指针先变换到 content 空间再下传
SetScrollOffset → MarkNeedsPaint only（除非 offset 导致虚拟窗口 bind 变化，见轨 B）
```

### A.2 任务表

| # | 任务 | DoD |
|---|------|-----|
| A1 | 新增 `RenderViewport`：Layout 收紧为视口尺寸；Child 用松约束 layout | 单测 size |
| A2 | `SetScrollOffset` / `ScrollBy`：只 MarkNeedsPaint | 断言 `NeedsLayout()==false` |
| A3 | Paint：`PushClip` 本地 rect + 以 `-offset` 平移 content paint | 像素或 visit 测 |
| A4 | HitTest：`p' = p + ScrollOffset` 再测 child | 单测 |
| A5 | Layer 构建：Viewport → Clip + OffsetLayer（scroll） | BuildLayerTree 含 offset |
| A6 | 文档：坐标与 offset 符号写进 `viewport.go` 包注释 | — |

### A.3 单测门禁

| 测试名（建议） | 断言 |
|----------------|------|
| `TestViewport_ScrollDoesNotLayout` | 10 次 SetScrollOffset，LayoutCount 不增 |
| `TestViewport_HitTestRespectsOffset` | 滚后点击命中正确行/子 |
| `TestViewport_ClipSkipsOffscreenPaint` | CompositeOnly 下离屏 child 不 paint（与轨 B 联调） |

---

## 轨 B — 虚拟列表（S5）

**目标：** 固定行高 MVP；1k 行 **不** 同时挂 1k 个 RO；每帧 bind 数 ≤ 可见行 + cache。

### B.1 `RenderSliverList`（名称可改为 `VirtualList`）

| 字段 | 说明 |
|------|------|
| `ItemCount` | 逻辑总行数 |
| `ItemExtent` | 固定行高（逻辑 px）；MVP 只做固定高 |
| `Builder` | `func(index int) RenderObject` 惰性创建 |
| `CacheExtent` | 视口外额外缓存（逻辑 px 或行数） |
| 内部 | `firstIndex`/`lastIndex`、`children` 复用池 |

**算法（MVP）：**

```text
visibleStart = floor(scrollY / itemExtent)
visibleEnd   = ceil((scrollY + viewportH) / itemExtent)
bind [visibleStart - cache, visibleEnd + cache] ∩ [0, count)
卸载窗口外节点（detach + 可选回收）
仅当 bind 窗口变化时 MarkNeedsLayout（局部）
滚动未改变窗口时只 MarkNeedsPaint（viewport）
```

### B.2 任务表

| # | 任务 | DoD |
|---|------|-----|
| B1 | `VirtualList` 结构 + Builder + ItemExtent | 编译 |
| B2 | `ensureChildren(scroll, viewport)` bind/unbind | 单测窗口 |
| B3 | Layout：仅 layout 已 bind 子；总 content 高度 = count×extent | 单测 |
| B4 | 与 Viewport 组合：list 作 content 或 viewport 内嵌 list | 单测 |
| B5 | 统计：`BindCount` / `PaintVisit` 可测 | 指标字段或 test counter |
| B6 | **S5 门禁测**：count=1000，scroll 扫一圈，单帧 bind ≤ visible+2×cache | 硬断言 |
| B7 | （可选）真窗 `examples/ui_l1_scroll` 显示 1k 行 + 滚轮 | 有 DISPLAY |

### B.3 数字门禁（S5）

| 指标 | 标准（可微调，须写入测试常量） |
|------|--------------------------------|
| `ItemCount` | 1000 |
| `ItemExtent` | 40 |
| `ViewportH` | 400 → 可见约 10 行 |
| `CacheExtent` | 2 行（或 80px） |
| 每帧 `len(mounted children)` | ≤ 10 + 4 = **14**（示例） |
| 全表 mount | **禁止**（`len(children)==1000` 则 Fail） |

```bash
go test ./ui/rendering -run 'TestS5_VirtualList' -count=1
```

---

## 轨 C — 拖拽滚动（S6）

**目标：** 指针拖动更新 offset；用**最新点**；不整表 layout。

### C.1 最小输入（不做完整 GestureArena）

在 **示例或 `ui/rendering` 测试 helper** 中：

```text
PointerDown → 记录 lastY
PointerMove → delta = lastY - y; ScrollBy(0, delta); lastY = y; ScheduleFrame
PointerUp   → 结束
```

引擎侧：`Viewport.ScrollBy` 已满足；可选 `Scrollable` 包装把事件接到 Viewport。

### C.2 任务表

| # | 任务 | DoD |
|---|------|-----|
| C1 | `Scrollable` 或 Viewport 方法：`HandlePointer(ev)` MVP | 单测序列 |
| C2 | Move 合并：同帧多次 move 只应用最后点（或累加 delta 一次） | 单测 |
| C3 | 断言拖拽过程 LayoutCount 不增（bind 窗口变化时允许有限 layout） | S6 |
| C4 | 真窗示例接入拖拽或滚轮（X11 可先滚轮/键模拟） | 可选 |

### C.3 门禁（S6）

| 项 | 标准 |
|----|------|
| 连续 50 次 PointerMove | 最终 offset 正确；`layout` 次数 ≪ move 次数 |
| 输入路径 | 不经过 Wait Present |

---

## 轨 D — 文本 · 图像 IO · 裁剪 · saveLayer

### D.1 文本（引擎级，非 Input 产品）

| # | 任务 | DoD |
|---|------|-----|
| D1 | `RenderText`：字符串 + font size；Layout 用 `render` measure（若有）或近似高宽 | 单测 |
| D2 | Paint：`painting.Context.DrawText` → `render.DrawString*`（逻辑坐标） | 像素或无 panic |
| D3 | （可选）简单 key 缓存：同 text/size 不重复 measure | 计数 |
| D4 | 多行：固定 maxWidth 换行 MVP 或单行 + ellipsis | 单测 |

**render 改动：** 仅当公开 API 无法从 ui 调用时，在 render 增加薄封装；优先用现有 `DrawString` / `DrawStringWrapped`。

### D.2 图像 IO（F12）

| # | 任务 | DoD |
|---|------|-----|
| D5 | 包 `ui/io`：`Decoder` worker 池（goroutine 数可配，默认 2） | 单测 |
| D6 | `DecodeFile` / `DecodeBytes` → 回调 **回到 UI 线程约定**（channel + Host.WakeUp 或 embedder 投递） | 单测：解码函数内不占「模拟 UI 线程」 |
| D7 | `RenderImage`：Loading 占位色块；完成 `SetImage` + MarkNeedsPaint | 单测状态机 |
| D8 | **禁止** 在 `Layout`/`Paint` 内同步 `image.Decode` 大文件 | 代码审查 + 测：Paint 路径无阻塞注入 |
| D9 | （可选）上传 GPU：经 render ImageBuf；失败保留 CPU 显示 | — |

**门禁：**

```text
模拟 UI 线程：10ms 内必须返回 Paint
解码 2MB PNG 在 worker；完成前 UI 可继续 ScheduleFrame
```

### D.3 裁剪

| # | 任务 | DoD |
|---|------|-----|
| D10 | Viewport clip 已含；补 `RenderClipRect` 通用节点（可选） | 单测 |
| D11 | Layer：`ClipRectLayer` 已在 scene；BuildLayerTree 接上 | 单测 |

### D.4 saveLayer 纪律（F16 起步）

| # | 任务 | DoD |
|---|------|-----|
| D12 | `painting.SaveLayerBudget`：单帧 max 面积或次数 | 超限返回 error / 降级 |
| D13 | 文档：默认 UI 路径禁止全屏 saveLayer | CLOSEOUT 或本卡注释 |

---

## 轨 E — 示例 · 指标 · 回归 · 文档

| # | 任务 | DoD |
|---|------|-----|
| E1 | `examples/ui_l1_scroll`：Viewport+VirtualList 1k 行；滚轮或定时 scroll | 真窗可跑 |
| E2 | JSON 增加：`bind_count`、`scroll_offset_y`、`layout_count`（本帧） | 字段稳定 |
| E3 | 回归：`go test ./ui/...`；spinner 测不挂 | 全绿 |
| E4 | 更新 `ENGINE_L1_CLOSEOUT` 或新增「P4 状态」小节；勾选本卡 F 项 | 文档 |
| E5 | （可选）`ui_l1_scroll` 与 spinner baseline 对比说明 | — |

---

## 2. 场景门禁总表（P4 关门）

| ID | 场景 | 通过标准 |
|----|------|----------|
| **S5** | VirtualList 1000 行滚动 | 单帧 mounted ≤ 可见+cache；禁止 1000 children |
| **S6** | 拖拽/程序化连滚 | Layout 次数受控；offset 正确 |
| **S2 回归** | Spinner | layout 不涨；脏层仍小 |
| **S0 回归** | 调度 IDLE | 仍可阻塞 |
| **IO** | 异步解码 | Paint 路径无同步大解码 |
| **依赖** | depcheck | ui 无 import gpu |

```bash
go test ./ui/... -count=1
go test ./ui/rendering -run 'TestS5_|TestS6_|TestViewport_' -count=1
go test ./ui/io -count=1
# 有 DISPLAY：
go run ./examples/ui_l1_scroll
go run ./examples/ui_l1_spinner   # 回归
```

---

## 3. F 项勾选（P4）

| ID | 内容 | 轨 | 状态 |
|----|------|-----|------|
| **F05** | Scroll offset Layer + 虚拟化 | A+B | ✅ |
| **F12** | IO 解码线程 | D | ✅ |
| **F16** | saveLayer 预算起步 | D | ✅ |
| **F10** | 文本基础可用 | D | ✅ |
| 回归 F01–F04/F08/F17 | 不回退 | E | ✅

---

## 4. PR 切片建议

| PR | 内容 | 合并前提 |
|----|------|----------|
| **P4a** | 轨 A Viewport + 测 | `go test` 绿 |
| **P4b** | 轨 B VirtualList + S5 | S5 硬断言绿 |
| **P4c** | 轨 C 拖拽/Scrollable + S6 | S6 绿 |
| **P4d** | 轨 D 文本/IO/clip/budget | io 测绿 |
| **P4e** | 轨 E 示例 + 文档勾选 | 真窗烟囱 + 回归 spinner |

禁止单 PR 混合 L2 手势与 P7 控件。

---

## 5. 风险

| 风险 | 缓解 |
|------|------|
| 虚拟列表 bind 时误 MarkNeedsLayout 整树 | 窗口未变只 paint；测 S5/S6 |
| 滚动 HitTest 坐标反号 | A 轨符号单测钉死 |
| image.Decode 误入 Paint | D8 明确禁止 + 审查 |
| 示例 X11 事件不全 | 先定时 ScrollBy 演示虚拟化，再补指针 |
| 与 P3 spinner 回归 | E3 必跑 spinner 测与示例 |

---

## 6. 完成定义（P4 Done）

- [x] 轨 A–E 主路径（可选真窗指针泵除外）  
- [x] S5、S6 单测硬断言绿  
- [x] `go test ./ui/...` 绿；ui 无 gpu import  
- [x] spinner 相关测仍绿  
- [x] `examples/ui_l1_scroll`  
- [x] 文档勾选  

## 6.1 实现状态（2026-07-27）

| 项 | 状态 |
|----|------|
| `RenderViewport` + ScrollBy / HitTest | ✅ |
| `VirtualList` 固定行高 + BindCount | ✅ |
| `Scrollable` 指针 MVP | ✅ |
| `RenderText` / `RenderImage` + `ui/io` | ✅ |
| `SaveLayerBudget` | ✅ |
| S5/S6 单测 | ✅ |
| `ui_l1_scroll` | ✅ |

---

## 7. 修订

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-07-27 | 首版 P4 细卡：A–E 分轨、S5/S6 数字、PR 切片 |
| 1.1 | 2026-07-27 | 实现完成勾选 |
