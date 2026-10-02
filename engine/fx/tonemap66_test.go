//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package fx

import (
	"math"
	"testing"

	"github.com/energye/gpui/engine/core"
)

const epsTonemap66 = 1e-9

// A: filmic curve is selectable and sane (finite in, [0,1] out, monotonic).
func TestTonemapFilmicCurve(t *testing.T) {
	if got := TonemapFilmic.String(); got != "filmic" {
		t.Fatalf("String = %q, want filmic", got)
	}
	if tm, err := ParseTonemap("filmic"); err != nil || tm != TonemapFilmic {
		t.Fatalf("parse filmic = %v, %v", tm, err)
	}
	prev := -1.0
	for _, x := range []float64{0, 0.25, 0.5, 1, 2, 4, 8} {
		got := TonemapFilmic.Map(x, 1)
		if math.IsNaN(got) || math.IsInf(got, 0) || got < 0 || got > 1 {
			t.Fatalf("filmic(%v) = %v, want [0,1]", x, got)
		}
		if got < prev {
			t.Fatalf("filmic not monotonic at %v", x)
		}
		prev = got
	}
	// 8x white compresses below plain ACES: shoulder, not clip.
	a := TonemapACES.Map(8, 1)
	f := TonemapFilmic.Map(8, 1)
	if !(f < a) {
		t.Fatalf("filmic(8) = %v, want below aces %v", f, a)
	}
}

// B: default freeze is Reinhard (windows and goldens pin this).
func TestTonemapDefaultFrozen(t *testing.T) {
	if DefaultTonemap != TonemapReinhard {
		t.Fatalf("DefaultTonemap = %v, want reinhard", DefaultTonemap)
	}
	if got := TonemapReinhard.Map(2, 1); math.Abs(got-2.0/3.0) > epsTonemap66 {
		t.Fatalf("reinhard(2) = %v, want %v", got, 2.0/3.0)
	}
}

// C: degraded bit mirrors the tonemap switch (no silent CPU).
func TestGradeDegradedBit(t *testing.T) {
	bloom, err := NewBloom(1, 1, 0.5)
	if err != nil {
		t.Fatalf("bloom: %v", err)
	}
	vig, err := NewVignette(core.V2(0.5, 0.5), 0.4, 0.9, 0.5)
	if err != nil {
		t.Fatalf("vignette: %v", err)
	}
	plain, err := NewGrade(bloom, vig, nil, TonemapNone, 0)
	if err != nil {
		t.Fatalf("grade none: %v", err)
	}
	if plain.Degraded() {
		t.Fatal("tonemap off must not mark degraded")
	}
	if _, on := plain.ExposureValue(); on {
		t.Fatal("tonemap off must report exposure off")
	}
	mapped, err := NewGrade(bloom, vig, nil, TonemapReinhard, 1)
	if err != nil {
		t.Fatalf("grade reinhard: %v", err)
	}
	if !mapped.Degraded() {
		t.Fatal("tonemap on must mark degraded (CPU reference)")
	}
	if e, on := mapped.ExposureValue(); !on || e != 1 {
		t.Fatalf("exposure = (%v,%v), want (1,true)", e, on)
	}
}

// D: bad tonemap/exposure still refuse (widening only admits filmic).
func TestTonemapRejectsBad(t *testing.T) {
	if _, err := ParseTonemap("nope"); err == nil {
		t.Fatal("bad name must fail")
	}
	bloom, err := NewBloom(1, 1, 0.5)
	if err != nil {
		t.Fatalf("bloom: %v", err)
	}
	vig, err := NewVignette(core.V2(0.5, 0.5), 0.4, 0.9, 0.5)
	if err != nil {
		t.Fatalf("vignette: %v", err)
	}
	if _, err := NewGrade(bloom, vig, nil, Tonemap(99), 1); err == nil {
		t.Fatal("bad tonemap must fail")
	}
	if _, err := NewGrade(bloom, vig, nil, TonemapReinhard, 99); err == nil {
		t.Fatal("bad exposure must fail")
	}
}

// E: HDR explains itself (2x white rolls over, never a white sheet).
func TestTonemapHDRVs8Bit(t *testing.T) {
	got := TonemapReinhard.Map(2, 1)
	if math.Abs(got-2.0/3.0) > epsTonemap66 {
		t.Fatalf("reinhard(2) = %v, want %v", got, 2.0/3.0)
	}
	if got >= 1 {
		t.Fatal("2x white must roll over, not clip to sheet")
	}
}

// F: filmic double replay is bitwise identical (parity gate material).
func TestTonemapFilmicReplay(t *testing.T) {
	a := TonemapFilmic.Map(3.5, 1.5)
	b := TonemapFilmic.Map(3.5, 1.5)
	if a != b {
		t.Fatalf("replay differs: %v vs %v", a, b)
	}
}
