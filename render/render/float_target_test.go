//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/gpu/types"
)

// A: construct + validate (bad sizes, over budget, bad texels never land).
func TestFloatTargetConstruct(t *testing.T) {
	if _, err := NewFloatTarget(0, 4); err == nil {
		t.Fatal("zero width must fail")
	}
	if _, err := NewFloatTarget(-1, 4); err == nil {
		t.Fatal("negative must fail")
	}
	if _, err := NewFloatTarget(9000, 9000); err == nil {
		t.Fatal("over budget must fail")
	}
	tg, err := NewFloatTarget(2, 2)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if tg.Format() != types.TextureFormatRGBA16Float {
		t.Fatalf("format = %v, want RGBA16Float", tg.Format())
	}
	if tg.Pixels() != nil {
		t.Fatal("Pixels must be nil so the 8-bit road declines")
	}
	if !tg.Set(0, 0, [4]float32{2, 0, 0, 1}) {
		t.Fatal("finite set must land")
	}
	if tg.Set(1, 0, [4]float32{float32(math.Inf(1)), 0, 0, 1}) {
		t.Fatal("inf set must refuse")
	}
	if tg.Set(9, 9, [4]float32{1, 1, 1, 1}) {
		t.Fatal("out of range set must refuse")
	}
	if !tg.ClearFloat([4]float32{0.5, 0.5, 0.5, 1}) {
		t.Fatal("finite clear must land")
	}
	if tg.ClearFloat([4]float32{1, 1, 1, 1}); false {
		t.Fatal("unreachable")
	}
}

// B: software renderer declines the float branch (old road untouched).
func TestFloatTargetSoftwareDeclines(t *testing.T) {
	tg, err := NewFloatTarget(4, 4)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	sr := NewSoftwareRenderer()
	if err := sr.Render(tg, NewScene()); err == nil {
		t.Fatal("software must decline float target")
	}
	// The 8-bit road still renders fine.
	px := NewPixmapTarget(4, 4)
	if err := sr.Render(px, NewScene()); err != nil {
		t.Fatalf("8-bit road must still work: %v", err)
	}
}

// C: encode golden from the frozen file (screenshot-vs-design language).
func TestFloatTargetEncodeGolden(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "float_encode.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var f struct {
		Exposure float64      `json:"exposure"`
		Texels   [][4]float32 `json:"texels"`
		WantRGB  [][3]uint8   `json:"want_rgb"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode golden: %v", err)
	}
	if len(f.Texels) != 4 || len(f.WantRGB) != 4 {
		t.Fatalf("golden wants 4 texels, got %d/%d", len(f.Texels), len(f.WantRGB))
	}
	tg, err := NewFloatTarget(2, 2)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	for i, c := range f.Texels {
		if !tg.Set(i%2, i/2, c) {
			t.Fatalf("set %d must land", i)
		}
	}
	img, err := tg.Encode(f.Exposure)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	for i, w := range f.WantRGB {
		oi := (i/2)*img.Stride + (i%2)*4
		if img.Pix[oi] != w[0] || img.Pix[oi+1] != w[1] || img.Pix[oi+2] != w[2] {
			t.Fatalf("texel %d = (%d,%d,%d), want (%d,%d,%d)",
				i, img.Pix[oi], img.Pix[oi+1], img.Pix[oi+2], w[0], w[1], w[2])
		}
	}
	if _, err := tg.Encode(0); err == nil {
		t.Fatal("zero exposure must fail")
	}
	if _, err := tg.Encode(99); err == nil {
		t.Fatal("over-cap exposure must fail")
	}
}

// D: double white explains itself (HDR vs 8-bit halo, not a white sheet).
func TestFloatTargetHDRVs8Bit(t *testing.T) {
	tg, err := NewFloatTarget(1, 1)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !tg.Set(0, 0, [4]float32{2, 2, 2, 1}) {
		t.Fatal("set must land")
	}
	img, err := tg.Encode(1)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	// 2x white rolls to 213, not 255: halo visible, never blown to sheet.
	if img.Pix[0] != 213 || img.Pix[1] != 213 || img.Pix[2] != 213 {
		t.Fatalf("2x white = (%d,%d,%d), want (213,213,213)", img.Pix[0], img.Pix[1], img.Pix[2])
	}
}
