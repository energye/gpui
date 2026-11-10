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

## 4 补量测执行方案（2026-11-10 新会话按 §0 重建镜像后，待拍板执行）

> 口径：用户拍板「继续 P4.3 两项、先量后改、量完再停一次拍板要不要改代码、本轮不改现网行为」。
> 本节只描述怎么量；真正造示例窗 + 跑 GPU 真窗前按 AGENTS.md 停下报告确认方向。
> 零引擎改动、零公开 API、既有正确性金图不重种。

### 4.1 backdrop 损伤区

- 现状（只读复核）：`render/context_layer_stack.go:467` 的 backdrop 层 Pop 走
  `drawGPUTextureWithOpacityFullSurface`（`render/context_image_gpu.go:298`），整层 blit；
  注释自陈「present 尾段重放，整层贴回才恢复 surface」，并引 Skia
  `cropFilterOutputToLayerBounds` 说明界外像素保持与父层相等 ⇒ 整层贴回像素正确，但界外也重画。
- 首轮缺口（`measure/results.md` §2）：两扇现有窗都满刷（`damage_area_px 960000` = 整窗），
  `frame_flushes` backdrop 11 vs maskops 3，**没有局部 backdrop 场景**，判不了增量。
- 要补的量：新增独立测量窗 `examples/ui_wr_p43_backdrop_damage`（**不动**
  `ui_render_base_backdrop` 及其金图），满窗内容 = 只占窗口中央 ~1/4 的局部 backdrop 层 +
  静止背景；指标浮画面左上角。采 JSON：`damage_area_px`、`frame_flushes`、
  `frame_raster_ms`、device-surface / filter-graph 两池 hits/misses。
- 判定线（客观计数，不用 mean 判好转）：若局部场景下 `damage_area_px` 仍等于整窗面积、
  且 `frame_flushes` 与满刷窗同量级 ⇒ 证明 present 对 backdrop 也整窗重放、损伤区裁剪
  无从下手，登记「不可优化」而非硬改；若 damage 收敛到局部、flush 下降 ⇒ 才值得进入
  「改」的影响面评估与拍板。
- 双后端：native 主测；go 档（`GPUI_BACKEND=go`）补测并登记差异；metal 本机不可验。

### 4.2 抖动零回读影响

- 现状（只读复核）：`render/m4_dither.go:34` 的 `applyDitherIfEnabled` 对整张 pixmap
  逐像素加 Bayer 4×4 噪声并 `NotifyPixelsChanged()`；调用点在
  `render/context_gpu.go:187`（FlushGPU）、`render/filter_ops_gpu.go:352,371`、
  `render/m4_quadimage.go:379`——都在 nil-view flush 回读进 pixmap 之后。首轮
  （`results.md` §3）headless 实测开/关 6400/9216 px 变、最大差 2 级，幅度受控。
- 要补的量：沿用现成 `examples/ui_render_base_dither`（自带 off/on 四卡 +
  `dither_gpu_ops` / `dither_repeat_readback_limit` 探针，README 有 `-auto-only` 跑法），
  开/关各一轮真窗，采 `frame_flushes`、读回计数、`gpu_ops`、`cpu_fallback_ops`、
  `dither_scope`；对照两次快照逐像素差（应只在渐变带 ±1~2 级）。
- 判定线：开 dither 若 `frame_flushes` / 读回计数相对关档上升、或出现 CPU fallback
  ⇒ 破坏零回读 GPU 路，登记并评估「present 期一次性施加」；若两档计数一致
  ⇒ 零回读不受影响，抖动维持现状、本项关闭。

### 4.3 GPU 真窗安全与影响面

- 隔离：当前桌面是 Wayland + rootless Xwayland :0，窗自带 x11grab screen check 在
  rootless 抓黑图是 P1.4 C1 在册环境问题 ⇒ 量测走隔离 :3（headless gnome-shell +
  rootless Xwayland，脚本 `evidence/P2.6/tools/start-xwayland.sh`，P1.4/P1.6/P2.6 同款）。
- 跑法：一轮一窗、串行、`RUN_SECONDS` 脚本跑 + `-auto-only`，应用走 `Run → Close`
  标准关闭（WM_DELETE → Quit → app.Close → win.Close），脚本等进程自己退出，不硬杀、
  不 pkill / killall；每轮前 `xdpyinfo :3` 健康检查，发现 `nvidia_drv.so` / Xorg 段错误 /
  `Invalid MIT-MAGIC-COOKIE-1 key` 立即停批报告。
