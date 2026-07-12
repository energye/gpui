// Package ffi provides a Foreign Function Interface for calling C functions from Go
// without CGO. It enables direct calls to C libraries with full type safety and
// platform abstraction.
//
// This package mirrors the API of github.com/go-webgpu/goffi/ffi to enable a
// drop-in replacement of the import path. Internally, it uses github.com/ebitengine/purego.
//
// # Basic Usage
//
//	// Load a library
//	handle, err := ffi.LoadLibrary("libm.so.6")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Get a function pointer
//	sqrtPtr, err := ffi.GetSymbol(handle, "sqrt")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Prepare call interface
//	var cif types.CallInterface
//	err = ffi.PrepareCallInterface(
//	    &cif,
//	    types.DefaultCall,
//	    types.DoubleTypeDescriptor,
//	    []*types.TypeDescriptor{types.DoubleTypeDescriptor},
//	)
//
//	// Call the function
//	var result float64
//	arg := 16.0
//	err = ffi.CallFunction(
//	    &cif,
//	    sqrtPtr,
//	    unsafe.Pointer(&result),
//	    []unsafe.Pointer{unsafe.Pointer(&arg)},
//	)
//	// result is now 4.0
//
// # Performance
//
// Overhead is approximately 5-15ns per call, which is negligible for most use
// cases (e.g., WebGPU rendering).
//
// # Thread Safety
//
//   - PrepareCallInterface and CallFunction are safe to call concurrently with
//     different CallInterface instances
//   - DO NOT use the same CallInterface from multiple goroutines simultaneously
//     without external synchronization
//   - Library handles (from LoadLibrary) are safe to use concurrently for
//     read operations (GetSymbol)
//   - DO NOT call FreeLibrary while other goroutines are using GetSymbol on the
//     same handle
package ffi

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/energye/gpui/ffi/types"
)

// RTLD constants from <dlfcn.h> for dynamic library loading.
const (
	// RTLD_LAZY resolves symbols on demand (lazy binding).
	RTLD_LAZY = 0x1

	// RTLD_NOW resolves all symbols when loading the library (recommended).
	RTLD_NOW = 0x2

	// RTLD_GLOBAL makes symbols available for subsequently loaded libraries.
	RTLD_GLOBAL = 0x100
)

// =============================================================================
// Error Types
// =============================================================================

// InvalidCallInterfaceError indicates CallInterface preparation failed due to
// invalid parameters.
type InvalidCallInterfaceError struct {
	Field  string // Which field was invalid ("cif", "returnType", "argTypes", etc.)
	Reason string // Why it was invalid (human-readable description)
	Index  int    // For array fields like argTypes (-1 if not applicable)
}

func (e *InvalidCallInterfaceError) Error() string {
	if e.Index >= 0 {
		return fmt.Sprintf("invalid call interface: %s[%d]: %s", e.Field, e.Index, e.Reason)
	}
	return fmt.Sprintf("invalid call interface: %s: %s", e.Field, e.Reason)
}

// Is implements error equality for errors.Is().
func (e *InvalidCallInterfaceError) Is(target error) bool {
	_, ok := target.(*InvalidCallInterfaceError)
	return ok
}

// UnsupportedPlatformError indicates the current platform is not supported by FFI.
type UnsupportedPlatformError struct {
	OS   string // Operating system (e.g., "linux", "windows", "darwin")
	Arch string // Architecture (e.g., "amd64", "arm64")
}

func (e *UnsupportedPlatformError) Error() string {
	return fmt.Sprintf("unsupported platform: %s/%s (FFI not implemented for this platform)",
		e.OS, e.Arch)
}

// Is implements error equality for errors.Is().
func (e *UnsupportedPlatformError) Is(target error) bool {
	_, ok := target.(*UnsupportedPlatformError)
	return ok
}

// LibraryError wraps dynamic library loading and symbol resolution errors.
type LibraryError struct {
	Operation string // "load", "symbol", or "free"
	Name      string // Library path or symbol name
	Err       error  // Underlying OS error (can be nil)
}

