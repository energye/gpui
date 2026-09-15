// Pass-scratch wiring test: an OnPaint-style RasterExtra mixing GPU draws
// with a CPU tint fallback must survive the retained bounds-texture record
// (recordLocalWith). Pixels prove it; geometry and probes are read from
// testdata/pass_scratch_cases.json (atlas pixels from
// testdata/pass_scratch_atlas.png), never hardcoded.
package gpupixel

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/ui/scene"
)

type scratchWireCases struct {
	Canvas [2]int `json:"canvas"`
	Atlas  string `json:"atlas"`
	Fill   struct {
		R float64 `json:"r"`
		G float64 `json:"g"`
		B float64 `json:"b"`
	} `json:"fill"`
	Plain struct {
		Src [4]float64 `json:"src"`
		Dst [4]float64 `json:"dst"`
	} `json:"plain"`
	Tint struct {
		Src  [4]float64 `json:"src"`
		Dst  [4]float64 `json:"dst"`
		Tint [4]float64 `json:"tint"`
	} `json:"tint"`
	Probes []struct {
		X, Y int
		Want [4]uint8 `json:"want"`
		Tol  uint8    `json:"tol"`
		Desc string   `json:"desc"`
	} `json:"probes"`
}

func loadScratchWireCases(t *testing.T) scratchWireCases {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "pass_scratch_cases.json"))
	if err != nil {
		t.Fatalf("read cases: %v", err)
	}
	var c scratchWireCases
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode cases: %v", err)
	}
	return c
}

func loadScratchWireAtlas(t *testing.T, name string) *render.ImageBuf {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open atlas: %v", err)
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode atlas: %v", err)
	}
	b := src.Bounds()
	img, err := render.NewImageBuf(b.Dx(), b.Dy(), render.FormatRGBA8)
	if err != nil {
		t.Fatalf("new atlas buf: %v", err)
	}
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			if err := img.SetRGBA(x, y, uint8(r>>8), uint8(g>>8), uint8(bl>>8), uint8(a>>8)); err != nil {
				t.Fatalf("set atlas pixel: %v", err)
			}
		}
	}
	return img
}

// TestRecordLocalWith_MixedExtraPixels records a bounds layer whose extra
// callback mixes GPU fill, a GPU plain sprite and a CPU tint sprite (the
// game_particle / game_sprite black-card shape), composites one textured
// frame, and asserts the card pixels at the composited offset.
func TestRecordLocalWith_MixedExtraPixels(t *testing.T) {
	requireGPU(t)
	c := loadScratchWireCases(t)
	atlas := loadScratchWireAtlas(t, c.Atlas)

	const W, H = 120, 100
	const offX, offY = 10, 10
	dc := render.NewContext(W, H)
	defer dc.Close()

	view, release := dc.CreateOffscreenTexture(W, H)
	if release == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer release()

	cw, ch := c.Canvas[0], c.Canvas[1]
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	b.PushOffset(offX, offY)
	pl := b.AddPicture(true)
	pl.SetCacheKey(9101)
	pl.ExtraBounds = image.Rect(0, 0, cw, ch)
	pl.RasterExtra = func(dc *render.Context) {
		dc.SetRGB(c.Fill.R, c.Fill.G, c.Fill.B)
		dc.DrawRectangle(0, 0, float64(cw), float64(ch))
		_ = dc.Fill()
		_, _ = dc.DrawAtlasEx(atlas, []render.AtlasSprite{{
			SrcX: c.Plain.Src[0], SrcY: c.Plain.Src[1], SrcW: c.Plain.Src[2], SrcH: c.Plain.Src[3],
			DstX: c.Plain.Dst[0], DstY: c.Plain.Dst[1], DstW: c.Plain.Dst[2], DstH: c.Plain.Dst[3],
			Opacity: 1, Filter: render.InterpNearest,
		}}, render.AtlasDrawOptions{})
		_, _ = dc.DrawAtlasEx(atlas, []render.AtlasSprite{{
			SrcX: c.Tint.Src[0], SrcY: c.Tint.Src[1], SrcW: c.Tint.Src[2], SrcH: c.Tint.Src[3],
			DstX: c.Tint.Dst[0], DstY: c.Tint.Dst[1], DstW: c.Tint.Dst[2], DstH: c.Tint.Dst[3],
			Opacity: 1, Tint: render.RGBA{R: c.Tint.Tint[0], G: c.Tint.Tint[1], B: c.Tint.Tint[2], A: c.Tint.Tint[3]},
			Filter: render.InterpNearest,
		}}, render.AtlasDrawOptions{})
	}
	b.Pop()
	pkt := b.BuildPacket(1, 1, W, H)

	tex := scene.NewPictureTextureCache(dc, 8)
	for i := 0; i < 5; i++ {
		dc.SetRGB(0.08, 0.09, 0.11)
		dc.DrawRectangle(0, 0, W, H)
		_ = dc.Fill()
		if err := dc.PresentFrame(view, W, H, func() error { return nil }); err != nil {
			t.Fatalf("pre present %d: %v", i, err)
		}
	}

	st := scene.CompositeFramePacketTextured(pkt, dc, tex)
	if !tex.Has(9101) {
		t.Fatal("mixed extra layer not cached (must record in isolated pass)")
	}
	if st.ReplayedOps != 0 {
		t.Fatalf("frame replayed %d ops want 0 (record+blit path)", st.ReplayedOps)
	}
	if err := dc.FlushGPUWithView(view, W, H); err != nil {
		t.Fatalf("frame flush: %v", err)
	}

	rb := render.NewContext(W, H)
	defer rb.Close()
	rb.ClearWithColor(render.Black)
	rb.DrawGPUTexture(view, 0, 0, W, H)
	if err := rb.FlushGPU(); err != nil {
		t.Fatalf("readback flush: %v", err)
	}
	img := rb.Image()
	for _, p := range c.Probes {
		r, g, bl, a := img.At(offX+p.X, offY+p.Y).RGBA()
		got := [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), uint8(a >> 8)}
		for i := 0; i < 4; i++ {
			d := int(got[i]) - int(p.Want[i])
			if d < 0 {
				d = -d
			}
			if d > int(p.Tol) {
				t.Errorf("%s at (%d,%d) = %v, want %v (tol %d)", p.Desc, offX+p.X, offY+p.Y, got, p.Want, p.Tol)
				break
			}
		}
	}
}
