package video

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/energye/gpui/video/clock"
)

// Sentinel errors. Messages stay in plain English so callers can match
// with errors.Is and show their own localized text on top.
var (
	ErrNoVideo   = errors.New("video: no video track")
	ErrNoAudio   = errors.New("video: no audio track")
	ErrNoFrames  = errors.New("video: no decodable frames")
	ErrClosed    = errors.New("video: player closed")
	ErrBadClip   = errors.New("video: bad clip")
	ErrDecodeEOF = errors.New("video: end of stream")
	// ErrMemOverCap is the S7 fail-fast: the pre-decode estimate exceeds
	// the grade cap (assembling the clip would not fit). Callers triage
	// it via Classify (KindMemOverCap), never as a silent OOM.
	ErrMemOverCap = errors.New("video: memory over cap")
)

// Info describes an opened clip.
type Info struct {
	Path      string
	Width     int
	Height    int
	Profile   string
	FrameRate float64
	DurMs     int64
	Frames    int
	KeyframeN int
	// Container and Codec name the backend that opened the clip.
	Container string
	Codec     string
	// Concealed counts bad samples skipped so far (always 0: ffmpeg
	// absorbs corrupt frames inside native code).
	Concealed int64
	// Fault names the first problem, "" when clean.
	Fault string
	// A2 sound presence: true when the clip opened with a sound track
	// (ffmpeg decodes it to 48kHz stereo float); silent clips stay
	// false and play video-only bit for bit like before.
	HasAudio        bool
	AudioSampleRate int
	AudioChannels   int
	AudioSamples    int
}

// Stats is the playback waterline the window HUD and JSON report.
type Stats struct {
	Decoded     int64
	Shown       int64
	Dropped     int64
	QueueDepth  int
	QueueMax    int
	QueueAvg    float64
	DecodeMsAvg float64
	DecodeMsP95 float64
	DriftMs     int64
	Ended       bool
	Error       string
	// PoolHitPct is the streaming RGBA pool hit% (S6 §11.7).
	PoolHitPct float64
	// Seek evidence (VR5): last seek landing vs request.
	SeekOK       int
	SeekDeltaMs  int64
	SeekForward  int64
	SeekLandedMs int64
	// Concealed mirrors Info.Concealed for HUD/JSON (always 0).
	Concealed int64
	// Rate is the playback speed (1 = normal); Seeking reports a seek
	// still travelling to its landing frame.
	Rate    float64
	Seeking int
	// S7 cap evidence (§11.7): the grade cap enforced at open, the
	// pre-decode estimate checked against it, and pooled evictions
	// during play (cap overflow drops, never silent growth). Zero
	// evictions is the healthy value.
	MemCapKB      int
	EstimateB     int64
	PoolEvictions int64
	// A2 AV sync evidence: audio master with real counts when a track
	// exists, else video master with zero audio (honest, never faked).
	Master       string
	AVDiffMs     int64
	AudioDecoded int64
	AudioShown   int64
	AudioDepth   int
	AudioDropped int64
}

// Options tunes the player. QueueCap <= 0 means clock.DefaultCap;
// NowMs nil means the wall clock. Loop replays from the first stamp
// (stamps keep counting up so the clock never jumps back).
// S2Parallel stays accepted for compatibility but is a no-op: ffmpeg
// owns threading natively. AudioQueueCap <= 0 means DefaultAudioQueueCap.
type Options struct {
	QueueCap      int
	NowMs         func() int64
	Loop          bool
	S2Parallel    bool
	AudioQueueCap int
}

// pooledLive is one streaming pool snapshot (S6 §11.7): immutable per
// resolution, swapped only on resolution change (cold path, never steady
// play). Shared atomically between the decoder thread and the display
// thread (Poll releases, queue OnDrop returns drops).
type pooledLive struct {
	pools    *Pools
	w, h     int
	workSize int
	capBytes int
}

