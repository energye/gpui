// Package anim freezes the CPU-side motion math every 2.5D play feeds.
//
// Frozen 2026-09-15 (capability 4.1, P0, S04/W1): Kind, Ease, Lerp,
// Name, Parse, Valid. Additive changes only.
//
// Frozen 2026-09-15 (capability 4.2, P1b, S17/W2): LoopMode
// (LoopOnce/LoopLoop/LoopPingPong), Key{At,Value,Ease}, Event{At,Name},
// Timeline{NewTimeline,SetLoop,AddKey,AddEvent,Sample,Update,OnEvent,
// Seek,Pos,IsPlaying,Pause,Resume,TrackNames,KeyCount,EventCount}.
// Time is core.Duration (integer milliseconds); values are plain float64
// sampled with the frozen 4.1 curves. Additive changes only.
//
// Frozen 2026-09-15 (capability 4.3, P2 long chain, S42/W6): Skeleton,
// Bone, Slot, Skin, Attachment, VertexWeight, IKConstraint,
// TransformConstraint, BoneLocal, Pose, BlendMode
// (BlendReplace/BlendAdditive), CurrentSkeletonVersion, ParseSkeleton,
// LoadSkeletonFile, NewPose. Additive changes only.
//
// What is frozen (Spine JSON subset, Godot Skeleton2D math): bones
// (name, parent, x, y, rotation, scale, shear, length, color), slots
// (name, bone, attachment, blend, color), skins (region plus mesh
// attachments, uvs, triangles, hull, per-vertex bone weights), draw
// order (slots order plus SetDrawOrder permutation), IK (one-bone aim
// plus two-bone analytic with bend and mix) and transform constraints
// (mix rotate/translate/scale/shear). Pose holds per-bone local
// overrides plus world matrices; SkinVertices blends mesh vertices in
// bone space; BlendAdditive is the mix entry the S46 state machine
// will drive. Only core numbers are used (Vec2, Mat2D, Color,
// AssetID, Version); render types are reached through the core
// ToRender helpers at the boundary, never imported here.
//
// What is reserved: physics inertia (JSON "physics" plus ApplyPhysics)
// and path constraints plus linkedmesh/boundingbox/path/point/clipping
// attachments report core Unsupported, never a guessed pose. Spine
// 3.x and 4.x both load when the major is 3 or 4; any other major
// reports VersionMismatch.
//
// Errors use game/core codes: empty path is InvalidArg, a missing file
// is NotFound, torn JSON is BadData, an unfrozen kind is Unsupported,
// a payload with a foreign major is VersionMismatch, an unknown
// bone/slot/skin/attachment is NotFound, a bad draw order or NaN/Inf
// number is InvalidArg. A nil Skeleton or Pose never panics.
//
// This package only computes numbers; it draws nothing. The caller feeds
// the eased progress into existing draws (positions, alpha, scales).
// Only core errors are used; no new Vec2/Color/AssetID is defined here.
//
// Why a new library when ui/animation already has curves:
// ui/animation owns interface animation (controller plus 4 curves for
// opacity and widgets). Game play needs the full Godot-style family
// (sine, quad, cubic, quart, quint, expo, circ, back, bounce, elastic
// each in, out, in-out plus linear) with frozen string names so timeline
// keys (capability 4.2) and asset files can name a curve and replay it.
// The two packages stay separate on purpose and never import each other.
//
// Math (Godot Tween TransitionType plus EaseType, Penner analytic form;
// Flutter Curves only supplies the cubic-bezier subset, so Godot is the
// primary reference):
//
//	t is clamped to [0,1] first (NaN maps to 0, infinities clamp).
//	Clamped 0 returns 0 and clamped 1 returns 1 exactly for every kind,
//	so expo and elastic never drift at the ends. Unknown kinds fall back
//	to linear (the clamped t) and never panic. Back and elastic may leave
//	[0,1] mid-flight by design (overshoot); every other family stays in
//	[0,1] and rises monotonically.
package anim
