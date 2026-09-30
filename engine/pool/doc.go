//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package pool reuses core number buffers across frames: Vec2 for path
// points and vertex positions, Color for vertex colors, float64 for raw
// streams.
//
// Frozen 2026-09-15: Pool, NewPool, Stats,
// GetVec, PutVec, GetColor, PutColor, GetFloat, PutFloat,
// RetainedBytes, HitRate, ResetStats, MaxLen, MaxRetained, VecBytes,
// ColorBytes, FloatBytes. Moved from engine/step in 2026-09-30 (E03);
// step keeps only fixed/time. Additive changes only.
//
// Put transfers ownership to the pool; the caller must not use the slice
// after Put. Get returns len n with undefined contents; the caller must
// overwrite the first n elements before drawing. A nil Pool never panics:
// Get reports InvalidArg, Put and ResetStats are no-ops, Stats is zero.
package pool
