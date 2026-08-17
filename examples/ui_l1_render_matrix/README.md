# ui_l1_render_matrix

多方位 L1 渲染验证烟囱（M1–M5）。**示例包**，不是 `ui/` 库的一部分。

## 对齐宣称（必读）

| 判定 | 说明 |
|------|------|
| **PARTIAL** | Flutter 式 L1 **契约**（Boundary / CompositeOnly 统计 / DirtyLayerIDs / 异步 Present / 虚拟列表）可测 |
| **非全量** | **不是** Flutter Engine 对等：无 dirty-RECT 局部 Present、无 Picture 显示列表、无 per-layer GPU RT |

`PipelineApp` 当前每帧仍 **全清 + force 全量 paint**（P6 再做真 damage）。  
因此：**层脏统计局部 ≠ GPU 局部 Present**。

### 允许

> L1 retained-paint 契约在场景/统计层按 Flutter 思路工作；帧成本与脏 **层** 相关（无头门禁）。

### 禁止

> 「已完成 dirty-rect Present / Picture 录制 / P6 / 与 Flutter Engine 全量对齐」。

---

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=15 go run ./examples/ui_l1_render_matrix

# 可选：后端在代码里指定 — platform.Options.Backend 取 DisplayAuto / DisplayX11 / DisplayWayland
```

无头契约门禁（CI）：

```bash
go test ./ui/rendering -count=1 -run 'TestS2_|TestS4_|TestS5_|TestS6_|TestM'
go test ./ui/painting -count=1
```

---

## 面板轴

| 轴 | 内容 |
|----|------|
| **M1** | Spinner + 静态色块（RepaintBoundary） |
| **M2** | 多个独立 Boundary，仅 2 个热区脉动 |
| **M3** | VirtualList 1000 行 + 自动滚动 |
| **M4** | Latin / CJK 文本；latin 仅改颜色 |
| **M5** | `FillLinearGradient` + `FillRoundRect`（painting 薄 API） |

---

## 指标怎么读

| 来源 | 含义 |
|------|------|
| 无头 `PaintVisits`（`FlushPaint(false)`） | retained paint 局部性（真 Flutter 式 walk） |
| `raster_layer_count` / DirtyLayerIDs | 场景脏层集合（契约），**不是** GPU 局部提交证明 |
| 真窗 Present | 仍全清全画 → **不要**用 present 路径 visits 当 P6 门禁 |
| `layout_flushes` | 应 ≪ `presents`；过高则脏区/layout 契约回退 |
| `bind` | 必须 ≪ `itemCount`（虚拟列表） |
| stdout JSON | `FrameMetrics` 快照，可作 baseline |

stderr 结束行会打印 backend / fps / layout_flushes / raster_layers / bind；stdout 为 JSON。

---

## 相关

- 无头：`ui/rendering/render_matrix_test.go`
- 经典门禁：`TestS2_*` / `TestS4_*` / `TestS5_*` / `TestS6_*`
- 架构纪律：`docs/ENGINE_CODING_RULES.md`
- 收口：`docs/ENGINE_L1_CLOSEOUT.md`
