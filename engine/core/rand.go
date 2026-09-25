package core

// Rand is a seeded deterministic generator (splitmix64). Same seed replays
// the same sequence, so particle and gameplay tests never depend on luck.
// Not for security use.
type Rand struct {
	state uint64
}

// NewRand builds a generator from seed.
func NewRand(seed uint64) *Rand {
	r := &Rand{}
	r.Seed(seed)
	return r
}

// Seed resets the sequence.
func (r *Rand) Seed(seed uint64) { r.state = seed }

// Uint64 returns the next 64-bit value.
func (r *Rand) Uint64() uint64 {
	r.state += 0x9E3779B97F4A7C15
	z := r.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// Int63 returns a non-negative 63-bit value.
func (r *Rand) Int63() int64 { return int64(r.Uint64() >> 1) }

// Int63n returns a value in [0, n). n <= 0 returns 0 (no panic).
// Modulo bias is accepted by design: sequence is frozen for replay,
// not for security or statistical use.
func (r *Rand) Int63n(n int64) int64 {
	if n <= 0 {
		return 0
	}
	return r.Int63() % n
}

// Float64 returns a value in [0, 1).
func (r *Rand) Float64() float64 { return float64(r.Uint64()>>11) / (1 << 53) }

// RangeFloat returns a value in [min, max]. Reversed bounds are swapped.
func (r *Rand) RangeFloat(min, max float64) float64 {
	if max < min {
		min, max = max, min
	}
	return min + r.Float64()*(max-min)
}

// RangeInt returns a value in [min, max). max <= min returns min.
func (r *Rand) RangeInt(min, max int64) int64 {
	if max <= min {
		return min
	}
	return min + r.Int63n(max-min)
}

// Shuffle permutes 0..n-1 (Fisher-Yates). n <= 0 or nil swap is a no-op.
func (r *Rand) Shuffle(n int, swap func(i, j int)) {
	if n <= 0 || swap == nil {
		return
	}
	for i := n - 1; i > 0; i-- {
		j := int(r.Int63n(int64(i + 1)))
		swap(i, j)
	}
}
