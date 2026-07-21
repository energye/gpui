package rwgpu

import (
	"testing"
	"unsafe"
)

func TestChainedStructSize(t *testing.T) {
	t.Log("ChainedStruct", unsafe.Sizeof(ChainedStruct{}))
	if unsafe.Sizeof(ChainedStruct{}) != 16 {
		t.Fatalf("want 16 got %d", unsafe.Sizeof(ChainedStruct{}))
	}
}

func TestInstanceExtrasWireSize(t *testing.T) {
	if unsafe.Sizeof(instanceExtrasWire{}) != 112 {
		t.Fatalf("instanceExtrasWire size=%d want 112", unsafe.Sizeof(instanceExtrasWire{}))
	}
	if unsafe.Offsetof(instanceExtrasWire{}.Backends) != 16 {
		t.Fatalf("backends offset=%d", unsafe.Offsetof(instanceExtrasWire{}.Backends))
	}
	if unsafe.Offsetof(instanceExtrasWire{}.BudgetForDeviceCreation) != 72 {
		t.Fatalf("budgetCreate offset=%d", unsafe.Offsetof(instanceExtrasWire{}.BudgetForDeviceCreation))
	}
}
