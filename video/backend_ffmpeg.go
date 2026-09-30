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
	"sync/atomic"
	"time"

	"github.com/energye/gpui/video/clock"
	ff "github.com/energye/gpui/video/ffmpeg"
)

// This file wires the ffmpeg backend behind the Player: ffmpeg owns
// demux, decode, reorder and seek natively; the Player only does
// queue/clock/seek orchestration on top.

// ffDecoder is the ffmpeg backend decoder the player drives
// (production value is always *ff.Decoder).
type ffDecoder = ff.Decoder

// ffAudio is the ffmpeg backend audio decoder (nil on silent clips).
type ffAudio = ff.AudioStream

// openFFmpeg opens path through libgpui_ffmpeg and starts the background
// decoder. The player fields it reuses: queue + clock + ready/stop/done
// channels + Info/Stats counters. Everything Go-decode-specific (samples,
func openFFmpeg(path string, opt Options) (*Player, error) {
	if !ff.Available() {
		return nil, fmt.Errorf("video: ffmpeg library missing (%s): %w", ff.LibPath(), ErrBadClip)
	}
	dec, err := ff.Open(path)
	if err != nil {
		return nil, fmt.Errorf("video: ffmpeg open %s: %v: %w", path, err, ErrBadClip)
	}
	ok := false
	defer func() {
		if !ok {
			dec.Close()
		}
	}()
	info := dec.Info()
	if info.Width <= 0 || info.Height <= 0 {
		return nil, fmt.Errorf("%w: %s bad size %dx%d", ErrBadClip, path, info.Width, info.Height)
	}
	now := opt.NowMs
	if now == nil {
		now = wallMs
	}
	qcap := opt.QueueCap
	if qcap <= 0 {
		qcap = clock.DefaultCap
	}
	q := clock.NewQueue(qcap)
	clk := clock.NewClock(now)
	acap := opt.AudioQueueCap
	if acap <= 0 {
		acap = DefaultAudioQueueCap
	}
	p := &Player{
		info: Info{
			Path: path, Width: info.Width, Height: info.Height,
			FrameRate: info.FPS, DurMs: info.DurMs, Frames: int(info.Frames),
			Container: "ffmpeg", Codec: ffCodecName(info.CodecID),
			HasAudio: false,
		},
		q:      q,
		aq:     NewAudioQueue(acap),
		clk:    clk,
		nowMs:  now,
		stopCh: make(chan struct{}), doneCh: make(chan struct{}), readyCh: make(chan struct{}),
		wakeCh: make(chan struct{}, 1),
	}
	// P1 硬解水位落 Info（打开时刻真值；Stats 读直播值）。
	hw := dec.HWStats()
	p.info.HWActive, p.info.HWName = hw.Active, hw.Name
	p.info.HWFallbacks, p.info.HWTransferMsAvg = hw.Fallbacks, hw.TransferMsAvg
	// Sound rides its own ffmpeg open (demux state is per-open, so a
	// shared open would serialize seeks): silent clips keep ffaud nil
	// and the A2 stubs stay honest at zero.
	if aud, aerr := ff.OpenAudio(path); aerr == nil {
		ai := aud.Info()
		p.ffaud = aud
		p.info.HasAudio = true
		p.info.AudioSampleRate = ai.SampleRate
		p.info.AudioChannels = ai.Channels
	} else {
		p.audioDone = true
		p.aq.Close()
	}
	if p.info.Frames <= 0 && p.info.DurMs > 0 && p.info.FrameRate > 1 {
		p.info.Frames = int(p.info.DurMs * int64(p.info.FrameRate) / 1000)
	}
	p.loop = opt.Loop
	p.path = path
	p.frameRate = info.FPS
	p.container = "ffmpeg"
	p.codec = p.info.Codec
	p.ffdec = dec
	p.memCapKB = MemCapKBFor(info.Width, info.Height)
	p.estimateB = EstimateLiveBytes(info.Width, info.Height, 4, qcap, 256<<10)
	p.q.SetOnDrop(func(fr *clock.Frame) {
		if fr == nil {
			return
		}
		p.releasePix(fr.Pix)
	})
	// Pooled reuse: ffmpeg scales straight into display-pool buffers,
	// so steady play borrows instead of allocating ~w*h*4 per frame.
	// get borrows when the pool already fits this size, else a fresh
	// buffer (cold open / resolution switch); put parks via releasePix
	// (wrong sizes drop and count, never pollute the pool).
	dec.SetPixPool(
		func(size int) []byte {
			if lv := p.pooled.Load(); lv != nil && lv.pools != nil && lv.pools.RGBA != nil &&
				lv.pools.RGBA.BufSize() == size {
				return lv.pools.RGBA.Acquire()
			}
			return make([]byte, size)
		},
		func(b []byte) { p.releasePix(b) },
	)
	// Sync phase: decode the first displayable frame on the opener
	// thread, so Open returns with pixels ready and Info honest. The
	// pool already exists for the real size, so the decode borrows into
	// it: the ffmpeg buffer IS the queued Pix (no second buffer).
	p.remakeLive(info.Width, info.Height)
	t0 := time.Now()
	fr0, err := dec.Next()
	syncEl := float64(time.Since(t0).Microseconds()) / 1000.0
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrNoFrames, path, err)
	}
	cf, el := p.wrapFFFrame(fr0)
	el += syncEl
	p.base = cf.PTSMs
	p.info.Width = cf.Width
	p.info.Height = cf.Height
	p.lastPTS = cf.PTSMs
	if ok, _ := p.q.Push(cf); !ok {
		p.releasePix(cf.Pix)
		return nil, fmt.Errorf("%w: queue closed during open %s", ErrClosed, path)
	}
	p.mu.Lock()
	p.decTimes = append(p.decTimes, el)
	p.decoded++
	p.mu.Unlock()
	p.hasFirst = true
	p.nextSeq = 1
	p.announceReady()
	clk.Start(p.base - frameStepMs(p.info.FrameRate))
	go p.ffDecodeLoop()
	ok = true
	return p, nil
}

