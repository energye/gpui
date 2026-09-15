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
package light
