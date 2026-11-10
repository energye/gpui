//go:build !nogpu

package recording_test

import (
	"image"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/render/recording"
)

// P4.3 ②③ shadow tile cache (2026-11-10): a static DrawShadow publishes a
// device-space tile on the first frame; the recorder hits it on the next frame
// and records a DrawTextureRefCommand{Device:true} instead of CmdDrawShadow.
// The hit must reproduce the chain image (the store crops the same layer pixmap
// the PopLayer composite uses), and the cache must invalidate on content change
// and stay off under GPUI_P43_BREAK=no-cache.

const (
	shadowCacheW = 260
	shadowCacheH = 200
	costW        = 1200
	costH        = 800
)

// shadowCachePathAt returns a per-test unique path (the key hashes path content,
// so tests must not share geometry or they would hit each other).
func shadowCachePathAt(dx, dy float64) *render.Path {
	p := render.NewPath()
	p.MoveTo(22+dx, 26+dy)
	p.LineTo(122+dx, 26+dy)
	p.LineTo(122+dx, 96+dy)
	p.LineTo(22+dx, 96+dy)
	p.Close()
	return p
}

func shadowCachePlay(t *testing.T, dc *render.Context, rec *recording.Recording) {
	t.Helper()
	if err := rec.Playback(recording.NewCanvasBackend(dc)); err != nil {
		t.Fatalf("playback: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Skipf("GPU flush unavailable: %v", err)
	}
}

func shadowCacheRecord(w, h int, p *render.Path, elevation float64) *recording.Recording {
	return shadowCacheRecordOpt(w, h, p, elevation, true)
}

func shadowCacheRecordOpt(w, h int, p *render.Path, elevation float64, withShadow bool) *recording.Recording {
	rec := recording.NewRecorder(w, h)
	// Opaque white surface as a normal fill (the real window's card background
	// is a fill too).
	rec.SetRGBA(1, 1, 1, 1)
	rec.DrawRectangle(0, 0, float64(w), float64(h))
	_ = rec.Fill()
	rec.Push()
	rec.Translate(17, 13) // non-identity CTM is part of the cache key
	rec.SetRGBA(0.05, 0.05, 0.08, 1)
	rec.DrawPath(p)
	_ = rec.Fill()
	if withShadow {
		rec.DrawShadow(p, render.RGBA{A: 1}, elevation, false)
	}
	rec.Pop()
	return rec.FinishRecording()
}

func shadowCacheHasRef(rec *recording.Recording) bool {
	for _, cmd := range rec.Commands() {
		if c, ok := cmd.(recording.DrawTextureRefCommand); ok && c.Device && c.Ref.Valid() {
			return true
		}
	}
	return false
}

func shadowCacheMaxDiff(a, b image.Image, r image.Rectangle) int {
	maxD := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			ar, ag, ab, _ := a.At(x, y).RGBA()
			br, bg, bb, _ := b.At(x, y).RGBA()
			for _, d := range []int{
				absInt(int(ar>>8) - int(br>>8)),
				absInt(int(ag>>8) - int(bg>>8)),
				absInt(int(ab>>8) - int(bb>>8)),
			} {
				if d > maxD {
					maxD = d
				}
			}
		}
	}
	return maxD
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func shadowCacheCommandTypes(rec *recording.Recording) []recording.CommandType {
	out := make([]recording.CommandType, 0, len(rec.Commands()))
	for _, cmd := range rec.Commands() {
		out = append(out, cmd.Type())
	}
	return out
}

