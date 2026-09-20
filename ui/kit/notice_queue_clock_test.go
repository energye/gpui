package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func TestNoticeQueue_VirtualClockFreezesOnHover(t *testing.T) {
	host := kit.BuildNoticeQueue(kit.DefaultScopeCtx(), kit.DefaultNoticeQueueProps())
	mcfg := kit.DefaultNoticeQueueMessageConfig()
	mcfg.Key, mcfg.Content = "hover-me", "hi"
	mcfg.Duration, mcfg.DurationSet = 3, true
	h := host.OpenMessage(mcfg)
	host.SetMessageHovered("hover-me", true)
	host.Tick(2)
	msgs := host.Messages()
	if len(msgs) != 1 {
		t.Fatalf("hovered tip must survive tick, count = %d", len(msgs))
	}
	if got := host.MessageProgress(h.ID()); got != 0 {
		t.Fatalf("hovered progress = %v want 0 (frozen)", got)
	}
	host.SetMessageHovered("hover-me", false)
	host.Tick(1)
	if got := host.MessageProgress(h.ID()); got < 0.3 || got > 0.4 {
		t.Fatalf("resumed progress = %v want ~0.33", got)
	}
	host.Tick(2)
	if host.MessageCount() != 0 {
		t.Fatal("tip must expire after total 3s of unhovered time")
	}
	select {
	case r := <-h.Done():
		if !r.Closed {
			t.Fatal("expiry must resolve closed")
		}
	default:
		t.Fatal("expired promise must resolve")
	}

	ncfg := kit.DefaultNoticeQueueNotificationConfig()
	ncfg.Key, ncfg.Title, ncfg.Description = "n-hover", "Title", "desc"
	ncfg.Duration, ncfg.DurationSet = 4.5, true
	ncfg.ShowProgress = true
	nh := host.OpenNotification(ncfg)
	host.SetNotificationHovered("n-hover", true)
	host.Tick(4.5)
	if host.NotificationCount() != 1 {
		t.Fatal("hovered card must freeze full duration")
	}
	if got := host.NotificationProgress(nh.ID()); got != 0 {
		t.Fatalf("hovered card progress = %v want 0", got)
	}
	host.SetNotificationHovered("n-hover", false)
	host.Tick(4.5)
	if host.NotificationCount() != 0 {
		t.Fatal("card must expire after unhovered 4.5s")
	}

	// Persist duration never expires.
	pcfg := kit.DefaultNoticeQueueMessageConfig()
	pcfg.Key, pcfg.Content = "persist", "stay"
	pcfg.Duration, pcfg.DurationSet = 0, true
	host.OpenMessage(pcfg)
	host.Tick(100)
	if host.MessageCount() != 1 {
		t.Fatal("duration 0 must persist through any tick")
	}
	host.DestroyMessage("persist")
}
