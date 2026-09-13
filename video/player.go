package video

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/energye/gpui/video/clock"
	"github.com/energye/gpui/video/color"
	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/mp4"
)

// Sentinel errors. Messages stay in plain English so callers can match
// with errors.Is and show their own localized text on top.
var (
	ErrNoVideo   = errors.New("video: no video track")
	ErrNoFrames  = errors.New("video: no decodable frames")
	ErrClosed    = errors.New("video: player closed")
	ErrBadClip   = errors.New("video: bad clip")
	ErrDecodeEOF = errors.New("video: end of stream")
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
	// Container and Codec name the registry entries that opened the
	// clip (capability-first UI reads these instead of guessing).
	Container string
	Codec     string
	// Concealed counts bad samples skipped at open (F20 isolation):
	// truncated/corrupt frames isolated, good tail keeps playing.
	Concealed int64
	// Fault names the first skipped sample's problem, "" when clean.
	Fault string
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
	// Seek evidence (VR5): last seek landing vs request.
	SeekOK       int
	SeekDeltaMs  int64
	SeekForward  int64
	SeekLandedMs int64
	// Concealed mirrors Info.Concealed for HUD/JSON.
	Concealed int64
}

// Options tunes the player. QueueCap <= 0 means clock.DefaultCap;
// NowMs nil means the wall clock. Loop replays from the first stamp
// (stamps keep counting up so the clock never jumps back).
type Options struct {
	QueueCap int
	NowMs    func() int64
	Loop     bool
}

// Player decodes in the background and serves frames by timestamp.
// Open with OpenFile; poll with Poll; Pause truly stops the picture;
// Seek jumps to a time (non-loop only in VR5); Close ends the thread.
// Poll is safe for one display thread; Pause, Resume, Seek and Stats
// are safe from any thread (Seek blocks for its forward decode).
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
	// live counts frames the queue still owes the display; Poll flips
	// Ended once it hits zero on a non-loop run.
	live int64
	// Seek inputs (kept for forward re-decode from the landing keyframe).
	path      string
	samples   []mp4.Sample
	keyframes []mp4.Keyframe
	avcc      *h264.AVCC
	copt      color.Options
	frameRate float64
	// container/codec/sampling ride along for seek re-decode: the
	// forward pass rebuilds through the same registry entries, never
	// by naming a format.
	container string
	codec     string
	sampling  string
	// frameSamples parallels frames: decode-order sample position that
	// produced each display-order frame.
	frameSamples []int
	// Last seek evidence for Stats/JSON.
	seekOK       bool
	seekDeltaMs  int64
	seekForward  int64
	seekLandedMs int64
	seekTargetMs int64
	seekKeyMs    int64
	// VR6 conceal evidence: bad samples skipped at open.
	concealed  int64
	firstFault error

	stopCh chan struct{}
	doneCh chan struct{}
	// readyCh closes once the first frame is queued, so hand-clock tests
	// (and real callers) never poll an empty queue by accident.
	readyCh chan struct{}
	frames  []*clock.Frame
	next    int
	loop    bool
	base    int64
}

