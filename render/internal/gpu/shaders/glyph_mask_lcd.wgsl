// glyph_mask_lcd.wgsl - LCD Subpixel (ClearType) Text Rendering Shader
//
// Renders CPU-rasterized LCD glyph masks as textured quads with per-channel
// alpha compositing. The R8 atlas stores 3 texels per logical pixel
// (R coverage, G coverage, B coverage), arranged horizontally.
//
// The fragment shader samples 3 adjacent horizontal texels to get per-channel
// coverage, then composites each color channel independently for ClearType
// subpixel rendering with 3x effective horizontal resolution.
//
// References:
// - Skia GrAtlasTextOp (separate LCD pipeline, per-channel alpha)
// - FreeType LCD rendering (3x oversampling + FIR filter)
// - DirectWrite ClearType (subpixel positioning + gamma correction)

struct GlyphMaskLCDUniforms {
    transform: mat4x4<f32>,
    color: vec4<f32>,
    atlas_size: vec2<f32>,
    _pad: vec2<f32>,
}

struct VertexInput {
    @location(0) position: vec2<f32>,
    @location(1) tex_coord: vec2<f32>,
}

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) tex_coord: vec2<f32>,
}

@group(0) @binding(0) var<uniform> uniforms: GlyphMaskLCDUniforms;
@group(0) @binding(1) var atlas_texture: texture_2d<f32>;
@group(0) @binding(2) var atlas_sampler: sampler;

// --- RRect clip uniform (shared across all pipelines) ---
struct ClipParams {
    clip_rect: vec4<f32>,
    clip_radius: f32,
    clip_enabled: f32,
    _pad: vec2<f32>,
}
@group(1) @binding(0) var<uniform> clip: ClipParams;

fn rrect_clip_coverage(frag_pos: vec2<f32>) -> f32 {
    // Text shaders: no per-pixel SDF clip. Returns 1.0.
    // See glyph_mask.wgsl for rationale (Intel Vulkan compatibility).
    return 1.0;
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

// Sample LCD R/G/B coverages from the 3x-wide atlas strip using texelLoad
// so filtering cannot collapse adjacent R/G/B subpixel coverages.
fn sample_lcd_cov(uv: vec2<f32>) -> vec3<f32> {
    let dims = vec2<f32>(textureDimensions(atlas_texture));
    let tex = uv * dims;
    let gx = i32(floor(tex.x));
    let gy = i32(floor(tex.y));
    let cov_r = textureLoad(atlas_texture, vec2<i32>(gx - 1, gy), 0).r;
    let cov_g = textureLoad(atlas_texture, vec2<i32>(gx, gy), 0).r;
    let cov_b = textureLoad(atlas_texture, vec2<i32>(gx + 1, gy), 0).r;
    return vec3<f32>(cov_r, cov_g, cov_b);
}

// Coverage vector for LCD (scaled by paint alpha + clip).
fn lcd_cov(uv: vec2<f32>, frag_pos: vec2<f32>) -> vec3<f32> {
    let clip_cov = rrect_clip_coverage(frag_pos);
    return sample_lcd_cov(uv) * uniforms.color.a * clip_cov;
}

// White-destination Replace path (legacy/fast for light UIs).
fn lcd_composite_white_dest(uv: vec2<f32>, frag_pos: vec2<f32>) -> vec4<f32> {
    let cov = lcd_cov(uv, frag_pos);
    let color = uniforms.color;
    let out_rgb = color.rgb * cov + (vec3<f32>(1.0, 1.0, 1.0) - cov);
    let a_max = max(cov.r, max(cov.g, cov.b));
    return vec4<f32>(out_rgb, max(a_max, max(out_rgb.r, max(out_rgb.g, out_rgb.b))));
}

// Pass 1: emit coverage as src color for blend dst * (1 - src)
// Blend: SrcFactor=Zero, DstFactor=OneMinusSrc  → out = dst * (1 - cov)
@fragment
fn fs_darken(in: VertexOutput) -> @location(0) vec4<f32> {
    let cov = lcd_cov(in.tex_coord, in.position.xy);
    let a = max(cov.r, max(cov.g, cov.b));
    return vec4<f32>(cov, a);
}

// Pass 2: emit color*cov for additive blend
// Blend: SrcFactor=One, DstFactor=One → out = dst + color*cov
@fragment
fn fs_add(in: VertexOutput) -> @location(0) vec4<f32> {
    let cov = lcd_cov(in.tex_coord, in.position.xy);
    let color = uniforms.color;
    let a = max(cov.r, max(cov.g, cov.b));
    return vec4<f32>(color.rgb * cov, a);
}

// Optional single-pass white-dest (Replace). Not used when two-pass is active.
@fragment
fn fs_lcd(in: VertexOutput) -> @location(0) vec4<f32> {
    return lcd_composite_white_dest(in.tex_coord, in.position.xy);
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    return lcd_composite_white_dest(in.tex_coord, in.position.xy);
}
