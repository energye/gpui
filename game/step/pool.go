package step

import (
	"sync"

	"github.com/energye/gpui/game/core"
)

// Frozen pool budgets. Get beyond MaxLen is OutOfMemory, never a guess.
// Put beyond MaxRetained or beyond MaxLen drops the buffer and keeps the
// retained set bounded, so long runs never grow.
const (
	// MaxLen caps one Get request (elements, per kind).
	MaxLen = 1 << 20
	// MaxRetained caps retained slices per kind (vec, color, float each).
	MaxRetained = 32
	// VecBytes is the retained accounting per Vec2 element.
	VecBytes = 16
	// ColorBytes is the retained accounting per Color element.
	ColorBytes = 32
	// FloatBytes is the retained accounting per float64 element.
	FloatBytes = 8
)

// Stats reports pool counters. Gets counts every Get call, Puts every kept
// Put call (nil and zero-cap Puts are no-ops and count for neither).
// Hits counts Gets served from the retained set, Misses the fresh makes.
// Retained counts and RetainedBytes describe what the pool holds now.
type Stats struct {
	Gets          int
	Puts          int
	Hits          int
	Misses        int
	RetainedVec   int
	RetainedColor int
	RetainedFloat int
	RetainedBytes int64
}

// Pool reuses core number buffers across frames: Vec2 for path points and
// vertex positions, Color for vertex colors, float64 for raw streams.
// Put transfers ownership to the pool; the caller must not use the slice
// after Put. Get returns len n with undefined contents; the caller must
// overwrite the first n elements before drawing. A nil Pool never panics:
// Get reports InvalidArg, Put and ResetStats are no-ops, Stats is zero.
type Pool struct {
	mu     sync.Mutex
	vecs   [][]core.Vec2
	colors [][]core.Color
	floats [][]float64
	gets   int
	puts   int
	hits   int
	misses int
}

// takeRetained pops the newest buffer with cap >= n. The caller holds the
// pool lock; slicing to [:n] reuses the backing without a new make.
func takeRetained[T any](stack *[][]T, n int) ([]T, bool) {
	for i := len(*stack) - 1; i >= 0; i-- {
		if cap((*stack)[i]) >= n {
			buf := (*stack)[i]
			*stack = append((*stack)[:i], (*stack)[i+1:]...)
			return buf[:n], true
		}
	}
	return nil, false
}

// keepRetained keeps b for reuse. False drops it: zero cap, cap beyond
// MaxLen, or already holding MaxRetained. The caller counts the Put.
func keepRetained[T any](stack *[][]T, b []T) bool {
	if cap(b) == 0 || cap(b) > MaxLen || len(*stack) >= MaxRetained {
		return false
	}
	*stack = append(*stack, b)
	return true
}

func stackBytes[T any](bufs [][]T, width int64) int64 {
	var total int64
	for _, b := range bufs {
		total += int64(cap(b)) * width
	}
	return total
}

// NewPool builds an empty pool.
func NewPool() *Pool { return &Pool{} }

// checkGet rejects a bad pool or size before locking.
func checkGet(p *Pool, op string, n int) error {
	if p == nil {
		return core.InvalidArg(op, "pool")
	}
	if n < 0 {
		return core.InvalidArg(op, "n")
	}
	if n > MaxLen {
		return core.OutOfMemory(op, "n")
	}
	return nil
}

// GetVec returns n Vec2 elements for path or vertex positions.
// n < 0 is InvalidArg, n > MaxLen is OutOfMemory, n == 0 returns an empty
// slice without touching the retained set.
func (p *Pool) GetVec(n int) ([]core.Vec2, error) {
	const op = "step.GetVec"
	if err := checkGet(p, op, n); err != nil {
		return nil, err
	}
	if n == 0 {
		p.mu.Lock()
		p.gets++
		p.hits++
		p.mu.Unlock()
		return []core.Vec2{}, nil
	}
	p.mu.Lock()
	p.gets++
	if buf, ok := takeRetained(&p.vecs, n); ok {
		p.hits++
		p.mu.Unlock()
		return buf, nil
	}
	p.misses++
	p.mu.Unlock()
	return make([]core.Vec2, n), nil
}

// PutVec returns b to the pool. Nil and zero-cap slices are no-ops.
// Oversized (cap > MaxLen) or over-budget puts count but drop the buffer.
func (p *Pool) PutVec(b []core.Vec2) {
	if p == nil || cap(b) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.puts++
	keepRetained(&p.vecs, b)
}

// GetColor returns n Color elements for vertex colors.
// n < 0 is InvalidArg, n > MaxLen is OutOfMemory, n == 0 is empty.
func (p *Pool) GetColor(n int) ([]core.Color, error) {
	const op = "step.GetColor"
	if err := checkGet(p, op, n); err != nil {
		return nil, err
	}
	if n == 0 {
		p.mu.Lock()
		p.gets++
		p.hits++
		p.mu.Unlock()
		return []core.Color{}, nil
	}
	p.mu.Lock()
	p.gets++
	if buf, ok := takeRetained(&p.colors, n); ok {
		p.hits++
		p.mu.Unlock()
		return buf, nil
	}
	p.misses++
	p.mu.Unlock()
	return make([]core.Color, n), nil
}

// PutColor returns b to the pool. Nil and zero-cap slices are no-ops.
func (p *Pool) PutColor(b []core.Color) {
	if p == nil || cap(b) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.puts++
	keepRetained(&p.colors, b)
}

// GetFloat returns n float64 elements for raw coordinate streams.
// n < 0 is InvalidArg, n > MaxLen is OutOfMemory, n == 0 is empty.
func (p *Pool) GetFloat(n int) ([]float64, error) {
	const op = "step.GetFloat"
	if err := checkGet(p, op, n); err != nil {
		return nil, err
	}
	if n == 0 {
		p.mu.Lock()
		p.gets++
		p.hits++
		p.mu.Unlock()
		return []float64{}, nil
	}
	p.mu.Lock()
	p.gets++
	if buf, ok := takeRetained(&p.floats, n); ok {
		p.hits++
		p.mu.Unlock()
		return buf, nil
	}
	p.misses++
	p.mu.Unlock()
	return make([]float64, n), nil
}

// PutFloat returns b to the pool. Nil and zero-cap slices are no-ops.
func (p *Pool) PutFloat(b []float64) {
	if p == nil || cap(b) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.puts++
	keepRetained(&p.floats, b)
}

// Stats returns a copy of the pool counters.
func (p *Pool) Stats() Stats {
	if p == nil {
		return Stats{}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return Stats{
		Gets:          p.gets,
		Puts:          p.puts,
		Hits:          p.hits,
		Misses:        p.misses,
		RetainedVec:   len(p.vecs),
		RetainedColor: len(p.colors),
		RetainedFloat: len(p.floats),
		RetainedBytes: retainedLocked(p),
	}
}

// RetainedBytes returns the retained backing bytes (cap * frozen width).
func (p *Pool) RetainedBytes() int64 {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return retainedLocked(p)
}

// HitRate returns hits / gets, or 0 with no Gets.
func (p *Pool) HitRate() float64 {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.gets == 0 {
		return 0
	}
	return float64(p.hits) / float64(p.gets)
}

// ResetStats clears Gets, Puts, Hits, Misses without dropping buffers.
func (p *Pool) ResetStats() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gets, p.puts, p.hits, p.misses = 0, 0, 0, 0
}

func retainedLocked(p *Pool) int64 {
	return stackBytes(p.vecs, VecBytes) +
		stackBytes(p.colors, ColorBytes) +
		stackBytes(p.floats, FloatBytes)
}
