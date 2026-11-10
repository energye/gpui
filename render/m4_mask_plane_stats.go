package render

// MaskPlaneStats reports the GPU mask plane/readback allocation ledger
// (calls, plane allocations, plane bytes, readback bytes). Zero on the CPU leg.
// P4.3 measurement: the mask plane is rebuilt per masked draw today.
func (c *Context) MaskPlaneStats() (calls, planes, planeBytes, readbackBytes uint64) {
	if c == nil {
		return 0, 0, 0, 0
	}
	type maskStatReader interface {
		MaskPlaneStats() (uint64, uint64, uint64, uint64)
	}
	if rc, ok := c.gpuCtxOps().(maskStatReader); ok {
		return rc.MaskPlaneStats()
	}
	return 0, 0, 0, 0
}
