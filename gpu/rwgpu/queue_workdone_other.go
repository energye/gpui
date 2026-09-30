//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !linux && !darwin

package rwgpu

import "unsafe"

// callQueueOnSubmittedWorkDone invokes wgpuQueueOnSubmittedWorkDone.
// Non-unix fallback: captures the returned pointer-width future id.
func callQueueOnSubmittedWorkDone(queue uintptr, callbackInfo *queueWorkDoneCallbackInfo) (Future, error) {
	future, _, err := procQueueOnSubmittedWorkDone.Call(
		queue,
		uintptr(unsafe.Pointer(callbackInfo)),
	)
	return Future{ID: uint64(future)}, err
}
