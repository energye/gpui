//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux && !(js && wasm)

package gles

import (
	"image"
	"testing"

	"github.com/energye/gpui/gpu/hal"
)

// TestPresentDamageUnionForAge locks the buffer-age contract: the presented
// union must cover the current frame plus every frame since the target back
// buffer was last displayed. Missing history must fall back to full.
func TestPresentDamageUnionForAge(t *testing.T) {
	cur := image.Rect(10, 10, 20, 20)
	h1 := image.Rect(40, 40, 50, 50)
	h2 := image.Rect(60, 60, 70, 70)

	cases := []struct {
		name   string
		hist   []image.Rectangle
		age    int
		want   image.Rectangle
		wantOK bool
	}{
		{name: "age1_current_only", hist: []image.Rectangle{h1}, age: 1, want: cur, wantOK: true},
		{name: "no_history", hist: nil, age: 1, wantOK: false},
		{name: "age2_union_prev", hist: []image.Rectangle{h1}, age: 2, want: cur.Union(h1), wantOK: true},
		{name: "age3_union_two", hist: []image.Rectangle{h1, h2}, age: 3, want: cur.Union(h1).Union(h2), wantOK: true},
		{name: "age0_undefined", hist: []image.Rectangle{h1}, age: 0, wantOK: false},
		{name: "age_beyond_history", hist: []image.Rectangle{h1}, age: 3, wantOK: false},
		{name: "age_beyond_cap", hist: []image.Rectangle{h1, h2}, age: maxPresentDamageHistory + 2, wantOK: false},
	}
	for _, c := range cases {
		s := &Surface{}
		for _, r := range c.hist {
			s.recordPresentDamage(r)
		}
		got, ok := s.damageUnionForAge(c.age, cur)
		if ok != c.wantOK {
			t.Fatalf("%s: ok=%v want %v", c.name, ok, c.wantOK)
		}
		if ok && got != c.want {
			t.Fatalf("%s: union=%v want %v", c.name, got, c.want)
		}
	}
	if _, ok := (&Surface{}).damageUnionForAge(1, image.Rectangle{}); ok {
		t.Fatal("empty current damage must fall back to full")
	}
}

// TestRecordPresentDamageKeepsNewestFirst pins the ring order used by
// damageUnionForAge: index 0 is the most recent presented damage.
func TestRecordPresentDamageKeepsNewestFirst(t *testing.T) {
	s := &Surface{}
	r1 := image.Rect(0, 0, 1, 1)
	r2 := image.Rect(1, 1, 2, 2)
	r3 := image.Rect(2, 2, 3, 3)
	s.recordPresentDamage(r1)
	s.recordPresentDamage(r2)
	s.recordPresentDamage(r3)
	if s.presentDamageHistLen != 3 {
		t.Fatalf("hist len = %d, want 3", s.presentDamageHistLen)
	}
	if s.presentDamageHist[0] != r3 || s.presentDamageHist[1] != r2 || s.presentDamageHist[2] != r1 {
		t.Fatalf("history order = %v, want [r3 r2 r1]", s.presentDamageHist[:3])
	}
	for i := 0; i < maxPresentDamageHistory+2; i++ {
		s.recordPresentDamage(image.Rect(i, i, i+1, i+1))
	}
	if s.presentDamageHistLen != maxPresentDamageHistory {
		t.Fatalf("hist len after overflow = %d, want %d", s.presentDamageHistLen, maxPresentDamageHistory)
	}
}

func TestPresentDamageCoversFull(t *testing.T) {
	if !presentDamageCoversFull(image.Rect(0, 0, 100, 100), 100, 100) {
		t.Fatal("full-surface rect must promote to full")
	}
	if presentDamageCoversFull(image.Rect(10, 10, 20, 20), 100, 100) {
		t.Fatal("small rect must not promote to full")
	}
	if !presentDamageCoversFull(image.Rect(0, 0, 1, 1), 0, 0) {
		t.Fatal("invalid surface size must fall back to full")
	}
	if !presentDamageCoversFull(image.Rectangle{}, 100, 100) {
		t.Fatal("empty damage must fall back to full")
	}
}

func TestCanScissorPresent(t *testing.T) {
	s := &Surface{config: &hal.SurfaceConfiguration{Width: 100, Height: 50}, fboWidth: 100, fboHeight: 50}
	if !s.canScissorPresent() {
		t.Fatal("matching FBO/config must allow the scissored path")
	}
	s.fboWidth = 90
	if s.canScissorPresent() {
		t.Fatal("borrowed old FBO must present full")
	}
	s.fboWidth = 100
	s.config = nil
	if s.canScissorPresent() {
		t.Fatal("missing config must present full")
	}
}

func TestResetPresentDamageHistory(t *testing.T) {
	s := &Surface{}
	s.recordPresentDamage(image.Rect(0, 0, 5, 5))
	s.resetPresentDamageHistory()
	if s.presentDamageHistLen != 0 {
		t.Fatalf("hist len = %d, want 0", s.presentDamageHistLen)
	}
	if _, ok := s.damageUnionForAge(1, image.Rect(0, 0, 5, 5)); ok {
		t.Fatal("no history after reset must force a full present")
	}
}

// TestPresentHistoryDoesNotAmplifyFullFrame guards the regression where
// recording the presented union (instead of the frame's real damage) turned
// one full frame into full presents forever: after the forced frame, small
// damage must become partial again once the buffer-age window rotates.
func TestPresentHistoryDoesNotAmplifyFullFrame(t *testing.T) {
	s := &Surface{}
	full := image.Rect(0, 0, 100, 100)
	small := image.Rect(10, 10, 20, 20)

	// Frame 1: no history -> full, record full.
	s.recordPresentDamage(full)
	if _, ok := s.damageUnionForAge(1, small); !ok {
		t.Fatal("age 1 after a full frame should be presentable")
	}

	// Frame 2 (age 2): union with frame 1 is full; record only the real
	// frame damage.
	if u, ok := s.damageUnionForAge(2, small); !ok || u != full {
		t.Fatalf("frame 2 union = %v ok=%v, want full", u, ok)
	}
	s.recordPresentDamage(small)

	// Frame 3 (age 2): the current buffer was displayed at frame 1 (full);
	// changes since are frame 2 + frame 3, both small.
	u, ok := s.damageUnionForAge(2, small)
	if !ok || u != small {
		t.Fatalf("frame 3 union = %v ok=%v, want small %v (full amplified)", u, ok, small)
	}
}
