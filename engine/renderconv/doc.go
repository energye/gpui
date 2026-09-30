//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package renderconv holds the only core<->render conversions.
//
// Frozen 2026-09-30 (E01): Vec2ToRenderPoint, Vec2FromRenderPoint,
// ColorToRender, ColorFromRender, Mat2DToRenderMatrix,
// Mat2DFromRenderMatrix. Moved verbatim from engine/core vec.go+color.go
// so core keeps zero external imports. Additive changes only.
//
// Rules:
//   - Field-for-field copy, no rounding, no clamping, no defaults.
//   - Game code converts once here before touching render types.
//   - Numbers stay frozen: boundary round trip is lossless.
package renderconv
