//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package asset

import (
	"fmt"
	"math"
	"sort"

	"github.com/energye/gpui/engine/core"
)

// S69 tool atlas budgets. The offline tool only takes these two sheet
// sizes; anything else is rejected with the got size in the reason so
// artists can fix the export instead of guessing.
const (
	// ToolAtlasSizeSmall is the 2048 tool sheet side.
	ToolAtlasSizeSmall = 2048
	// ToolAtlasSizeLarge is the 4096 tool sheet side.
	ToolAtlasSizeLarge = 4096
)

func toolSizeOK(v int) bool {
	return v == ToolAtlasSizeSmall || v == ToolAtlasSizeLarge
}

// ValidateToolAtlasSize gates one tool sheet: both sides must be
// 2048 or 4096. Anything else reports InvalidArg naming the got size.
func ValidateToolAtlasSize(w, h int) error {
	const op = "atlastool.ValidateToolAtlasSize"
	if toolSizeOK(w) && toolSizeOK(h) {
		return nil
	}
	return core.InvalidArg(op, fmt.Sprintf("size %dx%d, want 2048/4096", w, h))
}

// PackToolAtlas packs inputs under the frozen tool budget: w and h each
// must be 2048 or 4096, then it reuses the S65 PackWithOptions seam
// (gutter plus extrude bleed) unchanged. The sheet stays trimmed to the
// content; the gate only pins the budget, never the output size.
func PackToolAtlas(inputs []Input, w, h int, opts PackOptions) (*Atlas, error) {
	const op = "atlastool.PackToolAtlas"
	if !toolSizeOK(w) || !toolSizeOK(h) {
		return nil, core.InvalidArg(op, fmt.Sprintf("size %dx%d, want 2048/4096", w, h))
	}
	return PackWithOptions(inputs, w, h, opts)
}

// ToolAtlasToSpriteUV is the tool-side rotated inverse: it validates the
// call, then reuses the S65 AtlasToSpriteUV math unchanged. Atlas u,v
// must sit in 0..1 and the entry rect must sit inside the sheet.
func ToolAtlasToSpriteUV(e Entry, atlasW, atlasH int, au, av float64, rotated bool) (float64, float64, error) {
	const op = "atlastool.AtlasToSpriteUV"
	if atlasW < 1 || atlasH < 1 || atlasW > MaxAtlasSize || atlasH > MaxAtlasSize {
		return 0, 0, core.InvalidArg(op, fmt.Sprintf("atlas %dx%d, want 1..4096", atlasW, atlasH))
	}
	if e.w < 1 || e.h < 1 {
		return 0, 0, core.InvalidArg(op, fmt.Sprintf("entry %q size %dx%d, want >=1", string(e.name), e.w, e.h))
	}
	if e.x < 0 || e.y < 0 || e.x+e.w > atlasW || e.y+e.h > atlasH {
		return 0, 0, core.InvalidArg(op, fmt.Sprintf("entry %q rect %d,%d %dx%d outside %dx%d",
			string(e.name), e.x, e.y, e.w, e.h, atlasW, atlasH))
	}
	if math.IsNaN(au) || math.IsInf(au, 0) || math.IsNaN(av) || math.IsInf(av, 0) ||
		au < 0 || au > 1 || av < 0 || av > 1 {
		return 0, 0, core.InvalidArg(op, fmt.Sprintf("uv %v,%v, want 0..1", au, av))
	}
	u, v := AtlasToSpriteUV(e, atlasW, atlasH, au, av, rotated)
	return u, v, nil
}

// EdgeBleed names one leaked black pixel: sheet coords, its color, and
// the sprite whose bleed ring owns it. Empty Sprite means the pixel was
// caught on the sheet border, outside every ring.
type EdgeBleed struct {
	X, Y       int
	R, G, B, A uint8
	Sprite     core.AssetID
}

func bleedBlack(pix []byte, off int) bool {
	return pix[off+3] != 0 && pix[off] == 0 && pix[off+1] == 0 && pix[off+2] == 0
}

func bleedInRing(e Entry, xx, yy, extrude int) bool {
	return xx >= e.x-extrude && xx < e.x+e.w+extrude &&
		yy >= e.y-extrude && yy < e.y+e.h+extrude
}

// CheckStrokeBlackLine scans for outline black lines: opaque black
// pixels (0,0,0 with alpha, tolerance frozen at 0) inside a sprite bleed
// ring or on the sheet border. Interior sprite pixels are the art itself
// and are never flagged. Clean sheets report nil; every hit carries its
// sheet position so the card can be located. Nil or parsed atlases
// (no pixels) report nil.
func CheckStrokeBlackLine(a *Atlas, extrude int) []EdgeBleed {
	if a == nil || len(a.pixels) == 0 {
		return nil
	}
	if extrude < 0 {
		extrude = 0
	}
	var out []EdgeBleed
	at := func(xx, yy int) (uint8, uint8, uint8, uint8, bool) {
		if xx < 0 || yy < 0 || xx >= a.width || yy >= a.height {
			return 0, 0, 0, 0, false
		}
		off := (yy*a.width + xx) * 4
		p := a.pixels
		return p[off], p[off+1], p[off+2], p[off+3], true
	}
	for _, e := range a.entries {
		for yy := e.y - extrude; yy < e.y+e.h+extrude; yy++ {
			for xx := e.x - extrude; xx < e.x+e.w+extrude; xx++ {
				if xx >= e.x && xx < e.x+e.w && yy >= e.y && yy < e.y+e.h {
					continue
				}
				r, g, b, al, ok := at(xx, yy)
				if !ok {
					continue
				}
				off := (yy*a.width + xx) * 4
				if bleedBlack(a.pixels, off) {
					out = append(out, EdgeBleed{X: xx, Y: yy, R: r, G: g, B: b, A: al, Sprite: e.name})
				}
			}
		}
	}
	border := func(xx, yy int) {
		r, g, b, al, ok := at(xx, yy)
		if !ok {
			return
		}
		for _, e := range a.entries {
			if bleedInRing(e, xx, yy, extrude) {
				return
			}
		}
		off := (yy*a.width + xx) * 4
		if bleedBlack(a.pixels, off) {
			out = append(out, EdgeBleed{X: xx, Y: yy, R: r, G: g, B: b, A: al})
		}
	}
	for x := 0; x < a.width; x++ {
		border(x, 0)
		border(x, a.height-1)
	}
	for y := 1; y < a.height-1; y++ {
		border(0, y)
		border(a.width-1, y)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].X < out[j].X
	})
	return out
}
