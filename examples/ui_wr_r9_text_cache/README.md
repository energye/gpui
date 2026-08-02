# ui_wr_r9_text_cache — R9 文本 measure 缓存

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_wr_r9_text_cache                 # 默认无限运行，手动关窗退出
RUN_SECONDS=5 go run ./examples/ui_wr_r9_text_cache   # 5s 后自动关闭并输出门禁 JSON
```

## Window / Close duration

1200×800 · 默认**无限运行**（手动关窗退出）；传 `RUN_SECONDS` 按指定秒数自动关闭（`<5 → FAIL (U16)`）。

## 界面布局与预期效果

窗口纵向分区：**顶栏 48px → 左图例 260px → Body (284,60) 904×656 → 底部 HUD 72px**。

```
+--------------------------------------------------------------+
| TOP: ui_wr_r9_text_cache  R9 文本 measure 缓存                |
+----------+---------------------------------------------------+
| LEGEND   | 6 大字标签（2列x3行）     dense 标签列(8个)         |
| (260px)  | 长段落1（380宽换行）      第2段落（300宽换行）       |
|          | 长段落2（300宽换行）      (1064,80)起 2列x4行        |
+----------+---------------------------------------------------+
| HUD: R9 | phase | fps | p95 | hit | miss | presents | 门禁    |
+--------------------------------------------------------------+
```

| # | 区域 | 位置（窗口绝对坐标） | 内容 | 预期效果 |
|---|------|----------------------|------|----------|
| 1 | 大字标签 6 个 | (304,80) 起 2 列 × 3 行 | 字号/颜色各异（12–18px） | 每 tick 强制重布局但文字**不变** → 位置/字号/颜色稳定不抖；measure 全部命中 |
| 2 | 长段落 1 | (304,430) 起，最大宽 380 | 换行长文（每次布局多次 measureLine） | 换行结果稳定；宽高不抖 |
| 3 | 长段落 2 | (724,430) 起，最大宽 300 | 不同宽度换行长文 | 同上；不同断行集合 = 更多 measure 流量 |
| 4 | dense 标签列 | (1064,80) 起，2 列 × 4 行 | 8 个小字标签（9–13px，4 色） | 稳定不抖 |
| 5 | Spike 相位（1.5s→2s） | — | 仅标签 2 文字改变 | **只有它冷 miss**（`spike changed text at …`），其余全部命中 |
| 6 | HUD | (0,728) | fps/p95/hit/miss + 门禁 | **每 ~0.1s 刷新**：fps≈59.9、hit 持续累加（万级）、miss 远小于 hit（几十） |

### 判断标准

- 全场景文字无抖动 = measure 缓存未导致错误尺寸。
- HUD `hit ≫ miss` = 缓存有效（每 tick 布局 16 文本，仅 1 个变更）。
- Spike 相位 miss 微增后回落 = 失效只作用于变更文本（`TreeMeasureCacheStats` 诚实）。

## Gates

- `measure_cache_hit ≥ 1`
- `measure_cache_hit ≥ measure_cache_miss`（hits≥miss，缓存有效）
- `present_policy = full_paint`
- §2.2 全族 A–J JSON 输出
