package core

import "time"

// NotifyItem is one toast/message entry (C-NotifyQueue).
type NotifyItem struct {
	ID      string
	Content string
	// Kind: info/success/warning/error (product chrome).
	Kind string
	// DurationMs 0 = sticky until close.
	DurationMs int
	// CreatedAtMs wall clock (Unix milli); set on Push if zero. Used by Expire.
	CreatedAtMs int64
	// Seq is a monotonic sequence number for ordering.
	Seq int
}

// NotifyQueue is a simple FIFO/stack queue with max count.
type NotifyQueue struct {
	items    []NotifyItem
	max      int
	seq      int
	OnChange func()
}

// NewNotifyQueue creates a queue with maxCount (0 → 5).
func NewNotifyQueue(maxCount int) *NotifyQueue {
	if maxCount <= 0 {
		maxCount = 5
	}
	return &NotifyQueue{max: maxCount}
}

// Items returns a snapshot (newest last).
func (q *NotifyQueue) Items() []NotifyItem {
	if q == nil {
		return nil
	}
	out := make([]NotifyItem, len(q.items))
	copy(out, q.items)
	return out
}

// Push appends an item, dropping oldest if over max.
func (q *NotifyQueue) Push(it NotifyItem) string {
	if q == nil {
		return ""
	}
	q.seq++
	if it.ID == "" {
		it.ID = formatOverlayID(q.seq) // reuse small id helper
	}
	it.Seq = q.seq
	if it.CreatedAtMs == 0 {
		it.CreatedAtMs = time.Now().UnixMilli()
	}
	q.items = append(q.items, it)
	for len(q.items) > q.max {
		q.items = q.items[1:]
	}
	if q.OnChange != nil {
		q.OnChange()
	}
	return it.ID
}

// Remove drops by id.
func (q *NotifyQueue) Remove(id string) {
	if q == nil {
		return
	}
	for i, it := range q.items {
		if it.ID == id {
			q.items = append(q.items[:i], q.items[i+1:]...)
			if q.OnChange != nil {
				q.OnChange()
			}
			return
		}
	}
}

// Clear removes all.
func (q *NotifyQueue) Clear() {
	if q == nil {
		return
	}
	q.items = nil
	if q.OnChange != nil {
		q.OnChange()
	}
}

// Len returns count.
func (q *NotifyQueue) Len() int {
	if q == nil {
		return 0
	}
	return len(q.items)
}

// Expire removes items whose DurationMs has elapsed since CreatedAtMs.
// DurationMs <= 0 means sticky. nowMs <= 0 uses time.Now(). Returns true if any removed.
func (q *NotifyQueue) Expire(nowMs int64) bool {
	if q == nil || len(q.items) == 0 {
		return false
	}
	if nowMs <= 0 {
		nowMs = time.Now().UnixMilli()
	}
	n := q.items[:0]
	changed := false
	for _, it := range q.items {
		if it.DurationMs > 0 && it.CreatedAtMs > 0 && nowMs-it.CreatedAtMs >= int64(it.DurationMs) {
			changed = true
			continue
		}
		n = append(n, it)
	}
	if !changed {
		return false
	}
	for i := len(n); i < len(q.items); i++ {
		q.items[i] = NotifyItem{}
	}
	q.items = n
	if q.OnChange != nil {
		q.OnChange()
	}
	return true
}
