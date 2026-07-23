package primitive

import (
	"math"
	"unsafe"

	"github.com/energye/gpui/ui/core"
)

// RepaintBoundary isolates paint dirty bubbling (Flutter RepaintBoundary).
//
// With PaintContext.LayerCache (G2 compositor path):
//   - dirty → RasterizeBoundary into offscreen GPU RT (vector OK on that RT)
//   - clean → keep RT; BlitBoundary updates origin (and blits if !DeferLayerBlit)
//   - DeferLayerBlit=true → host Compositor does a blit-only present pass (G2.b)
//
// Without LayerCache: paints children directly into parent.
type RepaintBoundary struct {
	core.NodeBase
}

// NewRepaintBoundary wraps a child (or empty) as a paint isolation root.
func NewRepaintBoundary(children ...core.Node) *RepaintBoundary {
	b := &RepaintBoundary{}
	b.Init(b)
	b.Hit = core.HitDefer
	b.SetRepaintBoundary(true)
	for _, c := range children {
		b.AddChild(c)
	}
	return b
}

// TypeID implements core.Node.
func (b *RepaintBoundary) TypeID() string { return TypeRepaintBoundary }

// Layout implements core.Node.
func (b *RepaintBoundary) Layout(c core.Constraints) core.Size {
	if sz, ok := b.LayoutSkipIfClean(c); ok {
		return sz
	}
	kids := b.Children()
	content := core.Size{}
	if len(kids) == 1 {
		content = kids[0].Layout(c.Expand())
		kids[0].Base().SetOffset(core.Point{})
	} else if len(kids) > 1 {
		for _, child := range kids {
			sz := child.Layout(c.Expand())
			child.Base().SetOffset(core.Point{})
			content = core.MaxSize(content, sz)
		}
	}
	out := c.Tighten(content)
	b.SetSize(out)
	b.RememberConstraints(c)
	return out
}

func (b *RepaintBoundary) cacheKey() uintptr {
	return uintptr(unsafe.Pointer(b))
}

// Paint implements core.Node.
func (b *RepaintBoundary) Paint(pc *core.PaintContext) {
	if pc == nil {
		return
	}
	sz := b.Size()
	w := int(math.Ceil(sz.Width))
	h := int(math.Ceil(sz.Height))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	scale := pc.Scale
	if scale <= 0 {
		scale = 1
	}
	key := b.cacheKey()
	ox, oy := pc.Origin.X, pc.Origin.Y

	if pc.LayerCache != nil {
		// ForceFullPaint rebuilds the window base; clean boundaries keep their
		// retained RT and only update blit origin. Re-rasterizing every boundary
		// on any non-boundary dirty (or while Spin runs) made scroll thumb lag
		// the pointer and thrash with animations.
		needRaster := b.NeedsPaint()
		if !needRaster {
			if !pc.LayerCache.BlitBoundary(key, nil, ox, oy, w, h) {
				needRaster = true
			}
		}
		if needRaster {
			ok := pc.LayerCache.RasterizeBoundary(key, pc.DC, ox, oy, w, h, scale, func(childPC *core.PaintContext) {
				if childPC == nil {
					return
				}
				if pc.Theme != nil {
					childPC.Theme = pc.Theme
				}
				for _, ch := range b.Children() {
					cb := ch.Base()
					ch.Paint(childPC.WithOrigin(cb.Offset()))
				}
			})
			if !ok {
				b.paintDirect(pc)
				return
			}
		}
		if !pc.DeferLayerBlit && pc.DC != nil {
			_ = pc.LayerCache.BlitBoundary(key, pc.DC, ox, oy, w, h)
		}
		b.ClearPaintDirty()
		return
	}

	b.paintDirect(pc)
}

func (b *RepaintBoundary) paintDirect(pc *core.PaintContext) {
	if pc.CompositeOnly && !b.NeedsPaint() && !pc.ForceFullPaint {
		b.ClearPaintDirty()
		return
	}
	childPC := pc
	if b.NeedsPaint() || pc.ForceFullPaint {
		childPC = pc.WithForceFullPaint()
		childPC.CompositeOnly = false
	}
	for _, ch := range b.Children() {
		cb := ch.Base()
		ch.Paint(childPC.WithOrigin(pc.Origin.Add(cb.Offset())))
	}
	b.ClearPaintDirty()
}

// HitTest implements core.Node.
func (b *RepaintBoundary) HitTest(p core.Point) core.Node { return b.DefaultHitTest(p) }
