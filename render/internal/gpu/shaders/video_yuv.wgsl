// video_yuv.wgsl - NV12 Video Plane Compositing Shader (P3-A layer 4)
//
// Renders one NV12 video frame as a textured quad: Y comes from an R8
// texture, interleaved UV from an RG8 texture (half height, same width,
// same normalized UVs). The fragment shader converts to RGBA on the GPU
// with the BT.601 limited-range matrix — the same numbers as the CPU
// fallback oracle (render nv12ToRGBA):
//   R = 1.164*(Y-16) + 1.596*(V-128)
//   G = 1.164*(Y-16) - 0.391*(U-128) - 0.813*(V-128)
//   B = 1.164*(Y-16) + 2.018*(U-128), A = opaque.
//
// One WGSL source serves both backends (WebGPU natively, GLES via the
// WGSL→GLSL translator): symmetric by construction. Binding shape mirrors
// the dual-texture blend path (two texture_2d + one sampler, production
// proven), extended with the shared image uniform + RRect clip group so
// video quads clip exactly like image quads.
//
// NOTE: math avoids naga-problematic builtins (smoothstep, clamp, abs,
// min, max, select) like textured_quad.wgsl; plain if-statements are
// fine (dual_tex_blend.wgsl uses them in production).

struct ImageUniforms {
    transform: mat4x4<f32>,  // ortho projection matrix
    opacity_pad: vec4<f32>,  // x = opacity multiplier (0.0 to 1.0)
}

struct VertexInput {
    @location(0) position: vec2<f32>,   // quad corner in pixel coords
    @location(1) tex_coord: vec2<f32>,  // UV coordinates (0..1 range)
    @location(2) tint: vec4<f32>,       // premultiplied straight tint (1,1,1,1 = identity)
}

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) tex_coord: vec2<f32>,
    @location(1) tint: vec4<f32>,
}

@group(0) @binding(0) var<uniform> uniforms: ImageUniforms;
@group(0) @binding(1) var y_texture: texture_2d<f32>;
@group(0) @binding(2) var uv_texture: texture_2d<f32>;
@group(0) @binding(3) var video_sampler: sampler;

// --- RRect clip uniform (shared across all pipelines, same as textured_quad) ---
struct ClipParams {
    clip_rect: vec4<f32>,   // (left, top, right, bottom) device pixels
    clip_radius: f32,
    clip_enabled: f32,      // 0.0 = no clip, 1.0 = active
    _pad: vec2<f32>,
}
@group(1) @binding(0) var<uniform> clip: ClipParams;

fn rrect_clip_coverage(frag_pos: vec2<f32>) -> f32 {
    // Same analytic RRect coverage as textured_quad.wgsl.
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

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    let p = vec4<f32>(in.position, 0.0, 1.0);
    let col0 = uniforms.transform[0];
    let col1 = uniforms.transform[1];
    let col2 = uniforms.transform[2];
    let col3 = uniforms.transform[3];
    let pos = p.x * col0 + p.y * col1 + p.z * col2 + p.w * col3;
    out.position = pos;
    out.tex_coord = in.tex_coord;
    out.tint = in.tint;
    return out;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    // R8/RG8 Unorm sample to [0,1]; scale back to byte range for BT.601.
    let y = textureSample(y_texture, video_sampler, in.tex_coord).r * 255.0;
    let uvp = textureSample(uv_texture, video_sampler, in.tex_coord).rg * 255.0;
    let u = uvp.x - 128.0;
    let v = uvp.y - 128.0;
    let c = y - 16.0;
    var rf = 1.164 * c + 1.596 * v;
    var gf = 1.164 * c - 0.391 * u - 0.813 * v;
    var bf = 1.164 * c + 2.018 * u;
    if (rf < 0.0) {
        rf = 0.0;
    }
    if (rf > 255.0) {
        rf = 255.0;
    }
    if (gf < 0.0) {
        gf = 0.0;
    }
    if (gf > 255.0) {
        gf = 255.0;
    }
    if (bf < 0.0) {
        bf = 0.0;
    }
    if (bf > 255.0) {
        bf = 255.0;
    }
    let clip_cov = rrect_clip_coverage(in.position.xy);
    // Opaque video: premultiplied output is (rgb*o, o) with identity tint.
    let o = uniforms.opacity_pad.x * clip_cov;
    return vec4<f32>(rf / 255.0 * o, gf / 255.0 * o, bf / 255.0 * o, o) * in.tint;
}
