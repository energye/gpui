package codegen

import (
	"encoding/binary"
	"testing"

	"github.com/energye/gpui/gpu/shader/wgsl"
)

// Shader A: uses var + if/else to assign result, then reads it after merge.
// Control-flow validity is gated by TestVarIfElseSPIRV_ControlFlowValidation.
const shaderAVarIfElse = `
struct Params { value: f32, flag: u32, width: u32, height: u32 }
@group(0) @binding(0) var<uniform> params: Params;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let idx = gid.x;
    if idx >= params.width * params.height { return; }

    var result: f32;
    if params.flag != 0u {
        result = params.value * 2.0;
    } else {
        result = params.value;
    }
    out[idx] = u32(result * 255.0);
}
`

// compileWGSLToSPIRV compiles WGSL source to SPIR-V bytes.
func compileWGSLToSPIRV(t *testing.T, label, source string) []byte {
	t.Helper()

	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("[%s] Tokenize failed: %v", label, err)
	}

	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("[%s] Parse failed: %v", label, err)
	}

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("[%s] Lower failed: %v", label, err)
	}

	opts := Options{
		Version: Version1_3,
		Debug:   true,
	}
	backend := NewBackend(opts)
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("[%s] SPIR-V compile failed: %v", label, err)
	}

	validateSPIRVBinary(t, spirvBytes)
	return spirvBytes
}

// spirvInstruction represents a decoded SPIR-V instruction with offset info.
// Shared by package-level SPIR-V analysis tests (coverage, loops, atomics, …).
type spirvInstruction struct {
	offset    int
	opcode    OpCode
	wordCount int
	words     []uint32
}

// decodeSPIRVInstructions parses all instructions from SPIR-V binary (skipping header).
func decodeSPIRVInstructions(data []byte) []spirvInstruction {
	if len(data) < 20 || len(data)%4 != 0 {
		return nil
	}

	words := make([]uint32, len(data)/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(data[i*4:])
	}

	var instrs []spirvInstruction
	offset := 5 // skip header
	for offset < len(words) {
		wc := int(words[offset] >> 16)
		op := OpCode(words[offset] & 0xFFFF)
		if wc == 0 || offset+wc > len(words) {
			break
		}
		instrs = append(instrs, spirvInstruction{
			offset:    offset,
			opcode:    op,
			wordCount: wc,
			words:     words[offset : offset+wc],
		})
		offset += wc
	}
	return instrs
}

// TestVarIfElseSPIRV_ControlFlowValidation runs the control flow validator on Shader A.
func TestVarIfElseSPIRV_ControlFlowValidation(t *testing.T) {
	spirvA := compileWGSLToSPIRV(t, "ShaderA", shaderAVarIfElse)

	t.Log("Running control flow validation on Shader A (var + if/else)...")
	validateSPIRVControlFlow(t, spirvA)
	t.Log("Control flow validation passed!")
}
