//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package video

import (
	"runtime"
	"testing"
	"time"
)

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

func TestS6SmallClipPooled(t *testing.T) {
	h := &handClock{}
	p, err := OpenFile("testdata/vr2_m_bframes.mp4", Options{NowMs: h.at})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer p.Close()
	if p.Buffered() {
		t.Fatal("want ffmpeg streaming path, not buffered")
	}
	ended := false
	var shown int64
	deadline := time.Now().Add(30 * time.Second)
	for !ended {
		if time.Now().After(deadline) {
			t.Fatalf("small clip not ended, shown %d", shown)
		}
		h.now += 200
		fr, done := p.Poll()
		if fr != nil {
			shown++
		}
		ended = done
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	if shown != 5 {
		t.Fatalf("shown=%d, want 5", shown)
	}
	lv := p.pooled.Load()
	if lv == nil || lv.pools == nil || lv.pools.RGBA == nil {
		t.Fatal("streaming player holds no RGBA pool")
	}
	// Leak verdict: Close recycles the queued tail and the displayed
	// frame, so nothing stays borrowed.
	p.Close()
	if n := lv.pools.OutstandingTotal(); n != 0 {
		t.Fatalf("outstanding = %d after Close, want 0 (no leak)", n)
	}
}
