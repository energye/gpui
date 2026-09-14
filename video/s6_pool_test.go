package video

import (
	"runtime"
	"testing"
	"time"
)

// TestS6StreamingPoolSteady pins the S6 wiring on the real streaming path:
// convert buffers come from the RGBA pool (hits climb past 90%), every
// borrowed buffer comes back (zero outstanding after Close), and playback
// still runs to Ended. Heap-delta is deliberately NOT the probe here —
// the decoder's own YUV pictures are not pooled yet (S6 wires RGBA first,
// YUV follows later), so the crisp signal is pool misses staying flat
// while frames stream, not whole-process bytes.
func TestS6StreamingPoolSteady(t *testing.T) {
	name := longClip(t)
	h := &handClock{}
	p, err := OpenFile(name, Options{NowMs: h.at})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer p.Close()
	if p.Buffered() {
		t.Fatal("want streaming path")
	}
	// Play the whole 200-sample clip (hand time + real yields so the
	// background keeps up, same shape as the other long-clip gates).
	var shown int64
	var lastPTS int64
	ended := false
	deadline := time.Now().Add(90 * time.Second)
	for !ended {
		if time.Now().After(deadline) {
			t.Fatalf("not ended, shown %d", shown)
		}
		h.now += 200
		fr, done := p.Poll()
		if fr != nil {
			if shown > 0 && fr.PTSMs <= lastPTS {
				t.Fatalf("pts not monotonic: %d <= %d", fr.PTSMs, lastPTS)
			}
			lastPTS = fr.PTSMs
			shown++
		}
		ended = done
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	if shown < 190 {
		t.Fatalf("shown = %d, want >= 190", shown)
	}
	// Pool verdict: only the warmup frames miss (queue + displayed +
	// in-flight spares); the steady ~190 frames all hit.
	st := p.Stats()
	if st.PoolHitPct < 90.0 {
		t.Fatalf("pool_hit_pct = %.1f, want >= 90 (S6 RGBA wired)", st.PoolHitPct)
	}
	lv := p.pooled.Load()
	if lv == nil || lv.pools == nil || lv.pools.RGBA == nil {
		t.Fatal("streaming player holds no RGBA pool")
	}
	rst := lv.pools.RGBA.Stats()
	t.Logf("s6 steady: shown=%d acquires=%d hits=%d misses=%d hitpct=%.1f outstanding=%d",
		shown, rst.Acquires, rst.Hits, rst.Misses, rst.HitPct, rst.Outstanding)
	if rst.Misses > 10 {
		t.Fatalf("rgba misses = %d, want <= 10 (warmup only, steady must hit)", rst.Misses)
	}
	// Leak verdict: Close recycles the queued tail and the displayed
	// frame, so nothing stays borrowed.
	p.Close()
	if n := lv.pools.OutstandingTotal(); n != 0 {
		t.Fatalf("outstanding = %d after Close, want 0 (no leak)", n)
	}
}

// TestS6BufferedUnaffected pins the S6 boundary: small clips keep the
// exact old path (owned pixels, no pool), reporting 0 hit% honestly
// instead of a faked number.
func TestS6BufferedUnaffected(t *testing.T) {
	h := &handClock{}
	p, err := OpenFile("testdata/vr2_m_bframes.mp4", Options{NowMs: h.at})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer p.Close()
	if !p.Buffered() {
		t.Fatal("want buffered small-clip path")
	}
	ended := false
	var shown int64
	for i := 0; i < 8 && !ended; i++ {
		h.now += 200
		fr, done := p.Poll()
		if fr != nil {
			shown++
		}
		ended = done
	}
	if !ended || shown != 5 {
		t.Fatalf("shown=%d ended=%v, want 5/true", shown, ended)
	}
	if got := p.Stats().PoolHitPct; got != 0 {
		t.Fatalf("buffered pool_hit_pct = %v, want 0 (honest unavailable)", got)
	}
	if p.pooled.Load() != nil {
		t.Fatal("buffered player built a pool, want nil (owned pixels)")
	}
}
