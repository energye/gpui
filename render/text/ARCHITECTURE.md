# render/text 架构对应关系

> 运行时零依赖：全部自研 Go 实现，不绑 libfreetype（无 cgo）。
> 系统 FreeType 2.11.1（`/usr/share/...` / ftexp）仅作**开发期像素对照度量衡**，
> 不进运行时。本目录即 `github.com/energye/gpui/render/text`。

## 1. 管线总览（四段）

```
 font 文件
   │
   ▼
① 解析 FontSource/Face          font_parser_own.go / glyf_parser.go / cff_outline.go
   字形轮廓源                     gvar.go avar.go hvar.go font_cmap.go face.go
   参考：OpenType 规范 / FreeType ttobjs.c
   │   ParsedFont（字体形态分派点）
   ▼
② shaping 排版                  shaper_own.go（默认）/ shaper_hb.go / shaper_builtin.go
   GSUB/GPOS/kerning             gsub.go gpos.go gpos_mark.go kern.go
   复杂脚本                       indic_shaping.go indic_reorder.go arabic_joining.go
   双向                             bidi → shaped.go / shaped_run.go / layout.go / wrap.go
   参考：OpenType 规范 / HarfBuzz（indic/arabic 部分语义）/ Unicode UAX #9（bidi）
   │   GlyphOutline（cs 轮廓）
   ▼
③ hint 网格拟合（本文件重点）    见 §2 引擎分派树（参考：FreeType autofit+psaux / skrifa）
   │   *text.GlyphOutline（ds 拟合后轮廓）
   ▼
④ 光栅化 / 缓存 / 绘制            glyph_mask_rasterizer.go glyph_mask_atlas.go
                                  glyph_cache.go draw_aliased.go draw_emoji.go
                                  lcd_filter.go subpixel.go msdf/
   参考：FreeType raster/smooth + lcd.c（LCD 滤镜）/ msdfgen（MSDF 算法）
```

## 1.5 字体源共享（进程级缓存，2026-10-27）

字体文件是重型对象（一个 CJK/彩色字体几十 MB，解析表再翻倍），
`FontSource` 的注释从诞生起就写明“应全应用共享”，但此前没有任何按文件
共享的机制：每次 `NewFontSourceFromFile` 都重新读盘 + 解析，逐字符回退
（`fontScanFallback.resolveFace`）又只按 `(rune, size)` 缓存，同一文件按
字符被反复整份加载。m5 任意脚本窗实测 **75 次加载只覆盖 4 个文件**
（0.8s 内），存活堆 805MB 中有约 460MB 来自这里。

现在的模型对齐 Skia `SkTypefaceCache` / Flutter font manager：

- `NewFontSourceFromFile` 一律走 `globalFontSourceCache`（`source_cache.go`），
  键为 `(绝对路径, TTC index, 解析器)`；同一键只读盘、只解析一次，
  所有加载路径（UI 字体链、系统解析器、权重家族、缺字回退）拿到同一个
  `*FontSource`。并发未命中用 singleflight 合流。
- 缓存是 LRU，上限 32 条 / 256MB 文件字节；条目随进程存活。
  磁盘上字体更新不会自动生效，需要 `PurgeFontSourceCache()` 刷新
  （同时清掉回退/整形/摘要等会钉住源的全局缓存）。
- 缓存持有的源 `Close()` 是 no-op（共享数据不能由单个持有者释放）；
  `NewFontSource` 创建的内存源自持所有权，`Close()` 语义不变。
- 一个源可加入多个 `FontFamily`（`families` 不可变切片 + CAS 追加）；
  `Face` 按注册顺序取第一个能解析目标字重的家族（与 Skia 一个
  typeface 属于多个 style set 的歧义处理一致）。
- 缺字回退的 face 缓存键为 `(字体路径, 字号)`，不再按 rune 逐字符建
  face：同一文件的连续字符因此共享同一个 `Face`，`MultiFace.Runs`
  才能把它们合成一个整形段（此前逐字符拆段，藏文等簇脚本会退化成
  带虚线圈的孤立组合符）。
- 统计口：`FontSourceCacheSnapshot()` / `ResetFontSourceCacheStats()`。

m5 改后实测（60s，`GODEBUG=gctrace=1`）：加载 75→10 次（10 个不同文件，
每个恰一次）、存活堆 805MB→144MB 平台、RSS 1.67–1.75GB→464MB 稳态。

## 2. 引擎分派树（③ 的核心对应关系）

