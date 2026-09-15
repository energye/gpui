// Package particle freezes the CPU-side emission math every 2.5D effect feeds.
//
// Frozen 2026-09-15 (capability 5.1, P3, S38/W5): ShapeKind,
// ShapePoint/ShapeCone/ShapeBox/ShapeRing, String, ParseShape, Shape,
// PointShape, ConeShape, BoxShape, RingShape, Validate, Turbulence,
// NoTurbulence, NewTurbulence, Vec, Sub, NoSub, NewSub, EmitterConfig,
// MaxParticlesCap, MaxSubPerDeath, MaxRate, MaxSpeed, Particle, AgeFrac,
// Alive, ColorAt, Emitter, NewEmitter, Config, Spawn, Update, Alive,
// Spawned, Died, ChildSpawned, Particles, Clear, ColorOf, AppendToBatch.
// Additive changes only.
//
// This package only computes numbers; it draws nothing. The caller feeds
// AppendToBatch into the existing sprite Batch (game/sprite) and draws
// each flushed group with the existing render DrawAtlas. Only core
// numbers are used; no Vec2, Color, or AssetID is redefined here.
//
// Emission (main reference Godot CPUParticles2D, Go replay): one emitter
// owns a fixed budget (Max), a spawn rate, speed/life ranges, gravity,
// a start-to-end color ramp, one shape, one turbulence, and one
// one-level sub-emitter. Spawn samples position and direction from the
// shape, speed and life from their ranges, and a replay seed from the
// seeded generator. Update integrates existing particles first, then
// births the rate share, so newborns start at Age 0 in the same tick.
// Deaths spawn Sub.Count children at the death spot when the parent is
// not itself a child; children never spawn further.
//
// Shapes: point spawns at Origin with Dir; cone adds a uniform spread
// in [-Angle, +Angle] around Dir; box samples uniformly in
// [-Extents, +Extents] with Dir; ring samples uniformly in the annulus
// [Inner, Outer] with a radial outward direction. Turbulence is a
// deterministic swirl from (Seed, Age): Vec returns Strength-scaled
// sin/cos, so the same seed replays the same drift without consuming
// the generator per tick. Color is Start.Lerp(End, AgeFrac).
//
// Errors use game/core codes: bad constructor arguments are InvalidArg
// and store nothing; Max beyond MaxParticlesCap is OutOfMemory. Spawn
// beyond the budget drops the excess quietly (no error) so bursts never
// crash the frame. Nil receivers never panic: getters park at zero,
// writers report InvalidArg. Window intent: game_particle --case=fire
// (fire cone plus drifting smoke); pure math stays offscreen golden.
//
// Frozen 2026-09-15 (capability 5.2, P3, S43/W6): JointKind,
// JointMiter/JointBevel/JointRound, MiterLimit, RoundArcSteps,
// MaxTrailPointsCap, String, ParseJoint, TrailConfig, NewTrailConfig,
// Validate, Trail, NewTrail, Config, Len, Points, Push, Clear, WidthAt,
// ColorAt, TrailVert, Segments, TrailSegment, Verts, AppendToBatch,
// MaxPoolTrailsCap, GPUPool, NewGPUPool, Config, TrailConfig, Alive,
// Spawned, Died, ChildSpawned, Trails, TotalPoints, Particles, ColorOf,
// TrailOf, Clear, Spawn, Update, AppendToBatch.
//
// Trail (main reference Godot Line2D): one trajectory ring oldest to
// newest. Width lerps TailWidth (oldest) to HeadWidth (newest) so a knife
// slash reads wide-to-narrow; color lerps Start (head) to End (tail).
// Miter extends sharp corners up to MiterLimit then cuts to bevel, bevel
// cuts flat with one pair, round fans RoundArcSteps interior pairs.
// Verts carries the exact joint geometry; Segments carries the drawable
// center line (one rotated atlas sprite per segment through the existing
// sprite AtlasToRender plus DrawAtlasEx, no new submit path). GPUPool
// (main reference Godot GPUParticles2D) owns one 5.1 Emitter for positions
// plus one Trail ring per live particle keyed by birth Seed; Spawn/Update
// semantics are the frozen 5.1 ones. Window intent:
// game_particle --case=trail (knife slash wide to narrow).
package particle
