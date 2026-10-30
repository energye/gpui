//go:build !(js && wasm)

package webgpu

import (
	"os"
	"testing"
	"time"
)

// TestQueueCompletionToken pins hal.CompletionToken on the wgpu-native backend:
// Done is non-blocking and completes once the device callbacks are pumped.
func TestQueueCompletionToken(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	inst, adapter, device, queue := s2Device(t)
	defer s2Release(inst, adapter, device)

	if _, err := queue.Submit(); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	tok, err := queue.OnSubmittedWorkDone()
	if err != nil {
		t.Fatalf("OnSubmittedWorkDone: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !tok.Done() {
		if time.Now().After(deadline) {
			t.Fatal("token never completed within 5s")
		}
		time.Sleep(time.Millisecond)
	}
	if err := tok.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
}