func (e *LibraryError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("library %s failed for %q: %v", e.Operation, e.Name, e.Err)
	}
	return fmt.Sprintf("library %s failed for %q", e.Operation, e.Name)
}

// Unwrap returns the underlying error for errors.Unwrap().
func (e *LibraryError) Unwrap() error {
	return e.Err
}

// Is implements error equality for errors.Is().
func (e *LibraryError) Is(target error) bool {
	_, ok := target.(*LibraryError)
	return ok
}

// CallingConventionError indicates an unsupported or invalid calling convention.
type CallingConventionError struct {
	Convention int    // The invalid convention value
	Platform   string // Current platform (OS/Arch)
	Reason     string // Why it's not supported
}

func (e *CallingConventionError) Error() string {
	return fmt.Sprintf("unsupported calling convention %d on %s: %s",
		e.Convention, e.Platform, e.Reason)
}

// Is implements error equality for errors.Is().
func (e *CallingConventionError) Is(target error) bool {
	_, ok := target.(*CallingConventionError)
	return ok
}

// TypeValidationError indicates a type descriptor failed validation.
type TypeValidationError struct {
	TypeName string // Name or description of the type
	Kind     int    // The TypeKind value that failed
	Reason   string // Why validation failed
	Index    int    // For composite types (-1 if not applicable)
}

func (e *TypeValidationError) Error() string {
	if e.Index >= 0 {
		return fmt.Sprintf("type validation failed for %s[%d] (kind=%d): %s",
			e.TypeName, e.Index, e.Kind, e.Reason)
	}
	if e.TypeName != "" {
		return fmt.Sprintf("type validation failed for %s (kind=%d): %s",
			e.TypeName, e.Kind, e.Reason)
	}
	return fmt.Sprintf("type validation failed (kind=%d): %s", e.Kind, e.Reason)
}

// Is implements error equality for errors.Is().
func (e *TypeValidationError) Is(target error) bool {
	_, ok := target.(*TypeValidationError)
	return ok
}

// Deprecated: Legacy sentinel errors kept for backwards compatibility.
var (
	// ErrInvalidCallInterface is deprecated. Use InvalidCallInterfaceError instead.
	ErrInvalidCallInterface = &InvalidCallInterfaceError{
		Field:  "unknown",
		Reason: "invalid call interface",
		Index:  -1,
	}

	// ErrFunctionCallFailed is deprecated.
	ErrFunctionCallFailed = fmt.Errorf("function call failed")
)

// ErrTooManyArguments is returned when the argument count exceeds the platform limit.
var ErrTooManyArguments = errors.New("goffi: argument count exceeds platform limit")

// Helper functions for creating common errors

func newInvalidTypeError(typeName string, kind int, reason string) error {
	return &TypeValidationError{
		TypeName: typeName,
		Kind:     kind,
		Reason:   reason,
		Index:    -1,
	}
}

func newInvalidTypeAtIndexError(typeName string, kind int, index int, reason string) error {
	return &TypeValidationError{
		TypeName: typeName,
		Kind:     kind,
		Reason:   reason,
		Index:    index,
	}
}

// =============================================================================
// Library Loading
// =============================================================================

// LoadLibrary loads a shared library using dlopen.
//
// This function loads the specified shared library and returns a handle for use
// with GetSymbol. The library is loaded with RTLD_NOW|RTLD_GLOBAL flags.
//
// Parameters:
//   - name: Path to the shared library (e.g., "libm.so.6", "/usr/lib/libGL.so.1")
//
// Returns:
//   - Handle to the loaded library (use with GetSymbol and FreeLibrary)
//   - Error if loading fails
//
// Note: Always pair LoadLibrary with FreeLibrary to prevent resource leaks.
func LoadLibrary(name string) (unsafe.Pointer, error) {
	handle, err := purego.Dlopen(name, RTLD_NOW|RTLD_GLOBAL)
	if err != nil {
		return nil, &LibraryError{
			Operation: "load",
			Name:      name,
			Err:       err,
		}
	}
	//nolint:govet // handle is a dlopen result (non-Go memory); double-indirection per go.dev/issue/58625
	return *(*unsafe.Pointer)(unsafe.Pointer(&handle)), nil
}

