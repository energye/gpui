package scene

import (
	"hash/maphash"
	"image"
	"math"

	"github.com/energye/gpui/render"
)

// FilterResultCache caches the FILTERED pixels of ColorFilter/ImageFilter
// subtrees so the composite only pays the filter cost when something actually
// changed (Skia layer raster cache / Flutter RasterCache semantics).
//
// Motivation (R20): with no registered GPU filter graph, Apply* falls back to
// a full-surface CPU pass. The textured composite re-walks filter layers every
// frame, so an UNCHANGED filter subtree paid that CPU blur ~60×/s (measured:
// 51ms raster/frame, +435MB RSS / 10s in ui_wr_r20_filter). With the cache,
// unchanged frames blit the previous result via DrawImage — whose ImageBuf
// generation id keeps the GPU image-cache entry warm, so the steady state is
// blit-only.
//
// Invalidation is conservative: parameter fingerprint (matrix / radius) plus
// intersection of the frame's DirtyLayerIDs with the subtree layer-id set.
// Subtrees containing TransformLayer are NOT cached (bounds under rotation is
// not worth approximating here); they keep the honest per-frame isolated path.

type filterResultEntry struct {
	buf        *render.ImageBuf // filtered pixels; genID stable while untouched
	offX, offY int              // where to draw inside the layer-local space
	w, h       int
	fp         uint64
	lastUse    uint64
}

// FilterResultCache is owned by the composite pass; all methods run on the
// raster thread.
type FilterResultCache struct {
	entries map[uint64]*filterResultEntry
	stamp   uint64
	seed    maphash.Seed
}

func NewFilterResultCache() *FilterResultCache {
	return &FilterResultCache{entries: map[uint64]*filterResultEntry{}, seed: maphash.MakeSeed()}
}

// BeginFrame ages the cache; entries unused for 240 frames are dropped (they
// hold CPU pixmaps + GPU image-cache slots, so stale leaves must not linger).
func (c *FilterResultCache) BeginFrame() {
	if c == nil {
		return
	}
	c.stamp++
	for k, e := range c.entries {
		if c.stamp-e.lastUse > 240 {
			delete(c.entries, k)
		}
	}
}

func (c *FilterResultCache) Clear() {
	if c == nil {
		return
	}
	c.entries = map[uint64]*filterResultEntry{}
}

// get returns the cached result when the fingerprint matches. Fingerprints
// are combined with the cache key so multiple parameter versions of the same
// filter (e.g. Steady/Spike radius steps) coexist — a phase loop that returns
// to a previously-seen state hits the old entry instead of re-paying the
// filter cost.
func (c *FilterResultCache) get(key, fp uint64) *filterResultEntry {
	if c == nil {
		return nil
	}
	e := c.entries[c.slotKey(key, fp)]
	if e == nil || e.fp != fp {
		return nil
	}
	e.lastUse = c.stamp
	return e
}

func (c *FilterResultCache) put(key, fp uint64, buf *render.ImageBuf, offX, offY, w, h int) {
	if c == nil || buf == nil {
		return
	}
	c.entries[c.slotKey(key, fp)] = &filterResultEntry{buf: buf, offX: offX, offY: offY, w: w, h: h, fp: fp, lastUse: c.stamp}
}

func (c *FilterResultCache) slotKey(key, fp uint64) uint64 {
	h := maphash.Hash{}
	h.SetSeed(c.seed)
	var b [16]byte
	for i := 0; i < 8; i++ {
		b[i] = byte(key >> (8 * i))
		b[8+i] = byte(fp >> (8 * i))
	}
	h.Write(b[:])
	return h.Sum64()
}

// filterFingerprint hashes the layer params plus ONE byte: whether any
// descendant layer id appears in this frame's dirty set. Descendant ids
// themselves must NOT enter the hash — they are per-tree (rebuilt every
// frame), so hashing them would invalidate the cache every frame.
func filterFingerprint(l Layer, dirty map[uint64]struct{}, seed maphash.Seed) uint64 {
	h := maphash.Hash{}
	h.SetSeed(seed)
	switch t := l.(type) {
	case *ColorFilterLayer:
		h.WriteString("color")
		for _, v := range t.Matrix {
			bits := math.Float32bits(v)
			h.Write([]byte{byte(bits), byte(bits >> 8), byte(bits >> 16), byte(bits >> 24)})
		}
	case *ImageFilterLayer:
		h.WriteString("image")
		bits := math.Float64bits(t.BlurRadius)
		h.Write([]byte{byte(bits), byte(bits >> 8), byte(bits >> 16), byte(bits >> 24),
			byte(bits >> 32), byte(bits >> 40), byte(bits >> 48), byte(bits >> 56)})
	default:
		return 0
	}
	anyDirty := byte(0)
	subtreeIDs(l, func(id uint64) {
		if _, dirtyID := dirty[id]; dirtyID {
			anyDirty = 1
		}
	})
	h.Write([]byte{anyDirty})
	return h.Sum64()
}

// subtreeHasTransform reports whether the filter subtree contains a transform
// layer — such subtrees are not cached (see type doc).
func subtreeHasTransform(l Layer) bool {
	found := false
	walkFilterSubtree(l, func(_ uint64, child Layer) {
		if _, ok := child.(*TransformLayer); ok {
			found = true
		}
	})
	return found
}

func subtreeIDs(l Layer, visit func(uint64)) {
	walkFilterSubtree(l, func(id uint64, _ Layer) { visit(id) })
}

// walkFilterSubtree visits every descendant layer (not the filter layer itself).
func walkFilterSubtree(l Layer, visit func(uint64, Layer)) {
	var rec func(Layer)
	rec = func(cur Layer) {
		for _, ch := range cur.Children() {
			if ch == nil {
				continue
			}
			visit(ch.LayerID(), ch)
			rec(ch)
		}
	}
	rec(l)
}

// filterSubtreeBounds unions descendant picture bounds through offset/clip
// chains (layer-local coordinates). Returns false when the subtree carries no
// pictures or spans transforms (unsupported for caching).
func filterSubtreeBounds(l Layer) (image.Rectangle, bool) {
	okTotal := true
	union := image.Rectangle{}
	var rec func(cur Layer, dx, dy float64)
	rec = func(cur Layer, dx, dy float64) {
		for _, ch := range cur.Children() {
			if ch == nil {
				continue
			}
			switch t := ch.(type) {
			case *PictureLayer:
				r := t.Picture.Bounds
				r = r.Add(image.Pt(int(dx), int(dy)))
				union = union.Union(r)
			case *OffsetLayer:
				rec(t, dx+t.DX, dy+t.DY)
			case *BoundaryLayer:
				rec(t, dx+t.DX, dy+t.DY)
			case *ClipRectLayer:
				rec(t, dx, dy)
			case *ClipRRectLayer:
				rec(t, dx, dy)
			case *ContainerLayer:
				rec(t, dx, dy)
			default: // Transform / nested filters / unknown: not supported
				okTotal = false
				return
			}
		}
	}
	rec(l, 0, 0)
	return union, okTotal && !union.Empty()
}
