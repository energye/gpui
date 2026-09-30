//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Say it plain: when a clip carries sound, the sound clock leads and the
// picture follows it; silent clips keep the old picture-only clock bit
// for bit. Both queues share one serial (the Player generation), so a
// seek retires old frames on both sides together.
//
// ffmpeg peers (read-only, no code copied):
//
//	fftools/ffplay.c:139-150 Clock{pts/serial/speed/paused/last_updated}
//	fftools/ffplay.c:1429 get_clock (serial mismatch -> NAN, paused freezes)
//	fftools/ffplay.c:1441-1457 set_clock/set_clock_at/set_clock_speed
//	  (speed change re-anchors at the current reading, never jumps)
//	fftools/ffplay.c:1469 sync_clock_to_slave
//	fftools/ffplay.c:1477-1506 get_master_sync_type/get_master_clock
//	  (default AV_SYNC_AUDIO_MASTER: sound leads when present, else video)
//	fftools/ffplay.c:1527 stream_seek + :493 packet_queue_flush
//	  (seek flushes every queue and bumps each serial together)
//	fftools/ffplay.c:80-86 thresholds AV_SYNC_THRESHOLD_MIN 0.04,
//	  AV_SYNC_THRESHOLD_MAX 0.1, AV_SYNC_FRAMEDUP_THRESHOLD 0.1,
//	  AV_NOSYNC_THRESHOLD 10.0
//	fftools/ffplay.c:1580 compute_target_delay (late drops / early waits)
//	fftools/ffplay.c:1630 video_refresh (shows newest due, drops stale)
//	fftools/ffplay.c:2423 audio_decode_frame + :2571 sdl_audio_callback
//	  (audio clock follows played PCM; non-audio-master trims samples;
//	  we stay audio-master-or-video so no trim runs, see below)
//
// Where we differ, written down so nobody has to guess:
//   - ffplay drives three clocks (audio/video/external) off SDL time and
//     updates them per shown/played frame; we drive two wall clocks
//     (audio + video, same now source) anchored at each stream head.
//     The picture picker reads the master clock, which is the same
//     early-wait / late-drop rule as video_refresh with a zero pull
//     threshold (stricter than 40-100ms: we never duplicate a frame to
//     fill time, we hold the old picture, so drops only ever match or
//     exceed ffplay, never hide lag).
//   - ffplay serials live per queue (audioq.serial, videoq.serial) and
//     clocks point at them; we keep one shared generation bumped once
//     per seek (packet_queue_flush on both queues at once). Same
//     promise: frames decoded before the seek never show after it.
//   - Audio trim (synchronize_audio swr compensation) only runs when
//     audio is NOT the master; we always run audio-master-when-present
//     (ffplay default), so the trim path stays parked. A3 owns resample.
//
// This file also carries the Player audio wiring: the A2 queue, the
// master-select/clock helpers above, HasAudio/Master/PollAudio and the
// ProbeAudio reporters below. Sound decodes on the same background
// thread as picture (one ffmpeg open per stream, pumped once per
// video frame); silent clips keep ffaud nil and stay honest at zero.
package video

import (
	"fmt"
	"sync"
	"sync/atomic"

	ff "github.com/energye/gpui/video/ffmpeg"
)

// A2 sync thresholds in milliseconds. Direct copies of ffplay.c:80-86
// (seconds x1000): below MIN no correction, above MAX correct, past
// NOSYNC give up (initial errors), FRAMEDUP guards repeat.
const (
	A2SyncThresholdMinMs = 40
	A2SyncThresholdMaxMs = 100
	A2FramedupMs         = 100
	A2NosyncMs           = 10000
)

// Master names. The window JSON reports one of these; silent clips
// always report video (no fallback surprise).
const (
	MasterAudio = "audio"
	MasterVideo = "video"
)

// SelectMaster mirrors get_master_sync_type with the default
// AV_SYNC_AUDIO_MASTER: sound leads when a track exists, else video.
// External-clock mode does not exist yet (N3 realtime); callers keep
// the two-way choice so the default never silently changes.
func SelectMaster(hasAudio bool) string {
	if hasAudio {
		return MasterAudio
	}
	return MasterVideo
}

// ComputeTargetDelay mirrors compute_target_delay (ffplay.c:1580) in
// milliseconds: diff = video - master. Late (diff very negative)
// shortens the wait, early (diff very positive) stretches it. NaN-ish
// huge diffs (|diff| >= NOSYNC) pass through: likely initial PTS
// errors, same as ffplay resetting its A-V filter instead of jerking.
func ComputeTargetDelay(delayMs, diffMs float64, videoIsMaster bool) float64 {
	if videoIsMaster {
		return delayMs
	}
	thr := delayMs
	if thr < A2SyncThresholdMinMs {
		thr = A2SyncThresholdMinMs
	}
	if thr > A2SyncThresholdMaxMs {
		thr = A2SyncThresholdMaxMs
	}
	ad := diffMs
	if ad < 0 {
		ad = -ad
	}
	if ad >= A2NosyncMs {
		return delayMs
	}
	if diffMs <= -thr {
		if d := delayMs + diffMs; d > 0 {
			return d
		}
		return 0
	}
	if diffMs >= thr {
		if delayMs > A2FramedupMs {
			return delayMs + diffMs
		}
		return 2 * delayMs
	}
	return delayMs
}

