package scene

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

// TestPictureTextureCache_RerecordAttribution locks the R10 per-key
// re-record attribution: each record counts to its cache key with the
// record cause (full vs full-extra vs local), and the snapshot is a copy.
func TestPictureTextureCache_RerecordAttribution(t *testing.T) {
	tex := NewPictureTextureCache(nil, 0)
	p := Picture{}
	if _, ok := tex.RecordForTest(11, &p); !ok && len(tex.entries) == 0 {
		t.Skip("no-GPU degrade: record unavailable in this env")
	}
	tex.RecordForTest(11, &p) // second record: cumulative for key 11
	tex.RecordForTest(12, &p) // another key
	snap := tex.RerecordByKeySnapshot()
	if len(snap) == 0 {
		t.Fatal("attribution snapshot empty after records")
	}
	if n := snap[11]["full"]; n != 2 {
		t.Fatalf("key 11 full=%d want 2", n)
	}
	if n := snap[12]["full"]; n != 1 {
		t.Fatalf("key 12 full=%d want 1", n)
	}
	// Snapshot must be a copy: mutating it must not touch the cache.
	snap[11]["full"] = 99
	if n := tex.RerecordByKeySnapshot()[11]["full"]; n != 2 {
		t.Fatalf("snapshot not a copy: key 11 full=%d want 2", n)
	}
}

// TestPictureTextureCache_RerecordAttributionLocal locks the local-cause
// attribution via the local record path (RecordLocalForTest if available,
// else recordLocalWith through the internal entry point).
func TestPictureTextureCache_RerecordAttributionLocal(t *testing.T) {
	tex := NewPictureTextureCache(nil, 0)
	tex.width, tex.height = 64, 64
	p := Picture{}
	b := image.Rect(0, 0, 32, 32)
	_, ok1 := tex.recordLocalWith(21, &p, b, nil)                       // cause "local"
	_, ok2 := tex.recordLocalWith(21, &p, b, func(*render.Context) {}) // cause "local-extra"
	if !ok1 || !ok2 {
		t.Skip("no-GPU degrade: local record unavailable in this env")
	}
	snap := tex.RerecordByKeySnapshot()
	if n := snap[21]["local"]; n != 1 {
		t.Fatalf("key 21 local=%d want 1 (snap=%v)", n, snap[21])
	}
	if n := snap[21]["local-extra"]; n != 1 {
		t.Fatalf("key 21 local-extra=%d want 1 (snap=%v)", n, snap[21])
	}
}