// OpenFile opens path and starts the background decoder. The player owns
// nothing caller-side; Close must be called. Container and codec are
// resolved through the registry: the same clip that probes also plays,
// and unknown shells/codecs fail with the supported set named.
func OpenFile(path string, opt Options) (*Player, error) {
	movie, containerName, codecName, err := openViaRegistry(path)
	if err != nil {
		if errors.Is(err, ErrUnsupportedContainer) || errors.Is(err, ErrUnsupportedCodec) {
			return nil, err
		}
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return nil, fmt.Errorf("video: box unreadable %s: %w", path, err)
	}
	v := movie.Video
	if v == nil || len(v.Samples) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoVideo, path)
	}
	avcc, err := h264.ParseAVCC(v.AVCConfig)
	if err != nil {
		return nil, fmt.Errorf("video: header params %s: %w", path, err)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec, err := NewDecoder(codecName)
	if err != nil {
		return nil, fmt.Errorf("video: codec %s: %w", path, err)
	}
	for _, raw := range avcc.SPS {
		if err := dec.DecodeNALU(raw); err != nil {
			return nil, fmt.Errorf("video: sequence params %s: %w", path, err)
		}
	}
	for _, raw := range avcc.PPS {
		if err := dec.DecodeNALU(raw); err != nil {
			return nil, fmt.Errorf("video: picture params %s: %w", path, err)
		}
	}
	var sps *h264.SPS
	if len(avcc.SPS) > 0 {
		if sps, err = h264.ParseSPS(avcc.SPS[0]); err != nil {
			return nil, fmt.Errorf("video: sequence params %s: %w", path, err)
		}
	}
	// F1/F2 gate at open: non-B/M/H profiles and levels past 5.2 fail
	// fast with a namable error instead of flowering later.
	if err := checkStreamLimits(sps, path); err != nil {
		return nil, err
	}
	copt := color.Options{}
	if sps != nil && sps.VUI != nil {
		copt = color.OptionsFromVUI(sps.VUI.FullRange, sps.VUI.ColourPresent, sps.VUI.ColourMatrix)
	}
	// Decode every sample in stream order, then sort by the container PTS
	// into presentation order (B frames need later samples first).
	// The container stamps already carry the ctts correction, so no POC
	// math is needed here: equal POC ties keep sample order via pts.
	type sized struct {
		pic *h264.Picture
		pts int64
		// spos is the decode-order sample position (0-based into
		// v.Samples) that produced this picture; Seek needs it to map
		// a display frame back to its sample for forward re-decode.
		spos int
	}
	var pics []*sized
	var concealed int64
	var firstFault error
	remember := func(err error) {
		if firstFault == nil {
			firstFault = err
		}
	}
	// reset rebuilds a clean decoder after a corrupt sample fed partial
	// state: the bad frame is dropped, the next IDR restarts the tail.
	// Truncated reads feed nothing, so they keep the live decoder.
	// The rebuild goes through the same registry entry.
	reset := func() {
		nd, err := NewDecoder(codecName)
		if err != nil {
			return
		}
		for _, raw := range avcc.SPS {
			_ = nd.DecodeNALU(raw)
		}
		for _, raw := range avcc.PPS {
			_ = nd.DecodeNALU(raw)
		}
		dec = nd
	}
	for si, s := range v.Samples {
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			concealed++
			remember(fmt.Errorf("video: sample %d truncated %s (%v): %w", s.Number, path, err, mp4.ErrTruncated))
			continue
		}
		units, err := SplitUnits(codecName, buf, avcc.LengthSize)
		if err != nil {
			// Split failures are truncations; keep refs, skip frame.
			concealed++
			remember(fmt.Errorf("video: sample %d split %s: %w", s.Number, path, err))
			continue
		}
		if err := rejectF17Units(units, s.Number, path); err != nil {
			// F17 is stream-level unsupported: fail the whole clip.
			return nil, err
		}
		fed := false
		var decErr error
		for _, u := range units {
			if err := dec.DecodeNALU(u); err != nil {
				decErr = fmt.Errorf("video: sample %d decode %s: %w", s.Number, path, err)
				break
			}
			fed = true
		}
		if decErr != nil {
			if streamFatal(decErr) {
				return nil, decErr
			}
			concealed++
			remember(decErr)
			reset()
			continue
		}
		if !fed {
			// No slices fed (SEI-only sample): nothing to show.
			continue
		}
		pic, err := dec.FinishPicture()
		if err != nil {
			if streamFatal(err) {
				return nil, fmt.Errorf("video: sample %d finish %s: %w", s.Number, path, err)
			}
			concealed++
			remember(fmt.Errorf("video: sample %d finish %s: %w", s.Number, path, err))
			reset()
			continue
		}
		pics = append(pics, &sized{pic: pic, pts: s.PTSMs, spos: si})
	}
	if len(pics) == 0 {
		if firstFault != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrNoFrames, path, firstFault)
		}
		return nil, fmt.Errorf("%w: %s", ErrNoFrames, path)
	}
	sort.Slice(pics, func(i, j int) bool {
		if pics[i].pts != pics[j].pts {
			return pics[i].pts < pics[j].pts
		}
		return pics[i].pic.POC < pics[j].pic.POC
	})
	now := opt.NowMs
	if now == nil {
		now = wallMs
	}
	// Start anchors stamp postage: the clock begins one step before the
	// first stamp, so the first frame is due on the first tick and later
	// stamps follow in real time. QueueCap 0 means DefaultCap.
	if opt.QueueCap <= 0 {
		opt.QueueCap = clock.DefaultCap
	}
	// The background thread must hold every frame the queue cannot, so
	// the cap needs headroom for the whole clip. Tiny caps only shape
	// burst behaviour; capacity is len(frames) either way.
	cap := opt.QueueCap
	if cap < len(pics) {
		cap = len(pics)
	}
	q := clock.NewQueue(cap)
	clk := clock.NewClock(now)
	p := &Player{info: Info{Path: path, FrameRate: v.FrameRate, DurMs: v.DurationMs, Frames: len(pics), KeyframeN: v.KeyframeCount(), Concealed: concealed, Container: containerName, Codec: codecName}, q: q, clk: clk, nowMs: now, stopCh: make(chan struct{}), doneCh: make(chan struct{}), loop: opt.Loop, readyCh: make(chan struct{})}
	if firstFault != nil {
		p.info.Fault = Classify(firstFault).Readable()
	}
	p.concealed = concealed
	p.firstFault = firstFault
	p.path = path
	p.samples = append([]mp4.Sample(nil), v.Samples...)
	p.keyframes = append([]mp4.Keyframe(nil), v.Keyframes...)
	p.avcc = avcc
	p.copt = copt
	p.frameRate = v.FrameRate
	p.container = containerName
	p.codec = codecName
	p.sampling = dec.Sampling()
	for i, sp := range pics {
		t0 := time.Now()
		cf, err := color.Convert(p.sampling, sp.pic.Y, sp.pic.Cb, sp.pic.Cr, int(sp.pic.Width), int(sp.pic.Height), copt)
		if err != nil {
			return nil, fmt.Errorf("video: color frame %d %s: %w", i, path, err)
		}
		el := float64(time.Since(t0).Microseconds()) / 1000.0
		p.decTimes = append(p.decTimes, el)
		// Stamps come from the container (ctts-corrected PTS); B reorder
		// is display order, so sequence already matches. Only guard
		// against non-monotonic stamps from odd files.
		pts := sp.pts
		if i > 0 && pts <= p.frames[i-1].PTSMs {
			pts = p.frames[i-1].PTSMs + frameStepMs(v.FrameRate)
		}
		if i == 0 {
			p.base = pts
		}
		p.frames = append(p.frames, &clock.Frame{Width: cf.Width, Height: cf.Height, Pix: cf.Pix, PTSMs: pts, DurMs: frameStepMs(v.FrameRate), Seq: int64(i)})
		p.frameSamples = append(p.frameSamples, sp.spos)
		p.decoded++
	}
	if w := p.frames[0].Width; w > 0 {
		p.info.Width = w
		p.info.Height = p.frames[0].Height
	}
	if sps != nil {
		p.info.Profile = sps.Profile
	}
	// Start anchors stamp postage: the clock begins one step before the
	// first stamp, so the head frame is already due on the first tick
	// even when the background thread is still queueing.
	clk.Start(p.frames[0].PTSMs - frameStepMs(v.FrameRate))
	// The background thread owns all queue traffic (with backpressure),
	// so OpenFile never blocks even with a tiny cap.
	p.live = int64(len(p.frames))
	go p.feed()
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

