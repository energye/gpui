package video

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"sync/atomic"
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
	// Concealed counts bad samples skipped so far (F20 isolation):
	// truncated/corrupt frames isolated, good tail keeps playing.
	// Streaming: grows as the background discovers bad samples.
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
	// Rate is the playback speed (1 = normal); Seeking reports a seek
	// still travelling to its landing frame.
	Rate    float64
	Seeking int
}

// Options tunes the player. QueueCap <= 0 means clock.DefaultCap;
// NowMs nil means the wall clock. Loop replays from the first stamp
// (stamps keep counting up so the clock never jumps back).
type Options struct {
	QueueCap int
	NowMs    func() int64
	Loop     bool
}

// pendingPic is a decoded picture waiting for reorder: decode order in,
// PTS order out (B frames need later samples first).
type pendingPic struct {
	pic  *h264.Picture
	pts  int64
	spos int
}

// Player decodes in the background and serves frames by timestamp.
// Open with OpenFile; poll with Poll; Pause truly stops the picture;
// Seek jumps to a time (non-loop only in VR5); Close ends the thread.
// Poll is safe for one display thread; Pause, Resume, Seek and Stats
// are safe from any thread.
//
// Seeks are requests, not chores (ffplay stream_seek model): SeekTo only
// finds the landing keyframe, flushes the decoder, parks the read needle
// and re-anchors the clock, then returns in milliseconds. The background
// drops decoded frames until the landing and shows it; the caller keeps
// polling and shows the old picture meanwhile (never black). A second
// seek supersedes the first (drag-friendly); Seeking reports the gap.
// Only the tiny buffered path still decodes synchronously (deterministic
// gates, pixel-verified).
//
// Streaming (production): Open parses only headers (moov) and decodes
// just enough for the first displayable frame, then returns — seconds
// for gigabytes, not minutes. The background decodes ahead with a
// bounded queue (cap = QueueCap, default 4), so memory stays flat for
// any length and any Source (file, memory, HTTP Range). At end of stream
// the background parks with the queue open (never closes it), so a later
// seek revives playback instead of failing on a closed queue.
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
	// Seek inputs (immutable after open, except decode position).
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
	sps       *h264.SPS
	// Last seek evidence for Stats/JSON.
	seekOK       bool
	seekDeltaMs  int64
	seekForward  int64
	seekLandedMs int64
	seekTargetMs int64
	seekKeyMs    int64
	// VR6 conceal evidence: bad samples skipped so far.
	concealed  int64
	firstFault error

	// Streaming decode state (dmu guards decoder + position + reorder).
	dmu          sync.Mutex
	dec          Decoder
	source       Source
	pos          int // next sample index to decode
	pending      []*pendingPic
	reorderDepth int
	generation   int64 // atomic: Seek bumps to invalidate in-flight work
	epoch        int64 // loop PTS offset across passes
	base0        int64 // first sample PTS (epoch origin)
	spanMs       int64 // last PTS - first PTS (loop wrap step)
	nextSeq      int64
	lastPTS      int64 // last emitted PTS (monotonic guard)
	dropUntil    int64 // after seek: drop emissions with PTS <= this, -1 none
	hasFirst     bool
	// Seek request state (dmu-guarded): the background drops decoded
	// frames below seekLanded and shows the first at/above it, then
	// clears the flag. seekNeedSpos is the landing sample's decode-order
	// index: claims past it end a stale request (lost landing sample).
	seekActive  bool
	seekLanded  int64
	seekNeedSpos int
	// wakeCh wakes a decoder parked at end-of-stream (cap 1, coalescing,
	// never closed): a later seek revives playback on the same thread.
	wakeCh chan struct{}

	// Buffered mode (small clips ≤64 frames and ≤256MB estimate):
	// full decode at open into display-order cache (old semantics:
	// deterministic tests, pixel-verified seek, whole-clip loop).
	// Large clips stream above; small clips keep exact old behaviour.
	buffered    bool
	bufFrames   []*clock.Frame
	bufSamples  []int
	live        int64

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
// nothing caller-side; Close must be called. Container and codec are
// resolved through the registry: the same clip that probes also plays,
// and unknown shells/codecs fail with the supported set named.
//
// Fast open: only headers + first displayable frame(s) decode here
// (reorder delay + 1); the rest streams in the background. path may be
// a local file or an http(s) URL (Range streaming, never full download).
func OpenFile(path string, opt Options) (*Player, error) {
	src, err := NewSource(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		// Probe for a namable bucket (unsupported vs unreadable).
		if _, _, perr := ProbeFile(path); perr != nil {
			if errors.Is(perr, ErrUnsupportedContainer) || errors.Is(perr, ErrUnsupportedCodec) {
				return nil, perr
			}
		}
		return nil, err
	}
	p, err := OpenWithSource(src, opt)
	if err != nil {
		src.Close()
		// Keep the path in errors for triage (source name is the URL
		// already; file errors keep os chain for KindBadClip).
		return nil, err
	}
	return p, nil
}

// OpenWithSource opens a kept-open Source (file, memory, HTTP Range) and
// starts the background decoder. Caller must not Close src after success
// (Player owns it); on failure src is left open for the caller to close.
func OpenWithSource(src Source, opt Options) (*Player, error) {
	movie, containerName, codecName, err := openViaSource(src, src.Name())
	if err != nil {
		if errors.Is(err, ErrUnsupportedContainer) || errors.Is(err, ErrUnsupportedCodec) {
			return nil, err
		}
		if errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return nil, fmt.Errorf("video: box unreadable %s: %w", src.Name(), err)
	}
	v := movie.Video
	if v == nil || len(v.Samples) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoVideo, src.Name())
	}
	return openStream(src, src.Name(), movie, v, containerName, codecName, opt)
}

