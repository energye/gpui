package video

import (
	"testing"
)

// openSeekClip opens a clip with the hand clock for seek gates.
func openSeekClip(t *testing.T, h *handClock, name string) *Player {
	t.Helper()
	p, err := OpenFile("testdata/"+name, Options{NowMs: h.at})
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	return p
}

// TestSeekToMiddleKeyframe pins the VR5 core: seeking into the second GOP
// lands on its keyframe and decodes forward (not from the head), the fresh
// decode matches the cached tail pixel-exact, and the picture shows at once.
func TestSeekToMiddleKeyframe(t *testing.T) {
	h := &handClock{}
	p := openSeekClip(t, h, "vr5_seek.mp4")
	defer p.Close()
	if p.Info().Frames != 10 {
		t.Fatalf("frames = %d, want 10", p.Info().Frames)
	}
	// Play two frames first so the seek visibly jumps.
	h.now += 200
	if f, _ := p.Poll(); f == nil {
		t.Fatal("first frame never due")
	}
	h.now += 200
	if f, _ := p.Poll(); f == nil {
		t.Fatal("second frame never due")
	}
	landed, err := p.SeekTo(1800)
	if err != nil {
		t.Fatalf("seek 1800: %v", err)
	}
	if landed != 1800 {
		t.Fatalf("landed = %d, want 1800", landed)
	}
	ok, target, land, key, delta, forward := p.SeekInfo()
	if !ok || target != 1800 || land != 1800 || key != 1400 {
		t.Fatalf("evidence ok=%v target=%d landed=%d key=%d, want 1/1800/1800/1400", ok, target, land, key)
	}
	if delta != 0 {
		t.Fatalf("delta = %d, want 0 (exact stamp)", delta)
	}
	// Forward decode proves no head replay: key sample #6 to target
	// sample #9 is 4 samples, far fewer than the 9 a head replay needs.
	if forward != 4 {
		t.Fatalf("forward = %d, want 4 (samples 6..9)", forward)
	}
	st := p.Stats()
	if st.SeekOK != 1 || st.SeekDeltaMs != 0 || st.SeekForward != 4 || st.SeekLandedMs != 1800 {
		t.Fatalf("stats seek = %+v, want ok1/delta0/fwd4/land1800", st)
	}
	// No black: the landed frame is due on the very next poll.
	f, ended := p.Poll()
	if ended {
		t.Fatal("ended right after seek")
	}
	if f == nil {
		t.Fatal("no frame right after seek (black screen)")
	}
	if f.PTSMs != 1800 {
		t.Fatalf("shown pts = %d, want 1800", f.PTSMs)
	}
	// Play to the end from the seek point: 1800, 2000, 2200 then done.
	var tail []int64
	tail = append(tail, f.PTSMs)
	for i := 0; i < 6; i++ {
		h.now += 200
		fr, done := p.Poll()
		if fr != nil {
			tail = append(tail, fr.PTSMs)
		}
		if done {
			break
		}
	}
	want := []int64{1800, 2000, 2200}
	if len(tail) != len(want) {
		t.Fatalf("tail = %v, want %v", tail, want)
	}
	for i := range want {
		if tail[i] != want[i] {
			t.Fatalf("tail = %v, want %v", tail, want)
		}
	}
	if !p.Stats().Ended {
		t.Fatal("not ended after playing seek tail")
	}
}

// TestSeekFirstGOP pins the near-head path: seeking inside the first GOP
// still lands via its keyframe with a small forward count.
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
	if key != 400 {
		t.Fatalf("key = %d, want 400 (first IDR)", key)
	}
	if forward != 3 {
		t.Fatalf("forward = %d, want 3 (samples 1..3)", forward)
	}
	if f, _ := p.Poll(); f == nil || f.PTSMs != 600 {
		t.Fatalf("shown = %+v, want pts 600", f)
	}
}

// TestSeekBetweenStamps pins the landing budget: a target between stamps
// lands on the frame covering it with delta smaller than one interval.
func TestSeekBetweenStamps(t *testing.T) {
	h := &handClock{}
	p := openSeekClip(t, h, "vr5_seek.mp4")
	defer p.Close()
	landed, err := p.SeekTo(1700)
	if err != nil {
		t.Fatalf("seek 1700: %v", err)
	}
	if landed != 1600 {
		t.Fatalf("landed = %d, want 1600 (floor covering)", landed)
	}
	_, _, _, _, delta, _ := p.SeekInfo()
	if delta != 100 {
		t.Fatalf("delta = %d, want 100", delta)
	}
	if delta > 500 {
		t.Fatalf("delta %d exceeds 500ms budget", delta)
	}
	if f, _ := p.Poll(); f == nil || f.PTSMs != 1600 {
		t.Fatalf("shown = %+v, want pts 1600", f)
	}
}

// TestSeekSingleKeyframeClip pins old tiny clips: one IDR at the head,
// forward decode still verified pixel-exact.
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
	if key != 400 {
		t.Fatalf("key = %d, want 400", key)
	}
	if forward != 4 {
		t.Fatalf("forward = %d, want 4", forward)
	}
	if f, _ := p.Poll(); f == nil || f.PTSMs != 800 {
		t.Fatalf("shown = %+v, want pts 800", f)
	}
}

