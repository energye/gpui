//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package core freezes the shared value types every engine/* package builds on.
//
// Frozen 2026-09-14: Vec2, Rect, Mat2D (vec.go),
// Color (color.go), Duration, Step (time.go), Rand (rand.go), AssetID,
// Handle, Manager (asset.go), Code, Error (result.go), Version (version.go).
//
// Rules for everything under engine/:
//   - Reuse these types; do not redefine Vec2, Color, or AssetID elsewhere.
//   - render's types stay untouched; convert once at the render boundary
//     with the ToRender*/FromRender* helpers.
//   - Additive changes only. A breaking change bumps the data Version and
//     keeps the old call path compiling.
package core
