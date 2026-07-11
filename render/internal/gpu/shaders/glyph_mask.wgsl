// glyph_mask.wgsl - Alpha Mask Text Rendering Shader (Tier 6)
//
// Renders CPU-rasterized glyph alpha masks as textured quads. The atlas
// stores R8 (single-channel) coverage data produced by AnalyticFiller.
//
// The fragment shader outputs premultiplied alpha.
// Color is passed via uniform buffer (per-batch).
//
// References:
// - Skia GrAtlasTextOp (R8 atlas compositing)
// - Chrome cc::GlyphAtlas (alpha mask cache + GPU upload)

struct GlyphMaskUniforms {
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

@group(0) @binding(0) var<uniform> uniforms: GlyphMaskUniforms;
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
    // Text shaders: no per-pixel SDF clip. Returns 1.0 (no clipping).
    //
    // Enterprise research (GPU-CLIP-002) found that NO production 2D engine
    // (Vello, Skia Graphite/Ganesh, Pathfinder, Qt RHI) computes per-pixel
    // SDF clip inside text fragment shaders. The industry-standard approach
    // is stencil-buffer clip (Skia Ganesh) or depth-buffer clip (Graphite).
    //
    // Per-pixel SDF clip (11 sqrt calls) combined with textureSample causes
    // Intel Vulkan shader compiler to generate corrupt code — text becomes
    // invisible. This is a known Intel driver limitation with register
    // pressure from complex ALU + texture sampling in the same shader.
    //
    // Text clipping is handled by:
    //   1. Hardware scissor rect (axis-aligned, free) — GPU-CLIP-001
    //   2. Stencil-buffer RRect clip (planned) — GPU-CLIP-003
    //
    // The @group(1) binding is kept for uniform pipeline layout across all
    // tiers, avoiding per-tier bind group logic in GPURenderSession.
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
    let alpha = textureSample(atlas_texture, atlas_sampler, in.tex_coord).r;
    let clip_cov = rrect_clip_coverage(in.position.xy);
    let color = uniforms.color;
    let a = alpha * color.a * clip_cov;
    return vec4<f32>(color.rgb * a, a);
}
