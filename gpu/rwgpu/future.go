package rwgpu

import (
	"fmt"
	"runtime"
	"time"
	"unsafe"
)

type waitStatus uint32

const (
	waitStatusSuccess            waitStatus = 0x00000001
	waitStatusTimedOut           waitStatus = 0x00000002
	waitStatusUnsupportedTimeout waitStatus = 0x00000003
	waitStatusUnsupportedCount   waitStatus = 0x00000004
	waitStatusUnsupportedMixed   waitStatus = 0x00000005
	waitStatusError              waitStatus = 0x00000006
	waitStatusInstanceDropped    waitStatus = 0x00000007
)

type futureWaitInfo struct {
	Future    Future
	Completed Bool
}

func waitForFuture(instance uintptr, future Future, op string) error {
	if instance == 0 || future.ID == 0 {
		return nil
	}
	info := futureWaitInfo{Future: future}
	for {
		status, _, err := procInstanceWaitAny.Call(
			instance,
			1,
			uintptr(unsafe.Pointer(&info)),
			0,
		)
		if err != nil {
			return &WGPUError{Op: op, Message: err.Error()}
		}
		switch waitStatus(status) {
		case waitStatusSuccess:
			return nil
		case waitStatusTimedOut:
			runtime.Gosched()
			time.Sleep(time.Millisecond)
		case waitStatusUnsupportedTimeout:
			return &WGPUError{Op: op, Message: "wgpuInstanceWaitAny does not support timed waits"}
		case waitStatusUnsupportedCount:
			return &WGPUError{Op: op, Message: "wgpuInstanceWaitAny does not support this future count"}
		case waitStatusUnsupportedMixed:
			return &WGPUError{Op: op, Message: "wgpuInstanceWaitAny does not support mixed callback modes"}
		case waitStatusError:
			return &WGPUError{Op: op, Message: "wgpuInstanceWaitAny failed"}
		case waitStatusInstanceDropped:
			return &WGPUError{Op: op, Message: "instance dropped while waiting for future"}
		default:
			return &WGPUError{Op: op, Message: fmt.Sprintf("unexpected wait status 0x%x", status)}
		}
	}
}
