//go:build !nogpu

package gpu

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/render/text/emoji"
)

type colorStubParser struct{}

func (colorStubParser) Parse([]byte) (text.ParsedFont, error) { return &colorStubParsed{}, nil }

type colorStubParsed struct{}

func (c *colorStubParsed) Name() string     { return "ColorStub" }
func (c *colorStubParsed) FullName() string { return "ColorStub Regular" }
func (c *colorStubParsed) NumGlyphs() int   { return 100 }
func (c *colorStubParsed) UnitsPerEm() int  { return 1000 }
func (c *colorStubParsed) GlyphIndex(r rune) uint16 {
	if r == 'A' {
		return 65
	}
	return 66
}
func (c *colorStubParsed) GlyphAdvance(uint16, float64) float64 { return 10 }
func (c *colorStubParsed) GlyphBounds(uint16, float64) text.Rect {
	return text.Rect{}
}
func (c *colorStubParsed) Metrics(float64) text.FontMetrics { return text.FontMetrics{} }

func (c *colorStubParsed) HasColorTables() bool { return true }
func (c *colorStubParsed) GlyphType(gid uint16) text.GlyphType {
	if gid == 65 {
		return text.GlyphTypeBitmap
	}
	return text.GlyphTypeOutline
}
func (c *colorStubParsed) BitmapGlyph(gid uint16, ppem uint16) (*emoji.BitmapGlyph, error) {
	if gid != 65 {
		return nil, emoji.ErrNoCBDTTable
	}
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 128, B: 64, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	data := buf.Bytes()
	return &emoji.BitmapGlyph{
		GlyphID: gid,
		Data:    data,
		Format:  emoji.FormatPNG,
		Width:   8,
		Height:  8,
		OriginX: 0,
		OriginY: 8,
		PPEM:    16,
	}, nil
}
func (c *colorStubParsed) COLRGlyph(uint16, int) (*emoji.COLRGlyph, error) {
	return nil, emoji.ErrNoCOLRTable
}

func colorStubFace(t *testing.T, size float64) text.Face {
	t.Helper()
	text.RegisterParser("colorstub", colorStubParser{})
	src, err := text.NewFontSource([]byte("stub"), text.WithParser("colorstub"))
	if err != nil {
		t.Fatalf("NewFontSource: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })
	return src.Face(size)
}

func TestGlyphMaskRejectsColorRun(t *testing.T) {
	face := colorStubFace(t, 16)
	engine := NewGlyphMaskEngine()
	if _, err := engine.LayoutText(face, "A", 10, 50, render.Black, render.Identity(), 1); err == nil {
		t.Fatal("LayoutText(color glyph) = nil error, want explicit refusal to CPU fallback")
	}
}

func TestGlyphMaskKeepsOutlineRun(t *testing.T) {
	face := colorStubFace(t, 16)
	engine := NewGlyphMaskEngine()
	if _, err := engine.LayoutText(face, "B", 10, 50, render.Black, render.Identity(), 1); err != nil {
		t.Fatalf("LayoutText(outline glyph in color-capable font) = %v, want nil", err)
	}
}

func TestGlyphMaskRejectsShapedColorRun(t *testing.T) {
	face := colorStubFace(t, 16)
	engine := NewGlyphMaskEngine()
	glyphs := []text.ShapedGlyph{{GID: 65, XAdvance: 10}}
	if _, err := engine.LayoutShapedGlyphs(face, glyphs, 10, 50, render.Black, render.Identity(), 1, false); err == nil {
		t.Fatal("LayoutShapedGlyphs(color glyph) = nil error, want explicit refusal")
	}
}
