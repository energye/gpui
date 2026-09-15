// Custom case helpers for game_fx (capability 5.5, P3, S44/W6).
//
// Frozen look: transparent sheet, steel-blue block, white chip. Outline
// rings the block red, dissolve eats it with an orange edge. Only frozen
// game/fx constructors are called here; render is only fed buffers.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"github.com/energye/gpui/game/core"
	"github.com/energye/gpui/game/fx"
)

// Frozen custom-case params (review visible, never silent).
const (
	customOutlineWidth     = 1.0
	customOutlineThreshold = 0.5
	customOutlineR         = 1.0
	customOutlineG         = 0.0
	customOutlineB         = 0.0
	customDissolveAmount   = 0.5
	customDissolveEdge     = 0.1
	customDissolveSeed     = 1.0
	customDissolveR        = 1.0
	customDissolveG        = 0.5
	customDissolveB        = 0.0
	customOutlineMinPx     = 20 // outline must paint at least this many px
	customOutlineBar       = 0.1
)

// customDissolveCycle animates the window dissolve card (golden stays at
// customDissolveAmount).
var customDissolveCycle = []float64{0.2, 0.35, 0.5, 0.65, 0.8}

// customEdgeColor is the frozen dissolve edge (must match NewDissolve args).
func customEdgeColor() core.Color {
	return core.RGBA(customDissolveR, customDissolveG, customDissolveB, 1)
}

// makeCustomSrcColors builds the deterministic sprite-like sheet:
// transparent background, opaque steel-blue block, white chip inside.
func makeCustomSrcColors(w, h int) []core.Color {
	out := make([]core.Color, w*h)
	for y := h / 4; y < h*3/4; y++ {
		for x := w * 3 / 10; x < w*7/10; x++ {
			out[y*w+x] = core.RGBA(0.2, 0.4, 0.8, 1)
		}
	}
	for y := h * 4 / 10; y < h*6/10; y++ {
		for x := w * 4 / 10; x < w*6/10; x++ {
			out[y*w+x] = core.RGBA(0.9, 0.9, 0.9, 1)
		}
	}
	return out
}

// mustOutline builds the frozen red ring hook.
func mustOutline() fx.Custom {
	c, err := fx.NewOutline(customOutlineWidth, customOutlineThreshold,
		customOutlineR, customOutlineG, customOutlineB)
	if err != nil {
		panic(fmt.Sprintf("game_fx: NewOutline: %v", err))
	}
	return c
}

// mustDissolve builds the frozen dissolve hook at the golden amount.
func mustDissolve() fx.Custom {
	return mustDissolveAt(customDissolveAmount)
}

// mustDissolveAt builds the frozen dissolve hook at one cycle amount.
func mustDissolveAt(amount float64) fx.Custom {
	c, err := fx.NewDissolve(amount, customDissolveEdge, customDissolveSeed,
		customDissolveR, customDissolveG, customDissolveB)
	if err != nil {
		panic(fmt.Sprintf("game_fx: NewDissolve: %v", err))
	}
	return c
}

// runCustomLogicProbes checks identity exactness, degraded marks, the
// attach/call/detach wiring, and the bad-shader placeholder contract.
func runCustomLogicProbes(ol, dis fx.Custom) (map[string]any, bool) {
	out := map[string]any{}
	ok := true
	id := fx.NewIdentity()
	idOK := id.IsIdentity() && !id.Degraded()
	out["custom_identity_exact"] = idOK
	if !idOK {
		ok = false
	}
	olOK := !ol.IsIdentity() && ol.Degraded()
	out["custom_outline_marked"] = olOK
	if !olOK {
		ok = false
	}
	disOK := !dis.IsIdentity() && dis.Degraded()
	out["custom_dissolve_marked"] = disOK
	if !disOK {
		ok = false
	}
	reg := fx.NewRegistry()
	regOK := true
	hid, err := reg.Attach(ol)
	if err != nil {
		regOK = false
	}
	got, found := reg.Get(hid)
	if !found || got.Name != fx.CustomOutline {
		regOK = false
	}
	tiny, err := fx.NewImage(2, 2)
	if err != nil {
		regOK = false
	} else {
		tiny.Pix[0] = core.RGB(0.2, 0.4, 0.8)
		if _, _, err := reg.Apply(hid, tiny); err != nil {
			regOK = false
		}
		if err := reg.Detach(hid); err != nil || reg.Len() != 0 {
			regOK = false
		}
	}
	out["custom_registry_ok"] = regOK
	if !regOK {
		ok = false
	}
	glitch := fx.Custom{Name: "glitch-xyz", Params: map[string]float64{}}
	holder, deg, err := glitch.Apply(tiny)
	badOK := core.CodeOf(err) == core.CodeUnsupported && deg &&
		holder != nil && holder.ApproxEqual(tiny, 0)
	out["custom_badshader_held"] = badOK
	if !badOK {
		ok = false
	}
	out["custom_probe_ok"] = ok
	return out, ok
}

