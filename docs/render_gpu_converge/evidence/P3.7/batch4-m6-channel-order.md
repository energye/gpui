# P3.7 批 4 第二步 · 修 M6：效果离屏导出面 R/B 互换（2026-11-05）

在册项 M6（`tasks.md:393`，2026-11-03 由批 2 前置探针 P1 带出）：`SetEffectSurface(true)` 下
`ExportImageBuf` 读出红蓝整体互换。design §批5 明写"开工前须先按完整流程修 M6"，本文件即该修复记录。

## 1 复现红（探针转正成永久钉，实测数据）

钉：`render/m6_effect_surface_export_test.go`（两枚，`requireNativeGPU`，包 `render_test`）。
同一绘制（白底 + 红块 + 蓝块）在 `effectSurface` 两档下各导出一次，要求**逐通道等值**，
并先自检基准档红蓝各归其位（否则"两档相等"可以与"两档都错"同时成立）。

改造前实测（与 11-03 探针登记值逐字节一致）：

```
--- FAIL: TestM6EffectSurfaceExportKeepsChannelOrder
    红块 两档导出不等值：effectSurface=false [255 0 0 255] / true [0 0 255 255]
--- FAIL: TestM6EffectSurfaceExportIsRepeatable
    effectSurface=true 红块被读成 (0,0,255)：R/B 互换未修净
```

两档都断言过"真走 GPU"（`GPUOps>0` 且 `CPUFallbackOps==0`），排除 CPU 回落干扰。

## 2 取证文件的根因是错的，实测反了一次

`m6-export-byteorder.md` 把病灶定在 `Context.syncViewFlushIntoPixmap`「先试 BGRA 翻转读口、失败退直取」。
真修时给两个读回口与 sync 入口同时打点，跑一次看到：

```
DBG sync entered, ahead= false          ← 出现两次：这条导出根本不进 syncViewFlushIntoPixmap
DBG readback STRAIGHT(RGBA) fmt= 27    ← 全程只有这一处；27 = types.TextureFormatBGRA8Unorm (0x1B)
```

⇒ 真病灶是 **`render/filter_ops_gpu.go:806-813`**：那里把"filter 发布纹理是 RGBA8Unorm"当普适事实，
**无条件**调用不翻通道的 `ReadbackViewStraightRGBA`；而效果面这条路发布的纹理实测是 **BGRA8Unorm**，
直取即 R/B 互换。方向与取证猜测相反——不是"该直取的被翻了"，是**该翻的被直取了**。
取证原文已在 `m6-export-byteorder.md` 末尾加更正段（原文保留可对照，不删）。

## 3 修法（单一判定处，不新增 hal 口）

- 新增 `channelOrderOfView(view)`（`render/filter_ops_cpu.go`）：**问那张纹理自己的格式**——
  `gpucontext.TextureView` 是盒子化的不透明句柄，经正规出口 `UnpackTextureView` 取回 hal 接口，
  读 `Texture().Format()`：RGBA8(Unorm/Srgb) → 直取；BGRA8(Unorm/Srgb) → 翻转；其余 → Unknown。
- 病灶处 `materializeFilterGPU` 改为按该判定二选一；**格式未知一律 fail-closed**（`return false`），
  不再靠"先试一个、报错再退另一个"猜——那种 fallback 正是本洞的成因形态。
- 同类形状一并在 `syncViewFlushIntoPixmap` 改掉（那里也是"BGRA 优先 + 失败退直取"）；它不是 M6 成因，
  但同属"通道顺序靠调用点猜"，收敛到同一个判定处。与在册 M1「合成链色空间口径无单一声明点」并案。
- 引擎侧不新增公开口、渲染层不写 2.5D 概念（分层按用户重申口径）。

## 4 负向（摘掉必红，全部实测）

| 号 | 打断 | 红象 |
|---|---|---|
| N15 | 判据改回硬编 `switch viewOrderRGBA`（等于回到"无条件直取"） | 两枚全红：`两档导出不等值：false [255 0 0 255] / true [0 0 255 255]`、`effectSurface=true 红块被读成 (0,0,255)` |
| N16 | 把格式判定本身掰错（BGRA 也返回 RGBA） | `effectSurface=true 红块被读成 (0,0,255)：R/B 互换未修净` |

还原后两枚钉复绿。

## 5 门禁

- 全量逐文件 sweep（64 包 929 行，`WGPU_NATIVE_PATH` 已导出，跑前不再动代码）：
  **892 PASS / 12 FAIL / 24 SKIP_TAGGED / 1 NO_TESTS**。12 红逐名等于「批 3 收尾 37 包的 8 红」+
  「扩容 video 存量 4 红」，**零新增红**；唯一转绿的是上轮记为抖动的 `video/playback_parallel_test.go`
  （本轮复跑通过，仍属计时抖动族，不计成本批收益）。
- conformance 双后端（`TestConformanceDualBackend`/`TestConformanceInner`）：ok。
- `CGO_ENABLED=1 go test -race ./render/scene ./render/recording ./render/recording/backends/raster`：全 ok。
  `./render` 整包 `-race` 崩在 `thread panicked at src/conv.rs:519`（`invalid bind group layout for pipeline
  layout descriptor`，`TestContextFrameDamage_MultipleFillsSeparateRects`）——与在册 **M4 同一签名**，
  属"整包跑与逐文件口径不同"的存量问题，非本批引入（本批改动包逐文件全绿）。
- `go build ./...` rc=0；`go vet ./render ./render/scene ./engine/sprite` rc=0；`gofmt` 触及文件净
  （`render/context_layer.go`、`render/internal/clip/stack.go`、`render/p37_3d_test.go` 的脏是存量，未动）。
- `go run ./scripts/apidoc` rc=0：本批无新增公开 API（`channelOrderOfView` 是包内私有），无需登记新行。