func TestShadowCacheHitParity(t *testing.T) {
	p := shadowCachePathAt(0, 0)
	dc := render.NewContext(shadowCacheW, shadowCacheH)
	defer dc.Close()

	// Frame 1: the chain runs and publishes the tile.
	f1 := shadowCacheRecord(shadowCacheW, shadowCacheH, p, 12)
	shadowCachePlay(t, dc, f1)
	img1 := dc.Image()

	// Frame 2: the recorder must hit and record a device-space tile.
	f2 := shadowCacheRecord(shadowCacheW, shadowCacheH, p, 12)
	if !shadowCacheHasRef(f2) {
		t.Fatalf("second frame did not hit the shadow cache (commands: %v)", shadowCacheCommandTypes(f2))
	}
	dc.BeginFrame()
	shadowCachePlay(t, dc, f2)
	img2 := dc.Image()

	// Compare the interior: the 1px surface perimeter differs because the
	// chain's full-surface layer composite leaves a sub-LSB MSAA edge residue
	// (a pre-existing layer-composite artifact), which the window golden gate
	// excludes via its content rects. The drawn content must match exactly.
	interior := image.Rect(1, 1, shadowCacheW-1, shadowCacheH-1)
	if d := shadowCacheMaxDiff(img1, img2, interior); d > 1 {
		t.Fatalf("cache hit differs from the chain: max channel diff %d > 1", d)
	}
	// Both must carry a real shadow band below the body (the body ends at
	// device y≈109; elevation 12 lifts the silhouette to ≈121).
	band := 0
	for y := 130; y < 165; y++ {
		for x := 50; x < 130; x++ {
			r, g, b, _ := img2.At(x, y).RGBA()
			if r>>8 < 245 && g>>8 < 245 && b>>8 < 245 {
				band++
			}
		}
	}
	if band == 0 {
		t.Fatal("cache hit frame has no shadow band")
	}
}

func TestShadowCacheInvalidatesOnContentChange(t *testing.T) {
	p := shadowCachePathAt(20, 0)
	ctm := render.Translate(17, 13)
	dc := render.NewContext(shadowCacheW, shadowCacheH)
	defer dc.Close()
	f1 := shadowCacheRecord(shadowCacheW, shadowCacheH, p, 12)
	shadowCachePlay(t, dc, f1)

	tile, ok := render.LookupShadowCache(p, ctm, render.RGBA{A: 1}, 12, shadowCacheW, shadowCacheH, 1)
	if !ok || !tile.Ref.Valid() || tile.W <= 0 || tile.H <= 0 {
		t.Fatalf("lookup after publish = %+v ok=%v", tile, ok)
	}
	if _, ok := render.LookupShadowCache(p, ctm, render.RGBA{A: 1}, 13, shadowCacheW, shadowCacheH, 1); ok {
		t.Fatal("elevation change hit the cache")
	}
	if _, ok := render.LookupShadowCache(p, ctm, render.RGBA{R: 1, A: 1}, 12, shadowCacheW, shadowCacheH, 1); ok {
		t.Fatal("color change hit the cache")
	}
	if _, ok := render.LookupShadowCache(shadowCachePathAt(60, 0), ctm, render.RGBA{A: 1}, 12, shadowCacheW, shadowCacheH, 1); ok {
		t.Fatal("path change hit the cache")
	}
	if _, ok := render.LookupShadowCache(p, ctm, render.RGBA{A: 1}, 12, shadowCacheW+1, shadowCacheH, 1); ok {
		t.Fatal("device size change hit the cache")
	}
}

func TestShadowCacheNoCacheBreak(t *testing.T) {
	p := shadowCachePathAt(40, 0)
	dc := render.NewContext(shadowCacheW, shadowCacheH)
	defer dc.Close()
	f1 := shadowCacheRecord(shadowCacheW, shadowCacheH, p, 12)
	shadowCachePlay(t, dc, f1)

	t.Setenv("GPUI_P43_BREAK", "no-cache")
	if _, ok := render.LookupShadowCache(p, render.Translate(17, 13), render.RGBA{A: 1}, 12, shadowCacheW, shadowCacheH, 1); ok {
		t.Fatal("no-cache break still hit the cache")
	}
	f2 := shadowCacheRecord(shadowCacheW, shadowCacheH, p, 12)
	if shadowCacheHasRef(f2) {
		t.Fatal("no-cache break recorded a cached tile")
	}
}

