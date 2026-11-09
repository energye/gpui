# 登记：./render/ 全量测试红清单（测试间干扰 + 偶现，2026-11-07 实测）

- 实测环境：当前 HEAD（0608c605 批 6 提交点），`go test ./render/... -count=1 -timeout 30m`
- 性质判定：**单跑全绿、全量红**（抽样复验：TestS3c_M3_RecordingPlayback、TestFontSourceCacheConcurrentLoadOnce 单跑 ×5 绿）——测试间全局状态污染（全局 accelerator / 全局注册表 / 并发负载），非功能性失败
- 既有性：批 5 提交点 99a74660 worktree 干净树复测同集合红（批 5/6 改动无关）；更早起点未考——归交付后审查另线
- 门禁口径：本域各批门禁以「定向 -run + 触及面逐文件」为准（全量干扰不影响定向口径）

## ./render/ 全量红 20 枚

| # | 测试 | 备注 |
|---|---|---|
| 1 | TestTextTransformAutoRoutesOutlines | |
| 2 | TestVideoDrawFrameMarksDamage | |
| 3 | TestVideoBridgeFallbackConverged | |
| 4 | TestVideoBridgePlanesFallback | |
| 5 | TestMiscDrawGPUPath | |
| 6 | TestDrawShadowDrawsBlurredOffsetSilhouetteOnly | |
| 7 | TestPresentDamage_ImageMatchesView | |
| 8 | TestP1_Capability_K02_DrawIndirectGPU | |
| 9 | TestP1_Capability_CS02_RGBA16FloatSurfaceGPU | |
| 10 | TestR72_ApplyBlur_GPUFilterTextureNoExportPrefer | |
| 11 | TestRasterizerValueMatchesAnalytic | |
| 12 | TestS3c_M3_RecordingPlayback | 单跑绿已复验 |
| 13 | TestS3c_M3_BlendHue | |
| 14 | TestS3c_M3_ClipRectDifference | |
| 15 | TestS3c_M3_ClipRectReplace | |
| 16 | TestS3c_M3_PathBooleanIntersect | |
| 17 | TestS3c_M3_TextUnderline | |
| 18 | TestS3c_M3_DrawWithEmojiAPI | |
| 19 | TestS69_HeavyBudget_TierGates | |
| 20 | TestV3_VideoCrossDeviceRealWindow | |

## render/text 包（1006 测试函数）

- 全包非 race 20m / race 40m / 30m 三次超时——**体量问题非失败**；触及面文件级（color*/COLR/SVGTable/adapter/e2e/shape cache）race+非 race 全绿
- 全量截断前抓到 **TestFontSourceCacheConcurrentLoadOnce (0.00s) FAIL** 一枚——单跑 ×5 全绿，归并发负载偶现（上表同性质），建议交付后审查批复验

## 其余 render 树（全量同步实测）

