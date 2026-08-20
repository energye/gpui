package platform

// Wayland CSD 标题栏文本渲染。内嵌位图字体（wayland_csd_painter.go 的 csdFont）
// 只有 95 个可打印 ASCII 字形且字形内容偏小，与系统字体混排时中英文字号视觉
// 不一致。这里标题栏文本整体走系统字体链（font fallback，GTK/Flutter 同款）：
// 每个 rune 用链上第一个含其字形的 face 栅格化，英文中文同一字号同一度量，
// 视觉协调；位图字体仅保留为「系统无任何字体」时的兜底（英文位图 + '?'）。
//
// 字体链（按优先级）：全能 CJK 字体（自带 Latin，Noto Sans CJK 等）→ 纯 CJK
// → 纯拉丁兜底（DejaVu）。字形按 (face, rune) 缓存灰度位图，alpha 混合进
// 标题栏 ARGB8888 buffer。

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// csdSysFontSize is the title text em size (px). It matches the bitmap font
// slot height so the fallback path and the system path occupy the same strip.
const csdSysFontSize = csdBitmapGlyphH

// csdGlyph is a cached rasterized glyph: grayscale alpha (width*height,
// row-major), pixel advance, and the glyph's top-left offset (dx, dy)
// relative to its baseline. dy comes from GlyphBounds (baseline-relative
// fixed coordinates) rather than Glyph's dr rectangle, whose integer
// coordinate space is not a stable placement reference across faces.
type csdGlyph struct {
	width, height int
	advance       int
	dx, dy        int
	alpha         []byte
}

// csdGlyphKey identifies a cached glyph by font-chain slot + rune.
type csdGlyphKey struct {
	face int
	r    rune
}

var csdSysFontState = struct {
	sync.Mutex
	faces   []font.Face
	loaded  bool
	loadErr error // cached first-load failure: don't rescan candidates per glyph
	cache   map[csdGlyphKey]csdGlyph
}{cache: make(map[csdGlyphKey]csdGlyph)} //nolint:gochecknoglobals // CSD 标题字体链状态（懒加载 + 字形缓存）

// csdSysFontCandidates lists system font files in chain order: all-capable
// CJK fonts first (they carry Latin glyphs, so one face covers both scripts
// with one visual style), then pure-CJK fonts, then a pure-Latin fallback
// (DejaVu) for systems without any CJK font.
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

// loadCSDSysFaces parses every available font candidate into a chain of
// font.Face (priority order). TTC collections take face 0 (e.g. Noto Sans CJK
// JP variants, shape-compatible for the vast majority of han glyphs).
func loadCSDSysFaces() ([]font.Face, error) {
	var faces []font.Face
	for _, p := range csdSysFontCandidates() {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var f *opentype.Font
		if filepath.Ext(p) == ".ttc" {
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
			faces = append(faces, face)
		}
	}
	if len(faces) == 0 {
		return nil, fmt.Errorf("wayland csd: no system font for title text (tried %d candidates)", len(csdSysFontCandidates()))
	}
	return faces, nil
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

// csdSysGlyph returns the cached rasterized glyph for r, rasterizing on miss
// with the first font-chain face that contains r (standard font fallback).
// ok=false when no system face could be loaded or no face has the glyph
// (caller falls back to the bitmap font / '?').
func csdSysGlyph(r rune) (csdGlyph, bool) {
	csdSysFontState.Lock()
	defer csdSysFontState.Unlock()
	if !csdSysFontState.loaded {
		if csdSysFontState.loadErr == nil {
			faces, err := loadCSDSysFaces()
			if err != nil {
				csdSysFontState.loadErr = err
			} else {
				csdSysFontState.faces = faces
			}
		}
		csdSysFontState.loaded = true
	}
	for i, face := range csdSysFontState.faces {
		key := csdGlyphKey{face: i, r: r}
		if g, ok := csdSysFontState.cache[key]; ok {
			return g, true
		}
		// Font fallback: the first face that has the glyph wins.
		gb, advance, ok := face.GlyphBounds(r)
		if !ok {
			continue
		}
		_, mask, _, _, ok := face.Glyph(fixed.P(0, 0), r)
		if !ok {
			continue
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
		i2 := 0
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				_, _, _, a := mask.At(x, y).RGBA()
				g.alpha[i2] = byte(a >> 8)
				i2++
			}
		}
		csdSysFontState.cache[key] = g
		// Defensive cap: titles are a few dozen runes; a pathological burst
		// of distinct runes must not grow the map unbounded. A clear only
		// re-rasterizes on next use, so the steady state is unaffected.
		if len(csdSysFontState.cache) > 2048 {
			csdSysFontState.cache = make(map[csdGlyphKey]csdGlyph)
		}
		return g, true
	}
	return csdGlyph{}, false
}

// csdSysBaselineOffset returns the baseline's pixel offset from the strip
// top (yOff), computed from the chain-head face's metrics so the text block
// is vertically centered — Pango's formula, baseline = center -
// (ascent+descent)/2 + ascent. One baseline is shared by every glyph on the
// line, so fallback glyphs rasterized from different faces still sit on the
// same baseline (each glyph's dy is baseline-relative).
func csdSysBaselineOffset() int {
	csdSysFontState.Lock()
	defer csdSysFontState.Unlock()
	if !csdSysFontState.loaded || len(csdSysFontState.faces) == 0 {
		return csdBitmapGlyphH // bitmap fallback: slot bottom
	}
	m := csdSysFontState.faces[0].Metrics()
	ascent, descent := fixedRound(m.Ascent), fixedRound(m.Descent)
	return (csdBitmapGlyphH-ascent-descent)/2 + ascent
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
