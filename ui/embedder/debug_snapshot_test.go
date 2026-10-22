package embedder

import "testing"

// TestSnapshotPaintOptsCarriesDebugRepaint guards the R12b evidence snapshot:
// the final readback repaint must carry the same debug overlay flag as live
// presents, but must not add to the cumulative live-repaint counter.
func TestSnapshotPaintOptsCarriesDebugRepaint(t *testing.T) {
	var a PipelineApp

	a.SetDebugRepaint(true)
	opts := a.snapshotPaintOpts()
	if !opts.debugRepaint {
		t.Fatal("snapshotPaintOpts dropped debugRepaint=true")
	}
	if opts.debugDraws != nil {
		t.Fatal("snapshotPaintOpts must not accumulate into the live debug counter")
	}

	a.SetDebugRepaint(false)
	if a.snapshotPaintOpts().debugRepaint {
		t.Fatal("snapshotPaintOpts forced debugRepaint on")
	}
}
