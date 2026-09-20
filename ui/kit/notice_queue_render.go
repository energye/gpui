package kit

// Fixed notice geometry aligned to antd 6.5.1 component tokens.
// Painting stays in prim when F0-2 lands; this file only computes rects.
const (
	NoticeQueueMessageWidthMax   = 520.0
	NoticeQueueMessageItemH      = 40.0
	NoticeQueueMessageGap        = 8.0
	NoticeQueueNotificationWidth = 384.0
	NoticeQueueNotificationH     = 120.0
	NoticeQueueNotificationGap   = 16.0
	NoticeQueueEdge              = 24.0
)

// NoticeQueueRect is one card/bar rect in logical pixels.
type NoticeQueueRect struct {
	X, Y, W, H float64
	ZIndex     int
}

// ResolveNoticeQueueMessageTop returns the top-center offset (default 8).
func ResolveNoticeQueueMessageTop(p NoticeQueueProps) float64 {
	if p.MessageTopSet && p.MessageTop > 0 {
		return p.MessageTop
	}
	if p.MessageTop > 0 && p.MessageTopSet {
		return p.MessageTop
	}
	if !p.MessageTopSet && p.MessageTop > 0 {
		return p.MessageTop
	}
	return 8
}

// ResolveNoticeQueueMessageDuration returns the tip lifetime (default 3).
func ResolveNoticeQueueMessageDuration(p NoticeQueueProps) float64 {
	if p.MessageDurationSet {
		return p.MessageDuration
	}
	if p.MessageDuration > 0 {
		return p.MessageDuration
	}
	return 3
}

// ResolveNoticeQueueNotificationDuration returns the card lifetime (default 4.5).
func ResolveNoticeQueueNotificationDuration(p NoticeQueueProps) float64 {
	if p.NotificationDurSet {
		return p.NotificationDuration
	}
	if p.NotificationDuration > 0 {
		return p.NotificationDuration
	}
	return 4.5
}

// ResolveNoticeQueuePlacement returns the default corner (topRight).
func ResolveNoticeQueuePlacement(p NoticeQueueProps) NoticeQueuePlacement {
	if p.NotificationPlaceSet && p.NotificationPlacement != "" {
		return p.NotificationPlacement
	}
	if p.NotificationPlacement != "" {
		return p.NotificationPlacement
	}
	return NoticeQueuePlacementTopRight
}

// ZIndexForNoticeKind returns 2010 for message and 2050 for notification
// off base 1000 (antd zIndexPopup message +10, notification +50 over the
// 1000 container offset).
func ZIndexForNoticeKind(base int, kind string) int {
	if base == 0 {
		base = 1000
	}
	if kind == "message" {
		return base + 1010
	}
	return base + 1050
}

// ComputeNoticeQueueMessageBar centers one tip; Y stacks from top.
func ComputeNoticeQueueMessageBar(viewportW float64, top float64, index int, zIndex int) NoticeQueueRect {
	w := NoticeQueueMessageWidthMax
	if w > viewportW-48 {
		w = viewportW - 48
	}
	if w < 120 {
		w = 120
	}
	x := (viewportW - w) / 2
	y := top + float64(index)*(NoticeQueueMessageItemH+NoticeQueueMessageGap)
	return NoticeQueueRect{X: x, Y: y, W: w, H: NoticeQueueMessageItemH, ZIndex: zIndex}
}

// ComputeNoticeQueueNotificationCard places one card inside its corner pool.
func ComputeNoticeQueueNotificationCard(viewportW, viewportH float64, pl NoticeQueuePlacement, index int, topEdge, bottomEdge float64, zIndex int) NoticeQueueRect {
	w := NoticeQueueNotificationWidth
	if w > viewportW-48 {
		w = viewportW - 48
	}
	h := NoticeQueueNotificationH
	var x float64
	switch pl {
	case NoticeQueuePlacementTopLeft, NoticeQueuePlacementBottomLeft:
		x = NoticeQueueEdge
	case NoticeQueuePlacementTop, NoticeQueuePlacementBottom:
		x = (viewportW - w) / 2
	default:
		x = viewportW - NoticeQueueEdge - w
	}
	var y float64
	switch pl {
	case NoticeQueuePlacementBottom, NoticeQueuePlacementBottomLeft, NoticeQueuePlacementBottomRight:
		if bottomEdge <= 0 {
			bottomEdge = NoticeQueueEdge
		}
		y = viewportH - bottomEdge - float64(index+1)*h - float64(index)*NoticeQueueNotificationGap
	default:
		if topEdge <= 0 {
			topEdge = NoticeQueueEdge
		}
		y = topEdge + float64(index)*(h+NoticeQueueNotificationGap)
	}
	return NoticeQueueRect{X: x, Y: y, W: w, H: h, ZIndex: zIndex}
}

// NoticeQueueVisibleCount folds the queue when stack is over threshold:
// at most threshold triggers, over threshold only the latest shows.
func NoticeQueueVisibleCount(total int, enabled bool, threshold int) (visible int, folded int) {
	if threshold <= 0 {
		threshold = 3
	}
	if !enabled || total <= threshold {
		return total, 0
	}
	return 1, total - 1
}
