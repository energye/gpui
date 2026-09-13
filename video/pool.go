package video

import "sync"

// Pool is one fixed-size byte-slice pool (VR7 §2.7): borrow-use-return,
// same size only, resolution switch builds a new pool, steady reuse grows
// nothing. Stats stay readable for the window JSON (hits/misses/hit%,
// outstanding for leak checks, bytes held, evictions on cap overflow).
// Pure Go, standard library only.
type Pool struct {
	name    string
	bufSize int
	capByte int
	mu      sync.Mutex
	free    [][]byte
	held    int // bytes currently parked in free
	acq     int64
	hits    int64
	miss    int64
	rel     int64
	drop    int64
	out     int64
}

// NewPool builds a pool for bufSize-byte slices holding at most capByte
// parked bytes (capByte <= 0 means park everything; the window always
// passes a real cap so overflow is visible as evictions, never silent).
func NewPool(name string, bufSize, capByte int) *Pool {
	if bufSize <= 0 {
		bufSize = 1
	}
	return &Pool{name: name, bufSize: bufSize, capByte: capByte}
}

// Name reports the pool label (yuv/rgba/work).
func (p *Pool) Name() string { return p.name }

// BufSize reports the fixed slice length.
func (p *Pool) BufSize() int { return p.bufSize }

// Acquire borrows a bufSize slice: parked reuse counts a hit, fresh
// allocation a miss. The caller owns the slice until Release.
func (p *Pool) Acquire() []byte {
	p.mu.Lock()
	n := len(p.free)
	if n > 0 {
		b := p.free[n-1]
		p.free[n-1] = nil
		p.free = p.free[:n-1]
		p.held -= p.bufSize
		p.acq++
		p.hits++
		p.out++
		p.mu.Unlock()
		return b
	}
	p.acq++
	p.miss++
	p.out++
	p.mu.Unlock()
	return make([]byte, p.bufSize)
}

// Release returns a slice. Wrong sizes and cap overflow are dropped and
// counted (evictions), never silently grown.
func (p *Pool) Release(b []byte) {
	if p == nil || len(b) != p.bufSize {
		if p != nil {
			p.mu.Lock()
			p.rel++
			p.drop++
			if p.out > 0 {
				p.out--
			}
			p.mu.Unlock()
		}
		return
	}
	p.mu.Lock()
	p.rel++
	if p.out > 0 {
		p.out--
	}
	if p.capByte > 0 && p.held+p.bufSize > p.capByte {
		p.drop++
		p.mu.Unlock()
		return
	}
	p.free = append(p.free, b)
	p.held += p.bufSize
	p.mu.Unlock()
}

// PoolStats is the readable snapshot for JSON and gates.
type PoolStats struct {
	Acquires     int64
	Hits         int64
	Misses       int64
	HitPct       float64
	Releases     int64
	Outstanding  int64
	HeldBytes    int
	Evictions    int64
	BufSize      int
	CapBytes     int
}

// Stats snapshots the counters. HitPct is 0 with no acquires (honest
// unavailable, never faked to 100).
func (p *Pool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	hitPct := 0.0
	if p.acq > 0 {
		hitPct = float64(p.hits) / float64(p.acq) * 100.0
	}
	return PoolStats{Acquires: p.acq, Hits: p.hits, Misses: p.miss, HitPct: hitPct, Releases: p.rel, Outstanding: p.out, HeldBytes: p.held, Evictions: p.drop, BufSize: p.bufSize, CapBytes: p.capByte}
}

// Outstanding reports borrowed-but-unreturned slices (leak check: must be
// 0 after Close, N while frames are held by design).
func (p *Pool) Outstanding() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.out
}

// Test-only helper lives in pool_test.go (opaqueAlphaOK).

// RGBABytes returns the packed RGBA size for w×h (window + player share
// the same arithmetic so caps match the real buffers).
func RGBABytes(w, h int) int { return w * h * 4 }

// YUVBytes returns the planar 4:2:0 total for w×h.
func YUVBytes(w, h int) int { return w*h + w*h/2 }

// Pools bundles the §2.7 three independent pools: YUV frames, RGBA
// frames, bitstream workspace. Independent so one pressure never borrows
// from another; leak-checked separately.
type Pools struct {
	YUV  *Pool
	RGBA *Pool
	Work *Pool
}

// NewPools sizes the three pools for one resolution (call again on
// resolution switch; steady reuse never regrows). workSize covers one
// sample payload upper bound; capBytes bound total parked memory.
func NewPools(w, h, workSize int, capBytes int) *Pools {
	if workSize <= 0 {
		workSize = 256 << 10
	}
	per := capBytes / 3
	if per <= 0 {
		per = capBytes
	}
	return &Pools{
		YUV:  NewPool("yuv", YUVBytes(w, h), per),
		RGBA: NewPool("rgba", RGBABytes(w, h), per),
		Work: NewPool("work", workSize, per),
	}
}

// HitPctMin returns the smallest hit% among pools with acquires (the
// gate uses the minimum so no pool hides behind another; 0 acquires
// pools are skipped, all-empty returns 0).
func (ps *Pools) HitPctMin() float64 {
	if ps == nil {
		return 0
	}
	min := 100.0
	any := false
	for _, p := range []*Pool{ps.YUV, ps.RGBA, ps.Work} {
		if p == nil {
			continue
		}
		st := p.Stats()
		if st.Acquires == 0 {
			continue
		}
		any = true
		if st.HitPct < min {
			min = st.HitPct
		}
	}
	if !any {
		return 0
	}
	return min
}

// OutstandingTotal sums borrowed-but-unreturned across the three pools.
func (ps *Pools) OutstandingTotal() int64 {
	if ps == nil {
		return 0
	}
	var n int64
	for _, p := range []*Pool{ps.YUV, ps.RGBA, ps.Work} {
		if p != nil {
			n += p.Outstanding()
		}
	}
	return n
}

// MemCapKBFor1080p is the VR7 1080p decoder-side cap (pools + reference
// frames + queue): 512MB. Tighter than the old 1GB window cap on purpose
// (VR7 most strict); 4K takes a larger but still explicit cap (VC2).
// The window enforces RSS peak against it and reports overruns, never
// silent growth.
const MemCapKBFor1080p = 512 << 10

// EstimateDecoderBytes bounds one open of w×h: YUV+RGBA per frame plus one
// workspace, times frames. The window compares it against the cap before
// playing so oversize clips fail fast with a readable error.
func EstimateDecoderBytes(w, h, frames, workSize int) int64 {
	if frames <= 0 {
		frames = 1
	}
	return int64(frames)*(int64(YUVBytes(w, h))+int64(RGBABytes(w, h))) + int64(workSize)
}