// runCustomPixelAssertions checks the outline ring exists without touching
// the interior, and the dissolve mixes kept/edge/gone.
func runCustomPixelAssertions(src, outlined, dissolved *fx.Image) (outlinePx, kept, edged, gone int, ok bool) {
	for i := range src.Pix {
		if src.Pix[i] != outlined.Pix[i] {
			outlinePx++
		}
	}
	cx, cy := src.W/2, src.H/2
	centerKept := outlined.Pix[cy*src.W+cx] == src.Pix[cy*src.W+cx]
	edge := customEdgeColor()
	for i, p := range dissolved.Pix {
		switch {
		case p.A == 0:
			gone++
		case p == edge:
			edged++
		case p == src.Pix[i]:
			kept++
		}
	}
	ok = outlinePx >= customOutlineMinPx && centerKept &&
		kept > 0 && edged > 0 && gone > 0
	return outlinePx, kept, edged, gone, ok
}

// checkCustomOffscreenGolden compares the small outline+dissolve composite
// to the frozen PNG, zero tolerance (same rule as the grade golden).
func checkCustomOffscreenGolden(outlined, dissolved *fx.Image) (diffPct float64, totalPx int64, firstRun bool, ok bool) {
	_ = os.MkdirAll(testdataDir, 0o755)
	basePath := filepath.Join(testdataDir, "fx_custom_golden.png")
	cw, ch := outlined.W+dissolved.W, outlined.H
	if dissolved.H > ch {
		ch = dissolved.H
	}
	cur := image.NewRGBA(image.Rect(0, 0, cw, ch))
	blit := func(img *fx.Image, ox int) {
		for y := 0; y < img.H; y++ {
			for x := 0; x < img.W; x++ {
				r, g, b, a := img.Pix[y*img.W+x].ToBytes()
				cur.Set(x+ox, y, color.RGBA{R: r, G: g, B: b, A: a})
			}
		}
	}
	blit(outlined, 0)
	blit(dissolved, outlined.W)
	if _, err := os.Stat(basePath); err != nil {
		f, err := os.Create(basePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "game_fx: custom golden store %s: %v\n", basePath, err)
			return 100, 0, false, false
		}
		_ = png.Encode(f, cur)
		_ = f.Close()
		fmt.Fprintf(os.Stderr, "game_fx: custom golden baseline stored: %s\n", basePath)
		return 0, 0, true, true
	}
	f, err := os.Open(basePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "game_fx: custom golden open: %v\n", err)
		return 100, 0, false, false
	}
	want, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "game_fx: custom golden decode: %v\n", err)
		return 100, 0, false, false
	}
	if !want.Bounds().Eq(cur.Bounds()) {
		fmt.Fprintf(os.Stderr, "game_fx: custom golden size %v vs %v\n", want.Bounds(), cur.Bounds())
		return 100, int64(cw * ch), false, false
	}
	var diff int64
	total := int64(cw * ch)
	for y := 0; y < ch; y++ {
		for x := 0; x < cw; x++ {
			ar, ag, ab, aa := cur.At(x, y).RGBA()
			br, bg, bb, ba := want.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				diff++
			}
		}
	}
	if total > 0 {
		diffPct = 100 * float64(diff) / float64(total)
	}
	ok = diff == 0
	return diffPct, total, false, ok
}
