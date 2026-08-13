// sdf_render.wgsl - Vertex + Fragment shader for SDF shape rendering.
//
// Renders SDF shapes (circles, ellipses, rectangles, rounded rectangles)
// via a bounding-quad approach. Each shape becomes a screen-aligned quad;
// the fragment shader evaluates the signed distance function per pixel for
// smooth anti-aliased coverage.
//
// Shape kinds (encoded in shape_kind):
//   0 = circle/ellipse (param1=radius_x, param2=radius_y, param3=unused)
//   1 = rounded rectangle (param1=half_width, param2=half_height, param3=corner_radius)
//
// NOTE: All math avoids naga-problematic builtins (smoothstep, clamp, abs,
// min, max, select). Only sqrt() is used. See sdf_batch.wgsl header for
// the full list of naga SPIR-V backend issues.

struct Uniforms {
    viewport: vec2<f32>,   // width, height in pixels
    anti_alias: u32,       // 1 = anti-aliased (smoothstep), 0 = aliased (binary step)
    _pad: u32,
}

struct VertexInput {
    @location(0) position: vec2<f32>,  // quad corner in pixel coords
    @location(1) local: vec2<f32>,     // offset from shape center
    @location(2) shape_kind: f32,      // 0=circle/ellipse, 1=rrect, 2=annular arc sector (as f32)
    @location(3) param1: f32,          // radius_x or half_width
    @location(4) param2: f32,          // radius_y or half_height
    @location(5) param3: f32,          // corner_radius (rrect) or 0
    @location(6) half_stroke: f32,     // half stroke width (0 for filled)
    @location(7) is_stroked: f32,      // 1.0 for stroked, 0.0 for filled
    @location(8) color: vec4<f32>,     // premultiplied RGBA
    @location(9) angle0: f32,          // arc start angle (radians, parameter space), kind 2
    @location(10) angle1: f32,         // arc end angle (radians, parameter space), kind 2
}

struct VertexOutput {
    @builtin(position) clip_position: vec4<f32>,
    @location(0) local: vec2<f32>,
    @location(1) shape_kind: f32,
    @location(2) param1: f32,
    @location(3) param2: f32,
    @location(4) param3: f32,
    @location(5) half_stroke: f32,
    @location(6) is_stroked: f32,
    @location(7) color: vec4<f32>,
    @location(8) angle0: f32,
    @location(9) angle1: f32,
}

@group(0) @binding(0) var<uniform> u: Uniforms;

// --- RRect clip uniform (shared across all pipelines) ---
struct ClipParams {
    clip_rect: vec4<f32>,   // (left, top, right, bottom) device pixels
    clip_radius: f32,
    clip_enabled: f32,      // 0.0 = no clip, 1.0 = active
    _pad: vec2<f32>,
}
@group(1) @binding(0) var<uniform> clip: ClipParams;

// --- L.06 full-surface R8 alpha mask (SDF cover-inline sample) ---
struct MaskParams {
    mask_enabled: f32,
    _p0: f32,
    _p1: f32,
    _p2: f32,
}
@group(2) @binding(0) var mask_tex: texture_2d<f32>;
@group(2) @binding(1) var mask_samp: sampler;
@group(2) @binding(2) var<uniform> mask_u: MaskParams;

