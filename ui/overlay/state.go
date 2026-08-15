package overlay

import (
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// State is the overlay stack for one window/scope.
//
// Entries are ordered bottom → top (index 0 painted first; last hit-tested first).
type State struct {
	entries []*Entry
	nextID  uint64

	// Generation increments on Insert/Remove (callers may ScheduleFrame).
	Generation uint64

	// OverlayDirtyIDs from the last BuildOverlayBand (test helper).
	LastDirtyIDs []uint64
}

// New creates an empty overlay state.
func New() *State {
	return &State{}
}

// Len returns stack depth.
func (s *State) Len() int {
	if s == nil {
		return 0
	}
	return len(s.entries)
}

// Entries returns a copy of the stack (bottom → top).
func (s *State) Entries() []*Entry {
	if s == nil || len(s.entries) == 0 {
		return nil
	}
	out := make([]*Entry, len(s.entries))
	copy(out, s.entries)
	return out
}

// Insert pushes e on top of the stack. Returns e (with id assigned).
func (s *State) Insert(e *Entry) *Entry {
	if s == nil || e == nil || e.removed {
		return e
	}
	// Already in this stack?
	for _, x := range s.entries {
		if x == e {
			return e
		}
	}
	s.nextID++
	e.id = s.nextID
	e.removed = false
	e.NeedsPaint = true
	s.entries = append(s.entries, e)
	s.Generation++
	return e
}

// Remove pops e from the stack (by pointer or id). Invokes OnRemove once.
func (s *State) Remove(e *Entry) bool {
	if s == nil || e == nil {
		return false
	}
	for i, x := range s.entries {
		if x != e && (e.id == 0 || x.id != e.id) {
			continue
		}
		s.entries = append(s.entries[:i], s.entries[i+1:]...)
		if !x.removed {
			x.removed = true
			if x.OnRemove != nil {
				x.OnRemove()
			}
		}
		s.Generation++
		return true
	}
	return false
}

// Clear removes all entries.
func (s *State) Clear() {
	if s == nil {
		return
	}
	for len(s.entries) > 0 {
		s.Remove(s.entries[len(s.entries)-1])
	}
}

// Layout lays out each entry child and fills zero W/H from child size.
func (s *State) Layout(viewportW, viewportH float64) {
	if s == nil {
		return
	}
	for _, e := range s.entries {
		if e == nil || e.Child == nil {
			continue
		}
		maxW, maxH := e.W, e.H
		if maxW <= 0 {
			maxW = viewportW
		}
		if maxH <= 0 {
			maxH = viewportH
		}
		csz := e.Child.Layout(rendering.Constraints{
			MinWidth: 0, MaxWidth: maxW,
			MinHeight: 0, MaxHeight: maxH,
		})
		e.Child.SetOffset(rendering.Point{})
		if e.W <= 0 {
			e.W = csz.Width
		}
		if e.H <= 0 {
			e.H = csz.Height
		}
	}
}

// HitResult is the outcome of overlay-first hit testing.
type HitResult struct {
	// Entry is the overlay entry that claimed the hit (nil → miss overlay).
	Entry *Entry
	// Child is the deepest RenderObject inside the entry (may be nil for barrier-only).
	Child rendering.RenderObject
	// Consumed is true if the hit should not fall through to main.
	Consumed bool
}

// HitTest walks entries top → bottom. Point is window/logical coordinates.
func (s *State) HitTest(p rendering.Point) HitResult {
	if s == nil {
		return HitResult{}
	}
	for i := len(s.entries) - 1; i >= 0; i-- {
		e := s.entries[i]
		if e == nil || !e.Contains(p) {
			continue
		}
		local := rendering.Point{X: p.X - e.X, Y: p.Y - e.Y}
		var child rendering.RenderObject
		if e.Child != nil {
			child = e.Child.HitTest(local)
		}
		if child != nil {
			return HitResult{Entry: e, Child: child, Consumed: true}
		}
		if e.Barrier {
			return HitResult{Entry: e, Child: nil, Consumed: true}
		}
		// Non-barrier miss inside bounds: still stop if we want modal cards
		// with holes — MVP: non-barrier without child hit falls through this entry.
	}
	return HitResult{}
}

// Band identifies which band a hit-test result belongs to (main root vs overlay).
type Band int

const (
	BandNone Band = iota
	BandMain
	BandOverlay
)

// HitTestStack tests overlay then main.
// HitTestStack is overlay-first then main root. Returns band + object.
func HitTestStack(main rendering.RenderObject, ov *State, p rendering.Point) (Band, rendering.RenderObject, *Entry) {
	if ov != nil {
		hr := ov.HitTest(p)
		if hr.Consumed {
			if hr.Child != nil {
				return BandOverlay, hr.Child, hr.Entry
			}
			return BandOverlay, nil, hr.Entry
		}
	}
	if main != nil {
		if hit := main.HitTest(p); hit != nil {
			return BandMain, hit, nil
		}
	}
	return BandNone, nil, nil
}

// BuildOverlayBand constructs the retained overlay layer tree (bottom → top)
// and dirty layer ids for entries that need paint.
func (s *State) BuildOverlayBand() (scene.Layer, []uint64) {
	if s == nil || len(s.entries) == 0 {
		return scene.EnsureOverlayBand(nil), nil
	}
	root := scene.NewContainerLayer()
	var dirty []uint64
	for _, e := range s.entries {
		if e == nil {
			continue
		}
		off := scene.NewOffsetLayer(e.X, e.Y)
		if e.Child != nil {
			b := rendering.BuildLayerTree(e.Child)
			if b != nil && b.Root() != nil {
				off.Add(b.Root())
				if e.NeedsPaint {
					dirty = append(dirty, b.DirtyBoundaryIDs...)
					scene.Walk(b.Root(), func(l scene.Layer) {
						if pl, ok := l.(*scene.PictureLayer); ok && pl.NeedsRaster {
							dirty = append(dirty, pl.LayerID())
						}
					})
				}
			}
		} else if e.Barrier && e.NeedsPaint {
			pl := scene.NewPictureLayer()
			pl.NeedsRaster = true
			off.Add(pl)
			dirty = append(dirty, pl.LayerID())
		}
		root.Add(off)
	}
	s.LastDirtyIDs = append([]uint64(nil), dirty...)
	return root, dirty
}

// AttachToPacket sets pkt.Overlay from this state and merges overlay dirty ids.
// Main DirtyLayerIDs are left unchanged (D5: overlay open must not replace main dirties).
func (s *State) AttachToPacket(pkt *scene.FramePacket) {
	if pkt == nil {
		return
	}
	if s == nil || s.Len() == 0 {
		pkt.Overlay = scene.EnsureOverlayBand(nil)
		return
	}
	layer, dirty := s.BuildOverlayBand()
	pkt.Overlay = layer
	if len(dirty) > 0 {
		pkt.DirtyLayerIDs = append(pkt.DirtyLayerIDs, dirty...)
	}
}