// GetSymbol retrieves a function pointer from a loaded library using dlsym.
//
// This function looks up a symbol (function or variable) in the loaded library
// and returns its address for use with CallFunction.
//
// Parameters:
//   - handle: Library handle from LoadLibrary
//   - name: Name of the symbol to retrieve (e.g., "sqrt", "glClear")
//
// Returns:
//   - Function pointer (use with CallFunction)
//   - Error if symbol not found or lookup fails
//
// Note: The returned pointer is only valid while the library remains loaded.
func GetSymbol(handle unsafe.Pointer, name string) (unsafe.Pointer, error) {
	fnPtr, err := purego.Dlsym(uintptr(handle), name)
	if err != nil {
		return nil, &LibraryError{
			Operation: "symbol",
			Name:      name,
			Err:       err,
		}
	}

	if fnPtr == 0 {
		return nil, &LibraryError{
			Operation: "symbol",
			Name:      name,
			Err:       fmt.Errorf("symbol not found"),
		}
	}

	//nolint:govet // fnPtr is a dlsym result (non-Go memory); double-indirection per go.dev/issue/58625
	return *(*unsafe.Pointer)(unsafe.Pointer(&fnPtr)), nil
}

// FreeLibrary unloads a previously loaded library using dlclose.
//
// This function decrements the reference count of the loaded library. When the
// reference count reaches zero, the library is unloaded from memory.
//
// Parameters:
//   - handle: Library handle from LoadLibrary (can be nil)
//
// Returns:
//   - nil on success
//   - Error if the library could not be unloaded
//
// Safety:
//   - Do not use function pointers obtained from this library after FreeLibrary
//   - Always pair LoadLibrary with FreeLibrary to prevent resource leaks
//   - Safe to call with nil handle (returns nil without error)
func FreeLibrary(handle unsafe.Pointer) error {
	if handle == nil {
		return nil // Allow nil handle for convenience
	}

	err := purego.Dlclose(uintptr(handle))
	if err != nil {
		return &LibraryError{
			Operation: "free",
			Name:      "<library handle>",
			Err:       err,
		}
	}
	return nil
}

// =============================================================================
// Call Interface Preparation
// =============================================================================

// PrepareCallInterface prepares a function call interface for calling a C function.
//
// This function initializes the CallInterface structure with the necessary metadata
// for making FFI calls. It must be called before CallFunction.
//
// Parameters:
//   - cif: Pointer to CallInterface structure to initialize (must not be nil)
//   - convention: Calling convention (types.DefaultCall, types.CDecl, types.StdCall, etc.)
//   - returnType: Type descriptor for return value (use types.VoidTypeDescriptor for void)
//   - argTypes: Slice of type descriptors for each argument (nil or empty slice for no arguments)
//
// Returns:
//   - nil on success
//   - ErrInvalidCallInterface if parameters are invalid
//   - Other errors if type validation or platform preparation fails
func PrepareCallInterface(
	cif *types.CallInterface,
	convention types.CallingConvention,
	returnType *types.TypeDescriptor,
	argTypes []*types.TypeDescriptor,
) error {
	if cif == nil {
		return &InvalidCallInterfaceError{
			Field:  "cif",
			Reason: "must not be nil",
			Index:  -1,
		}
	}
	if returnType == nil {
		return &InvalidCallInterfaceError{
			Field:  "returnType",
			Reason: "must not be nil",
			Index:  -1,
		}
	}

	// Auto-resolve DefaultCall to platform-specific convention
	if convention == types.DefaultCall {
		convention = types.DefaultConvention()
	}

	// Validate calling convention
	if convention < types.UnixCallingConvention || convention > types.GnuWindowsCallingConvention {
		return &CallingConventionError{
			Convention: int(convention),
			Platform:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			Reason:     "value must be 1 (Unix), 2 (Windows), or 3 (GNU Windows)",
		}
	}

	// Validate return type
	if returnType.Kind == types.StructType && returnType.Size == 0 {
		if err := initializeCompositeType(returnType); err != nil {
			return err
		}
	}
	if !isValidType(returnType) {
		return newInvalidTypeError("returnType", int(returnType.Kind), "unsupported type kind")
	}

	// Validate argument types
	argCount := len(argTypes)
	for i, t := range argTypes {
		if t == nil {
			return newInvalidTypeAtIndexError("argTypes", -1, i, "must not be nil")
		}
		if t.Kind == types.StructType && t.Size == 0 {
			if err := initializeCompositeType(t); err != nil {
				return fmt.Errorf("argument type at index %d: %w", i, err)
			}
		}
		if !isValidType(t) {
			return newInvalidTypeAtIndexError("argTypes", int(t.Kind), i, "unsupported type kind")
		}
	}

	// Store metadata in the CIF
	cif.Convention = convention
	cif.ArgCount = argCount
	cif.ArgTypes = argTypes
	cif.ReturnType = returnType
	cif.Flags = 0
	cif.StackBytes = 0
	cif.FixedArgCount = 0

	return nil
}

