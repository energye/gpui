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
	// missingErr is cached so a missing symbol does not re-allocate on every Call.
	missingErr error
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
		if u.missingErr == nil {
			libName := ""
			if u.lib != nil {
				libName = u.lib.name
			}
			u.missingErr = fmt.Errorf("wgpu: failed to get symbol %s from %s", u.name, libName)
		}
		return 0, 0, u.missingErr
	}
	// wgpu C entry points do not report failure via errno. purego.SyscallN still
	// surfaces residual thread-local errno from unrelated libc activity, which
	// previously forced a fmt.Errorf allocation on nearly every present-path FFI
	// call. Real device errors are delivered through uncaptured-error callbacks.
	//
	// Use stock purego.SyscallN (no local purego forks / extra APIs).
	result, result2, _ := purego.SyscallN(u.fnPtr, args...)
	return result, result2, nil
}

// Fixed-arity call helpers avoid the variadic []uintptr allocation of Proc.Call
// on the present hot path. Only valid for unixProc with a resolved symbol.

func asUnixProc(p Proc) *unixProc {
	u, _ := p.(*unixProc)
	return u
}

func call1(p Proc, a0 uintptr) (uintptr, uintptr) {
	if u := asUnixProc(p); u != nil && u.fnPtr != 0 {
		r1, r2, _ := purego.SyscallN(u.fnPtr, a0)
		return r1, r2
	}
	r1, r2, _ := p.Call(a0)
	return r1, r2
}

func call2(p Proc, a0, a1 uintptr) (uintptr, uintptr) {
	if u := asUnixProc(p); u != nil && u.fnPtr != 0 {
		r1, r2, _ := purego.SyscallN(u.fnPtr, a0, a1)
		return r1, r2
	}
	r1, r2, _ := p.Call(a0, a1)
	return r1, r2
}

func call3(p Proc, a0, a1, a2 uintptr) (uintptr, uintptr) {
	if u := asUnixProc(p); u != nil && u.fnPtr != 0 {
		r1, r2, _ := purego.SyscallN(u.fnPtr, a0, a1, a2)
		return r1, r2
	}
	r1, r2, _ := p.Call(a0, a1, a2)
	return r1, r2
}

func call5(p Proc, a0, a1, a2, a3, a4 uintptr) (uintptr, uintptr) {
	if u := asUnixProc(p); u != nil && u.fnPtr != 0 {
		r1, r2, _ := purego.SyscallN(u.fnPtr, a0, a1, a2, a3, a4)
		return r1, r2
	}
	r1, r2, _ := p.Call(a0, a1, a2, a3, a4)
	return r1, r2
}

func call6(p Proc, a0, a1, a2, a3, a4, a5 uintptr) (uintptr, uintptr) {
	if u := asUnixProc(p); u != nil && u.fnPtr != 0 {
		r1, r2, _ := purego.SyscallN(u.fnPtr, a0, a1, a2, a3, a4, a5)
		return r1, r2
	}
	r1, r2, _ := p.Call(a0, a1, a2, a3, a4, a5)
	return r1, r2
}
