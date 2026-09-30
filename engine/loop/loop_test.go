//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package loop

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/engine/core"
)

type loopCases struct {
	Version       string   `json:"version"`
	DtMs          int64    `json:"dt_ms"`
	Systems       []string `json:"systems"`
	FramesMs      []int64  `json:"frames_ms"`
	WantTicks     []int    `json:"want_ticks"`
	WantSteps     uint64   `json:"want_steps"`
	WantElapsedMs int64    `json:"want_elapsed_ms"`
	WantAlpha     float64  `json:"want_alpha"`
}

func loadLoopCases(t *testing.T) loopCases {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "loop_cases.json"))
	if err != nil {
		t.Fatalf("read loop_cases.json: %v", err)
	}
	var c loopCases
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode loop_cases.json: %v", err)
	}
	if c.DtMs <= 0 || len(c.Systems) == 0 || len(c.FramesMs) != len(c.WantTicks) {
		t.Fatal("loop_cases.json is missing dt/systems/frames")
	}
	return c
}

// orderLog records system call order plus per-system call counts.
type orderLog struct {
	order []string
	calls map[string]int
}

func (o *orderLog) attach(l *Loop, systems []string) {
	o.calls = map[string]int{}
	for _, name := range systems {
		name := name
		if err := l.AddSystem(name, func(dt core.Duration) {
			o.order = append(o.order, name)
			o.calls[name]++
		}); err != nil {
			panic(name + ": " + err.Error())
		}
	}
}

// A: frozen frames split into frozen ticks, systems run in order,
// packets drain before the tick, submit sees the blend factor.
func TestLoopFromCases(t *testing.T) {
	c := loadLoopCases(t)
	dt := core.Duration(c.DtMs) * core.Millisecond
	l, err := New(dt)
	if err != nil {
		t.Fatalf("New(%dms): %v", c.DtMs, err)
	}
	var log orderLog
	log.attach(l, c.Systems)
	if got := l.Systems(); len(got) != len(c.Systems) {
		t.Fatalf("Systems = %v, want %v", got, c.Systems)
	} else {
		for i := range got {
			if got[i] != c.Systems[i] {
				t.Fatalf("Systems = %v, want %v", got, c.Systems)
			}
		}
	}
	if l.SystemCount() != len(c.Systems) {
		t.Fatalf("SystemCount = %d, want %d", l.SystemCount(), len(c.Systems))
	}
	var submits []float64
	l.SetSubmit(func(alpha float64) { submits = append(submits, alpha) })
	var drained [][]byte
	l.SetPacketHook(func(pkts [][]byte) { drained = append(drained, pkts...) })
	// Two packets go in before the third frame; they must drain on its
	// first tick, ahead of every system call of that tick.
	var totalTicks int
	for i, fms := range c.FramesMs {
		if i == 2 {
			if err := l.InjectPacket([]byte{0x01}); err != nil {
				t.Fatalf("InjectPacket: %v", err)
			}
			if err := l.InjectPacket([]byte{0x02, 0x03}); err != nil {
				t.Fatalf("InjectPacket: %v", err)
			}
			if l.PendingPackets() != 2 {
				t.Fatalf("PendingPackets = %d, want 2", l.PendingPackets())
			}
		}
		before := len(log.order)
		got, alpha := l.Frame(core.Duration(fms) * core.Millisecond)
		if got != c.WantTicks[i] {
			t.Errorf("frame[%d] ticks = %d, want %d", i, got, c.WantTicks[i])
		}
		if math.IsNaN(alpha) || alpha < 0 || alpha >= 1 {
			t.Errorf("frame[%d] alpha = %v, want [0,1)", i, alpha)
		}
		if i == 2 {
			if len(drained) != 2 {
				t.Fatalf("drained %d packets, want 2", len(drained))
			}
			if l.PendingPackets() != 0 {
				t.Fatalf("PendingPackets = %d, want 0", l.PendingPackets())
			}
			// Packets drain before the systems of that tick.
			if len(log.order) < before+len(c.Systems) {
				t.Fatal("no system ran on the draining tick")
			}
		}
		totalTicks += got
	}
	if uint64(totalTicks) != c.WantSteps || l.Steps() != c.WantSteps {
		t.Errorf("steps = %d/%d, want %d", totalTicks, l.Steps(), c.WantSteps)
	}
	if l.Elapsed() != core.Duration(c.WantElapsedMs)*core.Millisecond {
		t.Errorf("elapsed = %v, want %dms", l.Elapsed(), c.WantElapsedMs)
	}
	if math.Abs(l.Alpha()-c.WantAlpha) > 1e-12 {
		t.Errorf("alpha = %.17g, want %.17g", l.Alpha(), c.WantAlpha)
	}
	if len(submits) != len(c.FramesMs) {
		t.Fatalf("submit called %d times, want %d", len(submits), len(c.FramesMs))
	}
	if math.Abs(submits[len(submits)-1]-c.WantAlpha) > 1e-12 {
		t.Errorf("last submit alpha = %.17g, want %.17g", submits[len(submits)-1], c.WantAlpha)
	}
	// Every tick ran every system once, in registration order.
	if len(log.order) != int(c.WantSteps)*len(c.Systems) {
		t.Fatalf("system calls = %d, want %d", len(log.order), int(c.WantSteps)*len(c.Systems))
	}
	for i, name := range log.order {
		if want := c.Systems[i%len(c.Systems)]; name != want {
			t.Fatalf("call[%d] = %q, want %q", i, name, want)
		}
	}
	for _, name := range c.Systems {
		if log.calls[name] != int(c.WantSteps) {
			t.Errorf("system %q ran %d times, want %d", name, log.calls[name], c.WantSteps)
		}
	}
	if l.Dt() != dt {
		t.Errorf("Dt = %v, want %v", l.Dt(), dt)
	}
}

