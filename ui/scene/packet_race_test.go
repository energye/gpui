package scene

import (
	"sync"
	"testing"

	"github.com/energye/gpui/render"
)

// TestPacket_ImageDisposeRacesComposite locks the R3-1 true-path carrier
// for the D11/D15 class (UI-side release vs raster-side read through a
// handed-off packet). Production shape: exactly ONE raster composite at a
// time (concurrent composites of one packet never happen — NeedsRaster and
// entry clocks are single-writer by design); the UI thread may Dispose the
// buffer mid-flight. Run with -race. Red on the pre-fix plain fields
// (Dispose's flag/dims/data vs raster reads); many rounds make overlap
// near-certain for the red proof while the fix stays green always.
func TestPacket_ImageDisposeRacesComposite(t *testing.T) {
	for round := 0; round < 20; round++ {
		ResetLayerIDGen()
		img, err := render.NewImageBuf(16, 12, render.FormatRGBA8)
		if err != nil {
			t.Fatal(err)
		}
		if err := img.SetRGBA(0, 0, 255, 0, 0, 255); err != nil {
			t.Fatal(err)
		}
		b := NewLayerBuilder()
		pl := b.AddPicture(true)
		pl.SetCacheKey(501)
		pl.Picture = RecordPicture(func(r *PictureRecorder) {
			r.DrawImage(img, 0, 0, 16, 12)
		})
		pkt := b.BuildPacket(1, 1, 100, 80)
		if len(pkt.RetainedImages) != 1 {
			t.Fatalf("round %d: RetainedImages=%d want 1", round, len(pkt.RetainedImages))
		}

		dc := render.NewContext(100, 80)
		dc.BeginFrame()
		tex := NewPictureTextureCache(dc, 0) // no GPU → vector fallback reads buf

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				CompositeFramePacketTextured(pkt, dc, tex)
			}
		}()
		img.Dispose()
		_ = img.Disposed()
		wg.Wait()
		_ = dc.Close()
		if !img.Disposed() {
			t.Fatal("must report disposed")
		}
	}
}
