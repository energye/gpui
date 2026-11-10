# P4.3 ②③ 第二次尝试：实现、红绿与真窗失败定位（2026-11-10）

> 性质：**失败记录 + 回退结论**。按用户硬规则「搞不定如实说没修好并回退，
> 禁止交付留残缝/半吊子改动」，本会话 ②③ 实现已全部回退，工作区无 ②③ 代码。
> 底层治根（`9f48051c`）与 ① 归因（`82a4985d`）不受影响、保持已验证状态。

## 0 直话结论

**②③ 没修好。** 单元级红测先红后绿、直画 vs 录制回放 parity 逐位一致（diff=0）、
成本断言 0.2144 < 1/4；真窗 **fps 恢复了**（fps_interval 57.1、p95 19.98ms<22ms、
presents 344），但 **golden diff 恒 33.0371%**（缓存帧阴影偏淡/缺失）。
逐步定位后最好的一次仍 11.1802%，且伴随 fps 回落。没有逐位对齐就不标绿，全部回退。

真窗失败的唯一确定性结论：**缓存纹理能正确贴进 present 帧**（red-tile 292085 红像素，
对比上一版只有 1272 —— 底层 `DrawTextureRefCommand` 治根确实生效），但
**「直接 GPU quad 贴缓存纹理」与「PopLayer 层合成」的呈现口径不一致**，差异为整幅
偏亮、逐通道 1–12 级。剩余根因未定位，不猜、不调倍数、不留残缝。

## 1 实现（已回退）

按 `p43-rewire-design.md` 接线铁律：缓存表放 `render` 包进程级，
`Recorder.DrawShadow` **录制期**查缓存、命中录 `DrawTextureRefCommand`，
`Context.DrawShadow` 原链后发布 tile，命中走 `Canvas.DrawTextureRef`。

- 新增 `render/shadow_cache.go`：进程级 LRU 表（key = path 内容哈希 + elevation +
  color + 用户空间 CTM + 逻辑画布尺寸，禁指针键）、`shadowDeviceBounds`、
  `publishShadowTile`、dpr 变化时清表、`Context.Close` 释放 owner 条目。
- 新增 `render.LookupShadowCache(...)`（录制期查询桥接，唯一新增公开符号，
  catalog 同步过、回退时一并撤销）。
- `Context.DrawShadow`：命中直接 `DrawTextureRef`；未命中走原链并在 `ApplyBlur` 后
  `PopLayer` 前发布 tile；投影分支不缓存。
- `Recorder.DrawShadow`：命中录 `DrawTextureRefCommand`（设备空间紧致界几何）。
- `GPUI_P43_BREAK=no-cache` 关闭缓存（mutation 反证）。

## 2 红测（先红后绿，实现后全绿）

`render/recording/shadow_cache_test.go`（随回退删除）：

| 测试 | 实现前 | 实现后 |
|---|---|---|
| `TestShadowCacheRecorderHit` 首帧原链 / 第二帧命中录命令 / 两帧 parity | FAIL（第二帧无 `DrawTextureRefCommand`） | PASS，**max channel diff = 0 / 32000 px**，且阴影确实在（非两边全白） |
| `TestShadowCacheInvalidation` 改 elevation/颜色/path 内容不命中 | PASS（护栏） | PASS |
| `TestShadowCacheNoCacheBreak` 关缓存不命中 | PASS（护栏） | PASS |
| `TestShadowCacheCost` 静态二帧第二帧 < 1/4 | FAIL ratio 0.9056 | PASS first=88.5ms second=19.0ms ratio **0.2144** |

门禁：双构建、nogpu、`TestTextureRef*` / tier5 / upload plate 回归、`TestShadowCache*`
全绿；`TestDrawShadowReplayParity` 的 716/32000 与干净树既有红数字一致，非本批引入。

## 3 真窗客观数据（`ui_wr_p41_drawshadow -auto-only`，单窗、标准关闭、前后查显示健康）

| 运行 | golden diff | fps_interval | interval_p95 | presents |
|---|---|---|---|---|
| `no-cache`（基线） | **0.0000%** | 3.36 | 310.8ms | 19–24 |
| 缓存开启（紧致界 tile） | 33.0371% | 57.1 | 19.98ms | 344 |
| 整幅 tile / 接管 filter GPU view | 15.0818% | 31–34 | 61–70ms | 187–205 |
| 命中贴图走层合成包裹 | 11.1802% | 29.3 | 67.5ms | 178 |

诊断计数（缓存开启）：`REC hit 2450` / `CTX hit 7` / `STORE 7` /
`CB.DrawTextureRef 2478`（`dc=*render.Context`、ref 有效、rect 正确）——
**录制期命中、Playback 分发、最终执行三层都确认到位**。

