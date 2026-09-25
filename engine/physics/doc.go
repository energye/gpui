// Package physics freezes the CPU-side collision math every 2.5D play feeds.
//
// Frozen 2026-09-15 (capability 14.1a, P0, S06/W1): Shape, ShapeBox,
// ShapeCircle, Body, NewBox, NewCircle, Valid, Bounds, CanCollide,
// Overlaps, Contact, Query. Additive changes only.
// Frozen 2026-09-15 (capability 14.1b, P2, S21/W2): Ray, NewRay, Hit,
// CastRay, IsStandingOn, CarryRider. 14.1a overlap semantics unchanged.
// Frozen 2026-09-15 (capability 14.2, P2, S22/W2): Slope, NewSlope,
// NewOneWay, GroundYAt, Angle, Walkable, SlideDir, FeetOf, WithFeet,
// Grounded, Step. 14.1a/b semantics unchanged.
//
// This package only computes numbers; it draws nothing and steps nothing.
// The caller moves bodies (assign Pos or rebuild via NewBox/NewCircle),
// calls Query, then resolves solids and fires triggers. 14.1b adds rays
// and riding platforms: NewRay/CastRay for jump probes and bullet lines,
// IsStandingOn/CarryRider for a platform step carrying its rider.
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
//
// Ray rule (14.1b, probe and bullet line): NewRay needs a finite origin,
// a finite non-zero dir, and a finite MaxDist >= 0, else InvalidArg.
// CastRay takes the nearest body with a layer bit in the ray mask
// (one-sided: body.Layer&ray.Mask != 0; the body mask is ignored, unlike
// Query which needs both sides). Box uses the slab test, circle the
// quadratic test; edge touch and tangent count, origin inside reports
// Dist 0 with a zero normal. Dist runs along the normalized dir in world
// units and must sit within [0, MaxDist]. Ties keep the smaller input
// index. Empty input or Mask 0 reports no hit without error. An invalid
// ray or any invalid body is InvalidArg. Tilemap solid cells feed this
// path as boxes: cell center with half tile/2, no tilemap import here.
//
// Platform rule (14.1b, moving platform carries its rider): IsStandingOn
// is true only when both bodies are valid, the rider center sits at or
// above the platform center, the rider bottom sits within 1e-9 of the
// platform top, and the horizontal spans overlap edge-included (circles
// use their bounds footprint). CarryRider applies the platform step delta
// to rider.Pos only in that pre-move standing pose, else leaves rider
// untouched and reports carried=false. Nil rider, invalid bodies,
// non-finite delta, or a delta pushing rider off finite numbers is
// InvalidArg with rider untouched.
//
// Slope rule (14.2, platform and slope): thin segment A-B with a one-way
// flag. +Y runs down like tilemap cells; above means smaller Y. NewSlope
// is solid, NewOneWay is top-only; both need finite distinct endpoints
// else InvalidArg. GroundYAt interpolates Y at column x inside the
// inclusive span (vertical reports ok=false). Angle is radians from
// horizontal in [0, pi/2]. Walkable checks angle <= caller maxAngle.
// SlideDir is the downhill unit vector (flat reports ok=false, vertical
// reports (0,1)). FeetOf/WithFeet convert a Body center to the sole point
// and back, read-only over body.go. Grounded samples the highest surface
// within 1e-9 at the feet column, invalid slopes skipped. Step integrates
// feet+vel*dt: falling still lands on the highest crossed surface, rising
// along the same slope stays grounded, a solid slope crossed from below
// reports hitHead clamped to the ceiling, one-way never reports head.
// Any invalid slope in Step is InvalidArg; bad feet/vel/dt is InvalidArg
// with feet unchanged. Tilemap feeds this path as cell top edges, no
// tilemap import here. Window intent: game_physics --case=jump.
package physics
