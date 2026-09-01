# R6 文本栈审查（render/text 全目录 + render/text*.go + ui/textinput）

## 一句直话

**零件是工业级的，但整机没装好：字体解析、hinting、光栅化、图集这些"发动机零件"接近 FreeType/Skia 水准，可是 UI 主绘制路径根本没接 shaping（连字/kerning/阿拉伯文变形全丢），bidi 只有半套，IME 协议桥有真 bug，缓存 key 有一串碰撞隐患。**

---

## 维度一：正确性（栅格化质量 / 度量衡 / shaping / bidi / 断行）

### P0-1 主绘制管线不走 shaping（最大硬伤）

整条 UI 文本链路都是"逐字符 cmap 查表 + hmtx 步进"，没有 GSUB/GPOS：

- `render/text/face.go:258` `sourceFace.Glyphs` → `iterGlyphs`：纯 cmap + advance，无 kerning/ligature。
- GPU 路径：`render/text.go` dispatchText → engine `LayoutText` → `render/text/shape_result_cache.go:379` `LayoutGlyphs`，注释自己写明 "cmap + advance; no GSUB/GPOS"。
- CPU 路径：`render/text/draw.go` drawGlyphs 同样用 `sf.Glyphs`。
- 测量路径：`text.Measure` → `face.Advance` 也是逐 rune；UI 层（ui/rendering paint_context）只用 DrawString/WrapText/Measure。
- shaping 引擎本身存在且默认就是 HbShaper（`shaper.go:21`），但生产代码只有矢量路径和手动 `DrawShapedGlyphs` 用到它——**等于造好了 HarfBuzz 却插在展示架上**。

后果：拉丁文 kerning/连字丢失，阿拉伯文全部孤立形，Indic 不重组。这是"对标 SkParagraph"最核心的差距。

### P1-2 bidi 只保留奇偶两级，段序不重排

- `render/text/segment.go:80-84`：`runLevel = 1 if RTL else 0`，把 x/text 算出的真实嵌入层级压平成 0/1，嵌套（如 RTL 里嵌 LTR 再嵌 RTL）信息丢失。
- `render/text/layout.go:266-309` `shapeSegments` 按 segment 逻辑顺序从左往右摆放，不做 UAX#9 L2 的跨段视觉重排——RTL 基方向段落（阿语句子里嵌英文）整段顺序错；换行切断 RTL 段时阅读顺序乱。

### P1-3 IME delete_surrounding_text 被静默丢弃【已核实】

- `ui/platform/wayland_textinput_linux.go:411-423`：把删除请求编码为 IMECaretMove + Start 取负，注释声称 "The textinput editor interprets Start<0 as a delete request"。
- 但 `ui/textinput/editor.go:345-348` 的分支要求 `ev.Start >= 0 && ev.End >= 0` 才处理——负值直接被忽略。依赖此事件做删词的输入法（韩文等）会坏。

### P1-4 IME 组合窗光标字段错位【已核实】

- `wayland_textinput_linux.go:380-393` `wlTiPreedit` 把 index(cursor) 放 IMEStart、commit 放 IMEEnd；zwp v3 里 commit 是字符串指针，cast 成 int32 是垃圾值。
- `editor.go:354-358` `caretFromIME` 取的是 `ev.End` 当光标偏移——组合窗内光标位置错（被 clamp 兜底不崩但位置不对）。X11 传 -1 时 caretFromIME 返回 0（跳到开头），与 UpdateCompose 注释 "-1 = end" 相反。

### P1-5 变量字体在 GPU 路径丢 variations

- `RasterizeHinted`（glyph_mask_rasterizer.go）不接收 variations 参数；engine 栅格化也不传 `face.Variations()` → GPU glyph-mask 路径一律渲染默认实例的字形轮廓，而步进又走了 HVAR 变量宽度——轮廓与步进可能错位、字重不对。CPU 路径 `drawGlyphsVariable` 有正确处理，两条路不一致。

### P2-6 其余正确性问题

| 问题 | 位置 | 说明 |
|---|---|---|
| script 探测只看首字符 | shaper_own.go:684 / shaper_hb.go:172 | 混排文本直调 Shape() 整段用错 script tag |
| WrapText \r\n 偏移漂移 | wrap.go:410-441 | 归一化 \r\n→\n 后按 len(para)+1 累加，原文 2 字节算 1 字节，后续所有 Start/End 错位 |
| 逐字测量丢 kerning | wrap.go:543-556 measureRune | 单 rune Shape 累加宽度 ≠ 实际 shaped 宽度，断行点漂移 |
| LineSpacing 只乘 LineGap | layout.go:190 | LineGap=0 的字体行距调节完全无效（Flutter 乘总高） |
| emoji 序列拆开画 | draw_emoji.go:41-63 | ZWJ 家族 emoji/国旗逐码点绘制；COLR TODO 未接（draw_emoji.go:57-59） |
| cluster 单位混用 | layout.go shapeSegments `Cluster += seg.Start`（字节）+ shaper 内 cluster=rune index | 多字节文本断行/hit-test 映射可能错 |

