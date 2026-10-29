package shader_test

import (
	"testing"

	"github.com/energye/gpui/gpu/shader"
	"github.com/energye/gpui/gpu/shader/ir"
)

const basicWGSL = `
struct Data { values: array<f32, 4> };
@group(0) @binding(0) var<storage, read_write> data: Data;

@compute @workgroup_size(1)
fn main() {
    data.values[0] = 1.0;
}
`

const f64WGSL = `
@group(0) @binding(0) var<storage, read_write> value: f64;

@compute @workgroup_size(1)
fn main() {
    value = 1.0;
}
`

func TestParseLowerValidate(t *testing.T) {
	ast, err := shader.Parse(basicWGSL)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	module, err := shader.LowerWithSource(ast, basicWGSL)
	if err != nil {
		t.Fatalf("LowerWithSource: %v", err)
	}
	if len(module.EntryPoints) != 1 || module.EntryPoints[0].Name != "main" {
		t.Fatalf("entry points = %+v, want [main]", module.EntryPoints)
	}
	errs, err := shader.Validate(module)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("Validate errors: %v", errs)
	}
}

func TestLowerWithCapabilities(t *testing.T) {
	ast, err := shader.Parse(f64WGSL)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := shader.LowerWithCapabilities(ast, f64WGSL, 0); err == nil {
		t.Fatal("LowerWithCapabilities with no capabilities accepted f64")
	}
	if _, err := shader.LowerWithCapabilities(ast, f64WGSL, ir.CapAll); err != nil {
		t.Fatalf("LowerWithCapabilities with CapAll: %v", err)
	}
}
