// Package sprite freezes the CPU-side sprite ordering every 2.5D draw feeds.
//
// Frozen 2026-09-15 (capability 2.3, P1b, S10/W1): LayerWorld, LayerFX,
// LayerUI, Item, NewItem, Sort, IsSorted. Additive changes only.
//
// Frozen 2026-09-15 (capability 2.4, P1b, S11/W1): LoopOnce, LoopLoop,
// LoopPingPong, Clip, EventKind (EventNone/EventFrame/EventLoop/
// EventFinished), Event, Flipbook, NewFlipbook, AddClip, Has, Count,
// OnEvent, Play, Current, IsPlaying, Update. Additive changes only.
//
// Frozen 2026-09-15 (capability 2.1, P1b, S31/W4): Sprite, NewSprite,
// Skippable, EffectiveOpacity, Batch, NewBatch, Add, Flush, Len,
// Skipped, Clear. Additive changes only. Same image submits once:
// Flush groups by image in first-seen order and calls emit once per
// image. Pure numbers only; the caller draws each group with the
// existing render DrawAtlas (Src.X/Y/W/H, Dst.X/Y/W/H, EffectiveOpacity
// field for field), render main path untouched.
//
// Frozen 2026-09-15 (capability 2.2, P1b, S36/W5): AtlasFilter
// (AtlasFilterDefault/Nearest/Bilinear/Bicubic), ParseAtlasFilter,
// AtlasSprite, NewAtlasSprite, Validate, Skippable, HasTint, PivotPoint,
// FeetPivot, ToRender, AtlasToRender. Additive changes only. Rotation is
// radians about the pivot (Y down, positive clockwise, matching render R4);
// FlipX/FlipY mirror about the pivot before Rot; Pivot is the offset from
// the Dst origin in Dst units (zero is top-left, FeetPivot pins the feet);
// Tint zero struct means no tint, else straight multiply incl alpha;
// Filter zero means the historic bilinear road, per-sprite
// nearest/bilinear/bicubic each on its own. Pure numbers plus one boundary
// conversion; the caller draws the result with the existing render
// DrawAtlasEx (field for field), render main path untouched. Window intent
// game_sprite--case=rot lands with P2; S36 keeps the offscreen proof only.
// Bad numbers are core InvalidArg and change nothing.
//
// Frozen 2026-09-15 (capability 1.2, P1b, S41/W5): DeepItem, NewDeepItem,
// SetDepth, DepthSort, IsDepthSorted. Additive changes only. Depth follows
// R5/game/camera: bigger = farther, drawn first (far-to-near painter);
// ties keep the input order (stable, no flicker). Within the same depth,
// Layer/FeetY keep the 2.3 rule. Pure numbers; the caller feeds the ordered
// slice to render depth branch (DrawDepthSprites) or plain atlas draws.
// Bad depth (NaN/Inf) is a core InvalidArg error and changes nothing.
//
// Ordering (draw first = behind, draw last = front on top):
//
//	layer ascending, then feet Y ascending, stable for ties.
//
// Layer bands: world 0 (people, trees, ground), fx 1 (fire, smoke, trails),
// ui 2 (buttons, tips). A bigger layer always covers a smaller one, so an
// effect never drifts over the interface no matter its Y. Custom layers are
// plain integers between or outside the bands; the rule stays the same.
//
// Feet Y is the foot bottom in world units: smaller Y stands behind and is
// drawn first, bigger Y stands in front and is drawn later. Same layer and
// same Y keeps the input order (stable), so equal heights never flicker.
//
// Only core numbers are used. This package draws nothing; the caller draws
// the returned slice in order with the existing render draws. Bad feet Y
// (NaN/Inf) is a core InvalidArg error and changes nothing.
package sprite
