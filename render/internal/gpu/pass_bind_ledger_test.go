package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/webgpu"
)

// 账本只记指针一致，不碰 GPU：同 pass 同一套绑定才跳过 Set*，
// 换管线/换 pass/跨层失效后必须走全绑定。像素无影响，只省调用。
func TestPassBindLedger_SkipIdenticalInvalidatesOnChange(t *testing.T) {
	var l PassBindLedger
	rp := &webgpu.RenderPassEncoder{}
	rp2 := &webgpu.RenderPassEncoder{}
	pipe := &webgpu.RenderPipeline{}
	pipe2 := &webgpu.RenderPipeline{}
	bg0 := &webgpu.BindGroup{}
	vert := &webgpu.Buffer{}

	if l.skipBind(rp, pipe, bg0, nil, nil, vert) {
		t.Fatal("fresh ledger must take full bind path")
	}
	l.noteBind(rp, pipe, bg0, nil, nil, vert)
	if !l.skipBind(rp, pipe, bg0, nil, nil, vert) {
		t.Fatal("identical set on same pass should skip binds")
	}
	if l.skipBind(rp, pipe2, bg0, nil, nil, vert) {
		t.Fatal("pipeline switch must take full bind path")
	}
	l.noteBind(rp, pipe2, bg0, nil, nil, vert)
	if !l.skipBind(rp, pipe2, bg0, nil, nil, vert) {
		t.Fatal("identical set after rebind should skip binds")
	}
	l.Invalidate()
	if l.skipBind(rp, pipe2, bg0, nil, nil, vert) {
		t.Fatal("invalidated ledger must take full bind path")
	}
	l.noteBind(rp, pipe2, bg0, nil, nil, vert)
	l.BeginPassLedger()
	if l.skipBind(rp, pipe2, bg0, nil, nil, vert) {
		t.Fatal("new pass generation must take full bind path")
	}
	l.noteBind(rp, pipe2, bg0, nil, nil, vert)
	if l.skipBind(rp2, pipe2, bg0, nil, nil, vert) {
		t.Fatal("different pass encoder must take full bind path")
	}
	if l.skipBind(rp, nil, bg0, nil, nil, vert) {
		t.Fatal("nil pipeline must take full bind path")
	}
}

func TestPassBindLedger_NilSafe(t *testing.T) {
	var l *PassBindLedger
	l.BeginPassLedger()
	l.Invalidate()
	l.noteBind(nil, nil, nil, nil, nil, nil)
	if l.skipBind(nil, nil, nil, nil, nil, nil) {
		t.Fatal("nil ledger must never skip")
	}
}
