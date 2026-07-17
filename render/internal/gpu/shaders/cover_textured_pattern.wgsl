// Session-inline ImagePattern cover. Same clip/mask contract as cover.wgsl.
// group0: uniform + pattern 2D texture + sampler.

struct CoverUni {
    viewport: vec2<f32>,
    _pad0: vec2<f32>,
    inv_row0: vec4<f32>, // A, B, C, _
    inv_row1: vec4<f32>, // D, E, F, _
    pat_size: vec2<f32>,
    opacity: f32,
    clamp_mode: f32, // >0.5 => transparent OOB
}

@group(0) @binding(0) var<uniform> u: CoverUni;
@group(0) @binding(1) var pat_tex: texture_2d<f32>;
@group(0) @binding(2) var pat_samp: sampler;

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
    // Match sdf_render.wgsl aa_hw=0.75 (was d+0.5 → 0.5px half-width, harder clip edges).
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

fn wrap_unit(v: f32, size: f32) -> f32 {
    let s = max(size, 1.0);
    var r = v - floor(v / s) * s;
    if (r >= s) {
        r = 0.0;
    }
    if (r < 0.0) {
        r = r + s;
    }
    return r;
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
    let ix = u.inv_row0.x * px + u.inv_row0.y * py + u.inv_row0.z;
    let iy = u.inv_row1.x * px + u.inv_row1.y * py + u.inv_row1.z;

    var sx: f32;
    var sy: f32;
    if (u.clamp_mode > 0.5) {
        if (ix < 0.0 || iy < 0.0 || ix >= u.pat_size.x || iy >= u.pat_size.y) {
            return vec4<f32>(0.0);
        }
        sx = ix;
        sy = iy;
    } else {
        sx = wrap_unit(ix, u.pat_size.x);
        sy = wrap_unit(iy, u.pat_size.y);
    }
    let fx = floor(sx);
    let fy = floor(sy);
    let uv = (vec2<f32>(fx, fy) + vec2<f32>(0.5, 0.5)) / max(u.pat_size, vec2<f32>(1.0, 1.0));
    let color = textureSampleLevel(pat_tex, pat_samp, uv, 0.0);
    return color * u.opacity * final_cov;
}
