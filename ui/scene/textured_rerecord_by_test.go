package scene

import (
	"image"
	"testing"
)

// TestPictureTextureCache_RerecordAttribution locks the R10 per-key
// re-record attribution: each record counts to its cache key with the
// record cause (full vs local), and the snapshot is a copy.
func TestPictureTextureCache_RerecordAttribution(t *testing.T) {
	tex := NewPictureTextureCache(nil, 0)
	rec := fillRecording(image.Rect(0, 0, 10, 8))
	if _, ok := tex.RecordForTest(11, rec); !ok && len(tex.entries) == 0 {
		t.Skip("no-GPU degrade: record unavailable in this env")
	}
	tex.RecordForTest(11, rec) // second record: cumulative for key 11
	tex.RecordForTest(12, rec) // another key
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

// TestPictureTextureCache_RerecordAttributionLocal locks the two remaining
// record causes on one key: the bounds-sized local record ("local") and the
// full-window record ("full"). The two extra-callback causes ("full-extra" /
// "local-extra") are gone — a recording is the only content a layer carries.
func TestPictureTextureCache_RerecordAttributionLocal(t *testing.T) {
	tex := NewPictureTextureCache(nil, 0)
	tex.width, tex.height = 64, 64
	rec := fillRecording(image.Rect(0, 0, 32, 32))
	b := image.Rect(0, 0, 32, 32)
	_, ok1 := tex.recordLocal(21, rec, b) // cause "local"
	_, ok2 := tex.record(21, rec)         // cause "full"
	if !ok1 || !ok2 {
		t.Skip("no-GPU degrade: local record unavailable in this env")
	}
	snap := tex.RerecordByKeySnapshot()
	if n := snap[21]["local"]; n != 1 {
		t.Fatalf("key 21 local=%d want 1 (snap=%v)", n, snap[21])
	}
	if n := snap[21]["full"]; n != 1 {
		t.Fatalf("key 21 full=%d want 1 (snap=%v)", n, snap[21])
	}
}
