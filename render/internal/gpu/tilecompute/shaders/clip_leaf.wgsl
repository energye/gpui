// Copyright 2026 The gogpu Authors
// SPDX-License-Identifier: MIT
//
// clip_leaf.wgsl — EndClip draw monoid fixup (clip/restore stack matching).
//
// CPU reference: tilecompute/clip_leaf.go clipLeafScan()
// GPU reference: vello_shaders/shader/clip_leaf.wgsl lines 187-203
//
// draw_leaf produces one ClipInp per clip operation, indexed by the draw
// monoid's exclusive clip_ix: BeginClip stores its own draw index and path_ix
// (positive), EndClip stores its draw index negated (`~draw_ix`). Without
// fixup, an EndClip draw monoid points at its own dummy path (no geometry)
// and at a stale scene offset, so coarse cannot emit the clip-path coverage
// fill nor read the clip blend mode/alpha.
//
// This stage runs as a single workgroup with a workgroup-local stack and
// replays clip operations in clip order with LIFO matching — the exact
// semantics of the CPU clipLeafScan. Unmatched EndClips are skipped.
//
// Pipeline order: ... draw_leaf → **clip_leaf** → path_count → backdrop → coarse ...

// --- Shared types ---

struct ClipInp {
    ix: u32,       // draw object index of the clip operation
    path_ix: i32,  // BeginClip: positive path index; EndClip: ~draw_ix
}

struct DrawMonoid {
    path_ix: u32,
    clip_ix: u32,
    scene_offset: u32,
    info_offset: u32,
}

struct Config {
    width_in_tiles: u32,
    height_in_tiles: u32,
    target_width: u32,
    target_height: u32,
    n_drawobj: u32,
    n_path: u32,
    n_clip: u32,
    pathtag_base: u32,
    pathdata_base: u32,
    drawtag_base: u32,
    drawdata_base: u32,
    transform_base: u32,
    style_base: u32,
    n_lines: u32,
    bg_color: u32,
}

// --- Constants ---

// Maximum clip nesting depth supported by the workgroup stack.
// Matches the blend stack depth handled by fine.wgsl (BLEND_STACK_SPLIT=4
// in registers + spill); 64 covers realistic scenes with headroom. Exceeding
// the cap skips the pair (same as CPU's unmatched-EndClip path).
const MAX_CLIP_STACK: u32 = 64u;

// --- Bindings ---

@group(0) @binding(0) var<uniform> config: Config;
@group(0) @binding(1) var<storage, read> clip_inp: array<ClipInp>;
@group(0) @binding(2) var<storage, read_write> draw_monoids: array<DrawMonoid>;

// Workgroup-local stack of BeginClip indices into clip_inp.
var<workgroup> stack_state: array<u32, MAX_CLIP_STACK>;

// --- Main entry point ---
// Single workgroup; thread 0 replays all clip operations sequentially.
// Dispatch: (1, 1, 1) when config.n_clip > 0, skipped entirely when 0.

@compute @workgroup_size(1, 1, 1)
fn main() {
    var depth = 0u;

    for (var i = 0u; i < config.n_clip; i = i + 1u) {
        let inp = clip_inp[i];

        if inp.path_ix >= 0 {
            // BeginClip: push its clip_inp index onto the stack.
            if depth < MAX_CLIP_STACK {
                stack_state[depth] = i;
                depth = depth + 1u;
            }
        } else {
            // EndClip: pop the matching BeginClip and fix up its draw monoid.
            if depth == 0u {
                continue; // Malformed: unmatched EndClip, skip.
            }
            depth = depth - 1u;
            let parent = stack_state[depth];
            let parent_clip = clip_inp[parent];

            // Restore the EndClip draw index from the bitwise complement.
            let end_idx = u32(~inp.path_ix);

            if end_idx < config.n_drawobj && parent_clip.ix < config.n_drawobj {
                // EndClip gets BeginClip's path_ix so coarse can look up the clip path.
                draw_monoids[end_idx].path_ix = u32(parent_clip.path_ix);
                // EndClip gets BeginClip's scene_offset so coarse can read blend_mode/alpha.
                draw_monoids[end_idx].scene_offset = draw_monoids[parent_clip.ix].scene_offset;
            }
        }
    }
}