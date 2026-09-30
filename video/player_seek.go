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
)

func (p *Player) SeekTo(targetMs int64) (landedMs int64, err error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return 0, fmt.Errorf("%w: seek after close", ErrClosed)
	}
	loop := p.loop
	streamErr := p.err
	p.mu.Unlock()
	if loop {
		return 0, fmt.Errorf("%w: seek in loop mode goes to VC1", ErrBadClip)
	}
	if streamErr != "" {
		return 0, fmt.Errorf("%w: stream dead %s: %s", ErrBadClip, p.path, streamErr)
	}
	return p.seekFFmpeg(targetMs, false)
}

func (p *Player) seekAllowed() error {
	p.mu.Lock()
	closed, loop, streamErr := p.closed, p.loop, p.err
	p.mu.Unlock()
	switch {
	case closed:
		return fmt.Errorf("%w: seek after close", ErrClosed)
	case loop:
		return fmt.Errorf("%w: seek in loop mode goes to VC1", ErrBadClip)
	case streamErr != "":
		return fmt.Errorf("%w: stream dead %s: %s", ErrBadClip, p.path, streamErr)
	case p.ffdec == nil:
		return fmt.Errorf("%w: nothing to seek", ErrNoFrames)
	}
	return nil
}

// SeekFast jumps without forward discard: on the ffmpeg backend it
// echoes like SeekTo (ffmpeg always lands keyframes natively). Use it
// while dragging the progress bar, then SeekTo once on release.
func (p *Player) SeekFast(targetMs int64) (int64, error) {
	if err := p.seekAllowed(); err != nil {
		return 0, err
	}
	return p.seekFFmpeg(targetMs, true)
}

// SeekBy jumps relative to the current position (negative rewinds).
func (p *Player) SeekBy(deltaMs int64) (int64, error) {
	if err := p.seekAllowed(); err != nil {
		return 0, err
	}
	return p.SeekTo(p.PositionMs() + deltaMs)
}

// NextKeyframe lands one frame interval after the current position;
// PrevKeyframe lands one interval before it (clamped to zero). The
// ffmpeg demuxer owns keyframes; SeekTo lands on the keyframe anyway.
func (p *Player) NextKeyframe() (int64, error) {
	return p.seekKeyframe(true)
}

// PrevKeyframe lands the keyframe strictly before the current position.
func (p *Player) PrevKeyframe() (int64, error) {
	return p.seekKeyframe(false)
}

func (p *Player) seekKeyframe(next bool) (int64, error) {
	if err := p.seekAllowed(); err != nil {
		return 0, err
	}
	ref := p.PositionMs()
	p.mu.Lock()
	if !p.hasShown && p.seekOK {
		ref = p.seekLandedMs
	}
	p.mu.Unlock()
	step := frameStepMs(p.frameRate)
	if next {
		return p.seekFFmpeg(ref+step, true)
	}
	if ref-step < 0 {
		return p.seekFFmpeg(0, true)
	}
	return p.seekFFmpeg(ref-step, true)
}

// StepFrame advances exactly one frame while paused (the frame-step key):
// it seeks to one frame interval past the current picture and stays
// paused, so the landed frame shows on the next poll. Calling it while
// playing pauses first.
func (p *Player) StepFrame() (int64, error) {
	if err := p.seekAllowed(); err != nil {
		return 0, err
	}
	if !p.Paused() {
		p.Pause()
	}
	return p.SeekTo(p.PositionMs() + frameStepMs(p.frameRate))
}

// SetRate scales playback speed (1 = normal, 2 = double, 0.5 = half):
// the clock advances stamps faster or slower, and catch-up drops the
// surplus at high rates. Range is 0 < rate <= 8.
func (p *Player) PositionMs() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.hasShown {
		return p.lastShown
	}
	return 0
}

// Seeking reports a seek is still travelling: SeekTo returned the landing
// stamp but the background has not shown it yet.
func (p *Player) Seeking() bool {
	if p == nil {
		return false
	}
	p.dmu.Lock()
	defer p.dmu.Unlock()
	return p.seekActive
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// SeekInfo reports the last seek evidence.
func (p *Player) SeekInfo() (ok bool, targetMs, landedMs, keyMs, deltaMs int64, forward int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.seekOK, p.seekTargetMs, p.seekLandedMs, p.seekKeyMs, p.seekDeltaMs, p.seekForward
}

// Close ends the background thread and waits for it. Queued and
// displayed Pix buffers are recycled here, so the pools report zero
// outstanding afterwards (leak check); Poll results handed out before
// Close must no longer be touched.