```
                        按字体形态分派（font_tables.go / glyph_outline.go）
                                     │
         ┌───────────────────────────┼──────────────────────────────┐
         ▼                           ▼                              ▼
     CFF / CFF2                  glyf 有字节码                  glyf 无字节码
         │                           │                              │
         ▼                           ▼                              ▼
     cf2（psaux 语义）           tt_engine                     autofit（autohinter）
   hint/psh_light.go          tt_engine.go                autohint_*.go（Full，现有）
   hint/cffcs.go              tt_instance.go              hint/afcjk.go（CJK light 种子）
   hint/cff.go                tt_glyph.go ...             hint/latin_light.go（Latin light 种子）
         │                           │                              │
         └───────────┬───────────────┴──────────────────────────────┘
                     ▼
              FT_LOAD_TARGET_LIGHT 语义
              （grid-fit = 蓝区锚 + 弱网格对齐，无 drop-out）
```

| 字体形态 | 引擎 | 入口 | 参考库 | light 状态 |
|---|---|---|---|---|
| CFF / CFF2 | cf2（psaux 移植） | `hint/hint.go` → `hint/psh_light.go` + `hint/cffcs.go` + `hint/cff.go` | **FreeType psaux**（psblues.c/pshints.c/cffdecode.c） | ✅ M0–M3 完成（3000 常用字 + 韩 11172 + 泰 87 逐点 bad=0） |
| glyf + fpgm/prep | TT 字节码解释器 | `tt_engine.go`（tt_*.go 家族） | **skrifa**（fontations hint engine，非 FT ttinterp） | ❌ L0：加 InterpLight 分支（render/ 高风险，改前 question） |
| glyf 无字节码 | autofit（cjk/latin/indic） | `autohint_*.go`（Full 语义） | **FreeType autofit**（aflatin.c 为主）+ **skrifa**（架构参考，蓝区/t2b 对齐） | ❌ M4：改 light 语义；`hint/afcjk.go` / `hint/latin_light.go` 为移植种子；其余脚本透传不 hint |

## 3. light 模式三种引擎各自做什么（对应关系）

| 引擎 | 输入 | light 下的行为 | 参考库 | 状态 |
|---|---|---|---|---|
| cf2 | CFF 表 + charstring | 蓝区（幽灵区/真实区）→ capture 三态 → hintmap 插入/adjustHints → 全轮廓 cs→ds 映射 | FreeType psaux（2.14.3 语义，对照 2.11.1） | ✅ 完成 |
| tt_engine | fpgm/prep 字节码 | light = 只 Y 向网格调整，X 向指令/IUP 后 X 调整不发 drop-out，强 keeper | skrifa fontations（`hint/engine`） | ❌ L0 |
| autofit | 原始 glyf 轮廓 | afcjk light（顶横蓝线锚定）/ aflatin light / afindic；阿拉伯/希伯来/泰文等**透传** | FreeType autofit（afcjk.c/aflatin.c/afindic.c）+ skrifa（蓝区/t2b） | ❌ M4 |

## 4. 脚本 → 引擎 → 验证集

| 语言族 | 主字体形态 | 走哪个引擎 | 参考库 | 验证状态 |
|---|---|---|---|---|
| CJK（中/日/韩） | CFF（Noto CJK） | cf2 | FreeType psaux | ✅ 3000 字 + 11172 Hangul bad=0 |
| 泰文 | CFF（Noto Sans Thai） | cf2 | FreeType psaux | ✅ 87 字 bad=0 |
| 拉丁/西里尔/希腊 | glyf（DejaVu/Noto Sans 带字节码） | tt_engine | skrifa | ❌ L0–L1（255 字集归零） |
| Latin 无字节码样本 | glyf（DejaVu 版/自造） | autofit latin | FreeType autofit + skrifa | ❌ M4 |
| 阿拉伯/希伯来等 | glyf/CFF | autofit 透传 或 cf2 | —（M4 透传不 hint） | M4 透传 |

## 5. 对照工具链（开发期，仅测试用）

```
 ftexp（hint/cmd/fdiff）
   └─ 系统 FreeType 2.11.1 输出 FT_LOAD_TARGET_LIGHT 像素/轮廓
       对照自研输出 → 逐字 diff（bad=0 为绿）
   验证集：hint/testset.go VerifierSet()（649 字符，M3 扩韩/泰）
   扫描回归：hint/zz_scan_*.go（3000 字 / 韩 11172 / 泰 87）
```

## 6. 相关文档

- `docs/ENGINE_TEXT_HINT_LIGHT_PLAN.md` —— light 移植计划（M0–M7/L0–L1/§13 机制破译）
- `docs/ENGINE_TEXT_FREETYPE_PLAN.md` —— autohint Full 对齐（蓝区/skrifa/t2b）与 §9.8 待办
- `docs/ENGINE_TEXT_SHAPING_PLAN.md` —— shaping 对照（OwnShaper vs HarfBuzz）
