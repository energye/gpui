// A2 AV sync wiring (VW6 §11.3 A2, §12 A2 row): the Player side.
//
// Say it plain: when the shell carries sound, Open starts a second
// background thread that turns AAC packets into PCM frames into a small
// bounded queue, plus a second wall clock. Poll serves pictures against
// the sound clock (sound leads, picture follows); silent clips never
// touch any of this and keep the old picture-only path bit for bit.
//
// Peer map (ffmpeg read-only, fftools/ffplay.c; Serial == our shared
// generation, Queue.Clear == packet_queue_flush without the bump):
//
//	get_master_sync_type/get_master_clock (:1477-1506) -> Master/masterDue
//	stream_seek + packet_queue_flush (:1527+:493) -> seekStream parks both
//	  needles, bumps generation once, clears both queues, re-anchors both
//	  clocks, wakes both parked threads
//	set_clock_speed (:1455, re-anchor then scale) -> SetRate scales both
//	video_refresh (:1630 newest-due + stale drops, :1580 thresholds) ->
//	  Poll reads masterDue; PollDue already drops stale / waits when early
//	audio_decode_frame (:2423) + sdl_audio_callback (:2571, audclk follows
//	  played PCM) -> audioDecodeLoop decodes ahead; PollAudio consumes due
//	  PCM and re-anchors audioClk at each shown stamp, so a starved sound
//	  thread holds the picture back instead of letting it run free
//	synchronize_audio swr trim -> parked: audio stays master, A3 owns trim
package video

import (
	"fmt"
	"sync/atomic"

	"github.com/energye/gpui/video/clock"
	"github.com/energye/gpui/video/mp4"
)

// HasAudio reports the A2 sound path runs (second clock + PCM queue).
func (p *Player) HasAudio() bool {
	if p == nil {
		return false
	}
	return p.hasAudio
}

// Master names the leading clock ("audio" when sound rides along, else
// "video"). Mirrors get_master_sync_type with the default audio-master.
func (p *Player) Master() string {
	if p == nil || !p.hasAudio {
		return MasterVideo
	}
	return MasterAudio
}

// Serial is the shared seek serial both queues retire on (ffplay's
// packet serials, bumped together on every seek). Tests use it to prove
// one seek moved sound and picture as one.
func (p *Player) Serial() int64 {
	if p == nil {
		return 0
	}
	return atomic.LoadInt64(&p.generation)
}

