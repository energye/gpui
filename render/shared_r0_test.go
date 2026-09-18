package render

import (
	"errors"
	"testing"
)

// R0-5: fair-share arithmetic must split the process budget across windows
// and never cap a lone window. Pure core — no GPU needed.
func TestPictureCacheFairMaxN_SplitsBudget(t *testing.T) {
	// 768MB / 2 windows / divisor 4 = 96MB per window;
	// 1MB entries → 96 entries.
	if got := pictureCacheFairMaxN(2, 768, 1<<20, 0); got != 96 {
		t.Fatalf("2 windows got %d want 96", got)
	}
	// 4 windows → 48MB → 48 entries of 1MB, raised to the 64 floor.
	if got := pictureCacheFairMaxN(4, 768, 1<<20, 0); got != minPictureCacheEntries {
		t.Fatalf("4 windows got %d want floor %d", got, minPictureCacheEntries)
	}
	// 4 windows of 256KB entries → 192, above the floor.
	if got := pictureCacheFairMaxN(4, 768, 256<<10, 0); got != 192 {
		t.Fatalf("4 windows small entries got %d want 192", got)
	}
	// Fallback bytes used when the cache is still empty.
	if got := pictureCacheFairMaxN(2, 768, 0, 1<<20); got != 96 {
		t.Fatalf("fallback got %d want 96", got)
	}
	// Absurdly large entries clamp to the 64-entry floor, never 0/negative.
	if got := pictureCacheFairMaxN(8, 768, 1<<30, 0); got != minPictureCacheEntries {
		t.Fatalf("huge entries got %d want floor %d", got, minPictureCacheEntries)
	}
}

func TestPictureCacheFairMaxN_NoCapAlone(t *testing.T) {
	if got := pictureCacheFairMaxN(1, 768, 1<<20, 0); got != 0 {
		t.Fatalf("single window got %d want 0 (no cap)", got)
	}
	if got := pictureCacheFairMaxN(0, 768, 1<<20, 0); got != 0 {
		t.Fatalf("zero windows got %d want 0", got)
	}
	if got := pictureCacheFairMaxN(2, 0, 1<<20, 0); got != 0 {
		t.Fatalf("disabled budget got %d want 0", got)
	}
}

func TestSharedWindowCount_NoShare(t *testing.T) {
	if got := SharedWindowCount(); got != 0 {
		t.Fatalf("test env has no windows, count=%d", got)
	}
	if got := PictureCacheFairMax(1<<20, 0); got != 0 {
		t.Fatalf("no share must not cap, got %d", got)
	}
}

// R0-6: with no share published, recovery must fail closed with the
// sentinel (never touch nil devices). No GPU needed.
func TestRecoverSharedDevice_NoShare(t *testing.T) {
	err := RecoverSharedDevice()
	if !errors.Is(err, errSharedNotOpen) {
		t.Fatalf("no-share recover err=%v want errSharedNotOpen", err)
	}
}