// WallDeadline returns a wall-clock deadline ms milliseconds out.
func WallDeadline(ms int64) int64 { return wallMs() + ms }

// WallPast reports whether a WallDeadline passed.
func WallPast(deadline int64) bool { return wallMs() >= deadline }

// WallSleep pauses the caller (display-side pacing helper).
func WallSleep(ms int64) { time.Sleep(time.Duration(ms) * time.Millisecond) }

// feed serves the frames on the decoder thread with backpressure:
// Push waits while the queue is full, so a tiny cap never piles memory.
// Non-loop pushes the clip once; loop replays with stamps counting up.
// The loop pass is queued eagerly (same tick as the first pass drains),
// so the wrap never gaps the picture.
func (p *Player) feed() {
	defer close(p.doneCh)
	defer p.q.Close()
	announced := false
	announce := func() {
		if !announced {
			announced = true
			close(p.readyCh)
		}
	}
	if !p.loop {
		for _, fr := range p.frames {
			// Announce the head as soon as it is queued so Poll never
			// waits past the first frame.
			queuedHead := p.q.Pushes() == 0
			select {
			case <-p.stopCh:
				return
			default:
			}
			if ok, _ := p.q.Push(fr); !ok {
				return
			}
			if queuedHead {
				announce()
			}
		}
		<-p.stopCh
		return
	}
	// Loop: every pass is queued through Push backpressure; PollDue's
	// due gate paces display, so early queueing never shows early.
	// Push blocks while full and wakes on Queue.Close, so no helper
	// goroutine per frame: steady loop allocates one Frame struct per
	// push only (VR7 profile: the old go+chan per push cost ~8KB stack
	// each and dominated steady bytes).
	epoch := int64(0)
	base := p.frames[0].PTSMs
	for {
		for _, src := range p.frames {
			cp := *src
			cp.PTSMs = base + epoch + (src.PTSMs - p.base)
			select {
			case <-p.stopCh:
				return
			default:
			}
			// Queue cap always fits the clip, and Close wakes a
			// blocked Push via Broadcast, so this cannot hang past
			// Close without any per-frame goroutine or channel.
			if ok, _ := p.q.Push(&cp); !ok {
				return
			}
			// Announce the head as soon as it is queued so Poll never
			// waits past the first frame.
			if p.q.Pushes() == 1 {
				announce()
			}
		}
		epoch += p.spanMs() + frameStepMs(p.info.FrameRate)
	}
}

