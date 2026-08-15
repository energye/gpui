// textured_quad_bicubic.wgsl - Textured Quad Bicubic Rendering Shader (Tier 3, I.03)
//
// Same bind-group contract as textured_quad.wgsl, but the fragment shader
// performs a 4x4 (16-tap) bicubic convolution instead of hardware bilinear
// sampling. WebGPU samplers have no cubic filter; this matches Skia's
// SkCubicResampler approach (GPU-side cubic convolution).
//
// Kernel: Catmull-Rom (B=0, C=0.5) by default — pixel-identical to the CPU
// path render/internal/image/interp.go SampleBicubic. Switching the
// constants below to B=1/3, C=1/3 yields Mitchell-Netravali (Skia default).
//
// Sampling uses the nearest sampler: tap weights are computed in-shader, so
// hardware filtering must not further interpolate. Clamp-to-edge address
// mode keeps out-of-bounds taps clamped to the image border.
//
// Premultiplied alpha throughout (same as textured_quad.wgsl).

struct ImageUniforms {
    transform: mat4x4<f32>,  // ortho projection matrix
    opacity_pad: vec4<f32>,  // x = opacity multiplier (0.0 to 1.0)
}

struct VertexInput {
    @location(0) position: vec2<f32>,   // quad corner in pixel coords
    @location(1) tex_coord: vec2<f32>,  // UV coordinates (0..1 range)
}

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) tex_coord: vec2<f32>,
}

@group(0) @binding(0) var<uniform> uniforms: ImageUniforms;
@group(0) @binding(1) var image_texture: texture_2d<f32>;
@group(0) @binding(2) var image_sampler: sampler;

// --- RRect clip uniform (shared across all pipelines) ---
struct ClipParams {
    clip_rect: vec4<f32>,   // (left, top, right, bottom) device pixels
    clip_radius: f32,
    clip_enabled: f32,      // 0.0 = no clip, 1.0 = active
    _pad: vec2<f32>,
}
@group(1) @binding(0) var<uniform> clip: ClipParams;

fn rrect_clip_coverage(frag_pos: vec2<f32>) -> f32 {
    // Image shaders: no per-pixel SDF clip (see textured_quad.wgsl rationale).
    // Image clipping is handled by hardware scissor / stencil.
    return 1.0;
}

// Cubic kernel weight with B/C parameters (SkCubicResampler semantics).
// B=0, C=0.5 -> Catmull-Rom (matches CPU SampleBicubic).
// B=1/3, C=1/3 -> Mitchell-Netravali (Skia default cubic).
// Avoids naga-problematic builtins (abs/clamp/min/max/select): uses sqrt(x*x)
// for |t| and if branches for the piecewise kernel.
fn cubic_weight(t_in: f32, B: f32, C: f32) -> f32 {
    let t = sqrt(t_in * t_in);  // |t|
    if t < 1.0 {
        // 1/6 * ((12-9B-6C)|t|^3 + (-18+12B+6C)t^2 + (6-2B))
        let a = (12.0 - 9.0 * B - 6.0 * C) / 6.0;  // |t|^3 coeff
        let b = (-18.0 + 12.0 * B + 6.0 * C) / 6.0; // t^2 coeff
        let c = (6.0 - 2.0 * B) / 6.0;              // const
        return a * t * t * t + b * t * t + c;
    }
    if t < 2.0 {
        // 1/6 * ((-B-6C)|t|^3 + (6B+30C)t^2 + (-12B-48C)|t| + (8B+24C))
        let a = (-B - 6.0 * C) / 6.0;
        let b = (6.0 * B + 30.0 * C) / 6.0;
        let c = (-12.0 * B - 48.0 * C) / 6.0;
        let d = (8.0 * B + 24.0 * C) / 6.0;
        return a * t * t * t + b * t * t + c * t + d;
    }
    return 0.0;
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
    return out;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let clip_cov = rrect_clip_coverage(in.position.xy);
    let opacity = uniforms.opacity_pad.x * clip_cov;
    if opacity < 1.0 / 255.0 {
        discard;
    }

    // Catmull-Rom default (B=0, C=0.5); Mitchell = 1/3, 1/3.
    let B = 0.0;
    let C = 0.5;

    let dims = vec2<f32>(textureDimensions(image_texture));
    // Continuous texel coordinate: tex_coord * dims - 0.5 (texel centers).
    let texel_pos = in.tex_coord * dims - vec2<f32>(0.5, 0.5);
    let base = floor(texel_pos);
    let frac = texel_pos - base;

    var acc = vec4<f32>(0.0, 0.0, 0.0, 0.0);

    for (var j = 0; j < 4; j = j + 1) {
        let wy = cubic_weight(frac.y - f32(j - 1), B, C);
        let iy = base.y + f32(j - 1);
        for (var i = 0; i < 4; i = i + 1) {
            let wx = cubic_weight(frac.x - f32(i - 1), B, C);
            let ix = base.x + f32(i - 1);
            // texel center + 0.5 back to UV (sampler clamps to edge).
            // NOTE: negative kernel weights (Catmull-Rom overshoot) must be
            // included — skipping taps with w <= 0 changes the kernel sum and
            // brightens the image.
            let uv = vec2<f32>(ix + 0.5, iy + 0.5) / dims;
            let w = wx * wy;
            let sample_color = textureSampleLevel(image_texture, image_sampler, uv, 0.0);
            acc = acc + sample_color * w;
        }
    }

    // No weight normalization: Catmull-Rom kernel sums to 1.0, and clamp-to-
    // edge taps mirror the CPU path (internal/image bicubicInterp) and Skia
    // (direct weighted sum, no renormalization).
    let color = acc;
    return color * opacity;
}