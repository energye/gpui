package video

import "testing"

// TestPoolReuse pins the VR7 pool contract: second borrow after return is
// a hit with identical size, no growth.
func TestPoolReuse(t *testing.T) {
	p := NewPool("rgba", 16, 1<<20)
	a := p.Acquire()
	if len(a) != 16 {
		t.Fatalf("acquire len = %d, want 16", len(a))
	}
	if st := p.Stats(); st.Hits != 0 || st.Misses != 1 {
		t.Fatalf("first acquire = %+v, want 0 hits 1 miss", st)
	}
	a[0] = 7
	p.Release(a)
	b := p.Acquire()
	if len(b) != 16 {
		t.Fatalf("reacquire len = %d", len(b))
	}
	st := p.Stats()
	if st.Hits != 1 || st.Acquires != 2 {
		t.Fatalf("stats = %+v, want 2 acquires 1 hit", st)
	}
	if st.HitPct != 50.0 {
		t.Fatalf("hitpct = %v, want 50", st.HitPct)
	}
	p.Release(b)
	if p.Outstanding() != 0 {
		t.Fatalf("outstanding = %d, want 0 (no leak)", p.Outstanding())
	}
}

// TestPoolCap pins the cap: overflow releases are dropped and counted,
// never silently parked.
func TestPoolCap(t *testing.T) {
	p := NewPool("work", 8, 8)
	a := p.Acquire()
	b := p.Acquire()
	p.Release(a)
	p.Release(b)
	st := p.Stats()
	if st.Evictions != 1 {
		t.Fatalf("evictions = %d, want 1", st.Evictions)
	}
	if st.HeldBytes != 8 {
		t.Fatalf("held = %d, want 8", st.HeldBytes)
	}
	// Wrong size never parks.
	p.Release(make([]byte, 4))
	if st := p.Stats(); st.Evictions != 2 {
		t.Fatalf("evictions = %d, want 2 after wrong size", st.Evictions)
	}
}

// TestPoolsTriple pins three-pool independence: pressure on one never
// feeds another, min-hit skips empty pools honestly.
func TestPoolsTriple(t *testing.T) {
	ps := NewPools(96, 96, 1<<10, 1<<20)
	if ps.YUV.BufSize() != YUVBytes(96, 96) {
		t.Fatalf("yuv size = %d", ps.YUV.BufSize())
	}
	if ps.RGBA.BufSize() != RGBABytes(96, 96) {
		t.Fatalf("rgba size = %d", ps.RGBA.BufSize())
	}
	if ps.HitPctMin() != 0 {
		t.Fatalf("empty min = %v, want 0", ps.HitPctMin())
	}
	a := ps.RGBA.Acquire()
	ps.RGBA.Release(a)
	_ = ps.RGBA.Acquire()
	if m := ps.HitPctMin(); m != 50.0 {
		t.Fatalf("min = %v, want 50 (rgba only)", m)
	}
	if n := ps.OutstandingTotal(); n != 1 {
		t.Fatalf("outstanding = %d, want 1", n)
	}
}

// TestPoolZeroAlloc pins the hot path: acquire+release after warmup costs
// no heap allocation (the whole point of §2.7 pools).
func TestPoolZeroAlloc(t *testing.T) {
	p := NewPool("rgba", 64, 1<<20)
	b := p.Acquire()
	p.Release(b)
	n := testing.AllocsPerRun(50, func() {
		x := p.Acquire()
		p.Release(x)
	})
	if n != 0 {
		t.Fatalf("pool cycle allocs = %v, want 0", n)
	}
}

// TestMemCap pins the 1080p cap arithmetic: estimate fits well inside the
// 512MB cap (no false overrun), oversize math is monotonic.
func TestMemCap(t *testing.T) {
	est := EstimateDecoderBytes(1920, 1080, 5, 256<<10)
	capBytes := int64(MemCapKBFor1080p) << 10
	if est >= capBytes {
		t.Fatalf("1080p estimate %d exceeds cap %d", est, capBytes)
	}
	big := EstimateDecoderBytes(3840, 2160, 5, 256<<10)
	if big <= est {
		t.Fatalf("4K estimate %d <= 1080p %d", big, est)
	}
}
