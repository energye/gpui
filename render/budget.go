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

import "time"

// BudgetDomain names one GPU-side budget family. P2.4b 预算统一: every
// GPU-side cache/pool limit is declared here once (按域分账, declaration
// level); the individual sites read their defaults through DefaultBudget
// instead of private constants. Runtime per-domain usage aggregation lands
// with the P2.4c/d res/registry unification — the hal process ledger
// (VramAdd/VramForget/VramCheck) stays the single live byte account.
type BudgetDomain int

const (
	// BudgetProcessGlobal is the hal process VRAM ledger domain. The table
	// entry documents the default; the live value is always read through
	// VramBudgetMB (GPUI_VRAM_BUDGET_MB, 0 disables the gate).
	BudgetProcessGlobal BudgetDomain = iota
	// BudgetImageCache is the per-session cached image-texture LRU
	// (render/internal/gpu ImageCache). MaxEntries = textures, MaxBytes =
	// resident texture bytes; MinEntries/MinBytes are the multi-window
	// fair-share floors.
	BudgetImageCache
	// BudgetPictureCache is the per-window picture texture cache
	// (ui/scene PictureTextureCache). Fair-share derivation: the process
	// budget splits by live windows and pictureCacheBudgetDivisor, then
	// converts to entries with the MinEntries floor. MaxEntries is
	// unused (the cache auto-sizes to its frame working set).
	BudgetPictureCache
	// BudgetTexturePool is the deprecated render-target pool kept for
	// purge/telemetry until P2.4d removes it. MaxBytes = pool budget in
	// bytes (the site reads MB via MaxBytes>>20; MaxEntries unused).
	BudgetTexturePool
	// BudgetStencilScratch is the size-bucketed sc==1 depth/stencil cache
	// on each session textureSet. MaxEntries = size buckets.
	BudgetStencilScratch
	// BudgetClipMask is the depth-clip coverage-mask cache. MaxEntries =
	// masks, MaxBytes = mask bytes, TTL = unused-entry lifetime.
	BudgetClipMask
	// BudgetBlendDualTex is the advanced-blend BGRA intermediate pool.
	// MaxBytes = total pool bytes; MaxEntries = per-size-bucket cap.
	BudgetBlendDualTex
	// BudgetLayerOffscreen is the per-context layer/effect offscreen RT
	// pool. MaxBytes = total pool bytes; MaxEntries = per-size-bucket cap.
	BudgetLayerOffscreen
	// BudgetGlyphColor is the color-glyph atlas. MaxEntries = atlas pages
	// (each page is PageSize² RGBA at the construction site).
	BudgetGlyphColor
	// BudgetGlyphMask is the R8 glyph-mask atlas (render/text). MaxEntries
	// = cached glyph masks; MaxBytes = MaxAtlases pages of PageSize² R8.
	BudgetGlyphMask
	// BudgetVideoStage is the video texture pool (routing slots) plus the
	// single-upload staging cap. MaxEntries = pool slots; MaxBytes = one
	// staging upload cap.
	BudgetVideoStage
)

func (d BudgetDomain) String() string {
	switch d {
	case BudgetProcessGlobal:
		return "process-global"
	case BudgetImageCache:
		return "image-cache"
	case BudgetPictureCache:
		return "picture-cache"
	case BudgetTexturePool:
		return "texture-pool"
	case BudgetStencilScratch:
		return "stencil-scratch"
	case BudgetClipMask:
		return "clip-mask"
	case BudgetBlendDualTex:
		return "blend-dualtex"
	case BudgetLayerOffscreen:
		return "layer-offscreen"
	case BudgetGlyphColor:
		return "glyph-color"
	case BudgetGlyphMask:
		return "glyph-mask"
	case BudgetVideoStage:
		return "video-stage"
	default:
		return "unknown"
	}
}

// Budget is one declared limit set. A zero field means that dimension is
// unbounded / not applicable for the domain; the meaning of each field is
// documented on its BudgetDomain constant.
type Budget struct {
	MaxEntries int
	MaxBytes   int64
	// MinEntries/MinBytes are the multi-window fair-share floors (0 =
	// none); only meaningful where a fair share is derived.
	MinEntries int
	MinBytes   int64
	TTL        time.Duration
}

// pictureCacheBudgetDivisor splits the process budget across its big
// consumers: swapchain surfaces, the picture texture cache, the image
// cache, and staging/vertex buffers (moved here from shared_budget.go,
// P2.4b 预算统一 — single declaration point).
const pictureCacheBudgetDivisor = 4

// DefaultBudget returns the declared defaults for one budget family. The
// numbers are pinned by budget_default_test.go to the pre-P2.4b values —
// switching a site to this table must not move a single limit (P2 zero
// behavior); changing a limit is a deliberate P3+ decision, not a refactor
// side effect.
func DefaultBudget(d BudgetDomain) Budget {
	switch d {
	case BudgetProcessGlobal:
		// gpu/hal/vram_ledger.go vramBudgetMB default (768); live reads go
		// through hal.VramBudgetMB / VramBudgetMB.
		return Budget{MaxBytes: 768 << 20}
	case BudgetImageCache:
		return Budget{MaxEntries: 128, MaxBytes: 64 << 20, MinEntries: 16, MinBytes: 8 << 20}
	case BudgetPictureCache:
		// Fair-share floor for the derived per-window entry cap.
		return Budget{MinEntries: 64}
	case BudgetTexturePool:
		// MB semantics; MaxBytes carries 128 MiB, the site reads MB via
		// MaxBytes>>20.
		return Budget{MaxBytes: 128 << 20}
	case BudgetStencilScratch:
		return Budget{MaxEntries: 8}
	case BudgetClipMask:
		return Budget{MaxEntries: 24, MaxBytes: 16 << 20, TTL: 5 * time.Second}
	case BudgetBlendDualTex:
		return Budget{MaxEntries: 8, MaxBytes: 32 << 20}
	case BudgetLayerOffscreen:
		return Budget{MaxEntries: 4, MaxBytes: 32 << 20}
	case BudgetGlyphColor:
		// MaxEntries = atlas pages (NewColorGlyphAtlas(1024, 4)).
		return Budget{MaxEntries: 4}
	case BudgetGlyphMask:
		// MaxEntries = cached glyph masks; MaxBytes = 4 pages × 1024² R8.
		return Budget{MaxEntries: 16384, MaxBytes: 4 << 20}
	case BudgetVideoStage:
		return Budget{MaxEntries: 8, MaxBytes: 64 << 20}
	default:
		return Budget{}
	}
}

// FairSplit divides a per-process cap across live shared-device windows
// with a floor: max(total/windows, floor). windows<=1 returns total
// unchanged (single-window behavior is preserved exactly). This is the one
// fair-share arithmetic — the image cache (entries and bytes) and the
// picture cache (bytes) both derive through it.
func FairSplit(total, windows, floor int64) int64 {
	if windows <= 1 {
		return total
	}
	if fair := total / windows; fair >= floor {
		return fair
	}
	return floor
}
