package rendering_test

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// A custom leaf that paints through RenderObject.Paint (not one of the
// capturable fast-path types) must still be recorded into the retained layer
// tree. An empty recording has no bounds/damage: the retained present then
// plans idle, refuses to flush, and animated content freezes while queued
// draw commands accumulate.
func TestBuildLayerTree_CustomLeafPaintRecorded(t *testing.T) {
	sp := rendering.NewRenderSpinner(48)
	sp.SetPhase(0.25)
	sp.MarkNeedsPaint()

	pkt := rendering.BuildLayerTree(sp).BuildPacket(1, 1, 480, 320)
	ops := 0
	scene.Walk(pkt.Root, func(l scene.Layer) {
		if pl, ok := l.(*scene.PictureLayer); ok {
			ops += pl.Recording.PaintOpCount()
		}
	})
	if ops == 0 {
		t.Fatalf("custom leaf (RenderSpinner) recorded no paint commands; retained path would freeze and leak queued draws")
	}
}