// wrapFFFrame converts one ffmpeg RGBA picture into a clock frame.
// superseded display, Close). Loop passes add the epoch offset so stamps
// keep counting up.
func (p *Player) wrapFFFrame(vf *ff.VideoFrame) (*clock.Frame, float64) {
	w, h := vf.Width, vf.Height
	if lv := p.pooled.Load(); lv == nil || lv.w != w || lv.h != h {
		p.remakeLive(w, h)
	}
	buf := vf.Pix
	vf.Pix = nil
	vf.Release()
	pts := vf.PTSMs
	p.dmu.Lock()
	epoch := p.epoch
	p.dmu.Unlock()
	if epoch != 0 {
		pts += epoch
	}
	pts = p.assignPTS(pts)
	fr := &clock.Frame{Width: w, Height: h, Pix: buf, PTSMs: pts, DurMs: frameStepMs(p.frameRate), Seq: p.nextSeq}
	p.nextSeq++
	p.lastPTS = pts
	return fr, 0
}

// ffDecodeLoop serves ffmpeg pictures on the background thread with the
// same queue/backpressure/end-park rules as decodeLoop: Push waits while
// full, end parks with the queue open so a later seek revives playback,
// loop wraps by seeking to zero with stamps counting up.
//
// Thread rule: this loop is the ONLY thread that ever calls the ffmpeg
// backend (Next and SeekTo alike). The format and codec contexts are not
// thread-safe, so SeekTo from the caller thread only posts a request and
// waits; the actual av_seek_frame runs here between two Next calls.
func (p *Player) ffDecodeLoop() {
	defer close(p.doneCh)
	defer p.q.Close()
	for {
		select {
		case <-p.stopCh:
			return
		default:
		}
		if done := p.takeFFSeek(); done {
			continue
		}
		// Honest decode timing (VR7-D): the Next call below owns
		// demux + decode + RGBA scale, so its wall time IS the
		// per-frame decode cost recorded into decTimes (wrapFFFrame
		// itself only transfers ownership, ~0ms).
		t0 := time.Now()
		vf, err := p.ffdec.Next()
		nextEl := float64(time.Since(t0).Microseconds()) / 1000.0
		// Sound pumps on the same background pass: one audio chunk per
		// video picture keeps the PCM line ahead without a second
		// thread (both ffmpeg opens are touched only here).
		p.pumpAudio()
		if err != nil {
			p.mu.Lock()
			loop := p.loop
			p.mu.Unlock()
			if loop {
				if _, serr := p.ffdec.SeekTo(0); serr != nil {
					p.mu.Lock()
					if p.err == "" {
						p.err = serr.Error()
					}
					p.decodeDone = true
					p.mu.Unlock()
					return
				}
				p.dmu.Lock()
				p.epoch += p.ffSpanMs() + frameStepMs(p.frameRate)
				p.dmu.Unlock()
				// Same as the legacy loop wrap: stamps keep counting up
				// via epoch, but display sequence restarts so a second
				// pass shows seq 0 again (TestLoopReplays).
				p.nextSeq = 0
				continue
			}
			p.mu.Lock()
			p.decodeDone = true
			p.mu.Unlock()
			// Sound tail plays out before parking: pump until the
			// sound stream latches done (never blocks — realtime
			// push), so PollAudio ends on real data, not on the
			// picture line's schedule.
			for !p.loadAudioDone() {
				p.pumpAudio()
			}
			select {
			case <-p.stopCh:
				return
			case <-p.wakeCh:
				// A wake means a seek revived the stream: re-read.
				continue
			}
		}
		p.mu.Lock()
		gen := p.generation
		p.mu.Unlock()
		p.dmu.Lock()
		pendingSeek := p.ffSeekReq
		travelling := p.seekActive
		p.dmu.Unlock()
		if pendingSeek {
			// Decoded after the seek was posted but before the
			// background ran it: the picture sits at the pre-seek
			// position, past both the travelling filter (seekActive
			// is not set yet) and the generation check (gen was
			// captured after the bump). Queueing it would park a
			// stale head ahead of the fresh burst and stall exact
			// landings, so drop it here; the loop top runs
			// takeFFSeek next.
			vf.Release()
			continue
		}
		if travelling {
			// Seek recovery: ffmpeg already jumped on SeekTo, so the
			// first picture at/above the landing ends the seek; older
			// ones (stale flush tail) drop before queueing.
			p.dmu.Lock()
			if vf.PTSMs < p.seekLanded {
				p.dmu.Unlock()
				vf.Release()
				continue
			}
			p.seekActive = false
			p.dmu.Unlock()
		}
		cf, el := p.wrapFFFrame(vf)
		el += nextEl
		if ok, _ := p.q.Push(cf); !ok {
			p.releasePix(cf.Pix)
			return
		}
		p.dmu.Lock()
		if p.generation != gen {
			// A seek landed while this frame waited in Push: drop the
			// stale line so it never shows ahead of the new landing.
			p.dmu.Unlock()
			p.q.Clear()
			continue
		}
		p.dmu.Unlock()
		p.mu.Lock()
		p.decTimes = append(p.decTimes, el)
		p.decoded++
		p.mu.Unlock()
		p.announceReady()
	}
}

