package render

import (
	"errors"
	"testing"
)

type stubPurgeCache struct {
	entries int64
	calls   int
}

func (s *stubPurgeCache) PurgeEvictable() int64 {
	s.calls++
	freed := s.entries
	s.entries = 0
	return freed
}

func TestPurgeEvictables_RunsRegistered(t *testing.T) {
	a := &stubPurgeCache{entries: 3}
	b := &stubPurgeCache{entries: 5}
	RegisterPurgeEvictable("test-a", a)
	RegisterPurgeEvictable("test-b", b)
	total, byName := PurgeEvictables()
	if total < 8 {
		t.Fatalf("total=%d want >=8 (registered caches must run)", total)
	}
	if byName["test-a"] < 3 || byName["test-b"] < 5 {
		t.Fatalf("byName=%v want test-a>=3 test-b>=5", byName)
	}
	if a.calls < 1 || b.calls < 1 {
		t.Fatal("registered caches must be invoked")
	}
	// Registry keeps entries: second purge frees nothing from the stubs.
	total2, _ := PurgeEvictables()
	if total2 != total-8 {
		t.Fatalf("second total=%d want %d (stubs already empty)", total2, total-8)
	}
}

func TestPurgeEvictables_NilIgnored(t *testing.T) {
	RegisterPurgeEvictable("test-nil", nil)
	if _, byName := PurgeEvictables(); byName["test-nil"] != 0 {
		t.Fatalf("nil cache must not report, got %v", byName["test-nil"])
	}
}

func TestIsGPUOutOfMemory(t *testing.T) {
	for _, msg := range []string{
		"Not enough memory left",
		"OUT OF MEMORY",
		"out of device memory",
		"createTextureRetryOOM: out of memory",
	} {
		if !IsGPUOutOfMemory(errors.New(msg)) {
			t.Fatalf("want OOM for %q", msg)
		}
	}
	if IsGPUOutOfMemory(nil) {
		t.Fatal("nil must not be OOM")
	}
	if IsGPUOutOfMemory(errors.New("surface outdated")) {
		t.Fatal("non-OOM error must not match")
	}
}
