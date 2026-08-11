package rwgpu

import (
	"os"
	"testing"
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

// TestQueueWorkDoneStatusName guards the diagnostics enum mapping.
func TestQueueWorkDoneStatusName(t *testing.T) {
	cases := map[QueueWorkDoneStatus]string{
		QueueWorkDoneSuccess:    "success",
		QueueWorkDoneError:      "error",
		QueueWorkDoneUnknown:    "unknown",
		QueueWorkDoneDeviceLost: "device-lost",
	}
	for s, want := range cases {
		if got := QueueWorkDoneStatusName(s); got != want {
			t.Errorf("status %d: got %q want %q", int(s), got, want)
		}
	}
}