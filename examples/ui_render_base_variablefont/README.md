# ui_render_base_variablefont — B 组 VF 变体直调真窗

可变字体（VF）的独立直调真窗（原子验证）：`text.FontSource.IsVariable` /
`VariationAxes` / `NamedInstances` / `Face` + `text.WithVariations`，以及
`Context.LoadFontFaceWithVariations` / `Context.FontVariationAxes`。

对应总账：

- `docs/RENDER_FEATURE_COVERAGE.md` §1 #31「VF 变体」、§3.2 B 组点名项；
- `docs/ENGINE_UI_RENDER_BASE.md` §23 窗口表；
- 旧离屏示例 `examples/render_variable_font` 只出 PNG，不算真窗。

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 自动门禁：5 秒、JSON 到 stdout、失败退出 1
RUN_SECONDS=5 go run ./examples/ui_render_base_variablefont -auto-only

# 人工常驻：开窗直到手动关闭（事件写 stderr 并改标题）
go run ./examples/ui_render_base_variablefont

# 人工 + 15 秒自动收尾
go run ./examples/ui_render_base_variablefont -manual-seconds 15
```

首次建立 Golden（只在基图不存在时生效；播种后必须复跑确认 `0.0000%`）：

```bash
GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=5 go run ./examples/ui_render_base_variablefont -auto-only
RUN_SECONDS=5 go run ./examples/ui_render_base_variablefont -auto-only
```

## 画面（1200×800，2×2 四卡）

| 卡 | 内容 | 证据 |
|---|---|---|
| A | `wght` 200 / 400 / 600 / 900 四行同字符串 `Variable`，字号 34 | 逻辑 `face_variations_exact` / `advance_monotonic`；像素 4 点；墨量 `A_wght200` < `400` < `600` < `900` |
| B | fvar 命名实例 ExtraLight / Regular / Semibold / Black，字号 30 | 逻辑 `named_instances_present` / `instance_values_in_range`；像素 4 点；墨量逐档递增 |
| C | 默认实例、显式写默认值、Black 三行对照 | 逻辑 `default_matches_axis_default` / `empty_variations_matches_default`；像素 4 点；C_default 与 C_explicit 墨量相等 |
| D | 轴最小/最大、越界钳位、默认值、未知轴 `wdth` 六行 | 逻辑 `clamp_low_matches_min` / `clamp_high_matches_max` / `unknown_axis_ignored`；像素 5 点；越界行与边界行墨量相等 |

源字体 `testdata/SourceSans3VF-Upright.otf`（SHA-256 前缀
`3d0dfd6a3a644ab3`）只有 `wght` 200–900（默认 200）和 7 个命名实例；
`wdth 75` 是不存在的轴，按 API 契约忽略。

## 证据口径

- **逻辑 18/18**：字体存在、`IsVariable`、`wght` 范围、默认值在范围内、命名实例
  存在且值在范围内、`LoadFontFaceWithVariations` 设置轴、无变量时保留默认轴、
  `Face.Variations()` 精确回读、`Face.Source()` 身份、`Face.Advance` 随权重单调、
  默认 face 等于显式默认、空变体等于默认、低/高越界钳位、未知轴忽略。
- **像素 4/4**：四卡背景点（`meta` / `named` / `independence` / `bounds`），
  容差 8/255；用来确认快照确实包含完整卡片，不是只读逻辑。
- **墨量 17/17**（区域探针，只数亮墨而不依赖文字位置）：A 四档严格递增；
  B 四档严格递增；C_default 与 C_explicit 相等（容差 2px）、C_black 重于默认；
  D_min 与 D_clamp_low 相等、D_max 与 D_clamp_high 相等、D_default 与
  D_wdth 等于最小值档。
- **Golden**：表头 1200×48 + 图例 + 四卡（430×232 ×4），合计 **627200 px** 逐位
  零容差；播种后复跑 `diff=0.0000%`。指标 HUD 不参与 Golden。
- `-auto-only` 实测（2026-10-21）：`logic=18/18 point=4/4 ink=17/17
  golden=0.0000%`，`presents=300`，`fps_interval=60.03`，
  `interval_p95=17.31ms`，`hitch=0`，`gpu_ops=17703`，`cpu_fallback_ops=0`，
  `gpu_fallbacks=0`，`present_policy=full_paint`，`vsync_source=true`。
- 人工档 `-manual-seconds 15` 退出 0：`presents=899`，逻辑/像素/墨量/Golden
  同绿，`fps_interval=59.96`，`interval_p95=17.19ms`，`hitch=0`，
  `gpu_ops=53044`，`cpu_fallback_ops=0`。

## 同批修的引擎缺陷

**VF 变体没有到达 GPU 文本管线，四档权重画出来完全一样。** 这是先建窗取证后
发现的真问题，不是窗内绕过：

- MSDF 与 glyph-mask 的字体 ID 都只由 `FontSource` 计算，同一 VF 的不同权重
  共用 atlas key；glyph-mask 光栅化只调无变量的 `RasterizeHinted` /
  `RasterizeAliased` / `RasterizeLCD`，永远取默认 master。
- glyph-mask 的 pen 推进在 hinting 路径用 `GlyphOutline.Advance`；该值来自静态
  hmtx，不含 HVAR，所以即使轮廓按权重变了，整行宽度仍相同。
- HarfBuzz shaper 的缓存只按 `FontSource`，`Shape` 不设置实例坐标，`TextPath`
  和复杂整形路径的位置/advance 也停在默认 master。
- 轮廓缓存 key 只按 `FontSource` + 字号，不同权重的 `TextPath` 会命中同一个
  缓存轮廓。

修法（都按「变体是字体实例的一部分」处理，静态字体路径不变）：

1. `render/internal/gpu/gpu_text.go`：MSDF font ID 用
   `variationFontID(base, variations)` 组合变体哈希；MSDF 轮廓改用
   `ExtractOutlineHintedVar`。
2. `render/internal/gpu/glyph_mask_engine.go`：glyph-mask font ID 同样组合变体
   哈希；`glyphMaskParams` 携带 `face.Variations()`；新增
   `rasterizeVariationGlyph`，先按实例坐标抽轮廓，再复用现有
   `RasterizeOutline` / `RasterizeOutlineAliased` / `RasterizeLCDOutline`；
   hinting 路径的 pen 改为使用 shaped X 差（HVAR 已折入），不再用静态
   `GlyphOutline.Advance`。
3. `render/text/shaper_hb.go`：HarfBuzz face 缓存键改为
   `(FontSource, variation hash)`，每个实例建私有 `font.Face` 并
   `SetVariations`，避免不同权重共用默认坐标或并发改写同一 face。
4. `render/text_outline.go`：`TextPath` 的缓存 font ID 组合变体哈希，轮廓提取走
   `ExtractOutlineHintedVar`；`render/text_dispatch.go` 的 CPU 轮廓回退同样按
   face 变体提取。

永久回归：

- `render/internal/gpu/varfont_atlas_test.go`：glyph-mask 与 MSDF 的
  `wght 200` vs `900` 行宽/quad 必须不同；`variationFontID` 区分权重且空变体
  保持基 ID。
- `render/text/varfont_shape_test.go`：HarfBuzz `Shape` 的 advance 随权重变化。
- `render/text_variable_path_test.go`：`TextPath` 的 `wght 900` 比 `200` 宽，
  且缓存不串权重。

上述测试修前红、修后绿；`go test ./render/internal/gpu -count=1` 全绿，
`go test ./render -run 'TestTextPathVariable...'` 绿。

## 已知限界（不冒充完成）

1. **资源指标未绿**：本窗 5 秒自动档 `rss_start≈78.4MB → rss_end≈255.2MB`，
   `rss_slope≈528,846 KB/min`；15 秒人工档 `rss_end≈274.7MB`，
   `rss_slope≈136,940 KB/min`，关窗后 `rss_after_close≈255MB`。功能/像素/
   Golden 虽绿，但不能当资源绿；显存/堆快照与池化账本留 §7/§8 资源门禁处理。
   同批 Pattern 窗也有同量级现象，先记录为跨窗资源债，不在本窗用降画质掩盖。
2. **没有抓屏 vs 快照断言**：本窗三证据为逻辑探针 + GPU 快照像素/墨量 + Golden；
   `maskops/backdrop/cliprrect` 那类窗口额外做了 `xwd` 抓屏对比，本窗暂未接。
3. **仍是原子验证**：`LoadFontFaceWithVariations` / `WithVariations` 目前没有
   组件库、2.5D 或视频模块的生产消费者；本窗证明渲染链路正确，不标生产接线。
4. **只覆盖 `wght` + 未知轴忽略**：源字体没有 `wdth` 等其它轴，所以 D 卡的
   `wdth 75` 是「未知轴按契约忽略」证据，不是宽度轴真实变化的证据；其它轴
   （如 `opsz`、`slnt`）未在本窗覆盖。
5. **向量/复杂整形只钉了基础契约**：HarfBuzz 实例 face 和 `TextPath` 已接入变体，
   但本窗的绘制主路仍走 glyph-mask；阿拉伯/印度系等复杂脚本的 VF 变体没有
   独立真窗，不能把本窗结论外推到全部脚本。

## 文件

- `main.go`：窗口 / 门禁 / 背景点探针 / 墨量区域探针 / Golden / JSON / 手动事件。
- `scene.go`：字体加载、轴/实例读取、逻辑探针、四卡绘制、墨量 oracle。
- `testdata/SourceSans3VF-Upright.otf`：Source Sans 3 VF 字体（SHA-256 前缀
  `3d0dfd6a3a644ab3`）。
- `testdata/showcase_variablefont_base.png`：Golden 基线。
- `testdata/showcase_variablefont.png`：本轮快照，每跑重写。
