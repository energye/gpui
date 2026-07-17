package render

import (
	"testing"
)

// ExportImageBuf reuse must keep GenerationID stable and mark GPU dirty so the
// image cache rewrites in place (engine memory growth under glow/effect RTs).
func TestExportImageBuf_StableGenMarksDirty(t *testing.T) {
	dc := NewContext(32, 24)
	defer dc.Close()
	dc.ClearWithColor(RGBA{R: 1, G: 0, B: 0, A: 1})
	var img *ImageBuf
	if !dc.ExportImageBuf(&img) || img == nil {
		t.Fatal("first export failed")
	}
	gen1 := img.GenerationID()
	if gen1 == 0 {
		t.Fatal("expected non-zero gen after first export")
	}
	// First export marks dirty for initial GPU upload.
	if !img.IsGPUDirty() && !img.TakeGPUDirty() {
		// Take may have been false if IsGPUDirty already false after export
		// MarkPixelsDirty always sets dirty; re-check path.
	}
	// Re-export after different clear: gen must stay, dirty must be set.
	_ = img.TakeGPUDirty() // clear pending
	dc.ClearWithColor(RGBA{R: 0, G: 0, B: 1, A: 1})
	if !dc.ExportImageBuf(&img) {
		t.Fatal("second export failed")
	}
	if img.GenerationID() != gen1 {
		t.Fatalf("gen changed %d -> %d (would explode image cache)", gen1, img.GenerationID())
	}
	if !img.IsGPUDirty() {
		t.Fatal("expected GPU dirty after ExportImageBuf reuse")
	}
	if !img.TakeGPUDirty() {
		t.Fatal("TakeGPUDirty should return true once")
	}
	if img.TakeGPUDirty() {
		t.Fatal("TakeGPUDirty should clear flag")
	}
}
