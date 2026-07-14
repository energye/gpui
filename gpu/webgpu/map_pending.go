//go:build !(js && wasm)

package webgpu

import (
	"context"

	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// MapPending is a handle to an in-flight Buffer.MapAsync on the wgpu-native backend.
// On the wgpu-native backend, this wraps rwgpu MapPending which uses
// wgpu-native's callback mechanism.
type MapPending struct {
	r   *rwgpu.MapPending
	buf *Buffer
}

// Status returns the current state of the pending map without blocking.
func (p *MapPending) Status() (ready bool, err error) {
	if p == nil || p.r == nil {
		return true, ErrMapCanceled
	}
	ready, err = p.r.Status()
	if ready && err == nil && p.buf != nil {
		p.buf.mapped = true
	}
	return ready, err
}

// Wait blocks until the pending map resolves or ctx is canceled.
func (p *MapPending) Wait(ctx context.Context) error {
	if p == nil || p.r == nil {
		return ErrMapCanceled
	}
	err := p.r.Wait(ctx)
	if err == nil && p.buf != nil {
		p.buf.mapped = true
	}
	return err
}

// Release discards the MapPending handle.
func (p *MapPending) Release() {
	if p == nil {
		return
	}
	if p.r != nil {
		p.r.Release()
	}
	p.buf = nil
	p.r = nil
}
