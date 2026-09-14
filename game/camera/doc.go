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
