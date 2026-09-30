//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package video

import (
	"fmt"
	"sort"
	"time"

	"github.com/energye/gpui/video/clock"
)

func frameStepMs(fps float64) int64 {
	if fps > 1 {
		s := int64(float64(1000) / fps)
		if s >= 1 {
			return s
		}
	}
	return 200
}

func wallMs() int64 { return time.Now().UnixMilli() }

// WallDeadline returns a wall-clock deadline ms in the future.
func WallDeadline(ms int64) int64 { return wallMs() + ms }

// WallPast reports whether a WallDeadline passed.
func WallPast(deadline int64) bool { return wallMs() >= deadline }

// WallSleep sleeps ms wall-clock (paced tests yield the background).
func WallSleep(ms int64) { time.Sleep(time.Duration(ms) * time.Millisecond) }

// releasePix returns a streaming convert buffer. No-op for nil and
// for sizes the current pool no longer takes (resolution switch just
// rebuilt: the old slice falls back to GC on this cold path instead of
// polluting the new pool's counters).
func (p *Player) releasePix(b []byte) {
	if len(b) == 0 {
		return
	}
	lv := p.pooled.Load()
	if lv == nil || lv.pools == nil || lv.pools.RGBA == nil {
		return
	}
	if len(b) != lv.pools.RGBA.BufSize() {
		return
	}
	lv.pools.RGBA.Release(b)
}

// remakeLive builds (or rebuilds on resolution change) the streaming pool
// set. Cold path only: steady frames always match the loaded snapshot
// above. Caps park queue + displayed + in-flight + 1 spare RGBA frame, so
// steady play never evicts.
func (p *Player) remakeLive(w, h int) *pooledLive {
	work := 256 << 10
	qcap := p.q.Cap()
	if qcap <= 0 {
		qcap = clock.DefaultCap
	}
	spare := qcap + 3
	ps := &Pools{
		YUV:  NewPool("yuv", YUVBytes(w, h), YUVBytes(w, h)),
		RGBA: NewPool("rgba", RGBABytes(w, h), spare*RGBABytes(w, h)),
		Work: NewPool("work", work, work),
	}
	lv := &pooledLive{pools: ps, w: w, h: h, workSize: work, capBytes: spare * RGBABytes(w, h)}
	p.pooled.Store(lv)
	return lv
}

// assignPTS guards monotonic display stamps (caller sequences emissions).
func (p *Player) assignPTS(pts int64) int64 {
	if p.hasFirst && pts <= p.lastPTS {
		pts = p.lastPTS + frameStepMs(p.frameRate)
	}
	return pts
}

// Poll returns the newest due frame (at most one per tick), or
// (nil, true) when playback ended. Streaming Pix lifetime: the
// returned Pix stays valid until the next Poll or Close, then it is
// recycled — copy what the display needs during the tick (the windows
// blit synchronously).
func (p *Player) Poll() (f *clock.Frame, ended bool) {
	select {
	case <-p.stopCh:
		return nil, false
	default:
	}
	pollTimeout := 5 * time.Second
	p.dmu.Lock()
	travelling := p.seekActive
	p.dmu.Unlock()
	if travelling {
		pollTimeout = 0
	}
	select {
	case <-p.readyCh:
	default:
		select {
		case <-p.readyCh:
		case <-p.stopCh:
			return nil, false
		case <-time.After(pollTimeout):
			return nil, false
		}
	}
	due := p.masterDue()
	fr, skipped, ok := p.q.PollDue(due)
	_ = skipped
	if ok {
		// the display now owns fr.Pix; the previously shown
		// buffer goes back. Dropped stale frames never reach here
		// (the queue observer already returned them).
		p.mu.Lock()
		old := p.lastPix
		p.lastPix = fr.Pix
		p.mu.Unlock()
		p.releasePix(old)
		p.mu.Lock()
		p.shown++
		p.lastShown = fr.PTSMs
		p.hasShown = true
		if !p.loop {
			// Streaming: end when the decoder exhausted AND the
			// queue drained (empty; the queue stays open so a later
			// seek can revive this same thread).
			p.mu.Unlock()
			done := p.streamDrained()
			if done {
				p.mu.Lock()
				p.ended = true
				p.mu.Unlock()
				return fr, true
			}
			p.mu.Lock()
		}
		p.mu.Unlock()
		return fr, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.loop {
		return nil, false
	}
	// mu is already held here: read decodeDone directly instead of
	// streamDrained (which locks mu itself and would self-deadlock).
	if p.decodeDone && p.q.Depth() == 0 {
		p.ended = true
		return nil, true
	}
	return nil, false
}

// streamDrained reports the streaming end: the background decoded
// everything AND the queue emptied. The queue itself stays open (a later
// seek revives playback), so emptiness — not closure — is the signal.
func (p *Player) streamDrained() bool {
	p.mu.Lock()
	done := p.decodeDone
	p.mu.Unlock()
	return done && p.q.Depth() == 0
}

// Pause freezes the picture; the clock skips the held span on Resume.
func (p *Player) Pause() {
	p.mu.Lock()
	p.paused = true
	p.mu.Unlock()
	p.clk.Pause()
}

// Resume continues after Pause.
func (p *Player) Resume() {
	p.mu.Lock()
	p.paused = false
	p.mu.Unlock()
	p.clk.Resume()
}

// Paused reports the hold state.
func (p *Player) Paused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paused
}

