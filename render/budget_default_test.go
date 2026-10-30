//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"testing"
	"time"
)

// TestDefaultBudget_PinnedToPreP24bValues pins the declaration table to the
// audited pre-P2.4b constants (evidence/P2.4b/assessment.md §0). These
// numbers are behavior: the table switch must not move a single limit.
// Changing an expectation here is a deliberate budget decision (P3+), never
// a refactor side effect.
func TestDefaultBudget_PinnedToPreP24bValues(t *testing.T) {
	cases := []struct {
		d    BudgetDomain
		want Budget
	}{
		{BudgetProcessGlobal, Budget{MaxBytes: 768 << 20}}, // GPUI_VRAM_BUDGET_MB default
		{BudgetImageCache, Budget{MaxEntries: 128, MaxBytes: 64 << 20, MinEntries: 16, MinBytes: 8 << 20}},
		{BudgetPictureCache, Budget{MinEntries: 64}}, // divisor=4 lives in pictureCacheBudgetDivisor
		{BudgetTexturePool, Budget{MaxBytes: 128 << 20}},
		{BudgetStencilScratch, Budget{MaxEntries: 8}},
		{BudgetClipMask, Budget{MaxEntries: 24, MaxBytes: 16 << 20, TTL: 5 * time.Second}},
		{BudgetBlendDualTex, Budget{MaxEntries: 8, MaxBytes: 32 << 20}},
		{BudgetLayerOffscreen, Budget{MaxEntries: 4, MaxBytes: 32 << 20}},
		{BudgetGlyphColor, Budget{MaxEntries: 4}},                       // NewColorGlyphAtlas(1024, 4)
		{BudgetGlyphMask, Budget{MaxEntries: 16384, MaxBytes: 4 << 20}}, // 4 pages × 1024² R8
		{BudgetVideoStage, Budget{MaxEntries: 8, MaxBytes: 64 << 20}},
	}
	for _, c := range cases {
		got := DefaultBudget(c.d)
		if got != c.want {
			t.Errorf("%s: got %+v, want pinned %+v", c.d, got, c.want)
		}
	}
	if pictureCacheBudgetDivisor != 4 {
		t.Errorf("pictureCacheBudgetDivisor: got %d, want pinned 4", pictureCacheBudgetDivisor)
	}
	if got := DefaultBudget(BudgetDomain(99)); got != (Budget{}) {
		t.Errorf("unknown domain: got %+v, want zero Budget", got)
	}
}

// TestBudgetDomain_String locks the diagnostic names.
func TestBudgetDomain_String(t *testing.T) {
	names := map[BudgetDomain]string{
		BudgetProcessGlobal:  "process-global",
		BudgetImageCache:     "image-cache",
		BudgetPictureCache:   "picture-cache",
		BudgetTexturePool:    "texture-pool",
		BudgetStencilScratch: "stencil-scratch",
		BudgetClipMask:       "clip-mask",
		BudgetBlendDualTex:   "blend-dualtex",
		BudgetLayerOffscreen: "layer-offscreen",
		BudgetGlyphColor:     "glyph-color",
		BudgetGlyphMask:      "glyph-mask",
		BudgetVideoStage:     "video-stage",
	}
	for d, want := range names {
		if got := d.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", int(d), got, want)
		}
	}
}

// TestFairSplit_MatchesImageCacheArithmetic proves the unified fair-share
// helper reproduces the two pre-P2.4b derivations exactly:
//   - image cache: max(default/w, floor) for w>1, default untouched for w<=1
//   - picture cache: budgetBytes/w/divisor (floor 0 at byte level), then
//     entries with a floor
func TestFairSplit_MatchesImageCacheArithmetic(t *testing.T) {
	const def = 128
	const floor = 16
	// Single window: unchanged (image cache falls back to its default).
	if got := FairSplit(def, 1, floor); got != def {
		t.Errorf("FairSplit(w=1) = %d, want %d", got, def)
	}
	if got := FairSplit(def, 0, floor); got != def {
		t.Errorf("FairSplit(w=0) = %d, want %d", got, def)
	}
	// Even split above the floor.
	if got := FairSplit(def, 2, floor); got != 64 {
		t.Errorf("FairSplit(128,2,16) = %d, want 64", got)
	}
	// Split below the floor clamps to the floor.
	if got := FairSplit(def, 16, floor); got != floor {
		t.Errorf("FairSplit(128,16,16) = %d, want %d", got, floor)
	}
	if got := FairSplit(def, 32, floor); got != floor {
		t.Errorf("FairSplit(128,32,16) = %d, want %d", got, floor)
	}
	// Picture-cache byte derivation order: (bytes/w) then /divisor must
	// match the old budgetMB*MiB/windows/divisor integer arithmetic.
	const budgetBytes = int64(768) << 20
	for _, w := range []int64{2, 3, 4, 5, 8} {
		want := budgetBytes / w / pictureCacheBudgetDivisor
		got := FairSplit(budgetBytes, w, 0) / pictureCacheBudgetDivisor
		if got != want {
			t.Errorf("picture bytes w=%d: got %d, want %d", w, got, want)
		}
	}
}
