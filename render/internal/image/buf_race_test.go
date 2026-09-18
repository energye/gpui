package image

import (
	"sync"
	"testing"
)

// TestImageBuf_DisposeRacesRasterReads locks the D7/D15 thread-safety fix
// (post-T2 the raster thread reads buffers while the UI thread may Dispose
// them on image replace/clear): concurrent Dispose vs Bounds/Data/
// GenerationID/TakeGPUDirty/PremultipliedData must be race-clean. Run with
// -race. Red on the pre-fix plain fields.
func TestImageBuf_DisposeRacesRasterReads(t *testing.T) {
	b, err := NewImageBuf(16, 12, FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SetRGBA(0, 0, 255, 0, 0, 255); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_, _ = b.Bounds()
				_ = b.Data()
				_ = b.GenerationID()
				_ = b.TakeGPUDirty()
				_ = b.PremultipliedData()
				_ = b.Disposed()
				_ = b.Width()
			}
		}()
	}
	for i := 0; i < 50; i++ {
		nb, err := NewImageBuf(16, 12, FormatRGBA8)
		if err != nil {
			t.Fatal(err)
		}
		nb.Dispose()
	}
	b.Dispose()
	wg.Wait()
	if !b.Disposed() {
		t.Fatal("must report disposed")
	}
	if w, h := b.Bounds(); w != 0 || h != 0 {
		t.Fatalf("disposed bounds=(%d,%d) want (0,0)", w, h)
	}
}
