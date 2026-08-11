package rwgpu

import (
	"fmt"
	"sync"

	"github.com/ebitengine/purego"
)

// QueueWorkDoneStatus mirrors WGPUQueueWorkDoneStatus.
type QueueWorkDoneStatus int

const (
	// QueueWorkDoneSuccess indicates all submitted work completed.
	QueueWorkDoneSuccess QueueWorkDoneStatus = iota
	// QueueWorkDoneError indicates an error occurred during execution.
	QueueWorkDoneError
	// QueueWorkDoneUnknown indicates an unknown failure.
	QueueWorkDoneUnknown
	// QueueWorkDoneDeviceLost indicates the device was lost while executing.
	QueueWorkDoneDeviceLost
)

// queueWorkDoneCallbackInfo matches WGPUQueueWorkDoneCallbackInfo.
type queueWorkDoneCallbackInfo struct {
	NextInChain uintptr // *ChainedStruct
	Callback    uintptr // WGPUQueueWorkDoneCallback
	Userdata1   uintptr
	Userdata2   uintptr
}

// queueWorkDoneResult is the completion record for one OnSubmittedWorkDone.
type queueWorkDoneResult struct {
	done   chan struct{}
	status QueueWorkDoneStatus
}

var (
	// queueWorkDoneResults maps userdata id → pending completion record.
	// Protected by queueWorkDoneResultsMu (callbacks may fire from any thread).
	queueWorkDoneResults   = make(map[uintptr]*queueWorkDoneResult)
	queueWorkDoneResultsMu sync.Mutex
	queueWorkDoneResultID  uintptr

	queueWorkDoneCallbackPtr  uintptr
	queueWorkDoneCallbackOnce sync.Once
)

// queueWorkDoneCallbackHandler is the Go callback invoked by wgpu-native.
// Signature matches: void (*)(WGPUQueueWorkDoneStatus status, void* userdata1,
// void* userdata2).
func queueWorkDoneCallbackHandler(status, userdata1, _ uintptr) uintptr {
	queueWorkDoneResultsMu.Lock()
	result, ok := queueWorkDoneResults[userdata1]
	if ok {
		delete(queueWorkDoneResults, userdata1)
	}
	queueWorkDoneResultsMu.Unlock()
	if ok && result != nil {
		result.status = QueueWorkDoneStatus(status)
		close(result.done)
	}
	return 0 // void
}

func initQueueWorkDoneCallback() {
	queueWorkDoneCallbackPtr = purego.NewCallback(queueWorkDoneCallbackHandler)
}

func queueWorkDoneCallback() uintptr {
	queueWorkDoneCallbackOnce.Do(initQueueWorkDoneCallback)
	return queueWorkDoneCallbackPtr
}

// OnSubmittedWorkDone registers a callback that fires once all GPU work
// submitted before this call has completed (Skia command-buffer refs / fence
// semantics, P6).
//
// The returned Future must be polled (Device.Poll / WaitForFuture) for the
// callback to fire — this wgpu-native build uses callback-mode futures.
// The frame path should NOT block on the future: completion is also
// reachable at the existing sync points (BeginFrame vsync/drainQueue).
func (q *Queue) OnSubmittedWorkDone() (Future, error) {
	if err := gateQueue("Queue.OnSubmittedWorkDone", q); err != nil {
		return Future{}, err
	}
	if err := checkInit(); err != nil {
		return Future{}, err
	}
	queueWorkDoneResultsMu.Lock()
	queueWorkDoneResultID++
	id := queueWorkDoneResultID
	result := &queueWorkDoneResult{done: make(chan struct{})}
	queueWorkDoneResults[id] = result
	queueWorkDoneResultsMu.Unlock()

	info := queueWorkDoneCallbackInfo{
		Callback:  queueWorkDoneCallback(),
		Userdata1: id,
	}
	future, err := callQueueOnSubmittedWorkDone(q.handle, &info)
	if err != nil {
		queueWorkDoneResultsMu.Lock()
		delete(queueWorkDoneResults, id)
		queueWorkDoneResultsMu.Unlock()
		return Future{}, err
	}
	if future.ID == 0 {
		queueWorkDoneResultsMu.Lock()
		delete(queueWorkDoneResults, id)
		queueWorkDoneResultsMu.Unlock()
		return Future{}, &WGPUError{Op: "Queue.OnSubmittedWorkDone", Message: "wgpu returned null future"}
	}
	return future, nil
}

// WaitForFuture blocks until the given callback-mode future completes.
// This is the same polling path used by PopErrorScopeAsync — do NOT use on
// the per-frame hot path (P6; use BeginFrame sync points instead).
func WaitForFuture(instance *Instance, future Future) error {
	if instance == nil || future.ID == 0 {
		return nil
	}
	return waitForFuture(instance.handle, future, "Queue.OnSubmittedWorkDone")
}

// QueueWorkDoneStatusName returns a human-readable status name (diagnostics).
func QueueWorkDoneStatusName(s QueueWorkDoneStatus) string {
	switch s {
	case QueueWorkDoneSuccess:
		return "success"
	case QueueWorkDoneError:
		return "error"
	case QueueWorkDoneUnknown:
		return "unknown"
	case QueueWorkDoneDeviceLost:
		return "device-lost"
	default:
		return fmt.Sprintf("QueueWorkDoneStatus(%d)", int(s))
	}
}