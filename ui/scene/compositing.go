package scene

// CompositingNode is implemented by render objects that participate in
// needsCompositing propagation (F04). Defined here to avoid scene→rendering import;
// rendering types implement the same methods and call Propagate helpers.

// Note: actual bit storage lives on rendering.Base; this file documents band
// helpers and pure layer-side compositor dirty classification.

// ClassifyDirty splits layer ids into raster vs compositor-only using mutations.
func ClassifyDirty(muts []Mutation) (rasterIDs, compositorIDs []uint64) {
	seenR := map[uint64]struct{}{}
	seenC := map[uint64]struct{}{}
	for _, m := range muts {
		if m.LayerID == 0 {
			continue
		}
		switch m.Kind {
		case MutReplacePicture, MutAddChild, MutRemoveChild:
			if _, ok := seenR[m.LayerID]; !ok {
				seenR[m.LayerID] = struct{}{}
				rasterIDs = append(rasterIDs, m.LayerID)
			}
		case MutSetOpacity, MutSetOffset:
			if _, ok := seenC[m.LayerID]; !ok {
				seenC[m.LayerID] = struct{}{}
				compositorIDs = append(compositorIDs, m.LayerID)
			}
		}
	}
	return rasterIDs, compositorIDs
}

// EnsureOverlayBand returns overlay if non-nil, else an empty container (F13 reserve).
func EnsureOverlayBand(overlay Layer) Layer {
	if overlay != nil {
		return overlay
	}
	return NewContainerLayer()
}
