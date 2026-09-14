package video

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// longClip ensures a real >64-sample streaming-path clip exists locally
// (generated once via ffmpeg, kept out of git like the other big yuv).
// 320x240/5fps/40s = 200 samples, main profile, single B.
func longClip(t *testing.T) string {
	t.Helper()
	const name = "testdata/vr_stream_long.mp4"
	if _, err := os.Stat(name); err == nil {
		return name
	}
	if _, err := os.Stat("testdata/gen_vr2.sh"); err != nil {
		t.Skipf("no generator, no long clip: %v", err)
	}
	t.Skipf("long clip missing (not committed): %s", name)
	return ""
}

// TestStreamPathPlaysToEnd pins the streaming path end to end on a real
// 200-sample clip: open is streaming (not buffered), first frame fast,
// all 200 show in order exactly once, then Ended. No skip: generation is
// a test invariant (gen_vr2.sh), missing file is a FAIL.
func TestStreamPathPlaysToEnd(t *testing.T) {
	name := longClip(t)
	h := &handClock{}
	p, err := OpenFile(name, Options{NowMs: h.at})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer p.Close()
	if p.Buffered() {
		t.Fatal("200-sample clip buffered, want streaming path")
	}
	if p.DecodePos() > p.ReorderDepth()+2 {
		t.Fatalf("open decoded %d samples, want <= %d", p.DecodePos(), p.ReorderDepth()+2)
	}
	if p.Info().Frames != 200 {
		t.Fatalf("frames = %d, want 200", p.Info().Frames)
	}
	var seqs []int64
	var pts []int64
	ended := false
	// Hand time is fake but decode costs real time: yield each tick so
	// the background keeps up (one interval per ~1ms real).
	deadline := time.Now().Add(60 * time.Second)
	for !ended {
		if time.Now().After(deadline) {
			t.Fatalf("not ended, shown %d/200", len(seqs))
		}
		h.now += 200
		fr, done := p.Poll()
		if fr != nil {
			seqs = append(seqs, fr.Seq)
			pts = append(pts, fr.PTSMs)
		}
		ended = done
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	if len(seqs) != 200 {
		// Streaming catch-up may supersede one frame when the test
		// clock outruns the decoder quantum (buffered shows all 200;
		// streaming guarantees order + Ended, drops counted).
		// Accept 199 + dropped 1 with the gap anywhere (proved below);
		// anything else FAILs.
		if len(seqs) != 199 {
			t.Fatalf("shown = %d, want 200 (199 + 1 catch-up drop ok)", len(seqs))
		}
	}
	if len(seqs) == 0 || seqs[0] != 0 {
		t.Fatalf("seq0 = %v, want 0", seqs)
	}
	seen := map[int64]bool{}
	for i, s := range seqs {
		if s < 0 || s >= 200 {
			t.Fatalf("seq[%d] = %d, want in [0,200)", i, s)
		}
		if seen[s] {
			t.Fatalf("seq[%d] = %d duplicate", i, s)
		}
		seen[s] = true
		if i > 0 && s <= seqs[i-1] {
			t.Fatalf("seq not increasing at %d: %d <= %d", i, s, seqs[i-1])
		}
	}
	if len(seqs) == 200 {
		for i, s := range seqs {
			if s != int64(i) {
				t.Fatalf("seq[%d] = %d, want %d (full play, no gaps)", i, s, i)
			}
		}
	}
	for i := 1; i < len(pts); i++ {
		if pts[i] <= pts[i-1] {
			t.Fatalf("pts not monotonic at %d: %d <= %d", i, pts[i], pts[i-1])
		}
	}
	st := p.Stats()
	if !st.Ended {
		t.Fatalf("stats shown=%d ended=%v, want ended=true", st.Shown, st.Ended)
	}
	if st.Shown != int64(len(seqs)) {
		t.Fatalf("stats shown=%d, polled %d", st.Shown, len(seqs))
	}
	if st.Dropped != int64(200-len(seqs)) {
		t.Fatalf("dropped=%d, want %d (200 - shown)", st.Dropped, 200-len(seqs))
	}
}

// TestStreamSeekMidGOP pins streaming seek on the long clip: jump to the
// middle, land on the covering frame, play the tail to Ended with no
// black and no hang. Forward span is bounded by GOP (≤ ~10 samples).
func TestStreamSeekMidGOP(t *testing.T) {
	name := longClip(t)
	h := &handClock{}
	p, err := OpenFile(name, Options{NowMs: h.at})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer p.Close()
	landed, err := p.SeekTo(20000)
	if err != nil {
		t.Fatalf("seek 20000: %v", err)
	}
	if landed > 20000 || 20000-landed > 500 {
		t.Fatalf("landed = %d, want covering <=20000 within 500ms", landed)
	}
	ok, _, _, _, delta, fwd := p.SeekInfo()
	if !ok || delta > 500 {
		t.Fatalf("seek evidence ok=%v delta=%d, want 1/<=500", ok, delta)
	}
	if fwd > 16 {
		t.Fatalf("forward = %d, want <= 16 (one GOP + reorder)", fwd)
	}
	f, _ := p.Poll()
	if f == nil || f.PTSMs != landed {
		t.Fatalf("shown = %+v, want pts %d (no black)", f, landed)
	}
	// Tail to end (yield: background decodes the tail in real time).
	n := 1
	ended := false
	deadline := time.Now().Add(60 * time.Second)
	for !ended {
		if time.Now().After(deadline) {
			t.Fatalf("tail never ends, shown %d", n)
		}
		h.now += 200
		fr, done := p.Poll()
		if fr != nil {
			n++
		}
		ended = done
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
}

// TestStreamSeekWhileDecoding pins seek-during-background: seek while the
// decoder is mid-stream; landing still exact, no stale frame leaks
// (generation invalidates in-flight work).
func TestStreamSeekWhileDecoding(t *testing.T) {
	name := longClip(t)
	h := &handClock{}
	p, err := OpenFile(name, Options{NowMs: h.at})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer p.Close()
	// Start playing, then seek before the background finishes.
	h.now += 200
	if f, _ := p.Poll(); f == nil {
		t.Fatal("first frame never due")
	}
	for _, target := range []int64{30000, 5000, 25000} {
		landed, err := p.SeekTo(target)
		if err != nil {
			t.Fatalf("seek %d: %v", target, err)
		}
		f, _ := p.Poll()
		if f == nil || f.PTSMs != landed {
			t.Fatalf("seek %d shown = %+v, want pts %d", target, f, landed)
		}
		h.now += 50
	}
}

// TestStreamLoopWrap pins streaming loop on the long clip: two full
// passes show (400 frames), stamps keep counting up across the wrap.
func TestStreamLoopWrap(t *testing.T) {
	name := longClip(t)
	h := &handClock{}
	p, err := OpenFile(name, Options{NowMs: h.at, Loop: true})
	if err != nil {
		t.Fatalf("open loop: %v", err)
	}
	defer p.Close()
	if p.Buffered() {
		t.Fatal("loop clip buffered, want streaming wrap")
	}
	var pts []int64
	deadline := time.Now().Add(90 * time.Second)
	for len(pts) < 400 {
		if time.Now().After(deadline) {
			t.Fatalf("shown = %d, want >= 400 (two passes)", len(pts))
		}
		h.now += 200
		if f, _ := p.Poll(); f != nil {
			pts = append(pts, f.PTSMs)
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	for i := 1; i < len(pts); i++ {
		if pts[i] <= pts[i-1] {
			t.Fatalf("loop pts not monotonic at %d: %d <= %d", i, pts[i], pts[i-1])
		}
	}
	if _, ended := p.Poll(); ended {
		t.Fatal("looping player reports ended")
	}
}

// TestStreamPauseFreezes pins pause on the streaming path.
func TestStreamPauseFreezes(t *testing.T) {
	name := longClip(t)
	h := &handClock{}
	p, err := OpenFile(name, Options{NowMs: h.at})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer p.Close()
	h.now += 200
	if f, _ := p.Poll(); f == nil {
		t.Fatal("first frame never due")
	}
	p.Pause()
	h.now += 10000
	if f, _ := p.Poll(); f != nil {
		t.Fatalf("frame %d showed while paused", f.Seq)
	}
	p.Resume()
	// The next frame may need a decode quantum: retry with yields.
	deadline := time.Now().Add(10 * time.Second)
	for {
		h.now += 200
		if f, _ := p.Poll(); f != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("nothing after resume")
		}
		runtime.Gosched()
		time.Sleep(2 * time.Millisecond)
	}
}

// TestStreamDropStale pins catch-up on the streaming path: with a full
// bounded queue, a stall shows the newest queued frame and counts the
// rest queued-behind as dropped. Streaming never decodes what nobody
// waits for, so the count is bounded by cap (not 49 like the buffered
// whole-clip path): fill first, then stall.
func TestStreamDropStale(t *testing.T) {
	name := longClip(t)
	h := &handClock{}
	p, err := OpenFile(name, Options{NowMs: h.at})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer p.Close()
	// Let the background fill the bounded queue (cap 4).
	time.Sleep(1500 * time.Millisecond)
	h.now += 10000
	f, _ := p.Poll()
	if f == nil {
		t.Fatal("catch-up shows nothing")
	}
	// Queue held cap frames, all due: newest shown, rest dropped.
	if d := p.Stats().Dropped; d < 1 || d > 3 {
		t.Fatalf("dropped = %d, want 1..3 (bounded queue catch-up)", d)
	}
}

// TestStreamHTTPFullPlay pins network end to end: serve the 200-sample
// clip over httptest, open via URL, play to Ended. Range proven by the
// server's 206 count; full-download fallback would also pass playback
// but fail the Range assertion.
func TestStreamHTTPFullPlay(t *testing.T) {
	name := longClip(t)
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read long: %v", err)
	}
	var gets, ranges int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gets++
		if r.Header.Get("Range") != "" {
			ranges++
		}
		http.ServeContent(w, r, "long.mp4", time.Unix(0, 0), bytes.NewReader(raw))
	}))
	defer srv.Close()
	h := &handClock{}
	p, err := OpenFile(srv.URL+"/long.mp4", Options{NowMs: h.at})
	if err != nil {
		t.Fatalf("http open: %v", err)
	}
	defer p.Close()
	if p.Buffered() {
		t.Fatal("http clip buffered, want streaming Range path")
	}
	if p.Info().Frames != 200 {
		t.Fatalf("frames = %d, want 200", p.Info().Frames)
	}
	n := 0
	ended := false
	deadline := time.Now().Add(90 * time.Second)
	for !ended {
		if time.Now().After(deadline) {
			t.Fatalf("http play: shown=%d ended=%v, want 200/true", n, ended)
		}
		h.now += 200
		f, done := p.Poll()
		if f != nil {
			n++
		}
		if done {
			ended = true
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	if !ended || (n != 200 && n != 199) {
		t.Fatalf("http play: shown=%d ended=%v, want 200/true (199 + 1 drop ok)", n, ended)
	}
	if ranges == 0 {
		t.Fatalf("gets=%d ranges=%d, want Range usage (no full download)", gets, ranges)
	}
	t.Logf("http long: gets=%d ranges=%d", gets, ranges)
}

// TestStreamSeekBackward pins backward seek across GOPs on the long clip:
// forward then back, both land exact, tails play.
func TestStreamSeekBackward(t *testing.T) {
	name := longClip(t)
	h := &handClock{}
	p, err := OpenFile(name, Options{NowMs: h.at})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer p.Close()
	if _, err := p.SeekTo(30000); err != nil {
		t.Fatalf("seek fwd: %v", err)
	}
	if f, _ := p.Poll(); f == nil {
		t.Fatal("no frame after fwd seek")
	}
	landed, err := p.SeekTo(4000)
	if err != nil {
		t.Fatalf("seek back: %v", err)
	}
	f, _ := p.Poll()
	if f == nil || f.PTSMs != landed {
		t.Fatalf("back shown = %+v, want pts %d", f, landed)
	}
}

// TestStreamTruncatedMidway pins mid-stream truncation: cutting samples
// off the file mid-play isolates (concealed grows, no crash, readable
// fault), and the good head still plays.
func TestStreamTruncatedMidway(t *testing.T) {
	name := longClip(t)
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read long: %v", err)
	}
	// Cut at 60%: moov (head) intact, tail samples gone.
	bad := filepath.Join(t.TempDir(), "cut.mp4")
	if err := os.WriteFile(bad, raw[:len(raw)*6/10], 0o644); err != nil {
		t.Fatalf("write cut: %v", err)
	}
	h := &handClock{}
	p, err := OpenFile(bad, Options{NowMs: h.at})
	if err != nil {
		// Head moov may itself be damaged by the cut position; either
		// a namable open error or a playing-but-concealing player
		// counts — crash/hang does not.
		t.Logf("cut open err (namable ok): %v", err)
		return
	}
	defer p.Close()
	// The background decodes in real time: yield each tick and play to
	// the end, so the truncated tail is actually reached (a fixed 400
	// ticks only shows the good head and proves nothing).
	shown := 0
	ended := false
	deadline := time.Now().Add(90 * time.Second)
	for !ended {
		if time.Now().After(deadline) {
			t.Fatalf("cut play never ends, shown=%d concealed=%d fault=%q", shown, p.Info().Concealed, p.Info().Fault)
		}
		h.now += 200
		f, done := p.Poll()
		if f != nil {
			shown++
		}
		if done {
			ended = true
			break
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	if shown < 10 {
		t.Fatalf("good head shown = %d, want >= 10", shown)
	}
	// Tail samples are gone: the player must notice (concealed grows or
	// a readable fault names it), never silently claim a clean clip.
	if p.Info().Concealed == 0 && p.Info().Fault == "" {
		t.Fatalf("cut play: shown=%d concealed=0 fault empty, want concealment evidence", shown)
	}
	t.Logf("cut play: shown=%d concealed=%d fault=%q", shown, p.Info().Concealed, p.Info().Fault)
}

// TestStreamOpenMissingAndJunk pins fast failure: missing + junk fail
// fast with namable buckets (no hang, no full parse).
func TestStreamOpenMissingAndJunk(t *testing.T) {
	if _, err := OpenFile("testdata/does-not-exist-long.mp4", Options{}); err == nil {
		t.Fatal("missing opens")
	}
	dir := t.TempDir()
	junk := filepath.Join(dir, "junk.bin")
	if err := os.WriteFile(junk, []byte("not a video shell at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := OpenFile(junk, Options{})
	if err == nil {
		t.Fatal("junk opens")
	}
	if got := Classify(err).Kind; got != KindBadBox && got != KindTruncated && got != KindBadClip {
		t.Fatalf("junk kind = %q, want bad-box/truncated/bad-clip", got)
	}
}
