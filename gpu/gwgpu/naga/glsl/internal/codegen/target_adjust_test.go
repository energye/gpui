package codegen

import (
	"math"
	"strings"
	"testing"
)

// WriterFlagTargetCoordinateAdjust moves the coordinate-space adjustment into
// the GpuiTargetAdjust block so one compiled shader set serves both a render
// target and a window default framebuffer. These tests pin the emitted shape of
// both forms; the bit-identity claim that makes the flag safe to turn on for an
// existing render-target pipeline is covered by
// TestTargetAdjustRenderTargetValueMatchesLiteralFlip.

const targetAdjustVertexWGSL = `
@vertex
fn vs_main(@location(0) pos: vec4<f32>) -> @builtin(position) vec4<f32> {
    return pos;
}
`

const targetAdjustFragmentWGSL = `
struct FragIn {
    @builtin(position) pos: vec4<f32>,
    @location(0) color: vec4<f32>,
}

@fragment
fn fs_main(in: FragIn) -> @location(0) vec4<f32> {
    return in.pos + in.color;
}
`

func TestTargetAdjustFlagEmitsBlockAndUniformVertexFlip(t *testing.T) {
	output := wgslToGLSL(t, targetAdjustVertexWGSL, Options{
		LangVersion: Version330,
		WriterFlags: WriterFlagAdjustCoordinateSpace | WriterFlagTargetCoordinateAdjust,
	})

	glslMustContain(t, output, "layout(std140) uniform GpuiTargetAdjust {")
	glslMustContain(t, output, "vec4 gpui_target_adjust;")
	glslMustContain(t, output, "gl_Position.yz = vec2(gpui_target_adjust.x * gl_Position.y, gl_Position.z * 2.0 - gl_Position.w);")
	if strings.Contains(output, "vec2(-gl_Position.y") {
		t.Errorf("vertex flip must not bake -1.0 when the target adjust flag is set.\nGot:\n%s", output)
	}
}

func TestTargetAdjustFlagRoutesFragmentPositionThroughBlock(t *testing.T) {
	output := wgslToGLSL(t, targetAdjustFragmentWGSL, Options{
		LangVersion: Version330,
		WriterFlags: WriterFlagAdjustCoordinateSpace | WriterFlagTargetCoordinateAdjust,
	})

	glslMustContain(t, output, "layout(std140) uniform GpuiTargetAdjust {")
	glslMustContain(t, output, "gpui_target_adjust.y * gl_FragCoord.y + gpui_target_adjust.z")
	if strings.Contains(output, "in.pos = gl_FragCoord") {
		t.Errorf("fragment position read must go through the target adjust block.\nGot:\n%s", output)
	}
}

func TestTargetAdjustBlockDeclaredInBothStages(t *testing.T) {
	// A linked program shares one block between its stages, so both stages must
	// declare it with the same name and layout even when only one stage uses it.
	fragment := wgslToGLSL(t, `
@fragment
fn fs_main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
    return color;
}
`, Options{
		LangVersion: Version330,
		WriterFlags: WriterFlagAdjustCoordinateSpace | WriterFlagTargetCoordinateAdjust,
	})

	glslMustContain(t, fragment, "layout(std140) uniform GpuiTargetAdjust {")
}

func TestTargetAdjustFlagOffKeepsLiteralFlip(t *testing.T) {
	vertex := wgslToGLSL(t, targetAdjustVertexWGSL, Options{
		LangVersion: Version330,
		WriterFlags: WriterFlagAdjustCoordinateSpace,
	})

	glslMustContain(t, vertex, "gl_Position.yz = vec2(-gl_Position.y, gl_Position.z * 2.0 - gl_Position.w);")
	if strings.Contains(vertex, targetAdjustBlockName) {
		t.Errorf("target adjust block must not appear without the flag.\nGot:\n%s", vertex)
	}

	fragment := wgslToGLSL(t, targetAdjustFragmentWGSL, Options{
		LangVersion: Version330,
		WriterFlags: WriterFlagAdjustCoordinateSpace,
	})

	if strings.Contains(fragment, targetAdjustBlockName) {
		t.Errorf("target adjust block must not appear without the flag.\nGot:\n%s", fragment)
	}
}

func TestTargetAdjustRequiresCoordinateSpaceFlag(t *testing.T) {
	// The adjustment block only carries parameters for the coordinate-space
	// adjustment; on its own it must stay inert.
	output := wgslToGLSL(t, targetAdjustVertexWGSL, Options{
		LangVersion: Version330,
		WriterFlags: WriterFlagTargetCoordinateAdjust,
	})

	if strings.Contains(output, targetAdjustBlockName) {
		t.Errorf("target adjust block requires WriterFlagAdjustCoordinateSpace.\nGot:\n%s", output)
	}
	if strings.Contains(output, "gl_Position.yz") {
		t.Errorf("no coordinate adjustment expected without WriterFlagAdjustCoordinateSpace.\nGot:\n%s", output)
	}
}

func TestTargetAdjustSkippedForComputeStage(t *testing.T) {
	output := wgslToGLSL(t, `
@compute @workgroup_size(1)
fn cs_main(@builtin(global_invocation_id) id: vec3<u32>) {
}
`, Options{
		LangVersion: Version330,
		WriterFlags: WriterFlagAdjustCoordinateSpace | WriterFlagTargetCoordinateAdjust,
	})

	if strings.Contains(output, targetAdjustBlockName) {
		t.Errorf("compute stage must not declare the target adjust block.\nGot:\n%s", output)
	}
}

// TestTargetAdjustRenderTargetValueMatchesLiteralFlip pins the equivalence the
// render-target constants rely on: multiplying by -1.0 reproduces the literal
// negation bit for bit, so turning the flag on cannot move a pixel while the
// target stays a render target.
func TestTargetAdjustRenderTargetValueMatchesLiteralFlip(t *testing.T) {
	// Render-target constants: ySign -1.0, fragYScale 1.0, fragYOffset 0.0.
	const ySign float32 = -1.0
	const fragYScale float32 = 1.0
	const fragYOffset float32 = 0.0

	vertexValues := []float32{0.0, -0.0, 1.0, -1.0, 0.5, -0.5, 479.5, -479.5, 1e-38, -1e-38, 3.4e38, float32(math.Inf(1)), float32(math.Inf(-1))}
	for _, v := range vertexValues {
		if math.Float32bits(ySign*v) != math.Float32bits(-v) {
			t.Errorf("ySign %v: multiply by -1.0 = %v (bits %#x), literal negation = %v (bits %#x)",
				v, ySign*v, math.Float32bits(ySign*v), -v, math.Float32bits(-v))
		}
	}

	fragmentValues := []float32{0.5, 1.5, 320.5, 240.5, 1e-38, 3.4e38}
	for _, v := range fragmentValues {
		if got := fragYScale*v + fragYOffset; math.Float32bits(got) != math.Float32bits(v) {
			t.Errorf("fragYScale %v: 1.0*v+0.0 = %v, want identity %v", v, got, v)
		}
	}
}
