// Package physics freezes the CPU-side collision math every 2.5D play feeds.
//
// Frozen 2026-09-15 (capability 14.1a, P0, S06/W1): Shape, ShapeBox,
// ShapeCircle, Body, NewBox, NewCircle, Valid, Bounds, CanCollide,
// Overlaps, Contact, Query. Additive changes only.
//
// This package only computes numbers; it draws nothing and steps nothing.
// The caller moves bodies (assign Pos or rebuild via NewBox/NewCircle),
// calls Query, then resolves solids and fires triggers. Rays and riding
// platforms belong to 14.1b and stay out: no Ray type here.
// Only core numbers are used; no new Vec2/Rect is defined here.
//
// Shapes: box is a center plus half extents, circle is a center plus a
// radius. Pos is the center in world units. Half must be finite with
// X/Y >= 0 (zero is a point, negative is rejected). Radius must be
// finite and >= 0 (zero is a point). Non-finite or negative sizes are a
// core InvalidArg error and store nothing.
//
// Layers (Godot collision_layer/mask idea): Layer says which group this
// body sits on, Mask says which groups it scans. Both are plain bit
// masks; 0 scans or matches nothing. CanCollide is true only when each
// side scans the other: (a.Layer&b.Mask) != 0 && (b.Layer&a.Mask) != 0.
// Overlapping shapes on mismatched masks never meet in Query.
//
// Trigger: true means sensor (coin, door zone, jump probe). Query still
// reports the pair with Trigger set; the caller must not push solids out
// of a trigger. Solid pairs report Trigger false.
//
// Overlap rule (inclusive, replay-stable, no epsilon): edge touch counts
// as overlap so standing exactly on a platform still touches. Box-box
// compares |dx| <= hx sum and |dy| <= hy sum; circle-circle compares
// distSq <= (r1+r2)^2; box-circle clamps the circle center to the box
// and compares distSq <= r^2. Invalid bodies (bad numbers, unknown
// shape) never overlap: Overlaps reports false, Bounds reports ok=false,
// Query reports a core InvalidArg error with a nil result.
//
// Query order: pairs i<j in input order, never sorted, never mutated.
// The input slice is never mutated; the result is a fresh slice (nil
// when nothing touches). Bad inputs fail closed, never guessed.
package physics
