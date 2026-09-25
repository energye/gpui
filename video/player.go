package video

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/video/clock"
	ff "github.com/energye/gpui/video/ffmpeg"
)

// ffDecoder is the ffmpeg backend decoder the player drives
// (production value is always *ff.Decoder).
type ffDecoder = ff.Decoder

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
	// A2 sound presence: always false, the ffmpeg backend is video-only
	// until audio decode lands (see t-audio-ffmpeg). Silent-clip
	// semantics hold bit for bit.
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
	// A2 AV sync evidence: always video master with zero audio on the
	// video-only backend (honest unavailable, never faked).
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
// owns threading natively. AudioQueueCap is accepted and ignored
// (video-only backend).
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
	// ffmpeg seek handshake (dmu-guarded, see ff_player.go): the caller
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

func (p *Player) announceReady() {
	p.readyDo.Do(func() { close(p.readyCh) })
}

// OpenFile opens path and starts the background decoder. The player owns
// nothing caller-side; Close must be called. Decoding runs on ffmpeg
// (video/ffmpeg): its demuxer opens the path directly, so any container,
// codec, protocol or URL the bundled libgpui_ffmpeg supports plays —
// local files, http(s), rtmp and friends included. Missing files keep
// their os error; unopenable inputs fail readably, never silently.
func OpenFile(path string, opt Options) (*Player, error) {
	// Same cleaning as NewSource: drag-drop file://, spaces, brackets.
	p := strings.TrimSpace(path)
	p = strings.Trim(p, "<>")
	if strings.HasPrefix(p, "file://") {
		p = strings.TrimPrefix(p, "file://")
	}
	if p == "" {
		return nil, fmt.Errorf("%w: empty path", ErrBadClip)
	}
	if !IsURL(p) {
		if _, err := os.Stat(p); err != nil {
			return nil, err
		}
	}
	return openFFmpeg(p, opt)
}

// OpenWithSource opens a kept-open Source (file, memory, HTTP Range) and
// starts the background decoder. Caller must not Close src after success
// (Player owns it); on failure src is left open for the caller to close.
//
// Backend note: ffmpeg opens the path/URL itself. File and URL sources
// route by name; memory sources are spooled to a temp .mp4 which is
// removed on Close, so old callers keep working.
func OpenWithSource(src Source, opt Options) (*Player, error) {
	if src == nil {
		return nil, fmt.Errorf("%w: nil source", ErrBadClip)
	}
	if bs, ok := src.(*BytesSource); ok {
		f, err := os.CreateTemp("", "gpui-ffmpeg-*.mp4")
		if err != nil {
			return nil, fmt.Errorf("video: spool memory source %s: %w", src.Name(), err)
		}
		fname := f.Name()
		if _, err := f.Write(bs.b); err != nil {
			f.Close()
			os.Remove(fname)
			return nil, fmt.Errorf("video: spool memory source %s: %w", src.Name(), err)
		}
		if err := f.Close(); err != nil {
			os.Remove(fname)
			return nil, fmt.Errorf("video: spool memory source %s: %w", src.Name(), err)
		}
		p, err := openFFmpeg(fname, opt)
		if err != nil {
			os.Remove(fname)
			return nil, err
		}
		p.ffTemp = fname
		src.Close()
		return p, nil
	}
	p, err := openFFmpeg(src.Name(), opt)
	if err != nil {
		return nil, err
	}
	// The ffmpeg demuxer owns its own handle; the passed Source is no
	// longer needed, so close it here to keep the old ownership rule
	// (caller must not close after success) leak-free.
	src.Close()
	return p, nil
}

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

// releasePix returns a streaming convert buffer (S6). No-op for nil and
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
// (nil, true) when playback ended. Streaming Pix lifetime (S6): the
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
		// S6: the display now owns fr.Pix; the previously shown
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
	return st
}

// ConcealedFault reports the first skipped sample's problem; always nil
// on the ffmpeg backend (native code absorbs corrupt frames).
func (p *Player) ConcealedFault() error {
	return nil
}

// SeekTo jumps to targetMs and returns the landing stamp at once.
// The player contract lands the target stamp itself (echo): ffmpeg seeks
// natively to a keyframe, the background filter drops below it, and the
// first shown picture covers the target. Loop players are refused
// (loop+seek goes to VC1).
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
func (p *Player) Close() {
	p.mu.Lock()
	already := p.closed
	p.closed = true
	p.mu.Unlock()
	select {
	case <-p.stopCh:
	default:
		if !already {
			close(p.stopCh)
		}
	}
	p.q.Close()
	<-p.doneCh
	p.dmu.Lock()
	ffdec := p.ffdec
	p.ffdec = nil
	p.dmu.Unlock()
	for _, fr := range p.q.Drain() {
		if fr == nil {
			continue
		}
		p.releasePix(fr.Pix)
	}
	p.mu.Lock()
	last := p.lastPix
	p.lastPix = nil
	p.mu.Unlock()
	p.releasePix(last)
	if ffdec != nil {
		ffdec.Close()
	}
	p.mu.Lock()
	ffTemp := p.ffTemp
	p.ffTemp = ""
	p.mu.Unlock()
	if ffTemp != "" {
		os.Remove(ffTemp)
	}
}
