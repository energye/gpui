//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package conformance

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Comparator unit test: the same fixture image must pass against pass.json
// and must go red against each single-item mutation. No GPU is involved.
func TestComparatorPassAndMutations(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "conformance", "comparator")
	img := loadPNG(t, filepath.Join(dir, "pixels.png"))

	pass := loadCase(t, filepath.Join(dir, "pass.json"))
	rep := Evaluate("", pass, img, PathStats{}, nil)
	if !rep.OK {
		t.Fatalf("pass fixture went red: %v", rep.Failed())
	}
	if len(rep.Samples) != 2 || len(rep.Regions) != 2 {
		t.Fatalf("pass fixture evaluated %d samples / %d regions, want 2/2", len(rep.Samples), len(rep.Regions))
	}

	mut := loadCase(t, filepath.Join(dir, "mutated.json"))
	repMut := Evaluate("", mut, img, PathStats{}, nil)
	if repMut.OK {
		t.Fatal("mutated sample expectation stayed green")
	}
	failed := repMut.Failed()
	if len(failed) != 1 || !strings.Contains(failed[0], "sample left_pixel") {
		t.Fatalf("mutated sample failures = %v, want exactly the left_pixel sample", failed)
	}

	mutRegion := loadCase(t, filepath.Join(dir, "region_mutated.json"))
	repRegion := Evaluate("", mutRegion, img, PathStats{}, nil)
	if repRegion.OK {
		t.Fatal("mutated region expectation stayed green")
	}
	failed = repRegion.Failed()
	if len(failed) != 1 || !strings.Contains(failed[0], "region left_half") {
		t.Fatalf("mutated region failures = %v, want exactly the left_half region", failed)
	}
}

// The harness reads premultiplied pixels from image.Image and compares
// straight RGBA; half-alpha pixels must round-trip through the
// unpremultiply step.
func TestStraightPixelUnpremultiplies(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.NRGBA{R: 128, G: 64, B: 32, A: 128})
	got := straightPixel(img, 0, 0)
	want := [4]uint8{128, 64, 32, 128}
	for i := 0; i < 4; i++ {
		d := int(got[i]) - int(want[i])
		if d < 0 {
			d = -d
		}
		if d > 1 {
			t.Fatalf("straightPixel = %v, want %v ±1", got, want)
		}
	}
}

func loadPNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return img
}

func loadCase(t *testing.T, path string) *Case {
	t.Helper()
	c, err := LoadCase(path)
	if err != nil {
		t.Fatalf("LoadCase %s: %v", path, err)
	}
	return c
}