// PrepareVariadicCallInterface prepares a call interface for a C variadic function.
//
// nfixedargs is the count of fixed parameters before '...' in the C prototype.
// argTypes must contain ALL arguments (fixed + variadic) for this specific call.
// A new CIF must be prepared for each unique combination of variadic argument types.
func PrepareVariadicCallInterface(
	cif *types.CallInterface,
	convention types.CallingConvention,
	nfixedargs int,
	returnType *types.TypeDescriptor,
	argTypes []*types.TypeDescriptor,
) error {
	if nfixedargs < 0 {
		return errors.New("goffi: nfixedargs must be non-negative")
	}
	if nfixedargs > len(argTypes) {
		return errors.New("goffi: nfixedargs exceeds total argument count")
	}
	if err := PrepareCallInterface(cif, convention, returnType, argTypes); err != nil {
		return err
	}
	cif.FixedArgCount = nfixedargs
	return nil
}

// =============================================================================
// Function Calling
// =============================================================================

// CallFunctionContext executes a C function call with context support.
//
// This function performs the actual FFI call to the C function, handling all
// platform-specific calling convention details automatically. It checks the
// context before executing to prevent starting expensive operations when the
// context is already cancelled or has exceeded its deadline.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - cif: Prepared call interface (from PrepareCallInterface)
//   - fn: Function pointer obtained from GetSymbol (must not be nil)
//   - rvalue: Pointer to buffer for return value (can be nil for void functions)
//   - avalue: Slice of pointers to argument values (length must match argCount)
func CallFunctionContext(
	ctx context.Context,
	cif *types.CallInterface,
	fn unsafe.Pointer,
	rvalue unsafe.Pointer,
	avalue []unsafe.Pointer,
) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// Check context before expensive call
	if err := ctx.Err(); err != nil {
		return err
	}

	if cif == nil {
		return &InvalidCallInterfaceError{
			Field:  "cif",
			Reason: "must not be nil",
			Index:  -1,
		}
	}
	if fn == nil {
		return &InvalidCallInterfaceError{
			Field:  "fn",
			Reason: "function pointer must not be nil",
			Index:  -1,
		}
	}

	// Build the uintptr argument list for SyscallN from the []unsafe.Pointer args.
	// Each element in avalue is a pointer to the actual argument value in memory.
	argCount := cif.ArgCount
	if argCount == 0 {
		argCount = len(avalue)
	}

	if len(avalue) != argCount {
		return &InvalidCallInterfaceError{
			Field:  "avalue",
			Reason: fmt.Sprintf("argument count mismatch: got %d, want %d", len(avalue), argCount),
			Index:  -1,
		}
	}

	// Float/double and struct values need ABI-aware typed calls. purego.SyscallN
	// only has integer uintptr parameters, so use purego.RegisterFunc whenever
	// the signature contains values that may use FP registers or aggregate ABI
	// classification.
	if needsReflectCall(cif) {
		return callReflect(ctx, cif, fn, rvalue, avalue, argCount)
	}

	args := make([]uintptr, 0, argCount)
	for i := 0; i < argCount; i++ {
		// Determine the type of this argument
		var argType *types.TypeDescriptor
		if i < len(cif.ArgTypes) {
			argType = cif.ArgTypes[i]
		}

		// Get the pointer to the argument value
		var argPtr unsafe.Pointer
		if i < len(avalue) {
			argPtr = avalue[i]
		}

		// Convert the argument value to uintptr based on its type
		if argPtr == nil {
			args = append(args, 0)
			continue
		}

		args = append(args, readArgAsUintptr(argPtr, argType))
	}

	// Call the function via purego.SyscallN
	r1, _, _ := purego.SyscallN(uintptr(fn), args...)

	// Write the return value if needed
	if rvalue != nil && cif.ReturnType != nil && cif.ReturnType.Kind != types.VoidType {
		writeReturnValue(rvalue, r1, cif.ReturnType)
	}

	return nil
}

