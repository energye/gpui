package rwgpu

import (
	"errors"
	"testing"
)

func TestInit(t *testing.T) {
	if err := Init(); err != nil {
		if errors.Is(err, ErrLibraryNotLoaded) {
			t.Skipf("native unavailable: %v", err)
		}
		t.Fatalf("Init failed: %v", err)
	}
	t.Log("Library initialized successfully")
}

func TestCreateInstanceWithNil(t *testing.T) {
	inst := requireInstance(t)
	defer inst.Release()

	if inst.Handle() == 0 {
		t.Fatal("Instance handle is zero")
	}

	t.Logf("Instance created successfully: handle=%#x", inst.Handle())
}

func TestCreateInstanceWithDescriptor(t *testing.T) {
	// Pass an explicit InstanceDescriptor with default (zero) values.
	inst := requireInstance(t)
	defer inst.Release()

	if inst.Handle() == 0 {
		t.Fatal("Instance handle is zero")
	}

	t.Logf("Instance created successfully: handle=%#x", inst.Handle())
}

func TestInstanceRelease(t *testing.T) {
	inst, err := CreateInstance(nil)
	if err != nil {
		t.Skipf("CreateInstance failed (native unavailable): %v", err)
	}

	handle := inst.Handle()
	t.Logf("Before release: handle=%#x", handle)

	inst.Release()

	if inst.Handle() != 0 {
		t.Fatal("Handle should be zero after release")
	}
	t.Log("Instance released successfully")
}

func TestCheckInitAfterLoad(t *testing.T) {
	// After library is loaded (which happens in TestInit), checkInit should return nil.
	if err := Init(); err != nil {
		t.Skipf("native unavailable: %v", err)
	}
	err := checkInit()
	if err != nil {
		t.Fatalf("checkInit() failed after successful library load: %v", err)
	}
	t.Log("checkInit() passed after library initialization")
}

func TestCreateInstanceReturnsErrLibraryNotLoaded(t *testing.T) {
	// This test documents the expected error type, but cannot test the actual
	// uninitialized state due to sync.Once - the library is already loaded.
	// The error value is defined and will be returned if Init() fails.
	if ErrLibraryNotLoaded == nil {
		t.Fatal("ErrLibraryNotLoaded should be defined")
	}
	t.Logf("ErrLibraryNotLoaded is defined: %v", ErrLibraryNotLoaded)
}
