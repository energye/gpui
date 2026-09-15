// A2 audio-video sync core (VW6 §11.3 A2, §12 A2 row).
//
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
package video

import (
	"fmt"
	"sync"
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

// Push adds a frame, waiting while full. False when stopped (Close).
func (q *AudioQueue) Push(f *AudioFrame) (bool, error) {
	if f == nil {
		return false, fmt.Errorf("video: nil audio frame")
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

// Pushes counts accepted frames.
func (q *AudioQueue) Pushes() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pushes
}

// MaxDepth is the deepest fill seen.
func (q *AudioQueue) MaxDepth() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.maxDepth
}

// DepthAvg is the mean fill over sampled operations.
func (q *AudioQueue) DepthAvg() float64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.depthN == 0 {
		return 0
	}
	return float64(q.depthSum) / float64(q.depthN)
}

// Close stops the queue: blocked Push returns false, Drained turns true
// once remains are eaten.
func (q *AudioQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.closed = true
	q.room.Broadcast()
}

// Drained reports no more frames will ever come out.
func (q *AudioQueue) Drained() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed && len(q.buf) == 0
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

// Drain hands every queued frame to the closer (ownership moves).
func (q *AudioQueue) Drain() []*AudioFrame {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := append([]*AudioFrame(nil), q.buf...)
	for i := range q.buf {
		q.buf[i] = nil
	}
	q.buf = q.buf[:0]
	return out
}
