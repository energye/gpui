package primitive

import (
	"fmt"
	"sync/atomic"

	"github.com/energye/gpui/ui/core"
)

// portalSeq allocates unique OverlayPortal IDs when callers leave ID empty.
// Fixed IDs ("modal", "drawer") remain for intentional singletons.
var portalSeq atomic.Uint64

func nextPortalID() string {
	return fmt.Sprintf("portal-%d", portalSeq.Add(1))
}

// OverlayPortal teleports content into the tree OverlayHost when Open (C-PortalHost).
// The portal node itself has zero size in the main tree; content is laid out/painted
// via Tree.Overlays().
//
// ID: empty → assigned a unique stable id on first push (never collapses to a shared
// "portal" string that would clobber concurrent Tooltip/Select/Message overlays).
// Explicit IDs (e.g. "modal") replace by design (one instance per id).
type OverlayPortal struct {
	core.NodeBase

	Open   bool
	ID     string
	ZOrder int
	// Content is the floating node (Mask+panel stack, etc.).
	Content core.Node
	// ContentOffset is absolute position for the content root.
	ContentOffset core.Point

	tree   *core.Tree
	pushed bool
}

// NewOverlayPortal creates a closed portal.
func NewOverlayPortal(content core.Node) *OverlayPortal {
	p := &OverlayPortal{Content: content, ZOrder: 100}
	p.Init(p)
	p.Hit = core.HitTransparent
	return p
}

// TypeID implements core.Node.
func (p *OverlayPortal) TypeID() string { return TypeOverlayPortal }

// Layout implements core.Node — zero size placeholder.
func (p *OverlayPortal) Layout(c core.Constraints) core.Size {
	out := c.Tighten(core.Size{})
	p.SetSize(out)
	p.syncHost()
	return out
}

// Paint implements core.Node — nothing in place.
func (p *OverlayPortal) Paint(pc *core.PaintContext) {}

// HitTest implements core.Node.
func (p *OverlayPortal) HitTest(pt core.Point) core.Node { return nil }

// OnMount captures the tree.
func (p *OverlayPortal) OnMount() {
	if p.Tree() != nil {
		p.tree = p.Tree()
		p.syncHost()
	}
}

// OnUnmount removes overlay entry.
func (p *OverlayPortal) OnUnmount() {
	p.removeHost()
	p.tree = nil
}

// SetOpen toggles visibility in the overlay host.
func (p *OverlayPortal) SetOpen(open bool) {
	if p.Open == open {
		// Still re-sync offset/content when staying open (anchor moves).
		if open {
			p.syncHost()
		}
		return
	}
	p.Open = open
	p.syncHost()
	if t := p.Tree(); t != nil {
		t.MarkDirty()
	} else if p.tree != nil {
		p.tree.MarkDirty()
	}
}

// SetContentOffset sets absolute position of the floating content.
func (p *OverlayPortal) SetContentOffset(pt core.Point) {
	p.ContentOffset = pt
	if p.Content != nil {
		p.Content.Base().SetOffset(pt)
	}
	p.syncHost()
}

func (p *OverlayPortal) syncHost() {
	t := p.Tree()
	if t == nil {
		t = p.tree
	}
	if t == nil || t.Overlays() == nil {
		return
	}
	if !p.Open || p.Content == nil {
		p.removeHost()
		return
	}
	// Mount content onto the tree so hover/press MarkNeedsPaint schedules frames
	// (content is not under root; without mount, chrome never repaints in overlays).
	t.AttachSubtree(p.Content)
	p.Content.Base().SetOffset(p.ContentOffset)
	if p.ID == "" {
		p.ID = nextPortalID()
	}
	t.Overlays().Push(core.OverlayEntry{
		ID: p.ID, Node: p.Content, ZOrder: p.ZOrder,
	})
	p.pushed = true
}

func (p *OverlayPortal) removeHost() {
	t := p.Tree()
	if t == nil {
		t = p.tree
	}
	if t == nil || t.Overlays() == nil {
		p.pushed = false
		return
	}
	if p.pushed {
		if p.ID != "" {
			t.Overlays().Remove(p.ID)
		}
		// Detach so nodes do not keep a stale tree pointer after close.
		if p.Content != nil {
			t.DetachSubtree(p.Content)
		}
		p.pushed = false
	}
}
