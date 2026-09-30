//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux || darwin

package rwgpu

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// callQueueOnSubmittedWorkDone invokes wgpuQueueOnSubmittedWorkDone and
// captures the WGPUFuture returned by value (unix ABI via RegisterFunc).
func callQueueOnSubmittedWorkDone(queue uintptr, callbackInfo *queueWorkDoneCallbackInfo) (Future, error) {
	proc, ok := procQueueOnSubmittedWorkDone.(*unixProc)
	if !ok {
		future, _, err := procQueueOnSubmittedWorkDone.Call(
			queue,
			uintptr(unsafe.Pointer(callbackInfo)),
		)
		return Future{ID: uint64(future)}, err
	}
	if proc.fnPtr == 0 {
		return Future{}, &WGPUError{Op: "Queue.OnSubmittedWorkDone", Message: "wgpuQueueOnSubmittedWorkDone symbol is missing"}
	}

	var onSubmittedWorkDone func(uintptr, queueWorkDoneCallbackInfo) Future
	purego.RegisterFunc(&onSubmittedWorkDone, proc.fnPtr)
	return onSubmittedWorkDone(queue, *callbackInfo), nil
}
