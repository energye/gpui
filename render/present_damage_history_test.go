package render

import (
	"image"
	"testing"
)

// History starts empty: empty push returns empty, nothing stored.
func TestPushDamageHistory_EmptyStaysEmpty(t *testing.T) {
	tg := &PresentTarget{}
	if got := tg.pushDamageHistory(image.Rectangle{}); !got.Empty() {
		t.Fatalf("empty push should return empty, got %v", got)
	}
	if len(tg.damageHistory) != 0 {
		t.Fatalf("empty push should store nothing, got %d entries", len(tg.damageHistory))
	}
	if got := damageToRects(image.Rectangle{}); got != nil {
		t.Fatalf("empty union should map to nil (full present), got %v", got)
	}
}

// One push returns the rect itself; a second overlapping push unions.
func TestPushDamageHistory_UnionsRecent(t *testing.T) {
	tg := &PresentTarget{}
	r1 := image.Rect(0, 0, 10, 10)
	if got := tg.pushDamageHistory(r1); got != r1 {
		t.Fatalf("first push should return the rect, got %v", got)
	}
	r2 := image.Rect(5, 5, 20, 20)
	want := image.Rect(0, 0, 20, 20)
	if got := tg.pushDamageHistory(r2); got != want {
		t.Fatalf("second push should union to %v, got %v", want, got)
	}
	if len(tg.damageHistory) != 2 {
		t.Fatalf("history should hold 2 entries, got %d", len(tg.damageHistory))
	}
}

// History is bounded: older entries age out past DamageHistoryDepth.
func TestPushDamageHistory_BoundedDepth(t *testing.T) {
	tg := &PresentTarget{}
	for i := 0; i < DamageHistoryDepth+3; i++ {
		tg.pushDamageHistory(image.Rect(i*100, 0, i*100+10, 10))
	}
	if len(tg.damageHistory) != DamageHistoryDepth {
		t.Fatalf("history should cap at %d, got %d", DamageHistoryDepth, len(tg.damageHistory))
	}
	// Oldest surviving entry is the 4th pushed (index 3).
	if tg.damageHistory[0].Min.X != 300 {
		t.Fatalf("oldest entry should have aged out, head is %v", tg.damageHistory[0])
	}
}

// Clear drops everything so the next presents go full until fresh damage
// accumulates (reconfigure/resize path).
func TestClearDamageHistory_ResetsToFull(t *testing.T) {
	tg := &PresentTarget{}
	tg.pushDamageHistory(image.Rect(0, 0, 10, 10))
	tg.clearDamageHistory()
	if len(tg.damageHistory) != 0 {
		t.Fatalf("clear should empty history, got %d entries", len(tg.damageHistory))
	}
	r := image.Rect(40, 40, 50, 50)
	if got := tg.pushDamageHistory(r); got != r {
		t.Fatalf("push after clear should return only the new rect, got %v", got)
	}
}

// damageToRects maps a non-empty union to a single-rect list.
func TestDamageToRects_SingleRect(t *testing.T) {
	u := image.Rect(10, 20, 30, 40)
	rects := damageToRects(u)
	if len(rects) != 1 || rects[0] != u {
		t.Fatalf("union should map to single-rect list, got %v", rects)
	}
}
