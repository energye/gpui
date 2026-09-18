package scene_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/scene"
)

func mustImageBuf(t *testing.T, w, h int) *render.ImageBuf {
	t.Helper()
	img, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	return img
}

func pictureWithImage(t *testing.T, img *render.ImageBuf) scene.Picture {
	t.Helper()
	return scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.DrawImage(img, 0, 0, 4, 4)
	})
}

// TestPacket_RetainsDrawImageBuffers locks D15: every ImageBuf referenced by
// display-list ops must ride the packet until drop-after-present — including
// after the tree drops it (image replace clears the node while the packet
// is still in flight) and across CloneShallow.
func TestPacket_RetainsDrawImageBuffers(t *testing.T) {
	scene.ResetLayerIDGen()
	imgA := mustImageBuf(t, 4, 4)
	imgB := mustImageBuf(t, 2, 2)
	b := scene.NewLayerBuilder()
	pl1 := b.AddPicture(true)
	pl1.SetCacheKey(401)
	pl1.Picture = pictureWithImage(t, imgA)
	pl2 := b.AddPicture(true)
	pl2.SetCacheKey(402)
	pl2.Picture = pictureWithImage(t, imgA) // duplicate ref counted once
	pl3 := b.AddPicture(true)
	pl3.SetCacheKey(403)
	pl3.Picture = pictureWithImage(t, imgB)
	pkt := b.BuildPacket(1, 1, 100, 80)
	if len(pkt.RetainedImages) != 2 {
		t.Fatalf("RetainedImages=%d want 2 (deduped)", len(pkt.RetainedImages))
	}
	seen := map[*render.ImageBuf]bool{}
	for _, img := range pkt.RetainedImages {
		seen[img] = true
	}
	if !seen[imgA] || !seen[imgB] {
		t.Fatal("retained set must hold both buffers")
	}

	// Clone carries the set (append-copy: independent slice, same buffers).
	cl := pkt.CloneShallow()
	if len(cl.RetainedImages) != 2 {
		t.Fatalf("clone retained=%d want 2", len(cl.RetainedImages))
	}
	cseen := map[*render.ImageBuf]bool{}
	for _, img := range cl.RetainedImages {
		cseen[img] = true
	}
	if !cseen[imgA] || !cseen[imgB] {
		t.Fatal("clone must carry the same buffers")
	}
	cl.RetainedImages = append(cl.RetainedImages, mustImageBuf(t, 1, 1))
	if len(pkt.RetainedImages) != 2 {
		t.Fatal("clone append must not alias the original slice")
	}

	// RetainImagesFrom is idempotent across bands (overlay attach path).
	pkt.RetainImagesFrom(pkt.Root)
	if len(pkt.RetainedImages) != 2 {
		t.Fatalf("re-retain=%d want still 2", len(pkt.RetainedImages))
	}
}

// TestPacket_NoImages_NoRetention: imageless packets pay nothing.
func TestPacket_NoImages_NoRetention(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	pl := b.AddPicture(true)
	pl.SetCacheKey(404)
	pl.Picture = scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 10, 10, 1, 0, 0, 1)
	})
	pkt := b.BuildPacket(1, 1, 100, 80)
	if len(pkt.RetainedImages) != 0 {
		t.Fatalf("RetainedImages=%d want 0", len(pkt.RetainedImages))
	}
	if cl := pkt.CloneShallow(); len(cl.RetainedImages) != 0 {
		t.Fatal("clone of imageless packet must stay empty")
	}
}