// CallFunction executes a C function call without context support.
//
// This is equivalent to CallFunctionContext(context.Background(), cif, fn, rvalue, avalue).
func CallFunction(
	cif *types.CallInterface,
	fn unsafe.Pointer,
	rvalue unsafe.Pointer,
	avalue []unsafe.Pointer,
) error {
	return CallFunctionContext(context.Background(), cif, fn, rvalue, avalue)
}

// readArgAsUintptr reads the value at the given pointer and converts it to uintptr
// based on the type descriptor.
//
// IMPORTANT: ALL avalue elements are pointers to the actual argument values,
// regardless of type. This matches the goffi convention used by the Vulkan/GLES
// backends:
//
//	uint32 arg:  unsafe.Pointer(&myUint32)   → read *(*uint32)(ptr)
//	void*  arg:  unsafe.Pointer(&myPtr)      → read *(*uintptr)(ptr)
//	                 where myPtr = unsafe.Pointer(&data[0])
func readArgAsUintptr(ptr unsafe.Pointer, t *types.TypeDescriptor) uintptr {
	if t == nil {
		// Default: read as uintptr (pointer-sized value)
		return *(*uintptr)(ptr)
	}

	switch t.Kind {
	case types.VoidType:
		return 0
	case types.IntType, types.UInt32Type, types.SInt32Type:
		return uintptr(*(*uint32)(ptr))
	case types.UInt8Type, types.SInt8Type:
		return uintptr(*(*uint8)(ptr))
	case types.UInt16Type, types.SInt16Type:
		return uintptr(*(*uint16)(ptr))
	case types.UInt64Type, types.SInt64Type:
		return uintptr(*(*uint64)(ptr))
	case types.FloatType:
		return uintptr(math.Float32bits(*(*float32)(ptr)))
	case types.DoubleType:
		return uintptr(math.Float64bits(*(*float64)(ptr)))
	case types.PointerType:
		// Pointer-to-pointer convention: avalue element is a pointer TO the pointer.
		// Read the pointer value stored at ptr.
		return *(*uintptr)(ptr)
	default:
		// Fallback: read as uintptr
		return *(*uintptr)(ptr)
	}
}

// writeReturnValue writes the return value from SyscallN to the result buffer
// based on the return type descriptor.
func writeReturnValue(rvalue unsafe.Pointer, r1 uintptr, t *types.TypeDescriptor) {
	if rvalue == nil {
		return
	}

	switch t.Kind {
	case types.VoidType:
		// Nothing to write
	case types.IntType, types.UInt32Type, types.SInt32Type:
		*(*uint32)(rvalue) = uint32(r1)
	case types.UInt8Type, types.SInt8Type:
		*(*uint8)(rvalue) = uint8(r1)
	case types.UInt16Type, types.SInt16Type:
		*(*uint16)(rvalue) = uint16(r1)
	case types.UInt64Type, types.SInt64Type:
		*(*uint64)(rvalue) = uint64(r1)
	case types.FloatType:
		*(*float32)(rvalue) = math.Float32frombits(uint32(r1))
	case types.DoubleType:
		*(*float64)(rvalue) = math.Float64frombits(uint64(r1))
	case types.PointerType:
		*(*uintptr)(rvalue) = r1
	default:
		*(*uintptr)(rvalue) = r1
	}
}