### 亮点（要说公道话）

hinting 栈是真功夫：TT bytecode 解释器、CFF light（psh/cf2 移植）、autofit，配 FreeType 金标对照测试（CJK/韩文/泰文 bad=0）；ftgrays 光栅化移植有字节级对照；LCD/AA/次像素量化齐全。栅格化质量这一项可以放心。

---

## 维度二：性能（图集 / 缓存）

| 级别 | 问题 | 位置 |
|---|---|---|
| P1/P2 | CPU Draw 每帧重新栅格化每个字形，无掩码缓存；每次 NewGlyphMaskRasterizer 分配缓冲 | draw.go drawGlyphs |
| P2 | LayoutText 主排版路径用 ShapeUncached，完全绕开 shape 结果缓存，每帧全量重排 | layout.go:275 |
| P2 | WrapText 逐 rune Shape（每帧 n 次哈希查找），UI drawTextWrapped 每帧走它 | wrap.go:545 |
| P2 | markBreakOpportunitiesEnhanced cluster→rune 线性扫描 O(n²) | wrap.go/layout.go |
| P2 | OwnShaper 缓存满 64 个源就整体清空重建（抖动） | shaper_own.go:218-222 |
| P3 | globalShapeResultCache 单锁 + O(n) 驱逐扫描；IsHighChurnLabel 启发式误伤数字多的静态标签 | shape_result_cache.go |

正面：GPU 侧三层缓存设计不错——layout template（整串复用 quads）→ shape result 缓存 → glyph mask atlas（shelf 分配器 + 整页 reset + generation 失配同步），对齐 Skia 思路。atlas 默认 4 页 ×1024² R8 ≈ 4MB 显存，克制。

---

## 维度三：资源占用

| 级别 | 问题 | 位置 |
|---|---|---|
| P1 | fallback faceCache 按 (rune,size) 缓存，每个 miss 都 NewFontSourceFromFile **整读字体文件并新建 FontSource**——同一系统字体文件被几十个 rune×size 反复加载成几十份拷贝，无上限无 mmap | fontscan_fallback.go:100-127 |
| P2 | 字体内存双份驻留：parse 引用调用方 data + dataCopy 再存一份（≈2× 字体大小；Noto CJK TTC ~20MB→40MB+），HB typesetting 还有第三份解析；无 mmap（Skia 用 mmap） | source.go:60-79 |
| P2 | HbShaper.faces map 无上限，RemoveSource 无人调；FontSource.Close 不清任何 shaper/atlas 缓存（TASK-044 自认未做） | shaper_hb.go:40-47, source.go:141-152 |
| P2 | multiFaceRunsCache 以 *MultiFace 指针为 key，持强引用阻碍 GC，2048 条随机驱逐 | multi.go:318-370 |
| P2 | fontIDCache map[uintptr]uint64 永不清理 | glyph_mask_engine.go:1031-1047 |
| 正面 | 图集内存管理本身健康（4MB 上限、压缩、桶模式滞回） | glyph_mask_atlas.go |

---

## 维度四：隐藏 Bug（重点清单）

1. **P1 缓存 key 冲突族（三处同根）**：
   - `FontSourceID` = hash(FullName + NumGlyphs)（shape_result_cache.go:260-275）、`computeFontID`（glyph_renderer.go:256-287）、`computeGlyphMaskFontID`（glyph_mask_engine.go:1002）——两个不同字体文件同名同字数即撞 key，缓存/图集互串错字形。
   - `HashFontFeatures` XOR 折叠（shape_result_cache.go:279-297）：XOR 可交换 → {liga:1, kern:0} 与 {liga:0, kern:1} 哈希相同，feature 开关互换共享缓存。HashFontVariations 同病。
   - `fontIDCache` 按 uintptr 指针缓存：FontSource 被 GC 后地址复用 → 新源拿旧 ID（经典 ABA），图集可能串字体。
