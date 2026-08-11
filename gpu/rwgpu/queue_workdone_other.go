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