- 影响面：只新增 1 扇示例测量窗 + 跑真窗；引擎层、公开 API、catalog/apidoc、
  既有正确性金图全部零改动。量测数据落 `evidence/P4.3/measure/`。
- 停下报告：本节方案（造哪扇窗 / 采哪些字段 / 判定线 / 隔离跑法）经用户确认后才执行；
  量测结果出来后再停一次，拍板要不要改引擎代码。

## 5 实施中发现（2026-11-10 补量测执行前，客观复核）

### 5.1 隔离环境与标准关闭验证（安全前置，已过）

`evidence/P2.6/tools/start-xwayland.sh start` → `wayland gpui-p26: alive` / `X display :3: alive`
/ `check OK` / `DISPLAY=:3 xdpyinfo` 正常。用最简窗 `examples/ui_l1_blank` 做标准关闭验证
（`env -u WAYLAND_DISPLAY DISPLAY=:3 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=3 go run ./examples/ui_l1_blank`）：`backend=x11 presents=166 ~54.8fps wall 3.0s`、
`gpu_fallbacks=0`、进程自行退出（定时 Run→Close，无 kill/pkill）；跑前 `PRE_OK`、跑后 `POST_OK`。
⇒ GPU 安全条的「隔离环境标准关闭验证」前置通过，具备跑单窗测量的条件。

### 5.2 载体核对发现 ①：方案 §4.2 的 dither 载体不成立（必须修正）

`examples/ui_render_base_dither` 的 off/on **不是整窗开关**：`scene.go:137` 的
`bakeImage(kind, dither, w, h)` 在**离屏** `render.NewContext` 上 `SetDither(true)` 后
`dc.Image()` 烤成贴图，窗的 `OnPaint` 只是把 12 张 baked 图贴上去，主绘制从不调
`SetDither`。窗的 `dither_scope` 字段自述「pixmap postprocess; **not the
FlushGPUWithView window-present path**」（`main.go:316`）。
⇒ 照 §4.2「现成窗开/关各一轮，比 `frame_flushes`」量到的两轮都是「贴 12 张 bake 图」
的 flush，与「抖动是否逼 GPU 回读」无关，**是假量测**。要量真问题，需要一个在窗主
Context 每帧 `SetDither` 的载体（现成窗没有，须另造或加门控）。

### 5.3 载体核对发现 ②：backdrop 整层 blit 是刻意的正确性设计，不是「没优化」

`render/context_layer_stack.go:467-480` 注释自陈：「windowed present replays only the tail
once a mid-frame materialize happened, so the whole-layer blit is what restores the surface」，
并引 Skia `cropFilterOutputToLayerBounds` 说明界外像素保持与父层相等。P4.3 阴影专项两次改
层合成路径都以 **33% 像素缺失 / 全回退** 收场（`impl-attempt.md` / `impl-attempt2.md`）。
⇒ 该项量测的高概率结论是「不可优化」，量测必须能证到「改成局部 damage 会丢内容」，
而不是先假设能优化再凑一个好看的数字。

### 5.4 处置

按「动手修门槛（硬）」与「客观判据、禁 mean 糊弄」纪律：**不硬跑假量测**，§4.2 载体
需调整（造一扇零金图、零 screen-check 的专用量测窗，或用离屏真 GPU 探针），
§4.1 判定线保持「证伪即登记不优化」分支。停下报告待用户拍板载体方向。
本发现段为零代码、零金图、零公开 API 改动。

## 状态（更新 2026-11-10）

- 首轮量测已落 `measure/results.md`（零代码）：mask 口径需计数、backdrop 需局部场景、
  dither 零回读影响待补。
- mask 计数 + 按尺寸池化已落地并收口 ✅（台账 §7 两行）。
- 隔离 :3 + 标准关闭验证已过（§5.1）；补量测载体经复核发现 §4.2 不成立（§5.2）、
  §4.1 高概率证伪（§5.3）。
- **二轮量测已跑完**（用户拍板「造专用量测窗」后）：backdrop 新造
  `examples/ui_wr_p43_measure`（A/B 对照）、dither 改离屏真 GPU 探针
  `render/p43_dither_readback_test.go`；数据、A/B 表、诚实判读见
  `measure/results-round2.md`。dither 客观判不修；backdrop 量到净成本但裁剪可行性
  因满刷策略判不了、建议不优化。**停在「停下报告」拍板要不要改引擎（或补静止帧
  二轮）**；代码、金图、公开 API 全部零改动。
