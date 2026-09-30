// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
)

// arrayBufferTarget / elementBufferTarget mirror the GL enums used by the
// vertex/index setup path (kept here so the generic bind gate stays exact).
const (
	arrayBufferTarget   = uint32(gl.ARRAY_BUFFER)
	elementBufferTarget = uint32(gl.ELEMENT_ARRAY_BUFFER)
)

// glExecState tracks GL state within a single Queue.Submit execution so
// repeated setup calls collapse into one driver call. Scoped per Submit
// (fresh per Lock hold): setup outside Submit (device/resource init,
// direct actx users, Present surface switch) never poisons it.
type glExecState struct {
	progSet bool
	prog    uint32

	cullSet     bool
	cullEnabled bool
	cullFace    uint32

	frontSet  bool
	frontFace uint32

	depthSet     bool
	depthEnabled bool
	depthMask    bool
	depthFunc    uint32

	stencilSet     bool
	stencilEnabled bool
	stFrontFunc    uint32
	stFrontRef     int32
	stFrontMask    uint32
	stBackFunc     uint32
	stBackRef      int32
	stBackMask     uint32
	stFrontFailOp  uint32
	stFrontZFailOp uint32
	stFrontPassOp  uint32
	stBackFailOp   uint32
	stBackZFailOp  uint32
	stBackPassOp   uint32
	stFrontWMask   uint32
	stBackWMask    uint32

	colorMaskSet bool
	colorMask    [4]bool

	blendSet     bool
	blendEnabled bool
	blendSrcRGB  uint32
	blendDstRGB  uint32
	blendSrcA    uint32
	blendDstA    uint32
	blendEqRGB   uint32
	blendEqA     uint32

	vaoSet bool
	vao    uint32

	// Generic per-target binds (ARRAY/ ELEMENT/ UNIFORM/ COPY/ PIXEL_...).
	// Indexed Base/Range binds live in indexedBind below (separate GL state).
	genericBind map[uint32]uint32

	activeSet  bool
	activeUnit uint32

	texSet  bool
	texBind [maxTextureSlots]uint32

	samplerSet  bool
	samplerBind [maxTextureSlots]uint32

	scissorSet     bool
	scissorEnabled bool
	scissor        [4]int32

	viewportSet bool
	viewport    [4]int32
	depthNear   float64
	depthFar    float64

	blendColorSet bool
	blendColor    [4]float32

	fbSet     bool
	fb        uint32
	fbRead    uint32
	fbReadSet bool
	fbDraw    uint32
	fbDrawSet bool

	attribs map[uint32]attribState

	// Indexed buffer binds (UNIFORM_BUFFER / SHADER_STORAGE_BUFFER per index).
	indexedBind map[uint64]indexedBindVal
}

// indexedBindVal records one indexed buffer binding including range.
type indexedBindVal struct {
	id     uint32
	offset int
	size   int
}

type attribState struct {
	enabled    bool
	size       int32
	typ        uint32
	normalized bool
	stride     int32
	offset     uintptr
	buf        uint32
	divisor    uint32
}

func newGLExecState() *glExecState {
	return &glExecState{
		attribs:     make(map[uint32]attribState, 16),
		indexedBind: make(map[uint64]indexedBindVal, 8),
		genericBind: make(map[uint32]uint32, 8),
	}
}

// useProgram reports whether glUseProgram is needed.
func (s *glExecState) useProgram(id uint32) bool {
	if s.progSet && s.prog == id {
		return false
	}
	s.progSet, s.prog = true, id
	return true
}

// setCull gates Enable/Disable(CULL_FACE) + CullFace.
func (s *glExecState) setCull(enabled bool, face uint32) (needToggle, needFace bool) {
	if !s.cullSet || s.cullEnabled != enabled {
		s.cullSet, s.cullEnabled = true, enabled
		needToggle = true
	}
	if enabled && s.cullFace != face {
		s.cullFace = face
		needFace = true
	}
	return needToggle, needFace
}

// setFrontFace gates FrontFace.
func (s *glExecState) setFrontFace(face uint32) bool {
	if s.frontSet && s.frontFace == face {
		return false
	}
	s.frontSet, s.frontFace = true, face
	return true
}

// setDepth gates the depth-test cluster.
func (s *glExecState) setDepth(enabled, mask bool, fn uint32) (t, m, f bool) {
	if !s.depthSet || s.depthEnabled != enabled {
		s.depthSet, s.depthEnabled = true, enabled
		t = true
	}
	if enabled {
		if s.depthMask != mask {
			s.depthMask, m = mask, true
		}
		if s.depthFunc != fn {
			s.depthFunc, f = fn, true
		}
	}
	return t, m, f
}

