//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rwgpu

import (
	"fmt"
	"sync"
	"time"

	"github.com/ebitengine/purego"
)

// QueueWorkDoneStatus mirrors WGPUQueueWorkDoneStatus (v29 layout:
// Success=1, CallbackCancelled=2, Error=3 — the old iota 0..3 values were
// pre-v29 and made every successful callback look like an error).
type QueueWorkDoneStatus int

const (
	// QueueWorkDoneSuccess indicates all submitted work completed.
	QueueWorkDoneSuccess QueueWorkDoneStatus = 0x00000001
	// QueueWorkDoneCallbackCancelled indicates the callback was discarded
	// before it could run (future dropped).
	QueueWorkDoneCallbackCancelled QueueWorkDoneStatus = 0x00000002
	// QueueWorkDoneError indicates a deterministic error during execution.
	QueueWorkDoneError QueueWorkDoneStatus = 0x00000003
)

// queueWorkDoneCallbackInfo matches WGPUQueueWorkDoneCallbackInfo (v29 layout:
// nextInChain, mode, callback, userdata1, userdata2 — the mode field is
// mandatory since the future-based API; omitting it shifts every later field
// and wgpu answers with a null future).
type queueWorkDoneCallbackInfo struct {
	NextInChain uintptr // *ChainedStruct
	Mode        CallbackMode
	Callback    uintptr // WGPUQueueWorkDoneCallback
	Userdata1   uintptr
	Userdata2   uintptr
}

// queueWorkDoneResult is the completion record for one OnSubmittedWorkDone.
// The record stays in queueWorkDoneResults after firing so FutureDone can
// report the status without blocking; whoever observes completion deletes it.
type queueWorkDoneResult struct {
	status    QueueWorkDoneStatus
	completed bool // set under queueWorkDoneResultsMu
	nativeID  uint64
}

var (
	// queueWorkDoneResults maps our record id (the Future.ID we hand out) →
	// completion record. Protected by queueWorkDoneResultsMu (callbacks may
	// fire from any thread).
	queueWorkDoneResults   = make(map[uintptr]*queueWorkDoneResult)
	queueWorkDoneResultID  uintptr
	queueWorkDoneResultsMu sync.Mutex

	queueWorkDoneCallbackPtr  uintptr
	queueWorkDoneCallbackOnce sync.Once
)

// queueWorkDoneCallbackHandler is the Go callback invoked by wgpu-native.
// Signature (v29): void(WGPUQueueWorkDoneStatus status, WGPUStringView message,
// void* userdata1, void* userdata2); on SysV the StringView arrives as two
// integer words, so userdata1 is the fourth parameter. Reading it one slot
// early (the pre-v29 shape) silently drops every completion.
func queueWorkDoneCallbackHandler(status, _ /* message data */, _ /* message length */, userdata1, _ uintptr) uintptr {
	queueWorkDoneResultsMu.Lock()
	if result, ok := queueWorkDoneResults[userdata1]; ok && !result.completed {
		result.status = QueueWorkDoneStatus(status)
		result.completed = true
	}
	queueWorkDoneResultsMu.Unlock()
	return 0 // void
}

func initQueueWorkDoneCallback() {
	queueWorkDoneCallbackPtr = purego.NewCallback(queueWorkDoneCallbackHandler)
}

func queueWorkDoneCallback() uintptr {
	queueWorkDoneCallbackOnce.Do(initQueueWorkDoneCallback)
	return queueWorkDoneCallbackPtr
}

// OnSubmittedWorkDone registers a completion callback for all GPU work
// submitted before this call (Skia command-buffer refs; P6 / P2.4d).
//
// wgpu-native v29 still fires the callback but returns a zero WGPUFuture from
// wgpuQueueOnSubmittedWorkDone (checked against the C ABI: a C shim sees id 0
// too). The returned Future therefore carries this module's own record id,
// not a native future id — resolve it with FutureDone / WaitForFuture, never
// hand it to wgpuInstanceWaitAny. Callbacks fire from Device.Poll /
// Instance.ProcessEvents (callback mode AllowProcessEvents).
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
	result := &queueWorkDoneResult{}
	queueWorkDoneResults[id] = result
	queueWorkDoneResultsMu.Unlock()

	info := queueWorkDoneCallbackInfo{
		Mode:      CallbackModeAllowProcessEvents,
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
	queueWorkDoneResultsMu.Lock()
	result.nativeID = future.ID
	queueWorkDoneResultsMu.Unlock()
	return Future{ID: uint64(id)}, nil
}

// FutureDone reports whether work-done record `future` has completed, without
// blocking. The record is released once completion has been observed, so a
// second observation reports completed (implicit success).
func FutureDone(future Future) (QueueWorkDoneStatus, bool) {
	id := uintptr(future.ID)
	if id == 0 {
		return QueueWorkDoneSuccess, true
	}
	queueWorkDoneResultsMu.Lock()
	defer queueWorkDoneResultsMu.Unlock()
	result, ok := queueWorkDoneResults[id]
	if !ok || result == nil {
		return QueueWorkDoneSuccess, true
	}
	if !result.completed {
		return QueueWorkDoneSuccess, false
	}
	delete(queueWorkDoneResults, id)
	return result.status, true
}

// FutureForget drops an unobserved work-done record without waiting. Device-
// loss path (P2.4e J2): the callback will never fire and nothing observes
// this future again, so without the explicit drop the record would stay in
// the map forever. Idempotent; unknown or zero ids are no-ops.
func FutureForget(future Future) {
	id := uintptr(future.ID)
	if id == 0 {
		return
	}
	queueWorkDoneResultsMu.Lock()
	delete(queueWorkDoneResults, id)
	queueWorkDoneResultsMu.Unlock()
}

// WaitForFuture blocks until the given work-done record completes. It pumps
// instance callbacks itself (wgpu-native needs ProcessEvents for callback-mode
// futures). Do NOT use on the per-frame hot path (P6; use BeginFrame sync
// points instead).
func WaitForFuture(instance *Instance, future Future) error {
	id := uintptr(future.ID)
	if id == 0 {
		return nil
	}
	for {
		queueWorkDoneResultsMu.Lock()
		result, ok := queueWorkDoneResults[id]
		if !ok || result == nil {
			queueWorkDoneResultsMu.Unlock()
			return nil
		}
		if result.completed {
			status := result.status
			delete(queueWorkDoneResults, id)
			queueWorkDoneResultsMu.Unlock()
			if status != QueueWorkDoneSuccess {
				return &WGPUError{Op: "Queue.OnSubmittedWorkDone", Message: QueueWorkDoneStatusName(status)}
			}
			return nil
		}
		queueWorkDoneResultsMu.Unlock()
		if instance != nil {
			instance.ProcessEvents()
		}
		time.Sleep(time.Millisecond)
	}
}

// QueueWorkDoneStatusName returns a human-readable status name (diagnostics).
func QueueWorkDoneStatusName(s QueueWorkDoneStatus) string {
	switch s {
	case QueueWorkDoneSuccess:
		return "success"
	case QueueWorkDoneCallbackCancelled:
		return "callback-cancelled"
	case QueueWorkDoneError:
		return "error"
	default:
		return fmt.Sprintf("QueueWorkDoneStatus(%d)", int(s))
	}
}