// =============================================================================
// Typed ABI Support
// =============================================================================

// needsReflectCall reports whether the signature contains values that cannot be
// represented correctly with purego.SyscallN's uintptr-only parameter path.
func needsReflectCall(cif *types.CallInterface) bool {
	if cif == nil {
		return false
	}
	if needsTypedABI(cif.ReturnType) {
		return true
	}
	for _, argType := range cif.ArgTypes {
		if needsTypedABI(argType) {
			return true
		}
	}
	return false
}

func needsTypedABI(t *types.TypeDescriptor) bool {
	return t != nil && (t.Kind == types.FloatType || t.Kind == types.DoubleType || t.Kind == types.StructType)
}

// callReflect handles C function calls where any parameter or return value needs
// ABI classification that purego.SyscallN's uintptr-only path cannot represent:
// FP registers for float/double and aggregate rules for struct values.
//
// purego.RegisterFunc uses a typed Go function signature, so it can place and
// read FP registers and structs according to the target C calling convention.
func callReflect(
	ctx context.Context,
	cif *types.CallInterface,
	fn unsafe.Pointer,
	rvalue unsafe.Pointer,
	avalue []unsafe.Pointer,
	argCount int,
) error {
	// Build the function type using reflection
	inTypes := make([]reflect.Type, argCount)
	for i := 0; i < argCount; i++ {
		var argType *types.TypeDescriptor
		if i < len(cif.ArgTypes) {
			argType = cif.ArgTypes[i]
		}
		if argType == nil || argType.Kind == types.PointerType {
			inTypes[i] = reflect.TypeOf(uintptr(0))
		} else {
			inTypes[i] = typeDescToReflectType(argType)
		}
	}

	var outTypes []reflect.Type
	if cif.ReturnType != nil && cif.ReturnType.Kind != types.VoidType {
		outTypes = []reflect.Type{typeDescToReflectType(cif.ReturnType)}
	}
	funcType := reflect.FuncOf(inTypes, outTypes, false)

	// Create a function variable and register it with purego
	fnVar := reflect.New(funcType)
	purego.RegisterFunc(fnVar.Interface(), uintptr(fn))

	// Build argument values
	inValues := make([]reflect.Value, argCount)
	for i := 0; i < argCount; i++ {
		var argType *types.TypeDescriptor
		if i < len(cif.ArgTypes) {
			argType = cif.ArgTypes[i]
		}
		var argPtr unsafe.Pointer
		if i < len(avalue) {
			argPtr = avalue[i]
		}
		inValues[i] = readArgAsReflectValue(argPtr, argType, inTypes[i])
	}

	// Call the function
	outValues := fnVar.Elem().Call(inValues)

	// Write the return value
	if rvalue != nil && len(outValues) > 0 {
		writeReflectValueToRvalue(rvalue, outValues[0], cif.ReturnType)
	}

	return nil
}

// typeDescToReflectType converts a TypeDescriptor to a reflect.Type.
func typeDescToReflectType(t *types.TypeDescriptor) reflect.Type {
	if t == nil {
		return reflect.TypeOf(uintptr(0))
	}
	switch t.Kind {
	case types.VoidType:
		return reflect.TypeOf(uintptr(0))
	case types.IntType, types.SInt32Type:
		return reflect.TypeOf(int32(0))
	case types.UInt32Type:
		return reflect.TypeOf(uint32(0))
	case types.UInt8Type, types.SInt8Type:
		if t.Kind == types.SInt8Type {
			return reflect.TypeOf(int8(0))
		}
		return reflect.TypeOf(uint8(0))
	case types.UInt16Type:
		return reflect.TypeOf(uint16(0))
	case types.SInt16Type:
		return reflect.TypeOf(int16(0))
	case types.UInt64Type:
		return reflect.TypeOf(uint64(0))
	case types.SInt64Type:
		return reflect.TypeOf(int64(0))
	case types.FloatType:
		return reflect.TypeOf(float32(0))
	case types.DoubleType:
		return reflect.TypeOf(float64(0))
	case types.PointerType:
		return reflect.TypeOf(uintptr(0))
	case types.StructType:
		return structTypeToReflectType(t)
	default:
		return reflect.TypeOf(uintptr(0))
	}
}

