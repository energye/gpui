package scene

import "testing"

// TestClassifyDirty pins the pure compositor dirty classification
// (dirty-region P2): picture-content mutations re-raster the layer
// (rasterIDs); transform/opacity/offset mutations composite the existing
// texture at a new pose (compositorIDs). First occurrence wins order and
// duplicates collapse; LayerID 0 carries no layer and is skipped; one layer
// appearing with both kinds lands in both lists (it must re-raster AND
// re-composite).
func TestClassifyDirty(t *testing.T) {
	muts := []Mutation{
		{LayerID: 3, Kind: MutSetOpacity}, // compositor-only
		{LayerID: 7, Kind: MutReplacePicture},
		{LayerID: 3, Kind: MutSetOpacity},     // duplicate → collapsed
		{LayerID: 0, Kind: MutReplacePicture}, // no layer → skipped
		{LayerID: 9, Kind: MutSetOffset},
		{LayerID: 7, Kind: MutAddChild}, // same id, raster again → collapsed
		{LayerID: 11, Kind: MutRemoveChild},
		{LayerID: 13, Kind: MutSetTransform},
		{LayerID: 15, Kind: MutReplacePicture},
		{LayerID: 15, Kind: MutSetOpacity}, // both kinds → both lists
	}
	raster, compositor := ClassifyDirty(muts)

	wantRaster := []uint64{7, 11, 15}
	if len(raster) != len(wantRaster) {
		t.Fatalf("raster = %v, want %v", raster, wantRaster)
	}
	for i, id := range wantRaster {
		if raster[i] != id {
			t.Fatalf("raster = %v, want %v", raster, wantRaster)
		}
	}

	wantCompositor := []uint64{3, 9, 13, 15}
	if len(compositor) != len(wantCompositor) {
		t.Fatalf("compositor = %v, want %v", compositor, wantCompositor)
	}
	for i, id := range wantCompositor {
		if compositor[i] != id {
			t.Fatalf("compositor = %v, want %v", compositor, wantCompositor)
		}
	}

	if r, c := ClassifyDirty(nil); len(r) != 0 || len(c) != 0 {
		t.Fatalf("nil muts = (%v, %v), want empty", r, c)
	}
}