## 4 逐步定位链（全部客观判据，不用 mean）

| # | 实验 | 结果 | 排除了什么 |
|---|---|---|---|
| 1 | no-cache vs 缓存帧逐像素 | 缓存帧整体**更白**：`(488,82) 254→255`、`(475,120) 246→255`、`(900,260) 205→247`；`no-cache vs cache` diff 261674 px | 确认差异来自命中贴图 |
| 2 | `red-tile`：缓存 tile 填纯红不透明 | 快照 **292085 红像素**（bbox 覆盖全部 tile 区域） | **排除「贴图不进 present 帧」**——底层 `DrawTextureRefCommand` 治根有效（上一版仅 1272） |
| 3 | `gray-tile`：tile 填 `(0,0,0,128)` | `(900,260) 205→127`，半透明黑正确暗化 | 排除 alpha 混合 / shader 缩放 |
| 4 | tile CPU dump + **上传后 GPU 读回** | 两者 alpha 直方图与 center `(135,15,15,136)` 逐项一致，tile 是 premul 黑 alpha 0→136 渐变 | 排除上传/stating/纹理内容错误 |
| 5 | `amp-tile`：tile alpha ×6 | `(900,260) 247→207`（接近 gold 205），但整体 diff 33%→40.4%（强区过头） | 排除「均匀倍数」；确认是**非线性口径差**，调倍数=打补丁，放弃 |
| 6 | 整幅层内容（与 `seedTopLayerGPUFromPixmap` 同源、同尺寸、同上传口） | 15.0818%、`cache brighter than gold: 69010 px / darker: 393 px`、幅度 1–12 级 | 排除「紧致界抠取」；差异是**整幅偏亮** |
| 7 | 直接接管 `ApplyBlur` 发布的 `filterGPUView`（零读回零重传） | **与 #6 完全相同的 15.0818%** | 排除 CPU pixmap 往返；确认 `filterGPUView` 与 pixmap 内容一致，差别在**呈现调用** |
| 8 | 命中贴图走 `PushLayerIsolated + 层内 DrawGPUTexture + PopLayer`（复用标准层合成） | 11.1802%，fps 掉到 29 | 仍未逐位对齐，且性能退化；停止换贴法 |

**剩余根因假设（未证实，供下会话）**：`DrawGPUTexture*` 直接贴缓存纹理与
`PopLayer` 的 `canGPUComposite` 合成在**纹理格式 / premul 口径 / 颜色空间**上不一致
（`filterGPUView` 可能是 RGBA8 straight，层 RT 是 BGRA8 premul；`UploadRGBAToView`
无条件做 R↔B swizzle 而 `ReadbackViewStraightRGBA` 不做）。差异特征：整幅偏亮、
1–12 级、`69010 px 变亮 vs 393 变暗`。需要从 GPU 层格式契约查起，而不是继续换贴法。

## 5 回退清单（本会话 ②③，已全部撤回）

- 删：`render/shadow_cache.go`、`render/recording/shadow_cache_test.go`
- `git checkout --`：`render/context_misc_draw.go`、`render/context_texture_ref.go`、
  `render/context_transform.go`、`render/context_construct.go`、
  `render/recording/recorder_canvas_ops.go`、`render/recording/backend_canvas.go`、
  `docs/RENDER_API_CATALOG.md`（撤销唯一的 `LookupShadowCache` 登记）
- 所有临时诊断（red/gray/amp-tile、tile/PNG dump、READBACK、REC/CTX/CB 日志、
  fmt/os import）随回退清零，**工作区零 ②③ 代码残留**。
- 回退后验证：`go build ./...` / `-tags nogpu ./...` rc=0；
  `TestTextureRef*` 绿；`go run ./scripts/apidoc` rc=0。

## 6 状态与下一步

- ① 分段归因 ✅ · 底层治根（Canvas 一等可录制设备纹理命令）✅ · **②③ 未修好、已回退** ·
  P4.3 仍 🟡；原症状（330ms/帧 ≈3fps）仍在。
- 金图双后端 0.0000% 锁定**未重种**；`fps_gate=off` 正确性窗语义不变。
- 下一步（需先做影响面评估 + 停下报告拍板）：
  1. 查清 `filterGPUView` / 层 RT / `UploadRGBAToView` / `ReadbackView*RGBA` 的
     **格式与 premul 契约**，从 GPU 层对齐，而不是继续换贴法；
  2. 或改走「缓存 `PopLayer` 实际合成的那张 `layer.gpuView`」路径（在 PopLayer
     合成后接管，需评估层 RT 延迟释放语义）；
  3. 每条都要重新跑本文件 §2 红测 + §3 真窗三证据 + 双后端 + 显存 + 全门禁。