func structTypeToReflectType(t *types.TypeDescriptor) reflect.Type {
	if t == nil || t.Kind != types.StructType {
		return reflect.TypeOf(uintptr(0))
	}
	fields := make([]reflect.StructField, 0, len(t.Members)+1)
	for i, member := range t.Members {
		fields = append(fields, reflect.StructField{
			Name: fmt.Sprintf("F%d", i),
			Type: typeDescToReflectType(member),
		})
	}
	if len(fields) == 0 {
		return reflect.StructOf(fields)
	}
	st := reflect.StructOf(fields)
	if t.Size > st.Size() {
		fields = append(fields, reflect.StructField{
			Name: fmt.Sprintf("Pad%d", len(fields)),
			Type: reflect.ArrayOf(int(t.Size-st.Size()), reflect.TypeOf(byte(0))),
		})
		st = reflect.StructOf(fields)
	}
	return st
}

// readArgAsReflectValue reads the value at the given pointer and converts it
// to a reflect.Value of the specified type.
func readArgAsReflectValue(ptr unsafe.Pointer, t *types.TypeDescriptor, targetType reflect.Type) reflect.Value {
	if ptr == nil {
		return reflect.Zero(targetType)
	}
	if t == nil {
		return reflect.ValueOf(*(*uintptr)(ptr))
	}

	switch t.Kind {
	case types.VoidType:
		return reflect.Zero(targetType)
	case types.IntType, types.SInt32Type:
		return reflect.ValueOf(*(*int32)(ptr)).Convert(targetType)
	case types.UInt32Type:
		return reflect.ValueOf(*(*uint32)(ptr)).Convert(targetType)
	case types.UInt8Type:
		return reflect.ValueOf(*(*uint8)(ptr)).Convert(targetType)
	case types.SInt8Type:
		return reflect.ValueOf(*(*int8)(ptr)).Convert(targetType)
	case types.UInt16Type:
		return reflect.ValueOf(*(*uint16)(ptr)).Convert(targetType)
	case types.SInt16Type:
		return reflect.ValueOf(*(*int16)(ptr)).Convert(targetType)
	case types.UInt64Type:
		return reflect.ValueOf(*(*uint64)(ptr)).Convert(targetType)
	case types.SInt64Type:
		return reflect.ValueOf(*(*int64)(ptr)).Convert(targetType)
	case types.FloatType:
		return reflect.ValueOf(*(*float32)(ptr))
	case types.DoubleType:
		return reflect.ValueOf(*(*float64)(ptr))
	case types.PointerType:
		// Pointer-to-pointer convention: read the pointer value stored at ptr
		return reflect.ValueOf(*(*uintptr)(ptr))
	case types.StructType:
		return reflect.NewAt(targetType, ptr).Elem()
	default:
		return reflect.ValueOf(*(*uintptr)(ptr))
	}
}

// writeReflectValueToRvalue writes a reflect.Value to the rvalue buffer.
func writeReflectValueToRvalue(rvalue unsafe.Pointer, val reflect.Value, t *types.TypeDescriptor) {
	if rvalue == nil || t == nil {
		return
	}
	switch t.Kind {
	case types.VoidType:
		// Nothing to write
	case types.IntType, types.SInt32Type:
		*(*int32)(rvalue) = int32(val.Int())
	case types.UInt32Type:
		*(*uint32)(rvalue) = uint32(val.Uint())
	case types.UInt8Type:
		*(*uint8)(rvalue) = uint8(val.Uint())
	case types.SInt8Type:
		*(*int8)(rvalue) = int8(val.Int())
	case types.UInt16Type:
		*(*uint16)(rvalue) = uint16(val.Uint())
	case types.SInt16Type:
		*(*int16)(rvalue) = int16(val.Int())
	case types.UInt64Type:
		*(*uint64)(rvalue) = uint64(val.Uint())
	case types.SInt64Type:
		*(*int64)(rvalue) = int64(val.Int())
	case types.FloatType:
		*(*float32)(rvalue) = float32(val.Float())
	case types.DoubleType:
		*(*float64)(rvalue) = val.Float()
	case types.PointerType:
		*(*uintptr)(rvalue) = uintptr(val.Uint())
	case types.StructType:
		reflect.NewAt(val.Type(), rvalue).Elem().Set(val)
	default:
		*(*uintptr)(rvalue) = uintptr(val.Uint())
	}
}

