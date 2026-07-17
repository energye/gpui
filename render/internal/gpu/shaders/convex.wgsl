// convex.wgsl - Vertex + Fragment shader for convex polygon rendering with
// per-edge analytic anti-aliasing and optional full-surface R8 mask (L.06).
//
// Each vertex carries a pixel position, a coverage value (1.0 = interior,
// 0.0 = outermost AA fringe), and a premultiplied RGBA color.
//
// The vertex shader transforms pixel coordinates to NDC.
// The fragment shader outputs color * coverage * clip * mask, discarding
// fully transparent fragments.

struct Uniforms {
    viewport: vec2<f32>,  // width, height in pixels
    _pad: vec2<f32>,
}

struct VertexInput {
    @location(0) position: vec2<f32>,  // pixel position
    @location(1) coverage: f32,        // 1.0 = interior, 0.0..1.0 = AA ramp
    @location(2) color: vec4<f32>,     // premultiplied RGBA
}

struct VertexOutput {
    @builtin(position) clip_position: vec4<f32>,
    @location(0) coverage: f32,
    @location(1) color: vec4<f32>,
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

// --- L.06 full-surface R8 alpha mask (cover-inline sample) ---
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
    // Branchless enable: sample always (1x1 white default when disabled).
    let uv = frag_pos / max(u.viewport, vec2<f32>(1.0, 1.0));
    let m = textureSampleLevel(mask_tex, mask_samp, uv, 0.0).r;
    return mask_u.mask_enabled * m + (1.0 - mask_u.mask_enabled);
}

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    let ndc_x = in.position.x / u.viewport.x * 2.0 - 1.0;
    let ndc_y = 1.0 - in.position.y / u.viewport.y * 2.0;
    out.clip_position = vec4<f32>(ndc_x, ndc_y, 0.0, 1.0);
    out.coverage = in.coverage;
    out.color = in.color;
    return out;
}

// opt33: SkipAA mesh path — coverage is always 1.0, omitted from vertex buffer
// (12B: pos2 + unorm8x4 color). Same fragment path as vs_main.
struct MeshVertexInput {
    @location(0) position: vec2<f32>,
    @location(1) color: vec4<f32>,
}

@vertex
fn vs_mesh(in: MeshVertexInput) -> VertexOutput {
    var out: VertexOutput;
    let ndc_x = in.position.x / u.viewport.x * 2.0 - 1.0;
    let ndc_y = 1.0 - in.position.y / u.viewport.y * 2.0;
    out.clip_position = vec4<f32>(ndc_x, ndc_y, 0.0, 1.0);
    out.coverage = 1.0;
    out.color = in.color;
    return out;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let clip_cov = rrect_clip_coverage(in.clip_position.xy);
    let mask_cov = mask_coverage(in.clip_position.xy);
    let final_coverage = in.coverage * clip_cov * mask_cov;
    if final_coverage < 1.0 / 255.0 {
        discard;
    }
    return in.color * final_coverage;
}
