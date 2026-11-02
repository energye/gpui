# 批 1a 负向对照（红→还原绿）

两条，都是「把交付面人为破坏，看哪道门禁红」，验完立即还原并重跑绿。操作方式=临时 sed 改源码→跑定向门禁→`cp` 回备份→重跑。

## 1 摘掉「方向应用」这一步

破坏点：`render/internal/image/io.go` 的 `Decode` 里，在读出方向之后强制
`ori = OrientationUnspecified // NEGATIVE CONTROL`（等价于「EXIF 读了但不用」）。

```
$ go test -count=1 -run "TestLoadImageAppliesOrientation|TestEachOrientation|TestStaticDecoders" ./render/
    image_decode_p36_test.go:407: oriented size = 40x30, want 30x40
FAIL    github.com/energye/gpui/render  0.046s
FAIL
```

红了什么：
- `TestStaticDecodersStayUnoriented` — 默认路应得 30x40（orientation 6 转置），实际 40x30；
- `TestLoadImageAppliesOrientationMatchesReferenceGrid` — 8 档逐格比对整片区域换位（远超 3% 预算）；
- `TestEachOrientationProducesDistinctRaster` — 八档塌成同一张栅格。

还原后：`ok github.com/energye/gpui/render 0.045s`。

## 2 摘掉 WebP 魔数识别

破坏点：`render/internal/image/encoding.go` 的 `SniffEncoding`，RIFF/WEBP 分支改为返回
`EncodingUnknown // NEGATIVE CONTROL`（等价于「容器认不出，只能靠扩展名」）。

```
$ go test -count=1 -run "TestImageEncodingOfBytesMatchesSamples|TestLoadImageIgnoresFileExtension|TestLoadImage_WebP" ./render/ ./render/internal/image/
    io_test.go:458: LoadImage(.webp) failed: image: unsupported format: unrecognised container
FAIL    github.com/energye/gpui/render/internal/image  0.009s
FAIL
```

红了什么：
- 本批 `TestImageEncodingOfBytesMatchesSamples`（样本逐个嗅探）与 `TestLoadImageIgnoresFileExtension`（无扩展名的 WebP 字节）；
- **存量** `render/internal/image` 的 `TestLoadImage_WebP`（`.webp` 扩展名 + `.bin` 内容两路）——顺带证明原 `io.go` 里从没被返回过的死码哨兵 `ErrUnsupportedFormat` 已接上真实错误路径（错误文本就是它 + 具名「unrecognised container」）。

还原后：`ok github.com/energye/gpui/render 0.051s`。

## 3 未做的对照（说明）

批 1a 不含动图容器与 GPU 面，因此「摘 fdAT→IDAT 改写」「摘采样器 mip 接线」这类对照归批 1b / 批 2 的负向组（已写进 `evidence/P3.6/design.md` §3 门禁草案）。