// openStream builds a streaming player from parsed headers + source.
// It decodes synchronously only until the first displayable frame, then
// hands the rest to the background.
func openStream(src Source, nameHint string, movie *mp4.Movie, v *mp4.Track, containerName, codecName string, opt Options) (*Player, error) {
	avcc, err := h264.ParseAVCC(v.AVCConfig)
	if err != nil {
		return nil, fmt.Errorf("video: header params %s: %w", nameHint, err)
	}
	dec, err := NewDecoder(codecName)
	if err != nil {
		return nil, fmt.Errorf("video: codec %s: %w", nameHint, err)
	}
	if err := feedParams(dec, avcc, nameHint, ""); err != nil {
		return nil, err
	}
	var sps *h264.SPS
	if len(avcc.SPS) > 0 {
		if sps, err = h264.ParseSPS(avcc.SPS[0]); err != nil {
			return nil, fmt.Errorf("video: sequence params %s: %w", nameHint, err)
		}
	}
	// F1/F2 gate at open: non-B/M/H profiles and levels past 5.2 fail
	// fast with a namable error instead of flowering later.
	if err := checkStreamLimits(sps, nameHint); err != nil {
		return nil, err
	}
	copt := color.Options{}
	if sps != nil && sps.VUI != nil {
		copt = color.OptionsFromVUI(sps.VUI.FullRange, sps.VUI.ColourPresent, sps.VUI.ColourMatrix)
	}
	now := opt.NowMs
	if now == nil {
		now = wallMs
	}
	// Start anchors stamp postage: the clock begins one step before the
	// first stamp, so the first frame is due on the first tick and later
	// stamps follow in real time. QueueCap 0 means DefaultCap.
	// Streaming: the cap stays bounded (shock absorber, not storage) —
	// never grown to the clip length, so gigabytes stay flat.
	qcap := opt.QueueCap
	if qcap <= 0 {
		qcap = clock.DefaultCap
	}
	q := clock.NewQueue(qcap)
	clk := clock.NewClock(now)
	// Reorder depth from the stream (B delay): VUI truth when present,
	// conservative 2 otherwise (covers single-B without stalling open).
	depth := 2
	if sps != nil && sps.VUI != nil {
		depth = int(sps.VUI.NumReorderFrames)
	}
	if depth < 0 {
		depth = 0
	}
	if depth > 16 {
		depth = 16
	}
	base0 := v.Samples[0].PTSMs
	span := v.Samples[len(v.Samples)-1].PTSMs - base0
	if span < 0 {
		span = 0
	}
	p := &Player{
		info: Info{Path: nameHint, FrameRate: v.FrameRate, DurMs: v.DurationMs, Frames: len(v.Samples), KeyframeN: v.KeyframeCount(), Container: containerName, Codec: codecName},
		q: q, clk: clk, nowMs: now,
		stopCh: make(chan struct{}), doneCh: make(chan struct{}), readyCh: make(chan struct{}),
		wakeCh: make(chan struct{}, 1),
	}
	p.loop = opt.Loop
	p.source = src
	p.path = nameHint
	p.samples = append([]mp4.Sample(nil), v.Samples...)
	p.keyframes = append([]mp4.Keyframe(nil), v.Keyframes...)
	p.avcc = avcc
	p.copt = copt
	p.frameRate = v.FrameRate
	p.container = containerName
	p.codec = codecName
	p.sps = sps
	p.sampling = dec.Sampling()
	// Small-clip fast path: full buffer (deterministic, old semantics).
	// Estimate covers YUV+RGBA per frame + workspace; tiny clips decode
	// in milliseconds, so buffering keeps every existing gate exact
	// while large clips stream bounded above.
	estW, estH := 0, 0
	if sps != nil {
		estW, estH = int(sps.Width), int(sps.Height)
	} else {
		estW, estH = int(v.Width), int(v.Height)
	}
	if len(v.Samples) <= 64 && EstimateDecoderBytes(estW, estH, len(v.Samples), 256<<10) <= 256<<20 {
		return openBuffered(p, src, nameHint, v, containerName, codecName, avcc, copt, sps, dec, opt)
	}
	p.dec = dec
	p.pos = 0
	p.reorderDepth = depth
	p.base0 = base0
	p.spanMs = span
	p.dropUntil = -1
	// Info dimensions/profile without waiting for pixels: SPS truth.
	if sps != nil {
		p.info.Width = int(sps.Width)
		p.info.Height = int(sps.Height)
		p.info.Profile = sps.Profile
	} else {
		p.info.Width = int(v.Width)
		p.info.Height = int(v.Height)
	}
	// Sync phase: decode until the first displayable frame (reorder
	// delay + 1), so Open returns in ~100ms with pixels ready and Info
	// Honest. The background continues from p.pos.
	if err := p.primeFirst(); err != nil {
		return nil, err
	}
	// Anchor the clock one step before the first emitted stamp.
	clk.Start(p.base - frameStepMs(v.FrameRate))
	go p.decodeLoop()
	return p, nil
}

