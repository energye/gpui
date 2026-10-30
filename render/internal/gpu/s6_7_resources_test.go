//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
)

func TestS67_ImageCache_HitUploadBytes(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	ic := NewImageCache(device, queue)

	px := make([]byte, 16*16*4)
	for i := range px {
		px[i] = 200
	}
	cmd := &ImageDrawCommand{
		PixelData: px, GenerationID: 1001,
		ImgWidth: 16, ImgHeight: 16, ImgStride: 16 * 4,
	}
	if _, err := ic.GetOrUpload(cmd); err != nil {
		t.Fatalf("upload: %v", err)
	}
	st := ic.Stats()
	if st.Misses != 1 || st.Uploads != 1 || st.UploadBytes != 16*16*4 {
		t.Fatalf("after miss: %+v", st)
	}
	if _, err := ic.GetOrUpload(cmd); err != nil {
		t.Fatalf("hit: %v", err)
	}
	st = ic.Stats()
	if st.Hits != 1 || st.Uploads != 1 {
		t.Fatalf("after hit: %+v", st)
	}
	if st.UsedBytes != 16*16*4 {
		t.Fatalf("usedBytes=%d", st.UsedBytes)
	}
}

func TestS67_ImageCache_GenerationInvalidation(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	ic := NewImageCache(device, queue)
	px := make([]byte, 8*8*4)
	cmd := &ImageDrawCommand{PixelData: px, GenerationID: 1, ImgWidth: 8, ImgHeight: 8, ImgStride: 32}
	if _, err := ic.GetOrUpload(cmd); err != nil {
		t.Fatal(err)
	}
	// Content change → new generation
	cmd2 := &ImageDrawCommand{PixelData: px, GenerationID: 2, ImgWidth: 8, ImgHeight: 8, ImgStride: 32}
	if _, err := ic.GetOrUpload(cmd2); err != nil {
		t.Fatal(err)
	}
	st := ic.Stats()
	if st.Entries != 2 || st.Misses != 2 {
		t.Fatalf("want 2 entries after gen change: %+v", st)
	}
	// Hit gen 1 again
	if _, err := ic.GetOrUpload(cmd); err != nil {
		t.Fatal(err)
	}
	if ic.Stats().Hits < 1 {
		t.Fatal("expected hit on old gen still cached")
	}
}

func TestS67_ImageCache_ByteBudgetEvicts(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	ic := NewImageCache(device, queue)
	// Budget: 2 tiny textures max by bytes (8*8*4*2 = 512)
	ic.SetBudgets(100, 8*8*4*2)
	px := make([]byte, 8*8*4)
	for id := uint64(1); id <= 4; id++ {
		cmd := &ImageDrawCommand{PixelData: px, GenerationID: id, ImgWidth: 8, ImgHeight: 8, ImgStride: 32}
		if _, err := ic.GetOrUpload(cmd); err != nil {
			t.Fatalf("id %d: %v", id, err)
		}
	}
	st := ic.Stats()
	t.Logf("after 4 uploads under 2-tex byte budget: entries=%d used=%d evictions=%d", st.Entries, st.UsedBytes, st.Evictions)
	if st.Entries > 2 {
		t.Fatalf("byte budget should keep ≤2 entries, got %d", st.Entries)
	}
	if st.Evictions < 2 {
		t.Fatalf("expected evictions, got %d", st.Evictions)
	}
}

func TestS67_ImageCache_EphemeralReleased(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	ic := NewImageCache(device, queue)
	var retired int
	ic.SetRetireHook(func(tex hal.Texture, view hal.TextureView) {
		retired++
		if view != nil {
			view.Destroy()
		}
		if tex != nil {
			tex.Destroy()
		}
	})
	px := make([]byte, 4*4*4)
	cmd := &ImageDrawCommand{PixelData: px, GenerationID: 0, ImgWidth: 4, ImgHeight: 4, ImgStride: 16}
	if _, err := ic.GetOrUpload(cmd); err != nil {
		t.Fatal(err)
	}
	st := ic.Stats()
	if st.EphemeralUploads != 1 || st.Entries != 0 {
		t.Fatalf("ephemeral: %+v", st)
	}
	if retired != 1 {
		t.Fatalf("gen==0 must hand the pair to the retire hook, got %d calls", retired)
	}
	// Second frame ephemeral: never accumulates in the entry map.
	if _, err := ic.GetOrUpload(cmd); err != nil {
		t.Fatal(err)
	}
	if ic.Stats().Entries != 0 {
		t.Fatal("gen==0 uploads must not stay cached")
	}
	if retired != 2 {
		t.Fatalf("retire hook calls=%d want 2", retired)
	}
}

func TestS67_ImageCache_StrideStaging(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	ic := NewImageCache(device, queue)
	// Stride wider than width*4
	w, h := 5, 3
	stride := 32
	px := make([]byte, stride*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			off := y*stride + x*4
			px[off+0], px[off+1], px[off+2], px[off+3] = 10, 20, 30, 255
		}
	}
	cmd := &ImageDrawCommand{PixelData: px, GenerationID: 9, ImgWidth: w, ImgHeight: h, ImgStride: stride}
	if _, err := ic.GetOrUpload(cmd); err != nil {
		t.Fatal(err)
	}
	if ic.Stats().LastUploadBytes != int64(w*h*4) {
		t.Fatalf("upload bytes %d", ic.Stats().LastUploadBytes)
	}
}
