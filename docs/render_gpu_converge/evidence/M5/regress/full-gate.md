# M5 后全量回归（render → gpu 主链逐文件门禁 + 关键真窗，2026-11-11）

> 目的：M5 触及 render 公开面与 internal/gpu 会话/管线，按 AGENTS「回归按测试文件逐个跑」
> 对主链 13 包跑逐文件门禁，并用 pre-M5 worktree 做失败归因。

## 0 结果

- 范围：render、render/internal/gpu/res、render/internal/gpu、render/internal/image、
  render/scene、render/recording、render/recording/backends/raster、render/text、
  gpu、gpu/hal、gpu/hal/noop、gpu/webgpu、gpu/rwgpu。
- 725 个测试文件，耗时 1h01m：693 PASS / 4 FAIL / 28 SKIP_TAGGED。
- 失败归因（同一显示、同一环境）：
  - 1 个是 M5 回归（已修）：render/p37_6_h20_test.go。
  - 3 个是既有红（pre-M5 worktree 0ad8bad1 上同样失败、报文与计数一致）：
    render/recording 的 TestCanvasBackendReplaysAtDeviceScale（设备缩放着色边缘
    0.0196 premul 涂抹）、TestDrawShadowReplayParity（716/32000 = P4.3 期在册既有红）、
    TestP37E3CanvasBackendScopedShaderNoLeak（对照组无材质什么都没画）。
- 修复后复跑：H20 PASS；HDR 真窗 native/go 9/9 探针 maxdiff=0、金图 0.0000%。
  ⇒ 最终口径：694 PASS / 3 既有红 / 28 SKIP_TAGGED。

## 1 M5 回归的根因与修复（P2 引入）

- 现象：H20 子进程里 mem_gpu_lifecycle_test.go:118 报 CreateOffscreenTexture unavailable，
  钉变空（skip 当不过）。
- 根因：P2 的 CreateOffscreenTextureFormat 把能力探测放在 ensureGPU 之前；
  GPUShared.offscreenFormatRenderable 在 device 还是 nil 时探测必然 false，并把这个
  false 按 deviceGen 缓存——设备起来后同代次永远读回「不支持」，CreateOffscreenTexture
  从此返回空 view。
- 修法：
  1. CreateOffscreenTextureFormat 先做引擎白名单（无设备也能判），再 ensureGPU/取 device，
     最后才做设备级探测；
  2. GPUShared.offscreenFormatRenderable 在 device==nil 时返回 false 但不写缓存。
- 防回归钉：TestP5OffscreenBeforeDeviceInit（不预初始化设备直接 CreateOffscreenTexture，
  必须拿到目标）；H20 复跑 PASS。

## 2 证据

- per-file-summary.tsv：725 行原始结果。
- fail-before-fix__render__p37_6_h20_test.log.txt：修复前失败报文。
- pre-existing__*.txt：3 个既有红的本树报文（与 pre-M5 worktree 一致）。
- h20_after_fix.txt、hdr_after_fix_native.txt、hdr_after_fix_go.txt。

## 3 结论

M5 自身引入的回归 1 个，已定位到行、修复并钉死；其余 3 个为既有红，不阻塞 M5 收口，
按既有在册口径保留。