// rrect_clip_coverage computes anti-aliased coverage for the RRect clip
// region at the given fragment position. Returns 1.0 when clip is disabled.
// All math is naga-safe (no abs/min/max/clamp/smoothstep builtins).
// Uses single-return pattern to avoid naga SPIR-V early-return codegen
// issues on Intel Vulkan.
fn rrect_clip_coverage(frag_pos: vec2<f32>) -> f32 {
    // Branchless: Intel Vulkan shader compiler generates bad code for complex
    // sqrt-heavy math inside conditional blocks. Compute SDF unconditionally,
    // then arithmetic-select the result based on clip_enabled.
    // When enabled=0: 0*sdf + 1*1.0 = 1.0 (no clip).
    // When enabled=1: 1*sdf + 0*1.0 = sdf (clip active).
    // Workaround for GPU-CLIP-002 / naga SPIR-V codegen on Intel Vulkan.
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

fn mask_coverage(frag_pos: vec2<f32>) -> f32 {
    let uv = frag_pos / max(u.viewport, vec2<f32>(1.0, 1.0));
    let m = textureSampleLevel(mask_tex, mask_samp, uv, 0.0).r;
    return mask_u.mask_enabled * m + (1.0 - mask_u.mask_enabled);
}

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    // Transform pixel coordinates to NDC.
    let ndc_x = in.position.x / u.viewport.x * 2.0 - 1.0;
    let ndc_y = 1.0 - in.position.y / u.viewport.y * 2.0;
    out.clip_position = vec4<f32>(ndc_x, ndc_y, 0.0, 1.0);
    out.local = in.local;
    out.shape_kind = in.shape_kind;
    out.param1 = in.param1;
    out.param2 = in.param2;
    out.param3 = in.param3;
    out.half_stroke = in.half_stroke;
    out.is_stroked = in.is_stroked;
    out.color = in.color;
    out.angle0 = in.angle0;
    out.angle1 = in.angle1;
    return out;
}

