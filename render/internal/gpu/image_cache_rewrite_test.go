//go:build !nogpu

package gpu

import "testing"

func TestImageCache_RewriteInPlaceStableGen(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	ic := NewImageCache(device, queue)
	defer ic.Destroy()

	w, h := 16, 16
	px := make([]byte, w*h*4)
	for i := range px {
		px[i] = 10
	}
	cmd := &ImageDrawCommand{PixelData: px, GenerationID: 4242, ImgWidth: w, ImgHeight: h, ImgStride: w * 4}
	v1, err := ic.GetOrUpload(cmd)
	if err != nil || v1 == nil {
		t.Fatalf("first upload: %v", err)
	}
	st1 := ic.Stats()
	for i := range px {
		px[i] = 200
	}
	cmd.ContentDirty = true
	v2, err := ic.GetOrUpload(cmd)
	if err != nil || v2 == nil {
		t.Fatalf("rewrite: %v", err)
	}
	st2 := ic.Stats()
	if st2.Entries != st1.Entries {
		t.Fatalf("entries grew on rewrite: %d -> %d", st1.Entries, st2.Entries)
	}
	if st2.Uploads <= st1.Uploads {
		t.Fatalf("expected upload count to increase on rewrite: %d -> %d", st1.Uploads, st2.Uploads)
	}
	cmd.ContentDirty = false
	if _, err := ic.GetOrUpload(cmd); err != nil {
		t.Fatal(err)
	}
	st3 := ic.Stats()
	if st3.Entries != st2.Entries {
		t.Fatalf("entries changed on clean hit: %d -> %d", st2.Entries, st3.Entries)
	}
	if st3.Hits <= st2.Hits {
		t.Fatalf("expected hit on clean re-get")
	}
}
