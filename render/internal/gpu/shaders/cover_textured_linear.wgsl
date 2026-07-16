// Session-inline textured cover for linear/radial/sweep/focal gradients.
// Same clip (@group1) and mask (@group2) contract as solid cover.wgsl.
// group0: uniform + 1D ramp texture + sampler.

struct CoverUni {
    viewport: vec2<f32>,
    _pad0: vec2<f32>,
    // p0/p1/inv_len2/t_min/inv_span/mode match textured_stencil_linear packing.
    p0: vec2<f32>,
    p1: vec2<f32>,
    inv_len2: f32,
    t_min: f32,
    inv_span: f32,
    mode: f32,
}

@group(0) @binding(0) var<uniform> u: CoverUni;
@group(0) @binding(1) var ramp_tex: texture_2d<f32>;
@group(0) @binding(2) var ramp_samp: sampler;

struct ClipParams {
    clip_rect: vec4<f32>,
    clip_radius: f32,
    clip_enabled: f32,
    _pad: vec2<f32>,
}
@group(1) @binding(0) var<uniform> clip: ClipParams;

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
    let t_raw = d + 0.5;
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

fn gradient_t(px: f32, py: f32) -> f32 {
    if (u.mode < 0.5) {
        return ((px - u.p0.x) * u.p1.x + (py - u.p0.y) * u.p1.y) * u.inv_len2;
    }
    if (u.mode < 1.5) {
        let d = length(vec2<f32>(px, py) - u.p0);
        return (d - u.p1.x) * u.p1.y;
    }
    if (u.mode < 2.5) {
        let v = vec2<f32>(px, py) - u.p0;
        if (dot(v, v) < 1e-12) {
            return u.t_min;
        }
        let angle = atan2(v.y, v.x);
        var rel = angle - u.p1.x;
        let two_pi = 6.283185307179586;
        if (u.p1.y >= 0.0) {
            rel = rel - floor(rel / two_pi) * two_pi;
        } else {
            rel = rel - ceil(rel / two_pi) * two_pi;
        }
        return rel * u.p1.y;
    }
    // focal: p0=focus, p1=center, inv_len2=endRadius
    let focus = u.p0;
    let center = u.p1;
    let end_r = u.inv_len2;
    let d = vec2<f32>(px, py) - focus;
    let f = center - focus;
    let a = dot(d, d);
    if (a < 1e-12) {
        return 0.0;
    }
    let b = -2.0 * dot(d, f);
    let c = dot(f, f) - end_r * end_r;
    let disc = b * b - 4.0 * a * c;
    if (disc < 0.0) {
        return 1.0;
    }
    let sqrt_d = sqrt(disc);
    let t1 = (-b - sqrt_d) / (2.0 * a);
    let t2 = (-b + sqrt_d) / (2.0 * a);
    var t_ray = 0.0;
    let t1p = t1 > 0.0;
    let t2p = t2 > 0.0;
    if (t1p && t2p) {
        t_ray = min(t1, t2);
    } else if (t1p) {
        t_ray = t1;
    } else if (t2p) {
        t_ray = t2;
    } else {
        t_ray = 1.0;
    }
    return t_ray;
}

struct CoverVertexOutput {
    @builtin(position) position: vec4<f32>,
}

@vertex
fn vs_main(@location(0) pos: vec2<f32>) -> CoverVertexOutput {
    var out: CoverVertexOutput;
    let ndc_x = pos.x / u.viewport.x * 2.0 - 1.0;
    let ndc_y = 1.0 - pos.y / u.viewport.y * 2.0;
    out.position = vec4<f32>(ndc_x, ndc_y, 0.0, 1.0);
    return out;
}

@fragment
fn fs_main(in: CoverVertexOutput) -> @location(0) vec4<f32> {
    let clip_cov = rrect_clip_coverage(in.position.xy);
    let mask_cov = mask_coverage(in.position.xy);
    let final_cov = clip_cov * mask_cov;
    if final_cov < 1.0 / 255.0 {
        discard;
    }
    let px = in.position.x;
    let py = in.position.y;
    let tt = gradient_t(px, py);
    var ru = (tt - u.t_min) * u.inv_span;
    ru = clamp(ru, 0.0, 1.0);
    let color = textureSampleLevel(ramp_tex, ramp_samp, vec2<f32>(ru, 0.5), 0.0);
    return color * final_cov;
}
