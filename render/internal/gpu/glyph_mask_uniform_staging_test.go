//go:build !nogpu

package gpu

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/hal/noop"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

// deferringWriteQueue mirrors the GLES queue: WriteBuffer only stages the
// payload and the bytes are read at Submit. It deliberately keeps the caller's
// slice instead of copying, so a renderer that reuses one scratch buffer for
// several uploads hands every upload the last writer's bytes.
type deferringWriteQueue struct {
	*noop.Queue
	staged       []deferredWrite
	materialized map[hal.Buffer][]byte
}

type deferredWrite struct {
	buf    hal.Buffer
	offset uint64
	data   []byte
}

func newDeferringWriteQueue() *deferringWriteQueue {
	return &deferringWriteQueue{
		Queue:        &noop.Queue{},
		materialized: make(map[hal.Buffer][]byte),
	}
}

func (q *deferringWriteQueue) WriteBuffer(buf hal.Buffer, offset uint64, data []byte) error {
	q.staged = append(q.staged, deferredWrite{buf: buf, offset: offset, data: data})
	return nil
}

func (q *deferringWriteQueue) Submit(cbs ...hal.CommandBuffer) (uint64, error) {
	for _, w := range q.staged {
		dst := q.materialized[w.buf]
		if need := int(w.offset) + len(w.data); len(dst) < need {
			dst = append(dst, make([]byte, need-len(dst))...)
		}
		copy(dst[w.offset:], w.data)
		q.materialized[w.buf] = dst
	}
	q.staged = q.staged[:0]
	return q.Queue.Submit(cbs...)
}

func newDeferringSession(t *testing.T) (*GPURenderSession, *deferringWriteQueue) {
	t.Helper()
	instance, err := noop.NewBackend().CreateInstance(&hal.InstanceDescriptor{})
	if err != nil {
		t.Fatalf("noop instance: %v", err)
	}
	adapter, err := instance.RequestAdapter(&hal.RequestAdapterOptions{})
	if err != nil {
		t.Fatalf("noop adapter: %v", err)
	}
	device, err := adapter.RequestDevice(&hal.DeviceDescriptor{})
	if err != nil {
		t.Fatalf("noop device: %v", err)
	}
	queue := newDeferringWriteQueue()
	s := NewGPURenderSession(device, queue, 1)
	t.Cleanup(s.Destroy)
	return s, queue
}

func glyphUniformColor(data []byte) [4]float32 {
	var c [4]float32
	for i := range 4 {
		c[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[64+i*4:]))
	}
	return c
}

// TestGlyphMaskUniformsSurviveDeferredWrites pins the staging contract: every
// batch's uniform upload must own its bytes until the queue drains. Sharing
// one scratch across batches made GLES upload the last batch's transform and
// colour to every batch (all grayscale text took the HUD green, retained
// layers drew text with the wrong CTM).
func TestGlyphMaskUniformsSurviveDeferredWrites(t *testing.T) {
	s, queue := newDeferringSession(t)

	colors := [4][4]float32{
		{0.10, 0.20, 0.30, 1},
		{0.90, 0.11, 0.22, 1},
		{0.25, 0.85, 0.45, 1},
		{0.05, 0.06, 0.07, 0.5},
	}
	transforms := [4]render.Matrix{
		render.Identity(),
		{A: 1, E: 1, C: 12, F: 34},
		{A: 2, E: 2, C: -5, F: 7},
		{A: 0.5, E: 0.5, C: 100, F: 200},
	}
	batches := make([]GlyphMaskBatch, len(colors))
	for i := range batches {
		batches[i] = GlyphMaskBatch{
			Quads:     make([]GlyphMaskQuad, 1),
			Transform: transforms[i],
			Color:     colors[i],
			IsLCD:     i%2 == 1,
		}
	}

	s.ensureGlyphMaskBatchPools(len(batches))
	if _, err := s.buildGlyphMaskDrawCalls(batches, 800, 600); err != nil {
		t.Fatalf("buildGlyphMaskDrawCalls: %v", err)
	}
	if _, err := queue.Submit(); err != nil {
		t.Fatalf("submit: %v", err)
	}

	// Column-major transform: C/F land at floats 12/13. The upload composes
	// the batch CTM with the viewport ortho projection.
	ortho := render.Matrix{
		A: 2.0 / 800, B: 0, C: -1.0,
		D: 0, E: -2.0 / 600, F: 1.0,
	}
	for i := range batches {
		buf := s.glyphMaskUniformBufs[i]
		data := queue.materialized[buf]
		if len(data) < int(glyphMaskUniformSize) {
			t.Fatalf("batch %d: uniform buffer has %d materialized bytes, want >= %d",
				i, len(data), glyphMaskUniformSize)
		}
		if got := glyphUniformColor(data); got != colors[i] {
			t.Errorf("batch %d color = %v, want %v (deferred queue saw another batch's scratch)",
				i, got, colors[i])
		}
		want := ortho.Multiply(transforms[i])
		gotC := math.Float32frombits(binary.LittleEndian.Uint32(data[48:]))
		gotF := math.Float32frombits(binary.LittleEndian.Uint32(data[52:]))
		if gotC != float32(want.C) || gotF != float32(want.F) {
			t.Errorf("batch %d translate = (%v,%v), want (%v,%v) (deferred queue saw another batch's scratch)",
				i, gotC, gotF, want.C, want.F)
		}
	}
}

