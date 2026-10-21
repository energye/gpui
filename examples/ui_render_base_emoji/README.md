# ui_render_base_emoji — B 组 Emoji 直调真窗

`render/text/emoji` 的直调真窗（原子验证）：CBDT/CBLC 位图提取、`Segment` 分段
（混合文本 / ZWJ / 国旗 / 肤色 / keycap / VS16）、`RenderCOLRToImage` 图层合成，
以及提取出的 PNG 经 `render.ImageBuf` / `DrawImageEx` 真正画上 GPU 画面。

对应总账：
- `docs/RENDER_FEATURE_COVERAGE.md` §3.2 B 组点名项「Emoji」；
- §2 子包矩阵 `render/text/emoji` 行；
- 离屏提取 demo 是 `examples/render_color_emoji`（只写 PNG，不算绘制栈真窗）。

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 自动门禁：5 秒、JSON 到 stdout、失败退出 1
RUN_SECONDS=5 go run ./examples/ui_render_base_emoji -auto-only

# 人工窗：常驻，真实事件写 stderr 并改标题，关窗后出汇总 + JSON
go run ./examples/ui_render_base_emoji

# 人工自动关：15 秒
go run ./examples/ui_render_base_emoji -manual-seconds 15
```

首次建立 Golden（本机字体固定时）：

```bash
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_render_base_emoji -auto-only
# 复查必须 zero diff
RUN_SECONDS=5 go run ./examples/ui_render_base_emoji -auto-only
```

## 四张卡

| 卡 | 内容 | 证据 |
|---|---|---|
| A | 6×6 CBDT 位图网格，36 个字形，PPEM 109 | 逻辑 `cbdt_grid`；像素 `A_grid_ink`（ink 0.408 / chroma 9423）与 `A_grid_bg`（格缝卡底） |
| B | 最彩色字形（本机 gid=21）按原始 136×128 尺寸画出，旁列 glyph / PPEM / 尺寸 / PNG 字节数 / 管线 | 逻辑 `cbdt_glyph`；像素 `B_sample_ink`（ink 0.824 / chroma 14352）与 `B_sample_bg` |
| C | `testdata/emoji.txt` 六行，每行下方按 `emoji.Segment` 的 run 画一条框（蓝=文本、橙=emoji） | 逻辑 `segment_*` 9 项（混合/ZWJ/国旗/肤色/keycap/VS16/字节偏移/回环/无空 run）；像素 4 个 run 框颜色 |
| D | 合成 COLR 三层：红方块 → 绿圆 → 蓝三角，走真实 `emoji.RenderCOLRToImage` | 逻辑 `colr_render` / `colr_foreground`；像素红/绿/蓝/卡底四点 |

总计：**14 逻辑探针 + 12 像素探针 + Golden 627200px 逐位对比**。
`-auto-only` 实测（2026-10-21）：

```
logic=14/14 pixel=12/12 golden=0.0000% skip=0 presents=300
font=/usr/share/fonts/truetype/noto/NotoColorEmoji.ttf
font_sha256_16=0f03293ecdd10b89
cbdt_ppem=109 cbdt_glyphs=36 cbdt_sample_gid=21 colr_layers=3
present_policy=full_paint vsync_source=true gpu_ops≈36k cpu_fallback_ops=0
```

## 证据口径

- **像素断言的 oracle 是绘制源常量与同一份 assets**：卡底色、run 框颜色、COLR
  三层颜色都由场景和探针共用同一组常量；A/B 位图不锁逐像素（字体版本相关），
  用「相对卡底的 ink 比例 + 色度像素数」断言，Golden 再锁布局与合成图。
- **Golden 区域**：表头 1200×48 + 图例面板 + 四张卡（430×232 ×4），合计
  627200 px。播种后复跑 `diff=0.0000%`。
- **字体硬前提**：A/B 卡需要系统 CBDT/CBLC 彩色字体。本机为
  `/usr/share/fonts/truetype/noto/NotoColorEmoji.ttf`，SHA-256 前缀
  `0f03293ecdd10b89`。字体缺失时 A/B 逻辑探针入 `logic_skipped`、像素探针入
  `pixel_skipped`、Golden 入 `pixel_golden_skipped`，窗口不静默标绿。
- 字体缺失时 C/D 仍完整跑（8 个字体无关像素探针），报告 `font_missing=true`。

## 已知限界（不冒充完成）

1. **只烘 CBDT/CBLC**：TTC（Apple Color Emoji）和纯 COLR 字体不烘；本窗在 Linux
   NotoColorEmoji.ttf 上验证。D 卡 COLR 是**合成 glyph**，只证明
   `RenderCOLRToImage` 的图层顺序 / 调色板 / 前景层路径，不证明某个真实 COLR 字体。
2. **肤色修饰的 `Parse` 仍用简化基表**：`Segment("👍🏽")` 正确保持一个 run（本窗
   逻辑探针覆盖），但 `emoji.Parse("👍🏽")` 目前返回两个 `SequenceSimple`，
   `IsEmojiModifierBase(👍)` 为 false（`render/text/emoji/detect_test.go` 已把它
   标为 “Not in our simplified list”）。所以 C 卡只认证**分段**，不认证修饰序列
   解析；补齐完整 Unicode Emoji_Modifier_Base 表是后续独立子项。
3. **不是文本彩色字形全链路**：本窗把提取出的 PNG 直接走 `ImageBuf` +
   `DrawImageEx`，不走 `DrawString` 的 CBDT/COLR 字形分支；文本栈彩色字形由
   `ui_text_m5_anycase`（间接）与 `ENGINE_TEXT_SCALE_PLAN.md` M5 负责。C 卡文本
   里系统 UI 字体没有的 emoji 会退化成单色/字母（如国旗显示 CN/US），这是
   fallback 的真实呈现；**分段证据看下方 run 框，不看文本字形**。
4. **性质 = 原子验证**：直调 `render/text/emoji` + `render` 图像绘制一次验一组
   能力，没有组件/2.5D/视频生产调用方，不标生产接线。
5. 首帧 `frame_raster_ms≈128`、`rss_slope` 高是 36 张位图 + 每帧完整重绘的当前
   观测；本窗是正确性窗（`fps_gate=off_correctness`），性能债在性能阶段处理。

## 文件

- `main.go`：窗口 / 门禁 / 像素探针 / Golden / JSON。
- `scene.go`：字体 CBDT 提取、分段语料、合成 COLR、四卡绘制、逻辑探针。
- `testdata/emoji.txt`：六行混合语料（注释行跳过）。
- `testdata/showcase_emoji_base.png`：Golden 基线（字体 SHA 绑定）。
- `testdata/showcase_emoji.png`：本轮快照，每跑重写。
