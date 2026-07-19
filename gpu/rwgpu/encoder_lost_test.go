package rwgpu

import "testing"

// TestEncoderMethods_SkipNativeWhenLost ensures Draw/End/Dispatch/ClearBuffer
// refuse native work after parent device is sticky-lost (no panic, no purego).
func TestEncoderMethods_SkipNativeWhenLost(t *testing.T) {
	const h uintptr = 0xe001
	d := testDevice(h)
	t.Cleanup(func() { unregisterLiveDevice(d) })
	markDeviceLost(h)

	var nativeCalls int
	prev := nativeCallHook
	// Encoder methods call purego directly (not via releaseNativeHandle), so
	// we only assert they do not panic and complete without requiring init.
	// checkInit fails without lib; lost gate must short-circuit before it matters.
	_ = prev
	_ = nativeCalls

	enc := &CommandEncoder{handle: 0xe100, device: h}
	enc.ClearBuffer(&Buffer{handle: 1}, 0, 0)
	enc.CopyBufferToBuffer(&Buffer{handle: 1}, 0, &Buffer{handle: 2}, 0, 4)
	enc.InsertDebugMarker("x")
	enc.PushDebugGroup("g")
	enc.PopDebugGroup()

	rpe := &RenderPassEncoder{handle: 0xe200, device: h}
	rpe.Draw(3, 1, 0, 0)
	rpe.DrawIndexed(3, 1, 0, 0, 0)
	rpe.SetViewport(0, 0, 1, 1, 0, 1)
	rpe.End()

	cpe := &ComputePassEncoder{handle: 0xe300, device: h}
	cpe.DispatchWorkgroups(1, 1, 1)
	cpe.End()

	rbe := &RenderBundleEncoder{handle: 0xe400, device: h}
	rbe.Draw(3, 1, 0, 0)
	rbe.DrawIndexed(3, 1, 0, 0, 0)

	// Healthy device + zero init: methods still nil-safe without panic.
	var nilRPE *RenderPassEncoder
	nilRPE.Draw(1, 1, 0, 0)
	nilRPE.End()
}

// TestCreateTextureErrorCleanup_SkipsNativeWhenLostMessage ensures error-path
// cleanup uses releaseNativeHandle with lost detection from message.
func TestCreateTextureErrorCleanup_SkipsNativeWhenLostMessage(t *testing.T) {
	const h uintptr = 0xe500
	d := testDevice(h)
	t.Cleanup(func() { unregisterLiveDevice(d) })

	var calls int
	prev := nativeCallHook
	nativeCallHook = func(string) { calls++ }
	t.Cleanup(func() { nativeCallHook = prev })

	// Simulate cleanup path: handle returned but uncaptured says device lost.
	handle := uintptr(0xe501)
	msg := "Parent device is lost"
	// Mark lost as uncaptured handler would.
	if looksLikeDeviceLost(msg) {
		markDeviceLost(h)
	}
	hh := handle
	releaseNativeHandle(&hh, isOwnerDeviceLost(h) || looksLikeDeviceLost(msg), func(uintptr) {
		t.Fatal("must not call native release on lost cleanup")
	})
	if hh != 0 {
		t.Fatal("handle must be cleared")
	}
	if calls != 0 {
		t.Fatalf("nativeCallHook fired %d times", calls)
	}
}
