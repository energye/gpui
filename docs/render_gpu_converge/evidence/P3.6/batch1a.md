# P3.6 批 1a：格式嗅探 + EXIF 方向（2026-11-02 交付）

直话：批 1 太大，按可独立提交的粒度拆成 1a（格式识别与 EXIF 方向）与 1b（GIF/APNG/动图 WebP 三容器 + `AnimatedImage`）。本批交付 1a：**图片解码不再看文件名**，**EXIF 方向真在解码期应用且返回尺寸已交换宽高**，**AVIF/HEIF 被认出来并具名报错**，**原本零使用的死码哨兵 `ErrUnsupportedFormat` 接成真实错误路径**。公开面 +3 类型 / +4 函数 / +16 常量 / +2 变量。

## 1 交付内容

- `render/internal/image/encoding.go`（新增）：`Encoding` 容器枚举 + `SniffEncoding`（魔数识别：PNG/JPEG/GIF/WebP + ISO-BMFF `ftyp` 品牌分 AVIF/HEIF）+ `ErrDecodable`/`IsDecodable`。
- `render/internal/image/exif.go`（新增）：`ImageOrientation`（EXIF tag 0x0112 的 9 档，值对齐 Skia `SkEncodedOrigin`）+ `SwapsWidthHeight` + `orientImage`（解码期应用）+ 三个容器的 EXIF 段扫描（JPEG APP1 段扫、PNG `eXIf` chunk、WebP `EXIF` chunk）+ 有界头部窗口（`maxHeaderScan`=64 KiB，不整读文件）。
- `render/internal/image/io.go`（改）：`Decode(r, opts)`/`LoadImageFromBytes(data, opts)`/`LoadImageWithOptions(path, opts)`；`LoadImage` 改嗅探优先（原来扩展名优先，`io.go:73-81`）；`loadFile` 让 `LoadPNG/LoadJPEG/LoadWebP` 在内容与所求格式不符时**显式报错**而非静默换解码器；`Decode*` 原始族保持不加方向（两扇门语义写进 doc）。
- `render/context_image_decode.go`（新增）+ `render/context_image_buf.go`（改）：公开别名与常量组、四个新函数、两个错误变量。顺带清掉 `context_image_buf.go` 里 4 处重复粘贴的 doc 行（同一句注释写两遍，HEAD 存量缺陷）。
- `render/testdata/imgfmt/`（新增 28 个文件，≤2.9 KB/个）+ `scripts/gen_p36_testdata.py`：容器样本与 EXIF 方向参照网格（参照由 Pillow `ImageOps.exif_transpose` 产出，不用我方映射自算）。

## 2 关键实测（动手依据，详见 `../batch1-containers-probe.md`）

- JPEG 的 APP1 **不在第一个段**（JFIF APP0 在前，实测 `exif_1.jpg` 标记序 `0xe0,0xe1,0xdb,…`）→ 解析必须扫段，不能按固定偏移读。
- `ImageOps.exif_transpose` 能读本仓手工写的最小 APP1（`{274: N}`），因此可当独立裁判。
- stdlib GIF/EXIF 相关事实：`gif.GIF` 无 `Transparency` 字段，透明表达成调色板里 alpha=0 的项（`GOROOT/src/image/gif/reader.go:396-408`）；`LoopCount` 0=无限、-1=只播一次、N=播 N+1 次（`:577-582`）。批 1b 的映射表按此写。

## 3 钉与容差

`render/image_decode_p36_test.go`（新增 9 条）：

| 钉 | 口径 | 实测 |
|---|---|---|
| `TestImageEncodingOfBytesMatchesSamples` | 28 个样本逐个嗅探结果正确 | PASS |
| `TestImageEncodingOfBytesNamesAVIFHEIF` | AVIF/HEIF 品牌分档 + 具名 `ErrUnsupportedFormat` + `IsDecodable()=false` + 垃圾字节同哨兵 | PASS |
| `TestLoadImageIgnoresFileExtension` | PNG 字节装成 `.jpg`、JPEG 装成 `.png`、WebP 无扩展名，三者都按内容解出 40x30；`LoadWebP` 拿到 PNG 字节必须报错 | PASS |
| `TestLoadImageAppliesOrientationMatchesReferenceGrid` | 8 档方向逐格比对 Pillow 参照网格 + 尺寸（含 5..8 .transpose 后 30x40） | **逐格抖动 20/1200 = 1.67%**（8 档同数，说明差异集中在特征边缘，两套 JPEG 解码器的色度抽样不同）；声明预算 3%，写死在 `p36MaxMismatchPct` 注释里 |
| `TestPreserveOrientationKeepsStoredRaster` | 关掉应用后八档互相**逐字符全等**（同一段存储栅格），并对 orientation=1 参照在 3% 内 | PASS（同样 1.67%） |
| `TestEachOrientationProducesDistinctRaster` | 八档输出必须两两不同（防「解析了 tag 但没应用」的假绿） | PASS |
| `TestImageOrientationOfBytesReportsTagOnly` | 只报 tag 不解码；`noexif.jpg`/PNG/WebP/GIF = `OrientationUnspecified` 且无错误 | PASS |
| `TestImageOrientationSwapsWidthHeight` | 1..4 不交换、5..8 交换 | PASS |
| `TestStaticDecodersStayUnoriented` | `Decode*` 原始族不加方向；同一文件默认路 30x40 vs 原始 40x30 | PASS |

