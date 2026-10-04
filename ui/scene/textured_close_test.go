package scene_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/scene"
)

// TestPictureTextureCache_CloseReleases: Close drops every entry and drains
// the deferred release queue (no frames run after window close, so BeginFrame
// can never drain it again). Without GPU, recording degrades to no entries;
// the test locks the headless contract: empty close, double close and nil
// close are safe with Len 0. GPU-side release is verified via WR_MEMDIG=1
// close snapshots (see PresentTarget.Close).
func TestPictureTextureCache_CloseReleases(t *testing.T) {
	dc := render.NewContext(20, 20)
	defer dc.Close()
	tex := scene.NewPictureTextureCache(dc, 8)
	p := scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillRect(0, 0, 4, 4, 1, 0, 0, 1)
	})
	_, _ = tex.RecordForTest(1, &p)
	tex.Close()
	if got := tex.Len(); got != 0 {
		t.Fatalf("Len=%d after Close, want 0", got)
	}
	tex.Close() // idempotent, no panic
	if got := tex.Len(); got != 0 {
		t.Fatalf("Len=%d after second Close, want 0", got)
	}
	var nilTex *scene.PictureTextureCache
	nilTex.Close() // nil-safe, no panic
}
