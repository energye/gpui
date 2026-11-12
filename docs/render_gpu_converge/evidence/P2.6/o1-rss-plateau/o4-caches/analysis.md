# O4 m4 长跑缓存残差（2026-11-12）

> 输入：`fix1_m4_virtual_go_300s`（O1 修复后 300s，pprof 60/180/280s）。
> 目标：定位 `noteRerecord` 归因 map 与 `buildShapedCarets` 的增长性质，
> 只修无界项，有界缓存不制造「假修复」。

## 0 直话

- **`noteRerecord` 是无界诊断 map**：60→180→280s 绝对量 1.5→3.0→5.5MB 单调增；
  m4 的测量行在 `liveKeys` 里，仅靠 live-key 清扫清不掉 → 必须加硬上限。
- **`buildShapedCarets` 是有界缓存**：绝对量 9.0→16.0→13.0MB 非单调；
  持有者是每 `RenderText` 的 `layoutCache`，`putCached` 已有 4096 条上限
  （满则清表），不是泄漏。不修，只留证据。

## 1 持有栈（pprof -traces）

- `noteRerecord`：`recordLocal → noteRerecord`（`PictureTextureCache` 诊断归因，
  r10 窗按格归因门禁消费）。
- `buildShapedCarets`：`RenderText.measureLine → measureSize → Layout →
  VirtualList.Layout → FlushLayout`；另一条 `putCached → rowLinesParts`，
  即 `layoutCache`（每 RenderText 一份，4096 条上限）。

## 2 修复（noteRerecord 有界）

`ui/scene/textured.go`：

- `BeginFrame` 的 live-key 清扫加入 `rerecordBy`，并压缩 `rerecordOrder`
  （死 key 先清，治「滚出视口仍计」）。
- 硬上限 `maxRerecordAttrEntries=1024`（≈1.5MB 实测），`rerecordOrder` 最老优先
  淘汰，优先淘汰非 live key；live 全占满时硬淘汰最老（诊断不能压过正确性）。
- 语义不变：`RerecordByKeySnapshot` 仍是拷贝；r10 的 live 网格键（几十个）
  不会被淘汰；死行只影响历史归因。

### 先红后绿

| 用例 | 修前 | 修后 |
|---|---|---|
| `TestPictureTextureCache_RerecordAttributionSweepsDeadKeys` | 红：死 key 22 保留 | 绿：live key 11 计数保留、22 清掉 |
| `TestPictureTextureCache_RerecordAttributionBounded`（1024+32 键） | 红：4128 条 | 绿：≤1024，最新键保留 |
| `TestPictureTextureCache_RerecordAttributionKeepsLiveKey` | 红：live key 可能被挤掉 | 绿：最老 live key 1 保留 |

## 3 真窗复核（m4 go，单窗自关）

| run | 时长 | RSS steady | verdict | pprof `recordLocal`（含归因 map） |
|---|---|---:|---|---|
| 无上限基线 | 300s | 1581KB/min | pass | 1.5→3.0→5.5MB 单调增 |
| cap=4096 | 300s | 2509KB/min | pass | 2.0→3.0→6.5MB（填到上限） |
| **cap=1024** | 300s | **1212KB/min** | **pass** | **1.50→1.50→1.50MB（60/180/280s 持平）** |
| cap=1024 | 180s | −3328KB/min | pass | 仅 60s 点 1.5MB（示例固定 60/180/280 采样） |

- `buildShapedCarets` 300s：12.5→8.0→7.5MB（cap1024 轮）非单调，有界。
- r10 对照：当前树与干净 HEAD `82cadd45` 同窗 35s 都红在 `hitch_le_budget` +
  金图 1.2455%（环境既有红）；键级归因门禁因提前失败未执行，归因语义由
  `ui/scene` 单测锁定。不把既有红记到 O4 头上。

## 4 待拍板（停下报告）

- O4 已修掉唯一无界项，300s 封顶后 map 持平、RSS 1200KB/min pass；
  `buildShapedCarets` 有界（4096/每 RenderText）。
- 台账 O4 原计划「2h 档复核」：是否现在跑 2h soak（需隔离显示，单窗长期占 GPU），
  还是以「硬上限 + 300s 持平 + 有界缓存证明」收口，2h 另立 soak 批？
