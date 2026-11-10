# P4.3 ②③ 跨帧阴影 tile 缓存：实现与验收（2026-11-10）

> 性质：用户批准「设备空间 tile + 新增公开符号」后的落地记录。前置：滤波源帧内
> 累积治根 + 金图重种（`root-cause2-fix.md`）。本文件覆盖 ②跨帧缓存、紧致界裁剪
> 与 ④验收；P4.3 **阴影专项**至此收口。

## 0 直话结论

静态帧不再付整画布模糊：drawshadow 真窗 **native 58.91fps / p95 19.45ms**、
**go 58.46fps / p95 19.76ms**（门槛 <22ms），双后端金图 **0.0000%**；关缓存
（`GPUI_P43_BREAK=no-cache`）同窗回落到 **3.40fps / p95 306ms / presents 24**，
反证缓存就是 fps 恢复的原因。缓存命中帧的阴影成本在单元级降到噪声级（相对无阴影
基线 hitShadow≈0，fullShadow≈41.9ms）。

## 1 实现

| 项 | 内容 |
|---|---|
| 缓存位置 | `render` 包进程级（Recorder 在 UI/录制线程必须先于录制知道命中；Context 在光栅线程发布） |
| 键 | `hashShadowPath(path verbs/coords)` + 用户 CTM（6 floats）+ elevation + color + 设备尺寸 + dpr；**禁路径指针键**（每帧克隆路径，指针必失效） |
| tile 内容 | 首个未命中帧在 `Context.DrawShadow` 的 `ApplyBlur` 之后、`PopLayer` 之前，从 **layer pixmap**（已由 `materializeFilterGPU` 写入干净滤波输出）按紧致界裁剪；tile = `CreateOffscreenTexture` + `UploadRGBAToView` + `RegisterTextureRef` |
| 紧致界 | path 设备界 + 抬升 `elevation*dpr` + 3σ 模糊边距，仅与 surface 相交（**不与 clip 相交**：blit 时由现有 clip 裁，clip 变化不会错用旧 tile） |
| 命中（录制期） | `Recorder.DrawShadow` 查 `LookupShadowCache`，命中录 `DrawTextureRefCommand{Device:true, Bounds:...}`（用户空间 damage 足迹），不再录 `CmdDrawShadow`；未命中照旧 |
| 命中（直绘/快照） | `Context.DrawShadow` 查同一键，命中调 `Context.DrawTextureRefDevice`（临时置 user CTM/device scale/projection 为恒等，rect 按设备像素 blit，clip 生效） |
| 资源 | 谁建谁放：tile 由 cache 持有，`Context.Close` 按 owner 释放 `ReleaseTextureRef` + `CreateOffscreenTexture release`；容量 128 条 / 128MiB，只淘汰 **64 代**未用条目，满了不淘汰新条目（新 tile 直接释放，阴影照常走链） |
| 反证 | `GPUI_P43_BREAK=no-cache` 同时关 lookup 与 publish |
| 公开 API | `ShadowTile`、`LookupShadowCache`、`Context.DrawTextureRefDevice`、`Recorder.SetDeviceScale`、`DrawTextureRefCommand.Device/Bounds`；catalog §0/§1/§3.23/§6 同步、apidoc rc=0 |

## 2 单元级（`render/recording/shadow_cache_test.go`，先红后绿用 mutation 反证）

| 测试 | 关缓存（红） | 开缓存（绿） |
|---|---|---|
| `TestShadowCacheHitParity` 第二帧必须录到 device ref，且画面与链式首帧一致 | **FAIL**：第二帧仍是 `DrawShadowCommand`（无 ref） | **PASS**：第二帧录 `DrawTextureRefCommand`；内区逐通道 max diff ≤1 |
| `TestShadowCacheInvalidatesOnContentChange` elevation/color/path/设备尺寸变化必须 miss | — | **PASS**；同参数再查命中且 tile rect >0 |
| `TestShadowCacheNoCacheBreak` 环境反证 | — | **PASS**：lookup false、第二帧不录 ref |
| `TestShadowCacheSecondFrameCost` 命中帧阴影成本 < 链式 1/4 | **FAIL**：第二帧不命中（直接判红） | **PASS**：base=16.3ms / hitFrame=14.6ms / fullFrame=58.2ms ⇒ hitShadow≈0、fullShadow≈41.9ms、ratio 0.000 |