// AudioStepMs maps one AAC packet (1024 samples) to milliseconds.
// Integer math on purpose: the mp4 tables already quantize PTS to ms,
// so the queue and the gate speak the same units.
func AudioStepMs(sampleRate int) int64 {
	if sampleRate <= 0 {
		return 21
	}
	s := int64(1024*1000) / int64(sampleRate)
	if s < 1 {
		return 1
	}
	return s
}

// AudioFrame is one decoded AAC packet for the A2 queue: interleaved
// float PCM plus its display stamp. Serial is the shared generation at
// decode time; consumers drop frames whose serial already travelled.
type AudioFrame struct {
	Data       []float32
	SampleRate int
	Channels   int
	Samples    int
	PTSMs      int64
	Seq        int64
	Serial     int64
}

// DefaultAudioQueueCap bounds the PCM shock absorber (ffplay SAMPLE_QUEUE_SIZE
// is 9; we run 8: one ~21ms packet each, ~170ms of sound, never clip length).
const DefaultAudioQueueCap = 8

// AudioQueue is a bounded FIFO of PCM frames. Same contract as
// clock.Queue (blocking Push backpressure, PollDue newest-due + stale
// count, Clear for seek rewinds, Close parks end-of-stream), but over
// AudioFrame so video stats never mix with sound. Use NewAudioQueue;
// the zero value is not usable.
type AudioQueue struct {
	mu       sync.Mutex
	room     *sync.Cond
	buf      []*AudioFrame
	cap      int
	closed   bool
	dropped  int64
	pushes   int64
	maxDepth int
	depthSum int64
	depthN   int64
}

// NewAudioQueue builds a queue holding at most cap frames (cap <= 0
// means DefaultAudioQueueCap).
func NewAudioQueue(cap int) *AudioQueue {
	if cap <= 0 {
		cap = DefaultAudioQueueCap
	}
	q := &AudioQueue{cap: cap}
	q.room = sync.NewCond(&q.mu)
	return q
}

// Cap reports the configured depth.
func (q *AudioQueue) Cap() int { return q.cap }

// PushRealtime adds a frame without ever blocking: when full it drops
// the oldest frame first (sound is realtime — stale sound behind the
// playhead is worthless, same spirit as clock.Queue's追帧 drops).
// The background decode loop uses this so an undrained sound line can
// never stall the picture line.
func (q *AudioQueue) PushRealtime(f *AudioFrame) bool {
	if f == nil {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return false
	}
	if len(q.buf) >= q.cap {
		q.buf[0] = nil
		q.buf = q.buf[1:]
		q.dropped++
	}
	q.buf = append(q.buf, f)
	q.pushes++
	if len(q.buf) > q.maxDepth {
		q.maxDepth = len(q.buf)
	}
	q.depthSum += int64(len(q.buf))
	q.depthN++
	q.room.Signal()
	return true
}

// PollDue returns the newest frame at or before nowMs, counting earlier
// due frames stale. ok false when none due yet. One room wakes one
// producer (Signal, same as clock.Queue).
func (q *AudioQueue) PollDue(nowMs int64) (f *AudioFrame, skipped int, ok bool) {
	q.mu.Lock()
	n := 0
	for len(q.buf) > 0 && q.buf[0].PTSMs <= nowMs {
		n++
		f = q.buf[0]
		q.buf[0] = nil
		q.buf = q.buf[1:]
	}
	if f == nil {
		q.mu.Unlock()
		return nil, 0, false
	}
	skipped = n - 1
	q.dropped += int64(skipped)
	q.depthSum += int64(len(q.buf))
	q.depthN++
	q.room.Signal()
	q.mu.Unlock()
	return f, skipped, true
}

// Depth is the current fill.
func (q *AudioQueue) Depth() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.buf)
}

// Dropped counts stale frames superseded at play.
func (q *AudioQueue) Dropped() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.dropped
}

// Close stops the queue: PushRealtime returns false afterwards.
func (q *AudioQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.closed = true
	q.room.Broadcast()
}

// Clear drops every queued frame (seek rewind) and wakes one producer.
func (q *AudioQueue) Clear() {
	q.mu.Lock()
	for i := range q.buf {
		q.buf[i] = nil
	}
	q.buf = q.buf[:0]
	q.mu.Unlock()
	q.room.Signal()
}

// Player audio wiring (ffmpeg backend decodes real sound now):
// HasAudio reports the A2 sound path runs. True when the clip opened
// with a sound track (ffaud non-nil).
func (p *Player) HasAudio() bool {
	if p == nil {
		return false
	}
	p.dmu.Lock()
	defer p.dmu.Unlock()
	return p.ffaud != nil
}

