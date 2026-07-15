# S4.2 Glyph / Atlas — 命中与上传收敛

> 版本：1.0 | 日期：2026-07-15  
> 状态：**S4.2 本切片关闭**  
> 依赖：S4.0 基线、S4.1 batch  
> 架构：`render → gpu/webgpu → gpu/rwgpu → libwgpu_native`  
> 主路径：Tier 6 `GlyphMaskEngine` + `text.GlyphMaskAtlas`（R8）

---

## 1. 目标

在**不改字形像素语义**的前提下：

1. 减少 atlas **重复全页上传**（上传收敛）  
2. 保证 **hit 帧零上传**  
3. 每帧 **AdvanceFrame**，使 LRU / stale page compact 生效（Skia `postFlush` 模式）  
4. 可观测 hit/miss 与 upload 统计  

非目标：重做 MSDF 图集、改 hinting/LCD 语义、控件层。

---

## 2. 实现摘要

### 2.1 脏区局部上传（核心）

| 组件 | 变更 |
|------|------|
| `glyphMaskPage` | `dirtyMin/Max` 脏矩形；`expandDirty` 含 1px filter pad |
| `DirtyUploads()` | 返回 `GlyphMaskDirtyUpload`；脏面积 ≥50% 页 → full |
| `SyncAtlasTextures` | 首建纹理仍 full；其后 **partial `WriteTexture` + Origin**；行 pitch 256 对齐 |
| `MarkClean` / `resetPage` | 清/置满页脏区 |

### 2.2 帧推进

`SyncAtlasTextures` 末尾（含 **无 dirty 的 hit-only 帧**）调用 `atlas.AdvanceFrame()`，触发 32 帧 stale compact。

### 2.3 统计 API

```text
GlyphMaskEngine.LastUploadStats() → (bytes, regions, partial, full)
GlyphMaskEngine.AtlasStats()      → (hits, misses, entries, pages)
GlyphMaskAtlas.Stats()            → 既有 hits/misses
```

### 2.4 既有（本切片保留）

- `Get`/`GetOrRasterize` hit 路径  
- `UnderPressure` + `MakeGlyphMaskKeyBucketed`（非 CJK）  
- Text/Glyph batch `CanMerge`（S4.1 前已有）

---

## 3. 验证结果

### 单元 / 集成

| 测试 | 结果 |
|------|------|
| `TestS42_DirtyRegionPartialUnion` | ✅ |
| `TestS42_DirtyRegionFullWhenLarge` | ✅ |
| `TestS42_SyncAtlasTextures_PartialThenHit` | ✅ |

实测（本机，page=1024）：

| 步骤 | bytes | partial | full |
|------|-------|---------|------|
| 首 glyph（建纹理） | 1 048 576 | 0 | 1 |
| hit-only 再 Sync | 0 | 0 | 0 |
| 第二 glyph（纹理已存在） | **4 352** | **1** | 0 |

相对全页上传约 **~241× 字节收敛**（第二 glyph 场景）。

### 回归

- `TestGlyphMask*` / `TestS41_*` / `TestS42_*` ✅  
- D01/D04/D06/D08/D36 / P1.2 SourceOver ✅  
- S4 harness B03/B08/B13 ✅（`GPUOps>0`）

### 场景对照

| 场景 | 说明 |
|------|------|
| B03 TextRows40 | 文本压；首帧 cold atlas，后续 hit+零/少上传 |
| B08 ListScroll | 行文本 + 形态；依赖 hit + partial |
| 多帧 UI | AdvanceFrame 防 atlas 长期膨胀 |

Wall-time 仍可能由 **CPU readback/flush** 主导；S4.2 指标以 **upload 字节与 hit 帧零上传** 为准，而非绝对 FPS。

---

## 4. 复现

```bash
export WGPU_NATIVE_PATH=/path/to/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=.../lib:$LD_LIBRARY_PATH

go test -count=1 ./render/text -run 'TestS42_' -timeout 30s
go test -count=1 ./render/internal/gpu -run 'TestS42_|TestGlyphMask' -timeout 120s
go test -count=1 ./render -run 'TestP1_Comp_D0|TestP1_Comp_D36|TestS4_PerfBaseline' -timeout 180s
```

---

## 5. 退出条件

| 条件 | 状态 |
|------|------|
| 脏区 partial upload | ✅ |
| hit 帧零上传 | ✅ |
| AdvanceFrame 每 Sync | ✅ |
| 统计 API | ✅ |
| 测试 + 像素回归绿 | ✅ |

**S4.2 关闭。** 下一焦点：**S4.3 path/texture cache**。

---

## 6. 修订

| 日期 | 版本 | 说明 |
|------|------|------|
| 2026-07-15 | 1.0 | partial dirty upload + AdvanceFrame + LastUploadStats + TestS42_* |
