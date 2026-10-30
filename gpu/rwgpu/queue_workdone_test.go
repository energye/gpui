package rwgpu

import (
	"os"
	"testing"
	"time"
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