// setStencilTest gates Enable/Disable(STENCIL_TEST).
func (s *glExecState) setStencilTest(enabled bool) bool {
	if s.stencilSet && s.stencilEnabled == enabled {
		return false
	}
	s.stencilSet, s.stencilEnabled = true, enabled
	return true
}

// setStencilFunc gates one StencilFuncSeparate face.
func (s *glExecState) setStencilFunc(front bool, fn uint32, ref int32, mask uint32) bool {
	if front {
		if s.stencilSet && s.stFrontFunc == fn && s.stFrontRef == ref && s.stFrontMask == mask {
			return false
		}
		s.stFrontFunc, s.stFrontRef, s.stFrontMask = fn, ref, mask
		return true
	}
	if s.stencilSet && s.stBackFunc == fn && s.stBackRef == ref && s.stBackMask == mask {
		return false
	}
	s.stBackFunc, s.stBackRef, s.stBackMask = fn, ref, mask
	return true
}

// setStencilOp gates one StencilOpSeparate face.
func (s *glExecState) setStencilOp(front bool, fail, zfail, pass uint32) bool {
	if front {
		if s.stencilSet && s.stFrontFailOp == fail && s.stFrontZFailOp == zfail && s.stFrontPassOp == pass {
			return false
		}
		s.stFrontFailOp, s.stFrontZFailOp, s.stFrontPassOp = fail, zfail, pass
		return true
	}
	if s.stencilSet && s.stBackFailOp == fail && s.stBackZFailOp == zfail && s.stBackPassOp == pass {
		return false
	}
	s.stBackFailOp, s.stBackZFailOp, s.stBackPassOp = fail, zfail, pass
	return true
}

// setStencilMask gates one StencilMaskSeparate face.
func (s *glExecState) setStencilMask(front bool, mask uint32) bool {
	if front {
		if s.stencilSet && s.stFrontWMask == mask {
			return false
		}
		s.stFrontWMask = mask
		return true
	}
	if s.stencilSet && s.stBackWMask == mask {
		return false
	}
	s.stBackWMask = mask
	return true
}

// markStencilApplied flips the cluster known bit after a full stencil setup.
func (s *glExecState) markStencilApplied() { s.stencilSet = true }

// setColorMask gates ColorMask.
func (s *glExecState) setColorMask(r, g, b, a bool) bool {
	if s.colorMaskSet && s.colorMask == [4]bool{r, g, b, a} {
		return false
	}
	s.colorMaskSet, s.colorMask = true, [4]bool{r, g, b, a}
	return true
}

// setBlend gates the blend cluster. blend==false means Disable(BLEND).
func (s *glExecState) setBlend(blend bool, srcRGB, dstRGB, srcA, dstA, eqRGB, eqA uint32) (t, f, e bool) {
	if !s.blendSet || s.blendEnabled != blend {
		s.blendSet, s.blendEnabled = true, blend
		t = true
	}
	if blend {
		if s.blendSrcRGB != srcRGB || s.blendDstRGB != dstRGB || s.blendSrcA != srcA || s.blendDstA != dstA {
			s.blendSrcRGB, s.blendDstRGB, s.blendSrcA, s.blendDstA = srcRGB, dstRGB, srcA, dstA
			f = true
		}
		if s.blendEqRGB != eqRGB || s.blendEqA != eqA {
			s.blendEqRGB, s.blendEqA = eqRGB, eqA
			e = true
		}
	}
	return t, f, e
}

// bindVAO gates BindVertexArray.
func (s *glExecState) bindVAO(id uint32) bool {
	if s.vaoSet && s.vao == id {
		return false
	}
	s.vaoSet, s.vao = true, id
	return true
}

// bindArray gates ARRAY_BUFFER binds (vertex setup path).
func (s *glExecState) bindArray(id uint32) bool {
	return s.bindGeneric(arrayBufferTarget, id)
}

// bindElement gates ELEMENT_ARRAY_BUFFER binds.
func (s *glExecState) bindElement(id uint32) bool {
	return s.bindGeneric(elementBufferTarget, id)
}

// bindGeneric gates BindBuffer per target.
func (s *glExecState) bindGeneric(target, id uint32) bool {
	if old, ok := s.genericBind[target]; ok && old == id {
		return false
	}
	s.genericBind[target] = id
	return true
}

