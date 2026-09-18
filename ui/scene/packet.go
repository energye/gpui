package scene

import (
	"time"

	"github.com/energye/gpui/render"
)

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
//
// T1 read-only contract (§3): the packet is sealed at EndFrame (BuildPacket).
// After Seal, the UI thread must not mutate the shared Root tree in place
// nor the packet slices in place — derivatives go through CloneShallow.
// Overlay attach (overlay.State.AttachToPacket) is the documented EndFrame
// tail: it fills Overlay/OverlayDirtyLayerIDs before the handoff to raster.
// After the handoff, no mutation at all; the raster thread only reads.
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

	// Sealed marks EndFrame (G3): BuildPacket seals the main band.
	// True = handed-off read-only (overlay tail excepted, see above).
	Sealed bool
	// OverlaySealed marks the overlay tail done: AttachToPacket ran once.
	// A second attach is refused (returns false) — double-attach used to
	// silently duplicate overlay dirties. Copied by CloneShallow.
	OverlaySealed bool
	// Producer names the building thread (G1 platform-line split).
	// "ui" = interface thread; "" = unknown (pre-T1 packets in tests).
	Producer string

	// Per-frame four stamps (G10, UnixNano): build begin/end are stamped
	// by ui/rendering at build time; raster begin/end by the raster thread
	// around draw+present (T2 wires the raster pair; zero until then).
	BuildBeginNs  int64
	BuildEndNs    int64
	RasterBeginNs int64
	RasterEndNs   int64

	// PostFrameHooks run after present (G12 secondary-callback slot).
	// Registered pre-Seal on the UI thread; executed on the raster thread.
	// Copied (not shared) by CloneShallow like the dirty slices.
	PostFrameHooks []func(frameID uint64)

	// RetainedImages pins every ImageBuf referenced by display-list
	// OpDrawImage ops until the packet is dropped after present (D15).
	// The ops hold the same pointers (GC liveness), but the explicit list
	// is the audit trail for "what must survive this frame" and keeps
	// shared/disowned buffers alive after the tree drops them (image
	// replace clears the node while the packet is still in flight).
	// Collected at build (BuildPacket) and overlay attach; carried by
	// CloneShallow. Nil = no image content (zero cost).
	RetainedImages []*render.ImageBuf
}

// Producer identities (G1).
const (
	ProducerUnknown = ""
	ProducerUI      = "ui"
)

// Seal marks EndFrame: the main band is handed off read-only (G3).
// Idempotent. The overlay tail (AttachToPacket, once) is the only legal
// post-Seal mutation; everything else must happen pre-Seal.
func (p *FramePacket) Seal() {
	if p == nil {
		return
	}
	p.Sealed = true
}

// IsSealed reports the EndFrame seal (false on nil).
func (p *FramePacket) IsSealed() bool { return p != nil && p.Sealed }

// MarkBuildBegin stamps the build start (UI thread, UnixNano).
func (p *FramePacket) MarkBuildBegin() {
	if p == nil {
		return
	}
	p.BuildBeginNs = time.Now().UnixNano()
}

// MarkBuildEnd stamps the build end (UI thread, UnixNano).
func (p *FramePacket) MarkBuildEnd() {
	if p == nil {
		return
	}
	p.BuildEndNs = time.Now().UnixNano()
}

// MarkRasterBegin stamps the raster start (raster thread, UnixNano).
func (p *FramePacket) MarkRasterBegin() {
	if p == nil {
		return
	}
	p.RasterBeginNs = time.Now().UnixNano()
}

// MarkRasterEnd stamps the raster end (raster thread, UnixNano).
func (p *FramePacket) MarkRasterEnd() {
	if p == nil {
		return
	}
	p.RasterEndNs = time.Now().UnixNano()
}

// BuildMs returns (BuildEnd-BuildBegin) in ms, or 0 when unstamped.
func (p *FramePacket) BuildMs() float64 {
	if p == nil || p.BuildEndNs <= p.BuildBeginNs {
		return 0
	}
	return float64(p.BuildEndNs-p.BuildBeginNs) / 1e6
}

// RasterMs returns (RasterEnd-RasterBegin) in ms, or 0 when unstamped.
func (p *FramePacket) RasterMs() float64 {
	if p == nil || p.RasterEndNs <= p.RasterBeginNs {
		return 0
	}
	return float64(p.RasterEndNs-p.RasterBeginNs) / 1e6
}

// AddPostFrameHook registers a G12 hook. Must be called pre-Seal;
// returns false (no-op) when sealed or fn is nil.
func (p *FramePacket) AddPostFrameHook(fn func(frameID uint64)) bool {
	if p == nil || fn == nil || p.Sealed {
		return false
	}
	p.PostFrameHooks = append(p.PostFrameHooks, fn)
	return true
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
	if p.PostFrameHooks != nil {
		out.PostFrameHooks = append([]func(frameID uint64){}, p.PostFrameHooks...)
	}
	if p.RetainedImages != nil {
		out.RetainedImages = append([]*render.ImageBuf(nil), p.RetainedImages...)
	}
	return &out
}

// RetainImagesFrom collects ImageBufs referenced by OpDrawImage ops under l
// into the packet (D15). Called at build for the main band and by overlay
// attach for the overlay band (attached post-Seal, pre-handoff).
func (p *FramePacket) RetainImagesFrom(l Layer) {
	if p == nil || l == nil {
		return
	}
	seen := make(map[*render.ImageBuf]struct{}, len(p.RetainedImages))
	for _, img := range p.RetainedImages {
		seen[img] = struct{}{}
	}
	Walk(l, func(n Layer) {
		pl, ok := n.(*PictureLayer)
		if !ok {
			return
		}
		for i := range pl.Picture.Ops {
			if op := &pl.Picture.Ops[i]; op.Kind == OpDrawImage && op.Image != nil {
				if _, dup := seen[op.Image]; !dup {
					seen[op.Image] = struct{}{}
					p.RetainedImages = append(p.RetainedImages, op.Image)
				}
			}
		}
	})
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