// Buffered reports the small-clip full-cache path (tests only).
// Always false: every clip streams on the ffmpeg backend.
func (p *Player) Buffered() bool {
	if p == nil {
		return false
	}
	return false
}

// DecodePos reports samples consumed at open (tests only: streaming
// fast-open proof). The ffmpeg open decodes headers + first displayable
// frame, so 1.
func (p *Player) DecodePos() int {
	if p == nil {
		return 0
	}
	return 1
}

// ReorderDepth reports the B-delay used (tests only). ffmpeg reorders
// natively; 2 covers single-B without stalling open.
func (p *Player) ReorderDepth() int {
	if p == nil {
		return 0
	}
	return 2
}

// BWindows reports frame-threaded laps run so far (tests only).
// Always 0: the Go B machinery retired, ffmpeg reorders natively.
func (p *Player) BWindows() int64 {
	if p == nil {
		return 0
	}
	return 0
}

// S2Windows reports parallel windows run so far (tests only).
// Always 0: Options.S2Parallel stays accepted but ffmpeg owns threading.
func (p *Player) S2Windows() int64 {
	if p == nil {
		return 0
	}
	return 0
}

// Info describes the clip.
func (p *Player) Info() Info {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.info
}

// Stats snapshots the waterline.
func (p *Player) Stats() Stats {
	p.mu.Lock()
	ended := p.ended
	decodeDone := p.decodeDone
	p.mu.Unlock()
	p.mu.Lock()
	times := append([]float64(nil), p.decTimes...)
	p.mu.Unlock()
	sort.Float64s(times)
	avg, p95 := 0.0, 0.0
	if len(times) > 0 {
		sum := 0.0
		for _, t := range times {
			sum += t
		}
		avg = sum / float64(len(times))
		p95 = times[int(float64(len(times))*0.95)]
	}
	drift := int64(0)
	p.mu.Lock()
	hasShown := p.hasShown
	lastShown := p.lastShown
	p.mu.Unlock()
	if hasShown {
		drift = p.clk.DriftMs(lastShown)
		if drift < 0 {
			drift = 0
		}
	}
	p.mu.Lock()
	done := (!p.loop && (ended || (decodeDone && p.q.Depth() == 0 && hasShown)))
	p.mu.Unlock()
	p.mu.Lock()
	seekOK := 0
	if p.seekOK {
		seekOK = 1
	}
	seekDelta, seekFwd, seekLanded := p.seekDeltaMs, p.seekForward, p.seekLandedMs
	errStr := p.err
	p.mu.Unlock()
	rate := p.Rate()
	seeking := 0
	if p.Seeking() {
		seeking = 1
	}
	poolHit := 0.0
	var evict int64
	if lv := p.pooled.Load(); lv != nil && lv.pools != nil {
		poolHit = lv.pools.HitPctMin()
		for _, pl := range []*Pool{lv.pools.YUV, lv.pools.RGBA, lv.pools.Work} {
			if pl != nil {
				evict += pl.Stats().Evictions
			}
		}
	}
	p.mu.Lock()
	memCap, estimate := p.memCapKB, p.estimateB
	p.mu.Unlock()
	st := Stats{Decoded: p.decoded, Shown: p.shown, Dropped: p.q.Dropped(), QueueDepth: p.q.Depth(), QueueMax: p.q.MaxDepth(), QueueAvg: p.q.DepthAvg(), DecodeMsAvg: avg, DecodeMsP95: p95, DriftMs: drift, Ended: done, Error: errStr, SeekOK: seekOK, SeekDeltaMs: seekDelta, SeekForward: seekFwd, SeekLandedMs: seekLanded, Concealed: 0, Rate: rate, Seeking: seeking, PoolHitPct: poolHit, MemCapKB: memCap, EstimateB: estimate, PoolEvictions: evict}
	p.fillAudioStats(&st)
	// P1 硬解水位读真值（解码器原子记账，任意线程可读）。
	p.dmu.Lock()
	dec := p.ffdec
	p.dmu.Unlock()
	if dec != nil {
		hw := dec.HWStats()
		st.HWActive, st.HWName = hw.Active, hw.Name
		st.HWFallbacks, st.HWTransferMsAvg = hw.Fallbacks, hw.TransferMsAvg
	} else {
		st.HWName = "soft"
	}
	return st
}

// ConcealedFault reports the first skipped sample's problem; always nil
// on the ffmpeg backend (native code absorbs corrupt frames).
func (p *Player) ConcealedFault() error {
	return nil
}

func (p *Player) SetRate(r float64) error {
	if r != r || r <= 0 || r > 8 {
		return fmt.Errorf("video: bad rate %v (want 0 < rate <= 8)", r)
	}
	p.mu.Lock()
	paused := p.paused
	p.mu.Unlock()
	anchor := p.clk.DuePTSMS()
	if err := p.clk.SetRate(r); err != nil {
		return err
	}
	p.clk.Start(anchor)
	if paused {
		p.clk.Pause()
	}
	return nil
}

// Rate reports the current playback speed.
func (p *Player) Rate() float64 {
	if p == nil {
		return 1
	}
	return p.clk.Rate()
}

// PositionMs reports the current position: the last shown stamp, or 0
// before the first frame.
