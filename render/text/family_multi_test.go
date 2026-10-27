package text

import (
	"sync"
	"testing"
)

// TestSharedSourceInMultipleFamilies pins the behavior needed once file
// sources are process-wide shared: a source can belong to several families,
// registration order is the tie-break, and concurrent Face calls are safe.
func TestSharedSourceInMultipleFamilies(t *testing.T) {
	regular := cacheTestLoad(t, "testdata/wqy-microhei.ttf")
	boldA := cacheTestLoad(t, "testdata/wqy-microhei-nohint.ttf")
	boldB := cacheTestLoad(t, "testdata/goregular.ttf")

	newFamily := func(bold *FontSource) *FontFamily {
		return &FontFamily{
			byWeight: map[FontWeight]*FontSource{
				WeightRegular: regular,
				WeightBold:    bold,
			},
			sources: []*FontSource{regular, bold},
			regular: regular,
		}
	}
	famA := newFamily(boldA)
	famB := newFamily(boldB)

	regular.addFamily(famA)
	regular.addFamily(famB)
	regular.addFamily(famA) // duplicate registration is a no-op

	fams := regular.families.Load()
	if fams == nil || len(*fams) != 2 {
		t.Fatalf("families = %v, want 2 entries", fams)
	}
	if (*fams)[0] != famA || (*fams)[1] != famB {
		t.Fatal("family registration order not preserved")
	}

	// First family that resolves the weight wins: famA's Bold cut.
	face := regular.Face(16, WithWeight(WeightBold))
	sf, ok := face.(*sourceFace)
	if !ok {
		t.Fatalf("expected *sourceFace, got %T", face)
	}
	if sf.source != boldA {
		t.Errorf("Face(Bold) used %p, want famA's bold cut %p", sf.source, boldA)
	}

	// No weight request keeps the source as-is.
	plain := regular.Face(16)
	if ps, ok := plain.(*sourceFace); !ok || ps.source != regular {
		t.Errorf("Face() without weight should stay on the source")
	}

	// Concurrent family selection must be race-free and stable.
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f := regular.Face(18, WithWeight(WeightBold))
			if s, ok := f.(*sourceFace); !ok || s.source != boldA {
				t.Errorf("concurrent Face(Bold) picked %p, want %p", s, boldA)
			}
		}()
	}
	wg.Wait()
}
