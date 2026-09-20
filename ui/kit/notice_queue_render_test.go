package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func TestNoticeQueue_GeometryMatchesCases(t *testing.T) {
	cases := loadNoticeCases(t)
	geo := cases["geometry"].(map[string]any)
	mb := geo["message_bar"].(map[string]any)
	bar := kit.ComputeNoticeQueueMessageBar(1200, 8, 0, kit.ZIndexForNoticeKind(1000, "message"))
	if bar.W != mb["want_w"].(float64) || bar.X != mb["want_x"].(float64) || bar.Y != mb["want_y"].(float64) {
		t.Fatalf("message bar = %.1f,%.1f,%.1f want 340,8,520", bar.X, bar.Y, bar.W)
	}
	tr := geo["notification_top_right"].(map[string]any)
	card := kit.ComputeNoticeQueueNotificationCard(1200, 800, kit.NoticeQueuePlacementTopRight, 0, 24, 24, kit.ZIndexForNoticeKind(1000, "notification"))
	if card.W != tr["want_w"].(float64) || card.X != tr["want_x"].(float64) || card.Y != tr["want_y"].(float64) {
		t.Fatalf("topRight card = %.1f,%.1f want 792,24", card.X, card.Y)
	}
	bl := geo["notification_bottom_left"].(map[string]any)
	blCard := kit.ComputeNoticeQueueNotificationCard(1200, 800, kit.NoticeQueuePlacementBottomLeft, 0, 24, 24, kit.ZIndexForNoticeKind(1000, "notification"))
	if blCard.W != bl["want_w"].(float64) || blCard.X != bl["want_x"].(float64) {
		t.Fatalf("bottomLeft card = %.1f want x24", blCard.X)
	}
	z := cases["z"].(map[string]any)
	if float64(kit.ZIndexForNoticeKind(1000, "message")) != z["message"].(float64) {
		t.Fatalf("message z = %d want 2010", kit.ZIndexForNoticeKind(1000, "message"))
	}
	if float64(kit.ZIndexForNoticeKind(1000, "notification")) != z["notification"].(float64) {
		t.Fatalf("notification z = %d want 2050", kit.ZIndexForNoticeKind(1000, "notification"))
	}
	st := cases["stack"].(map[string]any)
	over := st["over_5_threshold_3"].(map[string]any)
	vis, folded := kit.NoticeQueueVisibleCount(5, true, 3)
	if float64(vis) != over["visible"].(float64) || float64(folded) != over["folded"].(float64) {
		t.Fatalf("stack 5/3 = %d+%d want 1+4", vis, folded)
	}
	under := st["under_2_threshold_3"].(map[string]any)
	vis, folded = kit.NoticeQueueVisibleCount(2, true, 3)
	if float64(vis) != under["visible"].(float64) || float64(folded) != under["folded"].(float64) {
		t.Fatalf("stack 2/3 = %d+%d want 2+0", vis, folded)
	}
	if got := kit.ResolveNoticeQueueMessageTop(kit.DefaultNoticeQueueProps()); got != 8 {
		t.Fatalf("message top = %v want 8", got)
	}
	if got := kit.ResolveNoticeQueuePlacement(kit.DefaultNoticeQueueProps()); got != kit.NoticeQueuePlacementTopRight {
		t.Fatalf("placement = %q want topRight", got)
	}
}
