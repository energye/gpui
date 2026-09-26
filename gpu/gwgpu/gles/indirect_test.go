//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

func TestRenderPassEncoderCountedIndirectRemainsUnsupported(t *testing.T) {
	enc := &CommandEncoder{}
	if err := enc.BeginEncoding("indirect"); err != nil {
		t.Fatal(err)
	}
	pass, _ := enc.BeginRenderPass(&hal.RenderPassDescriptor{ColorAttachments: []hal.RenderPassColorAttachment{}})

	pass.DrawIndirect(&Buffer{id: 7, size: 64}, 0)
	pass.SetIndexBuffer(&Buffer{id: 9, size: 64}, gputypes.IndexFormatUint32, 16)
	pass.DrawIndexedIndirect(&Buffer{id: 8, size: 64}, 0)

	if len(enc.commands) != 1 {
		t.Fatalf("commands = %d, want only SetIndexBufferCommand", len(enc.commands))
	}
}