// pumpAudio decodes one sound chunk into the PCM line. Background
// thread only (same pass as the video Next above, so both ffmpeg
// opens stay single-threaded). Silent clips (ffaud nil) no-op. The
// push never blocks (realtime drop-oldest), so an undrained sound
// line can never stall the picture line. At end of sound it latches
// audioDone with the queue left open (same as the video end-park),
// so a later seek revives sound; loop mode re-seeks sound with picture.
func (p *Player) pumpAudio() {
	aud := p.ffaud
	if aud == nil {
		return
	}
	p.mu.Lock()
	done := p.audioDone
	p.mu.Unlock()
	if done {
		return
	}
	af, err := aud.Next()
	if err != nil {
		p.mu.Lock()
		loop := p.loop
		p.mu.Unlock()
		if loop {
			if _, serr := aud.SeekTo(0); serr != nil {
				p.mu.Lock()
				p.audioDone = true
				p.mu.Unlock()
			} else {
				p.aq.Clear()
			}
			return
		}
		p.mu.Lock()
		p.audioDone = true
		p.mu.Unlock()
		return
	}
	fr := &AudioFrame{
		Data: af.Data, SampleRate: af.SampleRate, Channels: af.Channels,
		Samples: af.Samples, PTSMs: af.PTSMs, Seq: p.nextSeq,
		Serial: atomic.LoadInt64(&p.generation),
	}
	if !p.aq.PushRealtime(fr) {
		return
	}
	p.mu.Lock()
	p.audioDecoded++
	p.mu.Unlock()
}

// loadAudioDone reports the latched sound-end (background + display).
func (p *Player) loadAudioDone() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.audioDone
}

// ffSpanMs reports the clip span for loop wrap (ffmpeg backend: Info).
func (p *Player) ffSpanMs() int64 {
	if p.info.DurMs > 0 {
		return p.info.DurMs
	}
	return 0
}