// openBuffered fully decodes small clips at open into a display-order
// cache (pre-streaming semantics, byte-identical): deterministic Poll /
// Seek / Loop / Stats for every existing gate and window. Large clips
// never take this path (see openStream estimate gate).
func openBuffered(p *Player, src Source, nameHint string, v *mp4.Track, containerName, codecName string, avcc *h264.AVCC, copt color.Options, sps *h264.SPS, dec Decoder, opt Options) (*Player, error) {
	type sized struct {
		pic  *h264.Picture
		pts  int64
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
	reset := func() {
		nd, err := NewDecoder(codecName)
		if err != nil {
			return
		}
		feedParams(nd, avcc, nameHint, "")
		dec = nd
	}
	for si, s := range v.Samples {
		buf := make([]byte, s.Size)
		if _, err := readSourceRange(src, buf, int64(s.Offset)); err != nil {
			concealed++
			remember(fmt.Errorf("video: sample %d truncated %s (%v): %w", s.Number, nameHint, err, mp4.ErrTruncated))
			continue
		}
		units, err := SplitUnits(codecName, buf, avcc.LengthSize)
		if err != nil {
			concealed++
			remember(fmt.Errorf("video: sample %d split %s: %w", s.Number, nameHint, err))
			continue
		}
		if err := RejectUnits(codecName, units, s.Number, nameHint); err != nil {
			src.Close()
			return nil, err
		}
		fed := false
		var decErr error
		for _, u := range units {
			if err := dec.DecodeNALU(u); err != nil {
				decErr = fmt.Errorf("video: sample %d decode %s: %w", s.Number, nameHint, err)
				break
			}
			fed = true
		}
		if decErr != nil {
			if streamFatal(decErr) {
				src.Close()
				return nil, decErr
			}
			concealed++
			remember(decErr)
			reset()
			continue
		}
		if !fed {
			continue
		}
		pic, err := dec.FinishPicture()
		if err != nil {
			if streamFatal(err) {
				src.Close()
				return nil, fmt.Errorf("video: sample %d finish %s: %w", s.Number, nameHint, err)
			}
			concealed++
			remember(fmt.Errorf("video: sample %d finish %s: %w", s.Number, nameHint, err))
			reset()
			continue
		}
		pics = append(pics, &sized{pic: pic, pts: s.PTSMs, spos: si})
	}
	if len(pics) == 0 {
		src.Close()
		if firstFault != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrNoFrames, nameHint, firstFault)
		}
		return nil, fmt.Errorf("%w: %s", ErrNoFrames, nameHint)
	}
	sort.Slice(pics, func(i, j int) bool {
		if pics[i].pts != pics[j].pts {
			return pics[i].pts < pics[j].pts
		}
		return pics[i].pic.POC < pics[j].pic.POC
	})
	// Buffered queue holds the whole clip (tiny by gate), like before.
	if p.q.Cap() < len(pics) {
		p.q = clock.NewQueue(len(pics))
	}
	p.buffered = true
	p.dec = dec
	p.sampling = dec.Sampling()
	if firstFault != nil {
		p.info.Fault = Classify(firstFault).Readable()
	}
	p.info.Frames = len(pics)
	p.concealed = concealed
	p.info.Concealed = concealed
	p.firstFault = firstFault
	if w := pics[0].pic.Width; w > 0 {
		// Pics are cropped display size already when SPS crops.
		p.info.Width = int(pics[0].pic.Width)
		p.info.Height = int(pics[0].pic.Height)
	}
	// Convert all (timed like before for DecodeMs gates).
	for i, sp := range pics {
		t0 := time.Now()
		cf, err := color.Convert(p.sampling, sp.pic.Y, sp.pic.Cb, sp.pic.Cr, int(sp.pic.Width), int(sp.pic.Height), copt)
		if err != nil {
			src.Close()
			return nil, fmt.Errorf("video: color frame %d %s: %w", i, nameHint, err)
		}
		el := float64(time.Since(t0).Microseconds()) / 1000.0
		p.decTimes = append(p.decTimes, el)
		pts := sp.pts
		if i > 0 && pts <= p.bufFrames[i-1].PTSMs {
			pts = p.bufFrames[i-1].PTSMs + frameStepMs(v.FrameRate)
		}
		if i == 0 {
			p.base = pts
		}
		p.bufFrames = append(p.bufFrames, &clock.Frame{Width: cf.Width, Height: cf.Height, Pix: cf.Pix, PTSMs: pts, DurMs: frameStepMs(v.FrameRate), Seq: int64(i)})
		p.bufSamples = append(p.bufSamples, sp.spos)
		p.decoded++
	}
	// Source fully consumed: close now (seek re-opens via NewSource).
	src.Close()
	p.source = nil
	p.live = int64(len(p.bufFrames))
	p.clk.Start(p.bufFrames[0].PTSMs - frameStepMs(v.FrameRate))
	go p.feedCache()
	return p, nil
}