// =============================================================================
// Callback Support
// =============================================================================

// NewCallback registers a Go function as a C callback and returns a function pointer.
// The returned uintptr can be passed to C code as a callback function pointer.
//
// Requirements:
//   - fn must be a function (not nil)
//   - fn can have multiple arguments of basic types (int, float, pointer, etc.)
//   - fn can return at most one value of basic type
//   - Complex types (string, slice, map, chan, interface) are not supported
//
// Usage Example:
//
//	func myCallback(x int, y float64) int {
//	    return x + int(y)
//	}
//
//	callbackPtr := ffi.NewCallback(myCallback)
//	// Pass callbackPtr to C code as function pointer
func NewCallback(fn any) uintptr {
	if fn == nil {
		panic("ffi: callback function must not be nil")
	}
	return purego.NewCallback(fn)
}

// =============================================================================
// Internal Helpers
// =============================================================================

// isValidType validates type descriptor
func isValidType(t *types.TypeDescriptor) bool {
	if t == nil {
		return false
	}
	switch t.Kind {
	case types.VoidType, types.IntType, types.FloatType, types.DoubleType,
		types.UInt8Type, types.SInt8Type, types.UInt16Type, types.SInt16Type,
		types.UInt32Type, types.SInt32Type, types.UInt64Type, types.SInt64Type,
		types.StructType, types.PointerType:
		return true
	default:
		return false
	}
}

func initializeCompositeType(t *types.TypeDescriptor) error {
	if t == nil {
		return newInvalidTypeError("struct", -1, "must not be nil")
	}
	if t.Kind != types.StructType {
		return nil
	}
	var size uintptr
	var maxAlign uintptr
	for i, member := range t.Members {
		if member == nil {
			return newInvalidTypeAtIndexError("structMembers", -1, i, "must not be nil")
		}
		if member.Kind == types.StructType && member.Size == 0 {
			if err := initializeCompositeType(member); err != nil {
				return err
			}
		}
		if !isValidType(member) {
			return newInvalidTypeAtIndexError("structMembers", int(member.Kind), i, "unsupported type kind")
		}
		align := member.Alignment
		if align == 0 {
			align = defaultTypeAlignment(member)
		}
		memberSize := member.Size
		if memberSize == 0 {
			memberSize = defaultTypeSize(member)
		}
		size = alignTo(size, align)
		size += memberSize
		if align > maxAlign {
			maxAlign = align
		}
	}
	if maxAlign == 0 {
		maxAlign = 1
	}
	t.Size = alignTo(size, maxAlign)
	t.Alignment = maxAlign
	return nil
}

func defaultTypeSize(t *types.TypeDescriptor) uintptr {
	if t == nil {
		return 0
	}
	if t.Size != 0 {
		return t.Size
	}
	switch t.Kind {
	case types.VoidType:
		return 1
	case types.UInt8Type, types.SInt8Type:
		return 1
	case types.UInt16Type, types.SInt16Type:
		return 2
	case types.IntType, types.UInt32Type, types.SInt32Type, types.FloatType:
		return 4
	case types.UInt64Type, types.SInt64Type, types.DoubleType:
		return 8
	case types.PointerType:
		return unsafe.Sizeof(uintptr(0))
	default:
		return 0
	}
}

func defaultTypeAlignment(t *types.TypeDescriptor) uintptr {
	if t == nil {
		return 1
	}
	if t.Alignment != 0 {
		return t.Alignment
	}
	return defaultTypeSize(t)
}

func alignTo(value, alignment uintptr) uintptr {
	if alignment == 0 {
		return value
	}
	rem := value % alignment
	if rem == 0 {
		return value
	}
	return value + alignment - rem
}
