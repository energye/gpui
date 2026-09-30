//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package dirty tracks per-frame repaint boxes over core numbers.
//
// Frozen 2026-09-15: MaxDirtyRects,
// DirtyStats, DirtyTracker, NewDirtyTracker, NewSpriteDirtyTracker,
// DirtyForMove. Moved from engine/step in 2026-09-30 (E03). Additive
// changes only.
//
// Game-side twin of ui/scene DirtyLayer: pure core numbers, no render
// import. The loop marks each moved sprite's old-plus-new union, leaves
// still sprites unmarked so the sprite layer updates independently, and
// falls back to a full repaint when more than MaxDirtyRects boxes gather
// in one frame (same len > 16 rule as render Scene and ui/scene). Boxes
// are clipped to the tracker bounds; empty and non-finite inputs are
// ignored quietly. A nil *DirtyTracker never panics.
package dirty
