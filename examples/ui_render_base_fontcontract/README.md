# ui_render_base_fontcontract

text Face/ParsedFont/FontResolver/ColorFont 契约真窗（O12 批 2，RENDER_FEATURE_COVERAGE §12 NONE→EX）。仓内 testdata 真字体（goregular.ttf）驱动全部契约口。

## 覆盖面（logic 36）

- **卡 A Face 契约**：`Size/Source/Direction/Language/Metrics/Advance/HasGlyph/Glyphs/AppendGlyphs` + `WithLanguage/WithFeatures/WithHinting/WithWeight` 缔造读回=设置值 + `RuneAdvance` 单 rune 快路径 + `Embolden` 真值登记。语义实证：Metrics.Descent 为正值约定（asc+desc=行高带内，18.5+4.6≈24）。
- **卡 B ParsedFont 内省**：`NumGlyphs=712/UnitsPerEm=2048/Name="Go"/FullName="Go Regular"/GlyphAdvance(gid,ppem)` 两参签名。
- **卡 C FontResolver**：`DefaultFontResolver/SetFontFile/SetPaths/Candidates/Load`——语义实证：`SetFontFile/SetPaths` 是**追加前置**（首元素=设置值，后随系统扫描候选，非替换）；`SystemFontsByFamily` 环境相关如实登记；`NewFontFeature` 4 字节 tag + `HashFontFeatures` 顺序无关 XOR、nil→0。
- **卡 D ColorFont 能力面**：`ColorFontInfo.PreferredColorFormat`（CBDT>sbix>COLR>SVG 优先级）/`HasAnyColorTable`；goregular 非彩色字体 `ColorFont` 接口断言失败属真值（登记不判死）。

## 语义记录（不是洞）

- Metrics.Descent 正值约定（与 Skia metrics 负 desc 惯例不同，行高=asc+desc）。
- Resolver 候选列表=设置项前置+系统扫描追加（Override 语义不是替换）。
- GlyphID 与 Face.Glyphs 迭代的 GID 同一编号空间（goregular 'A'=gid 36）。

## 门禁

```
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/ui_render_base_fontcontract -auto-only
go run ./examples/ui_render_base_fontcontract            # manual: 常驻
```

首播种 `GPUI_ACCEPT_GOLDEN=1`；卡内容改动需重播种。
