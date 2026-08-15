// cover_aa.wgsl - Analytic-AA cover pass for stencil-then-cover path rendering
// at sampleCount==1 (Skia GPU analytic-AA equivalent; "fringe coverage").
//
// Unlike the binary cover (cover.wgsl bbox quad), this pass draws the path's
// fan triangles with a per-vertex signed distance to the triangle's
// path-boundary edge (edge_d, positive toward the fill interior). The distance
// to a line is affine, so vertex interpolation is exact; the fragment shader
// turns it into smoothstep coverage over the ±aa_hw band (same 0.75px half
// width as sdf_render.wgsl / convexAAExpand).
//
// The stencil test (NotEqual(0)) still gates inside/outside. The outside half
// of the band (pixels just outside the boundary, stencil==0) is painted by a
// companion "band" draw using the SAME shader with an Equal(0) stencil test,
// so the full fringe is symmetric around the boundary.

struct Uniforms {
    viewport: vec2<f32>,  // width, height in pixels
    _pad: vec2<f32>,
    color: vec4<f32>,     // fill color (premultiplied alpha)
}

@group(0) @binding(0) var<uniform> u: Uniforms;

// --- RRect clip uniform (shared across all pipelines) ---
struct ClipParams {
    clip_rect: vec4<f32>,
    clip_radius: f32,
    clip_enabled: f32,
    _pad: vec2<f32>,
}
@group(1) @binding(0) var<uniform> clip: ClipParams;

// --- L.06 full-surface R8 alpha mask (stencil cover-inline sample) ---
struct MaskParams {
    mask_enabled: f32,
    _p0: f32,
    _p1: f32,
    _p2: f32,
}
@group(2) @binding(0) var mask_tex: texture_2d<f32>;
@group(2) @binding(1) var mask_samp: sampler;
@group(2) @binding(2) var<uniform> mask_u: MaskParams;

fn rrect_clip_coverage(frag_pos: vec2<f32>) -> f32 {
    // Branchless: Intel Vulkan shader compiler generates bad code for complex
    // sqrt-heavy math inside conditional blocks. Compute SDF unconditionally,
    // then arithmetic-select the result based on clip_enabled.
    let cx = (clip.clip_rect.x + clip.clip_rect.z) * 0.5;
    let cy = (clip.clip_rect.y + clip.clip_rect.w) * 0.5;
    let hw = (clip.clip_rect.z - clip.clip_rect.x) * 0.5;
    let hh = (clip.clip_rect.w - clip.clip_rect.y) * 0.5;
    let r = clip.clip_radius;
    let dx = sqrt((frag_pos.x - cx) * (frag_pos.x - cx));
    let dy = sqrt((frag_pos.y - cy) * (frag_pos.y - cy));
    let qx = dx - hw + r;
    let qy = dy - hh + r;
    let mqx = (qx + sqrt(qx * qx)) * 0.5;
    let mqy = (qy + sqrt(qy * qy)) * 0.5;
    let outside = sqrt(mqx * mqx + mqy * mqy);
    let qdiff = qx - qy;
    let max_qxy = (qx + qy + sqrt(qdiff * qdiff)) * 0.5;
    let inside = (max_qxy - sqrt(max_qxy * max_qxy)) * 0.5;
    let d = outside + inside - r;
    let aa_hw = 0.75;
    let t_raw = d / (2.0 * aa_hw) + 0.5;
    let t_pos = (t_raw + sqrt(t_raw * t_raw)) * 0.5;
    let t_diff = t_pos - 1.0;
    let t = (t_pos + 1.0 - sqrt(t_diff * t_diff)) * 0.5;
    let sdf_cov = 1.0 - t * t * (3.0 - 2.0 * t);
    return clip.clip_enabled * sdf_cov + (1.0 - clip.clip_enabled);
}

struct AAVertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) edge_d: f32,
}

@vertex
fn vs_main(@location(0) pos: vec2<f32>, @location(1) edge_d: f32) -> AAVertexOutput {
    var out: AAVertexOutput;
    let ndc_x = pos.x / u.viewport.x * 2.0 - 1.0;
    let ndc_y = 1.0 - pos.y / u.viewport.y * 2.0;
    out.position = vec4<f32>(ndc_x, ndc_y, 0.0, 1.0);
    out.edge_d = edge_d;
    return out;
}

fn mask_coverage(frag_pos: vec2<f32>) -> f32 {
    let uv = frag_pos / max(u.viewport, vec2<f32>(1.0, 1.0));
    let m = textureSampleLevel(mask_tex, mask_samp, uv, 0.0).r;
    return mask_u.mask_enabled * m + (1.0 - mask_u.mask_enabled);
}

@fragment
fn fs_main(in: AAVertexOutput) -> @location(0) vec4<f32> {
    // Edge coverage: smoothstep ramp over the ±aa_hw band (naga-safe max/clamp
    // via arithmetic; same pattern as sdf_render.wgsl). edge_d is POSITIVE
    // inside the fill (0 at the boundary line): t=0.5 → 0.5 coverage at the
    // edge, t=1 (d≥+aa) → 1.0 inside, t=0 (d≤−aa) → 0.0 outside. The positive
    // smoothstep t*t*(3-2t) is used because edge_d grows into the fill (the
    // clip/mask helpers below use the opposite sign convention).
    // aa_hw must match tessellate.go aaCoverHalfWidth (band geometry width).
    let aa_hw = 0.35;
    let t_raw = in.edge_d / (2.0 * aa_hw) + 0.5;
    let t_pos = (t_raw + sqrt(t_raw * t_raw)) * 0.5;
    let t_diff = t_pos - 1.0;
    let t = (t_pos + 1.0 - sqrt(t_diff * t_diff)) * 0.5;
    let cov = t * t * (3.0 - 2.0 * t);
    let clip_cov = rrect_clip_coverage(in.position.xy);
    let mask_cov = mask_coverage(in.position.xy);
    let final_cov = cov * clip_cov * mask_cov;
    if final_cov < 1.0 / 255.0 {
        discard;
    }
    return u.color * final_cov;
}