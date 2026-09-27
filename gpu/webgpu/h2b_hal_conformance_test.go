//go:build !(js && wasm)

package webgpu

import (
	"errors"
	"strings"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"

	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// H2-b 换口行为回归：已换 hal 签名的方法收 hal 接口、内部拆包、
// 传具体指针照常用；错类型/空值一律守卫返回，不碰 native（本文件零 native 库依赖）。

type h2bFakeCmd struct{}

func (h2bFakeCmd) Destroy() {}

type h2bFakeBuf struct{}

func (h2bFakeBuf) Destroy()                         {}
func (h2bFakeBuf) NativeHandle() uintptr            { return 0 }
func (h2bFakeBuf) Size() uint64                     { return 64 }
func (h2bFakeBuf) Usage() types.BufferUsage         { return 0 }
func (h2bFakeBuf) Label() string                    { return "h2b-fake" }
func (h2bFakeBuf) Format() types.TextureFormat      { return 0 }
func (h2bFakeBuf) CurrentUsage() types.TextureUsage { return 0 }
func (h2bFakeBuf) AddPendingRef()                   {}
func (h2bFakeBuf) DecPendingRef()                   {}

// h2bLiveQueue returns a queue that passes prepareQueueCall without native:
// fresh rwgpu handles, device not marked lost.
func h2bLiveQueue() *Queue {
	dev := &Device{r: &rwgpu.Device{}}
	return &Queue{r: &rwgpu.Queue{}, device: dev}
}

func TestH2B_QueueSubmitWrongTypeAtomic(t *testing.T) {
	q := h2bLiveQueue()
	if _, err := q.Submit(h2bFakeCmd{}); err == nil || !strings.Contains(err.Error(), "not a webgpu command buffer") {
		t.Fatalf("Submit wrong type: got %v, want type error", err)
	}
}

func TestH2B_QueueWriteBufferWrongType(t *testing.T) {
	q := h2bLiveQueue()
	if err := q.WriteBuffer(h2bFakeBuf{}, 0, []byte{1}); err == nil || !strings.Contains(err.Error(), "not a webgpu buffer") {
		t.Fatalf("WriteBuffer wrong type: got %v, want type error", err)
	}
	if err := q.WriteBuffer(nil, 0, []byte{1}); err == nil {
		t.Fatal("WriteBuffer nil must error, not panic")
	}
}

func TestH2B_QueuePresentGuards(t *testing.T) {
	q := h2bLiveQueue()
	if err := q.Present(nil, nil, nil); err == nil {
		t.Fatal("Present nil surface/texture must error, not panic")
	}
}

func TestH2B_DeviceFreePushPopNoop(t *testing.T) {
	d := &Device{}
	d.FreeCommandBuffer(nil)
	d.FreeCommandBuffer(h2bFakeCmd{})
	d.FreeCommandBuffer(&CommandBuffer{})
	d.PushErrorScope(hal.ErrorFilterValidation)
	if gerr := d.PopErrorScope(); gerr != nil {
		t.Fatalf("PopErrorScope without native: got %v, want nil", gerr)
	}
}

func TestH2B_EncoderCopyWrongTypeNoop(t *testing.T) {
	e := &CommandEncoder{}
	e.CopyBufferToBuffer(h2bFakeBuf{}, 0, h2bFakeBuf{}, 0, 8)
	e.CopyBufferToBuffer(nil, 0, nil, 0, 8)
	e.ClearBuffer(h2bFakeBuf{}, 0, 8)
	e.ClearBuffer(nil, 0, 8)
	e.TransitionTextures([]hal.TextureBarrier{{Texture: h2bFakeBuf{}}})
	er := &CommandEncoder{released: true}
	er.CopyBufferToBuffer(h2bFakeBuf{}, 0, h2bFakeBuf{}, 0, 8)
	er.ClearBuffer(h2bFakeBuf{}, 0, 8)
}

func TestH2B_PassGuardsNoop(t *testing.T) {
	rp := &RenderPassEncoder{}
	rp.SetBindGroup(0, nil, nil)
	rp.SetBindGroup(0, h2bFakeCmd{}, nil)
	rp.SetVertexBuffer(0, nil, 0)
	rp.SetVertexBuffer(0, h2bFakeBuf{}, 0)
	rp.SetIndexBuffer(nil, types.IndexFormatUint16, 0)
	rp.DrawIndirect(h2bFakeBuf{}, 0)
	rp.DrawIndexedIndirect(h2bFakeBuf{}, 0)
	cp := &ComputePassEncoder{}
	cp.SetBindGroup(0, nil, nil)
	cp.SetBindGroup(0, h2bFakeCmd{}, nil)
	cp.DispatchIndirect(h2bFakeBuf{}, 0)
	cp.DispatchIndirect(nil, 0)
}

func TestH2B_RenderPassDescriptorAliasCompiles(t *testing.T) {
	desc := &hal.RenderPassDescriptor{
		Label: "h2b",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:       &TextureView{},
			LoadOp:     types.LoadOpClear,
			StoreOp:    types.StoreOpStore,
			ClearValue: types.Color{R: 0, G: 0, B: 0, A: 1},
		}},
	}
	er := &CommandEncoder{released: true}
	if _, err := er.BeginRenderPass(desc); !errors.Is(err, ErrReleased) {
		t.Fatalf("BeginRenderPass released: got %v, want ErrReleased", err)
	}
	if _, err := er.BeginComputePass(&hal.ComputePassDescriptor{Label: "h2b"}); !errors.Is(err, ErrReleased) {
		t.Fatalf("BeginComputePass released: got %v, want ErrReleased", err)
	}
}