// arc_angle_coverage returns 1.0 inside the arc's angular span, ramping to 0
// over ~ang_aa radians at both ends (≈1.5px of arc travel at the min radius).
// Kind 2 only; other kinds get 1.0. Parameter-space angle: θ = atan2(dy/ry, dx/rx),
// matching the arc construction p(θ) = (cx + rx·cosθ, cy + ry·sinθ).
fn arc_angle_coverage(kind_f: f32, local: vec2<f32>, p1: f32, p2: f32, a0: f32, a1: f32) -> f32 {
    let arc_f = step(1.5, kind_f); // 1.0 for kind 2, 0.0 otherwise
    let span = a1 - a0;            // positive, < 2π (DrawArc normalizes a2 > a1)
    let rdiff = p1 - p2;
    let rmin = (p1 + p2 - sqrt(rdiff * rdiff)) * 0.5;
    // 1.0 / max(rmin, 0.001) via arithmetic max
    let rinv = 1.0 / ((rmin + 0.001 + sqrt((rmin - 0.001) * (rmin - 0.001))) * 0.5);
    let ang_aa = 1.5 * rinv;       // ~1.5px of arc at min radius, in radians
    let th = atan2(local.y / p2, local.x / p1); // parameter angle in [-π, π]
    let tshift = th - a0;
    let two_pi = 6.283185307179586;
    // tmod = (th - a0) mod 2π, in [0, 2π); inside arc when tmod <= span
    let tmod = tshift - floor(tshift / two_pi) * two_pi;
    // max(tmod - span, 0.0) via arithmetic
    let d_after_end = (tmod - span + sqrt((tmod - span) * (tmod - span))) * 0.5;
    // max(two_pi - tmod, 0.0) via arithmetic
    let d_before_start = (two_pi - tmod + sqrt((two_pi - tmod) * (two_pi - tmod))) * 0.5;
    // min(d_after_end, d_before_start) via arithmetic
    let dd = d_after_end - d_before_start;
    let d_end = (d_after_end + d_before_start - sqrt(dd * dd)) * 0.5;
    // coverage ramps down over ang_aa: (1 - d/ang_aa) clamped via arithmetic
    let ramp = (ang_aa - d_end + sqrt((ang_aa - d_end) * (ang_aa - d_end))) * 0.5;
    let cov = ramp / max(ang_aa, 1e-6);
    // min(cov, 1.0) via arithmetic
    let cv = cov - 1.0;
    let capped = (cov + 1.0 - sqrt(cv * cv)) * 0.5;
    return 1.0 - arc_f + arc_f * capped;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let dx = in.local.x;
    let dy = in.local.y;

    // --- Circle/ellipse SDF (kind == 0) ---
    let nx = dx / in.param1;
    let ny = dy / in.param2;
    let elen = sqrt(nx * nx + ny * ny);
    // min(param1, param2) via arithmetic
    let rdiff = in.param1 - in.param2;
    let min_r = (in.param1 + in.param2 - sqrt(rdiff * rdiff)) * 0.5;
    let d_circle = (elen - 1.0) * min_r;

    // --- Rounded rectangle SDF (kind == 1) ---
    // abs via sqrt(x*x)
    let apx = sqrt(dx * dx);
    let apy = sqrt(dy * dy);
    let qx = apx - in.param1 + in.param3;
    let qy = apy - in.param2 + in.param3;
    // max(q, 0) via (q + sqrt(q*q)) * 0.5
    let mqx = (qx + sqrt(qx * qx)) * 0.5;
    let mqy = (qy + sqrt(qy * qy)) * 0.5;
    let outside = sqrt(mqx * mqx + mqy * mqy);
    // max(qx, qy) via arithmetic
    let qdiff = qx - qy;
    let max_qxy = (qx + qy + sqrt(qdiff * qdiff)) * 0.5;
    // min(max_qxy, 0) via arithmetic
    let inside = (max_qxy - sqrt(max_qxy * max_qxy)) * 0.5;
    let d_rrect = outside + inside - in.param3;

    // Select distance based on shape kind using arithmetic.
    let kind_f = in.shape_kind;
    // rrect only for kind exactly 1 (step(0.5,·)`·`step(1.5,·)); kind 2 (arc
    // sector) uses the circle distance field with annular stroke + angle cut.
    let is_rrect = step(0.5, kind_f) - step(1.5, kind_f);
    let is_circle = 1.0 - is_rrect;
    let d = d_circle * is_circle + d_rrect * is_rrect;

    // --- Stroke transformation ---
    let abs_d = sqrt(d * d);
    let effective_dist = d + in.is_stroked * (abs_d - in.half_stroke - d);

    // --- Coverage computation ---
    // AA path: smoothstep over ~1.5px (half-width 0.75, matches CPU sdfAntialiasWidth).
    // Wider than the old fixed 1px zone so diagonals/circles are less stair-stepped.
    let aa_hw = 0.75;
    let t_raw = effective_dist / (2.0 * aa_hw) + 0.5;
    // clamp(t_raw, 0, 1) via arithmetic
    let t_pos = (t_raw + sqrt(t_raw * t_raw)) * 0.5;
    let t_diff = t_pos - 1.0;
    let t = (t_pos + 1.0 - sqrt(t_diff * t_diff)) * 0.5;
    let aa_coverage = 1.0 - t * t * (3.0 - 2.0 * t);

    // No-AA path: binary step (inside=1.0, outside=0.0).
    // Uses steep ramp via naga-safe arithmetic (same max/min pattern).
    // Factor 65536 makes the transition zone < 0.001px — effectively binary.
    let steep = -effective_dist * 65536.0;
    let sp = (steep + sqrt(steep * steep)) * 0.5;
    let sd = sp - 1.0;
    let noaa_coverage = (sp + 1.0 - sqrt(sd * sd)) * 0.5;

    // Blend between AA and no-AA based on uniform flag.
    // anti_alias=1 → aa_coverage, anti_alias=0 → noaa_coverage.
    let aa_f = f32(u.anti_alias);
    let coverage = aa_f * aa_coverage + (1.0 - aa_f) * noaa_coverage;

    // Angular cut for kind 2 (arc sectors): fade coverage at the arc ends.
    let arc_cov = arc_angle_coverage(kind_f, in.local, in.param1, in.param2, in.angle0, in.angle1);
    let final_cov0 = coverage * arc_cov;

    // Apply RRect clip + L.06 full-surface R8 mask coverage.
    let clip_cov = rrect_clip_coverage(in.clip_position.xy);
    let mask_cov = mask_coverage(in.clip_position.xy);
    let final_coverage = final_cov0 * clip_cov * mask_cov;

    // Discard fully transparent pixels.
    if final_coverage < 1.0 / 255.0 {
        discard;
    }

    // Output premultiplied color scaled by coverage.
    return in.color * final_coverage;
}
