//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
)

func effectTestPath() *render.Path {
	p := render.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(100, 0)
	p.LineTo(100, 60)
	return p
}

// TestDashGeomCacheGetOrApplyEffect pins the P3.5 batch 4 cache contract:
// miss → compute + store; identical (path, effect, scale) → shared pointer
// hit; a different effect or scale never shares the entry; rejects store
// nothing.
func TestDashGeomCacheGetOrApplyEffect(t *testing.T) {
	dc := NewDashGeometryCache()
	line := effectTestPath()
	dash1 := render.NewDashPathEffect(render.NewDash(10, 5))
	dash2 := render.NewDashPathEffect(render.NewDash(8, 4))

	a := dc.GetOrApplyEffect(line, dash1, 1)
	if a == nil || a.NumVerbs() == 0 {
		t.Fatal("first miss produced no geometry")
	}
	if _, misses, entries := dc.Stats(); misses != 1 || entries != 1 {
		t.Fatalf("after first apply: misses=%d entries=%d, want 1/1", misses, entries)
	}

	b := dc.GetOrApplyEffect(line, dash1, 1)
	if b != a {
		t.Fatal("identical request must return the cached shared path")
	}
	if hits, _, _ := dc.Stats(); hits != 1 {
		t.Fatalf("hits = %d, want 1", hits)
	}

	// Equal-parameter effects hit the same entry (hash is by value).
	dash1copy := render.NewDashPathEffect(render.NewDash(10, 5))
	if c := dc.GetOrApplyEffect(line, dash1copy, 1); c != a {
		t.Fatal("equal-parameter effect missed the cache")
	}

	if d := dc.GetOrApplyEffect(line, dash2, 1); d == a || d == nil {
		t.Fatal("different dash pattern must not share the entry")
	}
	if _, misses, entries := dc.Stats(); misses != 2 || entries != 2 {
		t.Fatalf("after second pattern: misses=%d entries=%d, want 2/2", misses, entries)
	}

	// Different transform scale keys apart (user-space lengths vs device).
	if e := dc.GetOrApplyEffect(line, dash1, 2); e == a || e == nil {
		t.Fatal("scaled effect must not reuse the unscaled entry")
	}

	// Rejecting effect stores nothing and returns nil.
	empty := render.NewPath()
	if f := dc.GetOrApplyEffect(empty, dash1, 1); f != nil {
		t.Fatal("rejecting effect must yield nil")
	}
	if dc.GetOrApplyEffect(line, nil, 1) != nil {
		t.Fatal("nil effect must return nil")
	}
}

// valShiftEffect is a value-receiver custom effect (not pointer-shaped): it
// cannot be identified by address, so the cache must never key on it.
type valShiftEffect struct{ shift float64 }

func (v valShiftEffect) FilterPath(p *render.Path) (*render.Path, bool) {
	return p.Transform(render.Translate(0, v.shift)), true
}

// TestDashGeomCacheUnkeyableEffectNeverSharesAnEntry pins the P3.5 batch 4
// cache-safety contract: two different value-type effects must produce their
// own geometry and must not be stored under one key.
func TestDashGeomCacheUnkeyableEffectNeverSharesAnEntry(t *testing.T) {
	dc := NewDashGeometryCache()
	line := effectTestPath()

	_, _, before := dc.Stats()
	a := dc.GetOrApplyEffect(line, valShiftEffect{shift: 0}, 1)
	b := dc.GetOrApplyEffect(line, valShiftEffect{shift: 40}, 1)
	if a == nil || b == nil {
		t.Fatal("unkeyable effect must still be applied, not dropped")
	}
	if a == b {
		t.Fatal("two different value-type effects shared one cached path")
	}
	ay, by := a.Coords()[1], b.Coords()[1]
	if !nearFloat(by-ay, 40) {
		t.Fatalf("shift not applied: a.y=%v b.y=%v", ay, by)
	}
	_, _, after := dc.Stats()
	if after != before {
		t.Fatalf("unkeyable effects entered the cache (%d → %d entries)", before, after)
	}
}

func nearFloat(v, want float64) bool {
	d := v - want
	if d < 0 {
		d = -d
	}
	return d <= 1e-9
}
