package rendering

import (
	"image"
	"math"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/scene"
)

// BuildLayerTree walks the render tree and builds a retained scene.Layer tree.
// RepaintBoundary nodes become scene.BoundaryLayer units with dirty flags.
// The returned builder's Root is shared by pointer across frames (COW).
//
// Leaf nodes record their own content into PictureLayers (layer-local
// coordinates, 0-based under the ancestor offset/clip chain) so the retained
// texture-composite path (scene.CompositeFramePacketTextured) has real display
// lists to raster. CacheKey binds each picture to a stable RO identity
// (EnsureCacheID) — layer ids are per-tree and cannot key cross-frame caches.
// FrameBuildStats carries counts gathered during the single build walk
// (E2: replaces separate CountRepaintBoundaries / CountPictureOps /
// TreeMeasureCacheStats passes over the finished frame). Clearing of paint
// flags is NOT part of the build — ConsumeNeedsPaint still runs separately.
type FrameBuildStats struct {
	// BoundaryCount / BoundaryMaxDepth match CountRepaintBoundaries.
	BoundaryCount    int
	BoundaryMaxDepth int
	// PictureOpCount matches CountPictureOps on the fresh packet
	// (overlay band still empty at build time).
	PictureOpCount int
	// MeasureHits / MeasureMisses match TreeMeasureCacheStats read after
	// the build. Read post-order per node: recording a node's own text
	// warms its measure cache, so only a read after that node's content
	// is recorded equals the old post-build number.
	MeasureHits   int64
	MeasureMisses int64
}

func BuildLayerTree(root RenderObject) *scene.LayerBuilder {
	b, _ := BuildLayerTreeStats(root)
	return b
}

// BuildLayerTreeStats builds the layer tree like BuildLayerTree and also
// returns the frame counts gathered along the single walk.
func BuildLayerTreeStats(root RenderObject) (*scene.LayerBuilder, FrameBuildStats) {
	return buildLayerTreeWired(root, nil)
}

// saveLayerWire carries the embedder-owned per-frame SaveLayer budget and
// outcome stats into raster-time OnPaint callbacks (RasterExtra). Retained
// mode defers RenderBox.OnPaint to texture-record time on the raster thread;
// without the wire those SaveLayer requests bypass the budget entirely
// (observed: C6 group B wrongly allowed under MaxOps=1). nil = legacy
// unlimited path (tests / CPU builds).
type saveLayerWire struct {
	stats  *SaveLayerStats
	budget *SaveLayerBudget
}

func buildLayerTreeWired(root RenderObject, wire *saveLayerWire) (*scene.LayerBuilder, FrameBuildStats) {
	b := scene.NewLayerBuilder()
	var st FrameBuildStats
	if root == nil {
		return b, st
	}
	appendNode(root, b, wire, 0, &st)
	return b, st
}

// layerSubtreeNeedsPaint is the layer-build dirty check: like
// SubtreeNeedsPaint but stops at nested repaint boundaries. A child
// boundary's dirty state produces its own DirtyLayerID — it re-records
// independently, and the ancestor only re-composites by blitting the child's
// updated texture (Flutter RepaintBoundary semantics; BoundaryCache.tryReplay
// documents the same contract). Without the stop the inner boundary's every
// repaint forces every nested ancestor into DirtyLayerIDs → needless full
// re-record (texture churn, RSS growth). Paint-walk callers keep the
// penetrating SubtreeNeedsPaint (they must walk in to inspect child
// boundaries), so this helper is layer-build only.
func layerSubtreeNeedsPaint(n RenderObject) bool {
	if n == nil {
		return false
	}
	if n.NeedsPaint() {
		return true
	}
	for _, c := range n.Children() {
		if c.IsRepaintBoundary() {
			continue // child boundary re-records itself; ancestor only re-composites
		}
		if layerSubtreeNeedsPaint(c) {
			return true
		}
	}
	return false
}

