// Stencil fill vertex + fragment shader for stencil-then-cover path rendering.
//
// F2: vertices arrive in USER space; the vertex shader applies the per-draw
// affine transform (user → device pixels) from the uniform, then the same
// viewport projection as before. Baked device-space draws pass identity.
// The fragment shader outputs zero color (stencil-only pass uses WriteMask=0,
// but a fragment shader is included for maximum backend compatibility).

struct Uniforms {
    viewport: vec2<f32>,  // width, height in pixels
    m_row0: vec4<f32>,    // affine row 0 (m00, m01, m02, pad): world.x = dot(row0.xyz, vec3(pos, 1))
    m_row1: vec4<f32>,    // affine row 1 (m10, m11, m12, pad)
    color: vec4<f32>,     // fill color (premultiplied) — used by the AA band pipelines sharing this buffer
}

@group(0) @binding(0) var<uniform> u: Uniforms;

@vertex
fn vs_main(@location(0) pos: vec2<f32>) -> @builtin(position) vec4<f32> {
    let p = vec3<f32>(pos, 1.0);
    let wx = dot(u.m_row0.xyz, p);
    let wy = dot(u.m_row1.xyz, p);
    let ndc_x = wx / u.viewport.x * 2.0 - 1.0;
    let ndc_y = 1.0 - wy / u.viewport.y * 2.0;
    return vec4<f32>(ndc_x, ndc_y, 0.0, 1.0);
}

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(0.0, 0.0, 0.0, 0.0);
}
