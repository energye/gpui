//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// B2 录制侧去重门：完全重复只发一次，换值照发（webgpu renderpass.go 对等规则）。

func newDedupPass() *RenderPassEncoder {
	return &RenderPassEncoder{encoder: &CommandEncoder{}}
}

func TestRenderPassDedup_PipelineOnce(t *testing.T) {
	rpe := newDedupPass()
	p := &RenderPipeline{programID: 7}
	rpe.SetPipeline(p)
	rpe.SetPipeline(p)
	uses := 0
	for _, c := range rpe.encoder.commands {
		if _, ok := c.(*UseProgramCommand); ok {
			uses++
		}
	}
	if uses != 1 {
		t.Fatalf("duplicate SetPipeline emitted %d UseProgram, want 1", uses)
	}
}

func TestRenderPassDedup_BindGroupValueChangeEmits(t *testing.T) {
	rpe := newDedupPass()
	g1 := &BindGroup{}
	g2 := &BindGroup{}
	rpe.SetBindGroup(0, g1, nil)
	rpe.SetBindGroup(0, g1, nil)
	rpe.SetBindGroup(0, g2, nil)
	n := 0
	for _, c := range rpe.encoder.commands {
		if _, ok := c.(*SetBindGroupCommand); ok {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("want 2 SetBindGroup (dup skipped, change emitted), got %d", n)
	}
}

func TestRenderPassDedup_BindGroupPipelineChangeEmits(t *testing.T) {
	rpe := newDedupPass()
	g := &BindGroup{}
	p1 := &RenderPipeline{programID: 1}
	p2 := &RenderPipeline{programID: 2}
	rpe.SetPipeline(p1)
	rpe.SetBindGroup(0, g, nil)
	rpe.SetPipeline(p2)
	rpe.SetBindGroup(0, g, nil)
	n := 0
	for _, c := range rpe.encoder.commands {
		if _, ok := c.(*SetBindGroupCommand); ok {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("pipeline change must re-emit bind group, got %d want 2", n)
	}
}

func TestRenderPassDedup_ViewportScissorBlendStencil(t *testing.T) {
	rpe := newDedupPass()
	rpe.SetViewport(0, 0, 100, 100, 0, 1)
	rpe.SetViewport(0, 0, 100, 100, 0, 1)
	rpe.SetViewport(0, 0, 200, 100, 0, 1)
	rpe.SetScissorRect(0, 0, 10, 10)
	rpe.SetScissorRect(0, 0, 10, 10)
	rpe.SetBlendConstant(&gputypes.Color{R: 1, G: 1, B: 1, A: 1})
	rpe.SetBlendConstant(&gputypes.Color{R: 1, G: 1, B: 1, A: 1})
	rpe.SetStencilReference(3)
	rpe.SetStencilReference(3)
	rpe.SetStencilReference(4)
	count := func(name string) int {
		n := 0
		for _, c := range rpe.encoder.commands {
			switch c.(type) {
			case *SetViewportCommand:
				if name == "vp" {
					n++
				}
			case *SetScissorCommand:
				if name == "sc" {
					n++
				}
			case *SetBlendConstantCommand:
				if name == "bl" {
					n++
				}
			case *SetStencilRefCommand:
				if name == "st" {
					n++
				}
			}
		}
		return n
	}
	if got := count("vp"); got != 2 {
		t.Fatalf("viewport cmds=%d want 2", got)
	}
	if got := count("sc"); got != 1 {
		t.Fatalf("scissor cmds=%d want 1", got)
	}
	if got := count("bl"); got != 1 {
		t.Fatalf("blend cmds=%d want 1", got)
	}
	if got := count("st"); got != 2 {
		t.Fatalf("stencil cmds=%d want 2", got)
	}
	var _ hal.RenderPassEncoder = rpe
}
