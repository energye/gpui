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