// activeTexture gates ActiveTexture.
func (s *glExecState) activeTexture(unit uint32) bool {
	if s.activeSet && s.activeUnit == unit {
		return false
	}
	s.activeSet, s.activeUnit = true, unit
	return true
}

// bindTexture gates BindTexture per unit.
func (s *glExecState) bindTexture(unit, id uint32) bool {
	if unit >= maxTextureSlots {
		return true
	}
	if s.texSet && s.texBind[unit] == id {
		return false
	}
	s.texSet, s.texBind[unit] = true, id
	return true
}

// bindSampler gates BindSampler per unit.
func (s *glExecState) bindSampler(unit, id uint32) bool {
	if unit >= maxTextureSlots {
		return true
	}
	if s.samplerSet && s.samplerBind[unit] == id {
		return false
	}
	s.samplerSet, s.samplerBind[unit] = true, id
	return true
}

// setScissorTest gates Enable/Disable(SCISSOR_TEST).
func (s *glExecState) setScissorTest(enabled bool) bool {
	if s.scissorSet && s.scissorEnabled == enabled {
		return false
	}
	s.scissorSet, s.scissorEnabled = true, enabled
	return true
}

// setScissor gates Scissor box.
func (s *glExecState) setScissor(x, y, w, h int32) bool {
	box := [4]int32{x, y, w, h}
	if s.scissorSet && s.scissor == box {
		return false
	}
	s.scissor = box
	return true
}

// setViewport gates Viewport + DepthRange.
func (s *glExecState) setViewport(x, y, w, h int32, near, far float64) bool {
	box := [4]int32{x, y, w, h}
	if s.viewportSet && s.viewport == box && s.depthNear == near && s.depthFar == far {
		return false
	}
	s.viewportSet, s.viewport, s.depthNear, s.depthFar = true, box, near, far
	return true
}

// setBlendColor gates BlendColor.
func (s *glExecState) setBlendColor(r, g, b, a float32) bool {
	c := [4]float32{r, g, b, a}
	if s.blendColorSet && s.blendColor == c {
		return false
	}
	s.blendColorSet, s.blendColor = true, c
	return true
}

// bindFramebuffer gates one framebuffer target.
func (s *glExecState) bindFramebuffer(fbo uint32) bool {
	if s.fbSet && s.fb == fbo {
		return false
	}
	s.fbSet, s.fb = true, fbo
	return true
}

// bindRead gates READ_FRAMEBUFFER binds (resolve/blit paths).
func (s *glExecState) bindRead(fbo uint32) bool {
	if s.fbReadSet && s.fbRead == fbo {
		return false
	}
	s.fbRead, s.fbReadSet = fbo, true
	return true
}

// bindDraw gates DRAW_FRAMEBUFFER binds (resolve/blit paths).
func (s *glExecState) bindDraw(fbo uint32) bool {
	if s.fbDrawSet && s.fbDraw == fbo {
		return false
	}
	s.fbDraw, s.fbDrawSet = fbo, true
	return true
}

// bindIndexed gates BindBufferBase/Range per (target, index, id, range).
func (s *glExecState) bindIndexed(target, index, id uint32, offset, size int) bool {
	key := uint64(target)<<32 | uint64(index)
	want := indexedBindVal{id: id, offset: offset, size: size}
	if old, ok := s.indexedBind[key]; ok && old == want {
		return false
	}
	s.indexedBind[key] = want
	return true
}

// markDepthMask records an out-of-band DepthMask write (clear path).
func (s *glExecState) markDepthMask(mask bool) { s.depthMask = mask }

// markStencilWMask records an out-of-band stencil write-mask change.
func (s *glExecState) markStencilWMask(front bool, mask uint32) {
	if front {
		s.stFrontWMask = mask
		return
	}
	s.stBackWMask = mask
}

// setAttrib gates one vertex-attrib slot setup.
func (s *glExecState) setAttrib(loc uint32, a attribState) (en, ptr, div bool) {
	old, ok := s.attribs[loc]
	if !ok {
		s.attribs[loc] = a
		return true, true, true
	}
	if old.enabled != a.enabled {
		en = true
	}
	if old.size != a.size || old.typ != a.typ || old.normalized != a.normalized ||
		old.stride != a.stride || old.offset != a.offset || old.buf != a.buf {
		ptr = true
	}
	if old.divisor != a.divisor {
		div = true
	}
	if en || ptr || div {
		s.attribs[loc] = a
	}
	return en, ptr, div
}