// B: bad construction and bad calls fail closed with codes, never panic.
func TestLoopBadInputs(t *testing.T) {
	if _, err := New(0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("New(0) code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := New(-5 * core.Millisecond); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("New(-5ms) code = %v, want invalid-arg", core.CodeOf(err))
	}
	l, err := New(16 * core.Millisecond)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := l.AddSystem("", func(dt core.Duration) {}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty name code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := l.AddSystem("ai", nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil update code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := l.AddSystem("ai", func(dt core.Duration) {}); err != nil {
		t.Fatalf("AddSystem: %v", err)
	}
	if err := l.AddSystem("ai", func(dt core.Duration) {}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("duplicate code = %v, want invalid-arg", core.CodeOf(err))
	}
	if l.SystemCount() != 1 {
		t.Errorf("SystemCount = %d, want 1 (bad adds store nothing)", l.SystemCount())
	}
	if err := l.InjectPacket(nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty packet code = %v, want invalid-arg", core.CodeOf(err))
	}
	// Full queue refuses with out-of-memory and keeps the old packets.
	for i := 0; i < MaxPendingPackets; i++ {
		if err := l.InjectPacket([]byte{byte(i)}); err != nil {
			t.Fatalf("fill[%d]: %v", i, err)
		}
	}
	if err := l.InjectPacket([]byte{0xFF}); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("full queue code = %v, want out-of-memory", core.CodeOf(err))
	}
	if l.PendingPackets() != MaxPendingPackets {
		t.Errorf("PendingPackets = %d, want %d", l.PendingPackets(), MaxPendingPackets)
	}
	// Injected bytes are cloned: mutating the source changes nothing.
	src := []byte{0xAA}
	l2, _ := New(16 * core.Millisecond)
	var got []byte
	l2.SetPacketHook(func(pkts [][]byte) { got = append(got, pkts[0]...) })
	if err := l2.InjectPacket(src); err != nil {
		t.Fatalf("InjectPacket: %v", err)
	}
	src[0] = 0xBB
	l2.Frame(16 * core.Millisecond)
	if len(got) != 1 || got[0] != 0xAA {
		t.Errorf("packet aliased source: %v", got)
	}
	// Nil loop never panics.
	var nilLoop *Loop
	if n, a := nilLoop.Frame(16 * core.Millisecond); n != 0 || a != 0 {
		t.Errorf("nil Frame = %d,%v, want 0,0", n, a)
	}
	if nilLoop.Systems() != nil || nilLoop.SystemCount() != 0 || nilLoop.PendingPackets() != 0 {
		t.Error("nil getters did not park at zero")
	}
	if nilLoop.Dt() != 0 || nilLoop.Steps() != 0 || nilLoop.Elapsed() != 0 || nilLoop.Alpha() != 0 {
		t.Error("nil clock getters did not park at zero")
	}
	if err := nilLoop.AddSystem("x", func(dt core.Duration) {}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil AddSystem code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := nilLoop.InjectPacket([]byte{1}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil InjectPacket code = %v, want invalid-arg", core.CodeOf(err))
	}
	nilLoop.SetPacketHook(func([][]byte) {})
	nilLoop.SetSubmit(func(float64) {})
	// Negative frames park at zero ticks and still submit.
	l3, _ := New(16 * core.Millisecond)
	submits := 0
	l3.SetSubmit(func(float64) { submits++ })
	if n, _ := l3.Frame(-100 * core.Millisecond); n != 0 {
		t.Errorf("negative frame ticks = %d, want 0", n)
	}
	if submits != 1 {
		t.Errorf("submit called %d times on zero-tick frame, want 1", submits)
	}
}

// C: same frames twice replay identically (tick counts plus call order).
func TestLoopReplayIdentical(t *testing.T) {
	c := loadLoopCases(t)
	run := func() (ticks []int, order []string, submits int) {
		l, err := New(core.Duration(c.DtMs) * core.Millisecond)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		var log orderLog
		log.attach(l, c.Systems)
		l.SetSubmit(func(float64) { submits++ })
		for _, fms := range c.FramesMs {
			n, _ := l.Frame(core.Duration(fms) * core.Millisecond)
			ticks = append(ticks, n)
		}
		return ticks, log.order, submits
	}
	t1, o1, s1 := run()
	t2, o2, s2 := run()
	if len(t1) != len(t2) || len(o1) != len(o2) || s1 != s2 {
		t.Fatal("replay lengths diverged")
	}
	for i := range t1 {
		if t1[i] != t2[i] {
			t.Fatalf("replay ticks[%d] = %d vs %d", i, t1[i], t2[i])
		}
	}
	for i := range o1 {
		if o1[i] != o2[i] {
			t.Fatalf("replay order[%d] = %q vs %q", i, o1[i], o2[i])
		}
	}
}

// D: frame cost is on the books.
func TestLoopFrameCost(t *testing.T) {
	l, err := New(16 * core.Millisecond)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, name := range []string{"a", "b", "c"} {
		if err := l.AddSystem(name, func(dt core.Duration) {}); err != nil {
			t.Fatalf("AddSystem: %v", err)
		}
	}
	const n = 10000
	start := time.Now()
	var ticks int
	for i := 0; i < n; i++ {
		got, _ := l.Frame(16 * core.Millisecond)
		ticks += got
	}
	cost := time.Since(start)
	t.Logf("%d frames x3 systems: %v total, %.1fns/frame", n, cost, float64(cost.Nanoseconds())/n)
	if ticks != n {
		t.Errorf("ticks = %d, want %d", ticks, n)
	}
	if cost > 5*time.Second {
		t.Errorf("frame cost %v exceeds 5s budget", cost)
	}
}

// E: long runs stay on the ledger with no drift.
func TestLoopLongRun(t *testing.T) {
	mk := func() *Loop {
		l, err := New(16 * core.Millisecond)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		for _, name := range []string{"x", "y"} {
			name := name
			if err := l.AddSystem(name, func(dt core.Duration) {}); err != nil {
				t.Fatalf("AddSystem: %v", err)
			}
		}
		return l
	}
	a, b := mk(), mk()
	const n = 200000
	for i := 0; i < n; i++ {
		na, _ := a.Frame(16 * core.Millisecond)
		nb, _ := b.Frame(16 * core.Millisecond)
		if na != nb || na != 1 {
			t.Fatalf("step %d: ticks %d vs %d", i, na, nb)
		}
	}
	if a.Steps() != n || b.Steps() != n {
		t.Fatalf("steps %d/%d, want %d", a.Steps(), b.Steps(), n)
	}
	if a.Elapsed() != b.Elapsed() || a.Elapsed() != core.Duration(n*16)*core.Millisecond {
		t.Errorf("elapsed %v/%v diverged", a.Elapsed(), b.Elapsed())
	}
	if a.Alpha() != 0 || b.Alpha() != 0 {
		t.Errorf("alpha %v/%v, want 0 on exact ticks", a.Alpha(), b.Alpha())
	}
}

// F: frozen file pins the runway; drift fails here, not in a window.
func TestLoopCasesFrozen(t *testing.T) {
	c := loadLoopCases(t)
	if c.Version != "1.0" {
		t.Errorf("version = %q, want 1.0", c.Version)
	}
	sum := 0
	for _, w := range c.WantTicks {
		sum += w
	}
	if uint64(sum) != c.WantSteps {
		t.Errorf("want_ticks sum = %d, want_steps = %d", sum, c.WantSteps)
	}
	if c.WantAlpha < 0 || c.WantAlpha >= 1 {
		t.Errorf("want_alpha = %v, want [0,1)", c.WantAlpha)
	}
}
