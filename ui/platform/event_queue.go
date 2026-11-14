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

import "sync"

// eventQueue 是「生产者 → 事件循环」的交接队列：Push 必然唤醒停着的循环。
//
// 契约背景：平台层事件主循环空闲时停在 WaitEvents 的 deadline park（显示
// fd + wake pipe），只 append 不唤醒的队列会让事件积压到下一个 ticker 截止
// （闪烁/HUD，~500ms）才被一次倒出——2026-11-14 按住连发「一批一批」的病根
// 正是 pushKey 塞队列不唤醒（Wayland fireRepeat 在定时器 goroutine 上）。
// 入队与唤醒收成结构后，「忘了唤醒」这个洞类写不出来；dispatch 线程内的
// Push 多花一个随后被排空的管道字节，无害。
//
// 零值可用：wak 为 nil 时只入队不唤醒（host 未挂接/测试裸构造的防御态，
// 与旧行为一致）；host 挂接点调用 bindWake 统一绑定唤醒通道（waylandCreate
// 设 hostRef 后 / x11Host 构造后），推迟解析避免构造顺序依赖。
type eventQueue struct {
	mu  sync.Mutex
	evs []Event
	wak func()
}

// bindWake 绑定唤醒通道（host 挂接点调用一次）；nil 清除绑定。
func (q *eventQueue) bindWake(wak func()) {
	q.mu.Lock()
	q.wak = wak
	q.mu.Unlock()
}

// push 入队一个事件并唤醒停着的循环（线程安全）。
func (q *eventQueue) push(ev Event) {
	q.mu.Lock()
	q.evs = append(q.evs, ev)
	q.mu.Unlock()
	if q.wak != nil {
		q.wak()
	}
}

// drain 取走全部积压事件（循环线程调用），保持入队顺序。
func (q *eventQueue) drain() []Event {
	q.mu.Lock()
	out := q.evs
	q.evs = nil
	q.mu.Unlock()
	return out
}