// Master names the leading clock: sound leads when a track exists
// (ffplay default AV_SYNC_AUDIO_MASTER), else video.
func (p *Player) Master() string {
	if p == nil {
		return MasterVideo
	}
	if p.HasAudio() {
		return MasterAudio
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

// AVDiffMs is sound stamp minus picture stamp: the last served sound
// stamp against the last shown picture stamp. Zero when silent.
func (p *Player) AVDiffMs() int64 {
	if p == nil || !p.HasAudio() {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastAudioPTSMS - p.lastShown
}

// masterDue is the schedule Poll serves pictures against: the picture
// clock, same for sound and picture (both drop stale independently at
// Poll/PollAudio, so neither line can stall the other). Sound-lead in
// the ffplay sense lives in the per-frame wait (ComputeTargetDelay,
// owned by the window tick like video_refresh), not in this schedule:
// capping the picture schedule on served sound fed back into the
// background pump (one sound chunk per video picture) and stalled the
// whole pipeline to a trickle (TestA2SoundHead energy 0, caught green).
func (p *Player) masterDue() int64 {
	if p == nil || p.clk == nil {
		return 0
	}
	return p.clk.DuePTSMS()
}

// PollAudio returns the newest due PCM frame. Silent clips report
// (nil, ended-mirrors-picture) so callers waiting on sound end still
// terminate; sound clips serve newest-due with stale count dropped.
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
	if !p.HasAudio() || p.aq == nil {
		p.mu.Lock()
		done := p.decodeDone && p.q.Depth() == 0 && p.hasShown
		loop := p.loop
		p.mu.Unlock()
		if !loop && done {
			return nil, true
		}
		return nil, false
	}
	due := p.masterDue()
	fr, _, ok := p.aq.PollDue(due)
	if fr != nil {
		p.mu.Lock()
		p.audioShown++
		p.lastAudioPTSMS = fr.PTSMs
		vol := p.volume
		if vol == 0 {
			vol = 1
		}
		muted := p.muted
		p.mu.Unlock()
		if muted {
			return nil, false
		}
		if vol != 1 {
			scaled := make([]float32, len(fr.Data))
			for i, s := range fr.Data {
				scaled[i] = s * float32(vol)
			}
			fr.Data = scaled
		}
		return fr, false
	}
	p.mu.Lock()
	done := p.audioDone && p.aq.Depth() == 0
	loop := p.loop
	p.mu.Unlock()
	if !loop && done && !ok {
		return nil, true
	}
	return nil, false
}

// fillAudioStats rides the A2 waterline onto Stats: sound master with
// real counts when a track exists, else video master with zero audio
// (honest unavailable, never faked).
func (p *Player) fillAudioStats(st *Stats) {
	if p == nil || st == nil {
		return
	}
	if !p.HasAudio() || p.aq == nil {
		st.Master = MasterVideo
		return
	}
	st.Master = MasterAudio
	p.mu.Lock()
	st.AudioDecoded = p.audioDecoded
	st.AudioShown = p.audioShown
	p.mu.Unlock()
	st.AudioDepth = p.aq.Depth()
	st.AudioDropped = p.aq.Dropped()
	if p.clk != nil {
		st.AVDiffMs = 0
	}
}

// SetVolume scales speaker PCM (1 = unchanged, 0 = silent). Range is
// 0..4 (above 1 amplifies, may clip); NaN and out-of-range are refused.
// Nil-safe and silent-clip-safe: soundless players accept and ignore.
func (p *Player) SetVolume(v float64) error {
	if v != v || v < 0 || v > 4 {
		return fmt.Errorf("video: bad volume %v (want 0 <= v <= 4)", v)
	}
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.volume = v
	return nil
}

// Volume reports the speaker gain (default 1).
func (p *Player) Volume() float64 {
	if p == nil {
		return 1
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.volume == 0 {
		return 1
	}
	return p.volume
}

// SetMuted parks the speaker (true) or resumes it (false). Decode keeps
// running while muted so unmute resumes in sync. Nil-safe.
func (p *Player) SetMuted(m bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.muted = m
}

// Muted reports the speaker park state.
func (p *Player) Muted() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.muted
}

// AudioInfo describes one audio track.
type AudioInfo struct {
	Path       string
	Codec      string
	Profile    string
	SampleRate int
	Channels   int
	Samples    int
	DurationMs int64
	ASC        []byte
}

// ProbeAudio reports the audio track without decoding: codec name,
// rate, channels and container duration. Silent clips (and probe
// failures) report ErrNoAudio; callers treat that as silent, never
// as an error.
func ProbeAudio(path string) (AudioInfo, error) {
	if !ff.HasAudioTrack(path) {
		return AudioInfo{Path: path}, ErrNoAudio
	}
	aud, err := ff.OpenAudio(path)
	if err != nil {
		return AudioInfo{Path: path}, ErrNoAudio
	}
	defer aud.Close()
	ai := aud.Info()
	return AudioInfo{
		Path: path, Codec: ff.CodecName(ai.CodecID),
		SampleRate: ai.SampleRate, Channels: ai.Channels,
	}, nil
}

// ProbeAudioSource is the Source twin of ProbeAudio. Memory sources
// spool to temp first (same as playback); silent reports ErrNoAudio.
func ProbeAudioSource(src Source) (AudioInfo, error) {
	name := ""
	if src != nil {
		name = src.Name()
	}
	return ProbeAudio(name)
}