// appendNode walks one render node, counting repaint boundaries and text
// measure-cache totals along the way (E2: no separate counting passes).
// Boundary depth only nests at boundaries — same rule as
// CountRepaintBoundaries. Text stats are read after the node's own content
// is recorded (see FrameBuildStats).
func appendNode(n RenderObject, b *scene.LayerBuilder, wire *saveLayerWire, bdepth int, st *FrameBuildStats) {
	if n == nil || b == nil {
		return
	}
	if st != nil && n.IsRepaintBoundary() {
		bdepth++
		st.BoundaryCount++
		if bdepth > st.BoundaryMaxDepth {
			st.BoundaryMaxDepth = bdepth
		}
	}
	appendNodeInner(n, b, wire, bdepth, st)
	if st != nil {
		if t, ok := n.(*RenderText); ok {
			h, m := t.MeasureCacheStats()
			st.MeasureHits += h
			st.MeasureMisses += m
		}
	}
}

func appendNodeInner(n RenderObject, b *scene.LayerBuilder, wire *saveLayerWire, bdepth int, st *FrameBuildStats) {
	off := n.Offset()
	// One child-list fetch per node (E2): the branches below used to call
	// Children() twice (len check + range). The tree structure is not
	// mutated during the build, so a single snapshot serves both with
	// identical iteration.
	kids := n.Children()

	// Transform nodes push scene.TransformLayer (P1).
	if tr, ok := n.(*RenderTransform); ok {
		rot, sx, sy := tr.TransformParams()
		sz := n.Size()
		cx, cy := sz.Width*0.5, sz.Height*0.5
		if n.IsRepaintBoundary() {
			b.PushBoundary(off.X, off.Y, "transform", n.NeedsPaint() || layerSubtreeNeedsPaint(n))
			b.PushTransform(0, 0, rot, sx, sy).SetPivot(cx, cy)
			for _, ch := range kids {
				appendNode(ch, b, wire, bdepth, st)
			}
			if len(kids) == 0 {
				addLeafPicture(b, n, wire)
			}
			b.Pop() // transform
			b.Pop() // boundary
			return
		}
		b.PushTransform(off.X, off.Y, rot, sx, sy).SetPivot(cx, cy)
		if len(kids) == 0 {
			addLeafPicture(b, n, wire)
		}
		for _, ch := range kids {
			appendNode(ch, b, wire, bdepth, st)
		}
		b.Pop()
		return
	}

	// Opacity nodes push scene.OpacityLayer (Flutter OpacityLayer): group
	// opacity applies at composite time in the retained path; offset nests
	// children under it. op>=1 is identity (Flutter skips the layer);
	// op<=0 still emits structure so dirty bookkeeping survives, composite
	// renders nothing.
	if ro, ok := n.(*RenderOpacity); ok {
		op := ro.OpacityParams()
		pushOp := op > 0 && op < 1
		dirty := n.NeedsPaint() || layerSubtreeNeedsPaint(n)
		if n.IsRepaintBoundary() {
			b.PushBoundary(off.X, off.Y, "opacity", dirty)
			if pushOp {
				b.PushOpacity(op)
			}
			for _, ch := range kids {
				appendNode(ch, b, wire, bdepth, st)
			}
			if len(kids) == 0 {
				addLeafPicture(b, n, wire)
			}
			if pushOp {
				b.Pop() // opacity
			}
			b.Pop() // boundary
			return
		}
		b.PushOffset(off.X, off.Y)
		if pushOp {
			b.PushOpacity(op)
		}
		if len(kids) == 0 {
			addLeafPicture(b, n, wire)
		}
		for _, ch := range kids {
			appendNode(ch, b, wire, bdepth, st)
		}
		if pushOp {
			b.Pop() // opacity
		}
		b.Pop() // offset
		return
	}

	// ColorFilter nodes push scene.ColorFilterLayer (Flutter ColorFilterLayer):
	// the 4×5 matrix applies to the subtree at composite time. Identity matrices
	// are omitted so steady-state layer trees stay unchanged (same rule as the
	// paint path).
	if cf, ok := n.(*RenderColorFilter); ok {
		matrix := cf.ColorMatrix()
		pushFilter := !cf.isIdentity()
		dirty := n.NeedsPaint() || layerSubtreeNeedsPaint(n)
		if n.IsRepaintBoundary() {
			b.PushBoundary(off.X, off.Y, "color_filter", dirty)
			if pushFilter {
				b.PushColorFilter(matrix).SetCacheKey(cf.EnsureCacheID())
			}
			for _, ch := range kids {
				appendNode(ch, b, wire, bdepth, st)
			}
			if len(kids) == 0 {
				addLeafPicture(b, n, wire)
			}
			if pushFilter {
				b.Pop() // color_filter
			}
			b.Pop() // boundary
			return
		}
		b.PushOffset(off.X, off.Y)
		if pushFilter {
			b.PushColorFilter(matrix).SetCacheKey(cf.EnsureCacheID())
		}
		if len(kids) == 0 {
			addLeafPicture(b, n, wire)
		}
		for _, ch := range kids {
			appendNode(ch, b, wire, bdepth, st)
		}
		if pushFilter {
			b.Pop() // color_filter
		}
		b.Pop() // offset
		return
	}

	// ImageFilter nodes push scene.ImageFilterLayer (Flutter ImageFiltered):
	// uniform blur on the subtree at composite time. Radius ≤ 0 omits the layer.
	if imf, ok := n.(*RenderImageFilter); ok {
		radius := imf.BlurParams()
		pushFilter := radius > 0
		dirty := n.NeedsPaint() || layerSubtreeNeedsPaint(n)
		if n.IsRepaintBoundary() {
			b.PushBoundary(off.X, off.Y, "image_filter", dirty)
			if pushFilter {
				b.PushImageFilter(radius).SetCacheKey(imf.EnsureCacheID())
			}
			for _, ch := range kids {
				appendNode(ch, b, wire, bdepth, st)
			}
			if len(kids) == 0 {
				addLeafPicture(b, n, wire)
			}
			if pushFilter {
				b.Pop() // image_filter
			}
			b.Pop() // boundary
			return
		}
		b.PushOffset(off.X, off.Y)
		if pushFilter {
			b.PushImageFilter(radius).SetCacheKey(imf.EnsureCacheID())
		}
		if len(kids) == 0 {
			addLeafPicture(b, n, wire)
		}
		for _, ch := range kids {
			appendNode(ch, b, wire, bdepth, st)
		}
		if pushFilter {
			b.Pop() // image_filter
		}
		b.Pop() // offset
		return
	}

	// ClipRRect nodes push scene.ClipRRectLayer (Flutter ClipRRectLayer / pushClipRRect).
	// Offset establishes local origin; clip is (0,0,w,h) in that space so children nest under it.
	if cr, ok := n.(*RenderClipRRect); ok {
		w, h, radius := cr.ClipRRectParams()
		dirty := n.NeedsPaint() || layerSubtreeNeedsPaint(n)
		if n.IsRepaintBoundary() {
			b.PushBoundary(off.X, off.Y, "clip_rrect", dirty)
			b.PushClipRRect(0, 0, w, h, radius)
			for _, ch := range kids {
				appendNode(ch, b, wire, bdepth, st)
			}
			if len(kids) == 0 {
				addLeafPicture(b, n, wire)
			}
			b.Pop() // clip_rrect
			b.Pop() // boundary
			return
		}
		b.PushOffset(off.X, off.Y)
		b.PushClipRRect(0, 0, w, h, radius)
		if len(kids) == 0 {
			addLeafPicture(b, n, wire)
		}
		for _, ch := range kids {
			appendNode(ch, b, wire, bdepth, st)
		}
		b.Pop() // clip_rrect
		b.Pop() // offset
		return
	}

	// RenderViewport pushes boundary (it is its own repaint boundary) +
	// clip-rect so scrolled-out content is clipped at composite time and
	// never recorded into full-height textures (Flutter ViewportLayer
	// semantics). restrictive() treats the clip as restrictive: descendants
	// record into bounds-sized textures under the clip, not the surface.
	if v, ok := n.(*RenderViewport); ok {
		sz := n.Size()
		bl := b.PushBoundary(off.X, off.Y, "viewport", n.NeedsPaint() || layerSubtreeNeedsPaint(n))
		bl.Shell = v.IsShellBoundary()
		b.PushClipRect(0, 0, sz.Width, sz.Height)
		// Scroll translation (Flutter ViewportLayer): content coordinates
		// shift by -scroll inside the clip so the retained path can express
		// scroll position. Without it rows drift out of the clip and are
		// never replaced from below (C4 一行一行消失).
		so := v.ScrollOffset()
		b.PushTransform(-so.X, -so.Y, 0, 1, 1)
		addOwnContent(b, n, wire)
		for _, ch := range kids {
			appendNode(ch, b, wire, bdepth, st)
		}
		if len(kids) == 0 {
			addLeafPicture(b, n, wire)
		}
		b.Pop() // scroll transform
		b.Pop() // clip_rect
		b.Pop() // boundary
		return
	}

	if n.IsRepaintBoundary() {
		bl := b.PushBoundary(off.X, off.Y, typeName(n), n.NeedsPaint() || layerSubtreeNeedsPaint(n))
		if bse, ok := baseOf(n); ok {
			bl.Shell = bse.IsShellBoundary()
		}
		addOwnContent(b, n, wire)
		for _, ch := range kids {
			appendNode(ch, b, wire, bdepth, st)
		}
		// Leaf content inside boundary: ensure a picture slot when no children.
		if len(kids) == 0 {
			addLeafPicture(b, n, wire)
		}
		b.Pop()
		return
	}
	b.PushOffset(off.X, off.Y)
	addOwnContent(b, n, wire)
	if len(kids) == 0 {
		addLeafPicture(b, n, wire)
	}
	for _, ch := range kids {
		appendNode(ch, b, wire, bdepth, st)
	}
	b.Pop()
}

