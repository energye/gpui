# P4.3 量测项：mask 池化 / backdrop 损伤区 / 抖动（2026-11-10，量测与登记）

## 1 mask 池化

- 现状：`ModulateMaskedLayerGPU` 用 `rc.shared.maskR8` **单缓冲复用**（每设备一块，
  按需 append），不是按尺寸池化；尺寸变化会重新分配。
- 量测口径：连续切三种 mask 尺寸（64/256/1024）观察 `maskR8` 分配次数与字节。
- 待办：若真实场景尺寸抖动频繁，改为 `res.Cache` 的 `mask-plane` 域（与 filter-graph 同法）。

## 2 backdrop 损伤区

- 现状：backdrop 层 Pop 时**整层 blit**（`drawGPUTextureWithOpacityFullSurface`），
  注释明确是正确性优先（present 只重放尾段）。
- 量测口径：backdrop 卡片静止时的 `frame_raster_ms` 与 `damage_area_px`；
  若能证明窗口 present 对 backdrop 也走增量，则可切 damage-tight UV。
- 待办：先量再决定，禁止直接改（会重演 P4.3 mid-frame 丢失）。

## 3 抖动（dither）

- 现状：`applyDitherIfEnabled` 在每次 flush 后按像素加噪；`FlushGPU` 注释说明
  nil-view flush 会写回 pixmap。
- 量测口径：同场景开/关 dither 的两次快照逐像素差（应只在渐变带出现 ±1 级）；
  以及开 dither 时 GPU 快路径是否被迫回读。
- 待办：确认 dither 不破坏「零回读 GPU 路」——若破，改为 present 期一次性施加。

## 状态

**量测方案已定，本轮未跑真窗**（需单窗串行 + 标准关闭，按 GPU 真窗安全条执行）。
三项都以「先量后改」为前置，量测数据落 `evidence/P4.3/measure/` 后再决定动不动代码。