## 4 负向对照（红→还原绿）

1. **摘掉方向应用**（`io.go` 里把 `ori` 强制回 `OrientationUnspecified`）→ `TestLoadImageAppliesOrientationMatchesReferenceGrid`、`TestEachOrientationProducesDistinctRaster`、`TestStaticDecodersStayUnoriented` 红（`oriented size = 40x30, want 30x40`）；还原 → 绿。
2. **摘掉 WebP 魔数识别**（`encoding.go` 的 RIFF/WEBP 分支返回 `EncodingUnknown`）→ `TestLoadImageIgnoresFileExtension`、`TestImageEncodingOfBytesMatchesSamples` 与存量 `render/internal/image` 的 `TestLoadImage_WebP`（扩展名 + 内容两路）红（`image: unsupported format: unrecognised container`）；还原 → 绿。
   这同时证明：新增的嗅探路不是摆设，且死码哨兵已接到真实错误路径。

## 5 门禁

| 项 | 命令 | 结果 | 日志 |
|---|---|---|---|
| apidoc | `go run ./scripts/apidoc` | rc=0（23 个新符号逐名登记进 §3.5 图像族新行；斜杠合并写法不通过，必须逐名） | 本文 |
| 计数 | go/ast 实测（本仓 render 主包，非测试文件） | 类型 147 / 顶层函数 188 / 常量 188 / 变量 28（原 144/184/172/26；Context 219、Canvas 109、PresentTarget 18 不变） | catalog §0 |
| vet | `go vet ./render ./render/internal/image ./ui/io` | rc=0；`./render/...` 全扫仍命中 `render/recording/backends/raster` 的 P3.1 线存量坏包（HEAD 同红，非本批） | 本文 |
| 定向 | `go test -count=1 -run …` 两包 | ok | `targeted.txt` |
| conformance | `TestConformanceDualBackend` 双后端 | PASS（`ok github.com/energye/gpui/render 1.232s`） | `conformance.txt` |
| race | `-race -count=2`：`./render/internal/image`、`./render/recording`、`./render`（本批测试，整包 = 批 4 已登记的既有 native abort 不重跑） | 三组全 ok | `race/race.txt` |
| 五目标构建 | `build-gate.sh`（linux-amd64/arm64、windows-amd64、darwin-amd64/arm64 × {默认, nogpu}） | **5/5 各 2/2 全 exit 0**（包数 326/330、324/328、332/336；首轮 `go build ./...` 的 exit 1 = 存量已知坏包，脚本按 P2.6 同一口径列出并排除） | `build/*.txt` |
| 逐文件回归 | `tools/per-file-tests.sh tools/per-file-tests.pkgs` + `evidence/P3.5/per-file-extra.pkgs` | 主 **699 PASS / 21 SKIP / 9 FAIL / 1 NO_TESTS**（P3.5 收口基线 698/21/9/1，多出的 1 枚正是本批新增 `render/image_decode_p36_test.go`=PASS；**FAIL 九枚与基线逐名相同 = 零新增红**）；补充 **72 PASS / 2 SKIP / 6 FAIL**（6 枚 = P3.1 线 `render/recording/backends/raster` 存量坏包，名单与基线逐名相同） | `per-file-tests/summary.tsv`、`per-file-tests-extra/summary.tsv` |

## 6 未做与归属（不留空白）

- GIF/APNG/动图 WebP 解码与 `AnimatedImage` 帧接口 = **批 1b**（`Encoding` 已能识别 GIF，`decodeByFormat` 目前对它报「no decoder wired in」，这条错误路径 1b 会换成真解码）。
- `ui/io` 第二套注册表改委托 = 批 1b（D5 收敛的其中一半）。
- AVIF/HEIF 像素解码 = 远期（D1），本批只做品牌识别 + 具名错误。
- PNG `gAMA`/`cHRM`/`iCCP` 与 sRGB 目标格式 = P3.1b 色彩管理线；本批只碰方向，不碰色域。
- 真窗 `examples/ui_wr_p36_imgfmt` = 批 3；本批口径为原子验证（单测 + 参照网格），按 AGENTS 不得当生产接线标绿。

## 7 顺带发现（登记，本批未修）

- `gofmt -l render/` 在 HEAD 就命中 `filter_ops.go`、`filter_ops_gpu.go`、`matrix4.go`、`recording/recorder.go` 四个未格式化文件（非本批引入，本批只格式化了自己碰的文件）。
- `render/internal/image/format.go` 与批 1a 新增的容器类型同名冲突：像素布局的 `Format` 已占用该名字，容器类型因此定名 `Encoding`/`ImageEncoding`。踩过一次（写文件时覆盖了 `format.go`，git 恢复），记录以免再来。
