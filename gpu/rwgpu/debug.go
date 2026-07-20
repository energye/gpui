package rwgpu

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// debugMode controls whether resource tracking is enabled.
// When enabled, all resource allocations/releases are tracked.
// Zero overhead when disabled (just an atomic check).
var debugMode atomic.Bool

// resourceTracker tracks live GPU resources for leak detection.
var resourceTracker struct {
	mu        sync.Mutex
	resources map[uintptr]resourceInfo
}

type resourceInfo struct {
	Type  string
	Label string
}

func init() {
	resourceTracker.resources = make(map[uintptr]resourceInfo)
}

// SetDebugMode enables or disables resource tracking.
func SetDebugMode(enabled bool) {
	debugMode.Store(enabled)
}

// trackResource records a resource allocation (debug mode only).
func trackResource(handle uintptr, typeName string) {
	trackResourceLabel(handle, typeName, "")
}

// trackResourceLabel records allocation with an optional label.
func trackResourceLabel(handle uintptr, typeName, label string) {
	if !debugMode.Load() || handle == 0 {
		return
	}
	resourceTracker.mu.Lock()
	resourceTracker.resources[handle] = resourceInfo{Type: typeName, Label: label}
	resourceTracker.mu.Unlock()
}

// untrackResource records a resource release (debug mode only).
func untrackResource(handle uintptr) {
	if !debugMode.Load() || handle == 0 {
		return
	}
	resourceTracker.mu.Lock()
	delete(resourceTracker.resources, handle)
	resourceTracker.mu.Unlock()
}

// LeakReport contains information about unreleased GPU resources.
type LeakReport struct {
	Count int
	Types map[string]int
	Items []string // "Type:label" for each live resource
}

// String returns a human-readable summary of the leak report.
func (r *LeakReport) String() string {
	if r == nil || r.Count == 0 {
		return "no resource leaks detected"
	}
	s := fmt.Sprintf("%d unreleased GPU resource(s):", r.Count)
	for typ, count := range r.Types {
		s += fmt.Sprintf(" %s=%d", typ, count)
	}
	if len(r.Items) > 0 && len(r.Items) <= 32 {
		s += " [" + fmt.Sprintf("%v", r.Items) + "]"
	}
	return s
}

// ReportLeaks returns information about unreleased GPU resources.
func ReportLeaks() *LeakReport {
	if !debugMode.Load() {
		return nil
	}
	resourceTracker.mu.Lock()
	defer resourceTracker.mu.Unlock()

	count := len(resourceTracker.resources)
	if count == 0 {
		return nil
	}

	types := make(map[string]int)
	items := make([]string, 0, count)
	for _, info := range resourceTracker.resources {
		types[info.Type]++
		lab := info.Label
		if lab == "" {
			lab = "-"
		}
		items = append(items, info.Type+":"+lab)
	}

	return &LeakReport{
		Count: count,
		Types: types,
		Items: items,
	}
}

// ResetLeakTracker clears the resource tracker. Useful for test cleanup.
func ResetLeakTracker() {
	resourceTracker.mu.Lock()
	resourceTracker.resources = make(map[uintptr]resourceInfo)
	resourceTracker.mu.Unlock()
}
