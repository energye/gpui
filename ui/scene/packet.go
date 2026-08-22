package scene

// MutationKind classifies FramePacket mutations (incremental updates).
type MutationKind int

const (
	// MutNone is unused.
	MutNone MutationKind = iota
	// MutReplacePicture marks a picture layer for re-record/raster.
	MutReplacePicture
	// MutSetOpacity updates opacity without re-raster of children pictures.
	MutSetOpacity
	// MutSetOffset updates offset (compositor-only).
	MutSetOffset
	// MutSetTransform updates transform attributes (compositor-only candidate).
	MutSetTransform
	// MutAddChild / MutRemoveChild are structural (rare).
	MutAddChild
	MutRemoveChild
)

// Mutation is a COW delta against a shared layer tree.
type Mutation struct {
	Kind    MutationKind
	LayerID uint64
	// Optional payload
	Opacity float64
	DX, DY  float64
	// Transform payload (MutSetTransform)
	Rotation float64
	SX, SY   float64
}

// LayerBand selects main vs overlay stacking (F13).
type LayerBand int

const (
	// BandMain is the application retained tree.
	BandMain LayerBand = iota
	// BandOverlay is reserved for portals/modals above main (may be empty).
	BandOverlay
)

// FramePacket is the cross-thread frame description (UI → Raster).
// Root layers are shared by pointer (no deep copy of the tree).
type FramePacket struct {
	FrameID uint64
	DPR     float64
	// Width/Height are logical viewport size.
	Width, Height float64

	// Root is the main-band layer tree (shared / COW).
	Root Layer
	// Overlay is the overlay-band root (may be nil). Reserved F13.
	Overlay Layer

	// Mutations are this frame's deltas (optional if DirtyLayerIDs is enough).
	Mutations []Mutation

	// DirtyLayerIDs must re-raster (picture/boundary content).
	DirtyLayerIDs []uint64
	// OverlayDirtyLayerIDs is the overlay-band portion of the frame's dirty
	// set (F13). Kept separate so consumers can assert "opening an overlay
	// dirtied only the overlay band" — main-band ids stay in DirtyLayerIDs.
	OverlayDirtyLayerIDs []uint64
	// CompositorDirtyIDs only need attribute re-composite (opacity/offset).
	CompositorDirtyIDs []uint64

	// Generation is a monotonic producer stamp (not a deep clone id).
	Generation uint64
}

// CloneShallow shares Root/Overlay pointers and copies mutation/dirty slices.
// This is the only supported "copy" path — never deep-clone the layer tree.
func (p *FramePacket) CloneShallow() *FramePacket {
	if p == nil {
		return nil
	}
	out := *p
	if p.Mutations != nil {
		out.Mutations = append([]Mutation(nil), p.Mutations...)
	}
	if p.DirtyLayerIDs != nil {
		out.DirtyLayerIDs = append([]uint64(nil), p.DirtyLayerIDs...)
	}
	if p.OverlayDirtyLayerIDs != nil {
		out.OverlayDirtyLayerIDs = append([]uint64(nil), p.OverlayDirtyLayerIDs...)
	}
	if p.CompositorDirtyIDs != nil {
		out.CompositorDirtyIDs = append([]uint64(nil), p.CompositorDirtyIDs...)
	}
	return &out
}

// ShareRoot reports whether a and b reference the same root pointer (COW share).
func ShareRoot(a, b *FramePacket) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Root != nil && a.Root == b.Root
}

// CollectLayerIDs walks the tree and returns all non-zero ids (test helper).
func CollectLayerIDs(root Layer) []uint64 {
	var ids []uint64
	Walk(root, func(l Layer) {
		if id := l.LayerID(); id != 0 {
			ids = append(ids, id)
		}
	})
	return ids
}
