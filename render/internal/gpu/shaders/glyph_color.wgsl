// glyph_color.wgsl - RGBA Color Glyph Rendering Shader (Tier 6 color path)
//
// Renders CPU-rasterized color glyphs (CBDT bitmaps, flattened COLR layers)
// packed in RGBA8 atlas pages as textured quads. Texels are premultiplied
// RGBA; the uniform color carries white with opacity in alpha, matching the
// mask pipeline output convention (premultiplied, One / OneMinusSrcAlpha).
//
// References:
// - Skia GrAtlasTextOp (RGBA color atlas compositing)
// - Chrome cc::GlyphAtlas (color glyph cache + GPU upload)

struct GlyphColorUniforms {
    transform: mat4x4<f32>,
    color: vec4<f32>,
}

struct VertexInput {
    @location(0) position: vec2<f32>,
    @location(1) tex_coord: vec2<f32>,
}

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) tex_coord: vec2<f32>,
}

@group(0) @binding(0) var<uniform> uniforms: GlyphColorUniforms;
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
    // Same contract as glyph_mask.wgsl: per-pixel SDF clip is not computed
    // in text shaders (Intel Vulkan limitation); hardware scissor and the
    // stencil/depth clip paths handle clipping.
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

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let tex = textureSample(atlas_texture, atlas_sampler, in.tex_coord);
    let clip_cov = rrect_clip_coverage(in.position.xy);
    let cov = clip_cov * uniforms.color.a;
    return vec4<f32>(tex.rgb * cov, tex.a * cov);
}