// Player decodes in the background and serves frames by timestamp.
// Open with OpenFile; poll with Poll; Pause truly stops the picture;
// Seek jumps to a time (non-loop only); Close ends the thread.
// Poll is safe for one display thread; Pause, Resume, Seek and Stats
// are safe from any thread.
//
// Seeks are requests, not chores (ffplay stream_seek model): SeekTo only
// posts the target and returns the landing stamp at once; the background
// runs the actual av_seek_frame between two Next calls, drops stale tail
// frames until the landing shows, and the caller keeps polling with the
// old picture held (never black). A second seek supersedes the first.
//
// Decode backend: ffmpeg (video/ffmpeg, libgpui_ffmpeg via purego) is the
// only decode path: its demuxer opens the path directly, decodes the best
// video stream, and scales each picture to RGBA straight into pooled
// buffers. The background thread owns every ffmpeg call.
type Player struct {
	info  Info
	q     *clock.Queue
	clk   *clock.Clock
	nowMs func() int64

	mu        sync.Mutex
	paused    bool
	ended     bool
	closed    bool
	err       string
	decoded   int64
	shown     int64
	lastShown int64
	hasShown  bool
	decTimes  []float64
	// decodeDone latches when the background exhausted the stream
	// (non-loop): Poll ends once the queue also drains.
	decodeDone bool

	path      string
	frameRate float64
	container string
	codec     string
	// Last seek evidence for Stats/JSON.
	seekOK       bool
	seekDeltaMs  int64
	seekForward  int64
	seekLandedMs int64
	seekTargetMs int64
	seekKeyMs    int64

	// Streaming decode state (dmu guards position + seek handshake).
	dmu        sync.Mutex
	generation int64 // atomic: Seek bumps to invalidate in-flight work
	epoch      int64 // loop PTS offset across passes
	nextSeq    int64
	lastPTS    int64 // last emitted PTS (monotonic guard)
	hasFirst   bool
	// Seek request state (dmu-guarded): the background drops decoded
	// frames below seekLanded and shows the first at/above it, then
	// clears the flag.
	seekActive bool
	seekLanded int64
	// wakeCh wakes a decoder parked at end-of-stream (cap 1, coalescing,
	// never closed): a later seek revives playback on the same thread.
	wakeCh chan struct{}

	// Buffered stays false: every clip streams on the ffmpeg backend
	// (no whole-clip path). Kept so the Poll/Stats shape never branches
	// on a removed mode.
	buffered bool
	// S7 cap evidence: grade cap enforced at open + estimate checked.
	memCapKB  int
	estimateB int64
	// S6 streaming pool. convertPic is gone (ffmpeg scales straight
	// into pooled buffers); Poll, queue drops, seeks and Close return
	// them.
	pooled atomic.Pointer[pooledLive]
	// lastPix is the currently displayed Pix: valid until the next Poll
	// or Close, then recycled. Guarded by mu (Poll is one display
	// thread, Close races it).
	lastPix []byte
	// ffdec is the ffmpeg backend decoder.
	ffdec *ffDecoder
	// ffaud is the ffmpeg backend audio decoder (nil on silent clips).
	// Same single-thread rule as ffdec: only the background decode loop
	// touches it (Next and SeekTo alike).
	ffaud *ffAudio
	// aq is the decoded PCM line: the background pushes AudioFrames, the
	// speaker pump (or tests) pulls the newest due via PollAudio. Same
	// contract as clock.Queue, over sound frames.
	aq *AudioQueue
	// audioDone latches when the background exhausted the sound stream
	// (non-loop): PollAudio ends once the queue also drains.
	audioDone bool
	// lastAudioPTSMS is the last served sound stamp (masterDue caps the
	// picture schedule against it; AVDiffMs reads it against lastShown;
	// guarded by mu like the picture side).
	lastAudioPTSMS int64
	// volume scales PCM in PollAudio (1 = unchanged, 0 = silent).
	// Guarded by mu; the pump sees already-scaled data.
	volume float64
	// muted parks the speaker: PollAudio returns (nil, false) while
	// set (decode keeps running so unmute resumes in sync).
	// Guarded by mu.
	muted bool
	// audioDecoded/audioShown ride Stats (evidence, never faked).
	audioDecoded int64
	audioShown   int64
	// ffmpeg seek handshake (dmu-guarded, see backend_ffmpeg.go): the caller
	// thread only posts a request, the background thread runs the actual
	// av_seek_frame between two Next calls (ffmpeg contexts are not
	// thread-safe). ffSeekMu serializes concurrent SeekTo callers.
	ffSeekReq    bool
	ffSeekTarget int64
	ffSeekDone   chan struct{}
	ffSeekLanded int64
	ffSeekErr    error
	ffSeekMu     sync.Mutex
	// ffTemp is a spooled temp file for memory sources (BytesSource):
	// ffmpeg opens paths/URLs itself, so bytes are staged here and the
	// file is removed on Close. Empty on the normal path/URL path.
	ffTemp string

	stopCh  chan struct{}
	doneCh  chan struct{}
	readyCh chan struct{}
	readyDo sync.Once
	loop    bool
	base    int64
}
