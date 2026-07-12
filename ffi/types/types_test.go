package types

import (
	"runtime"
	"testing"
	"unsafe"
)

// =============================================================================
// TypeDescriptor Constants Verification
// =============================================================================

func TestTypeDescriptorConstants(t *testing.T) {
	pointerSize := unsafe.Sizeof(uintptr(0))
	pointerAlign := unsafe.Alignof(uintptr(0))
	tests := []struct {
		name      string
		desc      *TypeDescriptor
		wantSize  uintptr
		wantAlign uintptr
		wantKind  TypeKind
	}{
		{"VoidTypeDescriptor", VoidTypeDescriptor, 1, 1, VoidType},
		{"IntTypeDescriptor", IntTypeDescriptor, 4, 4, IntType},
		{"FloatTypeDescriptor", FloatTypeDescriptor, 4, 4, FloatType},
		{"DoubleTypeDescriptor", DoubleTypeDescriptor, 8, 8, DoubleType},
		{"UInt8TypeDescriptor", UInt8TypeDescriptor, 1, 1, UInt8Type},
		{"SInt8TypeDescriptor", SInt8TypeDescriptor, 1, 1, SInt8Type},
		{"UInt16TypeDescriptor", UInt16TypeDescriptor, 2, 2, UInt16Type},
		{"SInt16TypeDescriptor", SInt16TypeDescriptor, 2, 2, SInt16Type},
		{"UInt32TypeDescriptor", UInt32TypeDescriptor, 4, 4, UInt32Type},
		{"SInt32TypeDescriptor", SInt32TypeDescriptor, 4, 4, SInt32Type},
		{"UInt64TypeDescriptor", UInt64TypeDescriptor, 8, 8, UInt64Type},
		{"SInt64TypeDescriptor", SInt64TypeDescriptor, 8, 8, SInt64Type},
		{"PointerTypeDescriptor", PointerTypeDescriptor, pointerSize, pointerAlign, PointerType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.desc.Size != tt.wantSize {
				t.Errorf("Size = %d, want %d", tt.desc.Size, tt.wantSize)
			}
			if tt.desc.Alignment != tt.wantAlign {
				t.Errorf("Alignment = %d, want %d", tt.desc.Alignment, tt.wantAlign)
			}
			if tt.desc.Kind != tt.wantKind {
				t.Errorf("Kind = %d, want %d", tt.desc.Kind, tt.wantKind)
			}
		})
	}
}

// =============================================================================
// CallingConvention Verification
// =============================================================================

func TestCallingConventionValues(t *testing.T) {
	if UnixCallingConvention != 1 {
		t.Errorf("UnixCallingConvention = %d, want 1", UnixCallingConvention)
	}
	if WindowsCallingConvention != 2 {
		t.Errorf("WindowsCallingConvention = %d, want 2", WindowsCallingConvention)
	}
	if GnuWindowsCallingConvention != 3 {
		t.Errorf("GnuWindowsCallingConvention = %d, want 3", GnuWindowsCallingConvention)
	}
	if DefaultCall != 0 {
		t.Errorf("DefaultCall = %d, want 0", DefaultCall)
	}
}

func TestDefaultConvention(t *testing.T) {
	conv := DefaultConvention()
	switch runtime.GOOS {
	case "windows":
		if conv != WindowsCallingConvention {
			t.Errorf("DefaultConvention() on windows = %d, want %d", conv, WindowsCallingConvention)
		}
	default:
		if conv != UnixCallingConvention {
			t.Errorf("DefaultConvention() on %s = %d, want %d", runtime.GOOS, conv, UnixCallingConvention)
		}
	}
}

func TestCDeclAlias(t *testing.T) {
	// CDecl is UnixCallingConvention (updated by init on Windows)
	if runtime.GOOS != "windows" && CDecl != UnixCallingConvention {
		t.Errorf("CDecl on %s = %d, want %d", runtime.GOOS, CDecl, UnixCallingConvention)
	}
}

func TestStdCallAlias(t *testing.T) {
	if StdCall != WindowsCallingConvention {
		t.Errorf("StdCall = %d, want %d", StdCall, WindowsCallingConvention)
	}
}

