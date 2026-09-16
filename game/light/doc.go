// Package light freezes the CPU-side 2D light math every night scene feeds.
//
// Frozen 2026-09-15 (capability 6.1, P3, S45/W6): Kind, Cookie, Light,
// Image, Scene and every constructor below. Additive changes only.
//
// What it does: point and directional lights modulate a picture after the
// fx grade. A point light falls off with distance, is shaped by an
// optional cookie (flashlight mask), and only touches the layers in its
// mask; a directional light touches its layers uniformly. The global
// night color always applies, so zero lights still show the night picture
// and never a black screen. Light only computes numbers; the caller draws
// the returned image with the existing render draws, render main path
// untouched (Godot PointLight2D + CanvasModulate idea, Go reworked).
//
// Only core numbers are used (core.Vec2, core.Color, core codes). No new
// Vec2/Color/AssetID is defined here.
//
// Math (frozen, both backends share it): point attenuation is the Hermite
// smoothstep of t = 1 - dist/range (1 at the bulb, 0 at and past the
// edge); cookie is bilinear over the light's bounding square, nil or
// corrupt cookie reads 1 (missing slide falls back to bare bulb); layer
// miss reads 0; directional reads 1 on a hit layer. One pixel is
// base*night + base*sum(factor*intensity*lightRGB), clamped to [0,1],
// alpha rides through untouched. Pure function of its inputs: same inputs
// replay bitwise identical on both backends.
//
// Errors use game/core codes: bad constructors are InvalidArg, a pixel
// slice whose length disagrees with its size is BadData, a picture beyond
// MaxImagePixels or a scene beyond MaxLights is OutOfMemory/InvalidArg as
// documented. The per-pixel hot path (FactorAt, LitCPU/LitGPU) stays total
// instead: any non-finite input yields 0 contribution, never a panic and
// never a NaN leaking downstream.
//
// Frozen 2026-09-16 (capability 6.3, P3, S48/W7): Occluder, Shadow, and
// every constructor below. Additive changes only.
//
// What shadows do: each occluder is a closed polygon loop; a pixel is in
// the lee when a wall edge crosses the ray strictly between the light and
// the pixel (point light) or between the pixel and the source along -Dir
// over Length (directional light, Dir is the ray travel direction). The
// lee multiplier is 1-Occlusion, so a fully shadowed pixel keeps
// base*night and never goes black; alpha rides through untouched. Wall
// faces stay lit on purpose (endpoint touches never block). Zero Length
// casts nothing (valid no-op); empty sets, corrupt loops, non-finite
// geometry, zero Dir, and unknown kinds read unblocked, never a panic. Both backends share one path
// (Blocked/FactorFor/Lit/Apply and their GPU mirrors), so CPU and GPU
// agree bitwise by construction; errors reuse the same game/core codes
// (bad occluder InvalidArg/OutOfMemory, bad shadow InvalidArg, nil image
// InvalidArg, corrupt image BadData).
//
// Frozen 2026-09-16 (capability 6.2, P3, S47/W7): Normal, NormalMap,
// NormalScene, and every constructor below. Additive changes only.
//
// What normal-mapped light does: a bump sheet (NormalMap, 1:1 with the lit
// Image) tilts each pixel's share of the 6.1 light. The lamp hangs Height
// world units above the plane; LightDir points from the surface toward the
// lamp (bulb position for point lights, Dir for directional lights, so 6.3
// shadows extend along -Dir). One pixel is base*night +
// base*sum(factor*cosine*intensity*lightRGB) where factor is the frozen
// 6.1 geometry and cosine is NdotL (both unit inside, negative clamped to
// 0). Facing the lamp beats flat, flat beats facing away, the lee keeps
// base*night and never goes black; alpha rides through untouched.
//
// Missing tilt reads as flat (zero/degenerate normals fall back to
// straight-up, never NaN); a nil map to Apply is InvalidArg, a corrupt or
// mis-sized map is BadData, over-budget is OutOfMemory, bad height is
// InvalidArg, and the hot path (NormalFactor, LitNormalCPU/LitNormalGPU)
// stays total. Both backends share one path, so CPU and GPU agree bitwise
// by construction. Light only computes numbers after the fx grade; render
// main path untouched (game-engine normal-map idea, Go reworked).
package light
