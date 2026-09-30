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
	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/step"
)

// MaxPendingPackets caps queued net packets. Injects past the cap report
// OutOfMemory and store nothing, so a dead network never grows the loop.
const MaxPendingPackets = 1024

// System is one registered tick worker: a name plus its per-tick update.
// The update receives the fixed tick width. Systems report nothing back;
// they keep their own errors. A nil update never registers.
type System struct {
	name   string
	update func(dt core.Duration)
}

// Name returns the registration name.
func (s System) Name() string { return s.name }

// Loop is the generic runway: a fixed beat, an ordered system table, a
// net-packet socket, and a submit hook. The zero value holds nothing;
// New is the only way in. Not safe for concurrent use; a nil *Loop never
// panics: Frame parks at 0,0, getters park at zero, void setters are
// no-ops, fallible calls report InvalidArg.
type Loop struct {
	fixed      step.Fixed
	systems    []System
	packets    [][]byte
	packetHook func([][]byte)
	submitHook func(alpha float64)
}

// New builds a runway on a fixed tick width dt. Non-positive dt is a
// core InvalidArg error and returns nil, never a guessed beat.
func New(dt core.Duration) (*Loop, error) {
	fx, err := step.NewFixed(dt)
	if err != nil {
		return nil, err
	}
	return &Loop{fixed: fx}, nil
}

// AddSystem registers one system at the end of the table. Empty names,
// duplicate names, and nil updates are core InvalidArg errors and store
// nothing. A nil loop reports InvalidArg.
func (l *Loop) AddSystem(name string, update func(dt core.Duration)) error {
	const op = "loop.AddSystem"
	if l == nil {
		return core.InvalidArg(op, "loop")
	}
	if name == "" || update == nil {
		return core.InvalidArg(op, "system")
	}
	for _, s := range l.systems {
		if s.name == name {
			return core.InvalidArg(op, "duplicate")
		}
	}
	l.systems = append(l.systems, System{name: name, update: update})
	return nil
}

// Systems returns the registration names in run order. A nil loop
// returns nil. The result is a fresh slice.
func (l *Loop) Systems() []string {
	if l == nil {
		return nil
	}
	out := make([]string, len(l.systems))
	for i, s := range l.systems {
		out[i] = s.name
	}
	return out
}

// SystemCount counts registered systems. A nil loop reports 0.
func (l *Loop) SystemCount() int {
	if l == nil {
		return 0
	}
	return len(l.systems)
}

// SetPacketHook plugs the netcode drain: before every tick with queued
// packets, the loop hands the queue over and clears it. A nil hook
// stores packets without draining. Calling with nil clears the hook.
// A nil loop is a no-op. The hook must not retain the slice.
func (l *Loop) SetPacketHook(fn func([][]byte)) {
	if l == nil {
		return
	}
	l.packetHook = fn
}

// InjectPacket queues one net packet for the next tick. Empty packets
// are InvalidArg; a full queue (MaxPendingPackets) is OutOfMemory; a
// nil loop is InvalidArg. The bytes are cloned, never aliased.
func (l *Loop) InjectPacket(pkt []byte) error {
	const op = "loop.InjectPacket"
	if l == nil {
		return core.InvalidArg(op, "loop")
	}
	if len(pkt) == 0 {
		return core.InvalidArg(op, "packet")
	}
	if len(l.packets) >= MaxPendingPackets {
		return core.OutOfMemory(op, "packets")
	}
	l.packets = append(l.packets, append([]byte(nil), pkt...))
	return nil
}

// PendingPackets counts queued, undrained packets. A nil loop reports 0.
func (l *Loop) PendingPackets() int {
	if l == nil {
		return 0
	}
	return len(l.packets)
}

// SetSubmit plugs the render handoff: after every Frame the loop calls
// it once with the blend factor, even on zero-tick frames. Calling with
// nil clears the hook. A nil loop is a no-op.
func (l *Loop) SetSubmit(fn func(alpha float64)) {
	if l == nil {
		return
	}
	l.submitHook = fn
}

// Frame runs one real frame: split into fixed ticks, drain packets then
// run every system in order per tick, then submit the blend factor.
// Returns the tick count and the blend factor. Bad frame widths follow
// the fixed beat (park at 0, clamp at step.MaxFrame). A nil loop
// returns 0, 0 and calls nothing.
func (l *Loop) Frame(frame core.Duration) (ticks int, alpha float64) {
	if l == nil {
		return 0, 0
	}
	dt := l.fixed.Dt()
	ticks = l.fixed.Advance(frame)
	for i := 0; i < ticks; i++ {
		if l.packetHook != nil && len(l.packets) > 0 {
			l.packetHook(l.packets)
			l.packets = nil
		}
		for _, s := range l.systems {
			s.update(dt)
		}
	}
	alpha = l.fixed.Alpha()
	if l.submitHook != nil {
		l.submitHook(alpha)
	}
	return ticks, alpha
}

// Dt returns the fixed tick width. A nil loop reports 0.
func (l *Loop) Dt() core.Duration {
	if l == nil {
		return 0
	}
	return l.fixed.Dt()
}

// Steps counts ticks run so far. A nil loop reports 0.
func (l *Loop) Steps() uint64 {
	if l == nil {
		return 0
	}
	return l.fixed.Steps()
}

// Elapsed returns simulated time run so far. A nil loop reports 0.
func (l *Loop) Elapsed() core.Duration {
	if l == nil {
		return 0
	}
	return l.fixed.Elapsed()
}

// Alpha returns the current render blend factor. A nil loop reports 0.
func (l *Loop) Alpha() float64 {
	if l == nil {
		return 0
	}
	return l.fixed.Alpha()
}
