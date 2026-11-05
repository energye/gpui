//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package sprite freezes the CPU-side sprite ordering every 2.5D draw feeds.
//
// Frozen 2026-09-15: LayerWorld, LayerFX,
// LayerUI, Item, NewItem, Sort, IsSorted. Additive changes only.
// P3.7 batch 2 step 2b (2026-11-04) kept every signature and moved the
// judgement: Sort/IsSorted and DepthSort/IsDepthSorted no longer compare by a
// local copy of the rule — the band maps onto camera's z buckets
// (ZIndexForLayer) and camera.YSortLess decides (band bucket, foot Y with
// camera's tolerance-based equality, input position as the tie breaker).
// Measured bitwise-identical to the pre-delegation order on
// testdata/p37_zsort_parity.json (two 1000-item stage_chase-shaped sets
// included); the only intended delta is foot-Y differences inside camera's
// equality tolerance, which now resolve by input position instead of by the
// last float bits.
//
// Frozen 2026-09-15: LoopOnce, LoopLoop,
// LoopPingPong, Clip, EventKind (EventNone/EventFrame/EventLoop/
// EventFinished), Event, Flipbook, NewFlipbook, AddClip, Has, Count,
// OnEvent, Play, Current, IsPlaying, Update. Additive changes only.
//
// Frozen 2026-09-15: Sprite, NewSprite,
// Skippable, EffectiveOpacity, Batch, NewBatch, Add, Flush, Len,
// Skipped, Clear. Additive changes only. Same image submits once:
// Flush groups by image in first-seen order and calls emit once per
// image. Pure numbers only; the caller draws each group with the
// existing render DrawAtlas (Src.X/Y/W/H, Dst.X/Y/W/H, EffectiveOpacity
// field for field), render main path untouched.
//
// Frozen 2026-09-15, 采样枚举于 P3.7 批 4（2026-11-05）并入渲染层现成枚举：
// AtlasSprite.Filter 改 render.InterpolationMode（零值 FilterUnset=未指定，由 render
// 归一 Bilinear 老路；nearest/bilinear/bicubic 三档 1:1 保留），新增
// AtlasSprite.Mipmap render.MipmapMode（零值 MipmapNone=单档历史路，逐位不变）；
// 删 AtlasFilter 与其四常量（同一能力的重复副本），字符串词汇不裁剪——
// ParseAtlasFilter→ParseFilter（default/nearest/bilinear/bicubic 照旧）、
// 稳定小写名走 FilterName，档位另有 ParseMipmap/MipmapName（none/nearest/linear）。
// NewAtlasSprite 的 filter 形参随之换型（接口全量切换制，不留垫片）。
// 冻结面其余不变：AtlasSprite, NewAtlasSprite, Validate, Skippable, HasTint,
// PivotPoint, FeetPivot, ToRender, AtlasToRender. Additive changes only. Rotation is
// radians about the pivot;
// FlipX/FlipY mirror about the pivot before Rot; Pivot is the offset from
// the Dst origin in Dst units (zero is top-left, FeetPivot pins the feet);
// Tint zero struct means no tint, else straight multiply incl alpha;
// Filter zero means the historic bilinear road (render normalizes it),
// per-sprite nearest/bilinear/bicubic each on its own; Mipmap zero stays on
// the single-level road. Pure numbers plus one boundary
// conversion; the caller draws the result with the existing render
// DrawAtlasEx (field for field), render main path untouched. Window intent
// Bad numbers are core InvalidArg and change nothing.
//
// Frozen 2026-09-15: DeepItem, NewDeepItem,
// SetDepth, DepthSort, IsDepthSorted. Additive changes only. Depth follows
// R5/engine/camera: bigger = farther, drawn first (far-to-near painter);
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
