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
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/engine/core"
)

type atlasS69Size struct {
	W     int  `json:"w"`
	H     int  `json:"h"`
	Valid bool `json:"valid"`
}

type atlasS69Sprite struct {
	Name   string  `json:"name"`
	W      int     `json:"w"`
	H      int     `json:"h"`
	Color  [4]int  `json:"color"`
	PivotX float64 `json:"pivotX"`
	PivotY float64 `json:"pivotY"`
	Nine   [4]int  `json:"nine"`
}

type atlasS69UV struct {
	Name    string  `json:"name"`
	X       int     `json:"x"`
	Y       int     `json:"y"`
	W       int     `json:"w"`
	H       int     `json:"h"`
	AtlasW  int     `json:"atlas_w"`
	AtlasH  int     `json:"atlas_h"`
	U       float64 `json:"u"`
	V       float64 `json:"v"`
	Rotated bool    `json:"rotated"`
	Au      float64 `json:"au"`
	Av      float64 `json:"av"`
}

type atlasS69Cases struct {
	Sizes      []atlasS69Size   `json:"sizes"`
	Sprites    []atlasS69Sprite `json:"sprites"`
	UVs        []atlasS69UV     `json:"uv_cases"`
	BlackProbe [4]int           `json:"black_probe"`
}

func loadAtlasS69Cases(t *testing.T) atlasS69Cases {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "atlas_s69_cases.json"))
	if err != nil {
		t.Fatalf("read atlas_s69_cases.json: %v", err)
	}
	var c atlasS69Cases
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode atlas_s69_cases.json: %v", err)
	}
	if len(c.Sizes) == 0 || len(c.Sprites) == 0 || len(c.UVs) == 0 {
		t.Fatal("atlas_s69_cases.json missing sizes/sprites/uvs")
	}
	return c
}

func buildAtlasS69Inputs(c atlasS69Cases) []Input {
	ins := make([]Input, len(c.Sprites))
	for i, s := range c.Sprites {
		ins[i] = Input{
			ID: core.AssetID(s.Name), W: s.W, H: s.H,
			Pixels: fillSolid(s.W, s.H, s.Color),
			PivotX: s.PivotX, PivotY: s.PivotY, Nine: s.Nine,
		}
	}
	return ins
}

// S69尺寸定死: 2048/4096两档放行, 别档拒收且说清要哪档.
func TestAtlasToolSizeGate(t *testing.T) {
	c := loadAtlasS69Cases(t)
	if ToolAtlasSizeSmall != 2048 || ToolAtlasSizeLarge != 4096 {
		t.Fatalf("tool sizes = %d/%d, want 2048/4096", ToolAtlasSizeSmall, ToolAtlasSizeLarge)
	}
	for _, s := range c.Sizes {
		err := ValidateToolAtlasSize(s.W, s.H)
		if s.Valid && err != nil {
			t.Errorf("size %dx%d: Validate=%v, want nil", s.W, s.H, err)
		}
		if !s.Valid {
			if err == nil {
				t.Errorf("size %dx%d: Validate=nil, want reject", s.W, s.H)
				continue
			}
			if core.CodeOf(err) != core.CodeInvalidArg {
				t.Errorf("size %dx%d: code=%v, want invalid-arg", s.W, s.H, core.CodeOf(err))
			}
			if !strings.Contains(err.Error(), "2048/4096") {
				t.Errorf("size %dx%d: reason %q misses 2048/4096", s.W, s.H, err.Error())
			}
		}
	}
	// Pack path gates first: bad size never reaches the packer, even with
	// nil inputs the reason still names the size档.
	if _, err := PackToolAtlas(nil, 1024, 1024, DefaultPackOptions()); err == nil {
		t.Error("PackToolAtlas 1024: want reject")
	} else if !strings.Contains(err.Error(), "2048/4096") {
		t.Errorf("PackToolAtlas 1024: reason %q misses 2048/4096", err.Error())
	}
	for _, size := range [][2]int{{2048, 2048}, {4096, 4096}} {
		a, err := PackToolAtlas(buildAtlasS69Inputs(c), size[0], size[1], DefaultPackOptions())
		if err != nil {
			t.Fatalf("PackToolAtlas %dx%d: %v", size[0], size[1], err)
		}
		if a.Count() != len(c.Sprites) {
			t.Errorf("PackToolAtlas %dx%d: count=%d, want %d", size[0], size[1], a.Count(), len(c.Sprites))
		}
		if a.Width() > size[0] || a.Height() > size[1] {
			t.Errorf("PackToolAtlas %dx%d: sheet %dx%d over budget", size[0], size[1], a.Width(), a.Height())
		}
	}
}

