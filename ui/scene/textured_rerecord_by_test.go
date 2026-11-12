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

// O4: the diagnostics attribution map must not grow with dead cache keys.
// A virtual list visits a new layer key per scrolled row; keeping every key
// forever leaked ~1.3MB/min in the 300s m4 soak. BeginFrame already sweeps
// admission counts against liveKeys; the attribution map must join it.
func TestPictureTextureCache_RerecordAttributionSweepsDeadKeys(t *testing.T) {
	tex := NewPictureTextureCache(nil, 0)
	tex.mu.Lock()
	tex.noteRerecord(11, "full")
	tex.noteRerecord(11, "full")
	tex.noteRerecord(22, "local")
	tex.mu.Unlock()

	tex.SetLiveKeys([]uint64{11})
	tex.BeginFrame()
	snap := tex.RerecordByKeySnapshot()
	if _, ok := snap[22]; ok {
		t.Fatalf("dead key 22 kept after BeginFrame: %v", snap)
	}
	if n := snap[11]["full"]; n != 2 {
		t.Fatalf("live key 11 full=%d want 2 (snap=%v)", n, snap)
	}
}

// Without live-key info (unit tests / non-retained users) the map is kept:
// under-counting attribution is worse than keeping it.
func TestPictureTextureCache_RerecordAttributionKeptWithoutLiveKeys(t *testing.T) {
	tex := NewPictureTextureCache(nil, 0)
	tex.mu.Lock()
	tex.noteRerecord(33, "full")
	tex.mu.Unlock()

	tex.BeginFrame()
	if n := tex.RerecordByKeySnapshot()[33]["full"]; n != 1 {
		t.Fatalf("key 33 full=%d want 1 without live keys", n)
	}
}

// O4: attribution is diagnostics, not correctness; it must be hard-bounded.
// m4 keeps measured rows in the live set, so the live-key sweep alone cannot
// stop growth (300s reached 6.5MB). Cap at maxRerecordAttrEntries with an
// oldest-first eviction that prefers dead keys.
func TestPictureTextureCache_RerecordAttributionBounded(t *testing.T) {
	const capN = maxRerecordAttrEntries
	tex := NewPictureTextureCache(nil, 0)
	tex.mu.Lock()
	for i := uint64(1); i <= capN+32; i++ {
		tex.noteRerecord(i, "full")
	}
	tex.mu.Unlock()
	snap := tex.RerecordByKeySnapshot()
	if len(snap) > capN {
		t.Fatalf("attribution map=%d want <=%d", len(snap), capN)
	}
	if _, ok := snap[capN+32]; !ok {
		t.Fatal("newest key evicted; want oldest-first eviction")
	}
}

// A live (in-tree) key must survive eviction even when it is the oldest.
func TestPictureTextureCache_RerecordAttributionKeepsLiveKey(t *testing.T) {
	const capN = maxRerecordAttrEntries
	tex := NewPictureTextureCache(nil, 0)
	tex.SetLiveKeys([]uint64{1})
	tex.mu.Lock()
	for i := uint64(1); i <= capN+32; i++ {
		tex.noteRerecord(i, "full")
	}
	tex.mu.Unlock()
	if _, ok := tex.RerecordByKeySnapshot()[1]; !ok {
		t.Fatal("live key 1 evicted; eviction must prefer dead keys")
	}
}