2. **P1 GlyphMaskKey 缺维度**（glyph_mask_atlas.go）：key 无 hinting 位也无 variations 位，而 hinting 由 selectGlyphMaskHinting 按上下文（isCJK/矩阵）动态变化——同一字形不同 hinting 共享图集条目。
3. **P1 HbShaper 数据竞争**：`Shape` 在锁外改共享 hbFont 的 XScale/YScale（shaper_hb.go:67-68），多 goroutine 不同字号并发 shaping 会竞态 + 错误缩放。
4. **P2 IME preedit 字段错位**（见正确性 P1-4）+ 光标矩形链路断：EnableIME 只有示例里写死 Rect，CaretXForCluster 无生产消费者，候选词窗口不会跟随光标。
5. **P3 textHash 仅 FNV64 无长度守卫**：碰撞静默显示错字（概率低但是隐患）。

---

## 维度五：可读性与冗余

- **P2 两套断行实现并存**：wrap.go WrapText（逐字测量）vs layout.go wrapLinesWithMode（shaped runs），行为不一致还会继续漂移。
- **P2 CJK 判定三四份且已漂移**：wrap.go IsCJKRune 含全角区 FF00-FFEF，layout.go isCJK 不含；gpu 又有一份 stringContainsCJK。
- **P2 死代码**：cache/shaping.go 整包 ShapingCache 无生产消费者；fixTabGlyphs 仅测试调用。
- **P3 文档矛盾**：shaper.go 头部/SetShaper 注释说默认 OwnShaper，实际 defaultShaper=NewHbShaper()。
- **P3 仓库卫生**：b1c/*.log 提交入库；38 个 zz_* 调试测试混在生产包目录。
- **P3 并发卫生**：globalTabWidth 无锁全局可变。

---

## 对标 Flutter / SkParagraph 还差多少？

直话：**像素层已经摸到工业级门槛，排版集成层差整整一代。**

SkParagraph 的链路是：一次 `paragraph.layout()` 完成 bidi(ICU/UAX#9 完整层级) → shaping(HarfBuzz) → 断行(UAX#14，在 shaped runs 上量宽) → 绘制(同一套 shaped glyphs 进 Skia)，全程一套数据；字体 mmap 加载，fallback 按 typeface 页缓存。

本项目的对应现状：

| 能力 | 本项目 | 差距估计 |
|---|---|---|
| 拉丁 UI 文本 | 有 kerning/ligature 能力但主路径没接 | ~70%（差 kerning/连字） |
| CJK | 基本可用（无连写需求） | ~85% |
| 复杂文种（阿/印地） | 主路径不变形、bidi 半套 | ~20% |
| emoji 序列/COLR | 逐码点画、COLR TODO | ~40% |
| IME 闭环 | 协议半截、坐标链路断 | ~50% |
| hinting/栅格化质量 | FT 金标对照、bad=0 | ~90%（亮点） |
| GPU 缓存架构 | 三层缓存对齐 Skia | ~85% |

一句话总结差距：**不在零件，在总装。** 把 shaping 接进主管线、补全 bidi 和 IME，就能从"能看"跨到"工业可用"；否则复杂文种永远停在 demo 水准。

---

## 评分表

| 维度 | 得分 | 理由 |
|---|---|---|
| 正确性 | **6 / 10** | hinting/栅格化接近满分；但主管线无 shaping（P0）、bidi 半套、断行测量失真、多处隐藏 bug 拉分 |
| 性能 | **7 / 10** | GPU 三层缓存设计优秀；CPU 路径每帧重栅格化、WrapText 逐字 Shape、O(n²) 扫描拖后腿 |
| 资源占用 | **5 / 10** | fallback 字体重复加载无上限（P1）、双份驻留无 mmap、多个缓存无界；图集本身克制 |
| 可读性 | **6 / 10** | 分层清晰、注释足、测试文化好；扣分在死代码、双实现漂移、文档矛盾、调试残留 |

**总分：24 / 40** —— 底子好的 70 分项目，总装完成可上 85。

---

## TOP5 必修清单

1. **【P0】把 shaping 接进主绘制管线**：让 DrawString（GPU LayoutGlyphs 与 CPU drawGlyphs）改走 Shape()/HbShaper 结果（或至少先接 kern/GPOS pair），拉丁文立刻受益，复杂文种才有地基。
2. **【P1】修 IME 三件套**：实现 editor 对 Start<0 删除请求的处理（delete_surrounding_text）；修正 preedit cursor 字段映射（caretFromIME 应取 Start/index 而非 End）；从 CaretXForCluster 接出光标像素矩形动态更新 set_cursor_rectangle。
3. **【P1】缓存 key 加固**：GlyphMaskKey 补 hinting/variations 位；HashFontFeatures/HashFontVariations 从 XOR 改为排序后拼接哈希；fontID 从名字哈希换内容指纹；fontIDCache 弱引用化并在 Close 时清理。
4. **【P1】资源治理**：fallback faceCache 改为按文件路径去重 + 设上限；字体加载改 mmap 或至少消除双份拷贝；HbShaper.faces 设上限并在 Close 时失效；修 XScale 并发写（锁内复制或 per-shape font）。
5. **【P1】bidi 补全 + 断行统一**：保留 x/text 完整嵌入层级并按 UAX#9 L2 做段间视觉重排；统一 WrapText 与 wrapLinesWithMode 为一条基于 shaped runs 的断行路径；顺手修 WrapText 的 \r\n 偏移漂移。

---

## 2026-09-01 增补：X11 D-Bus IME 六洞复核（实测环境 X11, GTK_IM_MODULE=ibus, fcitx5 pid=2146260）

> **一句话**：不是小毛病，是整条通道没通——探测返回了总线上不存在的假路径，之后 9 处静默跳过，IME 全程零作用。

### 环境事实（gdbus 实测，非推测）

- `org.freedesktop.IBus` 与 `org.fcitx.Fcitx5` 的 owner 均为 `:1.812`，`GetConnectionUnixProcessID` 均为 2146260，`ps` 为 `fcitx5`——本机无真 ibus，fcitx5 抢注了 ibus 名。
- `org.fcitx.Fcitx5 /org/fcitx/Fcitx5/InputMethod` 报 `Unknown object`，不存在。
- 唯一可用的是 `org.fcitx.Fcitx-0 /inputmethod CreateICv3`，实测返回 `(5, true, 0,0,0,0)`。
- `libX11.so.6` 内 `XkbGetState/XkbKeycodeToKeysym/Xutf8LookupString/XRefreshKeyboardMapping` 四符号均存在。

### 六洞清单（按严重度）

| # | 洞 | 位置 | 现象 | 复核 |
|---|---|---|---|---|
| 1 致命 | 假路径导致全程空转 | `x11_dbus_ime_linux.go:524/532` 拼 `syntheticFcitxPrefix+"%d"`；`isSyntheticPath` 在 9 处静默 return（`setCapabilitiesFcitx:561`、`destroyObject:590`、`callFocusIn:1019`、`callFocusOut:1051`、`callSetCursorLocation:1079`、`callSetSurroundingText:1124`、`callSetContentType:1171`、`ProcessKeyEvent:1275`、`handleSignal:705`） | `gdbus introspect /` 仅有 `inputmethod` 节点，无 `InputContext_*`；`drainX:1088 continue` 再拦截本地插入，通道全断 |
| 2 严重 | ibus 松键被丢 | `ProcessKeyEvent:1283 if eng=="ibus" && !isPress {return false}` | 实测 `state=1<<30(IBUS_RELEASE_MASK)` 的 `ProcessKeyEvent` 可调通，松键永远到不了引擎 |
| 3 严重 | SetSurroundingText 签名错 | `callSetSurroundingText:1117 v:=dbus.MakeVariant(t)` 签名 `s`，ibus 要 `(sa{sv}sv)` 的 `IBusText` | 三种写法均 `err=nil` 静默失败，输入法拿不到上下文 |
| 4 严重 | 键盘未走 XKB | `xKeysymForState:1169` 只调 `XKeycodeToKeysym(dpy,k,0/1)` 异或大小写；全仓无 `Xkb*`/`Xutf8LookupString`/`MappingNotify` | 多布局 group 错、死键 `´+e=é`（P11）合不出；`libX11` 四符号已确认存在 |
| 5 中 | 焦点事件收而不发 | `x11_linux.go:89` 开 `xFocusChangeMask`，`drainX:1028` switch 无 `xFocusIn/xFocusOut` case | Alt+Tab 不会 `DisableIME+EndComposing`，preedit 残留，F-D7/P10 失败 |
| 6 轻 | 空壳与竞态 | `callFocusOut:1056` 仅日志无 `EndComposing`；`Commit:963` 仅清标志；`imeDirty` 在 `:876/:910/:925/:1272` 无锁读（`:949` 已修一半） | `go vet -race` 可复现；与洞1叠加时更难发现 |

### 影响

- 洞1 单独即可让 `ibus↔fcitx5 热切、光标跟随、上下文推送、按键转发` 全失效，且不报错不崩，表现为“没坏”。
- 未提交的 71 行补丁（`ime.go + x11_dbus_ime_linux.go + input_router.go`）修的 CapsLock 与英文导航直通在假路径之上，无法送达。

### 已验证无需改

- `composition.go:27 utf16Len(ev.Text[:cursor])` 正确：`cursor` 为 IBus 字节偏移，`ev.Text[:cursor]` 为字节切片语义。
