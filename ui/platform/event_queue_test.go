//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package platform

import "testing"

// TestEventQueuePushWakes 钉结构化契约：Push 必然唤醒——「忘了唤醒」这个
// 洞类（连发分批病根）在 eventQueue 上写不出来。nil wak（host 未挂接的
// 裸构造态）只入队不唤醒，与旧行为一致。
func TestEventQueuePushWakes(t *testing.T) {
	var q eventQueue
	q.push(Event{Type: EventKey, KeyCode: 1})
	if evs := q.drain(); len(evs) != 1 || evs[0].KeyCode != 1 {
		t.Fatalf("zero-value push/drain = %+v", evs)
	}
	// 绑定唤醒后，push 必触发唤醒令牌。
	wakes := 0
	q.bindWake(func() { wakes++ })
	q.push(Event{Type: EventKey, KeyCode: 2})
	q.push(Event{Type: EventKey, KeyCode: 3})
	if wakes != 2 {
		t.Fatalf("push wakes = %d, want 2", wakes)
	}
	evs := q.drain()
	if len(evs) != 2 || evs[0].KeyCode != 2 || evs[1].KeyCode != 3 {
		t.Fatalf("drain order = %+v", evs)
	}
	// bindWake(nil) 清除绑定：push 恢复不唤醒。
	q.bindWake(nil)
	q.push(Event{Type: EventKey, KeyCode: 4})
	if wakes != 2 {
		t.Fatalf("bindWake(nil) must unbind, wakes = %d", wakes)
	}
	if len(q.drain()) != 1 {
		t.Fatal("drain after unbind")
	}
}
