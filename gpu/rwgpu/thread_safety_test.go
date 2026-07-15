package rwgpu

import (
	"os"
	"sync"
	"testing"
)

func TestConcurrentInit(t *testing.T) {
	// Verify Init() is safe for concurrent calls
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = Init()
		}()
	}
	wg.Wait()
}

// TestMultipleAdapterRequests validates repeated RequestAdapter on one Instance.
//
// Concurrent RequestAdapter on the same Instance is NOT safe on some native
// backends (observed: wgpu-hal GLES egl BadAccess abort under purego). S1 gate
// therefore exercises serial multi-request; optional concurrent stress is
// behind RWGPU_CONCURRENT_STRESS=1.
func TestMultipleAdapterRequests(t *testing.T) {
	inst, err := CreateInstance(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Release()

	const n = 3
	for i := 0; i < n; i++ {
		a, e := inst.RequestAdapter(nil)
		if e != nil {
			t.Fatalf("adapter %d failed: %v", i, e)
		}
		if a == nil {
			t.Fatalf("adapter %d is nil", i)
		}
		a.Release()
	}
}

func TestConcurrentAdapterRequests(t *testing.T) {
	if os.Getenv("RWGPU_CONCURRENT_STRESS") != "1" {
		t.Skip("concurrent RequestAdapter can abort on GLES backends; set RWGPU_CONCURRENT_STRESS=1 to run")
	}

	inst, err := CreateInstance(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Release()

	var wg sync.WaitGroup
	errs := make([]error, 3)
	adapters := make([]*Adapter, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			a, e := inst.RequestAdapter(nil)
			adapters[idx] = a
			errs[idx] = e
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("adapter %d failed: %v", i, err)
		}
		if adapters[i] != nil {
			adapters[i].Release()
		}
	}
}