func (p *Player) spanMs() int64 {
	if len(p.frames) == 0 {
		return 0
	}
	return p.frames[len(p.frames)-1].PTSMs - p.frames[0].PTSMs
}

// Poll returns the newest due frame (nil when none is due, or after the
// stream ended and drained). Ended reports the stream played through;
// callers keep polling until Ended plus nil. The first call waits for
// the background thread to queue the head frame (bounded wait).
// Steady polls cost no timer: readyCh stays closed after the head, so the
// fast path below skips time.After entirely (VR7 profile: one timer per
// Poll dominated steady bytes before this fix).
func (p *Player) Poll() (f *clock.Frame, ended bool) {
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
		case <-time.After(5 * time.Second):
			return nil, false
		}
	}
	due := p.clk.DuePTSMS()
	fr, skipped, ok := p.q.PollDue(due)
	if ok {
		p.mu.Lock()
		p.shown++
		p.lastShown = fr.PTSMs
		p.hasShown = true
		last := false
		if !p.loop {
			// PollDue eats every due frame but shows the newest, so the
			// skipped ones are done too: count them all off live.
			p.live -= int64(skipped) + 1
			if p.live <= 0 {
				p.live = 0
				p.ended = true
				last = true
			}
		}
		p.mu.Unlock()
		// Report Ended on the same call that shows the last frame: the
		// queue may still hold closed-state, but nothing will ever show
		// again, so callers can stop without one more empty poll.
		if last {
			return fr, true
		}
		return fr, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.loop {
		return nil, false
	}
	if p.live <= 0 && p.q.Drained() {
		p.ended = true
		return nil, true
	}
	return nil, false
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

// Info describes the clip.
func (p *Player) Info() Info { return p.info }

// Stats snapshots the waterline.
func (p *Player) Stats() Stats {
	p.mu.Lock()
	ended := p.ended
	live := p.live
	p.mu.Unlock()
	times := append([]float64(nil), p.decTimes...)
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
	done := !p.loop && ended && live <= 0
	p.mu.Lock()
	seekOK := 0
	if p.seekOK {
		seekOK = 1
	}
	seekDelta, seekFwd, seekLanded := p.seekDeltaMs, p.seekForward, p.seekLandedMs
	concealed := p.concealed
	p.mu.Unlock()
	return Stats{Decoded: p.decoded, Shown: p.shown, Dropped: p.q.Dropped(), QueueDepth: p.q.Depth(), QueueMax: p.q.MaxDepth(), QueueAvg: p.q.DepthAvg(), DecodeMsAvg: avg, DecodeMsP95: p95, DriftMs: drift, Ended: done, Error: p.err, SeekOK: seekOK, SeekDeltaMs: seekDelta, SeekForward: seekFwd, SeekLandedMs: seekLanded, Concealed: concealed}
}

// ConcealedFault reports the first skipped sample's problem, "" when the
// clip opened clean. Window lists use Classify for the kind.
func (p *Player) ConcealedFault() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.firstFault
}