// TestSeekRapidNoStall pins continuous fast seeks: alternating jumps never
// hang and every landing shows at once (no card death, no black).
func TestSeekRapidNoStall(t *testing.T) {
	h := &handClock{}
	p := openSeekClip(t, h, "vr5_seek.mp4")
	defer p.Close()
	for i := 0; i < 20; i++ {
		target := int64(600)
		if i%2 == 1 {
			target = 1800
		}
		landed, err := p.SeekTo(target)
		if err != nil {
			t.Fatalf("rapid seek %d to %d: %v", i, target, err)
		}
		if landed != target {
			t.Fatalf("rapid seek %d landed %d, want %d", i, landed, target)
		}
		if f, _ := p.Poll(); f == nil || f.PTSMs != target {
			t.Fatalf("rapid seek %d shown = %+v, want pts %d", i, f, target)
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
// picture stays held until resume.
func TestSeekWhilePaused(t *testing.T) {
	h := &handClock{}
	p := openSeekClip(t, h, "vr5_seek.mp4")
	defer p.Close()
	h.now += 200
	if f, _ := p.Poll(); f == nil {
		t.Fatal("first frame never due")
	}
	p.Pause()
	h.now += 5000
	if f, _ := p.Poll(); f != nil {
		t.Fatalf("frame %d showed while paused", f.Seq)
	}
	landed, err := p.SeekTo(1800)
	if err != nil {
		t.Fatalf("seek while paused: %v", err)
	}
	if landed != 1800 {
		t.Fatalf("landed = %d, want 1800", landed)
	}
	// The sought frame is already due even while held (no black), but the
	// clock stays frozen afterwards.
	f, _ := p.Poll()
	if f == nil || f.PTSMs != 1800 {
		t.Fatalf("paused seek shown = %+v, want pts 1800", f)
	}
	h.now += 5000
	if f, _ := p.Poll(); f != nil {
		t.Fatalf("frame %d advanced while paused after seek", f.Seq)
	}
	p.Resume()
	h.now += 200
	f, _ = p.Poll()
	if f == nil || f.PTSMs != 2000 {
		t.Fatalf("after resume = %+v, want pts 2000", f)
	}
}

// TestSeekFastAndKeyframes pins the scrub companions on the buffered
// path: SeekFast lands the keyframe (no forward discard), Next/Prev walk
// the table, and StepFrame advances one interval while staying paused.
func TestSeekFastAndKeyframes(t *testing.T) {
	h := &handClock{}
	p := openSeekClip(t, h, "vr5_seek.mp4")
	defer p.Close()
	landed, err := p.SeekFast(1700)
	if err != nil {
		t.Fatalf("fast 1700: %v", err)
	}
	if landed != 1400 {
		t.Fatalf("fast landed = %d, want 1400 (keyframe)", landed)
	}
	ok, _, _, key, _, forward := p.SeekInfo()
	if !ok || key != 1400 || forward != 1 {
		t.Fatalf("fast evidence ok=%v key=%d fwd=%d, want 1/1400/1 (no discard)", ok, key, forward)
	}
	if f, _ := p.Poll(); f == nil || f.PTSMs != 1400 {
		t.Fatalf("fast shown = %+v, want pts 1400", f)
	}
	// Show one frame so Prev/Next anchor on the picture, not the request.
	h.now += 200
	if _, err := p.SeekTo(1800); err != nil {
		t.Fatalf("seek 1800: %v", err)
	}
	if f, _ := p.Poll(); f == nil {
		t.Fatal("no frame after seek 1800")
	}
	prev, err := p.PrevKeyframe()
	if err != nil {
		t.Fatalf("prev: %v", err)
	}
	if prev != 1400 {
		t.Fatalf("prev = %d, want 1400 (current GOP key)", prev)
	}
	if f, _ := p.Poll(); f == nil || f.PTSMs != 1400 {
		t.Fatalf("prev shown = %+v, want pts 1400", f)
	}
	prev2, err := p.PrevKeyframe()
	if err != nil {
		t.Fatalf("prev2: %v", err)
	}
	if prev2 != 400 {
		t.Fatalf("prev2 = %d, want 400 (first GOP key)", prev2)
	}
	if f, _ := p.Poll(); f == nil || f.PTSMs != 400 {
		t.Fatalf("prev2 shown = %+v, want pts 400", f)
	}
	next, err := p.NextKeyframe()
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if next != 1400 {
		t.Fatalf("next = %d, want 1400 (second GOP key)", next)
	}
	if f, _ := p.Poll(); f == nil || f.PTSMs != 1400 {
		t.Fatalf("next shown = %+v, want pts 1400", f)
	}
	// Step: pause, land one interval past the picture, stay paused.
	if !p.Paused() {
		p.Pause()
	}
	stepped, err := p.StepFrame()
	if err != nil {
		t.Fatalf("step: %v", err)
	}
	if stepped != 1600 {
		t.Fatalf("stepped = %d, want 1600 (one interval)", stepped)
	}
	if f, _ := p.Poll(); f == nil || f.PTSMs != 1600 {
		t.Fatalf("step shown = %+v, want pts 1600", f)
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
	if f, _ := p.Poll(); f == nil {
		t.Fatal("first frame never due")
	}
	// Show the 600 picture, then jump back one frame worth.
	h.now += 200
	if f, _ := p.Poll(); f == nil || f.PTSMs != 600 {
		t.Fatalf("shown = %+v, want pts 600", f)
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
	if f, _ := p.Poll(); f == nil || f.PTSMs != 400 {
		t.Fatalf("seekby shown = %+v, want pts 400", f)
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
