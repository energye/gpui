package rwgpu

import "unsafe"

// querySetDescriptor is the native structure for QuerySet descriptor (32 bytes).
type querySetDescriptor struct {
	nextInChain uintptr    // 8 bytes
	label       StringView // 16 bytes
	queryType   QueryType  // 4 bytes
	count       uint32     // 4 bytes
}

// QuerySetDescriptor describes a QuerySet to create.
type QuerySetDescriptor struct {
	Label string
	Type  QueryType
	Count uint32
}

// CreateQuerySet creates a new QuerySet for GPU profiling/timestamps.
func (d *Device) CreateQuerySet(desc *QuerySetDescriptor) (*QuerySet, error) {
	if err := prepareDeviceCall("CreateQuerySet", d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, &WGPUError{Op: "CreateQuerySet", Message: "descriptor is nil"}
	}

	nativeDesc := querySetDescriptor{
		nextInChain: 0,
		label:       stringToStringView(desc.Label),
		queryType:   desc.Type,
		count:       desc.Count,
	}

	gpuMu.Lock()
	defer gpuMu.Unlock()
	handle, _, _ := procDeviceCreateQuerySet.Call(
		d.handle,
		uintptr(unsafe.Pointer(&nativeDesc)),
	)
	if handle == 0 {
		return nil, &WGPUError{Op: "CreateQuerySet", Message: "wgpu returned null handle"}
	}
	trackResource(handle, "QuerySet")
	return &QuerySet{handle: handle, device: d.handle}, nil
}

// Destroy destroys the QuerySet, making it invalid.
// After Destroy the handle is nulled so subsequent ops cannot call native with
// a wild pointer. Idempotent; a following Release is a no-op.
// When the parent device is lost, only Go-side state is cleared.
func (qs *QuerySet) Destroy() {
	if qs == nil {
		return
	}
	lost := isOwnerDeviceLost(qs.device)
	destroyAndReleaseNativeHandle(&qs.handle, lost,
		func(h uintptr) { procQuerySetDestroy.Call(h) }, //nolint:errcheck
		func(h uintptr) { procQuerySetRelease.Call(h) }, //nolint:errcheck
	)
}

// Release releases the QuerySet reference.
// Nil-safe and idempotent. Skips native release when the parent device is lost.
func (qs *QuerySet) Release() {
	if qs == nil {
		return
	}
	releaseNativeHandle(&qs.handle, isOwnerDeviceLost(qs.device), func(h uintptr) {
		procQuerySetRelease.Call(h) //nolint:errcheck
	})
}

// Handle returns the underlying handle. For advanced use only.
func (qs *QuerySet) Handle() uintptr { return qs.handle }
