//go:build !nogpu

package gpu

import (
	"errors"
	"image"
	"image/color"
	"testing"
)

func solidRGBA(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestColorAtlasPutGet(t *testing.T) {
	atlas := NewColorGlyphAtlas(256, 2)
	key := ColorGlyphKey{FontID: 1, GlyphID: 65, PPEM: 16}
	region, err := atlas.Put(key, solidRGBA(8, 8, color.RGBA{255, 0, 0, 255}), 0, 8)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if region.Width != 8 || region.Height != 8 || region.Page != 0 {
		t.Fatalf("region = %+v, want 8x8 page 0", region)
	}
	got, ok := atlas.Get(key)
	if !ok {
		t.Fatal("Get missed after Put")
	}
	if got != region {
		t.Fatalf("Get = %+v, want %+v", got, region)
	}
	data, size := atlas.PageRGBAData(0)
	if size != 256 || len(data) != 256*256*4 {
		t.Fatalf("page data len = %d, want %d", len(data), 256*256*4)
	}
	// Top-left pixel of the 8x8 block must be the stored red.
	if !(data[0] == 255 && data[1] == 0 && data[2] == 0 && data[3] == 255) {
		t.Fatalf("pixel(0,0) = %v %v %v %v, want 255 0 0 255", data[0], data[1], data[2], data[3])
	}
}

func TestColorAtlasDirtyUpload(t *testing.T) {
	atlas := NewColorGlyphAtlas(256, 2)
	if len(atlas.DirtyUploads()) != 0 {
		t.Fatal("fresh atlas has dirty uploads")
	}
	key := ColorGlyphKey{FontID: 1, GlyphID: 65, PPEM: 16}
	if _, err := atlas.Put(key, solidRGBA(8, 8, color.RGBA{0, 255, 0, 255}), 0, 8); err != nil {
		t.Fatalf("Put: %v", err)
	}
	uploads := atlas.DirtyUploads()
	if len(uploads) != 1 || uploads[0].Index != 0 {
		t.Fatalf("uploads = %+v, want one for page 0", uploads)
	}
	atlas.MarkClean(0)
	if len(atlas.DirtyUploads()) != 0 {
		t.Fatal("uploads remain after MarkClean")
	}
}

func TestColorAtlasFull(t *testing.T) {
	atlas := NewColorGlyphAtlas(256, 1)
	placed := 0
	var fullErr error
	for i := 0; i < 100; i++ {
		key := ColorGlyphKey{FontID: 1, GlyphID: uint16(i), PPEM: 16}
		if _, err := atlas.Put(key, solidRGBA(30, 30, color.RGBA{R: 0, G: 0, B: 255, A: 255}), 0, 30); err != nil {
			fullErr = err
			break
		}
		placed++
	}
	if fullErr == nil {
		t.Fatal("100x30x30 in single 256 page = nil error, want bounded refusal")
	}
	if !errors.Is(fullErr, ErrColorAtlasFull) {
		t.Fatalf("err = %v, want ErrColorAtlasFull", fullErr)
	}
	if placed == 0 {
		t.Fatal("no placement succeeded before full")
	}
}