gpu/hdr/integration/internal/*/recording/scene/svg/text/cache/emoji/hint/msdf **全 ok**（hint 339s 为最大体量单包）。

---

## 2026-11-09 复核（HEAD `972cc37c`，带 `WGPU_NATIVE_PATH`）

> 口径：主包 `go test ./render -count=1` 单包隔离跑，失败项再逐个 `-run '^Test$'` 单跑；
> `go test ./render/...`（除 `render/text`）与 `go test ./gpu/...` 各跑一遍。
> 多包并行会因全局 GPU 状态多出假红（K02/CS02/ConformanceDualBackend 单跑均绿），不计入。
> 无库跑 `./render` 有 204 红（GPU 用例回落 CPU 后断言失败），也不计入。

### A. 单跑也红（真实红）

| # | 测试 | 当前现象 | 历史/性质 |
|---|---|---|---|
| 1 | `TestTextTransformAutoRoutesOutlines` | Auto vs Vector diff 569px（want ≤8） | P2.5 逐文件已红，存量 |
| 2 | `TestVideoDrawFrameMarksDamage` | DrawVideoFrame left no damage | P2.5 逐文件已红，存量 |
| 3 | `TestVideoBridgeFallbackConverged` | Bridge fallback left no damage | P2.5 逐文件已红，存量 |
| 4 | `TestVideoBridgePlanesFallback` | planes fallback left no damage | P2.5 逐文件已红，存量 |
| 5 | `TestMiscDrawGPUPath/DrawColorEffectSurfaceAdvancedBlend` | multiply=(0,0,0,255)，want≈(229,0,0,255) | P2.5 已红；基线是 255，当前更差 |
| 6 | `TestMiscDrawGPUPath/DrawShadow` | 上方 path=(254,254,254,255)，want white | 基线 PASS，当前红（1 LSB） |
| 7 | `TestPresentDamage_ImageMatchesView` | present 后 Image() 仍全白 | P2.5 逐文件已红，存量 |
| 8 | `TestP1_Capability_CS03_LinearBlendMidGPU` | mid=131，want≥150 | P1.3/P3.5 PASS（mid=189），当前红，线性插值疑似失效 |
| 9 | `TestRasterizerValueMatchesAnalytic/菱形(分数行起止)` | SparseStrips/TileCompute max diff 8（102px） | P2.5 逐文件已红，存量 |
| 10 | `TestLoadWebP_LosslessPixels` | format=RGBAPremul，want RGBA8 | P1.3 PASS，当前红 |
| 11 | `TestPlaybackDrawImage` | 右上=(95,159,0,255)，want green | P1.3 PASS；K1 构建洞修好后暴露 |
| 12 | `TestH4B3_GlesDeviceLostErrorScopeStubs` | IsLost()=true，want false | P1.3 PASS；P2.6 已红，GL 线 |

### B. 仅全量跑红、单跑绿（测试间干扰，8 个）

- `TestR72_ApplyBlur_GPUFilterTextureNoExportPrefer`
- `TestS3c_M3_RecordingPlayback`
- `TestS3c_M3_BlendHue`
- `TestS3c_M3_ClipRectDifference`
- `TestS3c_M3_ClipRectReplace`
- `TestS3c_M3_PathBooleanIntersect`
- `TestS3c_M3_TextUnderline`
- `TestS3c_M3_DrawWithEmojiAPI`

机制：全量跑里前序测试释放了 GPU device，后续用例报 `wgpu: resource already released` 或 CPU fallback；逐个单跑全绿。

### C. 环境红（1 个）

| 测试 | 包 | 现象 |
|---|---|---|
| `TestPresentDamageX11` | `gpu/gwgpu/gles` | partial present never engaged / BadMatch；需隔离真 Xorg 复核（G3） |

### D. 超时/偶现

- `render/text` 主包 1006 个测试，全包 20m/30m/40m 均超时——体量问题；触及面分文件跑绿。
- `TestFontSourceCacheConcurrentLoadOnce` 全量跑偶现 FAIL；单跑 ×1 绿（本轮复验）。

### 与 K6 20 枚清单的关系

K6 的 20 枚（2026-11-07）当前复核：8 枚仍是全量干扰（上表 B）、7 枚单跑也红（上表 A 的 1/2/3/4/5/7/9）、5 枚已绿（`TestDrawShadowDrawsBlurredOffsetSilhouetteOnly`、K02、CS02、S69、V3）。
K6 当时只抽样复验 2 枚就推广到 20 枚；本轮逐枚单跑证明不能直接沿用「全量红都是干扰」的结论。

### 新增回归候选（基线绿 → 当前红，需单独立项）

`TestLoadWebP_LosslessPixels`、`TestP1_Capability_CS03_LinearBlendMidGPU`、`TestMiscDrawGPUPath/DrawShadow`、`TestPlaybackDrawImage`、`TestH4B3_GlesDeviceLostErrorScopeStubs`。

### 2026-11-09 晚些复核（acc=6 单行内填修复后）

- **单跑红名单不变**：上表 A 的 12 项仍单跑红；`TestRasterizerValueMatchesAnalytic/菱形` 仍红（此前一次定向 `-run` 通过是测试顺序假绿，已按单跑复验纠正）。
- 全量 `./render`（带库）18 个顶层红 = 上表 A 的 9 个 render 项 + 上表 B 的 8 个干扰项 + `TestS69_HeavyBudget_TierGates`（单跑 PASS，归全量干扰，K6 旧清单也把它列为全量红）。
- acc=6 修复只改 `accuracy>2` 的短陡边路径；acc=2 虚线/实线圆 SHA256 与修复前一致，showcase 金图 `diff=0.0000%`。

## 2026-11-09 修复批次 1（acc=6 单行内填收口后）

按用户顺序「先 acc=6 → 记录单跑红 → 再修单跑红」推进。本批修掉 **5 个单跑红**，新增 **3 个钉测**；全部不动 acc=2 生产像素（showcase 真窗金图 `diff=0.0000%`）。

| 测试 | 真因 | 修法 | 验证 |
|---|---|---|---|
| `TestH4B3_GlesDeviceLostErrorScopeStubs` | H4-b3（9-28）共享钉测假定 GL `IsLost()` 恒 false；J5 campaign 后 Linux 端按 `lost \|\| ctx==nil` 报告（让 `ensureGPU` 重建已释放设备），Windows 端仍恒 false，`&Device{}` 零值 `ctx==nil` 于是报 lost | 共享测试去掉平台相关硬断言；新增 `device_lost_linux_test.go` 平台专有钉测 pin 存活/零值/lost 三态 | linux gles 单跑绿；`GOOS=windows go test -c` 编译 rc=0 |
| `TestVideoDrawFrameMarksDamage` / `TestVideoBridgeFallbackConverged` / `TestVideoBridgePlanesFallback` | `DrawImageEx` 的 GPU textured-quad 快路径（`tryGPUDrawImage` 成功）在 `Fill()` 前直接 `return`，绕过 `Fill()` 里的 `trackDamage(path.Bounds())`；带 GPU 会话时 `FrameDamage()` 为空，增量 present 会漏掉整块画面 | GPU 分支补 CTM 映射四角 AABB 的 `trackDamage`；新增 `video_frame_damage_test.go`（带/不带 WGPU 库都验 damage 矩形 + 读回像素真值 + 区域外空白，防「只补标记不真画」的假绿） | 三测带库单跑绿；带库全量 `./render` 顶层红 **18→15**、无新增红；GPU/CPU 两路读回像素均为 220,30,30,255 |
| `TestPlaybackDrawImage` | bbf04c4b 把默认采样从「`applyDeviceSpaceBrush` 丢模式→恒 nearest」修成真 bilinear；8×8 四象限图放大 4× 时采样点 (26,14) 落在源 x≈3.625，bilinear 混出 (95,159,0) 而非纯绿 | 测试显式 `InterpNearest` 锁定它原本验的纯色象限语义；新增 `TestPlaybackDrawImageBilinear` 钉住边界混合值 ≈(95,159,0) 与象限中心纯红 | `./render/recording/backends/raster` 整包绿；两用例双绿 |

回归：`./render/internal/image`、`./render/internal/raster`、`./render/svg`、`./render/recording/backends/raster` 全绿；`./gpu/gwgpu/gles` 仅剩已知环境红 `TestPresentDamageX11`（G3，X11/NVIDIA partial present 未启用）；`go vet ./render/...`、apidoc rc=0；showcase 金图 0.0000%。

### 剩余单跑红

- **`TestLoadWebP_LosslessPixels`：已修（用户拍板「真修洞 + 重种金图」，见下「修复批次 2」；以下为当时的定位记录）。** 真因是 52859bc5 把 `FromStdImage` 改成 `FormatRGBAPremul` 后，`*image.NRGBA` fast path 仍把 straight 字节原样 copy 进 premul buffer（缺 premultiply），透明像素携带隐藏 RGB；WebP 与 PNG 两个数据源在 alpha=0 处隐藏 RGB 不同（0,0,0 vs 240,200,40），被 `ToStdImage()` 暴露为 max=240 / 1137 样本差异。实验证明修 NRGBA premultiply 可让像素对比 **0/0**、alpha 直方图精确 **379/64/5701**，但会让 2 个原本绿的 `io_test.go` 契约过期、showcase 金图 diff **0.0023%**（需重种）——属「图片管线 premul 语义」跨线大改 + 共享状态改动，按「不动绿代码行为 / 不交付半吊子」已回退。
- CS03 见下「修复批次 3」。其余待下一批：`TestMiscDrawGPUPath` 两子用例（advanced blend / DrawShadow 1 LSB）、`TestTextTransformAutoRoutesOutlines`、`TestPresentDamage_ImageMatchesView`、`TestRasterizerValueMatchesAnalytic/菱形`；环境红 `TestPresentDamageX11`（X11/NVIDIA partial present 未启用，需隔离真 Xorg 复核，非代码红）。


## 2026-11-09 修复批次 2（WebP 真修洞 + 金图重种，用户拍板）

- 方向（用户拍板）：真修 `FromStdImage` 的 NRGBA 漏预乘 + 同步过期契约 + 重种金图。
- 修法：`render/internal/image/io.go` 的 `FromStdImage` 删掉 `*image.NRGBA` fast path——它把 **straight** 的 `nrgba.Pix` 原样 copy 进声明为 `FormatRGBAPremul` 的 buffer，违反 premul 不变量（RGB≤A）：透明像素携带隐藏 RGB、部分透明像素偏亮。改为让 NRGBA 落到 generic path 的 `At().RGBA()`（stdlib 自带 premultiply）。这是 52859bc5 修 `*image.RGBA` 与 generic 两条路径时漏掉的第三条。
- 契约同步 3 处（都是「引擎格式契约变了、测试没跟上」，不是放宽断言）：
  - `webp_contract_test.go`：`Format()` 期望 `RGBA8` → `RGBAPremul`
  - `io_test.go:TestFromStdImage_NRGBA`：`(128,64,32,200)` → premul `(100,50,25,200)`
  - `io_test.go:TestSavePNG_LoadPNG`：`(255,128,64,200)` → premul `(200,100,50,200)`
- 验证：`TestLoadWebP_LosslessPixels` 完整绿（像素逐位 **0/0**、alpha 直方图精确 **379/64/5701**）；`./render/internal/image` 全绿；带库全量 `./render` 顶层红 **15→14**、无新增红。
- **金图重种（像素位移真实，非 PNG 编码噪音）**：NRGBA 修复让半透明图片从「straight 被当 premul 用」（偏冷绿）变成正确 premul（偏暖橙）。逐窗对账只有 `ui_wr_p38_svg_showcase`（0.0023%）与 `ui_wr_p38_svg_structure`（0.0039%）两窗的 base 过期且 diff 稳定复现 4 次（排除 GPU 非确定性）；两窗删 base 重种后复验 **golden 0.0000%**，探针 logic 6/6 + pixel 20/20（structure）、logic 4/4 + pixel 11/11（showcase）。HEAD base vs 新 base 可见像素差 **75 / 355 px**。其余 6 扇 + color_font 双后端 + picture 双后端像素逐位不变，运行时 PNG 字节差异已全部还原。
- **判据教训（客观判据不可短路）**：第一轮用 PIL `ImageChops.difference(...).getbbox()` 判「相同」被短路——RGBA 差值图的 alpha 通道恒 0，`getbbox()` 返回 None，于是误判「重种没改像素」并把正确的 base 回滚了。改用逐像素 `any(c != 0 for c in p)` 计数 + Go `image.Decode/At().RGBA()` 复现（459 个可见像素差）才定位到真实变化。**结论：像素判据不能用 getbbox 短路，必须逐像素数；mean / 近似同样禁用。**


## 2026-11-09 修复批次 3（CS03 渐变契约归位）

- 真因：16ad6895（批 4c-4）把渐变 stop 插值从 linear-light 改成 **premultiplied sRGB**（SVG `color-interpolation` 缺省 + tiny-skia `PremultipliedColorU8` / Skia 同口径），提交说明明确把旧「linear 0.735≈187」判为**误读**，并同步重录了 `gradient_test.go`（0.735→0.5）、COLRv1（188→128）、conformance `solid_gradient.json`、GPU WGSL ramp。`TestP1_Capability_CS03_LinearBlendMidGPU` 是 P1 能力矩阵里**漏同步**的最后一个：仍断言中点 `avg>=150`（linear 187），实测 sRGB 中点 131。
- 修法：把断言改成 sRGB 中点窄带 `118<=avg<=145`，并明确注释「≈187 代表插值退回 linear-light」。带比旧的单边界 `>=150` **更窄更严**，不是放宽；端点 dark/light 极性断言保留。
- 验证：带库单跑绿（mid=131、端点 9/248）；带库全量 `./render` 顶层红 14→12、无新增红。不带库时 `requireNativeGPU` 的既有 FAIL 与本改动无关（有效门禁带库）。