// addOwnContent records a node's own visual content (its background / custom
// paint) into a PictureLayer under the node's offset/boundary, so retained
// texture compositing does not depend on LoadOpLoad pixel leftovers for
// container backgrounds or OnPaint-only boxes (Flutter paint semantics: every
// node's own paint is part of its display list). No-op for nodes with nothing
// of their own to draw.
func addOwnContent(b *scene.LayerBuilder, n RenderObject, wire *saveLayerWire) {
	if n == nil || b == nil {
		return
	}
	switch t := n.(type) {
	case *AbsoluteBox:
		if t.Background == nil {
			return
		}
		sz := n.Size()
		bg := t.Background
		addOwnPicture(b, n, n.NeedsPaint(), func(r *scene.PictureRecorder) {
			r.FillRect(0, 0, sz.Width, sz.Height, bg.R, bg.G, bg.B, bg.A)
		})
	default:
		// OnPaint-only leaves are handled by addLeafPicture (single cache key);
		// a box with BOTH OnPaint and children gets its own paint layer here.
		// ownBoxPaint covers RenderBox and wrappers embedding it (InputBox
		// family sets OnPaint on the embedded box); a type case would only
		// match exact *RenderBox and miss the wrappers.
		onPaint, ok := ownBoxPaint(n)
		if !ok || len(n.Children()) == 0 {
			return
		}
		sz := n.Size()
		// OnPaint needs a live DC (render.Context) which only exists on the
		// raster thread during texture record — defer it via RasterExtra.
		pl := addOwnPicture(b, n, n.NeedsPaint(), func(r *scene.PictureRecorder) {
			// Nothing capturable UI-side; the callback draws at raster time.
		})
		if pl != nil {
			pl.RasterExtra = func(dc *render.Context) {
				pc := &PaintContext{DC: dc, Scale: 1, LayerStats: wireStats(wire), LayerBudget: wireBudget(wire)}
				onPaint(pc, sz)
			}
			pl.ExtraBounds = image.Rect(0, 0,
				int(math.Ceil(sz.Width)), int(math.Ceil(sz.Height)))
		}
	}
}

