//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build windows

package rwgpu

import (
	"syscall"
)

// windowsLibrary wraps syscall.LazyDLL to implement the Library interface.
type windowsLibrary struct {
	dll *syscall.LazyDLL
}

// windowsProc wraps syscall.LazyProc to implement the Proc interface.
type windowsProc struct {
	proc *syscall.LazyProc
}

// loadLibrary loads a DLL using Windows syscall.NewLazyDLL.
// Returns a Library interface that can be used to get procedures.
// The DLL is eagerly loaded to report errors immediately.
func loadLibrary(name string) (Library, error) {
	dll := syscall.NewLazyDLL(name)
	// Force eager load to detect missing DLL immediately.
	if err := dll.Load(); err != nil {
		return nil, err
	}
	return &windowsLibrary{dll: dll}, nil
}

// NewProc retrieves a procedure from the Windows DLL.
func (w *windowsLibrary) NewProc(name string) Proc {
	return &windowsProc{
		proc: w.dll.NewProc(name),
	}
}

// Call invokes the Windows procedure with the given arguments.
// This directly delegates to syscall.LazyProc.Call().
func (w *windowsProc) Call(args ...uintptr) (uintptr, uintptr, error) {
	return w.proc.Call(args...)
}

// Fixed-arity call helpers mirror the unix loader's contract: a resolved
// symbol goes straight through syscall.SyscallN, an unresolved one falls back
// to Proc.Call so the lookup error is produced the same way on both platforms.

func asWindowsProc(p Proc) *windowsProc {
	w, _ := p.(*windowsProc)
	return w
}

// resolvedFnPtr mirrors the unix loader: known=false means p is not a
// loader-backed Proc; known=true with fn==0 means the symbol did not resolve.
func resolvedFnPtr(p Proc) (fn uintptr, known bool) {
	w, ok := p.(*windowsProc)
	if !ok {
		return 0, false
	}
	if w.proc.Find() != nil {
		return 0, true
	}
	return w.proc.Addr(), true
}

func call1(p Proc, a0 uintptr) (uintptr, uintptr) {
	if w := asWindowsProc(p); w != nil && w.proc.Find() == nil {
		r1, r2, _ := syscall.SyscallN(w.proc.Addr(), a0)
		return r1, r2
	}
	r1, r2, _ := p.Call(a0)
	return r1, r2
}

func call2(p Proc, a0, a1 uintptr) (uintptr, uintptr) {
	if w := asWindowsProc(p); w != nil && w.proc.Find() == nil {
		r1, r2, _ := syscall.SyscallN(w.proc.Addr(), a0, a1)
		return r1, r2
	}
	r1, r2, _ := p.Call(a0, a1)
	return r1, r2
}

func call3(p Proc, a0, a1, a2 uintptr) (uintptr, uintptr) {
	if w := asWindowsProc(p); w != nil && w.proc.Find() == nil {
		r1, r2, _ := syscall.SyscallN(w.proc.Addr(), a0, a1, a2)
		return r1, r2
	}
	r1, r2, _ := p.Call(a0, a1, a2)
	return r1, r2
}

func call5(p Proc, a0, a1, a2, a3, a4 uintptr) (uintptr, uintptr) {
	if w := asWindowsProc(p); w != nil && w.proc.Find() == nil {
		r1, r2, _ := syscall.SyscallN(w.proc.Addr(), a0, a1, a2, a3, a4)
		return r1, r2
	}
	r1, r2, _ := p.Call(a0, a1, a2, a3, a4)
	return r1, r2
}

func call6(p Proc, a0, a1, a2, a3, a4, a5 uintptr) (uintptr, uintptr) {
	if w := asWindowsProc(p); w != nil && w.proc.Find() == nil {
		r1, r2, _ := syscall.SyscallN(w.proc.Addr(), a0, a1, a2, a3, a4, a5)
		return r1, r2
	}
	r1, r2, _ := p.Call(a0, a1, a2, a3, a4, a5)
	return r1, r2
}
