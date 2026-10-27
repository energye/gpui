package text

import (
	"path/filepath"
	"testing"
)

// TestFallbackFaceCacheSharesPerPath verifies the fallback face cache is keyed
// by (font file, size), not (rune, size): many runes served by the same system
// font must share one FontSource and one Face per size.
func TestFallbackFaceCacheSharesPerPath(t *testing.T) {
	fontPath, err := filepath.Abs("testdata/wqy-microhei.ttf")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	g := globalFallback
	g.ensure()

	g.mu.Lock()
	g.pathCache = map[rune]string{
		'\u4e16': fontPath, // 世
		'\u754c': fontPath, // 界
	}
	g.faceCache = make(map[fallbackFaceKey]Face)
	g.mu.Unlock()
	t.Cleanup(ClearFontScanFallbackCache)

	f1 := g.resolveFace('\u4e16', 14)
	f2 := g.resolveFace('\u754c', 14)
	f3 := g.resolveFace('\u4e16', 20)
	f4 := g.resolveFace('\u754c', 20)
	for i, f := range []Face{f1, f2, f3, f4} {
		if f == nil {
			t.Fatalf("resolveFace call %d returned nil", i)
		}
	}

	if f1 != f2 {
		t.Error("two runes served by the same file must share the Face at the same size")
	}
	if f3 != f4 {
		t.Error("two runes served by the same file must share the Face at the same size")
	}
	if f1 == f3 {
		t.Error("different sizes must produce different faces")
	}

	srcOf := func(f Face) *FontSource {
		sf, ok := f.(*sourceFace)
		if !ok {
			t.Fatalf("expected *sourceFace, got %T", f)
		}
		return sf.source
	}
	if srcOf(f1) != srcOf(f3) {
		t.Error("both sizes must resolve to the same shared FontSource")
	}

	g.mu.Lock()
	n := len(g.faceCache)
	g.mu.Unlock()
	if n != 2 {
		t.Errorf("faceCache entries = %d, want 2 (one per size, not per rune)", n)
	}
}

// TestFallbackResolveWithoutScan verifies a preset path cache still resolves
// when the system font index is unavailable (scan failure degrades cleanly).
func TestFallbackResolveWithoutScan(t *testing.T) {
	fontPath, err := filepath.Abs("testdata/wqy-microhei.ttf")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	f := &fontScanFallback{
		pathCache: map[rune]string{'A': fontPath},
		faceCache: make(map[fallbackFaceKey]Face),
	}
	// force ensure() to a no-op so fontMap stays nil
	f.initOnce.Do(func() {})

	if face := f.resolveFace('A', 12); face == nil {
		t.Fatal("expected a face from a preset path even without a system index")
	}
	if face := f.resolveFace('B', 12); face != nil {
		t.Error("expected nil for a rune with no preset path and no index")
	}
}