// addOwnPicture adds a PictureLayer holding n's own content and binds n's
// stable cache key (own content and leaf share the identity — a node never
// appears twice, so keys cannot collide).
func addOwnPicture(b *scene.LayerBuilder, n RenderObject, needsRaster bool, record func(r *scene.PictureRecorder)) *scene.PictureLayer {
	pl := b.AddPicture(needsRaster)
	if base, ok := baseOf(n); ok {
		pl.SetCacheKey(base.EnsureCacheID())
	}
	scene.RecordInto(&pl.Picture, record)
	return pl
}

// addLeafPicture records a leaf node's own content into a PictureLayer and
// binds its stable cache key. Layer-local coordinates (0-based): the ancestor
// offset/clip/transform chain positions the picture at composite time.
// Leaves the layer builder can't faithfully record keep an empty picture and
// a zero cache key (vector replay fallback paints nothing — the RO type has
// no layer-tree representation, consistent with Flutter layer semantics).
func addLeafPicture(b *scene.LayerBuilder, n RenderObject, wire *saveLayerWire) {
	pl := b.AddPicture(n.NeedsPaint())
	if n == nil {
		return
	}
	if base, ok := baseOf(n); ok {
		pl.SetCacheKey(base.EnsureCacheID())
	} else {
		pl.SetCacheKey(0)
	}
	scene.RecordInto(&pl.Picture, func(r *scene.PictureRecorder) {
		recordLeafContent(r, n)
	})
	// OnPaint-only boxes have nothing the UI-side recorder can capture; defer
	// their paint to the raster thread during texture record (RasterExtra) so
	// the retained path renders them instead of leaving a transparent hole.
	if onPaint, ok := ownBoxPaint(n); ok && len(pl.Picture.Ops) == 0 {
		sz := n.Size()
		pl.RasterExtra = func(dc *render.Context) {
			pc := &PaintContext{DC: dc, Scale: 1, LayerStats: wireStats(wire), LayerBudget: wireBudget(wire)}
			onPaint(pc, sz)
		}
		pl.ExtraBounds = image.Rect(0, 0,
			int(math.Ceil(sz.Width)), int(math.Ceil(sz.Height)))
	}
}