// =============================================================================
// TypeKind Constants Verification
// =============================================================================

func TestTypeKindValues(t *testing.T) {
	if VoidType != 0 {
		t.Errorf("VoidType = %d, want 0", VoidType)
	}
	if IntType != 1 {
		t.Errorf("IntType = %d, want 1", IntType)
	}
	if FloatType != 2 {
		t.Errorf("FloatType = %d, want 2", FloatType)
	}
	if DoubleType != 3 {
		t.Errorf("DoubleType = %d, want 3", DoubleType)
	}
	if UInt8Type != 4 {
		t.Errorf("UInt8Type = %d, want 4", UInt8Type)
	}
	if SInt8Type != 5 {
		t.Errorf("SInt8Type = %d, want 5", SInt8Type)
	}
	if UInt16Type != 6 {
		t.Errorf("UInt16Type = %d, want 6", UInt16Type)
	}
	if SInt16Type != 7 {
		t.Errorf("SInt16Type = %d, want 7", SInt16Type)
	}
	if UInt32Type != 8 {
		t.Errorf("UInt32Type = %d, want 8", UInt32Type)
	}
	if SInt32Type != 9 {
		t.Errorf("SInt32Type = %d, want 9", SInt32Type)
	}
	if UInt64Type != 10 {
		t.Errorf("UInt64Type = %d, want 10", UInt64Type)
	}
	if SInt64Type != 11 {
		t.Errorf("SInt64Type = %d, want 11", SInt64Type)
	}
	if StructType != 12 {
		t.Errorf("StructType = %d, want 12", StructType)
	}
	if PointerType != 13 {
		t.Errorf("PointerType = %d, want 13", PointerType)
	}
}

// =============================================================================
// CallInterface Struct Verification
// =============================================================================

func TestCallInterfaceDefaults(t *testing.T) {
	var cif CallInterface
	if cif.Convention != 0 {
		t.Errorf("default Convention = %d, want 0", cif.Convention)
	}
	if cif.ArgCount != 0 {
		t.Errorf("default ArgCount = %d, want 0", cif.ArgCount)
	}
	if cif.ArgTypes != nil {
		t.Errorf("default ArgTypes = %v, want nil", cif.ArgTypes)
	}
	if cif.ReturnType != nil {
		t.Errorf("default ReturnType = %v, want nil", cif.ReturnType)
	}
	if cif.Flags != 0 {
		t.Errorf("default Flags = %d, want 0", cif.Flags)
	}
	if cif.StackBytes != 0 {
		t.Errorf("default StackBytes = %d, want 0", cif.StackBytes)
	}
	if cif.FixedArgCount != 0 {
		t.Errorf("default FixedArgCount = %d, want 0", cif.FixedArgCount)
	}
}

func TestCallInterfaceFilled(t *testing.T) {
	cif := CallInterface{
		Convention:    UnixCallingConvention,
		ArgCount:      2,
		ArgTypes:      []*TypeDescriptor{UInt32TypeDescriptor, PointerTypeDescriptor},
		ReturnType:    SInt32TypeDescriptor,
		Flags:         ReturnUInt32,
		StackBytes:    16,
		FixedArgCount: 0,
	}
	if cif.Convention != UnixCallingConvention {
		t.Errorf("Convention = %d, want %d", cif.Convention, UnixCallingConvention)
	}
	if cif.ArgCount != 2 {
		t.Errorf("ArgCount = %d, want 2", cif.ArgCount)
	}
	if len(cif.ArgTypes) != 2 {
		t.Errorf("len(ArgTypes) = %d, want 2", len(cif.ArgTypes))
	}
	if cif.ReturnType != SInt32TypeDescriptor {
		t.Errorf("ReturnType = %v, want SInt32TypeDescriptor", cif.ReturnType)
	}
}

// =============================================================================
// Return Flags Constants Verification
// =============================================================================

