package compare

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"
)

func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestImages_IdenticalPass(t *testing.T) {
	a := solid(16, 16, color.RGBA{10, 20, 30, 255})
	b := solid(16, 16, color.RGBA{10, 20, 30, 255})
	res, err := Images(a, b, DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Pass {
		t.Fatalf("expected pass, got %s stats=%+v", res.Reason, res.Stats)
	}
}

func TestImages_LargeDiffFail(t *testing.T) {
	a := solid(16, 16, color.RGBA{255, 255, 255, 255})
	b := solid(16, 16, color.RGBA{0, 0, 0, 255})
	res, err := Images(a, b, DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if res.Pass {
		t.Fatalf("expected fail on white vs black")
	}
}

func TestFiles_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	a := solid(8, 8, color.RGBA{40, 50, 60, 255})
	b := solid(8, 8, color.RGBA{40, 50, 61, 255}) // tiny delta
	ep := filepath.Join(dir, "e.png")
	ap := filepath.Join(dir, "a.png")
	if err := WritePNG(ep, a); err != nil {
		t.Fatal(err)
	}
	if err := WritePNG(ap, b); err != nil {
		t.Fatal(err)
	}
	pol := DefaultPolicy()
	pol.MaxMeanAbs = 1.0
	pol.MaxRMSE = 2.0
	pol.ChangedRatioMax = 1.0
	res, err := Files(ep, ap, pol)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Pass {
		t.Fatalf("tiny delta should pass: %s %+v", res.Reason, res.Stats)
	}
}