// recordLeafContent records a leaf node's own content in layer-local
// coordinates. Supports the render types that have a faithful display-list
// representation; anything else records nothing (empty picture).
func recordLeafContent(r *scene.PictureRecorder, n RenderObject) {
	if r == nil || n == nil {
		return
	}
	sz := n.Size()
	switch t := n.(type) {
	case *RenderColorBox:
		cw, chh := sz.Width, sz.Height
		if cw <= 0 {
			cw = t.Width
		}
		if chh <= 0 {
			chh = t.Height
		}
		r.FillRect(0, 0, cw, chh, t.R, t.G, t.B, t.A)
	case *RenderText:
		recordRenderText(r, t, 0, 0)
	case *RenderImage:
		dw, dh := sz.Width, sz.Height
		if dw <= 0 {
			dw = t.Width
		}
		if dh <= 0 {
			dh = t.Height
		}
		if t.Img != nil && !t.Img.Disposed() {
			r.DrawImage(t.Img, 0, 0, dw, dh)
		} else {
			r.FillRect(0, 0, dw, dh, t.PR, t.PG, t.PB, 1)
		}
	}
}

func typeName(n RenderObject) string {
	switch n.(type) {
	case *RenderColorBox:
		return "color"
	case *RenderBox:
		return "box"
	case *RenderTransform:
		return "transform"
	case *RenderClipRRect:
		return "clip_rrect"
	default:
		return "node"
	}
}