func TestReturnFlags(t *testing.T) {
	if ReturnVoid != 0 {
		t.Errorf("ReturnVoid = %d, want 0", ReturnVoid)
	}
	if ReturnUInt8 != 1 {
		t.Errorf("ReturnUInt8 = %d, want 1", ReturnUInt8)
	}
	if ReturnUInt16 != 2 {
		t.Errorf("ReturnUInt16 = %d, want 2", ReturnUInt16)
	}
	if ReturnUInt32 != 3 {
		t.Errorf("ReturnUInt32 = %d, want 3", ReturnUInt32)
	}
	if ReturnSInt8 != 4 {
		t.Errorf("ReturnSInt8 = %d, want 4", ReturnSInt8)
	}
	if ReturnSInt16 != 5 {
		t.Errorf("ReturnSInt16 = %d, want 5", ReturnSInt16)
	}
	if ReturnSInt32 != 6 {
		t.Errorf("ReturnSInt32 = %d, want 6", ReturnSInt32)
	}
	if ReturnInt64 != 7 {
		t.Errorf("ReturnInt64 = %d, want 7", ReturnInt64)
	}
	if ReturnInXMM32 != 8 {
		t.Errorf("ReturnInXMM32 = %d, want 8", ReturnInXMM32)
	}
	if ReturnInXMM64 != 9 {
		t.Errorf("ReturnInXMM64 = %d, want 9", ReturnInXMM64)
	}
	if ReturnStRaxRdx != 10 {
		t.Errorf("ReturnStRaxRdx = %d, want 10", ReturnStRaxRdx)
	}
	if ReturnStRaxXmm0 != 11 {
		t.Errorf("ReturnStRaxXmm0 = %d, want 11", ReturnStRaxXmm0)
	}
	if ReturnStXmm0Rax != 12 {
		t.Errorf("ReturnStXmm0Rax = %d, want 12", ReturnStXmm0Rax)
	}
	if ReturnStXmm0Xmm1 != 13 {
		t.Errorf("ReturnStXmm0Xmm1 = %d, want 13", ReturnStXmm0Xmm1)
	}
	if ReturnViaPointer != 1<<10 {
		t.Errorf("ReturnViaPointer = %d, want %d", ReturnViaPointer, 1<<10)
	}
	if ReturnHFA2 != 1<<11 {
		t.Errorf("ReturnHFA2 = %d, want %d", ReturnHFA2, 1<<11)
	}
	if ReturnHFA3 != 1<<12 {
		t.Errorf("ReturnHFA3 = %d, want %d", ReturnHFA3, 1<<12)
	}
	if ReturnHFA4 != 1<<13 {
		t.Errorf("ReturnHFA4 = %d, want %d", ReturnHFA4, 1<<13)
	}
}

// =============================================================================
// Error Variables Verification
// =============================================================================

func TestErrorVariables(t *testing.T) {
	if ErrUnsupportedArchitecture == nil {
		t.Error("ErrUnsupportedArchitecture is nil")
	}
	if ErrUnsupportedCallingConvention == nil {
		t.Error("ErrUnsupportedCallingConvention is nil")
	}
	if ErrInvalidTypeDefinition == nil {
		t.Error("ErrInvalidTypeDefinition is nil")
	}
	if ErrUnsupportedReturnType == nil {
		t.Error("ErrUnsupportedReturnType is nil")
	}

	// Verify error messages
	if ErrUnsupportedArchitecture.Error() != "unsupported architecture" {
		t.Errorf("ErrUnsupportedArchitecture message = %q, want %q",
			ErrUnsupportedArchitecture.Error(), "unsupported architecture")
	}
	if ErrUnsupportedCallingConvention.Error() != "unsupported calling convention" {
		t.Errorf("ErrUnsupportedCallingConvention message = %q, want %q",
			ErrUnsupportedCallingConvention.Error(), "unsupported calling convention")
	}
}

// =============================================================================
// RuntimeEnvironment Verification
// =============================================================================

func TestRuntimeEnvironment(t *testing.T) {
	os, arch := RuntimeEnvironment()
	if os != runtime.GOOS {
		t.Errorf("RuntimeEnvironment() os = %q, want %q", os, runtime.GOOS)
	}
	if arch != runtime.GOARCH {
		t.Errorf("RuntimeEnvironment() arch = %q, want %q", arch, runtime.GOARCH)
	}
}