// feedCache serves the buffered cache (small-clip path): non-loop once,
// loop cycling with stamps counting up. Mirrors pre-streaming feed.
func (p *Player) feedCache() {
	defer close(p.doneCh)
	defer p.q.Close()
	announced := false
	announce := func() {
		if !announced {
			announced = true
			p.announceReady()
		}
	}
	if !p.loop {
		for _, fr := range p.bufFrames {
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
	epoch := int64(0)
	base := p.bufFrames[0].PTSMs
	for {
		for _, src := range p.bufFrames {
			cp := *src
			cp.PTSMs = base + epoch + (src.PTSMs - p.base)
			select {
			case <-p.stopCh:
				return
			default:
			}
			if ok, _ := p.q.Push(&cp); !ok {
				return
			}
			if p.q.Pushes() == 1 {
				announce()
			}
		}
		epoch += p.spanCache() + frameStepMs(p.info.FrameRate)
	}
}

func (p *Player) spanCache() int64 {
	if len(p.bufFrames) == 0 {
		return 0
	}
	return p.bufFrames[len(p.bufFrames)-1].PTSMs - p.bufFrames[0].PTSMs
}

// primeFirst decodes synchronously until the first frame queues.
// It mirrors one decodeLoop step but runs on the opener thread, so the
// first Poll never races the background.
func (p *Player) primeFirst() error {
	for {
		emitted, done, ferr := p.decodeStep()
		if ferr != nil {
			return ferr
		}
		for _, e := range emitted {
			cf, el, cerr := p.convertPic(e.pic)
			if cerr != nil {
				return fmt.Errorf("video: color frame %s: %w", p.path, cerr)
			}
			pts := p.assignPTS(e.pts)
			fr := &clock.Frame{Width: cf.Width, Height: cf.Height, Pix: cf.Pix, PTSMs: pts, DurMs: frameStepMs(p.frameRate), Seq: p.nextSeq}
			p.nextSeq++
			p.mu.Lock()
			p.decTimes = append(p.decTimes, el)
			p.decoded++
			p.mu.Unlock()
			if !p.hasFirst {
				p.hasFirst = true
				p.base = pts
				// Refine Info dimensions from real pixels (crop truth).
				p.info.Width = cf.Width
				p.info.Height = cf.Height
			}
			p.lastPTS = pts
			if ok, _ := p.q.Push(fr); !ok {
				return fmt.Errorf("%w: queue closed during open %s", ErrClosed, p.path)
			}
			p.announceReady()
		}
		if p.hasFirst {
			return nil
		}
		if done {
			if !p.hasFirst {
				p.mu.Lock()
				ff := p.firstFault
				p.mu.Unlock()
				if ff != nil {
					return fmt.Errorf("%w: %s: %v", ErrNoFrames, p.path, ff)
				}
				return fmt.Errorf("%w: %s", ErrNoFrames, p.path)
			}
			return nil
		}
	}
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

// decodeStep decodes one sample (claim + IO + decode under dmu) and
// returns display-ready emissions (reorder). done reports the stream
// exhausted AND pending flushed. Fatal stream errors return err.
func (p *Player) decodeStep() (emitted []*pendingPic, done bool, err error) {
	// Claim position under dmu.
	p.dmu.Lock()
	if p.pos >= len(p.samples) {
		if len(p.pending) == 0 {
			p.dmu.Unlock()
			return nil, true, nil
		}
		// Flush all sorted at EOS (seek filter in decodeLoop drops
		// anything below a travelling landing; the first at/above it
		// ends the seek).
		sort.Slice(p.pending, func(i, j int) bool {
			if p.pending[i].pts != p.pending[j].pts {
				return p.pending[i].pts < p.pending[j].pts
			}
			return p.pending[i].spos < p.pending[j].spos
		})
		out := p.pending
		p.pending = nil
		p.dmu.Unlock()
		return out, true, nil
	}
	idx := p.pos
	p.pos++
	s := p.samples[idx]
	gen := atomic.LoadInt64(&p.generation)
	p.dmu.Unlock()

	// IO outside the decoder lock (network may stall; Seek must proceed).
	buf := make([]byte, s.Size)
	if _, rerr := readSourceRange(p.source, buf, int64(s.Offset)); rerr != nil {
		p.mu.Lock()
		p.concealed++
		if p.firstFault == nil {
			p.firstFault = fmt.Errorf("video: sample %d truncated %s (%v): %w", s.Number, p.path, rerr, mp4.ErrTruncated)
		}
		p.info.Concealed = p.concealed
		if p.firstFault != nil {
			p.info.Fault = Classify(p.firstFault).Readable()
		}
		p.mu.Unlock()
		return nil, false, nil
	}
	p.dmu.Lock()
	defer p.dmu.Unlock()
	if atomic.LoadInt64(&p.generation) != gen {
		// Seek invalidated this claim; drop the read.
		return nil, false, nil
	}
	fed := false
	units, uerr := SplitUnits(p.codec, buf, p.avcc.LengthSize)
	if uerr != nil {
		p.dmu.Unlock()
		p.mu.Lock()
		p.concealed++
		if p.firstFault == nil {
			p.firstFault = fmt.Errorf("video: sample %d split %s: %w", s.Number, p.path, uerr)
		}
		p.info.Concealed = p.concealed
		p.info.Fault = Classify(p.firstFault).Readable()
		p.mu.Unlock()
		p.dmu.Lock()
		return nil, false, nil
	}
	if rerr := RejectUnits(p.codec, units, s.Number, p.path); rerr != nil {
		// F17 is stream-level: fail the open/play loudly (no conceal).
		// dmu is held; decodeStep's defer unlocks on this return.
		return nil, false, rerr
	}
	var decErr error
	for _, u := range units {
		if derr := p.dec.DecodeNALU(u); derr != nil {
			decErr = fmt.Errorf("video: sample %d decode %s: %w", s.Number, p.path, derr)
			break
		}
		fed = true
	}
	if decErr != nil {
		if streamFatal(decErr) {
			return nil, false, decErr
		}
		p.dmu.Unlock()
		p.mu.Lock()
		p.concealed++
		if p.firstFault == nil {
			p.firstFault = decErr
		}
		p.info.Concealed = p.concealed
		p.info.Fault = Classify(p.firstFault).Readable()
		p.mu.Unlock()
		p.dmu.Lock()
		p.resetDecoderLocked()
		return nil, false, nil
	}
	if !fed {
		return nil, false, nil
	}
	pic, ferr := p.dec.FinishPicture()
	if ferr != nil {
		if streamFatal(ferr) {
			return nil, false, fmt.Errorf("video: sample %d finish %s: %w", s.Number, p.path, ferr)
		}
		p.dmu.Unlock()
		p.mu.Lock()
		p.concealed++
		if p.firstFault == nil {
			p.firstFault = fmt.Errorf("video: sample %d finish %s: %w", s.Number, p.path, ferr)
		}
		p.info.Concealed = p.concealed
		p.info.Fault = Classify(p.firstFault).Readable()
		p.mu.Unlock()
		p.dmu.Lock()
		p.resetDecoderLocked()
		return nil, false, nil
	}
	// Display PTS = epoch-shifted container PTS (loops count up).
	pts := p.base0 + p.epoch + (s.PTSMs - p.base0)
	p.pending = append(p.pending, &pendingPic{pic: pic, pts: pts, spos: idx})
	if len(p.pending) <= p.reorderDepth {
		return nil, false, nil
	}
	// Emit the smallest PTS.
	best := 0
	for i := 1; i < len(p.pending); i++ {
		if p.pending[i].pts < p.pending[best].pts ||
			(p.pending[i].pts == p.pending[best].pts && p.pending[i].spos < p.pending[best].spos) {
			best = i
		}
	}
	out := p.pending[best]
	p.pending = append(p.pending[:best], p.pending[best+1:]...)
	return []*pendingPic{out}, false, nil
}

// resetDecoderLocked rebuilds a clean decoder (caller holds dmu).
func (p *Player) resetDecoderLocked() {
	nd, err := NewDecoder(p.codec)
	if err != nil {
		return
	}
	feedParams(nd, p.avcc, p.path, "")
	p.dec = nd
}

// convertPic runs color conversion (stateless, no locks held).
func (p *Player) convertPic(pic *h264.Picture) (*color.Frame, float64, error) {
	t0 := time.Now()
	cf, err := color.Convert(p.sampling, pic.Y, pic.Cb, pic.Cr, int(pic.Width), int(pic.Height), p.copt)
	if err != nil {
		return nil, 0, err
	}
	el := float64(time.Since(t0).Microseconds()) / 1000.0
	return cf, el, nil
}

// assignPTS guards monotonic display stamps (caller sequences emissions).
func (p *Player) assignPTS(pts int64) int64 {
	if p.hasFirst && pts <= p.lastPTS {
		pts = p.lastPTS + frameStepMs(p.frameRate)
	}
	return pts
}

// decodeLoop serves frames on the decoder thread with backpressure:
// Push waits while the queue is full, so a tiny cap never piles memory.
// Non-loop ends after flushing; loop re-decodes from head with stamps
// counting up. Steady loop allocates one Frame per push only.
func (p *Player) decodeLoop() {
	defer close(p.doneCh)
	defer p.q.Close()
	for {
		select {
		case <-p.stopCh:
			return
		default:
		}
		gen := atomic.LoadInt64(&p.generation)
		emitted, done, ferr := p.decodeStep()
		if ferr != nil {
			p.mu.Lock()
			if p.err == "" {
				p.err = ferr.Error()
			}
			p.decodeDone = true
			p.mu.Unlock()
			return
		}
		for _, e := range emitted {
			// Seek filter: while a seek is pending, decoded frames below
			// the landing are forward-discard (dropped before convert, so
			// no wasted color work); the first at/above it ends the seek
			// and shows with its exact stamp. Older jumped-over lines use
			// dropUntil the same way.
			landing := false
			p.dmu.Lock()
			if p.seekActive {
				if e.pts < p.seekLanded {
					p.dmu.Unlock()
					continue
				}
				p.seekActive = false
				landing = true
			} else if p.dropUntil >= 0 && e.pts <= p.dropUntil {
				p.dmu.Unlock()
				continue
			}
			p.dmu.Unlock()
			if atomic.LoadInt64(&p.generation) != gen {
				break
			}
			cf, el, cerr := p.convertPic(e.pic)
			if cerr != nil {
				p.mu.Lock()
				if p.err == "" {
					p.err = fmt.Errorf("video: color %s: %w", p.path, cerr).Error()
				}
				p.mu.Unlock()
				continue
			}
			if atomic.LoadInt64(&p.generation) != gen {
				break
			}
			pts := e.pts
			p.dmu.Lock()
			if !landing && p.hasFirst && pts <= p.lastPTS {
				pts = p.lastPTS + frameStepMs(p.frameRate)
			}
			seq := p.nextSeq
			p.nextSeq++
			p.lastPTS = pts
			p.dmu.Unlock()
			fr := &clock.Frame{Width: cf.Width, Height: cf.Height, Pix: cf.Pix, PTSMs: pts, DurMs: frameStepMs(p.frameRate), Seq: seq}
			if ok, _ := p.q.Push(fr); !ok {
				return
			}
			if atomic.LoadInt64(&p.generation) != gen {
				// A seek landed while this frame waited in Push (or just
				// after it): drop the stale line so it never shows ahead
				// of the new landing. Single producer, so clearing here
				// only removes our own stale push.
				p.q.Clear()
				continue
			}
			p.mu.Lock()
			p.decTimes = append(p.decTimes, el)
			p.decoded++
			p.mu.Unlock()
			p.announceReady()
		}
		if done {
			p.mu.Lock()
			loop := p.loop
			p.mu.Unlock()
			if !loop {
				// Park at end of stream with the queue OPEN (never close
				// it here): a later seek revives this same thread via
				// wakeCh instead of failing on a closed queue. A seek
				// whose landing never decoded (lost tail sample) ends
				// here so Seeking never hangs.
				p.dmu.Lock()
				p.seekActive = false
				p.dmu.Unlock()
				p.mu.Lock()
				p.decodeDone = true
				p.mu.Unlock()
				select {
				case <-p.stopCh:
					return
				case <-p.wakeCh:
					continue
				}
			}
			// Loop wrap: re-decode from head, stamps keep counting up.
			p.dmu.Lock()
			p.epoch += p.spanMs + frameStepMs(p.frameRate)
			p.pos = 0
			p.pending = nil
			p.nextSeq = 0
			p.dropUntil = -1
			p.resetDecoderLocked()
			p.dmu.Unlock()
		}
	}
}

// Poll returns the newest due frame (nil when none is due, or after the
// stream ended and drained). Ended reports the stream played through;
// callers keep polling until Ended plus nil. The first call waits for
// the background thread to queue the head frame (bounded wait) — unless a
// seek is already travelling, in which case it returns at once so the
// caller never stalls 5s on a stale wait.
// Steady polls cost no timer: readyCh stays closed after the head, so the
// fast path below skips time.After entirely (VR7 profile: one timer per
// Poll dominated steady bytes before this fix).
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
	due := p.clk.DuePTSMS()
	fr, skipped, ok := p.q.PollDue(due)
	if ok {
		p.mu.Lock()
		p.shown++
		p.lastShown = fr.PTSMs
		p.hasShown = true
		last := false
		if !p.loop {
			if p.buffered {
				// Buffered: count every consumed frame off live (old
				// semantics: catch-up eats stale but counts all).
				p.live -= int64(skipped) + 1
				if p.live <= 0 {
					p.live = 0
					p.ended = true
					last = true
				}
			} else {
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
		}
		p.mu.Unlock()
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
	if p.buffered {
		if p.live <= 0 && p.q.Drained() {
			p.ended = true
			return nil, true
		}
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
func (p *Player) Buffered() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.buffered
}

// DecodePos reports samples consumed at open (tests only: streaming
// fast-open proof, buffered reports len(samples)).
func (p *Player) DecodePos() int {
	if p == nil {
		return 0
	}
	p.dmu.Lock()
	defer p.dmu.Unlock()
	if p.buffered {
		return len(p.samples)
	}
	return p.pos
}

// ReorderDepth reports the B-delay used (tests only).
func (p *Player) ReorderDepth() int {
	if p == nil {
		return 0
	}
	p.dmu.Lock()
	defer p.dmu.Unlock()
	return p.reorderDepth
}

// Info describes the clip (Concealed/Fault reflect streaming progress).
func (p *Player) Info() Info {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := p.info
	out.Concealed = p.concealed
	if p.firstFault != nil {
		out.Fault = Classify(p.firstFault).Readable()
	} else {
		out.Fault = ""
	}
	return out
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
	concealed := p.concealed
	errStr := p.err
	p.mu.Unlock()
	rate := p.Rate()
	seeking := 0
	if p.Seeking() {
		seeking = 1
	}
	return Stats{Decoded: p.decoded, Shown: p.shown, Dropped: p.q.Dropped(), QueueDepth: p.q.Depth(), QueueMax: p.q.MaxDepth(), QueueAvg: p.q.DepthAvg(), DecodeMsAvg: avg, DecodeMsP95: p95, DriftMs: drift, Ended: done, Error: errStr, SeekOK: seekOK, SeekDeltaMs: seekDelta, SeekForward: seekFwd, SeekLandedMs: seekLanded, Concealed: concealed, Rate: rate, Seeking: seeking}
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

// feedParams feeds the header sets (SPS then PPS) into a fresh decoder.
// Open, error isolation and seek forward share it, so IDR clearing is
// exercised identically on every path. tag names the caller in errors
// ("seek " for forward decode, "" at open); messages stay byte-identical
// to the pre-registry wording.
func feedParams(dec Decoder, avcc *h264.AVCC, path, tag string) error {
	for _, raw := range avcc.SPS {
		if err := dec.DecodeNALU(raw); err != nil {
			return fmt.Errorf("video: %ssequence params %s: %w", tag, path, err)
		}
	}
	for _, raw := range avcc.PPS {
		if err := dec.DecodeNALU(raw); err != nil {
			return fmt.Errorf("video: %spicture params %s: %w", tag, path, err)
		}
	}
	return nil
}

// SeekTo jumps to targetMs (container PTS milliseconds) and returns the
// covering frame's stamp at once — but decodes nothing itself (ffplay
// stream_seek model). It lands on the last sample at or before the target,
// flushes the decoder, parks the read needle on the landing keyframe,
// re-anchors the clock and lets the background drop forward frames until
// the landing shows. The caller keeps polling; the old picture holds
// meanwhile (never black). A second seek supersedes the first, so dragging
// the progress bar stays responsive. Buffered clips still verify
// pixel-exact against the cache and show instantly (deterministic gates).
// Loop players are refused in VR5 (loop+seek goes to VC1).
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
	if len(p.samples) == 0 {
		return 0, fmt.Errorf("%w: nothing to seek", ErrNoFrames)
	}
	if p.buffered {
		return p.seekBuffered(targetMs)
	}
	seekSpos, landed, delta, key, keyPos, err := p.seekPlan(targetMs)
	if err != nil {
		return 0, err
	}
	// The forward decode runs on the background thread (see decodeLoop's
	// seek filter); this call only reparks the needle and returns.
	return p.seekStream(keyPos, seekSpos, landed, delta, targetMs, key.PTSMs)
}

// seekPlan scans the immutable sample/keyframe tables for targetMs: the
// covering sample's decode-order index, its stamp, the budget delta, the
// landing keyframe and its decode-order index. Pure table math, no locks,
// no IO — safe on any thread.
func (p *Player) seekPlan(targetMs int64) (seekSpos int, landed, delta int64, key mp4.Keyframe, keyPos int, err error) {
	samples := p.samples
	keyframes := p.keyframes
	if len(keyframes) == 0 {
		return 0, 0, 0, mp4.Keyframe{}, 0, fmt.Errorf("%w: no keyframes", ErrBadClip)
	}
	// Display-order landing: last sample with PTS <= target (floor
	// covering), mirroring the old cached-frames behaviour. Samples are
	// in decode order, so scan for max PTS <= target.
	seekSpos = -1
	var landedPTS int64
	for i, s := range samples {
		if s.PTSMs <= targetMs && (seekSpos < 0 || s.PTSMs > landedPTS) {
			seekSpos = i
			landedPTS = s.PTSMs
		}
	}
	if seekSpos < 0 {
		// Target precedes all stamps: land on the earliest display frame.
		best := 0
		for i := 1; i < len(samples); i++ {
			if samples[i].PTSMs < samples[best].PTSMs {
				best = i
			}
		}
		seekSpos = best
		landedPTS = samples[best].PTSMs
	}
	landed = landedPTS
	delta = targetMs - landed
	if delta < 0 {
		delta = -delta
	}
	// Landing keyframe: last keyframe at or before the target (first
	// when the target precedes them all), mirroring KeyframeNear.
	key = keyframes[0]
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
	keyPos = -1
	for i, s := range samples {
		if s.Number == key.SampleNumber {
			keyPos = i
			break
		}
	}
	if keyPos < 0 {
		return 0, 0, 0, mp4.Keyframe{}, 0, fmt.Errorf("%w: keyframe sample %d lost", ErrBadClip, key.SampleNumber)
	}
	if keyPos > seekSpos {
		// Keyframe after target in decode order (B reorder): the target
		// display frame needs a ref decoded later — fall back to landing
		// on the keyframe's own display stamp via its sample.
		seekSpos = keyPos
		landed = samples[keyPos].PTSMs
		delta = targetMs - landed
		if delta < 0 {
			delta = -delta
		}
	}
	return seekSpos, landed, delta, key, keyPos, nil
}

// seekStream reparks a streaming seek: flush the decoder, park the read
// needle on the landing keyframe, arm the background drop-until-landing
// filter, re-anchor the clock and wake a decoder parked at end-of-stream.
// No frame is decoded here, so even huge GOPs return in milliseconds.
// Ordering matters: repark under dmu first, then bump the generation
// (in-flight claims on the old needle go stale), then clear the queue
// (wakes a Push blocked on full so it can notice the new generation).
func (p *Player) seekStream(keyPos, seekSpos int, landed, delta, targetMs, keyMs int64) (int64, error) {
	wasPaused := p.Paused()
	p.dmu.Lock()
	p.resetDecoderLocked()
	p.pending = nil
	p.pos = keyPos
	p.seekActive = true
	p.seekLanded = landed
	p.seekNeedSpos = seekSpos
	p.dropUntil = -1
	p.dmu.Unlock()
	atomic.AddInt64(&p.generation, 1)
	p.q.Clear()
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
	p.seekForward = int64(seekSpos - keyPos + 1)
	p.seekLandedMs = landed
	p.seekTargetMs = targetMs
	p.seekKeyMs = keyMs
	p.mu.Unlock()
	// Wake a decoder parked at end-of-stream (coalescing send: a stale
	// wake only causes one harmless extra end-check).
	select {
	case p.wakeCh <- struct{}{}:
	default:
	}
	p.announceReady()
	return landed, nil
}

// seekAllowed rejects seeks that can never work: closed player, loop mode
// (loop+seek goes to VC1), a dead stream, or an empty table.
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
	case len(p.samples) == 0:
		return fmt.Errorf("%w: nothing to seek", ErrNoFrames)
	}
	return nil
}

// SeekFast jumps to the keyframe at or before targetMs without forward
// discard: instant even across huge GOPs, at keyframe granularity. Use it
// while dragging the progress bar, then SeekTo once on release for the
// exact frame. Buffered clips land the same keyframe from the cache.
func (p *Player) SeekFast(targetMs int64) (int64, error) {
	if err := p.seekAllowed(); err != nil {
		return 0, err
	}
	if p.buffered {
		_, _, _, key, _, err := p.seekPlan(targetMs)
		if err != nil {
			return 0, err
		}
		return p.seekBuffered(key.PTSMs)
	}
	_, _, _, key, keyPos, err := p.seekPlan(targetMs)
	if err != nil {
		return 0, err
	}
	delta := targetMs - key.PTSMs
	if delta < 0 {
		delta = -delta
	}
	return p.seekStream(keyPos, keyPos, key.PTSMs, delta, targetMs, key.PTSMs)
}

// SeekBy jumps relative to the current position (negative rewinds):
// the standard long-press / J-L behaviour. True reverse decode does not
// exist (no mature player reverse-decodes H.264); rewind is a backward
// jump, same as every reference player.
func (p *Player) SeekBy(deltaMs int64) (int64, error) {
	if err := p.seekAllowed(); err != nil {
		return 0, err
	}
	return p.SeekTo(p.PositionMs() + deltaMs)
}

// NextKeyframe lands the next keyframe after the current position (clamped
// to the last one); PrevKeyframe lands the keyframe strictly before it
// (clamped to the first one), so repeated steps walk the whole table.
// Keyframe landings need no forward discard (forward = 1).
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
	if len(p.keyframes) == 0 {
		return 0, fmt.Errorf("%w: no keyframes", ErrBadClip)
	}
	ref := p.PositionMs()
	p.mu.Lock()
	if !p.hasShown && p.seekOK {
		ref = p.seekLandedMs
	}
	p.mu.Unlock()
	var key mp4.Keyframe
	found := false
	if next {
		for _, k := range p.keyframes {
			if k.PTSMs > ref && (!found || k.PTSMs < key.PTSMs) {
				key, found = k, true
			}
		}
		if !found {
			key = p.keyframes[len(p.keyframes)-1]
		}
	} else {
		for _, k := range p.keyframes {
			if k.PTSMs < ref && (!found || k.PTSMs > key.PTSMs) {
				key, found = k, true
			}
		}
		if !found {
			key = p.keyframes[0]
		}
	}
	keyPos := -1
	for i, s := range p.samples {
		if s.Number == key.SampleNumber {
			keyPos = i
			break
		}
	}
	if keyPos < 0 {
		return 0, fmt.Errorf("%w: keyframe sample %d lost", ErrBadClip, key.SampleNumber)
	}
	if p.buffered {
		return p.seekBuffered(key.PTSMs)
	}
	return p.seekStream(keyPos, keyPos, key.PTSMs, 0, key.PTSMs, key.PTSMs)
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
// surplus at high rates — the same speed-scaled clock every reference
// player uses. Only the picture clock scales (this stage is video-only,
// no audio to keep in sync). Range is 0 < rate <= 8.
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
// before the first frame (the controller `position` getter).
func (p *Player) PositionMs() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.hasShown {
		return p.lastShown
	}
	return 0
}

// Seeking reports a seek is still travelling: SeekTo returned the landing
// stamp but the background has not shown it yet. Buffered seeks complete
// synchronously and never report true.
func (p *Player) Seeking() bool {
	if p == nil || p.buffered {
		return false
	}
	p.dmu.Lock()
	defer p.dmu.Unlock()
	return p.seekActive
}

// seekBuffered is the buffered-clip seek (pre-streaming semantics,
// byte-identical): landing via display-order cache, forward count as
// sample span, verification against the cached tail.
func (p *Player) seekBuffered(targetMs int64) (int64, error) {
	frames := p.bufFrames
	frameSamples := append([]int(nil), p.bufSamples...)
	samples := p.samples
	keyframes := append([]mp4.Keyframe(nil), p.keyframes...)
	avcc := p.avcc
	copt := p.copt
	codec := p.codec
	sampling := p.sampling
	wasPaused := p.Paused()

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

	p.q.Clear()
	p.clk.Start(landed)
	if wasPaused {
		p.clk.Pause()
	}
	refilled := 0
	cap := p.q.Cap()
	for _, fr := range frames[seekIdx:] {
		if refilled >= cap {
			break
		}
		if ok, _ := p.q.Push(fr); !ok {
			break
		}
		refilled++
	}
	// Buffered queue keeps the whole tail regardless of cap: push the
	// rest (cap fits the clip by construction, so this never blocks).
	for _, fr := range frames[seekIdx+refilled:] {
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
// exercised) and returns the target picture as RGBA. Small seeks only in
// tests; production seeks bound this by GOP size.
func decodeForward(path string, samples []mp4.Sample, avcc *h264.AVCC, copt color.Options, codec, sampling string, keyPos, targetPos int) ([]byte, error) {
	src, err := NewSource(path)
	if err != nil {
		return nil, err
	}
	defer src.Close()
	return decodeForwardSource(src, samples, avcc, copt, codec, sampling, keyPos, targetPos)
}

// decodeForwardSource is the Source twin (network-safe, no re-open).
func decodeForwardSource(src Source, samples []mp4.Sample, avcc *h264.AVCC, copt color.Options, codec, sampling string, keyPos, targetPos int) ([]byte, error) {
	if avcc == nil {
		return nil, fmt.Errorf("%w: missing header params %s", ErrBadClip, src.Name())
	}
	dec, err := NewDecoder(codec)
	if err != nil {
		return nil, fmt.Errorf("video: seek codec %s: %w", src.Name(), err)
	}
	if err := feedParams(dec, avcc, src.Name(), "seek "); err != nil {
		return nil, err
	}
	var target *h264.Picture
	for si := keyPos; si <= targetPos; si++ {
		s := samples[si]
		buf := make([]byte, s.Size)
		if _, err := readSourceRange(src, buf, int64(s.Offset)); err != nil {
			return nil, fmt.Errorf("video: seek sample %d unreadable %s: %w", s.Number, src.Name(), err)
		}
		units, err := SplitUnits(codec, buf, avcc.LengthSize)
		if err != nil {
			return nil, fmt.Errorf("video: seek sample %d split %s: %w", s.Number, src.Name(), err)
		}
		for _, u := range units {
			if err := dec.DecodeNALU(u); err != nil {
				return nil, fmt.Errorf("video: seek sample %d decode %s: %w", s.Number, src.Name(), err)
			}
		}
		pic, err := dec.FinishPicture()
		if err != nil {
			return nil, fmt.Errorf("video: seek sample %d finish %s: %w", s.Number, src.Name(), err)
		}
		if si == targetPos {
			target = pic
		}
	}
	if target == nil {
		return nil, fmt.Errorf("%w: seek produced no picture %s", ErrBadClip, src.Name())
	}
	cf, err := color.Convert(sampling, target.Y, target.Cb, target.Cr, int(target.Width), int(target.Height), copt)
	if err != nil {
		return nil, fmt.Errorf("video: seek color %s: %w", src.Name(), err)
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
	if p.source != nil {
		p.source.Close()
	}
}
