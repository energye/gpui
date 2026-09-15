// Package fx freezes the CPU-side full-screen effect math every 2.5D play feeds.
//
// Frozen 2026-09-15 (capability 5.3, P3, S39/W5): Noise, Warp, NewWarp,
// OffsetAt, WarpPoint, SampleClamped, WarpRGBA, SetStrength, SetNoise,
// MaxWarpPixels. Additive changes only.
//
// What it does: heat-haze and water wobble sample the picture at a
// noise-shifted position instead of the raw pixel. Warp only computes the
// shift; the caller applies it to a fresh intermediate copy, never to the
// live frame, and the render main path stays untouched.
//
// Only core numbers are used (core.Vec2, core.Rect, core codes). This
// package draws nothing; the caller feeds WarpPoint into the existing
// render draws. No new Vec2/Color/AssetID is defined here.
//
// Math (frozen, both backends share it): 2D value noise over an integer
// lattice, splitmix64 hash of (ix, iy, seed, channel), smoothstep blend,
// mapped to [-1, 1] per channel. Offset(p, t) = strength * noise(p* freq
// + drift*t), one channel per axis with a different channel salt. Pure
// function of its inputs: same inputs replay bitwise identical on every
// machine, so the CPU and GPU sides agree by construction.
//
// Errors use game/core codes: bad constructor and wiring arguments are
// InvalidArg, a pixel slice whose length disagrees with its size is
// BadData, a picture beyond MaxWarpPixels is OutOfMemory. The per-pixel
// hot path OffsetAt stays total instead: strength 0 (including the zero
// Warp) is the exact identity, and any non-finite input yields the zero
// offset, never a panic and never a NaN leaking downstream.
//
// Frozen 2026-09-15 (capability 5.5, P3, S44/W6): Custom, NewCustom,
// NewIdentity, NewOutline, NewDissolve, Registry, MaxCustomNameLen,
// MaxCustomParams, MaxCustomOutlineWidth, MaxCustomSeed, MaxCustomSlots,
// CustomIdentity, CustomOutline, CustomDissolve. Additive changes only.
//
// What it does: artist hooks for outline and dissolve. Custom holds a
// name plus float params; Registry attaches hooks to handles and calls
// them by handle. Apply takes an Image and returns a fresh Image plus a
// degraded bit plus an error: identity is exact (degraded false),
// outline and dissolve are CPU reference paths the GPU replaces with a
// filtered shader (degraded true). A valid src with a bad hook still
// returns a placeholder copy so the main path never breaks. Only core
// numbers and core codes are used; render is never imported here.
package fx
