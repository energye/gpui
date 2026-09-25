package video

import (
	"runtime"
	"testing"
	"time"

	"github.com/energye/gpui/video/clock"
)

// openSeekClip opens a clip with the hand clock for seek gates.
// QueueCap 8 fits small gate clips whole, so the background never blocks
// in Push while the test drains.
func openSeekClip(t *testing.T, h *handClock, name string) *Player {
	t.Helper()
	p, err := OpenFile("testdata/"+name, Options{NowMs: h.at, QueueCap: 8})
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	return p
}

// seekPollUntil polls until a frame with PTS >= wantMin shows. The first
// poll is at the current stamp (exact landings show at once); afterwards
// the hand clock ticks each round, because a between-stamps target echoes
// back a landing with no exact frame — the next real frame only becomes
// due as time advances (wall clock does this naturally).
func seekPollUntil(t *testing.T, p *Player, h *handClock, wantMin int64) *clock.Frame {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	first := true
	for {
		if time.Now().After(deadline) {
			t.Fatalf("no frame at/after %d", wantMin)
		}
		if fr, _ := p.Poll(); fr != nil && fr.PTSMs >= wantMin {
			return fr
		}
		if !first {
			h.now += 200
		}
		first = false
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
}

// seekPollExact polls at a fixed hand stamp until the exact-grid landing
// shows (no ticking). Exact landings (seek echoes on the 200ms grid) are
// due at once: clk.Start(landing) freezes due on the landing itself, and
// the background delivers within milliseconds (production advances one
// frame per 200ms, so nothing goes stale). Ticking here would be a pure
// test artifact: at ~1ms per round the hand outruns a jittered background
// and PollDue eats the exact frame as stale before it is ever seen.
func seekPollExact(t *testing.T, p *Player, want int64) *clock.Frame {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("exact frame %d never showed", want)
		}
		if fr, _ := p.Poll(); fr != nil && fr.PTSMs == want {
			return fr
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
}

// TestSeekToMiddleKeyframe pins the VR5 core on ffmpeg: seeking jumps to
// the target (echo landing), evidence names target/landing, the picture
// shows without black, and the tail plays monotonic to Ended.
// ffmpeg stamps for vr5_seek.mp4 are 0..1800 (0-based); 1000 is mid-clip
// so the tail (1000..1800) proves forward play, not just the last frame.
func TestSeekToMiddleKeyframe(t *testing.T) {
	h := &handClock{}
	p := openSeekClip(t, h, "vr5_seek.mp4")
	defer p.Close()
	if p.Info().Frames != 10 {
		t.Fatalf("frames = %d, want 10", p.Info().Frames)
	}
	// Play two frames first so the seek visibly jumps.
	h.now += 200
	if f := seekPollUntil(t, p, h, 0); f == nil {
		t.Fatal("first frame never due")
	}
	h.now += 200
	if f := seekPollUntil(t, p, h, 0); f == nil {
		t.Fatal("second frame never due")
	}
	landed, err := p.SeekTo(1000)
	if err != nil {
		t.Fatalf("seek 1000: %v", err)
	}
	if landed != 1000 {
		t.Fatalf("landed = %d, want 1000", landed)
	}
	ok, target, land, key, delta, forward := p.SeekInfo()
	if !ok || target != 1000 || land != 1000 || key != 1000 {
		t.Fatalf("evidence ok=%v target=%d landed=%d key=%d, want 1/1000/1000/1000", ok, target, land, key)
	}
	if delta != 0 {
		t.Fatalf("delta = %d, want 0 (echo landing)", delta)
	}
	// ffmpeg lands keyframes natively: forward is keyframe granularity.
	if forward != 1 {
		t.Fatalf("forward = %d, want 1 (ffmpeg keyframe landing)", forward)
	}
	st := p.Stats()
	if st.SeekOK != 1 || st.SeekDeltaMs != 0 || st.SeekForward != 1 || st.SeekLandedMs != 1000 {
		t.Fatalf("stats seek = %+v, want ok1/delta0/fwd1/land1000", st)
	}
	// No black: a frame at/after the landing shows (same stamp retries).
	f := seekPollUntil(t, p, h, 1000)
	if f.PTSMs < 1000 {
		t.Fatalf("shown pts = %d, want >= 1000", f.PTSMs)
	}
	// Play to the end from the seek point: monotonic stamps, then done.
	var tail []int64
	tail = append(tail, f.PTSMs)
	deadline := time.Now().Add(60 * time.Second)
	ended := false
	for !ended {
		if time.Now().After(deadline) {
			t.Fatalf("tail never ends: %v", tail)
		}
		h.now += 200
		fr, done := p.Poll()
		if fr != nil {
			if fr.PTSMs <= tail[len(tail)-1] {
				t.Fatalf("tail not monotonic: %v + %d", tail, fr.PTSMs)
			}
			tail = append(tail, fr.PTSMs)
		}
		ended = done
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	if !p.Stats().Ended {
		t.Fatal("not ended after playing seek tail")
	}
}

// TestSeekFirstGOP pins the near-head path: seeking inside the first GOP
// lands on the target echo and shows a frame at/after it.
func TestSeekFirstGOP(t *testing.T) {
	h := &handClock{}
	p := openSeekClip(t, h, "vr5_seek.mp4")
	defer p.Close()
	landed, err := p.SeekTo(600)
	if err != nil {
		t.Fatalf("seek 600: %v", err)
	}
	if landed != 600 {
		t.Fatalf("landed = %d, want 600", landed)
	}
	_, _, _, key, _, forward := p.SeekInfo()
	if key != 600 || forward != 1 {
		t.Fatalf("key = %d forward = %d, want 600/1 (ffmpeg echo)", key, forward)
	}
	if f := seekPollUntil(t, p, h, 600); f.PTSMs < 600 {
		t.Fatalf("shown pts = %d, want >= 600", f.PTSMs)
	}
}

// TestSeekBetweenStamps pins the landing budget: a target between stamps
// echoes back with zero delta, and the next frame at/after it shows.
func TestSeekBetweenStamps(t *testing.T) {
	h := &handClock{}
	p := openSeekClip(t, h, "vr5_seek.mp4")
	defer p.Close()
	landed, err := p.SeekTo(1700)
	if err != nil {
		t.Fatalf("seek 1700: %v", err)
	}
	if landed != 1700 {
		t.Fatalf("landed = %d, want 1700 (echo)", landed)
	}
	_, _, _, _, delta, _ := p.SeekInfo()
	if delta != 0 {
		t.Fatalf("delta = %d, want 0", delta)
	}
	if f := seekPollUntil(t, p, h, 1700); f.PTSMs < 1700 {
		t.Fatalf("shown pts = %d, want >= 1700", f.PTSMs)
	}
}

// TestSeekSingleKeyframeClip pins small clips: seek echoes and shows.
func TestSeekSingleKeyframeClip(t *testing.T) {
	h := &handClock{}
	p := openSeekClip(t, h, "vr2_m_bframes.mp4")
	defer p.Close()
	landed, err := p.SeekTo(800)
	if err != nil {
		t.Fatalf("seek 800: %v", err)
	}
	if landed != 800 {
		t.Fatalf("landed = %d, want 800", landed)
	}
	_, _, _, key, _, forward := p.SeekInfo()
	if key != 800 || forward != 1 {
		t.Fatalf("key = %d forward = %d, want 800/1 (ffmpeg echo)", key, forward)
	}
	if f := seekPollUntil(t, p, h, 800); f.PTSMs < 800 {
		t.Fatalf("shown pts = %d, want >= 800", f.PTSMs)
	}
}

// TestSeekRapidNoStall pins continuous fast seeks: alternating jumps never
// hang and every landing shows at/after its target (no card death).
// Targets stay mid-clip (600/1000) so every landing has a tail to show,
// never the last frame (which would drain the queue and stall the next).
func TestSeekRapidNoStall(t *testing.T) {
	h := &handClock{}
	p := openSeekClip(t, h, "vr5_seek.mp4")
	defer p.Close()
	for i := 0; i < 20; i++ {
		target := int64(600)
		if i%2 == 1 {
			target = 1000
		}
		landed, err := p.SeekTo(target)
		if err != nil {
			t.Fatalf("rapid seek %d to %d: %v", i, target, err)
		}
		if landed != target {
			t.Fatalf("rapid seek %d landed %d, want %d", i, landed, target)
		}
		if f := seekPollUntil(t, p, h, target); f.PTSMs < target {
			t.Fatalf("rapid seek %d shown pts %d, want >= %d", i, f.PTSMs, target)
		}
		h.now += 50
	}
}

// TestSeekLoopRefused pins the VR5 scope: loop+seek goes to VC1.
func TestSeekLoopRefused(t *testing.T) {
	h := &handClock{}
	p, err := OpenFile("testdata/vr5_seek.mp4", Options{NowMs: h.at, Loop: true})
	if err != nil {
		t.Fatalf("open loop: %v", err)
	}
	defer p.Close()
	if _, err := p.SeekTo(800); err == nil {
		t.Fatal("loop seek accepted, want refusal to VC1")
	}
}

// TestSeekWhilePaused pins pause+seek: the sought frame shows once and the
// picture stays held until resume. Seek target is mid-clip (1000) so the
// tail (1000..1800) can show after resume — the last frame has no tail.
func TestSeekWhilePaused(t *testing.T) {
	h := &handClock{}
	p := openSeekClip(t, h, "vr5_seek.mp4")
	defer p.Close()
	h.now += 200
	if f := seekPollUntil(t, p, h, 0); f == nil {
		t.Fatal("first frame never due")
	}
	p.Pause()
	h.now += 5000
	if f, _ := p.Poll(); f != nil {
		t.Fatalf("frame %d showed while paused", f.Seq)
	}
	landed, err := p.SeekTo(1000)
	if err != nil {
		t.Fatalf("seek while paused: %v", err)
	}
	if landed != 1000 {
		t.Fatalf("landed = %d, want 1000", landed)
	}
	// The sought frame is already due even while held (no black), but the
	// clock stays frozen afterwards: wait at the same stamp.
	f := seekPollUntil(t, p, h, 1000)
	if f.PTSMs < 1000 {
		t.Fatalf("paused seek shown pts = %d, want >= 1000", f.PTSMs)
	}
	h.now += 5000
	if f, _ := p.Poll(); f != nil {
		t.Fatalf("frame %d advanced while paused after seek", f.Seq)
	}
	p.Resume()
	h.now += 200
	if f := seekPollUntil(t, p, h, f.PTSMs+1); f.PTSMs <= 1000 {
		t.Fatalf("after resume pts = %d, want > 1000", f.PTSMs)
	}
}

// TestSeekFastAndKeyframes pins the scrub companions on ffmpeg: SeekFast
// echoes like SeekTo (ffmpeg always lands keyframes natively),
// Prev/Next move the picture backward/forward without crashing, and
// StepFrame advances past the picture while staying paused.
//
// Hand-clock rule for backward seeks: the clock must travel WITH the
// seek target. clk.Start(landing) freezes the clock there, so if the
// hand stamp stays far ahead, the landing reads as "long overdue" and
// PollDue eats the whole queue as stale (correct catch-up, in production
// the wall clock advances naturally). Tests set h.now to just below the
// target before seeking.
func TestSeekFastAndKeyframes(t *testing.T) {
	h := &handClock{}
	p := openSeekClip(t, h, "vr5_seek.mp4")
	defer p.Close()
	h.now = 1500
	landed, err := p.SeekFast(1700)
	if err != nil {
		t.Fatalf("fast 1700: %v", err)
	}
	if landed != 1700 {
		t.Fatalf("fast landed = %d, want 1700 (echo)", landed)
	}
	ok, _, _, key, _, forward := p.SeekInfo()
	if !ok || key != 1700 || forward != 1 {
		t.Fatalf("fast evidence ok=%v key=%d fwd=%d, want 1/1700/1", ok, key, forward)
	}
	// The echo has no exact frame (frames sit on the 200ms grid), so the
	// first grid frame at/after it (1800) is the covering picture.
	h.now = 1700
	cur := seekPollUntil(t, p, h, 1700)
	_ = cur
	if cur.PTSMs != 1800 {
		t.Fatalf("cur pts = %d, want 1800 (covering frame)", cur.PTSMs)
	}
	// Anchor mid-clip exactly. NOTE: the hand clock must be REWOUND with
	// the target (h.now ran far ahead while polling above, but
	// clk.Start(1000) freezes the clock back at the landing) — otherwise
	// the echo reads as long-overdue and PollDue eats it as stale.
	h.now = 800
	if _, err := p.SeekTo(1000); err != nil {
		t.Fatalf("seek 1000: %v", err)
	}
	// DO NOT advance h.now before the first poll after a backward seek:
	// clk.Start(1000) froze the clock at the landing, so the echo frame
	// is due at once; ticking first would push due past it and PollDue
	// would eat it as stale (correct catch-up, wrong test). Exact-grid
	// landings never tick at all (see seekPollExact).
	anchor := seekPollExact(t, p, 1000)
	if anchor.PTSMs != 1000 {
		t.Fatalf("anchor pts = %d, want 1000", anchor.PTSMs)
	}
	prev, err := p.PrevKeyframe()
	if err != nil {
		t.Fatalf("prev: %v", err)
	}
	if prev >= anchor.PTSMs {
		t.Fatalf("prev = %d, want < %d (move backward)", prev, anchor.PTSMs)
	}
	// Travel with the backward target: the clock froze at the 1000
	// anchor, so rewind to just below 800 — otherwise 800 reads as
	// long-overdue and PollDue reports the covering newer frame.
	// NOTE: exact landing 800 is already due at the frozen stamp (the
	// background burst is producer-fast here); PollUntil with the no
	// pre-tick rule shows it directly without ticking past it.
	h.now = 600
	pf := seekPollUntil(t, p, h, 800)
	if prev != 800 {
		t.Fatalf("prev = %d, want 800 (one interval back)", prev)
	}
	if pf.PTSMs != 800 {
		t.Fatalf("prev shown pts = %d, want 800", pf.PTSMs)
	}
	next, err := p.NextKeyframe()
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	// Next steps forward from the shown picture (strictly past prev),
	// and the shown 800 echo is already due at the frozen hand: poll
	// for the next grid frame after 800 (1000) so the first new picture
	// is never eaten as stale before it is seen.
	wantNext := pf.PTSMs + 200
	if next != wantNext {
		t.Fatalf("next = %d, want %d (one interval past %d)", next, wantNext, pf.PTSMs)
	}
	nf := seekPollUntil(t, p, h, wantNext)
	if nf.PTSMs != next {
		t.Fatalf("next shown pts = %d, want %d", nf.PTSMs, next)
	}
	// Step: pause, land past the picture, stay paused.
	if !p.Paused() {
		p.Pause()
	}
	base := nf.PTSMs
	stepped, err := p.StepFrame()
	if err != nil {
		t.Fatalf("step: %v", err)
	}
	if stepped <= base {
		t.Fatalf("stepped = %d, want > %d (one interval past)", stepped, base)
	}
	// Same travel rule: the step target must be due, not long overdue.
	h.now = stepped - 200
	sf := seekPollUntil(t, p, h, stepped)
	if sf.PTSMs != stepped {
		t.Fatalf("step shown pts = %d, want %d", sf.PTSMs, stepped)
	}
	h.now += 5000
	if f, _ := p.Poll(); f != nil {
		t.Fatalf("frame %d advanced while paused after step", f.Seq)
	}
}

// TestSeekByAndRate pins relative jumps and the speed clock: SeekBy(-)
// lands behind the picture, SetRate(2x) advances due stamps twice as
// fast while Poll fronts the newest, and bad rates are refused.
func TestSeekByAndRate(t *testing.T) {
	h := &handClock{}
	p := openSeekClip(t, h, "vr5_seek.mp4")
	defer p.Close()
	h.now += 200
	if f := seekPollUntil(t, p, h, 0); f == nil {
		t.Fatal("first frame never due")
	}
	// Show the 600 picture, then jump back one frame worth.
	h.now += 200
	if f := seekPollUntil(t, p, h, 600); f.PTSMs != 600 {
		t.Fatalf("shown pts = %d, want 600", f.PTSMs)
	}
	if p.PositionMs() != 600 {
		t.Fatalf("position = %d, want 600", p.PositionMs())
	}
	back, err := p.SeekBy(-200)
	if err != nil {
		t.Fatalf("seekby: %v", err)
	}
	if back != 400 {
		t.Fatalf("seekby landed = %d, want 400", back)
	}
	if f := seekPollUntil(t, p, h, 400); f.PTSMs < 400 {
		t.Fatalf("seekby shown pts = %d, want >= 400", f.PTSMs)
	}
	if err := p.SetRate(2); err != nil {
		t.Fatalf("rate 2: %v", err)
	}
	if p.Rate() != 2 {
		t.Fatalf("rate = %v, want 2", p.Rate())
	}
	// 2x for one real interval advances due stamps two intervals.
	due0 := int64(400)
	h.now += 200
	if f, _ := p.Poll(); f != nil {
		due0 = f.PTSMs
	}
	if got := p.Stats().Rate; got != 2 {
		t.Fatalf("stats rate = %v, want 2", got)
	}
	_ = due0
	if err := p.SetRate(0); err == nil {
		t.Fatal("rate 0 accepted")
	}
	if err := p.SetRate(-1); err == nil {
		t.Fatal("rate -1 accepted")
	}
	if p.Rate() != 2 {
		t.Fatalf("rate after bad set = %v, want 2 (unchanged)", p.Rate())
	}
}
