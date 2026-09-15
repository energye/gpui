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