注：`TestShadowCacheHitParity` 比较内区（inset 1px）：链式的全表面层合成在 1px
表面边框留下亚 LSB 的 MSAA 残留（既有层合成行为），命中路径没有；真窗金图用
shell 内容矩形，天然排除该 1px 边框，内区逐位一致。

## 3 真窗（`ui_wr_p41_drawshadow -auto-only`，单窗/标准关闭/前后查显示）

| 运行 | fps_interval | interval_p95 | presents | frame_flushes | golden | probes | cpu_fb |
|---|---|---|---|---|---|---|---|
| native + 缓存 | **58.91** | **19.45ms** | 354 | 1 | **0.0000%** | 9/9 | 0 |
| go + 缓存 | **58.46** | **19.76ms** | 352 | 1 | **0.0000%** | 9/9 | 0 |
| native `no-cache`（反证） | 3.40 | 306.35ms | 24 | 8 | 0.0000% | 9/9 | 0 |

- 双后端金图都对比**重种后的干净 25% 阴影**（`golden-reseed/`），未再重种。
- fps 门槛来自 §2.1：`interval_p95 < 22ms`（或 fps 55+），两者均满足。
- 缓存开启后 `frame_flushes` 8→1、filter-graph 池 hits/misses 从 442/83 降到 10/11：
  静态帧不再跑模糊图。

## 4 显存/资源

- 关窗后池账本与 no-cache 逐项相同：`device-surface` 15.36MB / `filter-graph`
  30.72MB（`entries=3/8`，`idle` 全空闲）；6 个 tile 约 1.4MB，`Context.Close`
  的 `releaseShadowCacheOwner` 已释放，无残留。
- 首次发布仍付一次全画布链（首帧 445–645ms），随后 59fps 稳态。

## 5 边界（如实登记）

- **live FBO 抓帧**：本机窗口走 Wayland，P4.1 那套 `xwininfo` 抓帧钩子拿不到窗口
  （同一环境 `ui_render_base_backdrop` 的 screen check 也 `grab failed`）；上轮 live
  钩子是 go/GL 专用且已删除。本批的 live 路径由 ① 录制回放单元 parity（第二帧真实
  走 Recorder→Playback→CanvasBackend.DrawTextureRefDevice）＋ ② live 帧 fps/presents
  （350+ 帧）＋ ③ 快照/直绘同命中路径覆盖，未另抓 FBO。
- **真窗 `-race`**：`go run -race` 在 GPU 库的 unsafe 指针路径触发
  `fatal error: checkptr: pointer arithmetic result points to invalid allocation`（既有
  工具链限制，非本批代码）；单元 `-race`（`TestShadowCache*`、`render/internal/gpu`
  整包）全绿。真窗并发由 cache 互斥表 + TextureRef 注册表互斥保护，淘汰只动 64 代
  未用条目。
- **跨窗口**：键是进程级、按内容/CTM/尺寸/scale 隔离；两个窗口同内容同尺寸同 CTM
  会共享 tile（同一 GPU 设备可复用；不同设备需按 context 维度再隔离，登记限制）。
- **1px 表面边框**：见 §2 注，非内容区差异；如需逐位锁边框，应另立层合成 MSAA
  残留修复（不属 P4.3）。
- **偏移 Picture 层**：设备空间 tile 假定当前 target 原点 = surface 原点。目标窗的
  卡片直接画在 base（全窗 boundary 原点 0，`boundary_count=1`），不受影响；若未来
  在带平移的嵌套 Picture 层里命中，需要把设备 rect 反算成 layer-local（登记限制，
  不为它扩大本批范围）。

## 6 状态

- **P4.3 阴影专项 ✅**：①分段归因 + 两层治根（Canvas 一等设备纹理命令、滤波源
  帧内累积）+ ②③ 紧致界跨帧缓存 + ④红测/双后端金图/fps/显存/回归。
- P4.3 台账其余子项（mask 临时纹理池化、backdrop 损伤区合成、抖动路径优化）未动。
- 证据：本文件 + `root-cause2.md` / `root-cause2-fix.md` / `golden-reseed/` +
  `render/recording/shadow_cache_test.go`。