// S69旋转反算: 工具入口复用S65数学, 正反算对上冻结数.
func TestAtlasToolRotatedInverse(t *testing.T) {
	c := loadAtlasS69Cases(t)
	const eps = 1e-9
	for _, u := range c.UVs {
		e := Entry{name: core.AssetID(u.Name), x: u.X, y: u.Y, w: u.W, h: u.H}
		gu, gv := SpriteToAtlasUV(e, u.AtlasW, u.AtlasH, u.U, u.V, u.Rotated)
		if math.Abs(gu-u.Au) > eps || math.Abs(gv-u.Av) > eps {
			t.Errorf("%s: S65 forward=(%v,%v) want (%v,%v)", u.Name, gu, gv, u.Au, u.Av)
		}
		ru, rv, err := ToolAtlasToSpriteUV(e, u.AtlasW, u.AtlasH, u.Au, u.Av, u.Rotated)
		if err != nil {
			t.Errorf("%s: tool inverse=%v, want nil", u.Name, err)
			continue
		}
		if math.Abs(ru-u.U) > eps || math.Abs(rv-u.V) > eps {
			t.Errorf("%s: tool inverse=(%v,%v) want (%v,%v)", u.Name, ru, rv, u.U, u.V)
		}
	}
	bad := Entry{name: "s69/bad", x: 100, y: 200, w: 64, h: 32}
	for _, tc := range []struct {
		name   string
		entry  Entry
		atlasW int
		atlasH int
		au, av float64
	}{
		{"zero atlas", bad, 0, 2048, 0.05, 0.1},
		{"huge atlas", bad, 5000, 2048, 0.05, 0.1},
		{"uv out", bad, 2048, 2048, 2, 0.1},
		{"uv nan", bad, 2048, 2048, math.NaN(), 0.1},
		{"rect outside", Entry{name: "s69/out", x: 2000, y: 2000, w: 64, h: 32}, 2048, 2048, 0.05, 0.1},
		{"empty entry", Entry{}, 2048, 2048, 0.05, 0.1},
	} {
		if _, _, err := ToolAtlasToSpriteUV(tc.entry, tc.atlasW, tc.atlasH, tc.au, tc.av, true); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("%s: code=%v, want invalid-arg", tc.name, core.CodeOf(err))
		}
	}
}

// S69描边黑线: 干净图集零报, 脏像素报出坐标与归属.
func TestAtlasToolStrokeBlackLine(t *testing.T) {
	c := loadAtlasS69Cases(t)
	def := DefaultPackOptions()
	a, err := PackToolAtlas(buildAtlasS69Inputs(c), 2048, 2048, def)
	if err != nil {
		t.Fatalf("PackToolAtlas: %v", err)
	}
	if spots := CheckStrokeBlackLine(a, def.Extrude); len(spots) != 0 {
		t.Fatalf("clean sheet spots=%d, want 0", len(spots))
	}
	probe := c.BlackProbe
	// Leftmost entry owns a clear gutter to its left: its left bleed
	// pixel carries the edge color (opaque, not black). Painting black
	// over it simulates one串色 stroke pixel the scan must locate.
	left, ok := a.Entry(0)
	if !ok {
		t.Fatal("Entry(0) ok=false")
	}
	for i := 1; i < a.Count(); i++ {
		e, _ := a.Entry(i)
		if e.X() < left.X() {
			left = e
		}
	}
	px, py := left.X()-1, left.Y()
	if r, g, b, al, ok := a.At(px, py); !ok || al == 0 || (r == 0 && g == 0 && b == 0) {
		t.Fatalf("ring (%d,%d)=%d,%d,%d,%d, want opaque non-black bleed bed", px, py, r, g, b, al)
	}
	off := (py*a.Width() + px) * 4
	a.pixels[off] = byte(probe[0])
	a.pixels[off+1] = byte(probe[1])
	a.pixels[off+2] = byte(probe[2])
	a.pixels[off+3] = byte(probe[3])
	spots := CheckStrokeBlackLine(a, def.Extrude)
	found := false
	for _, s := range spots {
		if s.X == px && s.Y == py {
			found = true
			if s.Sprite != left.Name() {
				t.Errorf("ring spot sprite=%q, want %q", s.Sprite, left.Name())
			}
			if int(s.R) != probe[0] || int(s.G) != probe[1] || int(s.B) != probe[2] || int(s.A) != probe[3] {
				t.Errorf("ring spot color=%d,%d,%d,%d, want %v", s.R, s.G, s.B, s.A, probe)
			}
		}
	}
	if !found {
		t.Fatalf("ring (%d,%d) missing from %d spots", px, py, len(spots))
	}
	t.Logf("stroke-probe: ring (%d,%d) sprite=%q spots=%d", px, py, left.Name(), len(spots))
	// Sheet corner lives in the gutter: black there is border串色 with
	// no sprite owner.
	if r, g, b, al, ok := a.At(0, 0); !ok || r != 0 || g != 0 || b != 0 || al != 0 {
		t.Fatalf("corner (0,0)=%d,%d,%d,%d, want transparent probe bed", r, g, b, al)
	}
	a.pixels[3] = byte(probe[3])
	spots = CheckStrokeBlackLine(a, def.Extrude)
	found = false
	for _, s := range spots {
		if s.X == 0 && s.Y == 0 {
			found = true
			if !s.Sprite.Empty() {
				t.Errorf("corner spot sprite=%q, want empty", s.Sprite)
			}
		}
	}
	if !found {
		t.Fatalf("corner (0,0) missing from %d spots", len(spots))
	}
	// Nil and parsed atlases carry no pixels: nothing to flag.
	var nilA *Atlas
	if spots := CheckStrokeBlackLine(nilA, def.Extrude); len(spots) != 0 {
		t.Errorf("nil atlas spots=%d, want 0", len(spots))
	}
	parsed, err := Load(filepath.Join("testdata", "atlas_small.json"))
	if err != nil {
		t.Fatalf("Load atlas_small.json: %v", err)
	}
	if spots := CheckStrokeBlackLine(parsed, def.Extrude); len(spots) != 0 {
		t.Errorf("parsed atlas spots=%d, want 0", len(spots))
	}
}
