# M5 P2：float 离屏 RT（池键 / 账本 / 能力门 / 具名回退）

> 上游：evidence/M5/assessment-design.md §3 G3、§5 P2。本批只让 float 目标能建、
> 能池化、能记账、失败可见；不接 tonemap（P3）、不接真窗（P4）、不改公开 API。

## 0 结论

- 离屏 RT 的池键加入 Format（零值仍映射 BGRA8），RGBA16Float 与 8bit 同尺寸不再互相
  串用；每项字节数改由 hal.VramTextureBytes 按格式算（16F = 8 B/px）。
- 新入口：CreateOffscreenTextureFormat（严格，具名 ErrOffscreenFormatUnsupported）与
  CreateOffscreenTextureFormatOrFallback（不支持时回退 BGRA8，计数 + 告警日志）；
  历史 CreateOffscreenTexture 原签名保留，委托默认 BGRA8，行为不变。
- 能力门 = 引擎格式白名单（BGRA8 / RGBA8 / RGBA16Float）+ 每设备代次缓存的
  「1×1 clear pass」实测。理由：webgpu.Adapter.TextureFormatCapabilities 是空壳
  （gpu/webgpu/adapter.go:114 返回零值），GLES 单看 CreateTexture 也不够（存储可分配、
  FBO 未必完整），跑一遍 pass 才是跨后端的真判据。

## 1 改动

render/internal/gpu/offscreen_format.go（新增）：
- ErrOffscreenFormatUnsupported、offscreenEngineFormats（白名单，fail-closed）、
  normalizeOffscreenFormat（零值→BGRA8）。
- probeOffscreenFormatRenderable：1×1 纹理 + view + clear pass + submit；
  纹理/view/命令缓冲立即释放，不池化、不长期记账。
- GPUShared.offscreenFormatRenderable：按 (deviceGen, format) 缓存判据，探测在锁外跑。

render/internal/gpu/gpu_render_context.go：
- offscreenKey(w,h,format) 把 Format 写进 res.CacheKey（该字段本就存在）。
- offscreenRTBytes 走 hal.VramTextureBytes（16F 8B/px、8bit 4B/px），与 VRAM 账本同口径。
- createOffscreenRT 用 key 的格式建纹理与 view（零值=BGRA8）。
- CreateOffscreenTextureFormat / CreateOffscreenTextureFormatOrFallback /
  OffscreenFormatFallbacks；CreateOffscreenTexture 委托默认格式，旧契约（失败返回空
  view、日志）不变。

render/internal/gpu/gpu_shared.go：加 offscreenFormatProbe map + 代次标记。

## 2 单测（render/internal/gpu/offscreen_format_test.go，5 枚全 PASS）

| 用例 | 断言 |
|---|---|
| TestP2OffscreenFormatKeyAndBytes | 8bit/16F 键不同；16F 项 64×48=24576B（8B/px）、8bit=12288B；零格式仍 4B/px |
| TestP2OffscreenFloatTargetAccounting | 16F 面建出、view 格式正确；VRAM 增量 = 64×64×8；可作 clear pass 附件；release 后回到基线 |
| TestP2OffscreenFormatFailClosedAndFallback | 深度格式 → ErrOffscreenFormatUnsupported 且零分配；OrFallback 回退 BGRA8、计数=1 |
| TestP2OffscreenPoolSeparatesFormats | 同尺寸 8bit/16F 双 miss；release 后重取 16F 命中池且格式仍是 16F |
| TestP2OffscreenLegacyEntryStillWorks | 旧入口返回 BGRA8 面，release 可用 |

## 3 8bit 回归

- go build ./...、go vet ./render/...、go build/vet -tags nogpu ./render/... 全 rc=0。
- TestConformanceDualBackend PASS；离屏触达测试 S61（6）/S63（1）/S67（2）/P43 上传板（1）全绿。
- 真窗：p34_filters 0.0000%（29/29）、p35_rrect4 native/go 0.0000%（logic 6/6）、
  ui_wr_p41_drawshadow 见下（重种后 native/go 0.0000%）。

## 4 顺带发现并归位：drawshadow 是 SDF 覆盖线漏掉的窗（非 P2 引入）

- 现象：ui_wr_p41_drawshadow 在 HEAD（SDF LUT 之后）golden=3.2402%。
- A/B（同一显示、同一参数）：提交 d6cdb2b2（SDF LUT 之前）worktree **0.0000%**；
  HEAD（0ad8bad1）worktree **3.2402%**；P2 工作树同为 3.2402% ⇒ 差异来自 SDF 覆盖
  LUT，不是 P2。
- 差异形态（旧/新快照逐像素）：32018 px，max 200（形状/阴影边缘带），
  平坦区（7×7 变化≤2）只有 1 级差 2423 px（全部落在窗口最外列 254→255 与阴影缓变带）。
- 处置：按 SDF 线同一口径重种 native+go（旧图归档 evidence/SDF/golden-reseed/），
  复验双后端 0.0000%（logic 2/2）。教训：SDF 触及窗扫描不能只 grep DrawCircle/
  DrawRRect——经路径检测进 SDF 的图形（DrawShadow 的圆角卡片）也会命中，补登到
  coverage-line.md 的窗清单。

## 5 下一步

P3 tonemap/曝光 pass：RGBA16Float → 8bit，曲线与 render/hdr.Encode 同式
（曝光(0,16] → Reinhard → sRGB），单测逐像素 max ≤1（禁 mean）；之后 P4 真窗。
