package kit_test

import (
	"testing"
	"time"

	"github.com/energye/gpui/ui/kit"
)

func TestNoticeQueue_QueueDropsOldestKeyUpdateOnce(t *testing.T) {
	props := kit.DefaultNoticeQueueProps()
	props.MaxCount = 2
	host := kit.BuildNoticeQueue(kit.DefaultScopeCtx(), props)
	mk := func(key, content string) kit.NoticeQueueMessageConfig {
		cfg := kit.DefaultNoticeQueueMessageConfig()
		cfg.Key, cfg.Content = key, content
		cfg.Duration, cfg.DurationSet = 10, true
		return cfg
	}
	h1 := host.OpenMessage(mk("k1", "one"))
	host.OpenMessage(mk("k2", "two"))
	host.OpenMessage(mk("k3", "three"))
	if host.MessageCount() != 2 {
		t.Fatalf("message count = %d want 2 (drop oldest)", host.MessageCount())
	}
	msgs := host.Messages()
	if msgs[0].Key != "k2" || msgs[1].Key != "k3" {
		t.Fatalf("after maxCount want k2,k3 got %q,%q", msgs[0].Key, msgs[1].Key)
	}
	select {
	case r := <-h1.Done():
		if !r.Closed {
			t.Fatal("dropped oldest must resolve closed")
		}
	default:
		t.Fatal("dropped handle promise must resolve")
	}
	// Same key updates in place, no new entry.
	h2 := host.OpenMessage(mk("k2", "two-updated"))
	if host.MessageCount() != 2 {
		t.Fatalf("key update must not add, count = %d", host.MessageCount())
	}
	msgs = host.Messages()
	if msgs[0].Content != "two-updated" {
		t.Fatalf("key update content = %q", msgs[0].Content)
	}
	if h2.ID() == 0 {
		t.Fatal("key update must return live handle")
	}
	// Promise exactly once with Then.
	calls := 0
	done := make(chan struct{}, 1)
	h := host.OpenMessage(mk("once", "x"))
	h.Then(func() { calls++; done <- struct{}{} })
	h.Destroy()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Then must fire on destroy")
	}
	select {
	case r := <-h.Done():
		if !r.Closed {
			t.Fatal("destroy must resolve closed")
		}
	default:
		t.Fatal("destroyed promise must resolve")
	}
	extra := 0
	h.Then(func() { extra++ })
	if extra != 1 {
		t.Fatalf("late Then must fire once, got %d", extra)
	}
	if calls != 1 {
		t.Fatalf("Then calls = %d want 1", calls)
	}
	// Notification pools stay independent.
	nprops := kit.DefaultNoticeQueueProps()
	nhost := kit.BuildNoticeQueue(kit.DefaultScopeCtx(), nprops)
	nmk := func(key, title, pl string) kit.NoticeQueueNotificationConfig {
		cfg := kit.DefaultNoticeQueueNotificationConfig()
		cfg.Key, cfg.Title, cfg.Description = key, title, "desc"
		cfg.Duration, cfg.DurationSet = 10, true
		if pl != "" {
			cfg.Placement, cfg.PlacementSet = kit.NoticeQueuePlacement(pl), true
		}
		return cfg
	}
	nhost.OpenNotification(nmk("a", "t1", "topRight"))
	nhost.OpenNotification(nmk("b", "t2", "bottomLeft"))
	if nhost.NotificationCount(kit.NoticeQueuePlacementTopRight) != 1 {
		t.Fatal("topRight pool must hold 1")
	}
	if nhost.NotificationCount(kit.NoticeQueuePlacementBottomLeft) != 1 {
		t.Fatal("bottomLeft pool must hold 1")
	}
	if nhost.NotificationCount() != 2 {
		t.Fatalf("total = %d want 2", nhost.NotificationCount())
	}
	nhost.DestroyNotification("a")
	if nhost.NotificationCount(kit.NoticeQueuePlacementTopRight) != 0 {
		t.Fatal("destroy key must clear only that pool entry")
	}
	if nhost.NotificationCount() != 1 {
		t.Fatal("neighbour pool must survive single destroy")
	}
	nhost.DestroyAll()
	if nhost.NotificationCount() != 0 || nhost.MessageCount() != 0 {
		t.Fatal("destroyAll must clear both queues")
	}
}