// AVDiffMs is the last-played sound stamp minus the last-shown picture
// stamp (ffplay status A-V = audclk - vidclk). Positive means sound
// leads. Zero until both sides showed once, and always zero when silent.
func (p *Player) AVDiffMs() int64 {
	if p == nil || !p.hasAudio {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.hasAudioShown || !p.hasShown {
		return 0
	}
	return p.lastAudioShown - p.lastShown
}

// masterDue is the schedule Poll serves pictures against: the sound
// clock when present, else the picture clock (old behavior identical).
func (p *Player) masterDue() int64 {
	if p != nil && p.hasAudio && p.audioClk != nil {
		return p.audioClk.DuePTSMS()
	}
	if p == nil || p.clk == nil {
		return 0
	}
	return p.clk.DuePTSMS()
}

// setupAudio starts the A2 sound path when the shell carries AAC
// (streaming path only; the caller already kept silent small clips on
// the buffered path). Silent or sound-less shells return nil with the
// path off. Over-cap fails the open (S7 fail-fast, never silent OOM).
func (p *Player) setupAudio(movie *mp4.Movie, opt Options) error {
	if movie == nil || movie.Audio == nil || len(movie.Audio.Samples) == 0 || len(movie.Audio.ASC) == 0 {
		return nil
	}
	a := movie.Audio
	dec, err := NewAudioDecoder(CodecAAC)
	if err != nil {
		return nil
	}
	if err := dec.Configure(a.ASC); err != nil {
		return nil
	}
	ch := int(a.Channels)
	if ch <= 0 {
		ch = 2
	}
	rate := int(a.SampleRate)
	step := AudioStepMs(rate)
	qcap := opt.AudioQueueCap
	if qcap <= 0 {
		qcap = DefaultAudioQueueCap
	}
	p.hasAudio = true
	p.audioSamples = append([]mp4.Sample(nil), a.Samples...)
	p.audioPos = 0
	p.audioDec = dec
	p.audioASC = append([]byte(nil), a.ASC...)
	p.audioRate = rate
	p.audioBase0 = a.Samples[0].PTSMs
	span := a.Samples[len(a.Samples)-1].PTSMs - p.audioBase0
	if span < 0 {
		span = 0
	}
	p.audioSpanMs = span
	p.audioEpoch = 0
	p.audioStep = step
	p.audioQ = NewAudioQueue(qcap)
	p.audioClk = clock.NewClock(p.nowMs)
	p.audioDoneCh = make(chan struct{})
	p.audioWakeCh = make(chan struct{}, 1)
	// Bounded sound footprint joins the S7 estimate: queued PCM plus
	// decoder headroom (overlap + tables, ~1MB, conservative).
	p.estimateB += int64(qcap)*int64(1024*ch*4) + (1 << 20)
	if p.estimateB > int64(p.memCapKB)<<10 {
		p.hasAudio = false
		p.audioQ = nil
		p.audioClk = nil
		p.audioDec = nil
		p.audioSamples = nil
		p.info.HasAudio = false
		return fmt.Errorf("video: memory over cap %s audio %d packets estimate %dB over %dKB cap: %w", p.path, len(a.Samples), p.estimateB, p.memCapKB, ErrMemOverCap)
	}
	p.info.HasAudio = true
	p.info.AudioSampleRate = rate
	p.info.AudioChannels = ch
	p.info.AudioSamples = len(a.Samples)
	return nil
}

// degradeAudioToSilent parks a broken sound path and keeps the picture
// playing (bad sound never kills video; the fault stays readable).
// Opener thread only (no background running yet).
func (p *Player) degradeAudioToSilent(cause error) {
	p.hasAudio = false
	p.audioQ = nil
	p.audioClk = nil
	p.audioDec = nil
	p.audioSamples = nil
	p.info.HasAudio = false
	if cause != nil {
		p.audioErr = cause.Error()
		if p.audioFirstFault == nil {
			p.audioFirstFault = cause
		}
	}
}

// primeAudioFirst decodes the head PCM packet synchronously so Open
// returns with sound ready (same fast-open promise as primeFirst).
// Silent: no-op. Undecodable head degrades to silent, never fails Open.
func (p *Player) primeAudioFirst() error {
	if !p.hasAudio {
		return nil
	}
	for p.audioPos < len(p.audioSamples) {
		idx := p.audioPos
		p.audioPos++
		s := p.audioSamples[idx]
		buf := make([]byte, s.Size)
		if _, err := readSourceRange(p.source, buf, int64(s.Offset)); err != nil {
			p.audioConcealed++
			if p.audioFirstFault == nil {
				p.audioFirstFault = fmt.Errorf("video: audio packet %d truncated %s (%v): %w", s.Number, p.path, err, mp4.ErrTruncated)
			}
			continue
		}
		pts := p.audioBase0 + p.audioEpoch + (s.PTSMs - p.audioBase0)
		pcm, err := p.audioDec.DecodePacket(buf, pts)
		if err != nil {
			p.audioConcealed++
			if p.audioFirstFault == nil {
				p.audioFirstFault = fmt.Errorf("video: audio packet %d decode %s: %w", s.Number, p.path, err)
			}
			p.resetAudioDecoderLocked()
			continue
		}
		fr := &AudioFrame{
			Data:       append([]float32(nil), pcm.Data...),
			SampleRate: pcm.SampleRate,
			Channels:   pcm.Channels,
			Samples:    pcm.Samples,
			PTSMs:      pts,
			Seq:        p.audioNextSeq,
			Serial:     atomic.LoadInt64(&p.generation),
		}
		p.audioNextSeq++
		p.audioDecoded++
		if ok, _ := p.audioQ.Push(fr); !ok {
			p.degradeAudioToSilent(fmt.Errorf("%w: audio queue closed during open %s", ErrClosed, p.path))
			return nil
		}
		p.audioClk.Start(pts - p.audioStep)
		return nil
	}
	p.degradeAudioToSilent(fmt.Errorf("%w: %s: no decodable audio", ErrNoFrames, p.path))
	return nil
}

// startAudioLoop runs the sound background (silent: no-op).
func (p *Player) startAudioLoop() {
	if !p.hasAudio {
		return
	}
	go p.audioDecodeLoop()
}

// resetAudioDecoderLocked rebuilds a clean sound decoder (caller holds dmu).
func (p *Player) resetAudioDecoderLocked() {
	if !p.hasAudio || len(p.audioASC) == 0 {
		return
	}
	nd, err := NewAudioDecoder(CodecAAC)
	if err != nil {
		return
	}
	if err := nd.Configure(p.audioASC); err != nil {
		return
	}
	p.audioDec = nd
}

// audioFloorPos resolves targetMs to the last audio packet at or before
// it (every packet decodes alone, so all are landing-safe). Pure table
// math, no locks: tables are immutable after open.
func (p *Player) audioFloorPos(targetMs int64) int {
	n := len(p.audioSamples)
	if n == 0 {
		return -1
	}
	best := 0
	if targetMs < p.audioSamples[0].PTSMs {
		return 0
	}
	for i, s := range p.audioSamples {
		if s.PTSMs <= targetMs {
			best = i
		} else {
			break
		}
	}
	_ = best
	// Samples are decode-ordered with non-decreasing PTS on these
	// tracks; the scan above is the floor. Keep it linear: seeks are
	// rare, packets number in the low thousands.
	return best
}

// parkAudioLocked reparks the sound needle on its floor packet and
// resets the decoder (caller holds dmu, before the shared serial bump).
func (p *Player) parkAudioLocked(targetMs int64) {
	if !p.hasAudio || len(p.audioSamples) == 0 {
		return
	}
	p.audioPos = p.audioFloorPos(targetMs)
	p.resetAudioDecoderLocked()
}

// clearAudioQueue drops queued PCM (seek rewind, after the serial bump).
func (p *Player) clearAudioQueue() {
	if p == nil || !p.hasAudio || p.audioQ == nil {
		return
	}
	p.audioQ.Clear()
}

// closeAudioQueue stops the sound queue (Close path, silent-safe).
func (p *Player) closeAudioQueue() {
	if p == nil || !p.hasAudio || p.audioQ == nil {
		return
	}
	p.audioQ.Close()
}

// waitAudioLoop parks Close until the sound thread exits (silent-safe).
func (p *Player) waitAudioLoop() {
	if p == nil || !p.hasAudio || p.audioDoneCh == nil {
		return
	}
	<-p.audioDoneCh
}

// wakeAudioLoop revives a sound thread parked at end-of-stream.
func (p *Player) wakeAudioLoop() {
	if p == nil || !p.hasAudio || p.audioWakeCh == nil {
		return
	}
	select {
	case p.audioWakeCh <- struct{}{}:
	default:
	}
}

// startAudioClockAtSeek re-anchors the sound clock on its floor landing
// (seek path, after the queue clear; silent-safe).
func (p *Player) startAudioClockAtSeek(targetMs int64) {
	if p == nil || !p.hasAudio || p.audioClk == nil || len(p.audioSamples) == 0 {
		return
	}
	pos := p.audioFloorPos(targetMs)
	landed := p.audioSamples[pos].PTSMs + p.audioEpoch
	p.audioClk.Start(landed)
}

// pauseAudioClock / resumeAudioClock freeze and continue the sound
// clock with the picture one (silent-safe).
func (p *Player) pauseAudioClock() {
	if p == nil || !p.hasAudio || p.audioClk == nil {
		return
	}
	p.audioClk.Pause()
}

// resumeAudioClock continues the sound clock after Pause.
func (p *Player) resumeAudioClock() {
	if p == nil || !p.hasAudio || p.audioClk == nil {
		return
	}
	p.audioClk.Resume()
}

// setAudioRate scales the sound clock with re-anchor (ffplay
// set_clock_speed: read, scale, restart; silent-safe).
func (p *Player) setAudioRate(r float64) {
	if p == nil || !p.hasAudio || p.audioClk == nil {
		return
	}
	anchor := p.audioClk.DuePTSMS()
	if err := p.audioClk.SetRate(r); err != nil {
		return
	}
	p.audioClk.Start(anchor)
}

// audioDecodeLoop serves PCM on the sound thread with backpressure:
// Push waits while the queue is full, so a tiny cap never piles memory.
// Non-loop parks at end with the queue open (a later seek revives this
// same thread); loop re-decodes from head with stamps counting up.
func (p *Player) audioDecodeLoop() {
	defer close(p.audioDoneCh)
	defer p.audioQ.Close()
	for {
		select {
		case <-p.stopCh:
			return
		default:
		}
		p.dmu.Lock()
		if p.audioPos >= len(p.audioSamples) {
			if p.loop {
				p.audioEpoch += p.audioSpanMs + p.audioStep
				p.audioPos = 0
				p.resetAudioDecoderLocked()
				p.dmu.Unlock()
				continue
			}
			p.dmu.Unlock()
			p.mu.Lock()
			p.audioDone = true
			p.mu.Unlock()
			select {
			case <-p.stopCh:
				return
			case <-p.audioWakeCh:
				continue
			}
		}
		idx := p.audioPos
		p.audioPos++
		s := p.audioSamples[idx]
		gen := atomic.LoadInt64(&p.generation)
		p.dmu.Unlock()

		buf := make([]byte, s.Size)
		if _, rerr := readSourceRange(p.source, buf, int64(s.Offset)); rerr != nil {
			p.mu.Lock()
			p.audioConcealed++
			if p.audioFirstFault == nil {
				p.audioFirstFault = fmt.Errorf("video: audio packet %d truncated %s (%v): %w", s.Number, p.path, rerr, mp4.ErrTruncated)
			}
			p.mu.Unlock()
			continue
		}
		p.dmu.Lock()
		if atomic.LoadInt64(&p.generation) != gen {
			p.dmu.Unlock()
			continue
		}
		pts := p.audioBase0 + p.audioEpoch + (s.PTSMs - p.audioBase0)
		pcm, derr := p.audioDec.DecodePacket(buf, pts)
		if derr != nil {
			p.mu.Lock()
			p.audioConcealed++
			if p.audioFirstFault == nil {
				p.audioFirstFault = fmt.Errorf("video: audio packet %d decode %s: %w", s.Number, p.path, derr)
			}
			p.mu.Unlock()
			p.resetAudioDecoderLocked()
			p.dmu.Unlock()
			continue
		}
		fr := &AudioFrame{
			Data:       append([]float32(nil), pcm.Data...),
			SampleRate: pcm.SampleRate,
			Channels:   pcm.Channels,
			Samples:    pcm.Samples,
			PTSMs:      pts,
			Seq:        p.audioNextSeq,
			Serial:     gen,
		}
		p.audioNextSeq++
		p.dmu.Unlock()
		if ok, _ := p.audioQ.Push(fr); !ok {
			return
		}
		if atomic.LoadInt64(&p.generation) != gen {
			p.audioQ.Clear()
			continue
		}
		p.mu.Lock()
		p.audioDecoded++
		p.mu.Unlock()
	}
}

// streamAudioDrained reports the sound end: background exhausted AND the
// queue emptied (queue stays open so a later seek revives the thread).
func (p *Player) streamAudioDrained() bool {
	p.mu.Lock()
	done := p.audioDone
	p.mu.Unlock()
	return done && p.audioQ.Depth() == 0
}

// PollAudio returns the newest due PCM frame (nil when none due yet).
// The shown stamp re-anchors the sound clock (ffplay audclk follows
// played PCM), so the picture Poll that reads masterDue stalls when
// sound starves instead of running free. Stale serials from a travelled
// seek drop silently (seek rewinds never count as drops). ended reports
// the sound stream played through (non-loop, exhausted and drained).
// Silent clips return (nil, false) until video end, then (nil, true).
func (p *Player) PollAudio() (f *AudioFrame, ended bool) {
	if p == nil {
		return nil, true
	}
	select {
	case <-p.stopCh:
		return nil, false
	default:
	}
	if !p.hasAudio || p.audioQ == nil || p.audioClk == nil {
		p.mu.Lock()
		done := p.decodeDone && p.q.Depth() == 0 && p.hasShown
		loop := p.loop
		p.mu.Unlock()
		if !loop && done {
			return nil, true
		}
		return nil, false
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
	due := p.audioClk.DuePTSMS()
	fr, _, ok := p.audioQ.PollDue(due)
	if !ok {
		if !p.loop && p.streamAudioDrained() {
			return nil, true
		}
		return nil, false
	}
	if fr.Serial != atomic.LoadInt64(&p.generation) {
		if !p.loop && p.streamAudioDrained() {
			return nil, true
		}
		return nil, false
	}
	// Sound clock follows played PCM (sdl_audio_callback shape): the
	// stamp just played becomes the new base, so masterDue tracks the
	// speaker, not the wall.
	p.audioClk.Start(fr.PTSMs)
	p.mu.Lock()
	p.audioShown++
	p.lastAudioShown = fr.PTSMs
	p.hasAudioShown = true
	p.mu.Unlock()
	if !p.loop && p.streamAudioDrained() {
		return fr, true
	}
	return fr, false
}

// fillAudioStats rides the A2 waterline onto Stats (silent: Master=video
// plus zeros, honest unavailable).
func (p *Player) fillAudioStats(st *Stats) {
	if p == nil || st == nil {
		return
	}
	if !p.hasAudio {
		st.Master = MasterVideo
		return
	}
	p.mu.Lock()
	dec, shown := p.audioDecoded, p.audioShown
	aShown, hasAShown := p.lastAudioShown, p.hasAudioShown
	vShown, hasVShown := p.lastShown, p.hasShown
	p.mu.Unlock()
	depth, dropped := 0, int64(0)
	if p.audioQ != nil {
		depth, dropped = p.audioQ.Depth(), p.audioQ.Dropped()
	}
	st.Master = MasterAudio
	st.AudioDecoded = dec
	st.AudioShown = shown
	st.AudioDepth = depth
	st.AudioDropped = dropped
	if hasAShown && hasVShown {
		st.AVDiffMs = aShown - vShown
	}
}
