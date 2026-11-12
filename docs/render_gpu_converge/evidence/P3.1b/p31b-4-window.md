# P3.1b-4 真窗 `ui_wr_p31b_srgb_chain`（2026-11-12）

## 0 运行方式

```sh
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_p31b_srgb_chain -auto-only
GPUI_BACKEND=go RUN_SECONDS=5 go run ./examples/ui_wr_p31b_srgb_chain -auto-only
```

窗口：1200×800，左卡默认 `RGBA8Unorm` 目标，右卡 opt-in `RGBA8UnormSrgb`
目标；两卡同内容（13 个固定功能模式 + 50% 灰半透明底条），均以
`DrawGPUTexture` 合成到默认窗口目标；P3.1b-3 的边界转换保证两卡按各自目标
约定显示。金图容差 0，按后端分基线（`p31b_final.png` / `p31b_final_go.png`）。
本次运行显示后端为 Wayland（`backend=wayland`），native 指 webgpu-native，
go 指 gwgpu-gles。go 腿依赖 gles 在 sRGB 附件上开 `GL_FRAMEBUFFER_SRGB`
（同批 GL 状态改动：BeginRenderPass 按目标格式开关，present blit 前关闭）。

## 1 双后端结果

| 后端 | presents | 像素探针 | 金图 diff | 默认卡降级计数 | 线性卡降级计数 | 卡 GPU/CPU 回退 |
|---|---|---|---|---|---|---|
| native webgpu | 359 | 7/7 | 0.0000% / 821184 px | 14 | 0 | 29 / 0 |
| go gles | 357 | 7/7 | 0.0000% / 821184 px | 14 | 0 | 29 / 0 |

- 默认卡 14 = 13 个固定功能模式 + 半透明底条各 1 次降级；线性卡 0。
- 像素探针：Normal（默认 166 / 线性 171）、Plus（255 / 219）、底条（89 / 99）
  各两卡，Copy 两卡等值控制；共 7 项。
- 13 模式全表另由 `TestP31bLinearTargetAllFixedBlendModes` 逐模式钉住（sRGB
  目标线性参考 + Unorm 降级计数）；真窗负责整窗画面、合成边界和 13 模式
  同屏可见性。
- 其余能力钉：`TestP31bSrgbTargetGradientText`（渐变 + Ahem 全墨字形内部与
  Unorm 控制逐值一致）、`TestP31bAdvancedBlendSrgbTarget`（Screen 线性 138，
  Unorm/sRGB 双目标一致）、`TestP31bContextSrgbCPUFallbackPolicy`（线性目标
  GPU-only，无 GPU 时建目标显式失败，不交空目标）。原始日志：
  `p31b-2b-tests.txt`。
- GL 腿：`BeginRenderPass` 按颜色附件格式开关 `GL_FRAMEBUFFER_SRGB`，present
  前的 swapchain→FBO0 blit 显式关闭；负向见上表。Windows/linux-arm64 交叉
  编译、darwin 交叉编译、`-tags nogpu`、`go vet`、双文件 `-race` 均过。
- 原始 JSON / stderr：`window/native.json`、`window/native.stderr.txt`、
  `window/go.json`、`window/go.stderr.txt`。

## 2 负向对照（红了再还原）

| 破坏 | 预期 | 实测 |
|---|---|---|
| 线性卡强制 `RGBA8Unorm` | 线性卡降级计数变 14、Normal 像素 166≠171、金图红 | 红：`card_linear_linear_fallbacks=14`、`card1_normal got=166 want=171`、golden 21.2337%（`negatives/force-unorm.*`） |
| 摘掉 opt-in 线性输出转换（`shaderSourceForTarget` 不再翻 `LINEAR_OUT`） | 线性卡画面偏色、金图红 | 红：`card1_normal got=211 want=171`、golden 30.1565%（`negatives/no-linear-output.*`） |
| GL 腿不启用 `GL_FRAMEBUFFER_SRGB`（sRGB FBO 写入按编码域） | 单填充读回线性值 55/13/3；真窗线性卡偏暗、金图红 | 红：GL 单测 `center=[55 13 3]`；go 真窗 golden 30.1565%、像素 0/7（`negatives/gl-no-framebuffer-srgb.txt`） |
| 摘掉合成源 `SourceSRGB` 标记（sRGB 源按编码字节直接采样） | sRGB→Unorm 合成偏暗、四组合单测红 | 红：`src=srgb dst=unorm center=[34 8 3]`（`negatives/no-source-srgb.txt`） |

四次破坏均已还原；还原后 native 复跑 `pixel=7/7 golden=0.0000%`，go 复跑同值。

## 3 默认路径零变化

`ui_render_base_miscdraw` / `ui_render_base_webp` 各 native/go 5s：

- miscdraw native/go：logic 14/14、pixel 17/17、golden 0.0000%
- webp native/go：logic 19/19、pixel 24/24、golden 0.0000%

原始摘要：`default-regression.txt`。

## 4 金图

- native：`examples/ui_wr_p31b_srgb_chain/testdata/goldens/p31b_final.png`
- go：`examples/ui_wr_p31b_srgb_chain/testdata/goldens/p31b_final_go.png`
- 对齐 GoldenGate 掩码：顶栏 48px + 图例面板 + Body 面板（含两卡与标签）。
