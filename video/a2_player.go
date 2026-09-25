// A2 AV sync wiring, video-only stub (ffmpeg backend).
//
// Say it plain: the ffmpeg backend decodes pictures only. Sound stays
// off until native audio decode lands (see t-audio-ffmpeg). This file
// keeps the A2 public shape (HasAudio/Master/Serial/AVDiffMs/PollAudio
// plus Stats waterline) honest at zero, so callers never branch on a
// removed mode and old tests keep compiling against silence.
package video

import (
	"sync/atomic"
)

// HasAudio reports the A2 sound path runs. Always false on the
// video-only ffmpeg backend.
func (p *Player) HasAudio() bool { return false }

// Master names the leading clock. Always video until audio lands.
func (p *Player) Master() string {
	if p == nil {
		return MasterVideo
	}
	return MasterVideo
}

// Serial is the shared seek serial both queues would retire on.
// The video queue still bumps it on every seek, so tests can prove one
// seek moved the picture line.
func (p *Player) Serial() int64 {
	if p == nil {
		return 0
	}
	return atomic.LoadInt64(&p.generation)
}

// AVDiffMs is sound stamp minus picture stamp. Always zero: no sound.
func (p *Player) AVDiffMs() int64 { return 0 }

// masterDue is the schedule Poll serves pictures against: the picture
// clock. The audio-master branch retired with the Go AAC path.
func (p *Player) masterDue() int64 {
	if p == nil || p.clk == nil {
		return 0
	}
	return p.clk.DuePTSMS()
}

// PollAudio returns the newest due PCM frame. Always (nil, ended): the
// backend carries no sound, so silent clips report video end, and live
// clips report not-ended. ended mirrors the picture line so callers
// waiting on sound end still terminate.
func (p *Player) PollAudio() (f *AudioFrame, ended bool) {
	if p == nil {
		return nil, true
	}
	select {
	case <-p.stopCh:
		return nil, false
	default:
	}
	select {
	case <-p.readyCh:
	default:
		select {
		case <-p.readyCh:
		case <-p.stopCh:
			return nil, false
		}
	}
	p.mu.Lock()
	done := p.decodeDone && p.q.Depth() == 0 && p.hasShown
	loop := p.loop
	p.mu.Unlock()
	if !loop && done {
		return nil, true
	}
	return nil, false
}

// fillAudioStats rides the A2 waterline onto Stats: video master with
// zero audio, honest unavailable, never faked.
func (p *Player) fillAudioStats(st *Stats) {
	if p == nil || st == nil {
		return
	}
	st.Master = MasterVideo
}

// The lifecycle helpers below are no-ops kept for API shape: the Go AAC
// thread retired, ffmpeg owns threading natively. Seek/Close/Pause/Rate
// paths no longer call them; they stay so external callers do not break.

func (p *Player) startAudioLoop()             {}
func (p *Player) clearAudioQueue()            {}
func (p *Player) closeAudioQueue()            {}
func (p *Player) waitAudioLoop()              {}
func (p *Player) wakeAudioLoop()              {}
func (p *Player) parkAudioLocked(int64)       {}
func (p *Player) startAudioClockAtSeek(int64) {}
func (p *Player) pauseAudioClock()            {}
func (p *Player) resumeAudioClock()           {}
func (p *Player) setAudioRate(float64)        {}
func (p *Player) streamAudioDrained() bool    { return true }