// =============================================================================
// TypeDescriptor as Vulkan/GLES Signature Template
// =============================================================================

// TestVulkanLikeSignature verifies that the type descriptors can be used to build
// signature templates similar to how they're used in wgpu/hal/vulkan/vk/signatures.go.
func TestVulkanLikeSignature(t *testing.T) {
	// Simulate Vulkan signature patterns:
	// VkResult(handle, ptr, ptr) = int32(uint64, ptr, ptr)
	cif := CallInterface{}
	cif.Convention = DefaultCall
	cif.ArgCount = 3
	cif.ArgTypes = []*TypeDescriptor{UInt64TypeDescriptor, PointerTypeDescriptor, PointerTypeDescriptor}
	cif.ReturnType = SInt32TypeDescriptor

	if len(cif.ArgTypes) != 3 {
		t.Errorf("Vulkan signature should have 3 args, got %d", len(cif.ArgTypes))
	}
	if cif.ArgTypes[0] != UInt64TypeDescriptor {
		t.Errorf("Arg 0 should be UInt64TypeDescriptor")
	}
	if cif.ArgTypes[1] != PointerTypeDescriptor {
		t.Errorf("Arg 1 should be PointerTypeDescriptor")
	}
	if cif.ArgTypes[2] != PointerTypeDescriptor {
		t.Errorf("Arg 2 should be PointerTypeDescriptor")
	}
	if cif.ReturnType != SInt32TypeDescriptor {
		t.Errorf("Return type should be SInt32TypeDescriptor")
	}
}

// TestGLESLikeSignature verifies the type descriptors match GLES usage patterns.
func TestGLESLikeSignature(t *testing.T) {
	// Simulate GLES signatures:
	// void fn(float, float, float, float) - glClearColor
	cif := CallInterface{}
	cif.Convention = DefaultCall
	cif.ArgCount = 4
	cif.ArgTypes = []*TypeDescriptor{
		FloatTypeDescriptor,
		FloatTypeDescriptor,
		FloatTypeDescriptor,
		FloatTypeDescriptor,
	}
	cif.ReturnType = VoidTypeDescriptor

	if len(cif.ArgTypes) != 4 {
		t.Errorf("GLES signature should have 4 args, got %d", len(cif.ArgTypes))
	}
	for i, arg := range cif.ArgTypes {
		if arg != FloatTypeDescriptor {
			t.Errorf("Arg %d should be FloatTypeDescriptor", i)
		}
	}
	if cif.ReturnType != VoidTypeDescriptor {
		t.Errorf("Return type should be VoidTypeDescriptor")
	}
}

// TestMixedSignature verifies a mixed-type signature like Vulkan uses.
func TestMixedSignature(t *testing.T) {
	// void(handle, u32, u32, ptr, ptr) - vkCmdBindVertexBuffers
	cif := CallInterface{}
	cif.Convention = DefaultCall
	cif.ArgCount = 5
	cif.ArgTypes = []*TypeDescriptor{
		UInt64TypeDescriptor,  // VkCommandBuffer
		UInt32TypeDescriptor,  // firstBinding
		UInt32TypeDescriptor,  // bindingCount
		PointerTypeDescriptor, // pBuffers
		PointerTypeDescriptor, // pOffsets
	}
	cif.ReturnType = VoidTypeDescriptor

	if len(cif.ArgTypes) != 5 {
		t.Errorf("Mixed signature should have 5 args, got %d", len(cif.ArgTypes))
	}
	if cif.ArgTypes[0] != UInt64TypeDescriptor {
		t.Errorf("Arg 0 should be UInt64TypeDescriptor")
	}
	if cif.ArgTypes[1] != UInt32TypeDescriptor {
		t.Errorf("Arg 1 should be UInt32TypeDescriptor")
	}
	if cif.ArgTypes[4] != PointerTypeDescriptor {
		t.Errorf("Arg 4 should be PointerTypeDescriptor")
	}
}