// streamFatal reports stream-level unsupported inputs that must fail the
// whole open instead of concealing one frame: F17 old tools, F2 level,
// F12 interlace and non-B/M/H profiles.
func streamFatal(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, h264.ErrSliceGroups) || errors.Is(err, h264.ErrDataPartitioning) ||
		errors.Is(err, h264.ErrRedundantPic) || errors.Is(err, h264.ErrUnsupportedNAL) ||
		errors.Is(err, h264.ErrUnsupportedLevel) ||
		errors.Is(err, h264.ErrStageScope) || errors.Is(err, h264.ErrBadAVCC) ||
		errors.Is(err, h264.ErrNoParamSets) {
		return true
	}
	return false
}

// rejectF17Units spots old-tool NALUs inside a sample payload: data
// partitions (2/3/4) and slice extension (20). They stop the open with a
// namable F17 error instead of guessing.
func rejectF17Units(units [][]byte, sampleNum int, path string) error {
	for _, u := range units {
		t, ok := h264.NALType(u)
		if !ok {
			continue
		}
		switch t {
		case h264.NALSlicePartA, h264.NALSlicePartB, h264.NALSlicePartC:
			return fmt.Errorf("video: sample %d F17 data partition %s %s: %w", sampleNum, path, h264.TypeName(t), h264.ErrDataPartitioning)
		case h264.NALSliceExt:
			return fmt.Errorf("video: sample %d F17 slice extension %s: %w", sampleNum, path, h264.ErrUnsupportedNAL)
		}
	}
	return nil
}

// SeekTo jumps to targetMs (container PTS milliseconds): lands on the last
// keyframe at or before the target, re-decodes forward to the display
// frame covering the target, then re-anchors the clock and refills the
// queue from there. The already-decoded tail shows instantly (no black);
// the fresh forward decode is verified pixel-equal against the cached
// tail, proving IDR clearing and forward decode instead of a head replay.
// Loop players are refused in VR5 (loop+seek goes to VC1); rapid seeks
// are safe (decode runs outside locks, queue swap is bounded).
func (p *Player) SeekTo(targetMs int64) (landedMs int64, err error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return 0, fmt.Errorf("%w: seek after close", ErrClosed)
	}
	if p.loop {
		p.mu.Unlock()
		return 0, fmt.Errorf("%w: seek in loop mode goes to VC1", ErrBadClip)
	}
	if len(p.frames) == 0 || len(p.samples) == 0 {
		p.mu.Unlock()
		return 0, fmt.Errorf("%w: nothing to seek", ErrNoFrames)
	}
	// Snapshot display tail + decode inputs under lock; heavy file IO
	// and decode run unlocked so Poll/Stats never block.
	frames := p.frames
	frameSamples := append([]int(nil), p.frameSamples...)
	samples := p.samples
	keyframes := append([]mp4.Keyframe(nil), p.keyframes...)
	avcc := p.avcc
	copt := p.copt
	codec := p.codec
	sampling := p.sampling
	wasPaused := p.paused
	p.mu.Unlock()

	seekIdx := 0
	for i, fr := range frames {
		if fr.PTSMs <= targetMs {
			seekIdx = i
		} else {
			break
		}
	}
	landed := frames[seekIdx].PTSMs
	delta := targetMs - landed
	if delta < 0 {
		delta = -delta
	}
	// Landing keyframe: last keyframe at or before the target (first
	// when the target precedes them all), mirroring KeyframeNear.
	key := keyframes[0]
	found := targetMs >= key.PTSMs
	if !found {
		key = keyframes[0]
	} else {
		best := keyframes[0]
		for _, k := range keyframes[1:] {
			if k.PTSMs <= targetMs {
				best = k
			} else {
				break
			}
		}
		key = best
	}
	keyPos := -1
	for i, s := range samples {
		if s.Number == key.SampleNumber {
			keyPos = i
			break
		}
	}
	if keyPos < 0 {
		return 0, fmt.Errorf("%w: keyframe sample %d lost", ErrBadClip, key.SampleNumber)
	}
	targetPos := frameSamples[seekIdx]
	if keyPos > targetPos {
		return 0, fmt.Errorf("%w: keyframe after target (key %d target %d)", ErrBadClip, keyPos, targetPos)
	}
	fresh, err := decodeForward(p.path, samples, avcc, copt, codec, sampling, keyPos, targetPos)
	if err != nil {
		return 0, err
	}
	if len(fresh) != len(frames[seekIdx].Pix) || !equalBytes(fresh, frames[seekIdx].Pix) {
		return 0, fmt.Errorf("%w: forward decode mismatch at pts %d (key %d)", ErrBadClip, landed, key.PTSMs)
	}
	forward := int64(targetPos - keyPos + 1)

	// Swap the line: drain stale queue, re-anchor now onto landed, refill
	// the tail. Cap already fits the whole clip, so Push never blocks here.
	p.q.Clear()
	p.clk.Start(landed)
	if wasPaused {
		p.clk.Pause()
	}
	refilled := 0
	for _, fr := range frames[seekIdx:] {
		if ok, _ := p.q.Push(fr); !ok {
			break
		}
		refilled++
	}
	p.mu.Lock()
	p.live = int64(refilled)
	p.ended = false
	p.hasShown = false
	p.decoded += forward
	p.seekOK = true
	p.seekDeltaMs = delta
	p.seekForward = forward
	p.seekLandedMs = landed
	p.seekTargetMs = targetMs
	p.seekKeyMs = key.PTSMs
	p.mu.Unlock()
	return landed, nil
}

