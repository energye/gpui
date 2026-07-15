# S6.5 — Text / Shaping / Atlas

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S6.5 关闭**  
> 依赖：S4.2 glyph atlas partial upload、S6.3 text/glyph coalesce  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> 主路径：`DrawString` → `GlyphMaskEngine.LayoutText` → shape/layout cache → atlas Sync

---

## 1. 目标

在**不改字形像素语义**（LayoutText 仍走 Face.Glyphs/cmap+advance）前提下：

1. **shape/layout 结果缓存**：重复字符串零整形成本  
2. **字体 run 合并复用**：MultiFace 连续同 face 已合并；Runs 结果缓存  
3. **LCD/glyph 上传**：保留 S4.2 partial + hit 零上传；LCD 路径 smoke  
4. **atlas 增长策略**：沿用 MaxAtlases/LRU/UnderPressure bucket + AdvanceFrame  
5. **滚动复用**：U02/list 形态下 atlas hit↑、upload bytes↓  

**非目标**：控件层；把 LayoutText 强切 OT Shape（会改 ligature 像素，后置）。

---

## 2. 实现摘要

| 组件 | 改动 |
|------|------|
| `text.LayoutGlyphs` | Face.Glyphs → `[]ShapedGlyph` + 全局 soft-LRU |
| `text.Shape` | OT Shape 结果进同一缓存（`mode` 隔离 layout vs OT） |
| `GlyphMaskEngine.LayoutText/Aliased` | 走 `LayoutGlyphs`（热路径） |
| `MultiFace.Runs` | 连续同 face 合并 + 结果缓存 |
| `SDFAccelerator.GlyphMaskUploadStats` | 诊断 last sync upload |
| Stats | `ShapeResultCacheStats` / `ClearShapeResultCache` |

### Key 字段

`textHash · fontID · sizeBits · direction · features · language · variations · mode`

### 诊断

```go
st := text.ShapeResultCacheStats() // Hits/Misses/Entries/Evictions
text.ClearShapeResultCache()
text.ResetShapeResultCacheStats()
hits, misses, entries, pages := accel.GlyphAtlasStats()
bytes, regions, partial, full := accel.GlyphMaskUploadStats()
```

---

## 3. 验证结果（本机真实 GPU）

| 测试 | 结果 |
|------|------|
| `TestS65_LayoutGlyphs_CacheHit` | ✅ 二次共享 slice |
| `TestS65_Shape_CacheHit` | ✅ OT 缓存；与 Uncached GID 一致 |
| `TestS65_LayoutAndShape_ModeIsolation` | ✅ layout/OT 不串 key |
| `TestS65_MultiFaceRuns_CacheAndMerge` | ✅ 单 run 合并 + 缓存 |
| `TestS65_LayoutText_UsesShapeCache` | ✅ LayoutText 命中 |
| `TestS65_ScrollReuse_AtlasUploadConverges` | ✅ cold 1 048 576 → warm2 **0** bytes；hits 128→426 |
| `TestS65_LCD_LayoutCacheSmoke` | ✅ warm 零上传 + shape hit |
| `TestS65_PresentListScroll_NoRegress` | ✅ present-only **p50≈4.0ms**（budget 16.7）；shapeHits=88 |

### 滚动复用实测

| 步骤 | upload bytes | atlas hits |
|------|--------------|------------|
| cold | 1 048 576 | 128 |
| warm1 | 3 072 | 276 |
| warm2 | **0** | 426 |

---

## 4. 复现

```bash
export WGPU_NATIVE_PATH=/path/to/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=.../lib:$LD_LIBRARY_PATH

go test -count=1 ./render/text -run 'TestS65_' -timeout 60s
go test -count=1 ./render/internal/gpu -run 'TestS65_|TestS42_' -timeout 120s
go test -count=1 ./render -run 'TestS65_|TestS6_L0_|TestS64_' -timeout 300s
go test -count=1 ./render -run 'TestP1_Comp_(D01|D06|D08|D36|D63|D152)_' -timeout 300s
```

---

## 5. 退出条件

| 条件 | 状态 |
|------|------|
| shape/layout 结果缓存接入热路径 | ✅ |
| MultiFace run 合并 + 缓存 | ✅ |
| LCD + scroll 上传收敛 | ✅ |
| B03/B08/U02 类 present 不回退 | ✅（list-scroll p50≪16.7） |
| GlyphMask / X.* 抽样绿 | ✅（回归套件） |
| 无 silent CPU / 无降像素门禁 | ✅ |

**S6.5 关闭。** 下一：**S6.6 Path / Stroke / Geometry**。

---

## 6. 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | LayoutGlyphs + Shape 缓存；MultiFace Runs 缓存；TestS65_*；scroll/LCD 收敛 |