func TestShadowCacheNestedRecordingKeepsHit(t *testing.T) {
	p := shadowCachePathAt(100, 0)
	dc := render.NewContext(shadowCacheW, shadowCacheH)
	defer dc.Close()
	f1 := shadowCacheRecord(shadowCacheW, shadowCacheH, p, 12)
	shadowCachePlay(t, dc, f1)
	img1 := dc.Image()

	inner := shadowCacheRecord(shadowCacheW, shadowCacheH, p, 12)
	if !shadowCacheHasRef(inner) {
		t.Fatalf("inner frame did not hit (commands: %v)", shadowCacheCommandTypes(inner))
	}
	outer := recording.NewRecorder(shadowCacheW, shadowCacheH)
	if err := inner.Playback(recording.NewCanvasBackend(outer)); err != nil {
		t.Fatalf("inner playback into outer recorder: %v", err)
	}
	folded := outer.FinishRecording()
	if !shadowCacheHasRef(folded) {
		t.Fatalf("nested fold dropped the device tile (commands: %v)", shadowCacheCommandTypes(folded))
	}
	dc.BeginFrame()
	shadowCachePlay(t, dc, folded)
	img2 := dc.Image()
	interior := image.Rect(1, 1, shadowCacheW-1, shadowCacheH-1)
	if d := shadowCacheMaxDiff(img1, img2, interior); d > 1 {
		t.Fatalf("nested device tile differs: max channel diff %d > 1", d)
	}
}

// TestShadowCacheSecondFrameCost: the hit frame must cost less than 1/4 of the
// full chain frame. Times include the flush/readback so async GPU work is
// included; the ratio (not an absolute ms) is the assertion.
func TestShadowCacheSecondFrameCost(t *testing.T) {
	if testRaceEnabled {
		t.Skip("timing assertion is not meaningful under -race")
	}
	p := shadowCachePathAt(80, 0)
	dc := render.NewContext(costW, costH)
	defer dc.Close()
	f1 := shadowCacheRecord(costW, costH, p, 12)
	shadowCachePlay(t, dc, f1)

	measure := func(rec *recording.Recording) time.Duration {
		dc.BeginFrame()
		start := time.Now()
		shadowCachePlay(t, dc, rec)
		_ = dc.Image()
		return time.Since(start)
	}
	// Baseline: same scene without the shadow. Subtracting it cancels the
	// frame's fixed fill + Image() readback cost, so the assertion measures the
	// shadow work itself (chain vs one cached quad).
	baseRec := shadowCacheRecordOpt(costW, costH, p, 12, false)
	_ = measure(baseRec) // warm
	bestBase := time.Hour
	for i := 0; i < 3; i++ {
		if d := measure(baseRec); d < bestBase {
			bestBase = d
		}
	}

	hitRec := shadowCacheRecord(costW, costH, p, 12)
	if !shadowCacheHasRef(hitRec) {
		t.Fatalf("cost frame did not hit (commands: %v)", shadowCacheCommandTypes(hitRec))
	}
	_ = measure(hitRec) // warm

	bestHit, bestFull := time.Hour, time.Hour
	for i := 0; i < 3; i++ {
		if d := measure(hitRec); d < bestHit {
			bestHit = d
		}
		t.Setenv("GPUI_P43_BREAK", "no-cache")
		fullRec := shadowCacheRecord(costW, costH, p, 12)
		if shadowCacheHasRef(fullRec) {
			t.Fatal("no-cache mutation recorded a cached tile")
		}
		if d := measure(fullRec); d < bestFull {
			bestFull = d
		}
		t.Setenv("GPUI_P43_BREAK", "")
	}
	hit := bestHit - bestBase
	full := bestFull - bestBase
	if hit < 0 {
		hit = 0
	}
	if full <= 0 {
		t.Fatalf("baseline %v >= full chain %v (no shadow work to measure)", bestBase, bestFull)
	}
	if hit*4 >= full {
		t.Fatalf("cache hit shadow cost %v not < 1/4 of full chain %v (ratio %.3f; base=%v hitFrame=%v fullFrame=%v)",
			hit, full, float64(hit)/float64(full), bestBase, bestHit, bestFull)
	}
	t.Logf("shadow cache cost: base=%v hitFrame=%v fullFrame=%v => hitShadow=%v fullShadow=%v ratio=%.3f",
		bestBase, bestHit, bestFull, hit, full, float64(hit)/float64(full))
}