// takeFFSeek runs one posted seek request on the background thread and
// reports whether it did. The caller thread never touches ffmpeg: it only
// posts ffSeekTarget and waits on ffSeekDone (see seekFFmpeg).
func (p *Player) takeFFSeek() bool {
	p.dmu.Lock()
	if !p.ffSeekReq {
		p.dmu.Unlock()
		return false
	}
	target := p.ffSeekTarget
	done := p.ffSeekDone
	p.ffSeekReq = false
	p.dmu.Unlock()

	// No locks held here: the background is between two Next calls, so
	// no ffmpeg call runs anywhere else.
	// Echo-normalize the landing: ffmpeg seeks to a keyframe, but the
	// player contract lands the target stamp itself (the background
	// filter then drops anything below it, so the first shown picture
	// is the covering frame). This keeps SeekTo/SeekFast/SeekBy/Step
	// exact on every clip instead of keyframe-granular.
	var landed int64
	var serr error
	if _, serr = p.ffdec.SeekTo(target); serr != nil {
		p.dmu.Lock()
		p.seekActive = false
		p.ffSeekErr = serr
		p.dmu.Unlock()
		close(done)
		return true
	}
	landed = target
	err := error(nil)

	// Sound travels with picture: same target, same background pass.
	// A sound-seek error never fails the seek (silent fallback); the
	// PCM line clears and replays from the landing either way.
	if p.ffaud != nil {
		if _, aerr := p.ffaud.SeekTo(target); aerr != nil {
			p.ffaud.Close()
			p.ffaud = nil
			p.mu.Lock()
			p.audioDone = true
			p.mu.Unlock()
			p.aq.Close()
		} else {
			p.aq.Clear()
			p.mu.Lock()
			p.audioDone = false
			p.mu.Unlock()
		}
	}

	p.dmu.Lock()
	p.seekActive = err == nil
	p.seekLanded = landed
	p.ffSeekLanded = landed
	p.ffSeekErr = err
	// A backward seek replays earlier stamps by design: reset the
	// monotonic guard so wrapFFFrame's assignPTS does not push the new
	// segment's pictures past the stale high-water mark (that used to
	// turn a seek to 1000 into queued 2000s). hasFirst stays true (the
	// clock anchor logic needs it); lastPTS is what assignPTS reads.
	p.lastPTS = landed - frameStepMs(p.frameRate)
	p.dmu.Unlock()
	close(done)
	return true
}

// seekFFmpeg posts an ffmpeg seek and waits for the background thread to
// run it (see takeFFSeek). It clears the queued line, re-anchors the
// clock on the actual landing and wakes a decoder parked at end-of-stream.
// keyOnly is accepted for the SeekFast shape (ffmpeg always lands
// keyframes). Concurrent seeks serialize; a second one waits for the
// first instead of racing it inside native code.
func (p *Player) seekFFmpeg(targetMs int64, keyOnly bool) (int64, error) {
	_ = keyOnly
	p.ffSeekMu.Lock()
	defer p.ffSeekMu.Unlock()
	// Order matters: generation-bump and queue-clear run BEFORE the
	// request is visible, all under one dmu hold. The background can only
	// queue post-seek pictures after it observes the request, so the
	// Clear can never wipe a fresh landing (the old post-then-clear order
	// lost exact-grid frames whenever the background won the race and the
	// display waited forever). In-flight stale frames are still caught by
	// the generation check after Push in ffDecodeLoop.
	p.dmu.Lock()
	done := make(chan struct{})
	atomic.AddInt64(&p.generation, 1)
	p.q.Clear()
	p.ffSeekReq = true
	p.ffSeekTarget = targetMs
	p.ffSeekDone = done
	p.ffSeekErr = nil
	p.dmu.Unlock()
	// Wake a decoder parked at end-of-stream (coalescing send).
	select {
	case p.wakeCh <- struct{}{}:
	default:
	}
	select {
	case <-done:
	case <-p.stopCh:
		return 0, fmt.Errorf("%w: seek after close", ErrClosed)
	case <-time.After(15 * time.Second):
		p.mu.Lock()
		streamErr := p.err
		p.mu.Unlock()
		if streamErr != "" {
			return 0, fmt.Errorf("%w: seek %dms: stream dead %s", ErrBadClip, targetMs, streamErr)
		}
		return 0, fmt.Errorf("%w: seek %dms timed out", ErrBadClip, targetMs)
	}
	p.dmu.Lock()
	landed, err := p.ffSeekLanded, p.ffSeekErr
	p.dmu.Unlock()
	if err != nil {
		return 0, err
	}
	delta := targetMs - landed
	if delta < 0 {
		delta = -delta
	}
	wasPaused := p.Paused()
	p.clk.Start(landed)
	if wasPaused {
		p.clk.Pause()
	}
	p.mu.Lock()
	p.ended = false
	p.decodeDone = false
	p.hasShown = false
	p.seekOK = true
	p.seekDeltaMs = delta
	p.seekForward = 1
	p.seekLandedMs = landed
	p.seekTargetMs = targetMs
	p.seekKeyMs = landed
	p.mu.Unlock()
	p.announceReady()
	return landed, nil
}

// ffCodecName names the codec id for Info/Stats (short, human).
func ffCodecName(id int32) string {
	switch id {
	case 27:
		return "h264"
	case 173:
		return "h265"
	case 167:
		return "vp9"
	case 225:
		return "av1"
	case 12:
		return "mpeg4"
	case 2:
		return "mpeg2video"
	case 139:
		return "vp8"
	case 86017:
		return "mp3"
	case 86018:
		return "aac"
	case 86076:
		return "opus"
	case 86021:
		return "vorbis"
	case 86019:
		return "ac3"
	default:
		return fmt.Sprintf("ffmpeg-%d", id)
	}
}
