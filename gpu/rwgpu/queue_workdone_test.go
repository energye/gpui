package rwgpu

import (
	"os"
	"testing"
	"time"
	"unsafe"
)

// TestQueue_OnSubmittedWorkDone verifies the P6 fence primitive end to end:
// after submitting a command buffer, OnSubmittedWorkDone + WaitForFuture
// resolves once the GPU completes the work.
func TestQueue_OnSubmittedWorkDone(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	inst, err := CreateInstance(nil)
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	t.Cleanup(func() { inst.Release() })

	adapter, err := inst.RequestAdapter(nil)
	if err != nil {
		t.Fatalf("RequestAdapter: %v", err)
	}
	t.Cleanup(func() { adapter.Release() })

	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Fatalf("RequestDevice: %v", err)
	}
	t.Cleanup(func() { device.Release() })

	queue := device.Queue()
	if queue == nil {
		t.Skip("queue unavailable")
	}

	// Empty submit still returns a valid submission; the fence must resolve.
	if _, err := queue.Submit(); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	future, err := queue.OnSubmittedWorkDone()
	if err != nil {
		t.Fatalf("OnSubmittedWorkDone: %v", err)
	}
	if future.ID == 0 {
		t.Fatal("expected non-null future")
	}
	if err := WaitForFuture(inst, future); err != nil {
		t.Fatalf("WaitForFuture: %v", err)
	}
}

// TestQueueWorkDoneCallbackInfoV29Layout pins the memory layout of
// queueWorkDoneCallbackInfo to the v29 webgpu.h header (D1, P2.4e). The
// pre-v29 four-word layout (no Mode field) shifted every later field, so
// wgpu read the record id as the callback pointer: null future at call time,
// and a jump to address 1 (SIGSEGV PC=0x1) when Device.Release settled the
// pending callback. Reproduction: docs/render_gpu_converge/evidence/P2.4e/
// d1-repro-oldlayout.txt.
func TestQueueWorkDoneCallbackInfoV29Layout(t *testing.T) {
	word := unsafe.Sizeof(uintptr(0))
	if word != 8 {
		t.Skipf("layout pinned for 64-bit uintptr, got %d bytes", word)
	}
	var info queueWorkDoneCallbackInfo
	if unsafe.Sizeof(info) != 40 {
		t.Fatalf("queueWorkDoneCallbackInfo size: got %d want 40 (v29: 5 words)", unsafe.Sizeof(info))
	}
	checks := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"NextInChain", unsafe.Offsetof(info.NextInChain), 0},
		{"Mode", unsafe.Offsetof(info.Mode), 8},
		{"Callback", unsafe.Offsetof(info.Callback), 16},
		{"Userdata1", unsafe.Offsetof(info.Userdata1), 24},
		{"Userdata2", unsafe.Offsetof(info.Userdata2), 32},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("offset %s: got %d want %d (v29 header order: nextInChain/mode/callback/userdata1/userdata2)", c.name, c.got, c.want)
		}
	}
	// The mode value handed to wgpu must stay a header-defined enum.
	if CallbackModeAllowProcessEvents != 0x00000002 {
		t.Errorf("CallbackModeAllowProcessEvents: got %#x want 0x2 (webgpu.h WGPUCallbackMode_AllowProcessEvents)", CallbackModeAllowProcessEvents)
	}
	// WGPUFuture is a single uint64; Future must stay ABI-compatible.
	var f Future
	if unsafe.Sizeof(f) != 8 || unsafe.Offsetof(f.ID) != 0 {
		t.Errorf("Future: size %d offset %d, want 8/0 (webgpu.h WGPUFuture{uint64 id})", unsafe.Sizeof(f), unsafe.Offsetof(f.ID))
	}
}

// TestQueueWorkDoneStatusName guards the diagnostics enum mapping. Values are
// pinned to the v29 header (Success=1 / CallbackCancelled=2 / Error=3).
func TestQueueWorkDoneStatusName(t *testing.T) {
	cases := map[QueueWorkDoneStatus]string{
		QueueWorkDoneSuccess:           "success",
		QueueWorkDoneCallbackCancelled: "callback-cancelled",
		QueueWorkDoneError:             "error",
	}
	for s, want := range cases {
		if got := QueueWorkDoneStatusName(s); got != want {
			t.Errorf("status %d: got %q want %q", int(s), got, want)
		}
	}
	if QueueWorkDoneSuccess != 1 || QueueWorkDoneCallbackCancelled != 2 || QueueWorkDoneError != 3 {
		t.Fatalf("status values drifted from WGPUQueueWorkDoneStatus v29")
	}
}

// TestQueue_FutureDoneNonBlocking pins the token contract used by
// hal.CompletionToken: Done never blocks, and pumping the device callbacks
// (Device.Poll / FlushCallbacks) eventually reports completion.
func TestQueue_FutureDoneNonBlocking(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	inst, err := CreateInstance(nil)
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	t.Cleanup(inst.Release)
	adapter, err := inst.RequestAdapter(nil)
	if err != nil {
		t.Fatalf("RequestAdapter: %v", err)
	}
	t.Cleanup(adapter.Release)
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Fatalf("RequestDevice: %v", err)
	}
	t.Cleanup(device.Release)
	queue := device.Queue()

	if _, err := queue.Submit(); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	future, err := queue.OnSubmittedWorkDone()
	if err != nil {
		t.Fatalf("OnSubmittedWorkDone: %v", err)
	}
	// Poll until complete, without ever blocking inside FutureDone.
	deadline := time.Now().Add(5 * time.Second)
	for {
		device.Poll(false)
		inst.ProcessEvents()
		status, done := FutureDone(future)
		if done {
			if status != QueueWorkDoneSuccess {
				t.Fatalf("status: got %q want success", QueueWorkDoneStatusName(status))
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("work-done callback never fired within 5s")
		}
		time.Sleep(time.Millisecond)
	}
	// The record is released on first observation; a second read reports done.
	if _, done := FutureDone(future); !done {
		t.Fatal("consumed record must report completed")
	}
}

// TestFutureForgetDropsRecord pins J2 (P2.4e): an unobserved work-done record
// can be dropped explicitly (device-loss path), and the drop is idempotent.
func TestFutureForgetDropsRecord(t *testing.T) {
	queueWorkDoneResultsMu.Lock()
	queueWorkDoneResultID++
	id := queueWorkDoneResultID
	queueWorkDoneResults[id] = &queueWorkDoneResult{nativeID: 999}
	queueWorkDoneResultsMu.Unlock()

	f := Future{ID: uint64(id)}
	if _, done := FutureDone(f); done {
		t.Fatal("fresh record must not report completed")
	}
	FutureForget(f)
	queueWorkDoneResultsMu.Lock()
	_, ok := queueWorkDoneResults[uintptr(id)]
	queueWorkDoneResultsMu.Unlock()
	if ok {
		t.Fatal("FutureForget must delete the record")
	}
	// Repeat and unknown ids are no-ops (no panic).
	FutureForget(f)
	FutureForget(Future{ID: 0})
}
