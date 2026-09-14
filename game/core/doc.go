// Package core freezes the shared value types every game/* package builds on.
//
// Frozen 2026-09-14 (capability 0.0, P0): Vec2, Rect, Mat2D (vec.go),
// Color (color.go), Duration, Step (time.go), Rand (rand.go), AssetID,
// Handle, Manager (asset.go), Code, Error (result.go), Version (version.go).
//
// Rules for everything under game/:
//   - Reuse these types; do not redefine Vec2, Color, or AssetID elsewhere.
//   - render's types stay untouched; convert once at the render boundary
//     with the ToRender*/FromRender* helpers.
//   - Additive changes only. A breaking change bumps the data Version and
//     keeps the old call path compiling.
package core
