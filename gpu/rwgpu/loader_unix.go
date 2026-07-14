//go:build linux || darwin

package rwgpu

import (
	"fmt"

	"github.com/ebitengine/purego"
)

// unixLibrary wraps a purego dlopen handle to implement the Library interface.
type unixLibrary struct {
	handle uintptr
	name   string
}

// unixProc wraps a purego function pointer.
type unixProc struct {
	lib   *unixLibrary
	name  string
	fnPtr uintptr
}

// loadLibrary loads a shared library using purego.
// Returns a Library interface and an error if the library cannot be found.
func loadLibrary(name string) (Library, error) {
	handle, err := purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, fmt.Errorf("dlopen %s: %w", name, err)
	}

	return &unixLibrary{
		handle: handle,
		name:   name,
	}, nil
}

// NewProc retrieves a procedure from the Unix shared library.
func (u *unixLibrary) NewProc(name string) Proc {
	if u.handle == 0 {
		// Return a proc that will fail on Call
		return &unixProc{
			lib:  u,
			name: name,
		}
	}

	fnPtr, err := purego.Dlsym(u.handle, name)
	if err != nil {
		// Return a proc that will fail on Call
		return &unixProc{
			lib:  u,
			name: name,
		}
	}

	return &unixProc{
		lib:   u,
		name:  name,
		fnPtr: fnPtr,
	}
}

// Call invokes the Unix procedure with the given arguments.
// This uses purego.SyscallN and is suitable only for integer/pointer ABI calls.
// Calls with float parameters or by-value structs use typed purego.RegisterFunc wrappers.
func (u *unixProc) Call(args ...uintptr) (uintptr, uintptr, error) {
	if u.fnPtr == 0 {
		return 0, 0, fmt.Errorf("wgpu: failed to get symbol %s from %s", u.name, u.lib.name)
	}

	result, result2, errno := purego.SyscallN(u.fnPtr, args...)
	if errno != 0 {
		return result, result2, fmt.Errorf("wgpu: call to %s failed: errno %d", u.name, errno)
	}
	return result, result2, nil
}
