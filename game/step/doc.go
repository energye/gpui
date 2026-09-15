// Package step freezes the CPU-side step helpers every 2.5D loop feeds.
//
// Frozen 2026-09-15 (capability 8.2, P0, S12/W1): Pool, NewPool, Stats,
// GetVec, PutVec, GetColor, PutColor, GetFloat, PutFloat,
// RetainedBytes, HitRate, ResetStats, MaxLen, MaxRetained, VecBytes,
// ColorBytes, FloatBytes. Additive changes only.
//
// Frozen 2026-09-15 (capability 8.1+11.1, P0, S03/W1): Fixed, NewFixed,
// Dt, Steps, Elapsed, Accum, Alpha, Advance, Reset, Interp, InterpFloat,
// InterpVec, MaxFrame. Additive changes only.
//
// Fixed-step (8.1 logic beat plus 11.1 render blend in one file, tested
// together): the caller feeds each real frame into Advance, runs the
// returned number of fixed logic ticks, then blends the last two logic
// states with Alpha for the picture. Only core numbers are used; the
// caller converts once at the render boundary with Vec2.ToRenderPoint.
// No Vec2, Color, or AssetID is redefined here.
//
// Frozen 2026-09-15 (capability 11.2, P0, S18/W2): Clock, NewClock,
// MaxScale, SetTimeScale, TimeScale, SetPaused, Paused, Pause, Resume,
// Split, Advance, WorldElapsed, UIElapsed, Reset. Additive changes only.
//
// Layered time (11.2, Godot time-scale): one real frame splits into a
// scaled world delta and an unscaled UI delta. Pause stops the world
// only; the UI keeps moving. The frame clamps to [0, MaxFrame] first
// (shared with Fixed); world may exceed MaxFrame at scale > 1 and the
// caller feeds it into Fixed.Advance which clamps again. Bad scales
// clamp to [0, MaxScale] and never error: NaN/zero/negative park at 0.
//
// Math (Fix Your Timestep, Gaffer classic):
//
//	frame clamped to [0, MaxFrame] first (negative parks at 0, huge
//	clamps to MaxFrame so a hitch never spirals into hundreds of ticks).
//	accum += frame; while accum >= dt: accum -= dt, steps++.
//	alpha = accum / dt in [0, 1).
//	picture = prev + (curr - prev) * alpha.
//
// The ledger is integer milliseconds (core.Duration), so a million ticks
// land on the exact millisecond and slow and fast machines agree. Dt never
// changes mid-run. Interp clamps its alpha to [0, 1] first (NaN parks at
// prev); the blended ends pass through untouched, like anim.Lerp.
package step
