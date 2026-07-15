# S4.4 Damage / Retained — 脏区与保留层

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S4.4 本切片关闭**（同时 **S4.x 全线关闭**）  
> 依赖：S4.0–S4.3  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`

---

## 1. 目标

减少不必要重绘：

1. **Damage scissor**：group clip ∩ frame damage，无交集则跳过 draw  
2. **Blit-only LoadOpLoad**：纹理合成路径保留未脏区域（既有 ADR-021/028）  
3. **MSAA 路径**：仍 **LoadOpClear**（不可 LoadOpLoad），但 **scissor 与 damage 求交** 砍 overdraw  
4. **Retained Context**：跨帧复用同一 `render.Context`，摊销 S4.2/S4.3 cache

---

## 2. 实现

### 2.1 Scissor ∩ Damage

| API | 文件 | 行为 |
|-----|------|------|
| `computeDamageScissor` | `damage_scissor.go` | group ∩ damage ∩ surface |
| `damageRectsUnion` | 同上 | multi-rect → AABB union |
| `applyGroupScissorWithDamage` | `render_session.go` | empty damage → 全 group scissor；empty 交集 → skip |

接线位置：

- `encodeBlitOnlyPass` / `encodeBlitToEncoder`（overlay + multi-rect base）  
- `encodeSubmitSurfaceGrouped` / `encodeToEncoder`（**MSAA surface**）

### 2.2 ADR-021 边界（不变）

- **Blit-only**：`LoadOpLoad` + per-rect scissor → 真 damage preserve  
- **MSAA**：resolve 后 sample 丢弃，**禁止**依赖 LoadOpLoad 保内容；S4.4 仅 scissor 减绘  
- 日志：MSAA + damage 时 Debug 说明 scissor-only（不再误报 “ignored”）

### 2.3 Retained 测量（harness）

`s4Scene.Retained`：

- 测量迭代复用同一 `Context`（warmup 后 amortize path/glyph/image cache）  
- 帧间 `p1White` 清画布，避免脏像素串扰统计  
- 新增场景：

| ID | 意图 |
|----|------|
| **B14_RetainedPathText** | retained path stroke + text 行（S4.3 cache 受益） |
| **B15_RetainedMultiDamage** | 双面板 clip 局部更新 + `FrameDamage` API |

---

## 3. 调用链

```
PresentFrame / PresentFrameDamage(Rects)
  → GPURenderTarget.DamageRects
  → RenderFrameGrouped
      ├─ isBlitOnly? → encodeBlitOnlyPass (LoadOpLoad + multi scissor)
      └─ MSAA surface → encodeSubmitSurfaceGrouped
            → damageUnion = damageRectsUnion(rects)
            → applyGroupScissorWithDamage(group, damageUnion)
            → recordGroupDraws | skip
  → webgpu.Queue.Submit → rwgpu → libwgpu_native
```

Readback 测试路径（`activeView==nil`）仍全帧 clear/read；damage 优化面向 present/surface。

---

## 4. 测试与基线

| 测试 | 作用 |
|------|------|
| `TestComputeDamageScissor_*` / `damage_scissor_test` | 交/夹/空集 |
| `TestDamageBlit_*` | 真 native blit damage e2e |
| `TestS44_DamageUnion_MultiRegion` | multi-region union + skip |
| `TestS44_SharedPathCacheAcrossContexts` | retained path cache |
| `TestP1_Comp_D63` / `D152` | FrameDamage present 组合 |
| `TestS4_PerfBaseline_Scenes` | B06 / **B14** / **B15**，`GPUOps>0` |

### B14/B15 抽样（机器相关，warmup/iters 较小 run）

| 场景 | avg_ms | gpu_ops | cpu_fb | 备注 |
|------|--------|---------|--------|------|
| B14_RetainedPathText | ~131 | 34 | 0 | retained path+text |
| B15_RetainedMultiDamage | ~8.7 | 9 | 0 | 局部双区更新 |

完整 JSON：`tmp/s4_baseline.json`。

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=/home/yanghy/app/projects/gogpu/gpui/lib:$LD_LIBRARY_PATH

go test -count=1 ./render/internal/gpu -run 'TestS44_|TestDamage|TestS43_' -timeout 120s
S4_PERF_WARMUP=2 S4_PERF_ITERS=8 go test -count=1 ./render -run 'TestS4_PerfBaseline_Scenes' -timeout 300s
```

---

## 5. 退出条件

| 条件 | 状态 |
|------|------|
| multi-region damage scissor | ✅ |
| blit LoadOpLoad preserve（既有） | ✅ |
| MSAA scissor∩damage（非 LoadOpLoad） | ✅ |
| retained Context harness | ✅ B14/B15 |
| 回归绿 + `GPUOps>0` | ✅ |
| 无控件层 / 无 silent CPU | ✅ |

**S4.4 关闭。S4.0–S4.4 全线关闭。**

**明确后置（仍不阻塞）**：完整 multiplanar YUV、完整 PDF/SVG 引擎（R.02）、与 Skia 绝对 FPS 对标报表。

---

## 6. 修订记录

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | Damage scissor on MSAA+blit；retained B14/B15；关闭 S4.4 / S4.x |
