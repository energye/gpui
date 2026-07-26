package raster

import "sync/atomic"

// PendingSlot holds at most one unstarted frame job for latest-wins coalesce
// when the pipeline queue is full (Flutter-like pending replace).
type PendingSlot struct {
	job atomic.Value // FrameJob
	has atomic.Bool
}

// Store replaces any pending unstarted job with j.
func (p *PendingSlot) Store(j FrameJob) {
	if p == nil {
		return
	}
	p.job.Store(j)
	p.has.Store(true)
}

// Take removes and returns the pending job if any.
func (p *PendingSlot) Take() (FrameJob, bool) {
	if p == nil || !p.has.Swap(false) {
		return FrameJob{}, false
	}
	v := p.job.Load()
	if v == nil {
		return FrameJob{}, false
	}
	return v.(FrameJob), true
}

// Has reports whether a pending job exists.
func (p *PendingSlot) Has() bool {
	return p != nil && p.has.Load()
}
