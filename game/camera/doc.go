package camera

// Projector turns world positions plus depth into screen positions.
//
// Frozen 2026-09-14 (capability 1.1, P1b): NewProjector, DepthToScale,
// Project, ProjectQuad, Unproject. Additive changes only.
//
// The old 6-number matrix stays untouched. This helper only computes
// numbers on the CPU; the caller feeds the resulting screen corners to
// the existing trapezoid/triangle draws (render.DrawImageQuad,
// DrawVertices/DrawMesh). CPU and GPU therefore receive identical
// screen corners, so the two pictures differ only by edge smoothing.
//
// Math (pinhole in front of the screen plane):
//
//	scale = Focal / (Focal + depth)
//	screen = Center + (world - Camera) * scale
//
// Depth 0 sits on the screen (scale 1). Positive depth goes away and
// shrinks; negative depth within (-Focal, 0) comes closer and grows.
// Depth at or behind the eye (Focal+depth <= 0) is not drawable and
// reports ok=false instead of NaN/Inf. NaN/Inf inputs also report
// ok=false and never panic.

// Camera is the pure-math 2D lens (capability 1.3a, P0).
//
// Frozen 2026-09-15: NewCamera, SetViewport, SetPos, SetZoom, SetRotation,
// SetShake, DecayShake, SetLimit, SetSmoothing, SetAnchor, Follow,
// EffectivePos, View, WorldToScreen, ScreenToWorld, VisibleWorldRect,
// MinZoom. Additive changes only.
//
// State: Pos center, Zoom scale (>= MinZoom 1e-6), Rotation radians
// normalized to [-pi, pi], Shake caller-driven offset, Limit center clamp
// box (empty disables), Smoothing Follow factor (0 snaps, (0,1] eases),
// Anchor screen fraction ((0,0) top-left, (0.5,0.5) centered), Viewport
// screen size. Only core numbers are used; render types convert once at
// the boundary with Mat2D.ToRenderMatrix.
//
// Math:
//
//	eff = clamp(Pos) + Shake
//	View = Translate(Anchor*Viewport) * Rot(-Rotation) * Scale(Zoom)
//	       * Translate(-eff)
//
// Limits pin the center; shake may peek outside for a frame by design.
// Finite-but-out-of-range inputs clamp (zero/negative zoom to MinZoom,
// anchor to [0,1], viewport negatives to 0, smoothing to [0,1]);
// NaN/Inf inputs are core InvalidArg errors and change nothing.