// decodeForward re-decodes samples[keyPos..targetPos] with a fresh decoder
// from the same registry codec (SPS/PPS re-fed, so IDR clearing is
// exercised) and returns the target picture as RGBA. Small clips only;
// VR5 gates are ≤10 frames.
func decodeForward(path string, samples []mp4.Sample, avcc *h264.AVCC, copt color.Options, codec, sampling string, keyPos, targetPos int) ([]byte, error) {
	if avcc == nil {
		return nil, fmt.Errorf("%w: missing header params %s", ErrBadClip, path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec, err := NewDecoder(codec)
	if err != nil {
		return nil, fmt.Errorf("video: seek codec %s: %w", path, err)
	}
	for _, raw := range avcc.SPS {
		if err := dec.DecodeNALU(raw); err != nil {
			return nil, fmt.Errorf("video: seek sequence params %s: %w", path, err)
		}
	}
	for _, raw := range avcc.PPS {
		if err := dec.DecodeNALU(raw); err != nil {
			return nil, fmt.Errorf("video: seek picture params %s: %w", path, err)
		}
	}
	var target *h264.Picture
	for si := keyPos; si <= targetPos; si++ {
		s := samples[si]
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			return nil, fmt.Errorf("video: seek sample %d unreadable %s: %w", s.Number, path, err)
		}
		units, err := SplitUnits(codec, buf, avcc.LengthSize)
		if err != nil {
			return nil, fmt.Errorf("video: seek sample %d split %s: %w", s.Number, path, err)
		}
		for _, u := range units {
			if err := dec.DecodeNALU(u); err != nil {
				return nil, fmt.Errorf("video: seek sample %d decode %s: %w", s.Number, path, err)
			}
		}
		pic, err := dec.FinishPicture()
		if err != nil {
			return nil, fmt.Errorf("video: seek sample %d finish %s: %w", s.Number, path, err)
		}
		if si == targetPos {
			target = pic
		}
	}
	if target == nil {
		return nil, fmt.Errorf("%w: seek produced no picture %s", ErrBadClip, path)
	}
	cf, err := color.Convert(sampling, target.Y, target.Cb, target.Cr, int(target.Width), int(target.Height), copt)
	if err != nil {
		return nil, fmt.Errorf("video: seek color %s: %w", path, err)
	}
	return cf.Pix, nil
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

// SeekInfo reports the last seek evidence for HUD/JSON.
func (p *Player) SeekInfo() (ok bool, targetMs, landedMs, keyMs, deltaMs int64, forward int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.seekOK, p.seekTargetMs, p.seekLandedMs, p.seekKeyMs, p.seekDeltaMs, p.seekForward
}

// Close ends the background thread and waits for it.
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
}
