package clock

import (
	"fmt"
	"sync"
)

// DefaultCap is the queue depth when the caller passes no capacity.
// Single digits on purpose: the queue is a shock absorber, not storage.
const DefaultCap = 4

// Frame is one presentable picture. Pix is packed RGBA owned by whoever
// holds the frame; PTSMs is the display stamp in milliseconds, monotonic
// within a run (loops keep counting up); DurMs covers to the next stamp.
type Frame struct {
	Width  int
	Height int
	Pix    []byte
	PTSMs  int64
	DurMs  int64
	Seq    int64
}

// Queue is a bounded FIFO of frames. Push blocks while full (backpressure
// into the decoder thread, so a fast decoder never piles up memory);
// display-side PollDue supersedes stale frames and counts them dropped.
// Use NewQueue; the zero value is not usable.
type Queue struct {
	mu       sync.Mutex
	room     *sync.Cond
	buf      []*Frame
	cap      int
	closed   bool
	dropped  int64
	pushes   int64
	maxDepth int
	depthSum int64
	depthN   int64
}

// NewQueue builds a queue holding at most cap frames (cap <= 0 means
// DefaultCap).
func NewQueue(cap int) *Queue {
	if cap <= 0 {
		cap = DefaultCap
	}
	q := &Queue{cap: cap}
	q.room = sync.NewCond(&q.mu)
	return q
}

// Cap reports the configured depth.
func (q *Queue) Cap() int { return q.cap }

// Push adds a frame, waiting while the queue is full. It returns false
// when the queue is stopped (Close called), in which case the frame is
// not kept. Nil frames are rejected with an error.
func (q *Queue) Push(f *Frame) (bool, error) {
	if f == nil {
		return false, fmt.Errorf("clock: nil frame")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.buf) >= q.cap && !q.closed {
		q.room.Wait()
	}
	if q.closed {
		return false, nil
	}
	q.buf = append(q.buf, f)
	q.pushes++
	if len(q.buf) > q.maxDepth {
		q.maxDepth = len(q.buf)
	}
	q.depthSum += int64(len(q.buf))
	q.depthN++
	return true, nil
}

// PollDue returns the newest frame whose stamp is at or before nowMs,
// dropping any earlier due frames as stale (counted). ok is false when
// no frame is due yet. Callers show at most one frame per tick.
// Waiting producers are woken on every drain (Signal, not Broadcast:
// one room means one producer proceeds).
func (q *Queue) PollDue(nowMs int64) (f *Frame, skipped int, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for len(q.buf) > 0 && q.buf[0].PTSMs <= nowMs {
		f = q.buf[0]
		q.buf[0] = nil
		q.buf = q.buf[1:]
		n++
	}
	if n == 0 {
		return nil, 0, false
	}
	q.dropped += int64(n - 1)
	q.depthSum += int64(len(q.buf))
	q.depthN++
	q.room.Signal()
	return f, n - 1, true
}

// Depth is the current fill.
func (q *Queue) Depth() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.buf)
}

// Dropped counts stale frames superseded at display.
func (q *Queue) Dropped() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.dropped
}

// Pushes counts accepted frames.
func (q *Queue) Pushes() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pushes
}

// MaxDepth is the deepest fill seen.
func (q *Queue) MaxDepth() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.maxDepth
}

// DepthAvg is the mean fill over sampled operations.
func (q *Queue) DepthAvg() float64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.depthN == 0 {
		return 0
	}
	return float64(q.depthSum) / float64(q.depthN)
}

// Close stops the queue: blocked Push calls return false, and Drained
// reports true once the remaining frames are consumed.
func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.closed = true
	q.room.Broadcast()
}

// Drained reports no more frames will ever come out.
func (q *Queue) Drained() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed && len(q.buf) == 0
}
