# P4.3 治根·滤波源帧内累积：修法与验收（2026-11-10）

> 性质：用户拍板 **A（修根因 + 重种金图）** 后的落地记录。根因定位见
> `root-cause2.md`。本批只修「持久滤波源 RT 把上一笔滤镜内容带进下一笔」这一
> 通用正确性问题；②③ 阴影缓存尚未重新落地，P4.3 仍 🟡。

## 0 修法

| 项 | 内容 |
|---|---|
| 根因 | `filterSrcView` 为 Context 级持久全画布滤波源；`FlushAndFilterFromView` 用 `rc.Flush(srcView)` 把新剪影写进去，session 因 `viewRendered(srcView)`（本帧已渲染过）选 `LoadOpLoad`，上一笔阴影剪影被保留 → 后续每笔滤镜的输出都含前面全部剪影，PopLayer 再各自原位重画（帧内叠加） |
| 标准 | Skia `saveLayer`/`SkImageFilter` 每次应用的源面都是隔离/清空的；CPU 腿每次从干净 layer pixmap 起算（`context_misc_draw_test.go:189` 的 25% spot） |
| 落地 | ① `GPURenderSession.forgetViewContent` 把 view 从本帧 `renderedViews`/`colorContentViews` 移除；② `GPURenderContext.ForgetViewContent` 再清 `seededViews` 对应项；③ `ensureFilterSrcRT` 在**每次**返回滤波源（复用/新建）时调用 `forgetFilterSourceContent`，下一次 flush 即 LoadOpClear；seed 非空时仍由 `SeedFromData` 上传并重新标记 |
| 边界 | 只清「滤镜源 scratch」，不动嵌套层/backdrop 的 LoadOp 语义；公开 API 零变化（render 层用既有 type-assert 口调用 internal/gpu 方法） |

## 1 先红后绿（新增常驻钉 `render/p43_shadow_filter_source_test.go`）

| 测试 | 实现前 | 实现后 |
|---|---|---|
| `TestDrawShadowRepeatedFrameDoesNotStack` 同帧两笔不同位置 DrawShadow，第一笔阴影带必须与「只画第一笔」对照逐位一致 | **FAIL** max diff 38（clean 209 vs repeated 171） | **PASS** max diff 0 |
| `TestLayerFilterSourceNotReusedAcrossDraws` 同一 Context 连续两次层内 ApplyBlur，第二次输出不得含第一笔剪影 | **FAIL** max diff 27（clean 224 vs repeated 197） | **PASS** max diff 0 |
| 负向 mutation：`forgetFilterSourceContent` 直接 return | 两测再次 **FAIL**（38/27） | 还原后 **PASS**，`git diff` 零残留 |

测试全部用「对照 Context 只画第一笔 vs 测试 Context 画两笔」的逐像素比较，不使用 mean；
两个场景都断言 `GPUOps>0 && CPUFallbackOps==0`（真走 GPU 图）。

## 2 真窗与重种（`examples/ui_wr_p41_drawshadow -auto-only`）

单窗、标准关闭、前后查显示健康；9/9 pixel probe 全过、`cpu_fallback_ops=0`。

| 运行 | 旧图 → 新图 | 二次运行 |
|---|---|---|
| native | golden diff **35.2519%**（重种前，预期）；旧→新整图 278666 px（29.03%），max 225 | **0.0000%** |
| `GPUI_BACKEND=go` | 同 35.2519%；旧→新 278677 px（29.03%），max 225 | **0.0000%** |

新画面 = CPU 契约的干净 25% 阴影：A 卡本体下缘 `191`（α=0.251；旧 `53`=α=0.792），
本体色 `(11,11,17)`（旧 `(3,3,7)` 被叠加阴影压暗）；C1/C2/D 对应点 192/189/191。
新 go 与 native 只在 HUD fps 文字区 `(239,741)-(397,775)` 差 173 px（门禁已遮蔽 HUD）。

old/new sha256 与逐图差异见 `golden-reseed/README.md`；旧图原样归档
`golden-reseed/p41_drawshadow_final_old_{native,go}.png`。

## 3 影响面 A/B（干净树 worktree @ `f799ce3a`，同命令同环境）

| 窗口 | 干净树 | 本批 | 结论 |
|---|---|---|---|
| `ui_wr_p41_drawshadow` | 0.0000%（旧金图） | 35.2519% → 重种后 0.0000% | **本批唯一有意重种** |
| `ui_render_base_miscdraw` | **12.5014%**（既有红） | **11.4955%**（既有红改善） | 差异只在阴影带 x171-604/y134-275 + HUD 329 px；阴影回到 25% 契约。既有红不因本批变绿/变红，未重种（超出 P4.3 金图范围），登记 |
| `ui_render_base_filterops` | 0.4216% | 0.4216% | 既有红，逐位同 |
| `ui_render_base_recording` | 2.5893% | 2.5893% | 既有红，逐位同 |
| `ui_render_base_backdrop` | 0.0000% | 0.0000% | 无变化 |
| `ui_wr_p34_filters` | 0.0000% | 0.0000% | 无变化 |
| `ui_wr_p38_svg_filter` | 0.0000% | 0.0000% | 无变化 |

## 4 门禁

| 门禁 | 结果 |
|---|---|
| `go build ./...` / `-tags nogpu ./...` | ✅ 双 rc=0 |
| `go vet ./render/...` | ✅ rc=0 |
| `go run ./scripts/apidoc` | ✅ rc=0（公开 API 零变化） |
| `go test -race ./render/internal/gpu` | ✅ PASS（35.1s） |
| `go test -race ./render -run` 新增/滤镜测试 | ✅ PASS |
| 相关 `render` 测试文件逐个（18 个：filter/layer/blur/shadow/seed/backdrop/quadimage） | ✅ 18/18 PASS |
| `go test ./render/recording` | 与基线逐条一致：3 条既有红（`TestCanvasBackendReplaysAtDeviceScale`、`TestP37E3CanvasBackendScopedShaderNoLeak`、`TestDrawShadowReplayParity` 716/32000），**零新增** |
| 真窗安全 | 单窗、标准关闭、每轮后 `xdpyinfo` 健康；无硬杀 |

## 5 边界与未做

- **②③ 阴影缓存未重新落地**：本批只修了参考画面的累积根因；原症状（330ms/帧≈3fps）
  仍在。下一步按 `p43-rewire-design.md` 重落地，键需包含 dpr/逻辑尺寸且命中路径要
  绕过 user+device 矩阵（设备空间 tile）——这会新增公开符号（`LookupShadowCache`、
  设备空间 `DrawTextureRef`、Recorder 取 scale），按「若走到要加接口先停下问」需拍板。
- 缓存键**不含 clip**：本批建议 tile 只按 surface 紧致裁剪、blit 时由现有 clip 裁，
  避免 clip 变化导致的错缓存；与上版「与 clip 相交」不同，需在落地时确认。
- 跨窗口同内容同尺寸同 CTM 会共享进程级 cache 键；resolver 只能保证 owner 设备视图，
  多窗口/多设备语义需在落地时用 context 维度隔离或登记限制。

## 6 状态

- 治根（滤波源帧内累积）**✅ 修复 + 双后端金图重种 + 0.0000% 二次验证**。
- ②③ 缓存待落地；**P4.3 仍 🟡**。
- 本批改动：`render/internal/gpu/render_session.go`、`gpu_render_context.go`、
  `render/filter_ops_gpu.go`、新增 `render/p43_shadow_filter_source_test.go`、
  drawshadow 双后端金图重种、TASKS 台账与证据。
