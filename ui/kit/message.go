package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// MessageHost renders a NotifyQueue as stacked toasts (top-right).
type MessageHost struct {
	Portal   *primitive.OverlayPortal
	Queue    *core.NotifyQueue
	Face     text.Face
	Theme    *core.Theme
	Viewport core.Size
	layer    *messageLayer
}

// NewMessageHost creates a message host with its own queue.
func NewMessageHost() *MessageHost {
	h := &MessageHost{Queue: core.NewNotifyQueue(5)}
	h.rebuild()
	h.Queue.OnChange = func() { h.refresh() }
	return h
}

type messageLayer struct {
	core.NodeBase
	host *MessageHost
}

// Node returns the portal node to mount.
func (h *MessageHost) Node() core.Node {
	if h.Portal == nil {
		h.rebuild()
	}
	return h.Portal
}

// Info pushes an info message.
func (h *MessageHost) Info(text string) {
	h.Queue.Push(core.NotifyItem{Content: text, Kind: "info", DurationMs: 3000})
}

// Success pushes a success message.
func (h *MessageHost) Success(text string) {
	h.Queue.Push(core.NotifyItem{Content: text, Kind: "success", DurationMs: 3000})
}

// Error pushes an error message.
func (h *MessageHost) Error(text string) {
	h.Queue.Push(core.NotifyItem{Content: text, Kind: "error", DurationMs: 4000})
}

// Notification pushes a titled notification (title + body).
func (h *MessageHost) Notification(title, body string) {
	content := title
	if body != "" {
		if content != "" {
			content = title + " — " + body
		} else {
			content = body
		}
	}
	h.Queue.Push(core.NotifyItem{Content: content, Kind: "info", DurationMs: 4000})
}

// Count returns the number of active messages.
func (h *MessageHost) Count() int {
	if h == nil || h.Queue == nil {
		return 0
	}
	return h.Queue.Len()
}

// Sync expires timed toasts and rebuilds the toast list.
// Deprecated: prefer Tree.Layout + AnchoredPopup.RefreshOpenGeometry (automatic).
// Kept for one-shot forced reposition after external layout changes.
func (h *MessageHost) Sync() {
	if h == nil {
		return
	}
	if h.Queue != nil {
		h.Queue.Expire(0)
	}
	h.refresh()
}

func (h *MessageHost) theme() *core.Theme {
	var n core.Node
	if h.Portal != nil {
		n = h.Portal
	}
	return themeOf(h.Theme, n)
}

func (h *MessageHost) rebuild() {
	if h.layer == nil {
		h.layer = &messageLayer{host: h}
		h.layer.Init(h.layer)
		h.layer.Hit = core.HitDefer
	}
	if h.Portal == nil {
		h.Portal = primitive.NewOverlayPortal(h.layer)
		// Empty ID → unique auto-id so Message + Notification hosts do not clobber.
		h.Portal.ID = ""
		h.Portal.ZOrder = OverlayZMessage
	} else {
		h.Portal.Content = h.layer
		h.Portal.ZOrder = OverlayZMessage
	}
	h.refresh()
}

func (h *MessageHost) refresh() {
	if h == nil {
		return
	}
	if h.layer == nil {
		h.rebuild()
		return
	}
	h.layer.ClearChildren()
	th := h.theme()
	col := primitive.Column()
	col.Gap = 8
	col.CrossAlign = core.CrossEnd
	for _, it := range h.Queue.Items() {
		tx := primitive.NewText(it.Content)
		tx.FontSize = th.SizeOr(core.TokenFontSize, 14)
		tx.Face = h.Face
		tx.Color = th.Color(core.TokenColorText)
		card := primitive.NewDecorated(tx)
		card.Padding = primitive.Symmetric(12, 9) // Ant Message
		card.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
		card.Background = th.Color(core.TokenColorBgContainer)
		card.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
		switch it.Kind {
		case "success":
			card.BorderColor = th.Color(core.TokenColorSuccess)
		case "error":
			card.BorderColor = th.Color(core.TokenColorError)
		case "warning":
			card.BorderColor = th.Color(core.TokenColorWarning)
		default:
			card.BorderColor = th.Color(core.TokenColorPrimary)
		}
		id := it.ID
		press := primitive.NewPressable(card)
		press.Click = func() { h.Queue.Remove(id) }
		col.AddChild(press)
	}
	h.layer.AddChild(col)
	// Open portal whenever there are toasts (Info/Success/… only used to Push;
	// without this the overlay never mounts and gallery looks "dead").
	if h.Portal != nil {
		h.Portal.SetOpen(h.Queue.Len() > 0)
	}
	h.layer.MarkNeedsLayout()
	h.layer.MarkNeedsPaint()
}

func (l *messageLayer) TypeID() string { return "kit.MessageLayer" }

func (l *messageLayer) Layout(c core.Constraints) core.Size {
	var portal *primitive.OverlayPortal
	var vp core.Size
	if l.host != nil {
		portal = l.host.Portal
		vp = l.host.Viewport
	}
	vw, vh := resolveOverlayViewport(vp, portal, c.MaxWidth, c.MaxHeight)
	// content top-right
	for _, child := range l.Children() {
		sz := child.Layout(core.Loose(320, vh))
		child.Base().SetOffset(core.Point{X: vw - sz.Width - 16, Y: 16})
	}
	out := core.Size{Width: vw, Height: vh}
	l.SetSize(out)
	return out
}

func (l *messageLayer) Paint(pc *core.PaintContext) { l.DefaultPaintChildren(pc) }

func (l *messageLayer) HitTest(p core.Point) core.Node { return l.DefaultHitTest(p) }
