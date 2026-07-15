# S6.6 — Path / Stroke / Geometry

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S6.6 关闭**  
> 依赖：S4.3 Path/Stroke geometry cache、S6.2 submit path  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> 主路径：`FillPath` / `StrokePath` → convex / dash / stroke expand / tess → stencil|convex GPU

---

## 1. 目标

提高 **tess / stroke / dash / convex 分类** 在 retained UI 帧上的命中率，控制 AA/path 云成本：

1. **Tess 零拷贝 hit**（immutable vertices）  
2. **Stroke expand 共享 path hit**（无每 hit 重建 verbs）  
3. **Dash 几何缓存**（`ApplyDash` 不每帧重算）  
4. **Convex 分类缓存**（含 negative cache）  
5. **AA-off + dash** 在 snapped path 上应用 dash（正确性修复）  
6. **预算抬升** path 512 / stroke 256 / dash 256 / convex 512  

**非目标**：控件层；整路径迁 Vello compute；改像素填充语义。

---

## 2. 实现摘要

| 组件 | 改动 |
|------|------|
| `PathGeometryCache.GetOrTessellate` | hit 返回共享 `[]float32`；miss 单次 store |
| `StrokeGeometryCache` | 存 `*Path` clone；hit 共享指针 |
| `DashGeometryCache` | `GetOrApply(path,dash,scale)` |
| `ConvexPathCache` | `GetOrClassify` + 负缓存 |
| `FillPath` | 走 convex cache → tess cache |
| `StrokePath` | dash cache → stroke cache → FillPath |
| `GeometryCacheStats` | path/stroke/dash/convex 聚合 |

### 诊断

```go
st := accel.GeometryCacheStats()
// st.PathHits / StrokeHits / DashHits / ConvexHits
accel.ResetGeometryCacheStats()
```

---

## 3. 验证（真 GPU）

| 测试 | 结果 |
|------|------|
| `TestS66_PathTess_ZeroCopyHit` | ✅ 同 slice |
| `TestS66_StrokeCache_SharedPathHit` | ✅ 同 `*Path` |
| `TestS66_DashGeometryCache_Hit` | ✅ |
| `TestS66_ConvexCache_HitAndNegative` | ✅ |
| `TestS66_ComplexPolygon_TessReuse` | ✅ EvenOdd |
| `TestS66_PresentPathStrokeDash_NoRegress` | ✅ **p50≈4.98ms**（soft 66.8） |
| `TestS66_PresentRetainedStroke_NoRegress` | ✅ **p50≈1.16ms** |
| `TestS43_*` | ✅ 回归 |

对照历史（不同测量口径，仅方向参考）：S4 B12 draw_ms ~250ms；S6 H03 present ~130ms → 本机 S6.6 present-only B12-like **~5ms**。

---

## 4. 复现

```bash
export WGPU_NATIVE_PATH=.../lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=.../lib:$LD_LIBRARY_PATH

go test -count=1 ./render/internal/gpu -run 'TestS66_|TestS43_' -timeout 120s
go test -count=1 ./render -run 'TestS66_|TestS6_L0_|TestS65_' -timeout 300s
go test -count=1 ./render -run 'TestP1_Comp_(D01|D06|D08|D36|D63|D152)_' -timeout 300s
```

---

## 5. 退出条件

| 条件 | 状态 |
|------|------|
| tess/stroke 命中率加深（零拷贝/共享） | ✅ |
| dash 几何缓存 | ✅ |
| 复杂 polygon tess 复用 | ✅ |
| stencil vs convex 选择 + 分类缓存 | ✅ |
| B12 类 present 门禁绿 | ✅ |
| 无 silent CPU / 无降像素门禁 | ✅ |

**S6.6 关闭。** 下一：**S6.7 Resources / Image / Atlas / Buffer**。

---

## 6. 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | zero-copy tess、shared stroke、dash/convex cache、TestS66_* |
