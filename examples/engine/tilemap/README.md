# game_tilemap（S57：7.1 瓦片 / 7.2 分区 / 7.3 远近）

独立真窗，三 case，只读调用 `engine/tilemap` 真包（map/chunk/lod 三组冻结 API），认 TMX 子集。引擎代码一字不动。

## 跑法（和 game_step / game_save 同套路）

```sh
go run ./examples/engine/tilemap --case=map -auto-only          # 自检探针 + 短窗，stdout 出 wrgate JSON
go run ./examples/engine/tilemap --case=chunk -manual-seconds 60 # 常驻 60 秒，收真事件改标题，关窗出汇总 JSON
go run ./examples/engine/tilemap --case=lod                      # 自检后常驻到关窗（RUN_SECONDS 可定时）
```

`--case` 只要 `map/chunk/lod`，别的直接报错退出。`-auto-only` 默认跑 8 秒（`RUN_SECONDS` 可改，有下限门）。

## 三 case 验什么

| case | 调的真包 | 自动判（stdout JSON + stderr 自检行） | 人工看 |
|---|---|---|---|
| `map` | `ParseTMX/At/CellToWorld/WorldToCell/ObjectsIn/IsSolidAt/NavCostAt/OccludedAt/AutoMaskAt/AutoVariant4/NewIso` | 摆怪对（hero/chest/dot 全命中）、firstgid=1 不错位、10 格 GID 逐位对、5 连笔刷中点掩码=10 且零断缝、斜 45 度中心往返 | 大地图走，怪和箱子都在位 |
| `chunk` | `NewChunker/ChunkOf/ChunkBounds/Needed/Update/Visible/IsLoaded/Reset/LoadedCount` | 边走边装每步 `Loaded==Needed`、跳跃镜头无脏块残留、万级场（100x100=10000 瓦片）小视口只装 1/49、256 瓦片一批 | 拖视口走，块进出不闪 |
| `lod` | `NewLOD/Select/ChunkDist/ChunkLevel` | 13 格边表（192 算近、320 算远、带内抱老档）、带内 80 次抖动零翻转、扫过两线恰好 2 切 | 焦点来回扫，临界不闪 |

## 门槛数（只认总账 §5，这里只写本窗单数）

- 三证据：逻辑探针 + 像素断言（单通道容差 4）+ 离屏金图 **0 容差**，缺一即 FAIL。
- `-auto-only` 在 wrgate 通门（`MinPresents: 1`）之外另有领域门：map 要求 `misplaced=0/seam_breaks=0`；chunk 要求走装过程中 `loads>0/unloads>0`；lod 要求 `switches>=1/flickers==0`。
- 窗口 1200x800，正式包 presents400+、单窗 fps57+，帧门见总账 §5。

## testdata

- `map_ortho.tmx` / `map_chunk16.tmx` / `map_iso.tmx`：TMX 子集夹具（正交 4x3 四层三怪 / 16x16 分区场 / 斜 45 度 3x3），从 `engine/tilemap/testdata` 原样拷贝。
- `tilemap_map_golden.png` / `tilemap_chunk_golden.png` / `tilemap_lod_golden.png`：离屏确定性帧基线，首次 `-auto-only` 自动生成，之后逐位比对，改基准先查代码不许直接更新图。

## 对齐口径

Godot `modules/tilemap/tile_map_layer.h + tile_set.h`（G12）：三段寻址 + `rendering_quadrant_size=16`（256 块一批）+ `physics_quadrant 16` + 地形自动拼只按 gid 画；默认值对照 D10（tile 16x16、quadrant 16、y_sort_origin 0）差异见总账能力行回写。