// TestClipUniformsSurviveDeferredWrites pins the same staging contract for the
// clip pool: one frame pushes many distinct ClipParams into per-index uniform
// buffers, and a shared scratch made every clip read the last group's rect.
func TestClipUniformsSurviveDeferredWrites(t *testing.T) {
	s, queue := newDeferringSession(t)

	params := []ClipParams{
		{RectX1: 10, RectY1: 20, RectX2: 110, RectY2: 220, Radius: 4, Enabled: 1},
		{RectX1: 300, RectY1: 40, RectX2: 700, RectY2: 500, Radius: 12, Enabled: 1},
	}
	for range params {
		buf, err := s.device.CreateBuffer(&hal.BufferDescriptor{
			Size:  clipParamsSize,
			Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
		})
		if err != nil {
			t.Fatalf("clip buffer: %v", err)
		}
		bg, err := s.device.CreateBindGroup(&hal.BindGroupDescriptor{})
		if err != nil {
			t.Fatalf("clip bind group: %v", err)
		}
		s.clipUniformPool = append(s.clipUniformPool, buf)
		s.clipBindPool = append(s.clipBindPool, bg)
	}

	for i := range params {
		if _, err := s.getClipBindGroup(&params[i]); err != nil {
			t.Fatalf("getClipBindGroup %d: %v", i, err)
		}
	}
	if _, err := queue.Submit(); err != nil {
		t.Fatalf("submit: %v", err)
	}

	for i := range params {
		data := queue.materialized[s.clipUniformPool[i]]
		if len(data) < clipParamsSize {
			t.Fatalf("clip %d: %d materialized bytes, want %d", i, len(data), clipParamsSize)
		}
		got := params[i]
		got.RectX1 = math.Float32frombits(binary.LittleEndian.Uint32(data[0:]))
		got.RectY1 = math.Float32frombits(binary.LittleEndian.Uint32(data[4:]))
		got.RectX2 = math.Float32frombits(binary.LittleEndian.Uint32(data[8:]))
		got.RectY2 = math.Float32frombits(binary.LittleEndian.Uint32(data[12:]))
		got.Radius = math.Float32frombits(binary.LittleEndian.Uint32(data[16:]))
		if got.RectX1 != params[i].RectX1 || got.RectY1 != params[i].RectY1 ||
			got.RectX2 != params[i].RectX2 || got.RectY2 != params[i].RectY2 ||
			got.Radius != params[i].Radius {
			t.Errorf("clip %d = (%v,%v,%v,%v,r%v), want (%v,%v,%v,%v,r%v) (deferred queue saw another clip's scratch)",
				i, got.RectX1, got.RectY1, got.RectX2, got.RectY2, got.Radius,
				params[i].RectX1, params[i].RectY1, params[i].RectX2, params[i].RectY2, params[i].Radius)
		}
	}
}