// BuildFramePacket layouts are assumed already flushed; builds a packet for raster.
// Overlay band is an empty F13 reserve; use overlay.State.AttachToPacket to fill it.
//
// D3 inline mode: this is the synchronous inline path for unit tests (no
// raster Loop involved). Windows go through the async embedder+raster Loop;
// the packet shape is identical on both paths.
func BuildFramePacket(root RenderObject, frameID uint64, dpr, w, h float64) *scene.FramePacket {
	begin := time.Now().UnixNano()
	b := BuildLayerTree(root)
	pkt := b.BuildPacket(frameID, dpr, w, h)
	sealBuiltPacket(pkt, begin)
	return pkt
}

// BuildFramePacketWithSaveLayer builds the packet like BuildFramePacket while
// wiring the embedder-owned per-frame SaveLayer budget and outcome stats into
// raster-time OnPaint callbacks. Retained mode defers RenderBox.OnPaint to
// texture-record time; without this wiring a configured budget (PipelineOptions
// SaveLayerMaxOps/MaxArea) never sees those requests. stats/budget may be nil
// (= BuildFramePacket behavior).
func BuildFramePacketWithSaveLayer(root RenderObject, frameID uint64, dpr, w, h float64, stats *SaveLayerStats, budget *SaveLayerBudget) *scene.FramePacket {
	pkt, _ := BuildFramePacketWithSaveLayerStats(root, frameID, dpr, w, h, stats, budget)
	return pkt
}

// BuildFramePacketWithSaveLayerStats builds the packet like
// BuildFramePacketWithSaveLayer and also returns the frame counts gathered
// along the single build walk (boundary discovery, picture op total, text
// measure-cache totals). Old standalone counters stay for other callers.
func BuildFramePacketWithSaveLayerStats(root RenderObject, frameID uint64, dpr, w, h float64, stats *SaveLayerStats, budget *SaveLayerBudget) (*scene.FramePacket, FrameBuildStats) {
	begin := time.Now().UnixNano()
	b, st := buildLayerTreeWired(root, &saveLayerWire{stats: stats, budget: budget})
	pkt := b.BuildPacket(frameID, dpr, w, h)
	st.PictureOpCount = b.PictureOpCount()
	sealBuiltPacket(pkt, begin)
	return pkt, st
}

// sealBuiltPacket stamps the G10 build pair, tags the G1 producer, and
// seals the EndFrame point (G3). Raster stamps stay zero until the raster
// thread marks them (T2). Nil-safe for empty builds.
func sealBuiltPacket(pkt *scene.FramePacket, beginNs int64) {
	if pkt == nil {
		return
	}
	pkt.BuildBeginNs = beginNs
	pkt.MarkBuildEnd()
	pkt.Producer = scene.ProducerUI
	pkt.Seal()
}

func wireStats(w *saveLayerWire) *SaveLayerStats {
	if w == nil {
		return nil
	}
	return w.stats
}

func wireBudget(w *saveLayerWire) *SaveLayerBudget {
	if w == nil {
		return nil
	}
	return w.budget
}

// BuildFramePacketOverlay builds the main packet then attaches ov to pkt.Overlay.
// ov may be nil (empty band). Main DirtyLayerIDs are preserved and overlay dirties appended.
// The attach callback is the EndFrame tail: it runs before the seal so the
// handed-off packet already carries both bands.
func BuildFramePacketOverlay(root RenderObject, frameID uint64, dpr, w, h float64, attach func(pkt *scene.FramePacket)) *scene.FramePacket {
	begin := time.Now().UnixNano()
	b := BuildLayerTree(root)
	pkt := b.BuildPacket(frameID, dpr, w, h)
	if attach != nil && pkt != nil {
		attach(pkt)
	}
	sealBuiltPacket(pkt, begin)
	return pkt
}
