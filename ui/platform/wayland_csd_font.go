package platform

// Wayland CSD 标题栏的非 ASCII 字形渲染。内嵌位图字体（wayland_csd_painter.go
// 的 csdFont）只有 95 个可打印 ASCII 字形，标题含中文等多语言字符时全部回退
// 成 '?'（乱码）。这里用系统字体（golang.org/x/image/font/opentype，纯 Go 无
// cgo）栅格化非 ASCII rune，按 rune 缓存灰度位图，alpha 混合进标题栏的
// ARGB8888 buffer——GTK parity：CSD 标题栏支持任意 UTF-8 标题。
//
// 设计：ASCII（含 – — 的 '-' 映射）继续走内嵌位图字体（快、零依赖、与按钮
// 图标同风格）；只有非 ASCII 才查系统字体。字形缓存 map[rune]csdGlyph 避免
// 每次标题栏重绘（hover/按压/resize/title 变更）重复栅格化。找不到任何系统
// 字体或字形缺失时回退 '?' 占位（保持可见，不崩）。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// csdSysFontSize matches the embedded bitmap font height, so mixed ASCII +
// CJK titles keep one visual size.
const csdSysFontSize = csdBitmapGlyphH

// csdGlyph is a cached rasterized glyph: grayscale alpha (width*height,
// row-major), pixel advance and the glyph's top-left offset (dx, dy) relative
// to its placement point, derived from GlyphBounds (baseline-relative).
type csdGlyph struct {
	width, height int
	advance       int
	dx, dy        int
	alpha         []byte
}

var csdSysFontState = struct {
	sync.Mutex
	face    font.Face
	loadErr error // cached first-load failure: don't rescan candidates per glyph
	cache   map[rune]csdGlyph
}{cache: make(map[rune]csdGlyph)} //nolint:gochecknoglobals // CSD 标题系统字体状态（懒加载 + 字形缓存）

// csdSysFontCandidates lists system fonts able to render non-ASCII title
// characters, CJK first (Noto Sans CJK ttc / AR PL UMing / WQY / user font
// dir), DejaVu Sans as a last resort for non-CJK scripts.
func csdSysFontCandidates() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".local/share/fonts/NotoSansCJKsc-Regular.otf"),
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
		"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
		"/usr/share/fonts/truetype/arphic/uming.ttc",
		"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	}
}

// loadCSDSysFace parses the first available system font candidate into a
// font.Face. TTC collections take face 0 (e.g. Noto Sans CJK JP variants,
// shape-compatible for the vast majority of han glyphs).
func loadCSDSysFace() (font.Face, error) {
	for _, p := range csdSysFontCandidates() {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var f *opentype.Font
		if strings.EqualFold(filepath.Ext(p), ".ttc") {
			c, err := opentype.ParseCollection(data)
			if err != nil || c.NumFonts() == 0 {
				continue
			}
			if f, err = c.Font(0); err != nil {
				continue
			}
		} else {
			if f, err = opentype.Parse(data); err != nil {
				continue
			}
		}
		face, err := opentype.NewFace(f, &opentype.FaceOptions{
			Size:    csdSysFontSize,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err == nil {
			return face, nil
		}
	}
	return nil, fmt.Errorf("wayland csd: no system font for title text (tried %d candidates)", len(csdSysFontCandidates()))
}

// fixedRound converts a 26.6 fixed value to the nearest integer pixel.
func fixedRound(f fixed.Int26_6) int {
	return int((f + 32) >> 6)
}

// fixedFloor converts a 26.6 fixed value down to the integer pixel (safe for
// negative bearings: arithmetic shift rounds toward -inf).
func fixedFloor(f fixed.Int26_6) int {
	return int(f >> 6)
}

// csdSysGlyph returns the cached rasterized glyph for r, rasterizing on
// miss. ok=false when no system face could be loaded or the glyph could not
// be produced (caller falls back to '?').
func csdSysGlyph(r rune) (csdGlyph, bool) {
	csdSysFontState.Lock()
	defer csdSysFontState.Unlock()
	if g, ok := csdSysFontState.cache[r]; ok {
		return g, true
	}
	if csdSysFontState.face == nil {
		if csdSysFontState.loadErr == nil {
			face, err := loadCSDSysFace()
			if err != nil {
				csdSysFontState.loadErr = err
			} else {
				csdSysFontState.face = face
			}
		}
		if csdSysFontState.face == nil {
			return csdGlyph{}, false
		}
	}
	// Placement is derived from GlyphBounds (baseline-relative fixed
	// coordinates) rather than Glyph's dr rectangle, whose integer
	// coordinate space is not a stable placement reference across faces.
	// Glyph is called at integer dot (0,0): the mask content is the glyph
	// itself, dot only affects dr which we do not use.
	gb, advance, ok := csdSysFontState.face.GlyphBounds(r)
	if !ok {
		return csdGlyph{}, false
	}
	_, mask, _, _, ok := csdSysFontState.face.Glyph(fixed.P(0, 0), r)
	if !ok {
		return csdGlyph{}, false
	}
	b := mask.Bounds()
	g := csdGlyph{
		width:   b.Dx(),
		height:  b.Dy(),
		advance: fixedRound(advance),
		dx:      fixedFloor(gb.Min.X),
		dy:      fixedFloor(gb.Min.Y),
		alpha:   make([]byte, b.Dx()*b.Dy()),
	}
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := mask.At(x, y).RGBA()
			g.alpha[i] = byte(a >> 8)
			i++
		}
	}
	csdSysFontState.cache[r] = g
	return g, true
}

// blendCSDGlyph composites a cached glyph (grayscale alpha) into the
// ARGB8888 title-bar buffer at (gx, gy) with solid color c (B,G,R,A).
// The buffer's alpha channel is left untouched (title bar is opaque).
func blendCSDGlyph(buf []byte, stride, gx, gy int, g csdGlyph, c [4]byte) {
	if len(buf) == 0 || stride <= 0 {
		return
	}
	for row := 0; row < g.height; row++ {
		py := gy + row
		if py < 0 || py*stride >= len(buf) {
			continue
		}
		rowOff := py * stride * 4
		for col := 0; col < g.width; col++ {
			a := g.alpha[row*g.width+col]
			if a == 0 {
				continue
			}
			px := gx + col
			if px < 0 {
				continue
			}
			off := rowOff + px*4
			if off+3 >= len(buf) {
				continue
			}
			inv := 255 - int(a)
			buf[off] = byte((int(c[0])*int(a) + int(buf[off])*inv) / 255)
			buf[off+1] = byte((int(c[1])*int(a) + int(buf[off+1])*inv) / 255)
			buf[off+2] = byte((int(c[2])*int(a) + int(buf[off+2])*inv) / 255)
		}
	}
}
